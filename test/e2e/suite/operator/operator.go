// Package operator holds the smoke suite of the harness itself.
//
// It asserts that the Operator under test is the one the suites need: built from
// Dockerfile.bridge, installed with the bridge enabled, and granted the RBAC that
// ships in no release. Every other suite depends on all three, so when this one
// is red the rest are noise.
package operator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/test/e2e/framework"
)

// Where a missing permission comes from, so a failure names the fix. The
// v2alpha1 role ships in no release and is applied by hand; everything else
// arrives with the chart.
const (
	hintV2Alpha1 = "run `task install_v2alpha1_crds`"
	hintChart    = "the chart-granted manager role is incomplete"
)

// The verbs the cases ask about.
const (
	verbGet    = "get"
	verbList   = "list"
	verbWatch  = "watch"
	verbCreate = "create"
	verbUpdate = "update"
)

func init() {
	framework.Register("operator",
		"the Operator under test is bridge-enabled and reconciling",
		run)
}

func run(t *testing.T, env *framework.Env) {
	env.RequireAddons(t, framework.CertManager)

	// This is the assertion. RequireOperator installs or adopts the release, then
	// proves the Siphon controller is running: the chart value reached the pod, the
	// binary carries the bridge tag, and a Siphon really gets reconciled. It fails
	// with a distinct message for each of the three.
	env.RequireOperator(t)

	t.Run("the manager may write the resources it reconciles", func(t *testing.T) {
		serviceAccount := env.Config.NameOverride + "-manager"

		cases := []struct {
			what     string
			resource string
			verb     string
			// hint names what repairs the permission, because the two sources are
			// repaired differently: the v2alpha1 role is applied by hand, and the
			// chart role arrives with the release.
			hint string
		}{
			{"list siphons", "siphons", verbList, hintV2Alpha1},
			{"watch siphons", "siphons", verbWatch, hintV2Alpha1},
			{"update the status of a siphon", "siphons/status", verbUpdate, hintV2Alpha1},
			{"update the finalizers of a siphon", "siphons/finalizers", verbUpdate, hintV2Alpha1},
			{"list gitlabcores", "gitlabcores", verbList, hintV2Alpha1},
			{"watch gitlabcores", "gitlabcores", verbWatch, hintV2Alpha1},
			{"update the status of a gitlabcore", "gitlabcores/status", verbUpdate, hintV2Alpha1},
			{"update the finalizers of a gitlabcore", "gitlabcores/finalizers", verbUpdate, hintV2Alpha1},
			{"create the ConfigMap it publishes the tables to", "configmaps", verbCreate, hintV2Alpha1},
			{"read the Secret it pulls the tables with", "secrets", verbGet, hintV2Alpha1},
			// Neither of these is in the chart-granted role, and both fail late:
			// a PodDisruptionBudget the Siphon chart renders fails the apply of
			// the whole release, and a Pod the manager cannot read costs the
			// status the reason a workload is not ready.
			{"create the PodDisruptionBudget of a release", "poddisruptionbudgets", verbCreate, hintV2Alpha1},
			{"read a Pod to report why a workload is not ready", "pods", verbGet, hintV2Alpha1},
			// The deprecated v1beta1 definition, whose permissions come with the
			// chart rather than from the file the other cases need. A
			// SubjectAccessReview asks the authorizer and not the schema, so this
			// costs nothing on a cluster that does not serve the definition, and
			// it tells an incomplete chart role apart from a missing v2alpha1 one.
			{"list gitlabs", "gitlabs", verbList, hintChart},
		}

		for _, testCase := range cases {
			t.Run(testCase.what, func(t *testing.T) {
				name, subresource := splitResource(testCase.resource)

				attributes := authorizationv1.ResourceAttributes{
					Group:       resourceGroup(name),
					Resource:    name,
					Subresource: subresource,
					Verb:        testCase.verb,
				}

				allowed, reason, err := env.CanI(env.Ctx, serviceAccount, env.Config.OperatorNamespace, attributes)
				require.NoError(t, err)

				assert.True(t, allowed, "the manager ServiceAccount may not %s; %s: %s",
					testCase.what, testCase.hint, reason)
			})
		}
	})

	t.Run("preparing the Operator twice changes nothing", func(t *testing.T) {
		// Memoized on the harness, so this is the assertion that a warm rerun is
		// cheap rather than a second install.
		env.RequireOperator(t)
	})
}

// The resource strings above are `resource` or `resource/subresource`, and every
// resource is core, in policy, or in the Operator's own group. These helpers keep
// that mapping in one place rather than spelling out an attributes literal per
// case.
func splitResource(resource string) (name, subresource string) {
	name, subresource, _ = strings.Cut(resource, "/")

	return name, subresource
}

func resourceGroup(name string) string {
	switch name {
	case "configmaps", "secrets", "serviceaccounts", "pods":
		return ""
	case "poddisruptionbudgets":
		return "policy"
	default:
		return "apps.gitlab.com"
	}
}
