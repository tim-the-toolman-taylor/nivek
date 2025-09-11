
import 'bootstrap/dist/css/bootstrap.min.css';
import './assets/main.css'

import { createPinia } from 'pinia'
import { createApp } from 'vue'
import router from './router'
import App from './App.vue'
import { useAuthStore } from '@/stores/auth'

const pinia = createPinia()

const app = createApp(App)
    .use(pinia)
    .use(router)

const authStore = useAuthStore()
authStore.initAuth()

app.mount('#app')
