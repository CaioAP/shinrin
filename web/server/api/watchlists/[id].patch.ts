export default defineEventHandler(async (event) => {
  const id = encodeURIComponent(getRouterParam(event, 'id') ?? '')
  const { name } = await readJSONBody(event)
  await backendFetch(event, `/api/v1/watchlists/${id}`, { method: 'PATCH', body: { name } })
  setResponseStatus(event, 204)
  return null
})
