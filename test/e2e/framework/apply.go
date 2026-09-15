package framework

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FieldOwner marks everything the harness applies, so that a stray object can be
// told apart from one the Operator owns.
const FieldOwner = client.FieldOwner("gitlab-operator-e2e")

// manifestFetchTimeout bounds the download of a definition bundle.
const manifestFetchTimeout = time.Minute

// helm returns a helm runner bound to this cluster, writing into the harness log.
func (h *Harness) helm() *Helm {
	return &Helm{
		Binary:     h.Config.HelmBinary,
		Kubeconfig: h.Cluster.KubeconfigPath(),
		Context:    h.Cluster.KubeContext(),
		Out:        logWriter{log: h.Log},
		Log:        h.Log,
	}
}

// applyManifestURL downloads a multi-document manifest and applies every object
// in it, the way the Operator applies what it renders.
func (h *Harness) applyManifestURL(ctx context.Context, url string) error {
	fetchCtx, cancel := context.WithTimeout(ctx, manifestFetchTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building a request for %s: %w", url, err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: the server answered %s", url, response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading %s: %w", url, err)
	}

	return h.ApplyYAML(ctx, body)
}

// ApplyYAML applies every document of a manifest, server-side.
func (h *Harness) ApplyYAML(ctx context.Context, manifest []byte) error {
	objects, err := DecodeYAML(manifest)
	if err != nil {
		return err
	}

	for _, object := range objects {
		if err := h.Apply(ctx, object); err != nil {
			return err
		}
	}

	return nil
}

// Apply writes one object, server-side, so that applying a bundle twice changes
// nothing and a field another actor owns is reported rather than clobbered.
func (h *Harness) Apply(ctx context.Context, object *unstructured.Unstructured) error {
	err := h.Client.Apply(ctx, client.ApplyConfigurationFromUnstructured(object),
		FieldOwner, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("applying %s %s: %w",
			object.GetKind(), client.ObjectKeyFromObject(object), err)
	}

	return nil
}

// DecodeYAML splits a multi-document manifest into objects, dropping the empty
// documents a template leaves behind.
func DecodeYAML(manifest []byte) ([]*unstructured.Unstructured, error) {
	reader := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)

	var objects []*unstructured.Unstructured

	for {
		object := &unstructured.Unstructured{}

		err := reader.Decode(object)
		if errors.Is(err, io.EOF) {
			return objects, nil
		}

		if err != nil {
			return nil, fmt.Errorf("decoding the manifest: %w", err)
		}

		if len(object.Object) == 0 {
			continue
		}

		objects = append(objects, object)
	}
}

// pollUntil polls the condition until it holds or the budget runs out. It is the
// error-returning sibling of Eventually, for the harness paths that run outside a
// test.
func pollUntil(ctx context.Context, budget time.Duration, condition func(ctx context.Context) (bool, error)) error {
	return utilruntime.PollUntilContextTimeout(ctx, PollInterval, budget, true, condition)
}

// errorsAs is errors.As, wrapped so that the callers of the framework need not
// import errors just to tell a skip from a failure.
func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}

// logWriter turns a logger into an io.Writer, for the output of a subprocess.
type logWriter struct {
	log logr.Logger
}

func (w logWriter) Write(p []byte) (int, error) {
	w.log.Info(strings.TrimRight(string(p), "\n"))

	return len(p), nil
}
