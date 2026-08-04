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

// Package v2alpha1 contains API Schema definitions for the apps v2alpha1 API group.
//
// The group holds one custom resource per Helm chart that the Operator deploys, as decided in
// ADR 24 and designed in ADR 26. All resources are namespace-scoped peers: GitLab, Orbit, and
// DataInsightPlatform.
//
// +kubebuilder:object:generate=true
// +groupName=apps.gitlab.com
package v2alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "apps.gitlab.com", Version: "v2alpha1"}

	// GitLabCoreGroupKind is the group kind of the GitLabCore resource, used in webhooks.
	GitLabCoreGroupKind = schema.GroupKind{Group: GroupVersion.Group, Kind: "GitLabCore"}

	// OrbitGroupKind is the group kind of the Orbit resource, used in webhooks.
	OrbitGroupKind = schema.GroupKind{Group: GroupVersion.Group, Kind: "Orbit"}

	// DataInsightPlatformGroupKind is the group kind of the DataInsightPlatform resource, used in
	// webhooks.
	DataInsightPlatformGroupKind = schema.GroupKind{Group: GroupVersion.Group, Kind: "DataInsightPlatform"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
