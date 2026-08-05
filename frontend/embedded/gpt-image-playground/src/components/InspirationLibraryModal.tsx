import { useEffect, useMemo, useRef, useState } from 'react'
import { useStore } from '../store'
import { copyTextToClipboard, getClipboardFailureMessage } from '../lib/clipboard'
import {
  INSPIRATION_CASES,
  INSPIRATION_TEMPLATE_GROUPS,
  getFallbackInspirationLibrary,
  getInspirationCategoryLabel,
  getInspirationScenarioLabel,
  getInspirationStyleLabel,
  loadInspirationLibrary,
  type InspirationCase,
  type InspirationCategory,
  type InspirationLibraryData,
  type InspirationScenario,
  type InspirationStyle,
  type InspirationTemplate,
  type InspirationTemplateGroup,
} from '../lib/inspirationLibrary'
import { useCloseOnEscape } from '../hooks/useCloseOnEscape'
import { usePreventBackgroundScroll } from '../hooks/usePreventBackgroundScroll'
import { CloseIcon, CopyIcon, EditIcon, PlusIcon, SparklesIcon } from './icons'

interface Props {
  open: boolean
  onClose: () => void
}

type LibraryTab = 'cases' | 'trending' | 'templates'
type PromptAction = 'copy' | 'replace' | 'append'

const FALLBACK_LIBRARY = getFallbackInspirationLibrary()

function normalizeSearchText(value: string) {
  return value.trim().toLowerCase()
}

function appendPrompt(current: string, next: string) {
  const trimmedCurrent = current.trim()
  const trimmedNext = next.trim()
  if (!trimmedCurrent) return trimmedNext
  if (!trimmedNext) return trimmedCurrent
  return `${trimmedCurrent}\n\n${trimmedNext}`
}

function uniqueValues(values: string[]) {
  return Array.from(new Set(values.filter(Boolean)))
}

function formatCaseSourceLabel(source: string) {
  return source.replace(/^(GitHub|Trending)\s*\/\s*/i, '')
}

export default function InspirationLibraryModal({ open, onClose }: Props) {
  const modalRef = useRef<HTMLDivElement>(null)
  const [tab, setTab] = useState<LibraryTab>('cases')
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState<'all' | InspirationCategory>('all')
  const [style, setStyle] = useState<'all' | InspirationStyle>('all')
  const [scenario, setScenario] = useState<'all' | InspirationScenario>('all')
  const [selectedTemplateGroupId, setSelectedTemplateGroupId] = useState<InspirationCategory>('ui')
  const [libraryData, setLibraryData] = useState<InspirationLibraryData>(FALLBACK_LIBRARY)
  const [loadingLibrary, setLoadingLibrary] = useState(false)
  const [libraryError, setLibraryError] = useState<string | null>(null)
  const prompt = useStore((s) => s.prompt)
  const setPrompt = useStore((s) => s.setPrompt)
  const showToast = useStore((s) => s.showToast)

  useCloseOnEscape(open, onClose)
  usePreventBackgroundScroll(open, modalRef)

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setLoadingLibrary(true)
    setLibraryError(null)

    loadInspirationLibrary()
      .then((data) => {
        if (cancelled) return
        setLibraryData(data)
        setSelectedTemplateGroupId((current) => data.templateGroups.some((group) => group.id === current) ? current : data.templateGroups[0]?.id ?? current)
      })
      .catch((error) => {
        if (cancelled) return
        setLibraryData(FALLBACK_LIBRARY)
        setLibraryError(error instanceof Error ? error.message : String(error))
      })
      .finally(() => {
        if (!cancelled) setLoadingLibrary(false)
      })

    return () => {
      cancelled = true
    }
  }, [open])

  const activeCases = tab === 'trending' ? libraryData.trendingCases : libraryData.cases
  const categories = useMemo(() => uniqueValues(activeCases.map((item) => item.category)), [activeCases])
  const styles = useMemo(() => uniqueValues(activeCases.flatMap((item) => item.styles?.length ? item.styles : [item.style])), [activeCases])
  const scenarios = useMemo(() => uniqueValues(activeCases.flatMap((item) => item.scenarios?.length ? item.scenarios : [item.scenario])), [activeCases])
  useEffect(() => {
    if (category !== 'all' && !categories.includes(category)) setCategory('all')
  }, [categories, category])

  useEffect(() => {
    if (style !== 'all' && !styles.includes(style)) setStyle('all')
  }, [style, styles])

  useEffect(() => {
    if (scenario !== 'all' && !scenarios.includes(scenario)) setScenario('all')
  }, [scenario, scenarios])

  const filteredCases = useMemo(() => {
    const q = normalizeSearchText(query)
    return activeCases.filter((item) => {
      if (category !== 'all' && item.category !== category) return false
      if (style !== 'all' && !(item.styles?.length ? item.styles.includes(style) : item.style === style)) return false
      if (scenario !== 'all' && !(item.scenarios?.length ? item.scenarios.includes(scenario) : item.scenario === scenario)) return false
      if (!q) return true
      return [
        item.title,
        item.source,
        getInspirationCategoryLabel(item.category),
        item.style,
        item.scenario,
        item.prompt,
        item.promptPreview,
        item.githubUrl,
        item.sourceUrl,
        ...item.tags,
      ].join(' ').toLowerCase().includes(q)
    })
  }, [activeCases, category, query, scenario, style])

  const templateGroups = useMemo(() => {
    const q = normalizeSearchText(query)
    if (!q || tab !== 'templates') return libraryData.templateGroups
    return libraryData.templateGroups.filter((group) =>
      [
        group.title,
        group.description,
        ...group.tags,
        ...group.templates.flatMap((item) => [item.title, item.kind, item.content]),
      ].join(' ').toLowerCase().includes(q),
    )
  }, [libraryData.templateGroups, query, tab])

  const selectedTemplateGroup = templateGroups.find((group) => group.id === selectedTemplateGroupId) ?? templateGroups[0] ?? libraryData.templateGroups[0] ?? INSPIRATION_TEMPLATE_GROUPS[0]

  if (!open) return null

  const handlePromptAction = async (text: string, action: PromptAction) => {
    const content = text.trim()
    if (!content) return

    if (action === 'copy') {
      try {
        await copyTextToClipboard(content)
        showToast('提示词已复制', 'success')
      } catch (err) {
        showToast(getClipboardFailureMessage('复制提示词失败', err), 'error')
      }
      return
    }

    setPrompt(action === 'replace' ? content : appendPrompt(prompt, content))
    showToast(action === 'replace' ? '已替换当前提示词' : '已追加到提示词', 'success')
    onClose()
  }

  const handleTabChange = (nextTab: LibraryTab) => {
    setTab(nextTab)
    setQuery('')
    setCategory('all')
    setStyle('all')
    setScenario('all')
  }

  return (
    <div data-no-drag-select className="fixed inset-0 z-[75] flex items-center justify-center p-3 sm:p-6">
      <div className="absolute inset-0 bg-black/30 backdrop-blur-sm animate-overlay-in" onClick={onClose} />
      <div
        ref={modalRef}
        className="relative z-10 flex h-[88vh] w-full max-w-7xl flex-col overflow-hidden rounded-3xl border border-white/50 bg-white/95 shadow-2xl ring-1 ring-black/5 animate-modal-in dark:border-white/[0.08] dark:bg-gray-900/95 dark:ring-white/10"
      >
        <header className="border-b border-gray-200/70 px-4 py-4 dark:border-white/[0.08] sm:px-6">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="mb-1 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-blue-500 dark:text-blue-300">
                <SparklesIcon className="h-3.5 w-3.5" />
                Prompt Library
              </div>
              <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">灵感库 / 模板库</h2>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">按分类找案例，或直接套用结构化模板，不再从空白提示词开始。</p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="rounded-full p-2 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500/30 dark:hover:bg-white/[0.06] dark:hover:text-gray-200"
              aria-label="关闭灵感库"
              title="关闭"
            >
              <CloseIcon className="h-5 w-5" />
            </button>
          </div>

          <div className="mt-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex w-full rounded-2xl bg-gray-100/80 p-1 dark:bg-white/[0.04] sm:w-auto">
              <button
                type="button"
                onClick={() => handleTabChange('cases')}
                className={`flex-1 rounded-xl px-4 py-2 text-sm font-medium transition sm:flex-none ${
                  tab === 'cases'
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-700 dark:text-gray-100'
                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
              >
                案例库
              </button>
              <button
                type="button"
                onClick={() => handleTabChange('trending')}
                className={`flex-1 rounded-xl px-4 py-2 text-sm font-medium transition sm:flex-none ${
                  tab === 'trending'
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-700 dark:text-gray-100'
                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
              >
                热门库
              </button>
              <button
                type="button"
                onClick={() => handleTabChange('templates')}
                className={`flex-1 rounded-xl px-4 py-2 text-sm font-medium transition sm:flex-none ${
                  tab === 'templates'
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-700 dark:text-gray-100'
                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
              >
                模板库
              </button>
            </div>

            <div className="relative min-w-0 flex-1 lg:max-w-md">
              <svg className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={tab === 'templates' ? '搜索模板标题、内容、避坑指南...' : '搜索标题、Prompt、标签...'}
                className="w-full rounded-2xl border border-gray-200/70 bg-white/70 py-2.5 pl-10 pr-4 text-sm outline-none transition focus:border-blue-400 focus:ring-2 focus:ring-blue-500/20 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-100"
              />
            </div>
          </div>

          <div className="mt-4 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
            <StatPill label={`案例 ${libraryData.meta.totalCases ?? libraryData.cases.length}`} />
            <StatPill label={`热门 ${libraryData.trendingMeta?.totalCases ?? libraryData.trendingCases.length}`} />
            <StatPill label={`模板分类 ${libraryData.meta.totalTemplateCategories ?? libraryData.templateGroups.length}`} />
            {libraryData.meta.license && <StatPill label={`许可 ${libraryData.meta.license}`} />}
            {libraryData.trendingMeta?.license && <StatPill label={`热门许可 ${libraryData.trendingMeta.license}`} />}
            <StatPill label="支持复制 / 替换 / 追加" />
          </div>
          {libraryError && (
            <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-200">
              资源库加载失败，已使用内置兜底数据：{libraryError}
            </div>
          )}
        </header>

        {loadingLibrary ? (
          <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-gray-400 dark:text-gray-500">正在加载 prompt-library 资源库...</div>
        ) : tab === 'cases' || tab === 'trending' ? (
          <CaseLibrary
            cases={filteredCases}
            categories={categories}
            styles={styles}
            scenarios={scenarios}
            category={category}
            style={style}
            scenario={scenario}
            onCategoryChange={setCategory}
            onStyleChange={setStyle}
            onScenarioChange={setScenario}
            onAction={handlePromptAction}
            emptyText={tab === 'trending' ? '没有匹配的热门提示词' : '没有匹配的灵感案例'}
          />
        ) : (
          <TemplateLibrary
            groups={templateGroups}
            selectedGroup={selectedTemplateGroup}
            onSelectGroup={(id) => setSelectedTemplateGroupId(id)}
            onAction={handlePromptAction}
          />
        )}
      </div>
    </div>
  )
}

function StatPill({ label }: { label: string }) {
  return (
    <span className="rounded-full border border-gray-200/70 bg-gray-50 px-2.5 py-1 dark:border-white/[0.08] dark:bg-white/[0.04]">
      {label}
    </span>
  )
}

function CaseLibrary({
  cases,
  categories,
  styles,
  scenarios,
  category,
  style,
  scenario,
  onCategoryChange,
  onStyleChange,
  onScenarioChange,
  onAction,
  emptyText,
}: {
  cases: InspirationCase[]
  categories: InspirationCategory[]
  styles: InspirationStyle[]
  scenarios: InspirationScenario[]
  category: 'all' | InspirationCategory
  style: 'all' | InspirationStyle
  scenario: 'all' | InspirationScenario
  onCategoryChange: (value: 'all' | InspirationCategory) => void
  onStyleChange: (value: 'all' | InspirationStyle) => void
  onScenarioChange: (value: 'all' | InspirationScenario) => void
  onAction: (text: string, action: PromptAction) => void
  emptyText: string
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex flex-col gap-3 border-b border-gray-200/70 px-4 py-3 dark:border-white/[0.08] sm:px-6 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-wrap gap-2">
          <FilterSelect value={category} onChange={(value) => onCategoryChange(value as 'all' | InspirationCategory)} options={[['all', '全部分类'], ...categories.map((item) => [item, getInspirationCategoryLabel(item)] as const)]} />
          <FilterSelect value={style} onChange={(value) => onStyleChange(value as 'all' | InspirationStyle)} options={[['all', '全部风格'], ...styles.map((item) => [item, getInspirationStyleLabel(item)] as const)]} />
          <FilterSelect value={scenario} onChange={(value) => onScenarioChange(value as 'all' | InspirationScenario)} options={[['all', '全部场景'], ...scenarios.map((item) => [item, getInspirationScenarioLabel(item)] as const)]} />
        </div>
        <div className="text-sm text-gray-400 dark:text-gray-500">匹配 {cases.length} 条案例</div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto custom-scrollbar px-4 py-4 sm:px-6">
        {cases.length === 0 ? (
          <div className="flex h-full min-h-[240px] items-center justify-center text-sm text-gray-400 dark:text-gray-500">{emptyText}</div>
        ) : (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {cases.map((item) => (
              <CaseCard key={item.id} item={item} onAction={onAction} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function FilterSelect({
  value,
  onChange,
  options,
}: {
  value: string
  onChange: (value: string) => void
  options: ReadonlyArray<readonly [string, string]>
}) {
  return (
    <select
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className="rounded-xl border border-gray-200/70 bg-white/70 px-3 py-2 text-sm text-gray-700 outline-none transition focus:border-blue-400 focus:ring-2 focus:ring-blue-500/20 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-200"
    >
      {options.map(([optionValue, label]) => (
        <option key={optionValue} value={optionValue}>{label}</option>
      ))}
    </select>
  )
}

function CaseCard({ item, onAction }: { item: InspirationCase; onAction: (text: string, action: PromptAction) => void }) {
  const categoryLabel = getInspirationCategoryLabel(item.category)
  const styleLabel = getInspirationStyleLabel(item.style)
  const sourceLabel = formatCaseSourceLabel(item.source)
  return (
    <article className="flex min-h-[360px] flex-col overflow-hidden rounded-2xl border border-gray-200/70 bg-white/70 shadow-sm transition hover:-translate-y-0.5 hover:shadow-lg dark:border-white/[0.08] dark:bg-white/[0.03]">
      <div className="relative h-36 overflow-hidden bg-gray-100 dark:bg-white/[0.04] sm:h-40">
        {item.thumbnailUrl ? (
          <img
            src={item.thumbnailUrl}
            alt={item.title}
            className="h-full w-full object-cover"
            loading="lazy"
            referrerPolicy="no-referrer"
          />
        ) : (
          <div className={`absolute inset-0 ${getPreviewClass(item.category)}`} />
        )}
        <div className="absolute inset-0 bg-[linear-gradient(135deg,rgba(255,255,255,0.16),rgba(255,255,255,0)_45%,rgba(0,0,0,0.24))]" />
        <div className="absolute bottom-3 left-3 rounded-full border border-white/40 bg-black/25 px-2.5 py-1 text-xs font-medium text-white backdrop-blur">{categoryLabel}</div>
      </div>
      <div className="flex flex-1 flex-col p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="line-clamp-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{item.title}</h3>
            {sourceLabel && <p className="mt-1 text-xs text-gray-400 dark:text-gray-500">{sourceLabel}</p>}
          </div>
          <span className="shrink-0 rounded-full bg-blue-50 px-2 py-1 text-[11px] font-medium text-blue-600 dark:bg-blue-500/10 dark:text-blue-300">{styleLabel}</span>
        </div>
        <div className="mt-3 flex flex-wrap gap-1.5">
          {[getInspirationScenarioLabel(item.scenario), ...item.tags.slice(0, 2)].map((tag, tagIndex) => (
            <span key={`${tag}-${tagIndex}`} className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] text-gray-500 dark:bg-white/[0.05] dark:text-gray-400">{tag}</span>
          ))}
        </div>
        <p className="mt-3 line-clamp-5 flex-1 text-xs leading-5 text-gray-600 dark:text-gray-300">{item.promptPreview || item.prompt}</p>
        <ActionRow text={item.prompt} onAction={onAction} />
        {(item.githubUrl || item.remoteImageUrl || item.sourceUrl) && (
          <div className="mt-3 flex flex-wrap gap-3 text-xs">
            {item.githubUrl && <a href={item.githubUrl} target="_blank" rel="noopener noreferrer" className="text-gray-400 transition hover:text-blue-500 dark:text-gray-500 dark:hover:text-blue-300">GitHub</a>}
            {item.remoteImageUrl && <a href={item.remoteImageUrl} target="_blank" rel="noopener noreferrer" className="text-gray-400 transition hover:text-blue-500 dark:text-gray-500 dark:hover:text-blue-300">原图</a>}
            {item.sourceUrl && <a href={item.sourceUrl} target="_blank" rel="noopener noreferrer" className="text-gray-400 transition hover:text-blue-500 dark:text-gray-500 dark:hover:text-blue-300">来源</a>}
          </div>
        )}
      </div>
    </article>
  )
}

function TemplateLibrary({
  groups,
  selectedGroup,
  onSelectGroup,
  onAction,
}: {
  groups: InspirationTemplateGroup[]
  selectedGroup: InspirationTemplateGroup
  onSelectGroup: (id: InspirationCategory) => void
  onAction: (text: string, action: PromptAction) => void
}) {
  return (
    <div className="grid min-h-0 flex-1 grid-rows-[auto_1fr] lg:grid-cols-[280px_1fr] lg:grid-rows-1">
      <aside className="border-b border-gray-200/70 p-4 dark:border-white/[0.08] lg:min-h-0 lg:overflow-y-auto lg:border-b-0 lg:border-r lg:p-5 custom-scrollbar">
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-1">
          {groups.map((group) => (
            <button
              key={group.id}
              type="button"
              onClick={() => onSelectGroup(group.id)}
              className={`rounded-2xl border p-3 text-left transition ${
                group.id === selectedGroup.id
                  ? 'border-blue-300 bg-blue-50 text-blue-700 shadow-sm dark:border-blue-500/40 dark:bg-blue-500/10 dark:text-blue-200'
                  : 'border-gray-200/70 bg-white/70 text-gray-700 hover:bg-gray-50 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-300 dark:hover:bg-white/[0.06]'
              }`}
            >
              <div className="text-sm font-semibold">{group.title}</div>
              {group.coverUrl && (
                <div className="mt-3 aspect-[16/9] overflow-hidden rounded-xl bg-gray-100 dark:bg-white/[0.04]">
                  <img
                    src={group.coverUrl}
                    alt={group.title}
                    className="h-full w-full object-cover"
                    loading="lazy"
                    referrerPolicy="no-referrer"
                  />
                </div>
              )}
              <div className="mt-2 flex flex-wrap gap-1">
                {group.tags.map((tag) => (
                  <span key={tag} className="rounded-full bg-white/70 px-1.5 py-0.5 text-[10px] text-gray-500 dark:bg-white/[0.06] dark:text-gray-400">{tag}</span>
                ))}
              </div>
              <div className="mt-2 text-xs text-gray-400 dark:text-gray-500">{group.templates.length} 个条目</div>
            </button>
          ))}
        </div>
      </aside>

      <section className="min-h-0 overflow-y-auto custom-scrollbar p-4 sm:p-6">
        {groups.length === 0 ? (
          <div className="flex h-full min-h-[240px] items-center justify-center text-sm text-gray-400 dark:text-gray-500">没有匹配的模板分类</div>
        ) : (
          <>
            <div className="mb-5">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">{selectedGroup.title}</h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{selectedGroup.description}</p>
            </div>
            <div className="space-y-4">
              {selectedGroup.templates.map((item) => (
                <TemplateCard key={item.id} item={item} onAction={onAction} />
              ))}
            </div>
          </>
        )}
      </section>
    </div>
  )
}

function TemplateCard({ item, onAction }: { item: InspirationTemplate; onAction: (text: string, action: PromptAction) => void }) {
  const canApply = item.kind !== '避坑指南'
  return (
    <article className="overflow-hidden rounded-2xl border border-gray-200/70 bg-white/70 shadow-sm dark:border-white/[0.08] dark:bg-white/[0.03]">
      <div className="flex flex-col gap-3 border-b border-gray-200/70 p-4 dark:border-white/[0.08] sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="text-sm font-semibold text-gray-900 dark:text-gray-100">{item.title}</div>
          <div className="mt-1 text-xs text-gray-400 dark:text-gray-500">{item.kind}</div>
        </div>
        <ActionRow text={item.content} onAction={onAction} compact allowApply={canApply} />
      </div>
      <pre className="max-h-[360px] overflow-auto whitespace-pre-wrap break-words p-4 text-xs leading-6 text-gray-700 dark:text-gray-300 custom-scrollbar">
        {item.content}
      </pre>
    </article>
  )
}

function ActionRow({ text, onAction, compact = false, allowApply = true }: { text: string; onAction: (text: string, action: PromptAction) => void; compact?: boolean; allowApply?: boolean }) {
  const buttonClass = compact
    ? 'inline-flex items-center gap-1.5 rounded-lg border border-gray-200/70 px-2.5 py-1.5 text-xs font-medium text-gray-600 transition hover:bg-gray-50 dark:border-white/[0.08] dark:text-gray-300 dark:hover:bg-white/[0.06]'
    : 'inline-flex items-center gap-1.5 rounded-lg border border-gray-200/70 px-2.5 py-1.5 text-xs font-medium text-gray-600 transition hover:bg-gray-50 dark:border-white/[0.08] dark:text-gray-300 dark:hover:bg-white/[0.06]'

  return (
    <div className={`mt-4 flex flex-wrap gap-2 ${compact ? 'mt-0' : ''}`}>
      <button type="button" className={buttonClass} onClick={() => onAction(text, 'copy')}>
        <CopyIcon className="h-3.5 w-3.5" />
        复制
      </button>
      {allowApply && (
        <>
          <button type="button" className={buttonClass} onClick={() => onAction(text, 'replace')}>
            <EditIcon className="h-3.5 w-3.5" />
            替换
          </button>
          <button type="button" className={buttonClass} onClick={() => onAction(text, 'append')}>
            <PlusIcon className="h-3.5 w-3.5" />
            追加
          </button>
        </>
      )}
    </div>
  )
}

function getPreviewClass(category: InspirationCategory) {
  switch (category) {
    case 'ui':
      return 'bg-[linear-gradient(135deg,#111827,#1d4ed8)]'
    case 'poster':
      return 'bg-[linear-gradient(135deg,#7f1d1d,#f59e0b)]'
    case 'product':
      return 'bg-[linear-gradient(135deg,#0f172a,#0d9488)]'
    case 'brand':
      return 'bg-[linear-gradient(135deg,#14532d,#f5f5f4)]'
    case 'photo':
      return 'bg-[linear-gradient(135deg,#1f2937,#d6d3d1)]'
    case 'character':
      return 'bg-[linear-gradient(135deg,#581c87,#f0abfc)]'
    case 'scene':
      return 'bg-[linear-gradient(135deg,#1e1b4b,#f97316)]'
    case 'infographic':
      return 'bg-[linear-gradient(135deg,#0f766e,#f8fafc)]'
    case 'illustration':
      return 'bg-[linear-gradient(135deg,#be123c,#fef3c7)]'
  }
}
