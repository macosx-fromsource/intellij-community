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

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// testToken is the bearer token attached to requests by doRequest. The test
// ClientFactory ignores its value and returns the fake client, so any non-empty
// token authenticates.
const testToken = "test-token"

// newTestServer starts an httptest server backed by a fake client, registered
// for automatic teardown. The ClientFactory returns that fake client regardless
// of the bearer token, so the auth middleware is exercised without a real API
// server.
// It returns the fake client too, so a test can assert on the GitLabCore
// objects the handlers wrote rather than only on what they answered.
func newTestServer(t *testing.T) (*httptest.Server, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv2alpha1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	cf := ClientFactory(func(string) (client.Client, error) { return c, nil })
	_, handler := NewAPI(cf)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server, c
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

func TestListChartVersions(t *testing.T) {
	server, _ := newTestServer(t)

	resp := doRequest(t, server, http.MethodGet, "/api/v1/chart-versions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out ChartVersionsDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))

	// The catalog is empty in a unit test, but the endpoint must still answer a
	// JSON array rather than null so the client can iterate it unconditionally.
	require.NotNil(t, out.Versions)
}

func TestCRUDLifecycle(t *testing.T) {
	server, c := newTestServer(t)

	create := GitLabResource{
		Name:      "gitlab",
		Namespace: "gitlab-system",
		Hostname:  "gitlab.example.com",
		Edition:   "ee",
		License: &LicenseDTO{
			SecretRef: SecretRefDTO{Name: "gitlab-license", Key: "license"},
		},
		PostgreSQL: &PostgreSQLDTO{
			Host:              "gitlab-postgresql",
			PasswordSecretRef: SecretRefDTO{Name: "gitlab-postgresql-password", Key: "password"},
		},
		Redis: &RedisDTO{
			Host:              "gitlab-valkey",
			PasswordSecretRef: SecretRefDTO{Name: "gitlab-valkey-auth", Key: "default"},
		},
		ObjectStorage: &ObjectStorageDTO{
			ConnectionSecretRef: SecretRefDTO{Name: "gitlab-object-storage", Key: "connection"},
		},
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
		require.Equal(t, "gitlab.example.com", created.Hostname)
		require.Equal(t, "9.11.1", created.Chart.Version)
		require.Contains(t, created.Chart.Values, "global")
		require.Equal(t, "ee", created.Edition)
		require.NotNil(t, created.License)
		require.Equal(t, "gitlab-license", created.License.SecretRef.Name)
		require.Equal(t, "license", created.License.SecretRef.Key)
		require.NotNil(t, created.PostgreSQL)
		require.Equal(t, "gitlab-postgresql", created.PostgreSQL.Host)
		require.Equal(t, "gitlab-postgresql-password", created.PostgreSQL.PasswordSecretRef.Name)
		require.NotNil(t, created.Redis)
		require.Equal(t, "gitlab-valkey", created.Redis.Host)
		require.Equal(t, "default", created.Redis.PasswordSecretRef.Key)
		require.NotNil(t, created.ObjectStorage)
		require.Equal(t, "gitlab-object-storage", created.ObjectStorage.ConnectionSecretRef.Name)
	})

	t.Run("get", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		got := decodeResource(t, resp)
		require.Equal(t, "gitlab", got.Name)
		require.Equal(t, "gitlab.example.com", got.Hostname)
		require.NotNil(t, got.License)
	})

	t.Run("list", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var list GitLabList
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
		require.Len(t, list.Items, 1)
	})

	t.Run("stores a GitLabCore with structured fields", func(t *testing.T) {
		core := &apiv2alpha1.GitLabCore{}
		key := client.ObjectKey{Namespace: "gitlab-system", Name: "gitlab"}
		require.NoError(t, c.Get(t.Context(), key, core))

		require.Equal(t, "gitlab.example.com", core.Spec.Hostname)
		require.Equal(t, apiv2alpha1.EditionEE, core.Spec.Edition)
		require.Equal(t, "9.11.1", core.Spec.Chart.Version)
		require.Contains(t, core.Spec.Chart.Values.Object, "global")
		require.NotNil(t, core.Spec.License)
		require.Equal(t, "gitlab-license", core.Spec.License.SecretRef.Name)
		require.Equal(t, "license", core.Spec.License.SecretRef.Key)
		require.NotNil(t, core.Spec.PostgreSQL)
		require.Equal(t, "gitlab-postgresql", core.Spec.PostgreSQL.Host)
		require.Equal(t, "gitlab-postgresql-password", core.Spec.PostgreSQL.PasswordSecretRef.Name)
		require.Equal(t, "password", core.Spec.PostgreSQL.PasswordSecretRef.Key)
		require.NotNil(t, core.Spec.Redis)
		require.Equal(t, "gitlab-valkey", core.Spec.Redis.Host)
		require.Equal(t, "gitlab-valkey-auth", core.Spec.Redis.PasswordSecretRef.Name)
		require.Equal(t, "default", core.Spec.Redis.PasswordSecretRef.Key)
		require.NotNil(t, core.Spec.ObjectStorage)
		require.Equal(t, "gitlab-object-storage", core.Spec.ObjectStorage.ConnectionSecretRef.Name)
		require.Equal(t, "connection", core.Spec.ObjectStorage.ConnectionSecretRef.Key)
	})

	t.Run("update", func(t *testing.T) {
		create.Chart.Version = "9.11.4"
		create.Hostname = "gitlab.example.org"
		create.Edition = "ce"

		resp := doRequest(t, server, http.MethodPut, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab", create)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		updated := decodeResource(t, resp)
		require.Equal(t, "9.11.4", updated.Chart.Version)
		require.Equal(t, "gitlab.example.org", updated.Hostname)
		require.Equal(t, "ce", updated.Edition)
	})

	// An update replaces the resource, so a field the request leaves out is
	// cleared rather than kept from the stored object.
	t.Run("update clears an omitted license", func(t *testing.T) {
		create.License = nil

		resp := doRequest(t, server, http.MethodPut, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab", create)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Nil(t, decodeResource(t, resp).License)
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

// The wire types carry the constraints of the custom resource definition, so
// the bridge rejects a hostname the API server would reject anyway, and the
// caller gets the reason from the OpenAPI document rather than from a webhook.
func TestRejectsUnknownEdition(t *testing.T) {
	server, _ := newTestServer(t)

	resp := doRequest(t, server, http.MethodPost, "/api/v1/namespaces/gitlab-system/gitlabs", GitLabResource{
		Name:      "gitlab",
		Namespace: "gitlab-system",
		Edition:   "enterprise",
	})

	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestRejectsInvalidHostname(t *testing.T) {
	server, _ := newTestServer(t)

	resp := doRequest(t, server, http.MethodPost, "/api/v1/namespaces/gitlab-system/gitlabs", GitLabResource{
		Name:      "gitlab",
		Namespace: "gitlab-system",
		Hostname:  "Not A Hostname",
	})

	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestRequiresBearerToken(t *testing.T) {
	server, _ := newTestServer(t)

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
	server, _ := newTestServer(t)

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
