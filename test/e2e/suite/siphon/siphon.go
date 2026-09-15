// Package siphon holds the reconcile-level end-to-end suite of the Siphon
// resource.
//
// It needs no GitLab. The reconciler records the topology and takes the finalizer
// before it resolves the reference, so everything a Siphon does while it waits for
// an instance is observable against an empty cluster, which is most of the
// contract: the schema the API server serves, the wait reasons, the fact that
// nothing is rendered early, and what deletion leaves behind.
//
// What needs a running GitLab lives in the siphonrelease suite.
package siphon

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/test/e2e/framework"
)

// noInitializedCondition is the reason several waits report, so it is a constant
// rather than three copies that could drift apart.
const noInitializedCondition = "it carries no Initialized condition yet"

func init() {
	framework.Register("siphon",
		"the Siphon reconcile contract that holds before a GitLab is available",
		run)
}

func run(t *testing.T, env *framework.Env) {
	env.RequireAddons(t, framework.CertManager)
	env.RequireOperator(t)
	env.CreateSiphonSecrets(t)

	// Split in three because the three answer different questions: what the API
	// server enforces on its own, what the reconciler records before it can
	// render, and what it does on the way out.
	t.Run("schema", func(t *testing.T) { schemaCases(t, env) })
	t.Run("waiting", func(t *testing.T) { waitingCases(t, env) })
	t.Run("deletion", func(t *testing.T) { deletionCases(t, env) })
}

// schemaCases covers what the API server enforces with no reconciler involved:
// the defaults, and the validation rules that keep an unworkable pipeline from
// being admitted at all.
func schemaCases(t *testing.T, env *framework.Env) {
	t.Run("the API server applies the defaults", func(t *testing.T) {
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace))
		live := env.GetSiphon(t, siphon.Name)

		assert.EqualValues(t, 5432, live.Spec.Source.Port)
		assert.Equal(t, "gitlabhq_production", live.Spec.Source.Database)
		assert.Equal(t, "siphon", live.Spec.Source.User)
		assert.EqualValues(t, "require", live.Spec.Source.SSLMode)
		assert.EqualValues(t, 1, live.Spec.Source.AdvisoryLockID)
		assert.EqualValues(t, 9000, live.Spec.Sink.Port)
		assert.False(t, live.Spec.Sink.SSL)

		// The interesting one: `tables` itself carries an explicit empty-object
		// default so that the default nested inside it fires. Without that, an
		// unset `tables` leaves the source empty rather than Auto.
		assert.EqualValues(t, "Auto", live.Spec.Tables.Source,
			"an unset spec.tables should still default its source")
	})

	t.Run("a name longer than 31 characters is rejected", func(t *testing.T) {
		// The Operator derives `<name>-siphon-reconciler-clickhouse-sa` from the
		// name, and that has to fit the 63 characters a label value allows.
		tooLong := strings.Repeat("a", 32)

		err := env.Client.Create(env.Ctx, framework.NewSiphon(tooLong, env.Namespace))
		require.Error(t, err, "a 32-character name should be rejected")
		assert.Contains(t, err.Error(), "31")
	})

	t.Run("a queue URL that is not nats is rejected", func(t *testing.T) {
		siphon := framework.NewSiphon(name(t), env.Namespace)
		siphon.Spec.Queue.URL = "tls://nats.nats.svc.cluster.local:4222"

		err := env.Client.Create(env.Ctx, siphon)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.queue.url")
	})

	t.Run("the ClickHouse HTTP port is rejected", func(t *testing.T) {
		// 8123 is the HTTP interface, which Siphon does not speak. Accepting it
		// would produce a pipeline that fails at connect time instead of at admit
		// time.
		siphon := framework.NewSiphon(name(t), env.Namespace)
		siphon.Spec.Sink.Port = 8123

		err := env.Client.Create(env.Ctx, siphon)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "http")
	})

	t.Run("the source database is immutable", func(t *testing.T) {
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace))

		// An edit that leaves the database alone has to pass: the rule compares the
		// field with its old self, so it must not trip on an unrelated change.
		require.NoError(t, env.UpdateSiphon(siphon.Name, func(live *apiv2alpha1.Siphon) {
			live.Spec.Source.User = "siphon_two"
		}), "editing another field should not trip the immutability rule")

		err := env.UpdateSiphon(siphon.Name, func(live *apiv2alpha1.Siphon) {
			live.Spec.Source.Database = "somethingelse"
		})
		require.Error(t, err, "changing the database re-snapshots every table, so it is rejected")
		assert.Contains(t, err.Error(), "immutable")
	})

	t.Run("the required fields are rejected when absent", func(t *testing.T) {
		cases := map[string]func(*apiv2alpha1.Siphon){
			"gitlabRef.name":           func(s *apiv2alpha1.Siphon) { s.Spec.GitLabRef.Name = "" },
			"source.host":              func(s *apiv2alpha1.Siphon) { s.Spec.Source.Host = "" },
			"source.passwordSecretRef": func(s *apiv2alpha1.Siphon) { s.Spec.Source.PasswordSecretRef = apiv2alpha1.SecretKeySelector{} },
			"queue.url":                func(s *apiv2alpha1.Siphon) { s.Spec.Queue.URL = "" },
			"sink.host":                func(s *apiv2alpha1.Siphon) { s.Spec.Sink.Host = "" },
			"sink.database":            func(s *apiv2alpha1.Siphon) { s.Spec.Sink.Database = "" },
			"sink.username":            func(s *apiv2alpha1.Siphon) { s.Spec.Sink.Username = "" },
			"sink.passwordSecretRef":   func(s *apiv2alpha1.Siphon) { s.Spec.Sink.PasswordSecretRef = apiv2alpha1.SecretKeySelector{} },
		}

		for field, clear := range cases {
			t.Run(field, func(t *testing.T) {
				siphon := framework.NewSiphon(name(t), env.Namespace)
				clear(siphon)

				assert.Error(t, env.Client.Create(env.Ctx, siphon),
					"%s is required, so an absent one should be rejected", field)
			})
		}
	})
}

// waitingCases covers what a Siphon does while it has no instance to resolve,
// which is everything the reconciler records before it renders.
func waitingCases(t *testing.T, env *framework.Env) {
	t.Run("the reconciler takes the finalizer", func(t *testing.T) {
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace))

		env.AwaitSiphon(t, siphon.Name, "the finalizer is on the resource", framework.ShortTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				for _, finalizer := range live.Finalizers {
					if finalizer == framework.SiphonFinalizer {
						return true, ""
					}
				}

				return false, fmt.Sprintf("its finalizers are %v", live.Finalizers)
			})
	})

	t.Run("the topology is recorded whatever the reference resolves to", func(t *testing.T) {
		// The reconciler records these before it looks the instance up, which is why
		// this holds with no GitLab in the cluster. They are also the names a
		// database administrator has to know, so a rename is a breaking change.
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace))

		live := env.AwaitSiphon(t, siphon.Name, "the topology is recorded", framework.ShortTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				if live.Status.Publication == "" {
					return false, "its status carries no publication yet"
				}

				return true, ""
			})

		assert.Equal(t, framework.ExpectedPublication, live.Status.Publication)
		assert.Equal(t, framework.ExpectedSlot, live.Status.ReplicationSlot)
		assert.Equal(t, framework.ExpectedStream, live.Status.StreamName)
	})

	t.Run("a missing instance parks the pipeline in Preparing", func(t *testing.T) {
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace,
			framework.WithGitLabRef("no-such-gitlab")))

		live := env.AwaitSiphon(t, siphon.Name, "the wait reason is GitLabNotFound", framework.MediumTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				condition, found := framework.ConditionReason(live, framework.SiphonConditionInitialized)
				if !found {
					return false, noInitializedCondition
				}

				return condition.Reason == framework.SiphonReasonGitLabNotFound,
					fmt.Sprintf("the reason is %s", condition.Reason)
			})

		assert.Equal(t, framework.SiphonPhasePreparing, live.Status.Phase)

		condition, _ := framework.ConditionReason(live, framework.SiphonConditionInitialized)
		assert.Equal(t, metav1.ConditionFalse, condition.Status)
		assert.Contains(t, condition.Message, "no-such-gitlab",
			"the message should name the reference that did not resolve")
		assert.Equal(t, live.Generation, condition.ObservedGeneration)
	})

	t.Run("nothing is rendered while the reference does not resolve", func(t *testing.T) {
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace,
			framework.WithGitLabRef("no-such-gitlab")))

		env.AwaitSiphon(t, siphon.Name, "the reconciler has run at least once", framework.MediumTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				return live.Status.Phase != "", "its phase is not set yet"
			})

		// Consistently rather than once: a Deployment that is merely late would slip
		// past a single read.
		framework.Consistently(env.Ctx, t, 15*time.Second, "no workload is rendered",
			func(ctx context.Context) (bool, string, error) {
				deployments := &appsv1.DeploymentList{}
				if err := env.Client.List(ctx, deployments, client.InNamespace(env.Namespace)); err != nil {
					return false, "", err
				}

				if len(deployments.Items) == 0 {
					return true, "", nil
				}

				names := make([]string, 0, len(deployments.Items))
				for _, deployment := range deployments.Items {
					names = append(names, deployment.Name)
				}

				return false, "the namespace carries " + strings.Join(names, ", "), nil
			})

		live := env.GetSiphon(t, siphon.Name)
		assert.Empty(t, live.Status.Version, "the chart version is only recorded once the release is applied")
		assert.Empty(t, live.Status.GitLabVersion)
	})

	t.Run("the conditions that need a render are absent", func(t *testing.T) {
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace,
			framework.WithGitLabRef("no-such-gitlab")))

		live := env.AwaitSiphon(t, siphon.Name, "the reconciler has run at least once", framework.MediumTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				_, found := framework.ConditionReason(live, framework.SiphonConditionInitialized)

				return found, noInitializedCondition
			})

		_, tablesResolved := framework.ConditionReason(live, framework.SiphonConditionTablesResolved)
		assert.False(t, tablesResolved, "TablesResolved is set only past the reference resolution")

		_, available := framework.ConditionReason(live, framework.SiphonConditionAvailable)
		assert.False(t, available, "Available is set only once the release is applied")
	})

	t.Run("an instance that is not ready reports GitLabNotReady", func(t *testing.T) {
		if !env.Config.SiphonGitLabRef {
			t.Skip("this creates a GitLabCore the Operator starts rendering, which pulls the toolbox " +
				"image; set E2E_SIPHON_GITLABREF=1 to run it, or rely on the siphonrelease suite")
		}

		core := env.CreateBareGitLabCore(t, "gitlab")

		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace,
			framework.WithGitLabRef(core.Name)))

		// The two wait reasons are the whole gating contract, so the suite has to
		// tell them apart rather than accept either.
		env.AwaitSiphon(t, siphon.Name, "the wait reason is GitLabNotReady", framework.LongTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				condition, found := framework.ConditionReason(live, framework.SiphonConditionInitialized)
				if !found {
					return false, noInitializedCondition
				}

				return condition.Reason == framework.SiphonReasonGitLabNotReady,
					fmt.Sprintf("the reason is %s", condition.Reason)
			})
	})

	t.Run("the pipeline is reconciled again on a timer", func(t *testing.T) {
		// The controller watches nothing but the resource, so the requeue is the
		// only drift repair there is. Clearing a status field and watching it come
		// back is the only way to see it.
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace))

		env.AwaitSiphon(t, siphon.Name, "the topology is recorded", framework.ShortTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				return live.Status.Publication != "", "its status carries no publication yet"
			})

		require.NoError(t, env.UpdateSiphonStatus(siphon.Name, func(live *apiv2alpha1.Siphon) {
			live.Status.Publication = ""
		}))

		// The requeue delay is 30 seconds, so a budget of two minutes leaves room
		// for a busy cluster without making a real stall look like a slow one.
		env.AwaitSiphon(t, siphon.Name, "the requeue restores the topology", framework.MediumTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				return live.Status.Publication == framework.ExpectedPublication,
					fmt.Sprintf("its publication is %q", live.Status.Publication)
			})
	})
}

// deletionCases covers what deletion leaves behind, which is the part an
// administrator has to act on.
func deletionCases(t *testing.T, env *framework.Env) {
	t.Run("deleting the pipeline warns about the retained slot and lets go", func(t *testing.T) {
		siphon := env.CreateSiphon(t, framework.NewSiphon(name(t), env.Namespace))

		env.AwaitSiphon(t, siphon.Name, "the finalizer is on the resource", framework.ShortTimeout,
			func(live *apiv2alpha1.Siphon) (bool, string) {
				return len(live.Finalizers) > 0, "it carries no finalizer yet"
			})

		require.NoError(t, env.Client.Delete(env.Ctx, siphon))

		// Dropping a replication slot is destructive and irreversible, so the
		// Operator refuses to do it and tells the administrator what to run. The
		// event has to carry the exact statements, because a half-remembered slot
		// name leaves a slot retaining write-ahead log forever.
		event := env.AwaitEvent(t, "Siphon", siphon.Name, framework.SiphonEventSlotRetained,
			framework.MediumTimeout)

		assert.Contains(t, event.Message, framework.ExpectedSlot)
		assert.Contains(t, event.Message, framework.ExpectedPublication)

		framework.Eventually(env.Ctx, t, framework.MediumTimeout, "the resource is gone",
			func(ctx context.Context) (bool, string, error) {
				live := &apiv2alpha1.Siphon{}
				key := client.ObjectKey{Namespace: env.Namespace, Name: siphon.Name}

				err := env.Client.Get(ctx, key, live)
				if apierrors.IsNotFound(err) {
					return true, "", nil
				}

				if err != nil {
					return false, "", err
				}

				return false, fmt.Sprintf("its finalizers are %v", live.Finalizers), nil
			})
	})
}

// name derives a resource name from the test, so that a case reads its own
// resource and the failure names the case. It has to stay within the 31
// characters the schema allows, which is why it is a hash rather than the test
// name.
func name(t *testing.T) string {
	t.Helper()

	return framework.ShortName("s", t.Name())
}
