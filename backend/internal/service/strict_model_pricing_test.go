package service

import (
	"context"
	"errors"
	"testing"
)

func TestStrictPricingRejectsIncompleteChannelEntry(t *testing.T) {
	input := 1e-6
	pricing := &ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  &input,
	}
	if channelPricingUsable(pricing) {
		t.Fatal("channel token pricing without output price must be rejected")
	}
}

func TestStrictPricingAcceptsCompleteChannelEntry(t *testing.T) {
	input, output := 1e-6, 2e-6
	pricing := &ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  &input,
		OutputPrice: &output,
	}
	if !channelPricingUsable(pricing) {
		t.Fatal("complete channel token pricing must be accepted")
	}
}

func TestGetModelPricing_DeepSeekCNYPricePrecedesLiteLLM(t *testing.T) {
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"deepseek-v4-pro":   {InputCostPerToken: 4.35e-7, OutputCostPerToken: 8.7e-7, CacheReadInputTokenCost: 3.625e-9},
		"deepseek-v4-flash": {InputCostPerToken: 1.4e-7, OutputCostPerToken: 2.8e-7, CacheReadInputTokenCost: 2.8e-9},
		"deepseek-chat":     {InputCostPerToken: 2.8e-7, OutputCostPerToken: 4.2e-7, CacheReadInputTokenCost: 2.8e-8},
	}}
	billing := NewBillingService(nil, pricingService)

	tests := []struct {
		model       string
		wantInput   float64
		wantOutput  float64
		wantCacheIn float64
	}{
		{"deepseek-v4-pro", 3e-6, 6e-6, 2.5e-8},
		{"deepseek-v4-flash", 1e-6, 2e-6, 2e-8},
		{"deepseek-chat", 1e-6, 2e-6, 2e-8},
	}
	for _, tt := range tests {
		pricing, err := billing.GetModelPricing(tt.model)
		if err != nil {
			t.Fatalf("GetModelPricing(%s): %v", tt.model, err)
		}
		if pricing.InputPricePerToken != tt.wantInput || pricing.OutputPricePerToken != tt.wantOutput || pricing.CacheReadPricePerToken != tt.wantCacheIn {
			t.Fatalf("GetModelPricing(%s) = input %v output %v cache %v, want %v/%v/%v",
				tt.model, pricing.InputPricePerToken, pricing.OutputPricePerToken, pricing.CacheReadPricePerToken,
				tt.wantInput, tt.wantOutput, tt.wantCacheIn)
		}
	}
}

func TestResolverUsesLiteLLMThenBuiltInFallback(t *testing.T) {
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"priced-model": {
			InputCostPerToken:  1e-6,
			OutputCostPerToken: 2e-6,
		},
	}}
	billing := NewBillingService(nil, pricingService)
	resolver := NewModelPricingResolver(nil, billing)

	priced := resolver.Resolve(context.Background(), PricingInput{Model: "priced-model"})
	if priced.Source != PricingSourceLiteLLM || !resolvedPricingUsable(priced) {
		t.Fatalf("expected usable LiteLLM pricing, got source=%q", priced.Source)
	}

	fallback := resolver.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})
	if !resolvedPricingUsable(fallback) {
		t.Fatalf("expected usable built-in fallback pricing, got source=%q", fallback.Source)
	}

	unpriced := resolver.Resolve(context.Background(), PricingInput{Model: "totally-unknown-model"})
	if unpriced.Source != PricingSourceUnavailable || resolvedPricingUsable(unpriced) {
		t.Fatalf("unknown model must remain unavailable, got source=%q", unpriced.Source)
	}
	_, err := billing.CalculateCostUnified(CostInput{
		Ctx:      context.Background(),
		Model:    "totally-unknown-model",
		Resolver: resolver,
	})
	if !errors.Is(err, ErrModelPricingUnavailable) {
		t.Fatalf("expected ErrModelPricingUnavailable, got %v", err)
	}
}
