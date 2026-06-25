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
        <!-- Primary owner (read-only) -->
        <div class="space-y-1.5">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.accounts.owners.primaryOwner') }}
          </label>
          <div
            class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-300"
          >
            {{
              primaryOwnerUserId !== null
                ? t('admin.accounts.owners.userIdLabel', { id: primaryOwnerUserId })
                : t('admin.accounts.owners.noPrimaryOwner')
            }}
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
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'

const { t } = useI18n()

type OwnableAccount = Pick<Account, 'id' | 'name'>

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
const primaryOwnerUserId = ref<number | null>(null)
const coOwnersInput = ref('')

// Parse the free-form input into a clean list of co-owner user IDs:
// split on comma/space/newline, keep positive integers, dedupe, drop the primary owner id.
const parsedCoOwnerIds = computed<number[]>(() => {
  const seen = new Set<number>()
  const result: number[] = []
  for (const token of coOwnersInput.value.split(/[\s,]+/)) {
    if (!token) continue
    const id = Number(token)
    if (!Number.isInteger(id) || id <= 0) continue
    if (id === primaryOwnerUserId.value) continue
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
  primaryOwnerUserId.value = null
  coOwnersInput.value = ''
}

const loadOwners = async () => {
  if (!props.account) return
  loading.value = true
  error.value = ''
  primaryOwnerUserId.value = null
  coOwnersInput.value = ''
  try {
    const owners = await adminAPI.accounts.getAccountOwners(props.account.id)
    primaryOwnerUserId.value = owners.primary_owner_user_id
    coOwnersInput.value = (owners.co_owner_user_ids || []).join(', ')
  } catch (e) {
    console.error('Failed to load account owners:', e)
    error.value = t('admin.accounts.owners.loadFailed')
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!props.account || saving.value) return
  saving.value = true
  try {
    await adminAPI.accounts.setAccountOwners(props.account.id, parsedCoOwnerIds.value)
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
