import type { MaybeRefOrGetter } from 'vue'

/** The signed-in user's LLM key settings and monthly usage, with the mutations. */
export async function useLLMSettings() {
  const fetched = await useFetch('/api/me/llm', { key: 'llm-settings' })

  async function save(input: LLMSettingsInput) {
    fetched.data.value = await $fetch<LLMSettings>('/api/me/llm', { method: 'PUT', body: input })
  }
  async function test() {
    await $fetch('/api/me/llm/test', { method: 'POST' })
  }
  async function remove() {
    await $fetch('/api/me/llm', { method: 'DELETE' })
    await fetched.refresh()
  }

  return { ...fetched, save, test, remove }
}

/** The user's own AI reports on one asset, newest first, and generate(). */
export async function useAssetReports(market: MaybeRefOrGetter<string>, symbol: MaybeRefOrGetter<string>) {
  const path = () => `/api/assets/${toValue(market)}/${toValue(symbol)}/reports`
  const fetched = await useFetch<ListResponse<AIReport>>(path, {
    key: computed(() => `reports-${toValue(market)}-${toValue(symbol)}`),
    default: (): ListResponse<AIReport> => ({ items: [] }),
  })

  /** Writes a new report with the user's key; takes a minute or two. */
  async function generate(profile: RiskProfile, lang: string) {
    const r = await $fetch<AIReport>(path(), { method: 'POST', body: { profile, lang } })
    fetched.data.value = { items: [r, ...fetched.data.value.items] }
    await refreshNuxtData('llm-settings')
    return r
  }

  return { ...fetched, generate }
}
