package framework

import (
	"context"
	"fmt"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AddonName identifies a cluster-scoped prerequisite the harness can install.
type AddonName string

// The add-ons. They transcribe scripts/prepare.sh, which is the local twin of
// this table: a version bump belongs in both.
//
// Two deliberate divergences from that script. Object storage is Garage rather
// than SeaweedFS, because scripts/dev_dependencies.sh and scripts/test.sh already
// provision Garage and that is the path continuous integration exercises for the
// GitLab chart; SeaweedFS stays here because it is what the local bootstrap
// installs, but no suite asks for it. And Traefik is never installed, because the
// k3s module disables the built-in Traefik of k3s and no suite needs ingress.
const (
	CertManager        AddonName = "cert-manager"
	GatewayAPI         AddonName = "gateway-api"
	CloudNativePG      AddonName = "cnpg"
	ClickHouseOperator AddonName = "clickhouse-operator"
	NATS               AddonName = "nats"
	SeaweedFSOperator  AddonName = "seaweedfs-operator"
	Traefik            AddonName = "traefik"
)

// Names the cert-manager readiness probe relies on.
const (
	certManagerNamespace = "cert-manager"
	certManagerWebhook   = "cert-manager-webhook"
	probeName            = "probe"
	trueValue            = "true"
)

// Pinned versions of the third-party charts. They are pinned rather than floated
// because a suite that fails has to fail for a reason in this repository.
const (
	gatewayAPIVersion = "v1.6.0"
	// The chart versions, which for several of these are nothing like the version
	// of the software they install: the ClickHouse operator chart is 0.0.7 while the
	// operator is v0.0.7, and the Traefik chart is 41.x while Traefik is 3.x.
	cnpgVersion               = "0.29.0"
	clickHouseOperatorVersion = "0.0.7"
	natsVersion               = "2.14.6"
	seaweedFSOperatorVersion  = "0.1.40"
	traefikVersion            = "41.4.0"
)

// Addon is one prerequisite: how to install it, how to tell it is already there,
// and how to tell it is usable.
type Addon struct {
	// DependsOn is installed first.
	DependsOn []AddonName

	// Chart or Manifest, never both.
	Chart    *Release
	Manifest string

	// Present is a cheap probe. An add-on that is already there is skipped
	// entirely, so a rerun against a warm cluster costs one API call rather than
	// a minute of helm.
	Present func(ctx context.Context, h *Harness) (bool, error)

	// Ready blocks until the add-on is usable. `helm --wait` covers the
	// workloads; this is for what it cannot see, such as a webhook that answers.
	Ready func(ctx context.Context, h *Harness) error
}

// addonCatalog is the table of everything the harness knows how to install.
func (h *Harness) addonCatalog() map[AddonName]Addon {
	return map[AddonName]Addon{
		CertManager: {
			Chart: &Release{
				Name:      string(CertManager),
				Namespace: certManagerNamespace,
				Chart:     "oci://quay.io/jetstack/charts/cert-manager",
				Version:   h.Config.CertManagerVersion,
				Set:       map[string]string{"crds.enabled": trueValue},
				Wait:      true,
				Timeout:   5 * time.Minute,
				CreateNS:  true,
			},
			Present: servesKind(schema.GroupVersionKind{
				Group: "cert-manager.io", Version: "v1", Kind: "Certificate",
			}),
			Ready: certManagerCanSign,
		},

		GatewayAPI: {
			Manifest: "https://github.com/kubernetes-sigs/gateway-api/releases/download/" +
				gatewayAPIVersion + "/standard-install.yaml",
			Present: servesKind(schema.GroupVersionKind{
				Group: "gateway.networking.k8s.io", Version: "v1", Kind: "GatewayClass",
			}),
		},

		CloudNativePG: {
			Chart: &Release{
				Name:      "cnpg",
				Namespace: "cnpg-system",
				Chart:     "cloudnative-pg",
				Repo:      "https://cloudnative-pg.github.io/charts",
				Version:   cnpgVersion,
				Wait:      true,
				Timeout:   5 * time.Minute,
				CreateNS:  true,
			},
			Present: servesKind(schema.GroupVersionKind{
				Group: "postgresql.cnpg.io", Version: "v1", Kind: "Cluster",
			}),
		},

		ClickHouseOperator: {
			Chart: &Release{
				Name:      "clickhouse-operator",
				Namespace: "clickhouse-operator-system",
				Chart:     "oci://ghcr.io/clickhouse/clickhouse-operator-helm",
				Version:   clickHouseOperatorVersion,
				Wait:      true,
				Timeout:   5 * time.Minute,
				CreateNS:  true,
			},
			Present: servesKind(schema.GroupVersionKind{
				Group: "clickhouse.altinity.com", Version: "v1", Kind: "ClickHouseInstallation",
			}),
		},

		NATS: {
			Chart: &Release{
				Name:      string(NATS),
				Namespace: "nats",
				Chart:     string(NATS),
				Repo:      "https://nats-io.github.io/k8s/helm/charts/",
				Version:   natsVersion,
				Set: map[string]string{
					"config.jetstream.enabled":            trueValue,
					"config.jetstream.fileStore.pvc.size": "1Gi",
				},
				Wait:     true,
				Timeout:  5 * time.Minute,
				CreateNS: true,
			},
			Present: workloadExists("nats", "nats"),
		},

		SeaweedFSOperator: {
			Chart: &Release{
				Name:      "seaweedfs-operator",
				Namespace: "seaweedfs-system",
				Chart:     "seaweedfs-operator",
				Repo:      "https://seaweedfs.github.io/seaweedfs-operator/",
				Version:   seaweedFSOperatorVersion,
				Wait:      true,
				Timeout:   5 * time.Minute,
				CreateNS:  true,
			},
			Present: servesKind(schema.GroupVersionKind{
				Group: "seaweed.seaweedfs.com", Version: "v1", Kind: "Seaweed",
			}),
		},

		Traefik: {
			DependsOn: []AddonName{GatewayAPI},
			Chart: &Release{
				Name:      string(Traefik),
				Namespace: "traefik",
				Chart:     string(Traefik),
				Repo:      "https://traefik.github.io/charts",
				Version:   traefikVersion,
				Set: map[string]string{
					"providers.kubernetesGateway.enabled":        trueValue,
					"gateway.listeners.web.namespacePolicy.from": "All",
				},
				Wait:     true,
				Timeout:  5 * time.Minute,
				CreateNS: true,
			},
			Present: workloadExists("traefik", "traefik"),
		},
	}
}

// RequireAddons satisfies the named add-ons and everything they depend on.
//
// Installing one writes outside every test namespace, so E2E_ADDONS=skip turns
// this into a probe: on a shared cluster the harness reports what is missing
// rather than writing cluster-scoped objects into it.
func (e *Env) RequireAddons(t *testing.T, names ...AddonName) {
	t.Helper()

	for _, name := range names {
		if err := e.ensureAddon(e.Ctx, name); err != nil {
			var missing *addonMissingError
			if errorsAs(err, &missing) {
				e.Skipf(t, "%s", missing.Error())

				return
			}

			t.Fatalf("ensuring the add-on %s: %v", name, err)
		}
	}
}

// addonMissingError reports an add-on that is absent and that the harness was
// told not to install. It is a skip, not a failure: the cluster is not the
// harness's to change.
type addonMissingError struct {
	name AddonName
}

func (e *addonMissingError) Error() string {
	return fmt.Sprintf("the add-on %s is not installed and E2E_ADDONS is %q", e.name, AddonModeSkip)
}

// ensureAddon installs the add-on once per run, whatever order the suites ask in.
func (h *Harness) ensureAddon(ctx context.Context, name AddonName) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.ensureAddonLocked(ctx, name)
}

func (h *Harness) ensureAddonLocked(ctx context.Context, name AddonName) error {
	if err, done := h.addonState[name]; done {
		return err
	}

	catalog := h.addonCatalog()

	addon, known := catalog[name]
	if !known {
		return fmt.Errorf("no add-on named %q", name)
	}

	// Recorded before the work starts, so that a dependency cycle in the table
	// fails fast instead of recursing forever.
	h.addonState[name] = nil

	err := h.installAddon(ctx, name, addon)
	h.addonState[name] = err

	return err
}

func (h *Harness) installAddon(ctx context.Context, name AddonName, addon Addon) error {
	for _, dependency := range addon.DependsOn {
		if err := h.ensureAddonLocked(ctx, dependency); err != nil {
			return fmt.Errorf("the add-on %s needs %s: %w", name, dependency, err)
		}
	}

	present := false

	if addon.Present != nil {
		var err error

		present, err = addon.Present(ctx, h)
		if err != nil {
			return fmt.Errorf("probing for %s: %w", name, err)
		}
	}

	switch {
	case present:
		h.Log.Info("the add-on is already installed", "addon", name)
	case h.Config.Addons == AddonModeSkip:
		return &addonMissingError{name: name}
	default:
		if err := h.applyAddon(ctx, name, addon); err != nil {
			return err
		}
	}

	if addon.Ready != nil {
		if err := addon.Ready(ctx, h); err != nil {
			return fmt.Errorf("waiting for %s to become usable: %w", name, err)
		}
	}

	return nil
}

func (h *Harness) applyAddon(ctx context.Context, name AddonName, addon Addon) error {
	h.Log.Info("installing the add-on", "addon", name)

	switch {
	case addon.Chart != nil:
		if err := h.helm().UpgradeInstall(ctx, *addon.Chart); err != nil {
			return fmt.Errorf("installing %s: %w", name, err)
		}

	case addon.Manifest != "":
		if err := h.applyManifestURL(ctx, addon.Manifest); err != nil {
			return fmt.Errorf("applying the manifest of %s: %w", name, err)
		}

	default:
		return fmt.Errorf("the add-on %s has neither a chart nor a manifest", name)
	}

	return nil
}

// servesKind builds a probe that answers from discovery.
func servesKind(gvk schema.GroupVersionKind) func(ctx context.Context, h *Harness) (bool, error) {
	return func(_ context.Context, h *Harness) (bool, error) {
		_, err := h.Client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)

		return err == nil, nil
	}
}

// workloadExists builds a probe for an add-on that installs no definition, so
// that the only sign of it is its own workload.
func workloadExists(namespace, name string) func(ctx context.Context, h *Harness) (bool, error) {
	return func(ctx context.Context, h *Harness) (bool, error) {
		key := client.ObjectKey{Namespace: namespace, Name: name}

		err := h.Client.Get(ctx, key, &appsv1.Deployment{})
		if err == nil {
			return true, nil
		}

		if apierrors.IsNotFound(err) {
			err = h.Client.Get(ctx, key, &appsv1.StatefulSet{})
			if err == nil {
				return true, nil
			}
		}

		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}
}

// certManagerCanSign waits until cert-manager will really issue a certificate.
//
// `helm --wait` returns once the webhook pods are ready, which is not the same as
// the API server accepting a call to them, and the very next thing the harness
// does is install a chart that creates a Certificate. Signing a throwaway one
// here costs a few seconds warm and removes the whole class of "no endpoints
// available for service cert-manager-webhook" failures.
func certManagerCanSign(ctx context.Context, h *Harness) error {
	endpointsReady := func(ctx context.Context) (bool, error) {
		slices := &discoveryv1.EndpointSliceList{}

		err := h.Client.List(ctx, slices, client.InNamespace(certManagerNamespace),
			client.MatchingLabels{discoveryv1.LabelServiceName: certManagerWebhook})
		if err != nil {
			return false, err
		}

		for _, slice := range slices.Items {
			for _, endpoint := range slice.Endpoints {
				if len(endpoint.Addresses) > 0 {
					return true, nil
				}
			}
		}

		return false, nil
	}

	if err := pollUntil(ctx, MediumTimeout, endpointsReady); err != nil {
		return fmt.Errorf("the cert-manager webhook has no endpoints: %w", err)
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "e2e-certmanager-probe"}}
	if err := h.Client.Create(ctx, namespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating the probe namespace: %w", err)
	}

	defer func() {
		if err := h.Client.Delete(context.WithoutCancel(ctx), namespace); err != nil {
			h.Log.V(1).Info("could not delete the cert-manager probe namespace", "error", err)
		}
	}()

	issuer := &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{Name: probeName, Namespace: namespace.Name},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{SelfSigned: &certmanagerv1.SelfSignedIssuer{}},
		},
	}

	if err := h.Client.Create(ctx, issuer); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating the probe issuer: %w", err)
	}

	certificate := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: probeName, Namespace: namespace.Name},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: probeName,
			CommonName: "probe.e2e.invalid",
			IssuerRef:  cmmetav1.IssuerReference{Name: issuer.Name, Kind: "Issuer"},
		},
	}

	if err := h.Client.Create(ctx, certificate); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating the probe certificate: %w", err)
	}

	return pollUntil(ctx, MediumTimeout, func(ctx context.Context) (bool, error) {
		live := &certmanagerv1.Certificate{}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(certificate), live); err != nil {
			return false, err
		}

		for _, condition := range live.Status.Conditions {
			if condition.Type == certmanagerv1.CertificateConditionReady {
				return condition.Status == cmmetav1.ConditionTrue, nil
			}
		}

		return false, nil
	})
}
