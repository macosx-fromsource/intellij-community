package siphon

import (
	"fmt"
	"path"

	"github.com/pkg/errors"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// The chart value keys the specification maps onto.
const (
	configModeKey            = "configMode"
	namespaceKey             = "namespace"
	imageTagKey              = "image.tag"
	gitlabVersionKey         = "global.gitlabVersion"
	waitForMigrationsKey     = "waitForMigrations.enabled"
	connectionConfigMapKey   = "siphonConnectionConfigMap.create"
	layoutConfigMapKey       = "siphonLayoutConfigMap.create"
	connectionDataPrefix     = "siphonConnectionConfigMap.data"
	layoutDataPrefix         = "siphonLayoutConfigMap.data"
	deploymentsPrefix        = "deployments"
	podMonitorAPIVersionKind = "monitoring.coreos.com/v1/PodMonitor"
)

// siphonImageTag is the Siphon application version the Operator deploys.
//
// It is pinned here because there is nothing to derive it from. The chart
// defaults image.tag to null and declares no appVersion, and the published tags
// are 0.0.<n>-beta, unrelated to the chart version the resource names. An unset
// tag is not caught anywhere either: the chart interpolates "<repository>:", the
// render succeeds, and every pod fails InvalidImageName while the reconciler
// reports the release applied.
//
// Resolving it from the release manifest instead is
// https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/2208.
const siphonImageTag = "0.0.130-beta"

// configModeSplit is the mode the Operator renders in. In split mode the chart
// renders a connection base and a deployment layout, and each pod generates its
// half of the configuration at startup from the table definitions. Classic mode
// wants the whole configuration authored per deployment and does not track the
// deployed GitLab version, so the Operator does not use it.
const configModeSplit = "split"

// The keys of an envFromSecrets entry: the variable the configuration expands,
// and the Secret and key it is read from.
const (
	envNameKey   = "name"
	envSecretKey = "secretName"
	envKeyKey    = "secretKey"
)

// metricsPort is the port the chart serves metrics and the health endpoints on.
// It reaches the connection base as the Prometheus port and the pod as the
// container port the probes address.
const metricsPort = 8080

// natsCertsMountPath is where the chart mounts the NATS client certificate
// Secret. Each key of the Secret becomes a file of the same name, so the paths in
// the connection base are built from the key names.
const natsCertsMountPath = "/etc/ssl/certs/custom"

// queueDriver is the only queue Siphon speaks, and also the type of the object
// store it offloads an oversized event to.
const queueDriver = "nats"

// natsObjectStorageBucket is the JetStream object store the producer offloads an
// event too large for a stream message to.
const natsObjectStorageBucket = "siphon_oversize_bucket"

// The advisory and statement lock timings of the source connection.
//
// These are required fields of the producer configuration that the chart does not
// default in split mode: its appConfigDefaults supply them in classic mode only,
// and siphonConnectionDefaults covers the ClickHouse and replication blocks and
// nothing else. So the Operator writes them, using the values the chart uses in
// classic mode.
const (
	advisoryLockTimeoutMS          = 100
	advisoryLockTimeoutFuzzinessMS = 50
	lockTimeoutMS                  = 500
	lockTimeoutFuzzinessMS         = 300
)

// maxParallelWorkers is how many events a consumer applies concurrently.
//
// It belongs in the layout, not in the connection base. Siphon distributes only
// the queueing, clickhouse, replication and internal_events blocks from the
// connection base onto the component entries, so a top-level scalar there reaches
// nothing at all.
const maxParallelWorkers = 5

// refreshMode is how a refresh package reaches its consumer. Inline is the chart
// default and rides along the stream of the target; separate gives it a stream of
// its own, which the upstream generator recommends once a deployment is large
// enough to need it. That is a sharding decision, so it stays with the layout an
// administrator overrides.
const refreshMode = "inline"

// Cluster is what the target cluster tells the values layer. It is the discovered
// facts the derived values branch on, passed in rather than read here so the
// mapping stays a pure function.
type Cluster struct {
	// ServesPodMonitor reports whether the cluster serves the Prometheus
	// operator API, which is the only reason to render a PodMonitor.
	ServesPodMonitor bool
}

// Release is what the reconciler resolved before it derived any values: the
// GitLab application version the table definitions are pinned to, and where those
// definitions come from.
type Release struct {
	// GitLabVersion is the application version of the referenced instance,
	// without a leading v.
	GitLabVersion string

	// TablesImage is the fully resolved table definitions image reference.
	TablesImage string

	// TablesSource is the resolved source, never Auto.
	TablesSource apiv2alpha1.TablesSource

	// TablesConfigMap is the ConfigMap the extracted definitions were published
	// to. It is set only on the ConfigMap path, and the values layer does not
	// use it: the swap happens after the render, because the chart emits the
	// image volume unconditionally and appends rather than replaces the volumes
	// a value supplies.
	TablesConfigMap string
}

// EffectiveValues returns the values the chart is rendered with, in three layers:
//
//  1. The values derived from the structured fields of the specification.
//  2. spec.chart.values, merged over them. The free-form values win here, as
//     ADR 26 decides, which keeps them a working escape hatch. The layout is
//     among what they can replace: that is the route to a sharded topology.
//  3. The Operator overrides, which win over everything. Five of the six are
//     chart preconditions rather than policy: the chart fails a split render
//     without them.
func EffectiveValues(siphon *apiv2alpha1.Siphon, resolved Release, cluster Cluster) (support.Values, error) {
	values := support.Values{}

	setters := []func(support.Values, *apiv2alpha1.Siphon, Release, Cluster) error{
		setChartValues,
		setConnectionValues,
		setLayoutValues,
		setDeploymentValues,
	}

	for _, set := range setters {
		if err := set(values, siphon, resolved, cluster); err != nil {
			return nil, err
		}
	}

	if err := release.MergeUserValues(values, siphon.Spec.Chart.Values.Object); err != nil {
		return nil, err
	}

	return values, setOverrideValues(values, siphon)
}

// setChartValues sets what applies to the release as a whole.
func setChartValues(values support.Values, siphon *apiv2alpha1.Siphon, resolved Release, _ Cluster) error {
	// The chart stamps no namespace on what it renders and falls back to the
	// release namespace, which the reconciler also does when it applies. Setting
	// it makes the two agree explicitly.
	if err := values.SetValue(namespaceKey, siphon.Namespace); err != nil {
		return errors.Wrap(err, "setting the namespace")
	}

	// The tables image tag of the chart default. Each deployment also names the
	// image outright, so this is what a value the Operator does not reach falls
	// back to rather than the reference anything uses.
	if err := values.SetValue(gitlabVersionKey, resolved.GitLabVersion); err != nil {
		return errors.Wrap(err, "setting the GitLab version")
	}

	// A default rather than an override: an installation that has to run another
	// build of Siphon than the one the Operator pins sets it in
	// spec.chart.values, which is also the escape hatch while the tag is pinned
	// in source.
	if err := values.SetValue(imageTagKey, siphonImageTag); err != nil {
		return errors.Wrap(err, "setting the Siphon image tag")
	}

	return nil
}

// setConnectionValues derives the connection base, the document every component
// reads its endpoints and credentials from.
func setConnectionValues(values support.Values, siphon *apiv2alpha1.Siphon, _ Release, _ Cluster) error {
	set := func(key string, value interface{}) error {
		return values.SetValue(connectionDataPrefix+"."+key, value)
	}

	// The port the metrics and the health endpoints are served on. The pod
	// probes address it by name, so the two have to agree.
	if err := set("prometheus.port", metricsPort); err != nil {
		return errors.Wrap(err, "setting the metrics port")
	}

	if err := setQueueValues(set, siphon.Spec.Queue); err != nil {
		return err
	}

	// Without this the producer issues ALTER PUBLICATION directly, which requires
	// it to own the tables it adds. GitLab's tables are owned by the application
	// role, so the producer goes through the siphon_alter_publication SECURITY
	// DEFINER function instead, which the administrator created and which
	// executes as that role.
	if err := set("connection.replication.use_alter_publication_function", true); err != nil {
		return errors.Wrap(err, "setting the publication function")
	}

	if err := setSinkValues(set, siphon.Spec.Sink); err != nil {
		return err
	}

	return setSourceValues(set, siphon.Spec.Source)
}

// setQueueValues derives the NATS connection.
func setQueueValues(set func(string, interface{}) error, queue apiv2alpha1.QueueSpec) error {
	if err := set("connection.queueing.driver", queueDriver); err != nil {
		return errors.Wrap(err, "setting the queue driver")
	}

	if err := set("connection.queueing.url", queue.URL); err != nil {
		return errors.Wrap(err, "setting the queue URL")
	}

	// An event too large for a stream message is offloaded to a JetStream object
	// store, which is why the server needs JetStream rather than core NATS.
	objectStorage := map[string]interface{}{
		"identifier":  "nats-object-storage",
		"type":        queueDriver,
		"bucket_name": natsObjectStorageBucket,
	}

	if err := set("connection.queueing.object_storage_config", objectStorage); err != nil {
		return errors.Wrap(err, "setting the queue object storage")
	}

	if queue.Auth != nil {
		if err := set("connection.queueing.username", placeholder(queueUsernameEnv)); err != nil {
			return errors.Wrap(err, "setting the queue user name")
		}

		if err := set("connection.queueing.password", placeholder(queuePasswordEnv)); err != nil {
			return errors.Wrap(err, "setting the queue password")
		}
	}

	if queue.TLS == nil {
		return nil
	}

	// The chart mounts the Secret whole but injects the paths into the
	// configuration in classic mode only, so in split mode the Operator writes
	// them. Each key of the Secret is a file of the same name under the mount.
	paths := map[string]string{
		"connection.queueing.tls_config.ca_cert_path":     queue.TLS.CACertKey,
		"connection.queueing.tls_config.client_cert_path": queue.TLS.ClientCertKey,
		"connection.queueing.tls_config.client_key_path":  queue.TLS.ClientKeyKey,
	}

	for key, secretKey := range paths {
		if err := set(key, path.Join(natsCertsMountPath, secretKey)); err != nil {
			return errors.Wrapf(err, "setting %s", key)
		}
	}

	return nil
}

// setSinkValues derives the ClickHouse connection.
func setSinkValues(set func(string, interface{}) error, sink apiv2alpha1.ClickHouseSinkSpec) error {
	settings := map[string]interface{}{
		"connection.clickhouse.host":     sink.Host,
		"connection.clickhouse.port":     int(sink.Port),
		"connection.clickhouse.database": sink.Database,
		"connection.clickhouse.username": sink.Username,
		"connection.clickhouse.ssl":      sink.SSL,
		// The password is a placeholder the configuration expands at startup
		// from the environment, not the password itself.
		"connection.clickhouse.password": placeholder(sinkPasswordEnv),
	}

	for key, value := range settings {
		if err := set(key, value); err != nil {
			return errors.Wrapf(err, "setting %s", key)
		}
	}

	return nil
}

// setSourceValues derives the PostgreSQL connection.
func setSourceValues(set func(string, interface{}) error, source apiv2alpha1.PostgreSQLSourceSpec) error {
	prefix := "connection.databases." + sourceDB + "."

	settings := map[string]interface{}{
		"host":     source.Host,
		"port":     int(source.Port),
		"user":     source.User,
		"database": source.Database,
		// The key is ssl_mode, not sslmode: the chart's own split example writes
		// the latter, which the producer silently ignores, so a `require` intent
		// quietly degrades to whatever the driver defaults to.
		"ssl_mode": string(source.SSLMode),
		"password": placeholder(sourcePasswordEnv),

		// The producer takes this advisory lock to elect itself among its
		// replicas. It is scoped to the database, so two Siphon resources that
		// share it serialize against each other.
		"advisory_lock_id": int(source.AdvisoryLockID),

		// Required fields the chart does not default in split mode.
		"advisory_lock_timeout_ms":           advisoryLockTimeoutMS,
		"advisory_lock_timeout_fuzziness_ms": advisoryLockTimeoutFuzzinessMS,
		"lock_timeout_ms":                    lockTimeoutMS,
		"lock_timeout_fuzziness_ms":          lockTimeoutFuzzinessMS,

		// What the source server reports the connection as, which is how an
		// administrator finds it in pg_stat_activity.
		"application_name": producerID,
	}

	for key, value := range settings {
		if err := set(prefix+key, value); err != nil {
			return errors.Wrapf(err, "setting %s", prefix+key)
		}
	}

	return nil
}

// setLayoutValues derives the deployment layout, the document that assigns tables
// to application identifiers.
func setLayoutValues(values support.Values, _ *apiv2alpha1.Siphon, _ Release, _ Cluster) error {
	set := func(key string, value interface{}) error {
		return values.SetValue(layoutDataPrefix+"."+key, value)
	}

	settings := map[string]interface{}{
		"stream_name":  streamName,
		"refresh_mode": refreshMode,
		// A top-level scalar has to live here rather than in the connection
		// base, which distributes only whole blocks.
		"max_parallel_workers": maxParallelWorkers,

		"producers":   map[string]interface{}{sourceDB: []interface{}{producerID}},
		"consumers":   []interface{}{consumerID},
		"reconcilers": []interface{}{reconcilerID},

		// A self-managed instance is not decomposed, so the ci and sec logical
		// databases resolve to the one connection.
		"database_mapping": map[string]interface{}{"ci": sourceDB, "sec": sourceDB},
	}

	for key, value := range settings {
		if err := set(key, value); err != nil {
			return errors.Wrapf(err, "setting the layout %s", key)
		}
	}

	return nil
}

// setDeploymentValues derives the three workloads of the release.
func setDeploymentValues(values support.Values, siphon *apiv2alpha1.Siphon, resolved Release, cluster Cluster) error {
	// The chart ships a reference producer commented out. Disabling it says so
	// outright rather than relying on what the chart defaults it to.
	if err := values.SetValue(deploymentKey(exampleDeployment, "enabled"), false); err != nil {
		return errors.Wrap(err, "disabling the reference deployment")
	}

	for _, workload := range deployments(siphon.Name) {
		if err := setOneDeployment(values, siphon, resolved, cluster, workload); err != nil {
			return err
		}
	}

	return nil
}

// setOneDeployment derives one workload.
func setOneDeployment(values support.Values, siphon *apiv2alpha1.Siphon, resolved Release, cluster Cluster, workload deployment) error {
	set := func(key string, value interface{}) error {
		return values.SetValue(deploymentKey(workload.key, key), value)
	}

	settings := map[string]interface{}{
		"enabled": true,
		// Naming the image outright rather than leaving it to
		// global.gitlabVersion keeps the tag unambiguous: that value is also the
		// version the chart's migration wait selects on, and the two want
		// different strings.
		"split.tablesImage": resolved.TablesImage,
		"podmonitor":        cluster.ServesPodMonitor,
	}

	for key, value := range settings {
		if err := set(key, value); err != nil {
			return errors.Wrapf(err, "setting %s of %s", key, workload.key)
		}
	}

	if err := set("envFromSecrets", envFromSecrets(siphon, workload)); err != nil {
		return errors.Wrapf(err, "setting the credentials of %s", workload.key)
	}

	if siphon.Spec.Queue.TLS != nil {
		if err := set("natsClientCertsSecretName", siphon.Spec.Queue.TLS.SecretName); err != nil {
			return errors.Wrapf(err, "setting the queue certificate of %s", workload.key)
		}
	}

	if siphon.Spec.Tables.PullSecretRef == nil {
		return nil
	}

	// The chart renders no imagePullSecrets, but it passes podSpec through
	// verbatim. That is the only route a pull secret has to an image volume,
	// which the kubelet pulls with the credentials of the pod.
	pullSecrets := []interface{}{map[string]interface{}{envNameKey: siphon.Spec.Tables.PullSecretRef.Name}}

	return errors.Wrapf(set("podSpec.imagePullSecrets", pullSecrets),
		"setting the pull secret of %s", workload.key)
}

// envFromSecrets builds the credential wiring of one workload: the environment
// variables the configuration expands, each reading a key of a Secret an
// administrator created.
//
// Every workload carries the queue credentials, because the connection base
// distributes the queueing block to every component. An unset variable expands to
// nothing rather than failing, so a workload that does not need one is unharmed.
func envFromSecrets(siphon *apiv2alpha1.Siphon, workload deployment) []interface{} {
	entry := func(variable string, ref apiv2alpha1.SecretKeySelector) interface{} {
		return map[string]interface{}{
			envNameKey:   variable,
			envSecretKey: ref.Name,
			envKeyKey:    ref.Key,
		}
	}

	credential := siphon.Spec.Sink.PasswordSecretRef
	if workload.credentialName == sourcePasswordEnv {
		credential = siphon.Spec.Source.PasswordSecretRef
	}

	entries := []interface{}{entry(workload.credentialName, credential)}

	if siphon.Spec.Queue.Auth != nil {
		entries = append(entries,
			entry(queueUsernameEnv, siphon.Spec.Queue.Auth.UsernameSecretRef),
			entry(queuePasswordEnv, siphon.Spec.Queue.Auth.PasswordSecretRef))
	}

	return entries
}

// setOverrideValues sets what an instance may not choose, because the chart or
// the Operator, not the administrator, owns what it configures.
//
// The list is deliberately short. Five entries are chart preconditions: the chart
// fails a split render outright without them, so they are not a choice. The sixth
// disables a wait the chart cannot perform here.
func setOverrideValues(values support.Values, siphon *apiv2alpha1.Siphon) error {
	overrides := map[string]interface{}{
		// Split mode is what tracks the deployed GitLab version, and the chart
		// fails a render that mixes the two modes.
		configModeKey: configModeSplit,

		// The chart fails a split render unless it is rendering both documents.
		connectionConfigMapKey: true,
		layoutConfigMapKey:     true,

		// The chart's own migration wait cannot work against an
		// Operator-rendered GitLab. Its selector does match now, as the GitLab
		// chart emits the gitlab.com/target-version label it reads from 10.3 on
		// (gitlab-org/charts/gitlab!5218), but the Role and RoleBinding that let
		// the init container list Jobs are RBAC, which release.NeverApplied
		// skips, so the pod cannot run the query. Left on, every pod blocks to
		// its timeout and restarts, forever. The wait lives in the reconciler
		// instead: nothing is applied until the referenced instance reports
		// available.
		waitForMigrationsKey: false,
	}

	for key, value := range overrides {
		if err := values.SetValue(key, value); err != nil {
			return errors.Wrapf(err, "setting %s", key)
		}
	}

	for _, workload := range deployments(siphon.Name) {
		// A role that disagrees with the layout produces a pod generating
		// configuration for an identifier no layout names, which is a silent
		// no-op rather than an error.
		if err := values.SetValue(deploymentKey(workload.key, "split.role"), workload.role); err != nil {
			return errors.Wrapf(err, "setting the role of %s", workload.key)
		}

		if err := values.SetValue(deploymentKey(workload.key, configModeKey), configModeSplit); err != nil {
			return errors.Wrapf(err, "setting the config mode of %s", workload.key)
		}
	}

	return nil
}

// deploymentKey builds the dotted value key of one setting of one deployment.
func deploymentKey(name, key string) string {
	return fmt.Sprintf("%s.%s.%s", deploymentsPrefix, name, key)
}
