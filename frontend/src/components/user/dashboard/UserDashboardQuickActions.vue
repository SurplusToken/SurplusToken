<template>
  <div class="rounded-lg border border-border bg-card">
    <div class="border-b border-border px-6 py-4">
      <h2 class="text-sm font-medium text-foreground">{{ t('dashboard.quickActions') }}</h2>
    </div>
    <div class="space-y-2 p-4">
      <button @click="router.push('/keys')" class="group flex w-full items-center gap-3 rounded-lg border border-border px-4 py-3 text-left transition-colors hover:bg-muted">
        <Icon name="key" size="md" class="shrink-0 text-muted-foreground" />
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-foreground">{{ t('dashboard.createApiKey') }}</p>
          <p class="text-xs text-muted-foreground">{{ t('dashboard.generateNewKey') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="sm"
          class="shrink-0 text-muted-foreground"
        />
      </button>

      <button @click="router.push('/usage')" class="group flex w-full items-center gap-3 rounded-lg border border-border px-4 py-3 text-left transition-colors hover:bg-muted">
        <Icon name="chart" size="md" class="shrink-0 text-muted-foreground" />
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-foreground">{{ t('dashboard.viewUsage') }}</p>
          <p class="text-xs text-muted-foreground">{{ t('dashboard.checkDetailedLogs') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="sm"
          class="shrink-0 text-muted-foreground"
        />
      </button>

      <button v-if="canUseBatchImage" @click="router.push('/batch-image')" class="group flex w-full items-center gap-3 rounded-lg border border-border px-4 py-3 text-left transition-colors hover:bg-muted">
        <Icon name="sparkles" size="md" class="shrink-0 text-muted-foreground" />
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-foreground">{{ t('dashboard.batchImageAgent') }}</p>
          <p class="text-xs text-muted-foreground">{{ t('dashboard.batchImageAgentDesc') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="sm"
          class="shrink-0 text-muted-foreground"
        />
      </button>

      <button @click="router.push('/redeem')" class="group flex w-full items-center gap-3 rounded-lg border border-border px-4 py-3 text-left transition-colors hover:bg-muted">
        <Icon name="gift" size="md" class="shrink-0 text-muted-foreground" />
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-foreground">{{ t('dashboard.redeemCode') }}</p>
          <p class="text-xs text-muted-foreground">{{ t('dashboard.addBalanceWithCode') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="sm"
          class="shrink-0 text-muted-foreground"
        />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>
