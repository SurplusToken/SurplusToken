package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupRequestsAcceptZhipuPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		body   string
		target any
	}{
		{
			name:   "create",
			body:   `{"name":"glm","platform":"zhipu"}`,
			target: &CreateGroupRequest{},
		},
		{
			name:   "update",
			body:   `{"platform":"zhipu"}`,
			target: &UpdateGroupRequest{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = req

			require.NoError(t, ctx.ShouldBindJSON(tt.target))
		})
	}
}
