<script setup lang="ts">
import { computed, reactive, ref, type Ref } from 'vue'

import postgresqlLogo from '@/assets/icons/postgresql.svg'
import valkeyLogo from '@/assets/icons/valkey.svg'
import { useDirtyTracking } from '@/composables/useDirtyTracking'
import type { GitLabResource } from '@/lib/api/client'

const postgresql = reactive({ host: '', secretName: '', secretKey: '' })
const postgresqlError = ref<string | null>(null)
const valkey = reactive({ host: '', secretName: '', secretKey: '' })
const valkeyError = ref<string | null>(null)
const objectStorage = reactive({ secretName: '', secretKey: '' })
const objectStorageError = ref<string | null>(null)

const hasError = computed(
  () => Boolean(postgresqlError.value || valkeyError.value || objectStorageError.value),
)

const { isDirty, resetBaseline } = useDirtyTracking(() => ({
  postgresql: { ...postgresql },
  valkey: { ...valkey },
  objectStorage: { ...objectStorage },
}))

/** Editable form state of a data store connection. */
type ConnectionFields = { host: string; secretName: string; secretKey: string }

/** Wire shape the PostgreSQL and Valkey connections share. */
type Connection = NonNullable<GitLabResource['postgresql']>

/** Fills the form fields of a data store connection from a fetched resource. */
function loadConnection(fields: ConnectionFields, connection: Connection | undefined) {
  fields.host = connection?.host ?? ''
  fields.secretName = connection?.passwordSecretRef.name ?? ''
  fields.secretKey = connection?.passwordSecretRef.key ?? ''
}

/**
 * Builds a data store connection from its three fields. Returns `undefined`
 * when all are empty (leave the connection to the chart values), or `null`
 * when only some are filled, which the resource rejects as incomplete.
 */
function parseConnection(
  fields: ConnectionFields,
  error: Ref<string | null>,
  label: string,
): Connection | undefined | null {
  const host = fields.host.trim()
  const name = fields.secretName.trim()
  const key = fields.secretKey.trim()

  if (!host && !name && !key) {
    error.value = null

    return undefined
  }

  if (!host || !name || !key) {
    error.value = `${label} needs a hostname, a Secret name, and a Secret key.`

    return null
  }

  error.value = null

  return { host, passwordSecretRef: { name, key } }
}

/**
 * Builds the object storage connection from its Secret fields. Returns
 * `undefined` when both are empty, or `null` when only one is filled. Unlike
 * a data store, it has no host: the Secret holds the endpoint along with the
 * credentials.
 */
function parseObjectStorage(): GitLabResource['objectStorage'] | null {
  const secretName = objectStorage.secretName.trim()
  const secretKey = objectStorage.secretKey.trim()

  if (!secretName && !secretKey) {
    objectStorageError.value = null

    return undefined
  }

  if (!secretName || !secretKey) {
    objectStorageError.value = 'Object storage needs both a Secret name and a Secret key.'

    return null
  }

  objectStorageError.value = null

  return { connectionSecretRef: { name: secretName, key: secretKey } }
}

/** Checks the section and shows what it found. */
function validate(): boolean {
  // All three run, so an incomplete Valkey group is reported alongside an
  // incomplete PostgreSQL one rather than after it.
  const psqlValid = parseConnection(postgresql, postgresqlError, 'PostgreSQL') !== null
  const valkeyValid = parseConnection(valkey, valkeyError, 'Valkey') !== null
  const storageValid = parseObjectStorage() !== null

  return psqlValid && valkeyValid && storageValid
}

function loadFrom(resource: GitLabResource) {
  loadConnection(postgresql, resource.postgresql)
  loadConnection(valkey, resource.redis)
  objectStorage.secretName = resource.objectStorage?.connectionSecretRef.name ?? ''
  objectStorage.secretKey = resource.objectStorage?.connectionSecretRef.key ?? ''
}

function toPartial() {
  return {
    postgresql: parseConnection(postgresql, postgresqlError, 'PostgreSQL') ?? undefined,
    redis: parseConnection(valkey, valkeyError, 'Valkey') ?? undefined,
    objectStorage: parseObjectStorage() ?? undefined,
  }
}

defineExpose({ hasError, isDirty, validate, loadFrom, resetBaseline, toPartial })
</script>

<template>
  <div class="section-panel">
    <fieldset>
      <legend><img :src="postgresqlLogo" alt="" class="legend-icon" /> PostgreSQL</legend>
      <p class="hint">
        The chart bundles no database, so the instance needs one you run. For the versions and
        extensions it needs, see
        <a
          href="https://docs.gitlab.com/install/requirements/#postgresql"
          target="_blank"
          rel="noopener noreferrer"
          >the PostgreSQL requirements</a
        >.
      </p>

      <label>
        <span>Hostname</span>
        <input v-model="postgresql.host" placeholder="gitlab-postgresql.databases.svc.cluster.local" />
      </label>

      <label>
        <span>Password Secret name</span>
        <input v-model="postgresql.secretName" placeholder="gitlab-postgresql-password" />
      </label>

      <label>
        <span>Password Secret key</span>
        <input v-model="postgresql.secretKey" placeholder="password" />
      </label>

      <p v-if="postgresqlError" class="error">{{ postgresqlError }}</p>
    </fieldset>

    <fieldset>
      <legend>
        <img :src="valkeyLogo" alt="" class="legend-icon legend-icon--mono" /> Valkey
      </legend>
      <p class="hint">
        Valkey holds the caches, the queues, and the shared state. Redis works in its place, and
        the chart values still call it Redis. For the versions the instance needs, see
        <a
          href="https://docs.gitlab.com/install/requirements/#redis"
          target="_blank"
          rel="noopener noreferrer"
          >the Redis requirements</a
        >.
      </p>

      <label>
        <span>Hostname</span>
        <input v-model="valkey.host" placeholder="gitlab-valkey.databases.svc.cluster.local" />
      </label>

      <label>
        <span>Password Secret name</span>
        <input v-model="valkey.secretName" placeholder="gitlab-valkey-auth" />
      </label>

      <label>
        <span>Password Secret key</span>
        <input v-model="valkey.secretKey" placeholder="default" />
      </label>

      <p v-if="valkeyError" class="error">{{ valkeyError }}</p>
    </fieldset>

    <fieldset>
      <legend>
        <span class="legend-icon legend-icon--glyph" aria-hidden="true"></span> Object storage
      </legend>
      <p class="hint">
        Artifacts, uploads, and other blobs live in S3 compatible object storage. One Secret holds
        the endpoint, the region, and the credentials, in the
        <a
          href="https://docs.gitlab.com/charts/charts/globals/#connection"
          target="_blank"
          rel="noopener noreferrer"
          >connection format of the chart</a
        >. The registry, Pages, and backups read settings of their own, which stay in the
        overrides.
      </p>

      <label>
        <span>Connection Secret name</span>
        <input v-model="objectStorage.secretName" placeholder="gitlab-object-storage" />
      </label>

      <label>
        <span>Connection Secret key</span>
        <input v-model="objectStorage.secretKey" placeholder="connection" />
      </label>

      <p v-if="objectStorageError" class="error">{{ objectStorageError }}</p>
    </fieldset>
  </div>
</template>
