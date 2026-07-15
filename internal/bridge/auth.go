package bridge

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const bearerPrefix = "Bearer "

// ClientFactory builds a Kubernetes client that acts as the caller identified by
// the given bearer token. Every request to the bridge uses a client built this
// way so that authentication and authorization are delegated to the Kubernetes
// API server (the caller's own RBAC applies), rather than reusing the operator's
// privileged service account.
type ClientFactory func(token string) (client.Client, error)

// newClientFactory returns a ClientFactory derived from the operator's base
// rest.Config. The API server address and CA trust are kept, but the operator's
// credentials are stripped and replaced per request with the caller's token. A
// single RESTMapper is built up front and shared across the per-request clients
// so they do not each re-run API discovery.
func newClientFactory(base *rest.Config, scheme *runtime.Scheme) (ClientFactory, error) {
	httpClient, err := rest.HTTPClientFor(base)
	if err != nil {
		return nil, fmt.Errorf("building HTTP client for REST mapper: %w", err)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(base, httpClient)
	if err != nil {
		return nil, fmt.Errorf("building REST mapper: %w", err)
	}

	return func(token string) (client.Client, error) {
		cfg := rest.AnonymousClientConfig(base)
		cfg.BearerToken = token

		return client.New(cfg, client.Options{Scheme: scheme, Mapper: mapper})
	}, nil
}

// clientCtxKey is the context key under which the per-request client is stored.
type clientCtxKey struct{}

// clientFrom returns the per-request client stashed in the context by
// authMiddleware. It panics if no client is present, which can only happen if a
// handler is reached without the middleware having run — a programming error.
func clientFrom(ctx context.Context) client.Client {
	c, ok := ctx.Value(clientCtxKey{}).(client.Client)
	if !ok {
		panic("bridge: no client in context; auth middleware did not run")
	}

	return c
}

// authMiddleware enforces bearer-token authentication for API routes and builds
// a per-request, caller-scoped client. Non-API routes (the OpenAPI document, the
// docs UI, and the SPA) are left untouched so they remain reachable without a
// token.
func authMiddleware(api huma.API, cf ClientFactory) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !strings.HasPrefix(ctx.URL().Path, "/api/") {
			next(ctx)

			return
		}

		authz := ctx.Header("Authorization")
		if !strings.HasPrefix(authz, bearerPrefix) {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized,
				"missing or malformed Authorization header; expected 'Bearer <token>'")

			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authz, bearerPrefix))
		if token == "" {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "empty bearer token")

			return
		}

		cl, err := cf(token)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "failed to build client for token", err)

			return
		}

		next(huma.WithValue(ctx, clientCtxKey{}, cl))
	}
}
