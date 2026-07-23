export default {
  carpool: {
    title: '拼车',
    description: '发现正在招募的拼车，或发起一辆新车',
    create: '发起拼车',
    level: '{level} 级',
    types: {
      small: '小车',
      large: '大车',
      smallHint: '每级 1 个号、5 个席位',
      largeHint: '每级 1 个号、10 个席位'
    },
    rules: {
      title: 'GPT 拼车规则',
      monthlyLock: '开车时定级，本月锁定',
      accountCost: '每个号：1400',
      small: {
        title: '5 人精品小车（适合量大用户）',
        capacity: '每级 1 个号 · 5 人',
        upgrade: '1 级为 1 个号、5 人；2 级为 2 个号、10 人，之后每升 1 级增加 1 个号和 5 个席位。',
        baseFee: '基础费用：130 / 人',
        usageFee: '剩余费用：750 元 / 号，按成员的相对用量比例分摊'
      },
      large: {
        title: '10 人拼好车（适合中等用量用户，人均 2x Plus 用量）',
        capacity: '每级 1 个号 · 10 人',
        upgrade: '1 级为 1 个号、10 人；之后每升 1 级增加 1 个号和 10 个席位。',
        baseFee: '基础费用：65 / 人',
        usageFee: '剩余费用：750 元 / 号，按成员的相对用量比例分摊'
      },
      lockNotice: '开车前需同时确定车型和等级（账号数），确认后当月不再升降级。满员后自动创建同名 OpenAI 订阅分组，并为每位成员开通一个月订阅；倍率 1，不限制用量。'
    },
    plaza: '拼车广场',
    mine: '我的拼车',
    searchPlaceholder: '搜索拼车名称或发起人',
    allStatuses: '全部状态',
    stats: {
      recruiting: '正在招募',
      seats: '剩余座位',
      joined: '我已上车',
      launched: '已经开车'
    },
    status: {
      recruiting: '招募中',
      starting: '准备开车',
      active: '已开车',
      cancelled: '已取消',
      ended: '已结束',
      locked: '已封车',
      full: '已满员'
    },
    visibility: {
      public: '公开上车',
      inviteOnly: '仅限邀请'
    },
    fields: {
      name: '拼车名称',
      namePlaceholder: '例如：周末 Codex Pro 拼车',
      description: '备注',
      descriptionPlaceholder: '可选，补充开车时间或使用安排',
      platform: '平台',
      plan: '套餐',
      carType: '车型',
      level: '等级（账号数）',
      accounts: '账号数',
      capacity: '座位数',
      totalCost: '每月总成本',
      visibility: '加入方式',
      scheduledStart: '预计开车',
      organizer: '发起人',
      members: '已上车',
      seatsRemaining: '剩余 {count} 个座位'
    },
    roles: {
      owner: '我发起的',
      member: '我已上车'
    },
    actions: {
      join: '上车',
      joined: '已上车',
      invite: '邀请成员',
      details: '查看详情',
      lock: '停止上人',
      unlock: '重新开放',
      cancel: '取消拼车',
      copyLink: '复制链接',
      copied: '邀请链接已复制'
    },
    createDialog: {
      title: '发起新拼车',
      submit: '创建并生成邀请链接',
      success: '拼车已创建'
    },
    joinDialog: {
      title: '确认上车',
      message: '确认加入“{name}”吗？开车后会由管理员绑定对应订阅。',
      confirm: '确认上车',
      success: '已加入拼车'
    },
    cancelDialog: {
      title: '取消拼车',
      message: '取消“{name}”后，已有邀请链接将失效，当前座位也会被释放。',
      confirm: '确认取消',
      success: '拼车已取消'
    },
    inviteDialog: {
      title: '邀请成员',
      label: '邀请链接',
      uses: '当前 {used} / {max} 个座位',
      unavailable: '当前拼车不能继续邀请成员'
    },
    detailDialog: {
      title: '拼车详情',
      progress: '座位进度',
      runtime: '开车状态',
      linkedGroup: '关联分组',
      pendingGroup: '等待管理员开车时绑定'
    },
    admin: {
      locked: '已停止新成员上车',
      unlocked: '已重新开放上车'
    },
    empty: {
      title: '暂无符合条件的拼车',
      description: '调整筛选条件，或发起一辆新车'
    },
    unavailable: '这辆车当前不能上人',
    inviteNotFound: '邀请链接无效或已失效',
    loadFailed: '加载拼车失败',
    actionFailed: '拼车操作失败'
  }
}
