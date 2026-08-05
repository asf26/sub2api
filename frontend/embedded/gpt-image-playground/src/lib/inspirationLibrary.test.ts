import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  getFallbackInspirationLibrary,
  getInspirationCategoryLabel,
  getInspirationScenarioLabel,
  getInspirationStyleLabel,
  loadInspirationLibrary,
} from './inspirationLibrary'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('inspirationLibrary', () => {
  it('normalizes prompt-library resources', async () => {
    const fetchMock = vi.fn(async (url: string) => {
      const fileName = url.split('/').pop()
      const fixtures: Record<string, unknown> = {
        'meta.json': {
          repository: 'https://github.com/freestylefly/awesome-gpt-image-2',
          syncedAt: '2026-05-06T14:23:00.347Z',
          license: 'MIT',
          totalCases: 392,
          totalTemplateCategories: 13,
        },
        'cases.json': [
          {
            id: 395,
            title: '骑士法师大战石像魔像',
            category: 'Scenes & Storytelling',
            styles: ['UI', 'Realistic'],
            scenes: ['Tech', 'Story'],
            prompt: 'Create a cinematic dark fantasy action scene.',
            promptPreview: 'Create a cinematic dark fantasy...',
            sourceLabel: '@RamonVi25791296',
            sourceUrl: 'https://x.com/RamonVi25791296/status/2051568239142973832',
            githubUrl: 'https://github.com/freestylefly/awesome-gpt-image-2/blob/main/docs/gallery-part-2.md#case-395',
            remoteImageUrl: 'https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/images/case395.jpg',
            thumbnailSrc: 'cases/thumbs/case395.jpg',
          },
        ],
        'templates.json': [
          {
            id: 'ui',
            title: 'UI与界面',
            coverSrc: 'covers/ui.jpg',
            tags: ['UI', '截图'],
            entries: [
              {
                id: 'tpl-ui',
                title: '常规模板',
                kind: 'text',
                content: '为[产品类型]生成一张界面图。',
              },
              {
                id: 'tpl-ui-tips',
                title: '避坑指南',
                kind: 'tips',
                content: '不要给模糊指令。',
              },
            ],
          },
        ],
        'trending-prompts.json': [
          {
            rank: 1,
            id: '2017928823497453789',
            prompt: 'Create a technical infographic of [OBJECT].',
            author: 'TechieBySA',
            author_name: 'TechieSA',
            likes: 3567,
            views: 159772,
            image: 'https://images.meigen.ai/tweets/2017928823497453789/0.jpg',
            model: 'gptimage',
            categories: ['UI & Graphic'],
            source_url: 'https://x.com/TechieBySA/status/2017928823497453789',
          },
        ],
      }

      return {
        ok: true,
        json: async () => fixtures[fileName || ''],
      } as Response
    })

    vi.stubGlobal('fetch', fetchMock)

    const library = await loadInspirationLibrary()

    expect(library.source).toBe('prompt-library')
    expect(library.meta.totalCases).toBe(392)
    expect(library.trendingMeta).toMatchObject({
      repository: 'https://github.com/jau123/nanobanana-trending-prompts',
      license: 'CC BY 4.0',
      totalCases: 1,
    })
    expect(library.cases[0]).toMatchObject({
      id: '395',
      source: 'GitHub / @RamonVi25791296',
      category: 'Scenes & Storytelling',
      style: 'UI',
      scenario: 'Tech',
      thumbnailUrl: 'https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/images/case395.jpg',
    })
    expect(library.cases[0].tags).toContain('界面')
    expect(library.cases[0].tags).toContain('科技')
    expect(library.templateGroups[0].coverUrl).toContain('prompt-library/covers/ui.jpg')
    expect(library.templateGroups[0].templates[1].kind).toBe('避坑指南')
    expect(library.trendingCases[0]).toMatchObject({
      id: '2017928823497453789',
      source: 'Trending / @TechieSA',
      category: 'UI & Graphic',
      style: 'UI & Graphic',
      scenario: 'Social',
      thumbnailUrl: 'https://images.meigen.ai/tweets/2017928823497453789/0.jpg',
      sourceUrl: 'https://x.com/TechieBySA/status/2017928823497453789',
      rank: 1,
      likes: 3567,
      views: 159772,
    })
    expect(library.trendingCases[0].tags).toContain('UI 与图形')
    expect(library.trendingCases[0].tags).toContain('GPTIMAGE')
  })

  it('keeps a bundled fallback library', () => {
    const library = getFallbackInspirationLibrary()

    expect(library.source).toBe('fallback')
    expect(library.cases.length).toBeGreaterThan(0)
    expect(library.trendingCases).toEqual([])
    expect(library.templateGroups.length).toBeGreaterThan(0)
  })

  it('labels upstream categories and tags in Chinese', () => {
    expect(getInspirationCategoryLabel('Scenes & Storytelling')).toBe('场景与叙事')
    expect(getInspirationCategoryLabel('UI & Graphic')).toBe('UI 与图形')
    expect(getInspirationStyleLabel('Realistic')).toBe('写实')
    expect(getInspirationScenarioLabel('Commerce')).toBe('商业')
  })
})
