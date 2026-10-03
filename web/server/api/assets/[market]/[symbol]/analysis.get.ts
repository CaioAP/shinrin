export default defineEventHandler((event) => {
  const market = encodeURIComponent(getRouterParam(event, 'market') ?? '')
  const symbol = encodeURIComponent(getRouterParam(event, 'symbol') ?? '')
  const { profile } = getQuery(event)
  return backendFetch<Analysis>(event, `/api/v1/assets/${market}/${symbol}/analysis`, {
    query: { profile: profile?.toString() },
  })
})
