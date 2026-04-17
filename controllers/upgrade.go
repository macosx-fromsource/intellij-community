package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/internal"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/component"
)

const (
	gitlabLastRestartAnnotationKey = "gitlab.com/last-restart"
	timeFormat                     = "20060102150405"
	envVarNameBypassSchemaVersion  = "BYPASS_SCHEMA_VERSION" //nolint:gosec // for some reason this is suspected as an exposed credential
	initContainerNameDependencies  = "dependencies"
)

func (r *GitLabReconciler) getDeployment(ctx context.Context, adapter gitlab.Adapter, deploymentName string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	lookupKey := types.NamespacedName{Namespace: adapter.Name().Namespace, Name: deploymentName}

	if err := r.Get(ctx, lookupKey, deployment); err != nil {
		return deployment, fmt.Errorf("unable to get Deployment: %s", err.Error())
	}

	return deployment, nil
}

func (r *GitLabReconciler) unpauseDeployments(ctx context.Context, adapter gitlab.Adapter, deployments []client.Object) error {
	for i := range deployments {
		deployment, err := r.getDeployment(ctx, adapter, deployments[i].GetName())
		if err != nil {
			return err
		}

		if err := adapter.PopulateManagedObjects(deployment); err != nil {
			return err
		}

		deployment.Spec.Paused = false

		// If unpausing during an upgrade, then set BYPASS_SCHEMA_VERSION.
		if adapter.IsUpgrade() {
			addInitContainerEnvVar(deployment, initContainerNameDependencies, envVarNameBypassSchemaVersion, "true")
		}

		err = r.Update(ctx, deployment)
		if err != nil {
			return fmt.Errorf("unable to update deployment %s: %s", deployment.Name, err.Error())
		}
	}

	return nil
}

func (r *GitLabReconciler) unpauseWebserviceDeployments(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	return r.unpauseDeployments(ctx, adapter, gitlabctl.WebserviceDeployments(template))
}

func (r *GitLabReconciler) unpauseSidekiqDeployments(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	return r.unpauseDeployments(ctx, adapter, gitlabctl.SidekiqDeployments(template))
}

func (r *GitLabReconciler) rollingUpdateDeployments(ctx context.Context, adapter gitlab.Adapter, deployments []client.Object) error {
	for i := range deployments {
		deployment, err := r.getDeployment(ctx, adapter, deployments[i].GetName())
		if err != nil {
			return err
		}

		if err := adapter.PopulateManagedObjects(deployment); err != nil {
			return err
		}

		deployment.Spec.Template.Annotations[gitlabLastRestartAnnotationKey] = time.Now().Format(timeFormat)
		removeInitContainerEnvVar(deployment, initContainerNameDependencies, envVarNameBypassSchemaVersion)

		if err := r.Update(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) rollingUpdateWebserviceDeployments(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	return r.rollingUpdateDeployments(ctx, adapter, gitlabctl.WebserviceDeployments(template))
}

func (r *GitLabReconciler) rollingUpdateSidekiqDeployments(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	return r.rollingUpdateDeployments(ctx, adapter, gitlabctl.SidekiqDeployments(template))
}

func (r *GitLabReconciler) reconcileWebserviceAndSidekiqIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template, pause bool) error {
	if adapter.WantsComponent(component.Webservice) {
		if err := r.reconcileWebserviceDeployments(ctx, adapter, template, pause); err != nil {
			return err
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if err := r.reconcileSidekiqDeployments(ctx, adapter, template, pause); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) unpauseWebserviceAndSidekiqIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if adapter.WantsComponent(component.Webservice) {
		if err := r.unpauseWebserviceDeployments(ctx, adapter, template); err != nil {
			return err
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if err := r.unpauseSidekiqDeployments(ctx, adapter, template); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) webserviceAndSidekiqRunningIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if adapter.WantsComponent(component.Webservice) {
		if !r.webserviceRunning(ctx, adapter, template) {
			return fmt.Errorf("webservice has not started fully")
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if !r.sidekiqRunning(ctx, adapter, template) {
			return fmt.Errorf("sidekiq has not started fully")
		}
	}

	return nil
}

func (r *GitLabReconciler) rollingUpdateWebserviceAndSidekiqIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if adapter.WantsComponent(component.Webservice) {
		if err := r.rollingUpdateWebserviceDeployments(ctx, adapter, template); err != nil {
			return err
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if err := r.rollingUpdateSidekiqDeployments(ctx, adapter, template); err != nil {
			return err
		}
	}

	return nil
}

type containerInPlaceOperator = func(container *corev1.Container) error

func applyToContainer(containers []corev1.Container, name string, operator containerInPlaceOperator) error {
	for i := range containers {
		if containers[i].Name == name {
			return operator(&containers[i])
		}
	}

	return nil
}

func indexOfEnvVar(envVars []corev1.EnvVar, name string) int {
	idx := -1

	for i := range envVars {
		if envVars[i].Name == name {
			idx = i
			break
		}
	}

	return idx
}

func addEnvVar(name, value string) containerInPlaceOperator {
	return func(container *corev1.Container) error {
		idx := indexOfEnvVar(container.Env, name)
		if idx < 0 {
			container.Env = append(container.Env,
				corev1.EnvVar{Name: name, Value: value})
		}

		return nil
	}
}

func removeEnvVar(name string) containerInPlaceOperator {
	return func(container *corev1.Container) error {
		for {
			idx := indexOfEnvVar(container.Env, name)
			if idx > -1 {
				container.Env[idx] = container.Env[len(container.Env)-1]
				container.Env = container.Env[:len(container.Env)-1]
			} else {
				break
			}
		}

		return nil
	}
}

func removeInitContainerEnvVar(deployment *appsv1.Deployment, initContainerName, envVarName string) {
	_ = applyToContainer(deployment.Spec.Template.Spec.InitContainers,
		initContainerName, removeEnvVar(envVarName))
}

func addInitContainerEnvVar(deployment *appsv1.Deployment, initContainerName, envVarName, envVarValue string) {
	_ = applyToContainer(deployment.Spec.Template.Spec.InitContainers,
		initContainerName, addEnvVar(envVarName, envVarValue))
}

func (r *GitLabReconciler) scaleDownDeployments(ctx context.Context, adapter gitlab.Adapter, deployments []client.Object) error {
	zero := int32(0)

	for _, d := range deployments {
		deployment, err := r.getDeployment(ctx, adapter, d.GetName())
		if err != nil {
			return err
		}

		if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
			continue
		}

		deployment.Spec.Replicas = &zero

		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("unable to scale down deployment %s: %w", deployment.Name, err)
		}
	}

	return nil
}

func (r *GitLabReconciler) scaleDownWebserviceAndSidekiqIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if adapter.WantsComponent(component.Webservice) {
		if err := r.scaleDownDeployments(ctx, adapter, gitlabctl.WebserviceDeployments(template)); err != nil {
			return err
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if err := r.scaleDownDeployments(ctx, adapter, gitlabctl.SidekiqDeployments(template)); err != nil {
			return err
		}
	}

	return nil
}

func (r *GitLabReconciler) deploymentsScaledDown(ctx context.Context, adapter gitlab.Adapter, deployments []client.Object) bool {
	for _, d := range deployments {
		deployment, err := r.getDeployment(ctx, adapter, d.GetName())
		if err != nil {
			r.Log.V(1).Info("unable to check if deployment is scaled down",
				"deployment", d.GetName(), "error", err)

			return false
		}

		if deployment.Status.Replicas != 0 || deployment.Status.ReadyReplicas != 0 {
			return false
		}
	}

	return true
}

func (r *GitLabReconciler) webserviceAndSidekiqScaledDownIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template) bool {
	if adapter.WantsComponent(component.Webservice) {
		if !r.deploymentsScaledDown(ctx, adapter, gitlabctl.WebserviceDeployments(template)) {
			return false
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if !r.deploymentsScaledDown(ctx, adapter, gitlabctl.SidekiqDeployments(template)) {
			return false
		}
	}

	return true
}

func (r *GitLabReconciler) deploymentsHaveReadyReplica(ctx context.Context, adapter gitlab.Adapter, deployments []client.Object) bool {
	for _, d := range deployments {
		deployment, err := r.getDeployment(ctx, adapter, d.GetName())
		if err != nil {
			r.Log.V(1).Info("unable to check if deployment has a ready replica",
				"deployment", d.GetName(), "error", err)

			return false
		}

		if deployment.Status.ReadyReplicas == 0 {
			return false
		}
	}

	return true
}

func (r *GitLabReconciler) webserviceAndSidekiqHaveReadyReplicaIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template) bool {
	if adapter.WantsComponent(component.Webservice) {
		if !r.deploymentsHaveReadyReplica(ctx, adapter, gitlabctl.WebserviceDeployments(template)) {
			return false
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if !r.deploymentsHaveReadyReplica(ctx, adapter, gitlabctl.SidekiqDeployments(template)) {
			return false
		}
	}

	return true
}

func (r *GitLabReconciler) restoreDeploymentReplicas(ctx context.Context, adapter gitlab.Adapter, deployments []client.Object) error {
	for _, d := range deployments {
		templateDeployment, err := internal.AsDeployment(d)
		if err != nil {
			return err
		}

		if templateDeployment.Spec.Replicas != nil && *templateDeployment.Spec.Replicas == 0 {
			continue
		}

		deployment, err := r.getDeployment(ctx, adapter, d.GetName())
		if err != nil {
			return err
		}

		if err := adapter.PopulateManagedObjects(deployment); err != nil {
			return err
		}

		deployment.Spec.Replicas = templateDeployment.Spec.Replicas

		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("unable to restore replicas for deployment %s: %s", deployment.Name, err.Error())
		}
	}

	return nil
}

func (r *GitLabReconciler) restoreWebserviceAndSidekiqReplicasIfEnabled(ctx context.Context, adapter gitlab.Adapter, template helm.Template) error {
	if adapter.WantsComponent(component.Webservice) {
		if err := r.restoreDeploymentReplicas(ctx, adapter, gitlabctl.WebserviceDeployments(template)); err != nil {
			return err
		}
	}

	if adapter.WantsComponent(component.Sidekiq) {
		if err := r.restoreDeploymentReplicas(ctx, adapter, gitlabctl.SidekiqDeployments(template)); err != nil {
			return err
		}
	}

	return nil
}
