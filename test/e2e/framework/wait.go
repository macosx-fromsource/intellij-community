package framework

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// Polling intervals. The interval is short because everything polled here is a
// local API server, and the budgets are generous because the Siphon reconciler
// requeues every 30 seconds and a cold cluster pulls images.
const (
	PollInterval = 500 * time.Millisecond

	// ShortTimeout covers a reconcile that has already been triggered.
	ShortTimeout = time.Minute

	// MediumTimeout covers a reconcile that has to wait for the requeue.
	MediumTimeout = 2 * time.Minute

	// LongTimeout covers a workload that has to pull an image and roll out.
	LongTimeout = 10 * time.Minute
)

// Condition is what Eventually and Consistently poll. It returns a reason as well
// as a verdict so that a failure says what the cluster looked like on the last
// attempt: "the phase is Preparing" beats "timed out".
type Condition func(ctx context.Context) (bool, string, error)

// Eventually polls the condition until it holds or the budget runs out, and
// fails the test with the last reason the condition produced.
func Eventually(ctx context.Context, t *testing.T, budget time.Duration, what string, condition Condition) {
	t.Helper()

	var reason string

	err := wait.PollUntilContextTimeout(ctx, PollInterval, budget, true,
		func(ctx context.Context) (bool, error) {
			done, why, err := condition(ctx)
			reason = why

			return done, err
		})

	switch {
	case err == nil:
		return
	case errors.Is(err, context.DeadlineExceeded), wait.Interrupted(err):
		t.Fatalf("timed out after %s waiting until %s: %s", budget, what, orUnknown(reason))
	default:
		t.Fatalf("waiting until %s: %v", what, err)
	}
}

// Consistently polls the condition for the whole budget and fails as soon as it
// stops holding. It is how the harness asserts that something the reconciler
// could have created is really absent, rather than merely late.
func Consistently(ctx context.Context, t *testing.T, budget time.Duration, what string, condition Condition) {
	t.Helper()

	deadline := time.Now().Add(budget)

	for time.Now().Before(deadline) {
		holds, reason, err := condition(ctx)
		if err != nil {
			t.Fatalf("checking that %s: %v", what, err)
		}

		if !holds {
			t.Fatalf("expected %s throughout %s, and it stopped after %s: %s",
				what, budget, time.Until(deadline), orUnknown(reason))
		}

		select {
		case <-ctx.Done():
			t.Fatalf("checking that %s: %v", what, ctx.Err())
		case <-time.After(PollInterval):
		}
	}
}

func orUnknown(reason string) string {
	if reason == "" {
		return "no reason recorded"
	}

	return reason
}

// reasonf is sugar for a condition that wants a formatted reason.
func reasonf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
