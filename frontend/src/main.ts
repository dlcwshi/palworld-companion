import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { registerSW } from 'virtual:pwa-register'
import App from './App.vue'
import { router } from './router'
import './styles.css'

const updateIntervalMs = 60 * 60 * 1000
registerSW({
  immediate: true,
  onRegisteredSW(_workerUrl, registration) {
    if (!registration) return
    const checkForUpdate = () => {
      if (navigator.onLine) void registration.update().catch(() => undefined)
    }
    window.setInterval(checkForUpdate, updateIntervalMs)
    window.addEventListener('online', checkForUpdate)
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') checkForUpdate()
    })
  },
})
createApp(App).use(createPinia()).use(router).mount('#app')
