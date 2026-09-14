/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package siphontables reads the Siphon table definitions out of the
// gitlab-siphon-tables image, so the Operator can serve them from a ConfigMap
// where the cluster does not mount an OCI image into a pod.
//
// The image is config-only: it carries db/siphon/tables/*.yml at the paths they
// have in the GitLab repository and nothing else. Siphon reads that directory
// with `schema generate-values --tables-dir`, so the files have to arrive as
// files.
//
// Resolve before you fetch. Digest costs one request against the manifest and
// answers "has this changed", which is the question a reconcile asks on every
// pass; Fetch pulls layers and is what a change is worth.
package siphontables

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

// TablesPath is the directory inside the image that holds the table definitions.
// It is the path they have in the GitLab repository, which is why the mount is
// deeper than the volume root.
const TablesPath = "db/siphon/tables/"

// tablesExtension is the extension of a table definition. Anything else in the
// directory is not one.
const tablesExtension = ".yml"

// The size limits, both defaults of Options.
//
// A ConfigMap is one etcd object and cannot exceed 1 MiB, so the total is the
// limit that actually bites; the per-file limit is there so a single malformed
// or hostile entry fails on its own rather than by exhausting the budget. The
// total leaves headroom for the keys, the labels, the annotations and the
// managed-fields entry, none of which are the data.
const (
	defaultMaxFileBytes  = 1 << 20         // 1 MiB
	defaultMaxTotalBytes = 900 * (1 << 10) // 900 KiB
)

// defaultPlatform is the platform of the image. It is published for linux/amd64
// only, and it holds no executables, so the choice is about resolving the
// manifest rather than about running anything.
var defaultPlatform = &v1.Platform{OS: "linux", Architecture: "amd64"}

// Options configure a fetch. Build one with the With* options rather than by
// hand, so a zero value stays usable.
type Options struct {
	// Platform selects the manifest of a multi-platform image.
	Platform *v1.Platform

	// Auth authenticates against the registry. It defaults to anonymous.
	//
	// It is never the ambient keychain: that reads the Docker configuration of
	// the current user, which does not exist in the Operator pod, and silently
	// changes behavior between a developer machine and a cluster.
	Auth authn.Authenticator

	// MaxFileBytes caps one extracted file.
	MaxFileBytes int64

	// MaxTotalBytes caps the extracted set.
	MaxTotalBytes int64
}

// Option sets one field of Options.
type Option func(*Options)

// WithPlatform selects the manifest of a multi-platform image.
func WithPlatform(platform *v1.Platform) Option {
	return func(o *Options) { o.Platform = platform }
}

// WithAuth authenticates against the registry.
func WithAuth(auth authn.Authenticator) Option {
	return func(o *Options) { o.Auth = auth }
}

// WithMaxFileBytes caps one extracted file.
func WithMaxFileBytes(limit int64) Option {
	return func(o *Options) { o.MaxFileBytes = limit }
}

// WithMaxTotalBytes caps the extracted set.
func WithMaxTotalBytes(limit int64) Option {
	return func(o *Options) { o.MaxTotalBytes = limit }
}

// resolve applies the options over the defaults.
func resolve(options []Option) Options {
	resolved := Options{
		Platform:      defaultPlatform,
		Auth:          authn.Anonymous,
		MaxFileBytes:  defaultMaxFileBytes,
		MaxTotalBytes: defaultMaxTotalBytes,
	}

	for _, option := range options {
		option(&resolved)
	}

	return resolved
}

// craneOptions translates the resolved options for crane.
func (o Options) craneOptions(ctx context.Context) []crane.Option {
	return []crane.Option{
		crane.WithContext(ctx),
		crane.WithPlatform(o.Platform),
		crane.WithAuth(o.Auth),
	}
}

// Result is the outcome of a fetch.
type Result struct {
	// Files maps the base name of each table definition to its contents. The
	// directory prefix is dropped because a ConfigMap key cannot contain a
	// slash, which is also why the mount has to move to the directory the files
	// came from.
	Files map[string]string

	// Digest is the digest the reference resolved to.
	Digest string

	// TotalBytes is the size of the extracted set.
	TotalBytes int64
}

// TooLargeError reports an extracted set that does not fit in a ConfigMap. It is
// its own type because the caller reports it to an administrator as a
// configuration problem to act on rather than as a transient failure to retry.
type TooLargeError struct {
	Bytes int64
	Limit int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf(
		"the table definitions are %d bytes, over the %d byte budget of a ConfigMap", e.Bytes, e.Limit)
}

// Digest resolves a reference to the digest of its image, without pulling any
// layer. Use it to decide whether a fetch is needed, including for a tag that
// moves.
func Digest(ctx context.Context, reference string, options ...Option) (string, error) {
	resolved := resolve(options)

	digest, err := crane.Digest(reference, resolved.craneOptions(ctx)...)
	if err != nil {
		return "", fmt.Errorf("resolving the digest of %s: %w", reference, err)
	}

	return digest, nil
}

// Fetch pulls the image and returns the table definitions under TablesPath.
//
// The image is config-only, so its flattened filesystem is walked directly
// rather than layer by layer.
func Fetch(ctx context.Context, reference string, options ...Option) (*Result, error) {
	resolved := resolve(options)

	image, err := crane.Pull(reference, resolved.craneOptions(ctx)...)
	if err != nil {
		return nil, fmt.Errorf("pulling %s: %w", reference, err)
	}

	digest, err := image.Digest()
	if err != nil {
		return nil, fmt.Errorf("resolving the digest of %s: %w", reference, err)
	}

	files, total, err := extract(mutate.Extract(image), resolved)
	if err != nil {
		return nil, fmt.Errorf("extracting the table definitions from %s: %w", reference, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("%s holds no table definitions under %s", reference, TablesPath)
	}

	return &Result{Files: files, Digest: digest.String(), TotalBytes: total}, nil
}

// extract walks a flattened image filesystem and returns the regular files under
// TablesPath, keyed by base name.
func extract(reader io.ReadCloser, options Options) (map[string]string, int64, error) {
	defer func() { _ = reader.Close() }()

	files := map[string]string{}
	archive := tar.NewReader(reader)

	var total int64

	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, 0, err
		}

		// Only regular files: a directory carries no definition, and a symlink
		// or a device node in this position is not something to follow.
		name := strings.TrimPrefix(header.Name, "/")
		if header.Typeflag != tar.TypeReg ||
			!strings.HasPrefix(name, TablesPath) ||
			!strings.HasSuffix(name, tablesExtension) {
			continue
		}

		// One byte past the limit is enough to know the file is over it, and it
		// bounds what a malformed entry can read into memory.
		content, err := io.ReadAll(io.LimitReader(archive, options.MaxFileBytes+1))
		if err != nil {
			return nil, 0, err
		}

		if int64(len(content)) > options.MaxFileBytes {
			return nil, 0, fmt.Errorf(
				"the table definition %s is over the %d byte limit for one file", name, options.MaxFileBytes)
		}

		total += int64(len(content))
		if total > options.MaxTotalBytes {
			return nil, 0, &TooLargeError{Bytes: total, Limit: options.MaxTotalBytes}
		}

		files[path.Base(name)] = string(content)
	}

	return files, total, nil
}

// AuthFromDockerConfig builds an authenticator for the registry of a reference
// from the contents of a kubernetes.io/dockerconfigjson Secret.
//
// A configuration without an entry for that registry yields the anonymous
// authenticator rather than an error: a Secret can legitimately cover several
// registries, and a public image needs no credential at all.
func AuthFromDockerConfig(dockerConfigJSON []byte, reference string) (authn.Authenticator, error) {
	parsed, err := name.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("parsing the image reference %q: %w", reference, err)
	}

	var config struct {
		Auths map[string]authn.AuthConfig `json:"auths"`
	}

	if err := json.Unmarshal(dockerConfigJSON, &config); err != nil {
		return nil, fmt.Errorf("parsing the docker configuration: %w", err)
	}

	registry := parsed.Context().RegistryStr()

	for key, entry := range config.Auths {
		if matchesRegistry(key, registry) {
			return authn.FromConfig(entry), nil
		}
	}

	return authn.Anonymous, nil
}

// matchesRegistry reports whether a docker configuration key addresses a
// registry. The keys are written many ways, from a bare host to a full URL with
// a path, so the host is compared rather than the key.
func matchesRegistry(key, registry string) bool {
	trimmed := key

	for _, scheme := range []string{"https://", "http://"} {
		trimmed = strings.TrimPrefix(trimmed, scheme)
	}

	if host, _, found := strings.Cut(trimmed, "/"); found {
		trimmed = host
	}

	return trimmed == registry
}
