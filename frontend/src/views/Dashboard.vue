<script setup>
import { computed, onMounted, ref } from 'vue'
import { useDeviceStore } from '../stores/devices'
import api from '../api'

const deviceStore = useDeviceStore()
const profiles = ref([])

onMounted(async () => {
  try {
    profiles.value = await api.getProfiles()
  } catch {
    profiles.value = []
  }
})

const profileByID = computed(() => new Map(profiles.value.map(profile => [profile.id, profile])))

function toNumber(value) {
  if (value === null || value === undefined || value === '') return null
  const number = Number.parseFloat(value)
  return Number.isFinite(number) ? number : null
}

function firstNumber(values, names) {
  for (const name of names) {
    const number = toNumber(values?.[name])
    if (number !== null) return number
  }
  return null
}

function formatNumber(value, digits, unit) {
  return value === null ? `- ${unit}` : `${value.toFixed(digits)} ${unit}`
}

function outletNumbers(values, profile) {
  const numbers = new Set()
  const pattern = /^Outlet (\d+) (?:Name|State|Current|Power|Energy|Voltage)$/
  for (const name of Object.keys(values || {})) {
    const match = name.match(pattern)
    if (match) numbers.add(Number(match[1]))
  }
  for (const mapping of profile?.oid_mappings || []) {
    const match = mapping.name?.match(pattern)
    if (match) numbers.add(Number(match[1]))
  }
  return [...numbers].sort((a, b) => a - b)
}

function outletIsOn(value, profile) {
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['on', 'true', 'enabled'].includes(normalized)) return true
    if (['off', 'false', 'disabled'].includes(normalized)) return false
  }
  const numeric = Number(value)
  if (profile?.manufacturer === 'ATEN') return numeric === 2
  return numeric === 1
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

function cleanText(value, fallback = '-') {
  const text = String(value ?? '').replace(/\0/g, '').trim()
  return text || fallback
}

function buildDeviceView(device) {
  const state = deviceStore.getDeviceState(device.id)
  const values = state?.values || {}
  const profile = profileByID.value.get(device.profile_id)
  const category = profile?.category || 'device'
  const outlets = outletNumbers(values, profile).map(number => ({
    number,
    name: cleanText(values[`Outlet ${number} Name`], `Outlet ${number}`),
    on: outletIsOn(values[`Outlet ${number} State`], profile),
  }))
  const outletCurrent = sumOutletMetric(values, outlets.map(outlet => outlet.number), 'Current')
  const outletPower = sumOutletMetric(values, outlets.map(outlet => outlet.number), 'Power')
  const outletEnergy = sumOutletMetric(values, outlets.map(outlet => outlet.number), 'Energy')
  const voltage = firstNumber(values, ['Voltage', 'Output Voltage', 'Input Voltage'])
  const current = firstNumber(values, ['Total Current', 'Output Current']) ?? outletCurrent
  const power = firstNumber(values, ['Active Power', 'Output Active Power', 'Output Apparent Power'])
    ?? outletPower
    ?? (voltage !== null && current !== null ? voltage * current : null)
  const energy = firstNumber(values, ['Total Energy', 'Aggregate Energy']) ?? outletEnergy
  const sourceAName = cleanText(values['Source A Name'], 'Source A')
  const sourceBName = cleanText(values['Source B Name'], 'Source B')
  const selectedSource = values['Selected Source']
  const activeSource = selectedSource === 1 || selectedSource === '1' || selectedSource === 'Source A' || selectedSource === sourceAName
    ? sourceAName
    : selectedSource === 2 || selectedSource === '2' || selectedSource === 'Source B' || selectedSource === sourceBName
      ? sourceBName
      : cleanText(selectedSource, 'Unknown')

  let metrics = []
  if (category === 'pdu') {
    metrics = [
      { label: 'Current', value: formatNumber(current, 2, 'A') },
      { label: 'Power', value: formatNumber(power, 1, 'W') },
      { label: 'Energy', value: formatNumber(energy, 3, 'kWh') },
    ]
  } else if (category === 'ats') {
    metrics = [
      { label: 'Active source', value: activeSource },
      { label: 'Output', value: formatNumber(firstNumber(values, ['Output Voltage']), 1, 'V') },
      { label: 'Load', value: formatNumber(firstNumber(values, ['Output Current']), 2, 'A') },
    ]
  } else {
    metrics = [
      { label: 'Input', value: formatNumber(firstNumber(values, ['Input Voltage', 'Voltage']), 1, 'V') },
      { label: 'Load', value: formatNumber(firstNumber(values, ['Output Load', 'Load']), 0, '%') },
      { label: 'Battery', value: formatNumber(firstNumber(values, ['Battery Charge', 'Battery Capacity']), 0, '%') },
    ]
  }

  return {
    ...device,
    state,
    profile,
    category,
    outlets,
    metrics,
    current,
    power,
    errors: state?.errors || [],
  }
}

const deviceViews = computed(() => deviceStore.devices.map(buildDeviceView))
const onlinePercent = computed(() => {
  if (!deviceStore.enabledCount) return 0
  return Math.round((deviceStore.onlineCount / deviceStore.enabledCount) * 100)
})
const managedOutlets = computed(() => deviceViews.value.flatMap(device => device.outlets))
const outletsOn = computed(() => managedOutlets.value.filter(outlet => outlet.on).length)
const aggregateCurrent = computed(() => deviceViews.value.reduce((total, device) => total + (device.current || 0), 0))
const aggregatePower = computed(() => deviceViews.value.reduce((total, device) => total + (device.power || 0), 0))

function formatLastPoll(value) {
  if (!value) return 'Waiting for first poll'
  return new Intl.DateTimeFormat(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(new Date(value))
}
</script>

<template>
  <div class="dashboard-view">
    <section class="dashboard-intro">
      <div>
        <p class="eyebrow">Infrastructure overview</p>
        <h2>Power devices at a glance</h2>
        <p>Live SNMP state across PDUs, transfer switches and power equipment.</p>
      </div>
      <RouterLink to="/devices" class="btn btn-primary">Manage devices</RouterLink>
    </section>

    <section class="overview-grid" aria-label="Infrastructure summary">
      <article class="overview-card availability">
        <div class="overview-card-head">
          <span>Availability</span>
          <strong>{{ onlinePercent }}%</strong>
        </div>
        <div class="availability-track"><span :style="{ width: `${onlinePercent}%` }"></span></div>
        <p>{{ deviceStore.onlineCount }} online <span>/ {{ deviceStore.enabledCount }} enabled</span></p>
      </article>

      <article class="overview-card">
        <span class="overview-label">Managed outlets</span>
        <strong>{{ outletsOn }}<small>/{{ managedOutlets.length }}</small></strong>
        <p>Currently powered on</p>
      </article>

      <article class="overview-card">
        <span class="overview-label">Aggregate current</span>
        <strong>{{ aggregateCurrent.toFixed(2) }}<small>A</small></strong>
        <p>Reported by online devices</p>
      </article>

      <article class="overview-card">
        <span class="overview-label">Active power</span>
        <strong>{{ aggregatePower.toFixed(1) }}<small>W</small></strong>
        <p>Measured or calculated load</p>
      </article>
    </section>

    <section class="fleet-panel">
      <div class="fleet-heading">
        <div>
          <p class="eyebrow">Live inventory</p>
          <h2>Managed infrastructure</h2>
        </div>
        <span>{{ deviceViews.length }} devices</span>
      </div>

      <div v-if="deviceStore.loading" class="dashboard-empty">Loading device state...</div>
      <div v-else-if="deviceViews.length === 0" class="dashboard-empty">
        <p>No devices configured.</p>
        <RouterLink to="/devices">Add your first SNMP device</RouterLink>
      </div>

      <div v-else class="device-stack">
        <RouterLink
          v-for="device in deviceViews"
          :key="device.id"
          :to="`/devices/${device.id}`"
          class="device-row"
        >
          <div class="device-identity">
            <span class="device-type" :class="device.category">{{ device.category.toUpperCase() }}</span>
            <div class="min-w-0">
              <div class="device-name-line">
                <h3>{{ device.name }}</h3>
                <span :class="device.state?.online ? 'status-online' : 'status-offline'">
                  {{ device.state?.online ? 'Online' : 'Offline' }}
                </span>
              </div>
              <p>{{ device.ip_address }}:{{ device.port }} · {{ device.profile?.model || device.profile_id || 'Generic SNMP' }}</p>
            </div>
          </div>

          <div class="device-metrics">
            <div v-for="metric in device.metrics" :key="metric.label">
              <span>{{ metric.label }}</span>
              <strong>{{ device.state?.online ? metric.value : '-' }}</strong>
            </div>
          </div>

          <div v-if="device.category === 'pdu'" class="outlet-summary">
            <span class="outlet-summary-label">Outlets</span>
            <div class="outlet-dots">
              <span
                v-for="outlet in device.outlets"
                :key="outlet.number"
                :class="{ on: outlet.on }"
                :title="`${outlet.number}. ${outlet.name}: ${outlet.on ? 'On' : 'Off'}`"
              >{{ outlet.number }}</span>
              <em v-if="device.outlets.length === 0">No outlet data</em>
            </div>
          </div>
          <div v-else class="device-poll">
            <span>Last update</span>
            <strong>{{ formatLastPoll(device.state?.last_poll) }}</strong>
          </div>

          <svg class="device-chevron" viewBox="0 0 20 20" aria-hidden="true">
            <path d="m7.5 4 6 6-6 6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </RouterLink>
      </div>
    </section>
  </div>
</template>

<style scoped>
.dashboard-view {
  display: grid;
  gap: 22px;
}

.dashboard-intro,
.fleet-heading,
.overview-card-head,
.device-name-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.dashboard-intro h2,
.fleet-heading h2 {
  margin: 2px 0 0;
  color: var(--ui-text);
  font-size: 24px;
  letter-spacing: -0.035em;
}

.dashboard-intro > div > p:last-child {
  margin: 5px 0 0;
  color: var(--ui-muted);
  font-size: 14px;
}

.eyebrow {
  margin: 0;
  color: var(--ui-accent);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.overview-grid {
  display: grid;
  grid-template-columns: 1.25fr repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.overview-card,
.fleet-panel {
  border: 1px solid var(--ui-border);
  background: color-mix(in srgb, var(--ui-panel) 94%, transparent);
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.055);
}

.overview-card {
  min-height: 132px;
  padding: 19px 20px;
  border-radius: 12px;
}

.overview-card > strong,
.overview-card-head strong {
  display: block;
  margin-top: 12px;
  color: var(--ui-text);
  font-size: 31px;
  line-height: 1;
  letter-spacing: -0.05em;
}

.overview-card strong small {
  margin-left: 5px;
  color: var(--ui-muted);
  font-size: 15px;
  font-weight: 700;
}

.overview-card p {
  margin: 11px 0 0;
  color: var(--ui-muted);
  font-size: 12px;
}

.overview-card p span {
  color: color-mix(in srgb, var(--ui-muted) 70%, transparent);
}

.overview-label,
.overview-card-head > span {
  color: var(--ui-muted);
  font-size: 12px;
  font-weight: 700;
}

.overview-card-head strong {
  margin: 0;
  color: var(--ui-success);
  font-size: 19px;
}

.availability-track {
  height: 7px;
  margin-top: 21px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--ui-bg-soft);
}

.availability-track span {
  height: 100%;
  display: block;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--ui-accent), var(--ui-success));
  transition: width 400ms ease;
}

.fleet-panel {
  overflow: hidden;
  border-radius: 13px;
}

.fleet-heading {
  padding: 18px 20px;
  border-bottom: 1px solid var(--ui-border);
}

.fleet-heading h2 {
  font-size: 18px;
}

.fleet-heading > span {
  color: var(--ui-muted);
  font-size: 12px;
  font-weight: 700;
}

.device-stack {
  display: grid;
}

.device-row {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(250px, 1.35fr) minmax(300px, 1.2fr) minmax(210px, 0.85fr) 18px;
  align-items: center;
  gap: 24px;
  padding: 18px 20px;
  color: inherit;
  text-decoration: none;
  border-bottom: 1px solid var(--ui-border);
  transition: background 150ms ease;
}

.device-row:last-child {
  border-bottom: 0;
}

.device-row:hover {
  background: var(--ui-panel-strong);
}

.device-identity {
  min-width: 0;
  display: grid;
  grid-template-columns: 45px minmax(0, 1fr);
  align-items: center;
  gap: 13px;
}

.device-type {
  width: 45px;
  height: 45px;
  display: grid;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--ui-accent) 35%, var(--ui-border));
  border-radius: 11px;
  color: var(--ui-accent);
  background: var(--ui-accent-soft);
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 0.04em;
}

.device-type.ats {
  color: var(--ui-warning);
  border-color: color-mix(in srgb, var(--ui-warning) 38%, var(--ui-border));
  background: color-mix(in srgb, var(--ui-warning) 11%, var(--ui-panel));
}

.device-name-line {
  justify-content: flex-start;
  gap: 8px;
}

.device-name-line h3 {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--ui-text);
  font-size: 14px;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-identity p {
  margin: 4px 0 0;
  overflow: hidden;
  color: var(--ui-muted);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.device-metrics span,
.device-poll span,
.outlet-summary-label {
  display: block;
  color: var(--ui-muted);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.device-metrics strong,
.device-poll strong {
  display: block;
  margin-top: 4px;
  overflow: hidden;
  color: var(--ui-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.outlet-dots {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 7px;
}

.outlet-dots span {
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  border: 1px solid var(--ui-border-strong);
  border-radius: 7px;
  color: var(--ui-muted);
  background: var(--ui-bg-soft);
  font-size: 10px;
  font-weight: 800;
}

.outlet-dots span.on {
  color: var(--ui-success);
  border-color: color-mix(in srgb, var(--ui-success) 42%, var(--ui-border));
  background: var(--ui-success-soft);
  box-shadow: inset 0 -2px 0 color-mix(in srgb, var(--ui-success) 30%, transparent);
}

.outlet-dots em {
  color: var(--ui-muted);
  font-size: 11px;
  font-style: normal;
}

.device-chevron {
  width: 18px;
  color: var(--ui-muted);
}

.dashboard-empty {
  padding: 54px 20px;
  color: var(--ui-muted);
  text-align: center;
}

.dashboard-empty p {
  margin: 0 0 7px;
}

.dashboard-empty a {
  color: var(--ui-accent);
  font-weight: 700;
}

@media (max-width: 1180px) {
  .overview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .device-row {
    grid-template-columns: minmax(250px, 1.2fr) minmax(280px, 1fr) minmax(180px, 0.75fr) 18px;
    gap: 16px;
  }
}

@media (max-width: 820px) {
  .dashboard-intro {
    align-items: flex-start;
  }

  .dashboard-intro h2 {
    font-size: 21px;
  }

  .device-row {
    grid-template-columns: minmax(0, 1fr) 18px;
  }

  .device-metrics,
  .outlet-summary,
  .device-poll {
    grid-column: 1;
  }

  .device-chevron {
    grid-column: 2;
    grid-row: 1;
  }
}

@media (max-width: 560px) {
  .dashboard-intro {
    display: grid;
  }

  .dashboard-intro .btn {
    width: max-content;
  }

  .overview-grid {
    grid-template-columns: 1fr;
  }

  .overview-card {
    min-height: 112px;
  }
}
</style>
