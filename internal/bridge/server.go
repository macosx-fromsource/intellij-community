package bridge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	apiTitle            = "GitLab Operator Bridge"
	apiVersion          = "0.1.0"
	shutdownGracePeriod = 5 * time.Second
	bearerAuthScheme    = "bearer"
)

// Server is the backend-for-frontend HTTP server. It implements
// sigs.k8s.io/controller-runtime/pkg/manager.Runnable so it can be registered
// on the manager via mgr.Add.
type Server struct {
	// RESTConfig is the operator's base Kubernetes REST config. Per-request
	// clients are derived from it with the caller's bearer token, so the bridge
	// never lends out the operator's own credentials.
	RESTConfig *rest.Config
	// Scheme is the runtime scheme used to build per-request clients.
	Scheme *runtime.Scheme
	// Addr is the address the server binds to, e.g. ":8090".
	Addr string
	// Log is the logger for the server. When nil, slog.Default() is used.
	Log *slog.Logger
}

// logger returns the configured logger, falling back to slog.Default().
func (s *Server) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}

	return slog.Default()
}

// NewAPI builds the Huma API and returns it alongside the HTTP handler serving
// the CRUD endpoints, the OpenAPI document, the docs UI, and the SPA. Requests to
// the CRUD endpoints are authenticated by authMiddleware, which builds a
// per-request client via cf from the caller's bearer token. cf may be nil when
// the API is built only to emit the OpenAPI document (the middleware never runs).
func NewAPI(cf ClientFactory) (huma.API, http.Handler) {
	mux := http.NewServeMux()

	config := huma.DefaultConfig(apiTitle, apiVersion)
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		bearerAuthScheme: {
			Type:        "http",
			Scheme:      "bearer",
			Description: "Kubernetes bearer token. The API acts as the caller identified by this token, subject to the caller's own RBAC.",
		},
	}
	config.Security = []map[string][]string{{bearerAuthScheme: {}}}

	api := humago.New(mux, config)

	api.UseMiddleware(authMiddleware(api, cf))
	RegisterRoutes(api)
	registerStatic(mux, false)

	return api, mux
}

// NewLocalAPI builds the same HTTP handler as NewAPI but for local serve mode:
// instead of authenticating each request with a bearer token, it injects a
// single fixed client (built from the caller's own kubeconfig) for every API
// request. This backs the `kubectl bridge` plugin, which runs the bridge on the
// caller's machine so their kubectl identity — including a client-cert or
// exec/OIDC kubeconfig that has no bearer token — authenticates to the API
// server directly. The API therefore declares no bearer security scheme, and it
// performs no request authentication of its own, so the caller is responsible
// for where the handler is exposed — normally a loopback address.
func NewLocalAPI(c client.Client) (huma.API, http.Handler) {
	mux := http.NewServeMux()

	api := humago.New(mux, huma.DefaultConfig(apiTitle, apiVersion))

	api.UseMiddleware(localClientMiddleware(c))
	RegisterRoutes(api)
	registerStatic(mux, true)

	return api, mux
}

// Start runs the HTTP server until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	log := s.logger()

	cf, err := newClientFactory(s.RESTConfig, s.Scheme)
	if err != nil {
		return fmt.Errorf("building bridge client factory: %w", err)
	}

	_, handler := NewAPI(cf)

	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	// Graceful shutdown needs a context that survives ctx being cancelled (the
	// manager cancels ctx to trigger shutdown). WithoutCancel keeps ctx's values
	// while detaching it from the parent's cancellation.
	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGracePeriod)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("failed to gracefully shut down bridge server", "error", err)
		}
	}()

	log.Info("starting bridge server", "addr", s.Addr)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// NeedLeaderElection returns false so the bridge server runs on every replica
// rather than only on the elected leader.
func (s *Server) NeedLeaderElection() bool {
	return false
}
