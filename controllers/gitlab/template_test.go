package gitlab

import (
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"

	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Template", func() {
	/*
	 * All tests are packed together here to avoid rendering GitLab Chart repeatedly.
	 * This is done to speed up the test.
	 */

	It("must render the template only when the CR has changed", func() {
		mockGitLab1 := CreateMockGitLab(releaseName, namespace, support.Values{})
		mockGitLab1.UID = "a"
		mockGitLab1.Generation = 1

		mockGitLab2 := CreateMockGitLab(releaseName, namespace, support.Values{})
		mockGitLab1.UID = "a"
		mockGitLab1.Generation = 2

		adapter1 := CreateMockAdapter(mockGitLab1)
		adapter2 := CreateMockAdapter(mockGitLab2)

		template1, err := GetTemplate(adapter1)

		Expect(err).To(BeNil())
		Expect(template1).NotTo(BeNil())

		template1prime, err := GetTemplate(adapter1)

		Expect(err).To(BeNil())
		Expect(template1prime).To(BeIdenticalTo(template1))

		template2, err := GetTemplate(adapter2)

		Expect(err).To(BeNil())
		Expect(template2).NotTo(BeNil())
		Expect(template2).NotTo(BeIdenticalTo(template1))
	})
})

// dumpTemplate() will serialize the template and display the YAML for debugging.
func dumpTemplate(template helm.Template) string { //nolint:golint,unused
	output := new(strings.Builder)

	s := k8sjson.NewYAMLSerializer(k8sjson.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	for _, o := range template.Objects() {
		output.WriteString("---\n")
		_ = s.Encode(o, output)
	}

	return output.String()
}

// dumpTemplateToFile() will output the Helm template to a file.
// Note: the file is written to where the test runs NOT from where the
// tests were run from.
func dumpTemplateToFile(template helm.Template, filename string) error { //nolint:golint,deadcode,unused
	fh, err := os.Create(filename)
	if err != nil {
		return err
	}

	_, _ = fh.WriteString(dumpTemplate(template))

	fh.Close()

	return nil
}

// dumpHelmValues() will output the current values that Helm is using.
func dumpHelmValues(values support.Values) string { //nolint:golint,unused
	output, _ := json.MarshalIndent(values, "", "    ")
	return string(output)
}

// dumpHelmValuesToFile() will output the current values to a file.
// Note: the file is written to where the test runs NOT from where the
//
//	tests were run from.
func dumpHelmValuesToFile(values support.Values, filename string) error { //nolint:golint,deadcode,unused
	fh, err := os.Create(filename)
	if err != nil {
		return err
	}

	_, _ = fh.WriteString(dumpHelmValues(values))

	fh.Close()

	return nil
}
