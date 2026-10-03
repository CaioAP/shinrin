<script setup lang="ts">
const { t, locale } = useI18n()
const route = useRoute()
const prefs = usePreferencesStore()
const { profile } = storeToRefs(prefs)

const market = () => String(route.params.market)
const symbol = () => String(route.params.symbol)
const range = ref<PriceRange>('1y')

const [{ data: asset, error }, { data: analysis, error: analysisError }, { data: dividends }, { data: news }] = await Promise.all([
  useAsset(market, symbol),
  useAnalysis(market, symbol, profile),
  useDividends(market, symbol),
  useNews(market, symbol),
])
const { data: prices, status: pricesStatus } = usePriceHistory(market, symbol, range)

if (error.value) {
  throw createError({ statusCode: error.value.statusCode ?? 500, statusMessage: error.value.statusMessage, fatal: true })
}

// A 404 on the analysis means "not scored yet", which is a normal state.
const notScored = computed(() => analysisError.value?.statusCode === 404)
const currency = computed(() => asset.value?.currency ?? 'USD')
const missingFactors = computed<Factor[]>(() => {
  if (!analysis.value) return []
  const scored = new Set(analysis.value.factors.map(f => f.factor))
  return (['valuation', 'quality', 'growth', 'momentum', 'income', 'risk', 'sentiment'] as const).filter(f => !scored.has(f))
})
// Literal class names so Tailwind generates them.
const scoreText = { success: 'text-success', warning: 'text-warning', error: 'text-error', neutral: 'text-muted' } as const
const ranges = computed(() => (['1m', '3m', '6m', '1y', '5y', 'max'] as const).map(value => ({ label: t(`range.${value}`), value })))

useHead({ title: () => (asset.value ? `${asset.value.symbol} · Shinrin` : 'Shinrin') })
</script>

<template>
  <div v-if="asset" class="space-y-6">
    <UButton to="/explore" variant="link" icon="i-lucide-arrow-left" class="px-0">{{ t('asset.back') }}</UButton>

    <header class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold">{{ asset.symbol }}</h1>
        <p class="text-muted">{{ asset.name }}</p>
        <div class="flex flex-wrap gap-2 mt-2">
          <UBadge variant="outline" color="neutral">{{ t(`market.${asset.market}`) }}</UBadge>
          <UBadge variant="outline" color="neutral">{{ t(`class.${asset.class}`) }}</UBadge>
          <UBadge variant="outline" :color="asset.sector ? 'neutral' : 'warning'">{{ asset.sector ?? t('asset.noSector') }}</UBadge>
          <UBadge v-if="asset.indexMember" variant="outline" color="primary">{{ t('asset.indexMember') }}</UBadge>
        </div>
      </div>
      <div class="flex flex-col items-end gap-2">
        <div v-if="analysis && analysis.price > 0" class="text-end">
          <p class="text-xs text-muted">{{ t('asset.price') }}</p>
          <p class="text-2xl font-semibold tabular-nums">{{ formatMoney(analysis.price, currency, locale) }}</p>
        </div>
        <AddToWatchlist :asset="{ market: asset.market, symbol: asset.symbol }" />
      </div>
    </header>

    <AnalysisDisclaimer :as-of="analysis?.asOf" />

    <UAlert v-if="notScored" color="neutral" variant="subtle" icon="i-lucide-hourglass" :description="t('asset.notScored')" />
    <UAlert
      v-else-if="analysisError"
      color="error"
      variant="subtle"
      :description="t('common.loadError', { message: analysisError.statusMessage ?? analysisError.message })"
    />

    <template v-if="analysis">
      <div class="grid lg:grid-cols-3 gap-6">
        <UCard>
          <template #header>
            <h2 class="font-medium">{{ t('asset.composite') }}</h2>
          </template>
          <p class="text-5xl font-bold tabular-nums" :class="scoreText[scoreColor(analysis.composite)]">{{ Math.round(analysis.composite) }}</p>
          <p class="text-xs text-muted mt-2">
            {{ t('asset.compositeHint', { profile: t(`profile.${analysis.profile}`), coverage: formatFraction(analysis.coverage, locale, 0) }) }}
          </p>
        </UCard>
        <ViewCard kind="valuation" :view="analysis.valuation" />
        <ViewCard kind="timing" :view="analysis.timing" />
      </div>

      <DataGaps :notes="analysis.notes" :missing-factors="missingFactors" />
    </template>

    <UCard>
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 class="font-medium">{{ t('asset.chart') }}</h2>
            <p class="text-xs text-muted">
              {{ t('asset.chartHint') }}
              <span v-if="prices?.source">{{ t('common.source', { source: prices.source }) }}</span>
            </p>
          </div>
          <UTabs v-model="range" :items="ranges" :content="false" size="xs" />
        </div>
      </template>
      <!-- Prices load on the client only, so the server renders the skeleton. -->
      <ClientOnly>
        <USkeleton v-if="pricesStatus === 'idle' || (pricesStatus === 'pending' && !prices)" class="h-72 w-full" />
        <PriceChart v-else-if="prices?.items.length" :bars="prices.items" :currency="currency" />
        <p v-else class="text-sm text-muted">{{ t('asset.noPrices') }}</p>
        <template #fallback>
          <USkeleton class="h-72 w-full" />
        </template>
      </ClientOnly>
    </UCard>

    <div v-if="analysis" class="grid lg:grid-cols-2 gap-6">
      <UCard>
        <template #header>
          <h2 class="font-medium">{{ t('factor.title') }}</h2>
          <p class="text-xs text-muted">{{ t('factor.hint') }}</p>
        </template>
        <FactorScores :factors="analysis.factors" :currency="currency" />
      </UCard>
      <UCard>
        <template #header>
          <h2 class="font-medium">{{ t('fairValue.title') }}</h2>
          <p class="text-xs text-muted">{{ t('fairValue.hint') }}</p>
        </template>
        <FairValueRanges :fair-values="analysis.fairValues" :price="analysis.price" :currency="currency" />
      </UCard>
    </div>

    <UCard v-if="analysis">
      <template #header>
        <h2 class="font-medium">{{ t('asset.indicators') }}</h2>
      </template>
      <IndicatorGrid :indicators="analysis.indicators" :currency="currency" />
    </UCard>

    <AIReportSection v-if="analysis" :asset="{ market: asset.market, symbol: asset.symbol }" :currency="currency" />

    <div class="grid lg:grid-cols-2 gap-6">
      <UCard>
        <template #header>
          <h2 class="font-medium">{{ t('asset.dividends') }}</h2>
        </template>
        <DividendTable :items="dividends.items" :currency="currency" />
      </UCard>
      <UCard>
        <template #header>
          <h2 class="font-medium">{{ t('asset.news') }}</h2>
        </template>
        <NewsList :items="news.items" />
      </UCard>
    </div>
  </div>
</template>
