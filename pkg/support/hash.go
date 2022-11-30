package support

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SimpleObjectHash offers a default implementation for `ValueProvider.Hash`.
// It uses the original Custom Resource to calculate the hash value.
//
// Limitations: This implementation uses object UID and Generation. For these
// values to be valid, the resource must be populated with Kubernetes API,
// for example the resource must be retrieved with `Client.Get`.
//
// Note that object Generation does not change when resource metadata or status
// changes. So, as long as changes in resource metadata do not cause changes in
// resource UID, the hash does not change.
//
// For more details see:
//
// - https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#metadata
// - https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
func SimpleObjectHash(object client.Object) string {
	uid := object.GetUID()
	gen := object.GetGeneration()

	if uid == "" || gen == 0 {
		return ""
	}

	return fmt.Sprintf("%s.%d", uid, gen)
}
