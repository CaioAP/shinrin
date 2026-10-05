export default defineEventHandler(async (event) => {
  const { email, password } = await readJSONBody(event)
  const s = await backendFetch<Session>(event, '/api/v1/auth/signup', { method: 'POST', body: { email, password } })
  setSessionToken(event, s.token, s.expiresAt)
  setResponseStatus(event, 201)
  // The token stays in the HttpOnly cookie; the page only needs the user.
  return s.user
})
