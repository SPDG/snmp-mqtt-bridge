<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const settings = ref({})
const loading = ref(true)
const saving = ref(false)
const mqttStatus = ref({ connected: false, broker: '' })
const reconnecting = ref(false)
const testing = ref(false)
const testResult = ref(null)

const formFields = [
  { key: 'mqtt.broker', label: 'MQTT Broker', type: 'text', placeholder: 'localhost' },
  { key: 'mqtt.port', label: 'MQTT Port', type: 'number', placeholder: '1883' },
  { key: 'mqtt.username', label: 'MQTT Username', type: 'text' },
  { key: 'mqtt.password', label: 'MQTT Password', type: 'password' },
  { key: 'mqtt.topic_prefix', label: 'Topic Prefix', type: 'text', placeholder: 'snmp-bridge' },
  { key: 'mqtt.discovery_prefix', label: 'Discovery Prefix', type: 'text', placeholder: 'homeassistant' },
  { key: 'snmp.poll_interval', label: 'Poll Interval (seconds)', type: 'number', placeholder: '30' },
  { key: 'snmp.trap_port', label: 'Trap Port', type: 'number', placeholder: '162' },
]

onMounted(async () => {
  try {
    const [settingsData, status] = await Promise.all([
      api.getSettings(),
      api.getMQTTStatus()
    ])
    settings.value = settingsData
    mqttStatus.value = status
  } finally {
    loading.value = false
  }
})

async function refreshMQTTStatus() {
  try {
    mqttStatus.value = await api.getMQTTStatus()
  } catch (e) {
    console.error('Failed to get MQTT status:', e)
  }
}

async function saveSettings() {
  saving.value = true
  try {
    // Save all settings
    for (const field of formFields) {
      const value = settings.value[field.key]
      if (value !== undefined && value !== '') {
        await api.setSetting(field.key, String(value))
      }
    }

    // Check if any MQTT settings changed and reconnect
    const mqttSettingsChanged = formFields
      .filter(f => f.key.startsWith('mqtt'))
      .some(f => settings.value[f.key] !== undefined)

    if (mqttSettingsChanged) {
      await reconnectMQTT()
    } else {
      alert('Settings saved successfully')
    }
  } catch (e) {
    alert('Error: ' + e.message)
  } finally {
    saving.value = false
  }
}

async function reconnectMQTT() {
  reconnecting.value = true
  try {
    const result = await api.reconnectMQTT()
    mqttStatus.value = { connected: result.connected, broker: settings.value['mqtt.broker'] || '' }
    if (result.success) {
      alert('Settings saved and MQTT reconnected successfully')
    } else {
      alert('Settings saved but MQTT connection failed: ' + result.message)
    }
  } catch (e) {
    alert('Settings saved but MQTT reconnection failed: ' + e.message)
  } finally {
    reconnecting.value = false
    await refreshMQTTStatus()
  }
}

async function testMQTTConnection() {
  testing.value = true
  testResult.value = null
  try {
    const result = await api.testMQTTConnection({
      broker: settings.value['mqtt.broker'] || 'localhost',
      port: parseInt(settings.value['mqtt.port']) || 1883,
      username: settings.value['mqtt.username'] || '',
      password: settings.value['mqtt.password'] || '',
    })
    testResult.value = result
  } catch (e) {
    testResult.value = { success: false, message: e.message }
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold text-gray-900 mb-6">Settings</h1>

    <div class="card max-w-2xl">
      <div v-if="loading" class="text-center py-4">Loading...</div>

      <form v-else @submit.prevent="saveSettings" class="space-y-6">
        <div class="border-b pb-4">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-lg font-semibold">MQTT Configuration</h2>
            <div class="flex items-center gap-3">
              <span :class="mqttStatus.connected ? 'status-online' : 'status-offline'">
                {{ mqttStatus.connected ? 'Connected' : 'Disconnected' }}
              </span>
              <button
                type="button"
                @click="reconnectMQTT"
                :disabled="reconnecting"
                class="btn btn-secondary text-sm"
              >
                {{ reconnecting ? 'Reconnecting...' : 'Reconnect' }}
              </button>
            </div>
          </div>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div v-for="field in formFields.filter(f => f.key.startsWith('mqtt'))" :key="field.key">
              <label :for="field.key" class="label">{{ field.label }}</label>
              <input
                :id="field.key"
                v-model="settings[field.key]"
                :type="field.type"
                :placeholder="field.placeholder"
                class="input"
              />
            </div>
          </div>

          <!-- Test Connection -->
          <div class="mt-4 pt-4 border-t">
            <button
              type="button"
              @click="testMQTTConnection"
              :disabled="testing"
              class="btn btn-secondary"
            >
              {{ testing ? 'Testing...' : 'Test Connection' }}
            </button>
            <div v-if="testResult" class="mt-2 p-3 rounded" :class="testResult.success ? 'bg-green-100' : 'bg-red-100'">
              <p class="font-medium">{{ testResult.success ? 'Success' : 'Failed' }}</p>
              <p class="text-sm">{{ testResult.message }}</p>
            </div>
          </div>
        </div>

        <div class="border-b pb-4">
          <h2 class="text-lg font-semibold mb-4">SNMP Configuration</h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div v-for="field in formFields.filter(f => f.key.startsWith('snmp'))" :key="field.key">
              <label :for="field.key" class="label">{{ field.label }}</label>
              <input
                :id="field.key"
                v-model="settings[field.key]"
                :type="field.type"
                :placeholder="field.placeholder"
                class="input"
              />
            </div>
          </div>
          <p class="text-sm text-gray-500 mt-2">
            Note: SNMP settings require application restart to take effect.
          </p>
        </div>

        <div class="flex justify-end">
          <button type="submit" :disabled="saving" class="btn btn-primary">
            {{ saving ? 'Saving...' : 'Save Settings' }}
          </button>
        </div>
      </form>
    </div>

    <div class="card max-w-2xl mt-6">
      <h2 class="text-lg font-semibold mb-4">About</h2>
      <dl class="space-y-2">
        <div class="flex justify-between">
          <dt class="text-gray-500">Version</dt>
          <dd>1.0.0</dd>
        </div>
        <div class="flex justify-between">
          <dt class="text-gray-500">API</dt>
          <dd><a href="/health" class="text-blue-600 hover:underline">/health</a></dd>
        </div>
      </dl>
    </div>
  </div>
</template>
