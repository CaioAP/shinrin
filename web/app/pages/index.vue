<script setup lang="ts">
const prefs = usePreferencesStore()
const { market } = storeToRefs(prefs)

const { data, status, error } = await useAssets(() => ({ market: market.value }))

useHead({ title: 'Shinrin' })
</script>

<template>
  <section>
    <h1>Shinrin</h1>
    <p>
      Market, filings and news analysis for B3 and US investments, with optional
      AI reports using your own API key.
    </p>

    <h2>Assets</h2>
    <MarketFilter v-model="market" />
    <p v-if="error" role="alert">
      Could not load assets: {{ error.statusMessage ?? error.message }}
    </p>
    <p v-else-if="status === 'pending'">
      Loading…
    </p>
    <AssetTable v-else :assets="data.items" />
  </section>
</template>
