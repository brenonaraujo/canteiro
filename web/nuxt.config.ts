import { defineNuxtConfig } from 'nuxt/config'
import { i18nLocales, I18N_DEFAULT_LOCALE } from './i18n/options'

export default defineNuxtConfig({
  modules: [
    '@nuxt/eslint',
    '@nuxt/ui',
    '@nuxtjs/i18n',
    '@pinia/nuxt',
    '@vueuse/nuxt'
  ],

  components: [
    { path: '~/components/common', pathPrefix: false },
    { path: '~/components/feature', pathPrefix: false }
  ],

  css: ['~/assets/css/main.css'],

  runtimeConfig: {
    public: {
      apiBase: '',
      siteUrl: 'https://canteiro.brenon.cloud',
      siteHost: 'canteiro.brenon.cloud'
    }
  },

  routeRules: {
    '/': { prerender: true }
  },

  compatibilityDate: '2026-06-30',

  eslint: {
    config: {
      stylistic: {
        commaDangle: 'never',
        braceStyle: '1tbs'
      }
    }
  },

  i18n: {
    strategy: 'no_prefix',
    defaultLocale: I18N_DEFAULT_LOCALE,
    langDir: 'locales',
    locales: [...i18nLocales],
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'i18n_redirected',
      redirectOn: 'root',
      fallbackLocale: I18N_DEFAULT_LOCALE
    }
  }
})
