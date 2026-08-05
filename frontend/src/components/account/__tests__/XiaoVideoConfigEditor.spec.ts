import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import XiaoVideoConfigEditor from '../XiaoVideoConfigEditor.vue'

function mountEditor() {
  return mount(XiaoVideoConfigEditor, {
    props: {
      pricing: [
        {
          model: 'seedance-public',
          resolution: '720p',
          price_per_second: 0.12,
          audio_price_per_second: 0.03,
          default_resolution: true,
          default_duration: 8
        }
      ],
      mappings: []
    },
    global: {
      stubs: { Icon: true }
    }
  })
}

describe('XiaoVideoConfigEditor', () => {
  it('edits arbitrary public-to-upstream model mappings', async () => {
    const wrapper = mountEditor()

    await wrapper.get('[data-testid="xiao-add-mapping"]').trigger('click')
    await wrapper.get('[data-testid="xiao-mapping-public-0"]').setValue('video-public')
    await wrapper.get('[data-testid="xiao-mapping-upstream-0"]').setValue('provider-model-v3')

    expect((wrapper.props('mappings') as Array<{ from: string; to: string }>)[0]).toEqual({
      from: 'video-public',
      to: 'provider-model-v3'
    })
  })

  it('keeps only one default resolution for the same model', async () => {
    const wrapper = mountEditor()

    await wrapper.get('[data-testid="xiao-add-pricing"]').trigger('click')
    await wrapper.get('[data-testid="xiao-price-model-1"]').setValue('seedance-public')
    await wrapper.get('[data-testid="xiao-price-resolution-1"]').setValue('1080p')
    await wrapper.get('[data-testid="xiao-price-default-1"]').setValue(true)

    const pricing = wrapper.props('pricing') as Array<{ default_resolution: boolean }>
    expect(pricing.map((item) => item.default_resolution)).toEqual([false, true])
  })
})
