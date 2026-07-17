package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccount_ZhipuFirstClassAndLegacyCompatibility(t *testing.T) {
	firstClass := &Account{
		Platform: PlatformZhipu,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "key",
			"base_url": "https://open.bigmodel.cn/api/coding/paas/v4",
		},
	}
	require.True(t, firstClass.IsZhipu())
	require.True(t, firstClass.IsZhipuCoding())
	require.True(t, firstClass.IsOpenAICompatible())
	require.Equal(t, "zhipu", firstClass.OpenAICompatibleProvider())
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic", firstClass.GetZhipuAnthropicBaseURL())
	require.Equal(t, AnthropicAPIKeyAuthSchemeAuthorizationBearer, firstClass.GetAnthropicAPIKeyAuthScheme())

	legacy := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_compatible_provider": "zhipu"}}
	require.True(t, legacy.IsZhipu())
	require.Equal(t, "zhipu", legacy.OpenAICompatibleProvider())
}
