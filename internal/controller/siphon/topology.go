package siphon

import "fmt"

// The deployment topology of a Siphon release, fixed rather than configurable.
//
// Siphon shards by application identifier: the layout names one per producer,
// consumer and reconciler, and each identifier becomes one Deployment. Replicas
// buy active/standby failover and never parallelism, because the producer holds
// a PostgreSQL advisory lock and the consumer a NATS JetStream key-value lock,
// so scaling out means adding identifiers, not replicas.
//
// This is the single-shard layout GitLab is known to run. Nothing in the
// specification changes it. Overriding siphonLayoutConfigMap.data through
// spec.chart.values is the way to a sharded deployment, and it is a deliberate
// act: rendezvous hashing reassigns tables when the identifier set changes, and
// for a producer that means a new publication and a new replication slot.
//
// The identifiers use underscores, not hyphens. The producer identifier becomes
// the publication name and, with a _slot suffix, the replication slot name, and a
// hyphen is a syntax error in a slot name. The chart's own examples use hyphens;
// this must not copy them.
const (
	// producerID reads the logical replication stream. It is also the name of
	// the publication the administrator pre-creates, which has to be owned by
	// the owner of the siphon_alter_publication function, since SECURITY DEFINER
	// makes that the role the ADD TABLE runs as. Owned by anyone else, every
	// ADD TABLE fails with "must be owner of publication" while the workload
	// still reports Running. A name that does not match this identifier fails
	// loudly instead: the producer tries to create the publication itself and
	// the login role has no CREATE on the database.
	producerID = "siphon_producer_main"

	// consumerID applies the events to ClickHouse.
	consumerID = "siphon_consumer_clickhouse"

	// reconcilerID reconciles ClickHouse against the source on a schedule.
	reconcilerID = "siphon_reconciler_clickhouse"

	// streamName is the NATS JetStream stream the events pass through. The
	// code-indexing dispatcher of the Knowledge Graph hardcodes it, so renaming
	// it breaks code indexing.
	streamName = "siphon_stream_main_db"

	// sourceDB is the key of the source connection in the connection base. The
	// layout maps the ci and sec logical databases onto it, because a
	// self-managed instance is not decomposed.
	sourceDB = "main"
)

// replicationSlot is the slot the producer holds on the source server, as
// DeploymentLayout derives it from the producer identifier.
const replicationSlot = producerID + "_slot"

// The deployment keys of the release.
//
// The chart names the Deployment, the ServiceAccount and the pod selector from
// these verbatim, so they carry the name of the resource: without it two Siphon
// resources in one namespace would collide on all three, and a pod selector is
// immutable once a Deployment exists. The name of the resource is capped at 31
// characters for the same reason: the longest name derived from it is the
// ServiceAccount of the reconciler, <name>-siphon-reconciler-clickhouse-sa, and a
// label value holds 63 characters.
func producerDeployment(name string) string   { return name + "-siphon-producer-main" }
func consumerDeployment(name string) string   { return name + "-siphon-consumer-clickhouse" }
func reconcilerDeployment(name string) string { return name + "-siphon-reconciler-clickhouse" }

// maxResourceNameLength is the longest name a Siphon resource may carry, which a
// CEL rule on the resource enforces. It is what leaves room for the longest name
// derived from it inside the 63 characters a label value holds.
const maxResourceNameLength = 63 - len("-siphon-reconciler-clickhouse-sa")

// exampleDeployment is the reference producer the chart ships commented out. It
// is disabled explicitly, so the release holds only what the Operator asked for
// whatever the chart defaults to.
const exampleDeployment = "example-postgres-producer"

// The Siphon roles, as the chart and the schema generator name them.
const (
	roleProducer   = "producer"
	roleConsumer   = "consumer"
	roleReconciler = "reconciler"
)

// deployment is one workload of the release: the chart key, the role it runs, and
// the application identifier the layout gives it.
type deployment struct {
	key            string
	role           string
	applicationID  string
	credentialName string
}

// deployments returns the workloads of the release for a resource, in the order
// data flows through them.
func deployments(name string) []deployment {
	return []deployment{
		{key: producerDeployment(name), role: roleProducer, applicationID: producerID, credentialName: sourcePasswordEnv},
		{key: consumerDeployment(name), role: roleConsumer, applicationID: consumerID, credentialName: sinkPasswordEnv},
		{key: reconcilerDeployment(name), role: roleReconciler, applicationID: reconcilerID, credentialName: sinkPasswordEnv},
	}
}

// gatedDeployments returns the workloads whose readiness decides whether the
// release is available: the producer and the consumer.
//
// The reconciler is left out on purpose. Its health endpoints are placeholders
// that answer 200 unconditionally, so its readiness proves only that a process
// is listening. The producer answers 503 until it holds the advisory lock and the
// consumer until it holds the key-value lock, which makes those two a real
// signal.
func gatedDeployments(name string) []string {
	return []string{producerDeployment(name), consumerDeployment(name)}
}

// The environment variables the configuration substitutes credentials from. The
// chart creates no Secret: each of these names a key of a Secret an administrator
// already created, and the value is resolved at startup.
const (
	sourcePasswordEnv = "SIPHON_DB_PASSWORD" //nolint:gosec // A variable name, not a credential.
	sinkPasswordEnv   = "CLICKHOUSE_PASSWORD"
	queueUsernameEnv  = "NATS_USERNAME"
	queuePasswordEnv  = "NATS_PASSWORD"
)

// placeholder wraps an environment variable name in the substitution syntax the
// configuration is expanded with at startup.
func placeholder(variable string) string {
	return fmt.Sprintf("${%s}", variable)
}
