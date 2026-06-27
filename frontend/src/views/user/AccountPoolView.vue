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

          <div class="grid gap-3 md:grid-cols-[repeat(3,minmax(0,1fr))_auto] md:items-center">
            <div class="rounded-md border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40">
              <div class="text-xs text-gray-500 dark:text-dark-300">{{ t('accountPool.rewards.available') }}</div>
              <div class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ formatMoney(contributionSummary.contribution_quota) }}</div>
            </div>
            <div class="rounded-md border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40">
              <div class="text-xs text-gray-500 dark:text-dark-300">{{ t('accountPool.rewards.frozen') }}</div>
              <div class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ formatMoney(contributionSummary.contribution_frozen_quota) }}</div>
            </div>
            <div class="rounded-md border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40">
              <div class="text-xs text-gray-500 dark:text-dark-300">{{ t('accountPool.rewards.history') }}</div>
              <div class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ formatMoney(contributionSummary.contribution_history_quota) }}</div>
            </div>
            <button
              type="button"
              class="btn btn-secondary md:justify-self-end"
              :disabled="transferringContribution || contributionSummary.contribution_quota <= 0"
              @click="transferContribution"
            >
              <Icon name="arrowRight" size="sm" />
              <span>{{ transferringContribution ? t('accountPool.rewards.transferring') : t('accountPool.rewards.transfer') }}</span>
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
            <select v-model="planTypeFilter" class="input w-full sm:w-40" @change="reloadFirstPage">
              <option value="">{{ t('accountPool.planTypes.all') }}</option>
              <option value="plus">{{ t('accountPool.planTypes.plus') }}</option>
              <option value="pro">{{ t('accountPool.planTypes.pro') }}</option>
              <option value="prolite">{{ t('accountPool.planTypes.prolite') }}</option>
            </select>
          </div>

          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('accountPool.refresh')" @click="loadAccounts">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <div ref="columnsDropdownRef" class="relative">
              <button
                type="button"
                class="btn btn-secondary px-2 md:px-3"
                :title="t('accountPool.moreActions')"
                @click="showColumnsDropdown = !showColumnsDropdown"
              >
                <Icon name="more" size="sm" class="md:mr-1.5" />
                <span class="hidden md:inline">{{ t('accountPool.moreActions') }}</span>
                <Icon name="chevronDown" size="xs" class="ml-1 hidden md:inline" />
              </button>
              <div
                v-if="showColumnsDropdown"
                class="absolute right-0 z-50 mt-2 w-[min(20rem,calc(100vw-2rem))] origin-top-right overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
              >
                <div class="max-h-[70vh] overflow-y-auto p-2">
                  <div class="px-2 py-2">
                    <div class="flex items-center justify-between gap-3">
                      <span class="text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
                        {{ t('admin.accounts.viewColumns') }}
                      </span>
                      <Icon name="grid" size="sm" class="text-gray-400" />
                    </div>
                  </div>
                  <button
                    v-for="col in toggleableColumns"
                    :key="col.key"
                    type="button"
                    class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
                    @click="toggleColumn(col.key)"
                  >
                    <span class="truncate">{{ col.label }}</span>
                    <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" class="text-primary-500" />
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <template #table>
        <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <DataTable
            :columns="visibleColumns"
            :data="accounts"
            :loading="loading"
            row-key="id"
            :sticky-first-column="true"
            :sticky-actions-column="true"
            :estimate-row-height="96"
          >
            <template #empty>
              <div class="flex flex-col items-center">
                <Icon name="inbox" size="xl" class="mb-4 h-12 w-12 text-gray-400 dark:text-dark-500" />
                <p class="text-lg font-medium text-gray-900 dark:text-gray-100">{{ t('accountPool.empty') }}</p>
              </div>
            </template>

            <template #cell-account="{ row: account }">
              <div class="min-w-56">
                <div class="font-medium text-gray-900 dark:text-white">{{ account.name }}</div>
                <div class="text-xs text-gray-500 dark:text-dark-400">#{{ account.id }}</div>
                <div v-if="account.is_mine" class="mt-1 flex flex-wrap gap-1">
                  <span
                    v-for="group in account.groups || []"
                    :key="group.id"
                    class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                  >
                    {{ group.name }}
                  </span>
                  <span
                    v-if="account.model_mapping && Object.keys(account.model_mapping).length > 0"
                    class="inline-flex items-center rounded bg-blue-50 px-1.5 py-0.5 text-[11px] text-blue-700 dark:bg-blue-900/20 dark:text-blue-300"
                  >
                    {{ t('accountPool.settings.modelLimited') }}
                  </span>
                  <span
                    v-if="account.codex_cli_only"
                    class="inline-flex items-center rounded bg-green-50 px-1.5 py-0.5 text-[11px] text-green-700 dark:bg-green-900/20 dark:text-green-300"
                  >
                    Codex
                  </span>
                </div>
              </div>
            </template>

            <template #cell-account_type="{ row: account }">
              <PlatformTypeBadge
                :platform="account.platform"
                :type="account.type"
                :plan-type="account.plan_type || undefined"
                :privacy-mode="account.privacy_mode || undefined"
                :subscription-expires-at="account.subscription_expires_at || undefined"
              />
            </template>

            <template #cell-owner="{ row: account }">
              <span :class="account.is_mine ? 'badge badge-primary' : 'badge badge-gray'">
                {{ ownerLabel(account) }}
              </span>
            </template>

            <template #cell-status="{ row: account }">
              <div class="flex flex-col gap-1">
                <span :class="statusClass(account.status)">{{ statusLabel(account.status) }}</span>
                <span :class="account.effective_schedulable ? 'text-xs text-green-600 dark:text-green-400' : 'text-xs text-amber-600 dark:text-amber-400'">
                  {{ account.effective_schedulable ? t('accountPool.statuses.effective') : t('accountPool.statuses.blocked') }}
                </span>
              </div>
            </template>

            <template #cell-scheduling="{ row: account }">
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
            </template>

            <template #cell-concurrency="{ row: account }">
              <span v-if="account.is_mine" class="font-mono text-sm font-medium text-gray-900 dark:text-gray-100">
                {{ account.concurrency || DEFAULT_USER_ACCOUNT_CONCURRENCY }}
              </span>
              <span v-else class="text-sm text-gray-500 dark:text-dark-400">{{ t('accountPool.readonly') }}</span>
            </template>

            <template #cell-five_hour="{ row: account }">
              <div class="min-w-36 space-y-1 text-sm">
                <UsageProgressBar
                  label="5h"
                  :utilization="account.contribution_5h_usage_percent || 0"
                  :resets-at="account.five_hour_resets_at"
                  color="indigo"
                />
                <div v-if="account.window_cost_limit > 0" class="text-xs text-gray-500 dark:text-dark-400">
                  {{ formatMoney(account.current_window_cost || 0) }} / {{ formatMoney(account.window_cost_limit) }}
                </div>
                <div class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('accountPool.fields.reserve') }} {{ formatPercent(account.contribution_5h_reserve_percent) }}
                </div>
              </div>
            </template>

            <template #cell-weekly="{ row: account }">
              <div class="min-w-44 space-y-1 text-sm">
                <UsageProgressBar
                  label="7d"
                  :utilization="account.contribution_weekly_usage_percent || 0"
                  :resets-at="account.weekly_resets_at"
                  color="emerald"
                />
                <!-- Budget mode: show others' weekly spend against the shared budget -->
                <template v-if="account.contribution_share_mode === 'budget'">
                  <div
                    class="text-xs"
                    :class="isShareBudgetExhausted(account) ? 'text-amber-600 dark:text-amber-400' : 'text-gray-500 dark:text-dark-400'"
                  >
                    {{ t('accountPool.policy.othersUsed') }} {{ formatMoney(account.others_weekly_spend ?? 0) }}
                    / {{ t('accountPool.policy.budget') }} {{ formatMoney(account.contribution_weekly_share_budget) }}
                  </div>
                  <span v-if="isShareBudgetExhausted(account)" class="badge badge-warning">
                    {{ t('accountPool.policy.budgetExhausted') }}
                  </span>
                </template>
                <!-- Percent mode: existing cost/remaining lines + reserve % note -->
                <template v-else>
                  <div v-if="account.quota_weekly_limit > 0" class="text-xs text-gray-500 dark:text-dark-400">
                    {{ formatMoney(account.quota_weekly_used) }} / {{ formatMoney(account.quota_weekly_limit) }}
                  </div>
                  <div v-if="account.quota_weekly_limit > 0" :class="account.weekly_remaining_below_policy ? 'text-xs text-amber-600 dark:text-amber-400' : 'text-xs text-gray-500 dark:text-dark-400'">
                    {{ t('accountPool.fields.remaining') }} {{ formatMoney(account.quota_weekly_remaining) }}
                    <span v-if="account.quota_weekly_min_remaining > 0">
                      · {{ t('accountPool.fields.reserve') }} {{ formatMoney(account.quota_weekly_min_remaining) }}
                    </span>
                  </div>
                  <div class="text-xs text-gray-500 dark:text-dark-400">
                    {{ t('accountPool.fields.reserve') }} {{ formatPercent(account.contribution_weekly_reserve_percent) }}
                  </div>
                </template>
              </div>
            </template>

            <template #cell-protection="{ row: account }">
              <div class="min-w-48 space-y-1 text-sm">
                <span :class="account.contribution_protection_blocked || account.weekly_remaining_below_policy ? 'badge badge-warning' : 'badge badge-success'">
                  {{ account.contribution_protection_blocked || account.weekly_remaining_below_policy ? t('accountPool.policy.blocked') : t('accountPool.policy.sharing') }}
                </span>
                <div class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('accountPool.policy.probeFailureLabel') }}: {{ probeFailurePolicyLabel(account.contribution_probe_failure_policy) }}
                </div>
                <div v-if="account.weekly_remaining_below_policy" class="text-xs text-amber-600 dark:text-amber-400">
                  {{ t('accountPool.policy.weeklyRemainingBlocked') }}
                </div>
                <div v-else-if="account.contribution_protection_blocked" class="text-xs text-amber-600 dark:text-amber-400">
                  {{ t('accountPool.policy.protectionBlocked') }}
                </div>
              </div>
            </template>

            <template #cell-availability="{ row: account }">
              <div v-if="account.is_mine" class="min-w-64 space-y-1.5 text-sm">
                <div class="flex flex-wrap gap-1">
                  <span
                    v-if="isModelLimited(account)"
                    class="inline-flex items-center rounded bg-blue-50 px-1.5 py-0.5 text-[11px] text-blue-700 dark:bg-blue-900/20 dark:text-blue-300"
                  >
                    {{ t('accountPool.settings.modelCount', { count: modelLimitCount(account) }) }}
                  </span>
                  <span
                    v-else
                    class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                  >
                    {{ t('accountPool.settings.allModels') }}
                  </span>
                  <span
                    v-if="account.codex_cli_only"
                    class="inline-flex items-center rounded bg-green-50 px-1.5 py-0.5 text-[11px] text-green-700 dark:bg-green-900/20 dark:text-green-300"
                  >
                    {{ t('accountPool.settings.codexOnly') }}
                  </span>
                </div>
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="group in account.groups || []"
                    :key="group.id"
                    class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                  >
                    {{ group.name }}
                  </span>
                  <span
                    v-if="!account.groups || account.groups.length === 0"
                    class="text-xs text-gray-500 dark:text-dark-400"
                  >
                    {{ t('accountPool.settings.noGroups') }}
                  </span>
                </div>
              </div>
              <span v-else class="text-sm text-gray-500 dark:text-dark-400">{{ t('accountPool.readonly') }}</span>
            </template>

            <template #cell-expires_at="{ row: account }">
              <div class="min-w-40 text-sm">
                <div>{{ formatOptionalUnixDate(account.expires_at) }}</div>
                <div v-if="account.auto_pause_on_expired && account.expires_at" class="text-xs text-green-600 dark:text-green-400">
                  {{ t('accountPool.settings.autoPauseEnabled') }}
                </div>
                <div v-else class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('accountPool.settings.noExpiry') }}
                </div>
              </div>
            </template>

            <template #cell-actions="{ row: account }">
              <!-- Remote browser (Kasm) actions for Pro contributed accounts. -->
              <div v-if="isProAccount(account)" class="mb-1 flex flex-wrap items-center gap-1">
                <button
                  v-if="account.is_mine && !isRemoteSeedReady(account)"
                  type="button"
                  class="btn btn-secondary btn-xs"
                  :disabled="remoteState(account.id).busy"
                  @click="setupRemoteLogin(account)"
                >
                  {{ t('accountPool.remote.setup') }}
                </button>
                <template v-else-if="isRemoteSeedReady(account)">
                  <template v-if="remoteState(account.id).queued">
                    <span class="text-xs text-amber-600 dark:text-amber-400">
                      {{ t('accountPool.remote.queued', { position: remoteState(account.id).position ?? '—' }) }}
                    </span>
                    <button type="button" class="btn btn-secondary btn-xs" @click="cancelRemoteQueue(account)">
                      {{ t('accountPool.remote.cancelQueue') }}
                    </button>
                  </template>
                  <template v-else>
                    <button
                      type="button"
                      class="btn btn-primary btn-xs"
                      :disabled="remoteState(account.id).busy"
                      @click="connectRemoteSession(account)"
                    >
                      {{ remoteState(account.id).busy ? t('accountPool.remote.connecting') : t('accountPool.remote.connect') }}
                    </button>
                    <button
                      v-if="remoteState(account.id).kasmId"
                      type="button"
                      class="btn btn-secondary btn-xs"
                      :disabled="remoteState(account.id).busy"
                      @click="disconnectRemoteSession(account)"
                    >
                      {{ t('accountPool.remote.disconnect') }}
                    </button>
                    <!-- Owner can re-run setup anytime to refresh the seed login (e.g. when
                         the ChatGPT session expires), instead of being stuck on connect. -->
                    <button
                      v-if="account.is_mine"
                      type="button"
                      class="btn btn-secondary btn-xs"
                      :disabled="remoteState(account.id).busy"
                      @click="setupRemoteLogin(account)"
                    >
                      {{ t('accountPool.remote.relogin') }}
                    </button>
                  </template>
                </template>
              </div>
              <div v-if="account.is_mine" class="flex items-center gap-1">
                <button
                  type="button"
                  class="relative flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                  :title="t('accountPool.pool.button')"
                  @click="openPoolDialog(account)"
                >
                  <Icon name="gift" size="sm" :class="(poolAmounts.get(account.id) || 0) > 0 ? 'text-primary-600 dark:text-primary-400' : ''" />
                  <span class="text-xs">{{ t('accountPool.pool.button') }}</span>
                  <span
                    v-if="(poolAmounts.get(account.id) || 0) > 0"
                    class="absolute right-0 top-0 h-2 w-2 rounded-full bg-primary-500"
                  ></span>
                </button>
                <button
                  type="button"
                  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                  :disabled="savingIDs.has(account.id)"
                  @click="openScopeDialog(account)"
                >
                  <Icon name="edit" size="sm" />
                  <span class="text-xs">{{ t('common.edit') }}</span>
                </button>
                <button
                  type="button"
                  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                  :disabled="savingIDs.has(account.id)"
                  @click="openDeleteDialog(account)"
                >
                  <Icon name="trash" size="sm" />
                  <span class="text-xs">{{ t('common.delete') }}</span>
                </button>
                <button
                  type="button"
                  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:hover:bg-dark-700 dark:hover:text-white"
                  @click="openAccountMenu(account, $event)"
                >
                  <Icon name="more" size="sm" />
                  <span class="text-xs">{{ t('common.more') }}</span>
                </button>
              </div>
              <span v-else class="text-sm text-gray-500 dark:text-dark-400">{{ t('accountPool.readonly') }}</span>
            </template>

            <template #cell-updated="{ row: account }">
              <span class="whitespace-nowrap text-sm text-gray-500 dark:text-dark-400">
                {{ formatDate(account.updated_at) }}
              </span>
            </template>
          </DataTable>
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
              <Icon name="grid" size="sm" />
            </div>
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('accountPool.settings.title') }}
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">
                {{ t('accountPool.settings.description') }}
              </p>
            </div>
          </div>

          <div class="space-y-4">
            <div>
              <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>
              <div class="mb-2 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200">
                <Icon name="checkCircle" size="sm" class="text-primary-500" />
                {{ t('admin.accounts.modelWhitelist') }}
              </div>
              <ModelWhitelistSelector v-model="allowedModels" :platform="createForm.platform" />
              <p class="input-hint">{{ t('accountPool.settings.modelWhitelistHint') }}</p>
            </div>

            <div v-if="createForm.platform === 'openai'" class="flex items-center justify-between border-t border-gray-200 pt-4 dark:border-dark-600">
              <div>
                <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnly') }}</label>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.accounts.openai.codexCLIOnlyDesc') }}
                </p>
              </div>
              <button
                type="button"
                @click="createForm.codexCLIOnly = !createForm.codexCLIOnly"
                :class="toggleClass(createForm.codexCLIOnly)"
              >
                <span :class="toggleKnobClass(createForm.codexCLIOnly)" />
              </button>
            </div>

            <div class="grid gap-4 border-t border-gray-200 pt-4 dark:border-dark-600 lg:grid-cols-[12rem_1fr_auto] lg:items-start">
              <label class="block">
                <span class="input-label">{{ t('admin.accounts.concurrency') }}</span>
                <input
                  v-model.number="createForm.concurrency"
                  type="number"
                  min="1"
                  class="input"
                  @input="createForm.concurrency = normalizeConcurrency(createForm.concurrency)"
                />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.accounts.expiresAt') }}</span>
                <input v-model="expiresAtInput" type="datetime-local" class="input" />
                <span class="input-hint">{{ t('admin.accounts.expiresAtHint') }}</span>
              </label>
              <div class="flex min-w-64 items-center justify-between gap-4 rounded-lg border border-gray-200 bg-white px-3 py-2.5 dark:border-dark-600 dark:bg-dark-800">
                <div>
                  <label class="input-label mb-0">{{ t('admin.accounts.autoPauseOnExpired') }}</label>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.accounts.autoPauseOnExpiredDesc') }}
                  </p>
                </div>
                <button
                  type="button"
                  @click="createForm.autoPauseOnExpired = !createForm.autoPauseOnExpired"
                  :class="toggleClass(createForm.autoPauseOnExpired)"
                >
                  <span :class="toggleKnobClass(createForm.autoPauseOnExpired)" />
                </button>
              </div>
            </div>

            <GroupSelector
              v-model="createForm.groupIds"
              :groups="availableGroups"
              :platform="createForm.platform"
              searchable="auto"
            />

            <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
              <label class="input-label">{{ t('admin.accounts.proxy') }}</label>
              <ProxySelector
                v-model="createForm.proxyId"
                :proxies="proxies"
                :test-proxy="testUserProxy"
              />
            </div>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="mb-4 flex items-start gap-3">
            <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-emerald-500 text-white">
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

          <div class="mb-4">
            <label class="input-label">{{ t('accountPool.policy.shareModeLabel') }}</label>
            <div class="mt-2 flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
              <button
                v-for="mode in shareModeOptions"
                :key="`create-${mode.value}`"
                type="button"
                class="flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
                :class="
                  createForm.shareMode === mode.value
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-800 dark:text-white'
                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                "
                @click="createForm.shareMode = mode.value"
              >
                {{ mode.label }}
              </button>
            </div>
          </div>

          <div v-if="createForm.shareMode === 'percent'" class="grid gap-4 md:grid-cols-2">
            <label class="block">
              <span class="input-label">{{ t('accountPool.policy.fiveHourReserveLabel') }}</span>
              <input
                v-model.number="createForm.fiveHourReservePercent"
                type="number"
                min="0"
                max="100"
                step="1"
                class="input"
              />
              <span class="input-hint">{{ t('accountPool.policy.fiveHourReserveHint') }}</span>
            </label>
            <label class="block">
              <span class="input-label">{{ t('accountPool.policy.weeklyReservePercentLabel') }}</span>
              <input
                v-model.number="createForm.weeklyReservePercent"
                type="number"
                min="0"
                max="100"
                step="1"
                class="input"
              />
              <span class="input-hint">{{ t('accountPool.policy.weeklyReservePercentHint') }}</span>
            </label>
          </div>

          <label v-else class="block">
            <span class="input-label">{{ t('accountPool.policy.weeklyShareBudgetLabel') }}</span>
            <div class="relative">
              <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-400">$</span>
              <input
                v-model.number="createForm.weeklyShareBudget"
                type="number"
                min="0"
                step="0.01"
                class="input pl-7"
              />
            </div>
            <span class="input-hint">{{ t('accountPool.policy.weeklyShareBudgetHint') }}</span>
          </label>

          <div class="mt-4">
            <label class="input-label">{{ t('accountPool.policy.probeFailureLabel') }}</label>
            <select v-model="createForm.probeFailurePolicy" class="input">
              <option v-for="option in probeFailurePolicyOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <p class="input-hint">{{ t('accountPool.policy.probeFailureHint') }}</p>
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
          :show-refresh-token-option="createForm.platform === 'openai'"
          :show-mobile-refresh-token-option="createForm.platform === 'openai'"
          :show-session-token-option="false"
          :show-access-token-option="false"
          :show-codex-session-import-option="createForm.platform === 'openai'"
          :platform="createForm.platform"
          :show-project-id="createForm.platform === 'gemini' && createForm.oauthType === 'code_assist'"
          @generate-url="handleGenerateUrl"
          @validate-refresh-token="handleValidateRefreshToken"
          @validate-mobile-refresh-token="handleValidateMobileRefreshToken"
          @import-codex-session="handleImportCodexSession"
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
            v-if="isManualInputMethod"
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

    <BaseDialog
      :show="showScopeForm"
      :title="t('accountPool.settings.editTitle')"
      width="wide"
      @close="handleScopeClose"
    >
      <form
        id="user-account-pool-scope-form"
        class="space-y-5"
        @submit.prevent="saveScope"
      >
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="mb-4 flex items-start gap-3">
            <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-500 text-white">
              <Icon name="grid" size="sm" />
            </div>
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ scopeAccount?.name || t('accountPool.settings.title') }}
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">
                {{ t('accountPool.settings.description') }}
              </p>
            </div>
          </div>

          <div class="space-y-4">
            <div>
              <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>
              <div class="mb-2 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200">
                <Icon name="checkCircle" size="sm" class="text-primary-500" />
                {{ t('admin.accounts.modelWhitelist') }}
              </div>
              <ModelWhitelistSelector
                v-if="scopeAccount"
                v-model="scopeAllowedModels"
                :platform="scopeAccount.platform"
                :account-id="scopeAccount.id"
              />
              <p class="input-hint">{{ t('accountPool.settings.modelWhitelistHint') }}</p>
            </div>

            <div v-if="scopeAccount?.platform === 'openai'" class="flex items-center justify-between border-t border-gray-200 pt-4 dark:border-dark-600">
              <div>
                <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnly') }}</label>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.accounts.openai.codexCLIOnlyDesc') }}
                </p>
              </div>
              <button
                type="button"
                @click="scopeForm.codexCLIOnly = !scopeForm.codexCLIOnly"
                :class="toggleClass(scopeForm.codexCLIOnly)"
              >
                <span :class="toggleKnobClass(scopeForm.codexCLIOnly)" />
              </button>
            </div>

            <div class="grid gap-4 border-t border-gray-200 pt-4 dark:border-dark-600 lg:grid-cols-[12rem_1fr_auto] lg:items-start">
              <label class="block">
                <span class="input-label">{{ t('admin.accounts.concurrency') }}</span>
                <input
                  v-model.number="scopeForm.concurrency"
                  type="number"
                  min="1"
                  class="input"
                  @input="scopeForm.concurrency = normalizeConcurrency(scopeForm.concurrency)"
                />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.accounts.expiresAt') }}</span>
                <input v-model="scopeExpiresAtInput" type="datetime-local" class="input" />
                <span class="input-hint">{{ t('admin.accounts.expiresAtHint') }}</span>
              </label>
              <div class="flex min-w-64 items-center justify-between gap-4 rounded-lg border border-gray-200 bg-white px-3 py-2.5 dark:border-dark-600 dark:bg-dark-800">
                <div>
                  <label class="input-label mb-0">{{ t('admin.accounts.autoPauseOnExpired') }}</label>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.accounts.autoPauseOnExpiredDesc') }}
                  </p>
                </div>
                <button
                  type="button"
                  @click="scopeForm.autoPauseOnExpired = !scopeForm.autoPauseOnExpired"
                  :class="toggleClass(scopeForm.autoPauseOnExpired)"
                >
                  <span :class="toggleKnobClass(scopeForm.autoPauseOnExpired)" />
                </button>
              </div>
            </div>

            <GroupSelector
              v-if="scopeAccount"
              v-model="scopeForm.groupIds"
              :groups="availableGroups"
              :platform="scopeAccount.platform"
              searchable="auto"
            />

            <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
              <label class="input-label">{{ t('admin.accounts.proxy') }}</label>
              <ProxySelector
                v-model="scopeForm.proxyId"
                :proxies="proxies"
                :test-proxy="testUserProxy"
              />
            </div>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="mb-4 flex items-start gap-3">
            <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-emerald-500 text-white">
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

          <div class="mb-4">
            <label class="input-label">{{ t('accountPool.policy.shareModeLabel') }}</label>
            <div class="mt-2 flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
              <button
                v-for="mode in shareModeOptions"
                :key="`scope-${mode.value}`"
                type="button"
                class="flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
                :class="
                  scopeForm.shareMode === mode.value
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-800 dark:text-white'
                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                "
                @click="scopeForm.shareMode = mode.value"
              >
                {{ mode.label }}
              </button>
            </div>
          </div>

          <div v-if="scopeForm.shareMode === 'percent'" class="grid gap-4 md:grid-cols-2">
            <label class="block">
              <span class="input-label">{{ t('accountPool.policy.fiveHourReserveLabel') }}</span>
              <input
                v-model.number="scopeForm.fiveHourReservePercent"
                type="number"
                min="0"
                max="100"
                step="1"
                class="input"
              />
              <span class="input-hint">{{ t('accountPool.policy.fiveHourReserveHint') }}</span>
            </label>
            <label class="block">
              <span class="input-label">{{ t('accountPool.policy.weeklyReservePercentLabel') }}</span>
              <input
                v-model.number="scopeForm.weeklyReservePercent"
                type="number"
                min="0"
                max="100"
                step="1"
                class="input"
              />
              <span class="input-hint">{{ t('accountPool.policy.weeklyReservePercentHint') }}</span>
            </label>
          </div>

          <label v-else class="block">
            <span class="input-label">{{ t('accountPool.policy.weeklyShareBudgetLabel') }}</span>
            <div class="relative">
              <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-400">$</span>
              <input
                v-model.number="scopeForm.weeklyShareBudget"
                type="number"
                min="0"
                step="0.01"
                class="input pl-7"
              />
            </div>
            <span class="input-hint">{{ t('accountPool.policy.weeklyShareBudgetHint') }}</span>
          </label>

          <div class="mt-4">
            <label class="input-label">{{ t('accountPool.policy.probeFailureLabel') }}</label>
            <select v-model="scopeForm.probeFailurePolicy" class="input">
              <option v-for="option in probeFailurePolicyOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <p class="input-hint">{{ t('accountPool.policy.probeFailureHint') }}</p>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="handleScopeClose">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="user-account-pool-scope-form"
            class="btn btn-primary"
            :disabled="!scopeAccount || savingIDs.has(scopeAccount.id)"
          >
            {{ scopeAccount && savingIDs.has(scopeAccount.id) ? t('accountPool.saving') : t('accountPool.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Contribution reward pool (held-pool + manual distribute, Model B). -->
    <BaseDialog
      :show="poolDialog.show"
      :title="t('accountPool.pool.title')"
      width="wide"
      @close="closePoolDialog"
    >
      <div v-if="poolDialog.loading" class="py-10 text-center text-sm text-gray-500 dark:text-dark-300">
        {{ t('common.loading') }}
      </div>
      <div v-else class="space-y-5">
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="flex items-center justify-between">
            <div>
              <div class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ poolDialog.account?.name }}
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ t('accountPool.pool.description') }}</p>
            </div>
            <div class="text-right">
              <div class="text-xs text-gray-500 dark:text-dark-300">{{ t('accountPool.pool.held') }}</div>
              <div class="font-mono text-lg font-semibold text-primary-600 dark:text-primary-400">
                ${{ poolDialog.poolAmount.toFixed(6) }}
              </div>
            </div>
          </div>
        </div>

        <div v-if="poolDialog.poolAmount <= 0" class="py-6 text-center text-sm text-gray-500 dark:text-dark-300">
          {{ t('accountPool.pool.empty') }}
        </div>

        <template v-else>
          <div class="flex gap-2">
            <button
              type="button"
              class="btn btn-xs"
              :class="poolDialog.mode === 'even' ? 'btn-primary' : 'btn-secondary'"
              @click="poolDialog.mode = 'even'"
            >
              {{ t('accountPool.pool.modeEven') }}
            </button>
            <button
              type="button"
              class="btn btn-xs"
              :class="poolDialog.mode === 'custom' ? 'btn-primary' : 'btn-secondary'"
              @click="poolDialog.mode = 'custom'"
            >
              {{ t('accountPool.pool.modeCustom') }}
            </button>
          </div>

          <div v-if="poolDialog.mode === 'even'" class="text-sm text-gray-600 dark:text-dark-200">
            {{ t('accountPool.pool.evenHint', { count: poolDialog.recipients.length, each: (poolDialog.poolAmount / Math.max(poolDialog.recipients.length, 1)).toFixed(6) }) }}
          </div>

          <div class="space-y-2">
            <div
              v-for="r in poolDialog.recipients"
              :key="r.user_id"
              class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-700"
            >
              <div class="min-w-0 text-sm">
                <span class="font-medium text-gray-900 dark:text-gray-100">{{ r.display_name }}</span>
                <span v-if="r.is_primary_owner" class="ml-2 rounded bg-primary-50 px-1.5 py-0.5 text-[11px] text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">
                  {{ t('accountPool.pool.primaryOwner') }}
                </span>
              </div>
              <div v-if="poolDialog.mode === 'custom'" class="flex items-center gap-1">
                <span class="text-sm text-gray-400">$</span>
                <input
                  v-model="r.amount"
                  type="number"
                  min="0"
                  step="0.000001"
                  class="input w-32 text-right font-mono text-sm"
                  placeholder="0"
                />
              </div>
              <div v-else class="font-mono text-sm text-gray-600 dark:text-dark-200">
                ${{ (poolDialog.poolAmount / Math.max(poolDialog.recipients.length, 1)).toFixed(6) }}
              </div>
            </div>
          </div>

          <div v-if="poolDialog.mode === 'custom'" class="flex justify-between text-sm" :class="poolCustomOver ? 'text-red-600 dark:text-red-400' : 'text-gray-600 dark:text-dark-200'">
            <span>{{ t('accountPool.pool.total') }}</span>
            <span class="font-mono">${{ poolCustomTotal.toFixed(6) }} / ${{ poolDialog.poolAmount.toFixed(6) }}</span>
          </div>
        </template>
      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closePoolDialog">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="poolDialog.distributing || poolDialog.poolAmount <= 0 || (poolDialog.mode === 'custom' && (poolCustomOver || poolCustomTotal <= 0))"
            @click="distributePool"
          >
            {{ poolDialog.distributing ? t('accountPool.pool.distributing') : t('accountPool.pool.distribute') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <AccountStatsModal
      :show="showStats"
      :account="statsAccount"
      :load-stats="accountsAPI.getStats"
      @close="closeStatsModal"
    />

    <AccountTestModal
      :show="showTest"
      :account="testingAccount"
      :load-models="accountsAPI.getAvailableModels"
      :test-endpoint="userAccountTestEndpoint"
      @close="closeTestModal"
    />

    <ScheduledTestsPanel
      :show="showSchedulePanel"
      :account-id="scheduleAccount?.id ?? null"
      :model-options="scheduleModelOptions"
      :api="accountsAPI.scheduledTests"
      @close="closeSchedulePanel"
    />

    <BaseDialog
      :show="showReAuth"
      :title="t('admin.accounts.reAuthorizeAccount')"
      width="normal"
      @close="closeReAuthModal"
    >
      <div v-if="reAuthAccount" class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-green-500 to-green-600">
              <Icon name="link" size="md" class="text-white" />
            </div>
            <div>
              <span class="block font-semibold text-gray-900 dark:text-white">{{ reAuthAccount.name }}</span>
              <span class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.openaiAccount') }}</span>
            </div>
          </div>
        </div>

        <OAuthAuthorizationFlow
          ref="reAuthFlowRef"
          add-method="oauth"
          :auth-url="reAuthSession.authUrl"
          :session-id="reAuthSession.sessionId"
          :loading="reAuthLoading"
          :error="reAuthError"
          :show-help="false"
          :show-proxy-warning="false"
          :allow-multiple="false"
          :show-cookie-option="false"
          :show-refresh-token-option="false"
          :show-mobile-refresh-token-option="false"
          :show-session-token-option="false"
          :show-access-token-option="false"
          :show-codex-session-import-option="false"
          platform="openai"
          @generate-url="handleReAuthGenerateUrl"
        />
      </div>

      <template #footer>
        <div class="flex justify-between gap-3">
          <button type="button" class="btn btn-secondary" @click="closeReAuthModal">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            :disabled="!canReAuthExchangeCode"
            class="btn btn-primary"
            @click="handleReAuthExchangeCode"
          >
            <svg
              v-if="reAuthLoading"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ reAuthLoading ? t('admin.accounts.oauth.verifying') : t('admin.accounts.oauth.completeAuth') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <UserAccountActionMenu
      :show="actionMenu.show"
      :account="actionMenu.account"
      :position="actionMenu.position"
      @close="actionMenu.show = false"
      @test="openTestModal"
      @stats="openStatsModal"
      @schedule="openSchedulePanel"
      @reauth="openReAuthModal"
      @refresh-token="refreshAccountToken"
      @set-privacy="setAccountPrivacy"
      @recover-state="recoverAccountState"
      @toggle-scheduling="toggleSchedulable"
      @edit="openScopeDialog"
      @delete="openDeleteDialog"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('accountPool.delete.title')"
      :message="t('accountPool.delete.confirm', { name: deletingAccount?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="closeDeleteDialog"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import OAuthAuthorizationFlow from '@/components/account/OAuthAuthorizationFlow.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountStatsModal from '@/components/admin/account/AccountStatsModal.vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import ScheduledTestsPanel from '@/components/admin/account/ScheduledTestsPanel.vue'
import UserAccountActionMenu from '@/components/user/UserAccountActionMenu.vue'
import accountsAPI, {
  type ContributionProbeFailurePolicy,
  type ContributionShareMode,
  type UserOAuthAuthUrlRequest,
  type UserOAuthTokenInfo,
} from '@/api/accounts'
import userAPI from '@/api/user'
import { userGroupsAPI } from '@/api/groups'
import type { AccountPlatform, Group, Proxy, UserAccountPoolItem } from '@/types'
import type { SelectOption } from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  buildModelMappingObject,
  splitModelMappingObject,
} from '@/composables/useModelWhitelist'
import { formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'
import type { CodexSessionImportMessage } from '@/types'

type UserAccountModalItem = UserAccountPoolItem & {
  credentials?: Record<string, unknown>
}

interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  inputMethod: string
  refreshToken: string
  codexSession: string
  reset: () => void
}

const { t } = useI18n()
const appStore = useAppStore()

const platforms: AccountPlatform[] = ['anthropic', 'openai']
const accounts = ref<UserAccountPoolItem[]>([])
const availableGroups = ref<Group[]>([])
const proxies = ref<Proxy[]>([])
const loading = ref(false)
const creating = ref(false)
const transferringContribution = ref(false)
const oauthLoading = ref(false)
const oauthError = ref('')
const showCreateForm = ref(false)
const createStep = ref(1)
const searchQuery = ref('')
const platformFilter = ref<AccountPlatform | ''>('')
const planTypeFilter = ref<'' | 'plus' | 'pro' | 'prolite'>('')
const savingIDs = ref(new Set<number>())
const oauthFlowRef = ref<OAuthFlowExposed | null>(null)
const showColumnsDropdown = ref(false)
const columnsDropdownRef = ref<HTMLElement | null>(null)
const hiddenColumns = reactive<Set<string>>(new Set())
const HIDDEN_COLUMNS_KEY = 'account-pool-hidden-columns'
const DEFAULT_HIDDEN_COLUMNS = ['owner', 'updated']
const DEFAULT_USER_ACCOUNT_CONCURRENCY = 5
const actionMenu = reactive<{
  show: boolean
  account: UserAccountPoolItem | null
  position: { top: number; left: number } | null
}>({
  show: false,
  account: null,
  position: null,
})
const showDeleteDialog = ref(false)
const deletingAccount = ref<UserAccountPoolItem | null>(null)

// Account-level contribution reward pool (held-pool + manual distribute, Model B).
const poolAmounts = reactive<Map<number, number>>(new Map())
interface PoolDialogRecipient {
  user_id: number
  display_name: string
  is_primary_owner: boolean
  amount: string
}
const poolDialog = reactive<{
  show: boolean
  account: UserAccountPoolItem | null
  loading: boolean
  distributing: boolean
  poolAmount: number
  mode: 'even' | 'custom'
  recipients: PoolDialogRecipient[]
}>({
  show: false,
  account: null,
  loading: false,
  distributing: false,
  poolAmount: 0,
  mode: 'even',
  recipients: [],
})
const poolCustomTotal = computed(() =>
  poolDialog.recipients.reduce((sum, r) => sum + (parseFloat(r.amount) || 0), 0),
)
const poolCustomOver = computed(() => poolCustomTotal.value > poolDialog.poolAmount + 1e-9)
const showStats = ref(false)
const statsAccount = ref<UserAccountPoolItem | null>(null)
const showTest = ref(false)
const testingAccount = ref<UserAccountModalItem | null>(null)
const showSchedulePanel = ref(false)
const scheduleAccount = ref<UserAccountPoolItem | null>(null)
const scheduleModelOptions = ref<SelectOption[]>([])
const showReAuth = ref(false)
const reAuthAccount = ref<UserAccountPoolItem | null>(null)
const reAuthFlowRef = ref<OAuthFlowExposed | null>(null)
const reAuthLoading = ref(false)
const reAuthError = ref('')
const reAuthSession = reactive({
  authUrl: '',
  sessionId: '',
  state: '',
})
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

// --- Remote browser session (Kasm) state, keyed by account id ----------------
interface RemoteSessionState {
  busy: boolean // setup / connect request in flight
  queued: boolean
  position: number | null
  kasmId: string | null // set once a session is running, enables "断开"
  timer: number | null // polling timer for queued sessions
}
const remoteSessions = reactive<Map<number, RemoteSessionState>>(new Map())

// The opened Kasm Window handle and its close-poll interval are kept OUT of the
// reactive state on purpose: Vue introspects reactive values (reads __v_isReadonly),
// which throws a SecurityError on a cross-origin Window. Plain non-reactive maps.
const remoteWindows = new Map<number, Window>()
const remoteWatchTimers = new Map<number, number>()
// Keepalive ping timers (every ~30s while the Kasm tab is open). Kept out of reactive
// state for the same cross-origin reason as the window/watch maps above.
const remoteKeepaliveTimers = new Map<number, number>()

function remoteState(id: number): RemoteSessionState {
  let state = remoteSessions.get(id)
  if (!state) {
    state = { busy: false, queued: false, position: null, kasmId: null, timer: null }
    remoteSessions.set(id, state)
  }
  return state
}

function clearRemoteTimer(state: RemoteSessionState) {
  if (state.timer !== null) {
    window.clearTimeout(state.timer)
    state.timer = null
  }
}

function clearRemoteWatch(accountId: number) {
  const t = remoteWatchTimers.get(accountId)
  if (t !== undefined) {
    window.clearInterval(t)
    remoteWatchTimers.delete(accountId)
  }
  const k = remoteKeepaliveTimers.get(accountId)
  if (k !== undefined) {
    window.clearInterval(k)
    remoteKeepaliveTimers.delete(accountId)
  }
  remoteWindows.delete(accountId)
}

// watchRemoteWindow polls the opened Kasm tab; when the user closes it we
// auto-disconnect so the container is torn down instead of lingering until the
// Kasm keepalive / 4h hard limit. Closing the tab is the "user left" signal.
// The Window lives in a non-reactive map (see above); only window.closed is read,
// which is safe to access cross-origin.
function watchRemoteWindow(account: UserAccountPoolItem, win: Window | null) {
  clearRemoteWatch(account.id)
  if (!win) return
  remoteWindows.set(account.id, win)
  const timer = window.setInterval(() => {
    if (!win.closed) return
    clearRemoteWatch(account.id)
    const state = remoteState(account.id)
    const kid = state.kasmId
    state.kasmId = null
    if (kid) {
      // Best-effort; the slot frees up regardless.
      accountsAPI.disconnectRemoteSession(account.id, kid).catch(() => {})
    }
  }, 2000)
  remoteWatchTimers.set(account.id, timer)

  // Keepalive: while the tab is open, ping the backend every 30s so the reconciler keeps
  // the container alive (Kasm's connection_info is empty, so this is the liveness signal
  // that prevents the session from being reaped mid-use).
  const keepalive = window.setInterval(() => {
    if (win.closed) return
    accountsAPI.keepaliveRemoteSession(account.id).catch(() => {})
  }, 30000)
  remoteKeepaliveTimers.set(account.id, keepalive)
}

function isProAccount(account: UserAccountPoolItem): boolean {
  const plan = (account.plan_type || '').toLowerCase()
  return plan === 'pro' || plan === 'chatgptpro'
}

function isRemoteSeedReady(account: UserAccountPoolItem): boolean {
  // Top-level remote_seed_ready is exposed to owner AND co-owners (extra is owner-only).
  return account.remote_seed_ready === true || account.extra?.remote_seed_ready === true
}

async function setupRemoteLogin(account: UserAccountPoolItem) {
  const state = remoteState(account.id)
  if (state.busy) return
  state.busy = true
  try {
    const res = await accountsAPI.setupRemoteSession(account.id)
    state.kasmId = res.kasm_id ?? null
    if (res.connect_url) {
      const win = window.open(res.connect_url, '_blank')
      watchRemoteWindow(account, win)
    }
    appStore.showSuccess(t('accountPool.remote.setupHint'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.remote.setupFailed')))
  } finally {
    state.busy = false
  }
}

function applyRemoteResult(account: UserAccountPoolItem, res: {
  status: 'ready' | 'queued'
  connect_url?: string
  kasm_id?: string
  position?: number
}) {
  const state = remoteState(account.id)
  if (res.status === 'ready') {
    clearRemoteTimer(state)
    state.queued = false
    state.position = null
    state.kasmId = res.kasm_id ?? null
    if (res.connect_url) {
      const win = window.open(res.connect_url, '_blank')
      watchRemoteWindow(account, win)
    } else {
      appStore.showError(t('accountPool.remote.openFailed'))
    }
  } else {
    state.queued = true
    state.position = res.position ?? null
  }
}

function pollRemoteSession(account: UserAccountPoolItem) {
  const state = remoteState(account.id)
  state.timer = window.setTimeout(async () => {
    if (!state.queued) return
    try {
      const res = await accountsAPI.getRemoteSessionStatus(account.id)
      applyRemoteResult(account, res)
      if (state.queued) {
        pollRemoteSession(account)
      }
    } catch (err: unknown) {
      clearRemoteTimer(state)
      state.queued = false
      state.position = null
      appStore.showError(extractApiErrorMessage(err, t('accountPool.remote.openFailed')))
    }
  }, 3000)
}

async function connectRemoteSession(account: UserAccountPoolItem) {
  const state = remoteState(account.id)
  if (state.busy || state.queued) return
  state.busy = true
  try {
    const res = await accountsAPI.startRemoteSession(account.id)
    applyRemoteResult(account, res)
    if (state.queued) {
      pollRemoteSession(account)
    }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.remote.openFailed')))
  } finally {
    state.busy = false
  }
}

function cancelRemoteQueue(account: UserAccountPoolItem) {
  const state = remoteState(account.id)
  clearRemoteTimer(state)
  state.queued = false
  state.position = null
}

async function disconnectRemoteSession(account: UserAccountPoolItem) {
  const state = remoteState(account.id)
  if (!state.kasmId || state.busy) return
  state.busy = true
  try {
    await accountsAPI.disconnectRemoteSession(account.id, state.kasmId)
    clearRemoteWatch(account.id)
    state.kasmId = null
    appStore.showSuccess(t('accountPool.remote.disconnectSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.remote.disconnectFailed')))
  } finally {
    state.busy = false
  }
}

const contributionSummary = reactive({
  contribution_quota: 0,
  contribution_frozen_quota: 0,
  contribution_history_quota: 0,
})

const createForm = reactive({
  name: '',
  platform: 'openai' as AccountPlatform,
  oauthType: 'code_assist' as 'code_assist' | 'google_one' | 'ai_studio',
  projectId: '',
  tierId: '',
  schedulable: true,
  proxyId: null as number | null,
  concurrency: DEFAULT_USER_ACCOUNT_CONCURRENCY,
  groupIds: [] as number[],
  expiresAt: null as number | null,
  autoPauseOnExpired: true,
  codexCLIOnly: false,
  fiveHourReservePercent: 0,
  weeklyReservePercent: 0,
  shareMode: 'percent' as ContributionShareMode,
  weeklyShareBudget: 0,
  probeFailurePolicy: 'continue' as ContributionProbeFailurePolicy,
})

const allowedModels = ref<string[]>([])
const showScopeForm = ref(false)
const scopeAccount = ref<UserAccountPoolItem | null>(null)
const scopeAllowedModels = ref<string[]>([])
const scopeForm = reactive({
  groupIds: [] as number[],
  proxyId: null as number | null,
  concurrency: DEFAULT_USER_ACCOUNT_CONCURRENCY,
  expiresAt: null as number | null,
  autoPauseOnExpired: true,
  codexCLIOnly: false,
  fiveHourReservePercent: 0,
  weeklyReservePercent: 0,
  shareMode: 'percent' as ContributionShareMode,
  weeklyShareBudget: 0,
  probeFailurePolicy: 'continue' as ContributionProbeFailurePolicy,
})

const expiresAtInput = computed({
  get: () => formatDateTimeLocalInput(createForm.expiresAt),
  set: (value: string) => {
    createForm.expiresAt = parseDateTimeLocalInput(value)
  },
})

const availableGroupsForPlatform = computed(() =>
  availableGroups.value.filter((group) => group.platform === createForm.platform)
)

const scopeExpiresAtInput = computed({
  get: () => formatDateTimeLocalInput(scopeForm.expiresAt),
  set: (value: string) => {
    scopeForm.expiresAt = parseDateTimeLocalInput(value)
  },
})

const geminiOAuthOptions = [
  { value: 'code_assist' as const, label: 'Code Assist' },
  { value: 'google_one' as const, label: 'Google One' },
  { value: 'ai_studio' as const, label: 'AI Studio' },
]

const probeFailurePolicyOptions = computed(() => [
  { value: 'continue', label: t('accountPool.policy.probeFailureContinue') },
  { value: 'pause', label: t('accountPool.policy.probeFailurePause') },
  { value: 'local', label: t('accountPool.policy.probeFailureLocal') },
])

const shareModeOptions = computed<{ value: ContributionShareMode; label: string }[]>(() => [
  { value: 'percent', label: t('accountPool.policy.shareModePercent') },
  { value: 'budget', label: t('accountPool.policy.shareModeBudget') },
])

const isManualInputMethod = computed(() => oauthFlowRef.value?.inputMethod === 'manual')

const canExchangeCode = computed(() => {
  const authCode = oauthFlowRef.value?.authCode || ''
  return Boolean(
    isManualInputMethod.value &&
      authCode.trim() &&
      oauthSession.sessionId &&
      !oauthLoading.value &&
      !creating.value
  )
})

const canReAuthExchangeCode = computed(() => {
  const authCode = reAuthFlowRef.value?.authCode || ''
  return Boolean(authCode.trim() && reAuthSession.sessionId && !reAuthLoading.value)
})

const allColumns = computed<Column[]>(() => [
  { key: 'account', label: t('accountPool.columns.account'), sortable: false },
  { key: 'account_type', label: t('accountPool.columns.accountType'), sortable: false },
  { key: 'owner', label: t('accountPool.columns.owner'), sortable: false },
  { key: 'status', label: t('accountPool.columns.status'), sortable: false },
  { key: 'scheduling', label: t('accountPool.columns.scheduling'), sortable: false },
  { key: 'concurrency', label: t('accountPool.columns.concurrency'), sortable: false },
  { key: 'five_hour', label: t('accountPool.columns.fiveHour'), sortable: false },
  { key: 'weekly', label: t('accountPool.columns.weekly'), sortable: false },
  { key: 'protection', label: t('accountPool.columns.protection'), sortable: false },
  { key: 'availability', label: t('accountPool.columns.availability'), sortable: false },
  { key: 'expires_at', label: t('accountPool.columns.expiresAt'), sortable: false },
  { key: 'updated', label: t('accountPool.columns.updated'), sortable: false },
  { key: 'actions', label: t('accountPool.columns.actions'), sortable: false },
])

const toggleableColumns = computed(() =>
  allColumns.value.filter((column) => column.key !== 'account' && column.key !== 'actions')
)

const visibleColumns = computed(() =>
  allColumns.value.filter((column) =>
    column.key === 'account' || column.key === 'actions' || !hiddenColumns.has(column.key)
  )
)

function loadSavedColumns() {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      parsed.forEach((key) => hiddenColumns.add(key))
      return
    }
  } catch (err) {
    console.error('Failed to load account pool columns:', err)
  }
  DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
}

function saveColumnsToStorage() {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
  } catch (err) {
    console.error('Failed to save account pool columns:', err)
  }
}

function toggleColumn(key: string) {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
}

function isColumnVisible(key: string) {
  return !hiddenColumns.has(key)
}

if (typeof window !== 'undefined') {
  loadSavedColumns()
}

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

function formatPercent(value: number | null | undefined): string {
  const n = Number(value)
  if (!Number.isFinite(n)) return '-'
  return `${Math.max(0, Math.min(100, n)).toFixed(n % 1 === 0 ? 0 : 1)}%`
}

function normalizeReservePercent(value: number | null | undefined): number {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  return Math.max(0, Math.min(100, n))
}

function normalizeConcurrency(value: number | null | undefined): number {
  const n = Number(value)
  if (!Number.isFinite(n)) return 1
  return Math.max(1, Math.floor(n))
}

function normalizeShareBudget(value: number | null | undefined): number {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  return Math.max(0, n)
}

// Build the contribution sharing payload fields for a given form. share_mode is
// always sent; the weekly budget is sent in budget mode, the reserve percents in
// percent mode (kept in sync with how the backend stores the active mode).
function buildContributionPayload(form: {
  shareMode: ContributionShareMode
  weeklyShareBudget: number
  fiveHourReservePercent: number
  weeklyReservePercent: number
}) {
  return {
    contribution_share_mode: form.shareMode,
    contribution_weekly_share_budget:
      form.shareMode === 'budget' ? normalizeShareBudget(form.weeklyShareBudget) : undefined,
    contribution_5h_reserve_percent: normalizeReservePercent(form.fiveHourReservePercent),
    contribution_weekly_reserve_percent: normalizeReservePercent(form.weeklyReservePercent),
  }
}

// Whether others have exhausted the weekly share budget (budget mode only).
function isShareBudgetExhausted(account: UserAccountPoolItem): boolean {
  if (account.contribution_share_mode !== 'budget') return false
  const budget = Number(account.contribution_weekly_share_budget) || 0
  if (budget <= 0) return false
  return (account.others_weekly_spend ?? 0) >= budget
}

function formatMoney(value: number | null | undefined): string {
  const n = Number(value)
  if (!Number.isFinite(n)) return '-'
  return `$${n.toFixed(n >= 10 ? 2 : 4).replace(/\.?0+$/, '')}`
}

function formatDate(value: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

function formatOptionalUnixDate(value: string | null | undefined): string {
  if (!value) return t('accountPool.settings.noExpiry')
  return formatDate(value)
}

function isModelLimited(account: UserAccountPoolItem): boolean {
  return modelLimitCount(account) > 0
}

function modelLimitCount(account: UserAccountPoolItem): number {
  return Object.keys(account.model_mapping || {}).length
}

function parseUserModelWhitelist(mapping: Record<string, string> | null | undefined): string[] {
  const parsed = splitModelMappingObject(mapping)
  return parsed.allowedModels
}

function buildUserModelWhitelist(models: string[]): Record<string, string> {
  return buildModelMappingObject('whitelist', models, []) || {}
}

function probeFailurePolicyLabel(policy: UserAccountPoolItem['contribution_probe_failure_policy']): string {
  return t(`accountPool.policy.probeFailure${policy === 'continue' ? 'Continue' : policy === 'pause' ? 'Pause' : 'Local'}`)
}

function resetOAuthSession() {
  oauthSession.authUrl = ''
  oauthSession.sessionId = ''
  oauthSession.state = ''
  oauthError.value = ''
}

function resetReAuthSession() {
  reAuthSession.authUrl = ''
  reAuthSession.sessionId = ''
  reAuthSession.state = ''
  reAuthError.value = ''
  reAuthFlowRef.value?.reset()
}

function compactRecord(input: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(input).filter(([, value]) => value !== undefined && value !== null && value !== '')
  )
}

function withModelMapping(credentials: Record<string, unknown>): Record<string, unknown> {
  const modelMapping = buildUserModelWhitelist(allowedModels.value)
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  }
  return credentials
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
  if (createForm.platform === 'openai') return withModelMapping(buildOpenAICredentials(tokenInfo))
  if (createForm.platform === 'gemini') return withModelMapping(buildGeminiCredentials(tokenInfo))
  if (createForm.platform === 'antigravity') return withModelMapping(buildAntigravityCredentials(tokenInfo))
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
  if (createForm.platform === 'openai' && createForm.codexCLIOnly) {
    extra.codex_cli_only = true
  }
  return Object.keys(extra).length > 0 ? extra : undefined
}

function buildOpenAIExtra(tokenInfo: UserOAuthTokenInfo): Record<string, unknown> | undefined {
  const extra = compactRecord({
    email: tokenInfo.email,
    name: tokenInfo.name,
    privacy_mode: tokenInfo.privacy_mode,
  })
  return Object.keys(extra).length > 0 ? extra : undefined
}

function buildCreateOAuthPayload(
  name: string,
  tokenInfo: UserOAuthTokenInfo,
  credentials: Record<string, unknown> = buildCredentials(tokenInfo),
) {
  return {
    name,
    platform: createForm.platform,
    type: 'oauth' as const,
    credentials,
    model_mapping: buildUserModelWhitelist(allowedModels.value),
    extra: buildExtra(tokenInfo),
    proxy_id: createForm.proxyId,
    concurrency: normalizeConcurrency(createForm.concurrency),
    schedulable: createForm.schedulable,
    group_ids: createForm.groupIds,
    expires_at: createForm.expiresAt,
    auto_pause_on_expired: createForm.autoPauseOnExpired,
    ...buildContributionPayload(createForm),
    contribution_probe_failure_policy: createForm.probeFailurePolicy,
  }
}

function baseOpenAIAccountName(tokenInfo?: UserOAuthTokenInfo): string {
  const email = typeof tokenInfo?.email === 'string' ? tokenInfo.email.trim() : ''
  return createForm.name.trim() || email || 'OpenAI OAuth Account'
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
  createForm.groupIds = createForm.groupIds.filter((groupID) =>
    availableGroupsForPlatform.value.some((group) => group.id === groupID)
  )
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
  createForm.groupIds = []
  if (platform !== 'openai') {
    createForm.codexCLIOnly = false
  }
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

function toggleClass(active: boolean) {
  return [
    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
    active ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600',
  ]
}

function toggleKnobClass(active: boolean) {
  return [
    'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
    active ? 'translate-x-5' : 'translate-x-0',
  ]
}

function resetModelWhitelist() {
  allowedModels.value = []
}

function resetScopeModelWhitelist() {
  scopeAllowedModels.value = []
}

async function loadAvailableGroups() {
  try {
    const groups = await userGroupsAPI.getAvailable()
    availableGroups.value = groups
    createForm.groupIds = createForm.groupIds.filter((groupID) =>
      groups.some((group) => group.id === groupID)
    )
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.users.failedToLoadGroups')))
  }
}

async function loadProxies() {
  try {
    proxies.value = await accountsAPI.listProxies()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.proxies.failedToLoad')))
  }
}

async function testUserProxy(proxy: Proxy) {
  return accountsAPI.testProxy(proxy.id)
}

async function handleGenerateUrl() {
  oauthLoading.value = true
  oauthError.value = ''
  try {
    const projectId = oauthFlowRef.value?.projectId || createForm.projectId
    const payload: UserOAuthAuthUrlRequest = {
      platform: createForm.platform,
      proxy_id: createForm.proxyId,
    }
    if (createForm.platform === 'gemini') {
      payload.redirect_uri = `${window.location.origin}/auth/callback`
      payload.project_id = projectId || undefined
      payload.oauth_type = createForm.oauthType
      payload.tier_id = createForm.tierId || undefined
    }

    const result = await accountsAPI.generateOAuthAuthUrl(payload)
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

function replaceAccount(updated: UserAccountPoolItem) {
  const index = accounts.value.findIndex((item) => item.id === updated.id)
  if (index >= 0) {
    accounts.value[index] = updated
  }
}

async function loadAccounts() {
  loading.value = true
  try {
    const result = await accountsAPI.listPool(pagination.page, pagination.page_size, {
      platform: platformFilter.value,
      plan_type: planTypeFilter.value,
      search: searchQuery.value.trim(),
      sort_by: 'name',
      sort_order: 'asc',
    })
    accounts.value = result.items
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
    void loadPoolAmounts()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.loadFailed')))
  } finally {
    loading.value = false
  }
}

// Best-effort: fetch each owned account's held reward pool so the card can badge it.
async function loadPoolAmounts() {
  const mine = accounts.value.filter((a) => a.is_mine)
  await Promise.all(
    mine.map(async (a) => {
      try {
        const view = await accountsAPI.getContributionPool(a.id)
        poolAmounts.set(a.id, view.pool_amount || 0)
      } catch {
        // ignore — pool badge is best-effort
      }
    }),
  )
}

function openPoolDialog(account: UserAccountPoolItem) {
  poolDialog.account = account
  poolDialog.show = true
  poolDialog.loading = true
  poolDialog.distributing = false
  poolDialog.mode = 'even'
  poolDialog.recipients = []
  poolDialog.poolAmount = 0
  accountsAPI
    .getContributionPool(account.id)
    .then((view) => {
      poolDialog.poolAmount = view.pool_amount || 0
      poolAmounts.set(account.id, view.pool_amount || 0)
      poolDialog.recipients = (view.recipients || []).map((r) => ({
        user_id: r.user_id,
        display_name: r.display_name || `#${r.user_id}`,
        is_primary_owner: r.is_primary_owner,
        amount: '',
      }))
    })
    .catch((err: unknown) => {
      appStore.showError(extractApiErrorMessage(err, t('accountPool.pool.loadFailed')))
      poolDialog.show = false
    })
    .finally(() => {
      poolDialog.loading = false
    })
}

function closePoolDialog() {
  poolDialog.show = false
  poolDialog.account = null
  poolDialog.recipients = []
}

async function distributePool() {
  if (!poolDialog.account || poolDialog.distributing) return
  if (poolDialog.poolAmount <= 0) return
  const accountId = poolDialog.account.id
  let payload: { mode?: 'even'; allocations?: { user_id: number; amount: number }[] }
  if (poolDialog.mode === 'even') {
    payload = { mode: 'even' }
  } else {
    const allocations = poolDialog.recipients
      .map((r) => ({ user_id: r.user_id, amount: parseFloat(r.amount) || 0 }))
      .filter((a) => a.amount > 0)
    if (allocations.length === 0) {
      appStore.showError(t('accountPool.pool.noAllocations'))
      return
    }
    if (poolCustomOver.value) {
      appStore.showError(t('accountPool.pool.overPool'))
      return
    }
    payload = { allocations }
  }
  poolDialog.distributing = true
  try {
    const view = await accountsAPI.distributeContributionPool(accountId, payload)
    poolAmounts.set(accountId, view.pool_amount || 0)
    appStore.showSuccess(t('accountPool.pool.distributed'))
    void loadContributionSummary()
    closePoolDialog()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.pool.distributeFailed')))
  } finally {
    poolDialog.distributing = false
  }
}

async function loadContributionSummary() {
  try {
    const summary = await userAPI.getContributionSummary()
    contributionSummary.contribution_quota = summary.contribution_quota || 0
    contributionSummary.contribution_frozen_quota = summary.contribution_frozen_quota || 0
    contributionSummary.contribution_history_quota = summary.contribution_history_quota || 0
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.rewards.loadFailed')))
  }
}

async function transferContribution() {
  if (contributionSummary.contribution_quota <= 0 || transferringContribution.value) return
  transferringContribution.value = true
  try {
    const result = await userAPI.transferContributionQuota()
    contributionSummary.contribution_quota = result.contribution_quota || 0
    contributionSummary.contribution_frozen_quota = result.contribution_frozen_quota || 0
    contributionSummary.contribution_history_quota = result.contribution_history_quota || 0
    appStore.showSuccess(t('accountPool.rewards.transferSuccess', { amount: formatMoney(result.transferred_quota) }))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.rewards.transferFailed')))
  } finally {
    transferringContribution.value = false
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
      proxy_id: createForm.proxyId,
      project_id: projectId || undefined,
      oauth_type: createForm.platform === 'gemini' ? createForm.oauthType : undefined,
      tier_id: createForm.tierId || undefined,
    })
    const credentials = buildCredentials(tokenInfo)
    if (Object.keys(credentials).length === 0) {
      appStore.showError(t('accountPool.oauth.credentialsMissing'))
      return
    }

    await accountsAPI.createOAuth(buildCreateOAuthPayload(createForm.name, tokenInfo, credentials))
    appStore.showSuccess(t('accountPool.createSuccess'))
    resetCreateForm()
    handleCreateClose()
    reloadFirstPage()
  } catch (err: unknown) {
    oauthError.value = extractApiErrorMessage(err, t('common.error'))
    appStore.showError(oauthError.value)
  } finally {
    creating.value = false
  }
}

const OPENAI_MOBILE_RT_CLIENT_ID = 'app_LlGpXReQgckcGGUo2JrYvtJK'

function splitTokenLines(input: string): string[] {
  return input
    .split('\n')
    .map((value) => value.trim())
    .filter((value) => value)
}

async function handleBatchRefreshTokenInput(refreshTokenInput: string, clientId?: string) {
  const refreshTokens = splitTokenLines(refreshTokenInput)
  if (refreshTokens.length === 0) {
    oauthError.value = t('admin.accounts.oauth.openai.pleaseEnterRefreshToken')
    return
  }

  creating.value = true
  oauthError.value = ''
  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await accountsAPI.refreshOpenAIToken(
          refreshTokens[i],
          createForm.proxyId,
          clientId,
        )
        const credentials = withModelMapping(buildOpenAICredentials(tokenInfo))
        if (clientId) {
          credentials.client_id = clientId
        }
        if (Object.keys(credentials).length === 0) {
          throw new Error(t('accountPool.oauth.credentialsMissing'))
        }
        const baseName = baseOpenAIAccountName(tokenInfo)
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName
        await accountsAPI.createOAuth(buildCreateOAuthPayload(accountName, tokenInfo, credentials))
        successCount++
      } catch (err: unknown) {
        failedCount++
        errors.push(`#${i + 1}: ${extractApiErrorMessage(err, t('common.error'))}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('accountPool.createSuccess')
      )
      resetCreateForm()
      handleCreateClose()
      reloadFirstPage()
      return
    }

    oauthError.value = errors.join('\n')
    if (successCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      reloadFirstPage()
    } else {
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    creating.value = false
  }
}

function handleValidateRefreshToken(refreshTokenInput: string) {
  return handleBatchRefreshTokenInput(refreshTokenInput)
}

function handleValidateMobileRefreshToken(refreshTokenInput: string) {
  return handleBatchRefreshTokenInput(refreshTokenInput, OPENAI_MOBILE_RT_CLIENT_ID)
}

function formatCodexImportMessages(messages?: CodexSessionImportMessage[]) {
  return (messages || [])
    .map((item) => {
      const name = item.name ? ` ${item.name}` : ''
      return `#${item.index}${name}: ${item.message}`
    })
    .join('\n')
}

async function handleImportCodexSession(content: string) {
  const trimmed = content.trim()
  if (!trimmed) {
    oauthError.value = t('admin.accounts.oauth.openai.codexSessionEmpty')
    return
  }

  creating.value = true
  oauthError.value = ''
  try {
    const credentialExtras: Record<string, unknown> = {}
    const modelMapping = buildUserModelWhitelist(allowedModels.value)
    if (Object.keys(modelMapping).length > 0) {
      credentialExtras.model_mapping = modelMapping
    }

    const result = await accountsAPI.importCodexSession({
      content: trimmed,
      name: createForm.name,
      proxy_id: createForm.proxyId,
      concurrency: normalizeConcurrency(createForm.concurrency),
      group_ids: createForm.groupIds,
      expires_at: createForm.expiresAt,
      auto_pause_on_expired: createForm.autoPauseOnExpired,
      credential_extras: Object.keys(credentialExtras).length > 0 ? credentialExtras : undefined,
      extra: createForm.codexCLIOnly ? { codex_cli_only: true } : undefined,
      update_existing: true,
      schedulable: createForm.schedulable,
      codex_cli_only: createForm.codexCLIOnly,
      ...buildContributionPayload(createForm),
      contribution_probe_failure_policy: createForm.probeFailurePolicy,
    })

    const successCount = result.created + result.updated
    const params = {
      created: result.created,
      updated: result.updated,
      skipped: result.skipped,
      failed: result.failed,
    }

    if (successCount > 0 && result.failed === 0) {
      appStore.showSuccess(t('admin.accounts.oauth.openai.codexSessionImportSuccess', params))
      resetCreateForm()
      handleCreateClose()
      reloadFirstPage()
      return
    }

    const errorText = formatCodexImportMessages(result.errors)
    const warningText = formatCodexImportMessages(result.warnings)
    oauthError.value = [errorText, warningText].filter(Boolean).join('\n')

    if (result.failed === 0) {
      appStore.showWarning(t('admin.accounts.oauth.openai.codexSessionImportSuccess', params))
      return
    }

    if (successCount > 0) {
      appStore.showWarning(t('admin.accounts.oauth.openai.codexSessionImportPartial', params))
      reloadFirstPage()
      return
    }

    appStore.showError(t('admin.accounts.oauth.openai.codexSessionImportFailed'))
  } catch (err: unknown) {
    oauthError.value = extractApiErrorMessage(err, t('admin.accounts.oauth.openai.codexSessionImportFailed'))
    appStore.showError(oauthError.value)
  } finally {
    creating.value = false
  }
}

function resetCreateForm() {
  createForm.name = ''
  createForm.oauthType = 'code_assist'
  createForm.projectId = ''
  createForm.tierId = ''
  createForm.schedulable = true
  createForm.proxyId = null
  createForm.concurrency = DEFAULT_USER_ACCOUNT_CONCURRENCY
  createForm.groupIds = []
  createForm.expiresAt = null
  createForm.autoPauseOnExpired = true
  createForm.codexCLIOnly = false
  createForm.fiveHourReservePercent = 0
  createForm.weeklyReservePercent = 0
  createForm.shareMode = 'percent'
  createForm.weeklyShareBudget = 0
  createForm.probeFailurePolicy = 'continue'
  resetModelWhitelist()
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

function openScopeDialog(account: UserAccountPoolItem) {
  scopeAccount.value = account
  scopeForm.groupIds = [...(account.group_ids || [])]
  scopeForm.proxyId = account.proxy_id || null
  scopeForm.concurrency = account.concurrency || DEFAULT_USER_ACCOUNT_CONCURRENCY
  scopeForm.expiresAt = account.expires_at ? Math.floor(new Date(account.expires_at).getTime() / 1000) : null
  scopeForm.autoPauseOnExpired = account.auto_pause_on_expired
  scopeForm.codexCLIOnly = account.codex_cli_only
  scopeForm.fiveHourReservePercent = account.contribution_5h_reserve_percent
  scopeForm.weeklyReservePercent = account.contribution_weekly_reserve_percent
  scopeForm.shareMode = account.contribution_share_mode || 'percent'
  scopeForm.weeklyShareBudget = account.contribution_weekly_share_budget || 0
  scopeForm.probeFailurePolicy = account.contribution_probe_failure_policy
  scopeAllowedModels.value = parseUserModelWhitelist(account.model_mapping)
  showScopeForm.value = true
}

function handleScopeClose() {
  showScopeForm.value = false
  scopeAccount.value = null
  resetScopeModelWhitelist()
  scopeForm.groupIds = []
  scopeForm.proxyId = null
  scopeForm.concurrency = DEFAULT_USER_ACCOUNT_CONCURRENCY
  scopeForm.expiresAt = null
  scopeForm.autoPauseOnExpired = true
  scopeForm.codexCLIOnly = false
  scopeForm.fiveHourReservePercent = 0
  scopeForm.weeklyReservePercent = 0
  scopeForm.shareMode = 'percent'
  scopeForm.weeklyShareBudget = 0
  scopeForm.probeFailurePolicy = 'continue'
}

async function saveScope() {
  const account = scopeAccount.value
  if (!account) return
  savingIDs.value.add(account.id)
  try {
    const updated = await accountsAPI.updateScope(account.id, {
      group_ids: scopeForm.groupIds,
      proxy_id: scopeForm.proxyId || 0,
      concurrency: normalizeConcurrency(scopeForm.concurrency),
      expires_at: scopeForm.expiresAt ?? 0,
      auto_pause_on_expired: scopeForm.autoPauseOnExpired,
      model_mapping: buildUserModelWhitelist(scopeAllowedModels.value),
      codex_cli_only: account.platform === 'openai' ? scopeForm.codexCLIOnly : false,
      ...buildContributionPayload(scopeForm),
      contribution_probe_failure_policy: scopeForm.probeFailurePolicy,
    })
    replaceAccount(updated)
    appStore.showSuccess(t('accountPool.scopeSaveSuccess'))
    handleScopeClose()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    savingIDs.value.delete(account.id)
  }
}

function openAccountMenu(account: UserAccountPoolItem, event: MouseEvent) {
  actionMenu.account = account
  const target = event.currentTarget as HTMLElement | null
  if (target) {
    const rect = target.getBoundingClientRect()
    const menuWidth = 208
    const menuHeight = account.is_mine ? 360 : 54
    const padding = 8
    const viewportWidth = window.innerWidth
    const viewportHeight = window.innerHeight
    let left = Math.max(padding, Math.min(rect.right - menuWidth, viewportWidth - menuWidth - padding))
    let top = rect.bottom + 4
    if (top + menuHeight > viewportHeight - padding) {
      top = Math.max(padding, rect.top - menuHeight - 4)
    }
    actionMenu.position = { top, left }
  } else {
    actionMenu.position = { top: event.clientY, left: event.clientX - 208 }
  }
  actionMenu.show = true
}

function openStatsModal(account: UserAccountPoolItem) {
  if (!account.is_mine) {
    appStore.showError(t('accountPool.readonly'))
    return
  }
  statsAccount.value = account
  showStats.value = true
}

function closeStatsModal() {
  showStats.value = false
  statsAccount.value = null
}

function userAccountTestEndpoint(id: number): string {
  return `/api/v1/accounts/${id}/test`
}

function openTestModal(account: UserAccountPoolItem) {
  if (!account.is_mine) {
    appStore.showError(t('accountPool.readonly'))
    return
  }
  testingAccount.value = {
    ...account,
    credentials: {},
  }
  showTest.value = true
}

function closeTestModal() {
  showTest.value = false
  testingAccount.value = null
}

async function openSchedulePanel(account: UserAccountPoolItem) {
  if (!account.is_mine) {
    appStore.showError(t('accountPool.readonly'))
    return
  }
  scheduleAccount.value = account
  scheduleModelOptions.value = []
  showSchedulePanel.value = true
  try {
    const models = await accountsAPI.getAvailableModels(account.id)
    scheduleModelOptions.value = models.map((model) => ({
      value: model.id,
      label: model.display_name || model.id,
    }))
  } catch {
    scheduleModelOptions.value = []
  }
}

function closeSchedulePanel() {
  showSchedulePanel.value = false
  scheduleAccount.value = null
  scheduleModelOptions.value = []
}

function openReAuthModal(account: UserAccountPoolItem) {
  if (!account.is_mine || account.platform !== 'openai') {
    appStore.showError(t('accountPool.readonly'))
    return
  }
  reAuthAccount.value = account
  resetReAuthSession()
  showReAuth.value = true
}

function closeReAuthModal() {
  showReAuth.value = false
  reAuthAccount.value = null
  resetReAuthSession()
}

async function handleReAuthGenerateUrl() {
  const account = reAuthAccount.value
  if (!account) return
  reAuthLoading.value = true
  reAuthError.value = ''
  try {
    const result = await accountsAPI.generateOAuthAuthUrl({
      platform: 'openai',
      proxy_id: account.proxy_id || null,
    })
    reAuthSession.authUrl = result.auth_url
    reAuthSession.sessionId = result.session_id
    reAuthSession.state = result.state || ''
  } catch (err: unknown) {
    reAuthError.value = extractApiErrorMessage(err, t('accountPool.oauth.startFailed'))
    appStore.showError(reAuthError.value)
  } finally {
    reAuthLoading.value = false
  }
}

async function handleReAuthExchangeCode() {
  const account = reAuthAccount.value
  if (!account || !reAuthSession.sessionId) return

  const authCode = reAuthFlowRef.value?.authCode || ''
  const state = reAuthFlowRef.value?.oauthState || reAuthSession.state
  if (!authCode.trim()) {
    appStore.showError(t('accountPool.oauth.codeRequired'))
    return
  }
  if (!state.trim()) {
    reAuthError.value = t('admin.accounts.oauth.authFailed')
    appStore.showError(reAuthError.value)
    return
  }

  reAuthLoading.value = true
  reAuthError.value = ''
  try {
    const tokenInfo = await accountsAPI.exchangeOAuthCode({
      platform: 'openai',
      session_id: reAuthSession.sessionId,
      code: authCode.trim(),
      state,
      proxy_id: account.proxy_id || null,
    })
    const credentials = buildOpenAICredentials(tokenInfo)
    if (Object.keys(credentials).length === 0) {
      appStore.showError(t('accountPool.oauth.credentialsMissing'))
      return
    }
    const updated = await accountsAPI.applyOAuthCredentials(account.id, {
      type: 'oauth',
      credentials,
      extra: buildOpenAIExtra(tokenInfo),
    })
    replaceAccount(updated)
    appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
    closeReAuthModal()
  } catch (err: unknown) {
    reAuthError.value = extractApiErrorMessage(err, t('admin.accounts.oauth.authFailed'))
    appStore.showError(reAuthError.value)
  } finally {
    reAuthLoading.value = false
  }
}

async function refreshAccountToken(account: UserAccountPoolItem) {
  if (!account.is_mine) return
  savingIDs.value.add(account.id)
  try {
    const updated = await accountsAPI.refreshCredentials(account.id)
    replaceAccount(updated)
    appStore.showSuccess(t('accountPool.actions.refreshTokenSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    savingIDs.value.delete(account.id)
  }
}

async function recoverAccountState(account: UserAccountPoolItem) {
  if (!account.is_mine) return
  savingIDs.value.add(account.id)
  try {
    const updated = await accountsAPI.recoverState(account.id)
    replaceAccount(updated)
    appStore.showSuccess(t('admin.accounts.recoverStateSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.accounts.recoverStateFailed')))
  } finally {
    savingIDs.value.delete(account.id)
  }
}

async function setAccountPrivacy(account: UserAccountPoolItem) {
  if (!account.is_mine) return
  savingIDs.value.add(account.id)
  try {
    const updated = await accountsAPI.setPrivacy(account.id)
    replaceAccount(updated)
    appStore.showSuccess(t('accountPool.actions.privacySetSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.accounts.privacyFailed')))
  } finally {
    savingIDs.value.delete(account.id)
  }
}

function openDeleteDialog(account: UserAccountPoolItem) {
  if (!account.is_mine) return
  deletingAccount.value = account
  showDeleteDialog.value = true
}

function closeDeleteDialog() {
  showDeleteDialog.value = false
  deletingAccount.value = null
}

async function confirmDelete() {
  const account = deletingAccount.value
  if (!account) return
  savingIDs.value.add(account.id)
  try {
    await accountsAPI.deleteAccount(account.id)
    appStore.showSuccess(t('accountPool.delete.success'))
    closeDeleteDialog()
    await loadAccounts()
    await loadContributionSummary()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('accountPool.delete.failed')))
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

watch(
  () => createForm.platform,
  () => {
    createForm.groupIds = createForm.groupIds.filter((groupID) =>
      availableGroupsForPlatform.value.some((group) => group.id === groupID)
    )
  }
)

function handleScroll() {
  actionMenu.show = false
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (columnsDropdownRef.value && !columnsDropdownRef.value.contains(target)) {
    showColumnsDropdown.value = false
  }
}

onMounted(() => {
  loadAccounts()
  loadContributionSummary()
  loadAvailableGroups()
  loadProxies()
  window.addEventListener('scroll', handleScroll, true)
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll, true)
  document.removeEventListener('click', handleClickOutside)
  remoteSessions.forEach((state) => clearRemoteTimer(state))
})
</script>
