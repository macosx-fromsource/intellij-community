import { existsSync, readFileSync } from 'node:fs'
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

/**
 * The chart versions the Operator carries, read from CHART_VERSIONS at build
 * time and baked into the bundle. The file is the same source the image uses to
 * fetch the charts, and the SPA is built in that image, so the two agree.
 *
 * Two candidate paths: the repository root for a build from a checkout, and the
 * working directory for the Docker webbuilder stage, which copies the file next
 * to the SPA. A missing file fails the build rather than shipping an empty
 * dropdown.
 */
function chartVersions(): string[] {
  const candidates = [
    fileURLToPath(new URL('../../../CHART_VERSIONS', import.meta.url)),
    fileURLToPath(new URL('./CHART_VERSIONS', import.meta.url)),
  ]

  const path = candidates.find((candidate) => existsSync(candidate))
  if (!path) {
    throw new Error(`CHART_VERSIONS not found; looked in ${candidates.join(', ')}`)
  }

  return readFileSync(path, 'utf8')
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '' && !line.startsWith('#'))
}

// https://vite.dev/config/
export default defineConfig({
  base: '/',
  plugins: [vue(), vueDevTools()],
  define: {
    __CHART_VERSIONS__: JSON.stringify(chartVersions()),
  },
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
      // changeOrigin stays off so the Host header keeps naming the dev server the
      // browser actually asked for. The bridge's local serve mode (`kubectl
      // bridge`) rejects /api requests whose Origin does not match their Host, and
      // rewriting Host to the target while forwarding the browser's Origin would
      // look exactly like a cross-origin request. The target is a loopback address,
      // so no virtual-host trick needs it.
      proxyPaths.map((path) => [path, { target: bridgeTarget, changeOrigin: false }]),
    ),
  },
})
