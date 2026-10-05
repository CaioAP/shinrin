// CSRF protection for state-changing API calls: the request must come from
// this site's own pages. The session cookie is SameSite=Lax as well; this
// check covers what Lax lets through (same-site subdomains, old browsers).
export default defineEventHandler((event) => {
  const method = event.method
  if (!event.path.startsWith('/api/') || method === 'GET' || method === 'HEAD' || method === 'OPTIONS') return

  const host = getRequestHost(event, { xForwardedHost: true })
  const origin = getHeader(event, 'origin') ?? getHeader(event, 'referer')
  let originHost: string | undefined
  try {
    originHost = origin ? new URL(origin).host : undefined
  }
  catch {
    originHost = undefined
  }
  if (!originHost || originHost !== host) {
    throw createError({ statusCode: 403, statusMessage: 'cross-site request refused' })
  }
})
