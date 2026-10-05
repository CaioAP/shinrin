<script setup lang="ts">
const props = defineProps<{ strip: MacroStrip }>()
const { t, locale } = useI18n()

function display(m: MacroIndicator): string {
  return m.unit === 'brl_per_usd' ? formatMoney(m.value, 'BRL', locale.value) : formatPercent(m.value, locale.value)
}

// Present and missing indicators in one row, so a gap is visible.
const order: MacroCode[] = ['selic', 'cdi', 'ipca_12m', 'usdbrl', 'fed_funds', 'ust_10y', 'us_cpi_12m']
const cells = computed(() => order
  .filter(code => props.strip.items.some(i => i.code === code) || props.strip.missing.includes(code))
  .map(code => ({ code, item: props.strip.items.find(i => i.code === code) })))
</script>

<template>
  <dl class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-3">
    <div v-for="c in cells" :key="c.code" class="rounded-md border border-default p-3">
      <dt class="text-xs text-muted">{{ t(`macro.${c.code}`) }}</dt>
      <template v-if="c.item">
        <dd class="text-lg font-semibold tabular-nums">{{ display(c.item) }}</dd>
        <dd class="text-xs text-dimmed" :title="t('common.source', { source: c.item.source })">
          {{ formatDate(c.item.asOf, locale) }}
        </dd>
      </template>
      <dd v-else class="text-sm text-dimmed">{{ t('common.missing') }}</dd>
    </div>
  </dl>
</template>
