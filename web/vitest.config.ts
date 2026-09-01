import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    include: ['tests/unit/**/*.spec.ts'],
    environment: 'node',
    coverage: {
      provider: 'v8',
      reportsDirectory: './coverage',
      include: ['app/utils/**/*.ts', 'i18n/**/*.ts'],
      reporter: ['text', 'json', 'html']
    }
  }
})
