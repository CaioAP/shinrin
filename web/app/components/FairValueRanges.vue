<script setup lang="ts">
// Each model's range on one shared scale, with the price as a marker, so
// "below most ranges" is visible at a glance.
const props = defineProps<{ fairValues: FairValue[], price: number, currency: string }>()
const { t, locale } = useI18n()

const scale = computed(() => {
  const values = props.fairValues.flatMap(f => [f.low, f.high]).concat(props.price > 0 ? [props.price] : [])
  const lo = Math.min(...values)
  const hi = Math.max(...values)
  const pad = (hi - lo) * 0.1 || hi * 0.1 || 1
  return { lo: Math.max(0, lo - pad), hi: hi + pad }
})
function pos(v: number): number {
  return ((v - scale.value.lo) / (scale.value.hi - scale.value.lo)) * 100
}
</script>

<template>
  <div v-if="fairValues.length" class="space-y-4">
    <div v-for="f in fairValues" :key="f.method">
      <div class="flex justify-between text-sm mb-1">
        <span class="font-medium">{{ t(`fairValue.${f.method}`) }}</span>
        <span class="tabular-nums">{{ formatMoney(f.low, currency, locale) }} – {{ formatMoney(f.high, currency, locale) }}</span>
      </div>
      <div class="relative h-3 rounded-full bg-elevated">
        <div class="absolute h-3 rounded-full bg-primary/60" :style="{ left: `${pos(f.low)}%`, width: `${Math.max(pos(f.high) - pos(f.low), 1)}%` }" />
        <div
          v-if="price > 0"
          class="absolute -top-1 h-5 w-0.5 bg-highlighted"
          :style="{ left: `${pos(price)}%` }"
          :title="`${t('fairValue.price')}: ${formatMoney(price, currency, locale)}`"
        />
      </div>
    </div>
    <p class="text-xs text-muted">
      {{ t('fairValue.price') }}: <span class="tabular-nums">{{ formatMoney(price, currency, locale) }}</span>
    </p>
  </div>
  <p v-else class="text-sm text-muted">{{ t('fairValue.none') }}</p>
</template>
