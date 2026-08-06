package bridge

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
)

// newLocalTestServer starts an httptest server built with NewLocalAPI, backed by
// a fake client, and registered for automatic teardown. It mirrors what the
// `kubectl bridge` plugin does with a kubeconfig-derived client.
func newLocalTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv1beta1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	_, handler := NewLocalAPI(c)

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

// The local API declares no bearer security scheme, so the docs UI shows no
// Authorize button and the generated document does not advertise a token.
func TestLocalAPIHasNoBearerSecurityScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiv1beta1.AddToScheme(scheme))

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
