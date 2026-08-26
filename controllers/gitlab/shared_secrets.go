package gitlab

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

// SharedSecretsConfigMap returns the ConfigMaps of Shared Secret component.
func SharedSecretsConfigMap(adapter gitlab.Adapter, template helm.Template) (client.Object, error) {
	cfgMapName := fmt.Sprintf("%s-%s", adapter.ReleaseName(), SharedSecretsComponentName)
	cfgMap := template.Query().ObjectByKindAndName(ConfigMapKind, cfgMapName)

	return cfgMap, nil
}

// SharedSecretsJob returns the Job for Shared Secret component.
func SharedSecretsJob(adapter gitlab.Adapter, template helm.Template) (client.Object, error) {
	jobs := template.Query().ObjectsByKindAndLabels(JobKind, map[string]string{
		AppLabel: GitLabComponentName,
	})

	namePrefix := fmt.Sprintf("%s-%s", adapter.ReleaseName(), SharedSecretsComponentName)
	for _, j := range jobs {
		if strings.HasPrefix(j.GetName(), namePrefix) && !strings.HasSuffix(j.GetName(), "-selfsign") {
			return j, nil
		}
	}

	return nil, nil
}

// SelfSignedCertsJob returns the Job for Self Signed Certificates component.
func SelfSignedCertsJob(adapter gitlab.Adapter, template helm.Template) (client.Object, error) {
	jobs := template.Query().ObjectsByKindAndLabels(JobKind, map[string]string{
		AppLabel: GitLabComponentName,
	})

	namePrefix := fmt.Sprintf("%s-%s", adapter.ReleaseName(), SharedSecretsComponentName)
	for _, j := range jobs {
		if strings.HasPrefix(j.GetName(), namePrefix) && strings.HasSuffix(j.GetName(), "-selfsign") {
			return j, nil
		}
	}

	return nil, nil
}

// SharedSecretsResources returns Kubernetes resources for running shared secrets job.
func SharedSecretsResources(adapter gitlab.Adapter, template helm.Template) (client.Object, client.Object, error) {
	cfgMap, err := SharedSecretsConfigMap(adapter, template)
	if err != nil {
		return cfgMap, nil, err
	}

	job, err := SharedSecretsJob(adapter, template)
	if err != nil {
		return cfgMap, job, err
	}

	return cfgMap, job, nil
}
