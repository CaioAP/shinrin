import type { MaybeRefOrGetter } from 'vue'

/** The signed-in user's watchlists, plus the mutations that change them. */
export async function useWatchlists() {
  const fetched = await useFetch('/api/watchlists', {
    key: 'watchlists',
    default: (): ListResponse<Watchlist> => ({ items: [] }),
  })

  async function create(name: string) {
    const w = await $fetch<Watchlist>('/api/watchlists', { method: 'POST', body: { name } })
    await fetched.refresh()
    return w
  }
  async function rename(id: number, name: string) {
    await $fetch(`/api/watchlists/${id}`, { method: 'PATCH', body: { name } })
    await refreshNuxtData()
  }
  async function remove(id: number) {
    await $fetch(`/api/watchlists/${id}`, { method: 'DELETE' })
    await fetched.refresh()
  }
  async function addAsset(id: number, asset: AssetKey) {
    await $fetch(`/api/watchlists/${id}/items/${asset.market}/${asset.symbol}`, { method: 'PUT' })
    await refreshNuxtData()
  }
  async function removeAsset(id: number, asset: AssetKey) {
    await $fetch(`/api/watchlists/${id}/items/${asset.market}/${asset.symbol}`, { method: 'DELETE' })
    await refreshNuxtData()
  }

  return { ...fetched, create, rename, remove, addAsset, removeAsset }
}

/** One watchlist with each asset's scores for a profile. */
export function useWatchlist(id: MaybeRefOrGetter<number>, profile: MaybeRefOrGetter<RiskProfile>) {
  return useFetch(() => `/api/watchlists/${toValue(id)}`, {
    query: computed(() => ({ profile: toValue(profile) })),
  })
}

/** The risk questionnaire's question and answer codes. */
export function useQuestionnaire() {
  return useFetch('/api/risk-questionnaire', { key: 'questionnaire' })
}

/** A readable message from a failed $fetch. */
export function errorMessage(err: unknown): string {
  const e = err as { data?: { statusMessage?: string, message?: string }, statusMessage?: string, message?: string }
  return e.data?.statusMessage ?? e.statusMessage ?? e.data?.message ?? e.message ?? String(err)
}
