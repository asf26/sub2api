/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ensureImageCached } from '../store'
import { downloadImageEntriesAsZip, downloadImageIds } from './downloadImages'

vi.mock('../store', () => ({
  ensureImageCached: vi.fn(),
}))

describe('downloadImages', () => {
  let clickSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.mocked(ensureImageCached).mockReset()
    vi.stubGlobal('fetch', vi.fn())
    vi.stubGlobal('URL', {
      ...URL,
      createObjectURL: vi.fn(() => 'blob:download'),
      revokeObjectURL: vi.fn(),
    })
    clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  })

  afterEach(() => {
    clickSpy.mockRestore()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('downloads cached data URLs without fetching them again', async () => {
    vi.mocked(ensureImageCached).mockResolvedValue('data:image/png;base64,aGVsbG8=')

    const result = await downloadImageIds(['stored-image-id'], 'task-1')

    expect(result).toEqual({ successCount: 1, failCount: 0 })
    expect(fetch).not.toHaveBeenCalled()
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1)

    const blob = vi.mocked(URL.createObjectURL).mock.calls[0][0] as Blob
    expect(blob.type).toBe('image/png')
    expect(blob.size).toBe(5)
  })

  it('falls back to a direct URL download for a single remote image when fetch is blocked', async () => {
    vi.mocked(fetch).mockRejectedValue(new TypeError('Failed to fetch'))

    const result = await downloadImageIds(['https://cdn.example.com/result.webp?token=1'], 'task-2')

    expect(result).toEqual({ successCount: 1, failCount: 0 })
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    expect(clickSpy).toHaveBeenCalledTimes(1)

    const link = document.querySelector('a') as HTMLAnchorElement | null
    expect(link).toBeNull()
  })

  it('keeps zip downloads strict when image bytes cannot be read', async () => {
    vi.mocked(fetch).mockRejectedValue(new TypeError('Failed to fetch'))

    const result = await downloadImageEntriesAsZip(
      [{ imageId: 'https://cdn.example.com/result.png', fileNameBase: 'result' }],
      'task-3',
    )

    expect(result).toEqual({ successCount: 0, failCount: 1 })
    expect(URL.createObjectURL).not.toHaveBeenCalled()
  })
})
