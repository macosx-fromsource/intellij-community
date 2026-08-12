//go:build e2e

// This end-to-end test installs the GitLab chart into a real cluster the way
// a v2 controller should: render once with internal/render, apply the chart
// definitions, run the pre-install hooks through the executor, and only then
// apply the workloads. It is the worked example behind
// doc/developer/render.md.
//
// Run it with a kubeconfig pointing at a throwaway cluster:
//
//	# Set E2E_KEEP_NAMESPACE=1 to inspect the namespace afterwards.
//	HELM_CHARTS=$PWD/charts CHART_VERSION=$(head -n1 CHART_VERSIONS) \
//	go test -tags e2e -count=1 -timeout 30m -v -run TestGitLabChartHooks ./internal/render/hookexec/
//
// The hook phase runs by default and is fast. Applying the workloads needs
// real PostgreSQL, Redis, and object storage, so it is opt-in through
// E2E_APPLY_WORKLOADS.

package hookexec_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/kube"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/capabilities"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/hookexec"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// hookTimeout bounds a single hook. The shared-secrets Job pulls the toolbox
// image on a cold cluster, which dominates the wall time.
const hookTimeout = 10 * time.Minute

// e2eValues configure a chart that renders completely without pointing at
// real infrastructure. Keep the object storage entries: the chart asserts
// them from NOTES.txt and fails the render without them.
const e2eValues = `
certmanager-issuer:
  email: admin@example.com
nginx-ingress:
  enabled: false
prometheus:
  install: false
gitlab-runner:
  install: false
registry:
  storage:
    secret: e2e-registry-storage
    key: config
gitlab:
  toolbox:
    backups:
      objectStorage:
        config:
          secret: e2e-backup-storage
          key: config
global:
  gatewayApi:
    enabled: false
    installEnvoy: false
    configureCertmanager: false

  ingress:
    enabled: true
    provider: traefik
    class: traefik
    configureCertmanager: false
    tls:
      enabled: false
  redis:
    host: redis.example.com
  psql:
    host: psql.example.com
    password:
      secret: e2e-psql-password
      key: password
  pages:
    objectStore:
      connection:
        secret: e2e-object-storage
        key: connection
  appConfig:
    object_store:
      enabled: true
      connection:
        secret: e2e-object-storage
        key: connection
`

func TestGitLabChartHooks(t *testing.T) {
	ctx := context.Background()
	chartPath := locateChart(t)

	// A random suffix keeps concurrent runs, and reruns within the same
	// second, from colliding on the namespace or on the cluster-scoped
	// object names that the chart derives from the release name.
	runID := randomSuffix(t)
	namespace := "render-e2e-" + runID
	release := "e2e-" + runID

	result := renderChart(t, chartPath, release, namespace)
	kubeClient := newClient(t)

	// The release identity the renderer stamps is what lets cleanup find the
	// cluster-scoped objects, which outlive the namespace.
	releaseLabels := map[string]string{
		render.ReleaseNameLabel:      release,
		render.ReleaseNamespaceLabel: namespace,
	}

	leftovers := &clusterScopedTracker{labels: releaseLabels}
	apply := applier{client: kubeClient, namespace: namespace, leftovers: leftovers}

	createNamespace(ctx, t, kubeClient, namespace)

	t.Cleanup(func() {
		leftovers.cleanup(t, kubeClient)
		deleteNamespace(t, kubeClient, namespace)
	})

	t.Run("the chart definitions apply before anything else", func(t *testing.T) {
		// Helm installs crds/ before rendering, and a controller has to do
		// the same, or the custom resources in Objects have no kind to bind
		// to. These values disable the Gateway API, the source of the
		// bundled definitions, so the render may carry none.
		t.Logf("the render carries %d chart definitions", len(result.CRDs))

		if len(result.CRDs) == 0 {
			t.Skip("the enabled subcharts ship no crds/ definitions")
		}

		if os.Getenv("E2E_APPLY_CRDS") == "" {
			t.Skip("set E2E_APPLY_CRDS=1 to install the chart definitions; they are cluster-wide and may collide with existing ones")
		}

		for _, crd := range result.CRDs {
			apply.apply(ctx, t, crd)
		}
	})

	t.Run("the pre-install hooks run and generate the secrets", func(t *testing.T) {
		hooks := result.HooksFor("pre-install")
		require.NotEmpty(t, hooks, "the chart generates its secrets from pre-install hooks")

		// The order matters concretely: the shared-secrets RBAC carries
		// weight -5 and has to exist before the weight-0 Job that runs under
		// its ServiceAccount.
		var sawJob bool

		for _, hook := range hooks {
			if hook.Object.GetKind() == "Job" {
				sawJob = true
				continue
			}

			assert.False(t, sawJob, "%s sorts after a Job", hook.Object.GetKind())
		}

		assert.True(t, sawJob, "the pre-install hooks include a Job")

		// Hooks that are cluster-scoped survive the namespace deletion.
		for _, hook := range hooks {
			leftovers.track(hook.Object)
		}

		runner := hookexec.New(
			helmClient(t, namespace),
			hookexec.WithTimeout(hookTimeout),
			hookexec.WithLogger(testLogger(t)),
		)

		require.NoError(t, runner.Run(ctx, result, "pre-install"))

		// The proof that the hooks did their job: the chart itself renders no
		// Secret, yet the release now has one.
		assert.Nil(t, objects.First(result.Objects, objects.ByKind("Secret")),
			"the chart renders no Secret; the hooks create them")

		secrets := &corev1.SecretList{}
		require.NoError(t, kubeClient.List(ctx, secrets, client.InNamespace(namespace)))

		names := []string{}
		for _, secret := range secrets.Items {
			names = append(names, secret.Name)
		}

		assert.Contains(t, names, release+"-rails-secret")
		assert.Contains(t, names, release+"-gitlab-shell-host-keys")
	})

	t.Run("the succeeded hooks are cleaned up by their policy", func(t *testing.T) {
		// Every shared-secrets hook carries hook-succeeded, so a successful
		// event leaves nothing behind. The secrets it created remain.
		for _, hook := range result.HooksFor("pre-install") {
			if !slices.Contains(hook.DeletePolicies, hookexec.PolicyHookSucceeded) {
				continue
			}

			live := &unstructured.Unstructured{}
			live.SetGroupVersionKind(hook.Object.GroupVersionKind())

			err := kubeClient.Get(ctx,
				types.NamespacedName{Namespace: namespace, Name: hook.Object.GetName()}, live)

			assert.True(t, apierrors.IsNotFound(err),
				"%s %s should be deleted by hook-succeeded", hook.Object.GetKind(), hook.Object.GetName())
		}
	})

	t.Run("the hooks are repeatable", func(t *testing.T) {
		// A controller reconciles repeatedly, so running the same event twice
		// has to succeed. This works only because the executor honors
		// before-hook-creation: a completed Job is immutable.
		runner := hookexec.New(
			helmClient(t, namespace),
			hookexec.WithTimeout(hookTimeout),
			hookexec.WithLogger(testLogger(t)),
		)

		require.NoError(t, runner.Run(ctx, result, "pre-install"))
	})

	t.Run("the workloads apply after the hooks", func(t *testing.T) {
		if os.Getenv("E2E_APPLY_WORKLOADS") == "" {
			t.Skip("set E2E_APPLY_WORKLOADS=1 to apply the workloads; they need real dependencies to become ready")
		}

		for _, obj := range result.Objects {
			apply.apply(ctx, t, obj)
		}

		webservice := objects.First(result.Objects, objects.And(
			objects.ByKind("Deployment"), objects.ByComponent("webservice")))
		require.NotNil(t, webservice)

		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(webservice.GroupVersionKind())

		require.NoError(t, kubeClient.Get(ctx,
			types.NamespacedName{Namespace: namespace, Name: webservice.GetName()}, live))

		// The release labels make the applied objects selectable, which is
		// what cleanup and inspection rely on.
		for key, value := range releaseLabels {
			assert.Equal(t, value, live.GetLabels()[key], "label %s", key)
		}

		// The chart labels survive alongside them, and the selector, which
		// is immutable, is untouched.
		assert.Equal(t, "webservice", live.GetLabels()["app"])

		selector, _, err := unstructured.NestedStringMap(live.Object, "spec", "selector", "matchLabels")
		require.NoError(t, err)
		assert.NotContains(t, selector, render.ReleaseNameLabel)
	})
}

// locateChart resolves the GitLab chart, skipping when it is not staged.
func locateChart(t *testing.T) string {
	t.Helper()

	chartsDir := os.Getenv("HELM_CHARTS")
	chartVersion := os.Getenv("CHART_VERSION")

	if chartsDir == "" || chartVersion == "" {
		t.Skip("HELM_CHARTS and CHART_VERSION are not set; run the test via `task e2e-tests`")
	}

	chartPath, err := render.LocateChart(chartsDir, "gitlab", chartVersion)
	require.NoError(t, err, "run `task retrieve-charts`")

	return chartPath
}

// renderChart renders the whole chart once, the way a controller does.
//
// The capabilities come from the cluster, and the renderer takes them as
// authoritative. Left unset, the chart sees the Helm defaults, which carry no
// group-version-kind entries: it then misses policy/v1/PodDisruptionBudget
// and renders PodDisruptionBudget as policy/v1beta1.
func renderChart(t *testing.T, chartPath, release, namespace string) *render.Result {
	t.Helper()

	values := support.Values{}
	require.NoError(t, values.AddFromYAML(e2eValues))

	discovered := discoverCapabilities(t)

	result, err := render.Render(render.Request{
		ChartPath:   chartPath,
		ReleaseName: release,
		Namespace:   namespace,
		Values:      values,
		KubeVersion: discovered.KubeVersion,
		APIVersions: discovered.APIVersions,
	})
	require.NoError(t, err)

	for _, warning := range result.Warnings {
		t.Logf("render warning: %s", warning)
	}

	return result
}

// discoverCapabilities reads the capabilities of the target cluster, the way
// a controller does before rendering.
func discoverCapabilities(t *testing.T) *capabilities.Capabilities {
	t.Helper()

	restConfig, err := config.GetConfig()
	require.NoError(t, err)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	require.NoError(t, err)

	discovered, err := capabilities.Discover(discoveryClient)
	require.NoError(t, err)

	t.Logf("cluster %s serves %d api versions",
		discovered.KubeVersion.Version, len(discovered.APIVersions))

	return discovered
}

// newClient builds a controller-runtime client for the assertions.
func newClient(t *testing.T) client.Client {
	t.Helper()

	restConfig, err := config.GetConfig()
	require.NoError(t, err, "no kubeconfig; point KUBECONFIG at a throwaway cluster")

	kubeClient, err := client.New(restConfig, client.Options{})
	require.NoError(t, err)

	return kubeClient
}

// helmClient builds the Helm kube client the runner works through, bound to
// the namespace that hooks without one inherit.
func helmClient(t *testing.T, namespace string) *kube.Client {
	t.Helper()

	client, err := hookexec.NewClient(genericclioptions.NewConfigFlags(true), namespace)
	require.NoError(t, err)

	return client
}

func createNamespace(ctx context.Context, t *testing.T, kubeClient client.Client, name string) {
	t.Helper()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	require.NoError(t, kubeClient.Create(ctx, namespace))

	t.Logf("created namespace %s", name)
}

func deleteNamespace(t *testing.T, kubeClient client.Client, name string) {
	t.Helper()

	if os.Getenv("E2E_KEEP_NAMESPACE") != "" {
		t.Logf("keeping namespace %s", name)
		return
	}

	// A fresh context: the test context may already be done.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := kubeClient.Delete(ctx, namespace); err != nil && !apierrors.IsNotFound(err) {
		t.Logf("deleting namespace %s: %v", name, err)
	}
}

// applier applies rendered objects into one namespace under the field manager
// of the test, recording the cluster-scoped ones for cleanup.
type applier struct {
	client    client.Client
	namespace string
	leftovers *clusterScopedTracker
}

// apply creates or updates one rendered object.
func (a applier) apply(ctx context.Context, t *testing.T, obj *unstructured.Unstructured) {
	t.Helper()

	target := obj.DeepCopy()

	// The renderer never stamps the namespace, so the caller supplies it for
	// namespaced kinds that the chart left unqualified.
	if target.GetNamespace() == "" {
		scoped, err := a.client.IsObjectNamespaced(target)
		require.NoError(t, err, "resolving the scope of %s %s", target.GetKind(), target.GetName())

		if scoped {
			target.SetNamespace(a.namespace)
		}
	}

	a.leftovers.track(target)

	err := a.client.Apply(ctx, client.ApplyConfigurationFromUnstructured(target),
		client.FieldOwner("render-e2e"), client.ForceOwnership)
	require.NoError(t, err, "applying %s %s", target.GetKind(), target.GetName())
}

// clusterScopedTracker remembers the kinds that outlive the namespace, so
// that cleanup can delete them by label. Recording the kind rather than the
// object means the sweep also catches anything a hook created under the same
// labels.
type clusterScopedTracker struct {
	labels map[string]string
	kinds  []schema.GroupVersionKind
}

// track records the kind of an object when it carries no namespace, which
// for a rendered chart object means it is cluster-scoped.
func (c *clusterScopedTracker) track(obj *unstructured.Unstructured) {
	if obj.GetNamespace() != "" {
		return
	}

	gvk := obj.GroupVersionKind()
	if slices.Contains(c.kinds, gvk) {
		return
	}

	c.kinds = append(c.kinds, gvk)
}

// cleanup deletes every cluster-scoped object of the recorded kinds that
// carries the release labels.
func (c *clusterScopedTracker) cleanup(t *testing.T, kubeClient client.Client) {
	t.Helper()

	if len(c.kinds) == 0 {
		return
	}

	if os.Getenv("E2E_KEEP_NAMESPACE") != "" {
		t.Logf("keeping cluster-scoped objects of %d kinds; delete them with -l %s=%s",
			len(c.kinds), render.ReleaseNameLabel, c.labels[render.ReleaseNameLabel])

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for _, gvk := range c.kinds {
		target := &unstructured.Unstructured{}
		target.SetGroupVersionKind(gvk)

		err := kubeClient.DeleteAllOf(ctx, target, client.MatchingLabels(c.labels))
		if err != nil && !apierrors.IsNotFound(err) {
			t.Logf("deleting %s by label: %v", gvk.Kind, err)
		}
	}
}

// randomSuffix returns a short lowercase token unique to this run.
func randomSuffix(t *testing.T) string {
	t.Helper()

	buf := make([]byte, 4)
	_, err := rand.Read(buf)
	require.NoError(t, err)

	return hex.EncodeToString(buf)
}

// testLogger routes the runner output into the test log.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()

	return slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// testWriter adapts the test log to io.Writer for the slog handler.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))

	return len(p), nil
}
