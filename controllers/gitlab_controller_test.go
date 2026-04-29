package controllers

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gitlabv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/status"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("GitLab controller", func() {
	Context("GitLab CRD", func() {
		It("Should create a CR with the specified Chart values", func() {
			releaseName := "crd-testing"
			domain := "example.com"
			certMail := "webmaster@example.com"

			chartValues := support.Values{}
			_ = chartValues.SetValue("global.hosts.domain", domain)
			_ = chartValues.SetValue("certmanager-issuer.email", certMail)

			By("Creating a new GitLab resource")
			Expect(createObject(CreateMockGitLab(releaseName, Namespace, chartValues))).Should(Succeed())

			By("Checking the created GitLab resource")
			Eventually(func() error {
				gitlab := &gitlabv1beta1.GitLab{}
				if err := getObject(releaseName, gitlab); err != nil {
					return err
				}

				values := support.Values(gitlab.Spec.Chart.Values.Object)
				if v := values.GetString("global.hosts.domain"); v != domain {
					return fmt.Errorf("expected domain %q, got %q", domain, v)
				}

				if v := values.GetString("certmanager-issuer.email"); v != certMail {
					return fmt.Errorf("expected certmanager mail %q, got %q", certMail, v)
				}

				return nil
			}, PollTimeout, PollInterval).Should(Succeed())

			By("Deleting the created GitLab resource")
			Eventually(deleteObjectPromise(releaseName, &gitlabv1beta1.GitLab{}),
				PollTimeout, PollInterval).Should(Succeed())
		})
	})

	Context("GitLab CR spec", func() {
		It("Should change the managed resources when the Chart values change", func() {
			releaseName := "cr-spec-changes"
			chartValues := support.Values{}
			cfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)
			sharedSecretQuery := appLabels(releaseName, gitlabctl.GitLabComponentName)

			createGitLabResource(releaseName, chartValues)

			By("Checking shared secrets ConfigMap is created")
			Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
				PollTimeout, PollInterval).Should(Succeed())

			_ = chartValues.SetValue("shared-secrets.env", "test")
			_ = chartValues.SetValue("shared-secrets.annotations", map[string]string{
				"foo": "FOO",
				"bar": "BAR",
			})

			updateGitLabResource(releaseName, chartValues)

			By("Checking shared secrets ConfigMap picked up the change")
			Eventually(func() error {
				cfgMap := &corev1.ConfigMap{}
				if err := getObject(cfgMapName, cfgMap); err != nil {
					return err
				}

				if !strings.Contains(cfgMap.Data["generate-secrets"], "env=test") {
					return fmt.Errorf("`generate-secrets` does not contain the changes")
				}

				return nil
			}, PollTimeout, PollInterval).Should(Succeed())

			By("Checking shared secrets Jobs picked up the change")
			Eventually(func() error {
				jobs := &batchv1.JobList{}
				if err := listObjects(sharedSecretQuery, jobs); err != nil {
					return err
				}

				if len(jobs.Items) == 0 {
					return fmt.Errorf("Job list is emptry [%s]", sharedSecretQuery)
				}

				for _, job := range jobs.Items {
					if job.Spec.Template.ObjectMeta.Annotations["foo"] == "FOO" &&
						job.Spec.Template.ObjectMeta.Annotations["bar"] == "BAR" {
						return nil
					}
				}

				return fmt.Errorf("None of the Jobs had the expected annotations")
			}, PollTimeout, PollInterval).Should(Succeed())

			By("Deleting the created GitLab resource")
			Eventually(deleteObjectPromise(releaseName, &gitlabv1beta1.GitLab{}),
				PollTimeout, PollInterval).Should(Succeed())
		})

		It("Should fail the reconcile loop when invalid Chart values are provided", func() {
			releaseName := "cr-spec-invalid-chart-values"
			chartValues := support.Values{}

			_ = chartValues.SetValue("gitlab.gitlab-shell.extraVolumes", "some_invalid_k8s_spec: foobar")

			createGitLabResource(releaseName, chartValues)

			By("Checking the reconcile loop fails and records the error in the GitLab CR status")
			Eventually(func() error {
				gitlab := &gitlabv1beta1.GitLab{}
				if err := getObject(releaseName, gitlab); err != nil {
					return err
				}

				for _, condition := range gitlab.Status.Conditions {
					if condition.Type == status.ConditionInitialized.Name() &&
						condition.Status == metav1.ConditionFalse &&
						strings.Contains(condition.Message, "configuration error") {
						return nil
					}
				}

				return fmt.Errorf("Expected Initialized condition with False status and configuration error message not found, got: %v", gitlab.Status.Conditions)
			}, PollTimeout, PollInterval).Should(Succeed())

			By("Deleting the created GitLab resource")
			Eventually(deleteObjectPromise(releaseName, &gitlabv1beta1.GitLab{}),
				PollTimeout, PollInterval).Should(Succeed())
		})
	})

	Context("Shared secrets and Self signed certificates Jobs", func() {
		When("Both Jobs succeed", func() {
			releaseName := "jobs-succeeded"

			chartValues := support.Values{}
			_ = chartValues.SetValue("global.ingress.configureCertmanager", false)
			_ = chartValues.SetValue("global.ingress.tls.enabled", true)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
			})

			It("Should create resources for Jobs and continue the reconcile loop", func() {
				cfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)
				sharedSecretQuery := appLabels(releaseName, gitlabctl.GitLabComponentName)

				By("Checking Shared secrets Job and its ConfigMap are created")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
				Eventually(listObjectsPromise(sharedSecretQuery, &batchv1.JobList{}, 1),
					PollTimeout, PollInterval).Should(Succeed())

				By("Manipulating the Shared secrets Job to succeed")
				Eventually(updateJobStatusPromise(sharedSecretQuery, true),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking the Self signed certificates Job is created")
				Eventually(listObjectsPromise(sharedSecretQuery, &batchv1.JobList{}, 2),
					PollTimeout, PollInterval).Should(Succeed())

				By("Manipulating the Self signed certificates Job to succeed")
				Eventually(updateJobStatusPromise(sharedSecretQuery, true),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Jobs fail", func() {
			releaseName := "jobs-fail"

			BeforeEach(func() {
				createGitLabResource(releaseName, emptyValues)
			})

			It("Should fail the reconcile loop", func() {
				sharedSecretQuery := appLabels(releaseName, gitlabctl.GitLabComponentName)
				gitlabShellQuery := appLabels(releaseName, gitlabctl.GitLabShellComponentName)

				By("Manipulating the Job to fail")
				Eventually(updateJobStatusPromise(sharedSecretQuery, false),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Consistently(listConfigMapsPromise(gitlabShellQuery),
					10*time.Second, PollInterval).Should(BeEmpty())
			})
		})

		When("Jobs time out", func() {
			releaseName := "jobs-timeout"

			BeforeEach(func() {
				createGitLabResource(releaseName, emptyValues)
			})

			It("Should fail the reconcile loop", func() {
				gitlabShellQuery := appLabels(releaseName, gitlabctl.GitLabShellComponentName)
				sharedSecretQuery := appLabels(releaseName, gitlabctl.GitLabComponentName)

				By("Checking Shared secrets Job is created")
				Eventually(listObjectsPromise(sharedSecretQuery, &batchv1.JobList{}, 1),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Consistently(listConfigMapsPromise(gitlabShellQuery),
					10*time.Second, PollInterval).Should(BeEmpty())
			})
		})
	})

	Context("Gitaly", func() {
		When("Bundled Gitaly is enabled", func() {
			releaseName := "gitaly-enabled"
			cfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.GitalyComponentName)
			serviceName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.GitalyComponentName)
			statefulSetName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.GitalyComponentName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			chartValues := support.Values{}
			_ = chartValues.SetValue("global.gitaly.enabled", true)
			_ = chartValues.SetValue("global.ingress.configureCertmanager", true)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should create Gitaly resources and continue the reconcile loop", func() {
				By("Checking Gitaly Service exists")
				Eventually(getObjectPromise(serviceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Gitaly StatefulSet exists")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Gitaly ConfigMap exists")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Bundled Gitaly is disabled", func() {
			releaseName := "gitaly-disabled"
			cfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.GitalyComponentName)
			serviceName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.GitalyComponentName)
			statefulSetName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.GitalyComponentName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			chartValues := support.Values{}
			_ = chartValues.SetValue("global.ingress.configureCertmanager", true)
			values := `
global:
  gitaly:
    enabled: false
    external:
    - name: default
      hostname: gitaly.external.com
`
			_ = chartValues.AddFromYAML(values)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should not create Gitaly resources and continue the reconcile loop", func() {
				By("Checking Gitaly Service does not exist")
				Eventually(getObjectPromise(serviceName, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking Gitaly StatefulSet does not exist")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking Gitaly ConfigMap does not exist")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})
	})

	Context("Bundled NGINX with SSH support", func() {
		When("Bundled NGINX is disabled", func() {
			releaseName := "nginx-disabled"
			controllerServiceName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
			controllerDeploymentName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			chartValues := support.Values{}
			_ = chartValues.SetValue("nginx-ingress.enabled", false)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should not create NGINX resources and continue the reconcile loop", func() {
				By("Checking NGINX Controller Service does not exist")
				Eventually(getObjectPromise(controllerServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking NGINX Controller Deployment does not exist")
				Eventually(getObjectPromise(controllerDeploymentName, &appsv1.Deployment{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Bundled NGINX is enabled", func() {
			When("Controller Kind is Deployment", func() {
				releaseName := "nginx-deployment-enabled"
				controllerServiceName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
				controllerResourceName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
				nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)
				chartValues := support.Values{}

				BeforeEach(func() {
					createGitLabResource(releaseName, chartValues)
					processSharedSecretsJob(releaseName)
				})

				It("Should create NGINX resources by default and continue the reconcile loop", func() {
					By("Checking NGINX Controller Service exists")
					Eventually(getObjectPromise(controllerServiceName, &corev1.Service{}),
						PollTimeout, PollInterval).Should(Succeed())

					By("Checking NGINX Controller Deployment exists")
					Eventually(getObjectPromise(controllerResourceName, &appsv1.Deployment{}),
						PollTimeout, PollInterval).Should(Succeed())

					By("Checking NGINX Controller DaemonSet does not exist")
					Consistently(getObjectPromise(controllerResourceName, &appsv1.DaemonSet{}),
						10*time.Second, PollInterval).ShouldNot(Succeed())

					By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
					Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
						PollTimeout, PollInterval).Should(Succeed())
				})
			})

			When("Controller Kind is DaemonSet", func() {
				releaseName := "nginx-daemonset-enabled"
				controllerServiceName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
				controllerResourceName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
				nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)
				chartValues := support.Values{}
				_ = chartValues.SetValue("nginx-ingress.controller.kind", "DaemonSet")

				BeforeEach(func() {
					createGitLabResource(releaseName, chartValues)
					processSharedSecretsJob(releaseName)
				})

				It("Should create NGINX resources by default and continue the reconcile loop", func() {
					By("Checking NGINX Controller Service exists")
					Eventually(getObjectPromise(controllerServiceName, &corev1.Service{}),
						PollTimeout, PollInterval).Should(Succeed())

					By("Checking NGINX Controller DaemonSet exists")
					Eventually(getObjectPromise(controllerResourceName, &appsv1.DaemonSet{}),
						PollTimeout, PollInterval).Should(Succeed())

					By("Checking NGINX Controller Deployment does not exist")
					Consistently(getObjectPromise(controllerResourceName, &appsv1.Deployment{}),
						10*time.Second, PollInterval).ShouldNot(Succeed())

					By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
					Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
						PollTimeout, PollInterval).Should(Succeed())
				})
			})
		})

		Context("Registry", func() {
			When("Registry database migrations are enabled", func() {
				releaseName := "registry"
				chartValues := support.Values{}

				// Enable registry migrations and disable/externalize some components to
				// skip reconciler logic not relevant to the registry migrations.
				_ = chartValues.AddFromYAML(`
registry:
  database:
    enabled: true
    migrations:
      enabled: true
global:
  ingress:
    tls:
      secretName: mock
  redis:
    host: redis.example.com
  psql:
    host: psql.example.com
    password:
      secret: psql-secret
      key: psql-key
  gitaly:
    enabled: false
    external:
    - name: default
      hostname: gitaly.example.com
gitlab:
  webservice:
    enabled: false
  sidekiq:
    enabled: false
redis:
  install: false
postgresql:
  install: false
shared-secrets:
  enabled: false
`)

				BeforeEach(func() {
					Expect(createObject(newSecret("psql-secret", "psql-key", "foo"))).Should(Succeed())
					createGitLabResource(releaseName, chartValues)
				})

				It("Should create the registry migrations job", func() {
					Eventually(listObjectsPromise("app in ( registry, registry-migrations )", &batchv1.JobList{}, 1),
						PollTimeout, PollInterval).Should(Succeed())
				})
			})
		})
	})

	Context("Gateway API", func() {
		When("Gateway API is enabled", func() {
			releaseName := "gateway-api-enabled"

			chartValues := GatewayAPIModeValues()
			// Only standard Gateway API resources are loaded into the Cluster.
			// Envoy Gateway extensions are not available.
			_ = chartValues.SetValue("global.gatewayApi.installEnvoy", false)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should reconcile HTTPRoute resources", func() {
				httpRoutes := &unstructured.UnstructuredList{}
				httpRoutes.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "gateway.networking.k8s.io",
					Version: "v1",
					Kind:    "HTTPRouteList",
				})

				By("Checking at least one HTTPRoute is created")
				Eventually(func() (int, error) {
					if err := k8sClient.List(ctx, httpRoutes, &client.ListOptions{Namespace: Namespace}); err != nil {
						return 0, err
					}

					return len(httpRoutes.Items), nil
				}, PollTimeout, PollInterval).Should(BeNumerically(">", 0))
			})
		})
	})
})

func processSharedSecretsJob(releaseName string) {
	sharedSecretQuery := appLabels(releaseName, gitlabctl.GitLabComponentName)

	By("Manipulating the Shared secrets Job to succeed")
	Eventually(updateJobStatusPromise(sharedSecretQuery, true),
		PollTimeout, PollInterval).Should(Succeed())
}

func newSecret(name, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
	}
}
