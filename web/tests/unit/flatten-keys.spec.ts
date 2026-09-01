import { describe, expect, it } from 'vitest'
import { flattenKeys } from '../../app/utils/flattenKeys'

describe('flattenKeys', () => {
  it('joins nested object paths with dots', () => {
    const keys = flattenKeys({
      landing: { hero: { title: 'Hi' } },
      common: { app_name: 'Canteiro' }
    })

    expect(keys).toEqual(['common.app_name', 'landing.hero.title'])
  })

  it('returns empty list for empty object', () => {
    expect(flattenKeys({})).toEqual([])
  })
})
