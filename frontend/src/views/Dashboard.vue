<script setup>
import { computed } from 'vue'
import { useDeviceStore } from '../stores/devices'

const deviceStore = useDeviceStore()

const stats = computed(() => [
  { label: 'Total Devices', value: deviceStore.devices.length, icon: '📡' },
  { label: 'Online', value: deviceStore.onlineCount, icon: '✅', color: 'text-green-600' },
  { label: 'Offline', value: deviceStore.offlineCount, icon: '❌', color: 'text-red-600' },
])

const recentDevices = computed(() => {
  return deviceStore.devices
    .map(d => ({
      ...d,
      state: deviceStore.getDeviceState(d.id)
    }))
    .slice(0, 5)
})
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold text-gray-900 mb-6">Dashboard</h1>

    <!-- Stats -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
      <div v-for="stat in stats" :key="stat.label" class="card">
        <div class="flex items-center">
          <span class="text-3xl mr-4">{{ stat.icon }}</span>
          <div>
            <p class="text-sm text-gray-500">{{ stat.label }}</p>
            <p class="text-2xl font-bold" :class="stat.color || 'text-gray-900'">{{ stat.value }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Recent Devices -->
    <div class="card">
      <h2 class="text-lg font-semibold mb-4">Recent Devices</h2>

      <div v-if="deviceStore.loading" class="text-center py-4">
        Loading...
      </div>

      <div v-else-if="recentDevices.length === 0" class="text-center py-4 text-gray-500">
        No devices configured. <RouterLink to="/devices" class="text-blue-600 hover:underline">Add your first device</RouterLink>
      </div>

      <table v-else class="min-w-full divide-y divide-gray-200">
        <thead>
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">IP Address</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Seen</th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr v-for="device in recentDevices" :key="device.id">
            <td class="px-6 py-4 whitespace-nowrap">
              <RouterLink :to="`/devices/${device.id}`" class="text-blue-600 hover:underline">
                {{ device.name }}
              </RouterLink>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-gray-500">{{ device.ip_address }}</td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span :class="device.state?.online ? 'status-online' : 'status-offline'">
                {{ device.state?.online ? 'Online' : 'Offline' }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-gray-500">
              {{ device.last_seen ? new Date(device.last_seen).toLocaleString() : 'Never' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
