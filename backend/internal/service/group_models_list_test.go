package service

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestResolveAdvertisedModelsForGroup_UsesPlatformDefaults(t *testing.T) {
	group := &Group{Platform: PlatformOpenAI}
	models := resolveAdvertisedModelsForGroup(group, nil, false)

	require.NotEmpty(t, models)
	require.Equal(t, openai.DefaultModelIDs(), models)
}

func TestResolveAdvertisedModelsForGroup_PrefersAccountMappings(t *testing.T) {
	group := &Group{Platform: PlatformOpenAI}
	models := resolveAdvertisedModelsForGroup(group, []string{"custom-model", "gpt-*"}, false)

	require.Equal(t, []string{"custom-model", "gpt-*"}, models)
}

func TestResolveAdvertisedModelsForGroup_AppliesCustomAllowList(t *testing.T) {
	group := &Group{
		Platform: PlatformOpenAI,
		ModelsListConfig: GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"gpt-5", "not-available"},
		},
	}
	models := resolveAdvertisedModelsForGroup(group, []string{"gpt-*"}, false)

	require.Equal(t, []string{"gpt-5"}, models)
}

func TestResolveAdvertisedModelsForGroup_SharingFilterFailsClosed(t *testing.T) {
	group := &Group{Platform: PlatformOpenAI}
	models := resolveAdvertisedModelsForGroup(group, []string{}, true)

	require.NotNil(t, models)
	require.Empty(t, models)
}

func TestBuildSupportedModelsForDisplay_ExpandsWildcardsAndDeduplicates(t *testing.T) {
	pricingData := make(map[string]*LiteLLMModelPricing)
	for _, model := range openai.DefaultModelIDs() {
		pricingData[strings.ToLower(model)] = &LiteLLMModelPricing{
			InputCostPerToken:  1e-6,
			OutputCostPerToken: 2e-6,
		}
	}
	svc := NewChannelService(nil, nil, nil, &PricingService{pricingData: pricingData})
	models := svc.BuildSupportedModelsForDisplay(PlatformOpenAI, []string{"gpt-5*", "gpt-5", "GPT-5"})

	require.NotEmpty(t, models)
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		require.Equal(t, PlatformOpenAI, model.Platform)
		require.NotContains(t, model.Name, "*")
		key := strings.ToLower(model.Name)
		_, duplicate := seen[key]
		require.Falsef(t, duplicate, "duplicate model %q", model.Name)
		seen[key] = struct{}{}
	}
	require.Contains(t, seen, "gpt-5")
}
