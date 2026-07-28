<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-3">
          <!--
            异常条：管理员真正需要的是「哪几辆车现在要我处理」，而不是把 30 辆车
            从头看到尾。放在最上面，点一下就把表格筛到对应的那几辆。
          -->
          <div
            v-if="alertSummary.length > 0"
            data-testid="carpool-admin-alerts"
            class="flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs dark:border-amber-900/40 dark:bg-amber-900/20"
          >
            <span class="font-medium text-amber-800 dark:text-amber-300">{{ t('carpool.adminPage.alertsTitle') }}</span>
            <button
              v-for="item in alertSummary"
              :key="item.kind"
              type="button"
              class="rounded border border-amber-300 px-2 py-0.5 text-amber-800 hover:bg-amber-100 dark:border-amber-700 dark:text-amber-200 dark:hover:bg-amber-900/40"
              :class="{ 'bg-amber-200 dark:bg-amber-900/60': alertFilter === item.kind }"
              @click="alertFilter = alertFilter === item.kind ? '' : item.kind"
            >
              {{ t(`carpool.adminPage.alerts.${item.kind}`) }} · {{ item.count }}
            </button>
          </div>

          <div class="flex flex-wrap items-center gap-3">
            <Select v-model="statusFilter" :options="statusOptions" class="w-44" />
            <input v-model="searchQuery" type="search" class="input w-56" :placeholder="t('carpool.adminPage.searchPlaceholder')" />
            <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <span class="text-xs text-gray-500 dark:text-dark-300">
              {{ t('carpool.adminPage.total', { count: filteredCarpools.length, all: carpools.length }) }}
            </span>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="filteredCarpools" :loading="loading" row-key="id" :actions-count="7">
          <template #cell-name="{ row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
              <div class="mt-0.5 flex flex-wrap gap-1">
                <span v-if="row.visibility === 'invite_only'" class="badge badge-gray">
                  {{ t('carpool.visibility.inviteOnly') }}
                </span>
                <span v-if="!isQuotaCar(row)" class="badge badge-gray">
                  {{ t('carpool.customRule.badge') }}
                </span>
                <span
                  v-for="kind in carpoolAlerts(row)"
                  :key="kind"
                  class="badge badge-warning"
                >
                  {{ t(`carpool.adminPage.alerts.${kind}`) }}
                </span>
              </div>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span :class="['badge', statusBadgeClass(row)]">{{ statusLabel(row) }}</span>
          </template>

          <template #cell-owner="{ row }">
            <span class="truncate text-sm text-gray-700 dark:text-dark-200">{{ row.organizer }}</span>
          </template>

          <template #cell-declared="{ row }">
            <span v-if="!isQuotaCar(row)" class="text-xs text-gray-400">—</span>
            <span v-else class="text-sm text-gray-700 dark:text-dark-200">
              ${{ Math.round(row.declaredTotalUsd) }} / ${{ Math.round(row.weeklyLimitUsd) }}
              <span class="ml-1 text-[10px] text-gray-400">{{ declaredPercent(row) }}%</span>
            </span>
          </template>

          <template #cell-settled="{ row }">
            <span class="text-xs" :class="row.settledAt ? 'text-gray-500 dark:text-dark-300' : 'text-gray-400'">
              {{ row.settledAt ? formatDateOnly(row.settledAt) : '—' }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button type="button" class="action-btn" :title="t('carpool.adminPage.actions.members')" @click="openMembers(row)">
                <Icon name="users" size="sm" />
                <span class="text-xs">{{ t('carpool.adminPage.actions.members') }}</span>
              </button>
              <button v-if="canEdit(row)" type="button" class="action-btn" :title="t('carpool.adminPage.actions.edit')" @click="openEdit(row)">
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('carpool.adminPage.actions.edit') }}</span>
              </button>
              <button v-if="canTransfer(row)" type="button" class="action-btn" :title="t('carpool.adminPage.actions.transfer')" @click="openTransfer(row)">
                <Icon name="swap" size="sm" />
                <span class="text-xs">{{ t('carpool.adminPage.actions.transfer') }}</span>
              </button>
              <button v-if="row.status === 'recruiting'" type="button" class="action-btn" :title="row.joinLocked ? t('carpool.actions.unlock') : t('carpool.actions.lock')" @click="toggleLock(row)">
                <Icon name="lock" size="sm" />
                <span class="text-xs">{{ row.joinLocked ? t('carpool.actions.unlock') : t('carpool.actions.lock') }}</span>
              </button>
              <button v-if="row.status === 'confirmed'" type="button" class="action-btn" :title="t('carpool.actions.unconfirm')" @click="run('unconfirm', row)">
                <Icon name="arrowLeft" size="sm" />
                <span class="text-xs">{{ t('carpool.actions.unconfirm') }}</span>
              </button>
              <button v-if="row.status === 'confirmed'" type="button" class="action-btn action-btn-primary" :title="t('carpool.actions.launch')" @click="run('launch', row)">
                <Icon name="play" size="sm" />
                <span class="text-xs">{{ t('carpool.actions.launch') }}</span>
              </button>
              <button v-if="canCancel(row)" type="button" class="action-btn action-btn-danger" :title="t('carpool.actions.cancel')" @click="run('cancel', row)">
                <Icon name="ban" size="sm" />
                <span class="text-xs">{{ t('carpool.actions.cancel') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState :title="t('carpool.adminPage.empty')" />
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <!-- 成员管理 -->
    <BaseDialog :show="membersOpen" :title="t('carpool.adminPage.membersDialog.title', { name: activeCarpool?.name || '' })" width="normal" @close="closeMembers">
      <div v-if="activeCarpool" class="space-y-3">
        <p class="text-xs text-gray-500 dark:text-dark-300">
          {{ t('carpool.adminPage.membersDialog.hint') }}
        </p>
        <p v-if="!canManageMembers(activeCarpool)" class="rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-300">
          {{ t('carpool.adminPage.membersDialog.readOnly') }}
        </p>
        <!-- 建车时上传的群二维码：只在弹窗打开时现取，关掉即吊销 object URL（私密车的二维码=入场券） -->
        <div
          v-if="activeCarpool.hasGroupQrCode"
          class="flex items-center gap-3 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-700/40"
        >
          <img
            v-if="qrCodeUrl"
            :src="qrCodeUrl"
            :alt="t('carpool.adminPage.membersDialog.groupQr')"
            class="h-14 w-14 shrink-0 rounded-md border border-gray-200 object-cover dark:border-dark-600"
          />
          <div class="min-w-0 text-xs">
            <div class="font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.adminPage.membersDialog.groupQr') }}</div>
            <a
              v-if="qrCodeUrl"
              :href="qrCodeUrl"
              target="_blank"
              rel="noopener"
              class="mt-1 inline-block text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
            >
              {{ t('carpool.adminPage.membersDialog.qrOpen') }}
            </a>
            <span v-else-if="qrLoading" class="mt-1 inline-block text-gray-400">{{ t('carpool.adminPage.membersDialog.qrLoading') }}</span>
            <button
              v-else-if="qrFailed"
              type="button"
              class="mt-1 inline-block text-amber-600 hover:text-amber-700 dark:text-amber-400"
              @click="loadQrCode(activeCarpool.id)"
            >
              {{ t('carpool.adminPage.membersDialog.qrFailed') }}
            </button>
          </div>
        </div>
        <p v-if="rosterLoading" class="text-xs text-gray-400">{{ t('carpool.joinDialog.rosterLoading') }}</p>
        <p v-else-if="rosterFailed" class="text-xs text-amber-600">{{ t('carpool.joinDialog.rosterFailed') }}</p>
        <p v-else-if="roster.length === 0" class="text-xs text-gray-400">{{ t('carpool.adminPage.membersDialog.empty') }}</p>
        <ul v-else class="space-y-2" data-testid="carpool-admin-roster">
          <li
            v-for="member in roster"
            :key="member.userId"
            class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-gray-200 px-3 py-2 dark:border-dark-600"
          >
            <span class="flex min-w-0 flex-col text-sm">
              <span class="flex min-w-0 items-center gap-1.5">
                <span class="truncate text-gray-800 dark:text-dark-100">
                  {{ member.username || t('carpool.joinDialog.rosterAnonymous', { id: member.userId }) }}
                </span>
                <span v-if="member.role === 'owner'" class="badge badge-gray shrink-0">
                  {{ t('carpool.joinDialog.rosterOwner') }}
                </span>
              </span>
              <span v-if="member.email" class="truncate text-xs text-gray-400">{{ member.email }}</span>
            </span>
            <span class="flex items-center gap-2">
              <template v-if="editingUserId === member.userId">
                <input v-model.number="editingQuota" type="number" min="1" step="1" class="input h-8 w-24 text-sm" />
                <button type="button" class="btn-xs btn-xs-primary" :disabled="memberPending" @click="saveQuota(member.userId)">{{ t('common.save') }}</button>
                <button type="button" class="btn-xs" @click="editingUserId = null">{{ t('common.cancel') }}</button>
              </template>
              <template v-else>
                <span v-if="isQuotaCar(activeCarpool)" class="text-sm font-medium text-gray-600 dark:text-dark-200">${{ Math.round(member.declaredWeeklyQuotaUsd) }}/{{ t('carpool.adminPage.perWeek') }}</span>
                <button
                  v-if="canManageMembers(activeCarpool) && isQuotaCar(activeCarpool)"
                  type="button"
                  class="btn-xs"
                  @click="startEditQuota(member.userId, member.declaredWeeklyQuotaUsd)"
                >
                  {{ t('carpool.adminPage.actions.editQuota') }}
                </button>
                <button
                  v-if="canManageMembers(activeCarpool) && member.role !== 'owner'"
                  type="button"
                  class="btn-xs btn-xs-danger"
                  :disabled="memberPending"
                  @click="removeMember(member.userId)"
                >
                  {{ t('carpool.adminPage.actions.remove') }}
                </button>
              </template>
            </span>
          </li>
        </ul>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button type="button" class="btn btn-secondary" @click="closeMembers">{{ t('common.close') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- 编辑车辆 -->
    <BaseDialog :show="editOpen" :title="t('carpool.adminPage.editDialog.title')" width="normal" @close="editOpen = false">
      <div class="space-y-3">
        <div>
          <label class="input-label" for="admin-carpool-name">{{ t('carpool.fields.name') }}</label>
          <input id="admin-carpool-name" v-model="editForm.name" type="text" class="input" />
        </div>
        <div>
          <label class="input-label" for="admin-carpool-desc">{{ t('carpool.fields.description') }}</label>
          <textarea id="admin-carpool-desc" v-model="editForm.description" rows="2" class="input"></textarea>
        </div>
        <div>
          <label class="input-label" for="admin-carpool-visibility">{{ t('carpool.fields.visibility') }}</label>
          <Select
            id="admin-carpool-visibility"
            v-model="editForm.visibility"
            :options="visibilityOptions"
          />
        </div>
        <div>
          <label class="input-label" for="admin-carpool-start">{{ t('carpool.fields.scheduledStart') }}</label>
          <input id="admin-carpool-start" v-model="editForm.scheduledStartAt" type="date" class="input" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="editOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="actionPending || !editForm.name.trim()" @click="saveEdit">{{ t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- 转让车主 -->
    <BaseDialog :show="transferOpen" :title="t('carpool.adminPage.transferDialog.title')" width="normal" @close="transferOpen = false">
      <div class="space-y-3">
        <p class="text-xs text-gray-500 dark:text-dark-300">{{ t('carpool.adminPage.transferDialog.hint') }}</p>
        <p v-if="rosterLoading" class="text-xs text-gray-400">{{ t('carpool.joinDialog.rosterLoading') }}</p>
        <Select
          v-else
          v-model="transferTargetId"
          :options="transferOptions"
          :placeholder="t('carpool.adminPage.transferDialog.pick')"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="transferOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="actionPending || !transferTargetId" @click="saveTransfer">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- 危险操作确认 -->
    <BaseDialog :show="!!confirmAction" :title="confirmTitle" width="narrow" @close="confirmAction = null">
      <p class="text-sm text-gray-700 dark:text-dark-200">{{ confirmMessage }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="confirmAction = null">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="actionPending" @click="runConfirmed">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import carpoolAPI, {
  type Carpool,
  type CarpoolRosterMember,
  type CarpoolVisibility,
} from '@/api/carpools'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateOnly } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const STATUSES = ['recruiting', 'confirmed', 'starting', 'active', 'ended', 'cancelled'] as const

// 「确认后多久还没启动算超时」——与设计文档里承诺给车主的 24 小时一致。
const LAUNCH_OVERDUE_MS = 24 * 60 * 60 * 1000

type AlertKind = 'launchOverdue' | 'unsettled' | 'overDeclared' | 'readyToConfirm'
type ConfirmKind = 'unconfirm' | 'launch' | 'cancel'

const carpools = ref<Carpool[]>([])
const loading = ref(false)
const actionPending = ref(false)
const statusFilter = ref('')
const alertFilter = ref('')
const searchQuery = ref('')

const membersOpen = ref(false)
const editOpen = ref(false)
const transferOpen = ref(false)
const activeCarpool = ref<Carpool | null>(null)
const roster = ref<CarpoolRosterMember[]>([])
const rosterLoading = ref(false)
const rosterFailed = ref(false)
const memberPending = ref(false)
const editingUserId = ref<number | null>(null)
const editingQuota = ref<number | null>(null)
const transferTargetId = ref<number | null>(null)
const confirmAction = ref<{ kind: ConfirmKind; carpool: Carpool } | null>(null)
const qrCodeUrl = ref<string | null>(null)
const qrLoading = ref(false)
const qrFailed = ref(false)

const editForm = reactive({
  name: '',
  description: '',
  visibility: 'public' as CarpoolVisibility,
  scheduledStartAt: '',
})

const statusOptions = computed(() => [
  { value: '', label: t('carpool.adminPage.allStatuses') },
  ...STATUSES.map((s) => ({ value: s, label: t(`carpool.status.${s}`) })),
])

const visibilityOptions = computed(() => [
  { value: 'public', label: t('carpool.visibility.public') },
  { value: 'invite_only', label: t('carpool.visibility.inviteOnly') },
])

const transferOptions = computed(() =>
  roster.value
    .filter((m) => m.role !== 'owner')
    .map((m) => ({
      value: m.userId,
      label: m.username || t('carpool.joinDialog.rosterAnonymous', { id: m.userId }),
    })))

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('carpool.adminPage.columns.name'), sortable: false },
  { key: 'status', label: t('carpool.adminPage.columns.status'), sortable: false },
  { key: 'owner', label: t('carpool.adminPage.columns.owner'), sortable: false },
  { key: 'memberCount', label: t('carpool.adminPage.columns.members'), sortable: true },
  { key: 'declared', label: t('carpool.adminPage.columns.declared'), sortable: false },
  { key: 'settled', label: t('carpool.adminPage.columns.settled'), sortable: false },
  { key: 'actions', label: t('carpool.adminPage.columns.actions'), sortable: false },
])

// 自定义规则车（含升级前的老车）不走申报制，额度相关的列与操作对它们没有意义。
function isQuotaCar(carpool: Carpool): boolean {
  return (carpool.pricingModel || 'quota') === 'quota'
}

function declaredPercent(carpool: Carpool): number {
  if (carpool.weeklyLimitUsd <= 0) return 0
  return Math.round((carpool.declaredTotalUsd / carpool.weeklyLimitUsd) * 100)
}

// 与用户侧拼车页同一套徽章口径，管理员看到的和用户看到的状态观感一致。
function statusLabel(carpool: Carpool): string {
  if (carpool.status === 'recruiting' && carpool.joinLocked) return t('carpool.status.locked')
  if (carpool.status === 'recruiting' && isQuotaCar(carpool) && carpool.remainingJoinableUsd <= 0) return t('carpool.status.full')
  return t(`carpool.status.${carpool.status}`)
}

function statusBadgeClass(carpool: Carpool): string {
  if (carpool.status === 'cancelled' || carpool.status === 'ended') return 'badge-gray'
  if (carpool.status === 'active') return 'badge-success'
  if (carpool.status === 'confirmed') return 'badge-warning'
  if (carpool.joinLocked || (isQuotaCar(carpool) && carpool.remainingJoinableUsd <= 0)) return 'badge-warning'
  return 'badge-primary'
}

// carpoolAlerts 是「这辆车现在要不要我处理」的判据。只用列表里已有的字段算，
// 不额外发请求——管理页一次要渲染全部车。
function carpoolAlerts(carpool: Carpool): AlertKind[] {
  const kinds: AlertKind[] = []
  if (carpool.status === 'confirmed' && carpool.confirmedAt
    && Date.now() - new Date(carpool.confirmedAt).getTime() > LAUNCH_OVERDUE_MS) {
    kinds.push('launchOverdue')
  }
  if (carpool.status === 'ended' && !carpool.settledAt) kinds.push('unsettled')
  if (isQuotaCar(carpool)) {
    const overCap = carpool.declaredTotalUsd > carpool.launchMaxRatio * carpool.weeklyLimitUsd + 1e-9
    if (overCap) kinds.push('overDeclared')
    // 必须落在区间内才提示「待确认」：超出上限时 Confirm 本来就会被后端拒，
    // 再催车主去点一个必定失败的按钮只会制造困惑。
    if (carpool.status === 'recruiting' && !overCap
      && carpool.declaredTotalUsd >= carpool.launchMinRatio * carpool.weeklyLimitUsd) {
      kinds.push('readyToConfirm')
    }
  }
  return kinds
}

const alertSummary = computed(() => {
  const counts = new Map<AlertKind, number>()
  for (const carpool of carpools.value) {
    for (const kind of carpoolAlerts(carpool)) counts.set(kind, (counts.get(kind) || 0) + 1)
  }
  return [...counts.entries()].map(([kind, count]) => ({ kind, count }))
})

const filteredCarpools = computed(() => carpools.value.filter((carpool) => {
  if (statusFilter.value && carpool.status !== statusFilter.value) return false
  if (alertFilter.value && !carpoolAlerts(carpool).includes(alertFilter.value as AlertKind)) return false
  const q = searchQuery.value.trim().toLowerCase()
  if (q && !carpool.name.toLowerCase().includes(q) && !carpool.organizer.toLowerCase().includes(q)) return false
  return true
}))

// 发车后成员已绑定订阅、可能已产生用量，改人要连带处理退补款——后端只放行
// recruiting/confirmed，这里保持一致，不渲染改不了的按钮。
function canManageMembers(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' || carpool.status === 'confirmed'
}

function canEdit(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' || carpool.status === 'confirmed'
}

function canTransfer(carpool: Carpool): boolean {
  return carpool.status !== 'cancelled' && carpool.status !== 'ended'
}

function canCancel(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' || carpool.status === 'confirmed' || carpool.status === 'starting'
}

const confirmTitle = computed(() => {
  if (!confirmAction.value) return ''
  return t(`carpool.adminPage.confirm.${confirmAction.value.kind}.title`)
})

const confirmMessage = computed(() => {
  if (!confirmAction.value) return ''
  return t(`carpool.adminPage.confirm.${confirmAction.value.kind}.message`,
    { name: confirmAction.value.carpool.name })
})

function errorMessage(error: unknown): string {
  return extractApiErrorMessage(error, t('carpool.actionFailed'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    carpools.value = await carpoolAPI.adminOverview()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadRoster(carpoolID: number): Promise<void> {
  rosterLoading.value = true
  rosterFailed.value = false
  roster.value = []
  try {
    roster.value = await carpoolAPI.roster(carpoolID)
  } catch {
    rosterFailed.value = true
  } finally {
    rosterLoading.value = false
  }
}

function revokeQrCode(): void {
  if (qrCodeUrl.value) {
    URL.revokeObjectURL(qrCodeUrl.value)
    qrCodeUrl.value = null
  }
}

// 群二维码只在弹窗打开时现取（它是私密车的入场券，不该随列表预加载）；
// 关掉弹窗立刻吊销 object URL。
async function loadQrCode(carpoolID: number): Promise<void> {
  qrLoading.value = true
  qrFailed.value = false
  try {
    const blob = await carpoolAPI.groupQrCode(carpoolID)
    revokeQrCode()
    qrCodeUrl.value = URL.createObjectURL(blob)
  } catch {
    qrFailed.value = true
  } finally {
    qrLoading.value = false
  }
}

function openMembers(carpool: Carpool): void {
  activeCarpool.value = carpool
  editingUserId.value = null
  membersOpen.value = true
  qrFailed.value = false
  if (carpool.hasGroupQrCode) void loadQrCode(carpool.id)
  void loadRoster(carpool.id)
}

function closeMembers(): void {
  membersOpen.value = false
  revokeQrCode()
}

function openEdit(carpool: Carpool): void {
  activeCarpool.value = carpool
  editForm.name = carpool.name
  editForm.description = carpool.description
  editForm.visibility = carpool.visibility
  editForm.scheduledStartAt = carpool.scheduledStartAt || ''
  editOpen.value = true
}

function openTransfer(carpool: Carpool): void {
  activeCarpool.value = carpool
  transferTargetId.value = null
  transferOpen.value = true
  void loadRoster(carpool.id)
}

function startEditQuota(userId: number, current: number): void {
  editingUserId.value = userId
  editingQuota.value = Math.round(current)
}

// autoUnconfirmed 要显式告诉管理员：他刚才的操作把车退回招募中了。
// 悄悄改状态是最容易让人以为「系统坏了」的那类行为。
function reportMemberChange(autoUnconfirmed: boolean, successKey: string): void {
  if (autoUnconfirmed) {
    appStore.showWarning(t('carpool.adminPage.autoUnconfirmed'))
  } else {
    appStore.showSuccess(t(successKey))
  }
}

async function saveQuota(userId: number): Promise<void> {
  const carpool = activeCarpool.value
  const quota = editingQuota.value
  if (!carpool || !quota || quota <= 0) return
  memberPending.value = true
  try {
    const { autoUnconfirmed } = await carpoolAPI.updateMemberQuota(carpool.id, userId, quota)
    editingUserId.value = null
    reportMemberChange(autoUnconfirmed, 'carpool.adminPage.quotaUpdated')
    await Promise.all([load(), loadRoster(carpool.id)])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    memberPending.value = false
  }
}

async function removeMember(userId: number): Promise<void> {
  const carpool = activeCarpool.value
  if (!carpool) return
  memberPending.value = true
  try {
    const { autoUnconfirmed } = await carpoolAPI.removeMember(carpool.id, userId)
    reportMemberChange(autoUnconfirmed, 'carpool.adminPage.memberRemoved')
    await Promise.all([load(), loadRoster(carpool.id)])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    memberPending.value = false
  }
}

async function saveEdit(): Promise<void> {
  const carpool = activeCarpool.value
  if (!carpool) return
  actionPending.value = true
  try {
    await carpoolAPI.updateCarpool(carpool.id, {
      name: editForm.name.trim(),
      description: editForm.description,
      visibility: editForm.visibility,
      scheduled_start_at: editForm.scheduledStartAt || undefined,
    })
    editOpen.value = false
    appStore.showSuccess(t('carpool.adminPage.updated'))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

async function saveTransfer(): Promise<void> {
  const carpool = activeCarpool.value
  if (!carpool || !transferTargetId.value) return
  actionPending.value = true
  try {
    await carpoolAPI.transferOwner(carpool.id, transferTargetId.value)
    transferOpen.value = false
    appStore.showSuccess(t('carpool.adminPage.ownerTransferred'))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

async function toggleLock(carpool: Carpool): Promise<void> {
  actionPending.value = true
  try {
    await carpoolAPI.setJoinLocked(carpool.id, !carpool.joinLocked)
    appStore.showSuccess(t(!carpool.joinLocked ? 'carpool.admin.locked' : 'carpool.admin.unlocked'))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

function run(kind: ConfirmKind, carpool: Carpool): void {
  confirmAction.value = { kind, carpool }
}

async function runConfirmed(): Promise<void> {
  const action = confirmAction.value
  if (!action) return
  actionPending.value = true
  try {
    if (action.kind === 'unconfirm') await carpoolAPI.unconfirm(action.carpool.id)
    else if (action.kind === 'launch') await carpoolAPI.launch(action.carpool.id)
    else await carpoolAPI.cancel(action.carpool.id)
    confirmAction.value = null
    appStore.showSuccess(t(`carpool.adminPage.confirm.${action.kind}.success`))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

onMounted(load)
</script>

<style scoped>
/* 表格行内操作按钮：与 UsersView 同一套「图标 + 文字」约定。 */
.action-btn {
  @apply flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-primary-400;
}
.action-btn-primary {
  @apply hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-900/20 dark:hover:text-primary-400;
}
.action-btn-danger {
  @apply hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400;
}
.btn-xs {
  @apply rounded border border-gray-300 px-2 py-1 text-xs text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700;
}
.btn-xs-primary {
  @apply border-primary-500 text-primary-600 hover:bg-primary-50 dark:border-primary-500 dark:text-primary-400 dark:hover:bg-primary-900/20;
}
.btn-xs-danger {
  @apply border-red-300 text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/20;
}
</style>
