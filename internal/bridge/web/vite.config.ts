import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// The bridge server (default :8090) serves the API, the OpenAPI document, the
// docs UI, and the built SPA. During `vite dev` we proxy those paths to it so
// the SPA can be developed with HMR while talking to a real backend (run the
// operator locally with `task run`, or port-forward the in-cluster operator).
const bridgeTarget = process.env.BRIDGE_URL ?? 'http://localhost:8090'
const proxyPaths = ['/api', '/openapi.yaml', '/openapi.json', '/docs', '/schemas']

// https://vite.dev/config/
export default defineConfig({
  base: '/',
  plugins: [vue(), vueDevTools()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    // Must stay 'dist': the Go bridge embeds internal/bridge/web/dist via go:embed.
    outDir: 'dist',
    // Keep the committed dist/.gitkeep so a clean checkout still satisfies go:embed
    // before the SPA has been built. Docker builds dist fresh on an empty dir.
    emptyOutDir: false,
  },
  server: {
    proxy: Object.fromEntries(
      proxyPaths.map((path) => [path, { target: bridgeTarget, changeOrigin: true }]),
    ),
  },
})
