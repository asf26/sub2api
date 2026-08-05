<template>
  <div ref="switcherRef" class="user-theme-switcher">
    <button
      type="button"
      class="user-theme-switcher__trigger"
      :aria-expanded="open"
      aria-haspopup="menu"
      :title="`当前主题：${currentTheme.label}`"
      @click="open = !open"
      @keydown.esc="open = false"
    >
      <Icon name="sparkles" size="sm" />
      <span class="user-theme-switcher__trigger-label">{{ currentTheme.shortLabel }}</span>
      <Icon name="chevronDown" size="xs" />
    </button>

    <transition name="theme-menu">
      <div v-if="open" class="user-theme-switcher__menu" role="menu" aria-label="选择用户端主题">
        <div class="user-theme-switcher__menu-head">
          <strong>界面主题</strong>
        </div>
        <button
          v-for="item in themes"
          :key="item.value"
          type="button"
          role="menuitemradio"
          class="user-theme-switcher__option"
          :class="{ 'user-theme-switcher__option--active': item.value === theme }"
          :aria-checked="item.value === theme"
          @click="selectTheme(item.value)"
        >
          <span class="user-theme-switcher__swatch" :class="`user-theme-switcher__swatch--${item.value}`">
            <i></i>
            <i></i>
            <i></i>
          </span>
          <span class="user-theme-switcher__option-copy">
            <strong>{{ item.label }}</strong>
            <small>{{ item.description }}</small>
          </span>
          <Icon v-if="item.value === theme" name="check" size="sm" />
        </button>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import Icon from '@/components/icons/Icon.vue'
import { useUserThemeStore, type UserTheme } from '@/stores/userTheme'

const themeStore = useUserThemeStore()
const { theme } = storeToRefs(themeStore)
const open = ref(false)
const switcherRef = ref<HTMLElement | null>(null)

const themes = [
  {
    value: 'business',
    label: '专业风格',
    shortLabel: '专业',
    description: '克制、清晰、高效'
  },
  {
    value: 'cartoon',
    label: '卡通风格',
    shortLabel: '卡通',
    description: '活泼、明亮、有趣'
  }
] as const satisfies ReadonlyArray<{
  value: UserTheme
  label: string
  shortLabel: string
  description: string
}>

const currentTheme = computed(() => themes.find((item) => item.value === theme.value) ?? themes[0])

function selectTheme(nextTheme: UserTheme) {
  themeStore.setTheme(nextTheme)
  open.value = false
}

function handleOutsideClick(event: MouseEvent) {
  if (switcherRef.value && !switcherRef.value.contains(event.target as Node)) {
    open.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleOutsideClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleOutsideClick)
})
</script>

<style scoped>
.user-theme-switcher {
  position: relative;
  z-index: 90;
}

.user-theme-switcher__trigger {
  display: inline-flex;
  min-height: 2.55rem;
  align-items: center;
  justify-content: center;
  gap: 0.36rem;
  border: 1px solid var(--line-soft, rgba(33, 31, 28, 0.14));
  border-radius: 14px;
  background: var(--surface-strong, rgba(255, 255, 255, 0.92));
  color: var(--ink, #211f1c);
  box-shadow: 0 8px 18px rgba(29, 42, 42, 0.07);
  padding: 0 0.68rem;
  font-size: 0.78rem;
  font-weight: 900;
  white-space: nowrap;
  transition: transform 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.user-theme-switcher__trigger:hover {
  transform: translateY(-1px);
  border-color: var(--theme-accent, #48b9c8);
  box-shadow: 0 12px 22px rgba(29, 42, 42, 0.1);
}

.user-theme-switcher__menu {
  position: absolute;
  right: 0;
  top: calc(100% + 0.7rem);
  width: 18.5rem;
  overflow: hidden;
  border: 1px solid var(--line-soft, rgba(33, 31, 28, 0.14));
  border-radius: var(--theme-radius-lg, 18px);
  background: var(--surface-strong, rgba(255, 255, 255, 0.98));
  box-shadow: var(--theme-shadow-float, 0 24px 54px rgba(29, 42, 42, 0.18));
  padding: 0.42rem;
  backdrop-filter: blur(20px);
}

.user-theme-switcher__menu-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.55rem 0.65rem 0.48rem;
}

.user-theme-switcher__menu-head strong {
  font-size: 0.86rem;
  font-weight: 950;
}

.user-theme-switcher__option {
  display: grid;
  width: 100%;
  grid-template-columns: 2.8rem minmax(0, 1fr) 1rem;
  align-items: center;
  gap: 0.65rem;
  border: 1px solid transparent;
  border-radius: var(--theme-radius-md, 13px);
  padding: 0.58rem 0.62rem;
  color: var(--ink, #211f1c);
  text-align: left;
  transition: background 150ms ease, border-color 150ms ease, transform 150ms ease;
}

.user-theme-switcher__option:hover,
.user-theme-switcher__option--active {
  border-color: var(--theme-accent-soft, rgba(72, 185, 200, 0.22));
  background: var(--theme-accent-wash, #eaf8f7);
  transform: translateX(1px);
}

.user-theme-switcher__option-copy {
  min-width: 0;
  display: grid;
  gap: 0.08rem;
}

.user-theme-switcher__option-copy strong,
.user-theme-switcher__option-copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-theme-switcher__option-copy strong {
  font-size: 0.82rem;
  font-weight: 950;
}

.user-theme-switcher__option-copy small {
  color: var(--muted, rgba(33, 31, 28, 0.52));
  font-size: 0.7rem;
  font-weight: 720;
}

.user-theme-switcher__swatch {
  width: 2.8rem;
  height: 2rem;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 0.18rem;
  overflow: hidden;
  border: 1px solid rgba(33, 31, 28, 0.12);
  border-radius: 9px;
  padding: 0.28rem;
}

.user-theme-switcher__swatch i {
  width: 0.5rem;
  border-radius: 2px 2px 0 0;
}

.user-theme-switcher__swatch i:nth-child(1) {
  height: 0.65rem;
}

.user-theme-switcher__swatch i:nth-child(2) {
  height: 1rem;
}

.user-theme-switcher__swatch i:nth-child(3) {
  height: 0.8rem;
}

.user-theme-switcher__swatch--cartoon {
  background: #fff8df;
}

.user-theme-switcher__swatch--cartoon i:nth-child(1) {
  background: #e9849b;
}

.user-theme-switcher__swatch--cartoon i:nth-child(2) {
  background: #48b9c8;
}

.user-theme-switcher__swatch--cartoon i:nth-child(3) {
  background: #f5d765;
}

.user-theme-switcher__swatch--business {
  background: #f4f7f9;
}

.user-theme-switcher__swatch--business i:nth-child(1) {
  background: #147d78;
}

.user-theme-switcher__swatch--business i:nth-child(2) {
  background: #2456a6;
}

.user-theme-switcher__swatch--business i:nth-child(3) {
  background: #303943;
}

.theme-menu-enter-active,
.theme-menu-leave-active {
  transition: opacity 150ms ease, transform 150ms ease;
}

.theme-menu-enter-from,
.theme-menu-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}

@media (max-width: 760px) {
  .user-theme-switcher__trigger {
    width: 2.55rem;
    padding: 0;
  }

  .user-theme-switcher__trigger-label,
  .user-theme-switcher__trigger > :deep(svg:last-child) {
    display: none;
  }

  .user-theme-switcher__menu {
    right: -5.1rem;
    width: min(18rem, calc(100vw - 2rem));
  }
}
</style>
