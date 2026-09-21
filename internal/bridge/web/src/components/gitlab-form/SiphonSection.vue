<script setup lang="ts">
import { computed, reactive, ref } from 'vue'

import postgresqlLogo from '@/assets/icons/postgresql.svg'
import { useDirtyTracking } from '@/composables/useDirtyTracking'
import type { SiphonResource } from '@/lib/api/client'
import { parsePort, portError } from '@/lib/port'

const props = defineProps<{
  /**
   * Why the Siphon of the instance could not be read, if it could not. The
   * add-on is then locked: shown off, it could be turned on over a Siphon
   * that exists but went unseen, which the save would replace.
   */
  unreadable?: string | null
}>()

const enabled = ref(false)
const version = ref('')
const versionError = ref<string | null>(null)

const source = reactive({
  host: '',
  port: '',
  database: '',
  user: '',
  secretName: '',
  secretKey: '',
  sslMode: 'require' as NonNullable<SiphonResource['source']['sslMode']>,
  advisoryLockID: '',
})
const sourceError = ref<string | null>(null)

const queue = reactive({
  url: '',
  usernameSecretName: '',
  usernameSecretKey: '',
  passwordSecretName: '',
  passwordSecretKey: '',
  tlsSecretName: '',
  caCertKey: '',
  clientCertKey: '',
  clientKeyKey: '',
})
const queueError = ref<string | null>(null)

const sink = reactive({
  host: '',
  port: '',
  database: '',
  username: '',
  secretName: '',
  secretKey: '',
  ssl: false,
})
const sinkError = ref<string | null>(null)

const tables = reactive({
  source: 'Auto' as NonNullable<NonNullable<SiphonResource['tables']>['source']>,
  image: '',
  pullSecretName: '',
})

/**
 * The chart values and the observed status of the Siphon that was loaded.
 * The section shows no values editor, so they are carried back untouched
 * rather than dropped by the replace the save performs.
 */
const chartValues = ref<Record<string, unknown>>({})
const status = ref<SiphonResource['status'] | null>(null)

const hasError = computed(() =>
  Boolean(versionError.value || sourceError.value || queueError.value || sinkError.value),
)

const { isDirty, resetBaseline } = useDirtyTracking(() => ({
  enabled: enabled.value,
  version: version.value,
  source: { ...source },
  queue: { ...queue },
  sink: { ...sink },
  tables: { ...tables },
}))

function validateVersion(): boolean {
  if (!version.value.trim()) {
    versionError.value = 'Siphon needs a chart version, one the Operator bundles.'

    return false
  }

  return true
}

function validateSource(): boolean {
  if (!source.host.trim() || !source.secretName.trim() || !source.secretKey.trim()) {
    sourceError.value =
      'The PostgreSQL source needs a hostname, a Secret name, and a Secret key for the password.'

    return false
  }

  if (parsePort(source.port) === null) {
    sourceError.value = portError('PostgreSQL')

    return false
  }

  const lock = source.advisoryLockID.trim()
  if (lock && (!/^\d+$/.test(lock) || Number(lock) < 1 || Number(lock) > 2147483647)) {
    sourceError.value = 'The advisory lock ID must be a number between 1 and 2147483647.'

    return false
  }

  return true
}

/**
 * The four Secret references the NATS credentials are made of, trimmed. They
 * are all four or none, like a data store connection, which is what both the
 * check and the request below work out from them.
 */
function queueAuthFields() {
  return {
    usernameSecretName: queue.usernameSecretName.trim(),
    usernameSecretKey: queue.usernameSecretKey.trim(),
    passwordSecretName: queue.passwordSecretName.trim(),
    passwordSecretKey: queue.passwordSecretKey.trim(),
  }
}

/**
 * The NATS credentials of the resource, or `undefined` for a server that
 * accepts anonymous clients. A partly filled group is rejected by
 * `validateQueue` before this is reached.
 */
function queueAuth(): SiphonResource['queue']['auth'] {
  const fields = queueAuthFields()

  if (!Object.values(fields).every(Boolean)) {
    return undefined
  }

  return {
    usernameSecretRef: { name: fields.usernameSecretName, key: fields.usernameSecretKey },
    passwordSecretRef: { name: fields.passwordSecretName, key: fields.passwordSecretKey },
  }
}

function validateQueue(): boolean {
  const url = queue.url.trim()
  if (!url) {
    queueError.value = 'Siphon needs the URL of a NATS server with JetStream enabled.'

    return false
  }

  if (!url.startsWith('nats://')) {
    queueError.value = 'The NATS URL must start with nats://.'

    return false
  }

  const auth = Object.values(queueAuthFields())
  if (auth.some(Boolean) && !auth.every(Boolean)) {
    queueError.value =
      'NATS authentication needs a Secret name and key for both the user name and the password.'

    return false
  }

  const tlsKeys = [queue.caCertKey, queue.clientCertKey, queue.clientKeyKey]
  if (!queue.tlsSecretName.trim() && tlsKeys.some((key) => key.trim())) {
    queueError.value = 'A NATS client certificate needs the name of the Secret holding it.'

    return false
  }

  return true
}

function validateSink(): boolean {
  if (
    !sink.host.trim() ||
    !sink.database.trim() ||
    !sink.username.trim() ||
    !sink.secretName.trim() ||
    !sink.secretKey.trim()
  ) {
    sinkError.value =
      'The ClickHouse sink needs a hostname, a database, a username, a Secret name, and a Secret key for the password.'

    return false
  }

  const port = parsePort(sink.port)
  if (port === null) {
    sinkError.value = portError('ClickHouse')

    return false
  }

  // The native protocol port, which the CRD rejects 8123 for: that is the
  // HTTP interface, and Siphon does not speak it.
  if (port === 8123) {
    sinkError.value =
      'Port 8123 is the ClickHouse HTTP interface, which Siphon does not speak. Use the native protocol port, 9000 by default.'

    return false
  }

  return true
}

/** Checks the section and shows what it found. */
function validate(): boolean {
  versionError.value = null
  sourceError.value = null
  queueError.value = null
  sinkError.value = null

  if (!enabled.value) {
    return true
  }

  // All four run, so every incomplete group is reported at once rather than
  // one save at a time.
  const versionValid = validateVersion()
  const sourceValid = validateSource()
  const queueValid = validateQueue()
  const sinkValid = validateSink()

  return versionValid && sourceValid && queueValid && sinkValid
}

/** Fills the fields from the Siphon of the instance, if it has one. */
function loadFrom(siphon: SiphonResource | null) {
  if (!siphon) {
    return
  }

  enabled.value = true
  version.value = siphon.chart.version ?? ''
  chartValues.value = siphon.chart.values ?? {}
  status.value = siphon.status ?? null

  source.host = siphon.source.host
  source.port = siphon.source.port?.toString() ?? ''
  source.database = siphon.source.database ?? ''
  source.user = siphon.source.user ?? ''
  source.secretName = siphon.source.passwordSecretRef.name
  source.secretKey = siphon.source.passwordSecretRef.key
  source.sslMode = siphon.source.sslMode ?? 'require'
  source.advisoryLockID = siphon.source.advisoryLockID?.toString() ?? ''

  queue.url = siphon.queue.url
  queue.usernameSecretName = siphon.queue.auth?.usernameSecretRef.name ?? ''
  queue.usernameSecretKey = siphon.queue.auth?.usernameSecretRef.key ?? ''
  queue.passwordSecretName = siphon.queue.auth?.passwordSecretRef.name ?? ''
  queue.passwordSecretKey = siphon.queue.auth?.passwordSecretRef.key ?? ''
  queue.tlsSecretName = siphon.queue.tls?.secretName ?? ''
  queue.caCertKey = siphon.queue.tls?.caCertKey ?? ''
  queue.clientCertKey = siphon.queue.tls?.clientCertKey ?? ''
  queue.clientKeyKey = siphon.queue.tls?.clientKeyKey ?? ''

  sink.host = siphon.sink.host
  sink.port = siphon.sink.port?.toString() ?? ''
  sink.database = siphon.sink.database
  sink.username = siphon.sink.username
  sink.secretName = siphon.sink.passwordSecretRef.name
  sink.secretKey = siphon.sink.passwordSecretRef.key
  sink.ssl = siphon.sink.ssl ?? false

  tables.source = siphon.tables?.source ?? 'Auto'
  tables.image = siphon.tables?.image ?? ''
  tables.pullSecretName = siphon.tables?.pullSecretRef?.name ?? ''
}

/**
 * Builds the Siphon resource the section describes, or `undefined` when the
 * add-on is off. The instance it belongs to is not part of it: the endpoint
 * nests under that instance, which is what links the two.
 */
function toPartial(): SiphonResource | undefined {
  if (!enabled.value) {
    return undefined
  }

  const tlsSecretName = queue.tlsSecretName.trim()

  return {
    source: {
      host: source.host.trim(),
      port: parsePort(source.port) ?? undefined,
      database: source.database.trim() || undefined,
      user: source.user.trim() || undefined,
      passwordSecretRef: { name: source.secretName.trim(), key: source.secretKey.trim() },
      sslMode: source.sslMode,
      advisoryLockID: source.advisoryLockID.trim() ? Number(source.advisoryLockID) : undefined,
    },
    queue: {
      url: queue.url.trim(),
      auth: queueAuth(),
      tls: tlsSecretName
        ? {
            secretName: tlsSecretName,
            caCertKey: queue.caCertKey.trim() || undefined,
            clientCertKey: queue.clientCertKey.trim() || undefined,
            clientKeyKey: queue.clientKeyKey.trim() || undefined,
          }
        : undefined,
    },
    sink: {
      host: sink.host.trim(),
      port: parsePort(sink.port) ?? undefined,
      database: sink.database.trim(),
      username: sink.username.trim(),
      passwordSecretRef: { name: sink.secretName.trim(), key: sink.secretKey.trim() },
      ssl: sink.ssl,
    },
    tables: {
      source: tables.source,
      image: tables.image.trim() || undefined,
      pullSecretRef: tables.pullSecretName.trim() ? { name: tables.pullSecretName.trim() } : undefined,
    },
    chart: { version: version.value.trim(), values: chartValues.value },
  }
}

defineExpose({
  hasError,
  isDirty,
  // Read-only, like the other sections expose it: the parent reads the
  // add-on's state for the sidebar, it does not set it.
  enabled: computed(() => enabled.value),
  validate,
  loadFrom,
  resetBaseline,
  toPartial,
})
</script>

<template>
  <div class="section-panel">
    <label class="checkbox-field">
      <input v-model="enabled" type="checkbox" :disabled="Boolean(props.unreadable)" />
      <span>Enable Siphon</span>
    </label>
    <p v-if="props.unreadable" class="error">
      The Siphon of this instance could not be read, so whether it has one is unknown and the
      add-on is left as it is. Reading it needs access to <code>siphons.apps.gitlab.com</code>.
      ({{ props.unreadable }})
    </p>
    <p class="hint">
      Streams change data capture from the database of this instance into ClickHouse, through NATS
      JetStream. It is a
      <code>Siphon</code> resource of its own, referencing this instance, and it is saved alongside
      it. The three servers are referenced, never created: the publication, the
      <code>siphon_alter_publication</code> function, the login roles and the grants on PostgreSQL,
      the NATS server, and the ClickHouse target tables are all prerequisites.
    </p>

    <dl v-if="status" class="status">
      <dt>Phase</dt>
      <dd>{{ status.phase || '—' }}</dd>
      <dt>Tables</dt>
      <dd>{{ status.tableCount ?? 0 }} ({{ status.tablesSource || '—' }})</dd>
      <dt>GitLab version</dt>
      <dd>{{ status.gitlabVersion || '—' }}</dd>
      <dt>Publication</dt>
      <dd>{{ status.publication || '—' }}</dd>
      <dt>Replication slot</dt>
      <dd>{{ status.replicationSlot || '—' }}</dd>
      <dt>NATS stream</dt>
      <dd>{{ status.streamName || '—' }}</dd>
    </dl>

    <fieldset :disabled="!enabled">
      <legend>Chart</legend>

      <label>
        <span>Chart version</span>
        <input v-model="version" name="siphon-chart-version" placeholder="e.g. 1.21.0" />
        <span class="hint">
          The Siphon chart is not the GitLab chart, and it is never pulled: only a version the
          Operator bundles renders.
        </span>
      </label>

      <p v-if="versionError" class="error">{{ versionError }}</p>
    </fieldset>

    <fieldset :disabled="!enabled">
      <legend><img :src="postgresqlLogo" alt="" class="legend-icon" /> PostgreSQL source</legend>
      <p class="hint">
        The primary of the instance's database, running with <code>wal_level = logical</code>: a
        logical replication slot is not created on a standby.
      </p>

      <label>
        <span>Hostname</span>
        <input v-model="source.host" placeholder="gitlab-postgresql-rw.databases.svc.cluster.local" />
      </label>

      <label>
        <span>Port</span>
        <input v-model="source.port" placeholder="5432" />
      </label>

      <label>
        <span>Database</span>
        <input v-model="source.database" placeholder="gitlabhq_production" />
        <span class="hint">Immutable: changing it re-snapshots every table.</span>
      </label>

      <label>
        <span>User</span>
        <input v-model="source.user" placeholder="siphon" />
        <span class="hint">
          Needs REPLICATION, EXECUTE on <code>siphon_alter_publication</code>, and SELECT on the
          replicated tables.
        </span>
      </label>

      <label>
        <span>Password Secret name</span>
        <input v-model="source.secretName" placeholder="gitlab-siphon-postgresql" />
      </label>

      <label>
        <span>Password Secret key</span>
        <input v-model="source.secretKey" placeholder="password" />
      </label>

      <label>
        <span>SSL mode</span>
        <select v-model="source.sslMode">
          <option value="disable">disable</option>
          <option value="allow">allow</option>
          <option value="prefer">prefer</option>
          <option value="require">require</option>
          <option value="verify-ca">verify-ca</option>
          <option value="verify-full">verify-full</option>
        </select>
      </label>

      <label>
        <span>Advisory lock ID</span>
        <input v-model="source.advisoryLockID" placeholder="1" />
        <span class="hint">
          Elects the active producer. Change it only to run a second, independent stream off this
          database.
        </span>
      </label>

      <p v-if="sourceError" class="error">{{ sourceError }}</p>
    </fieldset>

    <fieldset :disabled="!enabled">
      <legend>NATS queue</legend>
      <p class="hint">
        JetStream must be enabled: Siphon uses a stream, a key-value bucket to elect the active
        consumer, and an object store for oversized events.
      </p>

      <label>
        <span>URL</span>
        <input v-model="queue.url" placeholder="nats://nats.nats.svc.cluster.local:4222" />
      </label>

      <label>
        <span>User name Secret name</span>
        <input v-model="queue.usernameSecretName" placeholder="nats-credentials" />
        <span class="hint">
          Leave the four credential fields empty for a server that accepts anonymous clients.
        </span>
      </label>

      <label>
        <span>User name Secret key</span>
        <input v-model="queue.usernameSecretKey" placeholder="username" />
      </label>

      <label>
        <span>Password Secret name</span>
        <input v-model="queue.passwordSecretName" placeholder="nats-credentials" />
      </label>

      <label>
        <span>Password Secret key</span>
        <input v-model="queue.passwordSecretKey" placeholder="password" />
      </label>

      <label>
        <span>Client certificate Secret name</span>
        <input v-model="queue.tlsSecretName" placeholder="nats-client-certs" />
        <span class="hint">
          Mounted whole, so each key becomes a file of that name. Leave it empty for a connection
          that presents no certificate.
        </span>
      </label>

      <label>
        <span>CA certificate key</span>
        <input v-model="queue.caCertKey" placeholder="ca.crt" />
      </label>

      <label>
        <span>Client certificate key</span>
        <input v-model="queue.clientCertKey" placeholder="tls.crt" />
      </label>

      <label>
        <span>Client private key key</span>
        <input v-model="queue.clientKeyKey" placeholder="tls.key" />
      </label>

      <p v-if="queueError" class="error">{{ queueError }}</p>
    </fieldset>

    <fieldset :disabled="!enabled">
      <legend>ClickHouse sink</legend>
      <p class="hint">
        The target tables are created by the ClickHouse migrations of GitLab, not by Siphon, so name
        the database they live in and a user holding INSERT and SELECT on them.
      </p>

      <label>
        <span>Hostname</span>
        <input v-model="sink.host" placeholder="clickhouse.databases.svc.cluster.local" />
      </label>

      <label>
        <span>Port</span>
        <input v-model="sink.port" placeholder="9000" />
        <span class="hint">The native protocol port. 8123 is the HTTP interface, which Siphon does not speak.</span>
      </label>

      <label>
        <span>Database</span>
        <input v-model="sink.database" placeholder="gitlab_clickhouse_main_production" />
      </label>

      <label>
        <span>Username</span>
        <input v-model="sink.username" placeholder="gitlab" />
      </label>

      <label>
        <span>Password Secret name</span>
        <input v-model="sink.secretName" placeholder="gitlab-clickhouse-gitlab" />
      </label>

      <label>
        <span>Password Secret key</span>
        <input v-model="sink.secretKey" placeholder="password" />
      </label>

      <label class="checkbox-field">
        <input v-model="sink.ssl" type="checkbox" />
        <span>Connect over TLS</span>
      </label>

      <p v-if="sinkError" class="error">{{ sinkError }}</p>
    </fieldset>

    <fieldset :disabled="!enabled">
      <legend>Table definitions</legend>
      <p class="hint">
        The definitions pin the pipeline to the schema of the deployed GitLab version. They ship as
        an image, which is either mounted into the pods or extracted by the Operator into a
        ConfigMap.
      </p>

      <label>
        <span>Source</span>
        <select v-model="tables.source">
          <option value="Auto">Auto</option>
          <option value="ConfigMap">ConfigMap</option>
          <option value="ImageVolume">ImageVolume</option>
        </select>
        <span class="hint">
          <code>Auto</code> resolves to <code>ConfigMap</code>, which works everywhere;
          <code>ImageVolume</code> needs a cluster and a runtime that serve OCI image volumes.
        </span>
      </label>

      <label>
        <span>Image</span>
        <input v-model="tables.image" placeholder="registry.gitlab.com/gitlab-org/gitlab/gitlab-siphon-tables:v19.2.0-ee" />
        <span class="hint">
          Left empty, it follows the application version of this instance. Set it only to pin one.
        </span>
      </label>

      <label>
        <span>Pull Secret name</span>
        <input v-model="tables.pullSecretName" placeholder="gitlab-registry-credentials" />
        <span class="hint">
          A <code>kubernetes.io/dockerconfigjson</code> Secret. Leave it empty for the public image.
        </span>
      </label>
    </fieldset>
  </div>
</template>

<style scoped>
.status {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.25rem 1rem;
  margin: 0;
  font-size: 0.9rem;
}

.status dt {
  font-weight: 500;
  opacity: 0.75;
}

.status dd {
  margin: 0;
}
</style>
