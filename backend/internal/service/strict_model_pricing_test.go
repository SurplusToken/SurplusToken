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

func TestStrictResolverUsesLiteLLMWithoutFallback(t *testing.T) {
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

	unpriced := resolver.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})
	if unpriced.Source != PricingSourceUnavailable || resolvedPricingUsable(unpriced) {
		t.Fatalf("built-in pricing must not enter strict resolver, got source=%q", unpriced.Source)
	}

	_, err := billing.CalculateCostUnified(CostInput{
		Ctx:      context.Background(),
		Model:    "claude-sonnet-4",
		Resolver: resolver,
	})
	if !errors.Is(err, ErrModelPricingUnavailable) {
		t.Fatalf("expected ErrModelPricingUnavailable, got %v", err)
	}
}
