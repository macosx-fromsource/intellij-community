import { defineStore } from 'pinia'
import { ref } from 'vue'

import { api, errorMessage, type SiphonResource } from '@/lib/api/client'

/**
 * Store for the Siphon custom resource of a GitLab instance, backed by the
 * bridge API. A Siphon is an add-on rather than a resource of its own in the
 * user interface, so there is one per instance and the endpoints nest under
 * it: the store holds that one, or `null` when the instance has none.
 *
 * Like the GitLab store, each action clears/sets `error` and toggles
 * `loading`, and returns whether it succeeded.
 */
export const useSiphonsStore = defineStore('siphons', () => {
  const current = ref<SiphonResource | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  /**
   * Why the Siphon of the instance could not be read, or `null` once it was:
   * a caller allowed on GitLab resources but not on Siphons, say. It is not
   * `error`, because the page it was fetched for works without it — only
   * whether the instance has a Siphon is unknown.
   */
  const unreadable = ref<string | null>(null)

  /**
   * Loads the Siphon of an instance. An instance without one is the normal
   * case rather than a failure, so a 404 leaves `current` null and succeeds.
   * Any other failure leaves it unknown: `current` null, and the reason in
   * `unreadable`.
   */
  async function fetchFor(namespace: string, name: string): Promise<boolean> {
    loading.value = true
    error.value = null
    unreadable.value = null
    current.value = null

    const { data, error: err, response } = await api.GET(
      '/api/v1/namespaces/{namespace}/gitlabs/{name}/siphon',
      { params: { path: { namespace, name } } },
    )

    loading.value = false

    if (err) {
      if (response.status === 404) {
        return true
      }

      unreadable.value = errorMessage(err)

      return false
    }

    current.value = data ?? null

    return true
  }

  /**
   * Forgets the Siphon held. The store outlives the page that filled it, so a
   * form that fetches none of its own starts from nothing rather than from
   * whichever instance was open last.
   */
  function reset() {
    current.value = null
    error.value = null
    unreadable.value = null
  }

  /** Creates the Siphon of an instance, or replaces the one it has. */
  async function save(
    namespace: string,
    name: string,
    resource: SiphonResource,
  ): Promise<boolean> {
    loading.value = true
    error.value = null

    const { data, error: err } = await api.PUT(
      '/api/v1/namespaces/{namespace}/gitlabs/{name}/siphon',
      { params: { path: { namespace, name } }, body: resource },
    )

    loading.value = false

    if (err) {
      error.value = errorMessage(err)

      return false
    }

    current.value = data ?? null

    return true
  }

  async function remove(namespace: string, name: string): Promise<boolean> {
    loading.value = true
    error.value = null

    const { error: err } = await api.DELETE(
      '/api/v1/namespaces/{namespace}/gitlabs/{name}/siphon',
      { params: { path: { namespace, name } } },
    )

    loading.value = false

    if (err) {
      error.value = errorMessage(err)

      return false
    }

    current.value = null

    return true
  }

  return { current, loading, error, unreadable, reset, fetchFor, save, remove }
})
