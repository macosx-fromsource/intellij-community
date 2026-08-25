package gitlabcore

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// The environment variables the migration choreography toggles, and the init
// container it toggles the schema bypass on. They match the names the GitLab
// image and the v1beta1 controller (controllers/upgrade.go) use.
const (
	// skipPostDeploymentMigrationsEnv makes the migrations Job run the regular
	// migrations only, leaving the post-deployment ones for after the new pods
	// are up. The pre-migrations Job carries it; the full one does not.
	skipPostDeploymentMigrationsEnv = "SKIP_POST_DEPLOYMENT_MIGRATIONS"

	// bypassSchemaVersionEnv lets the new pods start against the old schema,
	// before the post-deployment migrations have run. It is removed once they
	// have, which rolls the pods once more so they run with the check back on.
	bypassSchemaVersionEnv = "BYPASS_SCHEMA_VERSION" //nolint:gosec // an env var name, not a credential

	// dependenciesInitContainer is the init container of the webservice and
	// sidekiq pods that gates startup on the schema version.
	dependenciesInitContainer = "dependencies"
)

const (
	// maxJobNameLength bounds a derived migrations Job name. A Job stamps its
	// name onto its pods as the `job-name` label, and a label value cannot
	// exceed 63 characters, so the derived names have to fit within it.
	maxJobNameLength = 63

	// preMigrationsSuffix marks the pre-migrations variant of the derived Job.
	preMigrationsSuffix = "-pre"
)

// gatedWorkloads selects the Deployments a zero-downtime upgrade holds back: the
// webservice and sidekiq, whose old pods keep serving while the migrations run.
var gatedWorkloads objects.Predicate = objects.And(
	objects.ByKind("Deployment"),
	objects.Or(objects.ByComponent("webservice"), objects.ByComponent("sidekiq")),
)

// migrationsJobSelector selects the migrations Job the chart renders as a
// regular object, the one the choreography splits into a pre and a full run.
var migrationsJobSelector objects.Predicate = objects.And(
	objects.ByKind("Job"),
	objects.ByComponent("migrations"),
)

// isUpgrade reports whether the resource asks for a version above the one it
// runs. A fresh install and a resource already at its target are not upgrades
// and keep the plain apply path. A downgrade is not one either: it is applied
// directly, without the migration choreography, which only moves forward.
func isUpgrade(core *apiv2alpha1.GitLabCore) bool {
	deployed := core.Status.Version
	target := core.Spec.Chart.Version

	if deployed == "" || deployed == target {
		return false
	}

	from, errFrom := semver.NewVersion(deployed)
	to, errTo := semver.NewVersion(target)

	if errFrom != nil || errTo != nil {
		return false
	}

	return to.GreaterThan(from)
}

// reconcileUpgrade carries out one zero-downtime upgrade cycle, to renderVersion,
// which is one minor step toward the target (see nextChartVersion). It is a
// resumable state machine: each pass re-derives the same objects from the render
// and reads the progress back from the cluster, so an unfinished step returns a
// requeue and the next pass continues where this one left off.
//
// The order reproduces the v1beta1 choreography (controllers/zero_downtime_upgrade.go):
// stage the new configuration and non-gated components, hold the webservice and
// sidekiq paused while the pre-migrations run, roll them out with the schema
// check bypassed, run the post-deployment migrations, then drop the bypass and
// record the new version.
func (r *Reconciler) reconcileUpgrade(ctx context.Context, core *apiv2alpha1.GitLabCore, release *render.Result, renderVersion string, log logr.Logger) (ctrl.Result, error) {
	core.Status.Phase = PhaseUpgrading

	gated, nonGated := objects.Partition(release.Objects, gatedWorkloads)
	migrationsJobs, rest := objects.Partition(nonGated, migrationsJobSelector)

	// Stage everything but the gated workloads and the migrations Job on every
	// pass. It is idempotent under server-side apply, carries the new version's
	// configuration the migrations read, and rolls the components the upgrade
	// does not gate.
	if err := r.applyObjectList(ctx, core, rest, log); err != nil {
		setCondition(core, ConditionAvailable, metav1.ConditionFalse, reasonApplyFailed, err.Error())

		return ctrl.Result{}, err
	}

	// A release without a migrations component has nothing to order: apply the
	// gated workloads too and record the version.
	if len(migrationsJobs) == 0 {
		if err := r.applyObjectList(ctx, core, gated, log); err != nil {
			setCondition(core, ConditionAvailable, metav1.ConditionFalse, reasonApplyFailed, err.Error())

			return ctrl.Result{}, err
		}

		return r.completeUpgradeStep(core, renderVersion), nil
	}

	migrationsJob := migrationsJobs[0]

	fullName, preName := migrationsJobNames(migrationsJob.GetName(), upgradeHash(core, renderVersion))

	// Gate 1: run the pre-migrations while the gated workloads stay paused on
	// their new template, so the old pods keep serving.
	preDone, preFailed, err := r.jobSucceeded(ctx, core.Namespace, preName)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !preDone {
		return r.runPreMigrations(ctx, core, gated, migrationsJob, preName, preFailed, log)
	}

	// Whether the post-deployment migrations have run is the marker that splits
	// the rest of the cycle: before, the pods roll out with the schema bypassed;
	// after, the bypass comes off. Reading it up front keeps the two phases
	// apart, so a later pass never puts back a bypass an earlier one removed.
	fullDone, fullFailed, err := r.jobSucceeded(ctx, core.Namespace, fullName)
	if err != nil {
		return ctrl.Result{}, err
	}

	if fullDone {
		return r.finishUpgrade(ctx, core, gated, renderVersion, log)
	}

	// Gate 2: roll the new pods out with the schema check bypassed, and wait for
	// them to become ready before the post-deployment migrations run.
	if err := r.applyGated(ctx, core, gated, unpauseWithBypass, log); err != nil {
		return ctrl.Result{}, err
	}

	ready, pending, err := r.workloadsReadyIn(ctx, core, gated)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !ready {
		setCondition(core, ConditionProgressing, metav1.ConditionTrue, reasonUpgradingRails,
			fmt.Sprintf("upgrading the webservice and sidekiq to %s, waiting for %s", renderVersion, pending))

		return requeueWithDefaultDelay()
	}

	// Gate 3: run the full migrations, which now include the post-deployment
	// ones, against the pods that are up.
	fullJob := deriveMigrationsJob(migrationsJob, fullName)

	if err := r.applyObject(ctx, core, fullJob, log); err != nil {
		return ctrl.Result{}, err
	}

	if fullFailed {
		return r.migrationsFailed(core, fullName), nil
	}

	setCondition(core, ConditionProgressing, metav1.ConditionTrue, reasonRunningPostMigrations,
		fmt.Sprintf("running the post-deployment migrations of %s", renderVersion))

	return requeueWithDefaultDelay()
}

// finishUpgrade drops the schema bypass, which rolls the pods once more so they
// run with the check back on, waits for that roll to finish, and records the
// version. It is the last gate, reached once the post-deployment migrations have
// run, and is idempotent: the bypass is already gone on later passes, so the
// pristine template applies without another roll.
func (r *Reconciler) finishUpgrade(ctx context.Context, core *apiv2alpha1.GitLabCore, gated []*unstructured.Unstructured, renderVersion string, log logr.Logger) (ctrl.Result, error) {
	if err := r.applyGated(ctx, core, gated, finalizeGated, log); err != nil {
		return ctrl.Result{}, err
	}

	ready, pending, err := r.workloadsReadyIn(ctx, core, gated)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !ready {
		setCondition(core, ConditionProgressing, metav1.ConditionTrue, reasonRollingOutWorkloads,
			fmt.Sprintf("finishing the rollout of %s, waiting for %s", renderVersion, pending))

		return requeueWithDefaultDelay()
	}

	return r.completeUpgradeStep(core, renderVersion), nil
}

// runPreMigrations pauses the gated workloads on their new template and runs the
// pre-migrations Job, the first gate of reconcileUpgrade.
func (r *Reconciler) runPreMigrations(ctx context.Context, core *apiv2alpha1.GitLabCore, gated []*unstructured.Unstructured, migrationsJob *unstructured.Unstructured, preName string, preFailed bool, log logr.Logger) (ctrl.Result, error) {
	if err := r.applyGated(ctx, core, gated, pauseGated, log); err != nil {
		return ctrl.Result{}, err
	}

	preJob, err := derivePreMigrationsJob(migrationsJob, preName)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.applyObject(ctx, core, preJob, log); err != nil {
		return ctrl.Result{}, err
	}

	if preFailed {
		return r.migrationsFailed(core, preName), nil
	}

	setCondition(core, ConditionProgressing, metav1.ConditionTrue, reasonRunningPreMigrations,
		"running the pre-deployment migrations")

	return requeueWithDefaultDelay()
}

// completeUpgradeStep records the version this cycle converged to and reports
// whether the target is reached or another minor step remains.
func (r *Reconciler) completeUpgradeStep(core *apiv2alpha1.GitLabCore, renderVersion string) ctrl.Result {
	core.Status.Version = renderVersion

	if renderVersion == core.Spec.Chart.Version {
		core.Status.Phase = PhaseRunning
		setCondition(core, ConditionProgressing, metav1.ConditionFalse, reasonUpgradeComplete,
			fmt.Sprintf("upgraded to %s", renderVersion))
	} else {
		setCondition(core, ConditionProgressing, metav1.ConditionTrue, reasonAdvancingVersion,
			fmt.Sprintf("upgraded to %s, continuing toward %s", renderVersion, core.Spec.Chart.Version))
	}

	result, _ := requeueWithDefaultDelay()

	return result
}

// migrationsFailed records a failed migrations Job. The poll keeps surfacing it
// rather than hot-looping on an error: the Job spec is immutable, so the same
// Job cannot rerun, and recovery is an operator changing the resource, which
// bumps the generation and so the derived Job name.
func (r *Reconciler) migrationsFailed(core *apiv2alpha1.GitLabCore, jobName string) ctrl.Result {
	message := fmt.Sprintf("the migrations Job %q failed; inspect its pods and, once fixed, retry the upgrade", jobName)

	core.Status.Phase = PhaseFailed
	setCondition(core, ConditionProgressing, metav1.ConditionFalse, reasonMigrationsJobFailed, message)
	r.Recorder.Eventf(core, nil, corev1.EventTypeWarning, "MigrationsFailed", "Migrate", "%s", message)

	result, _ := requeueWithDefaultDelay()

	return result
}

// applyGated applies each gated workload through mutate, on a deep copy so the
// render output stays untouched.
func (r *Reconciler) applyGated(ctx context.Context, core *apiv2alpha1.GitLabCore, gated []*unstructured.Unstructured, mutate func(*unstructured.Unstructured) error, log logr.Logger) error {
	for _, obj := range gated {
		prepared := obj.DeepCopy()

		if err := mutate(prepared); err != nil {
			return fmt.Errorf("preparing %s %q: %w", prepared.GetKind(), prepared.GetName(), err)
		}

		if err := r.applyObject(ctx, core, prepared, log); err != nil {
			return err
		}
	}

	return nil
}

// pauseGated stages a workload's new template without rolling it out.
func pauseGated(obj *unstructured.Unstructured) error {
	return objects.SetPaused(obj, true)
}

// unpauseGated resumes a workload and lets the new pods start against the old
// schema, before the post-deployment migrations have run.
func unpauseWithBypass(obj *unstructured.Unstructured) error {
	if err := objects.SetPaused(obj, false); err != nil {
		return err
	}

	return objects.UpsertInitContainerEnv(obj, dependenciesInitContainer, bypassSchemaVersionEnv, "true")
}

// finalizeGated drops the schema bypass, which rolls the pods once more so they
// run with the check back on. The change of the pod template is the roll; no
// separate restart annotation is needed, and none may carry a timestamp, which
// would differ on every pass and roll the pods forever.
func finalizeGated(obj *unstructured.Unstructured) error {
	if err := objects.SetPaused(obj, false); err != nil {
		return err
	}

	return objects.RemoveInitContainerEnv(obj, dependenciesInitContainer, bypassSchemaVersionEnv)
}

// derivePreMigrationsJob returns the pre-migrations variant of the Job: a copy
// renamed and told to skip the post-deployment migrations.
func derivePreMigrationsJob(base *unstructured.Unstructured, name string) (*unstructured.Unstructured, error) {
	job := deriveMigrationsJob(base, name)

	if err := objects.UpsertEnvInAllContainers(job, skipPostDeploymentMigrationsEnv, "true"); err != nil {
		return nil, err
	}

	return job, nil
}

// deriveMigrationsJob returns a renamed copy of the migrations Job. The name
// carries the upgrade hash, so a Job of one upgrade step does not collide with
// an immutable one already applied for another.
func deriveMigrationsJob(base *unstructured.Unstructured, name string) *unstructured.Unstructured {
	job := base.DeepCopy()
	job.SetName(name)

	return job
}

// jobSucceeded reads a live Job and reports whether it has completed and whether
// it has a failed pod. A Job that does not exist yet has neither.
func (r *Reconciler) jobSucceeded(ctx context.Context, namespace, name string) (done, failed bool, err error) {
	live := &unstructured.Unstructured{}
	live.SetGroupVersionKind(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"})

	err = r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, live)

	switch {
	case apierrors.IsNotFound(err):
		return false, false, nil
	case err != nil:
		return false, false, err
	}

	succeeded, _, err := unstructured.NestedInt64(live.Object, "status", "succeeded")
	if err != nil {
		return false, false, fmt.Errorf("reading the succeeded count of Job %q: %w", name, err)
	}

	failures, _, err := unstructured.NestedInt64(live.Object, "status", "failed")
	if err != nil {
		return false, false, fmt.Errorf("reading the failed count of Job %q: %w", name, err)
	}

	return succeeded > 0, failures > 0, nil
}

// migrationsJobNames builds the names of the full and pre-migrations Jobs of one
// upgrade step from the chart's migrations Job name and the upgrade hash. The
// hash separates the Jobs of one step from the next; the base is truncated so
// that both names, the pre one included (the longer of the two), stay within the
// 63-character limit Kubernetes puts on the job-name label the Jobs stamp onto
// their pods. The pre name is the full name plus the "-pre" marker, so the pair
// stays related. Collisions across resources cannot happen even when the base is
// truncated, because the hash folds in the resource UID.
func migrationsJobNames(base, hash string) (fullName, preName string) {
	tail := "-" + hash

	// Reserve room for the hash and the "-pre" marker so both derived names fit.
	budget := maxJobNameLength - len(tail) - len(preMigrationsSuffix)
	if len(base) > budget {
		base = strings.TrimRight(base[:budget], "-")
	}

	fullName = base + tail
	preName = fullName + preMigrationsSuffix

	return fullName, preName
}

// upgradeHash is a short, stable token for the Job names of one upgrade step. It
// folds in the version so consecutive steps of a multi-minor upgrade, which
// share a generation, do not collide, and the uid and generation so a distinct
// upgrade gets fresh names rather than colliding with an immutable Job.
func upgradeHash(core *apiv2alpha1.GitLabCore, renderVersion string) string {
	digest := fnv.New32a()
	_, _ = fmt.Fprintf(digest, "%s-%d-%s", core.GetUID(), core.GetGeneration(), renderVersion)

	return fmt.Sprintf("%08x", digest.Sum32())
}
