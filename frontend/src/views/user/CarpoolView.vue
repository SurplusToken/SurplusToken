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
            <Icon name="lock" size="xs" />
            {{ t('carpool.rules.monthlyLock') }}
          </span>
        </div>

        <div class="grid lg:grid-cols-2 lg:divide-x lg:divide-amber-200 dark:lg:divide-amber-900/70">
          <div class="px-4 py-3">
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('carpool.types.small') }}</h3>
              <span class="text-xs font-medium text-gray-600 dark:text-dark-200">{{ t('carpool.rules.small.capacity') }}</span>
            </div>
            <p class="mt-1 text-xs leading-5 text-gray-600 dark:text-dark-200">{{ t('carpool.rules.small.upgrade') }}</p>
            <div class="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-700 dark:text-dark-100">
              <span>{{ t('carpool.rules.accountCost') }}</span>
              <span>{{ t('carpool.rules.small.baseFee') }}</span>
              <strong class="font-mono font-semibold text-amber-800 dark:text-amber-300">{{ t('carpool.rules.small.usageFee') }}</strong>
            </div>
          </div>

          <div class="border-t border-amber-200 px-4 py-3 dark:border-amber-900/70 lg:border-t-0">
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('carpool.types.large') }}</h3>
              <span class="text-xs font-medium text-gray-600 dark:text-dark-200">{{ t('carpool.rules.large.capacity') }}</span>
            </div>
            <p class="mt-1 text-xs leading-5 text-gray-600 dark:text-dark-200">{{ t('carpool.rules.large.upgrade') }}</p>
            <div class="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-700 dark:text-dark-100">
              <span>{{ t('carpool.rules.accountCost') }}</span>
              <span>{{ t('carpool.rules.large.baseFee') }}</span>
              <strong class="font-mono font-semibold text-amber-800 dark:text-amber-300">{{ t('carpool.rules.large.usageFee') }}</strong>
            </div>
          </div>
        </div>

        <p class="border-t border-amber-200 px-4 py-2.5 text-xs leading-5 text-amber-900 dark:border-amber-900/70 dark:text-amber-200">
          {{ t('carpool.rules.lockNotice') }}
        </p>
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
                GPT · {{ carTypeLabel(carpool.carType) }} · {{ t('carpool.level', { level: carpool.level }) }}
              </span>
            </div>

            <div class="mt-4">
              <div class="mb-2 flex items-center justify-between text-xs">
                <span class="font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.fields.members') }} {{ carpool.memberCount }} / {{ carpool.capacity }}</span>
                <span class="text-gray-500 dark:text-dark-300">{{ t('carpool.fields.seatsRemaining', { count: remainingSeats(carpool) }) }}</span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                <div
                  class="h-full rounded-full transition-all"
                  :class="seatProgressClass(carpool)"
                  :style="{ width: `${seatProgress(carpool)}%` }"
                />
              </div>
            </div>

            <dl class="mt-5 grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.organizer') }}</dt>
                <dd class="mt-1 truncate font-medium text-gray-700 dark:text-dark-100">{{ carpool.organizer }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.scheduledStart') }}</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">{{ formatDate(carpool.scheduledStartAt) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.fields.visibility') }}</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-100">{{ visibilityLabel(carpool.visibility) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-400 dark:text-dark-400">{{ t('carpool.detailDialog.linkedGroup') }}</dt>
                <dd class="mt-1 truncate font-medium text-gray-700 dark:text-dark-100">{{ carpool.groupName || t('carpool.detailDialog.pendingGroup') }}</dd>
              </div>
            </dl>

            <div class="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
              <div class="flex flex-wrap gap-2">
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
                @click="requestJoin(carpool)"
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
        <fieldset>
          <legend class="input-label">{{ t('carpool.fields.carType') }}</legend>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="option in carTypeOptions"
              :key="option.value"
              type="button"
              class="flex min-h-12 flex-col items-center justify-center rounded-lg border px-3 py-2 text-sm font-medium transition-colors"
              :class="createForm.carType === option.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:text-dark-200'"
              @click="createForm.carType = option.value"
            >
              <span>{{ option.label }}</span>
              <span class="mt-0.5 text-xs font-normal opacity-75">{{ option.hint }}</span>
            </button>
          </div>
        </fieldset>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label" for="carpool-level">{{ t('carpool.fields.level') }}</label>
            <input id="carpool-level" v-model.number="createForm.level" type="number" min="1" max="10" step="1" class="input" required />
          </div>
          <div>
            <label class="input-label" for="carpool-start">{{ t('carpool.fields.scheduledStart') }}</label>
            <input id="carpool-start" v-model="createForm.scheduledStartAt" type="date" class="input" required />
          </div>
        </div>
        <div class="grid grid-cols-3 divide-x divide-gray-200 border-y border-gray-200 py-3 text-center dark:divide-dark-600 dark:border-dark-600">
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.fields.accounts') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ createForm.level }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.fields.capacity') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ createCapacity }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-400">{{ t('carpool.fields.totalCost') }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ createTotalCost }}</div>
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

    <BaseDialog :show="inviteDialogOpen" :title="t('carpool.inviteDialog.title')" width="normal" @close="inviteDialogOpen = false">
      <div v-if="selectedCarpool" class="space-y-4">
        <div class="flex items-center justify-between gap-3 border-b border-gray-100 pb-3 dark:border-dark-700">
          <div>
            <div class="font-semibold text-gray-900 dark:text-white">{{ selectedCarpool.name }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ t('carpool.inviteDialog.uses', { used: selectedCarpool.memberCount, max: selectedCarpool.capacity }) }}</div>
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
            <span>{{ t('carpool.detailDialog.progress') }}</span>
            <span class="font-medium">{{ selectedCarpool.memberCount }} / {{ selectedCarpool.capacity }}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div class="h-full rounded-full" :class="seatProgressClass(selectedCarpool)" :style="{ width: `${seatProgress(selectedCarpool)}%` }" />
          </div>
        </div>
        <dl class="grid grid-cols-2 gap-4 border-y border-gray-100 py-4 text-sm dark:border-dark-700">
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

    <ConfirmDialog
      :show="confirmAction !== null"
      :title="confirmAction?.type === 'join' ? t('carpool.joinDialog.title') : t('carpool.cancelDialog.title')"
      :message="confirmMessage"
      :confirm-text="confirmAction?.type === 'join' ? t('carpool.joinDialog.confirm') : t('carpool.cancelDialog.confirm')"
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
import carpoolAPI, { type Carpool, type CarpoolType, type CarpoolVisibility } from '@/api/carpools'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

interface CreateForm {
  name: string
  description: string
  carType: CarpoolType
  level: number
  visibility: CarpoolVisibility
  scheduledStartAt: string
}

interface ConfirmAction {
  type: 'join' | 'cancel'
  carpool: Carpool
  inviteToken?: string
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
const inviteDialogOpen = ref(false)
const detailDialogOpen = ref(false)
const selectedCarpool = ref<Carpool | null>(null)
const selectedInviteToken = ref('')
const confirmAction = ref<ConfirmAction | null>(null)
const carpools = ref<Carpool[]>([])

const isoDateAfterDays = (days: number): string => {
  const date = new Date(Date.now() + days * 24 * 60 * 60 * 1000)
  return date.toISOString().slice(0, 10)
}

const newCreateForm = (): CreateForm => ({
  name: '',
  description: '',
  carType: 'small',
  level: 1,
  visibility: 'public',
  scheduledStartAt: isoDateAfterDays(7)
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
const carTypeOptions = computed(() => [
  { value: 'small' as const, label: t('carpool.types.small'), hint: t('carpool.types.smallHint') },
  { value: 'large' as const, label: t('carpool.types.large'), hint: t('carpool.types.largeHint') }
])
const createCapacity = computed(() => createForm.level * (createForm.carType === 'small' ? 5 : 10))
const createTotalCost = computed(() => createForm.level * 1400)
const stats = computed(() => [
  { label: t('carpool.stats.recruiting'), value: carpools.value.filter((item) => item.status === 'recruiting' && !item.joinLocked).length },
  { label: t('carpool.stats.seats'), value: carpools.value.reduce((sum, item) => sum + (canJoin(item) ? remainingSeats(item) : 0), 0) },
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
  && Number.isInteger(createForm.level)
  && createForm.level >= 1
  && createForm.level <= 10
  && createForm.scheduledStartAt.length > 0
))
const confirmMessage = computed(() => {
  if (!confirmAction.value) return ''
  return confirmAction.value.type === 'join'
    ? t('carpool.joinDialog.message', { name: confirmAction.value.carpool.name })
    : t('carpool.cancelDialog.message', { name: confirmAction.value.carpool.name })
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

function remainingSeats(carpool: Carpool): number {
  return Math.max(0, carpool.capacity - carpool.memberCount)
}

function seatProgress(carpool: Carpool): number {
  return Math.min(100, Math.round((carpool.memberCount / carpool.capacity) * 100))
}

function canJoin(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' && !carpool.joinLocked && remainingSeats(carpool) > 0
}

function canInvite(carpool: Carpool): boolean {
  return (carpool.memberRole === 'owner' || authStore.isAdmin) && canJoin(carpool)
}

function canCancel(carpool: Carpool): boolean {
  return carpool.memberRole === 'owner' && (carpool.status === 'recruiting' || carpool.status === 'starting')
}

function statusLabel(carpool: Carpool): string {
  if (carpool.status === 'recruiting' && carpool.joinLocked) return t('carpool.status.locked')
  if (carpool.status === 'recruiting' && remainingSeats(carpool) === 0) return t('carpool.status.full')
  return t(`carpool.status.${carpool.status}`)
}

function statusBadgeClass(carpool: Carpool): string {
  if (carpool.status === 'cancelled' || carpool.status === 'ended') return 'badge-gray'
  if (carpool.status === 'active') return 'badge-success'
  if (carpool.joinLocked || remainingSeats(carpool) === 0) return 'badge-warning'
  return 'badge-primary'
}

function seatProgressClass(carpool: Carpool): string {
  if (carpool.status === 'active') return 'bg-emerald-500'
  if (carpool.joinLocked || remainingSeats(carpool) === 0) return 'bg-amber-500'
  return 'bg-primary-500'
}

function visibilityLabel(visibility: CarpoolVisibility): string {
  return visibility === 'public' ? t('carpool.visibility.public') : t('carpool.visibility.inviteOnly')
}

function carTypeLabel(carType: CarpoolType): string {
  return t(`carpool.types.${carType}`)
}

function formatDate(value: string): string {
  if (!value) return '-'
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    month: 'short', day: 'numeric', year: 'numeric'
  }).format(new Date(`${value}T12:00:00`))
}

function openCreateDialog(): void {
  Object.assign(createForm, newCreateForm())
  createDialogOpen.value = true
}

async function createCarpool(): Promise<void> {
  if (!createFormValid.value || actionPending.value) return
  actionPending.value = true
  try {
    const result = await carpoolAPI.create({
      name: createForm.name,
      description: createForm.description,
      car_type: createForm.carType,
      level: createForm.level,
      visibility: createForm.visibility,
      scheduled_start_at: createForm.scheduledStartAt
    })
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

function requestJoin(carpool: Carpool, inviteToken = ''): void {
  if (!canJoin(carpool) || carpool.memberRole) {
    appStore.showWarning(t('carpool.unavailable'))
    return
  }
  confirmAction.value = { type: 'join', carpool, inviteToken }
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
    if (action.type === 'join') {
      if (action.inviteToken) await carpoolAPI.joinByInvite(action.inviteToken)
      else await carpoolAPI.join(action.carpool.id)
      activeTab.value = 'mine'
      appStore.showSuccess(t('carpool.joinDialog.success'))
    } else {
      await carpoolAPI.cancel(action.carpool.id)
      appStore.showSuccess(t('carpool.cancelDialog.success'))
    }
    await loadCarpools()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('carpool.actionFailed')))
  } finally {
    actionPending.value = false
  }
}

onMounted(async () => {
  await loadCarpools()
  const inviteToken = typeof route.params.token === 'string' ? route.params.token : ''
  if (!inviteToken) return
  try {
    const carpool = await carpoolAPI.resolveInvite(inviteToken)
    requestJoin(carpool, inviteToken)
  } catch {
    appStore.showWarning(t('carpool.inviteNotFound'))
  } finally {
    await router.replace('/carpools')
  }
})
</script>
