import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../api'

export const useDeviceStore = defineStore('devices', () => {
  const devices = ref([])
  const deviceStates = ref({})
  const loading = ref(false)
  const error = ref(null)

  const onlineCount = computed(() => {
    return Object.values(deviceStates.value).filter(s => s?.online).length
  })

  const offlineCount = computed(() => {
    return devices.value.length - onlineCount.value
  })

  async function fetchDevices() {
    loading.value = true
    error.value = null
    try {
      devices.value = await api.getDevices()
    } catch (e) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function createDevice(data) {
    const device = await api.createDevice(data)
    devices.value.push(device)
    return device
  }

  async function updateDevice(id, data) {
    const device = await api.updateDevice(id, data)
    const index = devices.value.findIndex(d => d.id === id)
    if (index !== -1) {
      devices.value[index] = device
    }
    return device
  }

  async function deleteDevice(id) {
    await api.deleteDevice(id)
    devices.value = devices.value.filter(d => d.id !== id)
    delete deviceStates.value[id]
  }

  function updateDeviceState(deviceId, state) {
    deviceStates.value[deviceId] = state
  }

  function setAllStates(states) {
    deviceStates.value = states || {}
  }

  function getDeviceState(deviceId) {
    return deviceStates.value[deviceId] || null
  }

  return {
    devices,
    deviceStates,
    loading,
    error,
    onlineCount,
    offlineCount,
    fetchDevices,
    createDevice,
    updateDevice,
    deleteDevice,
    updateDeviceState,
    setAllStates,
    getDeviceState,
  }
})
