<script setup lang="ts">
import { computed, ref } from 'vue'
import { parse, stringify } from 'yaml'

import YamlEditor from '@/components/YamlEditor.vue'
import { useDirtyTracking } from '@/composables/useDirtyTracking'
import type { GitLabResource } from '@/lib/api/client'

const valuesText = ref('')
const valuesError = ref<string | null>(null)

const hasError = computed(() => Boolean(valuesError.value))

const { isDirty, resetBaseline } = useDirtyTracking(() => ({ valuesText: valuesText.value }))

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

/** Checks the section and shows what it found. */
function validate(): boolean {
  return parseValues() !== null
}

function loadFrom(resource: GitLabResource) {
  const values = resource.chart.values ?? {}
  valuesText.value = Object.keys(values).length ? stringify(values) : ''
}

function toPartial(): Record<string, unknown> {
  return parseValues() ?? {}
}

defineExpose({ hasError, isDirty, validate, loadFrom, resetBaseline, toPartial })
</script>

<template>
  <div class="section-panel">
    <label>
      <span>Chart values (YAML)</span>
      <YamlEditor v-model="valuesText" />
      <span class="hint">
        Merged over the values derived from the earlier sections, and preferred on conflict.
      </span>
    </label>

    <p v-if="valuesError" class="error">{{ valuesError }}</p>
  </div>
</template>
