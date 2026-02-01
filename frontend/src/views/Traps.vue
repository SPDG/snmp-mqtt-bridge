<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const traps = ref([])
const total = ref(0)
const loading = ref(true)
const page = ref(1)
const limit = 20

async function fetchTraps() {
  loading.value = true
  try {
    const offset = (page.value - 1) * limit
    const result = await api.getTraps({ limit, offset })
    traps.value = result
    // Note: API returns meta.total but our simple api wrapper doesn't handle it
  } finally {
    loading.value = false
  }
}

onMounted(fetchTraps)

async function cleanup() {
  if (confirm('Delete trap logs older than 30 days?')) {
    try {
      const result = await api.cleanupTraps(30)
      alert(`Deleted ${result.deleted} records`)
      fetchTraps()
    } catch (e) {
      alert('Error: ' + e.message)
    }
  }
}

function getSeverityClass(severity) {
  switch (severity) {
    case 'critical': return 'bg-red-100 text-red-800'
    case 'error': return 'bg-red-100 text-red-800'
    case 'warning': return 'bg-yellow-100 text-yellow-800'
    default: return 'bg-blue-100 text-blue-800'
  }
}
</script>

<template>
  <div>
    <div class="flex justify-between items-center mb-6">
      <h1 class="text-2xl font-bold text-gray-900">SNMP Trap Logs</h1>
      <button @click="cleanup" class="btn btn-secondary">Cleanup Old Logs</button>
    </div>

    <div class="card">
      <div v-if="loading" class="text-center py-4">Loading...</div>

      <div v-else-if="traps.length === 0" class="text-center py-8 text-gray-500">
        No trap logs recorded yet.
      </div>

      <div v-else>
        <table class="min-w-full divide-y divide-gray-200">
          <thead>
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Time</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Source</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Severity</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Message</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            <tr v-for="trap in traps" :key="trap.id">
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ new Date(trap.received_at).toLocaleString() }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm">
                {{ trap.source_ip }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="px-2 py-1 text-xs rounded-full" :class="getSeverityClass(trap.severity)">
                  {{ trap.severity }}
                </span>
              </td>
              <td class="px-6 py-4 text-sm">
                {{ trap.message || trap.trap_oid }}
              </td>
            </tr>
          </tbody>
        </table>

        <div class="mt-4 flex justify-between items-center">
          <button
            @click="page--; fetchTraps()"
            :disabled="page === 1"
            class="btn btn-secondary"
          >
            Previous
          </button>
          <span class="text-gray-500">Page {{ page }}</span>
          <button
            @click="page++; fetchTraps()"
            :disabled="traps.length < limit"
            class="btn btn-secondary"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
