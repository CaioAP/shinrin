export default defineEventHandler(async (event) => {
  const { name } = await readJSONBody(event)
  setResponseStatus(event, 201)
  return backendFetch<Watchlist>(event, '/api/v1/watchlists', { method: 'POST', body: { name } })
})
