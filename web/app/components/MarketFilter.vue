<script setup lang="ts">
const market = defineModel<Market | undefined>()
const { t } = useI18n()

// UTabs needs string values, so "all" stands in for undefined.
const value = computed({
  get: () => market.value ?? 'all',
  set: (v: string | number) => { market.value = v === 'all' ? undefined : v as Market },
})
const items = computed(() => [
  { label: t('common.all'), value: 'all' },
  { label: t('market.B3'), value: 'B3' },
  { label: t('market.US'), value: 'US' },
])
</script>

<template>
  <UTabs v-model="value" :items="items" :content="false" size="sm" :aria-label="t('market.label')" />
</template>
