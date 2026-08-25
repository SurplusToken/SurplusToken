<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-3">
          <!--
            异常条：管理员真正需要的是「哪几辆车现在要我处理」，而不是把 30 辆车
            从头看到尾。放在最上面，点一下就把表格筛到对应的那几辆。
          -->
          <div
            v-if="alertSummary.length > 0"
            data-testid="carpool-admin-alerts"
            class="flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs dark:border-amber-900/40 dark:bg-amber-900/20"
          >
            <span class="font-medium text-amber-800 dark:text-amber-300">{{ t('carpool.adminPage.alertsTitle') }}</span>
            <button
              v-for="item in alertSummary"
              :key="item.kind"
              type="button"
              class="rounded border border-amber-300 px-2 py-0.5 text-amber-800 hover:bg-amber-100 dark:border-amber-700 dark:text-amber-200 dark:hover:bg-amber-900/40"
              :class="{ 'bg-amber-200 dark:bg-amber-900/60': alertFilter === item.kind }"
              @click="alertFilter = alertFilter === item.kind ? '' : item.kind"
            >
              {{ t(`carpool.adminPage.alerts.${item.kind}`) }} · {{ item.count }}
            </button>
          </div>

          <div class="flex flex-wrap items-center gap-3">
            <Select v-model="statusFilter" :options="statusOptions" class="w-44" />
            <input v-model="searchQuery" type="search" class="input w-56" :placeholder="t('carpool.adminPage.searchPlaceholder')" />
            <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button type="button" class="btn btn-primary" data-testid="open-create-carpool" @click="openCreate">
              <Icon name="plus" size="sm" />
              <span>{{ t('carpool.adminPage.create') }}</span>
            </button>
            <span class="text-xs text-gray-500 dark:text-dark-300">
              {{ t('carpool.adminPage.total', { count: filteredCarpools.length, all: carpools.length }) }}
            </span>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="filteredCarpools" :loading="loading" row-key="id" :actions-count="7">
          <template #cell-name="{ row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
              <div class="mt-0.5 flex flex-wrap gap-1">
                <span v-if="row.visibility === 'invite_only'" class="badge badge-gray">
                  {{ t('carpool.visibility.inviteOnly') }}
                </span>
                <span v-if="!isQuotaCar(row)" class="badge badge-gray">
                  {{ t('carpool.customRule.badge') }}
                </span>
                <!-- 车型标签（type 1/2/3）；自定义规则车已由上面的 badge 标注，不重复 -->
                <span v-if="carTypeLabel(row)" class="badge badge-gray">
                  {{ carTypeLabel(row) }}
                </span>
                <span
                  v-for="kind in carpoolAlerts(row)"
                  :key="kind"
                  class="badge badge-warning"
                >
                  {{ t(`carpool.adminPage.alerts.${kind}`) }}
                </span>
              </div>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span :class="['badge', statusBadgeClass(row)]">{{ statusLabel(row) }}</span>
          </template>

          <template #cell-owner="{ row }">
            <span class="truncate text-sm text-gray-700 dark:text-dark-200">{{ row.organizer }}</span>
          </template>

          <template #cell-declared="{ row }">
            <span v-if="!isQuotaCar(row)" class="text-xs text-gray-400">—</span>
            <span v-else class="text-sm text-gray-700 dark:text-dark-200">
              ${{ Math.round(row.declaredTotalUsd) }} / ${{ Math.round(row.weeklyLimitUsd) }}
              <span class="ml-1 text-[10px] text-gray-400">{{ declaredPercent(row) }}%</span>
            </span>
          </template>

          <template #cell-settled="{ row }">
            <span class="text-xs" :class="row.settledAt ? 'text-gray-500 dark:text-dark-300' : 'text-gray-400'">
              {{ row.settledAt ? formatDateOnly(row.settledAt) : '—' }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button type="button" class="action-btn" :title="t('carpool.adminPage.actions.members')" @click="openMembers(row)">
                <Icon name="users" size="sm" />
                <span class="text-xs">{{ t('carpool.adminPage.actions.members') }}</span>
              </button>
              <button v-if="canEdit(row)" type="button" class="action-btn" :title="t('carpool.adminPage.actions.edit')" @click="openEdit(row)">
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('carpool.adminPage.actions.edit') }}</span>
              </button>
              <button v-if="canTransfer(row)" type="button" class="action-btn" :title="t('carpool.adminPage.actions.transfer')" @click="openTransfer(row)">
                <Icon name="swap" size="sm" />
                <span class="text-xs">{{ t('carpool.adminPage.actions.transfer') }}</span>
              </button>
              <button v-if="row.status === 'recruiting'" type="button" class="action-btn" :title="row.joinLocked ? t('carpool.actions.unlock') : t('carpool.actions.lock')" @click="toggleLock(row)">
                <Icon name="lock" size="sm" />
                <span class="text-xs">{{ row.joinLocked ? t('carpool.actions.unlock') : t('carpool.actions.lock') }}</span>
              </button>
              <button v-if="row.status === 'confirmed'" type="button" class="action-btn" :title="t('carpool.actions.unconfirm')" @click="run('unconfirm', row)">
                <Icon name="arrowLeft" size="sm" />
                <span class="text-xs">{{ t('carpool.actions.unconfirm') }}</span>
              </button>
              <button v-if="row.status === 'confirmed'" type="button" class="action-btn action-btn-primary" :title="t('carpool.actions.launch')" @click="run('launch', row)">
                <Icon name="play" size="sm" />
                <span class="text-xs">{{ t('carpool.actions.launch') }}</span>
              </button>
              <button v-if="canCancel(row)" type="button" class="action-btn action-btn-danger" :title="t('carpool.actions.cancel')" @click="run('cancel', row)">
                <Icon name="ban" size="sm" />
                <span class="text-xs">{{ t('carpool.actions.cancel') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState :title="t('carpool.adminPage.empty')" />
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <!-- 成员管理 -->
    <BaseDialog :show="membersOpen" :title="t('carpool.adminPage.membersDialog.title', { name: activeCarpool?.name || '' })" width="normal" @close="closeMembers">
      <div v-if="activeCarpool" class="space-y-3">
        <p class="text-xs text-gray-500 dark:text-dark-300">
          {{ t('carpool.adminPage.membersDialog.hint') }}
        </p>
        <p v-if="!canManageMembers(activeCarpool)" class="rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-300">
          {{ t('carpool.adminPage.membersDialog.readOnly') }}
        </p>
        <!-- 群二维码：打开弹窗时现取，关掉即吊销 object URL（私密车的二维码=入场券）。
             管理员可在此更换——二维码会过期、群也会换，不必整车重建。 -->
        <div class="flex items-center gap-3 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-700/40">
          <img
            v-if="qrCodeUrl"
            :src="qrCodeUrl"
            :alt="t('carpool.adminPage.membersDialog.groupQr')"
            class="h-14 w-14 shrink-0 cursor-zoom-in rounded-md border border-gray-200 object-cover dark:border-dark-600"
            @click="qrZoomUrl = qrCodeUrl"
          />
          <div class="min-w-0 flex-1 text-xs">
            <div class="font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.adminPage.membersDialog.groupQr') }}</div>
            <div class="mt-1 flex flex-wrap items-center gap-2">
              <a
                v-if="qrCodeUrl"
                :href="qrCodeUrl"
                target="_blank"
                rel="noopener"
                class="text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              >
                {{ t('carpool.adminPage.membersDialog.qrOpen') }}
              </a>
              <span v-else-if="qrLoading" class="text-gray-400">{{ t('carpool.adminPage.membersDialog.qrLoading') }}</span>
              <button
                v-else-if="qrFailed"
                type="button"
                class="text-amber-600 hover:text-amber-700 dark:text-amber-400"
                @click="loadQrCode(activeCarpool.id)"
              >
                {{ t('carpool.adminPage.membersDialog.qrFailed') }}
              </button>
              <button
                type="button"
                class="text-primary-600 hover:text-primary-700 disabled:opacity-50 dark:text-primary-400 dark:hover:text-primary-300"
                :disabled="qrReplacing"
                @click="qrFileInput?.click()"
              >
                {{ qrReplacing ? t('common.loading') : t(activeCarpool.hasGroupQrCode ? 'carpool.adminPage.membersDialog.qrReplace' : 'carpool.adminPage.membersDialog.qrUpload') }}
              </button>
            </div>
          </div>
          <input ref="qrFileInput" type="file" accept="image/png,image/jpeg,image/webp" class="hidden" @change="handleQrFileChange" />
        </div>
        <p v-if="rosterLoading" class="text-xs text-gray-400">{{ t('carpool.joinDialog.rosterLoading') }}</p>
        <p v-else-if="rosterFailed" class="text-xs text-amber-600">{{ t('carpool.joinDialog.rosterFailed') }}</p>
        <p v-else-if="roster.length === 0" class="text-xs text-gray-400">{{ t('carpool.adminPage.membersDialog.empty') }}</p>
        <ul v-else class="space-y-2" data-testid="carpool-admin-roster">
          <li
            v-for="member in roster"
            :key="member.userId"
            class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-gray-200 px-3 py-2 dark:border-dark-600"
          >
            <span class="flex min-w-0 flex-col text-sm">
              <span class="flex min-w-0 items-center gap-1.5">
                <span class="truncate text-gray-800 dark:text-dark-100">
                  {{ member.username || t('carpool.joinDialog.rosterAnonymous', { id: member.userId }) }}
                </span>
                <span v-if="member.role === 'owner'" class="badge badge-gray shrink-0">
                  {{ t('carpool.joinDialog.rosterOwner') }}
                </span>
                <!-- 新 quota 车（type 3）成员的风险确认状态（只读）：上车时后端强制要求确认 -->
                <span
                  v-if="activeCarpool.carType === 3"
                  class="shrink-0 rounded px-1 text-[10px]"
                  :class="member.acknowledgedRisk
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
                    : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'"
                >
                  {{ t(member.acknowledgedRisk ? 'carpool.adminPage.membersDialog.riskAcked' : 'carpool.adminPage.membersDialog.riskNotAcked') }}
                </span>
              </span>
              <span v-if="member.email" class="truncate text-xs text-gray-400">{{ member.email }}</span>
            </span>
            <span class="flex items-center gap-2">
              <template v-if="editingUserId === member.userId">
                <input v-model.number="editingQuota" type="number" min="1" step="1" class="input h-8 w-24 text-sm" />
                <button type="button" class="btn-xs btn-xs-primary" :disabled="memberPending" @click="saveQuota(member.userId)">{{ t('common.save') }}</button>
                <button type="button" class="btn-xs" @click="editingUserId = null">{{ t('common.cancel') }}</button>
              </template>
              <template v-else>
                <span v-if="isQuotaCar(activeCarpool)" class="text-sm font-medium text-gray-600 dark:text-dark-200">${{ Math.round(member.declaredWeeklyQuotaUsd) }}/{{ t('carpool.adminPage.perWeek') }}</span>
                <button
                  v-if="canManageMembers(activeCarpool) && isQuotaCar(activeCarpool)"
                  type="button"
                  class="btn-xs"
                  @click="startEditQuota(member.userId, member.declaredWeeklyQuotaUsd)"
                >
                  {{ t('carpool.adminPage.actions.editQuota') }}
                </button>
                <button
                  v-if="canManageMembers(activeCarpool) && member.role !== 'owner'"
                  type="button"
                  class="btn-xs btn-xs-danger"
                  :disabled="memberPending"
                  @click="removeMember(member.userId)"
                >
                  {{ t('carpool.adminPage.actions.remove') }}
                </button>
              </template>
            </span>
          </li>
        </ul>

        <!--
          添加成员：type 2/3 需申报 + 代录风险确认（后端校验）；type 1/0 仅选人、
          添加即生效（后台直接建订阅，不走招募/发车）。
        -->
        <div
          v-if="canAddMembers(activeCarpool)"
          data-testid="add-member-section"
          class="rounded-lg border border-gray-200 px-3 py-3 dark:border-dark-600"
        >
          <div class="text-xs font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.adminPage.addMember.title') }}</div>
          <div class="mt-2">
            <Select
              v-model="addDraft.userId"
              :options="memberUserOptions"
              remote
              :loading="memberUserLoading"
              :placeholder="t('carpool.adminPage.memberEditor.pickUser')"
              @search="searchMemberUsers"
            />
          </div>
          <div v-if="activeCarpool.carType === 3 || activeCarpool.carType === 2" class="mt-2 space-y-2">
            <input
              id="add-member-declared"
              v-model.number="addDraft.declaredInput"
              type="number"
              :min="activeCarpool.carType === 3 ? 0.1 : 1"
              :step="activeCarpool.carType === 3 ? 0.1 : 1"
              class="input"
              :placeholder="t(activeCarpool.carType === 3 ? 'carpool.adminPage.memberEditor.declaredPercent' : 'carpool.adminPage.memberEditor.declaredUsd')"
            />
            <label class="flex items-center gap-2">
              <input
                id="add-member-risk"
                v-model="addDraft.acknowledgedRisk"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
              <span class="text-sm text-gray-700 dark:text-dark-200">{{ t('carpool.adminPage.memberEditor.riskInformed') }}</span>
            </label>
          </div>
          <div class="mt-2 flex justify-end">
            <button
              type="button"
              class="btn btn-primary h-8 px-3 py-1.5"
              data-testid="add-member-submit"
              :disabled="!canSubmitAddMember || memberPending"
              @click="submitAddMember"
            >
              {{ t('carpool.adminPage.memberEditor.add') }}
            </button>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button type="button" class="btn btn-secondary" @click="closeMembers">{{ t('common.close') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!--
      手动创建车辆：车型决定字段与后续流程——type 2/3 创建后 recruiting（二维码必传，
      之后走现有确认/启动流程）；type 1/0 创建即 active（后台自动建分组，无需二维码）。
      初始成员逐个代加：失败的逐个提示，成功的保留。
    -->
    <BaseDialog :show="createOpen" :title="t('carpool.adminPage.createDialog.title')" width="normal" @close="createOpen = false">
      <div class="space-y-4">
        <div>
          <label class="input-label" for="admin-create-car-type">{{ t('carpool.adminPage.createDialog.carType') }}</label>
          <Select id="admin-create-car-type" v-model="createForm.carType" :options="carTypeOptions" />
        </div>
        <div>
          <label class="input-label" for="admin-create-name">{{ t('carpool.fields.name') }}</label>
          <input id="admin-create-name" v-model.trim="createForm.name" class="input" maxlength="100" :placeholder="t('carpool.fields.namePlaceholder')" />
        </div>

        <!-- type 2/3（quota 车）：备注、预计发车日、可见性、群二维码 -->
        <template v-if="createIsQuota">
          <div>
            <label class="input-label" for="admin-create-desc">{{ t('carpool.fields.description') }}</label>
            <textarea id="admin-create-desc" v-model.trim="createForm.description" rows="2" class="input" :placeholder="t('carpool.fields.descriptionPlaceholder')"></textarea>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label" for="admin-create-start">{{ t('carpool.fields.scheduledStart') }}</label>
              <input id="admin-create-start" v-model="createForm.scheduledStartAt" type="date" class="input" />
            </div>
            <div>
              <label class="input-label" for="admin-create-visibility">{{ t('carpool.fields.visibility') }}</label>
              <Select id="admin-create-visibility" v-model="createForm.visibility" :options="visibilityOptions" />
            </div>
          </div>
          <div>
            <label class="input-label" for="admin-create-qr">{{ t('carpool.createDialog.qrLabel') }}</label>
            <div class="flex items-start gap-3">
              <div class="min-w-0 flex-1">
                <input
                  id="admin-create-qr"
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  class="block w-full text-sm text-gray-500 file:mr-3 file:rounded-md file:border-0 file:bg-primary-50 file:px-3 file:py-2 file:text-sm file:font-medium file:text-primary-700 hover:file:bg-primary-100 dark:text-dark-300 dark:file:bg-primary-900/20 dark:file:text-primary-300"
                  @change="handleCreateQrFileChange"
                />
                <p v-if="createQrError" class="mt-1 text-xs font-medium text-red-600 dark:text-red-400">{{ createQrError }}</p>
              </div>
              <img
                v-if="createForm.groupQrCode"
                :src="createForm.groupQrCode"
                :alt="t('carpool.createDialog.qrLabel')"
                class="h-16 w-16 shrink-0 rounded-md border border-gray-200 object-cover dark:border-dark-600"
              />
            </div>
          </div>
        </template>

        <!-- type 1（无保底老车）：仅需周限额 -->
        <div v-if="createForm.carType === 1">
          <label class="input-label" for="admin-create-weekly-limit">{{ t('carpool.adminPage.createDialog.weeklyLimit') }}</label>
          <input id="admin-create-weekly-limit" v-model.number="createForm.weeklyLimitUsd" type="number" min="1" step="1" class="input" />
        </div>

        <!-- type 0（自定义规则车）：规则说明是人工结算依据 -->
        <div v-if="createForm.carType === 0">
          <label class="input-label" for="admin-create-rule-note">{{ t('carpool.adminPage.createDialog.ruleNote') }}</label>
          <textarea id="admin-create-rule-note" v-model.trim="createForm.ruleNote" rows="3" class="input" :placeholder="t('carpool.adminPage.createDialog.ruleNotePlaceholder')"></textarea>
        </div>

        <!-- 成员编辑器：远程搜索用户加入暂存列表；type 3 申报填百分比、type 2 填美元，
             quota 车成员必须代录风险确认；type 1/0 只选人 -->
        <div class="rounded-lg border border-gray-200 px-3 py-3 dark:border-dark-600">
          <div class="text-xs font-medium text-gray-700 dark:text-dark-100">{{ t('carpool.adminPage.createDialog.members') }}</div>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ t('carpool.adminPage.createDialog.membersHint') }}</p>
          <div class="mt-2">
            <Select
              v-model="memberDraft.userId"
              :options="memberUserOptions"
              remote
              :loading="memberUserLoading"
              :placeholder="t('carpool.adminPage.memberEditor.pickUser')"
              @search="searchMemberUsers"
            />
          </div>
          <div v-if="createIsQuota" class="mt-2 space-y-2">
            <input
              id="create-member-declared"
              v-model.number="memberDraft.declaredInput"
              type="number"
              :min="createForm.carType === 3 ? 0.1 : 1"
              :step="createForm.carType === 3 ? 0.1 : 1"
              class="input"
              :placeholder="t(createForm.carType === 3 ? 'carpool.adminPage.memberEditor.declaredPercent' : 'carpool.adminPage.memberEditor.declaredUsd')"
            />
            <label class="flex items-center gap-2">
              <input
                id="create-member-risk"
                v-model="memberDraft.acknowledgedRisk"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
              <span class="text-sm text-gray-700 dark:text-dark-200">{{ t('carpool.adminPage.memberEditor.riskInformed') }}</span>
            </label>
          </div>
          <div class="mt-2 flex justify-end">
            <button
              type="button"
              class="btn btn-secondary h-8 px-3 py-1.5"
              data-testid="create-member-add"
              :disabled="!canStageMember"
              @click="stageMember"
            >
              {{ t('carpool.adminPage.memberEditor.add') }}
            </button>
          </div>
          <ul v-if="stagedMembers.length > 0" class="mt-2 space-y-1" data-testid="create-staged-members">
            <li
              v-for="member in stagedMembers"
              :key="member.userId"
              class="flex items-center justify-between gap-2 rounded-md bg-gray-50 px-2.5 py-1.5 text-xs dark:bg-dark-700/40"
            >
              <span class="min-w-0 truncate text-gray-700 dark:text-dark-200">
                {{ member.label }}
                <span v-if="member.declaredInput" class="text-gray-400">
                  · {{ createForm.carType === 3 ? `${member.declaredInput}%` : `$${member.declaredInput}` }}
                </span>
              </span>
              <button type="button" class="shrink-0 text-gray-400 hover:text-red-500" @click="unstageMember(member.userId)">
                <Icon name="xCircle" size="sm" />
              </button>
            </li>
          </ul>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="createOpen = false">{{ t('common.cancel') }}</button>
          <button
            type="button"
            class="btn btn-primary"
            data-testid="create-submit"
            :disabled="!createFormValid || createPending"
            @click="submitCreate"
          >
            {{ t('carpool.adminPage.createDialog.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- 编辑车辆 -->
    <BaseDialog :show="editOpen" :title="t('carpool.adminPage.editDialog.title')" width="normal" @close="editOpen = false">
      <div class="space-y-3">
        <div>
          <label class="input-label" for="admin-carpool-name">{{ t('carpool.fields.name') }}</label>
          <input id="admin-carpool-name" v-model="editForm.name" type="text" class="input" />
        </div>
        <div>
          <label class="input-label" for="admin-carpool-desc">{{ t('carpool.fields.description') }}</label>
          <textarea id="admin-carpool-desc" v-model="editForm.description" rows="2" class="input"></textarea>
        </div>
        <div>
          <label class="input-label" for="admin-carpool-visibility">{{ t('carpool.fields.visibility') }}</label>
          <Select
            id="admin-carpool-visibility"
            v-model="editForm.visibility"
            :options="visibilityOptions"
          />
        </div>
        <div>
          <label class="input-label" for="admin-carpool-start">{{ t('carpool.fields.scheduledStart') }}</label>
          <input id="admin-carpool-start" v-model="editForm.scheduledStartAt" type="date" class="input" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="editOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="actionPending || !editForm.name.trim()" @click="saveEdit">{{ t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- 转让车主 -->
    <BaseDialog :show="transferOpen" :title="t('carpool.adminPage.transferDialog.title')" width="normal" @close="transferOpen = false">
      <div class="space-y-3">
        <p class="text-xs text-gray-500 dark:text-dark-300">{{ t('carpool.adminPage.transferDialog.hint') }}</p>
        <p v-if="rosterLoading" class="text-xs text-gray-400">{{ t('carpool.joinDialog.rosterLoading') }}</p>
        <Select
          v-else
          v-model="transferTargetId"
          :options="transferOptions"
          :placeholder="t('carpool.adminPage.transferDialog.pick')"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="transferOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="actionPending || !transferTargetId" @click="saveTransfer">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- 危险操作确认 -->
    <BaseDialog :show="!!confirmAction" :title="confirmTitle" width="narrow" @close="confirmAction = null">
      <p class="text-sm text-gray-700 dark:text-dark-200">{{ confirmMessage }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="confirmAction = null">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="actionPending" @click="runConfirmed">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>
    <!-- 二维码点击放大 -->
    <Teleport to="body">
      <div
        v-if="qrZoomUrl"
        class="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 p-4"
        @click="qrZoomUrl = null"
      >
        <img :src="qrZoomUrl" :alt="t('carpool.adminPage.membersDialog.groupQr')" class="max-h-[85vh] max-w-[90vw] rounded-lg bg-white object-contain p-2" />
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import carpoolAPI, {
  type AddMemberRequest,
  type Carpool,
  type CarpoolRosterMember,
  type CarpoolType,
  type CarpoolVisibility,
  type CreateCarpoolRequest,
} from '@/api/carpools'
import usersAPI from '@/api/admin/users'
import type { AdminUser } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateOnly } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const STATUSES = ['recruiting', 'confirmed', 'starting', 'active', 'ended', 'cancelled'] as const

// 「确认后多久还没启动算超时」——与设计文档里承诺给车主的 24 小时一致。
const LAUNCH_OVERDUE_MS = 24 * 60 * 60 * 1000

// 与后端 CarpoolType3WeeklyLimitUSD / CarpoolDefaultWeeklyLimitUSD 对齐：
// 手动创建时 type 1 的默认周限额，也是 type 3 百分比申报的换算基准。
const DEFAULT_WEEKLY_LIMIT_USD = 2400

type AlertKind = 'launchOverdue' | 'unsettled' | 'overDeclared' | 'readyToConfirm'
type ConfirmKind = 'unconfirm' | 'launch' | 'cancel'

const carpools = ref<Carpool[]>([])
const loading = ref(false)
const actionPending = ref(false)
const statusFilter = ref('')
const alertFilter = ref('')
const searchQuery = ref('')

const membersOpen = ref(false)
const editOpen = ref(false)
const transferOpen = ref(false)
const activeCarpool = ref<Carpool | null>(null)
const roster = ref<CarpoolRosterMember[]>([])
const rosterLoading = ref(false)
const rosterFailed = ref(false)
const memberPending = ref(false)
const editingUserId = ref<number | null>(null)
const editingQuota = ref<number | null>(null)
const transferTargetId = ref<number | null>(null)
const confirmAction = ref<{ kind: ConfirmKind; carpool: Carpool } | null>(null)
const qrCodeUrl = ref<string | null>(null)
const qrLoading = ref(false)
const qrFailed = ref(false)
const qrZoomUrl = ref<string | null>(null)
const qrFileInput = ref<HTMLInputElement | null>(null)
const qrReplacing = ref(false)

// —— 手动创建车辆（设计文档「手动创建车辆 + 按车型添加成员」）——
const createOpen = ref(false)
const createPending = ref(false)
const createQrError = ref('')
const createForm = reactive({
  carType: 3 as CarpoolType,
  name: '',
  description: '',
  visibility: 'public' as CarpoolVisibility,
  scheduledStartAt: '',
  weeklyLimitUsd: DEFAULT_WEEKLY_LIMIT_USD,
  ruleNote: '',
  groupQrCode: '',
})
// 成员编辑器暂存：type 3 申报填百分比、type 2 填美元；type 1/0 只选人。
interface StagedMember {
  userId: number
  label: string
  declaredInput: number | null
  acknowledgedRisk: boolean
}
const stagedMembers = ref<StagedMember[]>([])
// 远程用户搜索（创建对话框与成员弹窗共用；两个弹窗不会同时打开）
const memberUserOptions = ref<{ value: number; label: string }[]>([])
const memberUserLoading = ref(false)
let memberSearchSeq = 0
const memberDraft = reactive({ userId: null as number | null, declaredInput: null as number | null, acknowledgedRisk: false })
// 成员弹窗「添加成员」区的草稿
const addDraft = reactive({ userId: null as number | null, declaredInput: null as number | null, acknowledgedRisk: false })

const editForm = reactive({
  name: '',
  description: '',
  visibility: 'public' as CarpoolVisibility,
  scheduledStartAt: '',
})

const statusOptions = computed(() => [
  { value: '', label: t('carpool.adminPage.allStatuses') },
  ...STATUSES.map((s) => ({ value: s, label: t(`carpool.status.${s}`) })),
])

const visibilityOptions = computed(() => [
  { value: 'public', label: t('carpool.visibility.public') },
  { value: 'invite_only', label: t('carpool.visibility.inviteOnly') },
])

// 车型选择：3 新 quota 车（默认）/ 2 现行 quota 车 / 1 无保底老车 / 0 自定义规则车。
const carTypeOptions = computed(() => ([3, 2, 1, 0] as CarpoolType[]).map((value) => ({
  value,
  label: t(`carpool.carTypes.type${value}`),
})))
// 2/3 是 quota 车（创建后 recruiting，需要发车日 + 群二维码）；1/0 创建即 active。
const createIsQuota = computed(() => createForm.carType === 2 || createForm.carType === 3)

const createFormValid = computed(() => {
  if (!createForm.name.trim()) return false
  if (createForm.carType === 0) return !!createForm.ruleNote.trim()
  if (createForm.carType === 1) return createForm.weeklyLimitUsd > 0
  return !!createForm.scheduledStartAt && !!createForm.groupQrCode && !createQrError.value
})

// 成员编辑器：quota 车（2/3）的暂存成员必须填申报并代录风险确认。
const canStageMember = computed(() => {
  if (!memberDraft.userId) return false
  if (stagedMembers.value.some((m) => m.userId === memberDraft.userId)) return false
  if (!createIsQuota.value) return true
  return !!memberDraft.declaredInput && memberDraft.declaredInput > 0 && memberDraft.acknowledgedRisk
})

// 成员弹窗的添加区：type 1/0 创建即 active，加人是主要操作（直接生效建订阅）；
// type 2/3 只在发车前（recruiting/confirmed）能加。
function canAddMembers(carpool: Carpool): boolean {
  if (carpool.carType === 0 || carpool.carType === 1) {
    return carpool.status !== 'cancelled' && carpool.status !== 'ended'
  }
  return canManageMembers(carpool)
}

const addMemberWeeklyLimit = computed(() => activeCarpool.value?.weeklyLimitUsd || DEFAULT_WEEKLY_LIMIT_USD)
const canSubmitAddMember = computed(() => {
  const car = activeCarpool.value
  if (!car || !addDraft.userId) return false
  if (car.carType === 2 || car.carType === 3) {
    return !!addDraft.declaredInput && addDraft.declaredInput > 0 && addDraft.acknowledgedRisk
  }
  return true
})

const transferOptions = computed(() =>
  roster.value
    .filter((m) => m.role !== 'owner')
    .map((m) => ({
      value: m.userId,
      label: m.username || t('carpool.joinDialog.rosterAnonymous', { id: m.userId }),
    })))

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('carpool.adminPage.columns.name'), sortable: false },
  { key: 'status', label: t('carpool.adminPage.columns.status'), sortable: false },
  { key: 'owner', label: t('carpool.adminPage.columns.owner'), sortable: false },
  { key: 'memberCount', label: t('carpool.adminPage.columns.members'), sortable: true },
  { key: 'declared', label: t('carpool.adminPage.columns.declared'), sortable: false },
  { key: 'settled', label: t('carpool.adminPage.columns.settled'), sortable: false },
  { key: 'actions', label: t('carpool.adminPage.columns.actions'), sortable: false },
])

// 自定义规则车（含升级前的老车）不走申报制，额度相关的列与操作对它们没有意义。
function isQuotaCar(carpool: Carpool): boolean {
  return (carpool.pricingModel || 'quota') === 'quota'
}

// 车型标签（与后端 CarType 对齐）：1=无保底老车，2=现行 quota 车，3=新 quota 车。
// type 0（自定义规则车）已由「自定义规则」badge 覆盖，仅在 pricing_model 数据异常时兜底显示。
function carTypeLabel(carpool: Carpool): string {
  if (!isQuotaCar(carpool)) return ''
  const keys: Record<number, string> = {
    0: 'carpool.carTypes.type0',
    1: 'carpool.carTypes.type1',
    2: 'carpool.carTypes.type2',
    3: 'carpool.carTypes.type3',
  }
  const key = keys[carpool.carType]
  return key ? t(key) : ''
}

function declaredPercent(carpool: Carpool): number {
  if (carpool.weeklyLimitUsd <= 0) return 0
  return Math.round((carpool.declaredTotalUsd / carpool.weeklyLimitUsd) * 100)
}

// 与用户侧拼车页同一套徽章口径，管理员看到的和用户看到的状态观感一致。
function statusLabel(carpool: Carpool): string {
  if (carpool.status === 'recruiting' && carpool.joinLocked) return t('carpool.status.locked')
  if (carpool.status === 'recruiting' && isQuotaCar(carpool) && carpool.remainingJoinableUsd <= 0) return t('carpool.status.full')
  return t(`carpool.status.${carpool.status}`)
}

function statusBadgeClass(carpool: Carpool): string {
  if (carpool.status === 'cancelled' || carpool.status === 'ended') return 'badge-gray'
  if (carpool.status === 'active') return 'badge-success'
  if (carpool.status === 'confirmed') return 'badge-warning'
  if (carpool.joinLocked || (isQuotaCar(carpool) && carpool.remainingJoinableUsd <= 0)) return 'badge-warning'
  return 'badge-primary'
}

// carpoolAlerts 是「这辆车现在要不要我处理」的判据。只用列表里已有的字段算，
// 不额外发请求——管理页一次要渲染全部车。
function carpoolAlerts(carpool: Carpool): AlertKind[] {
  const kinds: AlertKind[] = []
  if (carpool.status === 'confirmed' && carpool.confirmedAt
    && Date.now() - new Date(carpool.confirmedAt).getTime() > LAUNCH_OVERDUE_MS) {
    kinds.push('launchOverdue')
  }
  if (carpool.status === 'ended' && !carpool.settledAt) kinds.push('unsettled')
  if (isQuotaCar(carpool)) {
    const overCap = carpool.declaredTotalUsd > carpool.launchMaxRatio * carpool.weeklyLimitUsd + 1e-9
    if (overCap) kinds.push('overDeclared')
    // 必须落在区间内才提示「待确认」：超出上限时 Confirm 本来就会被后端拒，
    // 再催车主去点一个必定失败的按钮只会制造困惑。
    if (carpool.status === 'recruiting' && !overCap
      && carpool.declaredTotalUsd >= carpool.launchMinRatio * carpool.weeklyLimitUsd) {
      kinds.push('readyToConfirm')
    }
  }
  return kinds
}

const alertSummary = computed(() => {
  const counts = new Map<AlertKind, number>()
  for (const carpool of carpools.value) {
    for (const kind of carpoolAlerts(carpool)) counts.set(kind, (counts.get(kind) || 0) + 1)
  }
  return [...counts.entries()].map(([kind, count]) => ({ kind, count }))
})

const filteredCarpools = computed(() => carpools.value.filter((carpool) => {
  if (statusFilter.value && carpool.status !== statusFilter.value) return false
  if (alertFilter.value && !carpoolAlerts(carpool).includes(alertFilter.value as AlertKind)) return false
  const q = searchQuery.value.trim().toLowerCase()
  if (q && !carpool.name.toLowerCase().includes(q) && !carpool.organizer.toLowerCase().includes(q)) return false
  return true
}))

// 发车后成员已绑定订阅、可能已产生用量，改人要连带处理退补款——后端只放行
// recruiting/confirmed，这里保持一致，不渲染改不了的按钮。
function canManageMembers(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' || carpool.status === 'confirmed'
}

function canEdit(carpool: Carpool): boolean {
  return carpool.status === 'recruiting' || carpool.status === 'confirmed'
}

function canTransfer(carpool: Carpool): boolean {
  return carpool.status !== 'cancelled' && carpool.status !== 'ended'
}

function canCancel(carpool: Carpool): boolean {
  // active 也放行：后端对 confirmed/active 只认 admin（本页就是管理员页）。
  // 取消已发车的车会软删全员订阅，确认弹窗里有单独的重警示文案。
  return carpool.status === 'recruiting' || carpool.status === 'confirmed'
    || carpool.status === 'starting' || carpool.status === 'active'
}

const confirmTitle = computed(() => {
  if (!confirmAction.value) return ''
  return t(`carpool.adminPage.confirm.${confirmAction.value.kind}.title`)
})

const confirmMessage = computed(() => {
  if (!confirmAction.value) return ''
  const { kind, carpool } = confirmAction.value
  // 取消已发车的车比取消招募中的车严重得多：全员订阅立即失效，文案要单独说透。
  if (kind === 'cancel' && carpool.status === 'active') {
    return t('carpool.adminPage.confirm.cancelActive.message', { name: carpool.name })
  }
  return t(`carpool.adminPage.confirm.${kind}.message`, { name: carpool.name })
})

function errorMessage(error: unknown): string {
  return extractApiErrorMessage(error, t('carpool.actionFailed'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    carpools.value = await carpoolAPI.adminOverview()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadRoster(carpoolID: number): Promise<void> {
  rosterLoading.value = true
  rosterFailed.value = false
  roster.value = []
  try {
    roster.value = await carpoolAPI.roster(carpoolID)
  } catch {
    rosterFailed.value = true
  } finally {
    rosterLoading.value = false
  }
}

function revokeQrCode(): void {
  if (qrCodeUrl.value) {
    URL.revokeObjectURL(qrCodeUrl.value)
    qrCodeUrl.value = null
  }
}

// 群二维码只在弹窗打开时现取（它是私密车的入场券，不该随列表预加载）；
// 关掉弹窗立刻吊销 object URL。
async function loadQrCode(carpoolID: number): Promise<void> {
  qrLoading.value = true
  qrFailed.value = false
  try {
    const blob = await carpoolAPI.groupQrCode(carpoolID)
    revokeQrCode()
    qrCodeUrl.value = URL.createObjectURL(blob)
  } catch {
    qrFailed.value = true
  } finally {
    qrLoading.value = false
  }
}

function openMembers(carpool: Carpool): void {
  activeCarpool.value = carpool
  editingUserId.value = null
  addDraft.userId = null
  addDraft.declaredInput = null
  addDraft.acknowledgedRisk = false
  membersOpen.value = true
  qrFailed.value = false
  void searchMemberUsers('')
  if (carpool.hasGroupQrCode) void loadQrCode(carpool.id)
  void loadRoster(carpool.id)
}

function closeMembers(): void {
  membersOpen.value = false
  revokeQrCode()
}

// —— 手动创建车辆 / 添加成员 ——

function userLabel(user: AdminUser): string {
  return user.username ? `${user.username}（${user.email}）` : user.email
}

// 远程搜索用户（邮箱/用户名）。连输时先发的请求后回来会覆盖新结果，用序号丢弃。
async function searchMemberUsers(query: string): Promise<void> {
  const seq = ++memberSearchSeq
  memberUserLoading.value = true
  try {
    const res = await usersAPI.list(1, 10, { search: query || undefined })
    if (seq !== memberSearchSeq) return
    memberUserOptions.value = res.items.map((user) => ({ value: user.id, label: userLabel(user) }))
  } catch {
    // 搜索失败保留旧候选
  } finally {
    if (seq === memberSearchSeq) memberUserLoading.value = false
  }
}

// type 3 的申报口径是占全车额度的百分比，按周限额换算成美元提交；type 2 输入即美元。
function declaredInputToUsd(input: number, weeklyLimitUsd: number): number {
  return Math.round((input / 100) * weeklyLimitUsd * 100) / 100
}

// 代加成员的请求体：type 2/3 带申报与代录的风险确认；type 1/0 只带 user_id。
function buildAddMemberPayload(carType: CarpoolType, userId: number, declaredInput: number | null, acknowledgedRisk: boolean, weeklyLimitUsd: number): AddMemberRequest {
  const payload: AddMemberRequest = { user_id: userId }
  if (carType === 3) {
    payload.declared_weekly_quota_usd = declaredInputToUsd(declaredInput ?? 0, weeklyLimitUsd)
    payload.acknowledged_risk = acknowledgedRisk
  } else if (carType === 2) {
    payload.declared_weekly_quota_usd = declaredInput ?? 0
    payload.acknowledged_risk = acknowledgedRisk
  }
  return payload
}

function stageMember(): void {
  if (!canStageMember.value) return
  const userId = memberDraft.userId as number
  const label = memberUserOptions.value.find((o) => o.value === userId)?.label || `#${userId}`
  stagedMembers.value = [...stagedMembers.value, {
    userId,
    label,
    declaredInput: createIsQuota.value ? memberDraft.declaredInput : null,
    acknowledgedRisk: createIsQuota.value ? memberDraft.acknowledgedRisk : false,
  }]
  memberDraft.userId = null
  memberDraft.declaredInput = null
  memberDraft.acknowledgedRisk = false
}

function unstageMember(userId: number): void {
  stagedMembers.value = stagedMembers.value.filter((m) => m.userId !== userId)
}

function openCreate(): void {
  createForm.carType = 3
  createForm.name = ''
  createForm.description = ''
  createForm.visibility = 'public'
  createForm.scheduledStartAt = ''
  createForm.weeklyLimitUsd = DEFAULT_WEEKLY_LIMIT_USD
  createForm.ruleNote = ''
  createForm.groupQrCode = ''
  createQrError.value = ''
  stagedMembers.value = []
  memberDraft.userId = null
  memberDraft.declaredInput = null
  memberDraft.acknowledgedRisk = false
  memberUserOptions.value = []
  createOpen.value = true
  void searchMemberUsers('')
}

// 群二维码（type 2/3 需要）：读成 data URL，复用创建那套后端校验（png/jpeg/webp ≤2MB）。
function handleCreateQrFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  createQrError.value = ''
  createForm.groupQrCode = ''
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    createQrError.value = t('carpool.createDialog.qrInvalidType')
    input.value = ''
    return
  }
  if (file.size > 2 * 1024 * 1024) {
    createQrError.value = t('carpool.createDialog.qrTooLarge')
    input.value = ''
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    createForm.groupQrCode = typeof reader.result === 'string' ? reader.result : ''
  }
  reader.readAsDataURL(file)
}

// 先创建车，再逐个代加暂存的初始成员：失败的逐个列出来提示，成功的保留在车上。
async function submitCreate(): Promise<void> {
  if (!createFormValid.value || createPending.value) return
  createPending.value = true
  try {
    const payload: CreateCarpoolRequest = {
      name: createForm.name.trim(),
      description: createForm.description,
      visibility: createForm.visibility,
      // type 0/1 创建即 active，发车日无意义，给后端一个合法日期即可
      scheduled_start_at: createForm.scheduledStartAt || new Date().toISOString().slice(0, 10),
      // 契约：admin 手动创建时两项强制确认固定 true（后台代录）；二维码 type 0/1 不需要
      added_admin_wechat: true,
      acknowledged_risk: true,
      group_qr_code: createForm.groupQrCode,
      car_type: createForm.carType,
    }
    if (createForm.carType === 0) payload.rule_note = createForm.ruleNote.trim()
    if (createForm.carType === 1) payload.weekly_limit_usd = createForm.weeklyLimitUsd
    const { carpool } = await carpoolAPI.create(payload)
    const failed: string[] = []
    for (const member of stagedMembers.value) {
      try {
        await carpoolAPI.addMember(carpool.id, buildAddMemberPayload(createForm.carType, member.userId, member.declaredInput, member.acknowledgedRisk, payload.weekly_limit_usd ?? DEFAULT_WEEKLY_LIMIT_USD))
      } catch {
        failed.push(member.label)
      }
    }
    createOpen.value = false
    if (failed.length > 0) {
      appStore.showWarning(t('carpool.adminPage.createDialog.membersFailed', { names: failed.join('、') }))
    } else {
      appStore.showSuccess(t('carpool.adminPage.createDialog.success'))
    }
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    createPending.value = false
  }
}

// 成员弹窗「添加成员」：type 2/3 需申报+代录风险（后端校验）；type 1/0 添加即生效。
async function submitAddMember(): Promise<void> {
  const car = activeCarpool.value
  if (!car || !canSubmitAddMember.value || memberPending.value) return
  memberPending.value = true
  try {
    const { autoUnconfirmed } = await carpoolAPI.addMember(
      car.id,
      buildAddMemberPayload(car.carType, addDraft.userId as number, addDraft.declaredInput, addDraft.acknowledgedRisk, addMemberWeeklyLimit.value))
    addDraft.userId = null
    addDraft.declaredInput = null
    addDraft.acknowledgedRisk = false
    reportMemberChange(autoUnconfirmed, 'carpool.adminPage.addMember.success')
    await Promise.all([load(), loadRoster(car.id)])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    memberPending.value = false
  }
}

// 更换群二维码：读成 data URL 直接复用创建那套后端校验（png/jpeg/webp ≤2MB）。
async function handleQrFileChange(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  const carpool = activeCarpool.value
  if (!file || !carpool) return
  if (file.size > 2 * 1024 * 1024) {
    appStore.showError(t('carpool.createDialog.qrTooLarge'))
    return
  }
  qrReplacing.value = true
  try {
    const dataUrl = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result))
      reader.onerror = () => reject(reader.error)
      reader.readAsDataURL(file)
    })
    await carpoolAPI.replaceGroupQrCode(carpool.id, dataUrl)
    appStore.showSuccess(t('carpool.adminPage.membersDialog.qrReplaced'))
    await Promise.all([loadQrCode(carpool.id), load()])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    qrReplacing.value = false
  }
}

function openEdit(carpool: Carpool): void {
  activeCarpool.value = carpool
  editForm.name = carpool.name
  editForm.description = carpool.description
  editForm.visibility = carpool.visibility
  editForm.scheduledStartAt = carpool.scheduledStartAt || ''
  editOpen.value = true
}

function openTransfer(carpool: Carpool): void {
  activeCarpool.value = carpool
  transferTargetId.value = null
  transferOpen.value = true
  void loadRoster(carpool.id)
}

function startEditQuota(userId: number, current: number): void {
  editingUserId.value = userId
  editingQuota.value = Math.round(current)
}

// autoUnconfirmed 要显式告诉管理员：他刚才的操作把车退回招募中了。
// 悄悄改状态是最容易让人以为「系统坏了」的那类行为。
function reportMemberChange(autoUnconfirmed: boolean, successKey: string): void {
  if (autoUnconfirmed) {
    appStore.showWarning(t('carpool.adminPage.autoUnconfirmed'))
  } else {
    appStore.showSuccess(t(successKey))
  }
}

async function saveQuota(userId: number): Promise<void> {
  const carpool = activeCarpool.value
  const quota = editingQuota.value
  if (!carpool || !quota || quota <= 0) return
  memberPending.value = true
  try {
    const { autoUnconfirmed } = await carpoolAPI.updateMemberQuota(carpool.id, userId, quota)
    editingUserId.value = null
    reportMemberChange(autoUnconfirmed, 'carpool.adminPage.quotaUpdated')
    await Promise.all([load(), loadRoster(carpool.id)])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    memberPending.value = false
  }
}

async function removeMember(userId: number): Promise<void> {
  const carpool = activeCarpool.value
  if (!carpool) return
  memberPending.value = true
  try {
    const { autoUnconfirmed } = await carpoolAPI.removeMember(carpool.id, userId)
    reportMemberChange(autoUnconfirmed, 'carpool.adminPage.memberRemoved')
    await Promise.all([load(), loadRoster(carpool.id)])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    memberPending.value = false
  }
}

async function saveEdit(): Promise<void> {
  const carpool = activeCarpool.value
  if (!carpool) return
  actionPending.value = true
  try {
    await carpoolAPI.updateCarpool(carpool.id, {
      name: editForm.name.trim(),
      description: editForm.description,
      visibility: editForm.visibility,
      scheduled_start_at: editForm.scheduledStartAt || undefined,
    })
    editOpen.value = false
    appStore.showSuccess(t('carpool.adminPage.updated'))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

async function saveTransfer(): Promise<void> {
  const carpool = activeCarpool.value
  if (!carpool || !transferTargetId.value) return
  actionPending.value = true
  try {
    await carpoolAPI.transferOwner(carpool.id, transferTargetId.value)
    transferOpen.value = false
    appStore.showSuccess(t('carpool.adminPage.ownerTransferred'))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

async function toggleLock(carpool: Carpool): Promise<void> {
  actionPending.value = true
  try {
    await carpoolAPI.setJoinLocked(carpool.id, !carpool.joinLocked)
    appStore.showSuccess(t(!carpool.joinLocked ? 'carpool.admin.locked' : 'carpool.admin.unlocked'))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

function run(kind: ConfirmKind, carpool: Carpool): void {
  confirmAction.value = { kind, carpool }
}

async function runConfirmed(): Promise<void> {
  const action = confirmAction.value
  if (!action) return
  actionPending.value = true
  try {
    if (action.kind === 'unconfirm') await carpoolAPI.unconfirm(action.carpool.id)
    else if (action.kind === 'launch') await carpoolAPI.launch(action.carpool.id)
    else await carpoolAPI.cancel(action.carpool.id)
    confirmAction.value = null
    appStore.showSuccess(t(`carpool.adminPage.confirm.${action.kind}.success`))
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    actionPending.value = false
  }
}

onMounted(load)
</script>

<style scoped>
/* 表格行内操作按钮：与 UsersView 同一套「图标 + 文字」约定。 */
.action-btn {
  @apply flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-primary-400;
}
.action-btn-primary {
  @apply hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-900/20 dark:hover:text-primary-400;
}
.action-btn-danger {
  @apply hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400;
}
.btn-xs {
  @apply rounded border border-gray-300 px-2 py-1 text-xs text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700;
}
.btn-xs-primary {
  @apply border-primary-500 text-primary-600 hover:bg-primary-50 dark:border-primary-500 dark:text-primary-400 dark:hover:bg-primary-900/20;
}
.btn-xs-danger {
  @apply border-red-300 text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/20;
}
</style>
