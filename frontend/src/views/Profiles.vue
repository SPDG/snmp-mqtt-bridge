<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const profiles = ref([])
const loading = ref(true)
const selectedProfile = ref(null)

onMounted(async () => {
  try {
    profiles.value = await api.getProfiles()
  } finally {
    loading.value = false
  }
})

function viewProfile(profile) {
  selectedProfile.value = profile
}

function getCategoryIcon(category) {
  switch (category) {
    case 'ups': return '🔋'
    case 'ats': return '🔌'
    case 'pdu': return '⚡'
    default: return '📡'
  }
}
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold text-gray-900 mb-6">Device Profiles</h1>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Profile List -->
      <div class="card">
        <h2 class="text-lg font-semibold mb-4">Available Profiles</h2>

        <div v-if="loading" class="text-center py-4">Loading...</div>

        <div v-else class="space-y-2">
          <div
            v-for="profile in profiles"
            :key="profile.id"
            @click="viewProfile(profile)"
            class="p-4 border rounded-lg cursor-pointer hover:bg-gray-50 transition-colors"
            :class="selectedProfile?.id === profile.id ? 'border-blue-500 bg-blue-50' : 'border-gray-200'"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center">
                <span class="text-2xl mr-3">{{ getCategoryIcon(profile.category) }}</span>
                <div>
                  <p class="font-medium">{{ profile.name }}</p>
                  <p class="text-sm text-gray-500">{{ profile.manufacturer }} {{ profile.model }}</p>
                </div>
              </div>
              <span v-if="profile.is_builtin" class="text-xs px-2 py-1 bg-gray-100 rounded">Built-in</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Profile Details -->
      <div class="card">
        <h2 class="text-lg font-semibold mb-4">Profile Details</h2>

        <div v-if="!selectedProfile" class="text-gray-500 text-center py-8">
          Select a profile to view details
        </div>

        <div v-else>
          <dl class="space-y-3 mb-6">
            <div>
              <dt class="text-sm text-gray-500">ID</dt>
              <dd class="font-mono text-sm">{{ selectedProfile.id }}</dd>
            </div>
            <div>
              <dt class="text-sm text-gray-500">Category</dt>
              <dd class="capitalize">{{ selectedProfile.category }}</dd>
            </div>
            <div v-if="selectedProfile.sys_object_id">
              <dt class="text-sm text-gray-500">sysObjectID</dt>
              <dd class="font-mono text-sm">{{ selectedProfile.sys_object_id }}</dd>
            </div>
          </dl>

          <h3 class="font-medium mb-2">OID Mappings ({{ selectedProfile.oid_mappings?.length || 0 }})</h3>
          <div class="max-h-64 overflow-y-auto border rounded">
            <table class="min-w-full text-sm">
              <thead class="bg-gray-50 sticky top-0">
                <tr>
                  <th class="px-3 py-2 text-left">Name</th>
                  <th class="px-3 py-2 text-left">Type</th>
                  <th class="px-3 py-2 text-left">Component</th>
                </tr>
              </thead>
              <tbody class="divide-y">
                <tr v-for="mapping in selectedProfile.oid_mappings" :key="mapping.oid">
                  <td class="px-3 py-2">{{ mapping.name }}</td>
                  <td class="px-3 py-2 text-gray-500">{{ mapping.type }}</td>
                  <td class="px-3 py-2 text-gray-500">{{ mapping.ha_component }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
