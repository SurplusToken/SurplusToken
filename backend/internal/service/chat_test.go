package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type unavailableChatRepository struct{}

func (unavailableChatRepository) Available() bool { return false }
func (unavailableChatRepository) ListConversations(context.Context, int64, int, int) ([]ChatConversation, int64, error) {
	panic("unexpected call")
}
func (unavailableChatRepository) CreateConversation(context.Context, *ChatConversation) error {
	panic("unexpected call")
}
func (unavailableChatRepository) GetConversation(context.Context, int64, int64) (*ChatConversation, error) {
	panic("unexpected call")
}
func (unavailableChatRepository) UpdateConversation(context.Context, *ChatConversation) error {
	panic("unexpected call")
}
func (unavailableChatRepository) DeleteConversation(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (unavailableChatRepository) ListMessages(context.Context, int64, int64, int) ([]ChatMessage, error) {
	panic("unexpected call")
}
func (unavailableChatRepository) PrepareCompletion(context.Context, int64, int64, int64, string, string, string, string, string, int) (*PreparedChatCompletion, error) {
	panic("unexpected call")
}
func (unavailableChatRepository) CompleteAssistantMessage(context.Context, int64, int64, int64, string, string, string, json.RawMessage) error {
	panic("unexpected call")
}
func (unavailableChatRepository) FailAssistantMessage(context.Context, int64, int64, int64, string, string) error {
	panic("unexpected call")
}

func TestChatServiceUnavailableDoesNotCallRepository(t *testing.T) {
	svc := NewChatService(unavailableChatRepository{}, 100, 1000)
	_, _, err := svc.ListConversations(context.Background(), 1, 1, 20)
	require.ErrorIs(t, err, ErrChatUnavailable)
}

func TestNormalizeChatTitleUsesRuneLimit(t *testing.T) {
	title := normalizeChatTitle("这是一个会话标题")
	require.Equal(t, "这是一个会话标题", title)
	require.Len(t, []rune(normalizeChatTitle(strings.Repeat("界", 100))), 80)
}

func TestNormalizeChatReasoningEffort(t *testing.T) {
	tests := map[string]string{
		"":           "",
		"auto":       "",
		"default":    "",
		"none":       "none",
		"minimal":    "minimal",
		"LOW":        "low",
		"medium":     "medium",
		"high":       "high",
		"x-high":     "xhigh",
		"extra_high": "xhigh",
		"max":        "max",
		"ULTRA":      "ultra",
	}
	for input, expected := range tests {
		actual, err := NormalizeChatReasoningEffort(input)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	_, err := NormalizeChatReasoningEffort("turbo")
	require.Error(t, err)
}
