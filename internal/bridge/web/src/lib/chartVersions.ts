/**
 * The GitLab Chart versions the Operator carries, latest first.
 *
 * These are fixed when the SPA is built: Vite reads the CHART_VERSIONS file of
 * the repository and replaces `__CHART_VERSIONS__` with its contents (see
 * vite.config.ts). The operator image builds the SPA from the same file it
 * fetches its charts with, so the list cannot drift from what the image carries,
 * and the form needs no request — and therefore no token — to offer it.
 *
 * A version outside this list stays reachable by typing one, for an image whose
 * charts were changed after the SPA was built.
 */
export const chartVersions: readonly string[] = __CHART_VERSIONS__

/** The version a new instance takes: the latest the Operator carries. */
export const latestChartVersion: string | undefined = chartVersions[0]
