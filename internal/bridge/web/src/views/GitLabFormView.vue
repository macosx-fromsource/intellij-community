<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { parse, stringify } from 'yaml'

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

const namespace = ref(props.namespace ?? 'default')
const name = ref(props.name ?? '')
const version = ref('')
const valuesText = ref('')
const valuesError = ref<string | null>(null)

onMounted(async () => {
  if (!isEdit) {
    return
  }

  const ok = await store.fetchOne(props.namespace!, props.name!)
  if (ok && store.current) {
    version.value = store.current.chart.version ?? ''

    const values = store.current.chart.values ?? {}
    valuesText.value = Object.keys(values).length ? stringify(values) : ''
  }
})

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

async function submit() {
  const values = parseValues()
  if (values === null) {
    return
  }

  const resource: GitLabResource = {
    name: name.value,
    namespace: namespace.value,
    chart: {
      version: version.value || undefined,
      values,
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

    <p v-if="store.error" class="error" role="alert">{{ store.error }}</p>

    <form class="form" @submit.prevent="submit">
      <label>
        <span>Namespace</span>
        <input v-model="namespace" :disabled="isEdit" required />
      </label>

      <label>
        <span>Name</span>
        <input v-model="name" :disabled="isEdit" required />
      </label>

      <label>
        <span>Chart version</span>
        <input v-model="version" placeholder="e.g. 9.11.1" />
      </label>

      <label>
        <span>Chart values (YAML)</span>
        <YamlEditor v-model="valuesText" />
      </label>

      <p v-if="valuesError" class="error">{{ valuesError }}</p>

      <div class="actions">
        <button type="submit" :disabled="store.loading">
          {{ isEdit ? 'Save' : 'Create' }}
        </button>
        <button type="button" @click="router.push({ name: 'gitlabs' })">Cancel</button>
      </div>
    </form>
  </section>
</template>

<style scoped>
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

.actions {
  display: flex;
  gap: 0.5rem;
}

.error {
  color: #d64541;
  white-space: pre-wrap;
}
</style>
