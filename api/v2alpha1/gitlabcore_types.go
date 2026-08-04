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

// GitLabCoreSpec defines the desired state of a GitLab instance.
//
// The structured fields are a thin typed layer over the values of the GitLab umbrella chart. For
// this iteration the layer covers the hostname and the license. Every other setting stays reachable
// through the free-form chart values.
type GitLabCoreSpec struct {
	// Hostname is the fully qualified domain name that the GitLab instance is reached at, for
	// example `gitlab.example.com`.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$`
	Hostname string `json:"hostname,omitempty"`

	// License references the GitLab license to activate the instance with. The license is a
	// reference to a Secret, so no license key is stored in the custom resource.
	// +kubebuilder:validation:Optional
	License *LicenseSpec `json:"license,omitempty"`

	// Chart is the specification of the GitLab umbrella chart that is used to deploy the instance.
	// +kubebuilder:validation:Optional
	Chart ChartSpec `json:"chart,omitempty"`
}

// LicenseSpec points at the Secret that holds the GitLab license key.
type LicenseSpec struct {
	// SecretRef selects the key of the Secret that holds the license.
	// +kubebuilder:validation:Required
	SecretRef SecretKeySelector `json:"secretRef"`
}

// GitLabCoreStatus defines the observed state of a GitLab instance.
type GitLabCoreStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Version    string             `json:"version,omitempty"`
	Conditions []metav1.Condition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=glc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VERSION",type=string,JSONPath=`.status.version`
// +operator-sdk:csv:customresourcedefinitions:displayName="GitLab Core"
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,""},{Secret,v1,""},{Service,v1,""},{Pod,v1,""},{Deployment,v1,""},{StatefulSet,v1,""},{PersistentVolumeClaim,v1,""}}

// GitLabCore is a complete DevOps platform, delivered in a single application.
//
// It is a definition of its own, separate from the `v1beta1` GitLab resource, and the two do not
// convert into each other.
type GitLabCore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of a GitLab instance.
	Spec GitLabCoreSpec `json:"spec,omitempty"`

	// Most recently observed status of the GitLab instance.
	// It is read-only to the user.
	Status GitLabCoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GitLabCoreList contains a list of GitLabCore.
type GitLabCoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitLabCore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitLabCore{}, &GitLabCoreList{})
}
