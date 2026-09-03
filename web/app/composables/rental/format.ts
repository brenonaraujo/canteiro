import type { AppLocale } from '../../../i18n/options'

export function localeTag(loc: string): string {
  if (loc === 'pt-BR') return 'pt-BR'
  if (loc === 'es') return 'es-ES'
  return 'en-US'
}

export function formatCents(amountCents: number, loc: string): string {
  return new Intl.NumberFormat(localeTag(loc), {
    style: 'currency',
    currency: 'BRL'
  }).format(amountCents / 100)
}

export function formatUtcDateTime(iso: string, loc: string): string {
  return new Intl.DateTimeFormat(localeTag(loc), {
    dateStyle: 'medium',
    timeStyle: 'medium',
    timeZone: 'UTC',
    timeZoneName: 'short'
  }).format(new Date(iso))
}

export function formatUtcDate(iso: string, loc: string): string {
  return new Intl.DateTimeFormat(localeTag(loc), {
    dateStyle: 'medium',
    timeZone: 'UTC'
  }).format(new Date(iso))
}

export function formatRange(startIso: string, endIso: string, loc: string): string {
  const locTag = localeTag(loc)
  const fmt = new Intl.DateTimeFormat(locTag, {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: 'UTC',
    timeZoneName: 'short'
  })
  const start = new Date(startIso)
  const end = new Date(endIso)
  return `${fmt.format(start)} → ${fmt.format(end)}`
}

export function toAppLocale(code: string): AppLocale {
  if (code === 'pt-BR' || code === 'es' || code === 'en') {
    return code
  }
  return 'en'
}
