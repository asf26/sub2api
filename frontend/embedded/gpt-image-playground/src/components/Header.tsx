import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { useTooltip } from '../hooks/useTooltip'
import { dismissAllTooltips } from '../lib/tooltipDismiss'
import ViewportTooltip from './ViewportTooltip'
import HelpModal from './HelpModal'
import { useFavoriteCollectionTitle } from './FavoriteCollections'
import { HelpCircleIcon, SettingsIcon, SparklesIcon } from './icons'

const BRAND_NAME = 'AI 图工坊'
const BRAND_HOME_URL = '/images'

type BeforeInstallPromptEvent = Event & {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>
}

function isInstalledPwa() {
  const nav = window.navigator as Navigator & { standalone?: boolean }
  return window.matchMedia('(display-mode: standalone)').matches || nav.standalone === true
}

export default function Header() {
  const setShowSettings = useStore((s) => s.setShowSettings)
  const setConfirmDialog = useStore((s) => s.setConfirmDialog)
  const filterFavorite = useStore((s) => s.filterFavorite)
  const activeFavoriteCollectionId = useStore((s) => s.activeFavoriteCollectionId)
  const favoriteCollectionTitle = useFavoriteCollectionTitle()
  const showFavoriteCollectionTitle = Boolean(activeFavoriteCollectionId)
  const [showHelp, setShowHelp] = useState(false)
  const [installPrompt, setInstallPrompt] = useState<BeforeInstallPromptEvent | null>(null)
  const [isPwaInstalled, setIsPwaInstalled] = useState(isInstalledPwa)

  const installTooltip = useTooltip()
  const helpTooltip = useTooltip()
  const settingsTooltip = useTooltip()

  useEffect(() => {
    const handleBeforeInstallPrompt = (event: Event) => {
      event.preventDefault()
      setInstallPrompt(event as BeforeInstallPromptEvent)
      setIsPwaInstalled(false)
    }

    const handleAppInstalled = () => {
      setInstallPrompt(null)
      setIsPwaInstalled(true)
    }

    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
    window.addEventListener('appinstalled', handleAppInstalled)

    return () => {
      window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
      window.removeEventListener('appinstalled', handleAppInstalled)
    }
  }, [])

  const handleInstallClick = async () => {
    if (installPrompt) {
      const promptEvent = installPrompt
      setInstallPrompt(null)

      try {
        await promptEvent.prompt()
        const choice = await promptEvent.userChoice
        setIsPwaInstalled(choice.outcome === 'accepted')
      } catch {
        setIsPwaInstalled(isInstalledPwa())
      }
    } else {
      const isIos = /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
      if (isIos) {
        setConfirmDialog({
          title: '安装为应用',
          message: '在 Safari 浏览器中，点击底部「分享」按钮，选择「添加到主屏幕」即可安装此应用。',
          showCancel: false,
          confirmText: '我知道了',
          icon: 'info',
          action: () => {},
        })
      } else {
        setConfirmDialog({
          title: '安装为应用',
          message: '请在浏览器的菜单中选择「添加到主屏幕」或「安装应用」。\n\n（如果在微信等内置浏览器中，请先在外部浏览器打开）',
          showCancel: false,
          confirmText: '我知道了',
          icon: 'info',
          action: () => {},
        })
      }
    }
  }

  return (
    <>
      <header data-no-drag-select className="safe-area-top fixed top-0 left-0 right-0 z-40 translate-y-0 bg-white/80 backdrop-blur border-b border-gray-200 transition-transform duration-300 ease-in-out">
        <div className="safe-area-x safe-header-inner max-w-7xl mx-auto flex items-center justify-between relative">
          <div className="flex-1 min-w-0 pr-2 flex items-center gap-3">
            <a
              href={BRAND_HOME_URL}
              target="_parent"
              rel="noopener noreferrer"
              className="lingqu-header-logo shrink-0"
              aria-label="返回图工坊"
            >
              <SparklesIcon />
            </a>
            <h1 className="inline-grid min-w-0 items-start relative mr-2">
              {showFavoriteCollectionTitle ? (
                <>
                  <span className="min-w-0 truncate text-[17px] font-bold tracking-tight text-gray-800 sm:hidden" title={favoriteCollectionTitle}>{favoriteCollectionTitle}</span>
                  <a
                    href={BRAND_HOME_URL}
                    target="_parent"
                    rel="noopener noreferrer"
                    className="hidden text-lg font-bold tracking-tight text-gray-800 transition-colors hover:text-gray-600 sm:inline"
                  >
                    {BRAND_NAME}
                  </a>
                </>
              ) : (
                <a
                  href={BRAND_HOME_URL}
                  target="_parent"
                  rel="noopener noreferrer"
                  className="text-[17px] sm:text-lg font-bold tracking-tight text-gray-800 hover:text-gray-600 transition-colors"
                >
                  {BRAND_NAME}
                </a>
              )}
              <span className="hidden text-[11px] font-bold leading-none text-gray-500 sm:block">一个 Key 接入各大顶尖图像模型</span>
            </h1>
          </div>
          {showFavoriteCollectionTitle && (
            <div className="absolute left-1/2 top-1/2 hidden max-w-[30%] -translate-x-1/2 -translate-y-1/2 sm:flex">
              <div className="truncate rounded px-2 py-1 text-sm font-semibold text-gray-700" title={favoriteCollectionTitle}>
                {favoriteCollectionTitle}
              </div>
            </div>
          )}
          <div className="flex items-center gap-1 shrink-0">
            {!isPwaInstalled && window.self === window.top && (
              <div className="relative" {...installTooltip.handlers}>
                <button
                  onClick={() => {
                    dismissAllTooltips()
                    handleInstallClick()
                  }}
                  className="lingqu-toolbar-button p-2 rounded-lg transition-colors"
                  aria-label="安装为应用"
                >
                  <svg className="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" viewBox="0 0 24 24">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="7 10 12 15 17 10" />
                    <line x1="12" y1="15" x2="12" y2="3" />
                  </svg>
                </button>
                <ViewportTooltip visible={installTooltip.visible} className="whitespace-nowrap">
                  安装为应用
                </ViewportTooltip>
              </div>
            )}
            <div className="relative" {...helpTooltip.handlers}>
              <button
                onClick={() => {
                  dismissAllTooltips()
                  setShowHelp(true)
                }}
                className="lingqu-toolbar-button p-2 rounded-lg transition-colors"
                aria-label="操作指南"
              >
                <HelpCircleIcon className="w-5 h-5 text-gray-600" />
              </button>
              <ViewportTooltip visible={helpTooltip.visible} className="whitespace-nowrap">
                操作指南
              </ViewportTooltip>
            </div>
            <div className="relative" {...settingsTooltip.handlers}>
              <button
                onClick={() => setShowSettings(true)}
                className="lingqu-toolbar-button p-2 rounded-lg transition-colors"
                aria-label="设置"
              >
                <SettingsIcon className="w-5 h-5 text-gray-600" />
              </button>
              <ViewportTooltip visible={settingsTooltip.visible} className="whitespace-nowrap">
                设置
              </ViewportTooltip>
            </div>
          </div>
        </div>
      </header>

      <div className="safe-area-top invisible pointer-events-none max-h-[500px] opacity-100 transition-all duration-300 ease-in-out" aria-hidden="true">
        <div className="safe-header-inner" />
      </div>

      {showHelp && <HelpModal appMode="gallery" isFavoriteCollectionOverview={filterFavorite && !activeFavoriteCollectionId} onClose={() => setShowHelp(false)} />}
    </>
  )
}
