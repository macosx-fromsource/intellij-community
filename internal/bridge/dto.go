// Package bridge implements the backend-for-frontend HTTP server that exposes
// CRUD operations over the GitLab custom resource, serves an OpenAPI document,
// and hosts the single-page application used to configure GitLab instances.
package bridge

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
)

// GitLabResource is the wire representation of a GitLab custom resource. It
// intentionally exposes only the fields that a client can read or write, rather
// than reflecting the full Kubernetes object metadata.
type GitLabResource struct {
	Name      string            `json:"name" doc:"Name of the GitLab resource."`
	Namespace string            `json:"namespace" doc:"Namespace the GitLab resource lives in."`
	Labels    map[string]string `json:"labels,omitempty" doc:"Labels applied to the GitLab resource."`
	Chart     ChartDTO          `json:"chart" doc:"GitLab Chart configuration."`
	Status    *StatusDTO        `json:"status,omitempty" readOnly:"true" doc:"Most recently observed status. Read-only."`
}

// ChartDTO carries the GitLab Chart version and Helm values.
type ChartDTO struct {
	Version string         `json:"version,omitempty" doc:"Semantic version of the GitLab Chart."`
	Values  map[string]any `json:"values,omitempty" doc:"Free-form Helm values used to render the GitLab Chart."`
}

// StatusDTO is the read-only observed state of a GitLab resource.
type StatusDTO struct {
	Phase      string             `json:"phase,omitempty" doc:"Current lifecycle phase."`
	Version    string             `json:"version,omitempty" doc:"Deployed GitLab version."`
	Conditions []metav1.Condition `json:"conditions,omitempty" doc:"Detailed status conditions."`
}

// toResource converts an internal GitLab object into its wire representation.
func toResource(gl *apiv1beta1.GitLab) GitLabResource {
	res := GitLabResource{
		Name:      gl.Name,
		Namespace: gl.Namespace,
		Labels:    gl.Labels,
		Chart: ChartDTO{
			Version: gl.Spec.Chart.Version,
			Values:  gl.Spec.Chart.Values.Object,
		},
	}

	res.Status = &StatusDTO{
		Phase:      gl.Status.Phase,
		Version:    gl.Status.Version,
		Conditions: gl.Status.Conditions,
	}

	return res
}

// applyToGitLab writes the client-supplied fields of a GitLabResource onto an
// internal GitLab object. Status is never applied because it is read-only.
func applyToGitLab(res GitLabResource, gl *apiv1beta1.GitLab) {
	gl.Name = res.Name
	gl.Namespace = res.Namespace

	if res.Labels != nil {
		gl.Labels = res.Labels
	}

	gl.Spec.Chart.Version = res.Chart.Version
	gl.Spec.Chart.Values.Object = res.Chart.Values
}
