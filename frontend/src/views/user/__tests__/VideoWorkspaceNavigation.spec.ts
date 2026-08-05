import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import layoutSource from '@/components/layout/UserWorkspaceLayout.vue?raw'
import routerSource from '@/router/index.ts?raw'
import studioSource from '../VideoStudioView.vue?raw'
import docsSource from '../VideoAPIDocsView.vue?raw'

const testDirectory = dirname(fileURLToPath(import.meta.url))
const themeSource = readFileSync(resolve(testDirectory, '../../../styles/user-themes.css'), 'utf8')

describe('video user workspace navigation', () => {
  it('keeps creation and API docs directly after the image workshop in both themes', () => {
    const expectedOrder = /path: '\/images'[\s\S]*?path: '\/videos'[\s\S]*?path: '\/docs\/video-api'[\s\S]*?path: '\/monitor'/g
    expect(layoutSource.match(expectedOrder)).toHaveLength(2)
    expect(layoutSource).toContain("label: '视频创作', icon: 'play'")
    expect(layoutSource).toContain("label: '视频 API', icon: 'book'")
  })

  it('registers authenticated user routes for both pages', () => {
    expect(routerSource).toMatch(/path: '\/videos'[\s\S]*?requiresAuth: true[\s\S]*?requiresAdmin: false/)
    expect(routerSource).toMatch(/path: '\/docs\/video-api'[\s\S]*?requiresAuth: true[\s\S]*?requiresAdmin: false/)
  })

  it('uses the user workspace shell and current platform origin', () => {
    expect(studioSource).toMatch(/<template>\s*<UserWorkspaceLayout>/)
    expect(docsSource).toMatch(/<template>\s*<UserWorkspaceLayout>/)
    expect(docsSource).toContain('window.location.origin')
    expect(docsSource).not.toContain('sub2.pokexiao.com')
  })

  it('keeps business-theme layout rules outside scoped page styles', () => {
    expect(studioSource).not.toContain(':global(.user-workspace')
    expect(docsSource).not.toContain(':global(.user-workspace')
    expect(themeSource).toContain(".user-workspace[data-user-theme='business'] .video-studio")
    expect(themeSource).toContain(".user-workspace[data-user-theme='business'] .video-docs")
  })
})
