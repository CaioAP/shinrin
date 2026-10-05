export default defineEventHandler(async (event) => {
  const { email, password } = await readJSONBody(event)
  const s = await backendFetch<Session>(event, '/api/v1/auth/signin', { method: 'POST', body: { email, password } })
  setSessionToken(event, s.token, s.expiresAt)
  return s.user
})
