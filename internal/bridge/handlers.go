package bridge

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
)

const (
	tagGitLabs         = "gitlabs"
	pathNamespaced     = "/api/v1/namespaces/{namespace}/gitlabs"
	pathNamespacedName = "/api/v1/namespaces/{namespace}/gitlabs/{name}"
)

// namespacePath and namePath describe the shared path parameters.
type namespacePath struct {
	Namespace string `path:"namespace" doc:"Namespace of the GitLab resource." example:"gitlab-system"`
}

type namespacedNamePath struct {
	Namespace string `path:"namespace" doc:"Namespace of the GitLab resource." example:"gitlab-system"`
	Name      string `path:"name" doc:"Name of the GitLab resource." example:"gitlab"`
}

// createInput is the request for creating a GitLab resource.
type createInput struct {
	Namespace string `path:"namespace" doc:"Namespace of the GitLab resource." example:"gitlab-system"`
	Body      GitLabResource
}

// updateInput is the request for updating a GitLab resource.
type updateInput struct {
	Namespace string `path:"namespace" doc:"Namespace of the GitLab resource." example:"gitlab-system"`
	Name      string `path:"name" doc:"Name of the GitLab resource." example:"gitlab"`
	Body      GitLabResource
}

// GitLabList is the response body for list operations.
type GitLabList struct {
	Items []GitLabResource `json:"items" doc:"The GitLab resources."`
}

// resourceBody wraps a single GitLabResource as a request or response body.
type resourceBody struct {
	Body GitLabResource
}

type listOutput struct {
	Body GitLabList
}

// RegisterRoutes registers the CRUD operations for the GitLab custom resource on
// the given Huma API. Each handler obtains its Kubernetes client from the request
// context (populated by authMiddleware from the caller's bearer token), so no
// client is captured here and the routes can be registered even when the API is
// built solely to emit the OpenAPI document.
func RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-gitlabs-all-namespaces",
		Method:      http.MethodGet,
		Path:        "/api/v1/gitlabs",
		Summary:     "List GitLab resources across all watched namespaces",
		Tags:        []string{tagGitLabs},
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		return listGitLabs(ctx, clientFrom(ctx), "")
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-gitlabs",
		Method:      http.MethodGet,
		Path:        pathNamespaced,
		Summary:     "List GitLab resources in a namespace",
		Tags:        []string{tagGitLabs},
	}, func(ctx context.Context, in *namespacePath) (*listOutput, error) {
		return listGitLabs(ctx, clientFrom(ctx), in.Namespace)
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-gitlab",
		Method:      http.MethodGet,
		Path:        pathNamespacedName,
		Summary:     "Get a GitLab resource",
		Tags:        []string{tagGitLabs},
	}, func(ctx context.Context, in *namespacedNamePath) (*resourceBody, error) {
		gl := &apiv1beta1.GitLab{}
		if err := clientFrom(ctx).Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, gl); err != nil {
			return nil, mapError(err)
		}

		return &resourceBody{Body: toResource(gl)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-gitlab",
		Method:        http.MethodPost,
		Path:          pathNamespaced,
		Summary:       "Create a GitLab resource",
		Tags:          []string{tagGitLabs},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createInput) (*resourceBody, error) {
		in.Body.Namespace = in.Namespace

		gl := &apiv1beta1.GitLab{}
		applyToGitLab(in.Body, gl)

		if err := clientFrom(ctx).Create(ctx, gl); err != nil {
			return nil, mapError(err)
		}

		return &resourceBody{Body: toResource(gl)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-gitlab",
		Method:      http.MethodPut,
		Path:        pathNamespacedName,
		Summary:     "Update a GitLab resource",
		Tags:        []string{tagGitLabs},
	}, func(ctx context.Context, in *updateInput) (*resourceBody, error) {
		c := clientFrom(ctx)

		gl := &apiv1beta1.GitLab{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, gl); err != nil {
			return nil, mapError(err)
		}

		in.Body.Namespace = in.Namespace
		in.Body.Name = in.Name
		applyToGitLab(in.Body, gl)

		if err := c.Update(ctx, gl); err != nil {
			return nil, mapError(err)
		}

		return &resourceBody{Body: toResource(gl)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-gitlab",
		Method:        http.MethodDelete,
		Path:          pathNamespacedName,
		Summary:       "Delete a GitLab resource",
		Tags:          []string{tagGitLabs},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *namespacedNamePath) (*struct{}, error) {
		gl := &apiv1beta1.GitLab{}
		gl.Name = in.Name
		gl.Namespace = in.Namespace

		if err := clientFrom(ctx).Delete(ctx, gl); err != nil {
			return nil, mapError(err)
		}

		return nil, nil
	})
}

// listGitLabs lists GitLab resources, optionally scoped to a single namespace.
func listGitLabs(ctx context.Context, c client.Client, namespace string) (*listOutput, error) {
	list := &apiv1beta1.GitLabList{}

	var opts []client.ListOption
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := c.List(ctx, list, opts...); err != nil {
		return nil, mapError(err)
	}

	out := &listOutput{Body: GitLabList{Items: make([]GitLabResource, 0, len(list.Items))}}
	for i := range list.Items {
		out.Body.Items = append(out.Body.Items, toResource(&list.Items[i]))
	}

	return out, nil
}

// mapError translates Kubernetes API errors into Huma HTTP errors.
func mapError(err error) error {
	switch {
	case apierrors.IsUnauthorized(err):
		return huma.Error401Unauthorized("unauthorized", err)
	case apierrors.IsForbidden(err):
		return huma.Error403Forbidden("forbidden", err)
	case apierrors.IsNotFound(err):
		return huma.Error404NotFound("GitLab resource not found", err)
	case apierrors.IsAlreadyExists(err):
		return huma.Error409Conflict("GitLab resource already exists", err)
	case apierrors.IsConflict(err):
		return huma.Error409Conflict("conflict updating GitLab resource", err)
	case apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		return huma.Error422UnprocessableEntity("invalid GitLab resource", err)
	default:
		return huma.Error500InternalServerError("failed to process GitLab resource", err)
	}
}
