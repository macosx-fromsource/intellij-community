package framework

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
)

// CanI asks the API server whether a service account may perform a verb, the way
// `kubectl auth can-i --as=system:serviceaccount:...` does.
//
// This is how the harness checks the RBAC of the Operator without impersonating
// it: a review is a read, so it works with the credentials the test already has,
// and it reports what the cluster would really decide rather than what a role
// manifest appears to say.
func (h *Harness) CanI(ctx context.Context, serviceAccount, namespace string, attributes authorizationv1.ResourceAttributes) (bool, string, error) {
	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:               fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount),
			ResourceAttributes: &attributes,
		},
	}

	if err := h.Client.Create(ctx, review); err != nil {
		return false, "", fmt.Errorf("reviewing access for %s/%s: %w", namespace, serviceAccount, err)
	}

	return review.Status.Allowed, review.Status.Reason, nil
}
