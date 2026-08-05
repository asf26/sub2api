<template>
  <div
    class="user-workspace"
    :class="{ 'user-workspace--subpage': parentNavigation }"
    :data-user-theme="theme"
    @mousemove="handlePointerMove"
    @mouseleave="resetPointer"
  >
    <div class="user-workspace__bg" aria-hidden="true">
      <span class="user-workspace__spark user-workspace__spark--one"></span>
      <span class="user-workspace__spark user-workspace__spark--two"></span>
      <span class="user-workspace__comet"></span>
    </div>

    <aside class="user-workspace__business-rail" aria-label="专业工作台导航">
      <router-link to="/dashboard" class="user-workspace__rail-brand">
        <span class="user-workspace__rail-logo">
          <img :src="siteLogo || '/logo.svg'" alt="" />
        </span>
        <span>
          <strong>{{ siteName }}</strong>
          <small>用户控制台</small>
        </span>
      </router-link>

      <div class="user-workspace__rail-section">
        <small>工作台</small>
        <template
          v-for="item in businessNavItems"
          :key="`rail-${item.path}`"
        >
          <router-link
            :to="item.path"
            class="user-workspace__rail-link"
            :class="{ 'user-workspace__rail-link--active': isNavActive(item) }"
          >
            <Icon :name="item.icon" size="sm" />
            <span>{{ item.label }}</span>
            <Icon
              :name="item.path === '/purchase' && isNavActive(item) ? 'chevronDown' : 'chevronRight'"
              size="xs"
            />
          </router-link>
          <nav
            v-if="item.path === '/purchase' && isNavActive(item)"
            class="user-workspace__rail-subnav"
            aria-label="账单导航"
          >
            <router-link
              v-for="child in billingNavItems"
              :key="`rail-billing-${child.path}`"
              :to="child.path"
              :class="{ 'user-workspace__rail-sublink--active': isBillingNavActive(child) }"
              class="user-workspace__rail-sublink"
            >
              <span>{{ child.label }}</span>
            </router-link>
          </nav>
        </template>
      </div>

      <div class="user-workspace__rail-footer">
        <router-link to="/profile" class="user-workspace__rail-account">
          <span class="user-workspace__rail-avatar">{{ userInitials }}</span>
          <span class="user-workspace__rail-account-copy">
            <strong>{{ displayName }}</strong>
            <small>{{ user?.email }}</small>
          </span>
          <Icon name="chevronRight" size="xs" />
        </router-link>
        <div class="user-workspace__rail-balance">
          <span>账户余额</span>
          <strong>${{ balanceText }}</strong>
          <router-link to="/orders">查看账单</router-link>
        </div>
        <div class="user-workspace__rail-theme">
          <UserThemeSwitcher />
        </div>
        <router-link v-if="isAdmin" to="/admin/dashboard" class="user-workspace__rail-admin">
          <Icon name="shield" size="sm" />
          系统管理
        </router-link>
      </div>
    </aside>

    <header class="user-workspace__header">
      <nav class="user-workspace__nav" aria-label="用户控制台导航">
        <router-link to="/dashboard" class="user-workspace__brand" aria-label="AI 工作台">
          <span class="user-workspace__logo">
            <img :src="siteLogo || '/logo.svg'" alt="" />
          </span>
          <span class="user-workspace__brand-text">
            <strong>{{ siteName }}</strong>
            <small>One Key</small>
          </span>
        </router-link>

        <div class="user-workspace__context">
          <span>{{ currentSection.kicker }}</span>
          <strong>{{ currentSection.title }}</strong>
        </div>

        <button
          type="button"
          class="user-workspace__menu-button"
          aria-label="打开导航"
          @click="mobileMenuOpen = !mobileMenuOpen"
        >
          <Icon :name="mobileMenuOpen ? 'x' : 'menu'" size="md" />
        </button>

        <div
          class="user-workspace__links"
          :class="{ 'user-workspace__links--open': mobileMenuOpen }"
        >
          <template
            v-for="item in theme === 'business' ? businessNavItems : navItems"
            :key="item.path"
          >
            <router-link
              :to="item.path"
              class="user-workspace__link"
              :class="{ 'user-workspace__link--active': isNavActive(item) }"
              @click="mobileMenuOpen = false"
            >
              <Icon :name="item.icon" size="sm" />
              <span>{{ item.label }}</span>
            </router-link>
            <div
              v-if="theme === 'business' && item.path === '/purchase' && isNavActive(item)"
              class="user-workspace__mobile-subnav"
            >
              <router-link
                v-for="child in billingNavItems"
                :key="`mobile-billing-${child.path}`"
                :to="child.path"
                :class="{ 'user-workspace__mobile-sublink--active': isBillingNavActive(child) }"
                @click="mobileMenuOpen = false"
              >
                {{ child.label }}
              </router-link>
            </div>
          </template>
        </div>

        <div class="user-workspace__account">
          <div class="user-workspace__header-theme">
            <UserThemeSwitcher />
          </div>

          <router-link to="/keys?create=1" class="user-workspace__business-create">
            <Icon name="plus" size="sm" />
            <span>创建 Key</span>
          </router-link>

          <router-link to="/purchase" class="user-workspace__balance" title="账单中心">
            <Icon name="dollar" size="sm" />
            <span>${{ balanceText }}</span>
          </router-link>

          <div class="user-workspace__profile-menu" ref="dropdownRef">
            <button
              type="button"
              class="user-workspace__avatar"
              aria-label="账户菜单"
              @click="dropdownOpen = !dropdownOpen"
            >
              <span>{{ userInitials }}</span>
              <Icon name="chevronDown" size="xs" />
            </button>

            <transition name="user-menu">
              <div v-if="dropdownOpen" class="user-workspace__dropdown">
                <div class="user-workspace__dropdown-head">
                  <strong>{{ displayName }}</strong>
                  <small>{{ user?.email }}</small>
                </div>
                <router-link to="/profile" @click="closeDropdown">
                  <Icon name="user" size="sm" />
                  个人资料
                </router-link>
                <router-link to="/purchase" @click="closeDropdown">
                  <Icon name="creditCard" size="sm" />
                  账单中心
                </router-link>
                <router-link to="/usage" @click="closeDropdown">
                  <Icon name="chart" size="sm" />
                  使用记录
                </router-link>
                <router-link v-if="isAdmin" to="/admin/dashboard" @click="closeDropdown">
                  <Icon name="shield" size="sm" />
                  系统管理
                </router-link>
                <router-link to="/keys?create=1" @click="closeDropdown">
                  <Icon name="plus" size="sm" />
                  创建 Key
                </router-link>
                <button type="button" @click="handleLogout">
                  <Icon name="login" size="sm" />
                  退出登录
                </button>
              </div>
            </transition>
          </div>
        </div>
      </nav>

      <section class="user-workspace__summary" aria-label="系统公告">
        <div class="user-workspace__announcement-badge">
          <Icon name="bell" size="sm" />
          <span>公告</span>
        </div>

        <div
          class="user-workspace__announcement-ticker"
          :class="{
            'user-workspace__announcement-ticker--empty': tickerAnnouncements.length === 0,
            'user-workspace__announcement-ticker--single': tickerAnnouncements.length === 1
          }"
        >
          <div
            v-if="tickerAnnouncements.length > 0"
            class="user-workspace__announcement-track"
            :class="{ 'user-workspace__announcement-track--single': tickerAnnouncements.length === 1 }"
          >
            <div
              v-for="loop in announcementTickerLoopCount"
              :key="loop"
              class="user-workspace__announcement-sequence"
            >
              <button
                v-for="item in tickerAnnouncements"
                :key="`${loop}-${item.id}`"
                type="button"
                class="user-workspace__announcement-item"
                :class="{ 'user-workspace__announcement-item--unread': !item.read_at }"
                :tabindex="loop === 1 ? 0 : -1"
                :aria-hidden="loop > 1 ? 'true' : undefined"
                :aria-label="`查看公告：${item.title}`"
                @click="announcementStore.openPopup(item)"
              >
                <span class="user-workspace__announcement-dot" aria-hidden="true"></span>
                <span class="user-workspace__announcement-title" :title="item.title">
                  {{ formatAnnouncementTickerText(item) }}
                </span>
              </button>
            </div>
          </div>

          <div v-else class="user-workspace__announcement-empty">
            <Icon name="sparkles" size="sm" />
            <span>{{ announcementLoading ? '公告加载中' : '暂无公告' }}</span>
          </div>
        </div>
      </section>
    </header>

    <main class="user-workspace__main">
      <div v-if="parentNavigation" class="user-workspace__backbar">
        <router-link :to="parentNavigation.to" class="user-workspace__backlink">
          <Icon name="arrowLeft" size="sm" />
          <span>返回{{ parentNavigation.label }}</span>
        </router-link>
        <span class="user-workspace__breadcrumb">
          {{ parentNavigation.group }}
          <Icon name="chevronRight" size="xs" />
          {{ currentPageTitle }}
        </span>
      </div>
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAnnouncementStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import UserThemeSwitcher from '@/components/layout/UserThemeSwitcher.vue'
import { DEFAULT_SITE_LOGO, resolveBrandLogo, resolveBrandName } from '@/constants/brand'
import type { UserAnnouncement } from '@/types'
import { useUserThemeStore } from '@/stores/userTheme'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const announcementStore = useAnnouncementStore()
const userThemeStore = useUserThemeStore()
const { announcements, loading: announcementLoading } = storeToRefs(announcementStore)
const { theme } = storeToRefs(userThemeStore)

const dropdownOpen = ref(false)
const mobileMenuOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const user = computed(() => authStore.user)
const isAdmin = computed(() => authStore.isAdmin)
const tickerAnnouncements = computed(() => announcements.value.slice(0, 8))
const announcementTickerLoopCount = computed(() => tickerAnnouncements.value.length > 1 ? 2 : 1)

const siteName = computed(() => resolveBrandName(appStore.cachedPublicSettings?.site_name || appStore.siteName))
const siteLogo = computed(() => resolveBrandLogo(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo) || DEFAULT_SITE_LOGO)
const balanceText = computed(() => Number(user.value?.balance || 0).toFixed(2))

const parentNavigation = computed(() => {
  const path = route.path
  if (['/purchase', '/subscriptions', '/orders', '/redeem'].includes(path)) {
    if (theme.value === 'business') return null
    return { to: '/purchase', label: '账单中心', group: '账单' }
  }
  if (path.startsWith('/payment/')) {
    return { to: '/purchase', label: '充值/订阅', group: '支付' }
  }
  if (path === '/available-channels') {
    return { to: '/monitor', label: '渠道状态', group: '状态' }
  }
  if (path === '/usage') {
    if (theme.value === 'business') return null
    return { to: '/keys', label: 'Key 管理', group: 'Key' }
  }
  return null
})

const currentPageTitle = computed(() => {
  const title = route.meta.title
  return typeof title === 'string' ? title : ''
})

const currentSection = computed(() => {
  const sections = [
    {
      match: (path: string) => path === '/dashboard',
      kicker: 'Workspace',
      title: '今日工作台',
      description: '管理接入、查看状态，并继续你正在进行的工作。'
    },
    {
      match: (path: string) => path === '/keys' || path === '/usage',
      kicker: 'Credentials',
      title: route.path === '/usage' ? '使用记录' : '访问密钥',
      description: '集中管理 API Key、额度限制与调用记录。'
    },
    {
      match: (path: string) => path === '/images' || path === '/batch-image',
      kicker: 'Image workspace',
      title: route.path === '/batch-image' ? '批量生图' : '图像工作台',
      description: '创建、管理并追踪你的图像生成任务。'
    },
    {
      match: (path: string) => path === '/videos' || path === '/docs/video-api',
      kicker: 'Video workspace',
      title: route.path === '/docs/video-api' ? '视频 API 文档' : '视频创作',
      description: '创建、管理并接入平台的视频生成任务。'
    },
    {
      match: (path: string) => path === '/monitor' || path === '/available-channels',
      kicker: 'Service health',
      title: '服务状态',
      description: '查看模型渠道、可用性与当前服务状态。'
    },
    {
      match: (path: string) => path === '/affiliate',
      kicker: 'Affiliate',
      title: '邀请返利',
      description: '分享邀请链接，查看返利额度与邀请记录。'
    },
    {
      match: (path: string) => path === '/account-share',
      kicker: 'Account sharing', title: '账号共享',
      description: '管理 OAuth 账号资产与共享席位。'
    },
    {
      match: (path: string) => path === '/store',
      kicker: 'Digital store', title: '发卡商城', description: '购买并查看自动交付的数字商品。'
    },
    {
      match: (path: string) => path === '/conversations',
      kicker: 'Support', title: '工单服务', description: '在固定会话中持续跟进问题。'
    },
    {
      match: (path: string) => ['/purchase', '/subscriptions', '/orders', '/redeem'].includes(path) || path.startsWith('/payment/'),
      kicker: 'Billing',
      title: '账单与订阅',
      description: '管理余额、订阅套餐、充值和订单。'
    },
    {
      match: (path: string) => path === '/profile',
      kicker: 'Account',
      title: '账户与安全',
      description: '维护个人资料、通知方式和登录安全。'
    }
  ]

  return sections.find(section => section.match(route.path)) ?? {
    kicker: 'Workspace',
    title: currentPageTitle.value || '用户工作台',
    description: '查看并管理你的用户端资源。'
  }
})

const affiliateEnabled = computed(() => appStore.cachedPublicSettings?.affiliate_enabled === true)
const affiliateNavItem = {
  path: '/affiliate',
  activePaths: ['/affiliate'],
  label: '邀请返利',
  icon: 'users'
} as const

const baseNavItems = [
  { path: '/dashboard', activePaths: ['/dashboard'], label: '首页', icon: 'home' },
  { path: '/keys', activePaths: ['/keys', '/usage'], label: 'Key', icon: 'key' },
  { path: '/images', activePaths: ['/images'], label: '图工坊', icon: 'image' },
  { path: '/videos', activePaths: ['/videos'], label: '视频创作', icon: 'play' },
  { path: '/docs/video-api', activePaths: ['/docs/video-api'], label: '视频 API', icon: 'book' },
  { path: '/monitor', activePaths: ['/monitor', '/available-channels'], label: '状态', icon: 'server' },
  { path: '/purchase', activePaths: ['/purchase', '/payment', '/subscriptions', '/orders', '/redeem'], label: '账单', icon: 'creditCard' }
] as const

const baseBusinessNavItems = [
  { path: '/dashboard', activePaths: ['/dashboard'], label: '首页', icon: 'home' },
  { path: '/keys', activePaths: ['/keys'], label: 'Key', icon: 'key' },
  { path: '/usage', activePaths: ['/usage'], label: '使用记录', icon: 'chart' },
  { path: '/images', activePaths: ['/images'], label: '图工坊', icon: 'image' },
  { path: '/videos', activePaths: ['/videos'], label: '视频创作', icon: 'play' },
  { path: '/docs/video-api', activePaths: ['/docs/video-api'], label: '视频 API', icon: 'book' },
  { path: '/monitor', activePaths: ['/monitor', '/available-channels'], label: '状态', icon: 'server' },
  { path: '/purchase', activePaths: ['/purchase', '/payment', '/subscriptions', '/orders', '/redeem'], label: '账单', icon: 'creditCard' }
] as const

const navItems = computed(() => {
  if (!affiliateEnabled.value) return [...baseNavItems]
  return [...baseNavItems.slice(0, 2), affiliateNavItem, ...baseNavItems.slice(2)]
})

const businessNavItems = computed(() => {
  if (!affiliateEnabled.value) return [...baseBusinessNavItems]
  return [...baseBusinessNavItems.slice(0, 3), affiliateNavItem, ...baseBusinessNavItems.slice(3)]
})

const billingNavItems = [
  { path: '/purchase', label: '充值与订阅' },
  { path: '/subscriptions', label: '我的订阅' },
  { path: '/orders', label: '订单记录' },
  { path: '/redeem', label: '兑换码' }
] as const

const displayName = computed(() => {
  if (!user.value) return ''
  return user.value.username || user.value.email?.split('@')[0] || ''
})

const userInitials = computed(() => {
  const source = displayName.value || user.value?.email || 'AI'
  return source.slice(0, 2).toUpperCase()
})

function isNavActive(item: { activePaths: readonly string[] }): boolean {
  return item.activePaths.some(path => route.path === path || route.path.startsWith(`${path}/`))
}

function isBillingNavActive(item: (typeof billingNavItems)[number]): boolean {
  if (item.path === '/purchase') {
    return route.path === item.path || route.path.startsWith('/payment/')
  }
  return route.path === item.path
}

function closeDropdown() {
  dropdownOpen.value = false
}

function stripAnnouncementMarkup(value: string) {
  return value
    .replace(/<[^>]*>/g, ' ')
    .replace(/!\[[^\]]*]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)]\([^)]*\)/g, '$1')
    .replace(/[`*_~>#-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function formatAnnouncementTickerText(item: UserAnnouncement) {
  const content = stripAnnouncementMarkup(item.content || '')
  return content || item.title
}

async function handleLogout() {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
}

function handlePointerMove(event: MouseEvent) {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const x = Math.min(100, Math.max(0, ((event.clientX - rect.left) / rect.width) * 100))
  const y = Math.min(100, Math.max(0, ((event.clientY - rect.top) / rect.height) * 100))
  target.style.setProperty('--workspace-x', `${x}%`)
  target.style.setProperty('--workspace-y', `${y}%`)
}

function resetPointer(event: MouseEvent) {
  const target = event.currentTarget as HTMLElement
  target.style.setProperty('--workspace-x', '70%')
  target.style.setProperty('--workspace-y', '16%')
}

onMounted(() => {
  document.body.classList.add('user-workspace-active')
  document.body.dataset.userTheme = theme.value
  document.addEventListener('click', handleClickOutside)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  authStore.refreshUser().catch((error) => {
    console.warn('Failed to refresh user workspace profile:', error)
  })
})

onBeforeUnmount(() => {
  document.body.classList.remove('user-workspace-active')
  delete document.body.dataset.userTheme
  document.removeEventListener('click', handleClickOutside)
})

watch(theme, (nextTheme) => {
  if (document.body.classList.contains('user-workspace-active')) {
    document.body.dataset.userTheme = nextTheme
  }
})
</script>

<style scoped>
.user-workspace {
  --ink: #211f1c;
  --paper: #fffdf5;
  --yellow: #ffd447;
  --pink: #ff5f8f;
  --cyan: #08a9d6;
  --mint: #2ecf9f;
  min-height: 100vh;
  position: relative;
  overflow-x: hidden;
  color: var(--ink);
  background:
    radial-gradient(circle at var(--workspace-x, 70%) var(--workspace-y, 16%), rgba(255, 212, 71, 0.22), transparent 28rem),
    linear-gradient(115deg, #fff4ee 0%, #fffdf5 46%, #e8fbff 100%);
}

.user-workspace__bg {
  position: fixed;
  inset: 0;
  pointer-events: none;
  background-image:
    radial-gradient(circle at 1px 1px, rgba(33, 31, 28, 0.08) 1.2px, transparent 1.8px),
    linear-gradient(rgba(33, 31, 28, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(33, 31, 28, 0.035) 1px, transparent 1px);
  background-size: 24px 24px, 48px 48px, 48px 48px;
  opacity: 0.78;
}

.user-workspace__spark {
  position: absolute;
  width: 2rem;
  height: 2rem;
  background: var(--pink);
  clip-path: polygon(50% 0, 61% 37%, 100% 50%, 61% 63%, 50% 100%, 39% 63%, 0 50%, 39% 37%);
  filter: drop-shadow(2px 2px 0 rgba(33, 31, 28, 0.55));
  animation: userWorkspaceSpark 4.8s ease-in-out infinite;
}

.user-workspace__spark--one {
  right: 8vw;
  top: 10rem;
}

.user-workspace__spark--two {
  left: 7vw;
  top: 17rem;
  width: 1.2rem;
  height: 1.2rem;
  background: var(--yellow);
  animation-delay: 1.2s;
}

.user-workspace__comet {
  position: absolute;
  right: -8rem;
  top: 22rem;
  width: 24rem;
  height: 5px;
  border-radius: 999px;
  background: rgba(33, 31, 28, 0.12);
  transform: rotate(-8deg);
  animation: userWorkspaceComet 8s ease-in-out infinite;
}

.user-workspace__header,
.user-workspace__main {
  position: relative;
}

.user-workspace__header {
  z-index: 20;
  padding: clamp(0.85rem, 2vw, 1.45rem) 1rem 0;
}

.user-workspace__nav {
  position: relative;
  z-index: 40;
  width: min(86rem, calc(100vw - 2rem));
  min-height: 4.55rem;
  margin: 0 auto;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 1rem;
  border: 3px solid var(--ink);
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 7px 7px 0 rgba(33, 31, 28, 0.92);
  padding: 0.58rem 0.72rem;
  backdrop-filter: blur(16px);
  animation: userWorkspaceDrop 520ms cubic-bezier(0.16, 0.86, 0.28, 1.16) both;
}

.user-workspace__brand {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.68rem;
  color: inherit;
}

.user-workspace__logo {
  width: 3.05rem;
  height: 3.05rem;
  display: grid;
  place-items: center;
  overflow: hidden;
  border: 3px solid var(--ink);
  border-radius: 18px;
  background: #fff7d7;
  box-shadow: 4px 4px 0 rgba(33, 31, 28, 0.92);
  transition: transform 170ms ease, box-shadow 170ms ease;
}

.user-workspace__brand:hover .user-workspace__logo {
  transform: translate(-1px, -2px) rotate(-4deg);
  box-shadow: 6px 6px 0 rgba(33, 31, 28, 0.94);
}

.user-workspace__logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.user-workspace__brand-text {
  min-width: 0;
  display: grid;
  gap: 0.05rem;
}

.user-workspace__brand-text strong {
  overflow: hidden;
  font-family: theme('fontFamily.display');
  font-size: 1.16rem;
  font-weight: 950;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-workspace__brand-text small {
  overflow: hidden;
  color: rgba(33, 31, 28, 0.5);
  font-size: 0.7rem;
  font-weight: 950;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-workspace__links {
  min-width: 0;
  display: flex;
  justify-content: center;
  gap: 0.36rem;
  overflow-x: auto;
  padding: 0.2rem;
  scrollbar-width: none;
}

.user-workspace__links::-webkit-scrollbar {
  display: none;
}

.user-workspace__link {
  display: inline-flex;
  min-height: 2.6rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  border: 2px solid transparent;
  border-radius: 999px;
  color: rgba(33, 31, 28, 0.68);
  padding: 0 0.78rem;
  font-size: 0.86rem;
  font-weight: 950;
  transition:
    transform 160ms ease,
    color 160ms ease,
    border-color 160ms ease,
    background 160ms ease,
    box-shadow 160ms ease;
}

.user-workspace__link:hover,
.user-workspace__link--active {
  color: var(--ink);
  border-color: var(--ink);
  background: #fff7d0;
  box-shadow: 3px 3px 0 rgba(33, 31, 28, 0.78);
  transform: translateY(-2px) rotate(-0.6deg);
}

.user-workspace__link--admin {
  color: var(--ink);
  background: rgba(255, 255, 255, 0.68);
}

.user-workspace__link--admin:hover,
.user-workspace__link--admin.user-workspace__link--active {
  background: linear-gradient(135deg, #fff7d0, #d9f8ff);
}

.user-workspace__account {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.55rem;
}

.user-workspace__balance,
.user-workspace__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 3px solid var(--ink);
  color: var(--ink);
  font-weight: 950;
  transition: transform 160ms ease, box-shadow 160ms ease;
}

.user-workspace__balance {
  min-height: 2.75rem;
  gap: 0.35rem;
  border-radius: 16px;
  background: var(--yellow);
  box-shadow: 4px 4px 0 rgba(33, 31, 28, 0.9);
  padding: 0 0.75rem;
  white-space: nowrap;
}

.user-workspace__avatar {
  height: 2.75rem;
  gap: 0.28rem;
  border-radius: 16px;
  background: linear-gradient(135deg, #ff7aa5, #4ee9ff);
  box-shadow: 4px 4px 0 rgba(33, 31, 28, 0.9);
  padding: 0 0.58rem;
}

.user-workspace__balance:hover,
.user-workspace__avatar:hover {
  transform: translate(-1px, -2px);
  box-shadow: 6px 6px 0 rgba(33, 31, 28, 0.92);
}

.user-workspace__profile-menu {
  position: relative;
  z-index: 80;
}

.user-workspace__dropdown {
  position: absolute;
  right: 0;
  top: calc(100% + 0.8rem);
  z-index: 120;
  width: 13.5rem;
  display: grid;
  gap: 0.2rem;
  border: 3px solid var(--ink);
  border-radius: 20px;
  background: rgba(255, 253, 245, 0.98);
  box-shadow: 7px 7px 0 rgba(33, 31, 28, 0.9);
  padding: 0.55rem;
}

.user-workspace__dropdown-head {
  display: grid;
  gap: 0.1rem;
  border-bottom: 2px solid rgba(33, 31, 28, 0.1);
  padding: 0.45rem 0.55rem 0.65rem;
}

.user-workspace__dropdown-head strong,
.user-workspace__dropdown-head small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-workspace__dropdown-head small {
  color: rgba(33, 31, 28, 0.54);
  font-size: 0.75rem;
  font-weight: 800;
}

.user-workspace__dropdown a,
.user-workspace__dropdown button {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  border-radius: 12px;
  padding: 0.6rem 0.55rem;
  color: var(--ink);
  font-size: 0.9rem;
  font-weight: 900;
  text-align: left;
  transition: background 150ms ease, transform 150ms ease;
}

.user-workspace__dropdown a:hover,
.user-workspace__dropdown button:hover {
  background: #fff0bd;
  transform: translateX(2px);
}

.user-workspace__menu-button {
  display: none;
  width: 2.8rem;
  height: 2.8rem;
  place-items: center;
  border: 3px solid var(--ink);
  border-radius: 16px;
  background: #fff;
  box-shadow: 4px 4px 0 rgba(33, 31, 28, 0.9);
}

.user-workspace__summary {
  position: relative;
  z-index: 10;
  width: min(86rem, calc(100vw - 2rem));
  min-height: 3.7rem;
  margin: 0.8rem auto 0;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  border: 2px solid rgba(33, 31, 28, 0.78);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.72);
  box-shadow: 5px 5px 0 rgba(33, 31, 28, 0.72);
  padding: 0.62rem 0.72rem;
  backdrop-filter: blur(14px);
  animation: userWorkspaceRise 520ms ease 100ms both;
}

.user-workspace__announcement-badge {
  position: relative;
  z-index: 1;
  display: inline-flex;
  min-height: 2.35rem;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.42rem;
  border: 1px solid rgba(33, 31, 28, 0.2);
  border-radius: 999px;
  background: linear-gradient(135deg, #fff4c8, #ffe6ef);
  color: var(--ink);
  box-shadow: 0 8px 18px rgba(33, 31, 28, 0.08);
  padding: 0 0.85rem;
  font-size: 0.82rem;
  font-weight: 950;
  white-space: nowrap;
}

.user-workspace__announcement-badge::after {
  content: '';
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 999px;
  background: #e96f8e;
  box-shadow: 0 0 0 4px rgba(233, 111, 142, 0.14);
  animation: userWorkspaceNoticePulse 1.8s ease-in-out infinite;
}

.user-workspace__announcement-ticker {
  position: relative;
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  border: 1px solid rgba(33, 31, 28, 0.12);
  border-radius: 999px;
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.96), rgba(255, 250, 232, 0.84)),
    radial-gradient(circle at 96% 50%, rgba(70, 191, 209, 0.12), transparent 28%);
  min-height: 2.35rem;
}

.user-workspace__announcement-ticker::before,
.user-workspace__announcement-ticker::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  z-index: 2;
  width: 4rem;
  pointer-events: none;
}

.user-workspace__announcement-ticker::before {
  left: 0;
  background: linear-gradient(90deg, rgba(255, 255, 255, 0.98), rgba(255, 255, 255, 0));
}

.user-workspace__announcement-ticker::after {
  right: 0;
  background: linear-gradient(270deg, rgba(255, 250, 232, 0.98), rgba(255, 250, 232, 0));
}

.user-workspace__announcement-ticker--single::before,
.user-workspace__announcement-ticker--single::after {
  display: none;
}

.user-workspace__announcement-track {
  display: flex;
  width: max-content;
  min-height: 2.35rem;
  align-items: center;
  animation: userWorkspaceTicker 28s linear infinite;
  will-change: transform;
}

.user-workspace__announcement-track--single {
  width: 100%;
  animation: none;
  will-change: auto;
}

.user-workspace__announcement-ticker:hover .user-workspace__announcement-track,
.user-workspace__announcement-ticker:focus-within .user-workspace__announcement-track {
  animation-play-state: paused;
}

.user-workspace__announcement-sequence {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 2rem;
  padding-right: 2rem;
}

.user-workspace__announcement-track--single .user-workspace__announcement-sequence {
  width: 100%;
  padding: 0 1rem;
}

.user-workspace__announcement-item {
  display: inline-flex;
  max-width: 34rem;
  align-items: center;
  gap: 0.48rem;
  border: 0;
  background: transparent;
  color: var(--ink);
  padding: 0 0.1rem;
  font-size: 0.88rem;
  font-weight: 950;
  line-height: 1;
  white-space: nowrap;
  cursor: pointer;
  transition: color 150ms ease;
}

.user-workspace__announcement-item:hover {
  color: var(--cyan);
}

.user-workspace__announcement-item:focus-visible {
  border-radius: 4px;
  outline: 2px solid currentColor;
  outline-offset: 4px;
}

.user-workspace__announcement-dot {
  width: 0.45rem;
  height: 0.45rem;
  flex: 0 0 auto;
  border-radius: 999px;
  background: #46bfd1;
}

.user-workspace__announcement-item--unread .user-workspace__announcement-dot {
  background: #e96f8e;
  box-shadow: 0 0 0 4px rgba(233, 111, 142, 0.12);
}

.user-workspace__announcement-title {
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-workspace__announcement-track--single .user-workspace__announcement-title {
  min-width: 0;
}

.user-workspace__announcement-empty {
  display: inline-flex;
  min-height: 2.35rem;
  align-items: center;
  gap: 0.45rem;
  color: rgba(33, 31, 28, 0.55);
  padding: 0 1rem;
  font-size: 0.84rem;
  font-weight: 900;
}

.user-workspace__main {
  z-index: auto;
  width: min(86rem, calc(100vw - 2rem));
  max-width: calc(100vw - 2rem);
  min-width: 0;
  margin: 0 auto;
  padding: 1.15rem 0 4rem;
}

.user-workspace__backbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
  border: 1px solid rgba(33, 31, 28, 0.14);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.7);
  box-shadow: 0 12px 28px rgba(33, 31, 28, 0.06);
  padding: 0.52rem 0.65rem;
  backdrop-filter: blur(14px);
  animation: userWorkspaceRise 320ms ease both;
}

.user-workspace__backlink {
  display: inline-flex;
  min-height: 2.2rem;
  align-items: center;
  gap: 0.4rem;
  border: 1px solid rgba(33, 31, 28, 0.18);
  border-radius: 999px;
  background: #fff7d0;
  color: var(--ink);
  padding: 0 0.75rem;
  font-size: 0.82rem;
  font-weight: 950;
  transition: transform 150ms ease, box-shadow 150ms ease, background 150ms ease;
}

.user-workspace__backlink:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 20px rgba(33, 31, 28, 0.1);
}

.user-workspace__breadcrumb {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.32rem;
  color: rgba(33, 31, 28, 0.54);
  font-size: 0.78rem;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-workspace__main :deep(.card),
.user-workspace__main :deep(.table-scroll-container),
.user-workspace__main :deep(.modal-content),
.user-workspace__main :deep(.dialog-container) {
  position: relative;
  overflow: hidden;
  border: 3px solid var(--ink);
  border-radius: 24px;
  background:
    radial-gradient(circle at 100% 0%, rgba(78, 233, 255, 0.12), transparent 34%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(255, 249, 220, 0.86));
  box-shadow: 6px 6px 0 rgba(33, 31, 28, 0.86);
}

.user-workspace__main :deep(.card-hover:hover) {
  transform: translateY(-3px) rotate(-0.3deg);
  box-shadow: 8px 8px 0 rgba(33, 31, 28, 0.9);
}

.user-workspace__main :deep(.btn) {
  border: 2px solid var(--ink);
  border-radius: 14px;
  box-shadow: 3px 3px 0 rgba(33, 31, 28, 0.82);
  font-weight: 900;
  transition: transform 150ms ease, box-shadow 150ms ease, filter 150ms ease;
}

.user-workspace__main :deep(.btn:hover:not(:disabled)) {
  transform: translate(-1px, -2px);
  box-shadow: 5px 5px 0 rgba(33, 31, 28, 0.86);
  filter: saturate(1.05);
}

.user-workspace__main :deep(.btn-primary) {
  background: linear-gradient(135deg, #ff7aa5, #ffd95a);
  color: var(--ink);
}

.user-workspace__main :deep(.btn-secondary) {
  background: rgba(255, 255, 255, 0.86);
  color: var(--ink);
}

.user-workspace__main :deep(.input) {
  border: 2px solid rgba(33, 31, 28, 0.74);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 2px 2px 0 rgba(33, 31, 28, 0.4);
  color: var(--ink);
  font-weight: 750;
}

.user-workspace__main :deep(.input:focus) {
  border-color: var(--pink);
  box-shadow: 3px 3px 0 rgba(33, 31, 28, 0.68);
}

.user-workspace__main :deep(.page-header) {
  position: relative;
  overflow: hidden;
  border: 3px solid var(--ink);
  border-radius: 24px;
  background:
    radial-gradient(circle at 100% 0%, rgba(8, 169, 214, 0.14), transparent 32%),
    linear-gradient(135deg, rgba(255, 253, 245, 0.92), rgba(255, 247, 218, 0.88));
  box-shadow: 6px 6px 0 rgba(33, 31, 28, 0.82);
  padding: 1rem;
}

.user-workspace__main :deep(.page-title) {
  font-family: theme('fontFamily.display');
  color: var(--ink);
  font-weight: 950;
}

.user-workspace__main :deep(.page-description) {
  color: rgba(33, 31, 28, 0.62);
  font-weight: 750;
}

.user-workspace__main :deep(.lingqu-console-page) {
  display: grid;
  min-width: 0;
  max-width: 100%;
  gap: 1rem;
}

.user-workspace__main :deep(.lingqu-console-hero),
.user-workspace__main :deep(.lingqu-console-card) {
  position: relative;
  overflow: hidden;
  border: 3px solid var(--ink);
  border-radius: 22px;
  background:
    radial-gradient(circle at 92% 10%, rgba(78, 233, 255, 0.18), transparent 34%),
    linear-gradient(135deg, rgba(255, 253, 245, 0.94), rgba(255, 247, 208, 0.84));
  box-shadow: 7px 7px 0 rgba(33, 31, 28, 0.86);
  padding: clamp(0.82rem, 1.8vw, 1.1rem);
}

.user-workspace__main :deep(.lingqu-console-hero::before),
.user-workspace__main :deep(.lingqu-console-card::before) {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(circle at 10px 10px, rgba(33, 31, 28, 0.055) 1.2px, transparent 1.5px);
  background-size: 18px 18px;
  opacity: 0.55;
  pointer-events: none;
}

.user-workspace__main :deep(.lingqu-console-hero > *),
.user-workspace__main :deep(.lingqu-console-card > *) {
  position: relative;
  z-index: 1;
}

.user-workspace__main :deep(.lingqu-console-hero) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.8rem;
  animation: userWorkspaceRise 520ms ease both;
}

.user-workspace__main :deep(.lingqu-console-eyebrow) {
  display: inline-flex;
  align-items: center;
  gap: 0.32rem;
  width: fit-content;
  border: 2px solid rgba(33, 31, 28, 0.2);
  border-radius: 999px;
  background: linear-gradient(135deg, #fff6cc, #ffe6ef);
  box-shadow: 0 8px 18px rgba(33, 31, 28, 0.08);
  padding: 0.22rem 0.58rem;
  font-size: 0.7rem;
  font-weight: 950;
  color: var(--ink);
}

.user-workspace__main :deep(.lingqu-console-eyebrow::before) {
  content: '';
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 999px;
  background: var(--pink);
  box-shadow: 0 0 0 3px rgba(233, 111, 142, 0.14);
}

.user-workspace__main :deep(.lingqu-console-hero h1) {
  margin-top: 0.34rem;
  font-family: theme('fontFamily.display');
  font-size: clamp(1.52rem, 2.7vw, 2.22rem);
  font-weight: 950;
  letter-spacing: 0;
  line-height: 1.04;
  color: #ff5f8f;
  text-shadow: 1.5px 1.5px 0 rgba(33, 31, 28, 0.12);
}

.user-workspace__main :deep(.lingqu-console-hero p) {
  max-width: 36rem;
  margin-top: 0.3rem;
  color: rgba(33, 31, 28, 0.64);
  font-size: 0.92rem;
  font-weight: 750;
  line-height: 1.5;
}

.user-workspace__main :deep(.lingqu-console-actions) {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.65rem;
}

.user-workspace__main :deep(.lingqu-console-button) {
  display: inline-flex;
  min-height: 2.55rem;
  align-items: center;
  justify-content: center;
  gap: 0.42rem;
  border: 2px solid var(--ink);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.86);
  color: var(--ink);
  box-shadow: 4px 4px 0 rgba(33, 31, 28, 0.84);
  padding: 0 0.82rem;
  font-weight: 950;
  transition: transform 150ms ease, box-shadow 150ms ease, filter 150ms ease;
}

.user-workspace__main :deep(.lingqu-console-button--primary) {
  background: linear-gradient(135deg, #ff7aa5, #ffd95a);
}

.user-workspace__main :deep(.lingqu-console-button:hover:not(:disabled)) {
  transform: translate(-1px, -2px) rotate(-0.5deg);
  box-shadow: 6px 6px 0 rgba(33, 31, 28, 0.9);
  filter: saturate(1.04);
}

.user-workspace__main :deep(.lingqu-console-stats) {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.8rem;
}

.user-workspace__main :deep(.lingqu-console-stat) {
  position: relative;
  overflow: hidden;
  min-height: 6.8rem;
  display: grid;
  align-content: center;
  gap: 0.18rem;
  border: 3px solid var(--ink);
  border-radius: 22px;
  background:
    radial-gradient(circle at 100% 0%, rgba(255, 122, 165, 0.12), transparent 34%),
    rgba(255, 255, 255, 0.84);
  box-shadow: 5px 5px 0 rgba(33, 31, 28, 0.82);
  padding: 0.88rem;
  transition: transform 160ms ease, box-shadow 160ms ease;
}

.user-workspace__main :deep(.lingqu-console-stat:hover) {
  transform: translateY(-3px) rotate(-0.35deg);
  box-shadow: 8px 8px 0 rgba(33, 31, 28, 0.88);
}

.user-workspace__main :deep(.lingqu-console-stat small) {
  color: rgba(33, 31, 28, 0.52);
  font-size: 0.72rem;
  font-weight: 950;
}

.user-workspace__main :deep(.lingqu-console-stat strong) {
  overflow-wrap: anywhere;
  color: var(--ink);
  font-size: 1.25rem;
  font-weight: 950;
}

.user-workspace__main :deep(.lingqu-console-section-title) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
}

.user-workspace__main :deep(.lingqu-console-section-title h2) {
  font-family: theme('fontFamily.display');
  font-size: 1.55rem;
  font-weight: 950;
  color: var(--ink);
}

.user-workspace__main :deep(.lingqu-console-section-title p) {
  color: rgba(33, 31, 28, 0.56);
  font-size: 0.9rem;
  font-weight: 800;
}

@media (max-width: 900px) {
  .user-workspace__main :deep(.lingqu-console-hero) {
    grid-template-columns: 1fr;
  }

  .user-workspace__main :deep(.lingqu-console-actions) {
    justify-content: flex-start;
  }

  .user-workspace__main :deep(.lingqu-console-stats) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .user-workspace__main :deep(.lingqu-console-stats) {
    grid-template-columns: 1fr;
  }

  .user-workspace__main :deep(.lingqu-console-button) {
    width: 100%;
  }
}

.user-menu-enter-active,
.user-menu-leave-active {
  transition: opacity 160ms ease, transform 160ms ease;
}

.user-menu-enter-from,
.user-menu-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.96);
}

@keyframes userWorkspaceDrop {
  from {
    opacity: 0;
    transform: translateY(-16px) rotate(-0.5deg);
  }
  to {
    opacity: 1;
    transform: translateY(0) rotate(0);
  }
}

@keyframes userWorkspaceRise {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes userWorkspaceSpark {
  0%,
  100% {
    transform: translateY(0) rotate(0deg);
  }
  50% {
    transform: translateY(-10px) rotate(14deg);
  }
}

@keyframes userWorkspaceComet {
  0%,
  100% {
    transform: translateX(0) rotate(-8deg);
    opacity: 0.32;
  }
  50% {
    transform: translateX(-60px) rotate(-8deg);
    opacity: 0.12;
  }
}

@keyframes userWorkspaceTicker {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-50%);
  }
}

@keyframes userWorkspaceNoticePulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.76;
  }
  50% {
    transform: scale(1.24);
    opacity: 1;
  }
}

@media (max-width: 1180px) {
  .user-workspace__nav {
    grid-template-columns: auto auto auto;
  }

  .user-workspace__menu-button {
    display: grid;
    justify-self: center;
  }

  .user-workspace__links {
    position: absolute;
    left: 1rem;
    right: 1rem;
    top: calc(100% + 0.7rem);
    z-index: 8;
    display: none;
    flex-wrap: wrap;
    justify-content: flex-start;
    border: 3px solid var(--ink);
    border-radius: 22px;
    background: rgba(255, 253, 245, 0.98);
    box-shadow: 7px 7px 0 rgba(33, 31, 28, 0.86);
    padding: 0.75rem;
  }

  .user-workspace__links--open {
    display: flex;
  }
}

@media (max-width: 760px) {
  .user-workspace__nav {
    min-height: 4.2rem;
    gap: 0.6rem;
    border-width: 2px;
    border-radius: 22px;
    box-shadow: 5px 5px 0 rgba(33, 31, 28, 0.9);
  }

  .user-workspace__brand-text small,
  .user-workspace__balance span {
    display: none;
  }

  .user-workspace__logo {
    width: 2.75rem;
    height: 2.75rem;
  }

  .user-workspace__summary {
    align-items: stretch;
    gap: 0.5rem;
    border-radius: 20px;
    padding: 0.55rem;
  }

  .user-workspace__announcement-badge {
    width: fit-content;
    min-height: 2.1rem;
    padding: 0 0.75rem;
  }

  .user-workspace__announcement-ticker,
  .user-workspace__announcement-track,
  .user-workspace__announcement-empty {
    min-height: 2.2rem;
  }

  .user-workspace__announcement-ticker::before,
  .user-workspace__announcement-ticker::after {
    width: 2.4rem;
  }

  .user-workspace__announcement-track {
    animation-duration: 22s;
  }

  .user-workspace__announcement-item {
    max-width: 18rem;
    font-size: 0.82rem;
  }
}

/* Refined console skin: keep the Lingqu mascot warmth, make the workspace feel calmer. */
.user-workspace {
  --yellow: #f5d765;
  --pink: #e9849b;
  --cyan: #48b9c8;
  --mint: #54bea0;
  --surface: rgba(255, 255, 255, 0.84);
  --surface-strong: rgba(255, 255, 255, 0.94);
  --line-soft: rgba(33, 31, 28, 0.12);
  --shadow-soft: 0 12px 32px rgba(29, 42, 42, 0.08);
  --shadow-hover: 0 16px 36px rgba(29, 42, 42, 0.11);
  background:
    radial-gradient(circle at var(--workspace-x, 72%) var(--workspace-y, 12%), rgba(72, 185, 200, 0.08), transparent 24rem),
    linear-gradient(120deg, #fbf8f1 0%, #ffffff 48%, #f1fbfa 100%);
}

.user-workspace__bg {
  background-image:
    linear-gradient(rgba(33, 31, 28, 0.018) 1px, transparent 1px),
    linear-gradient(90deg, rgba(33, 31, 28, 0.018) 1px, transparent 1px);
  background-size: 56px 56px;
  opacity: 0.34;
}

.user-workspace__spark,
.user-workspace__comet {
  display: none;
}

.user-workspace__nav,
.user-workspace__summary,
.user-workspace__dropdown,
.user-workspace__main :deep(.card),
.user-workspace__main :deep(.table-scroll-container),
.user-workspace__main :deep(.modal-content),
.user-workspace__main :deep(.dialog-container),
.user-workspace__main :deep(.page-header),
.user-workspace__main :deep(.lingqu-console-hero),
.user-workspace__main :deep(.lingqu-console-card),
.user-workspace__main :deep(.lingqu-console-stat) {
  border-color: var(--line-soft);
  box-shadow: var(--shadow-soft);
}

.user-workspace__nav {
  border-width: 1px;
  min-height: 4.35rem;
  border-radius: 20px;
  background: var(--surface-strong);
  padding: 0.5rem 0.64rem;
  animation-duration: 360ms;
}

.user-workspace__logo,
.user-workspace__balance,
.user-workspace__avatar,
.user-workspace__menu-button,
.user-workspace__main :deep(.btn),
.user-workspace__main :deep(.lingqu-console-button),
.user-workspace__main :deep(.lingqu-console-eyebrow) {
  border-color: var(--line-soft);
  box-shadow: 0 8px 18px rgba(29, 42, 42, 0.07);
}

.user-workspace__logo {
  width: 2.85rem;
  height: 2.85rem;
  border-width: 1px;
  border-radius: 14px;
  background: #fffaf0;
}

.user-workspace__brand-text strong {
  font-size: 1.08rem;
}

.user-workspace__brand-text small {
  color: rgba(33, 31, 28, 0.44);
}

.user-workspace__brand:hover .user-workspace__logo,
.user-workspace__balance:hover,
.user-workspace__avatar:hover,
.user-workspace__main :deep(.btn:hover:not(:disabled)),
.user-workspace__main :deep(.lingqu-console-button:hover:not(:disabled)),
.user-workspace__main :deep(.lingqu-console-stat:hover),
.user-workspace__main :deep(.card-hover:hover) {
  transform: translateY(-2px);
  box-shadow: var(--shadow-hover);
}

.user-workspace__link:hover,
.user-workspace__link--active {
  border-color: rgba(72, 185, 200, 0.2);
  background: #eaf8f7;
  box-shadow: 0 8px 18px rgba(72, 185, 200, 0.12);
  color: #175e65;
  transform: translateY(-1px);
}

.user-workspace__link {
  min-height: 2.42rem;
  border-width: 1px;
  color: rgba(33, 31, 28, 0.58);
}

.user-workspace__balance {
  min-height: 2.55rem;
  border-width: 1px;
  border-radius: 14px;
  background: #fff4bf;
}

.user-workspace__avatar {
  height: 2.55rem;
  border-width: 1px;
  border-radius: 14px;
  background: #eaf8f7;
}

.user-workspace__summary {
  border-width: 1px;
  min-height: 2.56rem;
  margin-top: 0.56rem;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.72);
  box-shadow: 0 8px 20px rgba(29, 42, 42, 0.05);
  gap: 0.48rem;
  padding: 0.36rem 0.46rem;
}

.user-workspace__announcement-badge {
  min-height: 1.78rem;
  border-color: rgba(33, 31, 28, 0.12);
  background: #fff8df;
  box-shadow: none;
  padding: 0 0.58rem;
  font-size: 0.72rem;
}

.user-workspace__announcement-badge::after {
  display: none;
}

.user-workspace__announcement-ticker {
  min-height: 1.78rem;
  border-color: rgba(33, 31, 28, 0.08);
  background: rgba(255, 255, 255, 0.74);
}

.user-workspace__announcement-ticker--empty {
  flex: 0 1 14rem;
  background: rgba(255, 255, 255, 0.5);
}

.user-workspace__announcement-track,
.user-workspace__announcement-empty {
  min-height: 1.78rem;
}

.user-workspace__announcement-empty {
  padding: 0 0.72rem;
  font-size: 0.76rem;
}

.user-workspace__announcement-item {
  color: rgba(33, 31, 28, 0.68);
  font-size: 0.84rem;
  font-weight: 850;
}

.user-workspace__main :deep(.btn-primary),
.user-workspace__main :deep(.lingqu-console-button--primary) {
  background: linear-gradient(135deg, #f8e08a, #f4b4bd);
}

.user-workspace__main :deep(.card),
.user-workspace__main :deep(.table-scroll-container),
.user-workspace__main :deep(.modal-content),
.user-workspace__main :deep(.dialog-container),
.user-workspace__main :deep(.page-header),
.user-workspace__main :deep(.lingqu-console-hero),
.user-workspace__main :deep(.lingqu-console-card),
.user-workspace__main :deep(.lingqu-console-stat) {
  border-width: 1px;
  background: var(--surface);
}

.user-workspace__main :deep(.lingqu-console-hero::before),
.user-workspace__main :deep(.lingqu-console-card::before) {
  opacity: 0;
}

.user-workspace__main :deep(.lingqu-console-hero h1) {
  color: var(--ink);
  text-shadow: none;
}

.user-workspace__main :deep(.lingqu-console-hero) {
  min-height: 0;
  border-radius: 18px;
  padding: clamp(0.82rem, 1.8vw, 1.08rem);
}

.user-workspace__main :deep(.lingqu-console-hero h1) {
  margin-top: 0.32rem;
  font-size: clamp(1.5rem, 2.6vw, 2.16rem);
  line-height: 1.04;
}

.user-workspace__main :deep(.lingqu-console-hero p) {
  max-width: 34rem;
  margin-top: 0.28rem;
  font-size: 0.9rem;
  line-height: 1.48;
}

.user-workspace__main :deep(.lingqu-console-eyebrow) {
  min-height: 1.8rem;
  border-width: 1px;
  background: #fff8df;
  padding: 0 0.58rem;
  font-size: 0.7rem;
}

.user-workspace__main :deep(.lingqu-console-eyebrow::before) {
  background: #48b9c8;
  box-shadow: none;
}

.user-workspace__main :deep(.lingqu-console-button) {
  min-height: 2.5rem;
  border-radius: 14px;
}

.user-workspace__main :deep(.lingqu-console-button),
.user-workspace__main :deep(.btn) {
  border-width: 1px;
}

.user-workspace__main :deep(.input) {
  border-color: rgba(33, 31, 28, 0.18);
  box-shadow: none;
}

.user-workspace__main :deep(.input:focus) {
  border-color: rgba(72, 185, 200, 0.55);
  box-shadow: 0 0 0 3px rgba(72, 185, 200, 0.12);
}

:global(body.user-workspace-active .modal-overlay) {
  background:
    radial-gradient(circle at 50% 4%, rgba(255, 255, 255, 0.24), transparent 24rem),
    rgba(33, 31, 28, 0.34);
  backdrop-filter: blur(16px) saturate(1.04);
}

:global(body.user-workspace-active .modal-content) {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(33, 31, 28, 0.18);
  border-radius: 28px;
  background:
    radial-gradient(circle at 100% 0%, rgba(70, 191, 209, 0.14), transparent 32%),
    radial-gradient(circle at 0% 100%, rgba(248, 217, 107, 0.12), transparent 34%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(255, 250, 236, 0.94));
  box-shadow: 0 30px 80px rgba(33, 31, 28, 0.2);
}

:global(body.user-workspace-active .modal-content::before) {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image: radial-gradient(circle at 1px 1px, rgba(33, 31, 28, 0.045) 1px, transparent 1.45px);
  background-size: 18px 18px;
  opacity: 0.42;
}

:global(body.user-workspace-active .modal-header),
:global(body.user-workspace-active .modal-body),
:global(body.user-workspace-active .modal-footer) {
  position: relative;
  z-index: 1;
}

:global(body.user-workspace-active .modal-header) {
  border-bottom: 1px solid rgba(33, 31, 28, 0.1);
  background: linear-gradient(135deg, rgba(255, 247, 208, 0.8), rgba(255, 255, 255, 0.74));
}

:global(body.user-workspace-active .modal-title) {
  color: var(--ink);
  font-family: theme('fontFamily.display');
  font-size: 1.1rem;
  font-weight: 950;
  letter-spacing: 0;
}

:global(body.user-workspace-active .modal-body) {
  scrollbar-width: thin;
}

:global(body.user-workspace-active .modal-footer) {
  border-top: 1px solid rgba(33, 31, 28, 0.1);
  background: rgba(255, 250, 236, 0.82);
}

:global(body.user-workspace-active .modal-content .input),
:global(body.user-workspace-active .modal-content .select-trigger) {
  border-color: rgba(33, 31, 28, 0.18);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.75) inset, 0 8px 18px rgba(33, 31, 28, 0.05);
}

:global(body.user-workspace-active .modal-content .input:focus),
:global(body.user-workspace-active .modal-content .select-trigger-open) {
  border-color: rgba(233, 111, 142, 0.48);
  box-shadow: 0 0 0 4px rgba(233, 111, 142, 0.12);
}

:global(body.user-workspace-active .modal-content .input-label) {
  color: rgba(33, 31, 28, 0.82);
  font-weight: 900;
}

:global(body.user-workspace-active .modal-content .input-hint) {
  color: rgba(33, 31, 28, 0.56);
}

:global(body.user-workspace-active .modal-content .btn) {
  border-color: rgba(33, 31, 28, 0.2);
  box-shadow: 0 8px 18px rgba(33, 31, 28, 0.08);
}

:global(body.user-workspace-active .modal-content .btn:hover:not(:disabled)) {
  transform: translateY(-2px);
  box-shadow: 0 14px 26px rgba(33, 31, 28, 0.12);
}

:global(body.user-workspace-active .modal-content .btn-primary) {
  background: linear-gradient(135deg, #f2a8b8, #f8d96b);
  color: var(--ink);
}

:global(body.user-workspace-active .modal-content .btn-secondary) {
  background: rgba(255, 255, 255, 0.9);
  color: var(--ink);
}

:global(body.user-workspace-active .select-dropdown-portal) {
  border-color: rgba(33, 31, 28, 0.16);
  border-radius: 18px;
  background:
    radial-gradient(circle at 100% 0%, rgba(70, 191, 209, 0.1), transparent 34%),
    rgba(255, 255, 255, 0.98);
  box-shadow: 0 18px 44px rgba(33, 31, 28, 0.14);
}

.user-workspace--subpage .user-workspace__summary {
  min-height: 2.72rem;
  margin-top: 0.58rem;
  border-radius: 18px;
  padding: 0.42rem 0.54rem;
}

.user-workspace--subpage .user-workspace__announcement-badge {
  min-height: 1.92rem;
  padding: 0 0.62rem;
  font-size: 0.76rem;
}

.user-workspace--subpage .user-workspace__announcement-ticker,
.user-workspace--subpage .user-workspace__announcement-track,
.user-workspace--subpage .user-workspace__announcement-empty {
  min-height: 1.92rem;
}

.user-workspace--subpage .user-workspace__main {
  padding-top: 0.78rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-page) {
  gap: 0.78rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-hero),
.user-workspace--subpage .user-workspace__main :deep(.page-header) {
  border-radius: 18px;
  padding: 0.68rem 0.78rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-hero h1),
.user-workspace--subpage .user-workspace__main :deep(.page-title) {
  margin-top: 0.18rem;
  font-size: clamp(1.12rem, 1.75vw, 1.48rem);
  line-height: 1.08;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-hero p),
.user-workspace--subpage .user-workspace__main :deep(.page-description) {
  display: none;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-actions) {
  gap: 0.45rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-button),
.user-workspace--subpage .user-workspace__main :deep(.btn) {
  min-height: 2.2rem;
  border-radius: 12px;
  padding-left: 0.68rem;
  padding-right: 0.68rem;
  font-size: 0.82rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-stats) {
  gap: 0.58rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-stat) {
  min-height: 4.85rem;
  align-content: center;
  border-radius: 16px;
  padding: 0.68rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-stat strong) {
  font-size: 1.05rem;
}

.user-workspace--subpage .user-workspace__main :deep(.lingqu-console-stat small) {
  font-size: 0.68rem;
}

@media (max-width: 760px) {
  .user-workspace__backbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.38rem;
  }

  .user-workspace__backlink {
    width: 100%;
    justify-content: center;
  }

  .user-workspace--subpage .user-workspace__summary {
    margin-top: 0.55rem;
  }
}
</style>
