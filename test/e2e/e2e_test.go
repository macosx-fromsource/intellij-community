// Package e2e runs the black-box end-to-end suites.
//
// The harness deploys the Operator as an image and drives it through the API
// server, so a suite exercises the build tags, the RBAC and the manager wiring
// that a release actually ships. Nothing here runs a reconciler in the test
// process.
//
// The suites need a cluster and a built image, so TestE2E is gated on E2E rather
// than on a build tag: a tagged file is invisible to gopls unless every editor
// sets its build flags, and to golangci-lint unless the tag is on its command
// line. Run them with `task e2e-suite`.
package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/test/e2e/framework"

	// Every suite registers itself from its own init(), reached through this
	// blank import. A suite that is not imported there does not exist.
	_ "gitlab.com/gitlab-org/cloud-native/gitlab-operator/test/e2e/suite"
)

// listSuites prints the registered suites and exits, so that `task e2e-suites`
// needs no cluster.
var listSuites = flag.Bool("list-suites", false, "print the registered suites and exit")

func TestMain(m *testing.M) {
	flag.Parse()

	if *listSuites {
		for _, suite := range framework.Suites() {
			fmt.Printf("%-20s %s\n", suite.Name, suite.Summary)
		}

		os.Exit(0)
	}

	os.Exit(m.Run())
}

// TestE2E runs every registered suite as a subtest.
//
// Continuous integration runs one suite per job with an anchored pattern:
//
//	go test -run 'TestE2E/^siphon$' ./test/e2e/
//
// The anchors matter. A pattern element is an unanchored regular expression, so
// `TestE2E/siphon` would also match siphon-release and siphon-datapath.
func TestE2E(t *testing.T) {
	cfg, err := framework.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.Enabled {
		t.Skipf("%s is not set; run the suites with `task e2e-suite`", framework.EnabledVar)
	}

	suites := framework.Suites()
	if len(suites) == 0 {
		t.Fatal("no suite is registered; add a blank import to test/e2e/suite/doc.go")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := framework.NewLogger(t)

	// The client libraries log through controller-runtime, and an unset logger
	// there turns every API warning into a stack trace on stderr.
	ctrl.SetLogger(log)

	harness, err := framework.NewHarness(ctx, cfg, log)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := harness.Close(context.WithoutCancel(ctx)); err != nil {
			t.Errorf("releasing the cluster: %v", err)
		}
	})

	log.Info("running the end-to-end suites",
		"provider", harness.Cluster.Provider(),
		"server", harness.RESTConfig.Host,
		"suites", len(suites))

	var ran int

	for _, suite := range suites {
		if framework.RunSuite(ctx, t, harness, suite) {
			ran++
		}
	}

	// A run in which every suite skipped is a run that tested nothing. Under
	// E2E_STRICT that is a misconfigured job rather than a green one.
	//
	// Guarded on t.Failed(), because a suite that failed also never finished, and
	// reporting that as "every suite skipped" buries the real failure under a
	// diagnosis of something that did not happen.
	if ran == 0 && cfg.Strict && !t.Failed() {
		t.Fatal("E2E_STRICT is set and every suite skipped")
	}
}
