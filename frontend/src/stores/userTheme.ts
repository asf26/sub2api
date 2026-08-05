import { defineStore } from 'pinia'
import { ref } from 'vue'

export type UserTheme = 'cartoon' | 'business'

export const USER_THEME_STORAGE_KEY = 'user-workspace-theme'
export const DEFAULT_USER_THEME: UserTheme = 'business'

export function isUserTheme(value: unknown): value is UserTheme {
  return value === 'cartoon' || value === 'business'
}

function readPersistedTheme(): UserTheme {
  if (typeof window === 'undefined') return DEFAULT_USER_THEME

  try {
    const saved = window.localStorage.getItem(USER_THEME_STORAGE_KEY)
    return isUserTheme(saved) ? saved : DEFAULT_USER_THEME
  } catch {
    return DEFAULT_USER_THEME
  }
}

export const useUserThemeStore = defineStore('userTheme', () => {
  const theme = ref<UserTheme>(readPersistedTheme())

  function setTheme(nextTheme: UserTheme) {
    theme.value = nextTheme
    if (typeof window === 'undefined') return

    try {
      window.localStorage.setItem(USER_THEME_STORAGE_KEY, nextTheme)
    } catch {
      // Theme switching should still work when storage is unavailable.
    }
  }

  return {
    theme,
    setTheme
  }
})
