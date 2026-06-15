package gitlab

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

// WantedApplicationResources returns the app.k8s.io/v1beta1 Application objects from the
// template. The caller is responsible for checking CRD presence before applying them.
func WantedApplicationResources(template helm.Template) []client.Object {
	return template.Query().ObjectsByKind(ApplicationKind)
}
