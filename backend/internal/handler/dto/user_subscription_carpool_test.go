package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserSubscriptionFromServiceSetsCarpoolMarker(t *testing.T) {
	reserved := 80.0

	tests := []struct {
		name      string
		reserved  *float64
		isCarpool bool
	}{
		{name: "weekly reserve", reserved: &reserved, isCarpool: true},
		{name: "ordinary subscription", reserved: nil, isCarpool: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := UserSubscriptionFromService(&service.UserSubscription{
				ID:                101,
				WeeklyReservedUSD: tt.reserved,
			})

			require.NotNil(t, out)
			require.Equal(t, tt.isCarpool, out.IsCarpool)

			raw, err := json.Marshal(out)
			require.NoError(t, err)
			require.NotContains(t, string(raw), "weekly_reserved_usd")
		})
	}
}
