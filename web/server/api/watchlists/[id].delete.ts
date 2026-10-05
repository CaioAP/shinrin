export default defineEventHandler(async (event) => {
  const id = encodeURIComponent(getRouterParam(event, 'id') ?? '')
  await backendFetch(event, `/api/v1/watchlists/${id}`, { method: 'DELETE' })
  setResponseStatus(event, 204)
  return null
})
