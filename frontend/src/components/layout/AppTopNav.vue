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
      <router-link :to="homePath" class="flex shrink-0 items-center gap-2.5" @click="closeMobileMenu">
        <span class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-lg">
          <img v-if="settingsLoaded" :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
        </span>
        <span class="hidden text-base font-bold text-gray-900 transition-colors hover:text-primary-600 dark:text-white dark:hover:text-primary-400 sm:block">
          {{ siteName }}
        </span>
      </router-link>

      <!-- Center: Horizontal Nav (desktop) -->
      <nav v-if="!appStore.backendModeEnabled" class="hidden min-w-0 flex-1 items-center gap-1 lg:flex">
        <!-- Admin area: admin menu -->
        <template v-if="showAdminNav">
          <router-link
            v-for="item in adminCoreItems"
            :key="item.path"
            :to="item.path"
            class="whitespace-nowrap text-sm font-medium px-3 py-1.5 rounded-md transition-colors"
            :class="isActive(item.path) ? activeClass : inactiveClass"
            :id="tourIdFor(item.path)"
            :data-tour="dataTourFor(item.path)"
            @click="handleNavClick(item.path)"
          >
            {{ item.label }}
          </router-link>

          <router-link
            v-for="item in adminLeftoverItems"
            :key="item.path"
            :to="item.path"
            class="whitespace-nowrap text-sm font-medium px-3 py-1.5 rounded-md transition-colors"
            :class="isActive(item.path) ? activeClass : inactiveClass"
            :id="tourIdFor(item.path)"
            :data-tour="dataTourFor(item.path)"
            @click="handleNavClick(item.path)"
          >
            {{ item.label }}
          </router-link>

          <TopNavDropdown
            v-for="group in adminGroups"
            :key="group.key"
            :label="group.label"
            :items="group.items"
            @select="handleNavClick"
          />

          <!-- Back to user view -->
          <router-link
            to="/dashboard"
            class="ml-1 whitespace-nowrap text-sm font-medium px-3 py-1.5 rounded-md transition-colors"
            :class="inactiveClass"
            @click="closeMobileMenu"
          >
            {{ t('nav.userView') }}
          </router-link>
        </template>

        <!-- User area: user menu (+ admin panel entry for admins) -->
        <template v-else>
          <router-link
            v-for="item in primaryItems"
            :key="item.path"
            :to="item.path"
            class="whitespace-nowrap text-sm font-medium px-3 py-1.5 rounded-md transition-colors"
            :class="isActive(item.path) ? activeClass : inactiveClass"
            :data-tour="dataTourFor(item.path)"
            @click="handleNavClick(item.path)"
          >
            {{ item.label }}
          </router-link>

          <TopNavDropdown
            v-if="overflowItems.length"
            :label="t('nav.more')"
            :items="overflowItems"
            @select="handleNavClick"
          />

          <router-link
            v-if="isAdmin"
            to="/admin/dashboard"
            class="whitespace-nowrap rounded-md border border-primary-200 bg-primary-50/60 px-3 py-1.5 text-sm font-semibold text-primary-600 transition-colors hover:bg-primary-100 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-400 dark:hover:bg-primary-500/20"
            @click="closeMobileMenu"
          >
            {{ t('nav.adminPanel') }}
          </router-link>
        </template>
      </nav>
      <div v-else class="hidden min-w-0 flex-1 lg:block" />
      <div class="min-w-0 flex-1 lg:hidden" />

      <!-- Right: Announcements + Docs + Language + Subscription + Theme + Balance + User -->
      <div class="flex shrink-0 items-center gap-1 sm:gap-3">
        <!-- Announcement Bell -->
        <AnnouncementBell v-if="user" />

        <!-- Docs Link -->
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="hidden items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white sm:flex"
        >
          <Icon name="book" size="sm" />
          <span class="hidden xl:inline">{{ t('nav.docs') }}</span>
        </a>

        <!-- Model Plaza Entry -->
        <router-link
          v-if="user && modelPlazaEnabled"
          :to="{ path: '/model-plaza', query: { embedded: '1' } }"
          class="hidden items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white sm:flex"
        >
          <Icon name="grid" size="sm" />
          <span class="hidden xl:inline">{{ t('nav.modelPlaza') }}</span>
        </router-link>

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
          <!-- Admin area: grouped admin menu -->
          <template v-if="showAdminNav">
            <router-link
              v-for="item in adminCoreItems"
              :key="item.path"
              :to="item.path"
              class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
              :class="isActive(item.path) ? activeClass : inactiveClass"
              :id="tourIdFor(item.path)"
              :data-tour="dataTourFor(item.path)"
              @click="closeMobileMenu(); handleNavClick(item.path)"
            >
              <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 topnav-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
              <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
              {{ item.label }}
            </router-link>

            <router-link
              v-for="item in adminLeftoverItems"
              :key="item.path"
              :to="item.path"
              class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
              :class="isActive(item.path) ? activeClass : inactiveClass"
              :data-tour="dataTourFor(item.path)"
              @click="closeMobileMenu(); handleNavClick(item.path)"
            >
              <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 topnav-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
              <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
              {{ item.label }}
            </router-link>

            <template v-for="group in adminGroups" :key="group.key">
              <div class="px-3 pb-1 pt-3 text-xs font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
                {{ group.label }}
              </div>
              <router-link
                v-for="item in group.items"
                :key="item.path"
                :to="item.path"
                class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
                :class="isActive(item.path) ? activeClass : inactiveClass"
                @click="closeMobileMenu(); handleNavClick(item.path)"
              >
                <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 topnav-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
                <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
                {{ item.label }}
              </router-link>
            </template>

            <router-link
              to="/dashboard"
              class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
              :class="inactiveClass"
              @click="closeMobileMenu"
            >
              {{ t('nav.userView') }}
            </router-link>
          </template>

          <!-- User area -->
          <template v-else>
            <router-link
              v-for="item in userNavItems"
              :key="item.path"
              :to="item.path"
              class="flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors"
              :class="isActive(item.path) ? activeClass : inactiveClass"
              :data-tour="dataTourFor(item.path)"
              @click="closeMobileMenu(); handleNavClick(item.path)"
            >
              <span v-if="item.iconSvg" class="h-5 w-5 flex-shrink-0 topnav-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
              <component v-else :is="item.icon" class="h-5 w-5 flex-shrink-0" />
              {{ item.label }}
            </router-link>

            <router-link
              v-if="isAdmin"
              to="/admin/dashboard"
              class="flex items-center gap-3 rounded-md border border-primary-200 bg-primary-50/60 px-3 py-2 text-sm font-semibold text-primary-600 transition-colors hover:bg-primary-100 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-400 dark:hover:bg-primary-500/20"
              @click="closeMobileMenu"
            >
              {{ t('nav.adminPanel') }}
            </router-link>
          </template>
        </nav>
      </div>
    </transition>
  </header>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAdminSettingsStore, useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import Icon from '@/components/icons/Icon.vue'
import BalancePill from './BalancePill.vue'
import UserMenu from './UserMenu.vue'
import TopNavDropdown from './TopNavDropdown.vue'
import { useUserNavItems, type NavItem } from './useUserNavItems'
import { useAdminNavItems } from './useAdminNavItems'
import { sanitizeSvg } from '@/utils/sanitize'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()
const adminSettingsStore = useAdminSettingsStore()

const { userNavItems, refreshBatchImageAccess } = useUserNavItems()
const { adminNavItems } = useAdminNavItems()

const user = computed(() => authStore.user)
const isAdmin = computed(() => authStore.isAdmin)
const inAdminArea = computed(() => route.path.startsWith('/admin'))
const showAdminNav = computed(() => isAdmin.value && inAdminArea.value)

// Site settings from appStore (cached, no flicker)
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const homePath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

const docUrl = computed(() => sanitizeUrl('https://docs.surplustoken.com'))
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

const activeClass = 'text-primary-600 bg-primary-50 dark:bg-primary-500/10 dark:text-primary-400'
const inactiveClass = 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-dark-800'

// ---------- User nav ----------
// 主行核心项；其余收进"更多"下拉
const primaryPaths = ['/dashboard', '/keys', '/usage', '/subscriptions', '/purchase']
const primaryItems = computed(() => userNavItems.value.filter((item) => primaryPaths.includes(item.path)))
const overflowItems = computed(() => userNavItems.value.filter((item) => !primaryPaths.includes(item.path)))

// ---------- Admin nav ----------
// 展开带 children 的组（expandOnly 父项仅作分组 key，不生成条目）
const flatAdminItems = computed(() => {
  const out = new Map<string, NavItem>()
  for (const item of adminNavItems.value) {
    if (item.children?.length) {
      if (!item.expandOnly && !item.children.some((c) => c.path === item.path)) {
        out.set(item.path, item)
      }
      for (const child of item.children) {
        out.set(child.path, child)
      }
    } else {
      out.set(item.path, item)
    }
  }
  return out
})

// 主行核心项（分组/账号为新手引导锚点，必须保持顶层可见）
const adminCorePaths = ['/admin/dashboard', '/admin/users', '/admin/groups', '/admin/accounts']
const adminCoreItems = computed(() =>
  adminCorePaths.map((p) => flatAdminItems.value.get(p)).filter((i): i is NavItem => Boolean(i))
)

// 语义分组下拉
const adminGroupDefs: { key: string; labelKey: string; paths: string[] }[] = [
  { key: 'channels', labelKey: 'nav.channelManagement', paths: ['/admin/channels/pricing', '/admin/channels/monitor'] },
  {
    key: 'payment',
    labelKey: 'nav.groupPayment',
    paths: [
      '/admin/orders/dashboard',
      '/admin/orders',
      '/admin/orders/plans',
      '/admin/redeem',
      '/admin/promo-codes',
      '/admin/contribution-withdrawals',
      '/admin/affiliates/invites',
      '/admin/affiliates/rebates',
      '/admin/affiliates/transfers',
    ],
  },
  { key: 'ops', labelKey: 'nav.groupOps', paths: ['/admin/ops', '/admin/usage', '/admin/audit-logs'] },
  { key: 'security', labelKey: 'nav.groupSecurity', paths: ['/admin/risk-control', '/admin/prompt-audit'] },
  {
    key: 'system',
    labelKey: 'nav.groupSystem',
    paths: ['/admin/subscriptions', '/admin/carpools', '/admin/announcements', '/admin/proxies', '/admin/settings'],
  },
]

const adminGroups = computed(() => {
  const groups = adminGroupDefs.map((def) => {
    let items = def.paths
      .map((p) => flatAdminItems.value.get(p))
      .filter((i): i is NavItem => Boolean(i))
    // 自定义菜单页归入"系统"组
    if (def.key === 'system') {
      const customItems = [...flatAdminItems.value.values()].filter((i) => i.path.startsWith('/custom/'))
      items = [...items, ...customItems]
    }
    return { key: def.key, label: t(def.labelKey), items }
  })
  return groups.filter((g) => g.items.length > 0)
})

// 未分配的核心/分组条目（如简单模式下的 /keys）作为顶层链接展示，保证菜单不丢项
const adminLeftoverItems = computed(() => {
  const assigned = new Set<string>(adminCorePaths)
  for (const def of adminGroupDefs) {
    for (const p of def.paths) assigned.add(p)
  }
  return [...flatAdminItems.value.values()].filter(
    (i) => !assigned.has(i.path) && !i.path.startsWith('/custom/')
  )
})

// ---------- Tour anchors ----------
function tourIdFor(path: string): string | undefined {
  if (path === '/admin/groups') return 'sidebar-group-manage'
  if (path === '/admin/accounts') return 'sidebar-channel-manage'
  return undefined
}

function dataTourFor(path: string): string | undefined {
  return path === '/keys' ? 'sidebar-my-keys' : undefined
}

// ---------- UI state ----------
const mobileMenuOpen = ref(false)
const isDark = ref(document.documentElement.classList.contains('dark'))

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function closeMobileMenu() {
  mobileMenuOpen.value = false
}

function handleNavClick(itemPath: string) {
  // Map paths to tour selectors (mirrors the old AppSidebar behavior)
  const pathToSelector: Record<string, string> = {
    '/admin/groups': '#sidebar-group-manage',
    '/admin/accounts': '#sidebar-channel-manage',
    '/keys': '[data-tour="sidebar-my-keys"]'
  }

  const selector = pathToSelector[itemPath]
  if (selector && onboardingStore.isCurrentStep(selector)) {
    onboardingStore.nextStep(500)
  }
}

watch(
  () => route.path,
  () => {
    closeMobileMenu()
  }
)

// Fetch admin settings (for feature-gated nav items like Ops).
watch(
  isAdmin,
  (v) => {
    if (v) {
      adminSettingsStore.fetch()
    }
  },
  { immediate: true }
)

onMounted(() => {
  void refreshBatchImageAccess()
  if (isAdmin.value) {
    adminSettingsStore.fetch()
  }
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
