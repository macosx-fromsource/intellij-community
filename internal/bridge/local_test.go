package bridge

import (
	"cmp"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// newLocalTestServer starts an httptest server built with NewLocalAPI, backed by
// a fake client, and registered for automatic teardown. It mirrors what the
// `kubectl bridge` plugin does with a kubeconfig-derived client.
func newLocalTestServer(t *testing.T, acceptHosts ...string) *httptest.Server {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv2alpha1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	_, handler := NewLocalAPI(c, acceptHosts...)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// getLocal performs an unauthenticated GET against the local-mode test server.
func getLocal(t *testing.T, server *httptest.Server, path string) *http.Response {
	t.Helper()

	resp, err := server.Client().Get(server.URL + path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// In local serve mode the caller's kubeconfig already authenticated the client,
// so API requests must succeed without an Authorization header. The in-cluster
// path rejects the same request with 401.
func TestLocalAPIServesWithoutBearerToken(t *testing.T) {
	server := newLocalTestServer(t)

	resp := getLocal(t, server, "/api/v1/gitlabs")
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// A bogus token must not change anything in local mode: the fixed client is used
// regardless of what the caller sends.
func TestLocalAPIIgnoresBearerToken(t *testing.T) {
	server := newLocalTestServer(t)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/gitlabs", nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer not-a-real-token")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)

	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// doLocalWithHeaders performs a request against the local-mode test server with
// the given headers, mimicking what a browser sends. A "Host" entry sets the
// request's host rather than a plain header, which is how a DNS rebinding attack
// reaches a loopback server.
func doLocalWithHeaders(t *testing.T, server *httptest.Server, method, path string, headers map[string]string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, server.URL+path, nil)
	require.NoError(t, err)

	for name, value := range headers {
		if name == "Host" {
			req.Host = value

			continue
		}

		req.Header.Set(name, value)
	}

	resp, err := server.Client().Do(req)
	require.NoError(t, err)

	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// Local mode has no credential to check, so a page on any site the caller has
// open could otherwise drive the API with their kubeconfig permissions. Every
// shape a browser gives such a request away by must be refused, while the
// bridge's own UI, static routes and non-browser clients must not be. A 403 on
// the DELETE row also shows the guard runs before the handler: reaching the
// cluster would have answered 404 for a GitLab that does not exist.
func TestLocalGuard(t *testing.T) {
	server := newLocalTestServer(t)
	del := "/api/v1/namespaces/gitlab-system/gitlabs/gitlab"

	for name, tc := range map[string]struct {
		method  string
		path    string
		headers map[string]string
		want    int
	}{
		"cross-site fetch metadata": {"", "", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		"same-site fetch metadata":  {"", "", map[string]string{"Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		"foreign origin":            {"", "", map[string]string{"Origin": "http://evil.example"}, http.StatusForbidden},
		"opaque origin":             {"", "", map[string]string{"Origin": "null"}, http.StatusForbidden},
		"foreign referer":           {"", "", map[string]string{"Referer": "http://evil.example/p"}, http.StatusForbidden},
		"cross-origin write":        {http.MethodDelete, del, map[string]string{"Origin": "http://evil.example"}, http.StatusForbidden},
		// DNS rebinding points a name the attacker controls at loopback, so their
		// page is same-origin with the bridge and only the Host gives it away.
		"rebound host": {"", "", map[string]string{
			"Host": "rebind.evil.example", "Sec-Fetch-Site": "same-origin", "Origin": "http://rebind.evil.example",
		}, http.StatusForbidden},
		"SPA fetch": {"", "", map[string]string{
			"Sec-Fetch-Site": "same-origin", "Origin": server.URL, "Referer": server.URL + "/gitlabs",
		}, http.StatusOK},
		"user navigation": {"", "", map[string]string{"Sec-Fetch-Site": "none"}, http.StatusOK},
		"docs UI":         {"", "", map[string]string{"Referer": server.URL + "/docs"}, http.StatusOK},
		// Static assets carry no cluster permissions, so they stay reachable.
		"SPA document":     {"", "/", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
		"OpenAPI document": {"", "/openapi.yaml", map[string]string{"Origin": "http://evil.example"}, http.StatusOK},
	} {
		t.Run(name, func(t *testing.T) {
			method, path := cmp.Or(tc.method, http.MethodGet), cmp.Or(tc.path, "/api/v1/gitlabs")

			resp := doLocalWithHeaders(t, server, method, path, tc.headers)
			require.Equal(t, tc.want, resp.StatusCode)
		})
	}
}

// Reaching the bridge under another name is legitimate when asked for
// explicitly, e.g. bound to a routable address for a colleague or a container.
func TestLocalAPIAcceptsConfiguredHost(t *testing.T) {
	server := newLocalTestServer(t, "bridge.internal")

	resp := doLocalWithHeaders(t, server, http.MethodGet, "/api/v1/gitlabs", map[string]string{
		"Host":           "bridge.internal",
		"Sec-Fetch-Site": "same-origin",
		"Origin":         "http://bridge.internal",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// Only literals count as loopback: resolving a name here is exactly the hole the
// Host check closes, since an attacker's name can point at 127.0.0.1.
func TestIsLoopbackAuthority(t *testing.T) {
	for _, tc := range []struct {
		authority string
		want      bool
	}{
		{"127.0.0.1:8090", true},
		{"127.0.0.1", true},
		{"[::1]:8090", true},
		{"::1", true},
		{"localhost:8090", true},
		{"LocalHost", true},
		{"192.168.1.5:8090", false},
		{"bridge.internal:8090", false},
		{"localhost.evil.example", false},
		{"", false},
	} {
		t.Run(tc.authority, func(t *testing.T) {
			require.Equal(t, tc.want, isLoopbackAuthority(tc.authority))
		})
	}
}

// The local API declares no bearer security scheme, so the docs UI shows no
// Authorize button and the generated document does not advertise a token.
func TestLocalAPIHasNoBearerSecurityScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2alpha1.AddToScheme(scheme))

	api, _ := NewLocalAPI(fake.NewClientBuilder().WithScheme(scheme).Build())

	require.Empty(t, api.OpenAPI().Components.SecuritySchemes)
	require.Empty(t, api.OpenAPI().Security)
}

// The SPA detects local mode from a marker injected into index.html, so the
// token-entry UI can be hidden. It must be present only in local mode.
func TestIndexDocumentInjectsLocalAuthMarker(t *testing.T) {
	dist := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><head><title>x</title></head><body></body></html>")},
	}

	local := string(indexDocument(dist, true))
	require.Contains(t, local, localAuthMarker)
	require.Less(t, strings.Index(local, localAuthMarker), strings.Index(local, "</head>"))

	inCluster := string(indexDocument(dist, false))
	require.NotContains(t, inCluster, localAuthMarker)
}

// Without a </head> to anchor on, the marker still has to reach the document, or
// the SPA would fall back to asking for a token it does not need.
func TestIndexDocumentInjectsMarkerWithoutHead(t *testing.T) {
	dist := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<body></body>")}}

	require.Contains(t, string(indexDocument(dist, true)), localAuthMarker)
}

// A missing index.html (SPA not built) must not panic; it degrades to a
// placeholder page so the API stays usable.
func TestIndexDocumentWithoutBuiltSPA(t *testing.T) {
	require.Contains(t, string(indexDocument(fstest.MapFS{}, true)), "SPA not built")
}

// Client-side routes are unknown to the embedded file system and must fall back
// to the index document rather than 404, so deep links into the SPA work. The
// marker injection itself is covered by the indexDocument tests, which do not
// depend on whether web/dist was built in this checkout.
func TestLocalStaticFallbackServesIndex(t *testing.T) {
	server := newLocalTestServer(t)

	resp := getLocal(t, server, "/gitlabs/new")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotEmpty(t, body)
}
