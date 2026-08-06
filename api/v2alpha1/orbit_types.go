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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrbitSpec defines the desired state of an Orbit deployment.
//
// Orbit carries no structured settings yet. It accepts the reference to its GitLab instance, a
// chart version, and chart values.
type OrbitSpec struct {
	// GitLabRef names the GitLabCore instance that this deployment belongs to.
	// +kubebuilder:validation:Required
	GitLabRef GitLabReference `json:"gitlabRef"`

	// Chart is the specification of the Orbit chart that is used to deploy Orbit.
	// +kubebuilder:validation:Optional
	Chart ChartSpec `json:"chart,omitempty"`
}

// OrbitStatus defines the observed state of an Orbit deployment.
//
// ADR 26 defers the status design, including the conditions and the health detail that Bridge
// renders.
type OrbitStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Version    string             `json:"version,omitempty"`
	Conditions []metav1.Condition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=orb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="GITLAB",type=string,JSONPath=`.spec.gitlabRef.name`
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VERSION",type=string,JSONPath=`.status.version`
// +operator-sdk:csv:customresourcedefinitions:displayName="Orbit"
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,""},{Secret,v1,""},{Service,v1,""},{Pod,v1,""},{Deployment,v1,""},{StatefulSet,v1,""},{PersistentVolumeClaim,v1,""}}

// Orbit deploys the Orbit charts against a GitLab instance.
type Orbit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of an Orbit deployment.
	Spec OrbitSpec `json:"spec,omitempty"`

	// Most recently observed status of the Orbit deployment.
	// It is read-only to the user.
	Status OrbitStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrbitList contains a list of Orbit.
type OrbitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Orbit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Orbit{}, &OrbitList{})
}
