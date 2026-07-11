import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileSharingRateRangeCard from '../ProfileSharingRateRangeCard.vue'

const { getSharingRateRange, updateSharingRateRange, showError, showSuccess } = vi.hoisted(() => ({
  getSharingRateRange: vi.fn(),
  updateSharingRateRange: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api', () => ({
  userAPI: { getSharingRateRange, updateSharingRateRange },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('ProfileSharingRateRangeCard', () => {
  beforeEach(() => {
    getSharingRateRange.mockReset()
    updateSharingRateRange.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getSharingRateRange.mockResolvedValue({ min: 0, max: 2 })
    updateSharingRateRange.mockResolvedValue({ min: null, max: 1.25 })
  })

  it('loads and allows preconfiguring the range while filtering is off', async () => {
    const wrapper = mount(ProfileSharingRateRangeCard, {
      props: { filterEnabled: false, floor: 0, cap: 5 },
    })
    await flushPromises()

    expect(getSharingRateRange).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="sharing-rate-range-disabled"]').exists()).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-testid="sharing-rate-min"]').element.disabled).toBe(false)
    expect(wrapper.get<HTMLButtonElement>('[data-testid="sharing-rate-range-save"]').element.disabled).toBe(false)

    await wrapper.get('[data-testid="sharing-rate-min"]').setValue('0.5')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(updateSharingRateRange).toHaveBeenCalledWith({ min: 0.5, max: 2 })
  })

  it('sends both keys and preserves an explicit unbounded minimum', async () => {
    const wrapper = mount(ProfileSharingRateRangeCard, {
      props: { filterEnabled: true, floor: 0, cap: 3 },
    })
    await flushPromises()

    await wrapper.get('[data-testid="sharing-rate-min"]').setValue('')
    await wrapper.get('[data-testid="sharing-rate-max"]').setValue('1.25')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateSharingRateRange).toHaveBeenCalledWith({ min: null, max: 1.25 })
    expect(showSuccess).toHaveBeenCalledWith('profile.sharingRateRange.saveSuccess')
  })

  it('rejects a reversed interval before calling the API', async () => {
    const wrapper = mount(ProfileSharingRateRangeCard, {
      props: { filterEnabled: true, floor: 0, cap: 5 },
    })
    await flushPromises()

    await wrapper.get('[data-testid="sharing-rate-min"]').setValue('2')
    await wrapper.get('[data-testid="sharing-rate-max"]').setValue('1')
    await wrapper.get('form').trigger('submit.prevent')

    expect(updateSharingRateRange).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('profile.sharingRateRange.invalidOrder')
  })
})
