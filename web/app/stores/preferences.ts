/**
 * Client-side preferences shared across pages. Server data (assets, scores)
 * is never copied into a store: it stays in useFetch's cache via composables.
 */
export const usePreferencesStore = defineStore('preferences', () => {
  const market = ref<Market | undefined>(undefined)

  function setMarket(value: Market | undefined) {
    market.value = value
  }

  return { market, setMarket }
})
