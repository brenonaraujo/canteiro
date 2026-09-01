import { describe, expect, it } from 'vitest'
import { publicSiteUrl, SITE_HOST, SITE_URL } from '../../app/utils/site'

describe('site', () => {
  it('targets the public Canteiro host', () => {
    expect(SITE_HOST).toBe('canteiro.brenon.cloud')
    expect(SITE_URL).toBe('https://canteiro.brenon.cloud')
  })

  it('prefers an explicit public URL from the environment', () => {
    expect(publicSiteUrl('http://localhost:3000')).toBe('http://localhost:3000')
  })

  it('falls back to the production URL when override is empty', () => {
    expect(publicSiteUrl('')).toBe(SITE_URL)
    expect(publicSiteUrl(undefined)).toBe(SITE_URL)
  })
})
