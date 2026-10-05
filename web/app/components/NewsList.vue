<script setup lang="ts">
defineProps<{ items: NewsItem[] }>()
const { t, locale } = useI18n()
</script>

<template>
  <ul v-if="items.length" class="divide-y divide-default">
    <li v-for="n in items" :key="n.url" class="py-2">
      <a :href="n.url" target="_blank" rel="noopener noreferrer" class="font-medium hover:underline">{{ n.title }}</a>
      <p v-if="n.summary" class="text-sm text-muted line-clamp-2">{{ n.summary }}</p>
      <p class="text-xs text-dimmed">{{ formatDate(n.publishedAt, locale) }} · {{ n.source }}</p>
    </li>
  </ul>
  <p v-else class="text-sm text-muted">{{ t('asset.noNews') }}</p>
</template>
