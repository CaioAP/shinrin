<script setup lang="ts">
// Every factor of the model, scored or not, each expandable into the metrics
// behind it ("why this rating?").
const props = defineProps<{ factors: FactorScore[], currency: string }>()
const { t, te, locale } = useI18n()

const all: Factor[] = ['valuation', 'quality', 'growth', 'momentum', 'income', 'risk', 'sentiment']
const rows = computed(() => all.map(f => ({ factor: f, score: props.factors.find(x => x.factor === f) })))

function metricLabel(m: string): string {
  return te(`metric.${m}`) ? t(`metric.${m}`) : m
}
</script>

<template>
  <div class="divide-y divide-default">
    <UCollapsible v-for="r in rows" :key="r.factor" :disabled="!r.score" class="py-2">
      <button type="button" class="w-full flex items-center gap-2 text-start group disabled:cursor-default">
        <ScoreBar :value="r.score?.value" :label="t(`factor.${r.factor}`)" class="flex-1" />
        <UIcon v-if="r.score" name="i-lucide-chevron-down" class="size-4 text-muted transition-transform group-data-[state=open]:rotate-180" />
        <span v-else class="size-4" />
      </button>
      <template #content>
        <div v-if="r.score" class="mt-2 ms-26 text-sm">
          <p class="text-xs text-muted mb-1">{{ r.score.peerGroup }}</p>
          <table class="w-full">
            <thead class="text-xs text-muted">
              <tr>
                <th class="text-start font-normal">{{ t('factor.metric') }}</th>
                <th class="text-end font-normal">{{ t('factor.value') }}</th>
                <th class="text-end font-normal">{{ t('factor.points') }}</th>
                <th class="text-end font-normal">{{ t('factor.weight') }}</th>
              </tr>
            </thead>
            <tbody class="tabular-nums">
              <tr v-for="i in r.score.inputs" :key="i.metric">
                <td>
                  {{ metricLabel(i.metric) }}
                  <span v-if="i.peers" class="text-xs text-dimmed">({{ t('factor.peers', { n: i.peers }) }})</span>
                </td>
                <td class="text-end">{{ formatMetric(i.metric, i.value, currency, locale) }}</td>
                <td class="text-end">{{ Math.round(i.points) }}</td>
                <td class="text-end">{{ formatNumber(i.weight, locale, 1) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </UCollapsible>
  </div>
</template>
