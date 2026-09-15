package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Default values for the configuration the harness reads from the environment.
// They mirror the defaults of the Taskfile so that `task e2e-suite` needs no
// extra configuration, and so that a bare `go test` behaves the same way.
const (
	// EnabledVar gates the whole harness. The suites need a cluster and a built
	// image, so they must not run as part of `go test ./...`.
	EnabledVar = "E2E"

	// The Operator image, assembled from the same components the Taskfile builds
	// with, so that the harness needs no configuration after `task docker-build-bridge`.
	defaultImageRegistry   = "registry.gitlab.com"
	defaultImageRepository = "gitlab-org/cloud-native"
	defaultImageName       = "gitlab-operator"
	defaultImageTag        = "latest-bridge"

	defaultOperatorNamespace = "gitlab-system"
	defaultNameOverride      = "gitlab"
	defaultK3sImage          = "rancher/k3s:v1.37.0-k3s1"
	defaultCertManager       = "1.19.2"
	defaultArtifactsDir      = ".build/e2e"
	defaultHelmBinary        = "helm"
	defaultHelmTimeout       = 5 * time.Minute
)

// ProviderName selects how the harness reaches a cluster.
type ProviderName string

// The cluster providers. Existing is the default: the harness must not assume it
// owns the cluster.
const (
	ProviderExisting ProviderName = "existing"
	ProviderK3s      ProviderName = "k3s"
)

// AddonMode says what the harness may do about a missing add-on.
type AddonMode string

// The add-on modes. Install writes cluster-scoped objects; Skip only probes, for
// a shared cluster the harness must not write to.
const (
	AddonModeInstall AddonMode = "install"
	AddonModeSkip    AddonMode = "skip"
)

// Config is every knob the harness reads, parsed once. The suites read it
// through Env rather than reaching for the environment themselves, so that the
// set of knobs is the set of fields here.
type Config struct {
	// Enabled reports whether E2E is set. TestE2E skips when it is not.
	Enabled bool

	// Strict turns every skip into a failure. Continuous integration sets it, so
	// that a job whose requirements went unsatisfied is red rather than an empty
	// green.
	Strict bool

	// Provider and the provider settings.
	Provider     ProviderName
	KubeContext  string
	K3sImage     string
	K3sPlatform  string
	K3dCluster   string
	ImageLoadCmd string
	KeepCluster  bool

	// Addons and the versions of the charts the harness installs.
	Addons             AddonMode
	CertManagerVersion string

	// Operator deployment.
	OperatorImage     string
	OperatorNamespace string
	NameOverride      string
	OperatorReuse     bool

	// Chart staging, so that a render failure names the missing archive.
	ChartsDirectory string
	ChartVersion    string

	// Debugging.
	KeepNamespace bool
	ArtifactsDir  string

	// Helm.
	HelmBinary  string
	HelmTimeout time.Duration

	// SiphonGitLabRef opts into the suite case that makes the Operator start
	// rendering a real GitLab release. It is off by default because that pulls
	// the toolbox image.
	SiphonGitLabRef bool
}

// LoadConfig reads the configuration from the environment.
//
// The default of OperatorReuse depends on the provider, so a developer running
// against their own cluster adopts the Operator that is already installed there
// rather than upgrading over it, while a throwaway k3s container always installs
// from scratch.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Enabled:            envBool(EnabledVar, false),
		Strict:             envBool("E2E_STRICT", false),
		Provider:           ProviderName(envString("E2E_CLUSTER_PROVIDER", string(ProviderExisting))),
		KubeContext:        os.Getenv("E2E_KUBE_CONTEXT"),
		K3sImage:           envString("E2E_K3S_IMAGE", defaultK3sImage),
		K3sPlatform:        os.Getenv("E2E_K3S_PLATFORM"),
		K3dCluster:         os.Getenv("E2E_K3D_CLUSTER"),
		ImageLoadCmd:       os.Getenv("E2E_IMAGE_LOAD_CMD"),
		KeepCluster:        envBool("E2E_KEEP_CLUSTER", false),
		Addons:             AddonMode(envString("E2E_ADDONS", string(AddonModeInstall))),
		CertManagerVersion: envString("E2E_CERTMANAGER_VERSION", defaultCertManager),
		OperatorImage:      envString("E2E_OPERATOR_IMAGE", defaultOperatorImage()),
		OperatorNamespace:  envString("E2E_OPERATOR_NAMESPACE", defaultOperatorNamespace),
		NameOverride:       envString("E2E_NAME_OVERRIDE", defaultNameOverride),
		ChartsDirectory:    os.Getenv("HELM_CHARTS"),
		ChartVersion:       os.Getenv("CHART_VERSION"),
		KeepNamespace:      envBool("E2E_KEEP_NAMESPACE", false),
		ArtifactsDir:       envString("E2E_ARTIFACTS_DIR", defaultArtifactsDir),
		HelmBinary:         envString("E2E_HELM_BINARY", defaultHelmBinary),
		HelmTimeout:        defaultHelmTimeout,
		SiphonGitLabRef:    envBool("E2E_SIPHON_GITLABREF", false),
	}

	cfg.OperatorReuse = envBool("E2E_OPERATOR_REUSE", cfg.Provider == ProviderExisting)

	switch cfg.Provider {
	case ProviderExisting, ProviderK3s:
	default:
		return nil, fmt.Errorf("E2E_CLUSTER_PROVIDER is %q, want %q or %q",
			cfg.Provider, ProviderExisting, ProviderK3s)
	}

	switch cfg.Addons {
	case AddonModeInstall, AddonModeSkip:
	default:
		return nil, fmt.Errorf("E2E_ADDONS is %q, want %q or %q",
			cfg.Addons, AddonModeInstall, AddonModeSkip)
	}

	if cfg.Provider == ProviderK3s {
		// The k3s container is created by this process, so there is no release to
		// adopt whatever the environment says.
		cfg.OperatorReuse = false
	}

	// Resolved against the repository root rather than the working directory: go
	// test runs in the package directory, so a relative path would scatter
	// artifacts under test/e2e/ instead of the .build the repository ignores.
	if !filepath.IsAbs(cfg.ArtifactsDir) {
		root, err := RepositoryRoot()
		if err != nil {
			return nil, err
		}

		cfg.ArtifactsDir = filepath.Join(root, cfg.ArtifactsDir)
	}

	return cfg, nil
}

// defaultOperatorImage is the tag `task docker-build-bridge` produces.
func defaultOperatorImage() string {
	return fmt.Sprintf("%s/%s/%s:%s",
		defaultImageRegistry, defaultImageRepository, defaultImageName, defaultImageTag)
}

// envString returns the value of the variable, or the fallback when it is unset
// or empty. An empty value counts as unset so that a continuous integration
// template can neutralize an inherited variable by setting it to "".
func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}

	return fallback
}

// envBool parses the variable as a boolean, accepting what strconv accepts, and
// falls back on anything else. A typo therefore never silently turns a knob off
// that defaults to on: it keeps the default.
func envBool(name string, fallback bool) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}

	return value
}
