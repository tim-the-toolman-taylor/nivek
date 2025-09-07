
import 'bootstrap/dist/css/bootstrap.min.css';
import './assets/main.css'

import { createPinia } from 'pinia'
import { createApp } from 'vue'
import router from './router'
import App from './App.vue'

const pinia = createPinia()

createApp(App)
    .use(pinia)
    .use(router)
    .mount('#app')
