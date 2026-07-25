package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var carpoolHandlerPNGBase64 = base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D})

// carpoolHandlerRepoStub 仅实现 handler 测试触达的方法，其余 panic。
type carpoolHandlerRepoStub struct {
	createResult *service.CarpoolMutationResult
	qrData       []byte
	qrType       string
	qrErr        error
}

func (s *carpoolHandlerRepoStub) List(ctx context.Context, userID int64) ([]service.Carpool, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*service.Carpool, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) GetByInvite(ctx context.Context, userID int64, tokenHash string) (*service.Carpool, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) Create(ctx context.Context, ownerUserID int64, input service.CreateCarpoolInput, inviteHash, inviteHint string) (*service.CarpoolMutationResult, error) {
	return s.createResult, nil
}
func (s *carpoolHandlerRepoStub) CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool, inviteHash, inviteHint string) error {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool, inviteHash *string) (*service.CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) Leave(ctx context.Context, carpoolID, userID int64) (*service.CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) Confirm(ctx context.Context, carpoolID, ownerUserID int64) (*service.CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*service.CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) GetGroupQRCode(ctx context.Context, carpoolID int64) ([]byte, string, error) {
	return s.qrData, s.qrType, s.qrErr
}
func (s *carpoolHandlerRepoStub) ListSettlementMembers(ctx context.Context, carpoolID int64) ([]service.CarpoolSettlementMemberRow, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) GetRecentWeeklyUsageStats(ctx context.Context, userID int64) (float64, int, error) {
	panic("unexpected call")
}

func newCarpoolTestContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})
	return c, recorder
}

func newCarpoolTestHandler(repo service.CarpoolRepository) *CarpoolHandler {
	return NewCarpoolHandler(service.NewCarpoolService(repo, nil, nil, nil))
}

func decodeCarpoolError(t *testing.T, recorder *httptest.ResponseRecorder) (code int, reason string) {
	t.Helper()
	var resp struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp.Code, resp.Reason
}

// 创建车辆：缺 added_admin_wechat → 400 CARPOOL_CONTACT_CONFIRM_REQUIRED。
func TestCarpoolCreateRequiresAddedAdminWechat(t *testing.T) {
	h := newCarpoolTestHandler(&carpoolHandlerRepoStub{})
	body := `{"name":"weekend-car","visibility":"public","scheduled_start_at":"2026-08-01","group_qr_code":"` + carpoolHandlerPNGBase64 + `"}`
	c, recorder := newCarpoolTestContext(http.MethodPost, "/api/v1/carpools", body)
	h.Create(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, reason := decodeCarpoolError(t, recorder)
	require.Equal(t, "CARPOOL_CONTACT_CONFIRM_REQUIRED", reason)
}

// 创建车辆：缺 group_qr_code → 400 CARPOOL_GROUP_QR_CODE_REQUIRED。
func TestCarpoolCreateRequiresGroupQRCode(t *testing.T) {
	h := newCarpoolTestHandler(&carpoolHandlerRepoStub{})
	body := `{"name":"weekend-car","visibility":"public","scheduled_start_at":"2026-08-01","added_admin_wechat":true}`
	c, recorder := newCarpoolTestContext(http.MethodPost, "/api/v1/carpools", body)
	h.Create(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, reason := decodeCarpoolError(t, recorder)
	require.Equal(t, "CARPOOL_GROUP_QR_CODE_REQUIRED", reason)
}

// 创建车辆：确认齐全 → 201，响应附带 admin_wechat。
func TestCarpoolCreateSuccessReturnsAdminWechat(t *testing.T) {
	owner := int64(11)
	repo := &carpoolHandlerRepoStub{createResult: &service.CarpoolMutationResult{
		Carpool: &service.Carpool{ID: 7, Name: "weekend-car", OwnerUserID: &owner, Status: "recruiting", WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05, HasGroupQRCode: true},
	}}
	h := newCarpoolTestHandler(repo)
	body := `{"name":"weekend-car","visibility":"public","scheduled_start_at":"2026-08-01","added_admin_wechat":true,"group_qr_code":"` + carpoolHandlerPNGBase64 + `"}`
	c, recorder := newCarpoolTestContext(http.MethodPost, "/api/v1/carpools", body)
	h.Create(c)
	require.Equal(t, http.StatusCreated, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Carpool struct {
				ID             int64  `json:"id"`
				AdminWechat    string `json:"admin_wechat"`
				HasGroupQRCode bool   `json:"has_group_qr_code"`
			} `json:"carpool"`
			InviteToken string `json:"invite_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "Charlemartingale", resp.Data.Carpool.AdminWechat)
	require.True(t, resp.Data.Carpool.HasGroupQRCode)
	require.NotEmpty(t, resp.Data.InviteToken)
}

// 上车：未勾选入群确认 → 400 CARPOOL_GROUP_JOIN_REQUIRED。
func TestCarpoolJoinRequiresWechatGroupConfirmation(t *testing.T) {
	h := newCarpoolTestHandler(&carpoolHandlerRepoStub{})
	c, recorder := newCarpoolTestContext(http.MethodPost, "/api/v1/carpools/7/join", `{"declared_weekly_quota_usd":100}`)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	h.Join(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, reason := decodeCarpoolError(t, recorder)
	require.Equal(t, "CARPOOL_GROUP_JOIN_REQUIRED", reason)
}

// 群二维码：命中返回图片字节 + Content-Type + 缓存头；无二维码 → 404。
func TestCarpoolGroupQRCode(t *testing.T) {
	pngBytes, _ := base64.StdEncoding.DecodeString(carpoolHandlerPNGBase64)

	t.Run("found", func(t *testing.T) {
		h := newCarpoolTestHandler(&carpoolHandlerRepoStub{qrData: pngBytes, qrType: "image/png"})
		c, recorder := newCarpoolTestContext(http.MethodGet, "/api/v1/carpools/7/qr-code", "")
		c.Params = gin.Params{{Key: "id", Value: "7"}}
		h.GroupQRCode(c)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
		require.Contains(t, recorder.Header().Get("Cache-Control"), "max-age")
		require.Equal(t, pngBytes, recorder.Body.Bytes())
	})

	t.Run("missing", func(t *testing.T) {
		h := newCarpoolTestHandler(&carpoolHandlerRepoStub{qrErr: service.ErrCarpoolQRCodeNotFound})
		c, recorder := newCarpoolTestContext(http.MethodGet, "/api/v1/carpools/7/qr-code", "")
		c.Params = gin.Params{{Key: "id", Value: "7"}}
		h.GroupQRCode(c)
		require.Equal(t, http.StatusNotFound, recorder.Code)
		_, reason := decodeCarpoolError(t, recorder)
		require.Equal(t, "CARPOOL_QR_CODE_NOT_FOUND", reason)
	})
}
