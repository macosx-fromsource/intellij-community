/**
 * Compares two dotted numeric chart versions (e.g. "10.2.6"). Returns a negative
 * number when a < b, positive when a > b, and 0 when equal. Missing components
 * are treated as 0, and any non-numeric tail of a component is ignored, which is
 * enough for the bundled `major.minor.patch` versions.
 */
export function compareVersions(a: string, b: string): number {
  const pa = a.split('.').map((n) => parseInt(n, 10) || 0)
  const pb = b.split('.').map((n) => parseInt(n, 10) || 0)

  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const x = pa[i] ?? 0
    const y = pb[i] ?? 0

    if (x !== y) {
      return x - y
    }
  }

  return 0
}

/**
 * Returns the versions in `all` strictly greater than `current`, newest first.
 * An empty `current` yields no candidates, so an instance with no known version
 * offers no upgrade.
 */
export function higherVersions(current: string, all: string[]): string[] {
  if (!current) {
    return []
  }

  return all.filter((v) => compareVersions(v, current) > 0).sort((a, b) => compareVersions(b, a))
}

/**
 * The sequence of chart versions an upgrade converges through, from just above
 * `current` up to and including `target`, one per minor (the highest patch of
 * each minor available). This mirrors how the reconciler steps a multi-minor
 * upgrade one minor at a time. Returns [] when `target` is not above `current`;
 * when the catalog is empty it still ends on `target`.
 */
export function upgradePath(current: string, target: string, available: string[]): string[] {
  if (!current || !target || compareVersions(target, current) <= 0) {
    return []
  }

  const highestByMinor = new Map<string, string>()

  for (const v of available) {
    if (compareVersions(v, current) <= 0 || compareVersions(v, target) > 0) {
      continue
    }

    const parts = v.split('.')
    const key = `${parts[0]}.${parts[1]}`
    const existing = highestByMinor.get(key)

    if (!existing || compareVersions(v, existing) > 0) {
      highestByMinor.set(key, v)
    }
  }

  const hops = [...highestByMinor.values()]

  // Always finish exactly on the target, even when the catalog does not carry it.
  if (!hops.some((v) => compareVersions(v, target) === 0)) {
    hops.push(target)
  }

  return hops.sort(compareVersions)
}
