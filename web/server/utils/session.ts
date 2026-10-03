import type { H3Event } from 'h3'

// The session token lives in an HttpOnly cookie that only the Nuxt server
// reads; it is forwarded to Go as a bearer token by backendFetch.
const cookieName = 'shinrin_session'

export function getSessionToken(event: H3Event): string | undefined {
  return getCookie(event, cookieName)
}

export function setSessionToken(event: H3Event, token: string, expiresAt: string) {
  setCookie(event, cookieName, token, {
    httpOnly: true,
    secure: !import.meta.dev,
    sameSite: 'lax',
    path: '/',
    expires: new Date(expiresAt),
  })
}

export function clearSessionToken(event: H3Event) {
  deleteCookie(event, cookieName, { path: '/' })
}
