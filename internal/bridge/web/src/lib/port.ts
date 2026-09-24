/**
 * The bounds the custom resource definitions put on a port field, repeated
 * here so a value out of range is caught before the request rather than
 * surfacing as a raw Kubernetes API error.
 */
const minPort = 1
const maxPort = 65535

/**
 * Reads a port field. Returns `undefined` when it is empty, which leaves the
 * resource to default it, or `null` when what it holds is not a port. It
 * reports rather than shows: the section owns its error message, so that the
 * field it belongs to is named.
 */
export function parsePort(raw: string): number | undefined | null {
  const port = raw.trim()

  if (!port) {
    return undefined
  }

  if (!/^\d+$/.test(port) || Number(port) < minPort || Number(port) > maxPort) {
    return null
  }

  return Number(port)
}

/** What a section shows when `parsePort` rejects the field named here. */
export function portError(label: string): string {
  return `The ${label} port must be a number between ${minPort} and ${maxPort}.`
}
