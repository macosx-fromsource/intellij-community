package framework

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/platforms"
	"github.com/go-logr/logr"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// ErrLoadUnsupported reports a provider that cannot make a local image available
// to the cluster. The caller treats it as "the cluster had better be able to pull
// the image itself".
var ErrLoadUnsupported = errors.New("this provider cannot load a local image")

// Cluster is the Kubernetes cluster under test.
//
// The default provider is a cluster that already exists, so nothing here may
// assume the harness owns it: no implementation creates or deletes anything
// cluster-scoped of its own accord.
type Cluster interface {
	// Provider names the implementation, for the log and for error messages.
	Provider() ProviderName

	// RESTConfig connects to the API server.
	RESTConfig() *rest.Config

	// KubeconfigPath is a single-file kubeconfig on disk, for the helm binary.
	KubeconfigPath() string

	// KubeContext is the context helm should select, empty when the kubeconfig
	// holds exactly one.
	KubeContext() string

	// LoadImage makes a locally built image available to the kubelets of the
	// cluster, or returns ErrLoadUnsupported.
	LoadImage(ctx context.Context, ref string) error

	// ImagePullPolicy is what a pod referring to a loaded image must carry. The
	// harness always passes it explicitly: left to Kubernetes, a `latest` tag
	// defaults to Always, which turns a successful load into an ImagePullBackOff.
	ImagePullPolicy() corev1.PullPolicy

	// Close releases the cluster. A no-op for one that already existed.
	Close(ctx context.Context) error
}

// NewCluster returns the provider the configuration selects.
func NewCluster(ctx context.Context, cfg *Config, log logr.Logger) (Cluster, error) {
	switch cfg.Provider {
	case ProviderK3s:
		return newK3sCluster(ctx, cfg, log)
	case ProviderExisting:
		return newExistingCluster(cfg, log)
	default:
		return nil, fmt.Errorf("no cluster provider named %q", cfg.Provider)
	}
}

// existingCluster is the cluster of the current kubeconfig.
type existingCluster struct {
	cfg        *Config
	log        logr.Logger
	restConfig *rest.Config
	kubeconfig string
	loaded     bool
}

func newExistingCluster(cfg *Config, log logr.Logger) (Cluster, error) {
	restConfig, err := ctrlconfig.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("no kubeconfig; point KUBECONFIG at a throwaway cluster: %w", err)
	}

	kubeconfig, err := flattenedKubeconfig(cfg)
	if err != nil {
		return nil, err
	}

	return &existingCluster{cfg: cfg, log: log, restConfig: restConfig, kubeconfig: kubeconfig}, nil
}

func (c *existingCluster) Provider() ProviderName      { return ProviderExisting }
func (c *existingCluster) RESTConfig() *rest.Config    { return c.restConfig }
func (c *existingCluster) KubeconfigPath() string      { return c.kubeconfig }
func (c *existingCluster) KubeContext() string         { return c.cfg.KubeContext }
func (c *existingCluster) Close(context.Context) error { return nil }

// LoadImage delegates to whatever the environment says can reach this cluster.
// There is no way to guess: the same kubeconfig may point at k3d, at kind, at a
// remote cluster with a registry, or at a cluster that can pull the image itself.
func (c *existingCluster) LoadImage(ctx context.Context, ref string) error {
	switch {
	case c.cfg.ImageLoadCmd != "":
		// Running whatever the operator of the cluster configured is the point of
		// this hook, so the command is deliberately not sanitized.
		cmd := exec.CommandContext(ctx, "sh", "-c", c.cfg.ImageLoadCmd) //nolint:gosec // see above

		cmd.Env = append(os.Environ(), "IMAGE="+ref)

		output, err := cmd.CombinedOutput()
		c.log.Info("ran E2E_IMAGE_LOAD_CMD", "image", ref, "output", string(output))

		if err != nil {
			return fmt.Errorf("E2E_IMAGE_LOAD_CMD failed for %s: %w", ref, err)
		}

		c.loaded = true

		return nil

	case c.cfg.K3dCluster != "":
		//nolint:gosec // the cluster name comes from the environment of the run, by design
		cmd := exec.CommandContext(ctx, "k3d", "image", "import", "-c", c.cfg.K3dCluster, ref)

		output, err := cmd.CombinedOutput()
		c.log.Info("imported the image into k3d", "cluster", c.cfg.K3dCluster, "image", ref, "output", string(output))

		if err != nil {
			return fmt.Errorf("k3d image import of %s into %s failed: %w", ref, c.cfg.K3dCluster, err)
		}

		c.loaded = true

		return nil

	default:
		c.log.Info("not loading the image; the cluster has to be able to pull it. "+
			"Set E2E_K3D_CLUSTER or E2E_IMAGE_LOAD_CMD to load it", "image", ref)

		return ErrLoadUnsupported
	}
}

// ImagePullPolicy is Never once an image was really loaded: a bad import then
// fails at once with ErrImageNeverPull instead of five minutes of
// ImagePullBackOff.
func (c *existingCluster) ImagePullPolicy() corev1.PullPolicy {
	if c.loaded {
		return corev1.PullNever
	}

	return corev1.PullIfNotPresent
}

// k3sCluster is a single-use k3s container.
type k3sCluster struct {
	cfg        *Config
	log        logr.Logger
	container  *k3s.K3sContainer
	restConfig *rest.Config
	kubeconfig string
}

func newK3sCluster(ctx context.Context, cfg *Config, log logr.Logger) (Cluster, error) {
	log.Info("starting a k3s container", "image", cfg.K3sImage)

	// The module already runs the container privileged, exposes 6443, passes
	// `server --disable=traefik --tls-san=<daemon host>` and sets
	// K3S_KUBECONFIG_MODE, so none of that is repeated here.
	container, err := k3s.Run(ctx, cfg.K3sImage)
	if err != nil {
		return nil, fmt.Errorf("starting k3s %s: %w", cfg.K3sImage, err)
	}

	cluster := &k3sCluster{cfg: cfg, log: log, container: container}

	raw, err := container.GetKubeConfig(ctx)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("reading the kubeconfig of the k3s container: %w", err),
			testcontainers.TerminateContainer(container))
	}

	cluster.restConfig, err = clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("parsing the kubeconfig of the k3s container: %w", err),
			testcontainers.TerminateContainer(container))
	}

	// Written to disk for the helm binary, and so that a kept cluster can be
	// reached by hand.
	if err := os.MkdirAll(cfg.ArtifactsDir, 0o750); err != nil {
		return nil, errors.Join(fmt.Errorf("creating %s: %w", cfg.ArtifactsDir, err),
			testcontainers.TerminateContainer(container))
	}

	cluster.kubeconfig = filepath.Join(cfg.ArtifactsDir, "k3s.kubeconfig")
	if err := os.WriteFile(cluster.kubeconfig, raw, 0o600); err != nil {
		return nil, errors.Join(fmt.Errorf("writing %s: %w", cluster.kubeconfig, err),
			testcontainers.TerminateContainer(container))
	}

	log.Info("the k3s container is up", "kubeconfig", cluster.kubeconfig, "server", cluster.restConfig.Host)

	return cluster, nil
}

func (c *k3sCluster) Provider() ProviderName   { return ProviderK3s }
func (c *k3sCluster) RESTConfig() *rest.Config { return c.restConfig }
func (c *k3sCluster) KubeconfigPath() string   { return c.kubeconfig }
func (c *k3sCluster) KubeContext() string      { return "" }

// ImagePullPolicy is always Never: the node has no registry credentials and, for
// a private image, no route to the registry either.
func (c *k3sCluster) ImagePullPolicy() corev1.PullPolicy { return corev1.PullNever }

// LoadImage exports the image from the daemon this process talks to and imports
// it into the containerd store of the node.
//
// It pins the platform rather than calling LoadImages, because a `docker save`
// of a multi-architecture tag exports an index and the containerd import then
// resolves to whichever platform it likes. Pinning makes the mismatch impossible
// instead of intermittent.
func (c *k3sCluster) LoadImage(ctx context.Context, ref string) error {
	platform := platforms.DefaultSpec()
	platform.OS = "linux"

	if c.cfg.K3sPlatform != "" {
		parsed, err := platforms.Parse(c.cfg.K3sPlatform)
		if err != nil {
			return fmt.Errorf("E2E_K3S_PLATFORM is %q: %w", c.cfg.K3sPlatform, err)
		}

		platform = parsed
	}

	c.log.Info("importing the image into the k3s node", "image", ref, "platform", platforms.Format(platform))

	spec := ocispec.Platform{OS: platform.OS, Architecture: platform.Architecture, Variant: platform.Variant}
	if err := c.container.LoadImagesWithPlatform(ctx, []string{ref}, &spec); err != nil {
		return fmt.Errorf("importing %s into the k3s node: %w. "+
			"Build it with `task docker-build-bridge` using the same container daemon this harness talks to "+
			"(CONTAINER_CLI defaults to podman, while testcontainers reads DOCKER_HOST)", ref, err)
	}

	return nil
}

// Close terminates the container unless it was asked to be kept.
func (c *k3sCluster) Close(ctx context.Context) error {
	if c.cfg.KeepCluster {
		c.log.Info("keeping the k3s container", "kubeconfig", c.kubeconfig)

		return nil
	}

	return testcontainers.TerminateContainer(c.container)
}

// flattenedKubeconfig returns a path to a single-file kubeconfig.
//
// The helm binary takes one --kubeconfig path, while KUBECONFIG may hold several
// separated by the list separator. When it does, the merged view is written to a
// file of its own so that helm sees what the client library sees.
func flattenedKubeconfig(cfg *Config) (string, error) {
	paths := filepath.SplitList(os.Getenv("KUBECONFIG"))

	switch len(paths) {
	case 0:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("no KUBECONFIG and no home directory: %w", err)
		}

		return filepath.Join(home, ".kube", "config"), nil

	case 1:
		return paths[0], nil
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()

	merged, err := rules.Load()
	if err != nil {
		return "", fmt.Errorf("merging the kubeconfig files in KUBECONFIG: %w", err)
	}

	if err := os.MkdirAll(cfg.ArtifactsDir, 0o750); err != nil {
		return "", fmt.Errorf("creating %s: %w", cfg.ArtifactsDir, err)
	}

	path := filepath.Join(cfg.ArtifactsDir, "merged.kubeconfig")
	if err := clientcmd.WriteToFile(*merged, path); err != nil {
		return "", fmt.Errorf("writing the merged kubeconfig to %s: %w", path, err)
	}

	return path, nil
}
