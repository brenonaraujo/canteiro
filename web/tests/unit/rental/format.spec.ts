import { describe, expect, it } from 'vitest'
import { formatCents, formatRange, formatUtcDateTime, localeTag } from '../../../app/composables/rental/format'

describe('rental format helpers', () => {
  it('localeTag returns the tag the Intl APIs expect', () => {
    expect(localeTag('pt-BR')).toBe('pt-BR')
    expect(localeTag('es')).toBe('es-ES')
    expect(localeTag('en')).toBe('en-US')
    expect(localeTag('xx')).toBe('en-US')
  })

  it('formatCents renders positive amounts as currency', () => {
    // Different ICU versions place a thin space / no space / NBSP between
    // currency symbol and amount. We assert on the symbol, the decimals,
    // and the numeric value rather than the exact spacing.
    expect(formatCents(0, 'en')).toMatch(/R\$\s?0\.00/)
    expect(formatCents(1500, 'pt-BR')).toMatch(/R\$\s?15,00/)
    expect(formatCents(123456, 'en')).toMatch(/R\$\s?1\.234,56|R\$\s?1,234\.56/)
  })

  it('formatCents handles negative amounts cleanly', () => {
    const out = formatCents(-500, 'en')
    expect(out).toContain('-')
    expect(out).toMatch(/5[,.]00/)
  })

  it('formatUtcDateTime includes the year and a timezone marker', () => {
    const out = formatUtcDateTime('2026-09-02T15:30:00Z', 'en')
    expect(out).toContain('2026')
    // Some ICU versions emit "UTC" / "GMT" / "Coordinated Universal Time".
    expect(out.toUpperCase()).toMatch(/UTC|GMT|COOR/)
  })

  it('formatRange produces two timestamps separated by an arrow', () => {
    const out = formatRange('2026-09-02T15:00:00Z', '2026-09-02T17:00:00Z', 'en')
    expect(out).toContain('→')
  })
})
