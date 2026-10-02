export default defineEventHandler((event) => {
  const market = encodeURIComponent(getRouterParam(event, 'market') ?? '')
  const symbol = encodeURIComponent(getRouterParam(event, 'symbol') ?? '')
  return backendFetch<Asset>(event, `/api/v1/assets/${market}/${symbol}`)
})
