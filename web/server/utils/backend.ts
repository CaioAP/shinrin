import type { H3Event } from 'h3'
import { FetchError } from 'ofetch'

type BackendOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  query?: Record<string, string | number | boolean | undefined>
  body?: Record<string, unknown>
}

/**
 * Calls the Go API from the Nuxt server. Every server route goes through this
 * one function, so the base URL, headers and error mapping live in one place.
 * The session cookie, when present, is forwarded as a bearer token. Backend
 * errors keep their status code (404 stays 404); an unreachable backend
 * becomes 502.
 */
export async function backendFetch<T>(event: H3Event, path: string, opts: BackendOptions = {}): Promise<T> {
  const { apiBase } = useRuntimeConfig(event)
  const token = getSessionToken(event)
  try {
    return await $fetch<T, string>(path, {
      baseURL: apiBase,
      method: opts.method ?? 'GET',
      query: opts.query,
      body: opts.body,
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    })
  }
  catch (err) {
    if (err instanceof FetchError && err.statusCode) {
      if (err.statusCode === 401 && token) clearSessionToken(event)
      throw createError({ statusCode: err.statusCode, statusMessage: err.data?.error ?? err.statusMessage })
    }
    throw createError({ statusCode: 502, statusMessage: 'Shinrin API unavailable' })
  }
}

/** Reads a JSON object body, or fails with 400. */
export async function readJSONBody(event: H3Event): Promise<Record<string, unknown>> {
  const body = await readBody(event).catch(() => undefined)
  if (!body || typeof body !== 'object' || Array.isArray(body)) {
    throw createError({ statusCode: 400, statusMessage: 'request body must be a JSON object' })
  }
  return body as Record<string, unknown>
}
