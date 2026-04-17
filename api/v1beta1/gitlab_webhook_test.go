package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
)

var _ = Describe("Webhook", func() {
	Describe("CRD Update", func() {
		var warnings admission.Warnings
		var err error

		var obj *GitLab

		JustBeforeEach(func() {
			warnings, err = (&GitLabCustomValidator{}).ValidateUpdate(context.Background(), obj, obj)
		})

		Context("Version validation", func() {
			When("A valid version of the catalog is used", func() {
				BeforeEach(func() {
					obj = createGitLab(helm.GetChartVersion(), "")
				})

				It("The validation passes", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(warnings).To(BeEmpty())
				})
			})

			When("A unknown version is used", func() {
				BeforeEach(func() {
					obj = createGitLab("1.0.0", "")
				})

				It("The validation fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("spec.chart.version"))
					Expect(warnings).To(BeEmpty())
				})
			})
		})

		Context("Upgrade path validation", func() {
			When("Updated by one catalog version", func() {
				BeforeEach(func() {
					versions := helm.AvailableChartVersions()
					obj = createGitLab(versions[0], versions[1])
				})

				It("The validation passes", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(warnings).To(BeEmpty())
				})
			})

			When("Updated by two catalog versions", func() {
				BeforeEach(func() {
					versions := helm.AvailableChartVersions()
					obj = createGitLab(versions[0], versions[2])
				})

				It("The validation fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("spec.chart.version"))
					Expect(err.Error()).To(ContainSubstring("invalid zero downtime"))
					Expect(warnings).To(BeEmpty())
				})
			})

			When("Updated by two catalog versions with zero downtime upgrade disabled", func() {
				BeforeEach(func() {
					versions := helm.AvailableChartVersions()
					obj = createGitLabWithAnnotations(versions[0], versions[2],
						map[string]string{DisableZDUAnnotationKey: "true"})
				})

				It("The validation passes", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(warnings).To(BeEmpty())
				})
			})

			When("Updated by two catalog versions with zero downtime upgrade enabled", func() {
				BeforeEach(func() {
					versions := helm.AvailableChartVersions()
					obj = createGitLabWithAnnotations(versions[0], versions[2],
						map[string]string{DisableZDUAnnotationKey: "false"})
				})

				It("The validation fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("spec.chart.version"))
					Expect(err.Error()).To(ContainSubstring("invalid zero downtime"))
					Expect(warnings).To(BeEmpty())
				})
			})
		})
	})
})

func createGitLab(chartVersion, statusVersion string) *GitLab {
	return createGitLabWithAnnotations(chartVersion, statusVersion, nil)
}

func createGitLabWithAnnotations(chartVersion, statusVersion string, annotations map[string]string) *GitLab {
	return &GitLab{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.gitlab.com/v1beta1",
			Kind:       "GitLab",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "gitlab",
			Namespace:   "test",
			Annotations: annotations,
		},
		Spec: GitLabSpec{
			Chart: GitLabChartSpec{
				Version: chartVersion,
			},
		},
		Status: GitLabStatus{
			Version: statusVersion,
		},
	}
}
