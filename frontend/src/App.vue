<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { useDeviceStore } from './stores/devices'

const deviceStore = useDeviceStore()
const menuOpen = ref(false)
const darkMode = ref(false)

// Initialize dark mode from localStorage
function initDarkMode() {
  const saved = localStorage.getItem('darkMode')
  if (saved !== null) {
    darkMode.value = saved === 'true'
  } else {
    // Default to system preference
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

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/ws`

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
    console.log('WebSocket disconnected, reconnecting...')
    setTimeout(connectWebSocket, 3000)
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
  }
}

onMounted(() => {
  initDarkMode()
  deviceStore.fetchDevices()
  connectWebSocket()
})

onUnmounted(() => {
  if (ws) {
    ws.close()
  }
})
</script>

<template>
  <div class="min-h-screen bg-gray-100 dark:bg-dracula-bg transition-colors duration-200">
    <!-- Navigation -->
    <nav class="bg-white dark:bg-dracula-current shadow-sm">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex justify-between h-16">
          <div class="flex">
            <div class="flex-shrink-0 flex items-center">
              <span class="text-xl font-bold text-blue-600 dark:text-dracula-purple">SNMP-MQTT Bridge</span>
            </div>
            <div class="hidden sm:ml-6 sm:flex sm:space-x-8">
              <RouterLink
                to="/"
                class="border-transparent text-gray-500 dark:text-dracula-fg hover:border-gray-300 dark:hover:border-dracula-comment hover:text-gray-700 dark:hover:text-dracula-cyan inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium"
                active-class="border-blue-500 dark:border-dracula-purple text-gray-900 dark:text-dracula-cyan"
              >
                Dashboard
              </RouterLink>
              <RouterLink
                to="/devices"
                class="border-transparent text-gray-500 dark:text-dracula-fg hover:border-gray-300 dark:hover:border-dracula-comment hover:text-gray-700 dark:hover:text-dracula-cyan inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium"
                active-class="border-blue-500 dark:border-dracula-purple text-gray-900 dark:text-dracula-cyan"
              >
                Devices
              </RouterLink>
              <RouterLink
                to="/profiles"
                class="border-transparent text-gray-500 dark:text-dracula-fg hover:border-gray-300 dark:hover:border-dracula-comment hover:text-gray-700 dark:hover:text-dracula-cyan inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium"
                active-class="border-blue-500 dark:border-dracula-purple text-gray-900 dark:text-dracula-cyan"
              >
                Profiles
              </RouterLink>
              <RouterLink
                to="/traps"
                class="border-transparent text-gray-500 dark:text-dracula-fg hover:border-gray-300 dark:hover:border-dracula-comment hover:text-gray-700 dark:hover:text-dracula-cyan inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium"
                active-class="border-blue-500 dark:border-dracula-purple text-gray-900 dark:text-dracula-cyan"
              >
                Trap Logs
              </RouterLink>
              <RouterLink
                to="/settings"
                class="border-transparent text-gray-500 dark:text-dracula-fg hover:border-gray-300 dark:hover:border-dracula-comment hover:text-gray-700 dark:hover:text-dracula-cyan inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium"
                active-class="border-blue-500 dark:border-dracula-purple text-gray-900 dark:text-dracula-cyan"
              >
                Settings
              </RouterLink>
            </div>
          </div>

          <div class="flex items-center space-x-4">
            <!-- Dark mode toggle -->
            <button
              @click="toggleDarkMode"
              class="p-2 rounded-lg text-gray-500 dark:text-dracula-fg hover:bg-gray-100 dark:hover:bg-dracula-selection transition-colors"
              :title="darkMode ? 'Switch to light mode' : 'Switch to dark mode'"
            >
              <!-- Sun icon (shown in dark mode) -->
              <svg v-if="darkMode" class="w-5 h-5 text-dracula-yellow" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" />
              </svg>
              <!-- Moon icon (shown in light mode) -->
              <svg v-else class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
              </svg>
            </button>

            <!-- Mobile menu button -->
            <div class="sm:hidden">
              <button @click="menuOpen = !menuOpen" class="text-gray-500 dark:text-dracula-fg hover:text-gray-700 dark:hover:text-dracula-cyan">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Mobile menu -->
      <div v-if="menuOpen" class="sm:hidden">
        <div class="pt-2 pb-3 space-y-1">
          <RouterLink to="/" class="block pl-3 pr-4 py-2 text-base font-medium text-gray-600 dark:text-dracula-fg hover:bg-gray-50 dark:hover:bg-dracula-selection">Dashboard</RouterLink>
          <RouterLink to="/devices" class="block pl-3 pr-4 py-2 text-base font-medium text-gray-600 dark:text-dracula-fg hover:bg-gray-50 dark:hover:bg-dracula-selection">Devices</RouterLink>
          <RouterLink to="/profiles" class="block pl-3 pr-4 py-2 text-base font-medium text-gray-600 dark:text-dracula-fg hover:bg-gray-50 dark:hover:bg-dracula-selection">Profiles</RouterLink>
          <RouterLink to="/traps" class="block pl-3 pr-4 py-2 text-base font-medium text-gray-600 dark:text-dracula-fg hover:bg-gray-50 dark:hover:bg-dracula-selection">Trap Logs</RouterLink>
          <RouterLink to="/settings" class="block pl-3 pr-4 py-2 text-base font-medium text-gray-600 dark:text-dracula-fg hover:bg-gray-50 dark:hover:bg-dracula-selection">Settings</RouterLink>
        </div>
      </div>
    </nav>

    <!-- Main content -->
    <main class="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
      <RouterView />
    </main>
  </div>
</template>
