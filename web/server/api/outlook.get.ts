export default defineEventHandler((event) => {
  const { profile } = getQuery(event)
  return backendFetch<Outlook>(event, '/api/v1/outlook', { query: { profile: profile?.toString() } })
})
