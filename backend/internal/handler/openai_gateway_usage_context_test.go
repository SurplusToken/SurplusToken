package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &GatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "client-request-123", gotClientRequestID)
	require.Equal(t, "request-456", gotRequestID)
}

func TestOpenAISubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "openai-client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "openai-request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &OpenAIGatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "openai-client-request-123", gotClientRequestID)
	require.Equal(t, "openai-request-456", gotRequestID)
}

// The deferred usage-billing task must carry the resolved serving group so
// shouldApplySharingRateBilling can detect a dynamic sharing pool and apply the
// sharing rate. Regression guard for the bug where external consumers of a
// contributed account were silently billed at 1x.
func TestUsageRecordTaskCarriesServingGroup(t *testing.T) {
	group := &service.Group{
		ID:                 12,
		Name:               "dynamic",
		DynamicSharingPool: true,
		SubscriptionType:   service.SubscriptionTypeStandard,
		Status:             service.StatusActive,
		Hydrated:           true,
	}
	parent := context.WithValue(context.Background(), ctxkey.Group, group)

	var gotGroup *service.Group
	h := &GatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotGroup, _ = ctx.Value(ctxkey.Group).(*service.Group)
	})

	require.NotNil(t, gotGroup)
	require.Equal(t, int64(12), gotGroup.ID)
	require.True(t, gotGroup.IsDynamicSharingPool())
}
