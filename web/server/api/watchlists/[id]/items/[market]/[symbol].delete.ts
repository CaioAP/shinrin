export default defineEventHandler(async (event) => {
  const id = encodeURIComponent(getRouterParam(event, 'id') ?? '')
  const market = encodeURIComponent(getRouterParam(event, 'market') ?? '')
  const symbol = encodeURIComponent(getRouterParam(event, 'symbol') ?? '')
  await backendFetch(event, `/api/v1/watchlists/${id}/items/${market}/${symbol}`, { method: 'DELETE' })
  setResponseStatus(event, 204)
  return null
})
