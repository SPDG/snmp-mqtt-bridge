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

const devicesWithState = computed(() => {
  return deviceStore.devices.map(d => ({
    ...d,
    state: deviceStore.getDeviceState(d.id)
  }))
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
</script>

<template>
  <div>
    <div class="flex justify-between items-center mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Devices</h1>
      <button @click="openCreateModal" class="btn btn-primary">
        Add Device
      </button>
    </div>

    <div class="card">
      <div v-if="deviceStore.loading" class="text-center py-4">Loading...</div>

      <div v-else-if="devicesWithState.length === 0" class="text-center py-8 text-gray-500">
        No devices configured yet. Click "Add Device" to get started.
      </div>

      <table v-else class="min-w-full divide-y divide-gray-200">
        <thead>
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">IP Address</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Profile</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Enabled</th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          <tr v-for="device in devicesWithState" :key="device.id">
            <td class="px-6 py-4">
              <RouterLink :to="`/devices/${device.id}`" class="text-blue-600 hover:underline font-medium">
                {{ device.name }}
              </RouterLink>
            </td>
            <td class="px-6 py-4 text-gray-500">{{ device.ip_address }}:{{ device.port }}</td>
            <td class="px-6 py-4 text-gray-500">{{ device.profile_id || '-' }}</td>
            <td class="px-6 py-4">
              <span :class="device.state?.online ? 'status-online' : 'status-offline'">
                {{ device.state?.online ? 'Online' : 'Offline' }}
              </span>
            </td>
            <td class="px-6 py-4">
              <span :class="device.enabled ? 'text-green-600' : 'text-gray-400'">
                {{ device.enabled ? 'Yes' : 'No' }}
              </span>
            </td>
            <td class="px-6 py-4 text-right space-x-2">
              <button @click="openEditModal(device)" class="text-blue-600 hover:text-blue-800">Edit</button>
              <button @click="deleteDevice(device)" class="text-red-600 hover:text-red-800">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

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
