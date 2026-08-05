<template>
  <UserWorkspaceLayout>
    <div class="video-studio">
      <header class="video-studio__header">
        <div>
          <span class="video-studio__eyebrow"><Icon name="play" size="sm" /> AI 视频创作</span>
          <h1>把灵感变成有声画面</h1>
          <p>选择自己的视频 Key，模型、任务、计费与素材都通过当前平台完成。</p>
        </div>
        <div class="video-studio__keybar">
          <label>
            <span>创作 Key</span>
            <select v-model="selectedKeyId" :disabled="loadingKeys || videoKeys.length === 0">
              <option value="">选择 XiaoAPI 分组 Key</option>
              <option v-for="key in videoKeys" :key="key.id" :value="String(key.id)">
                {{ key.name }} · {{ key.group?.name || '视频分组' }}
              </option>
            </select>
          </label>
          <button type="button" class="video-icon-button" title="刷新 Key" :disabled="loadingKeys" @click="loadKeys">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingKeys }" />
          </button>
          <router-link to="/docs/video-api" class="video-doc-link">
            <Icon name="book" size="sm" /> API 文档
          </router-link>
        </div>
      </header>

      <div v-if="!selectedKey && !loadingKeys" class="video-studio__empty">
        <span><Icon name="key" size="xl" /></span>
        <strong>{{ videoKeys.length ? '选择一个视频 Key 开始创作' : '还没有可用的视频 Key' }}</strong>
        <p>{{ videoKeys.length ? '模型和历史任务会按照所选 Key 隔离。' : '先创建 XiaoAPI 分组 Key，再回到这里使用视频工作台。' }}</p>
        <router-link v-if="videoKeys.length === 0" to="/keys?create=1">创建 Key</router-link>
      </div>

      <div v-else class="video-studio__workspace">
        <aside class="video-studio__models" aria-label="视频模型">
          <div class="video-panel-title">
            <div>
              <span>模型</span>
              <strong>{{ capabilities.length }} 个可用</strong>
            </div>
            <button type="button" class="video-icon-button" title="刷新模型" :disabled="loadingModels" @click="loadWorkspace">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingModels }" />
            </button>
          </div>
          <div v-if="loadingModels" class="video-studio__loading">正在读取平台模型…</div>
          <button
            v-for="item in capabilities"
            v-else
            :key="item.id"
            type="button"
            class="video-model-option"
            :class="{ 'video-model-option--active': selectedModelId === item.id }"
            @click="selectedModelId = item.id"
          >
            <span class="video-model-option__icon"><Icon name="play" size="sm" /></span>
            <span>
              <strong>{{ item.label }}</strong>
              <small>{{ item.resolutions.join(' · ') }}</small>
            </span>
            <Icon v-if="selectedModelId === item.id" name="checkCircle" size="sm" />
          </button>
          <div v-if="!loadingModels && capabilities.length === 0" class="video-studio__loading">
            当前 Key 没有已定价的视频模型
          </div>
        </aside>

        <main class="video-composer">
          <div class="video-composer__topline">
            <div>
              <span>创作方式</span>
              <strong>{{ selectedCapability?.label || '请选择模型' }}</strong>
            </div>
            <span v-if="retryingSameRequest" class="video-retry-badge">可安全重试</span>
          </div>

          <div class="video-mode-tabs" role="tablist" aria-label="创作方式">
            <button
              v-for="item in availableModes"
              :key="item.value"
              type="button"
              role="tab"
              :aria-selected="creationMode === item.value"
              :class="{ 'video-mode-tab--active': creationMode === item.value }"
              @click="creationMode = item.value"
            >
              <Icon :name="item.icon" size="sm" />
              <span>{{ item.label }}</span>
            </button>
          </div>

          <section v-if="creationMode === 'frames'" class="video-media-band" aria-label="首尾帧">
            <MediaInput
              label="首帧"
              hint="PNG / JPEG / WebP，最大 10 MiB"
              accept="image/png,image/jpeg,image/webp"
              :required="selectedCapability?.requiresStartFrame"
              :item="startFrame"
              @select="setSingleMedia('start', $event)"
              @remove="clearSingleMedia('start')"
            />
            <MediaInput
              v-if="selectedCapability?.supportsEndFrame"
              label="尾帧"
              hint="让镜头自然过渡到目标构图"
              accept="image/png,image/jpeg,image/webp"
              :item="endFrame"
              @select="setSingleMedia('end', $event)"
              @remove="clearSingleMedia('end')"
            />
          </section>

          <section v-if="creationMode === 'references'" class="video-reference-section" aria-label="参考素材">
            <div class="video-reference-row" v-if="referenceLimit('image') > 0">
              <div><strong>参考图片</strong><span>最多 {{ referenceLimit('image') }} 张</span></div>
              <label class="video-add-media">
                <Icon name="plus" size="sm" /><span>添加图片</span>
                <input type="file" multiple accept="image/png,image/jpeg,image/webp" @change="addReferenceFiles('image', $event)" />
              </label>
            </div>
            <div v-if="referenceImages.length" class="video-media-list">
              <MediaChip v-for="item in referenceImages" :key="item.id" :item="item" @remove="removeReference('image', item.id)" />
            </div>

            <div class="video-reference-row" v-if="referenceLimit('video') > 0">
              <div><strong>参考视频</strong><span>MP4 / MOV，最大 100 MiB</span></div>
              <label class="video-add-media">
                <Icon name="plus" size="sm" /><span>添加视频</span>
                <input type="file" multiple accept="video/mp4,video/quicktime" @change="addReferenceFiles('video', $event)" />
              </label>
            </div>
            <div v-if="referenceVideos.length" class="video-media-list">
              <MediaChip v-for="item in referenceVideos" :key="item.id" :item="item" @remove="removeReference('video', item.id)" />
            </div>

            <div class="video-reference-row" v-if="referenceLimit('audio') > 0">
              <div><strong>参考音频</strong><span>MP3 / WAV，需同时提供图片或视频</span></div>
              <label class="video-add-media">
                <Icon name="plus" size="sm" /><span>添加音频</span>
                <input type="file" accept="audio/mpeg,audio/wav,audio/x-wav" @change="addReferenceFiles('audio', $event)" />
              </label>
            </div>
            <div v-if="referenceAudios.length" class="video-media-list">
              <MediaChip v-for="item in referenceAudios" :key="item.id" :item="item" @remove="removeReference('audio', item.id)" />
            </div>
          </section>

          <label class="video-prompt">
            <span>
              <strong>画面描述</strong>
              <small>{{ prompt.length }} / {{ selectedCapability?.promptLimit || 5000 }}</small>
            </span>
            <textarea
              v-model="prompt"
              :maxlength="selectedCapability?.promptLimit || 5000"
              rows="6"
              placeholder="描述主体、动作、环境、镜头运动、光线和整体风格…"
            ></textarea>
          </label>

          <section class="video-settings">
            <div class="video-setting video-setting--wide">
              <span>分辨率</span>
              <div class="video-segments">
                <button
                  v-for="item in selectedCapability?.resolutions || []"
                  :key="item"
                  type="button"
                  :class="{ 'video-segment--active': resolution === item }"
                  @click="resolution = item"
                >{{ item }}</button>
              </div>
            </div>

            <div class="video-setting">
              <span>时长</span>
              <div class="video-duration">
                <input v-model.number="durationIndex" type="range" min="0" :max="Math.max(0, durationOptions.length - 1)" step="1" />
                <strong>{{ duration }} 秒</strong>
              </div>
            </div>

            <div class="video-setting video-setting--wide">
              <span>画面比例</span>
              <div class="video-segments video-segments--ratios">
                <button
                  v-for="item in aspectRatioOptions"
                  :key="item"
                  type="button"
                  :class="{ 'video-segment--active': aspectRatio === item }"
                  @click="aspectRatio = item"
                >{{ item }}</button>
              </div>
            </div>

            <label v-if="selectedCapability?.supportsPromptEnhance" class="video-setting">
              <span>提示词增强</span>
              <select v-model="promptEnhance">
                <option value="AUTO">自动</option>
                <option value="ON" :disabled="selectedModelId === 'happy-horse-1.1' && Boolean(startFrame)">开启</option>
                <option value="OFF">关闭</option>
              </select>
            </label>

            <label v-if="selectedCapability?.supportsAudio" class="video-audio-toggle">
              <span>
                <strong>生成音轨</strong>
                <small>随视频生成匹配的声音</small>
              </span>
              <input v-model="audio" type="checkbox" />
              <i aria-hidden="true"></i>
            </label>
          </section>

          <div v-if="formError" class="video-form-error">
            <Icon name="exclamationCircle" size="sm" />
            <span>{{ formError }}</span>
          </div>

          <footer class="video-composer__actions">
            <span>{{ uploading ? '正在上传参考素材…' : submitting ? '正在提交任务…' : '创建后会自动进入任务列表' }}</span>
            <button type="button" :disabled="!canSubmit || submitting || uploading" @click="submitVideo">
              <Icon :name="submitting || uploading ? 'refresh' : 'play'" size="sm" :class="{ 'animate-spin': submitting || uploading }" />
              {{ retryingSameRequest ? '安全重试' : '开始生成' }}
            </button>
          </footer>
        </main>

        <aside class="video-jobs" aria-label="最近任务">
          <div class="video-panel-title">
            <div><span>最近任务</span><strong>{{ jobs.length }} 条</strong></div>
            <button type="button" class="video-icon-button" title="刷新任务" :disabled="loadingJobs" @click="loadJobs">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingJobs }" />
            </button>
          </div>

          <div v-if="previewUrl" class="video-preview">
            <video :src="previewUrl" controls autoplay playsinline></video>
            <button type="button" title="关闭预览" @click="closePreview"><Icon name="x" size="sm" /></button>
          </div>

          <div v-if="loadingJobs && jobs.length === 0" class="video-studio__loading">正在读取任务…</div>
          <div v-else-if="jobs.length === 0" class="video-jobs__empty">
            <Icon name="clock" size="lg" />
            <span>还没有视频任务</span>
          </div>
          <article v-for="job in jobs" v-else :key="job.job_id" class="video-job">
            <div class="video-job__head">
              <span :class="`video-status video-status--${job.status}`">{{ statusLabel(job.status) }}</span>
              <time>{{ formatJobTime(job.created_at) }}</time>
            </div>
            <strong>{{ modelLabel(job.model) }}</strong>
            <p>{{ job.resolution }} · {{ job.duration }} 秒 · {{ job.aspect_ratio }}</p>
            <div class="video-job__meta">
              <span>{{ formatAmount(job) }}</span>
              <small :title="job.job_id">{{ shortJobId(job.job_id) }}</small>
            </div>
            <div class="video-job__actions">
              <button v-if="job.status === 'completed'" type="button" title="播放" :disabled="contentLoadingId === job.job_id" @click="playJob(job)">
                <Icon name="play" size="sm" />
              </button>
              <button v-if="job.status === 'completed'" type="button" title="下载" :disabled="contentLoadingId === job.job_id" @click="downloadJob(job)">
                <Icon name="download" size="sm" />
              </button>
              <button v-if="isActiveStatus(job.status)" type="button" title="取消任务" :disabled="cancelingJobId === job.job_id" @click="cancelJob(job)">
                <Icon name="x" size="sm" />
              </button>
            </div>
          </article>
        </aside>
      </div>
    </div>
  </UserWorkspaceLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref, watch, type PropType } from 'vue'
import { useRoute } from 'vue-router'
import { keysAPI, videoAPI } from '@/api'
import type { ApiKey } from '@/types'
import { VideoAPIError, type UploadedVideoMedia, type VideoJob, type VideoJobStatus, type VideoModel } from '@/api/video'
import UserWorkspaceLayout from '@/components/layout/UserWorkspaceLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  aspectRatiosFor,
  createIdempotencyKey,
  durationsFor,
  resolveVideoCapability,
  type ReferenceKind,
  type VideoCreationMode,
  type VideoModelCapability,
} from '@/utils/videoCapabilities'

interface MediaSelection {
  id: string
  kind: ReferenceKind
  file: File
  previewUrl: string
  uploaded: UploadedVideoMedia | null
  status: 'ready' | 'uploading' | 'uploaded' | 'error'
  error: string
}

const MediaInput = defineComponent({
  props: {
    label: { type: String, required: true }, hint: { type: String, required: true },
    accept: { type: String, required: true }, required: Boolean,
    item: { type: Object as PropType<MediaSelection | null>, default: null },
  },
  emits: ['select', 'remove'],
  setup(props, { emit }) {
    return () => h('div', { class: 'video-media-input' }, [
      h('div', { class: 'video-media-input__copy' }, [
        h('strong', props.label), props.required ? h('em', '必需') : null, h('span', props.hint),
      ]),
      props.item
        ? h('div', { class: 'video-media-input__selected' }, [
            h('span', props.item.file.name),
            h('button', { type: 'button', title: '移除', onClick: () => emit('remove') }, [h(Icon, { name: 'x', size: 'sm' })]),
          ])
        : h('label', { class: 'video-media-input__add' }, [
            h(Icon, { name: 'upload', size: 'sm' }), h('span', '选择文件'),
            h('input', {
              type: 'file', accept: props.accept,
              onChange: (event: Event) => {
                const input = event.target as HTMLInputElement
                if (input.files?.[0]) emit('select', input.files[0])
                input.value = ''
              },
            }),
          ]),
    ])
  },
})

const MediaChip = defineComponent({
  props: { item: { type: Object as PropType<MediaSelection>, required: true } },
  emits: ['remove'],
  setup(props, { emit }) {
    return () => h('div', { class: ['video-media-chip', `video-media-chip--${props.item.status}`] }, [
      props.item.kind === 'image'
        ? h('img', { src: props.item.previewUrl, alt: '' })
        : h('span', { class: 'video-media-chip__type' }, props.item.kind === 'video' ? '视频' : '音频'),
      h('span', { title: props.item.file.name }, props.item.file.name),
      h('button', { type: 'button', title: '移除', onClick: () => emit('remove') }, [h(Icon, { name: 'x', size: 'xs' })]),
    ])
  },
})

const route = useRoute()
const appStore = useAppStore()
const apiKeys = ref<ApiKey[]>([])
const models = ref<VideoModel[]>([])
const jobs = ref<VideoJob[]>([])
const selectedKeyId = ref('')
const selectedModelId = ref('')
const creationMode = ref<VideoCreationMode>('text')
const prompt = ref('')
const resolution = ref('')
const aspectRatio = ref('')
const durationIndex = ref(0)
const audio = ref(false)
const promptEnhance = ref<'AUTO' | 'ON' | 'OFF'>('AUTO')
const startFrame = ref<MediaSelection | null>(null)
const endFrame = ref<MediaSelection | null>(null)
const referenceImages = ref<MediaSelection[]>([])
const referenceVideos = ref<MediaSelection[]>([])
const referenceAudios = ref<MediaSelection[]>([])
const loadingKeys = ref(false)
const loadingModels = ref(false)
const loadingJobs = ref(false)
const submitting = ref(false)
const uploading = ref(false)
const formError = ref('')
const cancelingJobId = ref('')
const contentLoadingId = ref('')
const previewUrl = ref('')
const pendingIdempotencyKey = ref('')
const pendingRequestBody = ref('')
let pollTimer: number | undefined
let workspaceRequest = 0

const videoKeys = computed(() => apiKeys.value.filter((key) => key.status === 'active' && key.group?.platform === 'xiaoapi'))
const selectedKey = computed(() => videoKeys.value.find((key) => String(key.id) === selectedKeyId.value) || null)
const capabilities = computed(() => models.value.map(resolveVideoCapability).filter((item) => item.resolutions.length > 0))
const selectedCapability = computed<VideoModelCapability | null>(() => capabilities.value.find((item) => item.id === selectedModelId.value) || null)
const durationOptions = computed(() => selectedCapability.value ? durationsFor(selectedCapability.value, resolution.value) : [5])
const duration = computed(() => durationOptions.value[durationIndex.value] || durationOptions.value[0] || 5)
const aspectRatioOptions = computed(() => selectedCapability.value ? aspectRatiosFor(selectedCapability.value, resolution.value) : ['16:9'])
const availableModes = computed(() => {
  const capability = selectedCapability.value
  const modes: Array<{ value: VideoCreationMode; label: string; icon: 'sparkles' | 'image' | 'grid' }> = [
    { value: 'text', label: '文生视频', icon: 'sparkles' },
  ]
  if (capability?.supportsStartFrame) modes.push({ value: 'frames', label: capability.supportsEndFrame ? '首尾帧' : '首帧驱动', icon: 'image' })
  if (capability && Object.values(capability.maxReferences).some((count) => count > 0)) {
    modes.push({ value: 'references', label: '参考素材', icon: 'grid' })
  }
  return modes
})
const canSubmit = computed(() => Boolean(selectedKey.value && selectedCapability.value && prompt.value.trim()))
const retryingSameRequest = computed(() => Boolean(pendingIdempotencyKey.value && pendingRequestBody.value))

function selectedIdStorageKey() { return 'lingqu:video-studio:selected-key-id' }
function modelLabel(id: string) { return knownCapabilityLabel(id) || id }
function knownCapabilityLabel(id: string) { return capabilities.value.find((item) => item.id === id)?.label || '' }
function shortJobId(id: string) { return id.length > 16 ? `${id.slice(0, 9)}…${id.slice(-5)}` : id }
function isActiveStatus(status: VideoJobStatus) { return ['pending', 'running', 'settling'].includes(status) }
function statusLabel(status: VideoJobStatus) {
  return ({ pending: '排队中', running: '生成中', settling: '结算中', completed: '已完成', failed: '失败', canceled: '已取消' } as const)[status] || status
}
function formatJobTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date)
}
function formatAmount(job: VideoJob) {
  const amount = Number(job.amount)
  return `${job.currency === 'USD' ? '$' : `${job.currency} `}${Number.isFinite(amount) ? amount.toFixed(2) : job.amount}`
}
function errorMessage(error: unknown) {
  if (error instanceof VideoAPIError) {
    const requestId = error.requestId ? `（请求 ID：${error.requestId}）` : ''
    return `${error.message}${requestId}`
  }
  return error instanceof Error ? error.message : '请求失败，请稍后重试'
}

function createMedia(file: File, kind: ReferenceKind): MediaSelection {
  return {
    id: `${kind}-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    kind, file, previewUrl: URL.createObjectURL(file), uploaded: null, status: 'ready', error: '',
  }
}
function releaseMedia(item: MediaSelection | null) { if (item?.previewUrl) URL.revokeObjectURL(item.previewUrl) }
function releaseAllMedia() {
  releaseMedia(startFrame.value); releaseMedia(endFrame.value)
  ;[...referenceImages.value, ...referenceVideos.value, ...referenceAudios.value].forEach(releaseMedia)
  startFrame.value = null; endFrame.value = null
  referenceImages.value = []; referenceVideos.value = []; referenceAudios.value = []
}
function validateMediaFile(file: File, kind: ReferenceKind): string {
  const allowed = kind === 'image'
    ? ['image/png', 'image/jpeg', 'image/webp']
    : kind === 'video' ? ['video/mp4', 'video/quicktime'] : ['audio/mpeg', 'audio/wav', 'audio/x-wav']
  const max = kind === 'image' ? 10 << 20 : kind === 'video' ? 100 << 20 : 15 << 20
  if (!allowed.includes(file.type)) return `${file.name} 的格式不受支持`
  if (file.size > max) return `${file.name} 超出 ${kind === 'image' ? 10 : kind === 'video' ? 100 : 15} MiB 限制`
  return ''
}
function setSingleMedia(target: 'start' | 'end', file: File) {
  const error = validateMediaFile(file, 'image')
  if (error) { formError.value = error; return }
  const current = target === 'start' ? startFrame.value : endFrame.value
  releaseMedia(current)
  if (target === 'start') startFrame.value = createMedia(file, 'image')
  else endFrame.value = createMedia(file, 'image')
  formError.value = ''
}
function clearSingleMedia(target: 'start' | 'end') {
  const current = target === 'start' ? startFrame.value : endFrame.value
  releaseMedia(current)
  if (target === 'start') startFrame.value = null
  else endFrame.value = null
}
function referenceList(kind: ReferenceKind) {
  return kind === 'image' ? referenceImages : kind === 'video' ? referenceVideos : referenceAudios
}
function referenceLimit(kind: ReferenceKind) { return selectedCapability.value?.maxReferences[kind] || 0 }
function addReferenceFiles(kind: ReferenceKind, event: Event) {
  const input = event.target as HTMLInputElement
  const list = referenceList(kind)
  const remaining = Math.max(0, referenceLimit(kind) - list.value.length)
  const files = Array.from(input.files || []).slice(0, remaining)
  const errors = files.map((file) => validateMediaFile(file, kind)).filter(Boolean)
  if (errors.length) formError.value = errors[0]
  list.value.push(...files.filter((file) => !validateMediaFile(file, kind)).map((file) => createMedia(file, kind)))
  input.value = ''
}
function removeReference(kind: ReferenceKind, id: string) {
  const list = referenceList(kind)
  const target = list.value.find((item) => item.id === id) || null
  releaseMedia(target)
  list.value = list.value.filter((item) => item.id !== id)
}

async function loadKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
    apiKeys.value = response.items
    const ids = new Set(videoKeys.value.map((key) => String(key.id)))
    const queryId = typeof route.query.key_id === 'string' ? route.query.key_id : ''
    const storedId = window.localStorage.getItem(selectedIdStorageKey()) || ''
    selectedKeyId.value = [queryId, selectedKeyId.value, storedId, String(videoKeys.value[0]?.id || '')].find((id) => ids.has(id)) || ''
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loadingKeys.value = false
  }
}
async function loadWorkspace() {
  const key = selectedKey.value
  const requestId = ++workspaceRequest
  clearPolling()
  models.value = []; jobs.value = []; selectedModelId.value = ''
  if (!key) return
  loadingModels.value = true
  try {
    const [nextModels] = await Promise.all([videoAPI.listModels(key.key), loadJobs()])
    if (requestId !== workspaceRequest) return
    models.value = nextModels
    const preferred = nextModels.find((model) => model.id === selectedModelId.value) || nextModels[0]
    selectedModelId.value = preferred?.id || ''
  } catch (error) {
    if (requestId === workspaceRequest) appStore.showError(errorMessage(error))
  } finally {
    if (requestId === workspaceRequest) loadingModels.value = false
  }
  schedulePolling()
}
async function loadJobs() {
  const key = selectedKey.value
  if (!key || loadingJobs.value) return
  loadingJobs.value = true
  try {
    jobs.value = await videoAPI.listJobs(key.key, 30)
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loadingJobs.value = false
  }
}
function schedulePolling() {
  clearPolling()
  pollTimer = window.setInterval(async () => {
    if (jobs.value.some((job) => isActiveStatus(job.status))) await loadJobs()
  }, 5000)
}
function clearPolling() { if (pollTimer) window.clearInterval(pollTimer); pollTimer = undefined }

async function ensureUploaded(item: MediaSelection): Promise<UploadedVideoMedia> {
  if (item.uploaded) return item.uploaded
  const key = selectedKey.value
  if (!key) throw new Error('请先选择视频 Key')
  item.status = 'uploading'; item.error = ''
  try {
    item.uploaded = await videoAPI.upload(key.key, item.file)
    item.status = 'uploaded'
    return item.uploaded
  } catch (error) {
    item.status = 'error'; item.error = errorMessage(error)
    throw error
  }
}
async function buildRequest() {
  const capability = selectedCapability.value
  if (!capability) throw new Error('请选择可用模型')
  if (!prompt.value.trim()) throw new Error('请填写画面描述')
  if (creationMode.value === 'frames' && capability.requiresStartFrame && !startFrame.value) {
    throw new Error(`${capability.label} 必须提供一张首帧`)
  }
  if (creationMode.value === 'references' && referenceAudios.value.length > 0 && referenceImages.value.length + referenceVideos.value.length === 0) {
    throw new Error('参考音频必须同时搭配至少一张参考图片或一个参考视频')
  }
  uploading.value = true
  try {
    const request: import('@/api/video').CreateVideoRequest = {
      model: capability.id,
      prompt: prompt.value.trim(),
      resolution: resolution.value,
      duration: duration.value,
      aspect_ratio: aspectRatio.value,
      audio: capability.supportsAudio ? audio.value : false,
    }
    if (capability.supportsPromptEnhance) request.prompt_enhance = promptEnhance.value
    if (creationMode.value === 'frames') {
      if (startFrame.value) request.start_frame_url = (await ensureUploaded(startFrame.value)).url
      if (endFrame.value && capability.supportsEndFrame) request.end_frame_url = (await ensureUploaded(endFrame.value)).url
    }
    if (creationMode.value === 'references') {
      const images = await Promise.all(referenceImages.value.map(ensureUploaded))
      const videos = await Promise.all(referenceVideos.value.map(ensureUploaded))
      const audios = await Promise.all(referenceAudios.value.map(ensureUploaded))
      request.guidances = {}
      if (images.length) request.guidances.image_reference = images.map((item, index) => ({
        image: { url: item.url, type: 'UPLOADED' }, strength: 'MID', order: index + 1,
      }))
      if (videos.length) request.guidances.video_reference_base = videos.map((item) => ({ video: { url: item.url, type: 'UPLOADED' } }))
      if (audios.length) request.guidances.audio_reference = audios.map((item) => ({ audio: { url: item.url, type: 'UPLOADED' } }))
      if (Object.keys(request.guidances).length === 0) delete request.guidances
    }
    return request
  } finally {
    uploading.value = false
  }
}
async function submitVideo() {
  const key = selectedKey.value
  if (!key) return
  formError.value = ''
  try {
    const request = await buildRequest()
    const body = JSON.stringify(request)
    if (!pendingIdempotencyKey.value || pendingRequestBody.value !== body) {
      pendingIdempotencyKey.value = createIdempotencyKey()
      pendingRequestBody.value = body
    }
    submitting.value = true
    const created = await videoAPI.create(key.key, request, pendingIdempotencyKey.value)
    pendingIdempotencyKey.value = ''; pendingRequestBody.value = ''
    appStore.showSuccess(`视频任务 ${shortJobId(created.job_id)} 已提交`)
    await loadJobs()
  } catch (error) {
    formError.value = errorMessage(error)
  } finally {
    submitting.value = false
  }
}
async function cancelJob(job: VideoJob) {
  const key = selectedKey.value
  if (!key) return
  cancelingJobId.value = job.job_id
  try {
    await videoAPI.cancelJob(key.key, job.job_id)
    appStore.showSuccess('已提交取消请求')
    await loadJobs()
  } catch (error) { appStore.showError(errorMessage(error)) }
  finally { cancelingJobId.value = '' }
}
async function playJob(job: VideoJob) {
  const key = selectedKey.value
  if (!key) return
  contentLoadingId.value = job.job_id
  try {
    const blob = await videoAPI.fetchContent(key.key, job.job_id)
    closePreview(); previewUrl.value = URL.createObjectURL(blob)
  } catch (error) { appStore.showError(errorMessage(error)) }
  finally { contentLoadingId.value = '' }
}
async function downloadJob(job: VideoJob) {
  const key = selectedKey.value
  if (!key) return
  contentLoadingId.value = job.job_id
  try {
    const blob = await videoAPI.fetchContent(key.key, job.job_id, true)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url; link.download = `${job.model}-${job.job_id}.mp4`; link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (error) { appStore.showError(errorMessage(error)) }
  finally { contentLoadingId.value = '' }
}
function closePreview() { if (previewUrl.value) URL.revokeObjectURL(previewUrl.value); previewUrl.value = '' }

watch(selectedKeyId, () => {
  try { if (selectedKeyId.value) window.localStorage.setItem(selectedIdStorageKey(), selectedKeyId.value) }
  catch { /* The studio still works without persisted selection. */ }
  releaseAllMedia(); pendingIdempotencyKey.value = ''; pendingRequestBody.value = ''
  loadWorkspace()
})
watch(selectedCapability, (capability) => {
  if (!capability) return
  resolution.value = capability.defaultResolution
  const durationValue = capability.defaultDuration
  durationIndex.value = Math.max(0, durationsFor(capability, resolution.value).indexOf(durationValue))
  aspectRatio.value = capability.defaultAspectRatio
  audio.value = false; promptEnhance.value = 'AUTO'; formError.value = ''
  if (capability.requiresStartFrame) creationMode.value = 'frames'
  else if (!availableModes.value.some((item) => item.value === creationMode.value)) creationMode.value = 'text'
  releaseAllMedia()
})
watch(resolution, () => {
  const durationValues = durationOptions.value
  if (durationIndex.value >= durationValues.length) durationIndex.value = Math.max(0, durationValues.length - 1)
  if (!aspectRatioOptions.value.includes(aspectRatio.value)) aspectRatio.value = aspectRatioOptions.value[0]
})
watch([prompt, resolution, aspectRatio, durationIndex, audio, promptEnhance, creationMode], () => {
  formError.value = ''
})
watch(startFrame, (value) => {
  if (selectedModelId.value === 'happy-horse-1.1' && value && promptEnhance.value === 'ON') promptEnhance.value = 'AUTO'
})

onMounted(loadKeys)
onBeforeUnmount(() => { clearPolling(); releaseAllMedia(); closePreview() })
</script>

<style scoped>
.video-studio { display: grid; gap: 1rem; color: #211f1c; }
.video-studio__header { display: flex; align-items: end; justify-content: space-between; gap: 1.5rem; border: 2px solid #211f1c; border-radius: 8px; background: #fffdf5; box-shadow: 5px 5px 0 #211f1c; padding: 1.1rem 1.25rem; }
.video-studio__eyebrow { display: inline-flex; align-items: center; gap: .4rem; color: #08799a; font-size: .72rem; font-weight: 900; }
.video-studio__header h1 { margin: .3rem 0 .18rem; font-family: Georgia, 'Songti SC', serif; font-size: clamp(1.35rem, 2vw, 2rem); line-height: 1.1; }
.video-studio__header p { margin: 0; color: #6e675f; font-size: .84rem; }
.video-studio__keybar { display: flex; align-items: end; gap: .45rem; }
.video-studio__keybar label { display: grid; gap: .25rem; }
.video-studio__keybar label > span, .video-setting > span, .video-prompt > span strong { font-size: .7rem; font-weight: 900; }
.video-studio select { min-height: 2.5rem; border: 1px solid #b8aea3; border-radius: 6px; background: #fff; padding: 0 .75rem; color: inherit; font-weight: 700; }
.video-studio__keybar select { width: min(22rem, 34vw); }
.video-icon-button, .video-doc-link { display: inline-flex; min-width: 2.5rem; min-height: 2.5rem; align-items: center; justify-content: center; gap: .4rem; border: 1px solid #211f1c; border-radius: 6px; background: #fff; color: inherit; font-size: .76rem; font-weight: 900; text-decoration: none; }
.video-doc-link { padding: 0 .75rem; background: #ffd447; }
.video-studio__workspace { display: grid; grid-template-columns: 13.5rem minmax(0, 1fr) 18rem; align-items: start; gap: 1rem; }
.video-studio__models, .video-composer, .video-jobs { min-width: 0; border: 1px solid rgba(33,31,28,.22); border-radius: 8px; background: rgba(255,253,245,.95); box-shadow: 0 8px 24px rgba(72,58,45,.08); }
.video-studio__models, .video-jobs { display: grid; gap: .5rem; padding: .8rem; }
.video-panel-title { display: flex; align-items: center; justify-content: space-between; gap: .5rem; padding-bottom: .55rem; border-bottom: 1px solid rgba(33,31,28,.12); }
.video-panel-title > div { display: grid; }
.video-panel-title span { color: #756d65; font-size: .67rem; font-weight: 800; }
.video-panel-title strong { font-size: .86rem; }
.video-panel-title .video-icon-button { min-width: 2rem; min-height: 2rem; border-color: #c9c0b7; }
.video-model-option { display: grid; grid-template-columns: 2rem minmax(0,1fr) auto; align-items: center; gap: .55rem; width: 100%; min-height: 3.4rem; border: 1px solid transparent; border-radius: 6px; background: transparent; padding: .5rem; color: inherit; text-align: left; }
.video-model-option:hover { border-color: #c3b7aa; background: #fff; }
.video-model-option--active { border-color: #211f1c; background: #fff; box-shadow: 2px 2px 0 #211f1c; }
.video-model-option__icon { display: grid; width: 2rem; height: 2rem; place-items: center; border-radius: 5px; background: #dff7ff; color: #08799a; }
.video-model-option strong, .video-model-option small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.video-model-option strong { font-size: .75rem; }.video-model-option small { margin-top: .15rem; color: #857c73; font-size: .61rem; }
.video-studio__loading, .video-jobs__empty { display: grid; min-height: 8rem; place-items: center; color: #817970; font-size: .74rem; text-align: center; }
.video-composer { padding: 1rem; }
.video-composer__topline { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.video-composer__topline > div { display: grid; }.video-composer__topline span { color: #7d746b; font-size: .67rem; font-weight: 800; }.video-composer__topline strong { font-size: .95rem; }
.video-retry-badge { border: 1px solid #15926f; border-radius: 999px; background: #e7faf3; padding: .25rem .55rem; color: #0b6b50 !important; }
.video-mode-tabs { display: grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap: .35rem; margin: .8rem 0; border-radius: 7px; background: #eee9e2; padding: .25rem; }
.video-mode-tabs button { display: flex; min-height: 2.35rem; align-items: center; justify-content: center; gap: .35rem; border: 0; border-radius: 5px; background: transparent; color: #736b63; font-size: .72rem; font-weight: 900; }
.video-mode-tabs .video-mode-tab--active { background: #fff; box-shadow: 0 2px 8px rgba(33,31,28,.11); color: #211f1c; }
.video-media-band { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: .55rem; margin-bottom: .8rem; }
.video-media-input { display: flex; min-height: 4.2rem; align-items: center; justify-content: space-between; gap: .7rem; border: 1px dashed #a69b90; border-radius: 7px; background: #fbfaf7; padding: .65rem; }
.video-media-input__copy { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: .3rem; }.video-media-input__copy strong { font-size: .73rem; }.video-media-input__copy em { border-radius: 3px; background: #ffe4eb; padding: .1rem .25rem; color: #a92852; font-size: .58rem; font-style: normal; }.video-media-input__copy span { width: 100%; color: #7e756c; font-size: .62rem; }
.video-media-input__add, .video-add-media { display: inline-flex; flex: 0 0 auto; cursor: pointer; align-items: center; gap: .28rem; border: 1px solid #b8aea3; border-radius: 5px; background: #fff; padding: .48rem .58rem; font-size: .66rem; font-weight: 900; }.video-media-input input, .video-add-media input { display: none; }
.video-media-input__selected { display: flex; min-width: 0; align-items: center; gap: .35rem; }.video-media-input__selected span { max-width: 8rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: .65rem; }.video-media-input__selected button { display: grid; width: 1.8rem; height: 1.8rem; place-items: center; border: 0; background: transparent; }
.video-reference-section { display: grid; gap: .48rem; margin-bottom: .8rem; border-block: 1px solid #ddd5cc; padding: .7rem 0; }
.video-reference-row { display: flex; align-items: center; justify-content: space-between; gap: .7rem; }.video-reference-row > div { display: grid; }.video-reference-row strong { font-size: .72rem; }.video-reference-row span { color: #80776e; font-size: .61rem; }
.video-media-list { display: flex; flex-wrap: wrap; gap: .35rem; }
.video-media-chip { display: grid; max-width: 10rem; grid-template-columns: 1.7rem minmax(0,1fr) 1.3rem; align-items: center; gap: .3rem; border: 1px solid #d1c7bc; border-radius: 5px; background: #fff; padding: .22rem; }.video-media-chip img, .video-media-chip__type { width: 1.7rem; height: 1.7rem; border-radius: 3px; object-fit: cover; }.video-media-chip__type { display: grid; place-items: center; background: #e7f7fb; color: #08799a; font-size: .48rem; font-weight: 900; }.video-media-chip > span:nth-child(2) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: .58rem; }.video-media-chip button { display: grid; place-items: center; border: 0; background: transparent; }.video-media-chip--uploading { opacity: .55; }.video-media-chip--error { border-color: #d93963; }
.video-prompt { display: grid; gap: .35rem; }.video-prompt > span { display: flex; justify-content: space-between; }.video-prompt small { color: #817870; font-size: .63rem; }.video-prompt textarea { width: 100%; resize: vertical; border: 1px solid #b8aea3; border-radius: 7px; background: #fff; padding: .75rem; color: inherit; font: inherit; font-size: .82rem; line-height: 1.65; }
.video-settings { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: .75rem; margin-top: .85rem; border-top: 1px solid #ddd5cc; padding-top: .8rem; }.video-setting { display: grid; align-content: start; gap: .35rem; }.video-setting--wide { grid-column: 1 / -1; }
.video-segments { display: flex; flex-wrap: wrap; gap: .3rem; }.video-segments button { min-height: 2rem; border: 1px solid #c9bfb5; border-radius: 5px; background: #fff; padding: 0 .65rem; color: inherit; font-size: .66rem; font-weight: 900; }.video-segments .video-segment--active { border-color: #08799a; background: #e1f8ff; color: #076582; box-shadow: inset 0 0 0 1px #08799a; }
.video-duration { display: grid; grid-template-columns: minmax(0,1fr) 3.5rem; align-items: center; gap: .65rem; min-height: 2.5rem; }.video-duration input { accent-color: #08799a; }.video-duration strong { border: 1px solid #ccc2b8; border-radius: 5px; background: #fff; padding: .45rem; font-size: .7rem; text-align: center; }
.video-audio-toggle { position: relative; display: flex; align-items: center; justify-content: space-between; gap: .6rem; cursor: pointer; }.video-audio-toggle > span { display: grid; }.video-audio-toggle strong { font-size: .72rem; }.video-audio-toggle small { color: #7e756c; font-size: .61rem; }.video-audio-toggle input { position: absolute; opacity: 0; }.video-audio-toggle i { position: relative; width: 2.6rem; height: 1.4rem; border: 1px solid #a99f94; border-radius: 999px; background: #d9d3cc; }.video-audio-toggle i::after { position: absolute; top: .16rem; left: .17rem; width: .95rem; height: .95rem; border-radius: 50%; background: #fff; box-shadow: 0 1px 3px rgba(0,0,0,.2); content: ''; transition: transform .18s; }.video-audio-toggle input:checked + i { border-color: #08799a; background: #08a9d6; }.video-audio-toggle input:checked + i::after { transform: translateX(1.16rem); }
.video-form-error { display: flex; align-items: start; gap: .4rem; margin-top: .75rem; border: 1px solid #edb3c3; border-radius: 6px; background: #fff0f4; padding: .55rem .65rem; color: #a7244b; font-size: .68rem; line-height: 1.45; }
.video-composer__actions { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-top: .85rem; border-top: 1px solid #ddd5cc; padding-top: .8rem; }.video-composer__actions > span { color: #7b7269; font-size: .65rem; }.video-composer__actions button { display: inline-flex; min-height: 2.6rem; align-items: center; gap: .45rem; border: 1px solid #211f1c; border-radius: 6px; background: #ff5f8f; box-shadow: 3px 3px 0 #211f1c; padding: 0 1rem; color: #211f1c; font-size: .75rem; font-weight: 900; }.video-composer__actions button:disabled { cursor: not-allowed; opacity: .45; }
.video-jobs { max-height: calc(100vh - 10rem); overflow-y: auto; }.video-preview { position: relative; overflow: hidden; border-radius: 6px; background: #111; }.video-preview video { display: block; width: 100%; max-height: 20rem; }.video-preview button { position: absolute; top: .35rem; right: .35rem; display: grid; width: 1.8rem; height: 1.8rem; place-items: center; border: 0; border-radius: 50%; background: rgba(0,0,0,.65); color: #fff; }
.video-jobs__empty { gap: .5rem; }.video-job { position: relative; border: 1px solid #d3c9be; border-radius: 6px; background: #fff; padding: .65rem; }.video-job__head, .video-job__meta { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }.video-job__head time { color: #8c8379; font-size: .56rem; }.video-job > strong { display: block; margin-top: .48rem; font-size: .72rem; }.video-job > p { margin: .18rem 0 .52rem; color: #756c63; font-size: .62rem; }.video-job__meta { border-top: 1px solid #eee8e2; padding-top: .45rem; }.video-job__meta span { color: #08799a; font-size: .68rem; font-weight: 900; }.video-job__meta small { max-width: 7rem; overflow: hidden; color: #948a80; font-family: monospace; font-size: .54rem; }.video-status { border-radius: 999px; padding: .18rem .38rem; font-size: .56rem; font-weight: 900; }.video-status--pending { background: #fff4cf; color: #846200; }.video-status--running,.video-status--settling { background: #dff6ff; color: #086884; }.video-status--completed { background: #dcf7ed; color: #087255; }.video-status--failed { background: #ffe4ec; color: #a72249; }.video-status--canceled { background: #ebe8e5; color: #6d655e; }
.video-job__actions { position: absolute; right: .45rem; bottom: 2.25rem; display: flex; gap: .2rem; }.video-job__actions button { display: grid; width: 1.75rem; height: 1.75rem; place-items: center; border: 1px solid #c9c0b7; border-radius: 4px; background: #fff; color: #4f4841; }
.video-studio__empty { display: grid; min-height: 24rem; place-items: center; align-content: center; gap: .5rem; border: 1px dashed #aa9e92; border-radius: 8px; background: rgba(255,253,245,.85); text-align: center; }.video-studio__empty > span { display: grid; width: 4rem; height: 4rem; place-items: center; border-radius: 8px; background: #e1f8ff; color: #08799a; }.video-studio__empty p { margin: 0; color: #746b62; font-size: .75rem; }.video-studio__empty a { margin-top: .4rem; border: 1px solid #211f1c; border-radius: 5px; background: #ffd447; padding: .5rem .8rem; color: inherit; font-size: .7rem; font-weight: 900; text-decoration: none; }
button:not(:disabled), select:not(:disabled), input[type='range'] { cursor: pointer; }

@media (max-width: 1180px) { .video-studio__workspace { grid-template-columns: 12rem minmax(0,1fr); }.video-jobs { grid-column: 1 / -1; max-height: none; grid-template-columns: repeat(3,minmax(0,1fr)); }.video-jobs .video-panel-title,.video-jobs .video-preview,.video-jobs .video-jobs__empty,.video-jobs .video-studio__loading { grid-column: 1 / -1; } }
@media (max-width: 760px) { .video-studio__header { align-items: stretch; flex-direction: column; }.video-studio__keybar { flex-wrap: wrap; }.video-studio__keybar label { width: 100%; }.video-studio__keybar select { width: 100%; }.video-studio__workspace { grid-template-columns: 1fr; }.video-studio__models { grid-template-columns: repeat(2,minmax(0,1fr)); }.video-studio__models .video-panel-title,.video-studio__models .video-studio__loading { grid-column: 1 / -1; }.video-media-band,.video-settings { grid-template-columns: 1fr; }.video-setting--wide { grid-column: auto; }.video-mode-tabs { grid-template-columns: 1fr; }.video-jobs { grid-column: auto; grid-template-columns: 1fr; }.video-composer__actions { align-items: stretch; flex-direction: column; }.video-composer__actions button { justify-content: center; }.video-studio__header h1 { font-size: 1.35rem; } }
</style>
