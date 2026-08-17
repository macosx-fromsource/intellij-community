<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

import type { GitLabResource } from '@/lib/api/client'
import { useGitLabsStore } from '@/stores/gitlabs'

const store = useGitLabsStore()
const router = useRouter()

onMounted(() => store.fetchAll())

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
        <tr v-for="item in store.items" :key="`${item.namespace}/${item.name}`">
          <td>{{ item.namespace }}</td>
          <td>{{ item.name }}</td>
          <td>{{ item.hostname || '—' }}</td>
          <td>{{ item.edition?.toUpperCase() ?? '—' }}</td>
          <td>{{ item.chart.version ?? '—' }}</td>
          <td>{{ item.status?.phase ?? '—' }}</td>
          <td class="row-actions">
            <button @click="editResource(item)">Edit</button>
            <button @click="deleteResource(item)">Delete</button>
          </td>
        </tr>
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

.error {
  color: #d64541;
  margin-bottom: 1rem;
  white-space: pre-wrap;
}
</style>
