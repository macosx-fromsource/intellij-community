package gitlabcore

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
)

var _ = Describe("derivePreMigrationsJob", func() {
	base := mockMigrationsJob()

	pre, err := derivePreMigrationsJob(base, "gitlab-migrations-abc-pre")

	It("renames the Job and skips the post-deployment migrations on every container", func() {
		Expect(err).To(BeNil())
		Expect(pre.GetName()).To(Equal("gitlab-migrations-abc-pre"))

		env := containerEnvValue(pre, "migrations", skipPostDeploymentMigrationsEnv)
		Expect(env).To(Equal("true"))
	})

	It("leaves the rendered Job untouched", func() {
		Expect(base.GetName()).To(Equal("gitlab-migrations"))
		Expect(containerEnvValue(base, "migrations", skipPostDeploymentMigrationsEnv)).To(BeEmpty())
	})
})

var _ = Describe("deriveMigrationsJob", func() {
	base := mockMigrationsJob()

	full := deriveMigrationsJob(base, "gitlab-migrations-abc")

	It("renames the Job and runs the post-deployment migrations", func() {
		Expect(full.GetName()).To(Equal("gitlab-migrations-abc"))
		Expect(containerEnvValue(full, "migrations", skipPostDeploymentMigrationsEnv)).To(BeEmpty())
	})
})

var _ = Describe("upgradeHash", func() {
	core := &apiv2alpha1.GitLabCore{ObjectMeta: metav1.ObjectMeta{UID: "uid-1", Generation: 3}}

	It("is stable for the same resource and version", func() {
		Expect(upgradeHash(core, "10.1.6")).To(Equal(upgradeHash(core, "10.1.6")))
	})

	It("differs across the steps of a multi-minor upgrade", func() {
		Expect(upgradeHash(core, "10.1.6")).NotTo(Equal(upgradeHash(core, "10.2.4")))
	})
})

var _ = Describe("jobSucceeded", func() {
	When("the Job has completed", func() {
		reconciler := mockReconciler(mockJobWithStatus("done", 1, 0))

		It("reports it done and not failed", func() {
			done, failed, err := reconciler.jobSucceeded(context.TODO(), testNamespace, "done")

			Expect(err).To(BeNil())
			Expect(done).To(BeTrue())
			Expect(failed).To(BeFalse())
		})
	})

	When("the Job has a failed pod", func() {
		reconciler := mockReconciler(mockJobWithStatus("broken", 0, 1))

		It("reports it failed and not done", func() {
			done, failed, err := reconciler.jobSucceeded(context.TODO(), testNamespace, "broken")

			Expect(err).To(BeNil())
			Expect(done).To(BeFalse())
			Expect(failed).To(BeTrue())
		})
	})

	When("the Job does not exist yet", func() {
		reconciler := mockReconciler()

		It("reports it neither done nor failed", func() {
			done, failed, err := reconciler.jobSucceeded(context.TODO(), testNamespace, "absent")

			Expect(err).To(BeNil())
			Expect(done).To(BeFalse())
			Expect(failed).To(BeFalse())
		})
	})
})

var _ = Describe("reconcileUpgrade", func() {
	When("the pre-migrations have not run yet", func() {
		core := &apiv2alpha1.GitLabCore{
			ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: testNamespace, UID: "uid-1", Generation: 2},
		}
		core.Spec.Chart.Version = "10.1.6"

		reconciler := mockReconciler()

		webservice := mockGatedDeployment("webservice", 2)
		sidekiq := mockGatedDeployment("sidekiq", 1)
		migrations := mockMigrationsJob()
		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")
		configMap.SetNamespace(testNamespace)

		release := &render.Result{Objects: []*unstructured.Unstructured{webservice, sidekiq, migrations, configMap}}

		result, err := reconciler.reconcileUpgrade(context.TODO(), core, release, "10.1.6", GinkgoLogr)

		fullName, preName := migrationsJobNames(migrations.GetName(), upgradeHash(core, "10.1.6"))

		It("requeues while it waits, reporting the pre-migrations", func() {
			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionProgressing)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal(reasonRunningPreMigrations))
		})

		It("holds the gated workloads paused so their old pods keep serving", func() {
			live := &appsv1.Deployment{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: "webservice"}, live)).To(Succeed())

			Expect(live.Spec.Paused).To(BeTrue())
		})

		It("runs the pre-migrations Job, skipping the post-deployment migrations", func() {
			live := &batchv1.Job{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: preName}, live)).To(Succeed())

			Expect(live.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			Expect(live.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
				HaveField("Name", skipPostDeploymentMigrationsEnv)))
		})

		It("does not run the full migrations Job before the deployment rolls out", func() {
			live := &batchv1.Job{}
			err := reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: fullName}, live)

			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("reconcileUpgrade past the pre-migrations", func() {
	// upgradeScenario stages a release whose pre-migrations have completed and
	// whose gated Deployments are in the given rollout state, and runs one
	// upgrade pass over it.
	upgradeScenario := func(renderVersion string, rolledOut bool) (*Reconciler, *apiv2alpha1.GitLabCore, ctrl.Result, error) {
		core := &apiv2alpha1.GitLabCore{
			ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: testNamespace, UID: "uid-1", Generation: 2},
		}
		core.Spec.Chart.Version = renderVersion

		migrations := mockMigrationsJob()
		_, preName := migrationsJobNames(migrations.GetName(), upgradeHash(core, renderVersion))

		reconciler := mockReconciler(
			mockJobWithStatus(preName, 1, 0),
			liveGatedDeployment("webservice", 2, rolledOut),
			liveGatedDeployment("sidekiq", 1, rolledOut),
		)

		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")
		configMap.SetNamespace(testNamespace)

		release := &render.Result{Objects: []*unstructured.Unstructured{
			mockGatedDeployment("webservice", 2),
			mockGatedDeployment("sidekiq", 1),
			migrations,
			configMap,
		}}

		result, err := reconciler.reconcileUpgrade(context.TODO(), core, release, renderVersion, GinkgoLogr)

		return reconciler, core, result, err
	}

	fullJobName := func(core *apiv2alpha1.GitLabCore, renderVersion string) string {
		fullName, _ := migrationsJobNames(testMigrationsJobName, upgradeHash(core, renderVersion))

		return fullName
	}

	When("the new pods have not rolled out yet", func() {
		reconciler, core, result, err := upgradeScenario("10.1.6", false)

		It("rolls the gated workloads out with the schema bypass and waits", func() {
			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionProgressing)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal(reasonUpgradingRails))

			value, present := liveDeploymentEnv(reconciler, "webservice", bypassSchemaVersionEnv)
			Expect(present).To(BeTrue())
			Expect(value).To(Equal("true"))
		})

		It("does not run the post-deployment migrations before the new pods are ready", func() {
			live := &batchv1.Job{}
			err := reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: fullJobName(core, "10.1.6")}, live)

			Expect(err).To(HaveOccurred())
		})
	})

	When("the new pods are ready", func() {
		reconciler, core, result, err := upgradeScenario("10.1.6", true)

		It("runs the full migrations, reporting the post-deployment migrations", func() {
			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionProgressing)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Reason).To(Equal(reasonRunningPostMigrations))
		})

		It("runs the full migrations Job, keeping the post-deployment migrations on", func() {
			live := &batchv1.Job{}
			Expect(reconciler.Get(context.TODO(),
				types.NamespacedName{Namespace: testNamespace, Name: fullJobName(core, "10.1.6")}, live)).To(Succeed())

			Expect(live.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			Expect(live.Spec.Template.Spec.Containers[0].Env).NotTo(ContainElement(
				HaveField("Name", skipPostDeploymentMigrationsEnv)))
		})

		It("does not record the version until the post-deployment migrations complete", func() {
			Expect(core.Status.Version).To(BeEmpty())
		})
	})

	When("the post-deployment migrations have completed", func() {
		core := &apiv2alpha1.GitLabCore{
			ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: testNamespace, UID: "uid-1", Generation: 2},
		}
		core.Spec.Chart.Version = "10.1.6"

		migrations := mockMigrationsJob()
		fullName, preName := migrationsJobNames(migrations.GetName(), upgradeHash(core, "10.1.6"))

		reconciler := mockReconciler(
			mockJobWithStatus(preName, 1, 0),
			mockJobWithStatus(fullName, 1, 0),
			liveGatedDeployment("webservice", 2, true),
			liveGatedDeployment("sidekiq", 1, true),
		)

		// Simulate the earlier pass that rolled the pods out with the schema
		// bypass, so the controller's own field manager owns the bypass env and
		// finishUpgrade can drop it, the way a real sequence of passes would.
		gated := []*unstructured.Unstructured{mockGatedDeployment("webservice", 2), mockGatedDeployment("sidekiq", 1)}
		Expect(reconciler.applyGated(context.TODO(), core, gated, unpauseWithBypass, GinkgoLogr)).To(Succeed())

		_, seeded := liveDeploymentEnv(reconciler, "webservice", bypassSchemaVersionEnv)
		Expect(seeded).To(BeTrue())

		configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")
		configMap.SetNamespace(testNamespace)

		release := &render.Result{Objects: []*unstructured.Unstructured{
			mockGatedDeployment("webservice", 2),
			mockGatedDeployment("sidekiq", 1),
			migrations,
			configMap,
		}}

		result, err := reconciler.reconcileUpgrade(context.TODO(), core, release, "10.1.6", GinkgoLogr)

		It("drops the schema bypass off the gated workloads", func() {
			Expect(err).To(BeNil())

			_, present := liveDeploymentEnv(reconciler, "webservice", bypassSchemaVersionEnv)
			Expect(present).To(BeFalse())
		})

		It("records the version and reports the upgrade complete", func() {
			Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))
			Expect(core.Status.Version).To(Equal("10.1.6"))
			Expect(core.Status.Phase).To(Equal(PhaseRunning))

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionProgressing)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal(reasonUpgradeComplete))
		})
	})
})

var _ = Describe("reconcileUpgrade of a release without migrations", func() {
	core := &apiv2alpha1.GitLabCore{
		ObjectMeta: metav1.ObjectMeta{Name: releaseName, Namespace: testNamespace, UID: "uid-1", Generation: 2},
	}
	core.Spec.Chart.Version = "10.1.6"

	reconciler := mockReconciler()

	configMap := mockObject("v1", "ConfigMap", "gitlab-configmap")
	configMap.SetNamespace(testNamespace)

	release := &render.Result{Objects: []*unstructured.Unstructured{
		mockGatedDeployment("webservice", 2),
		configMap,
	}}

	result, err := reconciler.reconcileUpgrade(context.TODO(), core, release, "10.1.6", GinkgoLogr)

	It("applies the gated workloads unpaused and records the version", func() {
		Expect(err).To(BeNil())
		Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))
		Expect(core.Status.Version).To(Equal("10.1.6"))

		live := &appsv1.Deployment{}
		Expect(reconciler.Get(context.TODO(),
			types.NamespacedName{Namespace: testNamespace, Name: "webservice"}, live)).To(Succeed())

		Expect(live.Spec.Paused).To(BeFalse())
	})
})

var _ = Describe("completeUpgradeStep", func() {
	reconciler := mockReconciler()

	When("a minor step short of the target converges", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Chart.Version = "10.2.4"

		result := reconciler.completeUpgradeStep(core, "10.1.6")

		It("records the step and keeps progressing toward the target", func() {
			Expect(core.Status.Version).To(Equal("10.1.6"))
			Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionProgressing)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(reasonAdvancingVersion))
		})
	})

	When("the final step reaches the target", func() {
		core := &apiv2alpha1.GitLabCore{}
		core.Spec.Chart.Version = "10.2.4"

		result := reconciler.completeUpgradeStep(core, "10.2.4")

		It("records the version, runs, and stops progressing", func() {
			Expect(core.Status.Version).To(Equal("10.2.4"))
			Expect(core.Status.Phase).To(Equal(PhaseRunning))
			Expect(result.RequeueAfter).To(Equal(defaultRequeueDelay))

			condition := apimeta.FindStatusCondition(core.Status.Conditions, ConditionProgressing)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal(reasonUpgradeComplete))
		})
	})
})

// containerEnvValue reads the value of an env variable on the named container of
// a workload's pod template, or the empty string when it is absent.
func containerEnvValue(obj *unstructured.Unstructured, containerName, envName string) string {
	return envValueIn(obj, "containers", containerName, envName)
}

// initContainerEnvValue is containerEnvValue for the init containers, which is
// where the schema-version bypass rides.
func initContainerEnvValue(obj *unstructured.Unstructured, containerName, envName string) string {
	return envValueIn(obj, "initContainers", containerName, envName)
}

// envValueIn reads the value of an env variable on the named container in the
// given list ("containers" or "initContainers") of a pod template, or the empty
// string when it is absent.
func envValueIn(obj *unstructured.Unstructured, list, containerName, envName string) string {
	containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", list)

	for _, item := range containers {
		container, ok := item.(map[string]interface{})
		if !ok || container["name"] != containerName {
			continue
		}

		env, _, _ := unstructured.NestedSlice(container, "env")

		for _, entry := range env {
			variable, ok := entry.(map[string]interface{})
			if ok && variable["name"] == envName {
				value, _ := variable["value"].(string)

				return value
			}
		}
	}

	return ""
}

// liveGatedDeployment builds a gated Deployment already present in the cluster,
// with a rollout status the reconcile reads back: rolledOut stamps every
// replica as updated to the new template, and a false one leaves the old
// replicas ready but none updated, the mid-rollout state. It is seeded rather
// than left to the apply so the status survives regardless of how the fake
// client treats the status subresource.
func liveGatedDeployment(component string, replicas int64, rolledOut bool) *unstructured.Unstructured {
	deployment := mockGatedDeployment(component, replicas)

	updated := int64(0)
	if rolledOut {
		updated = replicas
	}

	Expect(unstructured.SetNestedField(deployment.Object, observedGenerationSentinel, "status", "observedGeneration")).To(Succeed())

	for _, field := range []string{"readyReplicas", "availableReplicas", "replicas"} {
		Expect(unstructured.SetNestedField(deployment.Object, replicas, "status", field)).To(Succeed())
	}

	Expect(unstructured.SetNestedField(deployment.Object, updated, "status", "updatedReplicas")).To(Succeed())

	return deployment
}

// liveDeploymentEnv reads an env variable off a live gated Deployment's
// dependencies init container.
func liveDeploymentEnv(reconciler *Reconciler, name, envName string) (string, bool) {
	live := &unstructured.Unstructured{}
	live.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))

	Expect(reconciler.Get(context.TODO(),
		types.NamespacedName{Namespace: testNamespace, Name: name}, live)).To(Succeed())

	value := initContainerEnvValue(live, dependenciesInitContainer, envName)

	return value, value != ""
}

// mockGatedDeployment builds a rendered webservice or sidekiq Deployment, with
// the dependencies init container the choreography sets the schema bypass on.
func mockGatedDeployment(component string, replicas int64) *unstructured.Unstructured {
	deployment := mockObject("apps/v1", "Deployment", component)
	deployment.SetNamespace(testNamespace)
	deployment.SetLabels(map[string]string{"app": component})

	_ = unstructured.SetNestedField(deployment.Object, replicas, "spec", "replicas")

	_ = unstructured.SetNestedSlice(deployment.Object, []interface{}{
		map[string]interface{}{"name": dependenciesInitContainer, "image": "toolbox"},
	}, "spec", "template", "spec", "initContainers")

	_ = unstructured.SetNestedSlice(deployment.Object, []interface{}{
		map[string]interface{}{"name": component, "image": component},
	}, "spec", "template", "spec", "containers")

	return deployment
}

// testMigrationsJobName is the chart's migrations Job name the tests derive the
// pre and full upgrade Job names from.
const testMigrationsJobName = "gitlab-migrations"

// mockMigrationsJob builds a rendered migrations Job with a single container.
func mockMigrationsJob() *unstructured.Unstructured {
	job := mockObject("batch/v1", "Job", testMigrationsJobName)
	job.SetNamespace(testNamespace)
	job.SetLabels(map[string]string{"app": "migrations"})

	_ = unstructured.SetNestedSlice(job.Object, []interface{}{
		map[string]interface{}{"name": "migrations", "image": "toolbox"},
	}, "spec", "template", "spec", "containers")

	return job
}

// mockJobWithStatus builds a live Job with a succeeded and a failed count.
func mockJobWithStatus(name string, succeeded, failed int32) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: name},
		Status:     batchv1.JobStatus{Succeeded: succeeded, Failed: failed},
	}
}
