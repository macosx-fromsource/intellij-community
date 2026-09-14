package siphon

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// pgIdentifier is what PostgreSQL accepts unquoted, and what a replication slot
// name has to match: lowercase letters, digits and underscores, not starting with
// a digit. A hyphen is a syntax error there.
var pgIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func TestIdentifierShape(t *testing.T) {
	// The producer identifier becomes the publication name and, with a _slot
	// suffix, the replication slot name. The chart's own examples use hyphens,
	// which produce an invalid slot name, so this is the guard against copying
	// them.
	identifiers := map[string]string{
		"the producer":        producerID,
		"the consumer":        consumerID,
		"the reconciler":      reconcilerID,
		"the stream":          streamName,
		"the source database": sourceDB,
		"the slot":            replicationSlot,
	}

	for name, identifier := range identifiers {
		t.Run(name+" uses underscores", func(t *testing.T) {
			assert.Regexp(t, pgIdentifier, identifier)
			assert.NotContains(t, identifier, "-")
		})
	}
}

func TestReplicationSlot(t *testing.T) {
	t.Run("is the producer identifier with the suffix Siphon appends", func(t *testing.T) {
		assert.Equal(t, producerID+"_slot", replicationSlot)
	})
}

func TestDeploymentKeys(t *testing.T) {
	t.Run("carry the name of the resource", func(t *testing.T) {
		// The chart names the Deployment, the ServiceAccount and the pod
		// selector from these verbatim, so two resources in one namespace would
		// otherwise collide on all three, and a pod selector is immutable.
		first := deployments("one")
		second := deployments("two")

		for index := range first {
			assert.NotEqual(t, first[index].key, second[index].key)
			assert.True(t, strings.HasPrefix(first[index].key, "one-"), first[index].key)
		}
	})

	t.Run("fit a label value at the longest name the API allows", func(t *testing.T) {
		// The resource name is capped at maxResourceNameLength by a CEL rule for
		// exactly this reason. A pod selector value cannot exceed 63 characters,
		// and the longest name derived from the resource is the ServiceAccount of
		// the reconciler.
		longest := strings.Repeat("x", maxResourceNameLength)

		for _, workload := range deployments(longest) {
			assert.LessOrEqual(t, len(workload.key), 63, workload.key)
		}

		// And the ServiceAccount the chart derives from it, which is the longer
		// of the two.
		for _, workload := range deployments(longest) {
			assert.LessOrEqual(t, len(workload.key+"-sa"), 63, workload.key+"-sa")
		}
	})

	t.Run("are Kubernetes names, so they use hyphens", func(t *testing.T) {
		for _, workload := range deployments(testName) {
			assert.NotContains(t, workload.key, "_")
		}
	})
}

func TestGatedDeployments(t *testing.T) {
	t.Run("excludes the reconciler, whose health endpoints are placeholders", func(t *testing.T) {
		// Siphon answers 200 unconditionally on both endpoints for that role, so
		// its readiness proves only that a process is listening.
		gated := gatedDeployments(testName)

		assert.Contains(t, gated, producerDeployment(testName))
		assert.Contains(t, gated, consumerDeployment(testName))
		assert.NotContains(t, gated, reconcilerDeployment(testName))
	})
}

func TestDeployments(t *testing.T) {
	t.Run("gives each workload a distinct role and identifier", func(t *testing.T) {
		roles := map[string]string{}
		identifiers := map[string]string{}

		for _, workload := range deployments(testName) {
			assert.NotContains(t, roles, workload.role, "a role appears twice")
			assert.NotContains(t, identifiers, workload.applicationID, "an identifier appears twice")

			roles[workload.role] = workload.key
			identifiers[workload.applicationID] = workload.key
		}

		assert.Len(t, roles, 3)
	})

	t.Run("reads the producer from PostgreSQL and writes the rest to ClickHouse", func(t *testing.T) {
		for _, workload := range deployments(testName) {
			if workload.role == roleProducer {
				assert.Equal(t, sourcePasswordEnv, workload.credentialName)

				continue
			}

			assert.Equal(t, sinkPasswordEnv, workload.credentialName)
		}
	})
}

func TestPlaceholder(t *testing.T) {
	t.Run("wraps a variable in the substitution syntax", func(t *testing.T) {
		// The configuration is expanded at startup, so what reaches the document
		// is the placeholder rather than the credential.
		assert.Equal(t, "${SIPHON_DB_PASSWORD}", placeholder(sourcePasswordEnv))
	})
}
