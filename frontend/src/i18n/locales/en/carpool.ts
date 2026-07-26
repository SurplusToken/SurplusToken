export default {
  carpool: {
    title: 'Carpools',
    description: 'Find a recruiting carpool or start a new one',
    create: 'Start carpool',
    rules: {
      title: 'GPT Carpool Rules: Quota Reservation',
      weeklyBadge: 'Locked quota refreshes weekly',
      declare: {
        label: 'Declaration-based',
        text: 'Declare your expected weekly quota (USD) before joining and prepay against it. The car launches once total declarations reach 95%–105% of its weekly limit.'
      },
      reserve: {
        label: '80% reserved + 20% shared pool',
        text: '80% of your declaration is locked for you and cannot be taken by anyone; the rest goes to a shared pool, first come first served.'
      },
      pricing: {
        label: '¥400 + ¥1000 two-part pricing',
        text: 'Each car costs ¥400/month in seat fees split equally per member (more members, cheaper seats) plus a ¥1000/month usage pool split by billable usage.'
      },
      floor: {
        label: '80% floor settlement',
        text: 'At month end you are billed on max(actual usage, 80% × declaration) with refunds/top-ups — you always pay at least your reserved floor.'
      }
    },
    notices: {
      weeklyRefresh: 'Locked quota is weekly: it refreshes automatically every week and unused quota does not roll over.',
      consumeOrder: 'Usage drains your locked quota first, then the shared pool. The shared pool is first come first served and not guaranteed.',
      customRule: 'Custom rules supported: if you need a different quota pool, pricing, or reserve ratio, contact the admin to negotiate.'
    },
    plaza: 'Carpool plaza',
    mine: 'My carpools',
    searchPlaceholder: 'Search by carpool or organizer',
    allStatuses: 'All statuses',
    stats: {
      recruiting: 'Recruiting',
      joinableQuota: 'Joinable quota',
      joined: 'Joined by me',
      launched: 'Launched'
    },
    status: {
      recruiting: 'Recruiting',
      confirmed: 'Pending admin launch',
      starting: 'Starting',
      active: 'Launched',
      cancelled: 'Cancelled',
      ended: 'Ended',
      locked: 'Closed',
      full: 'Quota full'
    },
    visibility: {
      public: 'Public',
      inviteOnly: 'Invite only'
    },
    fields: {
      name: 'Name',
      namePlaceholder: 'For example: Weekend Codex Pro',
      description: 'Notes',
      descriptionPlaceholder: 'Optional schedule or usage notes',
      visibility: 'Join policy',
      scheduledStart: 'Expected start',
      organizer: 'Organizer',
      weeklyLimitBadge: 'GPT · {limit} USD/week',
      quotaProgress: 'Quota reservation',
      declaredOf: '{declared} / {limit} USD reserved',
      launchLine: '{ratio}% launch line',
      remainingJoinable: 'Joinable quota',
      plusEquivalents: 'Plus equivalents',
      avgPrice: 'Avg price',
      avgPriceUnit: '¥ / Plus-equiv / month',
      members: '{count} members joined'
    },
    roles: {
      owner: 'Organized by me',
      member: 'Joined by me'
    },
    actions: {
      join: 'Join',
      joined: 'Joined',
      invite: 'Invite',
      details: 'Details',
      confirm: 'Confirm launch',
      launch: 'Launch',
      forceLaunch: 'Launch anyway',
      leave: 'Leave',
      settlement: 'Settlement',
      lock: 'Close joining',
      unlock: 'Reopen',
      cancel: 'Cancel carpool',
      unconfirm: 'Withdraw confirmation',
      copyLink: 'Copy link',
      copied: 'Invite link copied'
    },
    // Backend error code -> user-facing message. Quota/full rejections are core
    // paths; users need to know what to do next, not the raw English error.
    errors: {
      quotaExceeded: 'Your declaration exceeds this car\'s remaining joinable quota. Lower it or wait for the next car.',
      full: 'This car is full. Please wait for the next one.',
      unavailable: 'This car is not accepting members right now (joining may be closed or the launch already confirmed). Please wait for the next car.',
      alreadyJoined: 'You are already on this car.',
      forbidden: 'You are not allowed to perform this action.',
      notFound: 'This carpool no longer exists or was cancelled.',
      alreadySettled: 'The settlement is already frozen. Ask an administrator to undo it if it needs recomputing.',
      notSettled: 'This carpool has not been settled yet.',
      notSettleable: 'The carpool must be launched before it can be settled.',
      inviteInvalid: 'This invite link is invalid or expired. Ask the owner for a new one.',
      nameConflict: 'A carpool with this name is already recruiting or running. Pick another name.',
      launchNotReady: 'Total declared quota is outside the launchable band; the car cannot launch yet.',
      notConfirmed: 'The owner has not confirmed the launch yet.',
      declarationTooSmall: 'Declared quota must be at least {min} USD/week.',
      interestTooFrequent: 'An enquiry was just sent to the admins. Please try again later.',
      customParamsForbidden: 'Custom quota/pricing parameters need an administrator. Pick "Custom rules" in the create dialog to reach out.',
      groupJoinRequired: 'Please confirm you have joined the WeChat group first.',
      contactConfirmRequired: 'Please confirm you have added the admin on WeChat first.',
      qrCodeRequired: 'A WeChat group QR code is required.',
      qrCodeInvalid: 'The QR code must be a png / jpeg / webp image under 2MB.',
      ownerCannotLeave: 'Owners cannot leave; cancel the whole carpool instead.',
      notMember: 'You are not a member of this carpool.'
    },
    wechat: {
      adminLabel: 'Admin WeChat',
      scanToJoin: 'Scan to join the WeChat group',
      copied: 'Admin WeChat ID copied'
    },
    createDialog: {
      title: 'Start a new carpool',
      submit: 'Create and generate invite',
      success: 'Carpool created',
      ruleMode: 'Carpool rules',
      ruleModeDefault: 'Default rules',
      ruleModeCustom: 'Custom rules',
      customRule: {
        title: 'Custom rules must be negotiated with the admin (quota pool, pricing, reserve ratio, etc.)',
        description: 'No need to fill in the form below. Click "Notify admin" to send your request; the admin will reach out by email or WeChat, then create and tune the car for you once agreed.',
        notify: 'Notify admin',
        notifySuccess: 'Admin notified. Please add WeChat {wechat} to continue the negotiation.'
      },
      ownerQuota: 'My declared quota (optional, USD/week)',
      ownerQuotaHint: 'Leave empty to organize without reserving quota; a value prepays as one member.',
      contactTitle: 'Contact & group QR code (required)',
      contactHint: 'Launches are performed manually by the admin. Add the admin on WeChat first, then upload the WeChat group QR code so members can join the group.',
      addedAdmin: 'I have added the admin on WeChat ({wechat})',
      qrLabel: 'Group QR code',
      qrHint: 'png / jpeg / webp only, up to 2MB',
      qrInvalidType: 'Only png, jpeg, or webp images are supported',
      qrTooLarge: 'Image must be 2MB or smaller'
    },
    joinDialog: {
      title: 'Confirm joining',
      quotaLabel: 'Declared quota (USD/week)',
      recommendationLoading: 'Loading a recommended declaration…',
      recommendationFailed: 'Could not load a recommendation; please estimate your own usage',
      previewFloor: 'Reserved floor',
      floorUnit: 'USD/week',
      previewPrepaid: 'Estimated prepay',
      previewAvgPrice: 'Current avg price',
      previewYourPrice: 'Your unit price',
      yourPriceUnit: '¥ / Plus-equivalent / month',
      priceAboveAverage: 'At your declaration your unit price is {yours}, about {times}x the car average ({average}). The seat fee splits per head, so a smaller declaration means a higher unit price.',
      floorNotice: 'Even if you use less, you are billed for at least 80% of your declaration.',
      exceedsRemaining: 'Declaration exceeds this car\'s remaining joinable quota ({amount} USD); lower it or wait for the next car',
      belowFloor: 'Declared quota must be at least {min} USD/week',
      groupSection: 'Join the WeChat group before boarding',
      joinedGroup: 'I have joined the group',
      confirm: 'Confirm join',
      success: 'Joined carpool, prepaid ¥{amount}',
      successNoPrepaid: 'Joined carpool'
    },
    confirmDialog: {
      title: 'Confirm launch',
      message: 'Confirm the launch of “{name}”? Total declared quota is {total} USD ({ratio}% of the weekly limit). Once confirmed, the carpool locks and an admin will launch it within 24 hours.',
      confirm: 'Confirm launch',
      notReady: '{amount} USD short of the {ratio}% launch line',
      aboveMax: 'Above the {ratio}% launch cap; a member has to leave before confirming',
      success: 'Confirmed. Waiting for the admin to launch.'
    },
    pendingLaunch: {
      title: 'Waiting to launch ({count})',
      overdue: 'Over 24h',
      overdueBadge: '{count} overdue',
      summary: '{members} members · {total} declared · waiting {hours}h',
      notLoaded: 'That carpool is not in the current list; refresh and try again'
    },
    unconfirmDialog: {
      title: 'Withdraw confirmation',
      message: 'Send “{name}” back to recruiting? Members and declarations are kept and joining reopens; you can confirm again any time before an admin launches it.',
      confirm: 'Withdraw',
      success: 'Confirmation withdrawn; the carpool is recruiting again'
    },
    leaveDialog: {
      title: 'Leave carpool',
      message: 'Leave “{name}”? Your declared quota will be released immediately.',
      confirm: 'Leave',
      success: 'Left the carpool; your declared quota was released'
    },
    launchDialog: {
      confirmTitle: 'Launch carpool',
      confirmMessage: 'Launch “{name}”? The owner has confirmed (total declared {total} USD, {ratio}% of the weekly limit). Limits of 80% reserve + shared pool apply once launched and are locked for the month.',
      forceTitle: 'Launch below the line',
      forceMessage: 'Total declared quota is {total} USD ({ratio}% of the weekly limit), below the 95% standard line. Launch “{name}” anyway? The shared pool grows; every member\'s reserve stays the same.',
      confirm: 'Launch',
      forceReady: 'Above 80%; eligible for a forced launch',
      success: 'Carpool launched'
    },
    cancelDialog: {
      title: 'Cancel carpool',
      message: 'Cancelling “{name}” invalidates its invite links and releases all reserved quota.',
      confirm: 'Cancel carpool',
      success: 'Carpool cancelled'
    },
    inviteDialog: {
      title: 'Invite members',
      label: 'Invite link',
      uses: '{count} members currently joined',
      unavailable: 'This carpool cannot accept more invitations'
    },
    detailDialog: {
      title: 'Carpool details',
      runtime: 'Launch status',
      linkedGroup: 'Linked group',
      pendingGroup: 'Will be linked by an administrator at launch'
    },
    settlement: {
      title: 'Monthly settlement',
      period: 'Billing period',
      member: 'Member',
      declared: 'Declared (USD/week)',
      actual: 'Actual (USD)',
      billable: 'Billable (USD)',
      floorBadge: '80% floor',
      prepaid: 'Prepaid (¥)',
      delta: 'Refund/Top-up (¥)',
      deltaRefund: 'Refund ¥{amount}',
      deltaTopUp: 'Top-up ¥{amount}',
      deltaEven: '¥0',
      deltaNote: 'Positive amounts are refunds, negative amounts are top-ups',
      selfOnly: 'Only your own settlement row is shown',
      fullView: 'All {count} members',
      usageDelta: 'Usage refund/top-up',
      seatFeeDelta: 'Seat fee refund/top-up',
      loadFailed: 'Failed to load the settlement',
      livePreview: 'Live preview: these amounts keep moving with usage. They are only fixed once the settlement is confirmed.',
      settle: 'Confirm settlement',
      settleSuccess: 'Settlement frozen; the amounts no longer change',
      frozenAt: 'Settled at {time}; amounts are frozen',
      unsettle: 'Undo settlement',
      unsettleSuccess: 'Settlement undone; back to live preview',
      blockedNotLaunched: 'The carpool must be launched before it can be settled'
    },
    admin: {
      locked: 'Joining has been closed',
      unlocked: 'Joining has been reopened'
    },
    empty: {
      title: 'No matching carpools',
      description: 'Change the filters or start a new carpool'
    },
    unavailable: 'This carpool is not accepting members',
    inviteNotFound: 'This invite is invalid or has expired',
    loadFailed: 'Failed to load carpools',
    actionFailed: 'Carpool action failed'
  }
}
