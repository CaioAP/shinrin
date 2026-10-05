export default defineEventHandler(async (event) => {
  await backendFetch(event, '/api/v1/me/llm', { method: 'DELETE' })
  setResponseStatus(event, 204)
  return null
})
