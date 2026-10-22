package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// 聊天功能全站下线：/api/v1/chat 路由组不再注册，任何调用都应得到 404。
// ChatHandler/ChatService/ChatRepository 源文件保留（wire 仍在注入），仅路由层摘除。
func TestChatRoutesAreNotRegistered(t *testing.T) {
	router := newAccountPoolTestRouter()

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "list conversations", method: http.MethodGet, path: "/api/v1/chat/conversations"},
		{name: "create conversation", method: http.MethodPost, path: "/api/v1/chat/conversations"},
		{name: "update conversation", method: http.MethodPatch, path: "/api/v1/chat/conversations/1"},
		{name: "delete conversation", method: http.MethodDelete, path: "/api/v1/chat/conversations/1"},
		{name: "list messages", method: http.MethodGet, path: "/api/v1/chat/conversations/1/messages"},
		{name: "completion", method: http.MethodPost, path: "/api/v1/chat/conversations/1/completions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Header.Set("Authorization", "Bearer admin-token")
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}
