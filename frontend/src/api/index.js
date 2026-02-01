const API_BASE = '/api'

async function request(method, path, data = null) {
  const options = {
    method,
    headers: {
      'Content-Type': 'application/json',
    },
  }

  if (data) {
    options.body = JSON.stringify(data)
  }

  const response = await fetch(`${API_BASE}${path}`, options)
  const json = await response.json()

  if (!response.ok) {
    throw new Error(json.error || 'Request failed')
  }

  return json.data
}

export const api = {
  // Devices
  getDevices: () => request('GET', '/devices'),
  getDevice: (id) => request('GET', `/devices/${id}`),
  createDevice: (data) => request('POST', '/devices', data),
  updateDevice: (id, data) => request('PUT', `/devices/${id}`, data),
  deleteDevice: (id) => request('DELETE', `/devices/${id}`),
  testConnection: (id) => request('POST', `/devices/${id}/test`),
  testNewConnection: (data) => request('POST', '/test-connection', data),
  getDeviceState: (id) => request('GET', `/devices/${id}/state`),

  // Device commands
  setDeviceValue: (id, oid, value) => request('POST', `/devices/${id}/set`, { oid, value }),
  getDeviceValue: (id, oid) => request('GET', `/devices/${id}/get?oid=${encodeURIComponent(oid)}`),
  // ATS commands
  switchSource: (id, source) => request('POST', `/devices/${id}/switch-source`, { source }),
  setSourceName: (id, source, name) => request('POST', `/devices/${id}/set-source-name`, { source, name }),
  // PDU commands
  setOutletState: (id, outlet, state) => request('POST', `/devices/${id}/outlet/state`, { outlet, state }),
  setOutletName: (id, outlet, name) => request('POST', `/devices/${id}/outlet/name`, { outlet, name }),
  rebootOutlet: (id, outlet) => request('POST', `/devices/${id}/outlet/reboot`, { outlet }),

  // Profiles
  getProfiles: () => request('GET', '/profiles'),
  getProfile: (id) => request('GET', `/profiles/${id}`),
  createProfile: (data) => request('POST', '/profiles', data),
  updateProfile: (id, data) => request('PUT', `/profiles/${id}`, data),
  deleteProfile: (id) => request('DELETE', `/profiles/${id}`),

  // Traps
  getTraps: (params = {}) => {
    const query = new URLSearchParams(params).toString()
    return request('GET', `/traps${query ? '?' + query : ''}`)
  },
  getTrap: (id) => request('GET', `/traps/${id}`),
  cleanupTraps: (days = 30) => request('DELETE', `/traps/cleanup?days=${days}`),

  // Settings
  getSettings: () => request('GET', '/settings'),
  getSetting: (key) => request('GET', `/settings/${key}`),
  setSetting: (key, value) => request('PUT', `/settings/${key}`, { value }),
  deleteSetting: (key) => request('DELETE', `/settings/${key}`),

  // MQTT
  getMQTTStatus: () => request('GET', '/mqtt/status'),
  reconnectMQTT: () => request('POST', '/mqtt/reconnect'),
  testMQTTConnection: (data) => request('POST', '/mqtt/test', data),
}

export default api
