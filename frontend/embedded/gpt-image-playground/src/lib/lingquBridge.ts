import type { ApiMode, AppSettings, CustomProviderDefinition } from '../types'
import { DEFAULT_STREAM_PARTIAL_IMAGES } from '../types'
import { DEFAULT_OPENAI_PROFILE_ID, normalizeSettings } from './apiProfiles'

export const LINGQU_BRIDGE_STORAGE_KEY = 'lingqu:image-playground:bridge'
export const LINGQU_ASYNC_PROVIDER_ID = 'lingqu-async-openai'

export interface LingquBridgePayload {
  apiUrl?: string
  keyId?: string | number
  apiKey?: string
  keyName?: string
  model?: string
  apiMode?: 'images' | 'responses'
  userEmail?: string
  userTheme?: 'cartoon' | 'business' | 'claude'
  launchedAt?: number
}

function readPayload(): LingquBridgePayload | null {
  try {
    const raw = window.sessionStorage.getItem(LINGQU_BRIDGE_STORAGE_KEY)
      || window.localStorage.getItem(LINGQU_BRIDGE_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null
    return parsed as LingquBridgePayload
  } catch {
    return null
  }
}

export function getLingquBridgePayload(): LingquBridgePayload | null {
  if (typeof window === 'undefined') return null
  return readPayload()
}

export function isLingquBridgeSettings(input: Partial<AppSettings> | unknown): boolean {
  const normalized = normalizeSettings(input)
  const active = normalized.profiles.find((profile) => profile.id === normalized.activeProfileId) ?? normalized.profiles[0]
  const provider = normalized.customProviders.find((item) => item.id === LINGQU_ASYNC_PROVIDER_ID)

  return Boolean(
    active &&
    provider?.poll &&
    active.provider === LINGQU_ASYNC_PROVIDER_ID &&
    active.apiKey.trim() &&
    active.baseUrl.trim()
  )
}

export function buildLingquSettings(currentSettings: AppSettings, payload: LingquBridgePayload | null): Partial<AppSettings> {
  const apiKey = payload?.apiKey?.trim()
  if (!apiKey) return {}

  const baseUrl = payload?.apiUrl?.trim() || ''
  const model = payload?.model?.trim() || 'gpt-image-2'
  const apiMode: ApiMode = payload?.apiMode === 'responses' ? 'responses' : 'images'
  const provider: CustomProviderDefinition = {
    id: LINGQU_ASYNC_PROVIDER_ID,
    name: '平台异步图工坊',
    template: 'http-image' as const,
    submit: {
      path: 'images/generations/async',
      method: 'POST' as const,
      contentType: 'json' as const,
      body: {
        model: '$profile.model',
        prompt: '$prompt',
        size: '$params.size',
        quality: '$params.quality',
        output_format: '$params.output_format',
        moderation: '$params.moderation',
        output_compression: '$params.output_compression',
        n: '$params.n',
      },
      taskIdPath: 'task_id',
      result: {
        imageUrlPaths: ['data.result.data.*.url'],
        b64JsonPaths: ['data.result.data.*.b64_json'],
      },
    },
    editSubmit: {
      path: 'images/edits/async',
      method: 'POST' as const,
      contentType: 'multipart' as const,
      body: {
        model: '$profile.model',
        prompt: '$prompt',
        size: '$params.size',
        quality: '$params.quality',
        output_format: '$params.output_format',
        moderation: '$params.moderation',
        output_compression: '$params.output_compression',
        n: '$params.n',
      },
      files: [
        { field: 'image[]', source: 'inputImages', array: true },
        { field: 'mask', source: 'mask' },
      ],
      taskIdPath: 'task_id',
      result: {
        imageUrlPaths: ['data.result.data.*.url'],
        b64JsonPaths: ['data.result.data.*.b64_json'],
      },
    },
    poll: {
      path: 'images/tasks/{task_id}',
      method: 'GET' as const,
      intervalSeconds: 3,
      statusPath: 'status',
      successValues: ['completed', 'succeeded', 'success'],
      failureValues: ['failed', 'cancelled', 'canceled'],
      errorPath: 'error.message',
      result: {
        imageUrlPaths: ['result.data.*.url'],
        b64JsonPaths: ['result.data.*.b64_json'],
      },
    },
  }
  const profile = {
    id: DEFAULT_OPENAI_PROFILE_ID,
      name: payload?.keyName?.trim() || '平台 Key',
    provider: LINGQU_ASYNC_PROVIDER_ID,
    baseUrl,
    apiKey,
    model,
    timeout: currentSettings.timeout || 600,
    apiMode,
    codexCli: false,
    apiProxy: false,
    streamImages: false,
    streamPartialImages: DEFAULT_STREAM_PARTIAL_IMAGES,
  }

  return {
    ...currentSettings,
    baseUrl: profile.baseUrl,
    apiKey: profile.apiKey,
    model: profile.model,
    apiMode: profile.apiMode,
    codexCli: false,
    apiProxy: false,
    streamImages: false,
    streamPartialImages: DEFAULT_STREAM_PARTIAL_IMAGES,
    providerOrder: undefined,
    customProviders: [provider],
    profiles: [profile],
    activeProfileId: profile.id,
  }
}
