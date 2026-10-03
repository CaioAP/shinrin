import type { MaybeRefOrGetter } from 'vue'

/** Daily prices for the chart, adjusted for dividends and splits. */
export function usePriceHistory(
  market: MaybeRefOrGetter<string>,
  symbol: MaybeRefOrGetter<string>,
  range: MaybeRefOrGetter<PriceRange>,
) {
  return useFetch(() => `/api/assets/${toValue(market)}/${toValue(symbol)}/prices`, {
    query: computed(() => ({ range: toValue(range) })),
    // The chart renders on the client only, so there is no point fetching on the server.
    server: false,
  })
}

/** Cash distributions (dividends and JCP) of the last five years. */
export function useDividends(market: MaybeRefOrGetter<string>, symbol: MaybeRefOrGetter<string>) {
  return useFetch(() => `/api/assets/${toValue(market)}/${toValue(symbol)}/dividends`, {
    default: (): ListResponse<Dividend> => ({ items: [] }),
  })
}

/** Headlines and regulatory filings of the last 90 days. */
export function useNews(market: MaybeRefOrGetter<string>, symbol: MaybeRefOrGetter<string>) {
  return useFetch(() => `/api/assets/${toValue(market)}/${toValue(symbol)}/news`, {
    default: (): ListResponse<NewsItem> => ({ items: [] }),
  })
}

/** Headline macro numbers for the dashboard strip. */
export function useMacro() {
  return useFetch('/api/macro', { key: 'macro' })
}
