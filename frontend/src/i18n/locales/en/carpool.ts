export default {
  carpool: {
    title: 'Carpools',
    description: 'Find a recruiting carpool or start a new one',
    create: 'Start carpool',
    level: 'Level {level}',
    types: {
      small: 'Small car',
      large: 'Large car',
      smallHint: '1 account and 5 seats per level',
      largeHint: '1 account and 10 seats per level'
    },
    rules: {
      title: 'GPT Carpool Rules',
      monthlyLock: 'Choose a level at departure; locked for the month',
      accountCost: 'Each account: 1,400',
      small: {
        capacity: '1 account · 5 people per level',
        upgrade: 'Level 1 has 1 account for 5 people; Level 2 has 2 accounts for 10 people. Each higher level adds 1 account and 5 seats.',
        baseFee: 'Base fee: 130 / person',
        usageFee: 'Relative usage: 1,400 - 5 × 130 = 750 / account'
      },
      large: {
        capacity: '1 account · 10 people per level',
        upgrade: 'Level 1 has 1 account for 10 people. Each higher level adds 1 account and 10 seats.',
        baseFee: 'Base fee: 65 / person',
        usageFee: 'Relative usage: 1,400 - 10 × 65 = 750 / account'
      },
      lockNotice: 'Choose both the car type and level (account count) before departure. The level cannot change during the month and can be selected again next month.'
    },
    plaza: 'Carpool plaza',
    mine: 'My carpools',
    searchPlaceholder: 'Search by carpool or organizer',
    allStatuses: 'All statuses',
    stats: {
      recruiting: 'Recruiting',
      seats: 'Open seats',
      joined: 'Joined by me',
      launched: 'Launched'
    },
    status: {
      recruiting: 'Recruiting',
      starting: 'Starting',
      active: 'Launched',
      cancelled: 'Cancelled',
      ended: 'Ended',
      locked: 'Closed',
      full: 'Full'
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
      platform: 'Platform',
      plan: 'Plan',
      carType: 'Car type',
      level: 'Level (account count)',
      accounts: 'Accounts',
      capacity: 'Seats',
      totalCost: 'Monthly cost',
      visibility: 'Join policy',
      scheduledStart: 'Expected start',
      organizer: 'Organizer',
      members: 'Members',
      seatsRemaining: '{count} seats remaining'
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
      lock: 'Close joining',
      unlock: 'Reopen',
      cancel: 'Cancel carpool',
      copyLink: 'Copy link',
      copied: 'Invite link copied'
    },
    createDialog: {
      title: 'Start a new carpool',
      submit: 'Create and generate invite',
      success: 'Carpool created'
    },
    joinDialog: {
      title: 'Confirm joining',
      message: 'Join “{name}”? An administrator will bind the subscription when it launches.',
      confirm: 'Confirm join',
      success: 'Carpool joined'
    },
    cancelDialog: {
      title: 'Cancel carpool',
      message: 'Cancelling “{name}” invalidates its invite links and releases all seats.',
      confirm: 'Cancel carpool',
      success: 'Carpool cancelled'
    },
    inviteDialog: {
      title: 'Invite members',
      label: 'Invite link',
      uses: '{used} / {max} seats currently occupied',
      unavailable: 'This carpool cannot accept more invitations'
    },
    detailDialog: {
      title: 'Carpool details',
      progress: 'Seat progress',
      runtime: 'Launch status',
      linkedGroup: 'Linked group',
      pendingGroup: 'Will be linked by an administrator at launch'
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
    inviteNotFound: 'This invite is invalid or has expired'
  }
}
