package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

// Helm runs the helm binary against one cluster.
//
// Every helm invocation of the harness goes through this type. The binary is a
// prerequisite the repository already has, and installing third-party charts is
// not the code under test, so re-implementing an OCI client, a repository index
// cache and the semantics of --wait on top of the Helm SDK would buy nothing.
// Concentrating the exec here is what would make a later move to the SDK a
// one-file change.
type Helm struct {
	Binary     string
	Kubeconfig string
	Context    string
	Out        io.Writer
	Log        logr.Logger
}

// Release is one helm release the harness manages.
type Release struct {
	Name      string
	Namespace string

	// Chart is an OCI reference, a chart name to be resolved against Repo, or a
	// local path. One invocation covers all three, which the SDK does not.
	Chart string

	// Repo is the URL of a classic chart repository, empty for an OCI reference
	// and for a local path.
	Repo string

	Version string

	// Set becomes repeated --set arguments, sorted so that the command line is
	// stable and a rerun is comparable.
	Set map[string]string

	Wait     bool
	Timeout  time.Duration
	CreateNS bool
}

// UpgradeInstall installs the release or upgrades it in place.
func (h *Helm) UpgradeInstall(ctx context.Context, release Release) error {
	args := []string{"upgrade", release.Name, release.Chart, "--install"}

	if release.Namespace != "" {
		args = append(args, "--namespace", release.Namespace)
	}

	if release.CreateNS {
		args = append(args, "--create-namespace")
	}

	if release.Repo != "" {
		args = append(args, "--repo", release.Repo)
	}

	if release.Version != "" {
		args = append(args, "--version", release.Version)
	}

	if release.Wait {
		args = append(args, "--wait")
	}

	if release.Timeout > 0 {
		args = append(args, "--timeout", release.Timeout.String())
	}

	for _, key := range sortedKeys(release.Set) {
		args = append(args, "--set", key+"="+release.Set[key])
	}

	_, err := h.run(ctx, args...)

	return err
}

// Uninstall removes the release, and reports no error when it is not there.
func (h *Helm) Uninstall(ctx context.Context, name, namespace string) error {
	output, err := h.run(ctx, "uninstall", name, "--namespace", namespace, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("uninstalling %s from %s: %w: %s", name, namespace, err, output)
	}

	return nil
}

// Deployed reports whether the release exists and its last operation succeeded.
func (h *Helm) Deployed(ctx context.Context, name, namespace string) (bool, error) {
	output, err := h.run(ctx, "status", name, "--namespace", namespace, "--output", "json")
	if err != nil {
		// helm exits non-zero for a release that is not there, which is an answer
		// rather than a failure.
		return false, nil
	}

	var status struct {
		Info struct {
			Status string `json:"status"`
		} `json:"info"`
	}

	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return false, fmt.Errorf("reading the status of %s in %s: %w", name, namespace, err)
	}

	return status.Info.Status == "deployed", nil
}

// Values returns the values the release was installed with, so that a failure
// artifact records what the Operator was actually configured to do.
func (h *Helm) Values(ctx context.Context, name, namespace string) (map[string]any, error) {
	output, err := h.run(ctx, "get", "values", name, "--namespace", namespace, "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("reading the values of %s in %s: %w", name, namespace, err)
	}

	values := map[string]any{}
	if err := json.Unmarshal([]byte(output), &values); err != nil {
		return nil, fmt.Errorf("parsing the values of %s in %s: %w", name, namespace, err)
	}

	return values, nil
}

// run invokes helm, always with an explicit --kubeconfig: inheriting KUBECONFIG
// would point at the wrong cluster the moment the k3s provider is in use.
func (h *Helm) run(ctx context.Context, args ...string) (string, error) {
	binary := h.Binary
	if binary == "" {
		binary = defaultHelmBinary
	}

	full := append([]string{"--kubeconfig", h.Kubeconfig}, args...)
	if h.Context != "" {
		full = append(full, "--kube-context", h.Context)
	}

	h.Log.V(1).Info("running helm", "args", strings.Join(args, " "))

	//nolint:gosec // the binary is E2E_HELM_BINARY, which exists so a caller can choose it
	cmd := exec.CommandContext(ctx, binary, full...)

	var combined bytes.Buffer

	// Streamed as it happens as well as captured: `helm --wait` can take minutes,
	// and its progress is the only sign that anything is happening.
	cmd.Stdout = io.MultiWriter(&combined, h.Out)
	cmd.Stderr = io.MultiWriter(&combined, h.Out)

	if err := cmd.Run(); err != nil {
		return combined.String(), fmt.Errorf("helm %s: %w", strings.Join(args, " "), err)
	}

	return combined.String(), nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
