import { describe, expect, it } from 'vitest'

import { compareVersions, higherVersions, upgradePath } from './versions'

describe('compareVersions', () => {
  it('orders by major, minor, then patch', () => {
    expect(compareVersions('10.2.6', '10.2.6')).toBe(0)
    expect(compareVersions('10.3.0', '10.2.6')).toBeGreaterThan(0)
    expect(compareVersions('10.2.6', '10.3.0')).toBeLessThan(0)
    expect(compareVersions('11.0.0', '10.9.9')).toBeGreaterThan(0)
    expect(compareVersions('10.2.10', '10.2.9')).toBeGreaterThan(0)
  })

  it('treats missing components as zero', () => {
    expect(compareVersions('10.2', '10.2.0')).toBe(0)
    expect(compareVersions('10.2.1', '10.2')).toBeGreaterThan(0)
  })
})

describe('higherVersions', () => {
  const all = ['10.3.2', '10.2.6', '10.1.8']

  it('returns only versions greater than current, newest first', () => {
    expect(higherVersions('10.1.8', all)).toEqual(['10.3.2', '10.2.6'])
    expect(higherVersions('10.2.6', all)).toEqual(['10.3.2'])
  })

  it('returns nothing when already at the latest', () => {
    expect(higherVersions('10.3.2', all)).toEqual([])
    expect(higherVersions('10.4.0', all)).toEqual([])
  })

  it('returns nothing when the current version is unknown', () => {
    expect(higherVersions('', all)).toEqual([])
  })
})

describe('upgradePath', () => {
  const all = ['10.3.2', '10.2.6', '10.1.8', '10.1.6']

  it('lists one hop per minor up to the target, highest patch each', () => {
    expect(upgradePath('10.1.8', '10.3.2', all)).toEqual(['10.2.6', '10.3.2'])
  })

  it('is a single hop for a single-minor upgrade', () => {
    expect(upgradePath('10.2.6', '10.3.2', all)).toEqual(['10.3.2'])
  })

  it('ends on the target even when the catalog lacks it', () => {
    expect(upgradePath('10.1.8', '10.4.0', all)).toEqual(['10.2.6', '10.3.2', '10.4.0'])
  })

  it('returns nothing when the target is not above current', () => {
    expect(upgradePath('10.3.2', '10.3.2', all)).toEqual([])
    expect(upgradePath('10.3.2', '10.2.6', all)).toEqual([])
  })
})
