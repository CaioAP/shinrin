<script setup lang="ts">
const { data: meta } = await useMeta()
</script>

<template>
  <div class="layout">
    <header class="header">
      <NuxtLink to="/" class="brand">Shinrin</NuxtLink>
      <small v-if="meta" class="version">v{{ meta.version }}</small>
    </header>
    <main>
      <slot />
    </main>
    <DisclaimerFooter />
  </div>
</template>

<style scoped>
.layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.header {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #ddd;
}
.version {
  margin-left: 0.5rem;
  color: #888;
}
.brand {
  font-weight: 600;
  color: inherit;
  text-decoration: none;
}
main {
  flex: 1;
  padding: 2rem 1rem;
  max-width: 960px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}
</style>
