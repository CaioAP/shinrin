<script setup lang="ts">
const props = defineProps<{ indicators: Record<string, number>, currency: string }>()
const { t, locale } = useI18n()

// Only indicators that exist: a missing key means it could not be computed.
const groups = computed(() => indicatorGroups
  .map(g => g.filter(name => props.indicators[name] !== undefined))
  .filter(g => g.length))
</script>

<template>
  <div v-if="groups.length" class="grid sm:grid-cols-2 lg:grid-cols-3 gap-x-8 gap-y-4">
    <dl v-for="(g, i) in groups" :key="i" class="space-y-1 text-sm">
      <div v-for="name in g" :key="name" class="flex justify-between gap-4">
        <dt class="text-muted">{{ t(`metric.${name}`) }}</dt>
        <dd class="tabular-nums font-medium">{{ formatMetric(name, indicators[name], currency, locale) }}</dd>
      </div>
    </dl>
  </div>
  <p v-else class="text-sm text-muted">{{ t('common.noData') }}</p>
</template>
