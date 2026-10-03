<script setup lang="ts">
// A 0-100 score with its bar; an absent score says so instead of showing 0.
const props = defineProps<{ value?: number, label?: string }>()
const { t } = useI18n()
const color = computed(() => scoreColor(props.value))
</script>

<template>
  <div class="flex items-center gap-2 min-w-24">
    <span v-if="label" class="w-24 shrink-0 text-sm">{{ label }}</span>
    <template v-if="value !== undefined">
      <UProgress :model-value="value" :max="100" :color="color" size="sm" class="flex-1" />
      <span class="w-8 text-end tabular-nums text-sm font-medium">{{ Math.round(value) }}</span>
    </template>
    <span v-else class="text-sm text-dimmed">{{ t('factor.missing') }}</span>
  </div>
</template>
