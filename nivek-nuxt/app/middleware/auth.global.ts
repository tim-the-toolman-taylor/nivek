export default defineNuxtRouteMiddleware((to) => {
  if (import.meta.server) return

  const auth = useAuthStore()
  if (to.meta.hideForAuth && auth.initialized && auth.isAuthenticated) {
    return navigateTo('/', { replace: true })
  }
})
