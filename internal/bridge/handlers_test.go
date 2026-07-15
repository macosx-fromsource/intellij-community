package bridge

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
)

// testToken is the bearer token attached to requests by doRequest. The test
// ClientFactory ignores its value and returns the fake client, so any non-empty
// token authenticates.
const testToken = "test-token"

// newTestServer starts an httptest server backed by a fake client, registered
// for automatic teardown. The ClientFactory returns that fake client regardless
// of the bearer token, so the auth middleware is exercised without a real API
// server.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv1beta1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	cf := ClientFactory(func(string) (client.Client, error) { return c, nil })
	_, handler := NewAPI(cf)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// doRequest performs an HTTP request against the test server, JSON-encoding the
// body when provided, and registers the response body for cleanup.
func doRequest(t *testing.T, server *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()

	var reader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)

		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, server.URL+path, reader)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+testToken)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := server.Client().Do(req)
	require.NoError(t, err)

	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// decodeResource decodes a single GitLabResource from the response body.
func decodeResource(t *testing.T, resp *http.Response) GitLabResource {
	t.Helper()

	var res GitLabResource
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))

	return res
}

func TestCRUDLifecycle(t *testing.T) {
	server := newTestServer(t)

	create := GitLabResource{
		Name:      "gitlab",
		Namespace: "gitlab-system",
		Chart: ChartDTO{
			Version: "9.11.1",
			Values:  map[string]any{"global": map[string]any{"hosts": map[string]any{"domain": "example.com"}}},
		},
	}

	t.Run("create", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodPost, "/api/v1/namespaces/gitlab-system/gitlabs", create)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		created := decodeResource(t, resp)
		require.Equal(t, "gitlab", created.Name)
		require.Equal(t, "gitlab-system", created.Namespace)
		require.Equal(t, "9.11.1", created.Chart.Version)
		require.Contains(t, created.Chart.Values, "global")
	})

	t.Run("get", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "gitlab", decodeResource(t, resp).Name)
	})

	t.Run("list", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var list GitLabList
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
		require.Len(t, list.Items, 1)
	})

	t.Run("update", func(t *testing.T) {
		create.Chart.Version = "9.11.4"

		resp := doRequest(t, server, http.MethodPut, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab", create)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "9.11.4", decodeResource(t, resp).Chart.Version)
	})

	t.Run("delete", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodDelete, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab", nil)
		require.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("not found after delete", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab", nil)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestRequiresBearerToken(t *testing.T) {
	server := newTestServer(t)

	get := func(path string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		require.NoError(t, err)

		resp, err := server.Client().Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { _ = resp.Body.Close() })

		return resp
	}

	// An API request without an Authorization header is rejected.
	require.Equal(t, http.StatusUnauthorized, get("/api/v1/gitlabs").StatusCode)

	// Non-API routes stay reachable without a token so the docs UI and SPA can
	// bootstrap before the user provides one.
	require.Equal(t, http.StatusOK, get("/openapi.yaml").StatusCode)
	require.Equal(t, http.StatusOK, get("/").StatusCode)
}

func TestServesOpenAPIAndSPA(t *testing.T) {
	server := newTestServer(t)

	resp := doRequest(t, server, http.MethodGet, "/openapi.yaml", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doRequest(t, server, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// The SPA is only built into web/dist inside the image build; in a bare
	// checkout (and CI) web/dist holds just .gitkeep, so skip the content
	// assertion when the app was not built.
	if _, statErr := fs.Stat(webFS, "web/dist/index.html"); statErr != nil {
		t.Skip("SPA not built (web/dist/index.html absent); run task frontend-build")
	}

	// The built SPA mounts into <div id="app">; assert on that stable marker
	// rather than page copy that changes as the UI evolves.
	require.Contains(t, string(data), `id="app"`)
}
