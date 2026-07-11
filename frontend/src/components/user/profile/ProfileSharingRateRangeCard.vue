<template>
  <div class="card" data-testid="sharing-rate-range-card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 class="text-lg font-medium text-gray-900 dark:text-white">
            {{ t('profile.sharingRateRange.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('profile.sharingRateRange.description') }}
          </p>
        </div>
        <span
          v-if="!filterEnabled"
          class="badge badge-gray"
          data-testid="sharing-rate-range-disabled"
        >
          {{ t('profile.sharingRateRange.filterDisabled') }}
        </span>
      </div>
    </div>

    <form class="space-y-5 px-6 py-6" @submit.prevent="saveRange">
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="block">
          <span class="input-label">{{ t('profile.sharingRateRange.minimum') }}</span>
          <input
            v-model="minimumInput"
            data-testid="sharing-rate-min"
            type="number"
            min="0"
            :max="effectiveCap"
            step="0.0001"
            class="input"
            :placeholder="t('profile.sharingRateRange.unbounded')"
            :disabled="loading || saving"
          />
        </label>
        <label class="block">
          <span class="input-label">{{ t('profile.sharingRateRange.maximum') }}</span>
          <input
            v-model="maximumInput"
            data-testid="sharing-rate-max"
            type="number"
            min="0"
            :max="effectiveCap"
            step="0.0001"
            class="input"
            :placeholder="t('profile.sharingRateRange.unbounded')"
            :disabled="loading || saving"
          />
        </label>
      </div>

      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('profile.sharingRateRange.marketBounds', { floor: effectiveFloor, cap: effectiveCap }) }}
      </p>

      <div class="flex justify-end">
        <button
          type="submit"
          class="btn btn-primary"
          data-testid="sharing-rate-range-save"
          :disabled="loading || saving"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { userAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = withDefaults(defineProps<{
  filterEnabled: boolean
  floor?: number
  cap?: number
}>(), {
  floor: 0,
  cap: 5,
})

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const minimumInput = ref<string | number>('')
const maximumInput = ref<string | number>('')

const effectiveFloor = computed(() => normalizeBound(props.floor, 0))
const effectiveCap = computed(() => normalizeBound(props.cap, 5))

function normalizeBound(value: number, fallback: number): number {
  return Number.isFinite(value) && value >= 0 && value <= 5 ? value : fallback
}

function setInputs(range: { min: number | null; max: number | null }): void {
  minimumInput.value = range.min == null ? '' : String(range.min)
  maximumInput.value = range.max == null ? '' : String(range.max)
}

function parseInput(value: string | number): number | null | undefined {
  if (String(value).trim() === '') return null
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed < 0 || parsed > effectiveCap.value) {
    return undefined
  }
  return parsed
}

async function loadRange(): Promise<void> {
  loading.value = true
  try {
    setInputs(await userAPI.getSharingRateRange())
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('profile.sharingRateRange.loadFailed')),
    )
  } finally {
    loading.value = false
  }
}

async function saveRange(): Promise<void> {
  if (saving.value) return
  const min = parseInput(minimumInput.value)
  const max = parseInput(maximumInput.value)
  if (min === undefined || max === undefined) {
    appStore.showError(t('profile.sharingRateRange.outOfRange', { cap: effectiveCap.value }))
    return
  }
  if (min != null && max != null && min > max) {
    appStore.showError(t('profile.sharingRateRange.invalidOrder'))
    return
  }

  saving.value = true
  try {
    setInputs(await userAPI.updateSharingRateRange({ min, max }))
    appStore.showSuccess(t('profile.sharingRateRange.saveSuccess'))
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('profile.sharingRateRange.saveFailed')),
    )
  } finally {
    saving.value = false
  }
}

onMounted(loadRange)
</script>
