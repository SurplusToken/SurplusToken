package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// The new upstream token-count path must retain sharing eligibility without
// acquiring generation capacity. Owner self-use remains independent of the
// exhausted contribution budget.
func TestTokenCountRetainsSharingBudgetAndOwnerSelfUse(t *testing.T) {
	ownerID, groupID := int64(223), int64(78)
	spent := 500.0
	account := Account{
		ID: 186, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		GroupIDs: []int64{groupID}, OwnerUserID: &ownerID, OthersWeeklySpend: &spent,
		Credentials: map[string]any{"openai_capabilities": []any{"chat_completions"}},
		Extra:       map[string]any{"contribution_share_mode": ContributionShareModeBudget, "contribution_weekly_share_budget": 400.0},
	}
	for _, tc := range []struct {
		name    string
		userID  int64
		allowed bool
	}{
		{"non_owner_exhausted", 168, false}, {"owner_self_use", ownerID, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acquired := []int64{}
			svc := &OpenAIGatewayService{
				accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
				cache:       &schedulerTestGatewayCache{}, cfg: &config.Config{},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquired}),
			}
			selected, err := svc.SelectAccountForTokenCount(WithRequestingUserID(context.Background(), tc.userID), &groupID, "", "gpt-6-astra", OpenAIEndpointCapabilityChatCompletions, PlatformOpenAI)
			if tc.allowed {
				require.NoError(t, err)
				require.NotNil(t, selected)
				require.Equal(t, account.ID, selected.ID)
			} else {
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				require.Nil(t, selected)
			}
			require.Empty(t, acquired)
		})
	}
}

func TestSharedOwnerDoesNotBypassUpstreamPrivacyRequirement(t *testing.T) {
	ownerID := int64(223)
	account := &Account{ID: 186, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, OwnerUserID: &ownerID}
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{}}
	allowed, reason := scheduler.isAccountRequestCompatibleReason(WithRequestingUserID(context.Background(), ownerID), account, OpenAIAccountScheduleRequest{RequestedModel: "gpt-6-astra", RequirePrivacySet: true})
	require.False(t, allowed)
	require.Equal(t, "privacy_not_set", reason)
}
