<script setup lang="ts">
defineProps<{ assets: Asset[] }>()
</script>

<template>
  <table v-if="assets.length" class="assets">
    <thead>
      <tr>
        <th>Symbol</th>
        <th>Name</th>
        <th>Market</th>
        <th>Class</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="a in assets" :key="`${a.market}:${a.symbol}`">
        <td>
          <NuxtLink :to="`/assets/${a.market}/${a.symbol}`">{{ a.symbol }}</NuxtLink>
        </td>
        <td>{{ a.name }}</td>
        <td>{{ a.market }}</td>
        <td>{{ a.class }}</td>
      </tr>
    </tbody>
  </table>
  <p v-else class="empty">
    No assets yet. They appear once the data pipeline has run.
  </p>
</template>

<style scoped>
.assets {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  text-align: left;
  padding: 0.5rem;
  border-bottom: 1px solid #eee;
}
.empty {
  color: #666;
}
</style>
