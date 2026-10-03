<script setup lang="ts">
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const prefs = usePreferencesStore()
const { profile } = storeToRefs(prefs)

// Filters live in the URL so a screen can be bookmarked and shared.
const market = computed<Market | undefined>({
  get: () => (route.query.market === 'B3' || route.query.market === 'US' ? route.query.market : undefined),
  set: v => router.replace({ query: { ...route.query, market: v } }),
})
const assetClass = computed<AssetClass | 'all'>({
  get: () => (route.query.class as AssetClass) ?? 'all',
  set: v => router.replace({ query: { ...route.query, class: v === 'all' ? undefined : v } }),
})
const classItems = computed(() => [
  { label: t('common.all'), value: 'all' },
  ...(['stock', 'fii', 'reit'] as const).map(value => ({ label: t(`class.${value}`), value })),
])

const { data, status, error } = await useRankings(() => ({
  market: market.value,
  class: assetClass.value === 'all' ? undefined : assetClass.value,
  profile: profile.value,
  limit: 200,
}))

useHead({ title: () => `${t('explore.title')} · Shinrin` })
</script>

<template>
  <div class="space-y-6">
    <header class="space-y-2">
      <h1 class="text-2xl font-bold">{{ t('explore.title') }}</h1>
      <p class="text-muted">{{ t('explore.intro') }}</p>
    </header>
    <AnalysisDisclaimer />
    <div class="flex flex-wrap items-center gap-3">
      <MarketFilter v-model="market" />
      <USelect v-model="assetClass" :items="classItems" :aria-label="t('class.label')" class="w-40" />
    </div>
    <UAlert v-if="error" color="error" variant="subtle" :description="t('common.loadError', { message: error.statusMessage ?? error.message })" />
    <RankingTable v-else :items="data?.items ?? []" :loading="status === 'pending'" />
  </div>
</template>
