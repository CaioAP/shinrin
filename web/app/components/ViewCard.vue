<script setup lang="ts">
const props = defineProps<{ kind: 'valuation' | 'timing', view: View<string> }>()
const { t } = useI18n()

const color = computed(() => {
  switch (props.view.view) {
    case 'cheap':
    case 'accumulate': return 'success'
    case 'expensive':
    case 'avoid': return 'error'
    default: return 'warning'
  }
})
</script>

<template>
  <UCard>
    <template #header>
      <div class="flex items-center justify-between">
        <h3 class="font-medium">{{ t(`view.${kind}`) }}</h3>
        <UBadge :color="color" size="lg">{{ t(`view.${view.view}`) }}</UBadge>
      </div>
    </template>
    <SignalList :signals="view.signals" />
  </UCard>
</template>
