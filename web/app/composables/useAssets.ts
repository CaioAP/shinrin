import type { MaybeRefOrGetter } from 'vue'

/** Reactive asset list: refetches whenever the filter changes. */
export function useAssets(filter: MaybeRefOrGetter<AssetFilter>) {
  return useFetch('/api/assets', {
    query: computed(() => toValue(filter)),
    default: (): ListResponse<Asset> => ({ items: [] }),
  })
}

/** One asset by market and symbol. */
export function useAsset(market: MaybeRefOrGetter<string>, symbol: MaybeRefOrGetter<string>) {
  return useFetch(() => `/api/assets/${toValue(market)}/${toValue(symbol)}`)
}
