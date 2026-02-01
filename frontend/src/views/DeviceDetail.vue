<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDeviceStore } from '../stores/devices'
import api from '../api'

const route = useRoute()
const router = useRouter()
const deviceStore = useDeviceStore()

const device = ref(null)
const profile = ref(null)
const profiles = ref([])
const loading = ref(true)
const switching = ref(false)
const editingSourceName = ref(null)
const newSourceName = ref('')

// Edit modal state
const showEditModal = ref(false)
const saving = ref(false)
const testResult = ref(null)
const testing = ref(false)
const form = ref({
  name: '',
  ip_address: '',
  port: 161,
  community: 'public',
  snmp_version: 'v2c',
  profile_id: '',
  poll_interval: 0,
  enabled: true,
})

const state = computed(() => {
  return deviceStore.getDeviceState(route.params.id)
})

const isATS = computed(() => {
  return profile.value?.category === 'ats'
})

const isPDU = computed(() => {
  return profile.value?.category === 'pdu'
})

// PDU outlet data
const outlets = computed(() => {
  if (!state.value?.values || !isPDU.value) return []

  const result = []
  const now = Date.now()
  const PENDING_TIMEOUT = 10000 // 10 seconds

  for (let i = 1; i <= 8; i++) {
    const name = state.value.values[`Outlet ${i} Name`] || `Outlet ${i}`
    let stateVal = state.value.values[`Outlet ${i} State`]
    const current = state.value.values[`Outlet ${i} Current`] ?? null

    // Check if there's a pending state that should override poll data
    const pending = pendingOutletStates.value[i]
    if (pending && (now - pending.timestamp) < PENDING_TIMEOUT) {
      // Use pending state to prevent stale poll data from reverting UI
      stateVal = pending.expectedState
    } else if (pending) {
      // Clear expired pending state
      delete pendingOutletStates.value[i]
    }

    const isOn = stateVal === 'On' || stateVal === 1
    result.push({
      number: i,
      name,
      state: isOn ? 'On' : 'Off',
      isOn,
      current
    })
  }
  return result
})

// PDU summary data
const pduSummary = computed(() => {
  if (!state.value?.values || !isPDU.value) return null

  const voltage = parseFloat(state.value.values['Voltage']) || 0
  const totalCurrent = parseFloat(state.value.values['Total Current']) || 0
  const activePower = parseFloat(state.value.values['Active Power']) || 0
  const totalEnergy = parseFloat(state.value.values['Total Energy']) || 0

  // Calculate power if not provided (P = V * I)
  const calculatedPower = voltage * totalCurrent
  const power = activePower > 0 ? activePower : calculatedPower

  return {
    voltage,
    totalCurrent,
    power,
    totalEnergy
  }
})

// PDU state
const outletLoading = ref({})
const editingOutletName = ref(null)
const newOutletName = ref('')
// Track pending outlet state changes to prevent stale poll data from reverting UI
const pendingOutletStates = ref({}) // { outletNum: { expectedState: 'On'|'Off', timestamp: Date.now() } }

const selectedSource = computed(() => {
  if (!state.value?.values) return null
  return state.value.values['Selected Source'] || state.value.values['.1.3.6.1.4.1.318.1.1.8.5.1.2.0']
})

const sourceAName = computed(() => {
  if (!state.value?.values) return 'Source A'
  return state.value.values['Source A Name'] || state.value.values['.1.3.6.1.4.1.318.1.1.8.5.3.2.1.6.1'] || 'Source A'
})

const sourceBName = computed(() => {
  if (!state.value?.values) return 'Source B'
  return state.value.values['Source B Name'] || state.value.values['.1.3.6.1.4.1.318.1.1.8.5.3.2.1.6.2'] || 'Source B'
})

onMounted(async () => {
  try {
    const [deviceData, profileList] = await Promise.all([
      api.getDevice(route.params.id),
      api.getProfiles()
    ])
    device.value = deviceData
    profiles.value = profileList
    if (device.value.profile_id) {
      profile.value = await api.getProfile(device.value.profile_id)
    }
  } catch (e) {
    alert('Device not found')
    router.push('/devices')
  } finally {
    loading.value = false
  }
})

async function testConnection() {
  try {
    const result = await api.testConnection(device.value.id)
    alert(result.success ? `Success: ${result.message}` : `Failed: ${result.message}`)
  } catch (e) {
    alert('Error: ' + e.message)
  }
}

async function switchToSource(source) {
  if (switching.value) return
  switching.value = true
  try {
    await api.switchSource(device.value.id, source)
  } catch (e) {
    alert('Error switching source: ' + e.message)
  } finally {
    switching.value = false
  }
}

function startEditSourceName(source) {
  editingSourceName.value = source
  newSourceName.value = source === 1 ? sourceAName.value : sourceBName.value
}

async function saveSourceName() {
  if (!editingSourceName.value || !newSourceName.value.trim()) return
  try {
    await api.setSourceName(device.value.id, editingSourceName.value, newSourceName.value.trim())
    editingSourceName.value = null
    newSourceName.value = ''
  } catch (e) {
    alert('Error setting source name: ' + e.message)
  }
}

function cancelEditSourceName() {
  editingSourceName.value = null
  newSourceName.value = ''
}

// PDU Outlet Functions
async function toggleOutlet(outlet) {
  if (outletLoading.value[outlet.number]) return
  outletLoading.value[outlet.number] = true
  const expectedState = outlet.isOn ? 'Off' : 'On'
  try {
    const newState = outlet.isOn ? 'off' : 'on'
    await api.setOutletState(device.value.id, outlet.number, newState)
    // Set pending state immediately to prevent stale poll data from reverting UI
    pendingOutletStates.value[outlet.number] = {
      expectedState,
      timestamp: Date.now()
    }
    // Wait briefly for poll to confirm the change
    await waitForOutletState(outlet.number, expectedState, 3000)
  } catch (e) {
    // Clear pending state on error
    delete pendingOutletStates.value[outlet.number]
    alert('Error toggling outlet: ' + e.message)
  } finally {
    outletLoading.value[outlet.number] = false
  }
}

// Wait for outlet state to change or timeout
async function waitForOutletState(outletNum, expectedState, timeoutMs) {
  const startTime = Date.now()
  while (Date.now() - startTime < timeoutMs) {
    await new Promise(r => setTimeout(r, 300))
    const currentState = state.value?.values?.[`Outlet ${outletNum} State`]
    if (currentState === expectedState || currentState === (expectedState === 'On' ? 1 : 0)) {
      // State confirmed - clear pending state
      delete pendingOutletStates.value[outletNum]
      return true
    }
  }
  // Timeout - keep pending state active to prevent flickering
  return false
}

async function rebootOutlet(outlet) {
  if (outletLoading.value[outlet.number]) return
  if (!confirm(`Reboot ${outlet.name}? This will turn off and then on the outlet.`)) return
  outletLoading.value[outlet.number] = true
  try {
    await api.rebootOutlet(device.value.id, outlet.number)
  } catch (e) {
    alert('Error rebooting outlet: ' + e.message)
  } finally {
    outletLoading.value[outlet.number] = false
  }
}

function startEditOutletName(outlet) {
  editingOutletName.value = outlet.number
  newOutletName.value = outlet.name
}

async function saveOutletName() {
  if (!editingOutletName.value || !newOutletName.value.trim()) return
  const outletNum = editingOutletName.value
  const expectedName = newOutletName.value.trim()
  outletLoading.value[outletNum] = true
  try {
    await api.setOutletName(device.value.id, outletNum, expectedName)
    // Wait for name to update (poll should trigger immediately)
    await waitForOutletName(outletNum, expectedName, 5000)
    editingOutletName.value = null
    newOutletName.value = ''
  } catch (e) {
    alert('Error setting outlet name: ' + e.message)
  } finally {
    outletLoading.value[outletNum] = false
  }
}

// Wait for outlet name to change or timeout
async function waitForOutletName(outletNum, expectedName, timeoutMs) {
  const startTime = Date.now()
  while (Date.now() - startTime < timeoutMs) {
    await new Promise(r => setTimeout(r, 300))
    const currentName = state.value?.values?.[`Outlet ${outletNum} Name`]
    if (currentName === expectedName) {
      return true
    }
  }
  return false
}

function cancelEditOutletName() {
  editingOutletName.value = null
  newOutletName.value = ''
}

function openEditModal() {
  form.value = {
    name: device.value.name,
    ip_address: device.value.ip_address,
    port: device.value.port,
    community: device.value.community,
    snmp_version: device.value.snmp_version,
    profile_id: device.value.profile_id || '',
    poll_interval: device.value.poll_interval || 0,
    enabled: device.value.enabled,
  }
  testResult.value = null
  showEditModal.value = true
}

async function testConnectionNew() {
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await api.testNewConnection({
      ip_address: form.value.ip_address,
      port: form.value.port,
      community: form.value.community,
      snmp_version: form.value.snmp_version,
    })
  } catch (e) {
    testResult.value = { success: false, message: e.message }
  } finally {
    testing.value = false
  }
}

async function saveDevice() {
  saving.value = true
  try {
    await deviceStore.updateDevice(device.value.id, form.value)
    device.value = await api.getDevice(device.value.id)
    if (device.value.profile_id) {
      profile.value = await api.getProfile(device.value.profile_id)
    } else {
      profile.value = null
    }
    showEditModal.value = false
  } catch (e) {
    alert('Error: ' + e.message)
  } finally {
    saving.value = false
  }
}

async function deleteDevice() {
  if (confirm(`Are you sure you want to delete "${device.value.name}"?`)) {
    await deviceStore.deleteDevice(device.value.id)
    router.push('/devices')
  }
}

function formatValue(mapping, value) {
  if (value === null || value === undefined) return '-'
  if (mapping.unit) return `${value} ${mapping.unit}`
  return String(value)
}

function getValueByMapping(mapping) {
  if (!state.value?.values) return null
  return state.value.values[mapping.name] || state.value.values[mapping.oid]
}
</script>

<template>
  <div>
    <div v-if="loading" class="text-center py-8">Loading...</div>

    <div v-else-if="device">
      <div class="flex justify-between items-start mb-6">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-dracula-fg">{{ device.name }}</h1>
          <p class="text-gray-500 dark:text-dracula-comment">{{ device.ip_address }}:{{ device.port }}</p>
        </div>
        <div class="flex items-center space-x-4">
          <span :class="state?.online ? 'status-online' : 'status-offline'">
            {{ state?.online ? 'Online' : 'Offline' }}
          </span>
          <button @click="openEditModal" class="btn btn-primary">Edit Settings</button>
          <button @click="testConnection" class="btn btn-secondary">Test Connection</button>
          <RouterLink :to="`/devices`" class="btn btn-secondary">Back</RouterLink>
        </div>
      </div>

      <!-- ATS Source Control Panel -->
      <div v-if="isATS && state?.online" class="card mb-6">
        <h2 class="text-lg font-semibold mb-4">Source Control</h2>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <!-- Source A -->
          <div class="p-4 rounded-lg border-2 transition-colors"
               :class="selectedSource === 'Source A' || selectedSource === 1 ? 'border-green-500 bg-green-50 dark:bg-dracula-green/10' : 'border-gray-200 dark:border-dracula-comment/50 dark:bg-dracula-bg/50'">
            <div class="flex justify-between items-start mb-3">
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-lg font-semibold text-gray-900 dark:text-dracula-fg">
                    <template v-if="editingSourceName === 1">
                      <input v-model="newSourceName"
                             class="input w-48"
                             @keyup.enter="saveSourceName"
                             @keyup.escape="cancelEditSourceName" />
                    </template>
                    <template v-else>
                      {{ sourceAName }}
                    </template>
                  </span>
                  <span v-if="selectedSource === 'Source A' || selectedSource === 1"
                        class="px-2 py-1 text-xs bg-green-500 dark:bg-dracula-green text-white rounded font-medium">ACTIVE</span>
                </div>
                <p class="text-sm text-gray-500 dark:text-dracula-cyan">Input A</p>
              </div>
              <div class="flex gap-2">
                <template v-if="editingSourceName === 1">
                  <button @click="saveSourceName" class="text-green-600 dark:text-dracula-green hover:text-green-800 text-sm">Save</button>
                  <button @click="cancelEditSourceName" class="text-gray-600 dark:text-dracula-fg hover:text-gray-800 text-sm">Cancel</button>
                </template>
                <template v-else>
                  <button @click="startEditSourceName(1)" class="text-blue-600 dark:text-dracula-pink hover:text-blue-800 text-sm">Rename</button>
                </template>
              </div>
            </div>

            <div class="mb-3">
              <span class="text-2xl font-bold text-gray-900 dark:text-dracula-yellow">
                {{ state?.values?.['Source A Voltage'] || state?.values?.['.1.3.6.1.4.1.318.1.1.8.5.3.3.1.3.1.1.1'] || '-' }} V
              </span>
            </div>

            <button @click="switchToSource(1)"
                    :disabled="switching || selectedSource === 'Source A' || selectedSource === 1"
                    class="btn btn-primary w-full disabled:opacity-40 disabled:cursor-not-allowed">
              {{ switching ? 'Switching...' : 'Switch to Source A' }}
            </button>
          </div>

          <!-- Source B -->
          <div class="p-4 rounded-lg border-2 transition-colors"
               :class="selectedSource === 'Source B' || selectedSource === 2 ? 'border-green-500 bg-green-50 dark:bg-dracula-green/10' : 'border-gray-200 dark:border-dracula-comment/50 dark:bg-dracula-bg/50'">
            <div class="flex justify-between items-start mb-3">
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-lg font-semibold text-gray-900 dark:text-dracula-fg">
                    <template v-if="editingSourceName === 2">
                      <input v-model="newSourceName"
                             class="input w-48"
                             @keyup.enter="saveSourceName"
                             @keyup.escape="cancelEditSourceName" />
                    </template>
                    <template v-else>
                      {{ sourceBName }}
                    </template>
                  </span>
                  <span v-if="selectedSource === 'Source B' || selectedSource === 2"
                        class="px-2 py-1 text-xs bg-green-500 dark:bg-dracula-green text-white rounded font-medium">ACTIVE</span>
                </div>
                <p class="text-sm text-gray-500 dark:text-dracula-cyan">Input B</p>
              </div>
              <div class="flex gap-2">
                <template v-if="editingSourceName === 2">
                  <button @click="saveSourceName" class="text-green-600 dark:text-dracula-green hover:text-green-800 text-sm">Save</button>
                  <button @click="cancelEditSourceName" class="text-gray-600 dark:text-dracula-fg hover:text-gray-800 text-sm">Cancel</button>
                </template>
                <template v-else>
                  <button @click="startEditSourceName(2)" class="text-blue-600 dark:text-dracula-pink hover:text-blue-800 text-sm">Rename</button>
                </template>
              </div>
            </div>

            <div class="mb-3">
              <span class="text-2xl font-bold text-gray-900 dark:text-dracula-yellow">
                {{ state?.values?.['Source B Voltage'] || state?.values?.['.1.3.6.1.4.1.318.1.1.8.5.3.3.1.3.2.1.1'] || '-' }} V
              </span>
            </div>

            <button @click="switchToSource(2)"
                    :disabled="switching || selectedSource === 'Source B' || selectedSource === 2"
                    class="btn btn-primary w-full disabled:opacity-40 disabled:cursor-not-allowed">
              {{ switching ? 'Switching...' : 'Switch to Source B' }}
            </button>
          </div>
        </div>

        <!-- Output Info -->
        <div class="mt-6 p-4 bg-gray-50 dark:bg-dracula-bg/50 rounded-lg">
          <h3 class="font-medium mb-3 text-gray-900 dark:text-dracula-fg">Output</h3>
          <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-center">
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Voltage</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ state?.values?.['Output Voltage'] || '-' }} V
              </p>
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Current</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ state?.values?.['Output Current'] || '-' }} A
              </p>
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Power</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ state?.values?.['Output Apparent Power'] || '-' }} VA
              </p>
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Load</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ state?.values?.['Output Load'] || '-' }} %
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- PDU Outlet Control Panel -->
      <div v-if="isPDU && state?.online" class="card mb-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-lg font-semibold dark:text-dracula-fg">Outlet Control</h2>
          <div class="text-sm text-gray-500 dark:text-dracula-comment">
            {{ outlets.filter(o => o.isOn).length }} / {{ outlets.length }} outlets on
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <div v-for="outlet in outlets" :key="outlet.number"
               class="p-4 rounded-lg border-2 transition-colors relative"
               :class="outlet.isOn ? 'border-green-500 bg-green-50 dark:bg-dracula-green/10 dark:border-dracula-green' : 'border-gray-200 bg-gray-50 dark:border-dracula-comment/50 dark:bg-dracula-bg/50'">
            <!-- Loading overlay -->
            <div v-if="outletLoading[outlet.number]"
                 class="absolute inset-0 bg-white/70 dark:bg-dracula-bg/70 rounded-lg flex items-center justify-center z-10">
              <div class="flex flex-col items-center">
                <svg class="animate-spin h-8 w-8 text-blue-500 dark:text-dracula-purple" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span class="text-sm mt-2 text-gray-600 dark:text-dracula-fg">Switching...</span>
              </div>
            </div>

            <div class="flex justify-between items-start mb-2">
              <div class="flex-1 min-w-0">
                <template v-if="editingOutletName === outlet.number">
                  <input v-model="newOutletName"
                         class="input w-full text-sm"
                         @keyup.enter="saveOutletName"
                         @keyup.escape="cancelEditOutletName" />
                  <div class="flex gap-2 mt-1">
                    <button @click="saveOutletName" class="text-green-600 dark:text-dracula-green hover:text-green-800 text-xs">Save</button>
                    <button @click="cancelEditOutletName" class="text-gray-600 dark:text-dracula-fg hover:text-gray-800 text-xs">Cancel</button>
                  </div>
                </template>
                <template v-else>
                  <p class="font-medium truncate dark:text-dracula-fg" :title="outlet.name">{{ outlet.name }}</p>
                  <button @click="startEditOutletName(outlet)" class="text-blue-600 dark:text-dracula-pink hover:text-blue-800 text-xs">Rename</button>
                </template>
              </div>
              <span class="ml-2 px-2 py-1 text-xs rounded"
                    :class="outlet.isOn ? 'bg-green-500 dark:bg-dracula-green text-white' : 'bg-gray-400 dark:bg-dracula-comment text-white'">
                {{ outlet.state }}
              </span>
            </div>

            <!-- Outlet current -->
            <div class="text-center mb-2">
              <span class="text-lg font-semibold dark:text-dracula-yellow">
                {{ outlet.current !== null ? outlet.current.toFixed(1) : '-' }} A
              </span>
            </div>

            <div class="flex gap-2 mt-3">
              <button @click="toggleOutlet(outlet)"
                      :disabled="outletLoading[outlet.number]"
                      class="btn text-sm flex-1"
                      :class="outlet.isOn ? 'btn-secondary' : 'btn-primary'">
                {{ outlet.isOn ? 'Turn Off' : 'Turn On' }}
              </button>
              <button @click="rebootOutlet(outlet)"
                      :disabled="outletLoading[outlet.number] || !outlet.isOn"
                      class="btn btn-secondary text-sm"
                      :class="{ 'opacity-50 cursor-not-allowed': !outlet.isOn }"
                      title="Reboot">
                ↻
              </button>
            </div>
          </div>
        </div>

        <!-- PDU Load Info -->
        <div class="mt-6 p-4 bg-gray-50 dark:bg-dracula-bg/50 rounded-lg">
          <h3 class="font-medium mb-3 text-gray-900 dark:text-dracula-fg">Power Summary</h3>
          <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-center">
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Voltage</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ pduSummary?.voltage > 0 ? pduSummary.voltage.toFixed(1) : '-' }} V
              </p>
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Total Current</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ pduSummary?.totalCurrent > 0 ? pduSummary.totalCurrent.toFixed(2) : '-' }} A
              </p>
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Power</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ pduSummary?.power > 0 ? pduSummary.power.toFixed(1) : '-' }} W
              </p>
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dracula-cyan">Total Energy</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-dracula-yellow">
                {{ pduSummary?.totalEnergy > 0 ? pduSummary.totalEnergy.toFixed(3) : '-' }} kWh
              </p>
            </div>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Device Info -->
        <div class="card">
          <h2 class="text-lg font-semibold mb-4 text-gray-900 dark:text-dracula-fg">Device Information</h2>
          <dl class="space-y-2">
            <div class="flex justify-between">
              <dt class="text-gray-500 dark:text-dracula-pink">SNMP Version</dt>
              <dd class="text-gray-900 dark:text-dracula-fg">{{ device.snmp_version }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500 dark:text-dracula-pink">Community</dt>
              <dd class="text-gray-900 dark:text-dracula-fg">{{ device.community }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500 dark:text-dracula-pink">Profile</dt>
              <dd class="dark:text-dracula-fg">{{ profile?.name || 'None' }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500 dark:text-dracula-pink">Poll Interval</dt>
              <dd class="dark:text-dracula-fg">{{ device.poll_interval || 'Default' }}s</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-gray-500 dark:text-dracula-pink">Last Seen</dt>
              <dd class="dark:text-dracula-fg">{{ device.last_seen ? new Date(device.last_seen).toLocaleString() : 'Never' }}</dd>
            </div>
          </dl>
        </div>

        <!-- Live State -->
        <div class="card">
          <h2 class="text-lg font-semibold mb-4 dark:text-dracula-fg">Status</h2>
          <div v-if="!state" class="text-gray-500 dark:text-dracula-comment">No state data available</div>
          <div v-else-if="state.errors?.length" class="text-red-600 dark:text-dracula-red">
            <p v-for="err in state.errors" :key="err">{{ err }}</p>
          </div>
          <div v-else>
            <p class="text-sm text-gray-500 dark:text-dracula-comment mb-4">Last poll: {{ new Date(state.last_poll).toLocaleString() }}</p>

            <!-- Status indicators for ATS -->
            <div v-if="isATS" class="space-y-2">
              <div class="flex justify-between items-center">
                <span class="dark:text-dracula-fg">Redundancy</span>
                <span :class="state.values?.['Redundancy State'] === 'Redundant' ? 'status-online' : 'status-offline'">
                  {{ state.values?.['Redundancy State'] || '-' }}
                </span>
              </div>
              <div class="flex justify-between items-center">
                <span class="dark:text-dracula-fg">Source A Status</span>
                <span :class="state.values?.['Source A Status'] === 'OK' ? 'status-online' : 'status-offline'">
                  {{ state.values?.['Source A Status'] || '-' }}
                </span>
              </div>
              <div class="flex justify-between items-center">
                <span class="dark:text-dracula-fg">Source B Status</span>
                <span :class="state.values?.['Source B Status'] === 'OK' ? 'status-online' : 'status-offline'">
                  {{ state.values?.['Source B Status'] || '-' }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- All Sensor Values -->
      <div v-if="profile && state?.values" class="card mt-6">
        <h2 class="text-lg font-semibold mb-4 dark:text-dracula-fg">All Sensor Values</h2>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <div v-for="mapping in profile.oid_mappings" :key="mapping.oid" class="p-4 bg-gray-50 dark:bg-dracula-bg/50 rounded-lg">
            <p class="text-sm text-gray-500 dark:text-dracula-cyan">{{ mapping.name }}</p>
            <p class="text-xl font-semibold dark:text-dracula-yellow">
              {{ formatValue(mapping, getValueByMapping(mapping)) }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Edit Modal -->
    <div v-if="showEditModal" class="fixed inset-0 bg-black bg-opacity-50 dark:bg-opacity-80 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-dracula-current rounded-lg shadow-xl w-full max-w-lg mx-4">
        <div class="px-6 py-4 border-b dark:border-dracula-comment/40 flex justify-between items-center">
          <h2 class="text-lg font-semibold dark:text-dracula-fg">Edit Device Settings</h2>
          <button @click="deleteDevice" class="text-red-600 dark:text-dracula-red hover:text-red-800 text-sm">Delete Device</button>
        </div>

        <div class="px-6 py-4 space-y-4">
          <div>
            <label class="label">Name</label>
            <input v-model="form.name" class="input" placeholder="My UPS" required />
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="label">IP Address</label>
              <input v-model="form.ip_address" class="input" placeholder="192.168.1.100" required />
            </div>
            <div>
              <label class="label">Port</label>
              <input v-model.number="form.port" type="number" class="input" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="label">{{ form.snmp_version === 'v3' ? 'Username' : 'Community' }}</label>
              <input v-model="form.community" class="input" :placeholder="form.snmp_version === 'v3' ? 'snmpuser' : 'public'" />
              <p v-if="form.snmp_version === 'v3'" class="text-xs text-gray-500 mt-1">noAuthNoPriv mode</p>
            </div>
            <div>
              <label class="label">SNMP Version</label>
              <select v-model="form.snmp_version" class="input">
                <option value="v1">v1</option>
                <option value="v2c">v2c</option>
                <option value="v3">v3</option>
              </select>
            </div>
          </div>

          <div>
            <label class="label">Profile</label>
            <select v-model="form.profile_id" class="input">
              <option value="">-- Select Profile --</option>
              <option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </div>

          <div>
            <label class="label">Poll Interval (seconds, 0 = default)</label>
            <input v-model.number="form.poll_interval" type="number" class="input" min="0" />
          </div>

          <div class="flex items-center">
            <input v-model="form.enabled" type="checkbox" id="enabled" class="mr-2" />
            <label for="enabled" class="dark:text-dracula-fg">Enabled</label>
          </div>

          <!-- Test Connection -->
          <div class="border-t dark:border-dracula-comment/40 pt-4">
            <button @click="testConnectionNew" :disabled="testing" class="btn btn-secondary w-full">
              {{ testing ? 'Testing...' : 'Test Connection' }}
            </button>
            <div v-if="testResult" class="mt-2 p-3 rounded" :class="testResult.success ? 'bg-green-100 dark:bg-dracula-green/20' : 'bg-red-100 dark:bg-dracula-red/20'">
              <p class="font-medium" :class="testResult.success ? 'dark:text-dracula-green' : 'dark:text-dracula-red'">{{ testResult.success ? 'Success' : 'Failed' }}</p>
              <p class="text-sm dark:text-dracula-fg">{{ testResult.message }}</p>
              <p v-if="testResult.sys_name" class="text-sm dark:text-dracula-fg">System: {{ testResult.sys_name }}</p>
              <p v-if="testResult.response_time_ms" class="text-sm dark:text-dracula-fg">Response: {{ testResult.response_time_ms }}ms</p>
            </div>
          </div>
        </div>

        <div class="px-6 py-4 border-t dark:border-dracula-comment/40 flex justify-end space-x-2">
          <button @click="showEditModal = false" class="btn btn-secondary">Cancel</button>
          <button @click="saveDevice" :disabled="saving" class="btn btn-primary">
            {{ saving ? 'Saving...' : 'Save' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
