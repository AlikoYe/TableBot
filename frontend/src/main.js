import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { initTelegram } from './composables/useTelegram'
import './styles/theme.css'

initTelegram()

createApp(App).use(createPinia()).use(router).mount('#app')
