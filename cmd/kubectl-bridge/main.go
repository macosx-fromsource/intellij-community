// Command kubectl-bridge is a kubectl plugin (`kubectl bridge`) that runs the
// GitLab Operator bridge UI locally, authenticated as the caller's own kubectl
// identity.
//
// Unlike the in-cluster bridge — which authenticates each request with a bearer
// token and so cannot represent a client-cert kubeconfig — this builds its
// Kubernetes client from the ambient kubeconfig via client-go. Authentication is
// therefore whatever the kubeconfig uses (client certificate, exec/OIDC, or
// token), identical to how kubectl itself connects, with no token to paste and
// no ServiceAccount to mint. The server does no request authentication of its
// own — it is as privileged as the kubeconfig on the machine — so it defaults to
// a loopback address and warns when bound anywhere else.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth" // register in-tree auth providers (oidc/gcp/azure)
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubectlscheme "k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/bridge"
)

const (
	defaultPort    = 8090
	defaultAddress = "127.0.0.1"
	shutdownGrace  = 5 * time.Second
	// localhostName is the host used in the printed URL when the bound address
	// is not one the browser should be pointed at literally.
	localhostName = "localhost"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kubectl-bridge:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		port        int
		address     string
		kubeContext string
		kubeconfig  string
		noOpen      bool
		verbose     bool
	)

	// A dedicated FlagSet avoids colliding with flags (e.g. --kubeconfig)
	// registered on the global flag.CommandLine by transitively-imported
	// Kubernetes libraries.
	fs := flag.NewFlagSet("kubectl-bridge", flag.ExitOnError)
	fs.IntVar(&port, "port", defaultPort, "local port to bind (falls back to a random free port if busy)")
	fs.IntVar(&port, "p", defaultPort, "shorthand for --port")
	fs.StringVar(&address, "address", defaultAddress, "address to bind; a non-loopback address is allowed but warned about")
	fs.StringVar(&kubeContext, "context", "", "kubeconfig context to use (default: current context)")
	fs.StringVar(&kubeconfig, "kubeconfig", "", "path to the kubeconfig file (default: standard loading rules)")
	fs.BoolVar(&noOpen, "no-open", false, "do not open a browser; just print the URL")
	fs.BoolVar(&verbose, "verbose", false, "debug logging (server startup details and every API request)")
	fs.BoolVar(&verbose, "v", false, "shorthand for --verbose")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	c, who, host, err := buildClient(kubeconfig, kubeContext)
	if err != nil {
		return err
	}

	log.Debug("built Kubernetes client from kubeconfig", "identity", who, "apiServer", host)

	warnIfNotLoopback(address)

	_, handler := bridge.NewLocalAPI(c)

	ln, url, err := listen(address, port)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           logRequests(log, handler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()

		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("GitLab Operator bridge UI (acting as %s)\n\n  %s\n\n", who, url)

	if noOpen {
		fmt.Println("Open the URL above in your browser. Press Ctrl-C to stop.")
	} else {
		openBrowser(url)
		fmt.Println("Opening in your browser. Press Ctrl-C to stop.")
	}

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// buildClient loads the ambient kubeconfig (honoring an explicit path and
// context override), builds a Kubernetes client from it, and returns the client
// plus a human-readable description of the identity and the API server address,
// both for logging. All kubeconfig auth methods work because client-go builds
// the transport itself.
func buildClient(kubeconfig, kubeContext string) (client.Client, string, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", "", fmt.Errorf("loading kubeconfig: %w", err)
	}

	scheme := kubectlscheme.Scheme
	if err := appsv1beta1.AddToScheme(scheme); err != nil {
		return nil, "", "", fmt.Errorf("registering v1beta1 scheme: %w", err)
	}

	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, "", "", fmt.Errorf("building HTTP client: %w", err)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(cfg, httpClient)
	if err != nil {
		return nil, "", "", fmt.Errorf("building REST mapper (is the cluster reachable?): %w", err)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, "", "", fmt.Errorf("building client: %w", err)
	}

	return c, describeIdentity(clientConfig, cfg), cfg.Host, nil
}

// describeIdentity returns a short, best-effort label for who the client acts
// as, for the startup banner. It never fails the command.
func describeIdentity(clientConfig clientcmd.ClientConfig, cfg *rest.Config) string {
	if raw, err := clientConfig.RawConfig(); err == nil && raw.CurrentContext != "" {
		if ctx, ok := raw.Contexts[raw.CurrentContext]; ok && ctx.AuthInfo != "" {
			return fmt.Sprintf("kubeconfig user %q (context %q)", ctx.AuthInfo, raw.CurrentContext)
		}

		return fmt.Sprintf("context %q", raw.CurrentContext)
	}

	return fmt.Sprintf("kubeconfig identity at %s", cfg.Host)
}

// logRequests logs every served request at debug level, so that --verbose shows
// the API calls the SPA makes. At the default level it adds nothing.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		log.Debug("request served", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

// listen binds address:port, falling back to a random free port when the
// requested one is busy. It returns the listener and the browser URL.
// net.JoinHostPort is used throughout so that IPv6 literals such as ::1 are
// bracketed and parse correctly.
func listen(address string, port int) (net.Listener, string, error) {
	addr := net.JoinHostPort(address, strconv.Itoa(port))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot bind %s (%v); trying a free port\n", addr, err)

		ln, err = net.Listen("tcp", net.JoinHostPort(address, "0"))
		if err != nil {
			return nil, "", fmt.Errorf("binding %s: %w", address, err)
		}
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port

	return ln, "http://" + net.JoinHostPort(browserHost(address), strconv.Itoa(actualPort)), nil
}

// browserHost picks the host to put in the printed URL. An unspecified address
// (0.0.0.0, ::, or empty) listens everywhere, and IPv4 loopback is more familiar
// as a name, so both become "localhost". Any other address is used verbatim, so
// that an IPv6 literal or a specific interface links to where the server really
// is rather than to a name that may resolve elsewhere.
func browserHost(address string) string {
	ip := net.ParseIP(address)
	if address == "" || (ip != nil && (ip.IsUnspecified() || ip.Equal(net.IPv4(127, 0, 0, 1)))) {
		return localhostName
	}

	return address
}

// warnIfNotLoopback prints a warning when the server will be reachable from
// beyond this machine. Binding elsewhere is allowed on purpose (running the
// plugin in a container or on a remote host is legitimate), but the server
// authenticates nothing, so the caller should know what they are exposing.
func warnIfNotLoopback(address string) {
	if isLoopback(address) {
		return
	}

	fmt.Fprintf(os.Stderr,
		"warning: binding %s, which is not loopback. This server performs no authentication:\n"+
			"         anyone who can reach the port acts with your kubeconfig permissions.\n",
		address)
}

// isLoopback reports whether every address the server may bind is a loopback
// address. An empty host binds every interface, so it is not loopback. Names are
// resolved, which keeps "localhost" from warning.
func isLoopback(address string) bool {
	if address == "" {
		return false
	}

	if ip := net.ParseIP(address); ip != nil {
		return ip.IsLoopback()
	}

	ips, err := net.LookupIP(address)
	if err != nil || len(ips) == 0 {
		return false
	}

	for _, ip := range ips {
		if !ip.IsLoopback() {
			return false
		}
	}

	return true
}

// openBrowser best-effort opens url in the default browser; failures are
// non-fatal (the URL is always printed too).
func openBrowser(url string) {
	var name string

	var args []string

	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "cmd", []string{"/c", "start", url}
	default:
		name, args = "xdg-open", []string{url}
	}

	if _, err := exec.LookPath(name); err != nil {
		return
	}

	// The command is a fixed per-OS opener and the only argument is a
	// http://localhost:<port> URL we construct ourselves — no user input.
	_ = exec.Command(name, args...).Start() //nolint:gosec // G204: fixed command + self-constructed localhost URL
}
