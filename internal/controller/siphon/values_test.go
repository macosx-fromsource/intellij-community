package siphon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// value reads a dotted key off a derived values document.
func value(t *testing.T, values support.Values, key string) interface{} {
	t.Helper()

	result, err := values.GetValue(key)
	require.NoError(t, err, key)

	return result
}

// assertAbsent asserts a dotted key carries no value. GetValue answers nil
// without an error for a missing leaf, so absence is a nil value rather than a
// failed lookup.
func assertAbsent(t *testing.T, values support.Values, key, why string) {
	t.Helper()

	result, err := values.GetValue(key)

	require.NoError(t, err, key)
	assert.Nil(t, result, "%s: %s", key, why)
}

// derive builds the values for a resource.
func derive(t *testing.T, siphon *apiv2alpha1.Siphon, options ...func(*Release, *Cluster)) support.Values {
	t.Helper()

	resolved := testRelease()
	cluster := Cluster{}

	for _, option := range options {
		option(&resolved, &cluster)
	}

	values, err := EffectiveValues(siphon, resolved, cluster)
	require.NoError(t, err)

	return values
}

func TestEffectiveValuesConnection(t *testing.T) {
	values := derive(t, newSiphon())

	t.Run("points the source at the server the specification names", func(t *testing.T) {
		prefix := connectionDataPrefix + ".connection.databases." + sourceDB + "."

		assert.Equal(t, testSourceHost, value(t, values, prefix+"host"))
		assert.Equal(t, 5432, value(t, values, prefix+"port"))
		assert.Equal(t, "gitlabhq_production", value(t, values, prefix+"database"))
		assert.Equal(t, "siphon", value(t, values, prefix+"user"))
	})

	t.Run("writes ssl_mode, not sslmode", func(t *testing.T) {
		// The chart's own split example writes sslmode, which the producer
		// silently ignores, so a `require` intent quietly degrades to whatever
		// the driver defaults to.
		prefix := connectionDataPrefix + ".connection.databases." + sourceDB + "."

		assert.Equal(t, "require", value(t, values, prefix+"ssl_mode"))

		assertAbsent(t, values, prefix+"sslmode", "the producer reads ssl_mode, so sslmode must not be set")
	})

	t.Run("supplies the lock settings the chart does not default in split mode", func(t *testing.T) {
		// These are required fields of the producer configuration, and
		// siphonConnectionDefaults covers only the clickhouse and replication
		// blocks. A release without them does not start.
		prefix := connectionDataPrefix + ".connection.databases." + sourceDB + "."

		assert.Equal(t, 1, value(t, values, prefix+"advisory_lock_id"))
		assert.Equal(t, advisoryLockTimeoutMS, value(t, values, prefix+"advisory_lock_timeout_ms"))
		assert.Equal(t, advisoryLockTimeoutFuzzinessMS, value(t, values, prefix+"advisory_lock_timeout_fuzziness_ms"))
		assert.Equal(t, lockTimeoutMS, value(t, values, prefix+"lock_timeout_ms"))
		assert.Equal(t, lockTimeoutFuzzinessMS, value(t, values, prefix+"lock_timeout_fuzziness_ms"))
		assert.Equal(t, producerID, value(t, values, prefix+"application_name"))
	})

	t.Run("carries credentials as placeholders, never as values", func(t *testing.T) {
		// The chart creates no Secret: each credential reaches the pod as an
		// environment variable and is substituted at startup.
		assert.Equal(t, placeholder(sourcePasswordEnv),
			value(t, values, connectionDataPrefix+".connection.databases."+sourceDB+".password"))
		assert.Equal(t, placeholder(sinkPasswordEnv),
			value(t, values, connectionDataPrefix+".connection.clickhouse.password"))
	})

	t.Run("points the sink at the native protocol", func(t *testing.T) {
		prefix := connectionDataPrefix + ".connection.clickhouse."

		assert.Equal(t, testSinkHost, value(t, values, prefix+"host"))
		assert.Equal(t, 9000, value(t, values, prefix+"port"))
		assert.Equal(t, testSinkDB, value(t, values, prefix+"database"))
		assert.Equal(t, testSinkUser, value(t, values, prefix+"username"))
		assert.Equal(t, false, value(t, values, prefix+"ssl"))
	})

	t.Run("turns the publication function on", func(t *testing.T) {
		// Without it the producer issues ALTER PUBLICATION directly, which
		// requires it to own the tables, and GitLab's tables are owned by the
		// application role.
		assert.Equal(t, true,
			value(t, values, connectionDataPrefix+".connection.replication.use_alter_publication_function"))
	})

	t.Run("configures the queue and its object store", func(t *testing.T) {
		assert.Equal(t, queueDriver, value(t, values, connectionDataPrefix+".connection.queueing.driver"))
		assert.Equal(t, testQueueURL, value(t, values, connectionDataPrefix+".connection.queueing.url"))

		// An event too large for a stream message is offloaded here, which is
		// why the server needs JetStream.
		assert.Equal(t, natsObjectStorageBucket,
			value(t, values, connectionDataPrefix+".connection.queueing.object_storage_config.bucket_name"))
	})

	t.Run("omits the queue credentials when none are given", func(t *testing.T) {
		assertAbsent(t, values, connectionDataPrefix+".connection.queueing.username",
			"a server that accepts anonymous clients needs no credential")
	})

	t.Run("does not set the stream name in the connection base", func(t *testing.T) {
		// Siphon writes it onto each producer entry from the layout, and its
		// merge only fills gaps, so a value here is shadowed for producers and
		// would silently diverge for anything else.
		assertAbsent(t, values, connectionDataPrefix+".connection.queueing.stream_name",
			"the layout supplies it, and a value here would be shadowed for producers")
	})
}

func TestEffectiveValuesLayout(t *testing.T) {
	values := derive(t, newSiphon())

	t.Run("names the fixed single-shard topology", func(t *testing.T) {
		assert.Equal(t, streamName, value(t, values, layoutDataPrefix+".stream_name"))
		assert.Equal(t, []interface{}{producerID}, value(t, values, layoutDataPrefix+".producers."+sourceDB))
		assert.Equal(t, []interface{}{consumerID}, value(t, values, layoutDataPrefix+".consumers"))
		assert.Equal(t, []interface{}{reconcilerID}, value(t, values, layoutDataPrefix+".reconcilers"))
	})

	t.Run("folds the decomposed databases onto the one connection", func(t *testing.T) {
		// A self-managed instance is not decomposed.
		assert.Equal(t, sourceDB, value(t, values, layoutDataPrefix+".database_mapping.ci"))
		assert.Equal(t, sourceDB, value(t, values, layoutDataPrefix+".database_mapping.sec"))
	})

	t.Run("puts the top-level scalars here rather than in the connection base", func(t *testing.T) {
		// Siphon distributes only whole blocks from the connection base, so a
		// scalar there reaches nothing at all.
		assert.Equal(t, maxParallelWorkers, value(t, values, layoutDataPrefix+".max_parallel_workers"))

		assertAbsent(t, values, connectionDataPrefix+".max_parallel_workers",
			"only whole blocks are distributed, so a scalar here reaches nothing")
	})
}

func TestEffectiveValuesDeployments(t *testing.T) {
	siphon := newSiphon()
	values := derive(t, siphon)

	t.Run("enables one workload per role, keyed by the name of the resource", func(t *testing.T) {
		for _, workload := range deployments(siphon.Name) {
			assert.Equal(t, true, value(t, values, deploymentKey(workload.key, "enabled")), workload.key)
			assert.Equal(t, workload.role, value(t, values, deploymentKey(workload.key, "split.role")), workload.key)
			assert.Equal(t, configModeSplit, value(t, values, deploymentKey(workload.key, configModeKey)), workload.key)
		}
	})

	t.Run("disables the reference producer the chart ships", func(t *testing.T) {
		assert.Equal(t, false, value(t, values, deploymentKey(exampleDeployment, "enabled")))
	})

	t.Run("pins the Siphon application image tag", func(t *testing.T) {
		// The chart defaults it to null and declares no appVersion, so an unset
		// tag renders "<repository>:" and every pod fails InvalidImageName while
		// the render succeeds.
		assert.Equal(t, siphonImageTag, value(t, values, imageTagKey))
	})

	t.Run("lets the free-form values replace that tag", func(t *testing.T) {
		// It is a default rather than an override, so an installation that has to
		// run another build says so, which is the escape hatch while the tag is
		// pinned in source.
		pinned := newSiphon()
		pinned.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.0.99-beta"},
		}}

		assert.Equal(t, "0.0.99-beta", value(t, derive(t, pinned), imageTagKey))
	})

	t.Run("names the tables image outright rather than by the chart default", func(t *testing.T) {
		// global.gitlabVersion is also the version the chart's migration wait
		// selects on, and the two want different strings.
		for _, workload := range deployments(siphon.Name) {
			assert.Equal(t, testRelease().TablesImage,
				value(t, values, deploymentKey(workload.key, "split.tablesImage")), workload.key)
		}
	})

	t.Run("gives the producer the source credential and the rest the sink one", func(t *testing.T) {
		producer := value(t, values, deploymentKey(producerDeployment(siphon.Name), "envFromSecrets"))
		assert.Equal(t, []interface{}{map[string]interface{}{ //nolint:gosec // Secret key names, not credentials.
			envNameKey:   sourcePasswordEnv,
			envSecretKey: "siphon-postgresql",
			envKeyKey:    testSecretKey,
		}}, producer)

		for _, key := range []string{consumerDeployment(siphon.Name), reconcilerDeployment(siphon.Name)} {
			assert.Equal(t, []interface{}{map[string]interface{}{
				envNameKey:   sinkPasswordEnv,
				envSecretKey: "clickhouse-gitlab",
				envKeyKey:    testSecretKey,
			}}, value(t, values, deploymentKey(key, "envFromSecrets")), key)
		}
	})

	t.Run("renders a PodMonitor only where the cluster serves the API", func(t *testing.T) {
		for _, workload := range deployments(siphon.Name) {
			assert.Equal(t, false, value(t, values, deploymentKey(workload.key, "podmonitor")))
		}

		serving := derive(t, siphon, func(_ *Release, cluster *Cluster) {
			cluster.ServesPodMonitor = true
		})

		for _, workload := range deployments(siphon.Name) {
			assert.Equal(t, true, value(t, serving, deploymentKey(workload.key, "podmonitor")))
		}
	})
}

func TestEffectiveValuesQueueAuth(t *testing.T) {
	siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
		s.Spec.Queue.Auth = &apiv2alpha1.QueueAuthSpec{
			UsernameSecretRef: apiv2alpha1.SecretKeySelector{Name: queueDriver, Key: "username"},
			PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: queueDriver, Key: testSecretKey},
		}
	})

	values := derive(t, siphon)

	t.Run("substitutes the credentials into the connection base", func(t *testing.T) {
		assert.Equal(t, placeholder(queueUsernameEnv),
			value(t, values, connectionDataPrefix+".connection.queueing.username"))
		assert.Equal(t, placeholder(queuePasswordEnv),
			value(t, values, connectionDataPrefix+".connection.queueing.password"))
	})

	t.Run("wires them into every workload", func(t *testing.T) {
		// The connection base distributes the queueing block to every component,
		// so every one of them resolves the placeholders.
		for _, workload := range deployments(siphon.Name) {
			entries, ok := value(t, values, deploymentKey(workload.key, "envFromSecrets")).([]interface{})
			require.True(t, ok)

			names := []string{}

			for _, entry := range entries {
				typed, ok := entry.(map[string]interface{})
				require.True(t, ok)

				name, ok := typed[envNameKey].(string)
				require.True(t, ok)

				names = append(names, name)
			}

			assert.Contains(t, names, queueUsernameEnv, workload.key)
			assert.Contains(t, names, queuePasswordEnv, workload.key)
		}
	})
}

func TestEffectiveValuesQueueTLS(t *testing.T) {
	siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
		s.Spec.Queue.TLS = &apiv2alpha1.QueueTLSSpec{ //nolint:gosec // Secret key names, not credentials.
			SecretName:    "nats-client-certs",
			CACertKey:     "ca.crt",
			ClientCertKey: "tls.crt",
			ClientKeyKey:  "tls.key",
		}
	})

	values := derive(t, siphon)

	t.Run("writes the paths the chart injects in classic mode only", func(t *testing.T) {
		prefix := connectionDataPrefix + ".connection.queueing.tls_config."

		assert.Equal(t, natsCertsMountPath+"/ca.crt", value(t, values, prefix+"ca_cert_path"))
		assert.Equal(t, natsCertsMountPath+"/tls.crt", value(t, values, prefix+"client_cert_path"))
		assert.Equal(t, natsCertsMountPath+"/tls.key", value(t, values, prefix+"client_key_path"))
	})

	t.Run("mounts the Secret on every workload", func(t *testing.T) {
		for _, workload := range deployments(siphon.Name) {
			assert.Equal(t, "nats-client-certs",
				value(t, values, deploymentKey(workload.key, "natsClientCertsSecretName")), workload.key)
		}
	})
}

func TestEffectiveValuesPullSecret(t *testing.T) {
	siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
		s.Spec.Tables.PullSecretRef = &apiv2alpha1.LocalSecretReference{Name: "registry-credentials"}
	})

	t.Run("reaches the pod through podSpec, which the chart passes through", func(t *testing.T) {
		// The chart declares no imagePullSecrets, and this is the only route a
		// credential has to an image volume, which the kubelet pulls with the
		// credentials of the pod.
		values := derive(t, siphon)

		for _, workload := range deployments(siphon.Name) {
			assert.Equal(t, []interface{}{map[string]interface{}{"name": "registry-credentials"}},
				value(t, values, deploymentKey(workload.key, "podSpec.imagePullSecrets")), workload.key)
		}
	})
}

func TestOverridesWinOverTheEscapeHatch(t *testing.T) {
	// Five of the six overrides are chart preconditions: the chart fails a split
	// render outright without them. One disables a wait the chart cannot perform
	// here. None of them is a choice, so none may be overridden.
	siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
		s.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: map[string]interface{}{
			"configMode":                "classic",
			"siphonConnectionConfigMap": map[string]interface{}{"create": false},
			"siphonLayoutConfigMap":     map[string]interface{}{"create": false},
			"waitForMigrations":         map[string]interface{}{"enabled": true},
			"deployments": map[string]interface{}{
				producerDeployment(testName): map[string]interface{}{
					"split": map[string]interface{}{"role": "consumer"},
				},
			},
		}}
	})

	values := derive(t, siphon)

	assert.Equal(t, configModeSplit, value(t, values, configModeKey))
	assert.Equal(t, true, value(t, values, connectionConfigMapKey))
	assert.Equal(t, true, value(t, values, layoutConfigMapKey))
	assert.Equal(t, false, value(t, values, waitForMigrationsKey))
	assert.Equal(t, roleProducer,
		value(t, values, deploymentKey(producerDeployment(testName), "split.role")))
}

func TestTheEscapeHatchWinsOverTheDerivedValues(t *testing.T) {
	// ADR 26: the free-form values beat what the structured fields derive, which
	// is what keeps them usable. The layout is deliberately among what they can
	// replace, because that is the route to a sharded topology.
	siphon := newSiphon(func(s *apiv2alpha1.Siphon) {
		s.Spec.Chart.Values = apiv2alpha1.ChartValues{Object: map[string]interface{}{
			"siphonLayoutConfigMap": map[string]interface{}{
				"data": map[string]interface{}{
					"producers": map[string]interface{}{
						sourceDB: []interface{}{"siphon_main_1", "siphon_main_2"},
					},
				},
			},
			"siphonConnectionConfigMap": map[string]interface{}{
				"data": map[string]interface{}{
					"connection": map[string]interface{}{
						"clickhouse": map[string]interface{}{"insert_batch_size": 50000},
					},
				},
			},
		}}
	})

	values := derive(t, siphon)

	t.Run("replaces the layout", func(t *testing.T) {
		assert.Equal(t, []interface{}{"siphon_main_1", "siphon_main_2"},
			value(t, values, layoutDataPrefix+".producers."+sourceDB))
	})

	t.Run("keeps the derived siblings of an overridden key", func(t *testing.T) {
		assert.Equal(t, 50000,
			value(t, values, connectionDataPrefix+".connection.clickhouse.insert_batch_size"))
		assert.Equal(t, testSinkHost,
			value(t, values, connectionDataPrefix+".connection.clickhouse.host"))
	})
}
