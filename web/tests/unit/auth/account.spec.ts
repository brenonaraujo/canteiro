import { describe, expect, it } from 'vitest'
import {
  canPublishListing,
  canStartRental,
  deriveStatus,
  postAuthPath,
  profileIsComplete,
  validateProfile
} from '../../../app/composables/auth/account'

describe('account domain', () => {
  it('treats missing name or phone as an incomplete profile', () => {
    expect(profileIsComplete({ displayName: '', phone: '11999999999' })).toBe(false)
    expect(profileIsComplete({ displayName: 'Ana', phone: '   ' })).toBe(false)
    expect(profileIsComplete({ displayName: 'Ana', phone: '11999999999' })).toBe(true)
  })

  it('derives incomplete until name and phone are both set', () => {
    expect(deriveStatus({
      id: 'acc-1',
      status: 'active',
      displayName: '',
      phone: ''
    })).toBe('incomplete')
  })

  it('keeps deactivated accounts deactivated even with a complete profile', () => {
    expect(deriveStatus({
      id: 'acc-1',
      status: 'deactivated',
      displayName: 'Ana',
      phone: '11999999999'
    })).toBe('deactivated')
  })

  it('promotes a filled profile to active', () => {
    expect(deriveStatus({
      id: 'acc-1',
      status: 'incomplete',
      displayName: 'Ana',
      phone: '11999999999'
    })).toBe('active')
  })

  it('allows renter actions only when the account is active', () => {
    expect(canStartRental('active')).toBe(true)
    expect(canStartRental('incomplete')).toBe(false)
    expect(canStartRental('deactivated')).toBe(false)
  })

  it('never allows publishing in F1', () => {
    expect(canPublishListing('active')).toBe(false)
    expect(canPublishListing('incomplete')).toBe(false)
    expect(canPublishListing('deactivated')).toBe(false)
  })

  it('sends incomplete Google sessions to the profile page', () => {
    expect(postAuthPath({
      hasError: false,
      authenticated: true,
      profileComplete: false
    })).toBe('/auth/profile')
    expect(postAuthPath({
      hasError: true,
      authenticated: false,
      profileComplete: false
    })).toBe('/auth/login')
    expect(postAuthPath({
      hasError: false,
      authenticated: true,
      profileComplete: true
    })).toBe('/')
  })

  it('rejects empty display name or phone on profile save', () => {
    expect(validateProfile({ displayName: '  ', phone: '1199' }).ok).toBe(false)
    expect(validateProfile({ displayName: 'Ana', phone: '' }).ok).toBe(false)
    expect(validateProfile({ displayName: 'Ana', phone: '11999999999' })).toEqual({
      ok: true,
      value: { displayName: 'Ana', phone: '11999999999' }
    })
  })
})
