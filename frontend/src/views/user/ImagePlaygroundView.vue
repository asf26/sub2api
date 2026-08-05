<template>
  <UserWorkspaceLayout>
    <div class="lingqu-image-bridge">
      <section class="image-bridge-hero" :class="{ 'image-bridge-hero--launched': frameSrc }">
        <div class="image-bridge-hero__copy">
          <span class="image-bridge-kicker">
            <Icon name="image" size="sm" />
            AI 图工坊
          </span>
          <h1>选择 Key，直接开始创作</h1>
          <p>提交后立即返回任务，结果会在图工坊里自动出现。</p>
        </div>

        <div class="image-bridge-panel">
          <div class="image-bridge-panel__top">
            <span>{{ imageKeys.length }} 个可用 Key</span>
            <button type="button" class="image-bridge-icon-button" :disabled="loadingKeys" title="刷新 Key" @click="loadApiKeys">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingKeys }" />
            </button>
          </div>

          <label class="image-bridge-field">
            <span>当前 Key</span>
            <select v-model="selectedKeyId" :disabled="loadingKeys || imageKeys.length === 0">
              <option value="">选择一个 OpenAI 分组 Key</option>
              <option v-for="key in imageKeys" :key="key.id" :value="String(key.id)">
                {{ key.name }} · {{ key.group?.name || 'OpenAI' }}
              </option>
            </select>
          </label>

          <div class="image-bridge-actions">
            <button
              type="button"
              class="image-bridge-secondary"
              :disabled="!selectedKey || launching"
              @click="launchWorkspace(true)"
            >
              <Icon name="externalLink" size="sm" />
              <span>新窗口</span>
            </button>
          </div>

          <router-link v-if="imageKeys.length === 0 && !loadingKeys" to="/keys?create=1" class="image-bridge-create">
            <Icon name="plus" size="sm" />
            先创建一个 Key
          </router-link>
        </div>
      </section>

      <section class="image-workbench-shell" :class="{ 'image-workbench-shell--empty': !frameSrc }">
        <div v-if="!frameSrc" class="image-workbench-empty">
          <div class="image-workbench-empty__badge">
            <Icon name="key" size="xl" />
          </div>
        <strong>{{ loadingKeys ? '正在读取你的 Key' : emptyTitle }}</strong>
        <span>{{ emptyHint }}</span>
        </div>

        <iframe
          v-else
          :key="frameKey"
          :src="frameSrc"
          title="AI 图工坊"
          class="image-workbench-frame"
          allow="clipboard-read; clipboard-write"
        ></iframe>
      </section>
    </div>
  </UserWorkspaceLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { keysAPI } from '@/api'
import UserWorkspaceLayout from '@/components/layout/UserWorkspaceLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { ApiKey } from '@/types'

const LINGQU_BRIDGE_STORAGE_KEY = 'lingqu:image-playground:bridge'
const LINGQU_SELECTED_KEY_STORAGE_KEY = 'lingqu:image-playground:selected-key-id'
const IMAGE_PLAYGROUND_ENTRY_PATH = import.meta.env.DEV ? '/image-playground/index.html' : '/image-playground/'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

const apiKeys = ref<ApiKey[]>([])
const loadingKeys = ref(false)
const launching = ref(false)
const selectedKeyId = ref('')
const frameSrc = ref('')
const frameKey = ref(0)

const imageKeys = computed(() => {
  return apiKeys.value.filter((key) => key.status === 'active' && key.group?.platform === 'openai')
})

const selectedKey = computed(() => {
  return imageKeys.value.find((key) => String(key.id) === selectedKeyId.value) || null
})

const emptyTitle = computed(() => {
  if (imageKeys.value.length === 0) return '还没有可用于创作的 Key'
  return '选择 Key 后进入图工坊'
})

const emptyHint = computed(() => {
  if (imageKeys.value.length === 0) return '创建一个 OpenAI 分组 Key 后，就能直接打开图工坊。'
  return '图工坊会自动接入当前 Key，生成记录会走当前平台后端。'
})

function buildBridgePayload(key: ApiKey) {
  return {
    apiUrl: `${window.location.origin}/v1`,
    keyId: key.id,
    apiKey: key.key,
    keyName: key.name,
    model: 'gpt-image-2',
    apiMode: 'images',
    userEmail: authStore.user?.email || '',
    userTheme: window.localStorage.getItem('user-workspace-theme') || 'cartoon',
    launchedAt: Date.now(),
  }
}

function readStoredSelectedKeyId() {
  try {
    return window.localStorage.getItem(LINGQU_SELECTED_KEY_STORAGE_KEY) || ''
  } catch {
    return ''
  }
}

function writeStoredSelectedKeyId(keyId: string) {
  try {
    if (keyId) {
      window.localStorage.setItem(LINGQU_SELECTED_KEY_STORAGE_KEY, keyId)
    } else {
      window.localStorage.removeItem(LINGQU_SELECTED_KEY_STORAGE_KEY)
    }
  } catch {
    // Storage can be unavailable in private browsing; the page still works without persistence.
  }
}

async function resetImagePlaygroundRuntime() {
  try {
    if ('serviceWorker' in navigator) {
      const registrations = await navigator.serviceWorker.getRegistrations()
      await Promise.all(
        registrations
          .filter((registration) => registration.scope.includes('/image-playground/'))
          .map((registration) => registration.unregister())
      )
    }

    if ('caches' in window) {
      const keys = await window.caches.keys()
      await Promise.all(
        keys
          .filter((key) => key.startsWith('lingqu-image-playground-'))
          .map((key) => window.caches.delete(key))
      )
    }
  } catch (error) {
    console.warn('Failed to reset image playground runtime:', error)
  }
}

async function launchWorkspace(openInNewWindow = false, silent = false) {
  const key = selectedKey.value
  if (!key) {
    if (!silent) appStore.showWarning('先选择一个可用 Key')
    return
  }

  launching.value = true
  try {
    const payload = JSON.stringify(buildBridgePayload(key))
    window.sessionStorage.setItem(LINGQU_BRIDGE_STORAGE_KEY, payload)
    window.localStorage.setItem(LINGQU_BRIDGE_STORAGE_KEY, payload)
    await resetImagePlaygroundRuntime()
    const url = `${IMAGE_PLAYGROUND_ENTRY_PATH}?lingqu=${Date.now()}`
    if (openInNewWindow) {
      window.open(url, '_blank', 'noopener,noreferrer')
    } else {
      frameSrc.value = url
      frameKey.value += 1
    }
    if (!silent) appStore.showSuccess('已接入 AI 图工坊')
  } catch (error) {
    console.error('Failed to launch image playground:', error)
    appStore.showError('图工坊启动失败，请刷新后重试')
  } finally {
    window.setTimeout(() => {
      launching.value = false
    }, 360)
  }
}

async function loadApiKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 100, {
      status: 'active',
      sort_by: 'created_at',
      sort_order: 'desc',
    })
    apiKeys.value = response.items
  } catch (error) {
    console.error('Failed to load image keys:', error)
    appStore.showError('Key 加载失败，请稍后重试')
  } finally {
    loadingKeys.value = false
  }
}

watch(imageKeys, (keys) => {
  const keyIds = new Set(keys.map((key) => String(key.id)))
  const queryKeyId = typeof route.query.key_id === 'string' ? route.query.key_id : ''
  const storedKeyId = readStoredSelectedKeyId()

  const nextKeyId = [
    queryKeyId,
    storedKeyId,
    selectedKeyId.value,
    keys[0] ? String(keys[0].id) : '',
  ].find((keyId) => keyId && keyIds.has(keyId)) || ''

  if (selectedKeyId.value !== nextKeyId) {
    selectedKeyId.value = nextKeyId
  }
}, { immediate: true })

watch(selectedKeyId, () => {
  if (selectedKey.value) {
    writeStoredSelectedKeyId(selectedKeyId.value)
    launchWorkspace(false, true)
  } else if (!selectedKeyId.value) {
    writeStoredSelectedKeyId('')
  }
})

onMounted(() => {
  loadApiKeys()
})
</script>

<style scoped>
.lingqu-image-bridge {
  display: grid;
  gap: 1rem;
}

.image-bridge-hero,
.image-workbench-shell {
  border: 1px solid rgba(33, 31, 28, 0.16);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 18px 38px rgba(33, 31, 28, 0.08);
}

.image-bridge-hero {
  position: relative;
  display: grid;
  min-height: 5.4rem;
  grid-template-columns: minmax(0, 1fr) minmax(32rem, auto);
  align-items: center;
  gap: 0.85rem;
  overflow: hidden;
  padding: 0.65rem 0.78rem;
}

.image-bridge-hero--launched {
  min-height: 4.25rem;
  grid-template-columns: minmax(0, 1fr) auto;
  padding: 0.5rem 0.6rem;
}

.image-bridge-hero::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 12% 18%, rgba(255, 79, 121, 0.16), transparent 28%),
    radial-gradient(circle at 88% 24%, rgba(39, 202, 255, 0.18), transparent 26%),
    linear-gradient(135deg, rgba(255, 247, 210, 0.68), rgba(229, 249, 255, 0.68));
  pointer-events: none;
}

.image-bridge-hero::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(circle at 10px 10px, rgba(33, 31, 28, 0.05) 1px, transparent 1.5px);
  background-size: 22px 22px;
  pointer-events: none;
}

.image-bridge-hero__copy,
.image-bridge-panel {
  position: relative;
  z-index: 1;
}

.image-bridge-kicker {
  display: inline-flex;
  min-height: 1.65rem;
  align-items: center;
  gap: 0.34rem;
  border: 1px solid rgba(33, 31, 28, 0.18);
  border-radius: 999px;
  background: #fff;
  padding: 0 0.58rem;
  color: #211f1c;
  font-size: 0.7rem;
  font-weight: 950;
}

.image-bridge-hero h1 {
  margin-top: 0.25rem;
  max-width: 42rem;
  color: #211f1c;
  font-family: theme('fontFamily.display');
  font-size: clamp(1.22rem, 1.9vw, 1.62rem);
  font-weight: 950;
  line-height: 1.04;
  letter-spacing: 0;
}

.image-bridge-hero--launched h1 {
  font-size: clamp(1.02rem, 1.55vw, 1.32rem);
}

.image-bridge-hero--launched p {
  display: none;
}

.image-bridge-hero p {
  margin-top: 0.12rem;
  max-width: 46rem;
  color: rgba(33, 31, 28, 0.64);
  font-size: 0.76rem;
  font-weight: 760;
  line-height: 1.35;
}

.image-bridge-panel {
  display: grid;
  grid-template-columns: auto minmax(14rem, 19rem) auto;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid rgba(33, 31, 28, 0.16);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.72);
  box-shadow: 0 12px 28px rgba(33, 31, 28, 0.08);
  padding: 0.46rem;
}

.image-bridge-hero--launched .image-bridge-panel {
  grid-template-columns: auto minmax(12rem, 18rem) auto;
  padding: 0.34rem;
}

.image-bridge-hero--launched .image-bridge-panel__top span,
.image-bridge-hero--launched .image-bridge-field span {
  display: none;
}

.image-bridge-hero--launched .image-bridge-panel button,
.image-bridge-hero--launched .image-bridge-create {
  min-height: 1.98rem;
  border-radius: 12px;
  padding: 0 0.58rem;
  font-size: 0.76rem;
}

.image-bridge-hero--launched .image-bridge-field select {
  min-height: 2rem;
  font-size: 0.78rem;
}

.image-bridge-panel__top,
.image-bridge-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.42rem;
}

.image-bridge-panel__top span {
  color: rgba(33, 31, 28, 0.62);
  font-size: 0.78rem;
  font-weight: 900;
  white-space: nowrap;
}

.image-bridge-panel button,
.image-bridge-create {
  display: inline-flex;
  min-height: 2.15rem;
  align-items: center;
  justify-content: center;
  gap: 0.42rem;
  border: 1px solid rgba(33, 31, 28, 0.18);
  border-radius: 13px;
  padding: 0 0.68rem;
  font-size: 0.8rem;
  font-weight: 950;
  transition: transform 150ms ease, box-shadow 150ms ease, background 150ms ease;
}

.image-bridge-panel button:hover:not(:disabled),
.image-bridge-create:hover {
  transform: translateY(-1px);
}

.image-bridge-panel button:disabled {
  cursor: not-allowed;
  opacity: 0.54;
}

.image-bridge-field {
  display: grid;
  gap: 0.22rem;
}

.image-bridge-field span {
  color: rgba(33, 31, 28, 0.72);
  font-size: 0.68rem;
  font-weight: 950;
}

.image-bridge-field select {
  min-height: 2.22rem;
  width: 100%;
  border: 1px solid rgba(33, 31, 28, 0.18);
  border-radius: 13px;
  background: #fff;
  color: #211f1c;
  padding: 0 0.65rem;
  font-size: 0.82rem;
  font-weight: 850;
  outline: none;
}

.image-bridge-field select:focus {
  border-color: #ff4f79;
  box-shadow: 0 0 0 4px rgba(255, 79, 121, 0.14);
}

.image-bridge-primary {
  flex: 1;
  border-color: #211f1c !important;
  background: linear-gradient(135deg, #ff4f79, #ff9f31);
  color: #fff;
  box-shadow: 0 10px 20px rgba(255, 79, 121, 0.22);
}

.image-bridge-icon-button {
  width: 2.15rem;
  flex: 0 0 auto;
  padding: 0 !important;
}

.image-bridge-secondary,
.image-bridge-panel__top button,
.image-bridge-create {
  background: #fff;
  color: #211f1c;
}

.image-bridge-create {
  text-decoration: none;
}

.image-workbench-shell {
  position: relative;
  min-height: min(70vh, 760px);
  overflow: hidden;
}

.image-workbench-shell--empty {
  display: grid;
  min-height: 26rem;
  place-items: center;
}

.image-workbench-empty {
  display: grid;
  justify-items: center;
  gap: 0.55rem;
  padding: 2rem;
  text-align: center;
}

.image-workbench-empty__badge {
  display: grid;
  height: 4.25rem;
  width: 4.25rem;
  place-items: center;
  border: 2px solid #211f1c;
  border-radius: 22px;
  background: #fff5c9;
  box-shadow: 6px 6px 0 #211f1c;
  color: #211f1c;
}

.image-workbench-empty strong {
  color: #211f1c;
  font-size: 1.05rem;
  font-weight: 950;
}

.image-workbench-empty span {
  max-width: 28rem;
  color: rgba(33, 31, 28, 0.6);
  font-size: 0.9rem;
  font-weight: 760;
}

.image-workbench-frame {
  display: block;
  height: min(78vh, 820px);
  min-height: 38rem;
  width: 100%;
  border: 0;
  background: #f8fafc;
}

@media (max-width: 920px) {
  .image-bridge-hero {
    grid-template-columns: 1fr;
  }

  .image-bridge-hero--launched {
    min-height: auto;
  }

  .image-bridge-panel {
    grid-template-columns: auto minmax(0, 1fr) auto;
  }

  .image-workbench-frame {
    height: 72vh;
    min-height: 34rem;
  }
}

@media (max-width: 560px) {
  .image-bridge-hero {
    gap: 0.55rem;
    padding: 0.55rem;
  }

  .image-bridge-hero--launched .image-bridge-kicker,
  .image-bridge-hero--launched .image-bridge-hero__copy {
    display: none;
  }

  .image-bridge-panel {
    grid-template-columns: 1fr;
  }

  .image-bridge-hero--launched .image-bridge-panel {
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 0.4rem;
    width: 100%;
  }

  .image-bridge-hero p {
    display: none;
  }

  .image-bridge-panel__top {
    justify-content: space-between;
  }

  .image-bridge-actions {
    align-items: center;
    flex-direction: row;
  }

  .image-bridge-secondary {
    flex: 0 0 auto;
  }

  .image-bridge-hero--launched .image-bridge-primary {
    max-width: 8.2rem;
  }

  .image-bridge-hero--launched .image-bridge-secondary {
    width: 2.25rem;
    padding: 0 !important;
  }

  .image-bridge-hero--launched .image-bridge-secondary .icon + * {
    display: none;
  }
}

.image-workbench-empty__badge {
  border: 1px solid rgba(33, 31, 28, 0.12);
  background: #fff8df;
  box-shadow: 0 10px 24px rgba(29, 42, 42, 0.08);
}

.image-workbench-frame {
  border-radius: 16px;
  background: #fbfcfc;
}
</style>
