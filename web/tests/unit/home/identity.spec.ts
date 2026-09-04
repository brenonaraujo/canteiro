import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function readWeb(path: string): string {
  return readFileSync(join(process.cwd(), path), 'utf8')
}

describe('shared Canteiro identity (#26)', () => {
  it('pins brand tokens away from the Nuxt UI scaffold defaults', () => {
    const config = readWeb('app/app.config.ts')
    const css = readWeb('app/assets/css/main.css')
    expect(config).toContain('primary:')
    expect(config).not.toMatch(/primary:\s*'green'/)
    expect(css).toContain('--font-sans')
    expect(css).not.toContain('Public Sans')
    expect(css).toContain('--ui-radius')
    expect(css).not.toMatch(/#[0-9a-fA-F]{3,8}/)
  })

  it('exposes a named logo mark for assistive tech', () => {
    const logo = readWeb('app/components/common/AppLogo.vue')
    expect(logo).toContain('a11y.logo')
    expect(logo).toContain('common.app_name')
  })

  it('keeps catalog discovery usable on a narrow header without duplicating account', () => {
    const header = readWeb('app/components/common/AppHeader.vue')
    expect(header).toContain('to: \'/listings\'')
    const body = header.split('#body')[1] ?? ''
    expect(body).not.toContain('AccountMenu')
    expect(header).toContain('AccountMenu')
  })
})
