<script setup lang="ts">
const { t } = useI18n()
const prefs = usePreferencesStore()
const { profile } = storeToRefs(prefs)

const items = computed(() => (['conservative', 'moderate', 'aggressive'] as const).map(value => ({
  label: t(`profile.${value}`),
  value,
})))
</script>

<template>
  <USelect
    v-model="profile"
    :items="items"
    :aria-label="t('profile.label')"
    icon="i-lucide-gauge"
    variant="ghost"
    class="w-40"
  />
</template>
