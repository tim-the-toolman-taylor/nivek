// Route guard for /overlay. plugins/auth.client.ts awaits initAuth() before the
// first route resolves, so `initialized` is true by the time this runs; the
// check is kept anyway so a not-yet-loaded profile never reads as "not the
// owner" and bounces a legitimate visit.
//
// This is a UX gate, not a security boundary. The device endpoints behind it are
// `authenticated` and always act on the signed-in account, so someone who forced
// their way to this page could only ever mint a token for themselves.
export default defineNuxtRouteMiddleware(() => {
  if (import.meta.server) return

  const auth = useAuthStore()
  if (!auth.initialized) return

  if (!isOverlayOwnerLogin(auth.user?.twitch_login)) {
    return navigateTo('/', { replace: true })
  }
})
