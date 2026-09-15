package framework

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// The topology the Siphon reconciler derives from the name of a resource.
//
// These are string literals rather than imports of the constants in
// internal/controller/siphon on purpose. A black-box test that imports the values
// it asserts cannot catch a rename: the stream name in particular is hardcoded by
// the consumer downstream, so a change here has to break a test.
const (
	SiphonFinalizer     = "siphon.apps.gitlab.com/finalizer"
	ExpectedPublication = "siphon_producer_main"
	ExpectedSlot        = "siphon_producer_main_slot"
	ExpectedStream      = "siphon_stream_main_db"

	// The Deployment suffixes of the fixed topology: one producer, one ClickHouse
	// consumer, one reconciler.
	ProducerSuffix   = "-siphon-producer-main"
	ConsumerSuffix   = "-siphon-consumer-clickhouse"
	ReconcilerSuffix = "-siphon-reconciler-clickhouse"
)

// Condition types and phases of a Siphon.
const (
	SiphonConditionInitialized    = "Initialized"
	SiphonConditionTablesResolved = "TablesResolved"
	SiphonConditionAvailable      = "Available"

	SiphonPhasePreparing = "Preparing"
	SiphonPhaseRunning   = "Running"
	SiphonPhaseFailed    = "Failed"

	SiphonReasonGitLabNotFound = "GitLabNotFound"
	SiphonReasonGitLabNotReady = "GitLabNotReady"

	// SiphonEventSlotRetained warns that deleting a Siphon leaves the replication
	// slot and the publication behind, because dropping them is destructive.
	SiphonEventSlotRetained = "ReplicationSlotRetained"
)

// Names of the Secrets a Siphon references. The reconciler does not read them
// before it renders, but a realistic resource points at real ones.
const (
	sourceSecretName  = "siphon-source"
	sinkSecretName    = "siphon-sink"
	secretPasswordKey = "password"
)

// SiphonOption edits a Siphon before it is created, so that a test case can state
// only the field it is about.
type SiphonOption func(*apiv2alpha1.Siphon)

// NewSiphon builds a resource that satisfies every required field and leaves
// everything optional unset, so that the defaults of the API server are what a
// test reads back.
func NewSiphon(name, namespace string, options ...SiphonOption) *apiv2alpha1.Siphon {
	siphon := &apiv2alpha1.Siphon{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: apiv2alpha1.SiphonSpec{
			GitLabRef: apiv2alpha1.GitLabReference{Name: "gitlab"},
			Source: apiv2alpha1.PostgreSQLSourceSpec{
				Host: "postgresql.databases.svc.cluster.local",
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{
					Name: sourceSecretName,
					Key:  secretPasswordKey,
				},
			},
			Queue: apiv2alpha1.QueueSpec{URL: "nats://nats.nats.svc.cluster.local:4222"},
			Sink: apiv2alpha1.ClickHouseSinkSpec{
				Host:     "clickhouse.databases.svc.cluster.local",
				Database: "gitlab_clickhouse_main_production",
				Username: "gitlab",
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{
					Name: sinkSecretName,
					Key:  secretPasswordKey,
				},
			},
		},
	}

	for _, option := range options {
		option(siphon)
	}

	return siphon
}

// ProbeSiphon is the minimal resource the verification of the harness creates to
// prove the reconcile loop runs.
func ProbeSiphon(namespace string) *apiv2alpha1.Siphon {
	return NewSiphon("probe", namespace)
}

// WithChartVersion pins the Siphon chart, which the reconciler needs to render.
func WithChartVersion(version string) SiphonOption {
	return func(siphon *apiv2alpha1.Siphon) {
		siphon.Spec.Chart = apiv2alpha1.ChartSpec{Version: version}
	}
}

// WithGitLabRef points the resource at another instance.
func WithGitLabRef(name string) SiphonOption {
	return func(siphon *apiv2alpha1.Siphon) {
		siphon.Spec.GitLabRef.Name = name
	}
}

// CreateSiphonSecrets creates the Secrets a Siphon references.
func (e *Env) CreateSiphonSecrets(t *testing.T) {
	t.Helper()

	for _, name := range []string{sourceSecretName, sinkSecretName} {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: e.Namespace},
			StringData: map[string]string{secretPasswordKey: "not-a-real-password"},
		}

		if err := e.Client.Create(e.Ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
			t.Fatalf("creating the Secret %s: %v", name, err)
		}
	}
}

// GetSiphon reads the resource back, failing the test when it is gone.
func (e *Env) GetSiphon(t *testing.T, name string) *apiv2alpha1.Siphon {
	t.Helper()

	live := &apiv2alpha1.Siphon{}
	key := client.ObjectKey{Namespace: e.Namespace, Name: name}

	if err := e.Client.Get(e.Ctx, key, live); err != nil {
		t.Fatalf("reading the Siphon %s: %v", name, err)
	}

	return live
}

// SiphonPredicate decides whether the resource has reached the state a case is
// waiting for, and says what it looks like when it has not.
type SiphonPredicate func(siphon *apiv2alpha1.Siphon) (bool, string)

// AwaitSiphon polls the resource until the predicate holds, and returns the last
// state it read. The predicate returns a reason so that a timeout says what the
// status looked like rather than only that it timed out.
func (e *Env) AwaitSiphon(t *testing.T, name, what string, budget time.Duration, predicate SiphonPredicate) *apiv2alpha1.Siphon {
	t.Helper()

	last := &apiv2alpha1.Siphon{}
	key := client.ObjectKey{Namespace: e.Namespace, Name: name}

	Eventually(e.Ctx, t, budget, what, func(ctx context.Context) (bool, string, error) {
		if err := e.Client.Get(ctx, key, last); err != nil {
			return false, "", err
		}

		done, reason := predicate(last)

		return done, reason, nil
	})

	return last
}

// DeploymentNames are the three Deployments a rendered Siphon release carries.
func DeploymentNames(siphon string) []string {
	return []string{
		siphon + ProducerSuffix,
		siphon + ConsumerSuffix,
		siphon + ReconcilerSuffix,
	}
}

// ConditionReason returns the reason of the condition, and reports whether the
// condition is present at all: the reconciler sets TablesResolved and Available
// only once it has rendered, so their absence is itself an assertion.
func ConditionReason(siphon *apiv2alpha1.Siphon, conditionType string) (metav1.Condition, bool) {
	for _, condition := range siphon.Status.Conditions {
		if condition.Type == conditionType {
			return condition, true
		}
	}

	return metav1.Condition{}, false
}

// CreateSiphon creates the resource and returns it, so that a case reads as one
// statement.
func (e *Env) CreateSiphon(t *testing.T, siphon *apiv2alpha1.Siphon) *apiv2alpha1.Siphon {
	t.Helper()

	if err := e.Client.Create(e.Ctx, siphon); err != nil {
		t.Fatalf("creating the Siphon %s: %v", siphon.Name, err)
	}

	return siphon
}

// ShortName derives a stable, short name from a seed.
//
// The schema rejects a Siphon whose name is longer than 31 characters, because
// the Operator appends up to 32 to it and the result has to fit a label value. A
// test name is nearly always longer than that, so it is hashed rather than
// truncated: truncating collides between two cases of the same parent.
func ShortName(prefix, seed string) string {
	sum := sha256.Sum256([]byte(seed))

	return prefix + "-" + hex.EncodeToString(sum[:6])
}

// UpdateSiphon applies the mutation, retrying on a conflict, and returns whatever
// the API server made of it.
//
// The retry is not optional. The reconciler writes the finalizer and the status of
// the same resource, so a read followed by a write races with it and loses often
// enough to be a permanent flake rather than an occasional one. Retrying re-reads
// and re-applies the mutation, which is what makes an assertion about a validation
// rule an assertion about that rule rather than about timing.
func (e *Env) UpdateSiphon(name string, mutate func(siphon *apiv2alpha1.Siphon)) error {
	key := client.ObjectKey{Namespace: e.Namespace, Name: name}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		live := &apiv2alpha1.Siphon{}
		if err := e.Client.Get(e.Ctx, key, live); err != nil {
			return err
		}

		mutate(live)

		return e.Client.Update(e.Ctx, live)
	})
}

// UpdateSiphonStatus is UpdateSiphon for the status subresource.
func (e *Env) UpdateSiphonStatus(name string, mutate func(siphon *apiv2alpha1.Siphon)) error {
	key := client.ObjectKey{Namespace: e.Namespace, Name: name}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		live := &apiv2alpha1.Siphon{}
		if err := e.Client.Get(e.Ctx, key, live); err != nil {
			return err
		}

		mutate(live)

		return e.Client.Status().Update(e.Ctx, live)
	})
}
