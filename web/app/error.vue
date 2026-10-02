<script setup lang="ts">
import type { NuxtError } from '#app'

const props = defineProps<{ error: NuxtError }>()

const title = computed(() => (props.error.statusCode === 404 ? 'Page not found' : 'Something went wrong'))
</script>

<template>
  <NuxtLayout>
    <section>
      <h1>{{ title }}</h1>
      <p>{{ error.statusMessage }}</p>
      <button type="button" @click="clearError({ redirect: '/' })">
        Back to Shinrin
      </button>
    </section>
  </NuxtLayout>
</template>
