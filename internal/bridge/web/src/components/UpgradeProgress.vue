<script setup lang="ts">
import { computed } from 'vue'

import type { GitLabResource } from '@/lib/api/client'
import { deriveUpgrade } from '@/lib/upgrade'
import { upgradePath } from '@/lib/versions'

const props = defineProps<{ resource: GitLabResource; availableVersions?: string[] }>()

const view = computed(() => deriveUpgrade(props.resource))

/** The version currently being upgraded to; its incoming arrow is highlighted. */
const activeVersion = computed(() => view.value?.intermediateVersion ?? '')

const targetVersion = computed(() => view.value?.targetVersion ?? '')

/**
 * The full version chain to render: the deployed version followed by each hop up
 * to the target. Falls back to the known intermediate + target when the catalog
 * of available versions is not supplied.
 */
const chain = computed<string[]>(() => {
  const v = view.value
  if (!v) {
    return []
  }

  // With the catalog we can render every hop up to the target. Without it (the
  // chart-versions fetch failed or has not resolved), upgradePath would still
  // return just [target] and collapse the chain to current -> target, hiding the
  // in-flight intermediate that deriveUpgrade knows from the Upgradeable
  // condition — so fall back to that intermediate instead.
  const available = props.availableVersions ?? []
  const list =
    available.length > 0
      ? upgradePath(v.currentVersion, v.targetVersion, available)
      : [v.intermediateVersion, v.targetVersion]
  const full = [v.currentVersion, ...list].filter((s): s is string => !!s)

  // Collapse consecutive duplicates (single-hop, or current already equal to a hop).
  return full.filter((s, i) => i === 0 || s !== full[i - 1])
})

const glyph: Record<string, string> = {
  done: '✓',
  failed: '✕',
  pending: '○',
}
</script>

<template>
  <div v-if="view" class="upgrade">
    <div class="versions">
      <template v-for="(ver, i) in chain" :key="ver">
        <span
          v-if="i > 0"
          class="arrow"
          :class="{ 'arrow--active': ver === activeVersion }"
          aria-hidden="true"
          >→</span
        >
        <span
          class="version"
          :class="{ 'version--active': ver === activeVersion, 'version--target': ver === targetVersion }"
          >{{ ver }}</span
        >
      </template>
    </div>

    <p
      v-if="view.failed"
      class="banner banner--failed"
      role="alert"
    >
      {{ view.blocked ? 'Upgrade blocked' : 'Upgrade failed' }}: {{ view.message }}
    </p>

    <ol class="gates" aria-label="Upgrade gates">
      <li v-for="gate in view.gates" :key="gate.key" class="gate" :class="`gate--${gate.state}`">
        <span class="gate-mark" aria-hidden="true">
          <span v-if="gate.state === 'active'" class="spinner"></span>
          <template v-else>{{ glyph[gate.state] }}</template>
        </span>
        <span class="gate-title">{{ gate.title }}</span>
        <span v-if="gate.state === 'active'" class="gate-message hint">{{ view.message }}</span>
      </li>
    </ol>
  </div>
</template>

<style scoped>
.upgrade {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  padding: 0.25rem 0;
}

.versions {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  font-size: 0.95rem;
}

.version {
  font-weight: 500;
}

.version--target {
  color: var(--color-heading);
}

.version--active {
  color: #3fb950;
  font-weight: 600;
}

.arrow {
  opacity: 0.6;
}

.arrow--active {
  color: #3fb950;
  opacity: 1;
}

.banner {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  white-space: pre-wrap;
}

.banner--failed {
  border: 1px solid color-mix(in srgb, #f85149 40%, transparent);
  background: color-mix(in srgb, #f85149 12%, transparent);
  color: #f85149;
}

.gates {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.gate {
  display: grid;
  grid-template-columns: 1.4rem 1fr;
  align-items: center;
  column-gap: 0.6rem;
  row-gap: 0.15rem;
  opacity: 0.55;
}

.gate--done,
.gate--active,
.gate--failed {
  opacity: 1;
}

.gate-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.4rem;
  height: 1.4rem;
  border-radius: 50%;
  background: var(--color-background-mute);
  font-size: 0.8rem;
}

.gate--done .gate-mark {
  color: #3fb950;
}

.gate--failed .gate-mark {
  color: #f85149;
}

.gate--active .gate-title {
  font-weight: 500;
}

.gate-message {
  grid-column: 2;
}

.spinner {
  width: 0.85rem;
  height: 0.85rem;
  border: 2px solid color-mix(in srgb, #58a6ff 30%, transparent);
  border-top-color: #58a6ff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .spinner {
    animation: none;
  }
}
</style>
