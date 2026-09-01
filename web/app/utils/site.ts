export const SITE_HOST = 'canteiro.brenon.cloud'
export const SITE_URL = `https://${SITE_HOST}`

export function publicSiteUrl(override?: string): string {
  if (override && override.length > 0) {
    return override
  }
  return SITE_URL
}
