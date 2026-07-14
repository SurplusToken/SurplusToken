<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.owners.title')"
    width="normal"
    @close="handleClose"
  >
    <div class="space-y-4">
      <!-- Account Info Card -->
      <div
        v-if="account"
        class="flex items-center justify-between rounded-xl border border-gray-200 bg-gradient-to-r from-gray-50 to-gray-100 p-3 dark:border-dark-500 dark:from-dark-700 dark:to-dark-600"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-primary-500 to-primary-600"
          >
            <Icon name="user" size="md" class="text-white" :stroke-width="2" />
          </div>
          <div>
            <div class="font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.owners.title') }}
            </div>
          </div>
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <!-- Error State -->
      <div
        v-else-if="error"
        class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-600 dark:border-red-800/40 dark:bg-red-900/20 dark:text-red-400"
      >
        {{ error }}
      </div>

      <template v-else>
        <!-- Primary owner (editable) -->
        <div class="space-y-1.5">
          <Input
            v-model="primaryOwnerInput"
            type="text"
            :label="t('admin.accounts.owners.primaryOwner')"
            :placeholder="t('admin.accounts.owners.primaryOwnerPlaceholder')"
            :hint="t('admin.accounts.owners.primaryOwnerHint')"
            :disabled="saving"
          />
        </div>

        <!-- Dynamic sharing rate (editable; requires a primary owner) -->
        <div class="space-y-1.5">
          <Input
            v-model="sharingRateInput"
            type="text"
            :label="t('admin.accounts.owners.sharingRate')"
            :placeholder="t('admin.accounts.owners.sharingRatePlaceholder')"
            :hint="t('admin.accounts.owners.sharingRateHint')"
            :disabled="saving || !hasPrimaryOwner"
          />
          <div v-if="!hasPrimaryOwner" class="text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.accounts.owners.sharingRateNeedsOwner') }}
          </div>
        </div>

        <!-- Co-owners (editable) -->
        <div class="space-y-1.5">
          <TextArea
            v-model="coOwnersInput"
            :label="t('admin.accounts.owners.coOwners')"
            :placeholder="t('admin.accounts.owners.coOwnersPlaceholder')"
            :hint="t('admin.accounts.owners.coOwnersHint')"
            :disabled="saving"
            rows="3"
          />
          <div v-if="parsedCoOwnerIds.length > 0" class="flex flex-wrap items-center gap-1.5 pt-1">
            <span class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.owners.currentCoOwners') }}:
            </span>
            <span
              v-for="id in parsedCoOwnerIds"
              :key="id"
              class="rounded-full bg-primary-100 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
            >
              {{ id }}
            </span>
          </div>
          <div v-else class="pt-1 text-xs text-gray-400 dark:text-gray-500">
            {{ t('admin.accounts.owners.noCoOwners') }}
          </div>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          @click="handleClose"
          class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
        >
          {{ t('admin.accounts.owners.cancel') }}
        </button>
        <button
          @click="handleSave"
          :disabled="loading || saving || !!error"
          :class="[
            'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all',
            loading || saving || !!error
              ? 'cursor-not-allowed bg-primary-400 text-white'
              : 'bg-primary-500 text-white hover:bg-primary-600'
          ]"
        >
          <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
          <span>{{ t('admin.accounts.owners.save') }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TextArea from '@/components/common/TextArea.vue'
import Input from '@/components/common/Input.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'

const { t } = useI18n()

type OwnableAccount = Pick<Account, 'id' | 'name' | 'sharing_rate_multiplier'>

const props = defineProps<{
  show: boolean
  account: OwnableAccount | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const primaryOwnerInput = ref('')
const originalPrimaryOwnerId = ref<number | null>(null)
const sharingRateInput = ref('')
const originalSharingRate = ref<number | null>(null)
const coOwnersInput = ref('')

// Parsed primary owner id: empty -> null (clear); a positive integer -> that id;
// anything else -> null (treated as cleared, and rejected on save when non-empty).
const parsedPrimaryOwnerId = computed<number | null>(() => {
  const raw = primaryOwnerInput.value.trim()
  if (!raw) return null
  const id = Number(raw)
  return Number.isInteger(id) && id > 0 ? id : null
})
const hasPrimaryOwner = computed(() => parsedPrimaryOwnerId.value !== null)

// Parsed sharing rate: empty -> null (leave unchanged); a finite number -> that value.
const parsedSharingRate = computed<number | null>(() => {
  const raw = sharingRateInput.value.trim()
  if (!raw) return null
  const v = Number(raw)
  return Number.isFinite(v) ? v : null
})

// Parse the free-form input into a clean list of co-owner user IDs:
// split on comma/space/newline, keep positive integers, dedupe, drop the primary owner id.
const parsedCoOwnerIds = computed<number[]>(() => {
  const seen = new Set<number>()
  const result: number[] = []
  for (const token of coOwnersInput.value.split(/[\s,]+/)) {
    if (!token) continue
    const id = Number(token)
    if (!Number.isInteger(id) || id <= 0) continue
    if (id === parsedPrimaryOwnerId.value) continue
    if (seen.has(id)) continue
    seen.add(id)
    result.push(id)
  }
  return result
})

watch(
  () => props.show,
  async (visible) => {
    if (visible && props.account) {
      await loadOwners()
    } else {
      resetState()
    }
  }
)

const resetState = () => {
  loading.value = false
  saving.value = false
  error.value = ''
  primaryOwnerInput.value = ''
  originalPrimaryOwnerId.value = null
  sharingRateInput.value = ''
  originalSharingRate.value = null
  coOwnersInput.value = ''
}

const loadOwners = async () => {
  if (!props.account) return
  loading.value = true
  error.value = ''
  resetState()
  loading.value = true
  try {
    const owners = await adminAPI.accounts.getAccountOwners(props.account.id)
    originalPrimaryOwnerId.value = owners.primary_owner_user_id
    primaryOwnerInput.value =
      owners.primary_owner_user_id !== null ? String(owners.primary_owner_user_id) : ''
    coOwnersInput.value = (owners.co_owner_user_ids || []).join(', ')
    const rate = props.account.sharing_rate_multiplier
    originalSharingRate.value = typeof rate === 'number' ? rate : null
    sharingRateInput.value = typeof rate === 'number' ? String(rate) : ''
  } catch (e) {
    console.error('Failed to load account owners:', e)
    error.value = t('admin.accounts.owners.loadFailed')
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!props.account || saving.value) return
  // Reject a non-empty but invalid primary owner id before touching anything.
  if (primaryOwnerInput.value.trim() && parsedPrimaryOwnerId.value === null) {
    error.value = t('admin.accounts.owners.saveFailed')
    return
  }
  saving.value = true
  error.value = ''
  try {
    // 1) Primary owner first, so the account is contributed before a rate is set.
    if (parsedPrimaryOwnerId.value !== originalPrimaryOwnerId.value) {
      await adminAPI.accounts.setAccountPrimaryOwner(props.account.id, parsedPrimaryOwnerId.value)
      originalPrimaryOwnerId.value = parsedPrimaryOwnerId.value
    }
    // 2) Co-owner set (replace semantics).
    await adminAPI.accounts.setAccountOwners(props.account.id, parsedCoOwnerIds.value)
    // 3) Sharing rate, only when an owner is present and the value actually changed.
    if (
      hasPrimaryOwner.value &&
      parsedSharingRate.value !== null &&
      parsedSharingRate.value !== originalSharingRate.value
    ) {
      await adminAPI.accounts.setAccountSharingRate(props.account.id, parsedSharingRate.value)
      originalSharingRate.value = parsedSharingRate.value
    }
    emit('saved')
    emit('close')
  } catch (e) {
    console.error('Failed to save account owners:', e)
    error.value = t('admin.accounts.owners.saveFailed')
  } finally {
    saving.value = false
  }
}

const handleClose = () => {
  emit('close')
}
</script>
