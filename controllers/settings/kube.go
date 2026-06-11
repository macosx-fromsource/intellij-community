package settings

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var settingslog = ctrl.Log.WithName("settings")

// managementVerbs are the verbs the Operator's ServiceAccount must be allowed to
// perform on a resource for the Operator to be able to fully manage (watch and
// modify) it.
var managementVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete"}

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

// CanManageResource reports whether the Operator's ServiceAccount is authorized to
// manage the given resource in the given API group. It performs a
// SelfSubjectAccessReview for each of the managementVerbs and only returns true
// when all of them are allowed.
// The checks are cluster wide if the namespace is empty.
func CanManageResource(client kubernetes.Interface, group, resource, namespace string) bool {
	return canManageResource(context.Background(), client, group, resource, namespace)
}

func canManageResource(ctx context.Context, client kubernetes.Interface, group, resource, namespace string) bool {
	for _, verb := range managementVerbs {
		review := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: namespace,
					Group:     group,
					Resource:  resource,
					Verb:      verb,
				},
			},
		}

		result, err := client.AuthorizationV1().SelfSubjectAccessReviews().
			Create(ctx, review, metav1.CreateOptions{})
		if err != nil {
			settingslog.Error(err, "unable to perform SelfSubjectAccessReview",
				"verb", verb, "group", group, "resource", resource)

			return false
		}

		if !result.Status.Allowed {
			settingslog.Info("SelfSubjectAccessReview denied",
				"verb", verb, "group", group, "resource", resource)

			return false
		}
	}

	return true
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
