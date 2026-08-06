package v2alpha1

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestV2Alpha1(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "GitLab v2alpha1 Suite")
}
