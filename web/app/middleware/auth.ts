// Pages with definePageMeta({ middleware: 'auth' }) need a signed-in user.
export default defineNuxtRouteMiddleware(async (to) => {
  const auth = useAuthStore()
  await auth.ensureLoaded()
  if (!auth.signedIn) {
    return navigateTo({ path: '/signin', query: { next: to.fullPath } })
  }
})
