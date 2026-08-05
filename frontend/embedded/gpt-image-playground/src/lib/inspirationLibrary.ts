export type InspirationCategory = string
export type InspirationStyle = string
export type InspirationScenario = string

export interface InspirationCase {
  id: string
  title: string
  source: string
  category: InspirationCategory
  style: InspirationStyle
  scenario: InspirationScenario
  styles?: InspirationStyle[]
  scenarios?: InspirationScenario[]
  tags: string[]
  prompt: string
  promptPreview?: string
  githubUrl?: string
  remoteImageUrl?: string
  sourceUrl?: string
  thumbnailUrl?: string
  featured?: boolean
  rank?: number
  likes?: number
  views?: number
}

export interface InspirationTemplate {
  id: string
  title: string
  kind: '文本模板' | 'JSON 模板' | '避坑指南'
  content: string
}

export interface InspirationTemplateGroup {
  id: InspirationCategory
  title: string
  description: string
  tags: string[]
  coverUrl?: string
  templates: InspirationTemplate[]
}

export interface InspirationLibraryMeta {
  repository?: string
  syncedAt?: string
  license?: string
  totalCases?: number
  totalTemplateCategories?: number
}

export interface InspirationLibraryData {
  cases: InspirationCase[]
  trendingCases: InspirationCase[]
  templateGroups: InspirationTemplateGroup[]
  meta: InspirationLibraryMeta
  trendingMeta?: InspirationLibraryMeta
  source: 'prompt-library' | 'fallback'
}

export const PROMPT_LIBRARY_REPOSITORY = 'https://github.com/freestylefly/awesome-gpt-image-2'
export const TRENDING_PROMPTS_REPOSITORY = 'https://github.com/jau123/nanobanana-trending-prompts'

export const INSPIRATION_CATEGORY_LABELS: Record<string, string> = {
  ui: 'UI 与界面',
  poster: '海报与排版',
  product: '商品与电商',
  brand: '品牌与标志',
  photo: '摄影与写实',
  character: '人物与角色',
  scene: '场景与叙事',
  infographic: '图表与信息',
  illustration: '插画与艺术',
  architecture: '建筑与空间',
  history: '历史与古风',
  document: '文档与出版物',
  other: '其他场景',
  'UI & Interfaces': 'UI 与界面',
  'Posters & Typography': '海报与排版',
  'Products & E-commerce': '商品与电商',
  'Brand & Logos': '品牌与标志',
  'Photography & Realism': '摄影与写实',
  'Characters & People': '人物与角色',
  'Scenes & Storytelling': '场景与叙事',
  'Charts & Infographics': '图表与信息',
  'Illustration & Art': '插画与艺术',
  'Architecture & Spaces': '建筑与空间',
  'History & Classical Themes': '历史与古风',
  'Documents & Publishing': '文档与出版物',
  'Other Use Cases': '其他场景',
  'UI & Graphic': 'UI 与图形',
  'Product & E-commerce': '商品与电商',
  'Photography': '摄影',
  'Character': '人物与角色',
  'Creative': '创意视觉',
  'Architecture': '建筑与空间',
  'Fashion': '时尚',
  'Food': '美食',
  'Education': '教育',
}

export const INSPIRATION_STYLE_LABELS: Record<string, string> = {
  UI: '界面',
  Realistic: '写实',
  Photography: '摄影',
  Poster: '海报',
  Illustration: '插画',
  '3D': '3D',
  Brand: '品牌',
  Product: '商品',
  Products: '商品',
  Character: '角色',
  Characters: '角色',
  Infographic: '信息图',
  Charts: '图表',
  Scenes: '场景',
  History: '历史',
  Classical: '古风',
  Architecture: '建筑',
  Documents: '文档',
  'Other Use Cases': '其他',
}

export const INSPIRATION_SCENARIO_LABELS: Record<string, string> = {
  Tech: '科技',
  Commerce: '商业',
  Story: '叙事',
  Travel: '旅行',
  Fashion: '时尚',
  Food: '美食',
  Education: '教育',
  Social: '社媒',
  History: '历史',
  Creative: '创意',
}

export const INSPIRATION_STYLES: InspirationStyle[] = ['写实', '插画', '3D', '海报', '界面', '信息图', '品牌']
export const INSPIRATION_SCENARIOS: InspirationScenario[] = ['商业', '社媒', '电商', '产品', '叙事', '知识', '角色']

export const INSPIRATION_CASES: InspirationCase[] = [
  {
    id: 'mobile-finance-dashboard',
    title: '高端移动金融仪表盘',
    source: '内置模板',
    category: 'ui',
    style: '界面',
    scenario: '商业',
    tags: ['移动端', '深色模式', '数据卡片'],
    prompt:
      '生成一张高保真的移动端金融 App 首页截图，比例 9:16，深色模式。顶部显示资产总览与收益趋势，中部是三张圆角数据卡片，底部是交易列表和清晰的导航栏。视觉风格克制、专业、适合高频使用，所有中文 UI 文案清晰可读，按钮和数字不要乱码。',
  },
  {
    id: 'ai-tool-launch-poster',
    title: 'AI 工具发布海报',
    source: '内置模板',
    category: 'poster',
    style: '海报',
    scenario: '社媒',
    tags: ['发布', '科技', '竖版'],
    prompt:
      '为一款名为「灵犀画布」的 AI 生图工具生成竖版 4:5 发布海报。画面中心是一块发光的工作台界面，周围漂浮着图层、参考图、参数面板与生成结果缩略图。标题文字为「灵犀画布」，副标题为「把一句话变成可交付视觉」。整体要现代、清爽、有产品发布感，文字必须清晰。',
  },
  {
    id: 'beauty-product-hero',
    title: '护肤品电商英雄图',
    source: '内置模板',
    category: 'product',
    style: '写实',
    scenario: '电商',
    tags: ['护肤', '详情页', '商业摄影'],
    prompt:
      '生成一张护肤精华瓶的电商详情页主视觉，横版 16:9。产品瓶身位于画面中央偏右，透明玻璃材质，标签文字为「AURORA SERUM」。背景是浅灰白实验室台面和柔和晨光，左侧保留干净留白用于卖点文案。画面质感写实、干净、高级，产品边缘清晰，反光自然。',
  },
  {
    id: 'brand-touchpoint-board',
    title: '咖啡品牌触点视觉板',
    source: '内置模板',
    category: 'brand',
    style: '品牌',
    scenario: '商业',
    tags: ['VI', '咖啡', '品牌系统'],
    prompt:
      '为精品咖啡品牌「北窗咖啡」生成一张品牌触点系统视觉板，比例 16:9。包含纸杯、咖啡豆包装、菜单卡、会员卡、外卖袋和门店小招牌。品牌调性是安静、城市感、温暖但不复古，主色为深墨绿和象牙白，点缀铜色。所有物料要统一，像真实品牌提案板。',
  },
  {
    id: 'editorial-portrait',
    title: '杂志感人物写真',
    source: '内置模板',
    category: 'photo',
    style: '写实',
    scenario: '社媒',
    tags: ['人像', '自然光', '杂志'],
    prompt:
      '生成一张写实杂志人像，年轻创意总监坐在工作室窗边，穿深灰羊毛外套，手边有速写本和咖啡。自然侧光打在脸上，背景是柔焦的书架和作品板。构图为半身肖像，色彩低饱和，皮肤质感自然，表情放松而专注，像高端生活方式杂志内页。',
  },
  {
    id: 'collectible-character-sheet',
    title: '收藏级角色设定表',
    source: '内置模板',
    category: 'character',
    style: '3D',
    scenario: '角色',
    tags: ['角色', '玩具', '设定表'],
    prompt:
      '生成一张收藏级 3D 角色设定表，角色是一位未来城市邮差，穿轻量机能外套和磁悬浮背包。画面包含正面全身、侧面小图、表情三连、背包道具拆解和配色样本。风格介于高端潮玩与影视概念设计之间，背景干净，标注文字清晰，整体像产品研发板。',
  },
  {
    id: 'fantasy-market-scene',
    title: '奇幻集市叙事场景',
    source: '内置模板',
    category: 'scene',
    style: '插画',
    scenario: '叙事',
    tags: ['奇幻', '场景', '电影感'],
    prompt:
      '生成一张奇幻城市夜市的宽幅场景图，比例 21:9。雨后的石板路反射暖色灯笼，摊位售卖发光药草、机械鸟和手写地图。前景有一位披斗篷的旅行者正在和摊主低声交谈，远处是高耸桥梁和漂浮电车。画面要有叙事感、空间层次、电影光影，不要拼贴。',
  },
  {
    id: 'knowledge-card-tea',
    title: '茶叶风味知识卡',
    source: '内置模板',
    category: 'infographic',
    style: '信息图',
    scenario: '知识',
    tags: ['知识卡', '茶', '小红书'],
    prompt:
      '生成一张竖版 9:16 茶叶风味知识卡，主题为「乌龙茶香气地图」。包含主标题、茶叶插画、四个香气分区、冲泡参数、风味雷达图和简短建议。中文文字必须清晰可读，版式像高级生活方式知识卡，颜色使用米白、乌龙茶褐、少量青绿色，避免信息拥挤。',
  },
  {
    id: 'watercolor-city-map',
    title: '手绘城市美食地图',
    source: '内置模板',
    category: 'illustration',
    style: '插画',
    scenario: '社媒',
    tags: ['地图', '美食', '水彩'],
    prompt:
      '生成一张手绘水彩城市美食地图，主题为「厦门周末散步地图」。以温柔鸟瞰视角画出海岸线、老街、小巷和码头，标注 8 个美食点：沙茶面、花生汤、海蛎煎、烧肉粽、土笋冻、芋包、馅饼、手冲咖啡。地图不需要精准比例，要可爱、清爽、适合社媒收藏。',
  },
]

export const INSPIRATION_TEMPLATE_GROUPS: InspirationTemplateGroup[] = [
  {
    id: 'ui',
    title: 'UI 与界面',
    description: '适合生成 App 截图、Web 控制台、SaaS 工作台和产品界面。',
    tags: ['UI', '截图', '界面'],
    templates: [
      {
        id: 'ui-basic',
        title: '常规模板',
        kind: '文本模板',
        content:
          '为[产品类型]生成一张[平台：iOS / Android / Web]界面截图。\n核心功能：[功能点A]、[功能点B]、[功能点C]。\n视觉风格：[极简 / 专业 / 拟物 / 科技]，主色为[颜色]，强调色为[颜色]。\n布局要求：[顶部导航 / 底部标签栏 / 双栏 / 卡片流]，信息层级清晰，按钮状态明确。\n输出要求：高保真 UI 截图，中文文字清晰可读，比例[9:16 / 16:9]。',
      },
      {
        id: 'ui-json',
        title: 'JSON 进阶模板',
        kind: 'JSON 模板',
        content:
          '{\n  "type": "UI Screenshot",\n  "platform": "Web Dashboard",\n  "product": "[产品名称]",\n  "layout": "[布局结构]",\n  "style": {\n    "theme": "[Light / Dark]",\n    "primary_color": "[主色]",\n    "density": "compact but readable"\n  },\n  "content": {\n    "header": "[主标题]",\n    "cards": ["[卡片1]", "[卡片2]", "[卡片3]"],\n    "table_or_feed": "[列表内容]"\n  },\n  "constraints": "High fidelity, readable Chinese text, no nonsense UI copy"\n}',
      },
      {
        id: 'ui-pitfall',
        title: '避坑指南',
        kind: '避坑指南',
        content:
          '不要只写“做一个好看的界面”。请明确平台、比例、导航结构、核心数据、按钮文案和视觉密度。额外强调“文字必须清晰可读，不要乱码，不要无意义按钮”。',
      },
    ],
  },
  {
    id: 'poster',
    title: '海报与排版',
    description: '适合活动海报、产品发布图、社媒封面和主题视觉。',
    tags: ['海报', '排版', '视觉'],
    templates: [
      {
        id: 'poster-basic',
        title: '主题海报模板',
        kind: '文本模板',
        content:
          '生成一张[主题]竖版海报，比例[4:5 / 9:16]。\n主标题文字：[标题]；副标题：[副标题]。\n画面主体：[核心视觉对象]，背景氛围：[场景/光线/材质]。\n排版要求：标题清晰、有层级，留白克制，视觉焦点明确。\n风格方向：[电影感 / 国潮 / 极简 / 复古印刷 / 科技发布]。',
      },
      {
        id: 'poster-pitfall',
        title: '避坑指南',
        kind: '避坑指南',
        content:
          '海报要指定“标题原文”和“文字位置”，否则容易出现错字或乱码。不要同时塞太多主体，最好一个主视觉加 2-3 个辅助元素。',
      },
    ],
  },
  {
    id: 'product',
    title: '商品与电商',
    description: '适合商品主图、详情页视觉、广告 KV 和场景化卖点图。',
    tags: ['商品', '电商', '详情页'],
    templates: [
      {
        id: 'product-hero',
        title: '商品主视觉模板',
        kind: '文本模板',
        content:
          '为[商品名称]生成一张电商主视觉，比例[1:1 / 4:5 / 16:9]。\n商品外观：[材质、颜色、包装细节]。\n卖点氛围：[清爽 / 高端 / 专业 / 年轻 / 户外]。\n构图：商品占画面[比例]，背景为[场景]，留出[左侧/上方]文案区域。\n要求：真实商业摄影质感，产品边缘清晰，标签文字可读，光影自然。',
      },
      {
        id: 'product-json',
        title: '详情页 JSON 模板',
        kind: 'JSON 模板',
        content:
          '{\n  "type": "E-commerce Hero Image",\n  "product": "[商品名称]",\n  "audience": "[目标人群]",\n  "scene": "[使用场景]",\n  "composition": "center hero product, benefit callouts, clean negative space",\n  "copy": ["[卖点1]", "[卖点2]", "[卖点3]"],\n  "style": "premium commercial photography, sharp label text, realistic reflections"\n}',
      },
    ],
  },
  {
    id: 'brand',
    title: '品牌与标志',
    description: '适合品牌提案板、Logo 应用、包装系统和视觉识别。',
    tags: ['品牌', '标志', '识别系统'],
    templates: [
      {
        id: 'brand-board',
        title: '品牌触点系统',
        kind: '文本模板',
        content:
          '为品牌「[品牌名]」生成一张品牌触点系统视觉板。\n品牌关键词：[关键词1]、[关键词2]、[关键词3]。\n包含物料：[包装]、[名片]、[海报]、[门店标牌]、[社媒封面]。\n色彩系统：主色[颜色]，辅助色[颜色]，背景[颜色/材质]。\n要求：所有物料统一、有真实提案感，Logo 和文字清晰，像成熟品牌识别系统。',
      },
    ],
  },
  {
    id: 'photo',
    title: '摄影与写实',
    description: '适合真实照片、人像、产品摄影和生活方式场景。',
    tags: ['摄影', '写实', '镜头'],
    templates: [
      {
        id: 'photo-shot',
        title: '镜头语言模板',
        kind: '文本模板',
        content:
          '生成一张写实摄影作品，主题为[主题]。\n镜头：[35mm / 50mm / 85mm / 微距 / 鱼眼]，景别：[特写 / 半身 / 全身 / 远景]。\n光线：[自然侧光 / 黄金时刻 / 柔光箱 / 夜景霓虹]。\n环境：[地点与道具]。\n质感要求：真实皮肤/材质纹理，自然景深，不要过度磨皮，不要塑料感。',
      },
    ],
  },
  {
    id: 'character',
    title: '人物与角色',
    description: '适合角色设定、潮玩设计、服装概念和表情设定。',
    tags: ['人物', '角色', '设定'],
    templates: [
      {
        id: 'character-sheet',
        title: '角色设定表',
        kind: '文本模板',
        content:
          '生成一张角色设定表，角色是[身份/职业/世界观]。\n包含：正面全身、侧面小图、三种表情、关键道具、配色样本、材质说明。\n风格：[半写实 / 3D 潮玩 / 日系插画 / 影视概念设计]。\n要求：角色一致性强，服装结构清晰，背景干净，标注文字可读。',
      },
    ],
  },
  {
    id: 'infographic',
    title: '图表与信息',
    description: '适合知识卡、科普图、数据可视化和结构化说明图。',
    tags: ['信息图', '图表', '知识卡'],
    templates: [
      {
        id: 'infographic-card',
        title: '科普知识卡',
        kind: '文本模板',
        content:
          '生成一张竖版 9:16 科普知识卡，主题为[主题]。\n版式包含：主标题、主题插画、3-5 个信息模块、一个小图表、重点总结。\n视觉风格：[博物图鉴 / 现代百科 / 社媒收藏卡]。\n要求：中文文字清晰，模块边界明确，信息密度适中，不要把文字堆成一团。',
      },
    ],
  },
]

export function getInspirationCaseCount() {
  return INSPIRATION_CASES.length
}

export function getInspirationTemplateCount() {
  return INSPIRATION_TEMPLATE_GROUPS.reduce((sum, group) => sum + group.templates.length, 0)
}

interface PromptLibraryCaseRecord {
  id?: string | number
  title?: string
  category?: string
  styles?: string[]
  scenes?: string[]
  prompt?: string
  promptPreview?: string
  sourceLabel?: string
  sourceUrl?: string
  githubUrl?: string
  thumbnailSrc?: string
  remoteImageUrl?: string
  featured?: boolean
}

interface PromptLibraryTemplateEntryRecord {
  id?: string
  title?: string
  kind?: string
  content?: string
}

interface PromptLibraryTemplateGroupRecord {
  id?: string
  title?: string
  description?: string
  coverSrc?: string
  tags?: string[]
  entries?: PromptLibraryTemplateEntryRecord[]
}

interface TrendingPromptRecord {
  rank?: number
  id?: string | number
  prompt?: string
  author?: string
  author_name?: string
  likes?: number
  views?: number
  image?: string
  images?: string[]
  model?: string
  categories?: string[]
  rating?: number
  score?: number
  date?: string
  source_url?: string
}

const PROMPT_LIBRARY_BASE_PATH = `${import.meta.env.BASE_URL}prompt-library/`

function labelFromMap(map: Record<string, string>, value: string) {
  return map[value] ?? value
}

export function getInspirationCategoryLabel(category: string) {
  return labelFromMap(INSPIRATION_CATEGORY_LABELS, category)
}

export function getInspirationStyleLabel(style: string) {
  return labelFromMap(INSPIRATION_STYLE_LABELS, style)
}

export function getInspirationScenarioLabel(scenario: string) {
  return labelFromMap(INSPIRATION_SCENARIO_LABELS, scenario)
}

function resolvePromptLibraryAsset(src?: string) {
  if (!src) return undefined
  if (/^https?:\/\//i.test(src)) return src
  return `${PROMPT_LIBRARY_BASE_PATH}${src.replace(/^\/+/, '')}`
}

function normalizeTemplateKind(kind?: string): InspirationTemplate['kind'] {
  if (kind === 'json') return 'JSON 模板'
  if (kind === 'tips') return '避坑指南'
  return '文本模板'
}

function isNonNull<T>(value: T | null): value is T {
  return value !== null
}

function normalizePromptLibraryCase(record: PromptLibraryCaseRecord): InspirationCase | null {
  const prompt = typeof record.prompt === 'string' ? record.prompt.trim() : ''
  const title = typeof record.title === 'string' ? record.title.trim() : ''
  if (!prompt || !title) return null

  const styles = Array.isArray(record.styles) ? record.styles.filter(Boolean) : []
  const scenarios = Array.isArray(record.scenes) ? record.scenes.filter(Boolean) : []
  const category = record.category || 'Other Use Cases'
  const style = styles[0] ?? 'Other Use Cases'
  const scenario = scenarios[0] ?? 'Creative'
  const source = record.sourceLabel ? `GitHub / ${record.sourceLabel}` : 'GitHub'

  return {
    id: String(record.id ?? title),
    title,
    source,
    category,
    style,
    scenario,
    styles,
    scenarios,
    tags: [...styles, ...scenarios].map((tag) => labelFromMap({ ...INSPIRATION_STYLE_LABELS, ...INSPIRATION_SCENARIO_LABELS }, tag)),
    prompt,
    promptPreview: record.promptPreview,
    githubUrl: record.githubUrl,
    remoteImageUrl: record.remoteImageUrl,
    sourceUrl: record.sourceUrl,
    thumbnailUrl: record.remoteImageUrl ?? resolvePromptLibraryAsset(record.thumbnailSrc),
    featured: Boolean(record.featured),
  }
}

function normalizePromptLibraryTemplateGroup(record: PromptLibraryTemplateGroupRecord): InspirationTemplateGroup | null {
  const id = typeof record.id === 'string' && record.id.trim() ? record.id.trim() : ''
  const title = typeof record.title === 'string' && record.title.trim() ? record.title.trim() : ''
  if (!id || !title) return null

  const templates = (Array.isArray(record.entries) ? record.entries : [])
    .map((entry) => {
      const content = typeof entry.content === 'string' ? entry.content.trim() : ''
      const entryTitle = typeof entry.title === 'string' ? entry.title.trim() : ''
      if (!content || !entryTitle) return null
      return {
        id: entry.id || `${id}-${entryTitle}`,
        title: entryTitle,
        kind: normalizeTemplateKind(entry.kind),
        content,
      } satisfies InspirationTemplate
    })
    .filter(isNonNull)

  return {
    id,
    title,
    description: record.description || `${title} 的提示词模板与避坑指南。`,
    tags: Array.isArray(record.tags) ? record.tags.filter(Boolean) : [],
    coverUrl: resolvePromptLibraryAsset(record.coverSrc),
    templates,
  }
}

function compactCount(value?: number) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null
  if (value >= 10000) return `${Math.round(value / 1000) / 10}w`
  if (value >= 1000) return `${Math.round(value / 100) / 10}k`
  return String(value)
}

function normalizeTrendingPrompt(record: TrendingPromptRecord): InspirationCase | null {
  const prompt = typeof record.prompt === 'string' ? record.prompt.trim() : ''
  if (!prompt) return null

  const categories = Array.isArray(record.categories) && record.categories.length > 0 ? record.categories.filter(Boolean) : ['Creative']
  const category = categories[0]
  const titleBase = prompt.replace(/\s+/g, ' ').slice(0, 30)
  const title = `#${record.rank ?? record.id ?? 'Trend'} ${titleBase}${prompt.length > 30 ? '...' : ''}`
  const author = record.author_name || record.author
  const image = record.image || record.images?.[0]
  const likes = compactCount(record.likes)
  const views = compactCount(record.views)
  const tags = [
    ...categories.map(getInspirationCategoryLabel),
    record.model ? record.model.toUpperCase() : null,
    likes ? `赞 ${likes}` : null,
    views ? `看 ${views}` : null,
  ].filter((tag): tag is string => Boolean(tag))

  return {
    id: String(record.id ?? record.rank ?? titleBase),
    title,
    source: author ? `Trending / @${author}` : 'Trending Prompts',
    category,
    style: category,
    scenario: 'Social',
    styles: categories,
    scenarios: ['Social'],
    tags,
    prompt,
    promptPreview: prompt,
    sourceUrl: record.source_url,
    remoteImageUrl: image,
    thumbnailUrl: image,
    featured: typeof record.rank === 'number' ? record.rank <= 24 : false,
    rank: record.rank,
    likes: record.likes,
    views: record.views,
  }
}

async function fetchPromptLibraryJson<T>(fileName: string): Promise<T> {
  const response = await fetch(`${PROMPT_LIBRARY_BASE_PATH}${fileName}`, { cache: 'no-store' })
  if (!response.ok) throw new Error(`资源库文件加载失败：${fileName}`)
  return response.json() as Promise<T>
}

export function getFallbackInspirationLibrary(): InspirationLibraryData {
  return {
    cases: INSPIRATION_CASES,
    trendingCases: [],
    templateGroups: INSPIRATION_TEMPLATE_GROUPS,
    meta: {
      repository: PROMPT_LIBRARY_REPOSITORY,
      license: 'MIT',
      totalCases: INSPIRATION_CASES.length,
      totalTemplateCategories: INSPIRATION_TEMPLATE_GROUPS.length,
    },
    trendingMeta: {
      repository: TRENDING_PROMPTS_REPOSITORY,
      license: 'CC BY 4.0',
      totalCases: 0,
    },
    source: 'fallback',
  }
}

export async function loadInspirationLibrary(): Promise<InspirationLibraryData> {
  const [meta, caseRecords, templateRecords, trendingRecords] = await Promise.all([
    fetchPromptLibraryJson<InspirationLibraryMeta>('meta.json'),
    fetchPromptLibraryJson<PromptLibraryCaseRecord[]>('cases.json'),
    fetchPromptLibraryJson<PromptLibraryTemplateGroupRecord[]>('templates.json'),
    fetchPromptLibraryJson<TrendingPromptRecord[]>('trending-prompts.json'),
  ])

  const cases = caseRecords.map(normalizePromptLibraryCase).filter(isNonNull)
  const trendingCases = trendingRecords.map(normalizeTrendingPrompt).filter(isNonNull)
  const templateGroups = templateRecords
    .map(normalizePromptLibraryTemplateGroup)
    .filter(isNonNull)
    .filter((item) => item.templates.length > 0)

  return {
    cases,
    trendingCases,
    templateGroups,
    meta: {
      repository: meta.repository || PROMPT_LIBRARY_REPOSITORY,
      syncedAt: meta.syncedAt,
      license: meta.license || 'MIT',
      totalCases: meta.totalCases ?? cases.length,
      totalTemplateCategories: meta.totalTemplateCategories ?? templateGroups.length,
    },
    trendingMeta: {
      repository: TRENDING_PROMPTS_REPOSITORY,
      license: 'CC BY 4.0',
      totalCases: trendingCases.length,
    },
    source: 'prompt-library',
  }
}
