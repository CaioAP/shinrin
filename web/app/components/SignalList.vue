<script setup lang="ts">
// Signals carry an English message from Go; the code picks the translation.
defineProps<{ signals: Signal[] }>()
const { t, te } = useI18n()

function text(s: Signal): string {
  const key = `signals.${s.code}`
  return te(key) ? t(key) : s.message
}
function icon(w: number): string {
  if (w > 0) return 'i-lucide-trending-up'
  if (w < 0) return 'i-lucide-trending-down'
  return 'i-lucide-minus'
}
function color(w: number): string {
  if (w > 0) return 'text-success'
  if (w < 0) return 'text-error'
  return 'text-muted'
}
</script>

<template>
  <ul v-if="signals.length" class="space-y-1.5">
    <li v-for="s in signals" :key="s.code" class="flex gap-2 text-sm">
      <UIcon :name="icon(s.weight)" :class="['size-4 shrink-0 mt-0.5', color(s.weight)]" />
      <span>{{ text(s) }}</span>
    </li>
  </ul>
  <p v-else class="text-sm text-muted">{{ t('view.noSignals') }}</p>
</template>
