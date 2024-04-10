package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/status"
)

// reconcileGitLabStatus runs at the end of the reconcile loop.
//
// It handles:
//   - calculating whether or not the reconcile loop should requeue
//     (if Webservice and Sidekiq are not yet running)
//   - setting the Status Condition on the GitLab custom resource
//   - recording the new version on the GitLab custom resource
func (r *GitLabReconciler) reconcileGitLabStatus(ctx context.Context, adapter gitlab.Adapter, template helm.Template, log logr.Logger) (ctrl.Result, error) {
	const (
		messageReady    = "GitLab is running and available to accept requests"
		messageNotReady = "Webservice and/or Sidekiq not yet Running"
	)

	resultRequeue := ctrl.Result{RequeueAfter: 10 * time.Second}
	resultNoRequeue := ctrl.Result{}
	result := resultNoRequeue

	if r.sidekiqAndWebserviceRunning(ctx, adapter, template) {
		if err := r.setStatusCondition(ctx, adapter, status.ConditionAvailable, true, messageReady); err != nil {
			return result, err
		}

		log.Info(messageReady)
	} else {
		result = resultRequeue

		log.Info(messageNotReady)
	}

	// Set the version regardless of whether Sidekiq and Webservice are fully running to
	// ensure we don't trigger the upgrade logic again on the next iteration.
	adapter.RecordVersion()

	if err := r.Status().Update(ctx, adapter.Origin()); err != nil {
		return result, err
	}

	time.Sleep(5 * time.Second)

	return result, nil
}

// Same check as used in the deployment utils in upstream Kubernetes
// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/deployment/util/deployment_util.go#L722
func deploymentComplete(deployment *appsv1.Deployment, newStatus *appsv1.DeploymentStatus) bool {
	return newStatus.UpdatedReplicas == *(deployment.Spec.Replicas) &&
		newStatus.Replicas == *(deployment.Spec.Replicas) &&
		newStatus.AvailableReplicas == *(deployment.Spec.Replicas) &&
		newStatus.ObservedGeneration >= deployment.Generation
}

func (r *GitLabReconciler) componentRunning(ctx context.Context, adapter gitlab.Adapter, deployments []client.Object) bool {
	running := true

	for _, deployment := range deployments {
		// This is only for safeguarding
		if deployment.GetObjectKind().GroupVersionKind().Kind != gitlabctl.DeploymentKind {
			continue
		}

		webservice := &appsv1.Deployment{}
		key := types.NamespacedName{
			Name:      deployment.GetName(),
			Namespace: adapter.Name().Namespace,
		}

		err := r.Get(ctx, key, webservice)
		if err != nil || !deploymentComplete(webservice, &webservice.Status) {
			running = false
			break
		}
	}

	return running
}

func (r *GitLabReconciler) sidekiqRunning(ctx context.Context, adapter gitlab.Adapter, template helm.Template) bool {
	return r.componentRunning(ctx, adapter, gitlabctl.SidekiqDeployments(template))
}

func (r *GitLabReconciler) webserviceRunning(ctx context.Context, adapter gitlab.Adapter, template helm.Template) bool {
	return r.componentRunning(ctx, adapter, gitlabctl.WebserviceDeployments(template))
}

func (r *GitLabReconciler) sidekiqAndWebserviceRunning(ctx context.Context, adapter gitlab.Adapter, template helm.Template) bool {
	return r.sidekiqRunning(ctx, adapter, template) && r.webserviceRunning(ctx, adapter, template)
}

func (r *GitLabReconciler) setStatusCondition(ctx context.Context, adapter gitlab.Adapter, reason gitlab.ConditionType, status bool, message string) error {
	statusValue := metav1.ConditionFalse
	if status {
		statusValue = metav1.ConditionTrue
	}

	adapter.SetCondition(metav1.Condition{
		Type:    reason.Name(),
		Status:  statusValue,
		Reason:  reason.Name(),
		Message: message,
	})

	return r.Status().Update(ctx, adapter.Origin())
}
