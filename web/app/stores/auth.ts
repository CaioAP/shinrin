/**
 * The signed-in user (client state shared by every page). The session token
 * itself stays in an HttpOnly cookie on the Nuxt server; the browser only
 * ever sees the user.
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loaded = ref(false)
  const signedIn = computed(() => !!user.value)
  const prefs = usePreferencesStore()
  // useRequestFetch forwards the browser's cookies during server rendering.
  const fetcher = useRequestFetch()

  /** Loads the user from the session cookie (null when signed out). */
  async function refresh() {
    // A signed-out /api/me answers 204 with no body, which arrives as undefined.
    user.value = (await fetcher<User | null>('/api/me')) ?? null
    loaded.value = true
    if (user.value?.profile) prefs.setProfile(user.value.profile)
  }

  /** Loads the user unless already loaded (on the server, state carries over). */
  async function ensureLoaded() {
    if (!loaded.value) await refresh()
  }

  async function signIn(email: string, password: string) {
    user.value = await $fetch<User>('/api/auth/signin', { method: 'POST', body: { email, password } })
    if (user.value.profile) prefs.setProfile(user.value.profile)
  }

  async function signUp(email: string, password: string) {
    user.value = await $fetch<User>('/api/auth/signup', { method: 'POST', body: { email, password } })
  }

  async function signOut() {
    await $fetch('/api/auth/signout', { method: 'POST' })
    user.value = null
    clearNuxtData()
  }

  async function saveRiskProfile(answers: Record<string, string>) {
    user.value = await $fetch<User>('/api/me/risk-profile', { method: 'PUT', body: { answers } })
    if (user.value.profile) prefs.setProfile(user.value.profile)
  }

  async function deleteAccount(password: string) {
    await $fetch('/api/me', { method: 'DELETE', body: { password } })
    user.value = null
    clearNuxtData()
  }

  return { user, loaded, signedIn, refresh, ensureLoaded, signIn, signUp, signOut, saveRiskProfile, deleteAccount }
})
