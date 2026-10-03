<script setup lang="ts">
// One AI report. The disclaimer comes first; every claim is traceable to the
// cited data below, which comes from the report's stored input snapshot.
const props = defineProps<{ report: AIReport, currency: string }>()
const { t, te, locale } = useI18n()

const o = computed(() => props.report.output)
const viewColor = (v: string) => (['cheap', 'accumulate', 'good', 'high'].includes(v) ? 'success' : ['expensive', 'avoid', 'poor', 'low'].includes(v) ? 'error' : 'warning')
const label = (key: string) => (key.startsWith('news:') ? t('ai.headline') : te(`metric.${key}`) ? t(`metric.${key}`) : key)

// Omitted sections arrive as JSON paths ("bear_case[1]"); name them in words.
const sectionKeys: Record<string, string> = {
  summary: 'ai.summary', bull_case: 'ai.bull', bear_case: 'ai.bear', key_risks: 'ai.risks',
  valuation_view: 'view.valuation', timing_view: 'view.timing', fit_for_profile: 'ai.fit', suggested_allocation_pct: 'ai.allocationShort',
}
function sectionName(path: string) {
  const m = /^(\w+)(?:\[(\d+)\])?$/.exec(path)
  const key = m && sectionKeys[m[1]!]
  if (!m || !key) return path
  return m[2] === undefined ? t(key) : t('ai.item', { section: t(key), n: Number(m[2]) + 1 })
}
const omitted = computed(() => props.report.omitted.map(sectionName).join('; '))
const citedValue = (c: CitedData) => (c.value !== undefined ? formatMetric(c.key, c.value, props.currency, locale.value) : c.text ?? '')
</script>

<template>
  <article class="space-y-4">
    <UAlert color="warning" variant="subtle" icon="i-lucide-triangle-alert" :title="t('disclaimer.title')" :description="t('ai.disclaimer')" />

    <p class="text-xs text-muted">
      {{ t('ai.meta', { model: report.model, provider: report.provider, date: formatDate(report.createdAt, locale), asOf: formatDate(report.asOf, locale), profile: t(`profile.${report.profile}`) }) }}
    </p>

    <div class="flex flex-wrap gap-2">
      <UBadge :color="viewColor(o.valuationView)" variant="subtle">{{ t('view.valuation') }}: {{ t(`view.${o.valuationView}`) }}</UBadge>
      <UBadge :color="viewColor(o.timingView)" variant="subtle">{{ t('view.timing') }}: {{ t(`view.${o.timingView}`) }}</UBadge>
      <UBadge :color="viewColor(o.fitForProfile)" variant="subtle">{{ t('ai.fit') }}: {{ t(`ai.fitValue.${o.fitForProfile}`) }}</UBadge>
      <UBadge color="neutral" variant="outline">{{ t('ai.confidence') }}: {{ t(`ai.confidenceValue.${o.confidence}`) }}</UBadge>
    </div>

    <p v-if="o.summary" class="leading-relaxed">{{ o.summary }}</p>

    <div class="grid md:grid-cols-2 gap-4">
      <section>
        <h4 class="text-sm font-medium text-success mb-1">{{ t('ai.bull') }}</h4>
        <ul v-if="o.bullCase.length" class="list-disc ps-5 space-y-1 text-sm">
          <li v-for="(x, i) in o.bullCase" :key="i">{{ x }}</li>
        </ul>
        <p v-else class="text-sm text-muted">{{ t('ai.sectionEmpty') }}</p>
      </section>
      <section>
        <h4 class="text-sm font-medium text-error mb-1">{{ t('ai.bear') }}</h4>
        <ul v-if="o.bearCase.length" class="list-disc ps-5 space-y-1 text-sm">
          <li v-for="(x, i) in o.bearCase" :key="i">{{ x }}</li>
        </ul>
        <p v-else class="text-sm text-muted">{{ t('ai.sectionEmpty') }}</p>
      </section>
    </div>

    <section v-if="o.keyRisks.length">
      <h4 class="text-sm font-medium mb-1">{{ t('ai.risks') }}</h4>
      <ul class="list-disc ps-5 space-y-1 text-sm">
        <li v-for="(x, i) in o.keyRisks" :key="i">{{ x }}</li>
      </ul>
    </section>

    <p class="text-sm">
      {{ t('ai.allocation', { min: formatNumber(o.suggestedAllocationPct.min, locale, 1), max: formatNumber(o.suggestedAllocationPct.max, locale, 1), profile: t(`profile.${report.profile}`) }) }}
    </p>

    <UAlert
      v-if="report.omitted.length"
      color="neutral"
      variant="subtle"
      icon="i-lucide-scissors"
      :description="t('ai.omitted', { sections: omitted })"
    />

    <section v-if="report.cited.length">
      <h4 class="text-xs font-medium text-muted mb-1">{{ t('ai.cited') }}</h4>
      <div class="flex flex-wrap gap-1.5">
        <UBadge v-for="c in report.cited" :key="c.key" color="neutral" variant="soft" class="max-w-full truncate">
          {{ label(c.key) }}<template v-if="citedValue(c)">: {{ citedValue(c) }}</template>
        </UBadge>
      </div>
    </section>
  </article>
</template>
