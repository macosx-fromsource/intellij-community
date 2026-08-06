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

package v2alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

// ChartSpec specifies the version of the Helm chart that backs a resource and the free-form values
// it is rendered with.
//
// The Operator derives values from the structured fields of the specification and merges these
// values over that result. The free-form values win on conflict, which keeps them a working escape
// hatch for every setting the structured layer does not cover yet.
type ChartSpec struct {
	// Version is the semantic version of the Helm chart.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	Version string `json:"version,omitempty"`

	// Values is the set of Helm values that is used to render the chart. A validating webhook
	// checks the effective values against the `values.schema.json` of the chart.
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Values ChartValues `json:"values,omitempty"`
}

// ChartValues holds unstructured values for rendering a Helm chart.
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

// DeepCopyInto copies the receiver into out. Declaring this here prevents it from being generated.
func (u *ChartValues) DeepCopyInto(out *ChartValues) {
	out.Object = runtime.DeepCopyJSON(u.Object)
}

// GitLabReference names the GitLabCore resource that another resource belongs to. The reference is
// resolved in the namespace of the referring resource, which lets a namespace hold more than one
// GitLab instance.
type GitLabReference struct {
	// Name is the name of the GitLabCore resource in the same namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// SecretKeySelector selects a single key of a Secret in the namespace of the resource.
type SecretKeySelector struct {
	// Name is the name of the Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Key is the key of the Secret to read.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Key string `json:"key"`
}
