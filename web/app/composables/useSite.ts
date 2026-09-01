import { publicSiteUrl, SITE_HOST, SITE_URL } from '~/utils/site'

export function useSite() {
  const config = useRuntimeConfig()
  const host = (config.public.siteHost as string) || SITE_HOST
  const url = publicSiteUrl(config.public.siteUrl as string) || SITE_URL

  return { host, url }
}
