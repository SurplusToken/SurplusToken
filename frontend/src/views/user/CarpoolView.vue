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
        </div>
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
              <span class="shrink-0 rounded-md border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 dark:border-dark-600 dark:text-dark-200">
                {{ t('carpool.fields.weeklyLimitBadge', { limit: formatUsd(carpool.weeklyLimitUsd) }) }}
              </span>
            </div>

            <div class="mt-4">
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
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.remainingJoinable') }}</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">{{ formatUsd(carpool.remainingJoinableUsd) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.plusEquivalents') }}</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">{{ formatDecimal(carpool.plusEquivalents) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.avgPrice') }}</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">
                  {{ formatCny(carpool.avgPriceCny) }}
                  <span class="block text-[10px] font-normal text-gray-400 dark:text-dark-400">{{ t('carpool.fields.avgPriceUnit') }}</span>
                </dd>
              </div>
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
                  v-if="canLaunch(carpool)"
                  type="button"
                  :data-testid="`launch-${carpool.id}`"
                  class="h-9 px-3 py-2"
                  :class="launchReady(carpool) ? 'btn btn-primary' : 'btn btn-secondary'"
                  :disabled="!launchReady(carpool) && !forceLaunchReady(carpool)"
                  :title="launchHint(carpool)"
                  @click="requestLaunch(carpool)"
                >
                  <Icon name="play" size="sm" />
                  <span>{{ forceLaunchReady(carpool) && !launchReady(carpool) ? t('carpool.actions.forceLaunch') : t('carpool.actions.launch') }}</span>
                </button>
                <span v-if="canLaunch(carpool) && !launchReady(carpool)" class="text-xs text-gray-500 dark:text-dark-300">{{ launchHint(carpool) }}</span>
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
        <div>
          <label class="input-label" for="carpool-name">{{ t('carpool.fields.name') }}</label>
          <input id="carpool-name" v-model.trim="createForm.name" class="input" maxlength="100" required :placeholder="t('carpool.fields.namePlaceholder')" />
        </div>
        <div>
          <label class="input-label" for="carpool-description">{{ t('carpool.fields.description') }}</label>
          <textarea id="carpool-description" v-model.trim="createForm.description" class="input min-h-20 resize-y" maxlength="300" :placeholder="t('carpool.fields.descriptionPlaceholder')" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label" for="carpool-start">{{ t('carpool.fields.scheduledStart') }}</label>
            <input id="carpool-start" v-model="createForm.scheduledStartAt" type="date" class="input" required />
          </div>
          <div>
            <label class="input-label" for="carpool-owner-quota">{{ t('carpool.createDialog.ownerQuota') }}</label>
            <input id="carpool-owner-quota" v-model.number="createForm.ownerQuota" type="number" min="0" step="1" class="input" />
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
              @click="createForm.visibility = visibility.value"
            >
              <Icon :name="visibility.value === 'public' ? 'globe' : 'lock'" size="sm" />
              {{ visibility.label }}
            </button>
          </div>
        </fieldset>
        <div class="rounded-lg border border-gray-200 dark:border-dark-600">
          <button type="button" class="flex w-full items-center justify-between px-3 py-2.5 text-sm font-medium text-gray-700 dark:text-dark-100" @click="advancedOpen = !advancedOpen">
            <span>{{ t('carpool.createDialog.advanced') }}</span>
            <Icon name="chevronDown" size="sm" class="transition-transform" :class="{ 'rotate-180': advancedOpen }" />
          </button>
          <div v-if="advancedOpen" class="space-y-3 border-t border-gray-200 px-3 py-3 dark:border-dark-600">
            <p class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.createDialog.advancedHint') }}</p>
            <div class="grid gap-3 sm:grid-cols-2">
              <div>
                <label class="input-label" for="carpool-weekly-limit">{{ t('carpool.createDialog.weeklyLimit') }}</label>
                <input id="carpool-weekly-limit" v-model.number="createForm.weeklyLimitUsd" type="number" min="1" step="1" class="input" />
              </div>
              <div>
                <label class="input-label" for="carpool-seat-fee">{{ t('carpool.createDialog.seatFee') }}</label>
                <input id="carpool-seat-fee" v-model.number="createForm.seatFeeCny" type="number" min="0" step="1" class="input" />
              </div>
              <div>
                <label class="input-label" for="carpool-usage-pool">{{ t('carpool.createDialog.usagePool') }}</label>
                <input id="carpool-usage-pool" v-model.number="createForm.usagePoolCny" type="number" min="0" step="1" class="input" />
              </div>
              <div>
                <label class="input-label" for="carpool-reserve-ratio">{{ t('carpool.createDialog.reserveRatio') }}</label>
                <input id="carpool-reserve-ratio" v-model.number="createForm.reserveRatio" type="number" min="0.01" max="1" step="0.05" class="input" />
              </div>
              <div>
                <label class="input-label" for="carpool-launch-min">{{ t('carpool.createDialog.launchMinRatio') }}</label>
                <input id="carpool-launch-min" v-model.number="createForm.launchMinRatio" type="number" min="0.01" step="0.01" class="input" />
              </div>
              <div>
                <label class="input-label" for="carpool-launch-max">{{ t('carpool.createDialog.launchMaxRatio') }}</label>
                <input id="carpool-launch-max" v-model.number="createForm.launchMaxRatio" type="number" min="0.01" step="0.01" class="input" />
              </div>
            </div>
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="createDialogOpen = false">{{ t('common.cancel') }}</button>
          <button type="submit" form="carpool-create-form" class="btn btn-primary" :disabled="!createFormValid || actionPending">
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
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ t('carpool.fields.members', { count: joinTarget.memberCount }) }}</div>
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

        <p v-if="joinExceedsRemaining" class="rounded-md bg-red-50 px-3 py-2 text-xs font-medium text-red-700 dark:bg-red-900/20 dark:text-red-300">
          {{ t('carpool.joinDialog.exceedsRemaining', { amount: formatUsd(joinTarget.remainingJoinableUsd) }) }}
        </p>

        <div class="grid grid-cols-3 divide-x divide-gray-200 rounded-lg border border-gray-200 py-3 text-center dark:divide-dark-600 dark:border-dark-600">
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.joinDialog.previewFloor') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatUsd(joinFloorQuota) }} {{ t('carpool.joinDialog.floorUnit') }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.joinDialog.previewPrepaid') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatCny(joinPrepaid) }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.joinDialog.previewAvgPrice') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatCny(joinTarget.avgPriceCny) }}</div>
          </div>
        </div>

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
          <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-600">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-600">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300">
                <tr>
                  <th class="px-3 py-2 text-left font-medium">{{ t('carpool.settlement.member') }}</th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.declared') }}</th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.actual') }}</th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.billable') }}</th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.prepaid') }}</th>
                  <th class="px-3 py-2 text-right font-medium">{{ t('carpool.settlement.delta') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="member in settlementData.members" :key="member.userId">
                  <td class="px-3 py-2">
                    <span class="font-medium text-gray-800 dark:text-dark-100">#{{ member.userId }}</span>
                    <span class="ml-1.5 text-xs text-gray-400">{{ t(`carpool.roles.${member.role}`) }}</span>
                  </td>
                  <td class="px-3 py-2 text-right font-mono">{{ formatDecimal(member.declaredWeeklyQuotaUsd) }}</td>
                  <td class="px-3 py-2 text-right font-mono">{{ formatDecimal(member.actualUsageUsd) }}</td>
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
                </tr>
              </tbody>
            </table>
          </div>
          <p class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.settlement.deltaNote') }}</p>
        </template>
      </div>
    </BaseDialog>

    <ConfirmDialog
      :show="confirmAction !== null"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-text="confirmText"
      :danger="confirmAction?.type === 'cancel'"
      @confirm="runConfirmedAction"
      @cancel="confirmAction = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
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
} from '@/api/carpools'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

// 与后端 CarpoolForceLaunchMinRatio 对齐：降档发车允许的最低总申报比例。
const FORCE_LAUNCH_MIN_RATIO = 0.8

interface CreateForm {
  name: string
  description: string
  visibility: CarpoolVisibility
  scheduledStartAt: string
  ownerQuota: number | null
  weeklyLimitUsd: number
  seatFeeCny: number
  usagePoolCny: number
  reserveRatio: number
  launchMinRatio: number
  launchMaxRatio: number
}

interface ConfirmAction {
  type: 'cancel' | 'launch' | 'forceLaunch'
  carpool: Carpool
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
const advancedOpen = ref(false)
const joinDialogOpen = ref(false)
const inviteDialogOpen = ref(false)
const detailDialogOpen = ref(false)
const settlementDialogOpen = ref(false)
const settlementLoading = ref(false)
const settlementData = ref<CarpoolSettlement | null>(null)
const selectedCarpool = ref<Carpool | null>(null)
const selectedInviteToken = ref('')
const joinTarget = ref<Carpool | null>(null)
const joinInviteToken = ref('')
const joinForm = reactive({ declaredQuota: null as number | null })
const recommendation = ref<DeclarationRecommendation | null>(null)
const recommendationLoading = ref(false)
const recommendationFailed = ref(false)
const confirmAction = ref<ConfirmAction | null>(null)
const carpools = ref<Carpool[]>([])

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
  weeklyLimitUsd: 2400,
  seatFeeCny: 400,
  usagePoolCny: 1000,
  reserveRatio: 0.8,
  launchMinRatio: 0.95,
  launchMaxRatio: 1.05,
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
  return carpools.value
    .filter((item) => activeTab.value === 'plaza' ? item.visibility === 'public' && item.status !== 'cancelled' : item.memberRole !== null)
    .filter((item) => !statusFilter.value || item.status === statusFilter.value)
    .filter((item) => !query || item.name.toLowerCase().includes(query) || item.organizer.toLowerCase().includes(query))
})
const createFormValid = computed(() => (
  createForm.name.length > 0
  && createForm.scheduledStartAt.length > 0
  && (createForm.ownerQuota === null || createForm.ownerQuota >= 0)
  && createForm.weeklyLimitUsd > 0
  && createForm.seatFeeCny > 0
  && createForm.usagePoolCny > 0
  && createForm.reserveRatio > 0
  && createForm.reserveRatio <= 1
  && createForm.launchMinRatio > 0
  && createForm.launchMinRatio <= createForm.launchMaxRatio
))
const joinFloorQuota = computed(() => {
  if (!joinTarget.value || !joinForm.declaredQuota || joinForm.declaredQuota <= 0) return 0
  return joinTarget.value.reserveRatio * joinForm.declaredQuota
})
const joinPrepaid = computed(() => {
  if (!joinTarget.value || !joinForm.declaredQuota || joinForm.declaredQuota <= 0) return 0
  const car = joinTarget.value
  const seatShare = car.seatFeeCny / Math.max(1, car.memberCount + 1)
  return seatShare + (car.weeklyLimitUsd > 0 ? car.usagePoolCny * joinForm.declaredQuota / car.weeklyLimitUsd : 0)
})
const joinExceedsRemaining = computed(() => (
  !!joinTarget.value && !!joinForm.declaredQuota && joinForm.declaredQuota > joinTarget.value.remainingJoinableUsd + 1e-9
))
const joinFormValid = computed(() => (
  !!joinForm.declaredQuota && joinForm.declaredQuota > 0 && !joinExceedsRemaining.value
))
const confirmTitle = computed(() => {
  if (!confirmAction.value) return ''
  if (confirmAction.value.type === 'cancel') return t('carpool.cancelDialog.title')
  return confirmAction.value.type === 'forceLaunch' ? t('carpool.launchDialog.forceTitle') : t('carpool.launchDialog.confirmTitle')
})
const confirmText = computed(() => {
  if (!confirmAction.value) return ''
  if (confirmAction.value.type === 'cancel') return t('carpool.cancelDialog.confirm')
  return t('carpool.launchDialog.confirm')
})
const confirmMessage = computed(() => {
  if (!confirmAction.value) return ''
  const action = confirmAction.value
  if (action.type === 'cancel') return t('carpool.cancelDialog.message', { name: action.carpool.name })
  const params = {
    name: action.carpool.name,
    total: formatUsd(action.carpool.declaredTotalUsd),
    ratio: launchRatioPercent(declaredRatio(action.carpool)),
  }
  return action.type === 'forceLaunch'
    ? t('carpool.launchDialog.forceMessage', params)
    : t('carpool.launchDialog.confirmMessage', params)
})

async function loadCarpools(): Promise<void> {
  try {
    carpools.value = await carpoolAPI.list()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.loadFailed')))
  } finally {
    loading.value = false
  }
}

function declaredRatio(carpool: Carpool): number {
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

function canJoin(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' && !carpool.joinLocked && carpool.remainingJoinableUsd > 0
}

function canInvite(carpool: Carpool): boolean {
  return (carpool.memberRole === 'owner' || authStore.isAdmin) && canJoin(carpool)
}

function canCancel(carpool: Carpool): boolean {
  return carpool.memberRole === 'owner' && (carpool.status === 'recruiting' || carpool.status === 'starting')
}

function canLaunch(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' && (carpool.memberRole === 'owner' || authStore.isAdmin)
}

function launchReady(carpool: Carpool): boolean {
  const ratio = declaredRatio(carpool)
  return ratio >= carpool.launchMinRatio && ratio <= carpool.launchMaxRatio
}

function forceLaunchReady(carpool: Carpool): boolean {
  const ratio = declaredRatio(carpool)
  return ratio >= FORCE_LAUNCH_MIN_RATIO && ratio < carpool.launchMinRatio
}

function launchHint(carpool: Carpool): string {
  if (launchReady(carpool)) return ''
  if (forceLaunchReady(carpool)) return t('carpool.launchDialog.forceReady')
  const missing = Math.max(0, carpool.launchMinRatio * carpool.weeklyLimitUsd - carpool.declaredTotalUsd)
  return t('carpool.launchDialog.notReady', {
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
  advancedOpen.value = false
  createDialogOpen.value = true
}

async function createCarpool(): Promise<void> {
  if (!createFormValid.value || actionPending.value) return
  actionPending.value = true
  try {
    const payload: CreateCarpoolRequest = {
      name: createForm.name,
      description: createForm.description,
      visibility: createForm.visibility,
      scheduled_start_at: createForm.scheduledStartAt,
    }
    if (createForm.ownerQuota && createForm.ownerQuota > 0) {
      payload.declared_weekly_quota_usd = createForm.ownerQuota
    }
    if (advancedOpen.value) {
      payload.weekly_limit_usd = createForm.weeklyLimitUsd
      payload.seat_fee_cny = createForm.seatFeeCny
      payload.usage_pool_cny = createForm.usagePoolCny
      payload.reserve_ratio = createForm.reserveRatio
      payload.launch_min_ratio = createForm.launchMinRatio
      payload.launch_max_ratio = createForm.launchMaxRatio
    }
    const result = await carpoolAPI.create(payload)
    createDialogOpen.value = false
    activeTab.value = 'mine'
    await loadCarpools()
    appStore.showSuccess(t('carpool.createDialog.success'))
    await openInvite(result.carpool, result.inviteToken)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.actionFailed')))
  } finally {
    actionPending.value = false
  }
}

async function openInvite(carpool: Carpool, token = ''): Promise<void> {
  try {
    selectedCarpool.value = carpool
    selectedInviteToken.value = token || await carpoolAPI.createInvite(carpool.id)
    inviteDialogOpen.value = true
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.actionFailed')))
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
    appStore.showError(extractApiErrorMessage(error, t('carpool.actionFailed')))
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
  recommendation.value = null
  recommendationFailed.value = false
  recommendationLoading.value = true
  joinDialogOpen.value = true
  carpoolAPI.declarationRecommendation()
    .then((rec) => {
      recommendation.value = rec
      if (rec.recommendedWeeklyQuotaUsd > 0) {
        joinForm.declaredQuota = Math.round(rec.recommendedWeeklyQuotaUsd * 10) / 10
      }
    })
    .catch(() => {
      recommendationFailed.value = true
    })
    .finally(() => {
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
    appStore.showError(extractApiErrorMessage(error, t('carpool.actionFailed')))
  } finally {
    actionPending.value = false
  }
}

function requestLaunch(carpool: Carpool): void {
  if (launchReady(carpool)) {
    confirmAction.value = { type: 'launch', carpool }
  } else if (forceLaunchReady(carpool)) {
    confirmAction.value = { type: 'forceLaunch', carpool }
  }
}

function requestCancel(carpool: Carpool): void {
  confirmAction.value = { type: 'cancel', carpool }
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
    } else {
      await carpoolAPI.launch(action.carpool.id, action.type === 'forceLaunch')
      appStore.showSuccess(t('carpool.launchDialog.success'))
    }
    await loadCarpools()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.actionFailed')))
  } finally {
    actionPending.value = false
  }
}

async function openSettlement(carpool: Carpool): Promise<void> {
  settlementDialogOpen.value = true
  settlementLoading.value = true
  settlementData.value = null
  try {
    settlementData.value = await carpoolAPI.settlement(carpool.id)
  } catch (error) {
    settlementDialogOpen.value = false
    appStore.showError(extractApiErrorMessage(error, t('carpool.settlement.loadFailed')))
  } finally {
    settlementLoading.value = false
  }
}

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
