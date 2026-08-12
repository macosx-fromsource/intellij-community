// Package hookexec runs the hooks of a rendered chart against a cluster.
//
// Run the hooks of every release: the GitLab chart generates its secrets from
// a pre-install hook, so an operator that ignores hooks installs a broken
// release. Unlike the render and objects packages, this one talks to the API
// server.
//
// A Runner is bound to the namespace of its client, so build one per release.
// It holds no mutable state of its own.
//
// Execution follows the Helm SDK algorithm, which charts are written against,
// and reuses the Helm kube client for building, creating, waiting, and
// deleting:
//
//  1. Hooks run in the order of render.Result.HooksFor: ascending weight,
//     then object name, within a kind-sorted list.
//  2. A hook without a "helm.sh/hook-delete-policy" annotation is treated as
//     "before-hook-creation", the Helm default.
//  3. Before creation, a hook with the "before-hook-creation" policy is
//     deleted and waited out, which is what makes a rerun of a completed Job
//     possible, because a Job spec is immutable.
//  4. Each hook is created and then watched to completion before the next
//     one starts.
//  5. After every hook of the event succeeds, hooks carrying
//     "hook-succeeded" are deleted in reverse order. When a hook fails, that
//     hook is deleted per "hook-failed" and the hooks that already succeeded
//     are deleted per "hook-succeeded".
//  6. CustomResourceDefinition hooks are never deleted, to avoid cascading
//     garbage collection.
//
// Three deviations from the Helm SDK: the "helm.sh/hook-output-log-policy"
// annotation is not honored, because the renderer does not model it; a hook
// is deleted per "hook-failed" only when it ran and failed, not when building
// or creating it failed, because the object may predate this release; and a
// delete policy that cannot be applied is logged rather than returned, as
// deleteByPolicy describes.
package hookexec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/kube"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8syaml "sigs.k8s.io/yaml"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

// Hook delete policies, as defined by the Helm SDK.
const (
	PolicyBeforeHookCreation = "before-hook-creation"
	PolicyHookSucceeded      = "hook-succeeded"
	PolicyHookFailed         = "hook-failed"
)

// crdKind is never deleted by a delete policy.
const crdKind = "CustomResourceDefinition"

// defaultTimeout bounds the wait for a single hook to complete. It is the
// Helm CLI default for --timeout, so a caller that configures nothing behaves
// like "helm install".
const defaultTimeout = 5 * time.Minute

// Runner executes rendered hooks against one cluster.
type Runner struct {
	client          kube.Interface
	waitStrategy    kube.WaitStrategy
	timeout         time.Duration
	serverSideApply bool
	log             *slog.Logger
}

// Option configures a Runner.
type Option func(*Runner)

// WithTimeout bounds the wait for a single hook to complete. Set it for the
// GitLab chart: the shared-secrets pre-install Job can exceed the five-minute
// default on a cold node while pulling the toolbox image.
func WithTimeout(timeout time.Duration) Option {
	return func(r *Runner) {
		r.timeout = timeout
	}
}

// WithWaitStrategy selects the Helm wait strategy. The default watches
// resource status instead of polling.
func WithWaitStrategy(strategy kube.WaitStrategy) Option {
	return func(r *Runner) {
		r.waitStrategy = strategy
	}
}

// WithServerSideApply selects the apply mode of hook creation. The default
// is server-side, matching the Helm CLI default for an install.
func WithServerSideApply(serverSideApply bool) Option {
	return func(r *Runner) {
		r.serverSideApply = serverSideApply
	}
}

// WithLogger reports each hook as it runs. The default discards everything.
func WithLogger(log *slog.Logger) Option {
	return func(r *Runner) {
		r.log = log
	}
}

// New builds a Runner on a Helm kube client, bound to the namespace of that
// client. Build one per release:
//
//	client, err := hookexec.NewClient(getter, namespace)
//	if err != nil {
//	    return err
//	}
//
//	err = hookexec.New(client).Run(ctx, result, "pre-install")
//
// Set the namespace of the client deliberately, with NewClient or through the
// Namespace field. It is what a hook inherits when its manifest omits
// metadata.namespace, and the Helm fallback is the namespace of the
// kubeconfig, which in-cluster is the namespace of the operator. Such a hook
// is then created beside the operator, with no error reported. Cluster-scoped
// hooks and hooks that name their own namespace are unaffected.
func New(client kube.Interface, options ...Option) *Runner {
	runner := &Runner{
		client:          client,
		waitStrategy:    kube.StatusWatcherStrategy,
		timeout:         defaultTimeout,
		serverSideApply: true,
		log:             slog.New(slog.DiscardHandler),
	}

	for _, option := range options {
		option(runner)
	}

	return runner
}

// NewClient builds a Helm kube client bound to a namespace, which is the
// namespace a hook inherits when its manifest omits one. The namespace is
// required, so that the client cannot fall back to the kubeconfig one; see
// New.
func NewClient(getter genericclioptions.RESTClientGetter, namespace string) (*kube.Client, error) {
	if namespace == "" {
		return nil, fmt.Errorf("hookexec: namespace is required")
	}

	client := kube.New(getter)
	client.Namespace = namespace

	return client, nil
}

// Run executes the hooks of the result that fire for the event, in the Helm
// execution order. It returns once every hook has completed, or at the first
// failure, after applying the delete policies of the hooks that ran.
func (r *Runner) Run(ctx context.Context, result *render.Result, event string) error {
	if result == nil {
		return nil
	}

	return r.RunHooks(ctx, result.HooksFor(event), event)
}

// RunHooks executes an already-ordered list of hooks. Use Run unless the
// caller needs to filter the hooks further; the order of the list is the
// execution order and is not sorted again.
func (r *Runner) RunHooks(ctx context.Context, hooks []render.Hook, event string) error {
	for index, hook := range hooks {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := r.runHook(ctx, hook, event)
		if err == nil {
			continue
		}

		// Only a hook that ran and failed is deleted by its hook-failed
		// policy. A hook that could not be built or created may predate
		// this release, so it is left alone, like the Helm SDK does.
		var failure hookFailure
		if errors.As(err, &failure) {
			r.deleteByPolicy(ctx, hook, PolicyHookFailed)
			r.deleteSucceeded(ctx, hooks[:index], forward)
		}

		return err
	}

	r.deleteSucceeded(ctx, hooks, reverse)

	return nil
}

// hookFailure marks the error of a hook that ran and did not complete, as
// opposed to one that never started.
type hookFailure struct {
	err error
}

func (f hookFailure) Error() string { return f.err.Error() }

func (f hookFailure) Unwrap() error { return f.err }

// runHook applies the before-hook-creation policy, creates the hook, and
// waits for it to complete.
func (r *Runner) runHook(ctx context.Context, hook render.Hook, event string) error {
	name := describe(hook)

	r.log.Info("running hook", "event", event, "hook", name, "weight", hook.Weight)

	if err := r.deleteByPolicyErr(ctx, hook, PolicyBeforeHookCreation); err != nil {
		return fmt.Errorf("hookexec: deleting %s before creation: %w", name, err)
	}

	// Helm validates strictly when it creates a hook and leniently when it
	// deletes one, so that a malformed manifest fails early.
	resources, err := r.build(hook, true)
	if err != nil {
		return err
	}

	if _, err := r.client.Create(resources, kube.ClientCreateOptionServerSideApply(r.serverSideApply, false)); err != nil {
		return fmt.Errorf("hookexec: creating %s: %w", name, err)
	}

	waiter, err := r.waiter(ctx)
	if err != nil {
		return fmt.Errorf("hookexec: building waiter for %s: %w", name, err)
	}

	if err := waiter.WatchUntilReady(resources, r.remaining(ctx)); err != nil {
		return hookFailure{err: fmt.Errorf("hookexec: waiting for %s: %w", name, err)}
	}

	r.log.Debug("hook completed", "event", event, "hook", name)

	return nil
}

// order selects the direction of a delete-policy sweep.
type order bool

const (
	// forward matches the Helm cleanup after a failed event.
	forward order = false

	// reverse matches the Helm cleanup after a successful event.
	reverse order = true
)

// deleteSucceeded deletes the hooks carrying the hook-succeeded policy. The
// Helm SDK sweeps backwards after a successful event and forwards when
// cleaning up after a failure.
func (r *Runner) deleteSucceeded(ctx context.Context, hooks []render.Hook, direction order) {
	for i := range hooks {
		index := i
		if direction == reverse {
			index = len(hooks) - 1 - i
		}

		r.deleteByPolicy(ctx, hooks[index], PolicyHookSucceeded)
	}
}

// waiter builds a Helm waiter bound to the context when the client supports
// it, so that a cancelled reconcile stops waiting instead of running to the
// timeout.
func (r *Runner) waiter(ctx context.Context) (kube.Waiter, error) {
	if client, ok := r.client.(kube.InterfaceWaitOptions); ok {
		return client.GetWaiterWithOptions(r.waitStrategy, kube.WithWaitContext(ctx))
	}

	return r.client.GetWaiter(r.waitStrategy)
}

// remaining caps the hook timeout at the deadline of the context.
func (r *Runner) remaining(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return r.timeout
	}

	if until := time.Until(deadline); until < r.timeout {
		return until
	}

	return r.timeout
}

// deleteByPolicy applies a delete policy and logs a failure instead of
// returning it, where the Helm SDK fails the release. A transient failure
// clears itself on the next reconcile, and the hook error is not masked by a
// cleanup error.
//
// A permanent failure, on an RBAC denial or a stuck finalizer, leaves the
// hook behind while the run reports success. The next reconcile applies over
// the live object and answers 422 once a chart upgrade changes the immutable
// spec of a Job. That 422 names the field, not the denial, so the error
// logged here is the only record of the cause.
func (r *Runner) deleteByPolicy(ctx context.Context, hook render.Hook, policy string) {
	if err := r.deleteByPolicyErr(ctx, hook, policy); err != nil {
		r.log.Error("deleting hook", "error", err, "hook", describe(hook), "policy", policy)
	}
}

// deleteByPolicyErr deletes the hook when it carries the policy, and waits
// for the deletion to complete so that a following create does not race it.
func (r *Runner) deleteByPolicyErr(ctx context.Context, hook render.Hook, policy string) error {
	if !hasPolicy(hook, policy) {
		return nil
	}

	// Deleting a definition would cascade to every custom resource of that
	// kind in the cluster.
	if hook.Object.GetKind() == crdKind {
		return nil
	}

	resources, err := r.build(hook, false)
	if err != nil {
		return err
	}

	if _, errs := r.client.Delete(resources, metav1.DeletePropagationBackground); len(errs) > 0 {
		if err := joinIgnoringMissing(errs); err != nil {
			return err
		}
	}

	waiter, err := r.waiter(ctx)
	if err != nil {
		return err
	}

	return waiter.WaitForDelete(resources, r.remaining(ctx))
}

// build converts a rendered hook back into the resource list the Helm kube
// client operates on. Validation is strict on the create path and lenient on
// the delete path, matching the Helm SDK.
func (r *Runner) build(hook render.Hook, validate bool) (kube.ResourceList, error) {
	manifest, err := k8syaml.Marshal(hook.Object.Object)
	if err != nil {
		return nil, fmt.Errorf("hookexec: encoding %s: %w", describe(hook), err)
	}

	resources, err := r.client.Build(strings.NewReader(string(manifest)), validate)
	if err != nil {
		return nil, fmt.Errorf("hookexec: building %s: %w", describe(hook), err)
	}

	return resources, nil
}

// hasPolicy reports whether the hook carries the delete policy, defaulting an
// unannotated hook to before-hook-creation like the Helm SDK.
//
// One degenerate annotation diverges: the renderer drops empty policy values,
// so a hook rendering "helm.sh/hook-delete-policy" as the empty string takes
// the default here, where the SDK suppresses it and never deletes the hook.
func hasPolicy(hook render.Hook, policy string) bool {
	if len(hook.DeletePolicies) == 0 {
		return policy == PolicyBeforeHookCreation
	}

	return slices.Contains(hook.DeletePolicies, policy)
}

// describe names a hook for logs and errors.
func describe(hook render.Hook) string {
	object := hook.Object

	return fmt.Sprintf("%s %s/%s", object.GetKind(), namespaceOf(object), object.GetName())
}

// namespaceOf reports the namespace of an object, or "_" when it carries
// none, so that a description never collapses to an empty segment.
func namespaceOf(object *unstructured.Unstructured) string {
	if namespace := object.GetNamespace(); namespace != "" {
		return namespace
	}

	return "_"
}

// joinIgnoringMissing drops the errors that mean "there was nothing to
// delete", the expected outcome of a before-hook-creation delete on a first
// install. The Helm client absorbs those itself, but other kube.Interface
// implementations, fakes included, surface them.
func joinIgnoringMissing(errs []error) error {
	kept := make([]error, 0, len(errs))

	for _, err := range errs {
		if apierrors.IsNotFound(err) || errors.Is(err, kube.ErrNoObjectsVisited) {
			continue
		}

		kept = append(kept, err)
	}

	return errors.Join(kept...)
}
