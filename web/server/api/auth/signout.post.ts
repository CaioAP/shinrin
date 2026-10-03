export default defineEventHandler(async (event) => {
  if (getSessionToken(event)) {
    await backendFetch(event, '/api/v1/auth/signout', { method: 'POST' }).catch(() => undefined)
  }
  clearSessionToken(event)
  setResponseStatus(event, 204)
  return null
})
