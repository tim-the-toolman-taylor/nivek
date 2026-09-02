// Route guard for /overlay. plugins/auth.client.ts awaits initAuth() before the
// first route resolves, so `initialized` is true by the time this runs; the check
// is kept anyway so a not-yet-loaded profile never reads as "no access" and
// bounces a legitimate visit.
//
// This is a UX gate, not a security boundary. The device endpoints behind it are
// `authenticated` and always act on the signed-in account, and the build download
// is separately allowlisted server-side, so forcing this page grants nothing.
export default defineNuxtRouteMiddleware(() => {
  if (import.meta.server) return

  const auth = useAuthStore()
  if (!auth.initialized) return

  if (auth.user?.overlay_access !== true) {
    return navigateTo('/', { replace: true })
  }
})
