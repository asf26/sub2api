export const DEFAULT_SITE_NAME = 'MoliGate'
export const DEFAULT_SITE_LOGO = '/logo.svg'
export const DEFAULT_SITE_TAGLINE = '一个 Key 接入各大顶尖大模型'

const LEGACY_SITE_NAMES = new Set(['Sub2API', 'sub2api'])
const LEGACY_SITE_SUBTITLES = new Set([
  'subscription to api conversion platform',
  'subscription api conversion platform',
  'sub2api',
])

function normalizeBrandValue(value?: string | null): string {
  return value?.trim().toLowerCase() ?? ''
}

export function resolveBrandName(value?: string | null): string {
  const normalized = value?.trim()
  if (!normalized || LEGACY_SITE_NAMES.has(normalized)) {
    return DEFAULT_SITE_NAME
  }
  return normalized
}

export function resolveBrandLogo(value?: string | null): string {
  const normalized = value?.trim()
  if (!normalized || normalized === '/logo.png') {
    return DEFAULT_SITE_LOGO
  }
  return normalized
}

export function resolveBrandSubtitle(value?: string | null): string {
  const normalized = value?.trim()
  if (!normalized) {
    return DEFAULT_SITE_TAGLINE
  }

  if (LEGACY_SITE_SUBTITLES.has(normalizeBrandValue(normalized))) {
    return DEFAULT_SITE_TAGLINE
  }

  return normalized
}

export function resolveBrandTitle(value?: string | null): string {
  return `${resolveBrandName(value)} - ${DEFAULT_SITE_TAGLINE}`
}
