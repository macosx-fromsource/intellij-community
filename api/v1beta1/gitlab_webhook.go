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

package v1beta1

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

// log is for logging in this package.
var gitlablog = logf.Log.WithName("gitlab-resource")

// SetupWebhookWithManager adds webhook to the controller runtime Manager.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &GitLab{}).
		WithValidator(&GitLabCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-apps-gitlab-com-v1beta1-gitlab,mutating=false,failurePolicy=fail,groups=apps.gitlab.com,resources=gitlabs,versions=v1beta1,name=vgitlab.kb.io,admissionReviewVersions=v1,sideEffects=None

type GitLabCustomValidator struct{}

var _ admission.Validator[*GitLab] = &GitLabCustomValidator{}

// ValidateUpdate validates create request for GitLab resources.
func (r *GitLabCustomValidator) ValidateCreate(ctx context.Context, gitlab *GitLab) (warnings admission.Warnings, err error) {
	gitlablog.Info("validate create", "name", gitlab.GetName())

	if validateErr := r.validateChartVersion(gitlab); validateErr != nil {
		err = newError(gitlab.GetName(), validateErr)
		return
	}

	return
}

// ValidateUpdate validates update request for GitLab resources.
func (r *GitLabCustomValidator) ValidateUpdate(ctx context.Context, _, gitlab *GitLab) (warnings admission.Warnings, err error) {
	gitlablog.Info("validate update", "name", gitlab.GetName())

	if validateErr := r.validateChartVersion(gitlab); validateErr != nil {
		err = newError(gitlab.GetName(), validateErr)
		return
	}

	if validateErr := r.validateUpgradePath(gitlab); validateErr != nil {
		err = newError(gitlab.GetName(), validateErr)
		return
	}

	return
}

// ValidateDelete validates delete request for GitLab resources.
func (r *GitLabCustomValidator) ValidateDelete(ctx context.Context, gitlab *GitLab) (warnings admission.Warnings, err error) {
	gitlablog.Info("validate delete", "name", gitlab.GetName())

	return
}

func (r *GitLabCustomValidator) validateChartVersion(obj *GitLab) *field.Error {
	key := field.NewPath("spec").Child("chart").Child("version")
	value := obj.Spec.Chart.Version

	if value == "" {
		return field.Invalid(key, value, "chart version must be configured")
	}

	if _, err := helm.ChartVersionSupported(obj.Spec.Chart.Version); err != nil {
		return field.Invalid(key, value, err.Error())
	}

	return nil
}

func (r *GitLabCustomValidator) validateUpgradePath(obj *GitLab) *field.Error {
	currentVer := obj.Status.Version
	currentSemver, err := semver.NewVersion(currentVer)

	if err != nil {
		return nil
	}

	key := field.NewPath("spec").Child("chart").Child("version")
	targetVer := obj.Spec.Chart.Version
	targetSemver, err := semver.NewVersion(targetVer)

	if err != nil {
		return field.Invalid(key, targetVer, err.Error())
	}

	if targetSemver.LessThan(currentSemver) {
		return nil
	}

	if IsZeroDowntimeUpgradeDisabled(obj) {
		gitlablog.Info("zero downtime upgrade is disabled, skipping upgrade path validation", "name", obj.GetName())

		return nil
	}

	if !isZeroDowntimePath(currentSemver, targetSemver) {
		return field.Invalid(key, targetVer,
			fmt.Sprintf("Upgrading from %s to %s is an invalid zero downtime upgrade path. The version must be updated one minor release at a time.", currentVer, targetVer))
	}

	return nil
}

func (r *GitLabCustomValidator) wrongTypeError(obj runtime.Object) error {
	return fmt.Errorf("expected a GitLab object but got %T", obj)
}

func newError(name string, err *field.Error) error {
	return apierrors.NewInvalid(GroupKind, name, field.ErrorList{err})
}
