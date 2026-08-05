import { useEffect, useState } from 'react'
import { initStore } from './store'
import { useStore } from './store'
import { buildSettingsFromUrlParams, clearUrlSettingParams, hasUrlSettingParams } from './lib/urlSettings'
import { isSystemApiSettings, mergeImportedSettings, normalizeSettings, normalizeSystemApiSettings } from './lib/apiProfiles'
import { getCustomProviderConfigUrl, loadCustomProviderSettingsFromUrl } from './lib/customProviderConfigUrl'
import { buildLingquSettings, getLingquBridgePayload, isLingquBridgeSettings } from './lib/lingquBridge'
import { useDockerApiUrlMigrationNotice } from './hooks/useDockerApiUrlMigrationNotice'
import Header from './components/Header'
import SearchBar from './components/SearchBar'
import TaskGrid from './components/TaskGrid'
import InputBar from './components/InputBar'
import DetailModal from './components/DetailModal'
import Lightbox from './components/Lightbox'
import SettingsModal from './components/SettingsModal'
import ConfirmDialog from './components/ConfirmDialog'
import Toast from './components/Toast'
import MaskEditorModal from './components/MaskEditorModal'
import ImageContextMenu from './components/ImageContextMenu'
import SupportPromptModal from './components/SupportPromptModal'
import InspirationLibraryModal from './components/InspirationLibraryModal'
import { FavoriteCollectionPickerModal, FavoriteCollectionsView, ManageCollectionsModal } from './components/FavoriteCollections'
import { useGlobalClickSuppression } from './lib/clickSuppression'

let customProviderConfigUrlImportStarted = false

export default function App() {
  const setSettings = useStore((s) => s.setSettings)
  const settings = useStore((s) => s.settings)
  const appMode = useStore((s) => s.appMode)
  const setAppMode = useStore((s) => s.setAppMode)
  const filterFavorite = useStore((s) => s.filterFavorite)
  const activeFavoriteCollectionId = useStore((s) => s.activeFavoriteCollectionId)
  const [showInspirationLibrary, setShowInspirationLibrary] = useState(false)
  useDockerApiUrlMigrationNotice()
  useGlobalClickSuppression()

  useEffect(() => {
    if (appMode === 'agent') setAppMode('gallery')
  }, [appMode, setAppMode])

  useEffect(() => {
    if (!isSystemApiSettings(settings) && !isLingquBridgeSettings(settings)) {
      setSettings(normalizeSystemApiSettings(settings))
    }
  }, [settings, setSettings])

  useEffect(() => {
    const searchParams = new URLSearchParams(window.location.search)
    const currentSettings = useStore.getState().settings
    const nextSettings = buildSettingsFromUrlParams(currentSettings, searchParams)
    const bridgePayload = getLingquBridgePayload()
    const bridgedSettings = buildLingquSettings(
      { ...currentSettings, ...nextSettings },
      bridgePayload,
    )
    const mergedSettings = { ...currentSettings, ...nextSettings, ...bridgedSettings }

    setSettings(
      isLingquBridgeSettings(mergedSettings)
        ? normalizeSettings(mergedSettings)
        : normalizeSystemApiSettings(mergedSettings),
    )

    if (hasUrlSettingParams(searchParams)) {
      clearUrlSettingParams(searchParams)

      const nextSearch = searchParams.toString()
      const nextUrl = `${window.location.pathname}${nextSearch ? `?${nextSearch}` : ''}${window.location.hash}`
      window.history.replaceState(null, '', nextUrl)
    }

    const customProviderConfigUrl = getCustomProviderConfigUrl()
    if (customProviderConfigUrl && !customProviderConfigUrlImportStarted) {
      customProviderConfigUrlImportStarted = true
      void loadCustomProviderSettingsFromUrl(customProviderConfigUrl)
        .then((importedSettings) => {
          if (!importedSettings) return
          const state = useStore.getState()
          state.setSettings(normalizeSystemApiSettings(mergeImportedSettings(state.settings, importedSettings)))
        })
        .catch((error) => {
          console.warn('Failed to import custom provider config URL:', error)
        })
    }

    initStore()
  }, [setSettings])

  useEffect(() => {
    const preventPageImageDrag = (e: DragEvent) => {
      if ((e.target as HTMLElement | null)?.closest('img')) {
        e.preventDefault()
      }
    }

    document.addEventListener('dragstart', preventPageImageDrag)
    return () => document.removeEventListener('dragstart', preventPageImageDrag)
  }, [])

  return (
    <>
      <Header />
      <main data-home-main data-drag-select-surface className="pb-48">
        <div className="safe-area-x max-w-7xl mx-auto">
          <SearchBar onOpenInspirationLibrary={() => setShowInspirationLibrary(true)} />
          {filterFavorite && !activeFavoriteCollectionId ? <FavoriteCollectionsView /> : <TaskGrid />}
        </div>
      </main>
      <InputBar />
      <DetailModal />
      <Lightbox />
      <SettingsModal />
      <ConfirmDialog />
      <SupportPromptModal />
      <FavoriteCollectionPickerModal />
      <ManageCollectionsModal />
      <Toast />
      <MaskEditorModal />
      <ImageContextMenu />
      <InspirationLibraryModal open={showInspirationLibrary} onClose={() => setShowInspirationLibrary(false)} />
    </>
  )
}
