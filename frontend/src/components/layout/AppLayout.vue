<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <AppTopNav />

    <!-- Main Content: admin keeps full-width pages, users get a centered container -->
    <main :class="isAdmin ? 'p-4 md:p-6 lg:p-8' : 'mx-auto max-w-7xl px-4 py-6 md:py-8'">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppTopNav from './AppTopNav.vue'

const authStore = useAuthStore()
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
