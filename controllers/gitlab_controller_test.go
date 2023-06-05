package controllers

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gitlabv1beta1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v1beta1"
	gitlabctl "gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/gitlab"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

var _ = Describe("GitLab controller", func() {
	Context("GitLab CRD", func() {
		It("Should create a CR with the specified Chart values", func() {
			releaseName := "crd-testing"

			chartValues := support.Values{}
			_ = chartValues.SetValue("global.hosts.domain", "mydomain.com")
			_ = chartValues.SetValue("certmanager-issuer.email", "me@mydomain.com")

			By("Creating a new GitLab resource")
			Expect(createObject(CreateMockGitLab(releaseName, Namespace, chartValues), true)).Should(Succeed())

			By("Checking the created GitLab resource")
			Eventually(func() error {
				gitlab := &gitlabv1beta1.GitLab{}
				if err := getObject(releaseName, gitlab); err != nil {
					return err
				}

				/* A workaround to stop reflect.DeepEqual naively reject the comparison */
				var expected map[string]interface{} = chartValues
				if !reflect.DeepEqual(gitlab.Spec.Chart.Values.Object, expected) {
					return fmt.Errorf("The Chart values of CR are not equal to the expected values. Observed: %s",
						gitlab.Spec.Chart.Values.Object)
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
	})

	Context("Shared secrets and Self signed certificates Jobs", func() {
		When("Both Jobs succeed", func() {
			releaseName := "jobs-succeeded"

			chartValues := support.Values{}
			_ = chartValues.SetValue("global.ingress.configureCertmanager", false)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
			})

			It("Should create resources for Jobs and continue the reconcile loop", func() {
				cfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)
				sharedSecretQuery := appLabels(releaseName, gitlabctl.GitLabComponentName)
				postgresQuery := fmt.Sprintf("app.kubernetes.io/instance=%s-%s", releaseName, gitlabctl.DefaultPostgresComponentName)

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

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(listConfigMapsPromise(postgresQuery),
					PollTimeout, PollInterval).ShouldNot(BeEmpty())
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

	Context("PostgreSQL", func() {
		When("Bundled PostgreSQL is disabled", func() {
			releaseName := "postgresql-disabled"
			pgName := gitlabctl.DefaultPostgresComponentName
			cfgMapName := fmt.Sprintf("%s-%s-init-db", releaseName, pgName)
			metricsServiceName := fmt.Sprintf("%s-%s-metrics", releaseName, pgName)
			headlessServiceName := fmt.Sprintf("%s-%s-headless", releaseName, pgName)
			serviceName := fmt.Sprintf("%s-%s", releaseName, pgName)
			statefulSetName := fmt.Sprintf("%s-%s", releaseName, pgName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			chartValues := support.Values{}
			_ = chartValues.SetValue("postgresql.install", false)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should not create PostgreSQL resources and continue the reconcile loop", func() {
				By("Checking PostgreSQL Metrics Service does not exist")
				Eventually(getObjectPromise(metricsServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking PostgreSQL Headless Service does not exist")
				Eventually(getObjectPromise(headlessServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking PostgreSQL Service does not exist")
				Eventually(getObjectPromise(serviceName, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking PostgreSQL StatefulSet does not exist")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking PostgreSQL ConfigMap does not exist")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Bundled PostgreSQL is enabled", func() {
			releaseName := "postgresql-enabled"
			pgName := gitlabctl.DefaultPostgresComponentName
			cfgMapName := fmt.Sprintf("%s-%s-init-db", releaseName, pgName)
			metricsServiceName := fmt.Sprintf("%s-%s-metrics", releaseName, pgName)
			headlessServiceName := fmt.Sprintf("%s-%s-hl", releaseName, pgName)
			serviceName := fmt.Sprintf("%s-%s", releaseName, pgName)
			statefulSetName := fmt.Sprintf("%s-postgresql", releaseName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			if gitlabctl.IsChartVersionOlderThan(helm.GetChartVersion(), gitlabctl.ChartVersion7) {
				headlessServiceName = fmt.Sprintf("%s-%s-headless", releaseName, pgName)
				statefulSetName = fmt.Sprintf("%s-%s", releaseName, pgName)
			}

			chartValues := support.Values{}
			_ = chartValues.SetValue("postgresql.install", true)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should create PostgreSQL resources and continue the reconcile loop", func() {
				By("Checking PostgreSQL Metrics Service exists")
				Eventually(getObjectPromise(metricsServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL Headless Service exists")
				Eventually(getObjectPromise(headlessServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL Service exists")
				Eventually(getObjectPromise(serviceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL StatefulSet exists")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL ConfigMap exists")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Bundled PostgreSQL has a overridden name", func() {
			releaseName := "postgresql-name-override"
			pgComponent := gitlabctl.DefaultPostgresComponentName

			cfgMapName := fmt.Sprintf("%s-%s-init-db", releaseName, pgComponent)
			metricsServiceName := fmt.Sprintf("%s-%s-metrics", releaseName, pgComponent)
			headlessServiceName := fmt.Sprintf("%s-%s-hl", releaseName, pgComponent)
			serviceName := fmt.Sprintf("%s-%s", releaseName, pgComponent)

			nameOverride := "foobar"
			statefulSetName := fmt.Sprintf("%s-postgresql", releaseName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			if gitlabctl.IsChartVersionOlderThan(helm.GetChartVersion(), gitlabctl.ChartVersion7) {
				headlessServiceName = fmt.Sprintf("%s-%s-headless", releaseName, pgComponent)
				statefulSetName = fmt.Sprintf("%s-%s", releaseName, nameOverride)
			}

			chartValues := support.Values{}
			_ = chartValues.SetValue("postgresql.install", true)
			_ = chartValues.SetValue("postgresql.nameOverride", nameOverride)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should create PostgreSQL resources with specified name and continue the reconcile loop", func() {
				By("Checking PostgreSQL Metrics Service exists")
				Eventually(getObjectPromise(metricsServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL Headless Service exists")
				Eventually(getObjectPromise(headlessServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL Service exists")
				Eventually(getObjectPromise(serviceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL StatefulSet exists")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking PostgreSQL ConfigMap exists")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})
	})

	Context("Redis", func() {
		When("Bundled Redis is disabled", func() {
			releaseName := "redis-disabled"
			cfgMapNameScripts := fmt.Sprintf("%s-scripts", releaseName)
			cfgMapNameHealth := fmt.Sprintf("%s-health", releaseName)
			cfgMapName := releaseName
			serviceNameHeadless := fmt.Sprintf("%s-headless", releaseName)
			serviceNameMetrics := fmt.Sprintf("%s-metrics", releaseName)
			serviceNameMaster := fmt.Sprintf("%s-master", releaseName)
			statefulSetName := fmt.Sprintf("%s-master", releaseName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.GitalyComponentName)

			chartValues := support.Values{}

			_ = chartValues.SetValue("redis.install", false)
			_ = chartValues.SetValue("global.redis.host", "redis.example.com")

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should not create Redis resources and continue the reconcile loop", func() {
				By("Checking Redis Scripts ConfigMap does not exist")
				Eventually(getObjectPromise(cfgMapNameScripts, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking Redis Health ConfigMap does not exist")
				Eventually(getObjectPromise(cfgMapNameHealth, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking Redis ConfigMap does not exist")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking PostgreSQL Headless Service does not exist")
				Eventually(getObjectPromise(serviceNameHeadless, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking Redis Metrics Service does not exist")
				Eventually(getObjectPromise(serviceNameMetrics, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking Redis Master Service does not exist")
				Eventually(getObjectPromise(serviceNameMaster, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking Redis StatefulSet does not exist")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Bundled Redis is enabled", func() {
			releaseName := "redis-enabled"
			cfgMapNameScripts := fmt.Sprintf("%s-scripts", releaseName)
			cfgMapNameHealth := fmt.Sprintf("%s-health", releaseName)
			cfgMapName := fmt.Sprintf("%s-configuration", releaseName)
			serviceNameHeadless := fmt.Sprintf("%s-headless", releaseName)
			serviceNameMetrics := fmt.Sprintf("%s-metrics", releaseName)
			serviceNameMaster := fmt.Sprintf("%s-master", releaseName)
			statefulSetName := fmt.Sprintf("%s-master", releaseName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			if gitlabctl.IsChartVersionOlderThan(helm.GetChartVersion(), gitlabctl.ChartVersion7) {
				cfgMapName = releaseName
			}

			chartValues := support.Values{}
			_ = chartValues.SetValue("redis.install", true)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should create Redis resources and continue the reconcile loop", func() {
				By("Checking Redis Scripts ConfigMap exists")
				Eventually(getObjectPromise(cfgMapNameScripts, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Health ConfigMap exists")
				Eventually(getObjectPromise(cfgMapNameHealth, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis ConfigMap exists")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Headless Service exists")
				Eventually(getObjectPromise(serviceNameHeadless, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Metrics Service exists")
				Eventually(getObjectPromise(serviceNameMetrics, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Master Service exists")
				Eventually(getObjectPromise(serviceNameMaster, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis StatefulSet exists")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Bundled Redis has a overridden name", func() {
			releaseName := "redis-name-override"
			nameOverride := "foobar"

			cfgMapNameScripts := fmt.Sprintf("%s-%s-scripts", releaseName, nameOverride)
			cfgMapNameHealth := fmt.Sprintf("%s-%s-health", releaseName, nameOverride)
			cfgMapName := fmt.Sprintf("%s-%s-configuration", releaseName, nameOverride)
			serviceNameHeadless := fmt.Sprintf("%s-%s-headless", releaseName, nameOverride)
			serviceNameMetrics := fmt.Sprintf("%s-%s-metrics", releaseName, nameOverride)
			serviceNameMaster := fmt.Sprintf("%s-%s-master", releaseName, nameOverride)
			statefulSetName := fmt.Sprintf("%s-%s-master", releaseName, nameOverride)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			if gitlabctl.IsChartVersionOlderThan(helm.GetChartVersion(), gitlabctl.ChartVersion7) {
				cfgMapName = fmt.Sprintf("%s-%s", releaseName, nameOverride)
			}

			chartValues := support.Values{}
			_ = chartValues.SetValue("redis.install", true)
			_ = chartValues.SetValue("redis.nameOverride", nameOverride)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should create Redis resources with the overridden names and continue the reconcile loop", func() {
				By("Checking Redis Scripts ConfigMap exists")
				Eventually(getObjectPromise(cfgMapNameScripts, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Health ConfigMap exists")
				Eventually(getObjectPromise(cfgMapNameHealth, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis ConfigMap exists")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Headless Service exists")
				Eventually(getObjectPromise(serviceNameHeadless, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Metrics Service exists")
				Eventually(getObjectPromise(serviceNameMetrics, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis Master Service exists")
				Eventually(getObjectPromise(serviceNameMaster, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking Redis StatefulSet exists")
				Eventually(getObjectPromise(statefulSetName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("External Redis has subqueue with no Secret created", func() {
			releaseName := "redis-subqueues-no-secret"
			chartValues := support.Values{}
			values := `
redis:
  install: false
global:
  redis:
    host: redis.example
    port: 9001
    password:
      enabled: true
      secret: custom-redis-secret
      key: redis-password
    cache:
      host: cache.redis.example
      password:
        enabled: true
        secret: custom-cache-secret
        key: cache-password
`
			_ = chartValues.AddFromYAML(values)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should stop when Secret does not exist", func() {
				By("Confirming no StatefulSets are created due to missing secrets")
				Eventually(getObjectPromise(fmt.Sprintf("%s-%s", releaseName, gitlabctl.DefaultPostgresComponentName), &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())
			})
		})

		When("External Redis has subqueue with Secret created", func() {
			releaseName := "redis-subqueues-with-secret"

			chartValues := support.Values{}
			values := `
redis:
  install: false
global:
  redis:
    host: redis.example
    port: 9001
    password:
      enabled: true
      secret: custom-redis-secret
      key: redis-password
    cache:
      host: cache.redis.example
      password:
        enabled: true
        secret: custom-cache-secret
        key: cache-password
`
			_ = chartValues.AddFromYAML(values)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should proceed when Secret does exist", func() {
				cacheSecret := newSecret("custom-cache-secret", Namespace, "cache-password", "foo")
				redisSecret := newSecret("custom-redis-secret", Namespace, "redis-password", "foo")

				By("Creating global Redis Secret")
				Expect(createObject(redisSecret, false)).Should(Succeed())

				By("Creating Cache Redis Secret")
				Expect(createObject(cacheSecret, false)).Should(Succeed())

				By("Confirming that StatefulSet is created")
				Eventually(getObjectPromise(fmt.Sprintf("%s-%s", releaseName, gitlabctl.DefaultPostgresComponentName), &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})
	})

	Context("MinIO", func() {
		When("Bundled MinIO is enabled", func() {
			releaseName := "minio-enabled"
			cfgMapName := fmt.Sprintf("%s-minio-config-cm", releaseName)
			serviceName := fmt.Sprintf("%s-minio-svc", releaseName)
			jobName := fmt.Sprintf("%s-minio-create-buckets-1", releaseName)
			ingressName := fmt.Sprintf("%s-minio", releaseName)
			pvcName := fmt.Sprintf("%s-minio", releaseName)
			deploymentName := fmt.Sprintf("%s-minio", releaseName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			chartValues := support.Values{}
			_ = chartValues.SetValue("global.minio.enabled", true)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
				processMinioBucketsJob(releaseName)
			})

			It("Should create MinIO resources and continue the reconcile loop", func() {
				By("Checking MinIO Service exists")
				Eventually(getObjectPromise(serviceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking MinIO Job exists")
				Eventually(getObjectPromise(jobName, &batchv1.Job{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking MinIO Ingress exists")
				Eventually(getObjectPromise(ingressName, &networkingv1.Ingress{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking MinIO PersristenVolumeClaim exists")
				Eventually(getObjectPromise(pvcName, &corev1.PersistentVolumeClaim{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking MinIO Deployment exists")
				Eventually(getObjectPromise(deploymentName, &appsv1.Deployment{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking MinIO ConfigMap exists")
				Eventually(getObjectPromise(cfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
			})
		})

		When("Bundled MinIO is disabled", func() {
			releaseName := "minio-disabled"
			cfgMapName := fmt.Sprintf("%s-minio-config-cm", releaseName)
			jobName := fmt.Sprintf("%s-minio-create-buckets-1", releaseName)
			ingressName := fmt.Sprintf("%s-minio", releaseName)
			pvcName := fmt.Sprintf("%s-minio", releaseName)
			serviceName := fmt.Sprintf("%s-minio-svc", releaseName)
			deploymentName := fmt.Sprintf("%s-minio", releaseName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			chartValues := support.Values{}
			values := `
global:
  minio:
    enabled: false
  appConfig:
    object_store:
      enabled: true
      connection:
        secret: global-object-storage-secret
        key: value
`
			_ = chartValues.AddFromYAML(values)

			BeforeEach(func() {
				createGitLabResource(releaseName, chartValues)
				processSharedSecretsJob(releaseName)
			})

			It("Should not create MinIO resources and continue the reconcile loop", func() {
				By("Checking MinIO Service does not exist")
				Eventually(getObjectPromise(serviceName, &corev1.Service{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking MinIO Job does not exist")
				Eventually(getObjectPromise(jobName, &batchv1.Job{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking MinIO Ingress does not exist")
				Eventually(getObjectPromise(ingressName, &networkingv1.Ingress{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking MinIO PersristenVolumeClaim does not exist")
				Eventually(getObjectPromise(pvcName, &corev1.PersistentVolumeClaim{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking MinIO Deployment does not exist")
				Eventually(getObjectPromise(deploymentName, &appsv1.StatefulSet{}),
					PollTimeout, PollInterval).ShouldNot(Succeed())

				By("Checking MinIO ConfigMap does not exist")
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
			releaseName := "nginx-enabled"
			controllerServiceName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
			controllerDeploymentName := fmt.Sprintf("%s-%s-controller", releaseName, gitlabctl.NGINXComponentName)
			nextCfgMapName := fmt.Sprintf("%s-%s", releaseName, gitlabctl.SharedSecretsComponentName)

			BeforeEach(func() {
				createGitLabResource(releaseName, support.Values{})
				processSharedSecretsJob(releaseName)
			})

			It("Should create NGINX resources by default and continue the reconcile loop", func() {
				By("Checking NGINX Controller Service exists")
				Eventually(getObjectPromise(controllerServiceName, &corev1.Service{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking NGINX Controller Deployment exists")
				Eventually(getObjectPromise(controllerDeploymentName, &appsv1.Deployment{}),
					PollTimeout, PollInterval).Should(Succeed())

				By("Checking next resources in the reconcile loop, e.g. ConfigMaps")
				Eventually(getObjectPromise(nextCfgMapName, &corev1.ConfigMap{}),
					PollTimeout, PollInterval).Should(Succeed())
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

func processMinioBucketsJob(releaseName string) {
	minioQuery := appLabels(releaseName, gitlabctl.MinioComponentName)

	By("Manipulating the MinIO buckets Job to succeed")
	Eventually(updateJobStatusPromise(minioQuery, true),
		PollTimeout, PollInterval).Should(Succeed())
}

func newSecret(name, namespace, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
	}
}
