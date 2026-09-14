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
// this iteration the layer covers the hostname, the edition, the license, and the connections to
// PostgreSQL, Redis, and object storage. Every other setting stays reachable through the free-form
// chart values.
type GitLabCoreSpec struct {
	// Hostname is the fully qualified domain name that the GitLab instance is reached at, for
	// example `gitlab.example.com`.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$`
	Hostname string `json:"hostname,omitempty"`

	// Edition is the GitLab edition to deploy: `ee` for Enterprise Edition, `ce` for Community
	// Edition. Enterprise Edition runs unlicensed with the Free feature set, so it is the default
	// and the edition to pick unless the instance has to carry no proprietary code at all.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=ee
	Edition Edition `json:"edition,omitempty"`

	// License references the GitLab license to activate the instance with. The license is a
	// reference to a Secret, so no license key is stored in the custom resource.
	// +kubebuilder:validation:Optional
	License *LicenseSpec `json:"license,omitempty"`

	// PostgreSQL is the PostgreSQL server the instance stores its data in. The chart bundles no
	// database, so an instance without this field, or without the equivalent free-form chart
	// values, does not render.
	// +kubebuilder:validation:Optional
	PostgreSQL *PostgreSQLSpec `json:"postgresql,omitempty"`

	// Redis is the Redis server the instance uses for caching, queues, and shared state. Valkey
	// serves as a drop-in replacement. The chart bundles no server, so an instance without this
	// field, or without the equivalent free-form chart values, does not render.
	// +kubebuilder:validation:Optional
	Redis *RedisSpec `json:"redis,omitempty"`

	// ObjectStorage is the S3 compatible object storage the instance keeps its artifacts,
	// uploads, and other blobs in. The chart enables object storage for these with no connection
	// of its own, so an instance without this field, or without the equivalent free-form chart
	// values, does not render.
	// +kubebuilder:validation:Optional
	ObjectStorage *ObjectStorageSpec `json:"objectStorage,omitempty"`

	// Chart is the specification of the GitLab umbrella chart that is used to deploy the instance.
	// +kubebuilder:validation:Optional
	Chart ChartSpec `json:"chart,omitempty"`

	// Upgrade tunes how the Operator carries out a zero-downtime upgrade.
	// +kubebuilder:validation:Optional
	Upgrade UpgradeSpec `json:"upgrade,omitempty"`
}

// UpgradeSpec tunes the zero-downtime upgrade choreography.
type UpgradeSpec struct {
	// SkipBatchedMigrationCheck lets an upgrade advance to the next version without waiting for the
	// batched background migrations of the current one to finish. GitLab requires those complete
	// before the next upgrade, so skipping the wait risks data inconsistency and failed migrations;
	// leave it off except on instances (for example development or test ones) where that risk is
	// acceptable in exchange for a faster upgrade.
	// +kubebuilder:validation:Optional
	SkipBatchedMigrationCheck bool `json:"skipBatchedMigrationCheck,omitempty"`
}

// Edition is the GitLab edition a instance is deployed from.
// +kubebuilder:validation:Enum=ce;ee
type Edition string

const (
	// EditionCE is the Community Edition, which carries the MIT-licensed code only.
	EditionCE Edition = "ce"
	// EditionEE is the Enterprise Edition, which runs the Free feature set until a license
	// activates more.
	EditionEE Edition = "ee"
)

// LicenseSpec points at the Secret that holds the GitLab license key.
type LicenseSpec struct {
	// SecretRef selects the key of the Secret that holds the license.
	// +kubebuilder:validation:Required
	SecretRef SecretKeySelector `json:"secretRef"`
}

// PostgreSQLSpec is the connection to the PostgreSQL server of the instance.
//
// For the versions and the extensions GitLab requires, see
// https://docs.gitlab.com/install/requirements/#postgresql.
type PostgreSQLSpec struct {
	// Host is the hostname of the PostgreSQL server, for example
	// `gitlab-postgresql.databases.svc.cluster.local`.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Host string `json:"host"`

	// PasswordSecretRef selects the key of the Secret that holds the password of the database
	// user. The password is a reference to a Secret, so no password is stored in the custom
	// resource.
	// +kubebuilder:validation:Required
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef"`
}

// RedisSpec is the connection to the Redis server of the instance. Valkey serves as a drop-in
// replacement.
//
// For the versions GitLab requires, see https://docs.gitlab.com/install/requirements/#redis.
type RedisSpec struct {
	// Host is the hostname of the Redis server, for example
	// `gitlab-valkey.databases.svc.cluster.local`.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Host string `json:"host"`

	// PasswordSecretRef selects the key of the Secret that holds the password of the server. The
	// password is a reference to a Secret, so no password is stored in the custom resource.
	// +kubebuilder:validation:Required
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef"`
}

// ObjectStorageSpec is the connection to the object storage of the instance.
//
// The connection is one Secret rather than a set of fields: it holds the endpoint, the region, and
// the credentials in the format the chart expects, and the Operator neither reads nor rewrites it.
// For the format, see
// https://docs.gitlab.com/charts/charts/globals/#connection.
//
// This covers the consolidated object storage of the application. The registry, the Pages daemon,
// and the backup toolbox read their own settings, which stay in the free-form chart values.
type ObjectStorageSpec struct {
	// ConnectionSecretRef selects the key of the Secret that holds the connection settings.
	// +kubebuilder:validation:Required
	ConnectionSecretRef SecretKeySelector `json:"connectionSecretRef"`
}

// GitLabCoreStatus defines the observed state of a GitLab instance.
type GitLabCoreStatus struct {
	Phase string `json:"phase,omitempty"`

	// Version is the version of the GitLab chart the release was rendered from, which is an
	// unrelated number to the version of the application it deploys. For that, read
	// GitLabVersion.
	Version string `json:"version,omitempty"`

	// GitLabVersion is the version of the application the release deploys, without the leading
	// `v`, such as `19.3.2`. It is recorded alongside Version, so it describes what is deployed
	// rather than what the spec asks for, and a resource stepping through a multi-minor upgrade
	// reports the version it has converged to so far.
	//
	// It is the only place the application version is published. Nothing else can derive it from
	// the resource: the chart version in the spec does not imply it, and the chart the Operator
	// renders may be one it does not carry on disk.
	// +optional
	GitLabVersion string `json:"gitlabVersion,omitempty"`

	Conditions []metav1.Condition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=glc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VERSION",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="GITLAB",type=string,JSONPath=`.status.gitlabVersion`
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
