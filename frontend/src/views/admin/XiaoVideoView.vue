<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl">
      <div class="grid min-h-[calc(100vh-10rem)] gap-6 lg:grid-cols-[18rem_minmax(0,1fr)] lg:gap-8">
        <aside class="min-w-0 border-b border-gray-200 pb-6 dark:border-dark-700 lg:border-b-0 lg:border-r lg:pb-0 lg:pr-6">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.xiaoVideo.upstreams') }}
              </h2>
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.xiaoVideo.upstreamCount', { count: accounts.length }) }}
              </span>
            </div>
            <button
              type="button"
              class="btn btn-primary flex h-9 w-9 items-center justify-center p-0"
              :title="t('admin.xiaoVideo.addUpstream')"
              data-testid="xiao-add-upstream"
              @click="beginCreate()"
            >
              <Icon name="plus" size="sm" />
            </button>
          </div>

          <div v-if="loading" class="flex justify-center py-10" data-testid="xiao-list-loading">
            <span class="h-6 w-6 animate-spin rounded-full border-2 border-gray-200 border-t-primary-500" />
          </div>

          <div v-else class="space-y-2">
            <button
              v-if="isCreating"
              type="button"
              class="xiao-upstream-item xiao-upstream-item-active"
              data-testid="xiao-new-upstream-item"
            >
              <span class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
                <Icon name="plus" size="sm" />
              </span>
              <span class="min-w-0 flex-1 text-left">
                <span class="block truncate text-sm font-medium">{{ t('admin.xiaoVideo.newUpstream') }}</span>
                <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">XiaoAPI</span>
              </span>
            </button>

            <button
              v-for="account in accounts"
              :key="account.id"
              type="button"
              class="xiao-upstream-item"
              :class="{ 'xiao-upstream-item-active': selectedId === account.id }"
              :data-testid="`xiao-upstream-${account.id}`"
              @click="selectAccount(account.id)"
            >
              <span
                class="h-2.5 w-2.5 flex-shrink-0 rounded-full"
                :class="account.status === 'active' ? 'bg-emerald-500' : account.status === 'error' ? 'bg-red-500' : 'bg-gray-300 dark:bg-dark-500'"
              />
              <span class="min-w-0 flex-1 text-left">
                <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ account.name }}</span>
                <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ accountStatusLabel(account) }} · {{ t('admin.xiaoVideo.concurrencyValue', { count: account.concurrency }) }}
                </span>
              </span>
              <Icon name="chevronRight" size="xs" class="flex-shrink-0 text-gray-400" />
            </button>

            <div v-if="accounts.length === 0 && !isCreating" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.xiaoVideo.empty') }}
            </div>
          </div>
        </aside>

        <main class="min-w-0">
          <div v-if="detailsLoading" class="flex items-center justify-center py-24" data-testid="xiao-details-loading">
            <span class="h-8 w-8 animate-spin rounded-full border-2 border-gray-200 border-t-primary-500" />
          </div>

          <form v-else class="space-y-8" novalidate @submit.prevent="save">
            <section class="border-b border-gray-200 pb-8 dark:border-dark-700">
              <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
                <div class="flex items-center gap-3">
                  <span class="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">
                    <Icon name="server" size="md" />
                  </span>
                  <div>
                    <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                      {{ t('admin.xiaoVideo.upstreamSettings') }}
                    </h2>
                    <span v-if="selectedAccount" class="mt-0.5 block font-mono text-xs text-gray-400">#{{ selectedAccount.id }}</span>
                  </div>
                </div>
                <span
                  v-if="selectedAccount"
                  class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium"
                  :class="selectedAccount.status === 'active'
                    ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300'
                    : selectedAccount.status === 'error'
                      ? 'bg-red-50 text-red-700 dark:bg-red-900/25 dark:text-red-300'
                      : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
                >
                  {{ accountStatusLabel(selectedAccount) }}
                </span>
              </div>

              <div class="grid gap-5 md:grid-cols-2">
                <label class="min-w-0">
                  <span class="input-label">{{ t('admin.xiaoVideo.name') }}</span>
                  <input
                    v-model="form.name"
                    type="text"
                    class="input"
                    autocomplete="off"
                    data-testid="xiao-name"
                  />
                </label>

                <label class="min-w-0">
                  <span class="input-label">{{ t('admin.xiaoVideo.baseUrl') }}</span>
                  <input
                    v-model="form.baseUrl"
                    type="url"
                    class="input font-mono"
                    autocomplete="url"
                    placeholder="https://video-upstream.example.com"
                    data-testid="xiao-base-url"
                  />
                </label>

                <label class="min-w-0">
                  <span class="input-label">{{ t('admin.xiaoVideo.apiKey') }}</span>
                  <input
                    v-model="form.apiKey"
                    type="password"
                    class="input font-mono"
                    autocomplete="new-password"
                    :placeholder="hasExistingApiKey ? t('admin.xiaoVideo.apiKeyUnchanged') : ''"
                    data-testid="xiao-api-key"
                  />
                </label>

                <div class="grid grid-cols-2 gap-3">
                  <label class="min-w-0">
                    <span class="input-label">{{ t('admin.xiaoVideo.status') }}</span>
                    <select v-model="form.status" class="input" data-testid="xiao-status">
                      <option value="active">{{ t('common.active') }}</option>
                      <option value="inactive">{{ t('common.inactive') }}</option>
                      <option v-if="form.status === 'error'" value="error">{{ t('admin.xiaoVideo.errorStatus') }}</option>
                    </select>
                  </label>
                  <label class="min-w-0">
                    <span class="input-label">{{ t('admin.xiaoVideo.concurrency') }}</span>
                    <input
                      v-model.number="form.concurrency"
                      type="number"
                      min="1"
                      max="10000"
                      step="1"
                      class="input"
                      data-testid="xiao-concurrency"
                    />
                  </label>
                </div>

                <label class="min-w-0 md:col-span-2">
                  <span class="input-label">{{ t('admin.xiaoVideo.notes') }}</span>
                  <textarea
                    v-model="form.notes"
                    rows="2"
                    class="input resize-y"
                    data-testid="xiao-notes"
                  />
                </label>
              </div>

              <div class="mt-6 border-t border-gray-100 pt-5 dark:border-dark-700">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <span class="input-label mb-0">{{ t('admin.xiaoVideo.groups') }}</span>
                  <router-link to="/admin/groups" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">
                    {{ t('admin.xiaoVideo.manageGroups') }}
                  </router-link>
                </div>
                <div v-if="groups.length > 0" class="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                  <label
                    v-for="group in groups"
                    :key="group.id"
                    class="flex min-w-0 cursor-pointer items-center gap-3 rounded-lg border border-gray-200 px-3 py-2.5 hover:border-primary-300 dark:border-dark-600 dark:hover:border-primary-700"
                  >
                    <input
                      type="checkbox"
                      class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                      :checked="form.groupIds.includes(group.id)"
                      :data-testid="`xiao-group-${group.id}`"
                      @change="toggleGroup(group.id, $event)"
                    />
                    <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-200">{{ group.name }}</span>
                    <span v-if="group.status !== 'active'" class="text-xs text-gray-400">{{ t('common.inactive') }}</span>
                  </label>
                </div>
                <div v-else class="rounded-lg border border-dashed border-gray-300 px-4 py-5 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
                  {{ t('admin.xiaoVideo.noGroups') }}
                </div>
              </div>
            </section>

            <section class="min-w-0 pb-2">
              <div class="mb-5 flex items-center gap-3">
                <span class="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                  <Icon name="dollar" size="md" />
                </span>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.xiaoVideo.modelsAndPricing') }}
                </h2>
              </div>
              <XiaoVideoConfigEditor v-model:pricing="pricing" v-model:mappings="mappings" />
            </section>

            <div class="-mx-4 flex flex-wrap items-center justify-between gap-3 border-t border-gray-200 px-4 py-4 dark:border-dark-700 sm:mx-0 sm:px-0">
              <div class="flex items-center gap-2">
                <button
                  v-if="selectedAccount"
                  type="button"
                  class="btn btn-secondary inline-flex items-center gap-2"
                  :disabled="testing || saving || deleting"
                  data-testid="xiao-test"
                  @click="testConnection"
                >
                  <Icon name="beaker" size="sm" />
                  {{ testing ? t('admin.xiaoVideo.testing') : t('admin.xiaoVideo.testConnection') }}
                </button>
                <button
                  v-if="selectedAccount"
                  type="button"
                  class="btn btn-secondary inline-flex items-center gap-2 text-red-600 dark:text-red-400"
                  :disabled="deleting || saving || testing"
                  data-testid="xiao-delete"
                  @click="removeAccount"
                >
                  <Icon name="trash" size="sm" />
                  {{ t('common.delete') }}
                </button>
              </div>
              <button
                type="submit"
                class="btn btn-primary inline-flex min-w-28 items-center justify-center gap-2"
                :disabled="saving || deleting || detailsLoading"
                data-testid="xiao-save"
              >
                <Icon name="check" size="sm" />
                {{ saving ? t('common.saving') : t('common.save') }}
              </button>
            </div>
          </form>
        </main>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import XiaoVideoConfigEditor from '@/components/account/XiaoVideoConfigEditor.vue'
import {
  createXiaoVideoPricingRule,
  normalizeXiaoVideoPricing,
  readXiaoVideoPricing,
  validateXiaoVideoPricing,
  type XiaoVideoModelMapping,
  type XiaoVideoPricingRule
} from '@/components/account/xiaoVideoPricing'
import { isValidWildcardPattern } from '@/composables/useModelWhitelist'
import { useAppStore } from '@/stores/app'
import type { Account, AdminGroup } from '@/types'

type EditableStatus = 'active' | 'inactive' | 'error'

interface XiaoVideoForm {
  name: string
  notes: string
  baseUrl: string
  apiKey: string
  concurrency: number
  status: EditableStatus
  groupIds: number[]
}

const { t } = useI18n()
const appStore = useAppStore()

const accounts = ref<Account[]>([])
const groups = ref<AdminGroup[]>([])
const selectedId = ref<number | null>(null)
const selectedAccount = ref<Account | null>(null)
const mappings = ref<XiaoVideoModelMapping[]>([])
const pricing = ref<XiaoVideoPricingRule[]>([createXiaoVideoPricingRule()])
const hasExistingApiKey = ref(false)
const loading = ref(true)
const detailsLoading = ref(false)
const saving = ref(false)
const testing = ref(false)
const deleting = ref(false)
const baseline = ref('')
let selectionRequest = 0

const form = reactive<XiaoVideoForm>({
  name: '',
  notes: '',
  baseUrl: '',
  apiKey: '',
  concurrency: 1,
  status: 'active',
  groupIds: []
})

const isCreating = computed(() => selectedId.value === null)
const isDirty = computed(() => baseline.value !== formSnapshot())

function formSnapshot(): string {
  return JSON.stringify({
    form: {
      ...form,
      groupIds: [...form.groupIds].sort((a, b) => a - b)
    },
    mappings: mappings.value,
    pricing: pricing.value
  })
}

function setBaseline() {
  baseline.value = formSnapshot()
}

function errorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return fallback
}

function editableStatus(account: Account): EditableStatus {
  if (account.status === 'active' || account.status === 'inactive' || account.status === 'error') {
    return account.status
  }
  return 'inactive'
}

function accountStatusLabel(account: Account): string {
  if (account.status === 'active') return t('common.active')
  if (account.status === 'error') return t('admin.xiaoVideo.errorStatus')
  return t('common.inactive')
}

function readMappings(value: unknown): XiaoVideoModelMapping[] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return []
  return Object.entries(value as Record<string, unknown>).flatMap(([from, rawTo]) => {
    if (typeof rawTo !== 'string') return []
    return [{ from, to: rawTo }]
  })
}

function resetForm() {
  Object.assign(form, {
    name: '',
    notes: '',
    baseUrl: '',
    apiKey: '',
    concurrency: 1,
    status: 'active' as EditableStatus,
    groupIds: [] as number[]
  })
  mappings.value = []
  pricing.value = [createXiaoVideoPricingRule()]
  hasExistingApiKey.value = false
  setBaseline()
}

function syncForm(account: Account) {
  const credentials = (account.credentials ?? {}) as Record<string, unknown>
  Object.assign(form, {
    name: account.name,
    notes: account.notes ?? '',
    baseUrl: typeof credentials.base_url === 'string' ? credentials.base_url : '',
    apiKey: '',
    concurrency: account.concurrency || 1,
    status: editableStatus(account),
    groupIds: [...(account.group_ids ?? [])]
  })
  mappings.value = readMappings(credentials.model_mapping)
  pricing.value = readXiaoVideoPricing(credentials.video_pricing)
  hasExistingApiKey.value = account.credentials_status?.has_api_key ?? typeof credentials.api_key === 'string'
  setBaseline()
}

function confirmDiscard(): boolean {
  return !isDirty.value || window.confirm(t('admin.xiaoVideo.discardChanges'))
}

function beginCreate(skipConfirm = false) {
  if (!skipConfirm && !confirmDiscard()) return
  selectionRequest += 1
  selectedId.value = null
  selectedAccount.value = null
  detailsLoading.value = false
  resetForm()
}

async function selectAccount(id: number, skipConfirm = false, force = false) {
  if (!force && selectedId.value === id && selectedAccount.value) return
  if (!skipConfirm && !confirmDiscard()) return

  const previousId = selectedId.value
  const requestId = ++selectionRequest
  selectedId.value = id
  detailsLoading.value = true
  try {
    const account = await adminAPI.accounts.getById(id)
    if (requestId !== selectionRequest) return
    if (account.platform !== 'xiaoapi') {
      throw new Error(t('admin.xiaoVideo.invalidPlatform'))
    }
    selectedAccount.value = account
    syncForm(account)
  } catch (error) {
    if (requestId === selectionRequest) {
      selectedId.value = previousId
      appStore.showError(errorMessage(error, t('admin.xiaoVideo.loadFailed')))
    }
  } finally {
    if (requestId === selectionRequest) detailsLoading.value = false
  }
}

async function loadAllAccounts(): Promise<Account[]> {
  const first = await adminAPI.accounts.list(1, 100, { platform: 'xiaoapi' })
  if (first.pages <= 1) return first.items
  const remaining = await Promise.all(
    Array.from({ length: first.pages - 1 }, (_, index) =>
      adminAPI.accounts.list(index + 2, 100, { platform: 'xiaoapi' })
    )
  )
  return [first, ...remaining].flatMap((page) => page.items)
}

async function reloadAccounts(preferredId?: number) {
  const allAccounts = await loadAllAccounts()
  accounts.value = allAccounts
  const nextId = preferredId ?? selectedId.value
  if (nextId && allAccounts.some((account) => account.id === nextId)) {
    await selectAccount(nextId, true, true)
  } else if (allAccounts.length > 0) {
    await selectAccount(allAccounts[0].id, true, true)
  } else {
    beginCreate(true)
  }
}

function toggleGroup(groupId: number, event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  if (checked && !form.groupIds.includes(groupId)) {
    form.groupIds = [...form.groupIds, groupId]
  } else if (!checked) {
    form.groupIds = form.groupIds.filter((id) => id !== groupId)
  }
}

function validBaseUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return (url.protocol === 'http:' || url.protocol === 'https:') && !url.search && !url.hash
  } catch {
    return false
  }
}

function validateMappings(): string | null {
  const seen = new Set<string>()
  for (const mapping of mappings.value) {
    const from = mapping.from.trim()
    const to = mapping.to.trim()
    if (!from || !to) return 'required'
    if (!isValidWildcardPattern(from)) return 'wildcard'
    if (to.includes('*')) return 'targetWildcard'
    if (seen.has(from)) return 'duplicate'
    seen.add(from)
  }
  return null
}

function buildMappings(): Record<string, string> | null {
  const value: Record<string, string> = {}
  for (const mapping of mappings.value) {
    value[mapping.from.trim()] = mapping.to.trim()
  }
  return Object.keys(value).length > 0 ? value : null
}

function validateForm(): boolean {
  if (!form.name.trim()) {
    appStore.showError(t('admin.xiaoVideo.validation.nameRequired'))
    return false
  }
  if (!validBaseUrl(form.baseUrl.trim())) {
    appStore.showError(t('admin.xiaoVideo.validation.baseUrlInvalid'))
    return false
  }
  if (!Number.isInteger(form.concurrency) || form.concurrency < 1 || form.concurrency > 10000) {
    appStore.showError(t('admin.xiaoVideo.validation.concurrencyInvalid'))
    return false
  }
  if (!form.apiKey.trim() && !hasExistingApiKey.value) {
    appStore.showError(t('admin.xiaoVideo.validation.apiKeyRequired'))
    return false
  }
  const mappingError = validateMappings()
  if (mappingError) {
    appStore.showError(t(`admin.xiaoVideo.validation.mapping.${mappingError}`))
    return false
  }
  const pricingError = validateXiaoVideoPricing(pricing.value)
  if (pricingError) {
    appStore.showError(t(`admin.accounts.xiaoapi.validation.${pricingError}`))
    return false
  }
  return true
}

function buildCredentials(): Record<string, unknown> {
  const currentCredentials = (selectedAccount.value?.credentials ?? {}) as Record<string, unknown>
  const credentials: Record<string, unknown> = {
    ...currentCredentials,
    base_url: form.baseUrl.trim(),
    video_pricing: normalizeXiaoVideoPricing(pricing.value)
  }
  if (form.apiKey.trim()) credentials.api_key = form.apiKey.trim()
  const mapping = buildMappings()
  if (mapping) credentials.model_mapping = mapping
  else delete credentials.model_mapping
  return credentials
}

async function save() {
  if (!validateForm()) return
  saving.value = true
  try {
    const credentials = buildCredentials()
    let saved: Account
    if (selectedAccount.value) {
      saved = await adminAPI.accounts.update(selectedAccount.value.id, {
        name: form.name.trim(),
        notes: form.notes.trim() || null,
        type: 'apikey',
        credentials,
        concurrency: form.concurrency,
        status: form.status,
        group_ids: [...form.groupIds],
        upstream_billing_probe_enabled: false,
        upstream_billing_rate_sync_enabled: false
      })
      appStore.showSuccess(t('admin.xiaoVideo.updated'))
    } else {
      saved = await adminAPI.accounts.create({
        name: form.name.trim(),
        notes: form.notes.trim() || null,
        platform: 'xiaoapi',
        type: 'apikey',
        credentials,
        concurrency: form.concurrency,
        group_ids: [...form.groupIds],
        upstream_billing_probe_enabled: false
      })
      if (form.status !== 'active') {
        saved = await adminAPI.accounts.update(saved.id, { status: form.status })
      }
      appStore.showSuccess(t('admin.xiaoVideo.created'))
    }
    form.apiKey = ''
    setBaseline()
    await reloadAccounts(saved.id)
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.xiaoVideo.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function testConnection() {
  if (!selectedAccount.value) return
  testing.value = true
  try {
    const result = await adminAPI.accounts.testAccount(selectedAccount.value.id)
    if (result.success) appStore.showSuccess(result.message || t('admin.xiaoVideo.testSucceeded'))
    else appStore.showError(result.message || t('admin.xiaoVideo.testFailed'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.xiaoVideo.testFailed')))
  } finally {
    testing.value = false
  }
}

async function removeAccount() {
  if (!selectedAccount.value) return
  if (!window.confirm(t('admin.xiaoVideo.deleteConfirm', { name: selectedAccount.value.name }))) return
  deleting.value = true
  try {
    await adminAPI.accounts.delete(selectedAccount.value.id)
    appStore.showSuccess(t('admin.xiaoVideo.deleted'))
    selectedId.value = null
    selectedAccount.value = null
    resetForm()
    await reloadAccounts()
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.xiaoVideo.deleteFailed')))
  } finally {
    deleting.value = false
  }
}

setBaseline()

onMounted(async () => {
  loading.value = true
  try {
    const [, xiaoGroups] = await Promise.all([
      reloadAccounts(),
      adminAPI.groups.getByPlatform('xiaoapi')
    ])
    groups.value = xiaoGroups
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.xiaoVideo.loadFailed')))
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.xiao-upstream-item {
  display: flex;
  width: 100%;
  min-height: 3.5rem;
  align-items: center;
  gap: 0.75rem;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  padding: 0.625rem 0.75rem;
  color: rgb(55 65 81);
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

.xiao-upstream-item:hover {
  border-color: rgb(226 232 240);
  background: rgb(255 255 255 / 0.72);
}

.xiao-upstream-item-active {
  border-color: rgb(253 186 116 / 0.7);
  background: rgb(255 255 255);
  box-shadow: 0 6px 18px rgb(15 23 42 / 0.05);
}

:global(.dark) .xiao-upstream-item {
  color: rgb(209 213 219);
}

:global(.dark) .xiao-upstream-item:hover {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55 / 0.6);
}

:global(.dark) .xiao-upstream-item-active {
  border-color: rgb(180 83 9 / 0.6);
  background: rgb(31 41 55);
}
</style>
