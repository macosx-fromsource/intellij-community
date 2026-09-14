package framework

import (
	"context"
	"fmt"
	"testing"
)

// RunSuite runs one suite as a subtest and reports whether its body ran to
// completion rather than skipping.
//
// The namespace, the logger and the helm runner are built here rather than in the
// suite, so that a suite body starts with what it is testing. Cleanup functions
// unwind last-registered-first, so the diagnostics dump is registered after the
// namespace deletion and therefore runs before it: a dump of a deleted namespace
// says nothing.
func RunSuite(ctx context.Context, t *testing.T, harness *Harness, suite *Suite) bool {
	var ran bool

	t.Run(suite.Name, func(t *testing.T) {
		runID, err := RunID()
		if err != nil {
			t.Fatal(err)
		}

		log := NewLogger(t).WithValues("suite", suite.Name)
		log.Info(suite.Summary, "run", runID)

		suiteCtx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		env := &Env{
			Harness:   harness,
			Ctx:       suiteCtx,
			Namespace: fmt.Sprintf("e2e-%s-%s", suite.Name, runID),
			RunID:     runID,
		}

		env.Helm = &Helm{
			Binary:     harness.Config.HelmBinary,
			Kubeconfig: harness.Cluster.KubeconfigPath(),
			Context:    harness.Cluster.KubeContext(),
			Out:        NewTestWriter(t),
			Log:        log,
		}

		CreateNamespace(suiteCtx, t, harness.Client, env.Namespace, runID)

		t.Cleanup(func() {
			if harness.Config.KeepNamespace {
				log.Info("keeping the namespace", "namespace", env.Namespace)

				return
			}

			DeleteNamespace(t, harness.Client, env.Namespace)
		})

		// Registered after the deletion, so it runs before it.
		t.Cleanup(func() {
			if !t.Failed() {
				return
			}

			env.DumpDiagnostics(t)
		})

		suite.Run(t, env)

		ran = true
	})

	return ran
}
