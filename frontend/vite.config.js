import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { readFileSync } from 'node:fs'

const addonManifest = readFileSync(new URL('../snmp-mqtt-bridge/config.yaml', import.meta.url), 'utf8')
const addonVersion = addonManifest.match(/^version:\s*["']?([^"'\s]+)["']?/m)?.[1] || 'dev'

export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(addonVersion),
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  base: './',  // Use relative paths for HA Ingress compatibility
})
