import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function vueFiles(dir: string): string[] {
  const entries = readdirSync(dir, { withFileTypes: true })
  const files: string[] = []
  for (const entry of entries) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) {
      files.push(...vueFiles(path))
    } else if (entry.name.endsWith('.vue')) {
      files.push(path)
    }
  }
  return files
}

describe('vue copy', () => {
  it('routes visible copy through i18n in every component', () => {
    const files = vueFiles(join(process.cwd(), 'app'))
    expect(files.length).toBeGreaterThan(0)

    for (const file of files) {
      const src = readFileSync(file, 'utf8')
      expect(src, file).not.toMatch(/>([A-Z][a-z]+(\s+[a-z]+){1,8})</)
    }
  })
})
