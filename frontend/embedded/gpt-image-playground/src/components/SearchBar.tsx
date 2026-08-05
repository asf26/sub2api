import { useEffect, useRef, useState } from 'react'
import { ALL_FAVORITES_COLLECTION_ID, clearFailedTasks, getTaskFavoriteCollectionIds, useStore } from '../store'
import { getActiveApiProfile } from '../lib/apiProfiles'
import Select from './Select'
import { ChevronLeftIcon, CollectionManageIcon, FavoriteIcon, SparklesIcon, TrashIcon } from './icons'

interface SearchBarProps {
  onOpenInspirationLibrary?: () => void
}

export default function SearchBar({ onOpenInspirationLibrary }: SearchBarProps) {
  const rootRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const [showApiKey, setShowApiKey] = useState(false)
  const settings = useStore((s) => s.settings)
  const setSettings = useStore((s) => s.setSettings)
  const searchQuery = useStore((s) => s.searchQuery)
  const setSearchQuery = useStore((s) => s.setSearchQuery)
  const filterStatus = useStore((s) => s.filterStatus)
  const setFilterStatus = useStore((s) => s.setFilterStatus)
  const clearSelection = useStore((s) => s.clearSelection)
  const filterFavorite = useStore((s) => s.filterFavorite)
  const setFilterFavorite = useStore((s) => s.setFilterFavorite)
  const activeFavoriteCollectionId = useStore((s) => s.activeFavoriteCollectionId)
  const setActiveFavoriteCollectionId = useStore((s) => s.setActiveFavoriteCollectionId)
  const openManageCollectionsModal = useStore((s) => s.openManageCollectionsModal)
  const failedCount = useStore((s) => {
    const q = s.searchQuery.trim().toLowerCase()
    return s.tasks.filter((task) => {
      if (task.status !== 'error') return false
      if (s.filterFavorite) {
        if (!task.isFavorite) return false
        if (s.activeFavoriteCollectionId && s.activeFavoriteCollectionId !== ALL_FAVORITES_COLLECTION_ID && !getTaskFavoriteCollectionIds(task).includes(s.activeFavoriteCollectionId)) return false
      }
      if (!q) return true
      const prompt = (task.prompt || '').toLowerCase()
      const paramStr = JSON.stringify(task.params).toLowerCase()
      return prompt.includes(q) || paramStr.includes(q)
    }).length
  })
  const setConfirmDialog = useStore((s) => s.setConfirmDialog)
  const inCollectionOverview = filterFavorite && !activeFavoriteCollectionId
  const isFailedFilter = filterStatus === 'error'
  const activeProfile = getActiveApiProfile(settings)

  useEffect(() => {
    const handleDocumentMouseDown = (event: MouseEvent) => {
      if (document.activeElement !== inputRef.current) return

      const target = event.target instanceof Element ? event.target : document.elementFromPoint(event.clientX, event.clientY)
      if (!target) return
      if (rootRef.current?.contains(target)) return
      if (!target.closest('[data-drag-select-surface]')) return
      if (target.closest('.task-card-wrapper, .favorite-collection-card-wrapper')) return

      inputRef.current?.blur()
    }

    document.addEventListener('mousedown', handleDocumentMouseDown, true)
    return () => document.removeEventListener('mousedown', handleDocumentMouseDown, true)
  }, [])

  const handleFavoriteClick = () => {
    if (activeFavoriteCollectionId) {
      setActiveFavoriteCollectionId(null)
      return
    }
    setFilterFavorite(!filterFavorite)
  }

  const handleClearFailed = () => {
    const state = useStore.getState()
    const q = state.searchQuery.trim().toLowerCase()
    const failedTaskIds = state.tasks
      .filter((task) => {
        if (task.status !== 'error') return false
        if (state.filterFavorite) {
          if (!task.isFavorite) return false
          if (state.activeFavoriteCollectionId && state.activeFavoriteCollectionId !== ALL_FAVORITES_COLLECTION_ID && !getTaskFavoriteCollectionIds(task).includes(state.activeFavoriteCollectionId)) return false
        }
        if (!q) return true
        const prompt = (task.prompt || '').toLowerCase()
        const paramStr = JSON.stringify(task.params).toLowerCase()
        return prompt.includes(q) || paramStr.includes(q)
      })
      .map((task) => task.id)
    const failedTaskCount = failedTaskIds.length
    if (failedTaskCount === 0) return

    setConfirmDialog({
      title: '清除失败记录',
      message: `确定清除筛选范围内的失败记录吗？\n将删除 ${failedTaskCount} 条失败记录，关联的孤立图片资源也会被清理。`,
      confirmText: '删除',
      cancelText: '取消',
      tone: 'danger',
      action: () => clearFailedTasks(failedTaskIds),
    })
  }

  const handleStatusChange = (val: any) => {
    if (val === filterStatus) return
    setFilterStatus(val)
    clearSelection()
  }

  return (
    <div ref={rootRef} data-no-drag-select data-image-toolbar className="mt-4 mb-4 flex flex-wrap gap-3 rounded-[22px] border border-[rgba(33,31,28,0.14)] bg-white/75 p-3 shadow-[0_12px_28px_rgba(33,31,28,0.07)] backdrop-blur">
      <div className="flex gap-2 flex-shrink-0 z-20">
        {onOpenInspirationLibrary && !inCollectionOverview && (
          <button
            type="button"
            onClick={onOpenInspirationLibrary}
            className="inline-flex h-[42px] items-center gap-2 rounded-xl border border-[rgba(33,31,28,0.16)] bg-[#fff5c9] px-3 text-sm font-black text-[#211f1c] shadow-[3px_3px_0_rgba(33,31,28,0.28)] transition-all hover:bg-[#ffe9a8] focus:outline-none focus:ring-2 focus:ring-[#ff5f8f]/25"
            title="打开灵感库"
          >
            <SparklesIcon className="h-4 w-4" />
            <span className="hidden sm:inline">灵感库</span>
          </button>
        )}
        <button
          onClick={handleFavoriteClick}
          className={`p-2.5 rounded-xl border transition-all ${
            filterFavorite
              ? 'border-yellow-400 bg-yellow-50 text-yellow-500 shadow-[3px_3px_0_rgba(33,31,28,0.22)]'
              : 'border-[rgba(33,31,28,0.14)] bg-white text-gray-400 hover:bg-[#fff5c9]'
          }`}
          title={activeFavoriteCollectionId ? '返回收藏夹' : filterFavorite ? '退出收藏夹视图' : '收藏夹'}
        >
          {activeFavoriteCollectionId ? <ChevronLeftIcon className="w-5 h-5" /> : <FavoriteIcon filled={filterFavorite} className="w-5 h-5" />}
        </button>
        {inCollectionOverview && (
          <button
            onClick={openManageCollectionsModal}
            className="p-2.5 rounded-xl border border-[rgba(33,31,28,0.14)] bg-white text-gray-400 hover:bg-[#fff5c9] transition-all"
            title="管理收藏夹"
          >
            <CollectionManageIcon className="w-5 h-5" />
          </button>
        )}
        {!inCollectionOverview && (
          <>
            <div className="relative w-[88px]">
              <Select
                value={filterStatus}
                onChange={handleStatusChange}
                options={[
                  { label: '全部', value: 'all' },
                  { label: '已完成', value: 'done' },
                  { label: '生成中', value: 'running' },
                  { label: '失败', value: 'error' },
                ]}
                className="px-3 py-2.5 rounded-xl border border-[rgba(33,31,28,0.14)] bg-white hover:bg-[#fffdf5] text-sm font-bold text-[#211f1c] focus:outline-none focus:ring-2 focus:ring-[#ff5f8f]/25 focus:border-[#ff5f8f] transition"
              />
            </div>
            {isFailedFilter && (
              <button
                type="button"
                onClick={handleClearFailed}
                disabled={failedCount === 0}
                title={failedCount > 0 ? `清除 ${failedCount} 条失败记录` : '没有失败记录'}
                aria-label={failedCount > 0 ? `清除 ${failedCount} 条失败记录` : '没有失败记录'}
                className="flex h-[42px] w-[42px] shrink-0 items-center justify-center rounded-xl border border-[rgba(33,31,28,0.14)] bg-white text-gray-400 transition-all hover:bg-red-50 hover:text-red-500 focus:outline-none focus:ring-2 focus:ring-[#ff5f8f]/25 disabled:cursor-not-allowed disabled:opacity-55 disabled:hover:bg-white disabled:hover:text-gray-400"
              >
                <TrashIcon className="h-[18px] w-[18px]" />
              </button>
            )}
          </>
        )}
      </div>
      <div className="relative z-10 min-w-[220px] flex-[1_1_340px]">
        <svg
          className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <input
          ref={inputRef}
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          type="text"
          placeholder={inCollectionOverview ? '搜索收藏夹名称...' : '搜索提示词、参数...'}
          className="w-full pl-10 pr-4 py-2.5 rounded-xl border border-[rgba(33,31,28,0.14)] bg-white text-sm text-[#211f1c] focus:outline-none focus:ring-2 focus:ring-[#ff5f8f]/25 focus:border-[#ff5f8f] transition placeholder:text-gray-400"
        />
      </div>
      <div className="relative z-10 flex min-w-[280px] flex-[0_1_420px] items-center overflow-hidden rounded-xl border border-[rgba(33,31,28,0.14)] bg-white shadow-[0_8px_16px_rgba(33,31,28,0.04)]">
        <span className="shrink-0 border-r border-gray-100 px-3 text-xs font-black text-gray-500">平台 Key</span>
        <input
          value={activeProfile.apiKey}
          onChange={(event) => setSettings({ apiKey: event.target.value.trim() })}
          type={showApiKey ? 'text' : 'password'}
          placeholder={activeProfile.provider === 'fal' ? 'FAL_KEY' : 'sk-...'}
          aria-label="API Key"
          className="min-w-0 flex-1 bg-transparent px-3 py-2.5 text-sm font-semibold text-gray-700 outline-none placeholder:text-gray-400"
        />
        <button
          type="button"
          onClick={() => setShowApiKey((visible) => !visible)}
          className="shrink-0 px-2.5 text-xs font-black text-gray-400 transition hover:text-gray-700"
          aria-label={showApiKey ? '隐藏 API Key' : '显示 API Key'}
        >
          {showApiKey ? '隐藏' : '显示'}
        </button>
        <a
          href="/keys?create=1"
          target="_parent"
          className="mr-1 shrink-0 rounded-lg border border-[rgba(33,31,28,0.16)] bg-[#fff5c9] px-2.5 py-1.5 text-xs font-black text-[#211f1c] transition hover:bg-[#ffe9a8]"
        >
          创建 Key
        </a>
      </div>
    </div>
  )
}
