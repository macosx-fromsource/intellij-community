package bridge

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

// localClientMiddleware injects a single, fixed client for API routes instead of
// building a per-request client from a bearer token. It is used by the local
// serve mode (the `kubectl bridge` plugin), where the client is built from the
// caller's own kubeconfig — so the caller's kubectl identity (client cert,
// exec/OIDC, or token, whatever their kubeconfig uses) authenticates to the API
// server directly, with no bearer token to pass. Non-API routes pass through
// untouched, exactly like authMiddleware.
//
// Because there is no token to know, any client that can reach the port is
// served with the caller's full cluster permissions — including a web page open
// in the caller's browser. localGuardMiddleware runs first to keep such a page
// out; do not use this middleware without it.
func localClientMiddleware(c client.Client) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !strings.HasPrefix(ctx.URL().Path, "/api/") {
			next(ctx)

			return
		}

		next(huma.WithValue(ctx, clientCtxKey{}, c))
	}
}

// localhostName is the only host name (as opposed to IP literal) that counts as
// loopback without being resolved.
const localhostName = "localhost"

// localGuardMiddleware protects local serve mode against the caller's browser
// being used as a confused deputy. Requests there carry no credential of their
// own, so a page on any site the caller happens to have open could otherwise
// fetch http://127.0.0.1:<port>/api/... and act with the caller's kubeconfig
// permissions — binding to loopback keeps other machines out, but not other tabs.
// Two checks, both on /api/ routes only:
//
//   - The Host header must name this machine over loopback (or be listed in
//     acceptHosts). This closes DNS rebinding, where a name the attacker controls
//     resolves to 127.0.0.1 so that their page is same-origin with the bridge.
//   - The request must not look like a browser fetch made on behalf of another
//     origin (Sec-Fetch-Site, Origin, Referer).
//
// This is the mitigation `kubectl proxy --accept-hosts` provides for the same
// class of issue (CVE-2020-8558). Non-browser clients such as curl send none of
// the fetch metadata headers and are unaffected.
func localGuardMiddleware(api huma.API, acceptHosts []string) func(huma.Context, func(huma.Context)) {
	accepted := make(map[string]struct{}, len(acceptHosts))

	for _, h := range acceptHosts {
		if host := strings.ToLower(hostOnly(strings.TrimSpace(h))); host != "" {
			accepted[host] = struct{}{}
		}
	}

	hostAllowed := func(authority string) bool {
		if isLoopbackAuthority(authority) {
			return true
		}

		_, ok := accepted[strings.ToLower(hostOnly(authority))]

		return ok
	}

	return func(ctx huma.Context, next func(huma.Context)) {
		if !strings.HasPrefix(ctx.URL().Path, "/api/") {
			next(ctx)

			return
		}

		if !hostAllowed(ctx.Host()) {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, fmt.Sprintf(
				"host %q is not accepted; the local bridge only answers to loopback names (use --accept-hosts to allow others)",
				ctx.Host()))

			return
		}

		if reason := crossOriginReason(ctx); reason != "" {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden,
				"request rejected as cross-origin: "+reason)

			return
		}

		next(ctx)
	}
}

// crossOriginReason returns a non-empty reason when the request looks like a
// browser fetch issued for a document that is not the bridge's own UI. A request
// carrying none of these headers is let through: that is a non-browser client
// (curl, the SPA's own same-origin GETs predate none of them), and a page cannot
// suppress the headers browsers do send.
func crossOriginReason(ctx huma.Context) string {
	if site := ctx.Header("Sec-Fetch-Site"); site != "" &&
		!strings.EqualFold(site, "same-origin") && !strings.EqualFold(site, "none") {
		return fmt.Sprintf("Sec-Fetch-Site is %q", site)
	}

	if origin := ctx.Header("Origin"); origin != "" {
		if !sameAuthority(origin, ctx.Host()) {
			return fmt.Sprintf("Origin %q does not match host %q", origin, ctx.Host())
		}

		return ""
	}

	if referer := ctx.Header("Referer"); referer != "" && !sameAuthority(referer, ctx.Host()) {
		return fmt.Sprintf("Referer %q does not match host %q", referer, ctx.Host())
	}

	return ""
}

// sameAuthority reports whether the URL in a browser-set Origin or Referer
// header has the same authority as the request's own Host header. "Origin: null"
// — a sandboxed document or a file:// page — has no authority and so never
// matches.
func sameAuthority(rawURL, host string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}

	return strings.EqualFold(u.Host, host)
}

// isLoopbackAuthority reports whether authority (a Host header or the authority
// of an Origin, with or without a port) names this machine over loopback. Only
// IP literals and the reserved name "localhost" qualify: resolving any other
// name would reinstate the DNS rebinding hole the check exists to close.
func isLoopbackAuthority(authority string) bool {
	host := hostOnly(authority)

	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}

	return strings.EqualFold(host, localhostName)
}

// hostOnly strips the port and any IPv6 brackets from an authority such as
// "127.0.0.1:8090", "[::1]:8090" or "localhost".
func hostOnly(authority string) string {
	if host, _, err := net.SplitHostPort(authority); err == nil {
		return host
	}

	return strings.Trim(authority, "[]")
}
