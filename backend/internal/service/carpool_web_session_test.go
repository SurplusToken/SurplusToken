//go:build unit

package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// webSessionRepoStub 是 CarpoolWebSessionRepository 的测试替身。
type webSessionRepoStub struct {
	carpool *service.Carpool
	account *service.CarpoolWebSessionAccount
}

func (s *webSessionRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*service.Carpool, error) {
	if s.carpool == nil {
		return nil, service.ErrCarpoolNotFound
	}
	return s.carpool, nil
}

func (s *webSessionRepoStub) GetWebSessionAccount(ctx context.Context, carpoolID int64) (*service.CarpoolWebSessionAccount, error) {
	return s.account, nil
}

// webSessionUserStub 是 CarpoolWebSessionUserDirectory 的测试替身。
type webSessionUserStub struct {
	user *service.User
	err  error
}

func (s *webSessionUserStub) GetByID(ctx context.Context, id int64) (*service.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

func webSessionMemberCarpool() *service.Carpool {
	role := "member"
	return &service.Carpool{ID: 10, Status: "active", Visibility: "invite_only", MemberRole: &role}
}

// newWebSessionFixture 组装完整 service：假网关（admin client）+ 测试签名密钥。
func newWebSessionFixture(t *testing.T, stub *webSessionRepoStub, users service.CarpoolWebSessionUserDirectory) *service.CarpoolWebSessionService {
	t.Helper()
	gateway := newD2wGatewayFixture(t)
	cfg, _ := writeD2wTestKey(t)
	cfg.Desktop2Web.AdminBaseURL = gateway.srv.URL
	cfg.Desktop2Web.AdminPassword = "admin-secret"
	return service.NewCarpoolWebSessionService(stub, users, service.NewD2wAdminClient(cfg), service.NewD2wTicketSigner(cfg))
}

func TestWebSessionDisabledWhenNotConfigured(t *testing.T) {
	stub := &webSessionRepoStub{carpool: webSessionMemberCarpool(), account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@example.com"}}
	svc := service.NewCarpoolWebSessionService(stub, nil, nil, nil)
	_, err := svc.Create(context.Background(), 10, 99, false)
	require.Error(t, err)
	require.Equal(t, 503, infraerrors.Code(err))
	require.Contains(t, err.Error(), "DESKTOP2WEB_NOT_CONFIGURED")
}

func TestWebSessionForbiddenForNonMember(t *testing.T) {
	// invite_only + active 的非成员不可见（upstream-quota 同闸：未上车者仅
	// public 且招募中可读，网页版按钮只对成员渲染）。
	stub := &webSessionRepoStub{carpool: &service.Carpool{ID: 10, Status: "active", Visibility: "invite_only"}, account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@example.com"}}
	svc := newWebSessionFixture(t, stub, nil)
	_, err := svc.Create(context.Background(), 10, 99, false)
	require.ErrorIs(t, err, service.ErrCarpoolForbidden)
}

func TestWebSessionForbiddenForNonMemberOnRecruitingPublicCar(t *testing.T) {
	// 比 upstream-quota 更严：额度信息允许 public+recruiting 的未上车者读，
	// 网页版直接消耗车上账号，未上车一律 403。
	stub := &webSessionRepoStub{carpool: &service.Carpool{ID: 10, Status: "recruiting", Visibility: "public"}, account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@example.com"}}
	svc := newWebSessionFixture(t, stub, nil)
	_, err := svc.Create(context.Background(), 10, 99, false)
	require.ErrorIs(t, err, service.ErrCarpoolForbidden)
}

func TestWebSessionAllowedForAdminWithoutMembership(t *testing.T) {
	stub := &webSessionRepoStub{carpool: &service.Carpool{ID: 10, Status: "active", Visibility: "invite_only"}, account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@example.com"}}
	users := &webSessionUserStub{user: &service.User{ID: 5, Username: "root"}}
	svc := newWebSessionFixture(t, stub, users)
	result, err := svc.Create(context.Background(), 10, 5, true)
	require.NoError(t, err)
	require.Equal(t, "http://d2w.test:4310", result.GatewayOrigin)
	require.NotEmpty(t, result.Ticket)
}

func TestWebSessionNotProvisionedWithoutAccount(t *testing.T) {
	stub := &webSessionRepoStub{carpool: webSessionMemberCarpool(), account: nil}
	svc := newWebSessionFixture(t, stub, nil)	_, err := svc.Create(context.Background(), 10, 99, false)
	require.Error(t, err)
	require.Equal(t, 409, infraerrors.Code(err))
	require.Contains(t, err.Error(), "WEB_SESSION_NOT_CONFIGURED")
}

func TestWebSessionNotProvisionedWithoutEmailOrSlot(t *testing.T) {
	// 账号有 email 就能推导 slot；没有 email 又没配 slot 覆盖 → 409。
	stub := &webSessionRepoStub{carpool: webSessionMemberCarpool(), account: &service.CarpoolWebSessionAccount{AccountID: 1}}
	svc := newWebSessionFixture(t, stub, nil)
	_, err := svc.Create(context.Background(), 10, 99, false)
	require.Error(t, err)
	require.Equal(t, 409, infraerrors.Code(err))
}

func TestWebSessionSlotOverrideWins(t *testing.T) {
	stub := &webSessionRepoStub{carpool: webSessionMemberCarpool(), account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@example.com", SlotID: "custom-slot-9"}}
	users := &webSessionUserStub{user: &service.User{ID: 42, Username: "小博"}}
	gateway := newD2wGatewayFixture(t)
	cfg, _ := writeD2wTestKey(t)
	cfg.Desktop2Web.AdminBaseURL = gateway.srv.URL
	cfg.Desktop2Web.AdminPassword = "admin-secret"
	svc := service.NewCarpoolWebSessionService(stub, users, service.NewD2wAdminClient(cfg), service.NewD2wTicketSigner(cfg))

	result, err := svc.Create(context.Background(), 10, 42, false)
	require.NoError(t, err)
	_, claims := decodeD2wTicket(t, result.Ticket)
	require.Equal(t, "custom-slot-9", claims["accessId"].(string)[len("st-42-"):])
	require.Equal(t, "小博", claims["displayName"])

	// PUT 载荷必须是 slot 覆盖值，而不是邮箱推导值。
	var putBody map[string]any
	require.NoError(t, json.Unmarshal([]byte(gateway.lastPutBody.Load().(string)), &putBody))
	require.Equal(t, "custom-slot-9", putBody["accountSlotId"])
	require.Equal(t, "chat-ui", putBody["role"])
	grant, ok := putBody["grantExpiresAtMs"].(float64)
	require.True(t, ok)
	require.Greater(t, grant, float64(time.Now().Add(29*24*time.Hour).UnixMilli()))
}

func TestWebSessionDerivesSlotFromEmail(t *testing.T) {
	stub := &webSessionRepoStub{carpool: webSessionMemberCarpool(), account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@surplustoken.com"}}
	users := &webSessionUserStub{user: &service.User{ID: 7, Username: ""}}
	svc := newWebSessionFixture(t, stub, users)

	result, err := svc.Create(context.Background(), 10, 7, false)
	require.NoError(t, err)
	_, claims := decodeD2wTicket(t, result.Ticket)
	require.Equal(t, "st-7-bob-at-surplustoken.com", claims["accessId"])
	// 用户名为空 → 归一 SurplusTokenLab。
	require.Equal(t, "SurplusTokenLab", claims["displayName"])
}

func TestWebSessionUserLookupFailureDegradesDisplayName(t *testing.T) {
	stub := &webSessionRepoStub{carpool: webSessionMemberCarpool(), account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@example.com"}}
	users := &webSessionUserStub{err: errors.New("db down")}
	svc := newWebSessionFixture(t, stub, users)

	result, err := svc.Create(context.Background(), 10, 7, false)
	require.NoError(t, err, "displayName 不是拒绝路径")
	_, claims := decodeD2wTicket(t, result.Ticket)
	require.Equal(t, "SurplusTokenLab", claims["displayName"])
}

func TestWebSessionRevokeForMemberDeletesAccess(t *testing.T) {
	// 下车/被移出后立即吊销：按 (userID, slotID) 推导 accessID 发 DELETE，
	// 连带吊销该 Access 下全部活跃会话。
	stub := &webSessionRepoStub{
		carpool: webSessionMemberCarpool(),
		account: &service.CarpoolWebSessionAccount{AccountID: 1, Email: "bob@surplustoken.com"},
	}
	gateway := newD2wGatewayFixture(t)
	cfg, _ := writeD2wTestKey(t)
	cfg.Desktop2Web.AdminBaseURL = gateway.srv.URL
	cfg.Desktop2Web.AdminPassword = "admin-secret"
	svc := service.NewCarpoolWebSessionService(stub, nil, service.NewD2wAdminClient(cfg), service.NewD2wTicketSigner(cfg))

	ctx := context.Background()
	// 先供给再吊销，验证精确命中同一 accessID。
	_, err := svc.EnsureChatUIAccess(ctx, "st-42-bob-at-surplustoken.com", "42", "bob-at-surplustoken.com", time.Now().Add(30*24*time.Hour).UnixMilli())
	require.NoError(t, err)
	require.NoError(t, svc.RevokeForMember(ctx, 10, 42))
	require.Equal(t, int32(1), gateway.deleteCalls.Load())
	require.Equal(t, "/__backend/admin/v1/accesses/st-42-bob-at-surplustoken.com", gateway.lastDeletePath.Load().(string))
	remaining, err := svc.ListAccesses(ctx)
	require.NoError(t, err)
	require.Empty(t, remaining, "吊销后 Access 列表应为空")
}
