<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="flex flex-col gap-3 rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex min-w-0 items-center gap-3">
              <Icon name="globe" size="lg" class="text-primary-600 dark:text-primary-400" />
              <div class="min-w-0">
                <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('accountPool.title') }}
                </h2>
                <p class="truncate text-sm text-gray-500 dark:text-dark-300">
                  {{ t('accountPool.description') }}
                </p>
              </div>
            </div>

            <button type="button" class="btn btn-primary" @click="openCreateDialog">
              <Icon name="plus" size="sm" />
              <span>{{ t('accountPool.addAccount') }}</span>
            </button>
          </div>
        </div>
      </template>

      <template #filters>
        <div class="flex flex-col justify-between gap-3 lg:flex-row lg:items-center">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-80">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500" />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('accountPool.searchPlaceholder')"
                class="input pl-10"
                @keyup.enter="reloadFirstPage"
              />
            </div>
            <select v-model="platformFilter" class="input w-full sm:w-44" @change="reloadFirstPage">
              <option value="">{{ t('common.all') }}</option>
              <option v-for="platform in platforms" :key="platform" :value="platform">
                {{ platformLabel(platform) }}
              </option>
            </select>
          </div>

          <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('accountPool.refresh')" @click="loadAccounts">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>{{ t('accountPool.columns.account') }}</th>
                <th>{{ t('accountPool.columns.platform') }}</th>
                <th>{{ t('accountPool.columns.owner') }}</th>
                <th>{{ t('accountPool.columns.status') }}</th>
                <th>{{ t('accountPool.columns.scheduling') }}</th>
                <th>{{ t('accountPool.columns.fiveHour') }}</th>
                <th>{{ t('accountPool.columns.weekly') }}</th>
                <th>{{ t('accountPool.columns.limits') }}</th>
                <th>{{ t('accountPool.columns.updated') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && accounts.length === 0">
                <td colspan="9" class="text-center text-gray-500 dark:text-dark-300">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="accounts.length === 0">
                <td colspan="9" class="text-center text-gray-500 dark:text-dark-300">
                  {{ t('accountPool.empty') }}
                </td>
              </tr>
              <tr v-for="account in accounts" :key="account.id">
                <td>
                  <div class="min-w-56">
                    <div class="font-medium text-gray-900 dark:text-white">{{ account.name }}</div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">#{{ account.id }}</div>
                  </div>
                </td>
                <td>
                  <span class="badge badge-gray">{{ platformLabel(account.platform) }}</span>
                </td>
                <td>
                  <span :class="account.is_mine ? 'badge badge-primary' : 'badge badge-gray'">
                    {{ ownerLabel(account) }}
                  </span>
                </td>
                <td>
                  <div class="flex flex-col gap-1">
                    <span :class="statusClass(account.status)">{{ statusLabel(account.status) }}</span>
                    <span :class="account.effective_schedulable ? 'text-xs text-green-600 dark:text-green-400' : 'text-xs text-amber-600 dark:text-amber-400'">
                      {{ account.effective_schedulable ? t('accountPool.statuses.effective') : t('accountPool.statuses.blocked') }}
                    </span>
                  </div>
                </td>
                <td>
                  <label v-if="account.is_mine" class="inline-flex items-center gap-2">
                    <input
                      :checked="account.schedulable"
                      type="checkbox"
                      class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                      :disabled="savingIDs.has(account.id)"
                      @change="toggleSchedulable(account, ($event.target as HTMLInputElement).checked)"
                    />
                    <span class="text-sm text-gray-700 dark:text-dark-200">{{ t('accountPool.fields.schedulable') }}</span>
                  </label>
                  <span v-else class="text-sm text-gray-500 dark:text-dark-400">
                    {{ account.schedulable ? t('common.enabled') : t('common.disabled') }}
                  </span>
                </td>
                <td>
                  <div class="min-w-32 text-sm">
                    <div>{{ formatMoney(account.current_window_cost ?? 0) }} / {{ formatMoney(account.window_cost_limit) }}</div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('accountPool.fields.windowCostReserve') }} {{ formatMoney(account.window_cost_sticky_reserve) }}
                    </div>
                  </div>
                </td>
                <td>
                  <div class="min-w-36 text-sm">
                    <div>{{ formatMoney(account.quota_weekly_used) }} / {{ formatMoney(account.quota_weekly_limit) }}</div>
                    <div :class="account.weekly_remaining_below_policy ? 'text-xs text-amber-600 dark:text-amber-400' : 'text-xs text-gray-500 dark:text-dark-400'">
                      {{ formatMoney(account.quota_weekly_remaining) }} / {{ formatMoney(account.quota_weekly_min_remaining) }}
                    </div>
                  </div>
                </td>
                <td>
                  <div v-if="account.is_mine" class="grid min-w-[430px] grid-cols-4 gap-2">
                    <input v-model.number="draftFor(account).windowCostLimit" type="number" min="0" step="0.01" class="input h-9 text-sm" :placeholder="t('accountPool.fields.windowCostLimit')" />
                    <input v-model.number="draftFor(account).windowCostReserve" type="number" min="0" step="0.01" class="input h-9 text-sm" :placeholder="t('accountPool.fields.windowCostReserve')" />
                    <input v-model.number="draftFor(account).quotaWeeklyLimit" type="number" min="0" step="0.01" class="input h-9 text-sm" :placeholder="t('accountPool.fields.quotaWeeklyLimit')" />
                    <div class="flex gap-2">
                      <input v-model.number="draftFor(account).quotaWeeklyMinRemaining" type="number" min="0" step="0.01" class="input h-9 text-sm" :placeholder="t('accountPool.fields.quotaWeeklyMinRemaining')" />
                      <button type="button" class="btn btn-primary btn-sm h-9" :disabled="savingIDs.has(account.id)" @click="saveLimits(account)">
                        {{ savingIDs.has(account.id) ? t('accountPool.saving') : t('accountPool.save') }}
                      </button>
                    </div>
                  </div>
                  <span v-else class="text-sm text-gray-500 dark:text-dark-400">{{ t('accountPool.readonly') }}</span>
                </td>
                <td class="whitespace-nowrap text-sm text-gray-500 dark:text-dark-400">
                  {{ formatDate(account.updated_at) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template #pagination>
        <Pagination
          :total="pagination.total"
          :page="pagination.page"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="showCreateForm"
      :title="t('accountPool.addAccount')"
      width="wide"
      @close="handleCreateClose"
    >
      <div class="mb-6 flex items-center justify-center">
        <div class="flex items-center space-x-4">
          <div class="flex items-center">
            <div
              :class="[
                'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
                createStep >= 1 ? 'bg-primary-500 text-white' : 'bg-gray-200 text-gray-500 dark:bg-dark-600'
              ]"
            >
              1
            </div>
            <span class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.oauth.authMethod') }}
            </span>
          </div>
          <div class="h-0.5 w-8 bg-gray-300 dark:bg-dark-600" />
          <div class="flex items-center">
            <div
              :class="[
                'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
                createStep >= 2 ? 'bg-primary-500 text-white' : 'bg-gray-200 text-gray-500 dark:bg-dark-600'
              ]"
            >
              2
            </div>
            <span class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.oauth.completeAuth') }}
            </span>
          </div>
        </div>
      </div>

      <form
        v-if="createStep === 1"
        id="user-account-pool-create-form"
        class="space-y-5"
        @submit.prevent="goToOAuthStep"
      >
        <div>
          <label class="input-label">{{ t('admin.accounts.accountName') }}</label>
          <input
            v-model.trim="createForm.name"
            type="text"
            required
            class="input"
            :placeholder="t('admin.accounts.enterAccountName')"
          />
        </div>

        <div>
          <label class="input-label">{{ t('admin.accounts.platform') }}</label>
          <div class="mt-2 flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
            <button
              type="button"
              @click="selectCreatePlatform('openai')"
              :class="platformButtonClass('openai', 'green')"
            >
              <Icon name="key" size="sm" />
              OpenAI
            </button>
            <button
              type="button"
              @click="selectCreatePlatform('gemini')"
              :class="platformButtonClass('gemini', 'blue')"
            >
              <Icon name="sparkles" size="sm" />
              Gemini
            </button>
            <button
              type="button"
              @click="selectCreatePlatform('antigravity')"
              :class="platformButtonClass('antigravity', 'purple')"
            >
              <Icon name="cloud" size="sm" />
              Antigravity
            </button>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
          <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <button
              type="button"
              class="flex items-center gap-3 rounded-lg border-2 border-primary-500 bg-primary-50 p-3 text-left transition-all dark:bg-primary-900/20"
            >
              <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-500 text-white">
                <Icon name="key" size="sm" />
              </div>
              <div>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">OAuth</span>
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.oauthSetupToken') }}</span>
              </div>
            </button>
          </div>
        </div>

        <div v-if="createForm.platform === 'gemini'" class="space-y-3">
          <label class="input-label">{{ t('admin.accounts.gemini.oauthType') }}</label>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <button
              v-for="option in geminiOAuthOptions"
              :key="option.value"
              type="button"
              @click="selectGeminiOAuthType(option.value)"
              :class="[
                'rounded-lg border-2 p-3 text-left transition-all',
                createForm.oauthType === option.value
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                  : 'border-gray-200 hover:border-blue-300 dark:border-dark-600 dark:hover:border-blue-700'
              ]"
            >
              <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ option.label }}</span>
            </button>
          </div>
          <input
            v-model.trim="createForm.tierId"
            class="input"
            :placeholder="t('accountPool.oauth.tierIdPlaceholder')"
          />
        </div>

        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="mb-3 flex items-start gap-3">
            <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-500 text-white">
              <Icon name="shield" size="sm" />
            </div>
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('accountPool.policy.title') }}
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">
                {{ t('accountPool.policy.description') }}
              </p>
            </div>
          </div>

          <div class="grid gap-4 lg:grid-cols-3">
            <label class="block">
              <span class="input-label">{{ t('accountPool.policy.windowLimitLabel') }}</span>
              <input
                v-model.number="createForm.windowCostLimit"
                type="number"
                min="0"
                step="0.01"
                class="input"
                :placeholder="t('accountPool.policy.noLimitPlaceholder')"
              />
              <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                {{ t('accountPool.policy.windowLimitHint') }}
              </span>
            </label>
            <label class="block">
              <span class="input-label">{{ t('accountPool.policy.weeklyLimitLabel') }}</span>
              <input
                v-model.number="createForm.quotaWeeklyLimit"
                type="number"
                min="0"
                step="0.01"
                class="input"
                :placeholder="t('accountPool.policy.noLimitPlaceholder')"
              />
              <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                {{ t('accountPool.policy.weeklyLimitHint') }}
              </span>
            </label>
            <label class="block">
              <span class="input-label">{{ t('accountPool.policy.weeklyReserveLabel') }}</span>
              <input
                v-model.number="createForm.quotaWeeklyMinRemaining"
                type="number"
                min="0"
                step="0.01"
                class="input"
                :placeholder="t('accountPool.policy.noReservePlaceholder')"
              />
              <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                {{ t('accountPool.policy.weeklyReserveHint') }}
              </span>
            </label>
          </div>
        </div>
      </form>

      <div v-else class="space-y-5">
        <OAuthAuthorizationFlow
          ref="oauthFlowRef"
          add-method="oauth"
          :auth-url="oauthSession.authUrl"
          :session-id="oauthSession.sessionId"
          :loading="oauthLoading || creating"
          :error="oauthError"
          :show-help="false"
          :show-proxy-warning="false"
          :allow-multiple="false"
          :show-cookie-option="false"
          :show-refresh-token-option="false"
          :show-mobile-refresh-token-option="false"
          :show-session-token-option="false"
          :show-access-token-option="false"
          :show-codex-session-import-option="false"
          :platform="createForm.platform"
          :show-project-id="createForm.platform === 'gemini' && createForm.oauthType === 'code_assist'"
          @generate-url="handleGenerateUrl"
        />
      </div>

      <template #footer>
        <div v-if="createStep === 1" class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="handleCreateClose">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="user-account-pool-create-form"
            class="btn btn-primary"
          >
            {{ t('common.next') }}
          </button>
        </div>
        <div v-else class="flex justify-between gap-3">
          <button type="button" class="btn btn-secondary" @click="goBackToBasicInfo">
            {{ t('common.back') }}
          </button>
          <button
            type="button"
            :disabled="!canExchangeCode"
            class="btn btn-primary"
            @click="handleExchangeCode"
          >
            <svg
              v-if="oauthLoading || creating"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ oauthLoading || creating ? t('admin.accounts.oauth.verifying') : t('admin.accounts.oauth.completeAuth') }}
          </button>
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
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import OAuthAuthorizationFlow from '@/components/account/OAuthAuthorizationFlow.vue'
import accountsAPI, { type UserOAuthTokenInfo } from '@/api/accounts'
import type { AccountPlatform, UserAccountPoolItem } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  reset: () => void
}

type LimitDraft = {
  windowCostLimit: number
  windowCostReserve: number
  quotaWeeklyLimit: number
  quotaWeeklyMinRemaining: number
}

const { t } = useI18n()
const appStore = useAppStore()

const platforms: AccountPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity']
const accounts = ref<UserAccountPoolItem[]>([])
const loading = ref(false)
const creating = ref(false)
const oauthLoading = ref(false)
const oauthError = ref('')
const showCreateForm = ref(false)
const createStep = ref(1)
const searchQuery = ref('')
const platformFilter = ref<AccountPlatform | ''>('')
const savingIDs = ref(new Set<number>())
const drafts = reactive<Record<number, LimitDraft>>({})
const oauthFlowRef = ref<OAuthFlowExposed | null>(null)
const oauthSession = reactive({
  authUrl: '',
  sessionId: '',
  state: '',
})

const pagination = reactive({
  page: 1,
  page_size: 50,
  total: 0,
})

const createForm = reactive({
  name: '',
  platform: 'openai' as AccountPlatform,
  oauthType: 'code_assist' as 'code_assist' | 'google_one' | 'ai_studio',
  projectId: '',
  tierId: '',
  schedulable: true,
  windowCostLimit: 0,
  windowCostReserve: 10,
  quotaWeeklyLimit: 0,
  quotaWeeklyMinRemaining: 0,
})

const geminiOAuthOptions = [
  { value: 'code_assist' as const, label: 'Code Assist' },
  { value: 'google_one' as const, label: 'Google One' },
  { value: 'ai_studio' as const, label: 'AI Studio' },
]

const canExchangeCode = computed(() => {
  const authCode = oauthFlowRef.value?.authCode || ''
  return Boolean(authCode.trim() && oauthSession.sessionId && !oauthLoading.value && !creating.value)
})

function platformLabel(platform: AccountPlatform): string {
  return t(`accountPool.platforms.${platform}`)
}

function statusLabel(status: UserAccountPoolItem['status']): string {
  return t(`accountPool.statuses.${status}`)
}

function statusClass(status: UserAccountPoolItem['status']): string {
  if (status === 'active') return 'badge badge-success'
  if (status === 'error') return 'badge badge-danger'
  return 'badge badge-gray'
}

function ownerLabel(account: UserAccountPoolItem): string {
  if (account.is_mine) return t('accountPool.mine')
  return account.is_user_contributed ? t('accountPool.shared') : t('accountPool.system')
}

function formatMoney(value: number | null | undefined): string {
  const n = Number(value ?? 0)
  if (!Number.isFinite(n) || n <= 0) return '$0.00'
  return `$${n.toFixed(n >= 100 ? 0 : 2)}`
}

function formatDate(value: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

function toLimit(value: unknown): number {
  const n = Number(value)
  return Number.isFinite(n) && n > 0 ? n : 0
}

function resetOAuthSession() {
  oauthSession.authUrl = ''
  oauthSession.sessionId = ''
  oauthSession.state = ''
  oauthError.value = ''
}

function compactRecord(input: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(input).filter(([, value]) => value !== undefined && value !== null && value !== '')
  )
}

function buildOpenAICredentials(tokenInfo: UserOAuthTokenInfo): Record<string, unknown> {
  return compactRecord({
    access_token: tokenInfo.access_token,
    refresh_token: tokenInfo.refresh_token,
    id_token: tokenInfo.id_token,
    expires_at: tokenInfo.expires_at,
    email: tokenInfo.email,
    chatgpt_account_id: tokenInfo.chatgpt_account_id,
    chatgpt_user_id: tokenInfo.chatgpt_user_id,
    organization_id: tokenInfo.organization_id,
    plan_type: tokenInfo.plan_type,
    client_id: tokenInfo.client_id,
  })
}

function buildGeminiCredentials(tokenInfo: UserOAuthTokenInfo): Record<string, unknown> {
  return compactRecord({
    access_token: tokenInfo.access_token,
    refresh_token: tokenInfo.refresh_token,
    token_type: tokenInfo.token_type,
    expires_at: tokenInfo.expires_at,
    scope: tokenInfo.scope,
    project_id: tokenInfo.project_id,
    oauth_type: tokenInfo.oauth_type,
    tier_id: tokenInfo.tier_id,
  })
}

function buildAntigravityCredentials(tokenInfo: UserOAuthTokenInfo): Record<string, unknown> {
  return compactRecord({
    access_token: tokenInfo.access_token,
    refresh_token: tokenInfo.refresh_token,
    token_type: tokenInfo.token_type,
    expires_at: tokenInfo.expires_at,
    project_id: tokenInfo.project_id,
    email: tokenInfo.email,
  })
}

function buildCredentials(tokenInfo: UserOAuthTokenInfo): Record<string, unknown> {
  if (createForm.platform === 'openai') return buildOpenAICredentials(tokenInfo)
  if (createForm.platform === 'gemini') return buildGeminiCredentials(tokenInfo)
  if (createForm.platform === 'antigravity') return buildAntigravityCredentials(tokenInfo)
  return {}
}

function buildExtra(tokenInfo: UserOAuthTokenInfo): Record<string, unknown> | undefined {
  const extra =
    createForm.platform === 'gemini' && tokenInfo.extra && typeof tokenInfo.extra === 'object'
      ? { ...(tokenInfo.extra as Record<string, unknown>) }
      : compactRecord({
          email: tokenInfo.email,
          name: tokenInfo.name,
          privacy_mode: tokenInfo.privacy_mode,
        })
  return Object.keys(extra).length > 0 ? extra : undefined
}

function openCreateDialog() {
  showCreateForm.value = true
  createStep.value = 1
}

function handleCreateClose() {
  showCreateForm.value = false
  createStep.value = 1
  resetOAuthSession()
  oauthFlowRef.value?.reset()
}

function goToOAuthStep() {
  createStep.value = 2
  resetOAuthSession()
  oauthFlowRef.value?.reset()
}

function goBackToBasicInfo() {
  createStep.value = 1
  resetOAuthSession()
  oauthFlowRef.value?.reset()
}

function selectCreatePlatform(platform: 'openai' | 'gemini' | 'antigravity') {
  createForm.platform = platform
  resetOAuthSession()
  oauthFlowRef.value?.reset()
}

function selectGeminiOAuthType(oauthType: 'code_assist' | 'google_one' | 'ai_studio') {
  createForm.oauthType = oauthType
  resetOAuthSession()
  oauthFlowRef.value?.reset()
}

function platformButtonClass(platform: 'openai' | 'gemini' | 'antigravity', color: 'green' | 'blue' | 'purple') {
  const active = createForm.platform === platform
  const activeClass = {
    green: 'bg-white text-green-600 shadow-sm dark:bg-dark-600 dark:text-green-400',
    blue: 'bg-white text-blue-600 shadow-sm dark:bg-dark-600 dark:text-blue-400',
    purple: 'bg-white text-purple-600 shadow-sm dark:bg-dark-600 dark:text-purple-400',
  }[color]
  return [
    'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
    active ? activeClass : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200',
  ]
}

async function handleGenerateUrl() {
  oauthLoading.value = true
  oauthError.value = ''
  try {
    const projectId = oauthFlowRef.value?.projectId || createForm.projectId
    const result = await accountsAPI.generateOAuthAuthUrl({
      platform: createForm.platform,
      redirect_uri: `${window.location.origin}/auth/callback`,
      project_id: projectId || undefined,
      oauth_type: createForm.platform === 'gemini' ? createForm.oauthType : undefined,
      tier_id: createForm.tierId || undefined,
    })
    oauthSession.authUrl = result.auth_url
    oauthSession.sessionId = result.session_id
    oauthSession.state = result.state || ''
  } catch (err: unknown) {
    oauthError.value = extractApiErrorMessage(err, t('accountPool.oauth.startFailed'))
    appStore.showError(oauthError.value)
  } finally {
    oauthLoading.value = false
  }
}

function draftFor(account: UserAccountPoolItem): LimitDraft {
  if (!drafts[account.id]) {
    drafts[account.id] = {
      windowCostLimit: account.window_cost_limit ?? 0,
      windowCostReserve: account.window_cost_sticky_reserve || 10,
      quotaWeeklyLimit: account.quota_weekly_limit ?? 0,
      quotaWeeklyMinRemaining: account.quota_weekly_min_remaining ?? 0,
    }
  }
  return drafts[account.id]
}

function replaceAccount(updated: UserAccountPoolItem) {
  const index = accounts.value.findIndex((item) => item.id === updated.id)
  if (index >= 0) {
    accounts.value[index] = updated
  }
  drafts[updated.id] = {
    windowCostLimit: updated.window_cost_limit ?? 0,
    windowCostReserve: updated.window_cost_sticky_reserve || 10,
    quotaWeeklyLimit: updated.quota_weekly_limit ?? 0,
    quotaWeeklyMinRemaining: updated.quota_weekly_min_remaining ?? 0,
  }
}

async function loadAccounts() {
  loading.value = true
  try {
    const result = await accountsAPI.listPool(pagination.page, pagination.page_size, {
      platform: platformFilter.value,
      search: searchQuery.value.trim(),
      sort_by: 'name',
      sort_order: 'asc',
    })
    accounts.value = result.items
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
    for (const account of result.items) {
      draftFor(account)
    }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.loadFailed')))
  } finally {
    loading.value = false
  }
}

function reloadFirstPage() {
  pagination.page = 1
  loadAccounts()
}

async function handleExchangeCode() {
  if (!oauthSession.sessionId) {
    appStore.showError(t('accountPool.oauth.sessionRequired'))
    return
  }
  const authCode = oauthFlowRef.value?.authCode || ''
  if (!authCode.trim()) {
    appStore.showError(t('accountPool.oauth.codeRequired'))
    return
  }

  creating.value = true
  oauthError.value = ''
  try {
    const projectId = oauthFlowRef.value?.projectId || createForm.projectId
    const state = oauthFlowRef.value?.oauthState || oauthSession.state
    const tokenInfo = await accountsAPI.exchangeOAuthCode({
      platform: createForm.platform,
      session_id: oauthSession.sessionId,
      code: authCode.trim(),
      state,
      project_id: projectId || undefined,
      oauth_type: createForm.platform === 'gemini' ? createForm.oauthType : undefined,
      tier_id: createForm.tierId || undefined,
    })
    const credentials = buildCredentials(tokenInfo)
    if (Object.keys(credentials).length === 0) {
      appStore.showError(t('accountPool.oauth.credentialsMissing'))
      return
    }

    await accountsAPI.createOAuth({
      name: createForm.name,
      platform: createForm.platform,
      type: 'oauth',
      credentials,
      extra: buildExtra(tokenInfo),
      schedulable: createForm.schedulable,
      window_cost_limit: toLimit(createForm.windowCostLimit),
      window_cost_sticky_reserve: toLimit(createForm.windowCostReserve),
      quota_weekly_limit: toLimit(createForm.quotaWeeklyLimit),
      quota_weekly_min_remaining: toLimit(createForm.quotaWeeklyMinRemaining),
    })
    appStore.showSuccess(t('accountPool.createSuccess'))
    createForm.name = ''
    handleCreateClose()
    reloadFirstPage()
  } catch (err: unknown) {
    oauthError.value = extractApiErrorMessage(err, t('common.error'))
    appStore.showError(oauthError.value)
  } finally {
    creating.value = false
  }
}

async function toggleSchedulable(account: UserAccountPoolItem, schedulable: boolean) {
  savingIDs.value.add(account.id)
  try {
    const updated = await accountsAPI.setSchedulable(account.id, schedulable)
    replaceAccount(updated)
    appStore.showSuccess(t('accountPool.schedulableSaved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    savingIDs.value.delete(account.id)
  }
}

async function saveLimits(account: UserAccountPoolItem) {
  const draft = draftFor(account)
  savingIDs.value.add(account.id)
  try {
    const updated = await accountsAPI.updateLimits(account.id, {
      window_cost_limit: toLimit(draft.windowCostLimit),
      window_cost_sticky_reserve: toLimit(draft.windowCostReserve),
      quota_weekly_limit: toLimit(draft.quotaWeeklyLimit),
      quota_weekly_min_remaining: toLimit(draft.quotaWeeklyMinRemaining),
    })
    replaceAccount(updated)
    appStore.showSuccess(t('accountPool.saveSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    savingIDs.value.delete(account.id)
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadAccounts()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadAccounts()
}

onMounted(loadAccounts)
</script>
