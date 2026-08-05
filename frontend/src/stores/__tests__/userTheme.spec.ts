import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import {
  DEFAULT_USER_THEME,
  USER_THEME_STORAGE_KEY,
  useUserThemeStore
} from '@/stores/userTheme'

describe('useUserThemeStore', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('defaults to the business theme', () => {
    expect(DEFAULT_USER_THEME).toBe('business')
    expect(useUserThemeStore().theme).toBe('business')
  })

  it('restores a valid persisted theme', () => {
    localStorage.setItem(USER_THEME_STORAGE_KEY, 'cartoon')
    setActivePinia(createPinia())

    expect(useUserThemeStore().theme).toBe('cartoon')
  })

  it('migrates the removed Claude theme to the business default', () => {
    localStorage.setItem(USER_THEME_STORAGE_KEY, 'claude')
    setActivePinia(createPinia())

    expect(useUserThemeStore().theme).toBe('business')
  })

  it('persists theme changes', () => {
    const store = useUserThemeStore()

    store.setTheme('business')

    expect(store.theme).toBe('business')
    expect(localStorage.getItem(USER_THEME_STORAGE_KEY)).toBe('business')
  })
})
