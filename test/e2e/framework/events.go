package framework

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AwaitEvent waits for an event with the reason, regarding the named object, and
// returns it.
//
// It polls rather than reads once, and it matches on the reason and the name
// rather than on a resource version, because the recorder batches: an event is
// emitted well before it is readable, and the object it regards is often deleted
// in between. The event outlives the object, so a match after the fact is still
// the right answer.
func (e *Env) AwaitEvent(t *testing.T, kind, name, reason string, budget time.Duration) corev1.Event {
	t.Helper()

	var found corev1.Event

	Eventually(e.Ctx, t, budget,
		reasonf("an event with the reason %s regards %s %s", reason, kind, name),
		func(ctx context.Context) (bool, string, error) {
			events := &corev1.EventList{}
			if err := e.Client.List(ctx, events, client.InNamespace(e.Namespace)); err != nil {
				return false, "", err
			}

			var seen []string

			for _, event := range events.Items {
				if event.InvolvedObject.Name != name || event.InvolvedObject.Kind != kind {
					continue
				}

				seen = append(seen, event.Reason)

				if event.Reason == reason {
					found = event

					return true, "", nil
				}
			}

			if len(seen) == 0 {
				return false, "no event regards it yet", nil
			}

			return false, reasonf("the reasons so far are %s", strings.Join(seen, ", ")), nil
		})

	return found
}
