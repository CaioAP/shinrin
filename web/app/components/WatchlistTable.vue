<script setup lang="ts">
import { h } from 'vue'
import type { TableColumn } from '@nuxt/ui'

defineProps<{ items: WatchlistEntry[] }>()
const emit = defineEmits<{ remove: [asset: AssetKey] }>()
const { t, locale } = useI18n()

const factors: Factor[] = ['valuation', 'quality', 'growth', 'momentum', 'income', 'risk', 'sentiment']
const columns = computed<TableColumn<WatchlistEntry>[]>(() => [
  { id: 'asset', header: t('explore.symbol') },
  { id: 'composite', header: t('explore.composite') },
  ...factors.map(f => ({ id: f, header: t(`factor.${f}`) })),
  { id: 'asOf', header: t('explore.asOf') },
  // An empty string header renders differently on server and client.
  { id: 'actions', header: () => h('span', { class: 'sr-only' }, t('watchlists.remove')) },
])
</script>

<template>
  <UTable :data="items" :columns="columns" :empty="t('watchlists.emptyList')" class="w-full">
    <template #asset-cell="{ row }">
      <NuxtLink :to="`/assets/${row.original.asset.market}/${row.original.asset.symbol}`" class="font-medium text-primary hover:underline">
        {{ row.original.asset.symbol }}
      </NuxtLink>
      <div class="text-xs text-muted truncate max-w-48">{{ row.original.asset.name }}</div>
    </template>
    <template #composite-cell="{ row }">
      <ScoreBar v-if="row.original.scores" :value="row.original.scores.composite" class="w-32" />
      <span v-else class="text-sm text-dimmed">{{ t('watchlists.notScored') }}</span>
    </template>
    <template v-for="f in factors" :key="f" #[`${f}-cell`]="{ row }">
      <UBadge
        v-if="row.original.scores?.factors[f] !== undefined"
        :color="scoreColor(row.original.scores.factors[f])"
        variant="subtle"
        class="tabular-nums"
      >
        {{ Math.round(row.original.scores.factors[f]!) }}
      </UBadge>
      <span v-else class="text-dimmed">—</span>
    </template>
    <template #asOf-cell="{ row }">
      <span class="text-muted">{{ formatDate(row.original.scores?.asOf, locale) }}</span>
    </template>
    <template #actions-cell="{ row }">
      <UButton
        icon="i-lucide-x"
        color="neutral"
        variant="ghost"
        size="xs"
        :aria-label="t('watchlists.remove')"
        @click="emit('remove', { market: row.original.asset.market, symbol: row.original.asset.symbol })"
      />
    </template>
  </UTable>
</template>
