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
	carpool      *service.Carpool
	invited      *service.Carpool
	qrData       []byte
	qrType       string
	qrErr        error
}

func (s *carpoolHandlerRepoStub) List(ctx context.Context, userID int64) ([]service.Carpool, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*service.Carpool, error) {
	if s.carpool == nil {
		return nil, service.ErrCarpoolNotFound
	}
	return s.carpool, nil
}
func (s *carpoolHandlerRepoStub) GetByInvite(ctx context.Context, userID int64, tokenHash string) (*service.Carpool, error) {
	if s.invited == nil {
		return nil, service.ErrCarpoolInviteInvalid
	}
	return s.invited, nil
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
func (s *carpoolHandlerRepoStub) Unconfirm(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*service.CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) ListPendingLaunch(ctx context.Context) ([]service.CarpoolPendingLaunch, error) {
	return nil, nil
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
func (s *carpoolHandlerRepoStub) PersistSettlement(ctx context.Context, carpoolID, actorUserID int64, members []service.CarpoolSettlementMember) error {
	panic("unexpected call")
}
func (s *carpoolHandlerRepoStub) ClearSettlement(ctx context.Context, carpoolID, actorUserID int64) error {
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

// publicRecruitingCarpool 是"任何登录用户都能看二维码"的基准车（招募中的公开车）。
func publicRecruitingCarpool() *service.Carpool {
	return &service.Carpool{ID: 7, Status: "recruiting", Visibility: service.CarpoolVisibilityPublic}
}

func inviteOnlyRecruitingCarpool() *service.Carpool {
	return &service.Carpool{ID: 7, Status: "recruiting", Visibility: service.CarpoolVisibilityInviteOnly}
}

func requestQRCode(t *testing.T, repo *carpoolHandlerRepoStub, query string) *httptest.ResponseRecorder {
	t.Helper()
	h := newCarpoolTestHandler(repo)
	c, recorder := newCarpoolTestContext(http.MethodGet, "/api/v1/carpools/7/qr-code"+query, "")
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	h.GroupQRCode(c)
	return recorder
}

// 群二维码：命中返回图片字节 + Content-Type + 缓存头；无二维码 → 404。
func TestCarpoolGroupQRCode(t *testing.T) {
	pngBytes, _ := base64.StdEncoding.DecodeString(carpoolHandlerPNGBase64)

	t.Run("found", func(t *testing.T) {
		recorder := requestQRCode(t, &carpoolHandlerRepoStub{
			carpool: publicRecruitingCarpool(), qrData: pngBytes, qrType: "image/png"}, "")
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
		require.Contains(t, recorder.Header().Get("Cache-Control"), "max-age")
		// 二维码按调用者身份授权，不能进共享缓存。
		require.Contains(t, recorder.Header().Get("Cache-Control"), "private")
		require.Equal(t, pngBytes, recorder.Body.Bytes())
	})

	t.Run("missing", func(t *testing.T) {
		recorder := requestQRCode(t, &carpoolHandlerRepoStub{
			carpool: publicRecruitingCarpool(), qrErr: service.ErrCarpoolQRCodeNotFound}, "")
		require.Equal(t, http.StatusNotFound, recorder.Code)
		_, reason := decodeCarpoolError(t, recorder)
		require.Equal(t, "CARPOOL_QR_CODE_NOT_FOUND", reason)
	})
}

// 私密车的群二维码等同入场券：非成员没有有效邀请时必须 403，
// 否则 List 的可见性控制形同虚设（任何登录用户猜 ID 就能进群）。
func TestCarpoolGroupQRCodeInviteOnlyRequiresInvite(t *testing.T) {
	pngBytes, _ := base64.StdEncoding.DecodeString(carpoolHandlerPNGBase64)

	t.Run("no token is forbidden", func(t *testing.T) {
		recorder := requestQRCode(t, &carpoolHandlerRepoStub{
			carpool: inviteOnlyRecruitingCarpool(), qrData: pngBytes, qrType: "image/png"}, "")
		require.Equal(t, http.StatusForbidden, recorder.Code)
		_, reason := decodeCarpoolError(t, recorder)
		require.Equal(t, "CARPOOL_FORBIDDEN", reason)
	})

	t.Run("invalid token is forbidden", func(t *testing.T) {
		recorder := requestQRCode(t, &carpoolHandlerRepoStub{
			carpool: inviteOnlyRecruitingCarpool(), qrData: pngBytes, qrType: "image/png"}, "?token=bogus")
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("token for a different carpool is forbidden", func(t *testing.T) {
		recorder := requestQRCode(t, &carpoolHandlerRepoStub{
			carpool: inviteOnlyRecruitingCarpool(),
			invited: &service.Carpool{ID: 99, Status: "recruiting", Visibility: service.CarpoolVisibilityInviteOnly},
			qrData:  pngBytes, qrType: "image/png"}, "?token=other-car")
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("valid token is allowed", func(t *testing.T) {
		recorder := requestQRCode(t, &carpoolHandlerRepoStub{
			carpool: inviteOnlyRecruitingCarpool(),
			invited: inviteOnlyRecruitingCarpool(),
			qrData:  pngBytes, qrType: "image/png"}, "?token=good")
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, pngBytes, recorder.Body.Bytes())
	})

	t.Run("member is always allowed", func(t *testing.T) {
		role := "member"
		car := inviteOnlyRecruitingCarpool()
		car.MemberRole = &role
		recorder := requestQRCode(t, &carpoolHandlerRepoStub{
			carpool: car, qrData: pngBytes, qrType: "image/png"}, "")
		require.Equal(t, http.StatusOK, recorder.Code)
	})
}

// 非 admin 传自定义额度参数 → 403，且不触达 repo。
func TestCarpoolCreateRejectsCustomQuotaParamsForNonAdmin(t *testing.T) {
	repo := &carpoolHandlerRepoStub{}
	h := newCarpoolTestHandler(repo)
	body := `{"name":"whale-car","visibility":"public","scheduled_start_at":"2026-08-01",` +
		`"added_admin_wechat":true,"weekly_limit_usd":1000000000,` +
		`"group_qr_code":"` + carpoolHandlerPNGBase64 + `"}`
	c, recorder := newCarpoolTestContext(http.MethodPost, "/api/v1/carpools", body)
	h.Create(c)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	_, reason := decodeCarpoolError(t, recorder)
	require.Equal(t, "CARPOOL_CUSTOM_PARAMS_FORBIDDEN", reason)
}

// 自定义规则咨询：冷却窗口内重复提交 → 429。
func TestCarpoolCustomRuleInterestIsRateLimited(t *testing.T) {
	h := newCarpoolTestHandler(&carpoolHandlerRepoStub{})

	c, recorder := newCarpoolTestContext(http.MethodPost, "/api/v1/carpools/custom-rule-interest", `{"note":"hi"}`)
	h.CustomRuleInterest(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	c, recorder = newCarpoolTestContext(http.MethodPost, "/api/v1/carpools/custom-rule-interest", `{"note":"hi again"}`)
	h.CustomRuleInterest(c)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	_, reason := decodeCarpoolError(t, recorder)
	require.Equal(t, "CARPOOL_INTEREST_TOO_FREQUENT", reason)
}

// 待启动列表仅 admin 可读。
func TestCarpoolPendingLaunchRequiresAdmin(t *testing.T) {
	h := newCarpoolTestHandler(&carpoolHandlerRepoStub{})
	c, recorder := newCarpoolTestContext(http.MethodGet, "/api/v1/carpools/pending-launch", "")
	h.PendingLaunch(c)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
