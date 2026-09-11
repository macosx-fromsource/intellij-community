<script setup lang="ts">
import { computed, onMounted, reactive, ref, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { parse, stringify } from 'yaml'

import postgresqlLogo from '@/assets/icons/postgresql.svg'
import valkeyLogo from '@/assets/icons/valkey.svg'
import YamlEditor from '@/components/YamlEditor.vue'
import type { GitLabResource } from '@/lib/api/client'
import { useGitLabsStore } from '@/stores/gitlabs'

const props = defineProps<{
  namespace?: string
  name?: string
}>()

const store = useGitLabsStore()
const router = useRouter()

const isEdit = Boolean(props.namespace && props.name)

/**
 * Where a new instance is created. The Operator installation owns this
 * namespace, so the form does not offer a choice; an existing resource is
 * edited wherever it already lives.
 */
const defaultNamespace = 'gitlab-system'

const namespace = ref(props.namespace ?? defaultNamespace)
const name = ref(props.name ?? 'gitlab')
const basicsError = ref<string | null>(null)
const hostname = ref('')
const edition = ref<NonNullable<GitLabResource['edition']>>('ee')
const licenseSecretName = ref('')
const licenseSecretKey = ref('')
const licenseError = ref<string | null>(null)
const postgresql = reactive({ host: '', secretName: '', secretKey: '' })
const postgresqlError = ref<string | null>(null)
const valkey = reactive({ host: '', secretName: '', secretKey: '' })
const valkeyError = ref<string | null>(null)
const objectStorage = reactive({ secretName: '', secretKey: '' })
const objectStorageError = ref<string | null>(null)
/**
 * The chart version, free-form: nothing about which ones the Operator can
 * render is known here. A bundled version renders immediately; any other is
 * pulled from the Operator's chart repository when it starts reconciling.
 */
const version = ref('')

const valuesText = ref('')
const valuesError = ref<string | null>(null)

/**
 * The phases of the form. Basics identify the instance, dependencies are the
 * data stores it needs, and overrides are the chart itself. Each is a step of
 * its own so a new instance is filled in the order it is decided, rather than
 * from one page that asks everything at once.
 */
const steps = [
  { title: 'Basics', summary: 'What the instance is, and which chart deploys it.' },
  { title: 'Dependencies', summary: 'The data stores the instance connects to.' },
  { title: 'Overrides', summary: 'The chart values, for everything the steps above do not cover.' },
] as const

const step = ref(0)
const currentStep = computed(() => steps[step.value] ?? steps[0])
const isLastStep = computed(() => step.value === steps.length - 1)

onMounted(async () => {
  if (!isEdit) {
    return
  }

  const ok = await store.fetchOne(props.namespace!, props.name!)
  if (ok && store.current) {
    hostname.value = store.current.hostname ?? ''
    edition.value = store.current.edition ?? 'ee'
    licenseSecretName.value = store.current.license?.secretRef.name ?? ''
    licenseSecretKey.value = store.current.license?.secretRef.key ?? ''
    loadConnection(postgresql, store.current.postgresql)
    loadConnection(valkey, store.current.redis)
    objectStorage.secretName = store.current.objectStorage?.connectionSecretRef.name ?? ''
    objectStorage.secretKey = store.current.objectStorage?.connectionSecretRef.key ?? ''
    version.value = store.current.chart.version ?? ''

    const values = store.current.chart.values ?? {}
    valuesText.value = Object.keys(values).length ? stringify(values) : ''
  }
})

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
 * when all are empty (leave the connection to the chart values), or `null` when
 * only some are filled, which the resource rejects as incomplete.
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
 * `undefined` when both are empty, or `null` when only one is filled. Unlike a
 * data store, it has no host: the Secret holds the endpoint along with the
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

/**
 * Builds the license reference from the two Secret fields. Returns `undefined`
 * when the edition takes no license or both fields are empty, or `null` when
 * only one is filled, which is incomplete rather than absent.
 */
function parseLicense(): GitLabResource['license'] | null {
  // Only the Enterprise Edition reads a license, and the form hides the fields
  // for the Community Edition, so what they still hold is not sent.
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

function parseValues(): Record<string, unknown> | null {
  const text = valuesText.value.trim()
  if (text === '') {
    return {}
  }

  try {
    const parsed = parse(text)
    if (parsed === null || parsed === undefined) {
      valuesError.value = null

      return {}
    }

    if (typeof parsed !== 'object' || Array.isArray(parsed)) {
      valuesError.value = 'Values must be a YAML mapping (object).'

      return null
    }

    valuesError.value = null

    return parsed as Record<string, unknown>
  } catch (err) {
    valuesError.value = `Invalid YAML: ${(err as Error).message}`

    return null
  }
}

/**
 * Checks one step and shows what it found. Every check runs the same parser the
 * submit does, so a step passes here exactly when its fields end up in the
 * resource.
 */
function validateStep(index: number): boolean {
  if (index === 0) {
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

  if (index === 1) {
    // Both run, so an incomplete Valkey group is reported alongside an
    // incomplete PostgreSQL one rather than after it.
    const psqlValid = parseConnection(postgresql, postgresqlError, 'PostgreSQL') !== null
    const valkeyValid = parseConnection(valkey, valkeyError, 'Valkey') !== null
    const storageValid = parseObjectStorage() !== null

    return psqlValid && valkeyValid && storageValid
  }

  return parseValues() !== null
}

/**
 * Moves to a step. Going forward checks every step that is passed over, and
 * stops at the first one that does not hold up, so the fields of a skipped step
 * cannot reach the resource unchecked. Going back checks nothing: leaving a
 * half-filled step to look at an earlier one is not an error.
 */
function goTo(target: number) {
  for (let index = step.value; index < target; index++) {
    if (!validateStep(index)) {
      step.value = index

      return
    }
  }

  step.value = target
}

async function submit() {
  // The submit button belongs to the last step, but a browser also submits a
  // form when Enter is pressed in a field. Advancing is what that means here.
  if (!isLastStep.value) {
    goTo(step.value + 1)

    return
  }

  for (let index = 0; index < steps.length; index++) {
    if (!validateStep(index)) {
      step.value = index

      return
    }
  }

  const resource: GitLabResource = {
    name: name.value,
    namespace: namespace.value,
    hostname: hostname.value.trim() || undefined,
    edition: edition.value,
    license: parseLicense() ?? undefined,
    postgresql: parseConnection(postgresql, postgresqlError, 'PostgreSQL') ?? undefined,
    redis: parseConnection(valkey, valkeyError, 'Valkey') ?? undefined,
    objectStorage: parseObjectStorage() ?? undefined,
    chart: {
      version: version.value.trim(),
      values: parseValues() ?? {},
    },
  }

  const ok = isEdit
    ? await store.update(namespace.value, name.value, resource)
    : await store.create(resource)

  if (ok) {
    router.push({ name: 'gitlabs' })
  }
}
</script>

<template>
  <section>
    <h1>{{ isEdit ? 'Edit GitLab instance' : 'New GitLab instance' }}</h1>

    <nav class="steps" aria-label="Steps">
      <button
        v-for="(entry, index) in steps"
        :key="entry.title"
        type="button"
        class="step"
        :class="{ 'step--current': index === step, 'step--visited': index < step }"
        :aria-current="index === step ? 'step' : undefined"
        @click="goTo(index)"
      >
        <span class="step-number">{{ index + 1 }}</span>
        <span>{{ entry.title }}</span>
      </button>
    </nav>

    <p class="hint step-summary">{{ currentStep.summary }}</p>

    <p v-if="store.error" class="error" role="alert">{{ store.error }}</p>

    <form class="form" @submit.prevent="submit">
      <!--
        v-show rather than v-if: the values editor is a CodeMirror instance that
        mounts once, and every step keeps its state while another one is shown.
      -->
      <div v-show="step === 0" class="step-panel">
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

      <div v-show="step === 1" class="step-panel">
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
            <input
              v-model="postgresql.host"
              placeholder="gitlab-postgresql.databases.svc.cluster.local"
            />
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
            Artifacts, uploads, and other blobs live in S3 compatible object storage. One Secret
            holds the endpoint, the region, and the credentials, in the
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

      <div v-show="step === 2" class="step-panel">
        <label>
          <span>Chart values (YAML)</span>
          <YamlEditor v-model="valuesText" />
          <span class="hint">
            Merged over the values derived from the earlier steps, and preferred on conflict.
          </span>
        </label>

        <p v-if="valuesError" class="error">{{ valuesError }}</p>
      </div>

      <div class="actions">
        <button v-if="step > 0" type="button" @click="goTo(step - 1)">Back</button>
        <button v-if="!isLastStep" type="button" @click="goTo(step + 1)">Next</button>
        <button v-else type="submit" :disabled="store.loading">
          {{ isEdit ? 'Save' : 'Create' }}
        </button>
        <button type="button" @click="router.push({ name: 'gitlabs' })">Cancel</button>
      </div>
    </form>
  </section>
</template>

<style scoped>
.steps {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin: 1rem 0 0.5rem;
}

.step {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  opacity: 0.7;
}

.step--current {
  border-color: var(--color-border-hover);
  font-weight: 500;
  opacity: 1;
}

.step--visited {
  opacity: 0.9;
}

.step-number {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.4rem;
  height: 1.4rem;
  border-radius: 50%;
  background: var(--color-background-mute);
  font-size: 0.8rem;
}

.step-summary {
  margin-bottom: 1rem;
}

.step-panel {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 640px;
}

label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

label span {
  font-weight: 500;
}

fieldset {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  padding: 0.75rem 1rem 1rem;
}

legend {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 500;
  padding: 0 0.35rem;
}

.legend-icon {
  width: 1.35rem;
  height: 1.35rem;
  object-fit: contain;
}

/*
 * Object storage is a concept rather than a product, so it carries a plain
 * glyph instead of a logo. The mask paints it in the text color, which follows
 * the theme without a variant per scheme.
 */
.legend-icon--glyph {
  background: currentColor;
  mask: url('../assets/icons/bucket.svg') center / contain no-repeat;
  -webkit-mask: url('../assets/icons/bucket.svg') center / contain no-repeat;
  opacity: 0.85;
}

/*
 * The Valkey mark is one solid dark shape, and the project publishes no light
 * variant, so the dark theme renders it as a white silhouette. Its cutouts are
 * holes in the path, so the shape survives that. The PostgreSQL logo carries a
 * white outline of its own and needs no such treatment: flattening it would
 * throw away the outlines that draw the elephant.
 */
@media (prefers-color-scheme: dark) {
  .legend-icon--mono {
    filter: brightness(0) invert(1);
    opacity: 0.85;
  }
}

.hint {
  font-size: 0.85rem;
  font-weight: 400;
  opacity: 0.75;
  margin: 0;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.error {
  color: #d64541;
  white-space: pre-wrap;
}
</style>
