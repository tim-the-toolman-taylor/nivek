import { defineStore } from 'pinia'
import { AxiosAdapter } from '@/services/AxiosAdapter'
import { User } from '@/constants'
import { ref } from 'vue'
import { createHttpClient } from '@/services/HttpClient'
import { AuthService } from '@/services/authService'

export const useAuthStore = defineStore('auth', () => {
    const token = ref<string | null>(localStorage.getItem('jwt_token'))
    const user = ref<User | null>(null)
    const isAuthenticated = ref(!!token.value)

    const http = createHttpClient(AxiosAdapter)
    const authService = new AuthService(http)

    const login = async (credentials: LoginCredentials) => {
        try {
            const result = await authService.login(credentials)

            token.value = result.token
            user.value = result.user
            isAuthenticated.value = true

            localStorage.setItem('jwt_token', result.token)

            return { success: true, user: result.user }
        } catch (error) {
            return { success: false, error: error.message }
        }
    }

    const logout = () => {
        token.value = null
        user.value = null
        isAuthenticated.value = false
        localStorage.removeItem('jwt_token')
    }

    const initAuth = () => {
        if (token.value) {
            // set token in headers in axiosAdpater
        }
    }

    return {
        // State
        token,
        user,
        isAuthenticated,
        // Actions
        login,
        logout,
        initAuth
    }
});

// export const useAuthStore = defineStore('auth', {
//     state: () => ({
//         user: null as User | null,
//     }),
//     getters: {
//         isAuthenticated: (state) => !!state.user,
//         userRole: (state) => state.user?.role ?? null,
//     },
//     actions: {
//         login(user: User) {
//             this.user = user;
//         },
//         logout() {
//             this.user = null;
//         },
//     },
// });
