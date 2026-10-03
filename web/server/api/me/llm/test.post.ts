export default defineEventHandler(async (event) => {
  await backendFetch(event, '/api/v1/me/llm/test', { method: 'POST' })
  setResponseStatus(event, 204)
  return null
})
