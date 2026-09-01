export const I18N_DEFAULT_LOCALE = 'pt-BR' as const

export const i18nLocales = [
  { code: 'en', language: 'en-US', name: 'English', file: 'en.json' },
  { code: 'pt-BR', language: 'pt-BR', name: 'Português', file: 'pt-BR.json' },
  { code: 'es', language: 'es-ES', name: 'Español', file: 'es.json' }
] as const

export type AppLocale = (typeof i18nLocales)[number]['code']
