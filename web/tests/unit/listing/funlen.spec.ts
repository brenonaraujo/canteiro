import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function source(path: string): string {
  return readFileSync(join(process.cwd(), 'app', path), 'utf8')
}

function functionLineCount(src: string, name: string): number {
  const start = src.indexOf(`export function ${name}`)
  if (start < 0) {
    throw new Error(`missing export function ${name}`)
  }
  const open = src.indexOf('{', start)
  let depth = 0
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') {
      depth++
    }
    if (src[i] === '}') {
      depth--
      if (depth === 0) {
        return src.slice(start, i + 1).split('\n').length
      }
    }
  }
  throw new Error(`unclosed function ${name}`)
}

describe('listing funlen', () => {
  it('keeps validateListingDraft at or under 35 lines (Decisão 4)', () => {
    expect(functionLineCount(
      source('composables/listing/validation.ts'),
      'validateListingDraft'
    )).toBeLessThanOrEqual(35)
  })

  it('keeps useListingOwner at or under 35 lines', () => {
    expect(functionLineCount(
      source('composables/listing/useListingOwner.ts'),
      'useListingOwner'
    )).toBeLessThanOrEqual(35)
  })

  it('keeps useListingList at or under 35 lines', () => {
    expect(functionLineCount(
      source('composables/listing/useListingList.ts'),
      'useListingList'
    )).toBeLessThanOrEqual(35)
  })
})
