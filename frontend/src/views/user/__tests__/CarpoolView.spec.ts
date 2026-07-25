import { ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CarpoolView from '../CarpoolView.vue'

const {
  replace,
  showWarning,
  listCarpools,
  createCarpoolMock,
  joinMock,
  joinByInviteMock,
  launchMock,
  leaveMock,
  confirmMock,
  groupQrCodeMock,
  recommendationMock,
  authState,
} = vi.hoisted(() => ({
  replace: vi.fn(),
  showWarning: vi.fn(),
  listCarpools: vi.fn(),
  createCarpoolMock: vi.fn(),
  joinMock: vi.fn(),
  joinByInviteMock: vi.fn(),
  launchMock: vi.fn(),
  leaveMock: vi.fn(),
  confirmMock: vi.fn(),
  groupQrCodeMock: vi.fn(),
  recommendationMock: vi.fn(),
  authState: { isAdmin: false },
}))

vi.mock('@/api/carpools', () => ({
  default: {
    list: listCarpools,
    create: createCarpoolMock,
    resolveInvite: vi.fn(),
    createInvite: vi.fn(),
    join: joinMock,
    joinByInvite: joinByInviteMock,
    leave: leaveMock,
    confirm: confirmMock,
    groupQrCode: groupQrCodeMock,
    launch: launchMock,
    declarationRecommendation: recommendationMock,
    settlement: vi.fn(),
    cancel: vi.fn(),
    setJoinLocked: vi.fn(),
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: ref('zh'),
      t: (key: string) => key,
    }),
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: {} }),
  useRouter: () => ({ replace }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showError: vi.fn(),
    showWarning,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isAdmin: authState.isAdmin,
    user: { id: 1, username: 'preview-user' },
  }),
}))

function makeCarpool(overrides: Record<string, unknown> = {}) {
  return {
    id: 10,
    name: 'open-car',
    description: '',
    organizer: 'owner-a',
    platform: 'openai',
    planType: 'openai_pro',
    carType: 'small',
    level: 1,
    capacity: 0,
    memberCount: 2,
    baseFeeCny: 0,
    usagePoolCnyPerAccount: 0,
    visibility: 'public',
    status: 'recruiting',
    joinLocked: false,
    scheduledStartAt: '2026-08-01',
    groupName: null,
    memberRole: null,
    createdAt: '2026-07-01T00:00:00Z',
    adminWechat: 'Charlemartingale',
    hasGroupQrCode: false,
    launchNotifiedAt: undefined,
    confirmedAt: undefined,
    weeklyLimitUsd: 2400,
    seatFeeCny: 400,
    usagePoolCny: 1000,
    reserveRatio: 0.8,
    launchMinRatio: 0.95,
    launchMaxRatio: 1.05,
    declaredTotalUsd: 1200,
    remainingJoinableUsd: 1320,
    plusEquivalents: 10,
    avgPriceCny: 90,
    ...overrides,
  }
}

function mountView() {
  return shallowMount(CarpoolView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: {
          name: 'BaseDialog',
          props: ['show', 'title', 'width'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: {
          name: 'ConfirmDialog',
          props: ['show', 'title', 'message', 'confirmText', 'cancelText', 'danger'],
          emits: ['confirm', 'cancel'],
          template:
            '<div v-if="show" data-testid="confirm-dialog">' +
            '<p data-testid="confirm-dialog-message">{{ message }}</p>' +
            '<button data-testid="confirm-dialog-ok" @click="$emit(\'confirm\')">ok</button>' +
            '</div>',
        },
        Icon: true,
      },
    },
  })
}

function findButton(wrapper: ReturnType<typeof mountView>, text: string) {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) throw new Error(`button not found: ${text}`)
  return button
}

describe('CarpoolView', () => {
  beforeEach(() => {
    localStorage.clear()
    replace.mockReset()
    showWarning.mockReset()
    listCarpools.mockReset()
    listCarpools.mockResolvedValue([])
    createCarpoolMock.mockReset()
    joinMock.mockReset()
    joinByInviteMock.mockReset()
    launchMock.mockReset()
    leaveMock.mockReset()
    confirmMock.mockReset()
    groupQrCodeMock.mockReset()
    groupQrCodeMock.mockResolvedValue(new Blob(['qr'], { type: 'image/png' }))
    recommendationMock.mockReset()
    recommendationMock.mockResolvedValue({
      recommendedWeeklyQuotaUsd: 0,
      rawWeeklyUsageUsd: 0,
      bufferRatio: 0,
      daysWithRecords: 0,
      basis: 'usage_history',
      message: '',
    })
    authState.isAdmin = false
    URL.createObjectURL = vi.fn(() => 'blob:qr') as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as typeof URL.revokeObjectURL
  })

  it('shows the quota reservation rules and does not seed any car', async () => {
    const wrapper = mountView()
    await flushPromises()
    const rules = wrapper.get('[data-testid="gpt-carpool-rules"]')

    expect(rules.text()).toContain('carpool.rules.declare.text')
    expect(rules.text()).toContain('carpool.rules.reserve.text')
    expect(rules.text()).toContain('carpool.rules.pricing.text')
    expect(rules.text()).toContain('carpool.rules.floor.text')
    expect(rules.text()).toContain('carpool.notices.weeklyRefresh')
    expect(rules.text()).toContain('carpool.notices.consumeOrder')
    expect(wrapper.findAll('article')).toHaveLength(0)
    expect(listCarpools).toHaveBeenCalledOnce()
  })

  it('renders joined state only from the current user membership returned by the API', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool(),
      makeCarpool({
        id: 11,
        name: 'joined-car',
        organizer: 'owner-b',
        memberCount: 3,
        memberRole: 'member',
      }),
    ])

    const wrapper = mountView()
    await flushPromises()
    const cards = wrapper.findAll('article')

    expect(cards).toHaveLength(2)
    expect(cards[0].text()).toContain('carpool.actions.join')
    expect(cards[0].text()).not.toContain('carpool.actions.joined')
    expect(cards[1].text()).toContain('carpool.actions.joined')
  })

  it('shows quota reservation metrics instead of seat counts on the card', async () => {
    listCarpools.mockResolvedValue([makeCarpool()])

    const wrapper = mountView()
    await flushPromises()
    const card = wrapper.get('article')

    expect(card.text()).toContain('carpool.fields.quotaProgress')
    expect(card.text()).toContain('carpool.fields.remainingJoinable')
    expect(card.text()).toContain('carpool.fields.plusEquivalents')
    expect(card.text()).toContain('carpool.fields.avgPrice')
    expect(card.text()).not.toContain('carpool.fields.seatsRemaining')
  })

  it('hides the join action when no joinable quota remains', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ remainingJoinableUsd: 0, declaredTotalUsd: 2520 }),
    ])

    const wrapper = mountView()
    await flushPromises()
    const card = wrapper.get('article')

    expect(card.text()).not.toContain('carpool.actions.join')
    expect(card.text()).toContain('carpool.status.full')
  })

  it('shows the owner an enabled confirm button inside the band and a disabled one below the line', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 20, name: 'ready-car', memberRole: 'owner', declaredTotalUsd: 2350, remainingJoinableUsd: 170 }),
      makeCarpool({ id: 21, name: 'early-car', memberRole: 'owner', declaredTotalUsd: 960, remainingJoinableUsd: 1560 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    const readyButton = wrapper.get('[data-testid="confirm-20"]')
    expect(readyButton.attributes('disabled')).toBeUndefined()

    const earlyButton = wrapper.get('[data-testid="confirm-21"]')
    expect(earlyButton.attributes('disabled')).toBeDefined()

    // owner 不再有直接发车入口
    expect(wrapper.find('[data-testid="launch-20"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="force-launch-20"]').exists()).toBe(false)
  })

  it('shows a leave button to joined members instead of the confirm button', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 22, memberRole: 'member', declaredTotalUsd: 2350 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="confirm-22"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="leave-22"]').exists()).toBe(true)
  })

  it('lets a member leave after a confirmation that warns about quota release', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 22, memberRole: 'member' })])
    leaveMock.mockResolvedValue(makeCarpool({ id: 22 }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="leave-22"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm-dialog-message"]').text()).toContain('carpool.leaveDialog.message')

    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(leaveMock).toHaveBeenCalledWith(22)
  })

  it('lets the owner confirm the launch inside the band', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 20, memberRole: 'owner', declaredTotalUsd: 2350, remainingJoinableUsd: 170 }),
    ])
    confirmMock.mockResolvedValue(makeCarpool({ id: 20, status: 'confirmed', joinLocked: true }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="confirm-20"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm-dialog-message"]').text()).toContain('carpool.confirmDialog.message')

    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(confirmMock).toHaveBeenCalledWith(20)
  })

  it('shows admins a launch button on confirmed carpools with the pending badge', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 30, status: 'confirmed', joinLocked: true, confirmedAt: '2026-07-20T00:00:00Z' }),
    ])
    launchMock.mockResolvedValue(makeCarpool({ id: 30, status: 'active' }))

    const wrapper = mountView()
    await flushPromises()
    const card = wrapper.get('article')

    expect(card.text()).toContain('carpool.status.confirmed')
    await wrapper.get('[data-testid="launch-30"]').trigger('click')
    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(launchMock).toHaveBeenCalledWith(30, false)
  })

  it('keeps the force launch entry for admins above 80% on recruiting carpools', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([makeCarpool({ id: 31, declaredTotalUsd: 2000, remainingJoinableUsd: 520 })])
    launchMock.mockResolvedValue(makeCarpool({ id: 31, status: 'active' }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="force-launch-31"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm-dialog-message"]').text()).toContain('carpool.launchDialog.forceMessage')

    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(launchMock).toHaveBeenCalledWith(31, true)
  })

  it('requires the admin wechat confirmation and a group qr code before creating', async () => {
    createCarpoolMock.mockResolvedValue({ carpool: makeCarpool({ id: 40 }), inviteToken: 'token-40' })

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.create').trigger('click')
    const submit = () => wrapper.get('button[form="carpool-create-form"]')

    // 名称/日期填好，但两项确认未完成 → 禁提交
    await wrapper.get('#carpool-name').setValue('weekend-car')
    expect(submit().attributes('disabled')).toBeDefined()

    // 只勾选已添加管理员微信，缺二维码 → 仍禁提交
    await wrapper.get('#carpool-added-admin').setValue(true)
    expect(submit().attributes('disabled')).toBeDefined()

    const file = new File([new Uint8Array([137, 80, 78, 71])], 'qr.png', { type: 'image/png' })
    const qrInput = wrapper.get('#carpool-group-qr')
    Object.defineProperty(qrInput.element, 'files', { value: [file], configurable: true })
    await qrInput.trigger('change')
    await flushPromises()
    // FileReader.onload 异步完成后再等一拍
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(submit().attributes('disabled')).toBeUndefined()

    await wrapper.get('#carpool-create-form').trigger('submit')
    await flushPromises()

    expect(createCarpoolMock).toHaveBeenCalledOnce()
    const payload = createCarpoolMock.mock.calls[0][0] as Record<string, unknown>
    expect(payload.added_admin_wechat).toBe(true)
    expect(String(payload.group_qr_code)).toMatch(/^data:image\/png/)
    // 高级设置已移除：池参数不再由前端提交
    expect(payload.weekly_limit_usd).toBeUndefined()
    expect(payload.seat_fee_cny).toBeUndefined()
  })

  it('rejects an oversized group qr code in the create dialog', async () => {
    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.create').trigger('click')
    const bigFile = new File([new Uint8Array(2 * 1024 * 1024 + 1)], 'big.png', { type: 'image/png' })
    const qrInput = wrapper.get('#carpool-group-qr')
    Object.defineProperty(qrInput.element, 'files', { value: [bigFile], configurable: true })
    await qrInput.trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.createDialog.qrTooLarge')
    expect(wrapper.get('button[form="carpool-create-form"]').attributes('disabled')).toBeDefined()
  })

  it('requires the group join confirmation before boarding', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, hasGroupQrCode: true })])
    joinMock.mockResolvedValue({ carpool: makeCarpool({ id: 10, memberRole: 'member' }), prepaidAmountCny: 0 })

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    // 对话框展示群二维码与管理员微信
    expect(wrapper.text()).toContain('carpool.joinDialog.groupSection')
    expect(wrapper.text()).toContain('Charlemartingale')
    expect(groupQrCodeMock).toHaveBeenCalledWith(10)

    const confirmButton = () => findButton(wrapper, 'carpool.joinDialog.confirm')
    await wrapper.get('#carpool-join-quota').setValue(100)
    // 未勾选"我已加入群聊" → 禁提交
    expect(confirmButton().attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-join-group').setValue(true)
    expect(confirmButton().attributes('disabled')).toBeUndefined()

    await confirmButton().trigger('click')
    await flushPromises()
    expect(joinMock).toHaveBeenCalledWith(10, 100)
  })
})
