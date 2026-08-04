<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
    <a
      :href="docUrl"
      target="_blank"
      rel="noopener noreferrer"
      class="fixed right-4 top-4 z-30 inline-flex h-10 w-10 items-center justify-center rounded-lg border border-gray-200 bg-white/90 text-gray-600 shadow-sm backdrop-blur transition-colors hover:bg-gray-100 hover:text-gray-900 dark:border-dark-700 dark:bg-dark-800/90 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
      :title="t('home.viewDocs')"
      :aria-label="t('home.viewDocs')"
    >
      <Icon name="book" size="md" />
    </a>
  </div>

  <!-- Default Home Page -->
  <div v-else class="flex min-h-screen flex-col bg-white dark:bg-dark-950">
    <!-- Header -->
    <header
      class="sticky top-0 z-20 border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-dark-950"
    >
      <nav class="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <!-- Logo -->
        <router-link to="/" class="flex items-center gap-2.5">
          <div class="h-8 w-8 overflow-hidden rounded-lg">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-lg font-semibold tracking-tight text-slate-900 dark:text-white">
            {{ siteName }}
          </span>
        </router-link>

        <!-- Nav Actions -->
        <div class="flex items-center gap-2">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-2 rounded-lg border border-slate-200 px-3 py-2 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-slate-800 dark:text-slate-200 dark:hover:bg-dark-900"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-primary-500 text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            {{ t('home.dashboard') }}
          </router-link>
          <router-link
            v-else
            to="/login"
            class="btn btn-primary rounded-lg px-4 py-2 text-sm"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="flex-1">
      <!-- Hero Section - Centered -->
      <section class="px-6 py-20 md:py-28">
        <div class="mx-auto max-w-4xl text-center">
          <h1
            class="text-5xl font-semibold tracking-tight md:text-7xl"
          >
            <span class="text-primary-500">{{ siteName }}</span>
          </h1>
          <p
            class="mx-auto mt-6 max-w-2xl text-xl font-light text-slate-500 dark:text-slate-400 md:text-2xl"
          >
            {{ siteSubtitle }}
          </p>

          <!-- CTA Buttons -->
          <div class="mt-10 flex flex-col items-center justify-center gap-4 sm:flex-row">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="btn btn-primary rounded-lg px-8 py-4 text-lg"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-secondary rounded-lg px-8 py-4 text-lg"
            >
              {{ t('home.docs') }}
            </a>
            <a
              v-else
              :href="githubUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-secondary rounded-lg px-8 py-4 text-lg"
            >
              GitHub
            </a>
          </div>

          <!-- Feature Tags -->
          <div class="mt-12 flex flex-wrap items-center justify-center gap-3">
            <div
              class="inline-flex items-center gap-2 rounded-full border border-slate-200 px-4 py-2 dark:border-slate-800"
            >
              <Icon name="swap" size="sm" class="text-primary-500" />
              <span class="text-sm font-medium text-slate-600 dark:text-slate-300">{{
                t('home.tags.subscriptionToApi')
              }}</span>
            </div>
            <div
              class="inline-flex items-center gap-2 rounded-full border border-slate-200 px-4 py-2 dark:border-slate-800"
            >
              <Icon name="shield" size="sm" class="text-primary-500" />
              <span class="text-sm font-medium text-slate-600 dark:text-slate-300">{{
                t('home.tags.stickySession')
              }}</span>
            </div>
            <div
              class="inline-flex items-center gap-2 rounded-full border border-slate-200 px-4 py-2 dark:border-slate-800"
            >
              <Icon name="chart" size="sm" class="text-primary-500" />
              <span class="text-sm font-medium text-slate-600 dark:text-slate-300">{{
                t('home.tags.realtimeBilling')
              }}</span>
            </div>
          </div>
        </div>
      </section>

      <!-- Features Section -->
      <section class="bg-[#F0F9FC] px-6 py-20 dark:bg-dark-900">
        <div class="mx-auto max-w-6xl">
          <div class="grid gap-6 md:grid-cols-3">
            <!-- Feature 1: Unified Gateway -->
            <div
              class="rounded-2xl border border-slate-200 bg-white p-6 dark:border-slate-800 dark:bg-dark-950"
            >
              <div
                class="mb-4 inline-flex items-center justify-center rounded-lg bg-primary-50 p-2 text-primary-600 dark:bg-primary-950 dark:text-primary-400"
              >
                <Icon name="server" size="lg" />
              </div>
              <h3 class="mb-2 text-lg font-semibold text-slate-900 dark:text-white">
                {{ t('home.features.unifiedGateway') }}
              </h3>
              <p class="text-sm leading-relaxed text-slate-500 dark:text-slate-400">
                {{ t('home.features.unifiedGatewayDesc') }}
              </p>
            </div>

            <!-- Feature 2: Account Pool -->
            <div
              class="rounded-2xl border border-slate-200 bg-white p-6 dark:border-slate-800 dark:bg-dark-950"
            >
              <div
                class="mb-4 inline-flex items-center justify-center rounded-lg bg-primary-50 p-2 text-primary-600 dark:bg-primary-950 dark:text-primary-400"
              >
                <svg
                  class="h-6 w-6"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z"
                  />
                </svg>
              </div>
              <h3 class="mb-2 text-lg font-semibold text-slate-900 dark:text-white">
                {{ t('home.features.multiAccount') }}
              </h3>
              <p class="text-sm leading-relaxed text-slate-500 dark:text-slate-400">
                {{ t('home.features.multiAccountDesc') }}
              </p>
            </div>

            <!-- Feature 3: Billing & Quota -->
            <div
              class="rounded-2xl border border-slate-200 bg-white p-6 dark:border-slate-800 dark:bg-dark-950"
            >
              <div
                class="mb-4 inline-flex items-center justify-center rounded-lg bg-primary-50 p-2 text-primary-600 dark:bg-primary-950 dark:text-primary-400"
              >
                <svg
                  class="h-6 w-6"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"
                  />
                </svg>
              </div>
              <h3 class="mb-2 text-lg font-semibold text-slate-900 dark:text-white">
                {{ t('home.features.balanceQuota') }}
              </h3>
              <p class="text-sm leading-relaxed text-slate-500 dark:text-slate-400">
                {{ t('home.features.balanceQuotaDesc') }}
              </p>
            </div>
          </div>
        </div>
      </section>

      <!-- Supported Providers -->
      <section class="px-6 py-20">
        <div class="mx-auto max-w-6xl">
          <div class="mb-10 text-center">
            <h2 class="mb-3 text-2xl font-semibold tracking-tight text-slate-900 dark:text-white">
              {{ t('home.providers.title') }}
            </h2>
            <p class="text-sm text-slate-500 dark:text-slate-400">
              {{ t('home.providers.description') }}
            </p>
          </div>

          <div class="flex flex-wrap items-center justify-center gap-4">
            <!-- Claude - Supported -->
            <div
              class="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-5 py-3 dark:border-slate-800 dark:bg-dark-900"
            >
              <div
                class="flex h-8 w-8 items-center justify-center rounded-lg bg-orange-500"
              >
                <span class="text-xs font-bold text-white">C</span>
              </div>
              <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('home.providers.claude') }}</span>
              <span
                class="rounded bg-primary-50 px-1.5 py-0.5 text-[10px] font-medium text-primary-600 dark:bg-primary-950 dark:text-primary-400"
                >{{ t('home.providers.supported') }}</span
              >
            </div>
            <!-- GPT - Supported -->
            <div
              class="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-5 py-3 dark:border-slate-800 dark:bg-dark-900"
            >
              <div
                class="flex h-8 w-8 items-center justify-center rounded-lg bg-green-500"
              >
                <span class="text-xs font-bold text-white">G</span>
              </div>
              <span class="text-sm font-medium text-slate-700 dark:text-slate-200">GPT</span>
              <span
                class="rounded bg-primary-50 px-1.5 py-0.5 text-[10px] font-medium text-primary-600 dark:bg-primary-950 dark:text-primary-400"
                >{{ t('home.providers.supported') }}</span
              >
            </div>
            <!-- Gemini - Supported -->
            <div
              class="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-5 py-3 dark:border-slate-800 dark:bg-dark-900"
            >
              <div
                class="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-500"
              >
                <span class="text-xs font-bold text-white">G</span>
              </div>
              <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('home.providers.gemini') }}</span>
              <span
                class="rounded bg-primary-50 px-1.5 py-0.5 text-[10px] font-medium text-primary-600 dark:bg-primary-950 dark:text-primary-400"
                >{{ t('home.providers.supported') }}</span
              >
            </div>
            <!-- Antigravity - Supported -->
            <div
              class="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-5 py-3 dark:border-slate-800 dark:bg-dark-900"
            >
              <div
                class="flex h-8 w-8 items-center justify-center rounded-lg bg-rose-500"
              >
                <span class="text-xs font-bold text-white">A</span>
              </div>
              <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('home.providers.antigravity') }}</span>
              <span
                class="rounded bg-primary-50 px-1.5 py-0.5 text-[10px] font-medium text-primary-600 dark:bg-primary-950 dark:text-primary-400"
                >{{ t('home.providers.supported') }}</span
              >
            </div>
            <!-- More - Coming Soon -->
            <div
              class="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-5 py-3 opacity-60 dark:border-slate-800 dark:bg-dark-900"
            >
              <div
                class="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-500"
              >
                <span class="text-xs font-bold text-white">+</span>
              </div>
              <span class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('home.providers.more') }}</span>
              <span
                class="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                >{{ t('home.providers.soon') }}</span
              >
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="border-t border-slate-200 px-6 py-8 dark:border-slate-800">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:justify-between sm:text-left"
      >
        <p class="text-sm text-slate-500 dark:text-slate-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-slate-500 transition-colors hover:text-slate-700 dark:text-slate-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-slate-500 transition-colors hover:text-slate-700 dark:text-slate-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'SurplusToken')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Shared AI token gateway for internal teams')
const docUrl = computed(() => sanitizeUrl('https://docs.surplustoken.com'))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/ypd666/SurplusAI'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
