const csrfCookieName = 'nivek_csrf'
const csrfHeaderName = 'X-CSRF-Token'
const safeMethods = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE'])

function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null
  const prefix = `${encodeURIComponent(name)}=`
  for (const part of document.cookie.split(';')) {
    const cookie = part.trim()
    if (cookie.startsWith(prefix)) {
      return decodeURIComponent(cookie.slice(prefix.length))
    }
  }
  return null
}

// Same-origin API client. The signed session is an HttpOnly cookie; JavaScript
// never receives or stores it. Unsafe cookie-authenticated requests carry a
// double-submit CSRF token from the readable CSRF cookie.
export const api = $fetch.create({
  baseURL: '/api',
  credentials: 'include',
  retry: 0,
  onRequest({ options }) {
    options.credentials = 'include'
    const method = String(options.method || 'GET').toUpperCase()
    if (safeMethods.has(method) || typeof window === 'undefined') return

    const csrf = readCookie(csrfCookieName)
    if (!csrf) return

    const headers = new Headers(options.headers as HeadersInit | undefined)
    headers.set(csrfHeaderName, csrf)
    options.headers = headers
  },
  onResponseError({ response }) {
    if (typeof window === 'undefined') return
    if (response.status === 401) {
      window.dispatchEvent(new CustomEvent('auth:unauthorized'))
    }
  },
})
