//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKimiAnthropicAPIEndpointCapabilities(t *testing.T) {
	account := &Account{
		Platform: PlatformKimi,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": KimiAPIAnthropicBaseURL,
			openAIEndpointCapabilitiesCredentialKey: []string{
				string(OpenAIEndpointCapabilityAnthropicMessages),
			},
		},
	}

	require.True(t, account.IsKimiAnthropicAPI())
	require.True(t, account.IsKimiNativeAnthropic())
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAnthropicMessages))
	require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
}

func TestKimiOpenAIEndpointSupportsMessagesThroughCompatibilityBridge(t *testing.T) {
	account := &Account{
		Platform: PlatformKimi,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": KimiAPIOpenAIBaseURL,
			openAIEndpointCapabilitiesCredentialKey: []string{
				string(OpenAIEndpointCapabilityChatCompletions),
			},
		},
	}

	require.False(t, account.IsKimiNativeAnthropic())
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAnthropicMessages))
}
