export default defineEventHandler((event) => {
  const id = encodeURIComponent(getRouterParam(event, 'id') ?? '')
  const { profile } = getQuery(event)
  return backendFetch<WatchlistEntries>(event, `/api/v1/watchlists/${id}`, { query: { profile: profile?.toString() } })
})
