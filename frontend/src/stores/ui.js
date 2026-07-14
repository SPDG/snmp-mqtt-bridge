import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '../api'

export const useUIStore = defineStore('ui', () => {
  const safeMode = ref(true)
  const mqttStatus = ref({
    connected: false,
    broker: '',
    port: null,
    loading: true,
    reachable: true,
  })
  const confirmation = ref(null)
  let confirmationResolve = null

  function initialize() {
    safeMode.value = localStorage.getItem('safeMode') !== 'false'
  }

  function toggleSafeMode() {
    safeMode.value = !safeMode.value
    localStorage.setItem('safeMode', String(safeMode.value))
  }

  async function refreshMQTTStatus() {
    try {
      const status = await api.getMQTTStatus()
      mqttStatus.value = {
        connected: Boolean(status.connected),
        broker: status.broker || '',
        port: status.port || null,
        loading: false,
        reachable: true,
      }
    } catch {
      mqttStatus.value = {
        ...mqttStatus.value,
        connected: false,
        loading: false,
        reachable: false,
      }
    }
  }

  function confirmAction(options) {
    if (!safeMode.value) return Promise.resolve(true)

    if (confirmationResolve) confirmationResolve(false)
    confirmation.value = {
      title: options.title || 'Confirm action',
      message: options.message || 'Do you want to continue?',
      confirmLabel: options.confirmLabel || 'Continue',
      tone: options.tone || 'warning',
    }

    return new Promise(resolve => {
      confirmationResolve = resolve
    })
  }

  function resolveConfirmation(result) {
    if (confirmationResolve) confirmationResolve(result)
    confirmationResolve = null
    confirmation.value = null
  }

  return {
    safeMode,
    mqttStatus,
    confirmation,
    initialize,
    toggleSafeMode,
    refreshMQTTStatus,
    confirmAction,
    resolveConfirmation,
  }
})
