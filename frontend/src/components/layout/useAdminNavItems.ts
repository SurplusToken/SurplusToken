import { computed, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAdminSettingsStore, useAuthStore } from '@/stores'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import {
  applyFeatureFlags,
  type NavItem,
  DashboardIcon,
  KeyIcon,
  ChartIcon,
  GiftIcon,
  UsersIcon,
  ChannelIcon,
  CreditCardIcon,
  GlobeIcon,
  SignalIcon,
} from './useUserNavItems'

export const FolderIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z'
        })
      ]
    )
}

export const ServerIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z'
        })
      ]
    )
}

export const BellIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75V9a6 6 0 10-12 0v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0'
        })
      ]
    )
}

export const TicketIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M16.5 6v.75m0 3v.75m0 3v.75m0 3V18m-9-5.25h5.25M7.5 15h3M3.375 5.25c-.621 0-1.125.504-1.125 1.125v3.026a2.999 2.999 0 010 5.198v3.026c0 .621.504 1.125 1.125 1.125h17.25c.621 0 1.125-.504 1.125-1.125v-3.026a2.999 2.999 0 010-5.198V6.375c0-.621-.504-1.125-1.125-1.125H3.375z'
        })
      ]
    )
}

export const CogIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z'
        })
      ]
    )
}

export const ShieldIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z'
        })
      ]
    )
}

export const PriceTagIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z'
        }),
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M6 6h.008v.008H6V6z'
        })
      ]
    )
}

export const OrderIcon = {
  render: () =>
    h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75 2.25 2.25 0 00-.1-.664m-5.8 0A2.251 2.251 0 0113.5 2.25H15a2.25 2.25 0 012.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25zM6.75 12h.008v.008H6.75V12zm0 3h.008v.008H6.75V15zm0 3h.008v.008H6.75V18z'
        })
      ]
    )
}

/**
 * 管理端导航项（含 feature flag / simple mode 过滤）。
 * 菜单定义与原 AppSidebar 完全一致，仅为迁移；由 AppTopNav 消费。
 */
export function useAdminNavItems() {
  const { t } = useI18n()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()

  // Public-settings flags go through the registry in utils/featureFlags.ts,
  // which handles the opt-in vs opt-out fallback when settings haven't loaded
  // yet. Admin-only flags (not in public settings) stay inline below.
  const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
  const flagRiskControl = makeSidebarFlag(FeatureFlags.riskControl)
  const flagAffiliate = makeSidebarFlag(FeatureFlags.affiliate)
  const flagOpsMonitoring = () => adminSettingsStore.opsMonitoringEnabled
  const flagAdminPayment = () => adminSettingsStore.paymentEnabled

  const customMenuItemsForAdmin = computed(() => {
    return adminSettingsStore.customMenuItems
      .filter((item) => item.visibility === 'admin')
      .sort((a, b) => a.sort_order - b.sort_order)
  })

  // Admin navigation items
  const adminNavItems = computed((): NavItem[] => {
    const baseItems: NavItem[] = [
      { path: '/admin/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
      { path: '/admin/ops', label: t('nav.ops'), icon: ChartIcon, featureFlag: flagOpsMonitoring },
      { path: '/admin/users', label: t('nav.users'), icon: UsersIcon, hideInSimpleMode: true },
      { path: '/admin/groups', label: t('nav.groups'), icon: FolderIcon, hideInSimpleMode: true },
      {
        path: '/admin/channels',
        label: t('nav.channelManagement'),
        icon: ChannelIcon,
        hideInSimpleMode: true,
        expandOnly: true,
        children: [
          { path: '/admin/channels/pricing', label: t('nav.channelPricing'), icon: PriceTagIcon },
          { path: '/admin/channels/monitor', label: t('nav.channelMonitor'), icon: SignalIcon, featureFlag: flagChannelMonitor },
        ],
      },
      { path: '/admin/subscriptions', label: t('nav.subscriptions'), icon: CreditCardIcon, hideInSimpleMode: true },
      { path: '/admin/carpools', label: t('nav.carpoolAdmin'), icon: UsersIcon, hideInSimpleMode: true },
      { path: '/admin/accounts', label: t('nav.accounts'), icon: GlobeIcon },
      { path: '/admin/announcements', label: t('nav.announcements'), icon: BellIcon },
      { path: '/admin/proxies', label: t('nav.proxies'), icon: ServerIcon },
      {
        path: '/admin/security-audit',
        label: t('nav.securityAudit'),
        icon: ShieldIcon,
        hideInSimpleMode: true,
        expandOnly: true,
        featureFlag: flagRiskControl,
        children: [
          { path: '/admin/risk-control', label: t('nav.contentModeration'), icon: ShieldIcon },
          { path: '/admin/prompt-audit', label: t('nav.promptAudit'), icon: ShieldIcon },
        ],
      },
      { path: '/admin/redeem', label: t('nav.redeemCodes'), icon: TicketIcon, hideInSimpleMode: true },
      { path: '/admin/promo-codes', label: t('nav.promoCodes'), icon: GiftIcon, hideInSimpleMode: true },
      { path: '/admin/contribution-withdrawals', label: t('nav.contributionWithdrawals'), icon: CreditCardIcon, hideInSimpleMode: true },
      {
        path: '/admin/affiliates',
        label: t('nav.affiliateManagement'),
        icon: UsersIcon,
        hideInSimpleMode: true,
        expandOnly: true,
        featureFlag: flagAffiliate,
        children: [
          { path: '/admin/affiliates/invites', label: t('nav.affiliateInviteRecords'), icon: UsersIcon },
          { path: '/admin/affiliates/rebates', label: t('nav.affiliateRebateRecords'), icon: OrderIcon },
          { path: '/admin/affiliates/transfers', label: t('nav.affiliateTransferRecords'), icon: CreditCardIcon },
        ],
      },
      {
        path: '/admin/orders',
        label: t('nav.orderManagement'),
        icon: OrderIcon,
        hideInSimpleMode: true,
        expandOnly: true,
        featureFlag: flagAdminPayment,
        children: [
          { path: '/admin/orders/dashboard', label: t('nav.paymentDashboard'), icon: ChartIcon },
          { path: '/admin/orders', label: t('nav.orderManagement'), icon: OrderIcon },
          { path: '/admin/orders/plans', label: t('nav.paymentPlans'), icon: CreditCardIcon },
        ],
      },
      { path: '/admin/usage', label: t('nav.usage'), icon: ChartIcon },
      { path: '/admin/audit-logs', label: t('nav.auditLogs'), icon: ShieldIcon, hideInSimpleMode: true }
    ]

    const visible = applyFeatureFlags(baseItems)

    // 简单模式下，在系统设置前插入 API密钥
    if (authStore.isSimpleMode) {
      const filtered = visible.filter(item => !item.hideInSimpleMode)
      filtered.push({ path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon })
      filtered.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
      for (const cm of customMenuItemsForAdmin.value) {
        filtered.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
      }
      return filtered
    }

    visible.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
    for (const cm of customMenuItemsForAdmin.value) {
      visible.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
    }
    return visible
  })

  return { adminNavItems }
}
