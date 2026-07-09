//go:build unit

package service

import (
	"testing"
)

// TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier locks in the fix
// that subscription-mode billing honours the group (and any user-specific) rate
// multiplier — i.e. cmd.SubscriptionCost tracks ActualCost (= TotalCost *
// RateMultiplier), not raw TotalCost.
func TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	subID := int64(42)

	tests := []struct {
		name           string
		totalCost      float64
		actualCost     float64
		isSubscription bool
		wantSub        float64
		wantBalance    float64
	}{
		{
			name:           "subscription with 2x multiplier consumes 2x quota",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			wantSub:        2.0,
			wantBalance:    0,
		},
		{
			name:           "subscription with 0.5x multiplier consumes 0.5x quota",
			totalCost:      1.0,
			actualCost:     0.5,
			isSubscription: true,
			wantSub:        0.5,
			wantBalance:    0,
		},
		{
			name:           "free subscription (multiplier 0) consumes no quota",
			totalCost:      1.0,
			actualCost:     0,
			isSubscription: true,
			wantSub:        0,
			wantBalance:    0,
		},
		{
			name:           "balance billing keeps using ActualCost (regression)",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: false,
			wantSub:        0,
			wantBalance:    2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &postUsageBillingParams{
				Cost:               &CostBreakdown{TotalCost: tt.totalCost, ActualCost: tt.actualCost},
				User:               &User{ID: 1},
				APIKey:             &APIKey{ID: 2, GroupID: &groupID},
				Account:            &Account{ID: 3},
				Subscription:       &UserSubscription{ID: subID},
				IsSubscriptionBill: tt.isSubscription,
			}

			cmd := buildUsageBillingCommand("req-1", nil, p)
			if cmd == nil {
				t.Fatal("buildUsageBillingCommand returned nil")
			}
			if cmd.SubscriptionCost != tt.wantSub {
				t.Errorf("SubscriptionCost = %v, want %v", cmd.SubscriptionCost, tt.wantSub)
			}
			if cmd.BalanceCost != tt.wantBalance {
				t.Errorf("BalanceCost = %v, want %v", cmd.BalanceCost, tt.wantBalance)
			}
		})
	}
}

func TestBuildUsageBillingCommand_ContributionRewardUsesActualCost(t *testing.T) {
	ownerID := int64(99)
	accountMultiplier := 1.5
	statsCost := 2.0
	p := &postUsageBillingParams{
		Cost:                          &CostBreakdown{TotalCost: 3.0, ActualCost: 4.0},
		User:                          &User{ID: 1},
		APIKey:                        &APIKey{ID: 2},
		Account:                       &Account{ID: 3, OwnerUserID: &ownerID, RateMultiplier: &accountMultiplier},
		AccountRateMultiplier:         accountMultiplier,
		ContributionRewardRatePercent: 80,
	}
	usageLog := &UsageLog{
		Model:            "gpt-test",
		AccountStatsCost: &statsCost,
	}

	cmd := buildUsageBillingCommand("req-contribution", usageLog, p)
	if cmd == nil {
		t.Fatal("buildUsageBillingCommand returned nil")
	}
	if cmd.ContributionPoolAccountID != p.Account.ID {
		t.Fatalf("ContributionPoolAccountID = %d, want %d", cmd.ContributionPoolAccountID, p.Account.ID)
	}
	// Contribution reward follows the consumer's actual charged cost, so group
	// or user-specific group multipliers are reflected in contributor earnings.
	if cmd.ContributionPoolAmount != 3.2 {
		t.Fatalf("ContributionPoolAmount = %v, want 3.2", cmd.ContributionPoolAmount)
	}
	if cmd.ContributionAccountStatsCost == nil || *cmd.ContributionAccountStatsCost != statsCost {
		t.Fatalf("ContributionAccountStatsCost = %#v, want %v", cmd.ContributionAccountStatsCost, statsCost)
	}
}

func TestBuildUsageBillingCommand_ContributionRewardTracksDiscountedActualCost(t *testing.T) {
	ownerID := int64(99)
	p := &postUsageBillingParams{
		Cost:                          &CostBreakdown{TotalCost: 10.0, ActualCost: 2.5},
		User:                          &User{ID: 1},
		APIKey:                        &APIKey{ID: 2},
		Account:                       &Account{ID: 3, OwnerUserID: &ownerID},
		AccountRateMultiplier:         1,
		ContributionRewardRatePercent: 80,
	}

	cmd := buildUsageBillingCommand("req-contribution-discount", nil, p)
	if cmd == nil {
		t.Fatal("buildUsageBillingCommand returned nil")
	}
	if cmd.ContributionPoolAmount != 2.0 {
		t.Fatalf("ContributionPoolAmount = %v, want 2.0", cmd.ContributionPoolAmount)
	}
}

func TestBuildUsageBillingCommand_SelfUseDoesNotReward(t *testing.T) {
	ownerID := int64(1)
	p := &postUsageBillingParams{
		Cost:                          &CostBreakdown{TotalCost: 3.0, ActualCost: 4.0},
		User:                          &User{ID: ownerID},
		APIKey:                        &APIKey{ID: 2},
		Account:                       &Account{ID: 3, OwnerUserID: &ownerID},
		AccountRateMultiplier:         1,
		ContributionRewardRatePercent: 80,
	}

	cmd := buildUsageBillingCommand("req-self", nil, p)
	if cmd == nil {
		t.Fatal("buildUsageBillingCommand returned nil")
	}
	if cmd.ContributionPoolAmount != 0 || cmd.ContributionPoolAccountID != 0 {
		t.Fatalf("self use should not reward, got pool_account=%d amount=%v", cmd.ContributionPoolAccountID, cmd.ContributionPoolAmount)
	}
}
