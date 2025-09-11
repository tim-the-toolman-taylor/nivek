import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { createHttpClient } from '@/services/HttpClient'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import { AuthService } from '@/services/AuthService'
import { TokenManager } from '@/utils/TokenManager'
import { User } from '@/constants'

export const useAuthStore = defineStore('auth', () => {
    const user = ref<User | null>(null)
    const tokenManager = TokenManager.getInstance()

    // Services
    const httpClient = createHttpClient(AxiosAdapter)
    const authService = new AuthService(httpClient)

    // Computed Properties
    const token = computed(() => tokenManager.getToken())
    const isAuthenticated = computed(() => !!token.value)

    // Listen for unauthorized events
    if (typeof window !== 'undefined') {
        window.addEventListener('auth:unauthorized', () => {
            logout()
        })
    }

    const login = async (credentials: LoginCredentials) => {
        try {
            const result = await authService.login(credentials)
            user.value = result.user
            return {success: true, user: result.user}
        } catch (error) {
            return {success: false, error: error.message || 'Login failed'}
        }
    }

    const logout = () => {
        // authService.logout()
        // user.value = null
    }

    const initAuth = () => {
        // Check if user is already authenticated on app start
        if (isAuthenticated.value) {
            // Optionally fetch user profile
            fetchUserProfile()
        }
    }

    const fetchUserProfile = async () => {
        try {
            const userProfileResponse = await httpClient.post(`/profile`)
            user.value = userProfileResponse.data
            console.log(user.value)
        } catch (error) {
            console.error('Failed to fetch user profile:', error)
        }
    }

    return {
        user,
        token,
        isAuthenticated,
        login,
        logout,
        initAuth,
        fetchUserProfile
    }
})
