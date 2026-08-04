import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const userNavItemsSource = readFileSync(resolve(dir, '../useUserNavItems.ts'), 'utf8')
const adminNavItemsSource = readFileSync(resolve(dir, '../useAdminNavItems.ts'), 'utf8')

describe('user nav payment entries', () => {
  it('shows purchase and orders according to the payment feature flag', () => {
    expect(userNavItemsSource).toContain('const flagPayment = makeSidebarFlag(FeatureFlags.payment)')
    expect(userNavItemsSource).toContain("{ path: '/purchase', label: t('nav.buySubscription'), icon: RechargeSubscriptionIcon, hideInSimpleMode: true, featureFlag: flagPayment }")
    expect(userNavItemsSource).toContain("{ path: '/orders', label: t('nav.myOrders'), icon: OrderListIcon, hideInSimpleMode: true, featureFlag: flagPayment }")
  })
})

describe('admin nav security group', () => {
  it('keeps risk-control and prompt-audit under an expand-only security group', () => {
    const group = adminNavItemsSource.slice(
      adminNavItemsSource.indexOf("path: '/admin/security-audit'"),
      adminNavItemsSource.indexOf("path: '/admin/redeem'")
    )
    expect(group).toContain('expandOnly: true')
    expect(group).toContain("path: '/admin/risk-control'")
    expect(group).toContain("path: '/admin/prompt-audit'")
  })
})
