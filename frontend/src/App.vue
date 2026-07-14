<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useDeviceStore } from './stores/devices'
import { useUIStore } from './stores/ui'
import { getBasePath } from './api'

const deviceStore = useDeviceStore()
const uiStore = useUIStore()
const route = useRoute()
const menuOpen = ref(false)
const darkMode = ref(false)
const navItems = [
  { to: '/', label: 'Dashboard', short: 'DB' },
  { to: '/devices', label: 'Devices', short: 'DV' },
  { to: '/profiles', label: 'Profiles', short: 'PR' },
  { to: '/traps', label: 'Trap Logs', short: 'TL' },
  { to: '/settings', label: 'Settings', short: 'ST' },
]

const pageTitle = computed(() => {
  if (route.path.startsWith('/devices')) return 'Devices'
  if (route.path.startsWith('/profiles')) return 'Profiles'
  if (route.path.startsWith('/traps')) return 'Trap Logs'
  if (route.path.startsWith('/settings')) return 'Settings'
  return 'Dashboard'
})

const statusSummary = computed(() => {
  const total = deviceStore.devices.length
  if (!total) return 'No devices configured'
  return `${deviceStore.onlineCount}/${total} devices online`
})

// Initialize dark mode from localStorage
function initDarkMode() {
  const saved = localStorage.getItem('darkMode')
  if (saved !== null) {
    darkMode.value = saved === 'true'
  } else {
    darkMode.value = window.matchMedia('(prefers-color-scheme: dark)').matches
  }
  applyDarkMode()
}

function toggleDarkMode() {
  darkMode.value = !darkMode.value
  localStorage.setItem('darkMode', darkMode.value)
  applyDarkMode()
}

function applyDarkMode() {
  if (darkMode.value) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

let ws = null
let wsReconnectTimer = null
let mqttStatusTimer = null
let shuttingDown = false

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const basePath = getBasePath()
  const wsUrl = `${protocol}//${window.location.host}${basePath}/api/ws`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('WebSocket connected')
  }

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    if (data.type === 'state_update') {
      // Map state_update event fields to match DeviceState structure
      const eventData = data.data
      const currentState = deviceStore.getDeviceState(eventData.device_id) || {}
      const updatedState = {
        ...currentState,
        device_id: eventData.device_id,
        online: eventData.online,
        last_poll: eventData.timestamp, // Map timestamp to last_poll
        values: { ...currentState.values, ...eventData.values },
        errors: eventData.errors || [],
      }
      deviceStore.updateDeviceState(eventData.device_id, updatedState)
    } else if (data.type === 'initial_state') {
      deviceStore.setAllStates(data.data)
    }
  }

  ws.onclose = () => {
    if (!shuttingDown) {
      console.log('WebSocket disconnected, reconnecting...')
      wsReconnectTimer = setTimeout(connectWebSocket, 3000)
    }
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
  }
}

onMounted(() => {
  initDarkMode()
  uiStore.initialize()
  deviceStore.fetchDevices()
  connectWebSocket()
  uiStore.refreshMQTTStatus()
  mqttStatusTimer = setInterval(() => uiStore.refreshMQTTStatus(), 10000)
})

onUnmounted(() => {
  shuttingDown = true
  clearTimeout(wsReconnectTimer)
  clearInterval(mqttStatusTimer)
  if (ws) {
    ws.close()
  }
})
</script>

<template>
  <div class="app-shell">
    <aside class="app-rail" aria-label="Navigation">
      <RouterLink to="/" class="app-brand" title="Dashboard">
        <span class="app-brand-mark">B</span>
        <strong>SNMP Bridge</strong>
      </RouterLink>

      <nav class="app-nav">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="app-nav-item"
          active-class="active"
        >
          <span>{{ item.short }}</span>
          <strong>{{ item.label }}</strong>
        </RouterLink>
      </nav>
    </aside>

    <div class="app-workspace">
      <header class="app-topbar">
        <div class="min-w-0">
          <h1>{{ pageTitle }}</h1>
          <p>{{ statusSummary }}</p>
        </div>

        <div class="app-top-actions">
          <span
            class="app-health-pill mqtt"
            :class="uiStore.mqttStatus.loading ? '' : (uiStore.mqttStatus.connected ? 'ok' : 'bad')"
            :title="uiStore.mqttStatus.broker ? `${uiStore.mqttStatus.broker}:${uiStore.mqttStatus.port}` : 'MQTT broker unavailable'"
          >
            <span class="app-health-dot"></span>
            MQTT {{ uiStore.mqttStatus.loading ? 'checking' : (uiStore.mqttStatus.connected ? 'connected' : 'offline') }}
          </span>

          <button
            type="button"
            class="safe-mode-toggle"
            :class="{ active: uiStore.safeMode }"
            :aria-pressed="uiStore.safeMode"
            title="Require confirmation before power switching"
            @click="uiStore.toggleSafeMode"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 3 5.5 5.7v5.8c0 4.1 2.7 7.8 6.5 9.2 3.8-1.4 6.5-5.1 6.5-9.2V5.7L12 3Zm0 2.2 4.5 1.9v4.4c0 3-1.8 5.8-4.5 7-2.7-1.2-4.5-4-4.5-7V7.1L12 5.2Zm-1 3v4.2l3.4 2 .9-1.5-2.5-1.5V8.2H11Z" />
            </svg>
            Safe mode
            <span class="safe-mode-switch"><span></span></span>
          </button>

          <span class="app-health-pill" :class="deviceStore.offlineCount === 0 ? 'ok' : 'bad'">
            <span class="app-health-dot"></span>
            {{ deviceStore.offlineCount === 0 ? 'Healthy' : `${deviceStore.offlineCount} offline` }}
          </span>

          <button
            @click="toggleDarkMode"
            class="app-icon-button"
            :title="darkMode ? 'Switch to light mode' : 'Switch to dark mode'"
          >
            <svg v-if="darkMode" class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" />
            </svg>
            <svg v-else class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
              <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
            </svg>
          </button>

          <button @click="menuOpen = !menuOpen" class="app-menu-button">
            <span></span>
          </button>
        </div>
      </header>

      <nav v-if="menuOpen" class="app-mobile-nav" aria-label="Mobile navigation">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          active-class="active"
          @click="menuOpen = false"
        >
          {{ item.label }}
        </RouterLink>
      </nav>

      <main class="app-content">
        <RouterView />
      </main>
    </div>

    <div
      v-if="uiStore.confirmation"
      class="confirm-backdrop"
      role="presentation"
      @click.self="uiStore.resolveConfirmation(false)"
    >
      <section class="confirm-dialog" role="alertdialog" aria-modal="true" :aria-labelledby="'confirm-title'">
        <div class="confirm-icon" :class="uiStore.confirmation.tone">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 2 1 21h22L12 2Zm0 5.2L19.6 19H4.4L12 7.2ZM11 10v5h2v-5h-2Zm0 6.5v2h2v-2h-2Z" />
          </svg>
        </div>
        <div>
          <p class="confirm-eyebrow">Safe mode</p>
          <h2 id="confirm-title">{{ uiStore.confirmation.title }}</h2>
          <p>{{ uiStore.confirmation.message }}</p>
        </div>
        <div class="confirm-actions">
          <button class="btn btn-secondary" @click="uiStore.resolveConfirmation(false)">Cancel</button>
          <button class="btn btn-danger" @click="uiStore.resolveConfirmation(true)">
            {{ uiStore.confirmation.confirmLabel }}
          </button>
        </div>
      </section>
    </div>
  </div>
</template>
