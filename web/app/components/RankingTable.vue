<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'

const props = defineProps<{ items: RankedAsset[], compact?: boolean, loading?: boolean }>()
const { t, locale } = useI18n()

const factors: Factor[] = ['valuation', 'quality', 'growth', 'momentum', 'income', 'risk', 'sentiment']

const columns = computed<TableColumn<RankedAsset>[]>(() => {
  const cols: TableColumn<RankedAsset>[] = [
    { id: 'asset', header: t('explore.symbol') },
    { id: 'composite', header: t('explore.composite') },
  ]
  if (!props.compact) {
    for (const f of factors) cols.push({ id: f, header: t(`factor.${f}`) })
    cols.push({ id: 'coverage', header: t('explore.coverage') }, { id: 'asOf', header: t('explore.asOf') })
  }
  return cols
})
</script>

<template>
  <UTable :data="items" :columns="columns" :loading="loading" :empty="t('explore.empty')" class="w-full">
    <template #asset-cell="{ row }">
      <NuxtLink :to="`/assets/${row.original.asset.market}/${row.original.asset.symbol}`" class="font-medium text-primary hover:underline">
        {{ row.original.asset.symbol }}
      </NuxtLink>
      <div class="text-xs text-muted truncate max-w-48">{{ row.original.asset.name }}</div>
    </template>
    <template #composite-cell="{ row }">
      <ScoreBar :value="row.original.composite" class="w-32" />
    </template>
    <template v-for="f in factors" :key="f" #[`${f}-cell`]="{ row }">
      <UBadge
        v-if="row.original.factors[f] !== undefined"
        :color="scoreColor(row.original.factors[f])"
        variant="subtle"
        class="tabular-nums"
      >
        {{ Math.round(row.original.factors[f]!) }}
      </UBadge>
      <span v-else class="text-dimmed">—</span>
    </template>
    <template #coverage-cell="{ row }">
      <span class="tabular-nums">{{ formatFraction(row.original.coverage, locale, 0) }}</span>
    </template>
    <template #asOf-cell="{ row }">
      <span class="text-muted">{{ formatDate(row.original.asOf, locale) }}</span>
    </template>
  </UTable>
</template>
