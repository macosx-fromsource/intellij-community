// Package framework is the harness the end-to-end suites run on.
//
// A suite is a name, a summary and a function, registered from its own package's
// init() and run as one subtest of TestE2E. The harness deploys the Operator as
// an image and drives it through the API server: nothing here runs a reconciler
// in the test process, so a suite exercises the build tags, the RBAC and the
// manager wiring that a release actually ships.
//
// Everything a suite needs, it asks for imperatively in its own first lines
// through the Require methods of Env. There is no declarative manifest of
// requirements: a new kind of prerequisite is a new method, not a new field that
// the runner has to interpret.
package framework

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Suite is one end-to-end scenario.
type Suite struct {
	// Name selects the suite. Continuous integration runs one suite per job with
	// `go test -run 'TestE2E/^<name>$'`.
	Name string

	// Summary is one line, logged when the suite starts, so a failing job says
	// what it was trying to do.
	Summary string

	// Run is the body. The namespace, client and logger of the Env are already
	// scoped to this suite.
	Run func(t *testing.T, env *Env)
}

var (
	registryMu sync.Mutex
	registry   = map[string]*Suite{}
)

// Register adds a suite. It panics rather than returns an error: it runs at init
// time, where there is no test to fail, and every condition it rejects is a
// programming error in the file that calls it.
//
// A name that holds a space or a slash is rejected because both break the -run
// pattern the Taskfile builds: go test replaces a space with an underscore, and
// a slash starts a subtest of its own.
func Register(name, summary string, run func(t *testing.T, env *Env)) {
	registryMu.Lock()
	defer registryMu.Unlock()

	switch {
	case name == "":
		panic("framework: a suite needs a name")
	case strings.ContainsAny(name, " \t/"):
		panic(fmt.Sprintf("framework: the suite name %q holds a space or a slash", name))
	case summary == "":
		panic(fmt.Sprintf("framework: the suite %q needs a summary", name))
	case run == nil:
		panic(fmt.Sprintf("framework: the suite %q needs a body", name))
	}

	if _, found := registry[name]; found {
		panic(fmt.Sprintf("framework: the suite %q is registered twice", name))
	}

	registry[name] = &Suite{Name: name, Summary: summary, Run: run}
}

// Suites returns every registered suite, ordered by name so that a run is
// reproducible and the -run patterns in continuous integration are stable.
func Suites() []*Suite {
	registryMu.Lock()
	defer registryMu.Unlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}

	sort.Strings(names)

	suites := make([]*Suite, 0, len(names))
	for _, name := range names {
		suites = append(suites, registry[name])
	}

	return suites
}
