import { describe, expect, it } from 'vitest'

import type { Condition, GitLabResource } from '@/lib/api/client'
import { deriveUpgrade, isUpgrading } from './upgrade'

function condition(type: string, status: string, reason: string, message: string): Condition {
  return { type, status, reason, message, lastTransitionTime: '2026-01-01T00:00:00Z' }
}

function resource(opts: {
  phase?: string
  version?: string
  targetVersion?: string
  conditions?: Condition[]
}): GitLabResource {
  return {
    name: 'demo',
    namespace: 'ns',
    chart: { version: opts.targetVersion ?? '10.3.0', values: {} },
    status: {
      phase: opts.phase,
      version: opts.version,
      conditions: opts.conditions ?? [],
    },
  } as unknown as GitLabResource
}

/** Gate keys in the order the reconciler walks them. */
const GATE_KEYS = [
  'pre-migrations',
  'rollout',
  'post-migrations',
  'finish-rollout',
  'background-migrations',
]

function statesByKey(view: NonNullable<ReturnType<typeof deriveUpgrade>>) {
  return Object.fromEntries(view.gates.map((g) => [g.key, g.state]))
}

describe('deriveUpgrade', () => {
  it('returns null when there is no status', () => {
    const item = { name: 'x', namespace: 'ns', chart: { version: '10.0.0' } } as GitLabResource
    expect(deriveUpgrade(item)).toBeNull()
  })

  it('returns null for an instance settled at its target', () => {
    const item = resource({
      phase: 'Running',
      version: '10.3.0',
      conditions: [
        condition('Progressing', 'False', 'UpgradeComplete', 'upgraded to 10.3.0'),
      ],
    })
    expect(deriveUpgrade(item)).toBeNull()
  })

  it('marks the pre-migrations gate active at the start of an upgrade', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Upgrading',
        version: '10.2.0',
        conditions: [
          condition('Progressing', 'True', 'RunningPreMigrations', 'running the pre-deployment migrations'),
        ],
      }),
    )!

    expect(view.inProgress).toBe(true)
    expect(view.failed).toBe(false)
    expect(statesByKey(view)).toEqual({
      'pre-migrations': 'active',
      rollout: 'pending',
      'post-migrations': 'pending',
      'finish-rollout': 'pending',
      'background-migrations': 'pending',
    })
  })

  it('marks earlier gates done and later ones pending mid-upgrade', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Upgrading',
        version: '10.2.0',
        conditions: [
          condition('Progressing', 'True', 'RunningPostMigrations', 'running the post-deployment migrations of 10.3.0'),
        ],
      }),
    )!

    const states = statesByKey(view)
    expect(states['pre-migrations']).toBe('done')
    expect(states['rollout']).toBe('done')
    expect(states['post-migrations']).toBe('active')
    expect(states['finish-rollout']).toBe('pending')
    expect(states['background-migrations']).toBe('pending')
  })

  it('marks all gates done while advancing between minor versions', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Upgrading',
        version: '10.1.0',
        conditions: [
          condition('Progressing', 'True', 'AdvancingVersion', 'upgraded to 10.1.0, continuing toward 10.3.0'),
        ],
      }),
    )!

    for (const key of GATE_KEYS) {
      expect(statesByKey(view)[key]).toBe('done')
    }
  })

  it('reads the intermediate version from the Upgradeable condition', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Upgrading',
        version: '10.1.0',
        targetVersion: '10.3.0',
        conditions: [
          condition('Upgradeable', 'True', 'UpgradePathValid', 'upgrading toward 10.3.0, next 10.2.0'),
          condition('Progressing', 'True', 'RunningPreMigrations', 'running the pre-deployment migrations'),
        ],
      }),
    )!

    expect(view.currentVersion).toBe('10.1.0')
    expect(view.targetVersion).toBe('10.3.0')
    expect(view.intermediateVersion).toBe('10.2.0')
  })

  it('falls back to a semver in the Progressing message for the intermediate version', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Upgrading',
        version: '10.2.0',
        conditions: [
          condition('Progressing', 'True', 'RunningPostMigrations', 'running the post-deployment migrations of 10.3.0'),
        ],
      }),
    )!

    expect(view.intermediateVersion).toBe('10.3.0')
  })

  it('attributes a failed full migrations Job to the post-migrations gate', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Failed',
        version: '10.2.0',
        conditions: [
          condition('Progressing', 'False', 'MigrationsJobFailed', 'the migrations Job "demo-abcd1234" failed; inspect its pods'),
        ],
      }),
    )!

    expect(view.failed).toBe(true)
    const states = statesByKey(view)
    expect(states['pre-migrations']).toBe('done')
    expect(states['rollout']).toBe('done')
    expect(states['post-migrations']).toBe('failed')
  })

  it('attributes a failed pre-migrations Job to the pre-migrations gate', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Failed',
        version: '10.2.0',
        conditions: [
          condition('Progressing', 'False', 'MigrationsJobFailed', 'the migrations Job "demo-abcd1234-pre" failed; inspect its pods'),
        ],
      }),
    )!

    expect(statesByKey(view)['pre-migrations']).toBe('failed')
  })

  it('attributes a failed batched-migrations check to the last gate', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Failed',
        version: '10.2.0',
        conditions: [
          condition('Progressing', 'False', 'BatchedMigrationsCheckFailed', 'the batched background migrations check Job "demo-abcd1234-bbm" failed'),
        ],
      }),
    )!

    const states = statesByKey(view)
    expect(states['background-migrations']).toBe('failed')
    expect(states['finish-rollout']).toBe('done')
  })

  it('flags a blocked upgrade path as failed and blocked', () => {
    const view = deriveUpgrade(
      resource({
        phase: 'Failed',
        version: '10.0.0',
        conditions: [
          condition('Upgradeable', 'False', 'MissingIntermediateChart', 'the intermediate chart version 10.2 is required but the Operator carries none'),
        ],
      }),
    )!

    expect(view.failed).toBe(true)
    expect(view.blocked).toBe(true)
    expect(view.message).toContain('intermediate chart version')
  })
})

describe('isUpgrading', () => {
  it('is true only for the Upgrading phase', () => {
    expect(isUpgrading(resource({ phase: 'Upgrading' }))).toBe(true)
    expect(isUpgrading(resource({ phase: 'Running' }))).toBe(false)
  })
})
