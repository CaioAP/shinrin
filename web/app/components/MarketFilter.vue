<script setup lang="ts">
const market = defineModel<Market | undefined>()

const options: { label: string, value: Market | undefined }[] = [
  { label: 'All', value: undefined },
  { label: 'B3', value: 'B3' },
  { label: 'US', value: 'US' },
]
</script>

<template>
  <div class="filter" role="group" aria-label="Market">
    <button
      v-for="opt in options"
      :key="opt.label"
      type="button"
      :aria-pressed="market === opt.value"
      @click="market = opt.value"
    >
      {{ opt.label }}
    </button>
  </div>
</template>

<style scoped>
.filter {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
button[aria-pressed='true'] {
  font-weight: 600;
}
</style>
