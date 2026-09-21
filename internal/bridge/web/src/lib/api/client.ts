import createClient, { type Middleware } from 'openapi-fetch'

import type { components, paths } from './schema'

/** localStorage key under which the caller's bearer token is persisted. */
export const TOKEN_STORAGE_KEY = 'bridge.token'

/**
 * Typed client for the bridge API. Requests are made relative to the current
 * origin: in production the SPA is served by the bridge itself, and in dev the
 * Vite proxy forwards /api to the bridge (see vite.config.ts).
 */
export const api = createClient<paths>({ baseUrl: '/' })

/**
 * Attaches the caller's bearer token to every request so the bridge acts as
 * that caller. The token is read from localStorage (kept in sync by the auth
 * store) rather than the store itself, to avoid coupling the client to Pinia's
 * lifecycle.
 */
const authMiddleware: Middleware = {
  onRequest({ request }) {
    const token = localStorage.getItem(TOKEN_STORAGE_KEY)
    if (token) {
      request.headers.set('Authorization', `Bearer ${token}`)
    }

    return request
  },
}

api.use(authMiddleware)

export type GitLabResource = components['schemas']['GitLabResource']
export type GitLabList = components['schemas']['GitLabList']
export type SiphonResource = components['schemas']['SiphonResource']
export type ErrorModel = components['schemas']['ErrorModel']
export type StatusDTO = components['schemas']['StatusDTO']
export type Condition = components['schemas']['Condition']
export type ChartVersionsDTO = components['schemas']['ChartVersionsDTO']

/**
 * Extracts a human-readable message from a bridge RFC7807 error body, including
 * the nested `errors[]` details (e.g. admission-webhook denials), each on its
 * own line.
 */
export function errorMessage(error: unknown): string {
  if (error && typeof error === 'object') {
    const model = error as Partial<ErrorModel>
    const parts: string[] = []

    parts.push(model.detail || model.title || '')

    if (Array.isArray(model.errors)) {
      for (const detail of model.errors) {
        if (detail?.message) {
          parts.push(detail.message)
        }
      }
    }

    const message = parts.filter(Boolean).join('\n')
    if (message) {
      return message
    }
  }

  return 'Unexpected error'
}
