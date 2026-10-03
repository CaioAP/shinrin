<script setup lang="ts">
defineProps<{ outlook: Outlook }>()
const { t } = useI18n()
</script>

<template>
  <div class="space-y-4">
    <div v-for="b in outlook.bands" :key="b.class">
      <div class="flex justify-between text-sm mb-1">
        <span class="font-medium">{{ t(`band.${b.class}`) }}</span>
        <span class="tabular-nums">{{ b.min }}–{{ b.max }}% <span class="text-muted">({{ t(`band.lean.${b.lean}`) }})</span></span>
      </div>
      <div class="relative h-2 rounded-full bg-elevated" role="img" :aria-label="`${b.min}–${b.max}%`">
        <div class="absolute h-2 rounded-full bg-primary" :style="{ left: `${b.min}%`, width: `${b.max - b.min}%` }" />
      </div>
    </div>
    <SignalList :signals="outlook.signals" />
    <ul v-if="outlook.notes.length" class="text-xs text-muted list-disc ps-4">
      <li v-for="n in outlook.notes" :key="n">{{ n }}</li>
    </ul>
  </div>
</template>
