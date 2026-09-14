// Package bridge implements the backend-for-frontend HTTP server that exposes
// CRUD operations over the GitLabCore custom resource, serves an OpenAPI
// document, and hosts the single-page application used to configure GitLab
// instances.
package bridge

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// GitLabResource is the wire representation of a GitLabCore custom resource
// (`apps.gitlab.com/v2alpha1`). It intentionally exposes only the fields that a
// client can read or write, rather than reflecting the full Kubernetes object
// metadata.
//
// The structured fields mirror the specification of the custom resource, so the
// generated OpenAPI document carries the same shape and the same constraints as
// the definition the API server validates against. Everything the structured
// layer does not cover yet stays reachable through the free-form chart values.
type GitLabResource struct {
	Name       string            `json:"name" doc:"Name of the GitLab resource."`
	Namespace  string            `json:"namespace" doc:"Namespace the GitLab resource lives in."`
	Labels     map[string]string `json:"labels,omitempty" doc:"Labels applied to the GitLab resource."`
	Hostname   string            `json:"hostname,omitempty" maxLength:"253" pattern:"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$" example:"gitlab.example.com" doc:"Fully qualified domain name the GitLab instance is reached at."`
	Edition    string            `json:"edition,omitempty" enum:"ce,ee" example:"ee" doc:"GitLab edition to deploy: 'ee' for Enterprise Edition, which runs unlicensed with the Free feature set, or 'ce' for Community Edition."`
	License    *LicenseDTO       `json:"license,omitempty" doc:"Reference to the GitLab license to activate the instance with."`
	PostgreSQL *PostgreSQLDTO    `json:"postgresql,omitempty" doc:"PostgreSQL server the instance stores its data in. The chart bundles no database. See https://docs.gitlab.com/install/requirements/#postgresql."`
	Redis      *RedisDTO         `json:"redis,omitempty" doc:"Redis server the instance uses for caching, queues, and shared state; Valkey serves as a drop-in replacement. The chart bundles no server. See https://docs.gitlab.com/install/requirements/#redis."`
	//nolint:lll // One doc string per field, and the URL is part of it.
	ObjectStorage *ObjectStorageDTO `json:"objectStorage,omitempty" doc:"S3 compatible object storage the instance keeps its artifacts, uploads, and other blobs in. The chart needs one: it enables object storage for these with no connection of its own. See https://docs.gitlab.com/charts/charts/globals/#connection."`
	Chart         ChartDTO          `json:"chart" doc:"GitLab Chart configuration."`
	Status        *StatusDTO        `json:"status,omitempty" readOnly:"true" doc:"Most recently observed status. Read-only."`
}

// LicenseDTO points at the Secret that holds the GitLab license key. Only the
// reference travels over the wire, so no license key passes through the bridge
// or is stored in the custom resource.
type LicenseDTO struct {
	SecretRef SecretRefDTO `json:"secretRef" doc:"Key of the Secret that holds the license."`
}

// PostgreSQLDTO is the connection to the PostgreSQL server of the instance.
type PostgreSQLDTO struct {
	Host              string       `json:"host" minLength:"1" maxLength:"253" example:"gitlab-postgresql.databases.svc.cluster.local" doc:"Hostname of the PostgreSQL server."`
	PasswordSecretRef SecretRefDTO `json:"passwordSecretRef" doc:"Key of the Secret that holds the password of the database user."`
}

// RedisDTO is the connection to the Redis server of the instance.
type RedisDTO struct {
	Host              string       `json:"host" minLength:"1" maxLength:"253" example:"gitlab-valkey.databases.svc.cluster.local" doc:"Hostname of the Redis server."`
	PasswordSecretRef SecretRefDTO `json:"passwordSecretRef" doc:"Key of the Secret that holds the password of the server."`
}

// ObjectStorageDTO is the connection to the object storage of the instance. The
// Secret holds the endpoint, the region, and the credentials in the format the
// chart expects; the bridge neither reads nor rewrites it.
type ObjectStorageDTO struct {
	ConnectionSecretRef SecretRefDTO `json:"connectionSecretRef" doc:"Key of the Secret that holds the connection settings."`
}

// SecretRefDTO selects a single key of a Secret in the namespace of the
// resource. Only the reference travels over the wire: no credential passes
// through the bridge or is stored in the custom resource.
type SecretRefDTO struct {
	Name string `json:"name" minLength:"1" maxLength:"253" doc:"Name of the Secret."`
	Key  string `json:"key" minLength:"1" maxLength:"253" doc:"Key of the Secret to read."`
}

// ChartDTO carries the GitLab Chart version and Helm values.
type ChartDTO struct {
	Version string         `json:"version,omitempty" doc:"Semantic version of the GitLab Chart."`
	Values  map[string]any `json:"values,omitempty" doc:"Free-form Helm values used to render the GitLab Chart. They are merged over the values derived from the structured fields and win on conflict."`
}

// StatusDTO is the read-only observed state of a GitLab resource.
type StatusDTO struct {
	Phase         string             `json:"phase,omitempty" doc:"Current lifecycle phase."`
	Version       string             `json:"version,omitempty" doc:"Deployed GitLab chart version, an unrelated number to the GitLab version it deploys."`
	GitLabVersion string             `json:"gitlabVersion,omitempty" doc:"Deployed GitLab version, such as 19.3.2."`
	Conditions    []metav1.Condition `json:"conditions,omitempty" doc:"Detailed status conditions."`
}

// toResource converts an internal GitLabCore object into its wire
// representation.
func toResource(core *apiv2alpha1.GitLabCore) GitLabResource {
	res := GitLabResource{
		Name:      core.Name,
		Namespace: core.Namespace,
		Labels:    core.Labels,
		Hostname:  core.Spec.Hostname,
		Edition:   string(core.Spec.Edition),
		Chart: ChartDTO{
			Version: core.Spec.Chart.Version,
			Values:  core.Spec.Chart.Values.Object,
		},
	}

	if license := core.Spec.License; license != nil {
		res.License = &LicenseDTO{SecretRef: toSecretRef(license.SecretRef)}
	}

	if psql := core.Spec.PostgreSQL; psql != nil {
		res.PostgreSQL = &PostgreSQLDTO{
			Host:              psql.Host,
			PasswordSecretRef: toSecretRef(psql.PasswordSecretRef),
		}
	}

	if redis := core.Spec.Redis; redis != nil {
		res.Redis = &RedisDTO{
			Host:              redis.Host,
			PasswordSecretRef: toSecretRef(redis.PasswordSecretRef),
		}
	}

	if storage := core.Spec.ObjectStorage; storage != nil {
		res.ObjectStorage = &ObjectStorageDTO{
			ConnectionSecretRef: toSecretRef(storage.ConnectionSecretRef),
		}
	}

	res.Status = &StatusDTO{
		Phase:         core.Status.Phase,
		Version:       core.Status.Version,
		GitLabVersion: core.Status.GitLabVersion,
		Conditions:    core.Status.Conditions,
	}

	return res
}

// applyToGitLabCore writes the client-supplied fields of a GitLabResource onto
// an internal GitLabCore object. Status is never applied because it is
// read-only. Every writable field is assigned, including the ones the request
// leaves out: the operations that use this replace the resource rather than
// patch it, so an omitted field clears what the object holds.
func applyToGitLabCore(res GitLabResource, core *apiv2alpha1.GitLabCore) {
	core.Name = res.Name
	core.Namespace = res.Namespace

	if res.Labels != nil {
		core.Labels = res.Labels
	}

	core.Spec.Hostname = res.Hostname
	core.Spec.Edition = apiv2alpha1.Edition(res.Edition)
	core.Spec.License = nil
	core.Spec.PostgreSQL = nil
	core.Spec.Redis = nil
	core.Spec.ObjectStorage = nil

	if res.License != nil {
		core.Spec.License = &apiv2alpha1.LicenseSpec{
			SecretRef: toSecretKeySelector(res.License.SecretRef),
		}
	}

	if res.PostgreSQL != nil {
		core.Spec.PostgreSQL = &apiv2alpha1.PostgreSQLSpec{
			Host:              res.PostgreSQL.Host,
			PasswordSecretRef: toSecretKeySelector(res.PostgreSQL.PasswordSecretRef),
		}
	}

	if res.Redis != nil {
		core.Spec.Redis = &apiv2alpha1.RedisSpec{
			Host:              res.Redis.Host,
			PasswordSecretRef: toSecretKeySelector(res.Redis.PasswordSecretRef),
		}
	}

	if res.ObjectStorage != nil {
		core.Spec.ObjectStorage = &apiv2alpha1.ObjectStorageSpec{
			ConnectionSecretRef: toSecretKeySelector(res.ObjectStorage.ConnectionSecretRef),
		}
	}

	core.Spec.Chart.Version = res.Chart.Version
	core.Spec.Chart.Values.Object = res.Chart.Values
}

// toSecretRef converts a Secret key selector into its wire representation.
func toSecretRef(selector apiv2alpha1.SecretKeySelector) SecretRefDTO {
	return SecretRefDTO{Name: selector.Name, Key: selector.Key}
}

// toSecretKeySelector converts a wire Secret reference into the internal one.
func toSecretKeySelector(ref SecretRefDTO) apiv2alpha1.SecretKeySelector {
	return apiv2alpha1.SecretKeySelector{Name: ref.Name, Key: ref.Key}
}
