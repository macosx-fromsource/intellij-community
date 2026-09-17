<script setup lang="ts">
import { computed, ref } from 'vue'

import { useDirtyTracking } from '@/composables/useDirtyTracking'
import type { GitLabResource } from '@/lib/api/client'

const props = defineProps<{
  namespace: string
  isEdit: boolean
  initialName: string
}>()

const name = ref(props.initialName)
const version = ref('')
const hostname = ref('')
const edition = ref<NonNullable<GitLabResource['edition']>>('ee')
const licenseSecretName = ref('')
const licenseSecretKey = ref('')

const basicsError = ref<string | null>(null)
const licenseError = ref<string | null>(null)

const hasError = computed(() => Boolean(basicsError.value || licenseError.value))

const { isDirty, resetBaseline } = useDirtyTracking(() => ({
  name: name.value,
  version: version.value,
  hostname: hostname.value,
  edition: edition.value,
  licenseSecretName: licenseSecretName.value,
  licenseSecretKey: licenseSecretKey.value,
}))

/**
 * Builds the license reference from the two Secret fields. Returns
 * `undefined` when the edition takes no license or both fields are empty, or
 * `null` when only one is filled, which is incomplete rather than absent.
 */
function parseLicense(): GitLabResource['license'] | null {
  // Only the Enterprise Edition reads a license, and the form hides the
  // fields for the Community Edition, so what they still hold is not sent.
  if (edition.value === 'ce') {
    licenseError.value = null

    return undefined
  }

  const secretName = licenseSecretName.value.trim()
  const secretKey = licenseSecretKey.value.trim()

  if (!secretName && !secretKey) {
    licenseError.value = null

    return undefined
  }

  if (!secretName || !secretKey) {
    licenseError.value = 'A license needs both a Secret name and a Secret key.'

    return null
  }

  licenseError.value = null

  return { secretRef: { name: secretName, key: secretKey } }
}

/** Checks the section and shows what it found. */
function validate(): boolean {
  basicsError.value = null

  if (!name.value.trim()) {
    basicsError.value = 'A name is required.'

    return false
  }

  // The reconciler renders nothing without a version, so the form does not
  // send a resource without one.
  if (!version.value.trim()) {
    basicsError.value = 'A chart version is required.'

    return false
  }

  return parseLicense() !== null
}

function loadFrom(resource: GitLabResource) {
  name.value = resource.name
  hostname.value = resource.hostname ?? ''
  edition.value = resource.edition ?? 'ee'
  licenseSecretName.value = resource.license?.secretRef.name ?? ''
  licenseSecretKey.value = resource.license?.secretRef.key ?? ''
  version.value = resource.chart.version ?? ''
}

/**
 * The fields this section contributes to the resource. `version` belongs to
 * `chart.version` rather than to a top-level field, so the parent assembles
 * that itself rather than spreading this directly onto a `GitLabResource`.
 */
function toPartial() {
  return {
    name: name.value,
    hostname: hostname.value.trim() || undefined,
    edition: edition.value,
    license: parseLicense() ?? undefined,
    version: version.value.trim(),
  }
}

defineExpose({ hasError, isDirty, validate, loadFrom, resetBaseline, toPartial })
</script>

<template>
  <div class="section-panel">
    <label>
      <span>Name</span>
      <input v-model="name" :disabled="isEdit" required />
      <span class="hint">
        {{ isEdit ? 'In' : 'Created in' }} the <code>{{ namespace }}</code> namespace.
      </span>
    </label>

    <p v-if="basicsError" class="error">{{ basicsError }}</p>

    <label>
      <span>Chart version</span>
      <input v-model="version" name="chart-version" placeholder="e.g. 10.2.2" />
      <span class="hint">
        It renders the instance, and changing it later triggers an upgrade. Any version GitLab
        publishes works: the Operator pulls one it does not already carry.
      </span>
    </label>

    <label>
      <span>Hostname</span>
      <input v-model="hostname" placeholder="gitlab.example.com" />
    </label>

    <label>
      <span>Edition</span>
      <select v-model="edition">
        <option value="ee">Enterprise Edition</option>
        <option value="ce">Community Edition</option>
      </select>
      <span class="hint">
        Enterprise Edition runs the Free feature set until a license activates more. Community
        Edition carries no proprietary code.
      </span>
    </label>

    <fieldset v-if="edition === 'ee'">
      <legend>License</legend>
      <p class="hint">
        The license is read from a Secret in the same namespace. Leave both fields empty to run
        without one.
      </p>

      <label>
        <span>Secret name</span>
        <input v-model="licenseSecretName" placeholder="gitlab-license" />
      </label>

      <label>
        <span>Secret key</span>
        <input v-model="licenseSecretKey" placeholder="license" />
      </label>

      <p v-if="licenseError" class="error">{{ licenseError }}</p>
    </fieldset>
  </div>
</template>
