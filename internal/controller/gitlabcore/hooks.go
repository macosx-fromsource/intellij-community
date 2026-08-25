package gitlabcore

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-logr/logr"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/hookexec"
)

// hookEventPreInstall is the only event the reconciler runs. Every render is
// composed as an install, and nothing in the chart fires on post-install once
// the ingress subcharts are overridden off.
const hookEventPreInstall = "pre-install"

// hooksAreCurrent reports whether the hooks have already run for the release the
// resource describes now.
//
// Hooks belong to an operation on a release, not to a pass of the loop. The
// shared secrets Job carries before-hook-creation, so running the event again
// deletes the Job, recreates it, and waits for a pod to complete, which on the
// continuous requeue would happen for as long as the instance exists.
//
// ConditionInitialized carries the generation whose hooks completed, and the
// generation covers the whole specification. Capabilities are left out: they
// change what the chart renders without changing the generation, but they do not
// decide the secrets the hooks generate.
//
// A multi-minor upgrade holds one generation across every intermediate step, so
// the hooks run once, for the first intermediate render, and are skipped for the
// later ones. That is safe because the only pre-install hook the Operator runs
// is the chart's shared-secrets Job, which is idempotent: it generates a secret
// only when it is missing, so the secrets an instance already has carry across
// the minors and no later minor's hooks add one the earlier run did not. The
// RBAC hooks are never applied here in any case (see runHooks). Were a future
// minor to require a brand-new secret, its hooks would have to run per render
// version, keyed on the deployed version rather than the generation.
func hooksAreCurrent(core *apiv2alpha1.GitLabCore) bool {
	condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionInitialized)

	return condition != nil &&
		condition.Status == metav1.ConditionTrue &&
		condition.ObservedGeneration == core.Generation
}

// runHooks runs the hooks of one event against the cluster.
//
// The hooks are what make a release work at all: the GitLab chart renders no
// Secret and generates every one of them from a pre-install hook, so a release
// whose hooks never ran has no credentials and cannot start. The pre-install
// event runs before the objects are applied and the post-install event after,
// like a Helm install.
//
// A hook that neverApplied matches is skipped, exactly as a rendered object is.
// The chart declares its RBAC as hooks too, and the Operator has no permission
// to create it: a ServiceAccount that could grant permissions could grant any of
// them. The shared secrets Job does not need that RBAC, because it runs under
// the ServiceAccount of the Operator, which the Operator installation grants
// what the Job needs. The v1beta1 controller reaches the same result by applying
// only the ConfigMap and the Job of that component.
//
// Running the same event again is expected, because a reconcile repeats. The
// executor honors the before-hook-creation policy, which deletes a completed
// Job before recreating it; a Job spec is immutable, so nothing else could
// rerun one.
//
// The call blocks until every hook of the event has completed. The shared
// secrets Job dominates the first reconcile of an instance, and the reconciler
// holds its worker for that time.
func (r *Reconciler) runHooks(ctx context.Context, core *apiv2alpha1.GitLabCore, result *render.Result, event string, log logr.Logger) error {
	hooks, skipped := applicableHooks(result.HooksFor(event))

	if skipped > 0 {
		log.Info("skipping the RBAC hooks of the chart, which the Operator never applies",
			"event", event, "count", skipped,
			"hint", "have the cluster administrator provision them")
	}

	if len(hooks) == 0 {
		return nil
	}

	log.V(1).Info("running chart hooks", "event", event, "count", len(hooks))

	// The client is bound to the namespace of the release, which is what a hook
	// inherits when its manifest omits one. Left to the client getter, that
	// namespace is the one of the Operator in-cluster, and the hook would be
	// created there without an error being reported.
	client, err := hookexec.NewClient(r.RESTClientGetter, core.Namespace)
	if err != nil {
		return err
	}

	runner := hookexec.New(client,
		hookexec.WithTimeout(r.HookTimeout),
		hookexec.WithLogger(slog.New(logr.ToSlogHandler(log))))

	if err := runner.RunHooks(ctx, hooks, event); err != nil {
		return fmt.Errorf("running the %s hooks: %w", event, err)
	}

	return nil
}

// applicableHooks drops the hooks the Operator never applies, keeping the
// execution order of the rest, and reports how many were dropped.
func applicableHooks(hooks []render.Hook) ([]render.Hook, int) {
	applicable := make([]render.Hook, 0, len(hooks))
	skipped := 0

	for _, hook := range hooks {
		if neverApplied(hook.Object) {
			skipped++

			continue
		}

		applicable = append(applicable, hook)
	}

	return applicable, skipped
}
