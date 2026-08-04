import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const topNavSource = readFileSync(resolve(dir, '../AppTopNav.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')

describe('doc_url sanitization', () => {
  it('AppTopNav imports sanitizeUrl', () => {
    expect(topNavSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('AppTopNav uses the public SurplusToken docs URL', () => {
    expect(topNavSource).toContain("sanitizeUrl('https://docs.surplustoken.com')")
  })

  it('HomeView imports sanitizeUrl', () => {
    expect(homeViewSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('HomeView exposes the public SurplusToken docs URL without authentication', () => {
    expect(homeViewSource).toContain("sanitizeUrl('https://docs.surplustoken.com')")
    expect(homeViewSource).toContain(':href="docUrl"')
  })

  it('KeyUsageView imports sanitizeUrl', () => {
    expect(keyUsageViewSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('KeyUsageView applies sanitizeUrl to docUrl', () => {
    expect(keyUsageViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl')
  })
})
