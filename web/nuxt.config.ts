// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: ['@nuxt/ui', '@nuxtjs/i18n', '@pinia/nuxt', '@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  // System fonts: no build-time download, nothing fetched from font CDNs.
  ui: { fonts: false },
  // Icons are bundled from @iconify-json/lucide instead of fetched at runtime.
  icon: { serverBundle: { collections: ['lucide'] }, clientBundle: { scan: true } },
  i18n: {
    defaultLocale: 'en',
    strategy: 'no_prefix',
    locales: [
      { code: 'en', language: 'en-US', name: 'English', file: 'en.json' },
      { code: 'pt-BR', language: 'pt-BR', name: 'Português', file: 'pt-BR.json' },
    ],
    detectBrowserLanguage: { useCookie: true, cookieKey: 'shinrin_locale', fallbackLocale: 'en' },
  },
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
