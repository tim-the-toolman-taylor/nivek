import { defineStore } from 'pinia'
import { User } from '@/constants'

export const useAuthStore = defineStore('auth', {
    state: () => ({
        user: null as User | null,
    }),
    getters: {
        isAuthenticated: (state) => !!state.user,
        userRole: (state) => state.user?.role ?? null,
    },
    actions: {
        login(user: User) {
            this.user = user;
        },
        logout() {
            this.user = null;
        },
    },
});
