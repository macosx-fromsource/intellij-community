package siphontables

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tarEntry is one entry of a fixture archive. A zero Typeflag means a regular
// file, which is what a table definition is.
type tarEntry struct {
	name     string
	content  string
	typeflag byte
	linkname string
}

// tarball builds a flattened image filesystem, the shape mutate.Extract yields.
func buildTar(t *testing.T, entries ...tarEntry) io.ReadCloser {
	t.Helper()

	buffer := &bytes.Buffer{}
	archive := tar.NewWriter(buffer)

	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}

		require.NoError(t, archive.WriteHeader(&tar.Header{
			Name:     entry.name,
			Size:     int64(len(entry.content)),
			Typeflag: typeflag,
			Linkname: entry.linkname,
			Mode:     0o644,
		}))

		_, err := archive.Write([]byte(entry.content))
		require.NoError(t, err)
	}

	require.NoError(t, archive.Close())

	return io.NopCloser(buffer)
}

func TestExtract(t *testing.T) {
	options := resolve(nil)

	t.Run("keys the definitions by base name", func(t *testing.T) {
		files, total, err := extract(buildTar(t,
			tarEntry{name: "db/siphon/tables/users.yml", content: "table: users"},
			tarEntry{name: "db/siphon/tables/merge_requests.yml", content: "table: merge_requests"},
		), options)

		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"users.yml":          "table: users",
			"merge_requests.yml": "table: merge_requests",
		}, files)
		assert.Equal(t, int64(len("table: users")+len("table: merge_requests")), total)
	})

	t.Run("tolerates a leading slash", func(t *testing.T) {
		files, _, err := extract(buildTar(t,
			tarEntry{name: "/db/siphon/tables/users.yml", content: "table: users"},
		), options)

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"users.yml": "table: users"}, files)
	})

	t.Run("skips what is not a table definition", func(t *testing.T) {
		files, total, err := extract(buildTar(t,
			// Not under the tables directory.
			tarEntry{name: "db/structure.sql", content: "CREATE TABLE"},
			tarEntry{name: "db/siphon/README.md", content: "docs"},
			// Under it, but not a definition.
			tarEntry{name: "db/siphon/tables/notes.txt", content: "notes"},
			tarEntry{name: "db/siphon/tables/nested.yaml", content: "wrong extension"},
			// Under it, but not a regular file.
			tarEntry{name: "db/siphon/tables/", typeflag: tar.TypeDir},
			tarEntry{name: "db/siphon/tables/link.yml", typeflag: tar.TypeSymlink, linkname: "users.yml"},
			tarEntry{name: "db/siphon/tables/users.yml", content: "table: users"},
		), options)

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"users.yml": "table: users"}, files)
		assert.Equal(t, int64(len("table: users")), total)
	})

	t.Run("rejects a file over the per-file limit", func(t *testing.T) {
		_, _, err := extract(buildTar(t,
			tarEntry{name: "db/siphon/tables/huge.yml", content: strings.Repeat("x", 11)},
		), resolve([]Option{WithMaxFileBytes(10)}))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "over the 10 byte limit for one file")
	})

	t.Run("accepts a file exactly at the per-file limit", func(t *testing.T) {
		files, _, err := extract(buildTar(t,
			tarEntry{name: "db/siphon/tables/exact.yml", content: strings.Repeat("x", 10)},
		), resolve([]Option{WithMaxFileBytes(10)}))

		require.NoError(t, err)
		assert.Len(t, files, 1)
	})

	t.Run("rejects a set over the total limit", func(t *testing.T) {
		_, _, err := extract(buildTar(t,
			tarEntry{name: "db/siphon/tables/one.yml", content: strings.Repeat("x", 6)},
			tarEntry{name: "db/siphon/tables/two.yml", content: strings.Repeat("x", 6)},
		), resolve([]Option{WithMaxTotalBytes(10)}))

		require.Error(t, err)

		// The caller distinguishes this from a transient failure, so it must be
		// recognizable rather than a string.
		var tooLarge *TooLargeError
		require.ErrorAs(t, err, &tooLarge)
		assert.Equal(t, int64(10), tooLarge.Limit)
		assert.Equal(t, int64(12), tooLarge.Bytes)
		assert.Contains(t, tooLarge.Error(), "budget of a ConfigMap")
	})

	t.Run("returns an empty set rather than an error", func(t *testing.T) {
		// Fetch turns this into the error; extract reports what it found, so the
		// two concerns stay separable.
		files, total, err := extract(buildTar(t,
			tarEntry{name: "db/structure.sql", content: "CREATE TABLE"},
		), options)

		require.NoError(t, err)
		assert.Empty(t, files)
		assert.Zero(t, total)
	})
}

// pushTables publishes a config-only image holding the given files under
// TablesPath to a registry that lives for the test, and returns its reference.
func pushTables(t *testing.T, files map[string]string) string {
	t.Helper()

	// The test registry is chatty and says nothing a failing assertion does not.
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	archive := tar.NewWriter(buffer)

	for name, content := range files {
		require.NoError(t, archive.WriteHeader(&tar.Header{
			Name: TablesPath + name,
			Size: int64(len(content)),
			Mode: 0o644,
		}))

		_, err := archive.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, archive.Close())

	contents := buffer.Bytes()

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(contents)), nil
	})
	require.NoError(t, err)

	image, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	reference := fmt.Sprintf("%s/gitlab-siphon-tables:v19.2.0-ee", parsed.Host)
	require.NoError(t, crane.Push(image, reference))

	return reference
}

func TestFetch(t *testing.T) {
	ctx := context.Background()

	t.Run("pulls the definitions and resolves the digest", func(t *testing.T) {
		reference := pushTables(t, map[string]string{
			"users.yml":          "table: users",
			"merge_requests.yml": "table: merge_requests",
		})

		result, err := Fetch(ctx, reference, WithPlatform(nil))

		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"users.yml":          "table: users",
			"merge_requests.yml": "table: merge_requests",
		}, result.Files)
		assert.True(t, strings.HasPrefix(result.Digest, "sha256:"), "got %q", result.Digest)
		assert.Equal(t, int64(len("table: users")+len("table: merge_requests")), result.TotalBytes)
	})

	t.Run("Digest agrees with Fetch without pulling a layer", func(t *testing.T) {
		reference := pushTables(t, map[string]string{"users.yml": "table: users"})

		digest, err := Digest(ctx, reference, WithPlatform(nil))
		require.NoError(t, err)

		result, err := Fetch(ctx, reference, WithPlatform(nil))
		require.NoError(t, err)

		assert.Equal(t, result.Digest, digest)
	})

	t.Run("reports an image that holds no definitions", func(t *testing.T) {
		reference := pushTables(t, map[string]string{})

		_, err := Fetch(ctx, reference, WithPlatform(nil))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "holds no table definitions under db/siphon/tables/")
	})

	t.Run("reports a reference that does not resolve", func(t *testing.T) {
		reference := pushTables(t, map[string]string{"users.yml": "table: users"})
		absent := strings.Replace(reference, ":v19.2.0-ee", ":v0.0.0-ee", 1)

		_, err := Fetch(ctx, absent, WithPlatform(nil))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "pulling")
	})

	t.Run("propagates the size guard", func(t *testing.T) {
		reference := pushTables(t, map[string]string{"users.yml": "table: users"})

		_, err := Fetch(ctx, reference, WithPlatform(nil), WithMaxTotalBytes(1))

		var tooLarge *TooLargeError
		require.ErrorAs(t, err, &tooLarge)
	})
}

func TestResolve(t *testing.T) {
	t.Run("defaults to an anonymous pull of linux/amd64", func(t *testing.T) {
		// Never the ambient keychain: it reads the Docker configuration of the
		// current user, which does not exist in the Operator pod.
		options := resolve(nil)

		assert.Equal(t, authn.Anonymous, options.Auth)
		assert.Equal(t, &v1.Platform{OS: "linux", Architecture: "amd64"}, options.Platform)
		assert.Equal(t, int64(1<<20), options.MaxFileBytes)
		assert.Equal(t, int64(900*(1<<10)), options.MaxTotalBytes)
	})

	t.Run("the total budget fits in a ConfigMap", func(t *testing.T) {
		assert.Less(t, int64(defaultMaxTotalBytes), int64(1<<20),
			"a ConfigMap is one etcd object and cannot exceed 1 MiB")
	})
}

func TestAuthFromDockerConfig(t *testing.T) {
	const reference = "registry.gitlab.com/gitlab-org/gitlab/gitlab-siphon-tables:v19.2.0-ee"

	// The credentials are compared as the authenticator resolves them rather
	// than as structs: authn.AuthConfig derives the base64 `auth` field from the
	// user name and the password while unmarshalling, so two configurations that
	// authenticate identically are not equal values.
	cases := []struct {
		name   string
		config string
		want   *authn.AuthConfig
	}{
		{
			name:   "a bare host key",
			config: `{"auths":{"registry.gitlab.com":{"username":"u","password":"p"}}}`,
			want:   &authn.AuthConfig{Username: "u", Password: "p"},
		},
		{
			name:   "a key written as a URL with a path",
			config: `{"auths":{"https://registry.gitlab.com/v1/":{"username":"u","password":"p"}}}`,
			want:   &authn.AuthConfig{Username: "u", Password: "p"},
		},
		{
			name:   "a configuration covering several registries",
			config: `{"auths":{"example.com":{"username":"x"},"registry.gitlab.com":{"username":"u","password":"p"}}}`,
			want:   &authn.AuthConfig{Username: "u", Password: "p"},
		},
		{
			// A Secret can legitimately cover only other registries, and a
			// public image needs no credential, so this is not an error.
			name:   "no entry for the registry",
			config: `{"auths":{"example.com":{"username":"x"}}}`,
			want:   &authn.AuthConfig{},
		},
		{
			name:   "no auths at all",
			config: `{}`,
			want:   &authn.AuthConfig{},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			auth, err := AuthFromDockerConfig([]byte(testCase.config), reference)
			require.NoError(t, err)

			resolved, err := auth.Authorization()
			require.NoError(t, err)

			assert.Equal(t, testCase.want.Username, resolved.Username)
			assert.Equal(t, testCase.want.Password, resolved.Password)
		})
	}

	t.Run("reports a malformed configuration", func(t *testing.T) {
		_, err := AuthFromDockerConfig([]byte("not json"), reference)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing the docker configuration")
	})

	t.Run("reports an unparseable reference", func(t *testing.T) {
		_, err := AuthFromDockerConfig([]byte(`{}`), "NOT A REFERENCE")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing the image reference")
	})
}
