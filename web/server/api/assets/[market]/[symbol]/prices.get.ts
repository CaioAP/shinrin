export default defineEventHandler((event) => {
  const market = encodeURIComponent(getRouterParam(event, 'market') ?? '')
  const symbol = encodeURIComponent(getRouterParam(event, 'symbol') ?? '')
  const { range } = getQuery(event)
  return backendFetch<PriceHistory>(event, `/api/v1/assets/${market}/${symbol}/prices`, {
    query: { range: range?.toString() },
  })
})
