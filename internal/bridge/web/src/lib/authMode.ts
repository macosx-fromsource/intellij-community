/**
 * Whether the SPA is served by the local `kubectl bridge` plugin. That plugin
 * runs the bridge on the user's machine and authenticates every request via the
 * caller's own kubeconfig, so no bearer token is needed and the token-entry UI
 * (and any future login gate) should be hidden. The local server signals this
 * by injecting `window.__BRIDGE_AUTH__ = "local"` into index.html.
 */
export function isLocalAuthMode(): boolean {
  return (window as unknown as { __BRIDGE_AUTH__?: string }).__BRIDGE_AUTH__ === 'local'
}
