<script setup lang="ts">
const { locale } = useI18n()
useHead({ htmlAttrs: { lang: locale } })

// Load the signed-in user once per page load, on the server when rendering.
const auth = useAuthStore()
await auth.ensureLoaded()
</script>

<template>
  <UApp>
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </UApp>
</template>
