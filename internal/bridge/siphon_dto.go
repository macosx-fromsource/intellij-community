package bridge

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// SiphonResource is the wire representation of a Siphon custom resource
// (`apps.gitlab.com/v2alpha1`), the change data capture pipeline that streams
// the PostgreSQL database of a GitLab instance into ClickHouse through NATS
// JetStream.
//
// A Siphon belongs to the instance it references, so the endpoints nest under
// that instance: `name`, `namespace` and `gitlabRef` are filled in from the
// path and returned rather than sent. Everything else mirrors the
// specification, with the constraints of the custom resource definition
// repeated as validation tags, the same way GitLabResource does.
//
// The three servers are referenced, never provisioned. The PostgreSQL
// publication, the `siphon_alter_publication` function, the login roles and the
// grants, the NATS server, and the ClickHouse target tables are all
// administrator prerequisites; see doc/developer/siphon.md.
type SiphonResource struct {
	Name      string           `json:"name,omitempty" readOnly:"true" doc:"Name of the Siphon resource. The bridge derives it from the GitLab instance when it creates one."`
	Namespace string           `json:"namespace,omitempty" readOnly:"true" doc:"Namespace the Siphon resource lives in, which is the namespace of the GitLab instance."`
	GitLabRef string           `json:"gitlabRef,omitempty" readOnly:"true" doc:"Name of the GitLabCore instance whose database this pipeline streams."`
	Source    SiphonSourceDTO  `json:"source" doc:"PostgreSQL server the change data capture stream reads. It must be the primary: a logical replication slot is not created on a standby."`
	Queue     SiphonQueueDTO   `json:"queue" doc:"NATS server the stream passes through. JetStream must be enabled: Siphon uses streams, a key-value bucket, and an object store."`
	Sink      SiphonSinkDTO    `json:"sink" doc:"ClickHouse server the stream is written to, over the native protocol. Its target tables are created by the ClickHouse migrations of GitLab, not by Siphon."`
	Tables    SiphonTablesDTO  `json:"tables,omitempty" doc:"How the table definitions of the deployed GitLab version reach the pods. They ship as an OCI image tagged with the GitLab version the referenced instance runs."`
	Chart     ChartDTO         `json:"chart" doc:"Siphon chart configuration. Only a version the Operator bundles renders: unlike the GitLab chart, a Siphon chart is never pulled."`
	Status    *SiphonStatusDTO `json:"status,omitempty" readOnly:"true" doc:"Most recently observed status. Read-only."`
}

// SiphonSourceDTO is the connection to the PostgreSQL server Siphon replicates
// from.
type SiphonSourceDTO struct {
	Host              string       `json:"host" minLength:"1" maxLength:"253" pattern:"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$" example:"gitlab-postgresql-rw.databases.svc.cluster.local" doc:"Hostname of the PostgreSQL primary."`
	Port              int32        `json:"port,omitempty" minimum:"1" maximum:"65535" example:"5432" doc:"Port of the PostgreSQL server. Defaults to 5432."`
	Database          string       `json:"database,omitempty" minLength:"1" maxLength:"63" example:"gitlabhq_production" doc:"Database to replicate. Defaults to 'gitlabhq_production' and is immutable: changing it re-snapshots every table."`
	User              string       `json:"user,omitempty" minLength:"1" maxLength:"63" example:"siphon" doc:"Login role the producer connects as. It needs REPLICATION, EXECUTE on siphon_alter_publication, and SELECT on the replicated tables. Defaults to 'siphon'."`
	PasswordSecretRef SecretRefDTO `json:"passwordSecretRef" doc:"Key of the Secret that holds the password of that role."`
	SSLMode           string       `json:"sslMode,omitempty" enum:"disable,allow,prefer,require,verify-ca,verify-full" example:"require" doc:"libpq TLS mode of the connection. Defaults to 'require'."`
	AdvisoryLockID    int32        `json:"advisoryLockID,omitempty" minimum:"1" maximum:"2147483647" doc:"PostgreSQL advisory lock the producer takes to elect itself. Defaults to 1; change it only to run a second, independent stream off the same database."`
}

// SiphonQueueDTO is the connection to the NATS server the events pass through.
type SiphonQueueDTO struct {
	URL  string              `json:"url" minLength:"1" maxLength:"2048" pattern:"^nats://" example:"nats://nats.nats.svc.cluster.local:4222" doc:"URL of the NATS server."`
	Auth *SiphonQueueAuthDTO `json:"auth,omitempty" doc:"User name and password the client authenticates with. Leave it out for a server that accepts anonymous clients."`
	TLS  *SiphonQueueTLSDTO  `json:"tls,omitempty" doc:"Client certificate the connection presents."`
}

// SiphonQueueAuthDTO holds the references to the NATS credentials.
type SiphonQueueAuthDTO struct {
	UsernameSecretRef SecretRefDTO `json:"usernameSecretRef" doc:"Key of the Secret that holds the user name."`
	PasswordSecretRef SecretRefDTO `json:"passwordSecretRef" doc:"Key of the Secret that holds the password."`
}

// SiphonQueueTLSDTO names the Secret holding the NATS client certificate. The
// whole Secret is mounted, so each key becomes a file of the same name.
type SiphonQueueTLSDTO struct {
	SecretName    string `json:"secretName" minLength:"1" maxLength:"253" doc:"Name of the Secret in the namespace of the resource."`
	CACertKey     string `json:"caCertKey,omitempty" minLength:"1" maxLength:"253" example:"ca.crt" doc:"Key holding the certificate authority bundle. Defaults to 'ca.crt'."`
	ClientCertKey string `json:"clientCertKey,omitempty" minLength:"1" maxLength:"253" example:"tls.crt" doc:"Key holding the client certificate. Defaults to 'tls.crt'."`
	ClientKeyKey  string `json:"clientKeyKey,omitempty" minLength:"1" maxLength:"253" example:"tls.key" doc:"Key holding the client private key. Defaults to 'tls.key'."`
}

// SiphonSinkDTO is the connection to the ClickHouse server the stream is
// written to.
type SiphonSinkDTO struct {
	Host              string       `json:"host" minLength:"1" maxLength:"253" pattern:"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$" example:"clickhouse.databases.svc.cluster.local" doc:"Hostname of the ClickHouse server."`
	Port              int32        `json:"port,omitempty" minimum:"1" maximum:"65535" example:"9000" doc:"Port of the ClickHouse native protocol, not of the HTTP interface. Defaults to 9000; 8123 is the HTTP interface, which Siphon does not speak."`
	Database          string       `json:"database" minLength:"1" maxLength:"63" example:"gitlab_clickhouse_main_production" doc:"Database holding the target tables."`
	Username          string       `json:"username" minLength:"1" maxLength:"63" example:"gitlab" doc:"ClickHouse user the consumer and the reconciler connect as. It needs INSERT and SELECT on the target tables."`
	PasswordSecretRef SecretRefDTO `json:"passwordSecretRef" doc:"Key of the Secret that holds the password of that user."`
	SSL               bool         `json:"ssl,omitempty" doc:"Turns TLS on for the native protocol connection. The port changes with it, so set both."`
}

// SiphonTablesDTO selects how the table definitions reach the pods.
type SiphonTablesDTO struct {
	Source        string             `json:"source,omitempty" enum:"Auto,ImageVolume,ConfigMap" example:"Auto" doc:"Where the definitions come from. 'ImageVolume' mounts the image into the pod, which needs a runtime that serves OCI image volumes. 'ConfigMap' has the Operator extract it. 'Auto', the default, resolves to 'ConfigMap'."`
	Image         string             `json:"image,omitempty" maxLength:"512" doc:"Overrides the table definitions image. Left out, it is the GitLab image tagged with the application version of the referenced instance."`
	PullSecretRef *LocalSecretRefDTO `json:"pullSecretRef,omitempty" doc:"A kubernetes.io/dockerconfigjson Secret the image is pulled with. Leave it out for the public image."`
}

// LocalSecretRefDTO names a Secret in the namespace of the resource, without
// selecting a key.
type LocalSecretRefDTO struct {
	Name string `json:"name" minLength:"1" maxLength:"253" doc:"Name of the Secret."`
}

// SiphonStatusDTO is the read-only observed state of a Siphon resource. The
// PostgreSQL and NATS objects are reported because deleting the resource leaves
// them behind, and a retained replication slot pins write-ahead log on the
// source server.
type SiphonStatusDTO struct {
	Phase           string             `json:"phase,omitempty" doc:"Current lifecycle phase."`
	Version         string             `json:"version,omitempty" doc:"Deployed Siphon chart version."`
	GitLabVersion   string             `json:"gitlabVersion,omitempty" doc:"Application version of the referenced instance, the version the table definitions are pinned to."`
	TablesSource    string             `json:"tablesSource,omitempty" doc:"What the table definitions source resolved to, 'ImageVolume' or 'ConfigMap'."`
	TablesImage     string             `json:"tablesImage,omitempty" doc:"Fully resolved table definitions image reference."`
	TableCount      int32              `json:"tableCount,omitempty" doc:"How many table definitions the pipeline carries."`
	Publication     string             `json:"publication,omitempty" doc:"PostgreSQL publication the producer reads. Deleting the resource leaves it behind."`
	ReplicationSlot string             `json:"replicationSlot,omitempty" doc:"PostgreSQL replication slot the producer holds. Deleting the resource leaves it behind."`
	StreamName      string             `json:"streamName,omitempty" doc:"NATS JetStream stream the events pass through. Deleting the resource leaves it behind."`
	Conditions      []metav1.Condition `json:"conditions,omitempty" doc:"Detailed status conditions."`
}

// toSiphonResource converts an internal Siphon object into its wire
// representation.
func toSiphonResource(siphon *apiv2alpha1.Siphon) SiphonResource {
	spec := siphon.Spec

	res := SiphonResource{
		Name:      siphon.Name,
		Namespace: siphon.Namespace,
		GitLabRef: spec.GitLabRef.Name,
		Source: SiphonSourceDTO{
			Host:              spec.Source.Host,
			Port:              spec.Source.Port,
			Database:          spec.Source.Database,
			User:              spec.Source.User,
			PasswordSecretRef: toSecretRef(spec.Source.PasswordSecretRef),
			SSLMode:           string(spec.Source.SSLMode),
			AdvisoryLockID:    spec.Source.AdvisoryLockID,
		},
		Queue: SiphonQueueDTO{URL: spec.Queue.URL},
		Sink: SiphonSinkDTO{
			Host:              spec.Sink.Host,
			Port:              spec.Sink.Port,
			Database:          spec.Sink.Database,
			Username:          spec.Sink.Username,
			PasswordSecretRef: toSecretRef(spec.Sink.PasswordSecretRef),
			SSL:               spec.Sink.SSL,
		},
		Tables: SiphonTablesDTO{
			Source: string(spec.Tables.Source),
			Image:  spec.Tables.Image,
		},
		Chart: ChartDTO{
			Version: spec.Chart.Version,
			Values:  spec.Chart.Values.Object,
		},
	}

	if auth := spec.Queue.Auth; auth != nil {
		res.Queue.Auth = &SiphonQueueAuthDTO{
			UsernameSecretRef: toSecretRef(auth.UsernameSecretRef),
			PasswordSecretRef: toSecretRef(auth.PasswordSecretRef),
		}
	}

	if tls := spec.Queue.TLS; tls != nil {
		res.Queue.TLS = &SiphonQueueTLSDTO{
			SecretName:    tls.SecretName,
			CACertKey:     tls.CACertKey,
			ClientCertKey: tls.ClientCertKey,
			ClientKeyKey:  tls.ClientKeyKey,
		}
	}

	if pull := spec.Tables.PullSecretRef; pull != nil {
		res.Tables.PullSecretRef = &LocalSecretRefDTO{Name: pull.Name}
	}

	// A Siphon the controller has not reported on yet carries no status, so
	// the panel of the add-on shows one once there is something to show
	// rather than a row of dashes from an empty object.
	if !reflect.DeepEqual(siphon.Status, apiv2alpha1.SiphonStatus{}) {
		res.Status = &SiphonStatusDTO{
			Phase:           siphon.Status.Phase,
			Version:         siphon.Status.Version,
			GitLabVersion:   siphon.Status.GitLabVersion,
			TablesSource:    siphon.Status.TablesSource,
			TablesImage:     siphon.Status.TablesImage,
			TableCount:      siphon.Status.TableCount,
			Publication:     siphon.Status.Publication,
			ReplicationSlot: siphon.Status.ReplicationSlot,
			StreamName:      siphon.Status.StreamName,
			Conditions:      siphon.Status.Conditions,
		}
	}

	return res
}

// applyToSiphon writes the client-supplied fields of a SiphonResource onto an
// internal Siphon object, referencing the GitLab instance the endpoint nests
// under. Status is never applied because it is read-only, and every writable
// field is assigned: the operation replaces the resource rather than patching
// it, so an omitted field clears what the object holds.
func applyToSiphon(res SiphonResource, gitlab string, siphon *apiv2alpha1.Siphon) {
	siphon.Spec.GitLabRef = apiv2alpha1.GitLabReference{Name: gitlab}

	siphon.Spec.Source = apiv2alpha1.PostgreSQLSourceSpec{
		Host:              res.Source.Host,
		Port:              res.Source.Port,
		Database:          res.Source.Database,
		User:              res.Source.User,
		PasswordSecretRef: toSecretKeySelector(res.Source.PasswordSecretRef),
		SSLMode:           apiv2alpha1.PostgreSQLSSLMode(res.Source.SSLMode),
		AdvisoryLockID:    res.Source.AdvisoryLockID,
	}

	siphon.Spec.Queue = apiv2alpha1.QueueSpec{URL: res.Queue.URL}

	if auth := res.Queue.Auth; auth != nil {
		siphon.Spec.Queue.Auth = &apiv2alpha1.QueueAuthSpec{
			UsernameSecretRef: toSecretKeySelector(auth.UsernameSecretRef),
			PasswordSecretRef: toSecretKeySelector(auth.PasswordSecretRef),
		}
	}

	if tls := res.Queue.TLS; tls != nil {
		siphon.Spec.Queue.TLS = &apiv2alpha1.QueueTLSSpec{
			SecretName:    tls.SecretName,
			CACertKey:     tls.CACertKey,
			ClientCertKey: tls.ClientCertKey,
			ClientKeyKey:  tls.ClientKeyKey,
		}
	}

	siphon.Spec.Sink = apiv2alpha1.ClickHouseSinkSpec{
		Host:              res.Sink.Host,
		Port:              res.Sink.Port,
		Database:          res.Sink.Database,
		Username:          res.Sink.Username,
		PasswordSecretRef: toSecretKeySelector(res.Sink.PasswordSecretRef),
		SSL:               res.Sink.SSL,
	}

	siphon.Spec.Tables = apiv2alpha1.TablesSpec{
		Source: apiv2alpha1.TablesSource(res.Tables.Source),
		Image:  res.Tables.Image,
	}

	if pull := res.Tables.PullSecretRef; pull != nil {
		siphon.Spec.Tables.PullSecretRef = &apiv2alpha1.LocalSecretReference{Name: pull.Name}
	}

	siphon.Spec.Chart.Version = res.Chart.Version
	siphon.Spec.Chart.Values.Object = res.Chart.Values
}
