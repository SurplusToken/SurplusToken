import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

export function formatHeaderMoney(value: number) {
  if (!Number.isFinite(value)) return '$0.00'
  return `$${value.toFixed(2)}`
}

/**
 * 头部余额展示共享逻辑（AppTopNav 的 BalancePill / UserMenu 共用）。
 */
export function useHeaderBalance() {
  const { t } = useI18n()
  const authStore = useAuthStore()

  const user = computed(() => authStore.user)
  const availableBalance = computed(() => Number(user.value?.balance || 0))
  const frozenBalance = computed(() => Number(user.value?.frozen_balance || 0))
  const totalBalance = computed(() => availableBalance.value + frozenBalance.value)
  const balanceAvailableText = computed(() => t('common.availableBalance') === 'common.availableBalance' ? '可用余额' : t('common.availableBalance'))
  const balanceFrozenText = computed(() => t('common.frozenBalance') === 'common.frozenBalance' ? '冻结金额' : t('common.frozenBalance'))
  const balanceTotalText = computed(() => t('common.totalBalance') === 'common.totalBalance' ? '总余额' : t('common.totalBalance'))
  const balanceFrozenLabel = computed(() => `${balanceFrozenText.value} ${formatHeaderMoney(frozenBalance.value)}`)

  return {
    availableBalance,
    frozenBalance,
    totalBalance,
    balanceAvailableText,
    balanceFrozenText,
    balanceTotalText,
    balanceFrozenLabel,
  }
}
