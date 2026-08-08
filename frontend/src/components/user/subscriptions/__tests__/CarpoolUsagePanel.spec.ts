import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { CarpoolUsageSnapshot } from '@/types'
import enMessages from '@/i18n/locales/en/misc'
import zhMessages from '@/i18n/locales/zh/misc'
import CarpoolUsagePanel from '../CarpoolUsagePanel.vue'

vi.mock('vue-i18n', () => {
  const messages: Record<string, string> = {
    'userSubscriptions.carpoolUsage.totalUsage': '总车用量',
    'userSubscriptions.carpoolUsage.sharedPool': '公共池',
    'userSubscriptions.carpoolUsage.me': '我',
    'userSubscriptions.carpoolUsage.member': '成员 {number}',
    'userSubscriptions.carpoolUsage.declaredQuota': '申报总额度 {amount}',
    'userSubscriptions.carpoolUsage.memberQuota': '成员额度 {amount}',
    'userSubscriptions.carpoolUsage.remaining': '剩余 {amount}',
    'userSubscriptions.carpoolUsage.sharedPoolUsed': '已使用公共池 {amount}',
    'userSubscriptions.carpoolUsage.loading': '正在加载拼车用量',
    'userSubscriptions.carpoolUsage.loadFailed': '加载拼车用量失败',
    'userSubscriptions.carpoolUsage.retry': '重试',
  }

  return {
    useI18n: () => ({
      locale: { value: 'zh' },
      t: (key: string, params: Record<string, string | number> = {}) =>
        (messages[key] ?? key).replace(/\{(\w+)\}/g, (_match, name: string) => String(params[name] ?? '')),
    }),
  }
})

const snapshot = {
  subscriptionId: 42,
  windowStart: '2026-08-03T00:00:00Z',
  windowEnd: '2026-08-10T00:00:00Z',
  totalUsageUsd: 1697.2,
  totalCapacityUsd: 2400,
  sharedPool: {
    usageUsd: 28,
    capacityUsd: 480,
    remainingUsd: 452,
  },
  members: [
    {
      memberNumber: 1,
      isCurrentUser: false,
      declaredQuotaUsd: 700,
      reservedQuotaUsd: 560,
      usageUsd: 588,
      sharedPoolUsageUsd: 28,
      email: 'member-one@example.com',
    },
    {
      memberNumber: 0,
      isCurrentUser: true,
      declaredQuotaUsd: 600,
      reservedQuotaUsd: 480,
      usageUsd: 426.4,
      sharedPoolUsageUsd: 0,
      username: 'current-user',
    },
    {
      memberNumber: 2,
      isCurrentUser: false,
      declaredQuotaUsd: 500,
      reservedQuotaUsd: 400,
      usageUsd: 100,
      sharedPoolUsageUsd: 0,
      username: 'another-member',
    },
  ],
} as unknown as CarpoolUsageSnapshot

function mountPanel(props: Record<string, unknown>) {
  return mount(CarpoolUsagePanel, {
    props,
  })
}

describe('CarpoolUsagePanel', () => {
  it('renders total, shared pool, current user, then anonymous members', () => {
    const wrapper = mountPanel({ snapshot })
    const rows = wrapper.findAll('[data-testid="usage-row"]')

    expect(rows.map((row) => row.attributes('data-usage-kind'))).toEqual([
      'total',
      'shared-pool',
      'current-user',
      'member-1',
      'member-2',
    ])
    expect(rows.map((row) => row.get('[data-testid="usage-label"]').text())).toEqual([
      '总车用量',
      '公共池',
      '我',
      '成员 1',
      '成员 2',
    ])

    expect(rows[0].text()).toContain('$1,697.20 / $2,400.00')
    expect(rows[1].text()).toContain('$28.00 / $480.00')
    expect(rows[1].text()).toContain('剩余 $452.00')
    expect(rows[2].text()).toContain('申报总额度 $600.00')
    expect(rows[2].text()).toContain('成员额度 $480.00')
    expect(rows[3].text()).toContain('申报总额度 $700.00')
    expect(rows[3].text()).toContain('成员额度 $560.00')
    expect(rows[3].text()).toContain('已使用公共池 $28.00')

    expect(wrapper.text()).not.toContain('member-one@example.com')
    expect(wrapper.text()).not.toContain('current-user')
    expect(wrapper.text()).not.toContain('another-member')
  })

  it('clamps progress at 100 and applies distinct semantic colors', () => {
    const wrapper = mountPanel({ snapshot })
    const progress = wrapper.findAll('[role="progressbar"]')

    expect(progress).toHaveLength(5)
    for (const bar of progress) {
      expect(bar.attributes('aria-valuemin')).toBe('0')
      expect(bar.attributes('aria-valuemax')).toBe('100')
    }

    expect(progress[0].get('[data-testid="progress-fill"]').classes()).toContain('bg-amber-500')
    expect(progress[1].get('[data-testid="progress-fill"]').classes()).toContain('bg-sky-500')
    expect(progress[2].get('[data-testid="progress-fill"]').classes()).toContain('bg-emerald-500')
    expect(progress[3].attributes('aria-valuenow')).toBe('100')
    expect(progress[3].get('[data-testid="progress-fill"]').attributes('style')).toContain('width: 100%')
    expect(progress[3].get('[data-testid="progress-fill"]').classes()).toContain('bg-red-500')
    expect(progress[4].get('[data-testid="progress-fill"]').classes()).toContain('bg-gray-400')

    const exhaustedLabel = wrapper.get('[data-usage-kind="member-1"] [data-testid="usage-label"]')
    expect(exhaustedLabel.classes()).toContain('text-red-700')
    expect(exhaustedLabel.classes()).not.toContain('text-gray-700')
  })

  it('treats positive usage with zero member quota as shared-pool usage', () => {
    const zeroQuotaSnapshot = {
      ...snapshot,
      members: [
        {
          memberNumber: 1,
          isCurrentUser: false,
          declaredQuotaUsd: 0,
          reservedQuotaUsd: 0,
          usageUsd: 12,
          sharedPoolUsageUsd: 12,
        },
      ],
    }
    const wrapper = mountPanel({ snapshot: zeroQuotaSnapshot })
    const row = wrapper.get('[data-usage-kind="member-1"]')
    const progress = row.get('[role="progressbar"]')

    expect(progress.attributes('aria-valuenow')).toBe('100')
    expect(progress.get('[data-testid="progress-fill"]').classes()).toContain('bg-red-500')
    expect(row.get('[data-testid="usage-label"]').classes()).toContain('text-red-700')
    expect(row.text()).toContain('已使用公共池 $12.00')
  })

  it('keeps loading and error states stable and emits retry', async () => {
    const loading = mountPanel({ loading: true })
    expect(loading.get('[data-testid="carpool-usage-loading"]').attributes('aria-busy')).toBe('true')
    expect(loading.get('[data-testid="carpool-usage-panel"]').classes()).toContain('min-h-[18rem]')

    const failed = mountPanel({ error: '网络超时' })
    expect(failed.get('[role="alert"]').text()).toContain('网络超时')
    await failed.get('button').trigger('click')
    expect(failed.emitted('retry')).toHaveLength(1)
  })

  it('keeps amount and label regions responsive', () => {
    const wrapper = mountPanel({ snapshot })
    const header = wrapper.get('[data-testid="usage-row-header"]')

    expect(header.classes()).toEqual(expect.arrayContaining(['flex-wrap', 'gap-x-4', 'gap-y-1']))
    expect(header.get('[data-testid="usage-amount"]').classes()).toContain('break-words')
  })

  it('defines concise Chinese and English copy', () => {
    expect(zhMessages.userSubscriptions.carpoolUsage).toMatchObject({
      totalUsage: '总车用量',
      sharedPool: '公共池',
      me: '我',
      member: '成员 {number}',
      declaredQuota: '申报总额度 {amount}',
      memberQuota: '成员额度 {amount}',
      sharedPoolUsed: '已使用公共池 {amount}',
    })
    expect(enMessages.userSubscriptions.carpoolUsage).toMatchObject({
      totalUsage: 'Total car usage',
      sharedPool: 'Shared pool',
      me: 'Me',
      member: 'Member {number}',
      declaredQuota: 'Declared quota {amount}',
      memberQuota: 'Member quota {amount}',
      sharedPoolUsed: 'Used shared pool {amount}',
    })
  })
})
