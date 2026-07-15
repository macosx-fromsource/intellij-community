import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { TOKEN_STORAGE_KEY } from '@/lib/api/client'

/**
 * Store for the caller's Kubernetes bearer token. The bridge acts as the caller
 * identified by this token (like the old Kubernetes Dashboard), so the SPA must
 * attach it to every API request; see the openapi-fetch middleware in
 * `@/lib/api/client`. The token is persisted to localStorage so it survives a
 * reload. Note: localStorage is readable by any script on the page, so this is a
 * PoC-grade store for short-lived tokens (mint one with `kubectl create token`).
 */
export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem(TOKEN_STORAGE_KEY) ?? '')

  const isAuthenticated = computed(() => token.value !== '')

  function setToken(value: string): void {
    token.value = value.trim()

    if (token.value) {
      localStorage.setItem(TOKEN_STORAGE_KEY, token.value)
    } else {
      localStorage.removeItem(TOKEN_STORAGE_KEY)
    }
  }

  function clearToken(): void {
    setToken('')
  }

  return { token, isAuthenticated, setToken, clearToken }
})
