<template>
  <!-- Balance Display -->
  <div
    v-if="user"
    class="group relative hidden items-center gap-2 rounded-lg bg-primary-50 px-3 py-1.5 dark:bg-primary-500/10 sm:flex"
  >
    <svg
      class="h-4 w-4 text-primary-600 dark:text-primary-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="1.5"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"
      />
    </svg>
    <span class="text-sm font-semibold text-primary-700 dark:text-primary-300">
      {{ formatHeaderMoney(availableBalance) }}
    </span>
    <span
      v-if="frozenBalance > 0"
      class="rounded-full bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
    >
      {{ balanceFrozenLabel }}
    </span>
    <div
      class="pointer-events-none absolute right-0 top-full mt-2 hidden w-56 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-lg group-hover:block dark:border-dark-700 dark:bg-dark-800"
    >
      <div class="flex items-center justify-between">
        <span class="text-gray-500 dark:text-dark-400">{{ balanceAvailableText }}</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ formatHeaderMoney(availableBalance) }}</span>
      </div>
      <div class="mt-2 flex items-center justify-between">
        <span class="text-gray-500 dark:text-dark-400">{{ balanceFrozenText }}</span>
        <span class="font-medium text-amber-700 dark:text-amber-200">{{ formatHeaderMoney(frozenBalance) }}</span>
      </div>
      <div class="mt-2 border-t border-gray-100 pt-2 dark:border-dark-700">
        <div class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-dark-400">{{ balanceTotalText }}</span>
          <span class="font-semibold text-gray-900 dark:text-white">{{ formatHeaderMoney(totalBalance) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { formatHeaderMoney, useHeaderBalance } from './useHeaderBalance'

const authStore = useAuthStore()
const user = computed(() => authStore.user)

const {
  availableBalance,
  frozenBalance,
  totalBalance,
  balanceAvailableText,
  balanceFrozenText,
  balanceTotalText,
  balanceFrozenLabel,
} = useHeaderBalance()
</script>
