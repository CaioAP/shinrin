<script setup lang="ts">
const { t } = useI18n()
const prefs = usePreferencesStore()
const { profile } = storeToRefs(prefs)

const [{ data: macro, error: macroError }, { data: outlook, error: outlookError }, { data: b3 }, { data: us }] = await Promise.all([
  useMacro(),
  useOutlook(profile),
  useRankings(() => ({ market: 'B3', profile: profile.value, limit: 5 })),
  useRankings(() => ({ market: 'US', profile: profile.value, limit: 5 })),
])

const tops = computed(() => [
  { market: 'B3' as const, data: b3.value },
  { market: 'US' as const, data: us.value },
])

useHead({ title: 'Shinrin' })
</script>

<template>
  <div class="space-y-10">
    <section class="space-y-4">
      <h1 class="text-3xl font-bold">{{ t('home.title') }}</h1>
      <p class="text-muted max-w-3xl">{{ t('home.intro') }}</p>
      <AnalysisDisclaimer />
      <UButton to="/explore" trailing-icon="i-lucide-arrow-right">{{ t('home.explore') }}</UButton>
    </section>

    <section class="space-y-3">
      <h2 class="text-xl font-semibold">{{ t('home.macro') }}</h2>
      <UAlert v-if="macroError" color="error" variant="subtle" :description="t('common.loadError', { message: macroError.statusMessage ?? macroError.message })" />
      <MacroStrip v-else-if="macro" :strip="macro" />
    </section>

    <div class="grid lg:grid-cols-3 gap-6">
      <UCard>
        <template #header>
          <h2 class="font-semibold">{{ t('home.outlook', { profile: t(`profile.${profile}`) }) }}</h2>
          <p class="text-xs text-muted">{{ t('home.outlookHint') }}</p>
        </template>
        <UAlert v-if="outlookError" color="error" variant="subtle" :description="t('common.loadError', { message: outlookError.statusMessage ?? outlookError.message })" />
        <OutlookBands v-else-if="outlook" :outlook="outlook" />
      </UCard>

      <UCard v-for="top in tops" :key="top.market">
        <template #header>
          <div class="flex items-center justify-between">
            <h2 class="font-semibold">{{ t('home.top', { market: t(`market.${top.market}`) }) }}</h2>
            <UButton :to="{ path: '/explore', query: { market: top.market } }" variant="link" size="sm">{{ t('home.seeAll') }}</UButton>
          </div>
        </template>
        <RankingTable :items="top.data?.items ?? []" compact />
      </UCard>
    </div>
  </div>
</template>
