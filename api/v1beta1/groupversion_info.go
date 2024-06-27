/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1beta1 contains API Schema definitions for the apps v1beta1 API group.
// +kubebuilder:object:generate=true
// +groupName=apps.gitlab.com
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/api"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	// GroupKind is a group kind used in webhooks.
	GroupKind = schema.GroupKind{Group: "apps.gitlab.com", Kind: "gitlab"}

	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "apps.gitlab.com", Version: "v1beta1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{
		GroupVersion: GroupVersion,
		SchemeBuilder: runtime.SchemeBuilder{
			clientgoscheme.AddToScheme,
			monitoringv1.AddToScheme,
			certmanagerv1.AddToScheme,
		},
	}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
