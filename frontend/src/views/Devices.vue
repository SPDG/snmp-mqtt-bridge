<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useDeviceStore } from '../stores/devices'
import api from '../api'

const deviceStore = useDeviceStore()
const showModal = ref(false)
const editingDevice = ref(null)
const profiles = ref([])
const testResult = ref(null)
const testing = ref(false)
const searchQuery = ref('')
const togglingEnabled = ref({})

const form = ref({
  name: '',
  ip_address: '',
  port: 161,
  community: 'public',
  write_community: '',
  snmp_version: 'v2c',
  profile_id: '',
  poll_interval: 0,
  enabled: true,
})

const profileByID = computed(() => new Map(profiles.value.map(profile => [profile.id, profile])))

function toNumber(value) {
  if (value === null || value === undefined || value === '') return null
  const number = Number.parseFloat(value)
  return Number.isFinite(number) ? number : null
}

function formatMetric(value, digits, unit) {
  return value === null ? `- ${unit}` : `${value.toFixed(digits)} ${unit}`
}

function outletNumbers(values) {
  const numbers = new Set()
  for (const name of Object.keys(values || {})) {
    const match = name.match(/^Outlet (\d+) State$/)
    if (match) numbers.add(Number(match[1]))
  }
  return [...numbers]
}

function outletIsOn(value, profile) {
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['on', 'true', 'enabled'].includes(normalized)) return true
    if (['off', 'false', 'disabled'].includes(normalized)) return false
  }
  const numeric = Number(value)
  return profile?.manufacturer === 'ATEN' ? numeric === 2 : numeric === 1
}

function sumOutletMetric(values, numbers, metric) {
  let total = 0
  let found = false
  for (const number of numbers) {
    const value = toNumber(values?.[`Outlet ${number} ${metric}`])
    if (value !== null) {
      total += value
      found = true
    }
  }
  return found ? total : null
}

function getDeviceSummary(device, state, profile) {
  if (!device.enabled) {
    return { primary: 'Polling disabled', secondary: 'Enable to resume monitoring' }
  }
  if (!state?.online) {
    return { primary: 'No live data', secondary: state?.errors?.[0] || 'Waiting for a successful poll' }
  }

  const values = state.values || {}
  if (profile?.category === 'pdu') {
    const numbers = outletNumbers(values)
    const active = numbers.filter(number => outletIsOn(values[`Outlet ${number} State`], profile)).length
    const current = toNumber(values['Total Current']) ?? sumOutletMetric(values, numbers, 'Current')
    const power = toNumber(values['Active Power']) ?? sumOutletMetric(values, numbers, 'Power')
    return {
      primary: `${active}/${numbers.length} outlets on`,
      secondary: `${formatMetric(current, 2, 'A')} · ${formatMetric(power, 1, 'W')}`,
    }
  }

  if (profile?.category === 'ats') {
    const selected = values['Selected Source']
    const sourceA = String(values['Source A Name'] || 'Source A').replace(/\0/g, '').trim()
    const sourceB = String(values['Source B Name'] || 'Source B').replace(/\0/g, '').trim()
    const active = selected === 1 || selected === '1' || selected === 'Source A' || selected === sourceA ? sourceA : sourceB
    return {
      primary: `Active: ${active}`,
      secondary: `Output ${formatMetric(toNumber(values['Output Voltage']), 1, 'V')}`,
    }
  }

  const voltage = toNumber(values['Output Voltage']) ?? toNumber(values['Input Voltage']) ?? toNumber(values.Voltage)
  const load = toNumber(values['Output Load']) ?? toNumber(values.Load)
  return {
    primary: `Voltage ${formatMetric(voltage, 1, 'V')}`,
    secondary: `Load ${formatMetric(load, 0, '%')}`,
  }
}

const devicesWithState = computed(() => deviceStore.devices.map(device => {
  const state = deviceStore.getDeviceState(device.id)
  const profile = profileByID.value.get(device.profile_id)
  return {
    ...device,
    state,
    profile,
    category: profile?.category || 'device',
    summary: getDeviceSummary(device, state, profile),
  }
}))

const filteredDevices = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return devicesWithState.value
  return devicesWithState.value.filter(device => [
    device.name,
    device.ip_address,
    device.profile?.name,
    device.profile?.manufacturer,
    device.profile?.model,
  ].some(value => String(value || '').toLowerCase().includes(query)))
})

// Get the selected profile
const selectedProfile = computed(() => {
  if (!form.value.profile_id) return null
  return profiles.value.find(p => p.id === form.value.profile_id)
})

// Get allowed SNMP versions for the selected profile
const allowedSnmpVersions = computed(() => {
  if (selectedProfile.value?.snmp_versions?.length > 0) {
    return selectedProfile.value.snmp_versions
  }
  // Default: all versions
  return ['v1', 'v2c', 'v3']
})

// Watch for profile changes and auto-select SNMP version if needed
watch(() => form.value.profile_id, () => {
  if (selectedProfile.value?.snmp_versions?.length > 0) {
    // If current version is not allowed, select first allowed version
    if (!selectedProfile.value.snmp_versions.includes(form.value.snmp_version)) {
      form.value.snmp_version = selectedProfile.value.snmp_versions[0]
    }
  }
})

onMounted(async () => {
  profiles.value = await api.getProfiles()
})

function openCreateModal() {
  editingDevice.value = null
  form.value = {
    name: '',
    ip_address: '',
    port: 161,
    community: 'public',
    write_community: '',
    snmp_version: 'v2c',
    profile_id: '',
    poll_interval: 0,
    enabled: true,
  }
  testResult.value = null
  showModal.value = true
}

function openEditModal(device) {
  editingDevice.value = device
  form.value = { ...device }
  testResult.value = null
  showModal.value = true
}

async function testConnection() {
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
  try {
    if (editingDevice.value) {
      await deviceStore.updateDevice(editingDevice.value.id, form.value)
    } else {
      await deviceStore.createDevice(form.value)
    }
    showModal.value = false
  } catch (e) {
    alert('Error: ' + e.message)
  }
}

async function deleteDevice(device) {
  if (confirm(`Are you sure you want to delete "${device.name}"?`)) {
    await deviceStore.deleteDevice(device.id)
  }
}

async function toggleDeviceEnabled(device) {
  if (togglingEnabled.value[device.id]) return
  togglingEnabled.value[device.id] = true
  try {
    await deviceStore.updateDevice(device.id, { enabled: !device.enabled })
  } catch (e) {
    alert('Error changing device state: ' + e.message)
  } finally {
    delete togglingEnabled.value[device.id]
  }
}
</script>

<template>
  <div class="devices-view">
    <section class="devices-intro">
      <div>
        <p class="devices-eyebrow">Device management</p>
        <h2>SNMP infrastructure</h2>
        <p>Configure polling and inspect the current state of every managed device.</p>
      </div>
      <button @click="openCreateModal" class="btn btn-primary">
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M9 3h2v6h6v2h-6v6H9v-6H3V9h6V3Z" /></svg>
        Add device
      </button>
    </section>

    <section class="devices-panel">
      <div class="devices-toolbar">
        <label class="device-search">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10.5 3a7.5 7.5 0 1 1-4.7 13.3L2.1 20 3.5 21.4l3.7-3.7A7.5 7.5 0 0 1 10.5 3Zm0 2a5.5 5.5 0 1 0 0 11 5.5 5.5 0 0 0 0-11Z" /></svg>
          <input v-model="searchQuery" type="search" placeholder="Search by name or IP address..." />
          <button v-if="searchQuery" type="button" aria-label="Clear search" @click="searchQuery = ''">×</button>
        </label>
        <div class="devices-count">
          <strong>{{ filteredDevices.length }}</strong>
          <span>{{ filteredDevices.length === 1 ? 'device' : 'devices' }}</span>
        </div>
      </div>

      <div v-if="deviceStore.loading" class="devices-empty">Loading devices...</div>

      <div v-else-if="devicesWithState.length === 0" class="devices-empty">
        <p>No devices configured yet.</p>
        <button type="button" @click="openCreateModal">Add your first SNMP device</button>
      </div>

      <div v-else-if="filteredDevices.length === 0" class="devices-empty">
        <p>No devices match “{{ searchQuery }}”.</p>
        <button type="button" @click="searchQuery = ''">Clear search</button>
      </div>

      <div v-else class="devices-table-wrap">
        <table class="devices-table">
        <thead>
          <tr>
            <th>Device</th>
            <th>Profile</th>
            <th>Live state</th>
            <th>Address</th>
            <th>Enabled</th>
            <th><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="device in filteredDevices" :key="device.id" :class="{ disabled: !device.enabled }">
            <td>
              <RouterLink :to="`/devices/${device.id}`" class="device-cell">
                <span class="device-category-icon" :class="device.category">
                  <svg v-if="device.category === 'ats'" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 3h5v5H8v3h3v2H7a1 1 0 0 1-1-1V8H5V3Zm2 2v1h1V5H7Zm7-2h5v5h-1v4a1 1 0 0 1-1 1h-4v-2h3V8h-2V3Zm2 2v1h1V5h-1ZM9 16h6v5H9v-5Zm2 2v1h2v-1h-2Z" /></svg>
                  <svg v-else-if="device.category === 'pdu'" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Zm0 2v14h14V5H5Zm3 2a2 2 0 1 1 0 4 2 2 0 0 1 0-4Zm0 2h.01L8 9Zm8-2a2 2 0 1 1 0 4 2 2 0 0 1 0-4Zm0 2h.01L16 9Zm-8 4a2 2 0 1 1 0 4 2 2 0 0 1 0-4Zm0 2h.01L8 15Zm8-2a2 2 0 1 1 0 4 2 2 0 0 1 0-4Zm0 2h.01L16 15Z" /></svg>
                  <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M7 2h10v3h2a2 2 0 0 1 2 2v13a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h2V2Zm2 2v3H5v13h14V7h-4V4H9Zm1 6h4v2h-4v-2Zm-2 4h8v2H8v-2Z" /></svg>
                </span>
                <span class="device-cell-copy">
                  <strong>{{ device.name }}</strong>
                  <span>
                    <i :class="device.enabled && device.state?.online ? 'online' : (device.enabled ? 'offline' : 'disabled')"></i>
                    {{ !device.enabled ? 'Disabled' : (device.state?.online ? 'Online' : 'Offline') }}
                  </span>
                </span>
              </RouterLink>
            </td>
            <td>
              <div class="profile-cell">
                <span>{{ device.category.toUpperCase() }}</span>
                <strong>{{ device.profile?.model || device.profile?.name || device.profile_id || 'Generic' }}</strong>
                <small>{{ device.profile?.manufacturer || 'SNMP' }}</small>
              </div>
            </td>
            <td>
              <div class="live-state-cell">
                <strong>{{ device.summary.primary }}</strong>
                <span :title="device.summary.secondary">{{ device.summary.secondary }}</span>
              </div>
            </td>
            <td>
              <div class="address-cell">
                <strong>{{ device.ip_address }}</strong>
                <span>Port {{ device.port }} · {{ device.snmp_version }}</span>
              </div>
            </td>
            <td>
              <button
                type="button"
                class="table-toggle"
                :class="{ active: device.enabled, loading: togglingEnabled[device.id] }"
                role="switch"
                :aria-checked="device.enabled"
                :aria-label="`${device.enabled ? 'Disable' : 'Enable'} ${device.name}`"
                :disabled="togglingEnabled[device.id]"
                @click="toggleDeviceEnabled(device)"
              ><span></span></button>
            </td>
            <td>
              <div class="device-actions">
                <RouterLink :to="`/devices/${device.id}`" title="Open device" aria-label="Open device">
                  <svg viewBox="0 0 20 20" aria-hidden="true"><path d="m7 3 7 7-7 7-1.4-1.4L11.2 10 5.6 4.4 7 3Z" /></svg>
                </RouterLink>
                <button type="button" title="Edit device" aria-label="Edit device" @click="openEditModal(device)">
                  <svg viewBox="0 0 20 20" aria-hidden="true"><path d="m13.8 2.8 3.4 3.4-9.6 9.6-4.3.9.9-4.3 9.6-9.6Zm0 2.8-7.8 7.8-.2.8.8-.2 7.8-7.8-.6-.6Z" /></svg>
                </button>
                <button type="button" class="danger" title="Delete device" aria-label="Delete device" @click="deleteDevice(device)">
                  <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M7 2h6l1 2h3v2H3V4h3l1-2Zm-2 6h10l-.7 10H5.7L5 8Zm2.1 2 .4 6h5l.4-6H7.1Z" /></svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
        </table>
      </div>
    </section>

    <!-- Modal -->
    <div v-if="showModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div class="bg-white rounded-lg shadow-xl w-full max-w-lg mx-4">
        <div class="px-6 py-4 border-b">
          <h2 class="text-lg font-semibold">{{ editingDevice ? 'Edit Device' : 'Add Device' }}</h2>
        </div>

        <div class="px-6 py-4 space-y-4">
          <!-- Profile selection first -->
          <div>
            <label class="label">Profile <span class="text-red-500">*</span></label>
            <select v-model="form.profile_id" class="input" required>
              <option value="">-- Select Profile --</option>
              <option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.name }} ({{ p.manufacturer }})</option>
            </select>
            <p v-if="selectedProfile" class="text-xs text-gray-500 mt-1">
              {{ selectedProfile.category.toUpperCase() }} - {{ selectedProfile.model }}
            </p>
          </div>

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
              <label class="label">SNMP Version</label>
              <select v-model="form.snmp_version" class="input">
                <option v-for="v in allowedSnmpVersions" :key="v" :value="v">{{ v }}</option>
              </select>
              <p v-if="allowedSnmpVersions.length === 1" class="text-xs text-gray-500 mt-1">
                This profile only supports {{ allowedSnmpVersions[0] }}
              </p>
            </div>
            <div>
              <label class="label">{{ form.snmp_version === 'v3' ? 'Username' : 'Community' }}</label>
              <input v-model="form.community" class="input" :placeholder="form.snmp_version === 'v3' ? 'snmpuser' : 'public'" />
              <p v-if="form.snmp_version === 'v3'" class="text-xs text-gray-500 mt-1">noAuthNoPriv mode</p>
            </div>
          </div>

          <div v-if="form.snmp_version !== 'v3'">
            <label class="label">Write Community (optional)</label>
            <input v-model="form.write_community" class="input" placeholder="private" />
            <p class="text-xs text-gray-500 mt-1">Used for SNMP SET commands (e.g., outlet control). Leave empty to use read community.</p>
          </div>

          <div>
            <label class="label">Poll Interval (seconds, 0 = default)</label>
            <input v-model.number="form.poll_interval" type="number" class="input" min="0" />
          </div>

          <div class="flex items-center">
            <input v-model="form.enabled" type="checkbox" id="enabled" class="mr-2" />
            <label for="enabled">Enabled</label>
          </div>

          <!-- Test Connection -->
          <div class="border-t pt-4">
            <button @click="testConnection" :disabled="testing" class="btn btn-secondary w-full">
              {{ testing ? 'Testing...' : 'Test Connection' }}
            </button>
            <div v-if="testResult" class="mt-2 p-3 rounded" :class="testResult.success ? 'bg-green-100' : 'bg-red-100'">
              <p class="font-medium">{{ testResult.success ? 'Success' : 'Failed' }}</p>
              <p class="text-sm">{{ testResult.message }}</p>
              <p v-if="testResult.sys_name" class="text-sm">System: {{ testResult.sys_name }}</p>
              <p v-if="testResult.response_time_ms" class="text-sm">Response: {{ testResult.response_time_ms }}ms</p>
            </div>
          </div>
        </div>

        <div class="px-6 py-4 border-t flex justify-end space-x-2">
          <button @click="showModal = false" class="btn btn-secondary">Cancel</button>
          <button @click="saveDevice" class="btn btn-primary">Save</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.devices-view {
  display: grid;
  gap: 22px;
}

.devices-intro {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.devices-intro h2 {
  margin: 2px 0 0;
  color: var(--ui-text);
  font-size: 24px;
  letter-spacing: -0.035em;
}

.devices-intro > div > p:last-child {
  margin: 5px 0 0;
  color: var(--ui-muted);
  font-size: 14px;
}

.devices-eyebrow {
  margin: 0;
  color: var(--ui-accent);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.devices-intro .btn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.devices-intro .btn svg {
  width: 16px;
  height: 16px;
  fill: currentColor;
}

.devices-panel {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: 13px;
  background: color-mix(in srgb, var(--ui-panel) 94%, transparent);
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.055);
}

.devices-toolbar {
  min-height: 67px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 13px 18px;
  border-bottom: 1px solid var(--ui-border);
}

.device-search {
  width: min(440px, 100%);
  height: 39px;
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) 22px;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 9px;
  color: var(--ui-muted);
  background: var(--ui-bg-soft);
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.device-search:focus-within {
  border-color: color-mix(in srgb, var(--ui-accent) 62%, transparent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-accent) 13%, transparent);
}

.device-search svg {
  width: 18px;
  height: 18px;
  fill: currentColor;
}

.device-search input {
  min-width: 0;
  border: 0;
  outline: 0;
  color: var(--ui-text);
  background: transparent;
  font-size: 13px;
}

.device-search input::placeholder {
  color: var(--ui-muted);
}

.device-search button {
  border: 0;
  color: var(--ui-muted);
  background: transparent;
  font-size: 19px;
  line-height: 1;
  cursor: pointer;
}

.devices-count {
  display: flex;
  align-items: baseline;
  gap: 5px;
  color: var(--ui-muted);
  font-size: 11px;
}

.devices-count strong {
  color: var(--ui-text);
  font-size: 14px;
}

.devices-table-wrap {
  overflow-x: auto;
}

.devices-table {
  width: 100%;
  min-width: 1040px;
  border-collapse: collapse;
}

.devices-table th {
  padding: 11px 16px;
  color: var(--ui-muted);
  background: color-mix(in srgb, var(--ui-panel-strong) 75%, transparent);
  border-bottom: 1px solid var(--ui-border);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.07em;
  text-align: left;
  text-transform: uppercase;
}

.devices-table th:first-child,
.devices-table td:first-child {
  padding-left: 20px;
}

.devices-table th:last-child,
.devices-table td:last-child {
  padding-right: 18px;
}

.devices-table td {
  padding: 14px 16px;
  border-bottom: 1px solid var(--ui-border);
  color: var(--ui-text);
  vertical-align: middle;
}

.devices-table tr:last-child td {
  border-bottom: 0;
}

.devices-table tbody tr {
  transition: background 140ms ease, opacity 140ms ease;
}

.devices-table tbody tr:hover {
  background: var(--ui-panel-strong);
}

.devices-table tbody tr.disabled {
  opacity: 0.64;
}

.devices-table tbody tr.disabled:hover {
  opacity: 0.86;
}

.device-cell {
  min-width: 205px;
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr);
  align-items: center;
  gap: 11px;
  color: inherit;
  text-decoration: none;
}

.device-category-icon {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--ui-accent) 35%, var(--ui-border));
  border-radius: 10px;
  color: var(--ui-accent);
  background: var(--ui-accent-soft);
}

.device-category-icon.ats {
  color: var(--ui-warning);
  border-color: color-mix(in srgb, var(--ui-warning) 38%, var(--ui-border));
  background: color-mix(in srgb, var(--ui-warning) 11%, var(--ui-panel));
}

.device-category-icon svg {
  width: 20px;
  height: 20px;
  fill: currentColor;
}

.device-cell-copy,
.profile-cell,
.live-state-cell,
.address-cell {
  min-width: 0;
  display: grid;
}

.device-cell-copy strong,
.profile-cell strong,
.live-state-cell strong,
.address-cell strong {
  overflow: hidden;
  color: var(--ui-text);
  font-size: 13px;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-cell-copy > span,
.profile-cell small,
.live-state-cell span,
.address-cell span {
  margin-top: 3px;
  overflow: hidden;
  color: var(--ui-muted);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-cell-copy > span {
  display: flex;
  align-items: center;
  gap: 5px;
}

.device-cell-copy i {
  width: 6px;
  height: 6px;
  display: inline-block;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--ui-muted);
}

.device-cell-copy i.online {
  background: var(--ui-success);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--ui-success) 15%, transparent);
}

.device-cell-copy i.offline {
  background: var(--ui-danger);
}

.profile-cell {
  min-width: 135px;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  column-gap: 7px;
}

.profile-cell > span {
  grid-row: 1 / 3;
  padding: 3px 5px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 5px;
  color: var(--ui-muted);
  background: var(--ui-bg-soft);
  font-size: 8px;
  font-weight: 900;
}

.profile-cell small {
  margin-top: 1px;
}

.live-state-cell {
  min-width: 170px;
  max-width: 255px;
}

.address-cell {
  min-width: 125px;
}

.table-toggle {
  width: 34px;
  height: 20px;
  display: block;
  padding: 3px;
  border: 0;
  border-radius: 999px;
  background: var(--ui-border-strong);
  cursor: pointer;
  transition: background 160ms ease, opacity 160ms ease;
}

.table-toggle span {
  width: 14px;
  height: 14px;
  display: block;
  border-radius: 50%;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.3);
  transition: transform 160ms ease;
}

.table-toggle.active {
  background: var(--ui-success);
}

.table-toggle.active span {
  transform: translateX(14px);
}

.table-toggle.loading {
  opacity: 0.48;
  cursor: wait;
}

.device-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
}

.device-actions a,
.device-actions button {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 7px;
  color: var(--ui-muted);
  background: transparent;
  cursor: pointer;
  transition: color 140ms ease, border-color 140ms ease, background 140ms ease;
}

.device-actions a:hover,
.device-actions button:hover {
  color: var(--ui-accent);
  border-color: var(--ui-border);
  background: var(--ui-accent-soft);
}

.device-actions button.danger:hover {
  color: var(--ui-danger);
  background: var(--ui-danger-soft);
}

.device-actions svg {
  width: 15px;
  height: 15px;
  fill: currentColor;
}

.devices-empty {
  padding: 58px 20px;
  color: var(--ui-muted);
  text-align: center;
}

.devices-empty p {
  margin: 0 0 7px;
}

.devices-empty button {
  padding: 0;
  border: 0;
  color: var(--ui-accent);
  background: transparent;
  font-weight: 700;
  cursor: pointer;
}

@media (max-width: 700px) {
  .devices-intro {
    align-items: flex-start;
  }

  .devices-intro h2 {
    font-size: 21px;
  }

  .devices-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .device-search {
    width: 100%;
  }
}

@media (max-width: 520px) {
  .devices-intro {
    display: grid;
  }

  .devices-intro .btn {
    width: max-content;
  }
}
</style>
