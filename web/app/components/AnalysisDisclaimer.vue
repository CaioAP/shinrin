<script setup lang="ts">
// Shown at the top of every view that carries a score, fair value or timing
// view (docs/conventions.md: the disclaimer is on every analysis view).
defineProps<{ asOf?: string }>()
const { locale } = useI18n()
</script>

<template>
  <UAlert
    color="warning"
    variant="subtle"
    icon="i-lucide-triangle-alert"
    :title="$t('disclaimer.title')"
    :description="$t('disclaimer.analysis') + (asOf ? ' ' + $t('common.asOf', { date: formatDate(asOf, locale) }) + '.' : '')"
  />
</template>
