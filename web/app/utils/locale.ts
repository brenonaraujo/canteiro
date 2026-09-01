import { i18nLocales, type AppLocale } from '../../i18n/options'

export function isAppLocale(code: string): code is AppLocale {
  return i18nLocales.some(locale => locale.code === code)
}
