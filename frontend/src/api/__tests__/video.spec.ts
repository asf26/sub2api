import { afterEach, describe, expect, it, vi } from 'vitest'
import { createVideo, fetchVideoContent, listVideoModels, uploadVideoMedia } from '../video'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('video gateway API', () => {
  it('uses the selected downstream key to fetch available models', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ object: 'list', data: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetchMock)

    await listVideoModels('sk-user-video')

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, init] = fetchMock.mock.calls[0]
    expect(String(url)).toContain('/v1/models')
    expect((init.headers as Headers).get('Authorization')).toBe('Bearer sk-user-video')
  })

  it('sends required async and idempotency headers when creating a job', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      job_id: 'vidjob_1', status: 'pending', status_url: '/v1/videos/jobs/vidjob_1',
    }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await createVideo('sk-user-video', {
      model: 'seedance-2.0', prompt: 'test', resolution: '480p', duration: 5, audio: true,
    }, 'video-request-1')

    const [, init] = fetchMock.mock.calls[0]
    const headers = init.headers as Headers
    expect(headers.get('Prefer')).toBe('respond-async')
    expect(headers.get('Idempotency-Key')).toBe('video-request-1')
    expect(headers.get('Authorization')).toBe('Bearer sk-user-video')
    expect(JSON.parse(init.body as string)).toMatchObject({ audio: true, model: 'seedance-2.0' })
  })

  it('uploads multipart media without overriding the browser boundary', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      media_id: 'vidmedia_1', url: '/v1/videos/uploads/vidmedia_1/content', type: 'image', expires_at: '2026-08-07T00:00:00Z',
    }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await uploadVideoMedia('sk-user-video', new File(['image'], 'opening.webp', { type: 'image/webp' }))

    const [, init] = fetchMock.mock.calls[0]
    expect(init.body).toBeInstanceOf(FormData)
    expect((init.headers as Headers).has('Content-Type')).toBe(false)
  })

  it('fetches completed video bytes with bearer authentication', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(new Blob(['video']), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await fetchVideoContent('sk-user-video', 'vidjob_1', true)

    const [url, init] = fetchMock.mock.calls[0]
    expect(String(url)).toContain('/v1/videos/jobs/vidjob_1/content?download=1')
    expect((init.headers as Headers).get('Authorization')).toBe('Bearer sk-user-video')
  })
})
