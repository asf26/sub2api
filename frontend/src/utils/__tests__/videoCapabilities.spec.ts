import { describe, expect, it } from 'vitest'
import type { VideoModel } from '@/api/video'
import { aspectRatiosFor, durationsFor, resolveVideoCapability } from '../videoCapabilities'

function model(id: string, resolutions: string[], overrides: Partial<VideoModel> = {}): VideoModel {
  return { id, object: 'model', owned_by: 'video', resolutions, ...overrides }
}

describe('video capabilities', () => {
  it('keeps Seedance generated audio enabled and limits 1080p to 12 seconds', () => {
    const capability = resolveVideoCapability(model('seedance-2.0', ['480p', '720p', '1080p']))

    expect(capability.supportsAudio).toBe(true)
    expect(durationsFor(capability, '1080p')).toEqual([4, 5, 6, 7, 8, 9, 10, 11, 12])
    expect(aspectRatiosFor(capability, '720p')).not.toContain('9:21')
    expect(aspectRatiosFor(capability, '480p')).toContain('9:21')
  })

  it('intersects documented capabilities with resolutions enabled by platform pricing', () => {
    const capability = resolveVideoCapability(model('ltx-2.3-fast', ['1440p']))

    expect(capability.resolutions).toEqual(['1440p'])
    expect(capability.defaultResolution).toBe('1440p')
    expect(capability.durations).toEqual([6, 8, 10, 12, 14, 16, 18, 20])
  })

  it('preserves safe runtime values for newly configured upstream models', () => {
    const capability = resolveVideoCapability(model('future-video-model', ['900p'], {
      default_duration: 7,
      default_aspect_ratio: '4:3',
      supports_audio: true,
      supports_guidances: true,
    }))

    expect(capability.label).toBe('future-video-model')
    expect(capability.durations).toEqual([7])
    expect(capability.supportsAudio).toBe(true)
    expect(capability.maxReferences.image).toBe(1)
  })
})
