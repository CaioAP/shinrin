<template>
  <div class="layout">
    <NuxtRouteAnnouncer />
    <main>
      <NuxtPage />
    </main>
    <DisclaimerFooter />
  </div>
</template>

<style>
body {
  margin: 0;
  font-family: system-ui, sans-serif;
}
.layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
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
