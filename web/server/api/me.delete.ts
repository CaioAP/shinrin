export default defineEventHandler(async (event) => {
  const { password } = await readJSONBody(event)
  await backendFetch(event, '/api/v1/me', { method: 'DELETE', body: { password } })
  clearSessionToken(event)
  setResponseStatus(event, 204)
  return null
})
