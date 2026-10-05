export default defineEventHandler(async (event) => {
  const { answers } = await readJSONBody(event)
  return backendFetch<User>(event, '/api/v1/me/risk-profile', { method: 'PUT', body: { answers } })
})
