import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts')
const routerSource = readFileSync(routerPath, 'utf8')

describe('payment route guard', () => {
  it('does not hard-block user payment routes through SurplusAI internal restrictions', () => {
    const match = routerSource.match(/const SURPLUSAI_INTERNAL_RESTRICTED_PATHS = \[([\s\S]*?)\]/)
    expect(match).not.toBeNull()

    const restrictedBlock = match?.[1] ?? ''
    expect(restrictedBlock).not.toContain("'/purchase'")
    expect(restrictedBlock).not.toContain("'/orders'")
    expect(restrictedBlock).not.toContain("'/payment/qrcode'")
    expect(restrictedBlock).not.toContain("'/payment/stripe'")
    expect(restrictedBlock).not.toContain("'/payment/airwallex'")
    expect(restrictedBlock).not.toContain("'/payment/stripe-popup'")
  })

  it('only blocks requiresPayment routes on a confirmed payment_enabled=false, not a transient load failure', () => {
    expect(routerSource).toContain('to.meta.requiresPayment &&')
    expect(routerSource).toContain('appStore.publicSettingsLoaded &&')
    expect(routerSource).toContain('appStore.cachedPublicSettings?.payment_enabled === false')
  })
})
