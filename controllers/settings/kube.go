package settings

import (
	"fmt"

	"github.com/pkg/errors"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	cfgEnvTest *rest.Config
)

func SetEnvTestConfig(cfg *rest.Config) {
	cfgEnvTest = cfg
}

func UnsetEnvTestConfig() {
	cfgEnvTest = nil
}

type KubeConfig struct {
	Config *rest.Config
	Error  error
}

func KubernetesConfig() KubeConfig {
	if cfgEnvTest != nil {
		return KubeConfig{Config: cfgEnvTest}
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return KubeConfig{Error: err}
	}

	return KubeConfig{Config: cfg}
}

func (k KubeConfig) NewKubernetesClient() (*kubernetes.Clientset, error) {
	conf := k.Config

	if err := k.Error; err != nil {
		panic(fmt.Sprintf("Error getting cluster config: %v", err))
	}

	return kubernetes.NewForConfig(conf)
}

// IsGroupVersionSupported checks for API endpoint for given Group and Version.
func IsGroupVersionSupported(group, version string) bool {
	client, err := KubernetesConfig().NewKubernetesClient()
	if err != nil {
		fmt.Printf("Unable to acquire k8s client: %v", err)
	}

	groupVersion := schema.GroupVersion{
		Group:   group,
		Version: version,
	}

	if err := discovery.ServerSupportsVersion(client, groupVersion); err != nil {
		return false
	}

	return true
}

func IsGroupVersionKindSupported(groupVersion, kind string) bool {
	client, err := KubernetesConfig().NewKubernetesClient()
	if err != nil {
		return false
	}

	rs, err := client.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return false
	}

	for _, r := range rs.APIResources {
		if r.Kind == kind {
			return true
		}
	}

	return false
}

func IsGroupVersionResourceSupported(groupVersion, resource string) bool {
	client, err := KubernetesConfig().NewKubernetesClient()
	if err != nil {
		return false
	}

	rs, err := client.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return false
	}

	for _, r := range rs.APIResources {
		if r.Name == resource {
			return true
		}
	}

	return false
}

func GetKubeCapabilities(actionCfg *action.Configuration) (*common.Capabilities, error) {
	dc, err := actionCfg.RESTClientGetter.ToDiscoveryClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not get Kubernetes discovery client")
	}

	dc.Invalidate()

	kubeVersion, err := dc.ServerVersion()
	if err != nil {
		return nil, errors.Wrap(err, "could not get server version from Kubernetes")
	}

	apiVersions, err := action.GetVersionSet(dc)
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, errors.Wrap(err, "could not get apiVersions from Kubernetes")
	}

	return &common.Capabilities{
		APIVersions: apiVersions,
		KubeVersion: common.KubeVersion{
			Version: kubeVersion.GitVersion,
			Major:   kubeVersion.Major,
			Minor:   kubeVersion.Minor,
		},
	}, nil
}
