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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitLabSpec defines the desired state of GitLab.
type GitLabSpec struct {
	// The specification of GitLab Chart that is used to deploy the instance.
	Chart GitLabChartSpec `json:"chart,omitempty"`
}

// GitLabChartSpec specifies GitLab Chart version and values.
type GitLabChartSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	// ChartVersion is the semantic version of the GitLab Chart.
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// ChartValues is the set of Helm values that is used to render the GitLab Chart.
	Values ChartValues `json:"values,omitempty"`
}

// Unstructured values for rendering GitLab Chart.
// +k8s:deepcopy-gen=false
type ChartValues struct {
	// Object is a JSON compatible map with string, float, int, bool, []interface{}, or
	// map[string]interface{} children.
	Object map[string]interface{} `json:"-"`
}

// MarshalJSON ensures that the unstructured object produces proper
// JSON when passed to Go's standard JSON library.
func (u *ChartValues) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.Object)
}

// UnmarshalJSON ensures that the unstructured object properly decodes
// JSON when passed to Go's standard JSON library.
func (u *ChartValues) UnmarshalJSON(data []byte) error {
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	u.Object = m

	return nil
}

// Declaring this here prevents it from being generated.
func (u *ChartValues) DeepCopyInto(out *ChartValues) {
	out.Object = runtime.DeepCopyJSON(u.Object)
}

// GitLabStatus defines the observed state of GitLab.
type GitLabStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Version    string             `json:"version,omitempty"`
	Conditions []metav1.Condition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=gl
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VERSION",type=string,JSONPath=`.status.version`
// +operator-sdk:csv:customresourcedefinitions:displayName="GitLab"
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,""},{Secret,v1,""},{Service,v1,""},{Pod,v1,""},{Deployment,v1,""},{StatefulSet,v1,""},{PersistentVolumeClaim,v1,""}}

// GitLab is a complete DevOps platform, delivered in a single application.
type GitLab struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of a GitLab instance.
	Spec GitLabSpec `json:"spec,omitempty"`

	// Most recently observed status of the GitLab instance.
	// It is read-only to the user.
	Status GitLabStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GitLabList contains a list of GitLab.
type GitLabList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitLab `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitLab{}, &GitLabList{})
}
