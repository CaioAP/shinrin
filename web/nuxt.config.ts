// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  runtimeConfig: {
    public: {
      // Go API base URL, override with NUXT_PUBLIC_API_BASE
      apiBase: 'http://localhost:8080',
    },
  },
})
