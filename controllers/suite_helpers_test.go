package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gitlabv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/adapter"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var (
	emptyValues = support.Values{}
)

func CreateMockGitLab(releaseName, namespace string, customValues support.Values) *gitlabv1beta1.GitLab {
	values := support.Values{}
	_ = values.AddFromYAML(minimalValues())
	_ = values.Merge(WithOverrides(IngressModeValues(), customValues))

	return &gitlabv1beta1.GitLab{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.gitlab.com/v1beta1",
			Kind:       "GitLab",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: namespace,
		},
		Spec: gitlabv1beta1.GitLabSpec{
			Chart: gitlabv1beta1.GitLabChartSpec{
				Version: helm.GetChartVersion(),
				Values: gitlabv1beta1.ChartValues{
					Object: values,
				},
			},
		},
	}
}

func CreateMockAdapter(mockGitLab *gitlabv1beta1.GitLab) gitlab.Adapter {
	adapter, _ := adapter.NewV1Beta1(context.TODO(), mockGitLab)

	return adapter
}

func minimalValues() string {
	return `
gitlab:
  toolbox:
    backups:
      cron:
        enabled: true
      objectStorage:
        config:
          secret: backup-storage-secret
          key: config
registry:
  storage:
    secret: registry-storage-secret
    key: config
global:
  redis:
    host: redis.example.com
  psql:
    host: psql.example.com
    password:
      secret: psql-password
      key: password
  appConfig:
    object_store:
      enabled: true
      connection:
        secret: object-storage-secret
        key: connection
`
}

func createMinimalSecrets() {
	secrets := []*corev1.Secret{
		newSecret("backup-storage-secret", "config", ""),
		newSecret("registry-storage-secret", "config", ""),
		newSecret("psql-password", "password", ""),
		newSecret("object-storage-secret", "connection", ""),
	}
	for _, s := range secrets {
		Expect(createObject(s)).Should(Succeed())
	}
}

func createObject(obj client.Object) error {
	err := k8sClient.Create(ctx, obj)
	if errors.IsAlreadyExists(err) {
		err = nil
	}

	return err
}

func updateObject(obj client.Object, mutate func(client.Object) error) error {
	key := client.ObjectKeyFromObject(obj)
	actual := obj.DeepCopyObject().(client.Object)

	if err := k8sClient.Get(ctx, key, actual); err != nil {
		return err
	}

	if err := mutate(actual); err != nil {
		return err
	}

	return k8sClient.Update(ctx, actual)
}

func getObject(name string, obj client.Object) error {
	lookupKey := types.NamespacedName{
		Name:      name,
		Namespace: Namespace,
	}

	return k8sClient.Get(ctx, lookupKey, obj)
}

func getObjectPromise(name string, obj client.Object) func() error {
	return func() error {
		return getObject(name, obj)
	}
}

func listObjects(query string, obj client.ObjectList) error {
	labelSelector, err := labels.Parse(query)
	if err != nil {
		return err
	}

	listOptions := &client.ListOptions{
		Namespace:     Namespace,
		LabelSelector: labelSelector,
	}

	if err := k8sClient.List(ctx, obj, listOptions); err != nil {
		return err
	}

	return nil
}

func listObjectsPromise(query string, obj client.ObjectList, expectedSize int) func() error {
	return func() error {
		if err := listObjects(query, obj); err != nil {
			return err
		}

		switch l := obj.(type) {
		case *batchv1.JobList:
			if len(l.Items) < expectedSize {
				return fmt.Errorf("Only %d Jobs found with [%s]. Expecting %d",
					len(l.Items), query, expectedSize)
			}
		}

		return nil
	}
}

func deleteObject(name string, obj client.Object) error {
	if err := getObject(name, obj); err != nil {
		if errors.IsNotFound(err) {
			err = nil
		}

		return err
	}

	return k8sClient.Delete(ctx, obj)
}

func deleteObjectPromise(name string, obj client.Object) func() error {
	return func() error {
		return deleteObject(name, obj)
	}
}

func listConfigMapsPromise(query string) func() []corev1.ConfigMap {
	return func() []corev1.ConfigMap {
		createdCfgMaps := &corev1.ConfigMapList{}
		if err := listObjects(query, createdCfgMaps); err != nil {
			return nil
		}

		return createdCfgMaps.Items
	}
}

func updateJobStatusPromise(query string, success bool) func() error {
	return func() error {
		createdJobs := &batchv1.JobList{}

		if err := listObjects(query, createdJobs); err != nil {
			return err
		}

		if len(createdJobs.Items) == 0 {
			return fmt.Errorf("Job not found [%s]", query)
		}

		for _, j := range createdJobs.Items {
			if success {
				j.Status.Succeeded = 1
				j.Status.Failed = 0
			} else {
				j.Status.Succeeded = 0
				j.Status.Failed = 1
			}

			if err := k8sClient.Status().Update(ctx, &j); err != nil {
				return err
			}
		}

		return nil
	}
}

func appLabels(releaseName, appName string) string {
	return fmt.Sprintf("release=%s,app=%s", releaseName, appName)
}

func createGitLabResource(releaseName string, chartValues support.Values) {
	merged := WithOverrides(IngressModeValues(), chartValues)

	By("Creating a new GitLab resource")
	Expect(createObject(CreateMockGitLab(releaseName, Namespace, merged))).Should(Succeed())

	By("Checking GitLab resource is created")
	Eventually(getObjectPromise(releaseName, &gitlabv1beta1.GitLab{}),
		PollTimeout, PollInterval).Should(Succeed())
}

func updateGitLabResource(releaseName string, customValues support.Values) {
	By("Update the existing GitLab resource")
	Expect(
		updateObject(
			CreateMockGitLab(releaseName, Namespace, support.Values{}),
			func(obj client.Object) error {
				values := support.Values{}
				_ = values.AddFromYAML(minimalValues())
				_ = values.Merge(WithOverrides(IngressModeValues(), customValues))

				gitlab := obj.(*gitlabv1beta1.GitLab)
				gitlab.Spec.Chart.Values.Object = values

				return nil
			})).Should(Succeed())
}
