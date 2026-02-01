import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import { getBasePath } from './api'

import Dashboard from './views/Dashboard.vue'
import Devices from './views/Devices.vue'
import DeviceDetail from './views/DeviceDetail.vue'
import Profiles from './views/Profiles.vue'
import Traps from './views/Traps.vue'
import Settings from './views/Settings.vue'

const routes = [
  { path: '/', name: 'dashboard', component: Dashboard },
  { path: '/devices', name: 'devices', component: Devices },
  { path: '/devices/:id', name: 'device-detail', component: DeviceDetail },
  { path: '/profiles', name: 'profiles', component: Profiles },
  { path: '/traps', name: 'traps', component: Traps },
  { path: '/settings', name: 'settings', component: Settings },
]

// Get base path for HA Ingress support
const basePath = getBasePath()

const router = createRouter({
  history: createWebHistory(basePath),
  routes,
})

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
