import { describe, expect, it } from 'vitest'

import {
  createXiaoVideoPricingRule,
  normalizeXiaoVideoPricing,
  readXiaoVideoPricing,
  validateXiaoVideoPricing,
  type XiaoVideoPricingRule
} from '../xiaoVideoPricing'

function rule(overrides: Partial<XiaoVideoPricingRule> = {}): XiaoVideoPricingRule {
  return {
    model: 'seedance-2.0',
    resolution: '720p',
    price_per_second: 0.12,
    audio_price_per_second: 0.03,
    default_resolution: true,
    default_duration: 8,
    ...overrides
  }
}

describe('xiaoVideoPricing', () => {
  it('creates an editable empty rule and recovers from missing stored data', () => {
    expect(createXiaoVideoPricingRule()).toEqual({
      model: '',
      resolution: '',
      price_per_second: 0,
      audio_price_per_second: 0,
      default_resolution: true,
      default_duration: 4
    })
    expect(readXiaoVideoPricing(undefined)).toEqual([createXiaoVideoPricingRule()])
  })

  it('normalizes strings and numeric input before submission', () => {
    expect(normalizeXiaoVideoPricing([
      rule({ model: ' seedance-public ', resolution: ' 1080p ', price_per_second: '0.5' as any })
    ])).toEqual([
      rule({ model: 'seedance-public', resolution: '1080p', price_per_second: 0.5 })
    ])
  })

  it('accepts arbitrary models, resolutions, and zero prices', () => {
    expect(validateXiaoVideoPricing([
      rule({ model: 'custom-public-model', resolution: 'portrait-hd', price_per_second: 0 })
    ])).toBeNull()
  })

  it.each([
    { rules: [] as XiaoVideoPricingRule[], expected: 'required' },
    { rules: [rule({ model: '' })], expected: 'modelRequired' },
    { rules: [rule({ resolution: '' })], expected: 'resolutionRequired' },
    { rules: [rule({ price_per_second: -1 })], expected: 'priceInvalid' },
    { rules: [rule({ audio_price_per_second: Number.NaN })], expected: 'audioPriceInvalid' },
    { rules: [rule({ default_duration: 0 })], expected: 'durationInvalid' },
    { rules: [rule(), rule()], expected: 'duplicateRule' },
    {
      rules: [rule(), rule({ resolution: '1080p' })],
      expected: 'multipleDefaults'
    },
    {
      rules: [
        rule({ default_resolution: false }),
        rule({ resolution: '1080p', default_resolution: false })
      ],
      expected: 'defaultRequired'
    }
  ])('rejects invalid pricing with $expected', ({ rules, expected }) => {
    expect(validateXiaoVideoPricing(rules)).toBe(expected)
  })
})
