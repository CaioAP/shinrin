/**
 * Client-side preferences shared across pages. Server data (assets, scores)
 * is never copied into a store: it stays in useFetch's cache via composables.
 * The risk profile lives in a cookie so server-rendered pages use it too;
 * once accounts exist it comes from the user's saved profile instead.
 */
export const usePreferencesStore = defineStore('preferences', () => {
  const market = ref<Market | undefined>(undefined)
  const profile = useCookie<RiskProfile>('shinrin_profile', {
    default: () => 'moderate',
    sameSite: 'lax',
    maxAge: 60 * 60 * 24 * 365,
  })

  function setMarket(value: Market | undefined) {
    market.value = value
  }

  function setProfile(value: RiskProfile) {
    profile.value = value
  }

  return { market, profile, setMarket, setProfile }
})
