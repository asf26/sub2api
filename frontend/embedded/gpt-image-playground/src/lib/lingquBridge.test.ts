import { describe, expect, it } from 'vitest'
import { DEFAULT_SETTINGS, normalizeSettings } from './apiProfiles'
import { buildLingquSettings, isLingquBridgeSettings, LINGQU_ASYNC_PROVIDER_ID } from './lingquBridge'

describe('Lingqu bridge settings', () => {
  it('keeps the Lingqu async provider as the active profile', () => {
    const settings = normalizeSettings({
      ...DEFAULT_SETTINGS,
      ...buildLingquSettings(DEFAULT_SETTINGS, {
        apiUrl: 'https://lingqu.example.com/v1',
        apiKey: 'lingqu-key',
        keyName: '生图专用分组',
      }),
    })
    const activeProfile = settings.profiles.find((profile) => profile.id === settings.activeProfileId)

    expect(isLingquBridgeSettings(settings)).toBe(true)
    expect(activeProfile).toMatchObject({
      provider: LINGQU_ASYNC_PROVIDER_ID,
      apiKey: 'lingqu-key',
      name: '生图专用分组',
    })
    expect(settings.customProviders[0]).toMatchObject({
      id: LINGQU_ASYNC_PROVIDER_ID,
      submit: expect.objectContaining({
        path: 'images/generations/async',
        taskIdPath: 'task_id',
      }),
      poll: expect.objectContaining({
        path: 'images/tasks/{task_id}',
        statusPath: 'status',
      }),
    })
  })
})
