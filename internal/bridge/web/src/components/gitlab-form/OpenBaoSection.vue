<script setup lang="ts">
import { computed, reactive, ref } from 'vue'

import { useDirtyTracking } from '@/composables/useDirtyTracking'
import type { GitLabResource } from '@/lib/api/client'

const openbaoEnabled = ref(false)
const openbaoError = ref<string | null>(null)
const openbaoPsql = reactive({ host: '', port: '', database: '', username: '', secretName: '', secretKey: '' })

const openbaoServiceAccountName = ref('')
const openbaoServiceAccountError = ref<string | null>(null)

const hasError = computed(() => Boolean(openbaoError.value || openbaoServiceAccountError.value))
const enabled = computed(() => openbaoEnabled.value)

const { isDirty, resetBaseline } = useDirtyTracking(() => ({
  openbaoEnabled: openbaoEnabled.value,
  openbaoPsql: { ...openbaoPsql },
  openbaoServiceAccountName: openbaoServiceAccountName.value,
}))

function validatePostgreSQL(): boolean {
  if (!openbaoPsql.host.trim() || !openbaoPsql.secretName.trim() || !openbaoPsql.secretKey.trim()) {
    openbaoError.value =
      "Secret Manager needs its PostgreSQL database's hostname, a Secret name, and a Secret key for the password."

    return false
  }

  const port = openbaoPsql.port.trim()
  if (port) {
    // Matches the CRD's own bounds (OpenBaoPostgreSQLSpec.Port: minimum 1, maximum 65535), so a
    // value out of range is caught here rather than surfacing as a raw Kubernetes API error.
    if (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535) {
      openbaoError.value = 'The PostgreSQL port must be a number between 1 and 65535.'

      return false
    }
  }

  return true
}

// The bridge and the Operator do not manage RBAC on the cluster they
// reconcile, so the ServiceAccount, its Role, and the RoleBinding are an
// administrator prerequisite; this field only names the one already granted
// what OpenBao needs.
function validateServiceAccount(): boolean {
  if (!openbaoServiceAccountName.value.trim()) {
    openbaoServiceAccountError.value =
      'Secret Manager needs the name of a ServiceAccount already granted the Role it needs on Pods.'

    return false
  }

  return true
}

/** Checks the section and shows what it found. */
function validate(): boolean {
  openbaoError.value = null
  openbaoServiceAccountError.value = null

  if (!openbaoEnabled.value) {
    return true
  }

  // Both run, so an incomplete ServiceAccount name is reported alongside an
  // incomplete PostgreSQL connection rather than after it.
  const psqlValid = validatePostgreSQL()
  const serviceAccountValid = validateServiceAccount()

  return psqlValid && serviceAccountValid
}

function loadFrom(resource: GitLabResource) {
  if (!resource.openbao) {
    return
  }

  openbaoEnabled.value = true
  openbaoPsql.host = resource.openbao.postgresql.host
  openbaoPsql.port = resource.openbao.postgresql.port?.toString() ?? ''
  openbaoPsql.database = resource.openbao.postgresql.database ?? ''
  openbaoPsql.username = resource.openbao.postgresql.username ?? ''
  openbaoPsql.secretName = resource.openbao.postgresql.passwordSecretRef.name
  openbaoPsql.secretKey = resource.openbao.postgresql.passwordSecretRef.key
  openbaoServiceAccountName.value = resource.openbao.serviceAccount.name
}

/**
 * Builds the `openbao` field of the resource from the PostgreSQL database it
 * needs of its own (never the main application database: OpenBao does not
 * inherit that password), and the pre-existing ServiceAccount its pod runs
 * as. Returns `undefined` when the add-on is off.
 */
function toPartial(): NonNullable<GitLabResource['openbao']> | undefined {
  if (!openbaoEnabled.value) {
    return undefined
  }

  const port = openbaoPsql.port.trim()

  return {
    postgresql: {
      host: openbaoPsql.host.trim(),
      port: port ? Number(port) : undefined,
      database: openbaoPsql.database.trim() || undefined,
      username: openbaoPsql.username.trim() || undefined,
      passwordSecretRef: { name: openbaoPsql.secretName.trim(), key: openbaoPsql.secretKey.trim() },
    },
    serviceAccount: { name: openbaoServiceAccountName.value.trim() },
  }
}

defineExpose({ hasError, isDirty, enabled, validate, loadFrom, resetBaseline, toPartial })
</script>

<template>
  <div class="section-panel">
    <label class="checkbox-field">
      <input v-model="openbaoEnabled" type="checkbox" />
      <span>Enable Secret Manager</span>
    </label>
    <p class="hint">
      Backs the GitLab Secret Manager with an OpenBao instance. It needs a PostgreSQL database of
      its own: the password is not inherited from the instance's main database, so create the
      database and the role ahead of time. See
      <a
        href="https://docs.gitlab.com/charts/charts/openbao/#database-configuration"
        target="_blank"
        rel="noopener noreferrer"
        >the OpenBao database configuration</a
      >.
    </p>

    <fieldset :disabled="!openbaoEnabled">
      <legend>PostgreSQL</legend>

      <label>
        <span>Hostname</span>
        <input v-model="openbaoPsql.host" placeholder="openbao-postgresql.databases.svc.cluster.local" />
      </label>

      <label>
        <span>Port</span>
        <input v-model="openbaoPsql.port" placeholder="5432" />
      </label>

      <label>
        <span>Database</span>
        <input v-model="openbaoPsql.database" placeholder="openbao" />
      </label>

      <label>
        <span>Username</span>
        <input v-model="openbaoPsql.username" placeholder="openbao" />
      </label>

      <label>
        <span>Password Secret name</span>
        <input v-model="openbaoPsql.secretName" placeholder="openbao-db-password" />
      </label>

      <label>
        <span>Password Secret key</span>
        <input v-model="openbaoPsql.secretKey" placeholder="password" />
      </label>

      <p v-if="openbaoError" class="error">{{ openbaoError }}</p>
    </fieldset>

    <fieldset :disabled="!openbaoEnabled">
      <legend>ServiceAccount</legend>
      <p class="hint">
        OpenBao's pod needs a Role granting <code>get</code>/<code>update</code>/<code>patch</code>
        on Pods in this namespace, and a RoleBinding to it. Neither is created here: the bridge and
        the Operator do not manage RBAC on the cluster they reconcile. Create the ServiceAccount,
        the Role, and the RoleBinding ahead of time, and name the ServiceAccount below.
      </p>

      <label>
        <span>Name</span>
        <input v-model="openbaoServiceAccountName" placeholder="openbao" />
      </label>

      <p v-if="openbaoServiceAccountError" class="error">{{ openbaoServiceAccountError }}</p>
    </fieldset>
  </div>
</template>
