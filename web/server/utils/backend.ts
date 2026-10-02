import type { H3Event } from 'h3'
import { FetchError } from 'ofetch'

type BackendOptions = { query?: Record<string, string | number | boolean | undefined> }

/**
 * Calls the Go API from the Nuxt server. Every server route goes through this
 * one function, so the base URL, headers and error mapping live in one place.
 * Backend errors keep their status code (404 stays 404); an unreachable
 * backend becomes 502.
 */
export async function backendFetch<T>(event: H3Event, path: string, opts: BackendOptions = {}): Promise<T> {
  const { apiBase } = useRuntimeConfig(event)
  try {
    return await $fetch<T, string>(path, { baseURL: apiBase, query: opts.query })
  }
  catch (err) {
    if (err instanceof FetchError && err.statusCode) {
      throw createError({ statusCode: err.statusCode, statusMessage: err.data?.error ?? err.statusMessage })
    }
    throw createError({ statusCode: 502, statusMessage: 'Shinrin API unavailable' })
  }
}
