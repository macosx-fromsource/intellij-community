package bridge

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

const (
	tagSiphons = "siphons"
	pathSiphon = pathNamespacedName + "/siphon"

	siphonKind = "Siphon resource"

	// siphonNameSuffix names the Siphon the bridge creates after the instance
	// it streams, and maxSiphonNameLength is what the custom resource
	// definition allows: the Operator derives workload names from it, and the
	// longest of those has to fit a 63 character label value.
	siphonNameSuffix    = "-siphon"
	maxSiphonNameLength = 31
)

// siphonInput is the request for creating or replacing the Siphon of a GitLab
// instance.
type siphonInput struct {
	NamespacedNamePath
	Body SiphonResource
}

// siphonBody wraps a single SiphonResource as a response body.
type siphonBody struct {
	Body SiphonResource
}

// registerSiphonRoutes registers the operations for the Siphon custom resource
// (`apps.gitlab.com/v2alpha1`). Siphon is an add-on of a GitLab instance rather
// than a resource of its own in the user interface, so the routes nest under
// the instance and address the one Siphon that streams it, whatever that
// Siphon is named.
func registerSiphonRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-siphon",
		Method:      http.MethodGet,
		Path:        pathSiphon,
		Summary:     "Get the Siphon of a GitLab instance",
		Tags:        []string{tagSiphons},
	}, func(ctx context.Context, in *NamespacedNamePath) (*siphonBody, error) {
		siphon, err := findSiphon(ctx, clientFrom(ctx), in.Namespace, in.Name)
		if err != nil {
			return nil, mapResourceError(err, siphonKind)
		}

		return &siphonBody{Body: toSiphonResource(siphon)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "put-siphon",
		Method:      http.MethodPut,
		Path:        pathSiphon,
		Summary:     "Create or replace the Siphon of a GitLab instance",
		Tags:        []string{tagSiphons},
	}, putSiphon)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-siphon",
		Method:        http.MethodDelete,
		Path:          pathSiphon,
		Summary:       "Delete the Siphon of a GitLab instance",
		Tags:          []string{tagSiphons},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *NamespacedNamePath) (*struct{}, error) {
		c := clientFrom(ctx)

		siphon, err := findSiphon(ctx, c, in.Namespace, in.Name)
		if err != nil {
			return nil, mapResourceError(err, siphonKind)
		}

		if err := c.Delete(ctx, siphon); err != nil {
			return nil, mapResourceError(err, siphonKind)
		}

		return nil, nil
	})
}

// putSiphon replaces the Siphon of the instance, creating it when the instance
// has none. An enabled add-on is saved the same way whether or not it was on
// before, so the operation is an upsert rather than a create and an update.
func putSiphon(ctx context.Context, in *siphonInput) (*siphonBody, error) {
	c := clientFrom(ctx)

	siphon, err := findSiphon(ctx, c, in.Namespace, in.Name)

	switch {
	case err == nil:
		applyToSiphon(in.Body, in.Name, siphon)

		if err := c.Update(ctx, siphon); err != nil {
			return nil, mapResourceError(err, siphonKind)
		}
	case apierrors.IsNotFound(err):
		name, nameErr := siphonNameFor(in.Name)
		if nameErr != nil {
			return nil, huma.Error422UnprocessableEntity(nameErr.Error())
		}

		siphon = &apiv2alpha1.Siphon{}
		siphon.Name = name
		siphon.Namespace = in.Namespace

		applyToSiphon(in.Body, in.Name, siphon)

		if err := c.Create(ctx, siphon); err != nil {
			return nil, mapResourceError(err, siphonKind)
		}
	default:
		return nil, mapResourceError(err, siphonKind)
	}

	return &siphonBody{Body: toSiphonResource(siphon)}, nil
}

// findSiphon returns the Siphon in the namespace that references the named
// GitLab instance, or a NotFound error when no Siphon does. The link is
// `spec.gitlabRef`, not the name, so a Siphon created by hand under any name is
// the one the bridge manages from then on. Two Siphons streaming one instance
// are not a topology the Operator deploys; the first by name wins.
//
// A cluster without the Siphon definition has no Siphon either, so the lookup
// that cannot resolve the kind reports the same absence rather than an error:
// the definition reaches no installation yet (ADR 27), and every instance would
// otherwise open with a failure. Creating one there still fails, and says why.
func findSiphon(ctx context.Context, c client.Client, namespace, gitlab string) (*apiv2alpha1.Siphon, error) {
	list := &apiv2alpha1.SiphonList{}
	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil && !apimeta.IsNoMatchError(err) {
		return nil, err
	}

	for i := range list.Items {
		if list.Items[i].Spec.GitLabRef.Name == gitlab {
			return &list.Items[i], nil
		}
	}

	resource := schema.GroupResource{Group: apiv2alpha1.GroupVersion.Group, Resource: "siphons"}

	return nil, apierrors.NewNotFound(resource, gitlab+siphonNameSuffix)
}

// siphonNameFor names the Siphon of an instance after that instance. A name
// the custom resource definition would reject is reported here rather than by
// the API server, because the caller never chose it.
func siphonNameFor(gitlab string) (string, error) {
	name := gitlab + siphonNameSuffix
	if len(name) > maxSiphonNameLength {
		return "", fmt.Errorf(
			"a Siphon for %q would be named %q, which is longer than the %d characters a Siphon name allows; "+
				"create it with kubectl under a shorter name, and it is managed here from then on",
			gitlab, name, maxSiphonNameLength)
	}

	return name, nil
}
