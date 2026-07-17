import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import TablePageLayout from '../TablePageLayout.vue'

describe('TablePageLayout', () => {
  it('supports document-level vertical scrolling for dense pages', () => {
    const wrapper = mount(TablePageLayout, {
      props: { pageScrollable: true },
      slots: { table: '<div data-testid="table-content">rows</div>' },
    })

    expect(wrapper.classes()).toContain('page-scrollable')
    expect(wrapper.get('[data-testid="table-content"]').exists()).toBe(true)
  })
})
