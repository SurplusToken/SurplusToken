package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountModelPricingOverrideFinalBilling(t *testing.T) {
	standardInput, standardOutput := 5e-6, 30e-6
	standardCacheRead, standardCacheWrite := 0.5e-6, 5e-6
	longInput, longOutput := 6e-6, 36e-6
	longCacheRead, longCacheWrite := 0.6e-6, 6e-6

	cache := newEmptyChannelCache()
	cache.loadedAt = time.Now()
	populateAccountModelPricingOverrides(cache, []AccountModelPricingOverride{{
		GroupID: 7, AccountID: 113, Platform: PlatformOpenAI,
		Pricing: ChannelModelPricing{
			Models: []string{"gpt-5.6-sol"}, BillingMode: BillingModeToken,
			InputPrice: &standardInput, OutputPrice: &standardOutput,
			CacheReadPrice: &standardCacheRead, CacheWritePrice: &standardCacheWrite,
			Intervals: []PricingInterval{{
				MinTokens: 200000, InputPrice: &longInput, OutputPrice: &longOutput,
				CacheReadPrice: &longCacheRead, CacheWritePrice: &longCacheWrite,
			}},
		},
	}})

	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	billingService := &BillingService{fallbackPrices: make(map[string]*ModelPricing)}
	resolver := NewModelPricingResolver(channelService, billingService)
	groupID, accountID := int64(7), int64(113)

	standardCost, err := billingService.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "gpt-5.6-sol", GroupID: &groupID, AccountID: &accountID,
		Tokens: UsageTokens{InputTokens: 200000}, RateMultiplier: 1, Resolver: resolver,
	})
	require.NoError(t, err)
	require.InDelta(t, 1.0, standardCost.ActualCost, 1e-9)

	longCost, err := billingService.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "gpt-5.6-sol", GroupID: &groupID, AccountID: &accountID,
		Tokens: UsageTokens{InputTokens: 200001}, RateMultiplier: 1, Resolver: resolver,
	})
	require.NoError(t, err)
	require.InDelta(t, 200001*longInput, longCost.ActualCost, 1e-9)

	otherAccountID := int64(117)
	otherAccountPricing := resolver.Resolve(context.Background(), PricingInput{
		Model: "gpt-5.6-sol", GroupID: &groupID, AccountID: &otherAccountID,
	})
	require.Equal(t, PricingSourceUnavailable, otherAccountPricing.Source)
}
