package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// quotaInheritanceRepoStub 记录 Create 收到的订阅，并按需返回一条"历史订阅"
// （模拟被撤销/软删的旧行）。实现了可选能力 subscriptionHistoryLookup。
type quotaInheritanceRepoStub struct {
	userSubRepoNoop
	previous    *UserSubscription
	previousErr error
	created     *UserSubscription
}

func (s *quotaInheritanceRepoStub) Create(_ context.Context, sub *UserSubscription) error {
	copied := *sub
	s.created = &copied
	sub.ID = 42
	return nil
}

func (s *quotaInheritanceRepoStub) GetByID(_ context.Context, _ int64) (*UserSubscription, error) {
	return s.created, nil
}

func (s *quotaInheritanceRepoStub) GetLatestIncludingDeletedByUserIDAndGroupID(_ context.Context, _, _ int64) (*UserSubscription, error) {
	if s.previousErr != nil {
		return nil, s.previousErr
	}
	if s.previous == nil {
		return nil, ErrSubscriptionNotFound
	}
	return s.previous, nil
}

func floatPtr(v float64) *float64 { return &v }

// 撤销订阅是软删除，ExistsByUserIDAndGroupID 看不到软删行，因此"撤销 + 重新分配"
// 会新建订阅行。不继承限额字段的话，拼车成员会失去保底额度、对组级公共池计数器
// 完全隐形，个人上限还会回落到分组级的整车周限额（一个人独占全车）。
func TestCreateSubscriptionInheritsCarpoolQuotaFromRevokedRow(t *testing.T) {
	repo := &quotaInheritanceRepoStub{
		previous: &UserSubscription{
			UserID:            7,
			GroupID:           3,
			WeeklyLimitUSD:    floatPtr(672),
			WeeklyReservedUSD: floatPtr(192),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	sub, err := svc.createSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 7, GroupID: 3, ValidityDays: 30,
	})
	require.NoError(t, err)
	require.NotNil(t, sub)

	require.NotNil(t, repo.created.WeeklyReservedUSD, "保底额度必须随重新分配继承，否则公共池计数器看不见这个人")
	require.InDelta(t, 192.0, *repo.created.WeeklyReservedUSD, 1e-9)
	require.NotNil(t, repo.created.WeeklyLimitUSD)
	require.InDelta(t, 672.0, *repo.created.WeeklyLimitUSD, 1e-9)
}

// 没有历史订阅（首次分配）时保持原样：新订阅不带订阅级覆盖，按分组级限额执行。
func TestCreateSubscriptionWithoutHistoryKeepsGroupLevelLimits(t *testing.T) {
	repo := &quotaInheritanceRepoStub{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	_, err := svc.createSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 7, GroupID: 3, ValidityDays: 30,
	})
	require.NoError(t, err)
	require.Nil(t, repo.created.WeeklyLimitUSD)
	require.Nil(t, repo.created.WeeklyReservedUSD)
}

// 历史查询失败不能阻塞分配（继承是尽力而为的补偿逻辑，不是分配的前置条件）。
func TestCreateSubscriptionSurvivesHistoryLookupFailure(t *testing.T) {
	repo := &quotaInheritanceRepoStub{previousErr: ErrSubscriptionNotFound}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	sub, err := svc.createSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 7, GroupID: 3, ValidityDays: 30,
	})
	require.NoError(t, err)
	require.NotNil(t, sub)
}
