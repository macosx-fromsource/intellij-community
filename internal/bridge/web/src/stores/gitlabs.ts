import { defineStore } from 'pinia'
import { ref } from 'vue'

import { api, errorMessage, type GitLabResource } from '@/lib/api/client'

/**
 * Store for GitLab custom resources, backed by the bridge CRUD API. Each action
 * clears/sets `error` and toggles `loading`, and returns a boolean indicating
 * success so views can navigate only when the operation succeeded.
 */
export const useGitLabsStore = defineStore('gitlabs', () => {
  const items = ref<GitLabResource[]>([])
  const current = ref<GitLabResource | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchAll(): Promise<boolean> {
    loading.value = true
    error.value = null

    const { data, error: err } = await api.GET('/api/v1/gitlabs')

    loading.value = false

    if (err) {
      error.value = errorMessage(err)

      return false
    }

    items.value = data?.items ?? []

    return true
  }

  async function fetchOne(namespace: string, name: string): Promise<boolean> {
    loading.value = true
    error.value = null
    current.value = null

    const { data, error: err } = await api.GET(
      '/api/v1/namespaces/{namespace}/gitlabs/{name}',
      { params: { path: { namespace, name } } },
    )

    loading.value = false

    if (err) {
      error.value = errorMessage(err)

      return false
    }

    current.value = data ?? null

    return true
  }

  async function create(resource: GitLabResource): Promise<boolean> {
    loading.value = true
    error.value = null

    const { error: err } = await api.POST('/api/v1/namespaces/{namespace}/gitlabs', {
      params: { path: { namespace: resource.namespace } },
      body: resource,
    })

    loading.value = false

    if (err) {
      error.value = errorMessage(err)

      return false
    }

    return true
  }

  async function update(
    namespace: string,
    name: string,
    resource: GitLabResource,
  ): Promise<boolean> {
    loading.value = true
    error.value = null

    const { error: err } = await api.PUT('/api/v1/namespaces/{namespace}/gitlabs/{name}', {
      params: { path: { namespace, name } },
      body: resource,
    })

    loading.value = false

    if (err) {
      error.value = errorMessage(err)

      return false
    }

    return true
  }

  async function remove(namespace: string, name: string): Promise<boolean> {
    loading.value = true
    error.value = null

    const { error: err } = await api.DELETE('/api/v1/namespaces/{namespace}/gitlabs/{name}', {
      params: { path: { namespace, name } },
    })

    loading.value = false

    if (err) {
      error.value = errorMessage(err)

      return false
    }

    items.value = items.value.filter(
      (item) => !(item.namespace === namespace && item.name === name),
    )

    return true
  }

  return { items, current, loading, error, fetchAll, fetchOne, create, update, remove }
})
