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
                <th>{{ t('accountPool.columns.protection') }}</th>
                <th>{{ t('accountPool.columns.availability') }}</th>
                <th>{{ t('accountPool.columns.expiresAt') }}</th>
                <th>{{ t('accountPool.columns.actions') }}</th>
                <th>{{ t('accountPool.columns.updated') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && accounts.length === 0">
                <td colspan="12" class="text-center text-gray-500 dark:text-dark-300">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="accounts.length === 0">
                <td colspan="12" class="text-center text-gray-500 dark:text-dark-300">
                  {{ t('accountPool.empty') }}
                </td>
              </tr>
              <tr v-for="account in accounts" :key="account.id">
                <td>
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
                  <div class="min-w-36 space-y-1 text-sm">
                    <div>{{ t('accountPool.fields.used') }} {{ formatPercent(account.contribution_5h_usage_percent) }}</div>
                    <div v-if="account.window_cost_limit > 0" class="text-xs text-gray-500 dark:text-dark-400">
                      {{ formatMoney(account.current_window_cost || 0) }} / {{ formatMoney(account.window_cost_limit) }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('accountPool.fields.reserve') }} {{ formatPercent(account.contribution_5h_reserve_percent) }}
                    </div>
                  </div>
                </td>
                <td>
                  <div class="min-w-44 space-y-1 text-sm">
                    <div>{{ t('accountPool.fields.used') }} {{ formatPercent(account.contribution_weekly_usage_percent) }}</div>
                    <div v-if="account.quota_weekly_limit > 0" class="text-xs text-gray-500 dark:text-dark-400">
                      {{ formatMoney(account.quota_weekly_used) }} / {{ formatMoney(account.quota_weekly_limit) }}
                    </div>
                    <div v-if="account.quota_weekly_limit > 0" :class="account.weekly_remaining_below_policy ? 'text-xs text-amber-600 dark:text-amber-400' : 'text-xs text-gray-500 dark:text-dark-400'">
                      {{ t('accountPool.fields.remaining') }} {{ formatMoney(account.quota_weekly_remaining) }}
                      <span v-if="account.quota_weekly_min_remaining > 0">
                        · {{ t('accountPool.fields.reserve') }} {{ formatMoney(account.quota_weekly_min_remaining) }}
                      </span>
                    </div>
                    <div v-else class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('accountPool.fields.reserve') }} {{ formatPercent(account.contribution_weekly_reserve_percent) }}
                    </div>
                  </div>
                </td>
                <td>
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
                </td>
                <td>
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
                </td>
                <td>
                  <div class="min-w-40 text-sm">
                    <div>{{ formatOptionalUnixDate(account.expires_at) }}</div>
                    <div v-if="account.auto_pause_on_expired && account.expires_at" class="text-xs text-green-600 dark:text-green-400">
                      {{ t('accountPool.settings.autoPauseEnabled') }}
                    </div>
                    <div v-else class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('accountPool.settings.noExpiry') }}
                    </div>
                  </div>
                </td>
                <td>
                  <button
                    v-if="account.is_mine"
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="savingIDs.has(account.id)"
                    @click="openScopeDialog(account)"
                  >
                    <Icon name="edit" size="sm" />
                    <span>{{ t('accountPool.settings.edit') }}</span>
                  </button>
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
              <div class="mb-3 flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
                <button
                  type="button"
                  @click="modelRestrictionEnabled = false"
                  :class="restrictionModeClass(!modelRestrictionEnabled)"
                >
                  {{ t('accountPool.settings.allowAllModels') }}
                </button>
                <button
                  type="button"
                  @click="enableModelRestriction"
                  :class="restrictionModeClass(modelRestrictionEnabled)"
                >
                  {{ t('accountPool.settings.limitModels') }}
                </button>
              </div>
              <ModelWhitelistSelector
                v-if="modelRestrictionEnabled"
                v-model="allowedModels"
                :platform="createForm.platform"
              />
              <p class="input-hint">
                <template v-if="modelRestrictionEnabled">
                  {{ t('admin.accounts.selectedModels', { count: allowedModels.length }) }}
                  <span v-if="allowedModels.length === 0">{{ t('admin.accounts.supportsAllModels') }}</span>
                </template>
                <template v-else>
                  {{ t('accountPool.settings.allowAllModelsHint') }}
                </template>
              </p>
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

            <div class="grid gap-4 border-t border-gray-200 pt-4 dark:border-dark-600 lg:grid-cols-[1fr_auto] lg:items-start">
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

          <div class="grid gap-4 md:grid-cols-2">
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
              <div class="mb-3 flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
                <button
                  type="button"
                  @click="scopeModelRestrictionEnabled = false"
                  :class="restrictionModeClass(!scopeModelRestrictionEnabled)"
                >
                  {{ t('accountPool.settings.allowAllModels') }}
                </button>
                <button
                  type="button"
                  @click="enableScopeModelRestriction"
                  :class="restrictionModeClass(scopeModelRestrictionEnabled)"
                >
                  {{ t('accountPool.settings.limitModels') }}
                </button>
              </div>
              <ModelWhitelistSelector
                v-if="scopeModelRestrictionEnabled && scopeAccount"
                v-model="scopeAllowedModels"
                :platform="scopeAccount.platform"
              />
              <p class="input-hint">
                <template v-if="scopeModelRestrictionEnabled">
                  {{ t('admin.accounts.selectedModels', { count: scopeAllowedModels.length }) }}
                  <span v-if="scopeAllowedModels.length === 0">{{ t('admin.accounts.supportsAllModels') }}</span>
                </template>
                <template v-else>
                  {{ t('accountPool.settings.allowAllModelsHint') }}
                </template>
              </p>
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

            <div class="grid gap-4 border-t border-gray-200 pt-4 dark:border-dark-600 lg:grid-cols-[1fr_auto] lg:items-start">
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

          <div class="grid gap-4 md:grid-cols-2">
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import OAuthAuthorizationFlow from '@/components/account/OAuthAuthorizationFlow.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import accountsAPI, {
  type ContributionProbeFailurePolicy,
  type UserOAuthAuthUrlRequest,
  type UserOAuthTokenInfo,
} from '@/api/accounts'
import { userGroupsAPI } from '@/api/groups'
import type { AccountPlatform, Group, UserAccountPoolItem } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildModelMappingObject, getModelsByPlatform } from '@/composables/useModelWhitelist'
import { formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'

interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  reset: () => void
}

const { t } = useI18n()
const appStore = useAppStore()

const platforms: AccountPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity']
const accounts = ref<UserAccountPoolItem[]>([])
const availableGroups = ref<Group[]>([])
const loading = ref(false)
const creating = ref(false)
const oauthLoading = ref(false)
const oauthError = ref('')
const showCreateForm = ref(false)
const createStep = ref(1)
const searchQuery = ref('')
const platformFilter = ref<AccountPlatform | ''>('')
const savingIDs = ref(new Set<number>())
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
  groupIds: [] as number[],
  expiresAt: null as number | null,
  autoPauseOnExpired: true,
  codexCLIOnly: false,
  fiveHourReservePercent: 0,
  weeklyReservePercent: 0,
  probeFailurePolicy: 'continue' as ContributionProbeFailurePolicy,
})

const modelRestrictionEnabled = ref(false)
const allowedModels = ref<string[]>([])
const showScopeForm = ref(false)
const scopeAccount = ref<UserAccountPoolItem | null>(null)
const scopeModelRestrictionEnabled = ref(false)
const scopeAllowedModels = ref<string[]>([])
const scopeForm = reactive({
  groupIds: [] as number[],
  expiresAt: null as number | null,
  autoPauseOnExpired: true,
  codexCLIOnly: false,
  fiveHourReservePercent: 0,
  weeklyReservePercent: 0,
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

function modelMappingToAllowedModels(mapping: Record<string, string> | null | undefined): string[] {
  return Object.entries(mapping || {})
    .filter(([from, to]) => from.trim() && from.trim() === String(to).trim())
    .map(([from]) => from.trim())
}

function buildWhitelistModelMapping(models: string[]): Record<string, string> {
  const mapping = buildModelMappingObject('whitelist', models, [])
  return mapping || {}
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

function compactRecord(input: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(input).filter(([, value]) => value !== undefined && value !== null && value !== '')
  )
}

function withModelMapping(credentials: Record<string, unknown>): Record<string, unknown> {
  if (!modelRestrictionEnabled.value) return credentials
  const modelMapping = buildModelMappingObject('whitelist', allowedModels.value, [])
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
  if (modelRestrictionEnabled.value) {
    allowedModels.value = [...getModelsByPlatform(platform)]
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

function restrictionModeClass(active: boolean) {
  return [
    'flex-1 rounded-md px-3 py-2 text-sm font-medium transition-all',
    active
      ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-600 dark:text-primary-400'
      : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200',
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

function enableModelRestriction() {
  modelRestrictionEnabled.value = true
  if (allowedModels.value.length === 0) {
    allowedModels.value = [...getModelsByPlatform(createForm.platform)]
  }
}

function enableScopeModelRestriction() {
  scopeModelRestrictionEnabled.value = true
  if (scopeAllowedModels.value.length === 0 && scopeAccount.value) {
    scopeAllowedModels.value = [...getModelsByPlatform(scopeAccount.value.platform)]
  }
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

async function handleGenerateUrl() {
  oauthLoading.value = true
  oauthError.value = ''
  try {
    const projectId = oauthFlowRef.value?.projectId || createForm.projectId
    const payload: UserOAuthAuthUrlRequest = {
      platform: createForm.platform,
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
      search: searchQuery.value.trim(),
      sort_by: 'name',
      sort_order: 'asc',
    })
    accounts.value = result.items
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
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
      group_ids: createForm.groupIds,
      expires_at: createForm.expiresAt,
      auto_pause_on_expired: createForm.autoPauseOnExpired,
      contribution_5h_reserve_percent: normalizeReservePercent(createForm.fiveHourReservePercent),
      contribution_weekly_reserve_percent: normalizeReservePercent(createForm.weeklyReservePercent),
      contribution_probe_failure_policy: createForm.probeFailurePolicy,
    })
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

function resetCreateForm() {
  createForm.name = ''
  createForm.oauthType = 'code_assist'
  createForm.projectId = ''
  createForm.tierId = ''
  createForm.schedulable = true
  createForm.groupIds = []
  createForm.expiresAt = null
  createForm.autoPauseOnExpired = true
  createForm.codexCLIOnly = false
  createForm.fiveHourReservePercent = 0
  createForm.weeklyReservePercent = 0
  createForm.probeFailurePolicy = 'continue'
  modelRestrictionEnabled.value = false
  allowedModels.value = []
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
  scopeForm.expiresAt = account.expires_at ? Math.floor(new Date(account.expires_at).getTime() / 1000) : null
  scopeForm.autoPauseOnExpired = account.auto_pause_on_expired
  scopeForm.codexCLIOnly = account.codex_cli_only
  scopeForm.fiveHourReservePercent = account.contribution_5h_reserve_percent
  scopeForm.weeklyReservePercent = account.contribution_weekly_reserve_percent
  scopeForm.probeFailurePolicy = account.contribution_probe_failure_policy
  scopeAllowedModels.value = modelMappingToAllowedModels(account.model_mapping)
  scopeModelRestrictionEnabled.value = scopeAllowedModels.value.length > 0
  showScopeForm.value = true
}

function handleScopeClose() {
  showScopeForm.value = false
  scopeAccount.value = null
  scopeModelRestrictionEnabled.value = false
  scopeAllowedModels.value = []
  scopeForm.groupIds = []
  scopeForm.expiresAt = null
  scopeForm.autoPauseOnExpired = true
  scopeForm.codexCLIOnly = false
  scopeForm.fiveHourReservePercent = 0
  scopeForm.weeklyReservePercent = 0
  scopeForm.probeFailurePolicy = 'continue'
}

async function saveScope() {
  const account = scopeAccount.value
  if (!account) return
  savingIDs.value.add(account.id)
  try {
    const updated = await accountsAPI.updateScope(account.id, {
      group_ids: scopeForm.groupIds,
      expires_at: scopeForm.expiresAt ?? 0,
      auto_pause_on_expired: scopeForm.autoPauseOnExpired,
      model_mapping: scopeModelRestrictionEnabled.value ? buildWhitelistModelMapping(scopeAllowedModels.value) : {},
      codex_cli_only: account.platform === 'openai' ? scopeForm.codexCLIOnly : false,
      contribution_5h_reserve_percent: normalizeReservePercent(scopeForm.fiveHourReservePercent),
      contribution_weekly_reserve_percent: normalizeReservePercent(scopeForm.weeklyReservePercent),
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

onMounted(() => {
  loadAccounts()
  loadAvailableGroups()
})
</script>
