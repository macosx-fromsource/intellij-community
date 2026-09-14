package framework

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// Names the harness relies on. They come from deploy/chart, so a rename there
// breaks the harness loudly rather than silently.
const (
	operatorRelease  = "gitlab-operator"
	operatorChart    = "deploy/chart"
	managerContainer = "manager"

	// enableBridgeVar is the runtime half of the double gate on the v2alpha1
	// reconcilers. The build tag is the other half.
	enableBridgeVar = "ENABLE_BRIDGE"

	// v2alpha1RoleFile carries the RBAC the GitLabCore and Siphon reconcilers need
	// on top of what the chart grants. It ships in no release and no bundle, which
	// is why it is a file rather than part of the chart.
	v2alpha1RoleFile = "config/rbac/v2alpha1_manager_role.yaml"

	// crdDirectory holds every definition, including the v2alpha1 ones that reach
	// no release manifest.
	crdDirectory = "config/crd/bases"

	// chartCRDDirectory holds the definitions Helm installs itself, by its own
	// crds/ convention. The harness must not touch those: applying one first
	// takes field ownership of it, and the install then fails on the conflict.
	chartCRDDirectory = "deploy/chart/crds"

	nameOverridePlaceholder = "NAME_OVERRIDE_PLACEHOLDER"
	namespacePlaceholder    = "NAMESPACE_PLACEHOLDER"
)

// The charts the Operator image has to carry for the reconcilers to render
// anything. The harness checks the staging directory rather than the image,
// because the runtime image has no shell to look inside.
const (
	gitlabChartName = "gitlab"
	siphonChartName = "siphon"
)

// RequireOperator makes sure a bridge-enabled Operator is running and really
// reconciling Siphons, once per run.
func (e *Env) RequireOperator(t *testing.T) {
	t.Helper()

	e.mu.Lock()

	if e.operatorRun {
		err := e.operatorErr
		e.mu.Unlock()

		if err != nil {
			e.reportOperatorProblem(t, err)
		}

		return
	}

	e.operatorRun = true
	e.mu.Unlock()

	err := e.deployOperator(e.Ctx)

	e.mu.Lock()
	e.operatorErr = err
	e.mu.Unlock()

	if err != nil {
		e.reportOperatorProblem(t, err)
	}
}

// reportOperatorProblem skips when the cluster is not the harness's to change and
// fails otherwise.
func (e *Env) reportOperatorProblem(t *testing.T, err error) {
	t.Helper()

	var unusable *operatorUnusableError
	if errors.As(err, &unusable) {
		e.Skipf(t, "%s", unusable.Error())

		return
	}

	t.Fatalf("preparing the Operator: %v", err)
}

// operatorUnusableError reports an Operator the harness found but may not
// replace. It is a skip: the default is to adopt what is installed rather than
// upgrade over the release a developer is debugging.
type operatorUnusableError struct {
	reason string
}

func (e *operatorUnusableError) Error() string { return e.reason }

func (h *Harness) deployOperator(ctx context.Context) error {
	root, err := RepositoryRoot()
	if err != nil {
		return err
	}

	if err := h.requireStagedCharts(); err != nil {
		return err
	}

	adopted, err := h.adoptOperator(ctx)
	if err != nil {
		return err
	}

	if !adopted {
		if err := h.installOperator(ctx, root); err != nil {
			return err
		}
	}

	if err := h.awaitManager(ctx); err != nil {
		return err
	}

	return h.verifySiphonController(ctx)
}

// requireStagedCharts checks that the charts the image bakes in are staged, so
// that a render failure names the missing archive instead of surfacing as an
// Operator that cannot reconcile.
func (h *Harness) requireStagedCharts() error {
	if h.Config.ChartsDirectory == "" {
		return &operatorUnusableError{
			reason: "HELM_CHARTS is not set; run the suites with `task e2e-suite`",
		}
	}

	versions := map[string]string{
		gitlabChartName: h.Config.ChartVersion,
		siphonChartName: "",
	}

	for name, version := range versions {
		if version == "" {
			// The Siphon chart version comes from the resource rather than the
			// environment, so the presence of any archive is what matters.
			staged, err := filepath.Glob(filepath.Join(h.Config.ChartsDirectory, name+"-*.tgz"))
			if err != nil || len(staged) == 0 {
				return &operatorUnusableError{
					reason: fmt.Sprintf("no %s chart in %s; run `task retrieve-charts`",
						name, h.Config.ChartsDirectory),
				}
			}

			continue
		}

		if _, err := render.LocateChart(h.Config.ChartsDirectory, name, version); err != nil {
			return &operatorUnusableError{
				reason: fmt.Sprintf("no %s chart %s in %s; run `task retrieve-charts`",
					name, version, h.Config.ChartsDirectory),
			}
		}
	}

	return nil
}

// adoptOperator reports whether an installed release was adopted.
//
// Adopting rather than upgrading is the default on a cluster the harness did not
// create: replacing the Operator a developer is debugging is not a thing a test
// run should do silently.
func (h *Harness) adoptOperator(ctx context.Context) (bool, error) {
	if !h.Config.OperatorReuse {
		return false, nil
	}

	deployed, err := h.helm().Deployed(ctx, operatorRelease, h.Config.OperatorNamespace)
	if err != nil {
		return false, err
	}

	if !deployed {
		return false, nil
	}

	deployment, err := h.managerDeployment(ctx)
	if err != nil {
		return false, err
	}

	container, err := managerContainerOf(deployment)
	if err != nil {
		return false, err
	}

	if !hasEnv(container, enableBridgeVar, trueValue) {
		return false, &operatorUnusableError{
			reason: fmt.Sprintf(
				"the Operator installed in %s is not bridge-enabled, so it runs no Siphon reconciler. "+
					"Redeploy it with `--set bridge.enabled=true`, or set E2E_OPERATOR_REUSE=0 to let the "+
					"harness install its own", h.Config.OperatorNamespace),
		}
	}

	h.Log.Info("adopting the Operator that is already installed",
		"namespace", h.Config.OperatorNamespace, "image", container.Image,
		"note", "set E2E_OPERATOR_REUSE=0 to install the image under test instead")

	return true, nil
}

func (h *Harness) installOperator(ctx context.Context, root string) error {
	image, err := parseImageRef(h.Config.OperatorImage)
	if err != nil {
		return err
	}

	if err := h.Cluster.LoadImage(ctx, h.Config.OperatorImage); err != nil &&
		!errors.Is(err, ErrLoadUnsupported) {
		return err
	}

	if err := h.applyV2Alpha1(ctx, root); err != nil {
		return err
	}

	release := Release{
		Name:      operatorRelease,
		Namespace: h.Config.OperatorNamespace,
		Chart:     filepath.Join(root, operatorChart),
		Wait:      true,
		Timeout:   h.Config.HelmTimeout,
		CreateNS:  true,
		Set: map[string]string{
			"nameOverride":     h.Config.NameOverride,
			"image.registry":   image.registry,
			"image.repository": image.repository,
			"image.name":       image.name,
			"image.tag":        image.tag,
			// Always explicit. Left to Kubernetes, a `latest` tag defaults to
			// Always, which turns a successful image load into an ImagePullBackOff.
			"image.pullPolicy": string(h.Cluster.ImagePullPolicy()),
			// The Siphon reconciler watches at cluster scope, and the ClusterRole of
			// the manager comes from this value.
			"watchCluster":      trueValue,
			"bridge.enabled":    trueValue,
			"manager.log.level": "debug",
			// Two ClusterRoles and two ServiceAccounts the suites have no use for.
			"nginx-ingress.create":                    "false",
			"prometheus.serviceAccount.server.create": "false",
		},
	}

	h.Log.Info("installing the Operator", "image", h.Config.OperatorImage,
		"namespace", h.Config.OperatorNamespace, "pullPolicy", h.Cluster.ImagePullPolicy())

	if err := h.helm().UpgradeInstall(ctx, release); err != nil {
		return fmt.Errorf("installing the Operator: %w", err)
	}

	return nil
}

// applyV2Alpha1 installs the definitions and the RBAC that ship in no release.
//
// The role is read from config/rbac rather than duplicated here: the Taskfile
// applies the same file with sed, so the two consumers cannot drift.
func (h *Harness) applyV2Alpha1(ctx context.Context, root string) error {
	definitions, err := filepath.Glob(filepath.Join(root, crdDirectory, "*.yaml"))
	if err != nil {
		return fmt.Errorf("listing %s: %w", crdDirectory, err)
	}

	if len(definitions) == 0 {
		return fmt.Errorf("no definition in %s", filepath.Join(root, crdDirectory))
	}

	chartOwned, err := chartOwnedDefinitions(root)
	if err != nil {
		return err
	}

	var applied []string

	for _, path := range definitions {
		manifest, err := os.ReadFile(path) //nolint:gosec // a path this process just globbed
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		objects, err := DecodeYAML(manifest)
		if err != nil {
			return err
		}

		for _, object := range objects {
			if chartOwned[object.GetName()] {
				h.Log.V(1).Info("leaving the definition to Helm", "definition", object.GetName())

				continue
			}

			if err := h.Apply(ctx, object); err != nil {
				return err
			}

			applied = append(applied, object.GetName())
		}
	}

	// A definition is not usable the moment it is accepted. The apiextensions
	// controller has to establish it, and the aggregated discovery document of the
	// API server lags behind that again, so a client that asks for a Siphon a
	// second later gets a no-match error and caches it.
	if err := h.awaitDefinitionsServed(ctx, applied); err != nil {
		return err
	}

	rolePath := filepath.Join(root, v2alpha1RoleFile)

	role, err := os.ReadFile(rolePath) //nolint:gosec // a path assembled from a constant
	if err != nil {
		return fmt.Errorf("reading %s: %w", rolePath, err)
	}

	substituted := strings.NewReplacer(
		nameOverridePlaceholder, h.Config.NameOverride,
		namespacePlaceholder, h.Config.OperatorNamespace,
	).Replace(string(role))

	return h.ApplyYAML(ctx, []byte(substituted))
}

// awaitDefinitionsServed waits until every named definition can really be used.
//
// Establishing a definition and serving it are not the same moment: the
// apiextensions controller sets the condition, and the discovery document of the
// API server catches up afterwards. Asking the REST mapper is what tells the two
// apart, and it is safe to ask repeatedly because the mapper controller-runtime
// builds reloads the group on a no-match rather than caching it.
//
// Nothing here replaces the mapper, deliberately. Swapping in a snapshot taken
// from discovery would freeze it, and every definition installed later would then
// be invisible for the rest of the run.
func (h *Harness) awaitDefinitionsServed(ctx context.Context, names []string) error {
	for _, name := range names {
		var last string

		err := pollUntil(ctx, MediumTimeout, func(ctx context.Context) (bool, error) {
			definition := &apiextensionsv1.CustomResourceDefinition{}
			if err := h.Client.Get(ctx, client.ObjectKey{Name: name}, definition); err != nil {
				return false, err
			}

			if !definitionEstablished(definition) {
				last = "the API server has not established it"

				return false, nil
			}

			for _, gvk := range servedKinds(definition) {
				if _, err := h.Client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
					last = fmt.Sprintf("discovery does not carry %s yet", gvk)

					return false, nil //nolint:nilerr // a kind that is not served yet is not a failure
				}
			}

			return true, nil
		})
		if err != nil {
			return fmt.Errorf("the definition %s is not usable (%s): %w", name, last, err)
		}
	}

	return nil
}

func definitionEstablished(definition *apiextensionsv1.CustomResourceDefinition) bool {
	var established, accepted bool

	for _, condition := range definition.Status.Conditions {
		switch condition.Type {
		case apiextensionsv1.Established:
			established = condition.Status == apiextensionsv1.ConditionTrue
		case apiextensionsv1.NamesAccepted:
			accepted = condition.Status == apiextensionsv1.ConditionTrue
		case apiextensionsv1.NonStructuralSchema, apiextensionsv1.Terminating,
			apiextensionsv1.KubernetesAPIApprovalPolicyConformant, apiextensionsv1.StorageMigrating:
			// Not what usability turns on.
		}
	}

	return established && accepted
}

// servedKinds are the kinds a definition serves, which is what a client has to be
// able to map.
func servedKinds(definition *apiextensionsv1.CustomResourceDefinition) []schema.GroupVersionKind {
	var kinds []schema.GroupVersionKind

	for _, version := range definition.Spec.Versions {
		if !version.Served {
			continue
		}

		kinds = append(kinds, schema.GroupVersionKind{
			Group:   definition.Spec.Group,
			Version: version.Name,
			Kind:    definition.Spec.Names.Kind,
		})
	}

	return kinds
}

// chartOwnedDefinitions is the set of definition names deploy/chart installs
// through its crds/ directory. Reading them rather than listing them by hand
// means a definition moved into or out of the chart needs no change here.
func chartOwnedDefinitions(root string) (map[string]bool, error) {
	paths, err := filepath.Glob(filepath.Join(root, chartCRDDirectory, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", chartCRDDirectory, err)
	}

	owned := map[string]bool{}

	for _, path := range paths {
		manifest, err := os.ReadFile(path) //nolint:gosec // a path this process just globbed
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		objects, err := DecodeYAML(manifest)
		if err != nil {
			return nil, err
		}

		for _, object := range objects {
			owned[object.GetName()] = true
		}
	}

	return owned, nil
}

// awaitManager waits for the manager Deployment to finish rolling out.
//
// `helm --wait` covers this transitively, so reaching here means it already
// happened. It is asserted again so that a failure names the Deployment rather
// than reporting that helm timed out.
func (h *Harness) awaitManager(ctx context.Context) error {
	return pollUntil(ctx, LongTimeout, func(ctx context.Context) (bool, error) {
		deployment, err := h.managerDeployment(ctx)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}

			return false, err
		}

		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentAvailable {
				return condition.Status == corev1.ConditionTrue, nil
			}
		}

		return false, nil
	})
}

// managerDeployment returns the Deployment of the Operator.
func (h *Harness) managerDeployment(ctx context.Context) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	key := client.ObjectKey{
		Namespace: h.Config.OperatorNamespace,
		Name:      h.Config.NameOverride + "-controller-manager",
	}

	if err := h.Client.Get(ctx, key, deployment); err != nil {
		return nil, err
	}

	return deployment, nil
}

func managerContainerOf(deployment *appsv1.Deployment) (corev1.Container, error) {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == managerContainer {
			return container, nil
		}
	}

	return corev1.Container{}, fmt.Errorf("the Deployment %s has no container named %s",
		deployment.Name, managerContainer)
}

func hasEnv(container corev1.Container, name, value string) bool {
	for _, variable := range container.Env {
		if variable.Name == name {
			return variable.Value == value
		}
	}

	return false
}

// parseImageRef splits a reference into the four values deploy/chart wants,
// because the chart assembles registry/repository/name:tag itself.
type imageRef struct {
	registry   string
	repository string
	name       string
	tag        string
}

func parseImageRef(ref string) (imageRef, error) {
	if strings.Contains(ref, "@") {
		return imageRef{}, fmt.Errorf(
			"E2E_OPERATOR_IMAGE %q is a digest reference, and the harness sets image.tag; use a tag", ref)
	}

	// The last colon separates the tag, unless it belongs to a registry port,
	// which is the case when a slash follows it.
	repository, tag := ref, ""

	if index := strings.LastIndex(ref, ":"); index >= 0 && !strings.Contains(ref[index:], "/") {
		repository, tag = ref[:index], ref[index+1:]
	}

	if tag == "" {
		return imageRef{}, fmt.Errorf("E2E_OPERATOR_IMAGE %q carries no tag", ref)
	}

	parts := strings.Split(repository, "/")
	if len(parts) < 3 {
		return imageRef{}, fmt.Errorf(
			"E2E_OPERATOR_IMAGE %q is not registry/repository/name:tag, which is what deploy/chart assembles", ref)
	}

	return imageRef{
		registry:   parts[0],
		repository: strings.Join(parts[1:len(parts)-1], "/"),
		name:       parts[len(parts)-1],
		tag:        tag,
	}, nil
}

// RepositoryRoot walks up from the working directory to the module root, so that
// the harness finds config/ and deploy/ whatever directory go test ran in.
func RepositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("reading the working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory, so the repository root is unknown")
		}

		dir = parent
	}
}

// operatorProbeBudget is how long the behavioural half of the verification waits
// for the reconciler to write the topology of a Siphon. One reconcile is enough,
// so this only has to cover the informer catching up.
const operatorProbeBudget = time.Minute
