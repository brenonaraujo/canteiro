import { readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig } from 'vitest/config'

/**
 * Locate a pnpm-virtualized dependency inside `node_modules/.pnpm/<pkg>@*`.
 *
 * Transitive deps (e.g. `vue` resolved through `nuxt` or `@pinia/nuxt`) are NOT
 * symlinked at the top of `node_modules/` in pnpm's strict layout, so the
 * Node ESM resolver cannot find them when our SUT imports them directly. The
 * only place they exist on disk is `.pnpm/<pkg>@<version>_<peerHash>/node_modules/<pkg>`.
 *
 * `pnpm-lock.yaml` guarantees exactly one such directory exists for each
 * direct resolution, so we pick the first match.
 */
function pnpmDepDir(pkg: string): string {
  const pnpmRoot = resolve(process.cwd(), 'node_modules', '.pnpm')
  const match = readdirSync(pnpmRoot).find(
    entry => entry.startsWith(`${pkg}@`)
  )
  if (!match) {
    throw new Error(
      `[vitest] cannot locate '${pkg}' in ${pnpmRoot}. `
      + 'Run `pnpm install --frozen-lockfile` from web/ first.'
    )
  }
  return resolve(pnpmRoot, match, 'node_modules', pkg)
}

export default defineConfig({
  resolve: {
    alias: {
      // Resolve `vue` and `pinia` (transitive deps, not hoisted by pnpm) to
      // their real location inside .pnpm/. Required for SUTs that import
      // them directly (e.g. app/stores/listing/*) — without this, vitest
      // running under environment: 'node' cannot find them.
      vue: pnpmDepDir('vue'),
      pinia: pnpmDepDir('pinia')
    }
  },
  test: {
    include: ['tests/unit/**/*.spec.ts'],
    environment: 'node',
    // Inline `vue` and `pinia` into vitest's transform graph so the alias
    // above is honoured for every SUT path, and the runtime ESM doesn't
    // try to resolve them from the bare specifier on its own.
    server: {
      deps: {
        inline: ['vue', 'pinia']
      }
    },
    coverage: {
      provider: 'v8',
      reportsDirectory: './coverage',
      include: [
        'app/utils/**/*.ts',
        'app/composables/auth/**/*.ts',
        'i18n/**/*.ts'
      ],
      exclude: ['app/composables/auth/useAuth.ts'],
      reporter: ['text', 'json', 'html']
    }
  }
})
