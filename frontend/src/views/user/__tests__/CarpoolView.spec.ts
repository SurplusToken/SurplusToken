import { ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CarpoolView from '../CarpoolView.vue'

const {
  replace,
  showSuccess,
  showWarning,
  listCarpools,
  createCarpoolMock,
  joinMock,
  joinByInviteMock,
  launchMock,
  leaveMock,
  confirmMock,
  groupQrCodeMock,
  rosterMock,
  recommendationMock,
  notifyCustomRuleInterestMock,
  unconfirmMock,
  pendingLaunchMock,
  createInviteMock,
  settlementMock,
  settleMock,
  unsettleMock,
  authState,
} = vi.hoisted(() => ({
  replace: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  listCarpools: vi.fn(),
  createCarpoolMock: vi.fn(),
  joinMock: vi.fn(),
  joinByInviteMock: vi.fn(),
  launchMock: vi.fn(),
  leaveMock: vi.fn(),
  confirmMock: vi.fn(),
  groupQrCodeMock: vi.fn(),
  rosterMock: vi.fn(),
  recommendationMock: vi.fn(),
  notifyCustomRuleInterestMock: vi.fn(),
  unconfirmMock: vi.fn(),
  pendingLaunchMock: vi.fn(),
  createInviteMock: vi.fn(),
  settlementMock: vi.fn(),
  settleMock: vi.fn(),
  unsettleMock: vi.fn(),
  authState: { isAdmin: false },
}))

vi.mock('@/api/carpools', () => ({
  default: {
    list: listCarpools,
    create: createCarpoolMock,
    resolveInvite: vi.fn(),
    createInvite: createInviteMock,
    join: joinMock,
    joinByInvite: joinByInviteMock,
    leave: leaveMock,
    confirm: confirmMock,
    unconfirm: unconfirmMock,
    pendingLaunch: pendingLaunchMock,
    groupQrCode: groupQrCodeMock,
    roster: rosterMock,
    launch: launchMock,
    declarationRecommendation: recommendationMock,
    notifyCustomRuleInterest: notifyCustomRuleInterestMock,
    settlement: settlementMock,
    settle: settleMock,
    unsettle: unsettleMock,
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
    showSuccess,
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
    pricingModel: 'quota',
    ruleNote: '',
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
    showSuccess.mockReset()
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
    rosterMock.mockReset()
    rosterMock.mockResolvedValue([])
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
    notifyCustomRuleInterestMock.mockReset()
    notifyCustomRuleInterestMock.mockResolvedValue(undefined)
    unconfirmMock.mockReset()
    pendingLaunchMock.mockReset()
    pendingLaunchMock.mockResolvedValue([])
    createInviteMock.mockReset()
    settlementMock.mockReset()
    settleMock.mockReset()
    unsettleMock.mockReset()
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
    expect(rules.text()).toContain('carpool.notices.customRule')
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
    expect(card.text()).toContain('carpool.fields.effectiveRate')
    expect(card.text()).toContain('carpool.fields.carMonthlyFee')
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

  it('custom rule mode disables the form, notifies the admin, and never creates', async () => {
    let resolveNotify: () => void = () => {}
    notifyCustomRuleInterestMock.mockImplementation(
      () => new Promise<void>((resolve) => { resolveNotify = resolve })
    )
    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.create').trigger('click')
    // 默认规则：表单可用、提交按钮存在
    expect(wrapper.get('[data-testid="rule-mode-default"]').attributes('class')).toContain('border-primary-500')
    expect(wrapper.get('#carpool-name').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('button[form="carpool-create-form"]').exists()).toBe(true)

    await wrapper.get('[data-testid="rule-mode-custom"]').trigger('click')

    // 自定义模式：表单其余项全部禁用，且不展示创建提交按钮
    expect(wrapper.get('#carpool-name').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#carpool-description').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#carpool-start').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#carpool-owner-quota').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#carpool-added-admin').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#carpool-group-qr').attributes('disabled')).toBeDefined()
    expect(wrapper.find('button[form="carpool-create-form"]').exists()).toBe(false)

    // 指引区块 + 通知管理员按钮
    expect(wrapper.get('[data-testid="custom-rule-panel"]').text()).toContain('carpool.createDialog.customRule.title')
    const notifyButton = wrapper.get('[data-testid="custom-rule-notify"]')
    await notifyButton.trigger('click')
    // loading 期间按钮禁用
    expect(wrapper.get('[data-testid="custom-rule-notify"]').attributes('disabled')).toBeDefined()
    resolveNotify()
    await flushPromises()

    expect(notifyCustomRuleInterestMock).toHaveBeenCalledOnce()
    expect(showSuccess).toHaveBeenCalledWith('carpool.createDialog.customRule.notifySuccess')
    // 成功后展示管理员微信号与复制入口
    const panel = wrapper.get('[data-testid="custom-rule-panel"]')
    expect(panel.text()).toContain('Charlemartingale')
    expect(panel.text()).toContain('common.copy')

    // 此模式下即使触发表单 submit 也不调用创建接口
    await wrapper.get('#carpool-create-form').trigger('submit')
    await flushPromises()
    expect(createCarpoolMock).not.toHaveBeenCalled()
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

  // 申报推荐是异步回填金额输入框的。迟到的响应绝不能覆盖用户已经改过的数字——
  // 否则用户会按一个自己没同意的额度上车，而这个额度直接决定保底和扣款。
  it('never overwrites a quota the user already typed with a late recommendation', async () => {
    let resolveRecommendation: (value: unknown) => void = () => {}
    recommendationMock.mockReturnValue(new Promise((resolve) => { resolveRecommendation = resolve }))
    listCarpools.mockResolvedValue([makeCarpool({ id: 10 })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    // 用户在推荐回来之前先自己填了金额
    await wrapper.get('#carpool-join-quota').setValue(250)

    resolveRecommendation({
      recommendedWeeklyQuotaUsd: 500,
      rawWeeklyUsageUsd: 450,
      bufferRatio: 1.1,
      daysWithRecords: 7,
      basis: 'usage_history',
      message: '',
    })
    await flushPromises()

    expect((wrapper.get('#carpool-join-quota').element as HTMLInputElement).value).toBe('250')
  })

  // 申报下限：低于下限时禁止提交并给出提示（后端也会拒，这里是即时反馈）。
  it('blocks joining with a declaration below the floor', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10 })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    await wrapper.get('#carpool-join-quota').setValue(5)
    await wrapper.get('#carpool-join-group').setValue(true)

    expect(wrapper.text()).toContain('carpool.joinDialog.belowFloor')
    expect(findButton(wrapper, 'carpool.joinDialog.confirm').attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-join-quota').setValue(20)
    expect(findButton(wrapper, 'carpool.joinDialog.confirm').attributes('disabled')).toBeUndefined()
  })

  // 加入对话框展示"你的折算单价"而不是全车均价：席位费按人头均摊，
  // 申报越小单价越高，均价对轻度用户是系统性低估。
  it('previews the joiner own effective rate instead of the car average', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, memberCount: 3, avgPriceCny: 90 })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.joinDialog.previewEffectiveRate')
    expect(wrapper.text()).not.toContain('carpool.joinDialog.previewAvgPrice')

    // 申报 $60：预付 = 400/4 + 1000×60/2400 = ¥125，31 天可用 60×31/7 = $265.7，
    // 等效倍率 = 125/265.7 ≈ 0.47；全车 = 1400/(2400×31/7) ≈ 0.13，约 3.6 倍 → 触发提示。
    await wrapper.get('#carpool-join-quota').setValue(60)
    expect(wrapper.text()).toContain('carpool.joinDialog.rateAboveAverage')
  })

  // 上车前要看得见"车上有几个人、席位费怎么分、别人各报了多少"——
  // 这三件事直接决定自己该报多少，只给一个合计预付是看不出来的。
  it('shows the roster and how the seat fee is split before joining', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, memberCount: 2 })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'owner-zhao', role: 'owner', declaredWeeklyQuotaUsd: 300 },
      { userId: 2, username: '', role: 'member', declaredWeeklyQuotaUsd: 900 },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    expect(rosterMock).toHaveBeenCalledWith(10, undefined)
    const roster = wrapper.get('[data-testid="carpool-join-roster"]')
    expect(roster.text()).toContain('owner-zhao')
    expect(roster.text()).toContain('$300')
    expect(roster.text()).toContain('$900')
    // 生产上确实有用户名为空的成员，不能渲染成一行空白
    expect(roster.text()).toContain('carpool.joinDialog.rosterAnonymous')
    // 席位费按"现有 2 人 + 我"= 3 人分摊，预付也拆成席位 + 用量两部分
    expect(wrapper.text()).toContain('carpool.joinDialog.seatShareHint')
    expect(wrapper.text()).toContain('carpool.joinDialog.prepaidBreakdown')
  })

  // 花名册只是参考信息，接口挂了不该把人挡在车外。
  it('degrades gracefully when the roster fails to load', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10 })])
    rosterMock.mockRejectedValue(new Error('boom'))

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.joinDialog.rosterFailed')
    expect(wrapper.find('[data-testid="carpool-join-roster"]').exists()).toBe(false)
    // 申报输入和预付试算照常工作——花名册失败不影响主流程
    await wrapper.get('#carpool-join-quota').setValue(100)
    expect(wrapper.text()).toContain('carpool.joinDialog.previewEffectiveRate')
  })

  // confirmed 的车原来在前端是死胡同：车主没有任何入口，只能等 admin。
  it('lets the owner withdraw a confirmation on a confirmed carpool', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, status: 'confirmed', memberRole: 'owner', joinLocked: true }),
    ])
    unconfirmMock.mockResolvedValue(makeCarpool({ id: 10, status: 'recruiting', memberRole: 'owner' }))

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.actions.unconfirm').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()

    expect(unconfirmMock).toHaveBeenCalledWith(10)
    expect(showSuccess).toHaveBeenCalledWith('carpool.unconfirmDialog.success')
  })

  // admin 待启动列表：确认通知只有一封邮件，邮件丢了车就无限挂起。
  it('shows the admin pending-launch banner with overdue marking', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, status: 'confirmed', joinLocked: true })])
    pendingLaunchMock.mockResolvedValue([{
      carpoolId: 10,
      name: 'stuck-car',
      ownerUserId: 3,
      ownerEmail: 'owner@example.com',
      memberCount: 8,
      declaredTotalUsd: 2400,
      weeklyLimitUsd: 2400,
      confirmedAt: '2026-07-20T00:00:00Z',
      pendingHours: 51.5,
      overdue: true,
    }])

    const wrapper = mountView()
    await flushPromises()

    const banner = wrapper.get('[data-testid="carpool-pending-launch"]')
    expect(banner.text()).toContain('stuck-car')
    expect(banner.text()).toContain('owner@example.com')
    expect(banner.text()).toContain('carpool.pendingLaunch.overdue')
  })

  it('hides the pending-launch banner from non-admins', async () => {
    authState.isAdmin = false
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, status: 'confirmed' })])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="carpool-pending-launch"]').exists()).toBe(false)
    expect(pendingLaunchMock).not.toHaveBeenCalled()
  })

  // 结算单未冻结时是实时预览（金额还会随用量走），车主能看到"确认结算"；
  // 冻结后换成"已结算"提示，结算按钮消失。
  it('offers settling on a live preview and shows the frozen banner afterwards', async () => {
    const car = makeCarpool({ id: 10, status: 'active', memberRole: 'owner' })
    listCarpools.mockResolvedValue([car])

    const live = {
      carpoolId: 10, status: 'active', weeklyLimitUsd: 2400, seatFeeCny: 400, usagePoolCny: 1000,
      reserveRatio: 0.8, memberCount: 2, fullView: true, members: [],
      settled: false, canSettle: true, settleBlockedFor: '',
    }
    settlementMock.mockResolvedValue(live)
    settleMock.mockResolvedValue({ ...live, settled: true, canSettle: false, settleBlockedFor: 'already_settled', settledAt: '2026-08-01T00:00:00Z' })

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.actions.settlement').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="carpool-settlement-live"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('carpool.settlement.livePreview')

    await findButton(wrapper, 'carpool.settlement.settle').trigger('click')
    await flushPromises()

    expect(settleMock).toHaveBeenCalledWith(10)
    expect(wrapper.find('[data-testid="carpool-settlement-frozen"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="carpool-settlement-live"]').exists()).toBe(false)
  })

  // 撤销结算只对 admin 出现——普通车主结算错了要找管理员。
  it('shows the undo-settlement action only to admins', async () => {
    const car = makeCarpool({ id: 10, status: 'active', memberRole: 'owner' })
    listCarpools.mockResolvedValue([car])
    settlementMock.mockResolvedValue({
      carpoolId: 10, status: 'active', weeklyLimitUsd: 2400, seatFeeCny: 400, usagePoolCny: 1000,
      reserveRatio: 0.8, memberCount: 2, fullView: true, members: [],
      settled: true, canSettle: false, settleBlockedFor: 'already_settled', settledAt: '2026-08-01T00:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.settlement').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="carpool-settlement-frozen"]').exists()).toBe(true)
    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.settlement.unsettle'))).toBe(false)
  })

  // 车主取消之后，车不该继续挂在「我的拼车」里——那会让人以为取消没生效。
  // 广场页本来就排除了已取消，这里曾经漏掉，只是不一致。
  it('hides a cancelled carpool from my-carpools by default', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'live-car', status: 'recruiting', memberRole: 'owner' }),
      makeCarpool({ id: 11, name: 'cancelled-car', status: 'cancelled', memberRole: 'owner' }),
    ])

    const wrapper = mountView()
    await flushPromises()

    // 切到「我的拼车」
    await findButton(wrapper, 'carpool.mine').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('live-car')
    expect(wrapper.text()).not.toContain('cancelled-car')
  })

  // 但要回看历史时，显式选「已取消」筛选仍然看得到。
  it('shows cancelled carpools when the status filter asks for them', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'live-car', status: 'recruiting', memberRole: 'owner' }),
      makeCarpool({ id: 11, name: 'cancelled-car', status: 'cancelled', memberRole: 'owner' }),
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.mine').trigger('click')
    await wrapper.get('select').setValue('cancelled')
    await flushPromises()

    expect(wrapper.text()).toContain('cancelled-car')
    expect(wrapper.text()).not.toContain('live-car')
  })

  // 广场页同样不出现已取消的车。
  it('keeps cancelled carpools out of the plaza', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 11, name: 'cancelled-car', status: 'cancelled', memberRole: null }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('cancelled-car')
  })

  // 下车之后必须能重新上车。此前 member_role 子查询不排除 status='left'，
  // 前端一直以为用户还在车上：「已上车」不消失、「上车」按钮不出现、
  // 车也退不出「我的拼车」——而后端其实一直允许重新上车。
  it('offers joining again after the member has left', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'left-car', status: 'recruiting', memberRole: null }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.actions.join'))).toBe(true)
    expect(wrapper.text()).not.toContain('carpool.actions.joined')
  })

  // 车主即使没有成员行也要能在「我的拼车」里看到自己的车：发车时申报为 0 的
  // 车主会被置成 'left'，memberRole 因此为 null，只按 memberRole 过滤会让他
  // 看不见自己发起的车、也就没法取消它。
  it('keeps an owned carpool in my-carpools even without a member row', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'owned-car', status: 'recruiting', memberRole: null, ownerUserId: 1 }),
      makeCarpool({ id: 11, name: 'stranger-car', status: 'recruiting', memberRole: null, ownerUserId: 999 }),
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.mine').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('owned-car')
    expect(wrapper.text()).not.toContain('stranger-car')
  })

  // 自定义规则车（含平台升级前建立的老车）：展示规则说明，不渲染额度进度与
  // 均价——它们的成员申报恒为 0，硬渲染出来就是 "0 / 2400"、"均价 ¥0"。
  it('renders the rule note instead of a quota bar for custom-rule carpools', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10,
      status: 'active',
      memberRole: 'member',
      pricingModel: 'custom',
      ruleNote: '旧版席位规则：共 5 席，基础费 ¥130/席。',
    })])

    const wrapper = mountView()
    await flushPromises()

    const box = wrapper.get('[data-testid="carpool-custom-rule"]')
    expect(box.text()).toContain('carpool.customRule.badge')
    expect(box.text()).toContain('基础费 ¥130/席')
    expect(wrapper.text()).not.toContain('carpool.fields.quotaProgress')
    expect(wrapper.text()).not.toContain('carpool.fields.carMonthlyFee')
    expect(wrapper.text()).not.toContain('carpool.fields.remainingJoinable')
  })

  // 自定义规则车不接新成员：升级前遗留的招募中老车永远达不到发车区间，
  // 再放人进去就是往开不走的车里交预付。
  it('never offers joining a custom-rule carpool', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10, status: 'recruiting', memberRole: null, pricingModel: 'custom',
    })])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.actions.join'))).toBe(false)
  })

  // 人工结算：只列实际用量，不出退补列。
  it('shows a usage-only settlement for custom-rule carpools', async () => {
    const car = makeCarpool({ id: 10, status: 'active', memberRole: 'owner', pricingModel: 'custom' })
    listCarpools.mockResolvedValue([car])
    settlementMock.mockResolvedValue({
      carpoolId: 10, status: 'active', weeklyLimitUsd: 2400, seatFeeCny: 400, usagePoolCny: 1000,
      reserveRatio: 0.8, memberCount: 1, fullView: true,
      members: [{
        userId: 12, role: 'member', declaredWeeklyQuotaUsd: 0, floorUsageUsd: 0,
        actualUsageUsd: 123.4, billableUsageUsd: 0, floorTriggered: false,
        prepaidAmountCny: 0, quotedPrepaidCny: 0, usagePrepaidCny: 0, usageFinalShareCny: 0,
        usageDeltaCny: 0, seatFeePrepaidCny: 0, seatFeeFinalCny: 0, seatFeeDeltaCny: 0, totalDeltaCny: 0,
      }],
      settled: false, canSettle: false, settleBlockedFor: 'manual_settlement',
      manualSettlement: true, pricingModel: 'custom', ruleNote: '旧版席位规则：共 5 席。',
    })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.settlement').trigger('click')
    await flushPromises()

    const banner = wrapper.get('[data-testid="carpool-settlement-manual"]')
    expect(banner.text()).toContain('carpool.settlement.manualTitle')
    expect(banner.text()).toContain('旧版席位规则')
    // 退补相关的列与提示一律不出现
    expect(wrapper.text()).not.toContain('carpool.settlement.delta')
    expect(wrapper.text()).not.toContain('carpool.settlement.prepaid')
    expect(wrapper.text()).not.toContain('carpool.settlement.deltaNote')
    // 但用量要在
    expect(wrapper.text()).toContain('carpool.settlement.actual')
    // 也不该出现"确认结算"入口
    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.settlement.settle'))).toBe(false)
  })

  // 连点两辆车的"邀请"：先发的请求后回来时不能覆盖对话框，
  // 否则会显示 B 车、链接却是 A 车的邀请码，分享出去就是错的车。
  it('discards a stale invite response when another carpool is clicked', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'car-a' }),
      makeCarpool({ id: 11, name: 'car-b' }),
    ])

    let resolveA: (value: string) => void = () => {}
    let resolveB: (value: string) => void = () => {}
    createInviteMock
      .mockReturnValueOnce(new Promise<string>((resolve) => { resolveA = resolve }))
      .mockReturnValueOnce(new Promise<string>((resolve) => { resolveB = resolve }))

    const wrapper = mountView()
    await flushPromises()

    const inviteButtons = wrapper.findAll('button').filter((item) => item.text().includes('carpool.actions.invite'))
    expect(inviteButtons.length).toBe(2)
    await inviteButtons[0].trigger('click')
    await inviteButtons[1].trigger('click')

    // B 先回，A 后回（乱序）
    resolveB('token-b')
    await flushPromises()
    resolveA('token-a')
    await flushPromises()

    const link = (wrapper.get('#carpool-invite-link').element as HTMLInputElement).value
    expect(link).toContain('token-b')
    expect(link).not.toContain('token-a')
    expect(wrapper.text()).toContain('car-b')
  })
})
