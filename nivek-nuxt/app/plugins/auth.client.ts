export default defineNuxtPlugin(async () => {
  const auth = useAuthStore()
  await auth.initAuth()
})
