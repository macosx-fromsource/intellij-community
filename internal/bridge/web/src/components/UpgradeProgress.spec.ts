import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { GitLabResource } from '@/lib/api/client'
import UpgradeProgress from './UpgradeProgress.vue'

function resource(conditions: Record<string, string>[], phase: string, version: string): GitLabResource {
  return {
    name: 'demo',
    namespace: 'ns',
    chart: { version: '10.3.0', values: {} },
    status: {
      phase,
      version,
      conditions: conditions.map((c) => ({ ...c, lastTransitionTime: '2026-01-01T00:00:00Z' })),
    },
  } as unknown as GitLabResource
}

describe('UpgradeProgress', () => {
  it('renders nothing when there is no upgrade to show', () => {
    const wrapper = mount(UpgradeProgress, {
      props: {
        resource: resource(
          [{ type: 'Progressing', status: 'False', reason: 'UpgradeComplete', message: 'upgraded to 10.3.0' }],
          'Running',
          '10.3.0',
        ),
      },
    })

    expect(wrapper.find('.upgrade').exists()).toBe(false)
  })

  it('shows the version chain and the five gates while upgrading', () => {
    const wrapper = mount(UpgradeProgress, {
      props: {
        resource: resource(
          [
            { type: 'Upgradeable', status: 'True', reason: 'UpgradePathValid', message: 'upgrading toward 10.3.0, next 10.2.0' },
            { type: 'Progressing', status: 'True', reason: 'RunningPreMigrations', message: 'running the pre-deployment migrations' },
          ],
          'Upgrading',
          '10.1.0',
        ),
        availableVersions: ['10.3.0', '10.2.0', '10.1.0'],
      },
    })

    expect(wrapper.findAll('.gate')).toHaveLength(5)

    // Every hop of a multi-minor upgrade is listed as an arrow chain.
    const versions = wrapper.findAll('.version').map((v) => v.text())
    expect(versions).toEqual(['10.1.0', '10.2.0', '10.3.0'])

    // The in-flight hop and its incoming arrow are highlighted.
    expect(wrapper.find('.version--active').text()).toBe('10.2.0')
    expect(wrapper.find('.arrow--active').exists()).toBe(true)

    expect(wrapper.find('.gate--active').text()).toContain('Pre-deployment migrations')
  })

  it('still shows the intermediate hop when the version catalog is unavailable', () => {
    const wrapper = mount(UpgradeProgress, {
      props: {
        resource: resource(
          [
            { type: 'Upgradeable', status: 'True', reason: 'UpgradePathValid', message: 'upgrading toward 10.3.0, next 10.2.0' },
            { type: 'Progressing', status: 'True', reason: 'RunningPreMigrations', message: 'running the pre-deployment migrations' },
          ],
          'Upgrading',
          '10.1.0',
        ),
        // availableVersions omitted: the chart-versions fetch failed or has not resolved.
      },
    })

    // The chain must not collapse to current -> target; the known intermediate stays.
    const versions = wrapper.findAll('.version').map((v) => v.text())
    expect(versions).toEqual(['10.1.0', '10.2.0', '10.3.0'])
    expect(wrapper.find('.version--active').text()).toBe('10.2.0')
  })

  it('surfaces a failure banner when the upgrade failed', () => {
    const wrapper = mount(UpgradeProgress, {
      props: {
        resource: resource(
          [{ type: 'Progressing', status: 'False', reason: 'MigrationsJobFailed', message: 'the migrations Job "demo-abcd" failed; inspect its pods' }],
          'Failed',
          '10.2.0',
        ),
      },
    })

    const banner = wrapper.find('.banner--failed')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('Upgrade failed')
    expect(wrapper.find('.gate--failed').exists()).toBe(true)
  })
})
