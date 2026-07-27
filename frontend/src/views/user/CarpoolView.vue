<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-5">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
            <Icon name="users" size="lg" />
          </div>
          <div class="min-w-0">
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('carpool.title') }}</h1>
            <p class="mt-0.5 text-sm text-gray-500 dark:text-dark-300">{{ t('carpool.description') }}</p>
          </div>
        </div>
        <button type="button" class="btn btn-primary shrink-0" @click="openCreateDialog">
          <Icon name="plus" size="sm" />
          <span>{{ t('carpool.create') }}</span>
        </button>
      </header>

      <section
        data-testid="gpt-carpool-rules"
        class="overflow-hidden rounded-lg border border-amber-200 bg-amber-50/70 dark:border-amber-900/70 dark:bg-amber-950/20"
      >
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-amber-200 px-4 py-3 dark:border-amber-900/70">
          <div class="flex items-center gap-2">
            <Icon name="book" size="sm" class="text-amber-700 dark:text-amber-400" />
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('carpool.rules.title') }}</h2>
          </div>
          <span class="inline-flex items-center gap-1.5 text-xs font-medium text-amber-800 dark:text-amber-300">
            <Icon name="refresh" size="xs" />
            {{ t('carpool.rules.weeklyBadge') }}
          </span>
        </div>

        <div class="grid gap-x-6 px-4 py-3 sm:grid-cols-2 lg:grid-cols-4">
          <div v-for="item in ruleItems" :key="item.label" class="py-1.5">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ item.label }}</h3>
            <p class="mt-1 text-xs leading-5 text-gray-600 dark:text-dark-200">{{ item.text }}</p>
          </div>
        </div>

        <div class="space-y-1 border-t border-amber-200 px-4 py-2.5 text-xs leading-5 text-amber-900 dark:border-amber-900/70 dark:text-amber-200">
          <p>{{ t('carpool.notices.weeklyRefresh') }}</p>
          <p>{{ t('carpool.notices.consumeOrder') }}</p>
          <p>{{ t('carpool.notices.customRule') }}</p>
        </div>
      </section>

      <!--
        管理员待启动列表。确认发车的通知过去只有一封邮件，邮件一丢车就连人带钱
        无限期挂起，后台连个"等我启动"的清单都没有。这里把 confirmed 的车全部
        列出来，并标出超过 24 小时承诺的。
      -->
      <section
        v-if="authStore.isAdmin && pendingLaunches.length > 0"
        data-testid="carpool-pending-launch"
        class="overflow-hidden rounded-lg border border-blue-200 bg-blue-50/70 dark:border-blue-900/70 dark:bg-blue-950/20"
      >
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-blue-200 px-4 py-3 dark:border-blue-900/70">
          <div class="flex items-center gap-2">
            <Icon name="clock" size="sm" class="text-blue-700 dark:text-blue-400" />
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('carpool.pendingLaunch.title', { count: pendingLaunches.length }) }}
            </h2>
          </div>
          <span v-if="overduePendingCount > 0" class="badge badge-warning">
            {{ t('carpool.pendingLaunch.overdueBadge', { count: overduePendingCount }) }}
          </span>
        </div>
        <ul class="divide-y divide-blue-200 dark:divide-blue-900/70">
          <li
            v-for="item in pendingLaunches"
            :key="item.carpoolId"
            class="flex flex-wrap items-center justify-between gap-3 px-4 py-2.5"
          >
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.name }}</span>
                <span v-if="item.overdue" class="badge badge-warning shrink-0">
                  {{ t('carpool.pendingLaunch.overdue') }}
                </span>
              </div>
              <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-300">
                {{ t('carpool.pendingLaunch.summary', {
                  members: item.memberCount,
                  total: formatUsd(item.declaredTotalUsd),
                  hours: formatDecimal(item.pendingHours),
                }) }}
                <span v-if="item.ownerEmail"> · {{ item.ownerEmail }}</span>
              </div>
            </div>
            <button
              type="button"
              class="btn btn-primary h-8 shrink-0 px-3 py-1.5"
              :disabled="actionPending"
              @click="requestLaunchFromPending(item)"
            >
              <Icon name="play" size="sm" />
              <span>{{ t('carpool.actions.launch') }}</span>
            </button>
          </li>
        </ul>
      </section>

      <section class="grid grid-cols-2 border-y border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800 sm:grid-cols-4">
        <div v-for="stat in stats" :key="stat.label" class="border-gray-200 px-4 py-3 odd:border-r dark:border-dark-700 sm:border-r sm:last:border-r-0">
          <div class="text-xs text-gray-500 dark:text-dark-300">{{ stat.label }}</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ stat.value }}</div>
        </div>
      </section>

      <section class="space-y-4">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div class="inline-flex h-10 w-fit rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              class="min-w-28 rounded-md px-4 text-sm font-medium transition-colors"
              :class="activeTab === tab.value ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-dark-300 dark:hover:text-white'"
              @click="activeTab = tab.value"
            >
              {{ tab.label }}
            </button>
          </div>

          <div class="flex flex-col gap-2 sm:flex-row">
            <div class="relative sm:w-72">
              <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model.trim="searchQuery" type="search" class="input h-10 pl-9" :placeholder="t('carpool.searchPlaceholder')" />
            </div>
            <select v-model="statusFilter" class="input h-10 sm:w-40">
              <option value="">{{ t('carpool.allStatuses') }}</option>
              <option value="recruiting">{{ t('carpool.status.recruiting') }}</option>
              <option value="confirmed">{{ t('carpool.status.confirmed') }}</option>
              <option value="active">{{ t('carpool.status.active') }}</option>
              <option value="cancelled">{{ t('carpool.status.cancelled') }}</option>
            </select>
          </div>
        </div>

        <div v-if="loading" class="flex min-h-64 items-center justify-center border-y border-gray-200 dark:border-dark-700">
          <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
        </div>

        <div v-else-if="filteredCarpools.length" class="grid gap-4 lg:grid-cols-2">
          <article
            v-for="carpool in filteredCarpools"
            :key="carpool.id"
            class="flex min-h-[292px] flex-col rounded-lg border border-gray-200 bg-white p-5 shadow-sm transition-colors hover:border-gray-300 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-dark-600"
          >
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ carpool.name }}</h2>
                  <span :class="['badge', statusBadgeClass(carpool)]">{{ statusLabel(carpool) }}</span>
                  <span v-if="carpool.memberRole" class="badge badge-primary">{{ t(`carpool.roles.${carpool.memberRole}`) }}</span>
                </div>
                <p class="mt-1 line-clamp-2 min-h-10 text-sm text-gray-500 dark:text-dark-300">{{ carpool.description }}</p>
              </div>
              <!-- 自定义规则车的 weekly_limit_usd 是迁移填的默认值，对它没有意义 -->
              <span
                v-if="isQuotaCar(carpool)"
                class="shrink-0 rounded-md border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 dark:border-dark-600 dark:text-dark-200"
              >
                {{ t('carpool.fields.weeklyLimitBadge', { limit: formatUsd(carpool.weeklyLimitUsd) }) }}
              </span>
              <span
                v-else
                class="shrink-0 rounded-md border border-slate-300 px-2 py-1 text-xs font-medium text-slate-600 dark:border-slate-600 dark:text-slate-300"
              >
                {{ t('carpool.customRule.badge') }}
              </span>
            </div>

            <!--
              自定义规则车（含平台升级前建立的老车）不走申报制：额度进度、
              剩余可预约、Plus 等价、均价对它们都不成立，硬渲染只会显示
              "0 / 2400"、"均价 ¥0" 这种误导数字。改为直接展示规则说明。
            -->
            <div
              v-if="!isQuotaCar(carpool)"
              data-testid="carpool-custom-rule"
              class="mt-4 rounded-md bg-slate-50 px-3 py-2 text-xs leading-5 text-slate-700 dark:bg-slate-900/30 dark:text-slate-300"
            >
              <div class="font-medium">{{ t('carpool.customRule.badge') }}</div>
              <p v-if="carpool.ruleNote" class="mt-1">{{ carpool.ruleNote }}</p>
              <p v-else class="mt-1">{{ t('carpool.customRule.noNote') }}</p>
            </div>

            <div v-else class="mt-4">
              <div class="mb-2 flex items-center justify-between text-xs">
                <span class="font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.fields.quotaProgress') }}</span>
                <span class="text-gray-500 dark:text-dark-300">
                  {{ t('carpool.fields.declaredOf', { declared: formatUsd(carpool.declaredTotalUsd), limit: formatUsd(carpool.weeklyLimitUsd) }) }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                <div
                  class="h-full rounded-full transition-all"
                  :class="quotaProgressClass(carpool)"
                  :style="{ width: `${quotaProgress(carpool)}%` }"
                />
              </div>
              <div class="relative h-3">
                <span
                  class="absolute top-0 h-2 w-px bg-amber-500"
                  :style="{ left: `${launchLinePercent(carpool)}%` }"
                  :title="t('carpool.fields.launchLine', { ratio: launchRatioPercent(carpool.launchMinRatio) })"
                />
                <span
                  class="absolute top-2 -translate-x-1/2 whitespace-nowrap text-[10px] leading-3 text-amber-600 dark:text-amber-400"
                  :style="{ left: `${launchLinePercent(carpool)}%` }"
                >
                  {{ t('carpool.fields.launchLine', { ratio: launchRatioPercent(carpool.launchMinRatio) }) }}
                </span>
              </div>
            </div>

            <dl class="mt-4 grid grid-cols-3 gap-x-4 gap-y-3 text-sm">
              <template v-if="isQuotaCar(carpool)">
                <div>
                  <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.remainingJoinable') }}</dt>
                  <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">{{ formatUsd(carpool.remainingJoinableUsd) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.effectiveRate') }}</dt>
                  <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">
                    {{ formatRate(carEffectiveRate(carpool)) }}
                    <span class="block text-[10px] font-normal text-gray-400 dark:text-dark-400">
                      {{ t('carpool.fields.effectiveRateHint', { usd: formatDecimal(carUsdPerCny(carpool)) }) }}
                    </span>
                  </dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.carMonthlyFee') }}</dt>
                  <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">
                    {{ formatCny(carpool.seatFeeCny + carpool.usagePoolCny) }}
                    <span class="block text-[10px] font-normal text-gray-400 dark:text-dark-400">
                      {{ t('carpool.fields.carMonthlyFeeUnit', { seat: formatCny(carpool.seatFeeCny), pool: formatCny(carpool.usagePoolCny) }) }}
                    </span>
                  </dd>
                </div>
              </template>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.organizer') }}</dt>
                <dd class="mt-1 truncate font-medium text-gray-700 dark:text-dark-100">{{ carpool.organizer }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.scheduledStart') }}</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">{{ formatDate(carpool.scheduledStartAt) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.detailDialog.linkedGroup') }}</dt>
                <dd class="mt-1 truncate font-medium text-gray-700 dark:text-dark-100">{{ carpool.groupName || t('carpool.detailDialog.pendingGroup') }}</dd>
              </div>
            </dl>

            <div
              v-if="carpool.hasGroupQrCode"
              class="mt-4 flex items-center gap-3 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-700/40"
            >
              <img
                v-if="qrCodeUrls[carpool.id]"
                :src="qrCodeUrls[carpool.id]"
                :alt="t('carpool.wechat.scanToJoin')"
                class="h-14 w-14 shrink-0 rounded-md border border-gray-200 object-cover dark:border-dark-600"
              />
              <div class="min-w-0 text-xs">
                <div class="font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.wechat.scanToJoin') }}</div>
                <button
                  type="button"
                  class="mt-1 inline-flex items-center gap-1 text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                  :title="t('common.copy')"
                  @click="copyAdminWechat(carpool.adminWechat)"
                >
                  <Icon name="copy" size="xs" />
                  <span>{{ t('carpool.wechat.adminLabel') }}: {{ carpool.adminWechat || ADMIN_WECHAT }}</span>
                </button>
              </div>
            </div>

            <div class="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
              <div class="flex flex-wrap items-center gap-2">
                <button type="button" class="btn btn-secondary h-9 px-3 py-2" @click="openDetails(carpool)">
                  <Icon name="eye" size="sm" />
                  <span>{{ t('carpool.actions.details') }}</span>
                </button>
                <button
                  v-if="canInvite(carpool)"
                  type="button"
                  class="btn btn-secondary h-9 px-3 py-2"
                  @click="openInvite(carpool)"
                >
                  <Icon name="link" size="sm" />
                  <span>{{ t('carpool.actions.invite') }}</span>
                </button>
                <button
                  v-if="canConfirm(carpool)"
                  type="button"
                  :data-testid="`confirm-${carpool.id}`"
                  class="h-9 px-3 py-2"
                  :class="launchReady(carpool) ? 'btn btn-primary' : 'btn btn-secondary'"
                  :disabled="!launchReady(carpool)"
                  :title="launchHint(carpool)"
                  @click="requestConfirm(carpool)"
                >
                  <Icon name="checkCircle" size="sm" />
                  <span>{{ t('carpool.actions.confirm') }}</span>
                </button>
                <span v-if="canConfirm(carpool) && !launchReady(carpool)" class="text-xs text-gray-500 dark:text-dark-300">{{ launchHint(carpool) }}</span>
                <button
                  v-if="canAdminLaunch(carpool)"
                  type="button"
                  :data-testid="`launch-${carpool.id}`"
                  class="btn btn-primary h-9 px-3 py-2"
                  @click="requestLaunch(carpool)"
                >
                  <Icon name="play" size="sm" />
                  <span>{{ t('carpool.actions.launch') }}</span>
                </button>
                <button
                  v-if="canForceLaunch(carpool)"
                  type="button"
                  :data-testid="`force-launch-${carpool.id}`"
                  class="btn btn-secondary h-9 px-3 py-2"
                  :title="t('carpool.launchDialog.forceReady')"
                  @click="requestForceLaunch(carpool)"
                >
                  <Icon name="play" size="sm" />
                  <span>{{ t('carpool.actions.forceLaunch') }}</span>
                </button>
                <button
                  v-if="canLeave(carpool)"
                  type="button"
                  :data-testid="`leave-${carpool.id}`"
                  class="btn btn-ghost h-9 px-3 py-2 text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                  @click="requestLeave(carpool)"
                >
                  <Icon name="xCircle" size="sm" />
                  <span>{{ t('carpool.actions.leave') }}</span>
                </button>
                <button
                  v-if="canViewSettlement(carpool)"
                  type="button"
                  class="btn btn-secondary h-9 px-3 py-2"
                  @click="openSettlement(carpool)"
                >
                  <Icon name="document" size="sm" />
                  <span>{{ t('carpool.actions.settlement') }}</span>
                </button>
                <button
                  v-if="authStore.isAdmin && carpool.status === 'recruiting'"
                  type="button"
                  class="btn btn-secondary h-9 px-3 py-2"
                  @click="toggleJoinLock(carpool)"
                >
                  <Icon name="lock" size="sm" />
                  <span>{{ carpool.joinLocked ? t('carpool.actions.unlock') : t('carpool.actions.lock') }}</span>
                </button>
              </div>

              <!-- 撤回确认：confirmed 的车在等 admin 启动时唯一的温和出口 -->
              <button
                v-if="canUnconfirm(carpool)"
                type="button"
                class="btn btn-ghost h-9 px-3 py-2 text-amber-600 hover:bg-amber-50 dark:text-amber-400 dark:hover:bg-amber-900/20"
                @click="requestUnconfirm(carpool)"
              >
                <Icon name="refresh" size="sm" />
                <span>{{ t('carpool.actions.unconfirm') }}</span>
              </button>
              <button
                v-if="canCancel(carpool)"
                type="button"
                class="btn btn-ghost h-9 px-3 py-2 text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                @click="requestCancel(carpool)"
              >
                <Icon name="trash" size="sm" />
                <span>{{ t('carpool.actions.cancel') }}</span>
              </button>
              <button
                v-else-if="!carpool.memberRole && carpool.visibility === 'public' && canJoin(carpool)"
                type="button"
                class="btn btn-primary h-9 px-4 py-2"
                @click="openJoin(carpool)"
              >
                <Icon name="userPlus" size="sm" />
                <span>{{ t('carpool.actions.join') }}</span>
              </button>
              <span v-else-if="carpool.memberRole" class="inline-flex h-9 items-center gap-1.5 text-sm font-medium text-emerald-600 dark:text-emerald-400">
                <Icon name="checkCircle" size="sm" />
                {{ t('carpool.actions.joined') }}
              </span>
            </div>
          </article>
        </div>

        <div v-else class="flex min-h-64 flex-col items-center justify-center border-y border-gray-200 py-12 text-center dark:border-dark-700">
          <Icon name="users" size="xl" class="text-gray-300 dark:text-dark-500" />
          <h2 class="mt-3 text-base font-semibold text-gray-900 dark:text-white">{{ t('carpool.empty.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ t('carpool.empty.description') }}</p>
        </div>
      </section>
    </div>

    <BaseDialog :show="createDialogOpen" :title="t('carpool.createDialog.title')" width="normal" @close="createDialogOpen = false">
      <form id="carpool-create-form" class="space-y-4" @submit.prevent="createCarpool">
        <fieldset>
          <legend class="input-label">{{ t('carpool.createDialog.ruleMode') }}</legend>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="mode in ruleModeOptions"
              :key="mode.value"
              type="button"
              :data-testid="`rule-mode-${mode.value}`"
              class="flex h-10 items-center justify-center gap-2 rounded-lg border text-sm font-medium transition-colors"
              :class="createRuleMode === mode.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:text-dark-200'"
              @click="createRuleMode = mode.value"
            >
              <Icon :name="mode.value === 'default' ? 'checkCircle' : 'users'" size="sm" />
              {{ mode.label }}
            </button>
          </div>
        </fieldset>

        <div
          v-if="createRuleMode === 'custom'"
          data-testid="custom-rule-panel"
          class="rounded-lg border border-amber-200 bg-amber-50/70 px-3 py-3 dark:border-amber-900/70 dark:bg-amber-950/20"
        >
          <div class="text-xs font-medium text-amber-800 dark:text-amber-300">{{ t('carpool.createDialog.customRule.title') }}</div>
          <p class="mt-1 text-xs leading-5 text-amber-900/80 dark:text-amber-200/80">{{ t('carpool.createDialog.customRule.description') }}</p>
          <div class="mt-3">
            <button
              type="button"
              data-testid="custom-rule-notify"
              class="btn btn-primary h-9 px-3 py-2"
              :disabled="customRuleNotifyPending"
              @click="notifyCustomRule"
            >
              <Icon name="bell" size="sm" />
              <span>{{ t('carpool.createDialog.customRule.notify') }}</span>
            </button>
          </div>
          <div
            v-if="customRuleNotified"
            class="mt-3 flex items-center justify-between gap-2 rounded-md bg-white/70 px-2.5 py-2 dark:bg-dark-700/40"
          >
            <span class="text-sm text-gray-700 dark:text-dark-100">
              {{ t('carpool.wechat.adminLabel') }}: <span class="font-mono font-medium">{{ ADMIN_WECHAT }}</span>
            </span>
            <button type="button" class="btn btn-secondary h-7 px-2 py-1 text-xs" @click="copyAdminWechat(ADMIN_WECHAT)">
              <Icon name="copy" size="xs" />
              <span>{{ t('common.copy') }}</span>
            </button>
          </div>
        </div>

        <div>
          <label class="input-label" for="carpool-name">{{ t('carpool.fields.name') }}</label>
          <input id="carpool-name" v-model.trim="createForm.name" class="input" maxlength="100" required :placeholder="t('carpool.fields.namePlaceholder')" :disabled="createFieldsDisabled" />
        </div>
        <div>
          <label class="input-label" for="carpool-description">{{ t('carpool.fields.description') }}</label>
          <textarea id="carpool-description" v-model.trim="createForm.description" class="input min-h-20 resize-y" maxlength="300" :placeholder="t('carpool.fields.descriptionPlaceholder')" :disabled="createFieldsDisabled" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label" for="carpool-start">{{ t('carpool.fields.scheduledStart') }}</label>
            <input id="carpool-start" v-model="createForm.scheduledStartAt" type="date" class="input" required :disabled="createFieldsDisabled" />
          </div>
          <div>
            <label class="input-label" for="carpool-owner-quota">{{ t('carpool.createDialog.ownerQuota') }}</label>
            <input id="carpool-owner-quota" v-model.number="createForm.ownerQuota" type="number" min="0" step="1" class="input" :disabled="createFieldsDisabled" />
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.createDialog.ownerQuotaHint') }}</p>
          </div>
        </div>
        <fieldset>
          <legend class="input-label">{{ t('carpool.fields.visibility') }}</legend>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="visibility in visibilityOptions"
              :key="visibility.value"
              type="button"
              class="flex h-10 items-center justify-center gap-2 rounded-lg border text-sm font-medium transition-colors"
              :class="createForm.visibility === visibility.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:text-dark-200'"
              :disabled="createFieldsDisabled"
              @click="createForm.visibility = visibility.value"
            >
              <Icon :name="visibility.value === 'public' ? 'globe' : 'lock'" size="sm" />
              {{ visibility.label }}
            </button>
          </div>
        </fieldset>
        <div class="rounded-lg border border-gray-200 px-3 py-3 dark:border-dark-600">
          <div class="text-xs font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.createDialog.contactTitle') }}</div>
          <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-300">{{ t('carpool.createDialog.contactHint') }}</p>
          <div class="mt-2 flex items-center justify-between gap-2 rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-700/40">
            <span class="text-sm text-gray-700 dark:text-dark-100">
              {{ t('carpool.wechat.adminLabel') }}: <span class="font-mono font-medium">{{ ADMIN_WECHAT }}</span>
            </span>
            <button type="button" class="btn btn-secondary h-7 px-2 py-1 text-xs" @click="copyAdminWechat(ADMIN_WECHAT)">
              <Icon name="copy" size="xs" />
              <span>{{ t('common.copy') }}</span>
            </button>
          </div>
          <label class="mt-3 flex items-center gap-2">
            <input
              id="carpool-added-admin"
              v-model="createForm.addedAdminWechat"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :disabled="createFieldsDisabled"
            />
            <span class="text-sm text-gray-700 dark:text-dark-200">{{ t('carpool.createDialog.addedAdmin', { wechat: ADMIN_WECHAT }) }}</span>
          </label>
          <div class="mt-3">
            <label class="input-label" for="carpool-group-qr">{{ t('carpool.createDialog.qrLabel') }}</label>
            <div class="flex items-start gap-3">
              <div class="min-w-0 flex-1">
                <input
                  id="carpool-group-qr"
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  class="block w-full text-sm text-gray-500 file:mr-3 file:rounded-md file:border-0 file:bg-primary-50 file:px-3 file:py-2 file:text-sm file:font-medium file:text-primary-700 hover:file:bg-primary-100 dark:text-dark-300 dark:file:bg-primary-900/20 dark:file:text-primary-300"
                  :disabled="createFieldsDisabled"
                  @change="handleQrFileChange"
                />
                <p class="mt-1 text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.createDialog.qrHint') }}</p>
                <p v-if="createQrError" class="mt-1 text-xs font-medium text-red-600 dark:text-red-400">{{ createQrError }}</p>
              </div>
              <img
                v-if="createForm.groupQrCode"
                :src="createForm.groupQrCode"
                :alt="t('carpool.createDialog.qrLabel')"
                class="h-16 w-16 shrink-0 rounded-md border border-gray-200 object-cover dark:border-dark-600"
              />
            </div>
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="createDialogOpen = false">{{ t('common.cancel') }}</button>
          <button v-if="createRuleMode === 'default'" type="submit" form="carpool-create-form" class="btn btn-primary" :disabled="!createFormValid || actionPending">
            <Icon name="plus" size="sm" />
            {{ t('carpool.createDialog.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="joinDialogOpen" :title="t('carpool.joinDialog.title')" width="normal" @close="joinDialogOpen = false">
      <div v-if="joinTarget" class="space-y-4">
        <div class="flex items-center justify-between gap-3 border-b border-gray-100 pb-3 dark:border-dark-700">
          <div>
            <div class="font-semibold text-gray-900 dark:text-white">{{ joinTarget.name }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">
              {{ t('carpool.fields.members', { count: joinTarget.memberCount }) }}
              <span class="text-gray-400 dark:text-dark-400">
                · {{ t('carpool.joinDialog.seatShareHint', {
                  total: formatCny(joinTarget.seatFeeCny),
                  people: joinHeadcount,
                  each: formatCny(joinSeatShare),
                }) }}
              </span>
            </div>
          </div>
          <span :class="['badge', statusBadgeClass(joinTarget)]">{{ statusLabel(joinTarget) }}</span>
        </div>

        <div>
          <label class="input-label" for="carpool-join-quota">{{ t('carpool.joinDialog.quotaLabel') }}</label>
          <input id="carpool-join-quota" v-model.number="joinForm.declaredQuota" type="number" min="1" step="1" class="input" required />
          <p v-if="recommendationLoading" class="mt-1 text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.joinDialog.recommendationLoading') }}</p>
          <p v-else-if="recommendation" class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ recommendation.message }}</p>
          <p v-else-if="recommendationFailed" class="mt-1 text-xs text-amber-600 dark:text-amber-400">{{ t('carpool.joinDialog.recommendationFailed') }}</p>
        </div>

        <!-- 车上还有谁、各自报了多少：申报额度是在跟这些人分同一个池子，
             看不到别人的申报就只能凭空猜自己该报多少。 -->
        <div class="rounded-lg border border-gray-200 px-3 py-3 dark:border-dark-600">
          <div class="flex items-center justify-between gap-2 text-xs">
            <span class="font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.joinDialog.rosterTitle') }}</span>
            <span v-if="!joinRosterLoading && !joinRosterFailed" class="shrink-0 text-gray-400 dark:text-dark-400">
              {{ t('carpool.joinDialog.rosterSummary', { count: joinRoster.length, total: formatUsd(joinRosterTotal) }) }}
            </span>
          </div>
          <p v-if="joinRosterLoading" class="mt-2 text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.joinDialog.rosterLoading') }}</p>
          <p v-else-if="joinRosterFailed" class="mt-2 text-xs text-amber-600 dark:text-amber-400">{{ t('carpool.joinDialog.rosterFailed') }}</p>
          <p v-else-if="joinRoster.length === 0" class="mt-2 text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.joinDialog.rosterEmpty') }}</p>
          <ul v-else class="mt-2 space-y-1" data-testid="carpool-join-roster">
            <li v-for="member in joinRoster" :key="member.userId" class="flex items-center justify-between gap-2 text-xs">
              <span class="flex min-w-0 items-center gap-1.5">
                <span class="truncate text-gray-700 dark:text-dark-200">
                  {{ member.username || t('carpool.joinDialog.rosterAnonymous', { id: member.userId }) }}
                </span>
                <span
                  v-if="member.role === 'owner'"
                  class="shrink-0 rounded bg-gray-100 px-1 text-[10px] text-gray-500 dark:bg-dark-700 dark:text-dark-300"
                >
                  {{ t('carpool.joinDialog.rosterOwner') }}
                </span>
              </span>
              <span class="shrink-0 font-medium text-gray-600 dark:text-dark-200">{{ formatUsd(member.declaredWeeklyQuotaUsd) }}</span>
            </li>
            <li
              v-if="joinForm.declaredQuota && joinForm.declaredQuota > 0"
              class="flex items-center justify-between gap-2 border-t border-dashed border-gray-200 pt-1 text-xs dark:border-dark-600"
            >
              <span class="text-primary-600 dark:text-primary-400">{{ t('carpool.joinDialog.rosterYou') }}</span>
              <span class="font-medium text-primary-600 dark:text-primary-400">{{ formatUsd(joinForm.declaredQuota) }}</span>
            </li>
          </ul>
        </div>

        <div class="rounded-lg border border-gray-200 px-3 py-3 dark:border-dark-600">
          <div class="text-xs font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.joinDialog.groupSection') }}</div>
          <div class="mt-2 flex items-center gap-3">
            <img
              v-if="joinQrUrl"
              :src="joinQrUrl"
              :alt="t('carpool.wechat.scanToJoin')"
              class="h-20 w-20 shrink-0 rounded-md border border-gray-200 object-cover dark:border-dark-600"
            />
            <div class="min-w-0 text-xs leading-5 text-gray-500 dark:text-dark-300">
              <p>{{ t('carpool.wechat.scanToJoin') }}</p>
              <button
                type="button"
                class="mt-1 inline-flex items-center gap-1 text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                :title="t('common.copy')"
                @click="copyAdminWechat(joinTarget.adminWechat)"
              >
                <Icon name="copy" size="xs" />
                <span>{{ t('carpool.wechat.adminLabel') }}: {{ joinTarget.adminWechat || ADMIN_WECHAT }}</span>
              </button>
            </div>
          </div>
          <label class="mt-3 flex items-center gap-2">
            <input
              id="carpool-join-group"
              v-model="joinForm.joinedGroup"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
            <span class="text-sm text-gray-700 dark:text-dark-200">{{ t('carpool.joinDialog.joinedGroup') }}</span>
          </label>
        </div>

        <p v-if="joinExceedsRemaining" class="rounded-md bg-red-50 px-3 py-2 text-xs font-medium text-red-700 dark:bg-red-900/20 dark:text-red-300">
          {{ t('carpool.joinDialog.exceedsRemaining', { amount: formatUsd(joinTarget.remainingJoinableUsd) }) }}
        </p>
        <p v-else-if="joinBelowFloor" class="rounded-md bg-red-50 px-3 py-2 text-xs font-medium text-red-700 dark:bg-red-900/20 dark:text-red-300">
          {{ t('carpool.joinDialog.belowFloor', { min: MIN_DECLARED_USD }) }}
        </p>

        <div class="grid grid-cols-3 divide-x divide-gray-200 rounded-lg border border-gray-200 py-3 text-center dark:divide-dark-600 dark:border-dark-600">
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.joinDialog.previewFloor') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatUsd(joinFloorQuota) }} {{ t('carpool.joinDialog.floorUnit') }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.joinDialog.previewPrepaid') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatCny(joinPrepaid) }}</div>
            <!-- 席位费和用量池的分摊口径不同（人头 vs 申报占比），
                 合成一个数字看不出多来一个人能省多少。 -->
            <div class="mt-0.5 text-[10px] text-gray-400 dark:text-dark-400">
              {{ t('carpool.joinDialog.prepaidBreakdown', { seat: formatCny(joinSeatShare), pool: formatCny(joinPoolShare) }) }}
            </div>
          </div>
          <!-- 显示"你的"倍率而非全车平均：申报越小的人越贵，
               拿平均当报价对轻度用户是系统性低估。 -->
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.joinDialog.previewEffectiveRate') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatRate(joinEffectiveRate) }}</div>
            <div class="mt-0.5 text-[10px] text-gray-400 dark:text-dark-400">
              {{ joinUsdPerCny > 0
                ? t('carpool.joinDialog.effectiveRateUnit', { usd: formatDecimal(joinUsdPerCny) })
                : t('carpool.joinDialog.effectiveRateBasis') }}
            </div>
          </div>
        </div>
        <p
          v-if="joinRateRatio > 1.2"
          class="rounded-md bg-amber-50 px-3 py-2 text-xs font-medium text-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
        >
          {{ t('carpool.joinDialog.rateAboveAverage', {
            yours: formatRate(joinEffectiveRate),
            average: formatRate(carEffectiveRate(joinTarget)),
            times: formatDecimal(joinRateRatio),
          }) }}
        </p>

        <p class="rounded-md bg-amber-50 px-3 py-2 text-xs font-semibold text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
          {{ t('carpool.joinDialog.floorNotice') }}
        </p>
        <div class="space-y-1 text-xs leading-5 text-gray-500 dark:text-dark-300">
          <p>{{ t('carpool.notices.weeklyRefresh') }}</p>
          <p>{{ t('carpool.notices.consumeOrder') }}</p>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="joinDialogOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="!joinFormValid || actionPending" @click="submitJoin">
            <Icon name="userPlus" size="sm" />
            {{ t('carpool.joinDialog.confirm') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="inviteDialogOpen" :title="t('carpool.inviteDialog.title')" width="normal" @close="inviteDialogOpen = false">
      <div v-if="selectedCarpool" class="space-y-4">
        <div class="flex items-center justify-between gap-3 border-b border-gray-100 pb-3 dark:border-dark-700">
          <div>
            <div class="font-semibold text-gray-900 dark:text-white">{{ selectedCarpool.name }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ t('carpool.inviteDialog.uses', { count: selectedCarpool.memberCount }) }}</div>
          </div>
          <span :class="['badge', statusBadgeClass(selectedCarpool)]">{{ statusLabel(selectedCarpool) }}</span>
        </div>
        <div>
          <label class="input-label" for="carpool-invite-link">{{ t('carpool.inviteDialog.label') }}</label>
          <div class="flex gap-2">
            <input id="carpool-invite-link" :value="inviteURL(selectedCarpool)" readonly class="input min-w-0 font-mono text-xs" @focus="($event.target as HTMLInputElement).select()" />
            <button type="button" class="btn btn-primary shrink-0 px-3" :title="t('carpool.actions.copyLink')" @click="copyInvite(selectedCarpool)">
              <Icon name="copy" size="sm" />
              <span class="hidden sm:inline">{{ t('carpool.actions.copyLink') }}</span>
            </button>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="detailDialogOpen" :title="t('carpool.detailDialog.title')" width="normal" @close="detailDialogOpen = false">
      <div v-if="selectedCarpool" class="space-y-5">
        <div class="flex items-start justify-between gap-3">
          <div>
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ selectedCarpool.name }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ selectedCarpool.description }}</p>
          </div>
          <span :class="['badge', statusBadgeClass(selectedCarpool)]">{{ statusLabel(selectedCarpool) }}</span>
        </div>
        <div>
          <div class="mb-2 flex justify-between text-sm text-gray-600 dark:text-dark-200">
            <span>{{ t('carpool.fields.quotaProgress') }}</span>
            <span class="font-medium">
              {{ t('carpool.fields.declaredOf', { declared: formatUsd(selectedCarpool.declaredTotalUsd), limit: formatUsd(selectedCarpool.weeklyLimitUsd) }) }}
            </span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div class="h-full rounded-full" :class="quotaProgressClass(selectedCarpool)" :style="{ width: `${quotaProgress(selectedCarpool)}%` }" />
          </div>
        </div>
        <dl class="grid grid-cols-2 gap-4 border-y border-gray-100 py-4 text-sm dark:border-dark-700">
          <div>
            <dt class="text-xs text-gray-400">{{ t('carpool.fields.remainingJoinable') }}</dt>
            <dd class="mt-1 font-medium text-gray-800 dark:text-dark-100">{{ formatUsd(selectedCarpool.remainingJoinableUsd) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400">{{ t('carpool.fields.avgPrice') }}</dt>
            <dd class="mt-1 font-medium text-gray-800 dark:text-dark-100">{{ formatCny(selectedCarpool.avgPriceCny) }} {{ t('carpool.fields.avgPriceUnit') }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400">{{ t('carpool.fields.organizer') }}</dt>
            <dd class="mt-1 font-medium text-gray-800 dark:text-dark-100">{{ selectedCarpool.organizer }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400">{{ t('carpool.fields.scheduledStart') }}</dt>
            <dd class="mt-1 font-medium text-gray-800 dark:text-dark-100">{{ formatDate(selectedCarpool.scheduledStartAt) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400">{{ t('carpool.detailDialog.runtime') }}</dt>
            <dd class="mt-1 font-medium text-gray-800 dark:text-dark-100">{{ statusLabel(selectedCarpool) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400">{{ t('carpool.detailDialog.linkedGroup') }}</dt>
            <dd class="mt-1 font-medium text-gray-800 dark:text-dark-100">{{ selectedCarpool.groupName || t('carpool.detailDialog.pendingGroup') }}</dd>
          </div>
        </dl>
        <div
          v-if="selectedCarpool.hasGroupQrCode"
          class="flex items-center gap-3 rounded-lg border border-gray-200 px-3 py-3 dark:border-dark-600"
        >
          <img
            v-if="qrCodeUrls[selectedCarpool.id]"
            :src="qrCodeUrls[selectedCarpool.id]"
            :alt="t('carpool.wechat.scanToJoin')"
            class="h-20 w-20 shrink-0 rounded-md border border-gray-200 object-cover dark:border-dark-600"
          />
          <div class="min-w-0 text-xs leading-5 text-gray-500 dark:text-dark-300">
            <div class="font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.wechat.scanToJoin') }}</div>
            <button
              type="button"
              class="mt-1 inline-flex items-center gap-1 text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              :title="t('common.copy')"
              @click="copyAdminWechat(selectedCarpool.adminWechat)"
            >
              <Icon name="copy" size="xs" />
              <span>{{ t('carpool.wechat.adminLabel') }}: {{ selectedCarpool.adminWechat || ADMIN_WECHAT }}</span>
            </button>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="settlementDialogOpen" :title="t('carpool.settlement.title')" width="wide" @close="settlementDialogOpen = false">
      <div class="space-y-4">
        <div v-if="settlementLoading" class="flex min-h-32 items-center justify-center">
          <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
        </div>
        <template v-else-if="settlementData">
          <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-dark-300">
            <span>
              {{ t('carpool.settlement.period') }}:
              {{ settlementData.periodStart ? `${formatDateTime(settlementData.periodStart)} – ${formatDateTime(settlementData.periodEnd || '')}` : '-' }}
            </span>
            <span class="font-medium">
              {{ settlementData.fullView ? t('carpool.settlement.fullView', { count: settlementData.memberCount }) : t('carpool.settlement.selfOnly') }}
            </span>
          </div>

          <!--
            自定义规则车按 rule_note 人工结算：后端只回实际用量，金额字段全为零
            （是"不适用"而非"算出来是 0"）。这里明确说明，并隐藏退补列，避免
            把老车按新定价的假账渲染出来。
          -->
          <div
            v-if="settlementData.manualSettlement"
            data-testid="carpool-settlement-manual"
            class="rounded-md bg-slate-50 px-3 py-2 text-xs leading-5 text-slate-700 dark:bg-slate-900/30 dark:text-slate-300"
          >
            <div class="font-medium">{{ t('carpool.settlement.manualTitle') }}</div>
            <p v-if="settlementData.ruleNote" class="mt-1">{{ settlementData.ruleNote }}</p>
            <p class="mt-1">{{ t('carpool.settlement.manualHint') }}</p>
          </div>

          <!--
            冻结 vs 实时：未结算时表里的数字会随用量继续走，车主拿它收款就会
            出现"我按 A 收的、他看到的是 B"。结算把这一份钉死。
          -->
          <div
            v-else-if="settlementData.settled"
            data-testid="carpool-settlement-frozen"
            class="flex flex-wrap items-center justify-between gap-2 rounded-md bg-emerald-50 px-3 py-2 text-xs text-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300"
          >
            <span>
              {{ t('carpool.settlement.frozenAt', { time: formatDateTime(settlementData.settledAt || '') }) }}
            </span>
            <button
              v-if="authStore.isAdmin"
              type="button"
              class="btn btn-ghost h-7 px-2 py-1 text-xs"
              :disabled="settlePending"
              @click="unsettleCarpool"
            >
              {{ t('carpool.settlement.unsettle') }}
            </button>
          </div>
          <div
            v-else
            data-testid="carpool-settlement-live"
            class="flex flex-wrap items-center justify-between gap-2 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
          >
            <span>{{ t('carpool.settlement.livePreview') }}</span>
            <button
              v-if="settlementData.canSettle"
              type="button"
              class="btn btn-primary h-7 px-3 py-1 text-xs"
              :disabled="settlePending"
              @click="settleCarpool"
            >
              {{ t('carpool.settlement.settle') }}
            </button>
            <span v-else-if="settlementData.settleBlockedFor === 'not_launched'">
              {{ t('carpool.settlement.blockedNotLaunched') }}
            </span>
          </div>
          <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-600">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-600">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300">
                <tr>
                  <th class="px-3 py-2 text-left font-medium">{{ t('carpool.settlement.member') }}</th>
                  <th v-if="!settlementData.manualSettlement" class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.declared') }}</th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.actual') }}</th>
                  <template v-if="!settlementData.manualSettlement">
                    <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.billable') }}</th>
                    <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.prepaid') }}</th>
                    <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.delta') }}</th>
                  </template>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="member in settlementData.members" :key="member.userId">
                  <!-- 带上邮箱/用户名：只有 #id 的话车主没法对应到微信群里的真人收款 -->
                  <td class="px-3 py-2">
                    <div class="flex items-baseline gap-1.5">
                      <span class="font-medium text-gray-800 dark:text-dark-100">
                        {{ member.username || member.email || `#${member.userId}` }}
                      </span>
                      <span class="text-xs text-gray-400">{{ t(`carpool.roles.${member.role}`) }}</span>
                    </div>
                    <div v-if="member.email && member.username" class="text-[11px] text-gray-400 dark:text-dark-400">
                      {{ member.email }}
                    </div>
                    <div class="text-[11px] text-gray-400 dark:text-dark-400">#{{ member.userId }}</div>
                  </td>
                  <td v-if="!settlementData.manualSettlement" class="px-3 py-2 text-right font-mono">{{ formatDecimal(member.declaredWeeklyQuotaUsd) }}</td>
                  <td class="px-3 py-2 text-right font-mono">{{ formatDecimal(member.actualUsageUsd) }}</td>
                  <template v-if="!settlementData.manualSettlement">
                    <td class="px-3 py-2 text-right font-mono">
                      {{ formatDecimal(member.billableUsageUsd) }}
                      <span v-if="member.floorTriggered" class="ml-1 rounded bg-amber-100 px-1 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                        {{ t('carpool.settlement.floorBadge') }}
                      </span>
                    </td>
                    <td class="px-3 py-2 text-right font-mono">{{ formatCny(member.prepaidAmountCny) }}</td>
                    <td class="px-3 py-2 text-right font-mono font-semibold" :class="deltaClass(member.totalDeltaCny)">
                      {{ deltaLabel(member.totalDeltaCny) }}
                    </td>
                  </template>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-if="!settlementData.manualSettlement" class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.settlement.deltaNote') }}</p>
        </template>
      </div>
    </BaseDialog>

    <ConfirmDialog
      :show="confirmAction !== null"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-text="confirmText"
      :danger="confirmAction?.type === 'cancel' || confirmAction?.type === 'leave'"
      @confirm="runConfirmedAction"
      @cancel="confirmAction = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import carpoolAPI, {
  type Carpool,
  type CarpoolSettlement,
  type CarpoolVisibility,
  type CreateCarpoolRequest,
  type DeclarationRecommendation,
  type CarpoolRosterMember,
  type PendingLaunchCarpool,
} from '@/api/carpools'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

// 与后端 CarpoolForceLaunchMinRatio 对齐：降档发车允许的最低总申报比例。
const FORCE_LAUNCH_MIN_RATIO = 0.8

// 与后端 CarpoolMinDeclaredWeeklyQuotaUSD 对齐：申报下限（美元/周）。
const MIN_DECLARED_USD = 20
// 一个计费周期按 31 天算：申报是"每周"额度，周期内可用额度 = 申报 × 31 / 7。
// "相当于几个 Plus"要求用户先知道一个 Plus 值多少额度；等效倍率不用——
// 付多少钱换多少官方额度，直接相除即可（付 ¥1 拿到 $8 就是 0.125，不折汇率）。
const BILLING_PERIOD_DAYS = 31
const DAYS_PER_WEEK = 7

// 与后端 CarpoolAdminWechatID 对齐：创建/上车前必须添加的管理员微信号。
const ADMIN_WECHAT = 'Charlemartingale'

// 与后端 CarpoolGroupQRCodeMaxBytes 对齐：群二维码大小上限（2MB）。
const QR_CODE_MAX_BYTES = 2 * 1024 * 1024
const QR_CODE_TYPES = ['image/png', 'image/jpeg', 'image/webp']

interface CreateForm {
  name: string
  description: string
  visibility: CarpoolVisibility
  scheduledStartAt: string
  ownerQuota: number | null
  addedAdminWechat: boolean
  groupQrCode: string
}

// 创建对话框的规则模式：default 走现有创建流程；custom 仅通知管理员协商，不调用创建接口。
type CreateRuleMode = 'default' | 'custom'

// ConfirmTarget 是确认对话框真正需要的那几个字段。刻意比 Carpool 窄：
// 待启动列表里的车可能根本不在 carpools 列表中（admin 看不到别人的私密车），
// 只能从 pendingLaunch 的行里拿到这几项。Carpool 结构上满足该类型，可直接传。
interface ConfirmTarget {
  id: number
  name: string
  declaredTotalUsd: number
  weeklyLimitUsd: number
}

interface ConfirmAction {
  type: 'cancel' | 'launch' | 'forceLaunch' | 'confirm' | 'unconfirm' | 'leave'
  carpool: ConfirmTarget
}

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const activeTab = ref<'plaza' | 'mine'>('plaza')
const searchQuery = ref('')
const statusFilter = ref('')
const loading = ref(true)
const actionPending = ref(false)
const createDialogOpen = ref(false)
const createQrError = ref('')
const createRuleMode = ref<CreateRuleMode>('default')
const customRuleNotifyPending = ref(false)
const customRuleNotified = ref(false)
const joinDialogOpen = ref(false)
const inviteDialogOpen = ref(false)
const detailDialogOpen = ref(false)
const settlementDialogOpen = ref(false)
const settlementLoading = ref(false)
const settlementData = ref<CarpoolSettlement | null>(null)
const settlementTarget = ref<Carpool | null>(null)
const settlePending = ref(false)
const selectedCarpool = ref<Carpool | null>(null)
const selectedInviteToken = ref('')
const joinTarget = ref<Carpool | null>(null)
const joinInviteToken = ref('')
const joinForm = reactive({ declaredQuota: null as number | null, joinedGroup: false })
const joinQrUrl = ref('')
const qrCodeUrls = ref<Record<number, string>>({})
const recommendation = ref<DeclarationRecommendation | null>(null)
const recommendationLoading = ref(false)
const recommendationFailed = ref(false)
const joinRoster = ref<CarpoolRosterMember[]>([])
const joinRosterLoading = ref(false)
const joinRosterFailed = ref(false)
const joinRosterTotal = computed(() => joinRoster.value.reduce((sum, item) => sum + item.declaredWeeklyQuotaUsd, 0))
const confirmAction = ref<ConfirmAction | null>(null)
const carpools = ref<Carpool[]>([])
const pendingLaunches = ref<PendingLaunchCarpool[]>([])
const overduePendingCount = computed(() => pendingLaunches.value.filter((item) => item.overdue).length)
// 申报推荐是异步回填金额输入框的，用序号丢弃过期响应（见 openJoin）。
let recommendationSeq = 0
// 花名册同样要丢弃过期响应：连点两辆车会把 A 车的成员显示在 B 车的弹窗里。
let rosterSeq = 0

// carpoolErrorMessages 把后端错误码映射成中文提示。
// 超额度、车满、私密车无权限这些都是核心拒绝路径，直接把英文原文抛给用户
// 既看不懂、也没告诉他下一步该干什么（设计文档要求提示"等下一辆车"）。
function carpoolErrorMessages(): Record<string, string> {
  return {
    CARPOOL_QUOTA_EXCEEDED: t('carpool.errors.quotaExceeded'),
    CARPOOL_FULL: t('carpool.errors.full'),
    CARPOOL_UNAVAILABLE: t('carpool.errors.unavailable'),
    CARPOOL_ALREADY_JOINED: t('carpool.errors.alreadyJoined'),
    CARPOOL_FORBIDDEN: t('carpool.errors.forbidden'),
    CARPOOL_NOT_FOUND: t('carpool.errors.notFound'),
    CARPOOL_ALREADY_SETTLED: t('carpool.errors.alreadySettled'),
    CARPOOL_NOT_SETTLED: t('carpool.errors.notSettled'),
    CARPOOL_NOT_SETTLEABLE: t('carpool.errors.notSettleable'),
    CARPOOL_CUSTOM_RULE_CLOSED: t('carpool.errors.customRuleClosed'),
    CARPOOL_INVITE_INVALID: t('carpool.errors.inviteInvalid'),
    CARPOOL_NAME_CONFLICT: t('carpool.errors.nameConflict'),
    CARPOOL_LAUNCH_NOT_READY: t('carpool.errors.launchNotReady'),
    CARPOOL_NOT_CONFIRMED: t('carpool.errors.notConfirmed'),
    CARPOOL_DECLARATION_TOO_SMALL: t('carpool.errors.declarationTooSmall', { min: MIN_DECLARED_USD }),
    CARPOOL_INTEREST_TOO_FREQUENT: t('carpool.errors.interestTooFrequent'),
    CARPOOL_CUSTOM_PARAMS_FORBIDDEN: t('carpool.errors.customParamsForbidden'),
    CARPOOL_GROUP_JOIN_REQUIRED: t('carpool.errors.groupJoinRequired'),
    CARPOOL_CONTACT_CONFIRM_REQUIRED: t('carpool.errors.contactConfirmRequired'),
    CARPOOL_GROUP_QR_CODE_REQUIRED: t('carpool.errors.qrCodeRequired'),
    CARPOOL_GROUP_QR_CODE_INVALID: t('carpool.errors.qrCodeInvalid'),
    CARPOOL_OWNER_CANNOT_LEAVE: t('carpool.errors.ownerCannotLeave'),
    CARPOOL_NOT_MEMBER: t('carpool.errors.notMember'),
  }
}

// carpoolError 统一的错误提示：优先按错误码取中文文案，取不到再回落。
function carpoolError(error: unknown, fallbackKey = 'carpool.actionFailed'): string {
  return extractApiErrorMessage(error, t(fallbackKey), carpoolErrorMessages())
}

const isoDateAfterDays = (days: number): string => {
  const date = new Date(Date.now() + days * 24 * 60 * 60 * 1000)
  return date.toISOString().slice(0, 10)
}

const newCreateForm = (): CreateForm => ({
  name: '',
  description: '',
  visibility: 'public',
  scheduledStartAt: isoDateAfterDays(7),
  ownerQuota: null,
  addedAdminWechat: false,
  groupQrCode: '',
})

const createForm = reactive<CreateForm>(newCreateForm())
const tabs = computed(() => [
  { value: 'plaza' as const, label: t('carpool.plaza') },
  { value: 'mine' as const, label: t('carpool.mine') }
])
const visibilityOptions = computed(() => [
  { value: 'public' as const, label: t('carpool.visibility.public') },
  { value: 'invite_only' as const, label: t('carpool.visibility.inviteOnly') }
])
const ruleModeOptions = computed(() => [
  { value: 'default' as const, label: t('carpool.createDialog.ruleModeDefault') },
  { value: 'custom' as const, label: t('carpool.createDialog.ruleModeCustom') }
])
// 自定义规则模式下表单其余项全部禁用。
const createFieldsDisabled = computed(() => createRuleMode.value === 'custom')
const ruleItems = computed(() => [
  { label: t('carpool.rules.declare.label'), text: t('carpool.rules.declare.text') },
  { label: t('carpool.rules.reserve.label'), text: t('carpool.rules.reserve.text') },
  { label: t('carpool.rules.pricing.label'), text: t('carpool.rules.pricing.text') },
  { label: t('carpool.rules.floor.label'), text: t('carpool.rules.floor.text') },
])
const stats = computed(() => [
  { label: t('carpool.stats.recruiting'), value: carpools.value.filter((item) => item.status === 'recruiting' && !item.joinLocked).length },
  { label: t('carpool.stats.joinableQuota'), value: formatUsd(carpools.value.reduce((sum, item) => sum + (canJoin(item) ? item.remainingJoinableUsd : 0), 0)) },
  { label: t('carpool.stats.joined'), value: carpools.value.filter((item) => item.memberRole !== null && item.status !== 'cancelled').length },
  { label: t('carpool.stats.launched'), value: carpools.value.filter((item) => item.status === 'active').length }
])
const filteredCarpools = computed(() => {
  const query = searchQuery.value.toLowerCase()
  // 已取消的车默认不出现在任何标签页里。取消之后它对用户来说就是"没了"，
  // 继续挂在「我的拼车」里会让人以为取消没生效——广场页本来就排除了，
  // 这里漏掉只是不一致。要回看历史就从状态筛选里显式选「已取消」。
  const showCancelled = statusFilter.value === 'cancelled'
  return carpools.value
    .filter((item) => showCancelled || item.status !== 'cancelled')
    // 「我的拼车」= 我还在车上，或者我是车主。后者不能省：发车时申报为 0 的
    // 车主会被置成 'left'（launchCarpool 跳过零申报成员），而 member_role 现在
    // 排除了 'left'，只按 memberRole 过滤会让车主看不见自己发起的车、也就没法
    // 取消它。
    .filter((item) => activeTab.value === 'plaza'
      ? item.visibility === 'public'
      : item.memberRole !== null || item.ownerUserId === authStore.user?.id)
    .filter((item) => !statusFilter.value || item.status === statusFilter.value)
    .filter((item) => !query || item.name.toLowerCase().includes(query) || item.organizer.toLowerCase().includes(query))
})
const createFormValid = computed(() => (
  createForm.name.length > 0
  && createForm.scheduledStartAt.length > 0
  && (createForm.ownerQuota === null || createForm.ownerQuota >= 0)
  && createForm.addedAdminWechat
  && createForm.groupQrCode.length > 0
  && !createQrError.value
))
const joinFloorQuota = computed(() => {
  if (!joinTarget.value || !joinForm.declaredQuota || joinForm.declaredQuota <= 0) return 0
  return joinTarget.value.reserveRatio * joinForm.declaredQuota
})
// 预付拆成两块：席位费按人头均摊（跟车上现有几个人直接相关），
// 用量池按申报占比分摊。合成一个数字就看不出"多少钱是席位费"。
const joinHeadcount = computed(() => (joinTarget.value ? joinTarget.value.memberCount + 1 : 1))
const joinSeatShare = computed(() => (
  joinTarget.value ? joinTarget.value.seatFeeCny / Math.max(1, joinHeadcount.value) : 0
))
const joinPoolShare = computed(() => {
  const car = joinTarget.value
  const declared = joinForm.declaredQuota
  if (!car || !declared || declared <= 0 || car.weeklyLimitUsd <= 0) return 0
  return car.usagePoolCny * declared / car.weeklyLimitUsd
})
const joinPrepaid = computed(() => {
  if (!joinTarget.value || !joinForm.declaredQuota || joinForm.declaredQuota <= 0) return 0
  return joinSeatShare.value + joinPoolShare.value
})
const joinExceedsRemaining = computed(() => (
  !!joinTarget.value && !!joinForm.declaredQuota && joinForm.declaredQuota > joinTarget.value.remainingJoinableUsd + 1e-9
))
const joinBelowFloor = computed(() => (
  joinForm.declaredQuota !== null && joinForm.declaredQuota > 0 && joinForm.declaredQuota < MIN_DECLARED_USD
))
// 等效倍率：这一位成员付的钱相当于官方价的几倍。
// 分母是一个计费周期（31 天）内他能用的额度 = 申报 × 31 / 7。
// 用"你的"而不是全车平均——席位费按人头均摊、用量池按申报分摊，
// 申报越小的人越贵（设计文档举过"实际单价可能是均价两倍"的例子）。
const joinPeriodQuotaUsd = computed(() => {
  const declared = joinForm.declaredQuota
  if (!declared || declared <= 0) return 0
  return declared * BILLING_PERIOD_DAYS / DAYS_PER_WEEK
})
// 等效倍率 = 付出的人民币 ÷ 拿到的官方额度（美元）。
const joinEffectiveRate = computed(() => (
  joinPeriodQuotaUsd.value > 0 ? joinPrepaid.value / joinPeriodQuotaUsd.value : 0
))
// 倒数：¥1 换到多少官方额度。小数倍率不好念，这个更直观。
const joinUsdPerCny = computed(() => (joinEffectiveRate.value > 0 ? 1 / joinEffectiveRate.value : 0))
// 整车口径的同一个指标，用于卡片展示和"你比平均贵多少"的对比。
function carEffectiveRate(carpool: Carpool): number {
  const periodQuota = carpool.weeklyLimitUsd * BILLING_PERIOD_DAYS / DAYS_PER_WEEK
  if (periodQuota <= 0) return 0
  return (carpool.seatFeeCny + carpool.usagePoolCny) / periodQuota
}
function carUsdPerCny(carpool: Carpool): number {
  const rate = carEffectiveRate(carpool)
  return rate > 0 ? 1 / rate : 0
}
// 与全车平均倍率的偏离，> 1 说明这位用户比平均更贵。
const joinRateRatio = computed(() => {
  const car = joinTarget.value
  if (!car || joinEffectiveRate.value <= 0) return 0
  const avg = carEffectiveRate(car)
  return avg > 0 ? joinEffectiveRate.value / avg : 0
})
const joinFormValid = computed(() => (
  !!joinForm.declaredQuota && joinForm.declaredQuota > 0
  && !joinExceedsRemaining.value && !joinBelowFloor.value && joinForm.joinedGroup
))
const confirmTitle = computed(() => {
  if (!confirmAction.value) return ''
  switch (confirmAction.value.type) {
    case 'cancel': return t('carpool.cancelDialog.title')
    case 'leave': return t('carpool.leaveDialog.title')
    case 'confirm': return t('carpool.confirmDialog.title')
    case 'unconfirm': return t('carpool.unconfirmDialog.title')
    case 'forceLaunch': return t('carpool.launchDialog.forceTitle')
    default: return t('carpool.launchDialog.confirmTitle')
  }
})
const confirmText = computed(() => {
  if (!confirmAction.value) return ''
  if (confirmAction.value.type === 'cancel') return t('carpool.cancelDialog.confirm')
  if (confirmAction.value.type === 'leave') return t('carpool.leaveDialog.confirm')
  if (confirmAction.value.type === 'confirm') return t('carpool.confirmDialog.confirm')
  if (confirmAction.value.type === 'unconfirm') return t('carpool.unconfirmDialog.confirm')
  return t('carpool.launchDialog.confirm')
})
const confirmMessage = computed(() => {
  if (!confirmAction.value) return ''
  const action = confirmAction.value
  if (action.type === 'cancel') return t('carpool.cancelDialog.message', { name: action.carpool.name })
  if (action.type === 'leave') return t('carpool.leaveDialog.message', { name: action.carpool.name })
  if (action.type === 'unconfirm') return t('carpool.unconfirmDialog.message', { name: action.carpool.name })
  const params = {
    name: action.carpool.name,
    total: formatUsd(action.carpool.declaredTotalUsd),
    ratio: launchRatioPercent(declaredRatio(action.carpool)),
  }
  if (action.type === 'confirm') return t('carpool.confirmDialog.message', params)
  return action.type === 'forceLaunch'
    ? t('carpool.launchDialog.forceMessage', params)
    : t('carpool.launchDialog.confirmMessage', params)
})

async function loadCarpools(): Promise<void> {
  try {
    carpools.value = await carpoolAPI.list()
    ensureQrCodes(carpools.value)
  } catch (error) {
    appStore.showError(carpoolError(error, 'carpool.loadFailed'))
  } finally {
    loading.value = false
  }
  await loadPendingLaunches()
}

// 待启动列表只对 admin 有意义，非 admin 直接跳过（后端也会 403）。
async function loadPendingLaunches(): Promise<void> {
  if (!authStore.isAdmin) {
    pendingLaunches.value = []
    return
  }
  try {
    pendingLaunches.value = await carpoolAPI.pendingLaunch()
  } catch {
    // 列表加载失败不影响主页面
    pendingLaunches.value = []
  }
}

// 从待启动列表启动：直接用待启动行自己的数据构造确认目标。
//
// 不能去 carpools 列表里查——List 只返回公开车与自己参与的车，别人的
// 私密车 admin 根本看不到；而待启动列表是 admin 专用、不做可见性过滤。
// 早先的实现会让这种车在横幅里可见却点不动（后端其实允许启动）。
function requestLaunchFromPending(item: PendingLaunchCarpool): void {
  confirmAction.value = {
    type: 'launch',
    carpool: {
      id: item.carpoolId,
      name: item.name,
      declaredTotalUsd: item.declaredTotalUsd,
      weeklyLimitUsd: item.weeklyLimitUsd,
    },
  }
}

// 为有群二维码的车辆预取图片，object URL 按车辆缓存。
// 私密车的二维码只对成员/车主/admin 开放（非成员需要带邀请 token，见 openJoin），
// 这里直接跳过，免得列表渲染打出一片必然 403 的请求。
function ensureQrCodes(items: Carpool[]): void {
  for (const item of items) {
    if (!item.hasGroupQrCode || qrCodeUrls.value[item.id]) continue
    if (item.visibility === 'invite_only' && item.memberRole === null && !authStore.isAdmin) continue
    carpoolAPI.groupQrCode(item.id)
      .then((blob) => {
        qrCodeUrls.value = { ...qrCodeUrls.value, [item.id]: URL.createObjectURL(blob) }
      })
      .catch(() => {
        // 二维码加载失败不阻塞卡片展示
      })
  }
}

function declaredRatio(carpool: Pick<Carpool, 'weeklyLimitUsd' | 'declaredTotalUsd'>): number {
  return carpool.weeklyLimitUsd > 0 ? carpool.declaredTotalUsd / carpool.weeklyLimitUsd : 0
}

function quotaProgress(carpool: Carpool): number {
  return Math.min(100, Math.round(declaredRatio(carpool) * 100))
}

function launchLinePercent(carpool: Carpool): number {
  return Math.min(100, Math.round(carpool.launchMinRatio * 100))
}

function launchRatioPercent(ratio: number): number {
  return Math.round(ratio * 100)
}

// isQuotaCar 报告是否为额度预约制的车。自定义规则车（含平台升级前建立的老车）
// 不参与申报/保底/公共池那套，展示与操作都要走另一条分支。
function isQuotaCar(carpool: Pick<Carpool, 'pricingModel'>): boolean {
  return !carpool.pricingModel || carpool.pricingModel === 'quota'
}

function canJoin(carpool: Carpool): boolean {
  // 自定义规则车不接新成员（后端同样拒绝）：它们不走申报制，而升级前遗留的
  // 招募中老车 Σ申报 恒为 0、永远达不到发车区间，再放人进去就是往开不走的
  // 车里交预付。
  if (!isQuotaCar(carpool)) return false
  return carpool.status === 'recruiting' && !carpool.joinLocked && carpool.remainingJoinableUsd > 0
}

function canInvite(carpool: Carpool): boolean {
  return (carpool.memberRole === 'owner' || authStore.isAdmin) && canJoin(carpool)
}

function canCancel(carpool: Carpool): boolean {
  if (carpool.status === 'confirmed') {
    // confirmed 全锁：后端只允许 admin 强制取消，前端过去完全没有入口，
    // 车一旦确认就成了死胡同（车主只能撤回确认，见 canUnconfirm）。
    return authStore.isAdmin
  }
  return (carpool.memberRole === 'owner' || authStore.isAdmin)
    && (carpool.status === 'recruiting' || carpool.status === 'starting')
}

// 撤回确认：车主或 admin 把 confirmed 的车退回招募中，重新开放上车。
// 这是"等管理员启动"这段状态唯一的温和出口——成员和申报都保留。
function canUnconfirm(carpool: Carpool): boolean {
  return carpool.status === 'confirmed' && (carpool.memberRole === 'owner' || authStore.isAdmin)
}

function canLeave(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' && carpool.memberRole === 'member'
}

// 两段确认第一段：车主在 recruiting 且 Σ 申报进入发车区间后确认发车。
function canConfirm(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' && carpool.memberRole === 'owner'
}

// 两段确认第二段：仅管理员启动已确认的车。
function canAdminLaunch(carpool: Carpool): boolean {
  return authStore.isAdmin && carpool.status === 'confirmed'
}

// 降档发车（force）：仅管理员、recruiting 且 Σ≥80%，跳过确认流程。
function canForceLaunch(carpool: Carpool): boolean {
  return authStore.isAdmin && carpool.status === 'recruiting' && declaredRatio(carpool) >= FORCE_LAUNCH_MIN_RATIO
}

function launchReady(carpool: Carpool): boolean {
  const ratio = declaredRatio(carpool)
  return ratio >= carpool.launchMinRatio && ratio <= carpool.launchMaxRatio
}

function launchHint(carpool: Carpool): string {
  if (launchReady(carpool)) return ''
  if (declaredRatio(carpool) > carpool.launchMaxRatio) {
    return t('carpool.confirmDialog.aboveMax', { ratio: launchRatioPercent(carpool.launchMaxRatio) })
  }
  const missing = Math.max(0, carpool.launchMinRatio * carpool.weeklyLimitUsd - carpool.declaredTotalUsd)
  return t('carpool.confirmDialog.notReady', {
    ratio: launchRatioPercent(carpool.launchMinRatio),
    amount: formatUsd(missing),
  })
}

function canViewSettlement(carpool: Carpool): boolean {
  return carpool.status === 'active' && (carpool.memberRole !== null || authStore.isAdmin)
}

function statusLabel(carpool: Carpool): string {
  if (carpool.status === 'recruiting' && carpool.joinLocked) return t('carpool.status.locked')
  if (carpool.status === 'recruiting' && carpool.remainingJoinableUsd <= 0) return t('carpool.status.full')
  return t(`carpool.status.${carpool.status}`)
}

function statusBadgeClass(carpool: Carpool): string {
  if (carpool.status === 'cancelled' || carpool.status === 'ended') return 'badge-gray'
  if (carpool.status === 'active') return 'badge-success'
  if (carpool.status === 'confirmed') return 'badge-warning'
  if (carpool.joinLocked || carpool.remainingJoinableUsd <= 0) return 'badge-warning'
  return 'badge-primary'
}

function quotaProgressClass(carpool: Carpool): string {
  if (carpool.status === 'active') return 'bg-emerald-500'
  if (carpool.joinLocked || carpool.remainingJoinableUsd <= 0) return 'bg-amber-500'
  if (launchReady(carpool)) return 'bg-emerald-500'
  return 'bg-primary-500'
}

function formatUsd(value: number): string {
  return `$${new Intl.NumberFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', { maximumFractionDigits: 0 }).format(value)}`
}

function formatCny(value: number): string {
  return `¥${new Intl.NumberFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', { minimumFractionDigits: 0, maximumFractionDigits: 1 }).format(value)}`
}

function formatDecimal(value: number): string {
  return new Intl.NumberFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', { maximumFractionDigits: 1 }).format(value)
}

// 倍率是 0.0x 量级，formatDecimal 的 1 位小数会把它全部压成 "0"。
function formatRate(value: number): string {
  if (!(value > 0)) return '—'
  return `${new Intl.NumberFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', { maximumFractionDigits: 3 }).format(value)}×`
}

function formatDate(value: string): string {
  if (!value) return '-'
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    month: 'short', day: 'numeric', year: 'numeric'
  }).format(new Date(`${value}T12:00:00`))
}

function formatDateTime(value: string): string {
  if (!value) return '-'
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    month: 'short', day: 'numeric', year: 'numeric'
  }).format(new Date(value))
}

function deltaClass(delta: number): string {
  if (delta > 0.004) return 'text-emerald-600 dark:text-emerald-400'
  if (delta < -0.004) return 'text-red-600 dark:text-red-400'
  return 'text-gray-500 dark:text-dark-300'
}

function deltaLabel(delta: number): string {
  if (delta > 0.004) return t('carpool.settlement.deltaRefund', { amount: Math.abs(delta).toFixed(1) })
  if (delta < -0.004) return t('carpool.settlement.deltaTopUp', { amount: Math.abs(delta).toFixed(1) })
  return t('carpool.settlement.deltaEven')
}

function openCreateDialog(): void {
  Object.assign(createForm, newCreateForm())
  createQrError.value = ''
  createRuleMode.value = 'default'
  customRuleNotifyPending.value = false
  customRuleNotified.value = false
  createDialogOpen.value = true
}

function handleQrFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  createQrError.value = ''
  createForm.groupQrCode = ''
  if (!file) return
  if (!QR_CODE_TYPES.includes(file.type)) {
    createQrError.value = t('carpool.createDialog.qrInvalidType')
    input.value = ''
    return
  }
  if (file.size > QR_CODE_MAX_BYTES) {
    createQrError.value = t('carpool.createDialog.qrTooLarge')
    input.value = ''
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    createForm.groupQrCode = typeof reader.result === 'string' ? reader.result : ''
  }
  reader.readAsDataURL(file)
}

async function copyAdminWechat(wechat: string): Promise<void> {
  await navigator.clipboard.writeText(wechat || ADMIN_WECHAT)
  appStore.showSuccess(t('carpool.wechat.copied'))
}

// 自定义规则模式：不创建车辆，仅通知管理员协商；成功后展示管理员微信供添加。
async function notifyCustomRule(): Promise<void> {
  if (customRuleNotifyPending.value) return
  customRuleNotifyPending.value = true
  try {
    await carpoolAPI.notifyCustomRuleInterest()
    customRuleNotified.value = true
    appStore.showSuccess(t('carpool.createDialog.customRule.notifySuccess', { wechat: ADMIN_WECHAT }))
  } catch (error) {
    appStore.showError(carpoolError(error))
  } finally {
    customRuleNotifyPending.value = false
  }
}

async function createCarpool(): Promise<void> {
  if (createRuleMode.value !== 'default' || !createFormValid.value || actionPending.value) return
  actionPending.value = true
  try {
    const payload: CreateCarpoolRequest = {
      name: createForm.name,
      description: createForm.description,
      visibility: createForm.visibility,
      scheduled_start_at: createForm.scheduledStartAt,
      added_admin_wechat: createForm.addedAdminWechat,
      group_qr_code: createForm.groupQrCode,
    }
    if (createForm.ownerQuota && createForm.ownerQuota > 0) {
      payload.declared_weekly_quota_usd = createForm.ownerQuota
    }
    const result = await carpoolAPI.create(payload)
    createDialogOpen.value = false
    activeTab.value = 'mine'
    await loadCarpools()
    appStore.showSuccess(t('carpool.createDialog.success'))
    await openInvite(result.carpool, result.inviteToken)
  } catch (error) {
    appStore.showError(carpoolError(error))
  } finally {
    actionPending.value = false
  }
}

// 连续点两辆车的"邀请"时，先发的请求可能后回来。用序号丢弃过期响应，
// 否则对话框会显示 B 车、链接却是 A 车的邀请码——分享出去就是错的车。
let inviteRequestSeq = 0

async function openInvite(carpool: Carpool, token = ''): Promise<void> {
  const seq = ++inviteRequestSeq
  try {
    const resolvedToken = token || await carpoolAPI.createInvite(carpool.id)
    if (seq !== inviteRequestSeq) return
    selectedCarpool.value = carpool
    selectedInviteToken.value = resolvedToken
    inviteDialogOpen.value = true
  } catch (error) {
    if (seq !== inviteRequestSeq) return
    appStore.showError(carpoolError(error))
  }
}

function openDetails(carpool: Carpool): void {
  selectedCarpool.value = carpool
  detailDialogOpen.value = true
}

function inviteURL(_carpool: Carpool): string {
  return `${window.location.origin}/carpools/join/${selectedInviteToken.value}`
}

async function copyInvite(carpool: Carpool): Promise<void> {
  await navigator.clipboard.writeText(inviteURL(carpool))
  appStore.showSuccess(t('carpool.actions.copied'))
}

async function toggleJoinLock(carpool: Carpool): Promise<void> {
  try {
    await carpoolAPI.setJoinLocked(carpool.id, !carpool.joinLocked)
    await loadCarpools()
    appStore.showSuccess(t(!carpool.joinLocked ? 'carpool.admin.locked' : 'carpool.admin.unlocked'))
  } catch (error) {
    appStore.showError(carpoolError(error))
  }
}

function openJoin(carpool: Carpool, inviteToken = ''): void {
  if (!canJoin(carpool) || carpool.memberRole) {
    appStore.showWarning(t('carpool.unavailable'))
    return
  }
  joinTarget.value = carpool
  joinInviteToken.value = inviteToken
  joinForm.declaredQuota = null
  joinForm.joinedGroup = false
  joinQrUrl.value = qrCodeUrls.value[carpool.id] || ''
  if (carpool.hasGroupQrCode && !joinQrUrl.value) {
    // 私密车走邀请链接进来时，二维码请求必须带上 token（后端据此授权）。
    carpoolAPI.groupQrCode(carpool.id, inviteToken || undefined)
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        qrCodeUrls.value = { ...qrCodeUrls.value, [carpool.id]: url }
        if (joinTarget.value?.id === carpool.id) joinQrUrl.value = url
      })
      .catch(() => {
        // 二维码加载失败不阻塞上车流程
      })
  }
  joinRoster.value = []
  joinRosterFailed.value = false
  joinRosterLoading.value = true
  const rosterRequest = ++rosterSeq
  carpoolAPI.roster(carpool.id, inviteToken || undefined)
    .then((members) => {
      if (rosterRequest !== rosterSeq) return
      joinRoster.value = members
    })
    .catch(() => {
      // 花名册只是参考信息，加载失败不该挡住上车。
      if (rosterRequest !== rosterSeq) return
      joinRosterFailed.value = true
    })
    .finally(() => {
      if (rosterRequest !== rosterSeq) return
      joinRosterLoading.value = false
    })
  recommendation.value = null
  recommendationFailed.value = false
  recommendationLoading.value = true
  joinDialogOpen.value = true
  const seq = ++recommendationSeq
  carpoolAPI.declarationRecommendation()
    .then((rec) => {
      // 迟到的响应不能再回写：这是金额输入框，覆盖用户已经改过的数字
      // 会让他按自己没同意的额度上车。
      if (seq !== recommendationSeq) return
      recommendation.value = rec
      if (rec.recommendedWeeklyQuotaUsd > 0 && joinForm.declaredQuota === null) {
        joinForm.declaredQuota = Math.round(rec.recommendedWeeklyQuotaUsd * 10) / 10
      }
    })
    .catch(() => {
      if (seq !== recommendationSeq) return
      recommendationFailed.value = true
    })
    .finally(() => {
      if (seq !== recommendationSeq) return
      recommendationLoading.value = false
    })
}

async function submitJoin(): Promise<void> {
  if (!joinTarget.value || !joinFormValid.value || actionPending.value) return
  actionPending.value = true
  const declared = joinForm.declaredQuota as number
  try {
    const result = joinInviteToken.value
      ? await carpoolAPI.joinByInvite(joinInviteToken.value, declared)
      : await carpoolAPI.join(joinTarget.value.id, declared)
    joinDialogOpen.value = false
    activeTab.value = 'mine'
    appStore.showSuccess(
      result.prepaidAmountCny > 0
        ? t('carpool.joinDialog.success', { amount: result.prepaidAmountCny.toFixed(1) })
        : t('carpool.joinDialog.successNoPrepaid')
    )
    await loadCarpools()
  } catch (error) {
    appStore.showError(carpoolError(error))
  } finally {
    actionPending.value = false
  }
}

function requestLaunch(carpool: Carpool): void {
  confirmAction.value = { type: 'launch', carpool }
}

function requestForceLaunch(carpool: Carpool): void {
  confirmAction.value = { type: 'forceLaunch', carpool }
}

function requestConfirm(carpool: Carpool): void {
  if (!launchReady(carpool)) return
  confirmAction.value = { type: 'confirm', carpool }
}

function requestLeave(carpool: Carpool): void {
  confirmAction.value = { type: 'leave', carpool }
}

function requestCancel(carpool: Carpool): void {
  confirmAction.value = { type: 'cancel', carpool }
}

function requestUnconfirm(carpool: Carpool): void {
  confirmAction.value = { type: 'unconfirm', carpool }
}

async function runConfirmedAction(): Promise<void> {
  if (!confirmAction.value || actionPending.value) return
  const action = confirmAction.value
  confirmAction.value = null
  actionPending.value = true
  try {
    if (action.type === 'cancel') {
      await carpoolAPI.cancel(action.carpool.id)
      appStore.showSuccess(t('carpool.cancelDialog.success'))
    } else if (action.type === 'leave') {
      await carpoolAPI.leave(action.carpool.id)
      appStore.showSuccess(t('carpool.leaveDialog.success'))
    } else if (action.type === 'confirm') {
      await carpoolAPI.confirm(action.carpool.id)
      appStore.showSuccess(t('carpool.confirmDialog.success'))
    } else if (action.type === 'unconfirm') {
      await carpoolAPI.unconfirm(action.carpool.id)
      appStore.showSuccess(t('carpool.unconfirmDialog.success'))
    } else {
      await carpoolAPI.launch(action.carpool.id, action.type === 'forceLaunch')
      appStore.showSuccess(t('carpool.launchDialog.success'))
    }
    await loadCarpools()
  } catch (error) {
    appStore.showError(carpoolError(error))
  } finally {
    actionPending.value = false
  }
}

async function openSettlement(carpool: Carpool): Promise<void> {
  settlementDialogOpen.value = true
  settlementLoading.value = true
  settlementData.value = null
  settlementTarget.value = carpool
  try {
    settlementData.value = await carpoolAPI.settlement(carpool.id)
  } catch (error) {
    settlementDialogOpen.value = false
    appStore.showError(carpoolError(error, 'carpool.settlement.loadFailed'))
  } finally {
    settlementLoading.value = false
  }
}

// 冻结结算单：之后所有人读到的都是这一份，车主可以按它收退款。
async function settleCarpool(): Promise<void> {
  const target = settlementTarget.value
  if (!target || settlePending.value) return
  settlePending.value = true
  try {
    settlementData.value = await carpoolAPI.settle(target.id)
    appStore.showSuccess(t('carpool.settlement.settleSuccess'))
    await loadCarpools()
  } catch (error) {
    appStore.showError(carpoolError(error))
  } finally {
    settlePending.value = false
  }
}

// 撤销结算（仅 admin）：回到实时预览，可以重新结算。
async function unsettleCarpool(): Promise<void> {
  const target = settlementTarget.value
  if (!target || settlePending.value) return
  settlePending.value = true
  try {
    await carpoolAPI.unsettle(target.id)
    settlementData.value = await carpoolAPI.settlement(target.id)
    appStore.showSuccess(t('carpool.settlement.unsettleSuccess'))
    await loadCarpools()
  } catch (error) {
    appStore.showError(carpoolError(error))
  } finally {
    settlePending.value = false
  }
}

onBeforeUnmount(() => {
  for (const url of Object.values(qrCodeUrls.value)) {
    URL.revokeObjectURL(url)
  }
})

onMounted(async () => {
  await loadCarpools()
  const inviteToken = typeof route.params.token === 'string' ? route.params.token : ''
  if (!inviteToken) return
  try {
    const carpool = await carpoolAPI.resolveInvite(inviteToken)
    openJoin(carpool, inviteToken)
  } catch {
    appStore.showWarning(t('carpool.inviteNotFound'))
  } finally {
    await router.replace('/carpools')
  }
})
</script>
