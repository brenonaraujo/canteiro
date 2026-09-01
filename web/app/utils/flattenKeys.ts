export function flattenKeys(
  value: Record<string, unknown>,
  prefix = ''
): string[] {
  const keys: string[] = []

  for (const [key, nested] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (isPlainObject(nested)) {
      keys.push(...flattenKeys(nested, path))
    } else {
      keys.push(path)
    }
  }

  return keys.sort()
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}
