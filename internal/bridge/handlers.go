package bridge

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/mod/semver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	charts "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

const (
	tagGitLabs         = "gitlabs"
	tagCharts          = "charts"
	pathNamespaced     = "/api/v1/namespaces/{namespace}/gitlabs"
	pathNamespacedName = "/api/v1/namespaces/{namespace}/gitlabs/{name}"

	// gitlabChartName is the name of the GitLab Chart in the Operator's catalog.
	gitlabChartName = "gitlab"
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

// chartVersionsOutput wraps the list of bundled chart versions.
type chartVersionsOutput struct {
	Body ChartVersionsDTO
}

// RegisterRoutes registers the CRUD operations for the GitLabCore custom
// resource (`apps.gitlab.com/v2alpha1`) on the given Huma API. The paths keep
// the `gitlabs` segment: it names the GitLab instances the bridge manages, and
// `/api/v1` is the version of the bridge API, not of the custom resource.
//
// Each handler obtains its Kubernetes client from the request context
// (populated by authMiddleware from the caller's bearer token), so no client is
// captured here and the routes can be registered even when the API is built
// solely to emit the OpenAPI document.
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
		core := &apiv2alpha1.GitLabCore{}
		if err := clientFrom(ctx).Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, core); err != nil {
			return nil, mapError(err)
		}

		return &resourceBody{Body: toResource(core)}, nil
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

		core := &apiv2alpha1.GitLabCore{}
		applyToGitLabCore(in.Body, core)

		if err := clientFrom(ctx).Create(ctx, core); err != nil {
			return nil, mapError(err)
		}

		return &resourceBody{Body: toResource(core)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-gitlab",
		Method:      http.MethodPut,
		Path:        pathNamespacedName,
		Summary:     "Update a GitLab resource",
		Tags:        []string{tagGitLabs},
	}, func(ctx context.Context, in *updateInput) (*resourceBody, error) {
		c := clientFrom(ctx)

		core := &apiv2alpha1.GitLabCore{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, core); err != nil {
			return nil, mapError(err)
		}

		in.Body.Namespace = in.Namespace
		in.Body.Name = in.Name
		applyToGitLabCore(in.Body, core)

		if err := c.Update(ctx, core); err != nil {
			return nil, mapError(err)
		}

		return &resourceBody{Body: toResource(core)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-gitlab",
		Method:        http.MethodDelete,
		Path:          pathNamespacedName,
		Summary:       "Delete a GitLab resource",
		Tags:          []string{tagGitLabs},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *namespacedNamePath) (*struct{}, error) {
		core := &apiv2alpha1.GitLabCore{}
		core.Name = in.Name
		core.Namespace = in.Namespace

		if err := clientFrom(ctx).Delete(ctx, core); err != nil {
			return nil, mapError(err)
		}

		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-chart-versions",
		Method:      http.MethodGet,
		Path:        "/api/v1/chart-versions",
		Summary:     "List the GitLab chart versions available to install or upgrade to",
		Tags:        []string{tagCharts},
	}, func(_ context.Context, _ *struct{}) (*chartVersionsOutput, error) {
		// This needs no caller client: see listChartVersions.
		return &chartVersionsOutput{Body: ChartVersionsDTO{Versions: listChartVersions(slog.Default())}}, nil
	})
}

// listChartVersions lists the GitLab chart versions to offer in the version
// picker. It prefers the configured chart repository
// (settings.DynamicChartRepository) — the same source the GitLabCore
// controller itself falls back to for a version its bundled catalog does not
// carry (see internal/controller/gitlabcore/release.go) — over the bundled
// catalog, which only the in-cluster bridge has: the catalog is populated from
// HELM_CHARTS at manager startup (see cmd/manager/main.go), an image-only path
// the `kubectl bridge` plugin never runs, since it runs on the caller's own
// machine outside the Operator image. Reaching the repository directly lets the
// plugin offer the same versions without needing any chart on disk.
//
// The bundled catalog remains the fallback for a disconnected cluster that has
// turned dynamic chart pulling off (settings.DynamicChartPullEnabled), and for
// a repository lookup that fails for any other reason (e.g. no network from the
// plugin's machine); either way an empty result is a missing "upgrade to" list,
// not a request failure, so an error here is only logged.
//
// Either source yields versions newest-first, so the picker sees a stable
// ordering regardless of which one answered.
func listChartVersions(log *slog.Logger) []string {
	if settings.DynamicChartPullEnabled && settings.DynamicChartRepository != "" {
		versions, err := render.RemoteChartVersions(
			settings.DynamicChartRepository, gitlabChartName, settings.DynamicChartAllowHTTP, log)
		if err == nil {
			sortChartVersionsDescending(versions)

			return versions
		}

		log.Warn("could not list chart versions from the chart repository; falling back to the bundled catalog",
			"repository", settings.DynamicChartRepository, "error", err)
	}

	// charts.Catalog.Versions already returns its result newest-first.
	versions := charts.GlobalCatalog().Versions(gitlabChartName)
	if versions == nil {
		versions = []string{}
	}

	return versions
}

// sortChartVersionsDescending orders chart versions newest-first, mirroring the
// order charts.Catalog.Versions returns, so the version picker sees the same
// ordering whether the versions came from the chart repository or the bundled
// catalog.
func sortChartVersionsDescending(versions []string) {
	slices.SortFunc(versions, func(a, b string) int {
		if res := semver.Compare("v"+b, "v"+a); res != 0 {
			return res
		}

		return cmp.Compare(b, a)
	})
}

// listGitLabs lists GitLabCore resources, optionally scoped to a single
// namespace.
func listGitLabs(ctx context.Context, c client.Client, namespace string) (*listOutput, error) {
	list := &apiv2alpha1.GitLabCoreList{}

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
