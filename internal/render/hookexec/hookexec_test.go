package hookexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/kube"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8syaml "sigs.k8s.io/yaml"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

func TestRunHooks(t *testing.T) {
	t.Run("runs hooks in order and deletes before creating", func(t *testing.T) {
		client := newRecordingClient(t)
		runner := New(client)

		hooks := []render.Hook{
			testHook("ServiceAccount", "rbac", -5),
			testHook("Job", "migrate", 0),
		}

		require.NoError(t, runner.RunHooks(context.Background(), hooks, "pre-install"))

		// Without a delete policy the Helm default applies, so each hook is
		// deleted before it is created and never after it succeeds.
		assert.Equal(t, []string{
			"delete ServiceAccount/rbac",
			"wait-delete ServiceAccount/rbac",
			"create ServiceAccount/rbac",
			"watch ServiceAccount/rbac",
			"delete Job/migrate",
			"wait-delete Job/migrate",
			"create Job/migrate",
			"watch Job/migrate",
		}, client.calls)
	})

	t.Run("deletes succeeded hooks in reverse order after the event", func(t *testing.T) {
		client := newRecordingClient(t)
		runner := New(client)

		first := testHook("Job", "first", 0)
		first.DeletePolicies = []string{PolicyHookSucceeded}
		second := testHook("Job", "second", 1)
		second.DeletePolicies = []string{PolicyHookSucceeded}

		require.NoError(t, runner.RunHooks(context.Background(), []render.Hook{first, second}, "pre-install"))

		// hook-succeeded alone means no delete before creation, and the
		// cleanup runs from the last hook backwards.
		assert.Equal(t, []string{
			"create Job/first",
			"watch Job/first",
			"create Job/second",
			"watch Job/second",
			"delete Job/second",
			"wait-delete Job/second",
			"delete Job/first",
			"wait-delete Job/first",
		}, client.calls)
	})

	t.Run("stops at the first failure and cleans up what ran", func(t *testing.T) {
		client := newRecordingClient(t)
		client.failWatchFor = "Job/second"
		runner := New(client)

		first := testHook("Job", "first", 0)
		first.DeletePolicies = []string{PolicyHookSucceeded}
		second := testHook("Job", "second", 1)
		second.DeletePolicies = []string{PolicyHookFailed}
		third := testHook("Job", "third", 2)

		err := runner.RunHooks(context.Background(), []render.Hook{first, second, third}, "pre-upgrade")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Job default/second")

		// The third hook never runs, the failed hook is deleted by its
		// hook-failed policy, and the succeeded one by hook-succeeded.
		assert.Equal(t, []string{
			"create Job/first",
			"watch Job/first",
			"create Job/second",
			"watch Job/second",
			"delete Job/second",
			"wait-delete Job/second",
			"delete Job/first",
			"wait-delete Job/first",
		}, client.calls)
	})

	t.Run("never deletes a custom resource definition", func(t *testing.T) {
		client := newRecordingClient(t)
		runner := New(client)

		hook := testHook("CustomResourceDefinition", "widgets.example.com", 0)
		hook.DeletePolicies = []string{PolicyBeforeHookCreation, PolicyHookSucceeded}

		require.NoError(t, runner.RunHooks(context.Background(), []render.Hook{hook}, "pre-install"))

		assert.Equal(t, []string{
			"create CustomResourceDefinition/widgets.example.com",
			"watch CustomResourceDefinition/widgets.example.com",
		}, client.calls)
	})

	t.Run("tolerates a missing hook on the first install", func(t *testing.T) {
		// Nothing is live, so the before-hook-creation delete matches no
		// object. The Helm client absorbs that silently; this fake returns
		// ErrNoObjectsVisited, which a kube.Interface may do.
		client := newRecordingClient(t)
		runner := New(client)

		require.NoError(t, runner.RunHooks(
			context.Background(), []render.Hook{testHook("Job", "migrate", 0)}, "pre-install"))
	})

	t.Run("reruns a hook that was left in place", func(t *testing.T) {
		// A hook annotated only hook-failed is never deleted after success,
		// so the second reconcile creates it again. This pins the call
		// sequence only: a real cluster answers 422 on the second apply once
		// a chart upgrade changes the immutable spec of the Job.
		client := newRecordingClient(t)
		runner := New(client)

		hook := testHook("Job", "migrate", 0)
		hook.DeletePolicies = []string{PolicyHookFailed}
		hooks := []render.Hook{hook}

		require.NoError(t, runner.RunHooks(context.Background(), hooks, "pre-install"))
		require.NoError(t, runner.RunHooks(context.Background(), hooks, "pre-install"))

		assert.Equal(t, []string{
			"create Job/migrate",
			"watch Job/migrate",
			"create Job/migrate",
			"watch Job/migrate",
		}, client.calls)
	})

	t.Run("validates strictly when creating and leniently when deleting", func(t *testing.T) {
		client := newRecordingClient(t)
		runner := New(client)

		require.NoError(t, runner.RunHooks(
			context.Background(), []render.Hook{testHook("Job", "migrate", 0)}, "pre-install"))

		// The create is the last build of the run, so the flag is the strict
		// one; the delete before it built leniently.
		assert.True(t, client.lastValidate)
	})

	t.Run("passes the context to the waiter", func(t *testing.T) {
		client := newRecordingClient(t)
		runner := New(client)

		require.NoError(t, runner.RunHooks(
			context.Background(), []render.Hook{testHook("Job", "migrate", 0)}, "pre-install"))

		// The waiter is built through the options interface, once for the
		// delete and once for the watch, each carrying the context.
		assert.Equal(t, 2, client.waitOptions)
		assert.NotContains(t, client.calls, "get-waiter plain")
	})

	t.Run("leaves a hook that could not be created alone", func(t *testing.T) {
		// Helm deletes by hook-failed only when the hook ran and failed. A
		// create error may concern an object that predates this release.
		client := newRecordingClient(t)
		client.failCreateFor = "Job/second"
		runner := New(client)

		first := testHook("Job", "first", 0)
		first.DeletePolicies = []string{PolicyHookSucceeded}
		second := testHook("Job", "second", 1)
		second.DeletePolicies = []string{PolicyHookFailed}

		err := runner.RunHooks(context.Background(), []render.Hook{first, second}, "pre-install")

		require.Error(t, err)
		assert.NotContains(t, client.calls, "delete Job/second")
		assert.NotContains(t, client.calls, "delete Job/first")
	})

	t.Run("stops when the context is cancelled", func(t *testing.T) {
		client := newRecordingClient(t)
		runner := New(client)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := runner.RunHooks(ctx, []render.Hook{testHook("Job", "migrate", 0)}, "pre-install")

		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, client.calls)
	})
}

func TestRun(t *testing.T) {
	t.Run("selects and orders the hooks of the event", func(t *testing.T) {
		client := newRecordingClient(t)
		runner := New(client)

		heavy := testHook("Job", "heavy", 10)
		heavy.Events = []string{"pre-install", "pre-upgrade"}
		light := testHook("Job", "light", -10)
		light.Events = []string{"pre-upgrade"}
		other := testHook("Job", "other", 0)
		other.Events = []string{"post-install"}

		result := &render.Result{Hooks: []render.Hook{heavy, light, other}}

		require.NoError(t, runner.Run(context.Background(), result, "pre-upgrade"))

		assert.Equal(t, []string{
			"delete Job/light",
			"wait-delete Job/light",
			"create Job/light",
			"watch Job/light",
			"delete Job/heavy",
			"wait-delete Job/heavy",
			"create Job/heavy",
			"watch Job/heavy",
		}, client.calls)
	})
}

func TestNewClient(t *testing.T) {
	t.Run("requires a namespace", func(t *testing.T) {
		_, err := NewClient(genericclioptions.NewConfigFlags(true), "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "namespace")
	})

	t.Run("binds the namespace instead of leaving it to the kubeconfig", func(t *testing.T) {
		// The bound namespace is what a hook without metadata.namespace
		// inherits, so it must not fall back to the kubeconfig.
		client, err := NewClient(genericclioptions.NewConfigFlags(true), "gitlab-system")

		require.NoError(t, err)
		assert.Equal(t, "gitlab-system", client.Namespace)
	})
}

func TestHasPolicy(t *testing.T) {
	t.Run("defaults an unannotated hook to before-hook-creation", func(t *testing.T) {
		hook := testHook("Job", "migrate", 0)

		assert.True(t, hasPolicy(hook, PolicyBeforeHookCreation))
		assert.False(t, hasPolicy(hook, PolicyHookSucceeded))
		assert.False(t, hasPolicy(hook, PolicyHookFailed))
	})

	t.Run("uses the annotation when present", func(t *testing.T) {
		hook := testHook("Job", "migrate", 0)
		hook.DeletePolicies = []string{PolicyHookSucceeded}

		assert.False(t, hasPolicy(hook, PolicyBeforeHookCreation))
		assert.True(t, hasPolicy(hook, PolicyHookSucceeded))
	})
}

// testHook builds a rendered hook for the executor tests.
func testHook(kind, name string, weight int) render.Hook {
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       kind,
	}}

	object.SetName(name)
	object.SetNamespace("default")

	return render.Hook{
		Path:   fmt.Sprintf("chart/templates/%s.yaml", name),
		Events: []string{"pre-install"},
		Weight: weight,
		Object: object,
	}
}

// recordingClient is a kube.Interface that records the operations the runner
// performs and keeps the objects it created, so the tests pin the execution
// algorithm without a cluster.
type recordingClient struct {
	t *testing.T

	calls []string

	// live models the objects that exist, so that a create without a
	// preceding delete conflicts the way an API server would.
	live map[string]bool

	// serverSideApply makes a repeated create a patch instead of a
	// conflict, like the Helm client default.
	serverSideApply bool

	// failWatchFor is the short key of a hook whose wait fails.
	failWatchFor string

	// failCreateFor is the short key of a hook whose create fails.
	failCreateFor string

	// lastValidate records the flag of the last Build call.
	lastValidate bool

	// waitOptions counts the options passed to GetWaiterWithOptions.
	waitOptions int
}

// newRecordingClient builds a fake that behaves like the Helm client default:
// server-side apply, so a repeated create succeeds.
func newRecordingClient(t *testing.T) *recordingClient {
	t.Helper()

	return &recordingClient{t: t, live: map[string]bool{}, serverSideApply: true}
}

// key identifies the single resource the runner built for a hook, including
// the namespace so that same-named hooks stay distinguishable.
func key(t *testing.T, resources kube.ResourceList) string {
	t.Helper()

	require.Len(t, resources, 1, "the runner builds exactly one resource per hook")

	info := resources[0]

	return fmt.Sprintf("%s/%s/%s", info.Mapping.GroupVersionKind.Kind, info.Namespace, info.Name)
}

// short renders a key without the namespace, for readable assertions.
func short(key string) string {
	parts := strings.Split(key, "/")

	return parts[0] + "/" + parts[len(parts)-1]
}

func (c *recordingClient) record(format string, args ...interface{}) {
	c.calls = append(c.calls, fmt.Sprintf(format, args...))
}

func (c *recordingClient) Build(reader io.Reader, validate bool) (kube.ResourceList, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	c.lastValidate = validate

	fields := map[string]interface{}{}
	if err := k8syaml.Unmarshal(content, &fields); err != nil {
		return nil, err
	}

	object := &unstructured.Unstructured{Object: fields}

	return kube.ResourceList{{
		Name:      object.GetName(),
		Namespace: object.GetNamespace(),
		Mapping: &meta.RESTMapping{
			GroupVersionKind: object.GroupVersionKind(),
		},
	}}, nil
}

func (c *recordingClient) Create(resources kube.ResourceList, _ ...kube.ClientCreateOption) (*kube.Result, error) {
	name := key(c.t, resources)

	c.record("create %s", short(name))

	if c.failCreateFor == short(name) {
		return nil, errors.New("create rejected")
	}

	// A live object models the API server rejecting a second create, which
	// is what makes the before-hook-creation policy load-bearing.
	if c.live[name] && !c.serverSideApply {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "jobs"}, name)
	}

	c.live[name] = true

	return &kube.Result{}, nil
}

func (c *recordingClient) Delete(resources kube.ResourceList, _ metav1.DeletionPropagation) (*kube.Result, []error) {
	name := key(c.t, resources)

	c.record("delete %s", short(name))

	if !c.live[name] {
		// The Helm client reports a manifest that matched nothing this way.
		return &kube.Result{}, []error{fmt.Errorf("object not found, skipping delete: %w", kube.ErrNoObjectsVisited)}
	}

	delete(c.live, name)

	return &kube.Result{}, nil
}

func (c *recordingClient) GetWaiter(_ kube.WaitStrategy) (kube.Waiter, error) {
	c.record("get-waiter plain")

	return &recordingWaiter{client: c}, nil
}

// GetWaiterWithOptions satisfies kube.InterfaceWaitOptions, which the runner
// prefers so that the context reaches the waiter.
func (c *recordingClient) GetWaiterWithOptions(_ kube.WaitStrategy, opts ...kube.WaitOption) (kube.Waiter, error) {
	c.waitOptions += len(opts)

	return &recordingWaiter{client: c}, nil
}

func (c *recordingClient) Get(kube.ResourceList, bool) (map[string][]runtime.Object, error) {
	return nil, nil
}

func (c *recordingClient) Update(kube.ResourceList, kube.ResourceList, ...kube.ClientUpdateOption) (*kube.Result, error) {
	return &kube.Result{}, nil
}

func (c *recordingClient) IsReachable() error { return nil }

func (c *recordingClient) GetPodList(string, metav1.ListOptions) (*corev1.PodList, error) {
	return &corev1.PodList{}, nil
}

type logWriterFunc = func(namespace, pod, container string) io.Writer

func (c *recordingClient) OutputContainerLogsForPodList(*corev1.PodList, string, logWriterFunc) error {
	return nil
}

func (c *recordingClient) BuildTable(io.Reader, bool) (kube.ResourceList, error) {
	return nil, nil
}

// recordingWaiter records the waits and can fail one named resource.
type recordingWaiter struct {
	client *recordingClient
}

func (w *recordingWaiter) WatchUntilReady(resources kube.ResourceList, _ time.Duration) error {
	name := short(key(w.client.t, resources))

	w.client.record("watch %s", name)

	if w.client.failWatchFor == name {
		return errors.New("hook did not complete")
	}

	return nil
}

func (w *recordingWaiter) WaitForDelete(resources kube.ResourceList, _ time.Duration) error {
	w.client.record("wait-delete %s", short(key(w.client.t, resources)))

	return nil
}

func (w *recordingWaiter) Wait(kube.ResourceList, time.Duration) error { return nil }

func (w *recordingWaiter) WaitWithJobs(kube.ResourceList, time.Duration) error { return nil }
