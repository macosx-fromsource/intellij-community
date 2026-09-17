import type { Condition, GitLabResource } from '@/lib/api/client'

/**
 * A single gate can be settled (`done`), in flight (`active`), not yet reached
 * (`pending`), or the one an upgrade died on (`failed`).
 */
export type GateState = 'done' | 'active' | 'pending' | 'failed'

/** One gate of the zero-downtime upgrade choreography. */
export interface UpgradeGate {
  key: string
  title: string
  state: GateState
}

/**
 * The view model the UI renders for an upgrade, derived entirely from
 * `status.conditions` and the deployed-vs-requested chart versions. There is no
 * dedicated status field for any of this; the operator reports progress through
 * the `Progressing` condition's reason (which gate) and the condition messages
 * (which intermediate version), so this module reads them back.
 */
export interface UpgradeView {
  /** True while a zero-downtime upgrade is actively in flight. */
  inProgress: boolean
  /** True once the upgrade reached a terminal failure. */
  failed: boolean
  /** True when the requested upgrade path cannot be carried out at all. */
  blocked: boolean
  /** Chart version currently deployed (`status.version`). */
  currentVersion: string
  /** Final chart version requested (`spec.chart.version`). */
  targetVersion: string
  /**
   * Intermediate chart version this cycle converges to. On a multi-minor upgrade
   * the operator steps through the minors one at a time; this is the in-flight
   * hop. Empty, or equal to the target, on a single-minor upgrade.
   */
  intermediateVersion: string
  /** Ordered gates with their per-gate state. */
  gates: UpgradeGate[]
  /** Human-readable status line from the active or terminal condition. */
  message: string
}

/**
 * The five gates of the zero-downtime upgrade, in the order the reconciler walks
 * them (internal/controller/gitlabcore/upgrade.go). Each maps to the
 * `Progressing` condition reason the reconciler sets while that gate is active.
 */
const GATES: { key: string; title: string; reason: string }[] = [
  { key: 'pre-migrations', title: 'Pre-deployment migrations', reason: 'RunningPreMigrations' },
  { key: 'rollout', title: 'Rolling out new version', reason: 'UpgradingRails' },
  { key: 'post-migrations', title: 'Post-deployment migrations', reason: 'RunningPostMigrations' },
  { key: 'finish-rollout', title: 'Finishing rollout', reason: 'RollingOutWorkloads' },
  {
    key: 'background-migrations',
    title: 'Background migrations',
    reason: 'WaitingForBatchedMigrations',
  },
]

/** Reasons the operator sets on `Progressing` once a step has converged. */
const DONE_REASONS = new Set(['AdvancingVersion', 'UpgradeComplete'])

/** Terminal `Progressing` reasons that fail the upgrade. */
const FAILURE_REASONS = new Set(['MigrationsJobFailed', 'BatchedMigrationsCheckFailed'])

function findCondition(item: GitLabResource, type: string): Condition | null {
  return item.status?.conditions?.find((c) => c.type === type) ?? null
}

/** True while the instance reports itself mid-upgrade. */
export function isUpgrading(item: GitLabResource): boolean {
  return item.status?.phase === 'Upgrading'
}

/**
 * Which gate a terminal failure died on, or -1 when it cannot be attributed. A
 * batched-migrations check failure is always the last gate. A migrations Job
 * failure is either the pre or the post gate; the message names the Job, and the
 * pre-migrations Job carries a "-pre" suffix, so it tells the two apart.
 */
function failedGateIndex(reason: string, message: string): number {
  if (reason === 'BatchedMigrationsCheckFailed') {
    return GATES.findIndex((g) => g.key === 'background-migrations')
  }

  if (reason === 'MigrationsJobFailed') {
    return /-pre"/.test(message)
      ? GATES.findIndex((g) => g.key === 'pre-migrations')
      : GATES.findIndex((g) => g.key === 'post-migrations')
  }

  return -1
}

function buildGates(reason: string, message: string): UpgradeGate[] {
  const failedIndex = failedGateIndex(reason, message)
  const activeIndex = GATES.findIndex((g) => g.reason === reason)
  const allDone = DONE_REASONS.has(reason)

  return GATES.map((gate, index) => {
    let state: GateState = 'pending'

    if (failedIndex >= 0) {
      if (index < failedIndex) {
        state = 'done'
      } else if (index === failedIndex) {
        state = 'failed'
      }
    } else if (allDone) {
      state = 'done'
    } else if (activeIndex >= 0) {
      if (index < activeIndex) {
        state = 'done'
      } else if (index === activeIndex) {
        state = 'active'
      }
    }

    return { key: gate.key, title: gate.title, state }
  })
}

/**
 * Pulls the intermediate chart version out of the condition messages. The
 * `Upgradeable` condition spells it out ("upgrading toward X, next Y"); failing
 * that, the `Progressing` messages embed the in-flight version as a semver.
 */
function intermediateVersion(upgradeable: Condition | null, progressing: Condition | null): string {
  const next = upgradeable?.message?.match(/next\s+(\S+)/)
  if (next && next[1]) {
    return next[1]
  }

  const semver = progressing?.message?.match(/\b\d+\.\d+\.\d+\b/)
  if (semver && semver[0]) {
    return semver[0]
  }

  return ''
}

/**
 * Derives the upgrade view model for an instance, or null when there is no
 * upgrade to visualize (a fresh install, or an instance settled at its target).
 */
export function deriveUpgrade(item: GitLabResource): UpgradeView | null {
  const status = item.status
  if (!status) {
    return null
  }

  const progressing = findCondition(item, 'Progressing')
  const upgradeable = findCondition(item, 'Upgradeable')

  const reason = progressing?.reason ?? ''
  const progressingMessage = progressing?.message ?? ''

  const blocked = upgradeable?.status === 'False'
  const inProgress = status.phase === 'Upgrading'
  const failed = status.phase === 'Failed' && (FAILURE_REASONS.has(reason) || blocked)

  // Nothing to show unless an upgrade is in flight or has just failed.
  if (!inProgress && !failed) {
    return null
  }

  const message = blocked
    ? (upgradeable?.message ?? progressingMessage)
    : progressingMessage

  return {
    inProgress,
    failed,
    blocked,
    currentVersion: status.version ?? '',
    targetVersion: item.chart?.version ?? '',
    intermediateVersion: intermediateVersion(upgradeable, progressing),
    gates: buildGates(reason, progressingMessage),
    message,
  }
}
