package bridge

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// decodeSiphon decodes a single SiphonResource from the response body.
func decodeSiphon(t *testing.T, resp *http.Response) SiphonResource {
	t.Helper()

	var res SiphonResource
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))

	return res
}

// siphonRequest is a complete Siphon, the body the form sends once the add-on
// is enabled.
func siphonRequest() SiphonResource {
	return SiphonResource{
		Source: SiphonSourceDTO{
			Host:              "gitlab-postgresql-rw.databases.svc.cluster.local",
			Port:              5432,
			Database:          "gitlabhq_production",
			User:              "siphon",
			PasswordSecretRef: SecretRefDTO{Name: "gitlab-siphon-postgresql", Key: "password"},
			SSLMode:           "require",
		},
		Queue: SiphonQueueDTO{URL: "nats://nats.nats.svc.cluster.local:4222"},
		Sink: SiphonSinkDTO{
			Host:              "clickhouse.databases.svc.cluster.local",
			Port:              9000,
			Database:          "gitlab_clickhouse_main_production",
			Username:          "gitlab",
			PasswordSecretRef: SecretRefDTO{Name: "gitlab-clickhouse-gitlab", Key: "password"},
		},
		Tables: SiphonTablesDTO{Source: "Auto"},
		Chart:  ChartDTO{Version: "1.21.0"},
	}
}

func TestSiphonLifecycle(t *testing.T) {
	server, c := newTestServer(t)

	create := siphonRequest()

	t.Run("none before it is created", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", nil)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("create", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodPut, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", create)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		created := decodeSiphon(t, resp)
		require.Equal(t, "gitlab-siphon", created.Name)
		require.Equal(t, "gitlab-system", created.Namespace)
		require.Equal(t, "gitlab", created.GitLabRef)
		require.Equal(t, "1.21.0", created.Chart.Version)
	})

	// The link to the instance is spec.gitlabRef, which the path supplies: the
	// body never carries it.
	t.Run("stores a Siphon referencing the instance", func(t *testing.T) {
		siphon := &apiv2alpha1.Siphon{}
		key := client.ObjectKey{Namespace: "gitlab-system", Name: "gitlab-siphon"}
		require.NoError(t, c.Get(t.Context(), key, siphon))

		require.Equal(t, "gitlab", siphon.Spec.GitLabRef.Name)
		require.Equal(t, "gitlab-postgresql-rw.databases.svc.cluster.local", siphon.Spec.Source.Host)
		require.Equal(t, int32(5432), siphon.Spec.Source.Port)
		require.Equal(t, "gitlab-siphon-postgresql", siphon.Spec.Source.PasswordSecretRef.Name)
		require.Equal(t, apiv2alpha1.PostgreSQLSSLMode("require"), siphon.Spec.Source.SSLMode)
		require.Equal(t, "nats://nats.nats.svc.cluster.local:4222", siphon.Spec.Queue.URL)
		require.Nil(t, siphon.Spec.Queue.Auth)
		require.Equal(t, "gitlab_clickhouse_main_production", siphon.Spec.Sink.Database)
		require.Equal(t, "gitlab", siphon.Spec.Sink.Username)
		require.Equal(t, apiv2alpha1.TablesSourceAuto, siphon.Spec.Tables.Source)
		require.Equal(t, "1.21.0", siphon.Spec.Chart.Version)
	})

	t.Run("get", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		got := decodeSiphon(t, resp)
		require.Equal(t, "gitlab-siphon", got.Name)
		require.Equal(t, "gitlabhq_production", got.Source.Database)

		// Nothing has reconciled it, so it carries no status rather than an
		// empty one the user interface would render as a row of dashes.
		require.Nil(t, got.Status)
	})

	// The second PUT finds the Siphon of the instance and replaces it rather
	// than failing on a name that already exists.
	t.Run("update", func(t *testing.T) {
		create.Chart.Version = "1.22.0"
		create.Queue.Auth = &SiphonQueueAuthDTO{
			UsernameSecretRef: SecretRefDTO{Name: "nats-credentials", Key: "username"},
			PasswordSecretRef: SecretRefDTO{Name: "nats-credentials", Key: "password"},
		}

		resp := doRequest(t, server, http.MethodPut, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", create)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		updated := decodeSiphon(t, resp)
		require.Equal(t, "gitlab-siphon", updated.Name)
		require.Equal(t, "1.22.0", updated.Chart.Version)
		require.NotNil(t, updated.Queue.Auth)
		require.Equal(t, "nats-credentials", updated.Queue.Auth.PasswordSecretRef.Name)

		list := &apiv2alpha1.SiphonList{}
		require.NoError(t, c.List(t.Context(), list))
		require.Len(t, list.Items, 1)
	})

	t.Run("delete", func(t *testing.T) {
		resp := doRequest(t, server, http.MethodDelete, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", nil)
		require.Equal(t, http.StatusNoContent, resp.StatusCode)

		resp = doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", nil)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// A Siphon created by hand carries whatever name its author chose, and the
// bridge manages it through the reference rather than the name.
func TestSiphonFoundByReference(t *testing.T) {
	server, c := newTestServer(t)

	existing := &apiv2alpha1.Siphon{}
	existing.Name = "analytics"
	existing.Namespace = "gitlab-system"
	existing.Spec.GitLabRef = apiv2alpha1.GitLabReference{Name: "gitlab"}
	existing.Spec.Chart.Version = "1.21.0"
	require.NoError(t, c.Create(t.Context(), existing))

	resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "analytics", decodeSiphon(t, resp).Name)

	update := siphonRequest()
	resp = doRequest(t, server, http.MethodPut, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", update)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "analytics", decodeSiphon(t, resp).Name)
}

// The Siphon of another instance in the same namespace is not this one.
func TestSiphonScopedToItsInstance(t *testing.T) {
	server, c := newTestServer(t)

	other := &apiv2alpha1.Siphon{}
	other.Name = "other-siphon"
	other.Namespace = "gitlab-system"
	other.Spec.GitLabRef = apiv2alpha1.GitLabReference{Name: "other"}
	require.NoError(t, c.Create(t.Context(), other))

	resp := doRequest(t, server, http.MethodGet, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// The name is derived, so a name the definition would reject is reported
// before the API server sees it.
func TestSiphonRejectsDerivedNameTooLong(t *testing.T) {
	server, _ := newTestServer(t)

	path := "/api/v1/namespaces/gitlab-system/gitlabs/gitlab-instance-with-a-long-name/siphon"
	resp := doRequest(t, server, http.MethodPut, path, siphonRequest())

	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// The wire type carries the constraints of the custom resource definition, so
// a queue URL of the wrong scheme is a 422 from the bridge.
func TestSiphonRejectsInvalidQueueURL(t *testing.T) {
	server, _ := newTestServer(t)

	body := siphonRequest()
	body.Queue.URL = "https://nats.nats.svc.cluster.local:4222"

	resp := doRequest(t, server, http.MethodPut, "/api/v1/namespaces/gitlab-system/gitlabs/gitlab/siphon", body)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}
