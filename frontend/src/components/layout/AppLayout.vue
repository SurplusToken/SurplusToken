<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Admin: sidebar layout (unchanged) -->
    <template v-if="isAdmin">
      <!-- Sidebar -->
      <AppSidebar />

      <!-- Main Content Area -->
      <div
        class="relative min-h-screen transition-all duration-300"
        :class="[sidebarCollapsed ? 'lg:ml-12' : 'lg:ml-52']"
      >
        <!-- Header -->
        <AppHeader />

        <!-- Main Content -->
        <main class="p-4 md:p-6 lg:p-8">
          <slot />
        </main>
      </div>
    </template>

    <!-- Regular user: top horizontal navigation layout -->
    <template v-else>
      <AppTopNav />

      <!-- Main Content -->
      <main class="mx-auto max-w-7xl px-4 py-6 md:py-8">
        <slot />
      </main>
    </template>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import AppTopNav from './AppTopNav.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
