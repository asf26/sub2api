import { buildGatewayUrl } from './url'

export type VideoJobStatus =
  | 'pending'
  | 'running'
  | 'settling'
  | 'completed'
  | 'failed'
  | 'canceled'

export interface VideoModel {
  id: string
  object: 'model'
  owned_by: string
  resolutions: string[]
  default_resolution?: string
  default_duration?: number
  default_aspect_ratio?: string
  supports_audio?: boolean
  supports_guidances?: boolean
}

export interface UploadedVideoMedia {
  media_id: string
  url: string
  type: 'image' | 'video' | 'audio' | string
  expires_at: string
}

export interface VideoGuidances {
  image_reference?: Array<{
    image: { url: string; type: 'UPLOADED' }
    strength?: 'LOW' | 'MID' | 'HIGH'
    order?: number
  }>
  video_reference_base?: Array<{
    video: { url: string; type: 'UPLOADED' }
  }>
  audio_reference?: Array<{
    audio: { url: string; type: 'UPLOADED' }
  }>
}

export interface CreateVideoRequest {
  model: string
  prompt: string
  resolution?: string
  duration?: number
  aspect_ratio?: string
  audio?: boolean
  prompt_enhance?: 'AUTO' | 'ON' | 'OFF'
  start_frame_url?: string
  end_frame_url?: string
  guidances?: VideoGuidances
}

export interface CreatedVideoJob {
  job_id: string
  status: VideoJobStatus
  status_url: string
}

export interface VideoJob {
  job_id: string
  status: VideoJobStatus
  model: string
  resolution: string
  duration: number
  aspect_ratio: string
  amount: string
  currency: string
  created_at: string
  updated_at: string
  status_url: string
  content_url?: string
  error?: { code: string; message: string }
}

interface ListEnvelope<T> {
  object: 'list'
  data: T[]
}

export class VideoAPIError extends Error {
  status: number
  code: string
  requestId: string
  retryAfter: string

  constructor(message: string, status: number, code = '', response?: Response) {
    super(message)
    this.name = 'VideoAPIError'
    this.status = status
    this.code = code
    this.requestId = response?.headers.get('X-Request-Id') || ''
    this.retryAfter = response?.headers.get('Retry-After') || ''
  }
}

function videoHeaders(apiKey: string, extra?: HeadersInit): Headers {
  const headers = new Headers(extra)
  headers.set('Authorization', `Bearer ${apiKey}`)
  headers.set('Accept', 'application/json')
  return headers
}

async function parseVideoError(response: Response): Promise<VideoAPIError> {
  let message = `请求失败（HTTP ${response.status}）`
  let code = ''
  try {
    const payload = await response.json() as {
      error?: { message?: string; code?: string }
      message?: string
      code?: string
    }
    message = payload.error?.message || payload.message || message
    code = payload.error?.code || payload.code || ''
  } catch {
    // Upstream error pages are intentionally not exposed to the user.
  }
  return new VideoAPIError(message, response.status, code, response)
}

async function videoRequest<T>(apiKey: string, path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(buildGatewayUrl(path), {
    ...init,
    headers: videoHeaders(apiKey, init.headers),
  })
  if (!response.ok) throw await parseVideoError(response)
  return response.json() as Promise<T>
}

export async function listVideoModels(apiKey: string, signal?: AbortSignal): Promise<VideoModel[]> {
  const result = await videoRequest<ListEnvelope<VideoModel>>(apiKey, '/v1/models', { signal })
  return result.data
}

export async function uploadVideoMedia(
  apiKey: string,
  file: File,
  signal?: AbortSignal,
): Promise<UploadedVideoMedia> {
  const form = new FormData()
  form.append('file', file, file.name)
  return videoRequest<UploadedVideoMedia>(apiKey, '/v1/videos/uploads', {
    method: 'POST',
    body: form,
    signal,
  })
}

export async function createVideo(
  apiKey: string,
  request: CreateVideoRequest,
  idempotencyKey: string,
  signal?: AbortSignal,
): Promise<CreatedVideoJob> {
  return videoRequest<CreatedVideoJob>(apiKey, '/v1/videos/generations', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Prefer': 'respond-async',
      'Idempotency-Key': idempotencyKey,
    },
    body: JSON.stringify(request),
    signal,
  })
}

export async function listVideoJobs(
  apiKey: string,
  limit = 20,
  signal?: AbortSignal,
): Promise<VideoJob[]> {
  const safeLimit = Math.max(1, Math.min(100, Math.trunc(limit)))
  const result = await videoRequest<ListEnvelope<VideoJob>>(
    apiKey,
    `/v1/videos/jobs?limit=${safeLimit}`,
    { signal },
  )
  return result.data
}

export function getVideoJob(apiKey: string, jobId: string, signal?: AbortSignal): Promise<VideoJob> {
  return videoRequest<VideoJob>(apiKey, `/v1/videos/jobs/${encodeURIComponent(jobId)}`, { signal })
}

export function cancelVideoJob(apiKey: string, jobId: string, signal?: AbortSignal): Promise<VideoJob> {
  return videoRequest<VideoJob>(apiKey, `/v1/videos/jobs/${encodeURIComponent(jobId)}`, {
    method: 'DELETE',
    signal,
  })
}

export async function fetchVideoContent(
  apiKey: string,
  jobId: string,
  download = false,
  signal?: AbortSignal,
): Promise<Blob> {
  const suffix = download ? '?download=1' : ''
  const response = await fetch(
    buildGatewayUrl(`/v1/videos/jobs/${encodeURIComponent(jobId)}/content${suffix}`),
    { headers: videoHeaders(apiKey), signal },
  )
  if (!response.ok) throw await parseVideoError(response)
  return response.blob()
}

export const videoAPI = {
  listModels: listVideoModels,
  upload: uploadVideoMedia,
  create: createVideo,
  listJobs: listVideoJobs,
  getJob: getVideoJob,
  cancelJob: cancelVideoJob,
  fetchContent: fetchVideoContent,
}
