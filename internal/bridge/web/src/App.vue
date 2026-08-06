<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, RouterView } from 'vue-router'

import { isLocalAuthMode } from '@/lib/authMode'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const localMode = isLocalAuthMode()
const draft = ref(auth.token)
const editing = ref(!auth.isAuthenticated)

function save(): void {
  auth.setToken(draft.value)
  editing.value = false
}

function edit(): void {
  draft.value = auth.token
  editing.value = true
}

function clear(): void {
  auth.clearToken()
  draft.value = ''
  editing.value = true
}
</script>

<template>
  <header class="app-header">
    <RouterLink to="/" class="brand">GitLab Operator</RouterLink>
    <div class="token-bar">
      <template v-if="localMode">
        <span class="token-status" title="Authenticated via your kubeconfig (kubectl bridge)">
          Local — kubeconfig identity
        </span>
      </template>
      <template v-else-if="editing">
        <input
          v-model="draft"
          type="password"
          class="token-input"
          placeholder="Bearer token (kubectl create token …)"
          autocomplete="off"
          @keyup.enter="save"
        />
        <button type="button" @click="save">Save token</button>
      </template>
      <template v-else>
        <span class="token-status" title="A bearer token is set">Token set</span>
        <button type="button" @click="edit">Change</button>
        <button type="button" @click="clear">Clear</button>
      </template>
      <a href="/docs" target="_blank" rel="noopener">API docs</a>
    </div>
  </header>

  <main class="app-main">
    <RouterView />
  </main>
</template>

<style scoped>
.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1.5rem;
  border-bottom: 1px solid var(--color-border);
}

.brand {
  font-weight: 600;
  font-size: 1.1rem;
  color: var(--color-heading);
  text-decoration: none;
}

.token-bar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.token-input {
  width: 16rem;
  padding: 0.3rem 0.5rem;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  background: var(--color-background);
  color: var(--color-text);
}

.token-status {
  font-size: 0.9rem;
  color: var(--color-text);
}

.token-bar a {
  color: var(--color-text);
  text-decoration: none;
}

.token-bar a:hover {
  text-decoration: underline;
}

.app-main {
  max-width: 960px;
  margin: 0 auto;
  padding: 1.5rem;
}
</style>
