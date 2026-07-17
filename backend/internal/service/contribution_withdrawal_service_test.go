package service

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type contributionWithdrawalRepoCapture struct {
	createCalls int
	createUser  int64
	createReq   CreateContributionWithdrawalRequest
	reviewCalls int
	reviewReq   ReviewContributionWithdrawalRequest
	adminStatus string
	adminSearch string
	adminPage   int
	adminSize   int
}

func (r *contributionWithdrawalRepoCapture) EnsureUserContribution(context.Context, int64) (*ContributionSummary, error) {
	return &ContributionSummary{}, nil
}

func (r *contributionWithdrawalRepoCapture) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (r *contributionWithdrawalRepoCapture) TransferQuotaToBalance(context.Context, int64) (*ContributionTransferResponse, error) {
	return &ContributionTransferResponse{}, nil
}

func (r *contributionWithdrawalRepoCapture) CreateWithdrawal(_ context.Context, userID int64, req CreateContributionWithdrawalRequest) (*ContributionWithdrawal, error) {
	r.createCalls++
	r.createUser = userID
	r.createReq = req
	return &ContributionWithdrawal{ID: 11, UserID: userID, Status: ContributionWithdrawalStatusPending}, nil
}

func (r *contributionWithdrawalRepoCapture) ListWithdrawals(context.Context, int64, int, int) ([]ContributionWithdrawal, int64, error) {
	return nil, 0, nil
}

func (r *contributionWithdrawalRepoCapture) ListWithdrawalsAdmin(_ context.Context, status, search string, page, pageSize int) ([]ContributionWithdrawal, int64, error) {
	r.adminStatus = status
	r.adminSearch = search
	r.adminPage = page
	r.adminSize = pageSize
	return nil, 0, nil
}

func (r *contributionWithdrawalRepoCapture) CancelWithdrawal(context.Context, int64, int64) (*ContributionWithdrawal, error) {
	return &ContributionWithdrawal{}, nil
}

func (r *contributionWithdrawalRepoCapture) ReviewWithdrawal(_ context.Context, req ReviewContributionWithdrawalRequest) (*ContributionWithdrawal, error) {
	r.reviewCalls++
	r.reviewReq = req
	return &ContributionWithdrawal{ID: req.WithdrawalID, UserID: 9, Status: req.Status}, nil
}

func validContributionWithdrawalRequest() CreateContributionWithdrawalRequest {
	return CreateContributionWithdrawalRequest{
		Amount:         12.3456789,
		PaymentMethod:  " alipay ",
		PaymentAccount: " recipient@example.com ",
		PayeeName:      " Example User ",
		RequestNote:    " payout note ",
		IdempotencyKey: " withdrawal-1 ",
	}
}

func TestCreateContributionWithdrawalCanonicalizesAndFingerprintsRequest(t *testing.T) {
	t.Parallel()
	repo := &contributionWithdrawalRepoCapture{}
	svc := NewContributionService(repo, nil, nil)

	result, err := svc.CreateWithdrawal(context.Background(), 9, validContributionWithdrawalRequest())
	require.NoError(t, err)
	require.Equal(t, int64(11), result.ID)
	require.Equal(t, 1, repo.createCalls)
	require.Equal(t, int64(9), repo.createUser)
	require.Equal(t, 12.3456789, repo.createReq.Amount)
	require.Equal(t, "alipay", repo.createReq.PaymentMethod)
	require.Equal(t, "recipient@example.com", repo.createReq.PaymentAccount)
	require.Equal(t, "Example User", repo.createReq.PayeeName)
	require.Equal(t, "payout note", repo.createReq.RequestNote)
	require.Equal(t, "withdrawal-1", repo.createReq.IdempotencyKey)
	require.Len(t, repo.createReq.RequestFingerprint, 64)

	firstFingerprint := repo.createReq.RequestFingerprint
	second := validContributionWithdrawalRequest()
	second.IdempotencyKey = "withdrawal-2"
	_, err = svc.CreateWithdrawal(context.Background(), 9, second)
	require.NoError(t, err)
	require.Equal(t, firstFingerprint, repo.createReq.RequestFingerprint, "idempotency key must not change the payload fingerprint")
}

func TestCreateContributionWithdrawalRejectsInvalidPayloadBeforeRepository(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*CreateContributionWithdrawalRequest)
	}{
		{name: "amount below minimum", mutate: func(req *CreateContributionWithdrawalRequest) { req.Amount = 0.009 }},
		{name: "nan amount", mutate: func(req *CreateContributionWithdrawalRequest) { req.Amount = math.NaN() }},
		{name: "too many decimals", mutate: func(req *CreateContributionWithdrawalRequest) { req.Amount = 1.123456789 }},
		{name: "unsupported method", mutate: func(req *CreateContributionWithdrawalRequest) { req.PaymentMethod = "cash" }},
		{name: "missing account", mutate: func(req *CreateContributionWithdrawalRequest) { req.PaymentAccount = " " }},
		{name: "missing payee", mutate: func(req *CreateContributionWithdrawalRequest) { req.PayeeName = " " }},
		{name: "note too long", mutate: func(req *CreateContributionWithdrawalRequest) { req.RequestNote = strings.Repeat("a", 501) }},
		{name: "missing idempotency key", mutate: func(req *CreateContributionWithdrawalRequest) { req.IdempotencyKey = " " }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := &contributionWithdrawalRepoCapture{}
			svc := NewContributionService(repo, nil, nil)
			req := validContributionWithdrawalRequest()
			test.mutate(&req)
			_, err := svc.CreateWithdrawal(context.Background(), 9, req)
			require.Error(t, err)
			require.Zero(t, repo.createCalls)
		})
	}
}

func TestReviewContributionWithdrawalEnforcesTerminalStateEvidence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  ReviewContributionWithdrawalRequest
	}{
		{name: "paid requires reference", req: ReviewContributionWithdrawalRequest{WithdrawalID: 3, AdminUserID: 2, Status: "paid"}},
		{name: "rejected requires note", req: ReviewContributionWithdrawalRequest{WithdrawalID: 3, AdminUserID: 2, Status: "rejected"}},
		{name: "pending is not an admin decision", req: ReviewContributionWithdrawalRequest{WithdrawalID: 3, AdminUserID: 2, Status: "pending"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := &contributionWithdrawalRepoCapture{}
			svc := NewContributionService(repo, nil, nil)
			_, err := svc.ReviewWithdrawal(context.Background(), test.req)
			require.Error(t, err)
			require.Zero(t, repo.reviewCalls)
		})
	}
}

func TestReviewContributionWithdrawalCanonicalizesValidDecision(t *testing.T) {
	t.Parallel()
	repo := &contributionWithdrawalRepoCapture{}
	svc := NewContributionService(repo, nil, nil)

	_, err := svc.ReviewWithdrawal(context.Background(), ReviewContributionWithdrawalRequest{
		WithdrawalID:     3,
		AdminUserID:      2,
		Status:           " PAID ",
		ReviewNote:       " reconciled ",
		PaymentReference: " trade-123 ",
	})
	require.NoError(t, err)
	require.Equal(t, 1, repo.reviewCalls)
	require.Equal(t, ContributionWithdrawalStatusPaid, repo.reviewReq.Status)
	require.Equal(t, "reconciled", repo.reviewReq.ReviewNote)
	require.Equal(t, "trade-123", repo.reviewReq.PaymentReference)
}

func TestListContributionWithdrawalsAdminNormalizesFiltersAndPagination(t *testing.T) {
	t.Parallel()
	repo := &contributionWithdrawalRepoCapture{}
	svc := NewContributionService(repo, nil, nil)

	_, _, err := svc.ListWithdrawalsAdmin(context.Background(), " PAID ", " user@example.com ", 0, 500)
	require.NoError(t, err)
	require.Equal(t, "paid", repo.adminStatus)
	require.Equal(t, "user@example.com", repo.adminSearch)
	require.Equal(t, 1, repo.adminPage)
	require.Equal(t, 100, repo.adminSize)
}
