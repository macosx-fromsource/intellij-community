<script setup lang="ts">
import { nextTick, onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'

import StatusBadge from '@/components/StatusBadge.vue'
import UpgradeProgress from '@/components/UpgradeProgress.vue'
import type { GitLabResource } from '@/lib/api/client'
import { deriveUpgrade, isUpgrading } from '@/lib/upgrade'
import { higherVersions } from '@/lib/versions'
import { useGitLabsStore } from '@/stores/gitlabs'

const store = useGitLabsStore()
const router = useRouter()

/** Keys of the rows whose upgrade detail panel is expanded. */
const expanded = reactive(new Set<string>())

/** Rows currently showing the version picker (Upgrade clicked, not yet confirmed). */
const picking = reactive(new Set<string>())

/** Rows with an upgrade request in flight. */
const busy = reactive(new Set<string>())

/** Per-row chosen upgrade target, keyed by rowKey; defaults to the latest. */
const selected = reactive<Record<string, string>>({})

/** Live references to each row's version <select>, so it can be opened on demand. */
const selectRefs = new Map<string, HTMLSelectElement>()

function setSelectRef(key: string, el: unknown) {
  if (el instanceof HTMLSelectElement) {
    selectRefs.set(key, el)
  } else {
    selectRefs.delete(key)
  }
}

function rowKey(item: GitLabResource): string {
  return `${item.namespace}/${item.name}`
}

/** True when there is an upgrade in flight or a failure worth expanding into. */
function hasUpgradeInfo(item: GitLabResource): boolean {
  return deriveUpgrade(item) !== null
}

/** The version actually deployed, falling back to the desired one before first install. */
function deployedVersion(item: GitLabResource): string {
  return item.status?.version || item.chart.version || ''
}

/** Bundled versions higher than the version currently deployed. */
function upgradeCandidates(item: GitLabResource): string[] {
  return higherVersions(deployedVersion(item), store.chartVersions)
}

/** An upgrade can be offered when a higher version exists and none is in flight. */
function canUpgrade(item: GitLabResource): boolean {
  return !isUpgrading(item) && upgradeCandidates(item).length > 0
}

/** True while an upgrade request is in flight or the instance is upgrading. */
function isBusy(item: GitLabResource): boolean {
  return busy.has(rowKey(item)) || isUpgrading(item)
}

/** True once versions are known and the instance is already at the newest one. */
function isLatest(item: GitLabResource): boolean {
  return (
    store.chartVersions.length > 0 &&
    !isUpgrading(item) &&
    !!deployedVersion(item) &&
    upgradeCandidates(item).length === 0
  )
}

/** The chosen target for a row, defaulting to the latest candidate. */
function selectedVersion(item: GitLabResource): string {
  return selected[rowKey(item)] ?? upgradeCandidates(item)[0] ?? ''
}

async function startPicking(item: GitLabResource) {
  const key = rowKey(item)
  picking.add(key)

  // Open the dropdown as soon as it renders, so the user picks a version straight
  // away. showPicker needs the click's user activation, which survives nextTick.
  await nextTick()

  const select = selectRefs.get(key) as (HTMLSelectElement & { showPicker?: () => void }) | undefined
  if (select && typeof select.showPicker === 'function') {
    try {
      select.showPicker()
    } catch {
      // Opening the picker can be blocked or unsupported; the closed dropdown still works.
    }
  }
}

function cancelPicking(item: GitLabResource) {
  picking.delete(rowKey(item))
}

function onSelect(item: GitLabResource, event: Event) {
  selected[rowKey(item)] = (event.target as HTMLSelectElement).value
}

// Starts the upgrade to the row's selected version. Confirm is the deliberate
// second step (Upgrade reveals the picker), so no extra dialog is shown.
async function confirmUpgrade(item: GitLabResource) {
  const version = selectedVersion(item)
  if (!version) {
    return
  }

  const key = rowKey(item)

  picking.delete(key)
  busy.add(key)

  const updated: GitLabResource = { ...item, status: undefined, chart: { ...item.chart, version } }

  if (await store.update(item.namespace, item.name, updated)) {
    await store.fetchAll()
  }

  busy.delete(key)
}

function toggle(item: GitLabResource) {
  const key = rowKey(item)

  if (expanded.has(key)) {
    expanded.delete(key)
  } else {
    expanded.add(key)
  }
}

onMounted(() => {
  store.fetchAll()
  store.fetchChartVersions()
})

function editResource(item: GitLabResource) {
  router.push({
    name: 'gitlab-edit',
    params: { namespace: item.namespace, name: item.name },
  })
}

async function deleteResource(item: GitLabResource) {
  if (!confirm(`Delete GitLab "${item.name}" in "${item.namespace}"?`)) {
    return
  }

  await store.remove(item.namespace, item.name)
}
</script>

<template>
  <section>
    <div class="toolbar">
      <h1>GitLab instances</h1>
      <div class="actions">
        <button :disabled="store.loading" @click="store.fetchAll()">Refresh</button>
        <button @click="router.push({ name: 'gitlab-create' })">New</button>
      </div>
    </div>

    <p v-if="store.error" class="error" role="alert">{{ store.error }}</p>
    <p v-if="store.loading">Loading…</p>

    <table v-if="store.items.length">
      <thead>
        <tr>
          <th class="expander-col"></th>
          <th>Namespace</th>
          <th>Name</th>
          <th>Hostname</th>
          <th>Edition</th>
          <th>Chart version</th>
          <th>Phase</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="item in store.items" :key="rowKey(item)">
          <tr>
            <td class="expander-col">
              <button
                v-if="hasUpgradeInfo(item)"
                class="expander"
                :aria-expanded="expanded.has(rowKey(item))"
                :aria-label="expanded.has(rowKey(item)) ? 'Collapse upgrade details' : 'Show upgrade details'"
                @click="toggle(item)"
              >
                {{ expanded.has(rowKey(item)) ? '▾' : '▸' }}
              </button>
            </td>
            <td>{{ item.namespace }}</td>
            <td>{{ item.name }}</td>
            <td>{{ item.hostname || '—' }}</td>
            <td>{{ item.edition?.toUpperCase() ?? '—' }}</td>
            <td>
              <div class="chart-cell">
                <span>{{ deployedVersion(item) || '—' }}</span>
                <span v-if="isLatest(item)" class="muted">Latest version</span>
              </div>
            </td>
            <td><StatusBadge :phase="item.status?.phase" /></td>
            <td class="row-actions">
              <button @click="editResource(item)">Edit</button>
              <button @click="deleteResource(item)">Delete</button>

              <span v-if="isBusy(item)" class="upgrading">
                <span class="spinner" aria-hidden="true"></span>
                <span class="muted">Upgrading…</span>
              </span>
              <template v-else-if="picking.has(rowKey(item))">
                <button class="upgrade-button" @click="confirmUpgrade(item)">Confirm</button>
                <select
                  :ref="(el) => setSelectRef(rowKey(item), el)"
                  class="version-select"
                  aria-label="Target upgrade version"
                  :value="selectedVersion(item)"
                  @change="onSelect(item, $event)"
                >
                  <option v-for="v in upgradeCandidates(item)" :key="v" :value="v">{{ v }}</option>
                </select>
                <button class="link-button" @click="cancelPicking(item)">Cancel</button>
              </template>
              <button
                v-else-if="canUpgrade(item)"
                class="upgrade-button"
                @click="startPicking(item)"
              >
                Upgrade
              </button>
            </td>
          </tr>
          <tr v-if="expanded.has(rowKey(item))" class="detail-row">
            <td></td>
            <td colspan="7">
              <UpgradeProgress :resource="item" :available-versions="store.chartVersions" />
            </td>
          </tr>
        </template>
      </tbody>
    </table>

    <p v-else-if="!store.loading">No GitLab instances yet.</p>
  </section>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
}

.actions,
.row-actions {
  display: flex;
  gap: 0.5rem;
}

.row-actions {
  align-items: center;
  flex-wrap: nowrap;
}

/* Keep the main rows on a single line so expanding the actions (dropdown, Confirm)
   doesn't reflow names onto two lines. The detail row is excluded so the upgrade
   breakdown can wrap normally. */
thead th,
tbody tr:not(.detail-row) > td {
  white-space: nowrap;
}

.error {
  color: #d64541;
  margin-bottom: 1rem;
  white-space: pre-wrap;
}

.expander-col {
  width: 1.5rem;
  padding-right: 0;
}

.expander {
  padding: 0 0.25rem;
  border: none;
  background: none;
  color: var(--color-text);
  font-size: 0.9rem;
  line-height: 1;
  cursor: pointer;
}

.detail-row > td {
  background: var(--color-background-soft);
  vertical-align: top;
}

.chart-cell {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.version-select {
  width: auto;
  padding: 0.15rem 0.3rem;
}

.muted {
  opacity: 0.55;
  font-style: italic;
  font-size: 0.85rem;
}

.upgrade-button {
  padding: 0.15rem 0.7rem;
  border: none;
  border-radius: 4px;
  background: hsla(160, 100%, 37%, 1);
  color: #fff;
  cursor: pointer;
}

.upgrade-button:hover {
  background: hsla(160, 100%, 30%, 1);
}

.link-button {
  padding: 0 0.25rem;
  border: none;
  background: none;
  color: var(--color-text);
  opacity: 0.75;
  cursor: pointer;
}

.upgrading {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.spinner {
  display: inline-block;
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
