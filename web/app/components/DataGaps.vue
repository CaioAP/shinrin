<script setup lang="ts">
// The engine's notes on what it could not compute. Shown, never hidden.
defineProps<{ notes: string[], missingFactors: Factor[] }>()
const { t } = useI18n()
</script>

<template>
  <UAlert
    v-if="notes.length || missingFactors.length"
    color="info"
    variant="subtle"
    icon="i-lucide-info"
    :title="t('asset.dataGaps')"
  >
    <template #description>
      <p class="mb-1">{{ t('asset.dataGapsHint') }}</p>
      <ul class="list-disc ps-4">
        <li v-if="missingFactors.length">
          {{ t('factor.missing') }}: {{ missingFactors.map(f => t(`factor.${f}`)).join(', ') }}
        </li>
        <li v-for="n in notes" :key="n">{{ n }}</li>
      </ul>
    </template>
  </UAlert>
</template>
