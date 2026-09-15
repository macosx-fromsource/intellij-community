package framework

import (
	"context"
	"fmt"
	"sync"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubectlscheme "k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// Harness is the state shared by every suite of one run: the cluster, the
// add-ons that were installed, the Operator release, and the expensive fixtures.
// Each is built at most once, whatever order the suites ask in.
type Harness struct {
	Config  *Config
	Cluster Cluster
	Log     logr.Logger

	RESTConfig *rest.Config
	Client     client.Client
	Clientset  kubernetes.Interface
	Discovery  discovery.DiscoveryInterface

	mu          sync.Mutex
	serverInfo  *version.Info
	addonState  map[AddonName]error
	operatorErr error
	operatorRun bool
}

// NewHarness builds the harness. The logger belongs to the root test, so the
// cluster lifecycle is logged even when no suite runs.
func NewHarness(ctx context.Context, cfg *Config, log logr.Logger) (*Harness, error) {
	// The scheme of kubectl is what the manager uses, so the harness sees the
	// cluster the way the Operator does. cert-manager is added on top because the
	// harness signs a throwaway certificate to prove cert-manager is usable.
	runtime.Must(apiv2alpha1.AddToScheme(kubectlscheme.Scheme))
	runtime.Must(certmanagerv1.AddToScheme(kubectlscheme.Scheme))
	// The definitions themselves, so that the harness can wait for the API server
	// to establish the ones it applies.
	runtime.Must(apiextensionsv1.AddToScheme(kubectlscheme.Scheme))

	cluster, err := NewCluster(ctx, cfg, log)
	if err != nil {
		return nil, err
	}

	harness := &Harness{
		Config:     cfg,
		Cluster:    cluster,
		Log:        log,
		RESTConfig: cluster.RESTConfig(),
		addonState: map[AddonName]error{},
	}

	harness.Client, err = client.New(harness.RESTConfig, client.Options{Scheme: kubectlscheme.Scheme})
	if err != nil {
		return nil, fmt.Errorf("building a client: %w", err)
	}

	harness.Clientset, err = kubernetes.NewForConfig(harness.RESTConfig)
	if err != nil {
		return nil, fmt.Errorf("building a clientset: %w", err)
	}

	harness.Discovery, err = discovery.NewDiscoveryClientForConfig(harness.RESTConfig)
	if err != nil {
		return nil, fmt.Errorf("building a discovery client: %w", err)
	}

	return harness, nil
}

// ServerVersion reports the version of the API server, read once.
func (h *Harness) ServerVersion() (*version.Info, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.serverInfo != nil {
		return h.serverInfo, nil
	}

	info, err := h.Discovery.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("reading the version of the API server: %w", err)
	}

	h.serverInfo = info

	return info, nil
}

// Close releases the cluster.
func (h *Harness) Close(ctx context.Context) error {
	return h.Cluster.Close(ctx)
}

// Env is the handle a suite body gets. Namespace and RunID are scoped to the
// suite; everything reached through Harness is shared.
type Env struct {
	*Harness

	// Ctx is cancelled when the suite ends.
	Ctx context.Context

	// Namespace is created for this suite and deleted afterwards, unless
	// E2E_KEEP_NAMESPACE is set.
	Namespace string

	// RunID distinguishes one run from another in the names of everything the
	// suite creates outside its namespace.
	RunID string

	// Helm runs the helm binary against this cluster.
	Helm *Helm
}

// Skipf skips the suite, or fails it when E2E_STRICT is set.
//
// A skip means the harness cannot satisfy a requirement: no cluster support, an
// add-on it may not install, a missing definition. Continuous integration sets
// the strict flag so that a job whose requirements went unsatisfied is red rather
// than a green job that tested nothing.
func (e *Env) Skipf(t *testing.T, format string, args ...any) {
	t.Helper()

	message := fmt.Sprintf(format, args...)

	if e.Config.Strict {
		t.Fatalf("E2E_STRICT is set and a requirement is unsatisfied: %s", message)
	}

	t.Skip(message)
}

// RequireKubeVersion skips the suite on a cluster older than the minimum.
//
// This is the axis that decides how the table definitions reach the Siphon pods:
// the reconciler mounts the image directly from Kubernetes 1.35 and extracts it
// into a ConfigMap below that.
func (e *Env) RequireKubeVersion(t *testing.T, minimum string) {
	t.Helper()

	info, err := e.ServerVersion()
	if err != nil {
		t.Fatal(err)
	}

	atLeast, err := atLeastVersion(info, minimum)
	if err != nil {
		t.Fatal(err)
	}

	if !atLeast {
		e.Skipf(t, "this suite needs Kubernetes %s or later, and the cluster serves %s", minimum, info.GitVersion)
	}
}

// RequireAPI skips the suite when the cluster does not serve the kind. The remedy
// is part of the message, because a missing definition is nearly always one
// command away.
func (e *Env) RequireAPI(t *testing.T, gvk schema.GroupVersionKind, remedy string) {
	t.Helper()

	_, err := e.Client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		return
	}

	if meta.IsNoMatchError(err) {
		e.Skipf(t, "the cluster does not serve %s; %s", gvk, remedy)

		return
	}

	t.Fatalf("looking up %s: %v", gvk, err)
}
