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

// SiphonSpec defines the desired state of a Siphon deployment.
//
// Siphon streams change data capture from a PostgreSQL source, through NATS JetStream, into a
// ClickHouse sink. The resource is a reference to those three, not a provisioner of them: the
// Operator renders the Siphon chart against the endpoints named here and applies it. Creating the
// PostgreSQL publication, the `siphon_alter_publication` function, the login roles and the grants,
// provisioning NATS, and creating the ClickHouse target tables are all administrator
// prerequisites. See doc/developer/siphon.md.
//
// The deployment topology is fixed: one producer, one ClickHouse consumer, one reconciler, one
// NATS stream. It is the layout GitLab is known to run. Nothing in the specification changes it;
// `spec.chart.values` is the escape hatch.
type SiphonSpec struct {
	// GitLabRef names the GitLabCore instance whose database this deployment streams. It resolves
	// the GitLab application version, which pins the table definitions to the deployed schema,
	// and it gates the rollout: the release is not applied until that instance reports available.
	// +kubebuilder:validation:Required
	GitLabRef GitLabReference `json:"gitlabRef"`

	// Source is the PostgreSQL server the change data capture stream reads.
	// +kubebuilder:validation:Required
	Source PostgreSQLSourceSpec `json:"source"`

	// Queue is the NATS JetStream server the stream passes through.
	// +kubebuilder:validation:Required
	Queue QueueSpec `json:"queue"`

	// Sink is the ClickHouse server the stream is written to.
	// +kubebuilder:validation:Required
	Sink ClickHouseSinkSpec `json:"sink"`

	// Tables selects how the table definitions of the deployed GitLab version reach the pods.
	// It defaults to an empty object so that the defaults of its own fields apply, which a nested
	// default does not do while the object itself is absent.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	Tables TablesSpec `json:"tables,omitempty"`

	// Chart is the specification of the Siphon chart that is used to deploy the pipeline.
	// +kubebuilder:validation:Optional
	Chart ChartSpec `json:"chart,omitempty"`
}

// PostgreSQLSourceSpec is the connection to the PostgreSQL server Siphon replicates from.
//
// The server must already carry the publication named by `status.publication`, owned by the GitLab
// application role, the `siphon_alter_publication` SECURITY DEFINER function, and the SELECT
// grants the initial snapshot needs. The Operator creates none of them.
type PostgreSQLSourceSpec struct {
	// Host is the hostname of the PostgreSQL server, for example
	// `gitlab-postgresql-rw.databases.svc.cluster.local`. It must be the primary: a logical
	// replication slot is not created on a standby.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Host string `json:"host"`

	// Port is the port of the PostgreSQL server.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=5432
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Database is the database to replicate. Changing it re-snapshots every table, so it is
	// immutable; delete and recreate the resource to point the pipeline elsewhere.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=gitlabhq_production
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.source.database is immutable"
	Database string `json:"database,omitempty"`

	// User is the login role the producer connects as. It needs REPLICATION, EXECUTE on
	// `siphon_alter_publication`, and SELECT on the replicated tables.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=siphon
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	User string `json:"user,omitempty"`

	// PasswordSecretRef selects the key of the Secret that holds the password of that role. The
	// chart creates no Secret and the Operator reads none: the value reaches the pod as an
	// environment variable and is substituted into the configuration at startup.
	// +kubebuilder:validation:Required
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef"`

	// SSLMode is the libpq TLS mode of the connection.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=require
	SSLMode PostgreSQLSSLMode `json:"sslMode,omitempty"`

	// AdvisoryLockID is the PostgreSQL advisory lock the producer takes to elect itself. Every
	// producer against this database must agree on it, and two Siphon deployments that share it
	// serialize rather than run in parallel. Change it only to run a second, independent stream
	// off the same database.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=2147483647
	AdvisoryLockID int32 `json:"advisoryLockID,omitempty"`
}

// PostgreSQLSSLMode is the libpq TLS mode of a PostgreSQL connection.
// +kubebuilder:validation:Enum=disable;allow;prefer;require;verify-ca;verify-full
type PostgreSQLSSLMode string

// QueueSpec is the connection to the NATS server the change data capture events pass through.
// JetStream must be enabled: Siphon uses streams, a key-value bucket for consumer election, and an
// object store for oversized events.
type QueueSpec struct {
	// URL is the NATS server URL, for example `nats://nats.nats.svc.cluster.local:4222`.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^nats://`
	URL string `json:"url"`

	// Auth is the user name and password the client authenticates with. Leave it unset for a
	// server that accepts anonymous clients.
	// +kubebuilder:validation:Optional
	Auth *QueueAuthSpec `json:"auth,omitempty"`

	// TLS is the client certificate the connection presents.
	// +kubebuilder:validation:Optional
	TLS *QueueTLSSpec `json:"tls,omitempty"`
}

// QueueAuthSpec holds the NATS credentials.
type QueueAuthSpec struct {
	// UsernameSecretRef selects the key of the Secret that holds the user name.
	// +kubebuilder:validation:Required
	UsernameSecretRef SecretKeySelector `json:"usernameSecretRef"`

	// PasswordSecretRef selects the key of the Secret that holds the password.
	// +kubebuilder:validation:Required
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef"`
}

// QueueTLSSpec names the Secret that holds the NATS client certificate, and the keys within it.
//
// The whole Secret is mounted read-only at /etc/ssl/certs/custom, so each key becomes a file of
// the same name and the key names are what the configuration points at.
type QueueTLSSpec struct {
	// SecretName is the name of the Secret in the namespace of the resource.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	SecretName string `json:"secretName"`

	// CACertKey is the key holding the certificate authority bundle.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=ca.crt
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	CACertKey string `json:"caCertKey,omitempty"`

	// ClientCertKey is the key holding the client certificate.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=tls.crt
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ClientCertKey string `json:"clientCertKey,omitempty"`

	// ClientKeyKey is the key holding the client private key.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=tls.key
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ClientKeyKey string `json:"clientKeyKey,omitempty"`
}

// ClickHouseSinkSpec is the connection to the ClickHouse server the stream is written to.
//
// The database must already hold the target tables. For GitLab those are created by the ClickHouse
// migrations of the application, not by Siphon, so the database and the user are required rather
// than defaulted: the Operator uses the reference it is given instead of guessing the schema
// layout of the instance.
//
// +kubebuilder:validation:XValidation:rule="self.port != 8123",message="spec.sink.port is the ClickHouse native protocol port, not the HTTP interface; 8123 is the HTTP interface and Siphon does not speak it"
type ClickHouseSinkSpec struct {
	// Host is the hostname of the ClickHouse server.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Host string `json:"host"`

	// Port is the port of the ClickHouse native protocol, not of the HTTP interface.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=9000
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Database is the database holding the target tables, for example
	// `gitlab_clickhouse_main_production`.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Database string `json:"database"`

	// Username is the ClickHouse user the consumer and the reconciler connect as. It needs INSERT
	// and SELECT on the target tables.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Username string `json:"username"`

	// PasswordSecretRef selects the key of the Secret that holds the password of that user.
	// +kubebuilder:validation:Required
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef"`

	// SSL turns TLS on for the native protocol connection. The port changes with it, so set both.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	SSL bool `json:"ssl,omitempty"`
}

// TablesSpec selects how the table definitions of the deployed GitLab version reach the pods.
//
// Each pod generates its half of the configuration at startup from those definitions, which ship
// as an OCI image tagged with the GitLab application version. The image is either mounted
// directly, which needs a recent Kubernetes, or extracted by the Operator into a ConfigMap it
// owns.
type TablesSpec struct {
	// Source selects where the definitions come from. `ImageVolume` mounts the image into the pod,
	// which needs both a cluster and a container runtime that serve native OCI image volumes.
	// `ConfigMap` has the Operator pull the image, extract the definitions, and publish them to a
	// ConfigMap it owns. `Auto`, the default, resolves to `ConfigMap`, which works everywhere:
	// nothing in the Kubernetes API reports whether the runtime serves image volumes, and a pod
	// that names one it cannot mount is admitted and then fails to create its container.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=Auto
	Source TablesSource `json:"source,omitempty"`

	// Image overrides the table definitions image. Left unset, it is
	// `registry.gitlab.com/gitlab-org/gitlab/gitlab-siphon-tables` tagged with the application
	// version of the GitLab chart the referenced instance deploys. Set it only to pin a version
	// the reference does not resolve to.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=512
	Image string `json:"image,omitempty"`

	// PullSecretRef names a `kubernetes.io/dockerconfigjson` Secret the image is pulled with. It
	// authenticates the pull the Operator performs on the `ConfigMap` path, and it becomes the pod
	// image pull secret on the `ImageVolume` path. Leave it unset for the public image.
	// +kubebuilder:validation:Optional
	PullSecretRef *LocalSecretReference `json:"pullSecretRef,omitempty"`
}

// TablesSource selects where the table definitions come from.
// +kubebuilder:validation:Enum=Auto;ImageVolume;ConfigMap
type TablesSource string

const (
	// TablesSourceAuto resolves to TablesSourceConfigMap, which works everywhere.
	TablesSourceAuto TablesSource = "Auto"
	// TablesSourceImageVolume mounts the image into the pod.
	TablesSourceImageVolume TablesSource = "ImageVolume"
	// TablesSourceConfigMap has the Operator extract the image into a ConfigMap.
	TablesSourceConfigMap TablesSource = "ConfigMap"
)

// SiphonStatus defines the observed state of a Siphon deployment.
type SiphonStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Version    string             `json:"version,omitempty"`
	Conditions []metav1.Condition `json:"conditions"`

	// GitLabVersion is the application version of the referenced instance, the version the table
	// definitions are pinned to.
	// +optional
	GitLabVersion string `json:"gitlabVersion,omitempty"`

	// TablesSource is what `spec.tables.source` resolved to, `ImageVolume` or `ConfigMap`.
	// +optional
	TablesSource string `json:"tablesSource,omitempty"`

	// TablesImage is the fully resolved table definitions image reference.
	// +optional
	TablesImage string `json:"tablesImage,omitempty"`

	// TablesDigest is the digest that image resolved to. It is the change detector for a tag that
	// moves, and it is empty while the definitions are mounted rather than extracted.
	// +optional
	TablesDigest string `json:"tablesDigest,omitempty"`

	// TablesCheckedAt is when the digest was last resolved against the registry. It paces that
	// lookup, which would otherwise run on every reconcile.
	// +optional
	TablesCheckedAt *metav1.Time `json:"tablesCheckedAt,omitempty"`

	// TablesConfigMap is the ConfigMap the Operator publishes the extracted definitions to.
	// +optional
	TablesConfigMap string `json:"tablesConfigMap,omitempty"`

	// TableCount is how many table definitions that ConfigMap holds.
	// +optional
	TableCount int32 `json:"tableCount,omitempty"`

	// Publication and ReplicationSlot are the PostgreSQL objects the producer uses. They are
	// derived from the fixed topology rather than observed, and they are recorded because deleting
	// this resource leaves both behind: a retained slot pins write-ahead log and will eventually
	// fill the volume of the source server.
	// +optional
	Publication string `json:"publication,omitempty"`
	// +optional
	ReplicationSlot string `json:"replicationSlot,omitempty"`

	// StreamName is the NATS JetStream stream the events pass through. It is left behind on
	// deletion too.
	// +optional
	StreamName string `json:"streamName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=sph
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="GITLAB",type=string,JSONPath=`.spec.gitlabRef.name`
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VERSION",type=string,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="TABLES",type=integer,JSONPath=`.status.tableCount`,priority=1
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 31",message="the name must be at most 31 characters: the Operator derives Deployment, ServiceAccount and pod selector names from it, and the longest of those is <name>-siphon-reconciler-clickhouse-sa, which a 63 character label value has to hold"
// +operator-sdk:csv:customresourcedefinitions:displayName="Siphon"
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,""},{ServiceAccount,v1,""},{Pod,v1,""},{Deployment,v1,""},{PodMonitor,v1,""}}

// Siphon streams change data capture from the PostgreSQL database of a GitLab instance into
// ClickHouse, through NATS JetStream.
//
// It is a namespace-scoped peer of GitLabCore and Orbit, backed by the Siphon chart. See ADR 27.
type Siphon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of a Siphon deployment.
	Spec SiphonSpec `json:"spec,omitempty"`

	// Most recently observed status of the Siphon deployment.
	// It is read-only to the user.
	Status SiphonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SiphonList contains a list of Siphon.
type SiphonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Siphon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Siphon{}, &SiphonList{})
}
