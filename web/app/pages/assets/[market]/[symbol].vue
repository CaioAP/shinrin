<script setup lang="ts">
const route = useRoute()

const { data: asset, error } = await useAsset(
  () => String(route.params.market),
  () => String(route.params.symbol),
)

if (error.value) {
  throw createError({ statusCode: error.value.statusCode ?? 500, statusMessage: error.value.statusMessage, fatal: true })
}

useHead({ title: () => (asset.value ? `${asset.value.symbol} · Shinrin` : 'Shinrin') })
</script>

<template>
  <section v-if="asset">
    <NuxtLink to="/">← All assets</NuxtLink>
    <h1>{{ asset.symbol }}</h1>
    <p>{{ asset.name }}</p>
    <dl>
      <dt>Market</dt>
      <dd>{{ asset.market }} ({{ asset.currency }})</dd>
      <dt>Class</dt>
      <dd>{{ asset.class }}</dd>
      <template v-if="asset.sector">
        <dt>Sector</dt>
        <dd>{{ asset.sector }}</dd>
      </template>
      <dt>Index member</dt>
      <dd>{{ asset.indexMember ? 'Yes' : 'No' }}</dd>
    </dl>
  </section>
</template>
