import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'

import { api } from '@/lib/api/client'
import { useGitLabsStore } from './gitlabs'

vi.mock('@/lib/api/client', () => ({
  api: {
    GET: vi.fn<() => Promise<unknown>>(),
    POST: vi.fn<() => Promise<unknown>>(),
    PUT: vi.fn<() => Promise<unknown>>(),
    DELETE: vi.fn<() => Promise<unknown>>(),
  },
  errorMessage: (error: unknown) => (error as { detail?: string })?.detail ?? 'error',
}))

// The mocked client is untyped so mockResolvedValue accepts plain fixtures.
const mockApi = api as unknown as { GET: Mock; POST: Mock; PUT: Mock; DELETE: Mock }

describe('gitlabs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('fetchAll populates items from the list response', async () => {
    mockApi.GET.mockResolvedValue({
      data: { items: [{ name: 'a', namespace: 'ns', chart: {} }] },
      error: undefined,
    })

    const store = useGitLabsStore()
    const ok = await store.fetchAll()

    expect(ok).toBe(true)
    expect(store.items).toHaveLength(1)
    expect(store.error).toBeNull()
    expect(mockApi.GET).toHaveBeenCalledWith('/api/v1/gitlabs')
  })

  it('fetchAll surfaces the error detail on failure', async () => {
    mockApi.GET.mockResolvedValue({ data: undefined, error: { detail: 'boom' } })

    const store = useGitLabsStore()
    const ok = await store.fetchAll()

    expect(ok).toBe(false)
    expect(store.error).toBe('boom')
  })

  it('create posts to the namespaced path with the resource body', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const store = useGitLabsStore()
    const resource = {
      name: 'demo',
      namespace: 'ns',
      hostname: 'gitlab.example.com',
      edition: 'ee' as const,
      license: { secretRef: { name: 'gitlab-license', key: 'license' } },
      postgresql: {
        host: 'gitlab-postgresql',
        passwordSecretRef: { name: 'gitlab-postgresql-password', key: 'password' },
      },
      redis: {
        host: 'gitlab-valkey',
        passwordSecretRef: { name: 'gitlab-valkey-auth', key: 'default' },
      },
      chart: { version: '9.11.1', values: {} },
    }
    const ok = await store.create(resource)

    expect(ok).toBe(true)
    expect(mockApi.POST).toHaveBeenCalledWith('/api/v1/namespaces/{namespace}/gitlabs', {
      params: { path: { namespace: 'ns' } },
      body: resource,
    })
  })
})
