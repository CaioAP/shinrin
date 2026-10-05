<script setup lang="ts">
import type { NuxtError } from '#app'

const props = defineProps<{ error: NuxtError }>()
const { t } = useI18n()

const title = computed(() => (props.error.statusCode === 404 ? t('error.notFound') : t('error.generic')))
</script>

<template>
  <UApp>
    <NuxtLayout>
      <section class="space-y-4">
        <h1 class="text-2xl font-bold">{{ title }}</h1>
        <p class="text-muted">{{ error.statusMessage }}</p>
        <UButton icon="i-lucide-arrow-left" @click="clearError({ redirect: '/' })">{{ t('error.home') }}</UButton>
      </section>
    </NuxtLayout>
  </UApp>
</template>
