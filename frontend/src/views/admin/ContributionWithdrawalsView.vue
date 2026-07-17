<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="min-w-56 flex-1 sm:max-w-72">
            <input
              v-model.trim="searchQuery"
              type="search"
              class="input"
              :placeholder="t('admin.contributionWithdrawals.searchPlaceholder')"
              @keyup.enter="applySearch"
            />
          </div>
          <Select
            v-model="filters.status"
            :options="statusFilterOptions"
            class="w-40"
            @change="handleFilterChange"
          />
          <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadWithdrawals">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="withdrawals" :loading="loading" row-key="id">
          <template #cell-user_id="{ row }">
            <div class="max-w-64">
              <div class="truncate font-medium text-gray-900 dark:text-white">
                {{ row.username || row.user_email || `#${row.user_id}` }}
              </div>
              <div v-if="row.user_email && row.username" class="truncate text-xs text-gray-500 dark:text-dark-300">
                {{ row.user_email }}
              </div>
              <div class="text-xs text-gray-400">#{{ row.user_id }}</div>
            </div>
          </template>

          <template #cell-amount="{ value }">
            <span class="font-mono font-semibold text-gray-900 dark:text-white">{{ formatAmount(value) }}</span>
          </template>

          <template #cell-payment="{ row }">
            <div class="max-w-72 space-y-0.5 whitespace-normal">
              <div class="font-medium text-gray-900 dark:text-white">
                {{ paymentMethodLabel(row.payment_method) }} · {{ row.payee_name }}
              </div>
              <div class="break-all text-xs text-gray-600 dark:text-dark-200">{{ row.payment_account }}</div>
              <div v-if="row.request_note" class="line-clamp-2 text-xs text-gray-400">{{ row.request_note }}</div>
            </div>
          </template>

          <template #cell-status="{ value }">
            <span :class="['badge', statusClass(value)]">{{ statusLabel(value) }}</span>
          </template>

          <template #cell-requested_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-300">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-review="{ row }">
            <div class="max-w-64 whitespace-normal text-xs text-gray-500 dark:text-dark-300">
              <div v-if="row.payment_reference">
                {{ t('admin.contributionWithdrawals.paymentReferenceShort') }}: {{ row.payment_reference }}
              </div>
              <div v-if="row.review_note" class="mt-0.5">{{ row.review_note }}</div>
              <span v-if="!row.payment_reference && !row.review_note">-</span>
            </div>
          </template>

          <template #cell-actions="{ row }">
            <div v-if="row.status === 'pending'" class="flex items-center gap-1">
              <button type="button" class="btn btn-primary btn-xs" @click="openReview(row, 'paid')">
                <Icon name="check" size="sm" />
                {{ t('admin.contributionWithdrawals.markPaid') }}
              </button>
              <button type="button" class="btn btn-secondary btn-xs text-red-600 dark:text-red-400" @click="openReview(row, 'rejected')">
                <Icon name="x" size="sm" />
                {{ t('admin.contributionWithdrawals.reject') }}
              </button>
            </div>
            <span v-else class="text-xs text-gray-400">{{ t('admin.contributionWithdrawals.completed') }}</span>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.pageSize"
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="reviewDialog.show"
      :title="reviewDialog.mode === 'paid' ? t('admin.contributionWithdrawals.paidDialogTitle') : t('admin.contributionWithdrawals.rejectDialogTitle')"
      width="normal"
      @close="closeReview"
    >
      <form id="contribution-withdrawal-review-form" class="space-y-4" @submit.prevent="submitReview">
        <div v-if="reviewDialog.target" class="rounded-md border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="flex items-center justify-between gap-3">
            <span class="text-sm text-gray-600 dark:text-dark-200">{{ reviewDialog.target.user_email || `#${reviewDialog.target.user_id}` }}</span>
            <span class="font-mono font-semibold text-gray-900 dark:text-white">{{ formatAmount(reviewDialog.target.amount) }}</span>
          </div>
          <div class="mt-2 break-all text-xs text-gray-500 dark:text-dark-300">
            {{ paymentMethodLabel(reviewDialog.target.payment_method) }} · {{ reviewDialog.target.payee_name }} · {{ reviewDialog.target.payment_account }}
          </div>
        </div>

        <div v-if="reviewDialog.target?.has_payment_qr_code">
          <span class="input-label">{{ t('admin.contributionWithdrawals.qrCode') }}</span>
          <div class="mt-2 flex min-h-64 items-center justify-center rounded-md border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900">
            <span v-if="reviewDialog.qrLoading" class="text-sm text-gray-500">{{ t('common.loading') }}</span>
            <a v-else-if="reviewDialog.qrURL" :href="reviewDialog.qrURL" target="_blank" rel="noopener noreferrer">
              <img :src="reviewDialog.qrURL" :alt="t('admin.contributionWithdrawals.qrCode')" class="max-h-80 max-w-full object-contain" />
            </a>
            <span v-else class="text-sm text-red-500">{{ t('admin.contributionWithdrawals.qrCodeLoadFailed') }}</span>
          </div>
        </div>

        <label v-if="reviewDialog.mode === 'paid'" class="block">
          <span class="input-label">{{ t('admin.contributionWithdrawals.paymentReference') }}</span>
          <input v-model.trim="reviewDialog.paymentReference" type="text" maxlength="255" required class="input" />
          <span class="input-hint">{{ t('admin.contributionWithdrawals.paymentReferenceHint') }}</span>
        </label>

        <label class="block">
          <span class="input-label">{{ t('admin.contributionWithdrawals.reviewNote') }}</span>
          <textarea
            v-model.trim="reviewDialog.reviewNote"
            rows="3"
            maxlength="500"
            class="input"
            :required="reviewDialog.mode === 'rejected'"
            :placeholder="reviewDialog.mode === 'rejected' ? t('admin.contributionWithdrawals.rejectReasonPlaceholder') : t('common.optional')"
          ></textarea>
        </label>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="reviewDialog.submitting" @click="closeReview">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="contribution-withdrawal-review-form"
            :class="reviewDialog.mode === 'paid' ? 'btn btn-primary' : 'btn btn-danger'"
            :disabled="!canSubmitReview || reviewDialog.submitting"
          >
            {{ reviewDialog.submitting ? t('common.processing') : t('common.confirm') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatDateTime } from '@/utils/format'
import type { Column } from '@/components/common/types'
import type { ContributionWithdrawal, ContributionWithdrawalStatus } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const withdrawals = ref<ContributionWithdrawal[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive<{ status: ContributionWithdrawalStatus | '' }>({ status: 'pending' })
const pagination = reactive({ page: 1, pageSize: getPersistedPageSize(), total: 0 })
const reviewDialog = reactive<{
  show: boolean
  submitting: boolean
  mode: 'paid' | 'rejected'
  target: ContributionWithdrawal | null
  paymentReference: string
  reviewNote: string
  qrLoading: boolean
  qrURL: string
}>({
  show: false,
  submitting: false,
  mode: 'paid',
  target: null,
  paymentReference: '',
  reviewNote: '',
  qrLoading: false,
  qrURL: '',
})

const columns = computed<Column[]>(() => [
  { key: 'id', label: t('admin.contributionWithdrawals.columns.id') },
  { key: 'user_id', label: t('admin.contributionWithdrawals.columns.user') },
  { key: 'amount', label: t('admin.contributionWithdrawals.columns.amount') },
  { key: 'payment', label: t('admin.contributionWithdrawals.columns.payment') },
  { key: 'status', label: t('admin.contributionWithdrawals.columns.status') },
  { key: 'requested_at', label: t('admin.contributionWithdrawals.columns.requestedAt') },
  { key: 'review', label: t('admin.contributionWithdrawals.columns.review') },
  { key: 'actions', label: t('admin.contributionWithdrawals.columns.actions') },
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.contributionWithdrawals.allStatuses') },
  { value: 'pending', label: statusLabel('pending') },
  { value: 'paid', label: statusLabel('paid') },
  { value: 'rejected', label: statusLabel('rejected') },
  { value: 'cancelled', label: statusLabel('cancelled') },
])

const canSubmitReview = computed(() => {
  if (!reviewDialog.target) return false
  return reviewDialog.mode === 'paid'
    ? reviewDialog.paymentReference.length > 0
      && (!reviewDialog.target.has_payment_qr_code || reviewDialog.qrURL.length > 0)
    : reviewDialog.reviewNote.length > 0
})

function formatAmount(value: number): string {
  return `$${Number(value || 0).toFixed(6)}`
}

function paymentMethodLabel(method: string): string {
  return t(`admin.contributionWithdrawals.methods.${method}`)
}

function statusLabel(status: string): string {
  return t(`admin.contributionWithdrawals.statuses.${status}`)
}

function statusClass(status: string): string {
  if (status === 'paid') return 'badge-success'
  if (status === 'pending') return 'badge-warning'
  if (status === 'rejected') return 'badge-danger'
  return 'badge-gray'
}

async function loadWithdrawals(): Promise<void> {
  loading.value = true
  try {
    const response = await adminAPI.contributionWithdrawals.list(pagination.page, pagination.pageSize, {
      status: filters.status,
      search: searchQuery.value,
    })
    withdrawals.value = response.items
    pagination.total = response.total
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.contributionWithdrawals.loadFailed'))
  } finally {
    loading.value = false
  }
}

function applySearch(): void {
  pagination.page = 1
  void loadWithdrawals()
}

function handleFilterChange(): void {
  pagination.page = 1
  void loadWithdrawals()
}

function handlePageChange(page: number): void {
  pagination.page = page
  void loadWithdrawals()
}

function handlePageSizeChange(pageSize: number): void {
  pagination.pageSize = pageSize
  pagination.page = 1
  void loadWithdrawals()
}

function revokeReviewQRCode(): void {
  if (reviewDialog.qrURL) URL.revokeObjectURL(reviewDialog.qrURL)
  reviewDialog.qrURL = ''
}

async function openReview(withdrawal: ContributionWithdrawal, mode: 'paid' | 'rejected'): Promise<void> {
  revokeReviewQRCode()
  reviewDialog.target = withdrawal
  reviewDialog.mode = mode
  reviewDialog.paymentReference = ''
  reviewDialog.reviewNote = ''
  reviewDialog.show = true
  if (!withdrawal.has_payment_qr_code) return
  reviewDialog.qrLoading = true
  try {
    const blob = await adminAPI.contributionWithdrawals.getQRCode(withdrawal.id)
    if (reviewDialog.target?.id === withdrawal.id) {
      reviewDialog.qrURL = URL.createObjectURL(blob)
    }
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.contributionWithdrawals.qrCodeLoadFailed'))
  } finally {
    if (reviewDialog.target?.id === withdrawal.id) reviewDialog.qrLoading = false
  }
}

function closeReview(): void {
  if (reviewDialog.submitting) return
  reviewDialog.show = false
  reviewDialog.target = null
  reviewDialog.qrLoading = false
  revokeReviewQRCode()
}

async function submitReview(): Promise<void> {
  if (!canSubmitReview.value || !reviewDialog.target || reviewDialog.submitting) return
  reviewDialog.submitting = true
  try {
    await adminAPI.contributionWithdrawals.review(reviewDialog.target.id, {
      status: reviewDialog.mode,
      payment_reference: reviewDialog.mode === 'paid' ? reviewDialog.paymentReference : undefined,
      review_note: reviewDialog.reviewNote || undefined,
    })
    appStore.showSuccess(
      reviewDialog.mode === 'paid'
        ? t('admin.contributionWithdrawals.paidSuccess')
        : t('admin.contributionWithdrawals.rejectedSuccess'),
    )
    reviewDialog.show = false
    reviewDialog.target = null
    reviewDialog.qrLoading = false
    revokeReviewQRCode()
    await loadWithdrawals()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.contributionWithdrawals.reviewFailed'))
  } finally {
    reviewDialog.submitting = false
  }
}

onMounted(loadWithdrawals)
onUnmounted(revokeReviewQRCode)
</script>
