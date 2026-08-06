import { defineStore } from 'pinia'
import { api } from '~/utils/api'
import type { User } from '~/utils/constants'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const initialized = ref(false)
  const loading = ref(false)

  const isAuthenticated = computed(() => user.value !== null)

  const onUnauthorized = () => {
    user.value = null
  }

  if (typeof window !== 'undefined') {
    window.addEventListener('auth:unauthorized', onUnauthorized)
    onScopeDispose(() => window.removeEventListener('auth:unauthorized', onUnauthorized))
  }

  async function fetchUserProfile(): Promise<User> {
    const profile = await api<User>('/profile', { method: 'GET' })
    user.value = profile
    return profile
  }

  async function initAuth(): Promise<void> {
    if (typeof window === 'undefined' || initialized.value) return
    loading.value = true
    try {
      await fetchUserProfile()
    } catch {
      user.value = null
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  async function logout(): Promise<void> {
    try {
      await api('/logout', { method: 'POST' })
    } finally {
      user.value = null
      initialized.value = true
    }
  }

  return {
    user,
    initialized,
    loading,
    isAuthenticated,
    fetchUserProfile,
    initAuth,
    logout,
  }
})
