package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

type contributionPoolRepoCapture struct {
	req    PoolDistributionRequest
	result *PoolDistributionResult
	err    error
}

func (r *contributionPoolRepoCapture) GetPool(context.Context, int64) (float64, error) {
	return 0, nil
}

func (r *contributionPoolRepoCapture) ResolveDisplayNames(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}

func (r *contributionPoolRepoCapture) Distribute(_ context.Context, req PoolDistributionRequest) (*PoolDistributionResult, error) {
	r.req = req
	return r.result, r.err
}

func TestDistributeAccountPoolRequiresValidIdempotencyKeyAndMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		mode string
		want error
	}{
		{name: "missing key", mode: "even", want: ErrIdempotencyKeyRequired},
		{name: "invalid key", key: "bad\nkey", mode: "even", want: ErrIdempotencyKeyInvalid},
		{name: "invalid mode", key: "request-1", mode: "other", want: ErrContributionPoolDistributionModeInvalid},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := &contributionPoolRepoCapture{}
			svc := NewAccountContributionService(nil, repo)
			err := svc.DistributeAccountPool(context.Background(), 7, 9, test.key, test.mode, nil)
			require.ErrorIs(t, err, test.want)
			require.Empty(t, repo.req.IdempotencyKey)
		})
	}
}

func TestDistributeAccountPoolRejectsInvalidAllocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mode        string
		allocations []PoolAllocation
	}{
		{name: "even with allocations", mode: "even", allocations: []PoolAllocation{{UserID: 1, Amount: 1}}},
		{name: "custom empty", mode: "custom"},
		{name: "invalid recipient", mode: "custom", allocations: []PoolAllocation{{UserID: 0, Amount: 1}}},
		{name: "zero", mode: "custom", allocations: []PoolAllocation{{UserID: 1, Amount: 0}}},
		{name: "negative", mode: "custom", allocations: []PoolAllocation{{UserID: 1, Amount: -1}}},
		{name: "nan", mode: "custom", allocations: []PoolAllocation{{UserID: 1, Amount: math.NaN()}}},
		{name: "infinity", mode: "custom", allocations: []PoolAllocation{{UserID: 1, Amount: math.Inf(1)}}},
		{name: "duplicate", mode: "custom", allocations: []PoolAllocation{{UserID: 1, Amount: 1}, {UserID: 1, Amount: 2}}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := &contributionPoolRepoCapture{}
			svc := NewAccountContributionService(nil, repo)
			err := svc.DistributeAccountPool(context.Background(), 7, 9, "request-1", test.mode, test.allocations)
			require.ErrorIs(t, err, ErrContributionBadAllocation)
			require.Empty(t, repo.req.IdempotencyKey)
		})
	}
}

func TestDistributeAccountPoolCanonicalizesCustomPayload(t *testing.T) {
	t.Parallel()

	repo := &contributionPoolRepoCapture{result: &PoolDistributionResult{}}
	svc := NewAccountContributionService(nil, repo)
	err := svc.DistributeAccountPool(context.Background(), 7, 9, "  request-1  ", "custom", []PoolAllocation{
		{UserID: 12, Amount: 0.333333333},
		{UserID: 10, Amount: 1.25},
	})
	require.NoError(t, err)
	require.Equal(t, int64(9), repo.req.AccountID)
	require.Equal(t, int64(7), repo.req.RequesterUserID)
	require.Equal(t, "request-1", repo.req.IdempotencyKey)
	require.Equal(t, "custom", repo.req.Mode)
	require.Equal(t, []PoolAllocation{
		{UserID: 10, Amount: 1.25},
		{UserID: 12, Amount: 0.33333333},
	}, repo.req.Allocations)
	require.Len(t, repo.req.RequestFingerprint, 64)

	wantFingerprint := repo.req.RequestFingerprint
	err = svc.DistributeAccountPool(context.Background(), 7, 9, "request-2", "custom", []PoolAllocation{
		{UserID: 10, Amount: 1.25},
		{UserID: 12, Amount: 0.33333333},
	})
	require.NoError(t, err)
	require.Equal(t, wantFingerprint, repo.req.RequestFingerprint)
}
