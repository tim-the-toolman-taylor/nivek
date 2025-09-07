import { createMemoryHistory, createRouter, RouteRecordRaw } from 'vue-router'

import Welcome from '@/pages/Welcome/Welcome.vue'
import LoginPage from '@/pages/Login/Login.vue'
import SignupPage from '@/pages/Signup/Signup.vue'
import DashboardPage from '@/pages/Dashboard/Dashboard.vue'

import { useAuthStore } from '@/stores/auth'

const routes: Array<RouteRecordRaw> = [
    { path: '/', component: Welcome },
    { path: '/login', component: LoginPage },
    { path: '/signup', component: SignupPage },
    {
        path: '/dashboard',
        component: DashboardPage,
        meta: { requiresAuth: true, roles: ['user', 'admin'] }
    }
]

const router = createRouter({
    history: createMemoryHistory(),
    routes,
})

router.beforeEach((to, from, next) => {
    const auth = useAuthStore()

    if (to.meta.requiresAuth && !auth.isAuthenticated) {
        // not logged in → redirect to login
        next('/login')
    } else if (to.meta.roles && !to.meta.roles.includes(auth.userRole)) {
        // logged in but not enough permission → redirect or show error
        next('/')
    } else {
        next() // allow access
    }
})

export default router
