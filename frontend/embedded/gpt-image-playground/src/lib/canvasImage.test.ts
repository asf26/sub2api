import { afterEach, describe, expect, it, vi } from 'vitest'
import { dataUrlToBlob, maskDataUrlToPngBlob } from './canvasImage'

describe('canvasImage data URL conversion', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('converts base64 data URLs to blobs without fetch', async () => {
    vi.stubGlobal('fetch', vi.fn(() => {
      throw new Error('fetch should not be used for data URLs')
    }))

    const blob = await dataUrlToBlob('data:image/png;base64,aGVsbG8=')

    expect(fetch).not.toHaveBeenCalled()
    expect(blob.type).toBe('image/png')
    expect(blob.size).toBe(5)
  })

  it('uses the fallback MIME type when a data URL omits a type', async () => {
    const blob = await dataUrlToBlob('data:;base64,aGVsbG8=', 'image/webp')

    expect(blob.type).toBe('image/webp')
    expect(blob.size).toBe(5)
  })

  it('keeps png masks as-is without canvas conversion', async () => {
    vi.stubGlobal('fetch', vi.fn(() => {
      throw new Error('fetch should not be used for mask data URLs')
    }))

    const blob = await maskDataUrlToPngBlob('data:image/png;base64,aGVsbG8=')

    expect(fetch).not.toHaveBeenCalled()
    expect(blob.type).toBe('image/png')
    expect(blob.size).toBe(5)
  })
})
