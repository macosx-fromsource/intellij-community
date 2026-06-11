package settings

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// ssarReactor returns a reactor for "create selfsubjectaccessreviews" that sets
// the review's Allowed status based on the provided decide function. The decide
// function receives the requested ResourceAttributes so tests can assert on the
// group, resource, namespace and verb being checked.
func ssarReactor(decide func(attrs authorizationv1.ResourceAttributes) bool) clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (bool, runtime.Object, error) {
		review := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)

		return true, &authorizationv1.SelfSubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: decide(*review.Spec.ResourceAttributes),
			},
		}, nil
	}
}

var _ = Describe("CanManageResource", func() {
	const (
		group     = "gateway.networking.k8s.io"
		resource  = "gateways"
		namespace = "gitlab-system"
	)

	var client *fake.Clientset

	BeforeEach(func() {
		client = fake.NewClientset()
	})

	When("the ServiceAccount is allowed all management verbs", func() {
		It("returns true", func() {
			client.PrependReactor("create", "selfsubjectaccessreviews",
				ssarReactor(func(_ authorizationv1.ResourceAttributes) bool { return true }))

			Expect(canManageResource(context.Background(), client, group, resource, namespace)).To(BeTrue())
		})
	})

	When("the ServiceAccount is denied one of the management verbs", func() {
		It("returns false", func() {
			// Read-only access: every verb is allowed except the mutating "delete".
			client.PrependReactor("create", "selfsubjectaccessreviews",
				ssarReactor(func(attrs authorizationv1.ResourceAttributes) bool {
					return attrs.Verb != "delete"
				}))

			Expect(canManageResource(context.Background(), client, group, resource, namespace)).To(BeFalse())
		})
	})

	When("the ServiceAccount is denied all management verbs", func() {
		It("returns false", func() {
			client.PrependReactor("create", "selfsubjectaccessreviews",
				ssarReactor(func(_ authorizationv1.ResourceAttributes) bool { return false }))

			Expect(canManageResource(context.Background(), client, group, resource, namespace)).To(BeFalse())
		})
	})

	When("every management verb is checked", func() {
		It("performs a SelfSubjectAccessReview for each verb", func() {
			var checkedVerbs []string

			client.PrependReactor("create", "selfsubjectaccessreviews",
				ssarReactor(func(attrs authorizationv1.ResourceAttributes) bool {
					checkedVerbs = append(checkedVerbs, attrs.Verb)
					return true
				}))

			Expect(canManageResource(context.Background(), client, group, resource, namespace)).To(BeTrue())
			Expect(checkedVerbs).To(ConsistOf(managementVerbs))
		})
	})

	When("the requested resource attributes are inspected", func() {
		It("scopes the review to the given group, resource and namespace", func() {
			var seen authorizationv1.ResourceAttributes

			client.PrependReactor("create", "selfsubjectaccessreviews",
				ssarReactor(func(attrs authorizationv1.ResourceAttributes) bool {
					seen = attrs
					return true
				}))

			Expect(canManageResource(context.Background(), client, group, resource, namespace)).To(BeTrue())
			Expect(seen.Group).To(Equal(group))
			Expect(seen.Resource).To(Equal(resource))
			Expect(seen.Namespace).To(Equal(namespace))
		})
	})

	When("the SelfSubjectAccessReview request fails", func() {
		It("returns false", func() {
			client.PrependReactor("create", "selfsubjectaccessreviews",
				func(_ clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("boom")
				})

			Expect(canManageResource(context.Background(), client, group, resource, namespace)).To(BeFalse())
		})
	})
})

var _ = Describe("WatchNamespace", func() {
	AfterEach(func() {
		Expect(os.Unsetenv("WATCH_NAMESPACE")).To(Succeed())
		Load()
	})

	When("WATCH_NAMESPACE is set", func() {
		It("is loaded with its value", func() {
			Expect(os.Setenv("WATCH_NAMESPACE", "gitlab-system")).To(Succeed())
			Load()
			Expect(WatchNamespace).To(Equal("gitlab-system"))
		})
	})

	When("WATCH_NAMESPACE is unset", func() {
		It("is loaded as an empty string", func() {
			Expect(os.Unsetenv("WATCH_NAMESPACE")).To(Succeed())
			Load()
			Expect(WatchNamespace).To(BeEmpty())
		})
	})
})
