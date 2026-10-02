// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: ['@pinia/nuxt', '@nuxt/eslint'],
  runtimeConfig: {
    // Server-only. The browser talks to Nuxt's own /api routes, which call the
    // Go API from the server (see server/utils/backend.ts).
    // Override with NUXT_API_BASE.
    apiBase: 'http://localhost:8080',
  },
  typescript: {
    strict: true,
  },
})
