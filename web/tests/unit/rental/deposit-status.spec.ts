import { describe, expect, it } from 'vitest'
import { mapDepositStatus } from '../../../app/composables/rental/state'

describe('mapDepositStatus', () => {
  it('maps the four backend deposit states to UI labels', () => {
    expect(mapDepositStatus('released')).toBe('released')
    expect(mapDepositStatus('captured')).toBe('forfeit')
    expect(mapDepositStatus('partial')).toBe('partial')
    expect(mapDepositStatus('held')).toBe('held')
  })

  it('falls back to held when undefined or unknown', () => {
    expect(mapDepositStatus(undefined)).toBe('held')
    expect(mapDepositStatus('weird-value')).toBe('held')
  })
})
