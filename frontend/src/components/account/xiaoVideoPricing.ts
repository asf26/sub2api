export interface XiaoVideoPricingRule {
  model: string
  resolution: string
  price_per_second: number
  audio_price_per_second: number
  default_resolution: boolean
  default_duration: number
}

export interface XiaoVideoModelMapping {
  from: string
  to: string
}

export type XiaoVideoPricingValidationCode =
  | 'required'
  | 'modelRequired'
  | 'resolutionRequired'
  | 'priceInvalid'
  | 'audioPriceInvalid'
  | 'durationInvalid'
  | 'duplicateRule'
  | 'multipleDefaults'
  | 'defaultRequired'

export function createXiaoVideoPricingRule(): XiaoVideoPricingRule {
  return {
    model: '',
    resolution: '',
    price_per_second: 0,
    audio_price_per_second: 0,
    default_resolution: true,
    default_duration: 4
  }
}

export function readXiaoVideoPricing(value: unknown): XiaoVideoPricingRule[] {
  if (!Array.isArray(value)) return [createXiaoVideoPricingRule()]
  const rules = value.flatMap((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) return []
    const record = item as Record<string, unknown>
    return [{
      model: typeof record.model === 'string' ? record.model : '',
      resolution: typeof record.resolution === 'string' ? record.resolution : '',
      price_per_second: Number(record.price_per_second ?? 0),
      audio_price_per_second: Number(record.audio_price_per_second ?? 0),
      default_resolution: record.default_resolution === true,
      default_duration: Number(record.default_duration ?? 0)
    }]
  })
  return rules.length > 0 ? rules : [createXiaoVideoPricingRule()]
}

export function normalizeXiaoVideoPricing(rules: XiaoVideoPricingRule[]): XiaoVideoPricingRule[] {
  return rules.map((rule) => ({
    model: rule.model.trim(),
    resolution: rule.resolution.trim(),
    price_per_second: Number(rule.price_per_second),
    audio_price_per_second: Number(rule.audio_price_per_second),
    default_resolution: rule.default_resolution === true,
    default_duration: Number(rule.default_duration)
  }))
}

export function validateXiaoVideoPricing(
  rules: XiaoVideoPricingRule[]
): XiaoVideoPricingValidationCode | null {
  if (rules.length === 0) return 'required'
  const seen = new Set<string>()
  const countByModel = new Map<string, number>()
  const defaultsByModel = new Map<string, number>()

  for (const rule of rules) {
    const model = rule.model.trim()
    const resolution = rule.resolution.trim()
    if (!model) return 'modelRequired'
    if (!resolution) return 'resolutionRequired'
    if (!Number.isFinite(Number(rule.price_per_second)) || Number(rule.price_per_second) < 0) {
      return 'priceInvalid'
    }
    if (!Number.isFinite(Number(rule.audio_price_per_second)) || Number(rule.audio_price_per_second) < 0) {
      return 'audioPriceInvalid'
    }
    if (!Number.isInteger(Number(rule.default_duration)) || Number(rule.default_duration) < 1 || Number(rule.default_duration) > 3600) {
      return 'durationInvalid'
    }
    const key = `${model}\u0000${resolution}`
    if (seen.has(key)) return 'duplicateRule'
    seen.add(key)
    countByModel.set(model, (countByModel.get(model) ?? 0) + 1)
    if (rule.default_resolution) {
      const count = (defaultsByModel.get(model) ?? 0) + 1
      if (count > 1) return 'multipleDefaults'
      defaultsByModel.set(model, count)
    }
  }

  for (const [model, count] of countByModel) {
    if (count > 1 && (defaultsByModel.get(model) ?? 0) !== 1) return 'defaultRequired'
  }
  return null
}
