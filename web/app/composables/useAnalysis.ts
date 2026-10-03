import type { MaybeRefOrGetter } from 'vue'

/** One asset's analysis (scores, fair values, views) for a risk profile. */
export function useAnalysis(
  market: MaybeRefOrGetter<string>,
  symbol: MaybeRefOrGetter<string>,
  profile: MaybeRefOrGetter<RiskProfile> = 'moderate',
) {
  return useFetch(() => `/api/assets/${toValue(market)}/${toValue(symbol)}/analysis`, {
    query: computed(() => ({ profile: toValue(profile) })),
  })
}

/** Assets ranked by composite score for a profile (the screener). */
export function useRankings(filter: MaybeRefOrGetter<RankingFilter>) {
  return useFetch('/api/rankings', {
    query: computed(() => toValue(filter)),
  })
}

/** Allocation bands for a profile, tilted by macro conditions. */
export function useOutlook(profile: MaybeRefOrGetter<RiskProfile>) {
  return useFetch('/api/outlook', {
    query: computed(() => ({ profile: toValue(profile) })),
  })
}
