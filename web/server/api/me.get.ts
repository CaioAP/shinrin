// Returns null instead of 401 when signed out, so pages can render either way.
export default defineEventHandler(async (event) => {
  if (!getSessionToken(event)) return null
  try {
    return await backendFetch<User>(event, '/api/v1/me')
  }
  catch (err) {
    if ((err as { statusCode?: number }).statusCode === 401) return null
    throw err
  }
})
