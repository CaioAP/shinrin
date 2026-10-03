<script setup lang="ts">
defineProps<{ items: Dividend[], currency: string }>()
const { t, locale } = useI18n()
</script>

<template>
  <table v-if="items.length" class="w-full text-sm">
    <thead class="text-xs text-muted">
      <tr>
        <th class="text-start font-normal">{{ t('asset.exDate') }}</th>
        <th class="text-start font-normal">{{ t('asset.type') }}</th>
        <th class="text-end font-normal">{{ t('asset.value') }}</th>
      </tr>
    </thead>
    <tbody class="tabular-nums">
      <tr v-for="d in items" :key="`${d.exDate}-${d.type}`" class="border-t border-default">
        <td class="py-1">{{ formatDate(d.exDate, locale) }}</td>
        <td>{{ t(`action.${d.type}`) }}</td>
        <td class="text-end">{{ formatMoney(d.value, currency, locale) }}</td>
      </tr>
    </tbody>
  </table>
  <p v-else class="text-sm text-muted">{{ t('asset.noDividends') }}</p>
</template>
