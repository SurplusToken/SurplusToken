<template>
  <header
    class="sticky top-0 z-40 border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-dark-900"
  >
    <div class="mx-auto flex h-14 items-center gap-4 px-4">
      <!-- Mobile: Hamburger -->
      <button
        class="btn-ghost btn-icon rounded-lg lg:hidden"
        :aria-label="t('common.toggleMenu')"
        @click="mobileMenuOpen = !mobileMenuOpen"
      >
        <Icon :name="mobileMenuOpen ? 'x' : 'menu'" size="md" />
      </button>

      <!-- Left: Logo + Site Name -->
      <router-link to="/dashboard" class="flex shrink-0 items-center gap-2.5" @click="closeMobileMenu">
        <span class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-lg">
          <img v-if="settingsLoaded" :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
        </span>
        <span class="hidden text-base font-bold text-gray-900 transition-colors hover:text-primary-600 dark:text-white dark:hover:text-primary-400 sm:block">
          {{ siteName }}
        </span>
      </router-link>

      <!-- Center: Horizontal Nav (desktop) -->
      <nav v-if="!appStore.backendModeEnabled" class="hidden min-w-0 flex-1 items-center gap-1 lg:flex">
        <router-link
          v-for="item in primaryItems"
          :key="item.path"
          :to="item.path"
          class="whitespace-nowrap text-sm font-medium px-3 py-1.5 rounded-md transition-colors"
          :class="isActive(item.path)
            ? 'text-primary-600 bg-primary-50 dark:bg-primary-500/10 dark:text-primary-400'
            : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-dark-800'"
          :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
          @click="handleNavClick(item.path)"
        >
          {{ item.label }}
        </router-link>

        <!-- More Dropdown -->
        <div v-if="overflowItems.length" class="relative" ref="moreRef">
          <button
            class="flex items-center gap-1 whitespace-nowrap text-sm font-medium px-3 py-1.5 rounded-md transition-colors"
            :class="moreActive
              ? 'text-primary-600 bg-primary-50 dark:bg-primary-500/10 dark:text-primary-400'
              : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-dark-800'"
            :aria-label="t('nav.more')"
            @click="moreOpen = !moreOpen"
          >
            {{ t('nav.more') }}
            <Icon name="chevronDown" size="sm" class="transition-transform duration-200" :class="{ 'rotate-180': moreOpen }" />
          </button>
          <transition name="dropdown">
            <div v-if="moreOpen" class="dropdown left-0 mt-2 w-52">
              <div class="py-1">
                <router-link
                  v-for="item in overflowItems"
                  :key="item.path"
                  :to="item.path"
                  class="dropdown-item"
                  :class="{ 'bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-400': isActive(item.path) }"
                  @click="closeMore(); handleNavClick(item.path)"
                >
                  <span v-if="item.iconSvg" class="h-4 w-4 flex-shrink-0 topnav-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
                  <component v-else :is="item.icon" class="h-4 w-4 flex-shrink-0" />
                  {{ item.label }}
                </router-link>
              </div>
            </div>
          </transition>
        </div>
      </nav>
      <div v-else class="hidden min-w-0 flex-1 lg:block" />
      <div class="min-w-0 flex-1 lg:hidden" />

      <!-- Right: Announcements + Language + Subscription + Theme + Balance + User -->
      <div class="flex shrink-0 items-center gap-1 sm:gap-3">
        <!-- Announcement Bell -->
        <AnnouncementBell v-if="user" />

        <!-- Language Switcher -->
        <LocaleSwitcher />

        <!-- Subscription Progress (for users with active subscriptions) -->
        <SubscriptionProgressMini v-if="user" />

        <!-- Theme Toggle -->
        <button
          class="btn-ghost btn-icon rounded-lg"
          :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
          :title="isDark ? t('nav.lightMode') : t('nav.darkMode')"
          @click="toggleTheme"
        >
          <Icon v-if="isDark" name="sun" size="md" class="text-amber-500" />
          <Icon v-else name="moon" size="md" />
        </button>

        <!-- Balance Display -->
        <BalancePill />

        <!-- User Dropdown -->
        <UserMenu />
      </div>
    </div>

    <!-- Mobile Nav Panel -->
    <transition name="dropdown">
      <div
        v-if="mobileMenuOpen && !appStore.backendModeEnabled"
        class="border-t border-slate-200 bg-white dark:border-slate-800 dark:bg-dark-900 lg:hidden"
      >
        <nav class="mx-auto max-w-7xl space-y-0.5 px-4 py-3">
          <router-link
            v-for="item in userNavItems"
            :key="item.path"
            :to="item.path"
            class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
            :class="isActive(item.path)
              ? 'text-primary-600 bg-primary-50 dark:bg-primary-500/10 dark:text-primary-400'
              : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-dark-800'"
            :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
            @click="closeMobileMenu(); handleNavClick(item.path)"
          >
            <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 topnav-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
            <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            {{ item.label }}
          </router-link>
        </nav>
      </div>
    </transition>
  </header>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import Icon from '@/components/icons/Icon.vue'
import BalancePill from './BalancePill.vue'
import UserMenu from './UserMenu.vue'
import { useUserNavItems } from './useUserNavItems'
import { sanitizeSvg } from '@/utils/sanitize'
import { sanitizeUrl } from '@/utils/url'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()

const { userNavItems, refreshBatchImageAccess } = useUserNavItems()

const user = computed(() => authStore.user)

// Site settings from appStore (cached, no flicker)
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

// 主行核心项；其余收进"更多"下拉
const primaryPaths = ['/dashboard', '/keys', '/usage', '/subscriptions', '/purchase']
const primaryItems = computed(() => userNavItems.value.filter((item) => primaryPaths.includes(item.path)))
const overflowItems = computed(() => userNavItems.value.filter((item) => !primaryPaths.includes(item.path)))

const moreOpen = ref(false)
const moreRef = ref<HTMLElement | null>(null)
const mobileMenuOpen = ref(false)

const isDark = ref(document.documentElement.classList.contains('dark'))

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

const moreActive = computed(() => overflowItems.value.some((item) => isActive(item.path)))

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function closeMore() {
  moreOpen.value = false
}

function closeMobileMenu() {
  mobileMenuOpen.value = false
}

function handleNavClick(itemPath: string) {
  // Map paths to tour selectors (mirrors AppSidebar behavior)
  const pathToSelector: Record<string, string> = {
    '/keys': '[data-tour="sidebar-my-keys"]'
  }

  const selector = pathToSelector[itemPath]
  if (selector && onboardingStore.isCurrentStep(selector)) {
    onboardingStore.nextStep(500)
  }
}

function handleClickOutside(event: MouseEvent) {
  if (moreRef.value && !moreRef.value.contains(event.target as Node)) {
    closeMore()
  }
}

watch(
  () => route.path,
  () => {
    closeMobileMenu()
    closeMore()
  }
)

onMounted(() => {
  void refreshBatchImageAccess()
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}

/* Custom SVG icon: constrain size without overriding uploaded SVG colors */
.topnav-svg-icon {
  color: currentColor;
}

.topnav-svg-icon :deep(svg) {
  display: block;
  width: 1rem;
  height: 1rem;
}
</style>
