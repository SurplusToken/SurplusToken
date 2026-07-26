package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

var (
	testPNGBytes  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	testJPEGBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	testGIFBytes  = []byte("GIF89a\x00\x00")
)

// launchFlowRepoStub 是可配置的 CarpoolRepository 桩：记录入参并返回预设结果。
type launchFlowRepoStub struct {
	createInput   *CreateCarpoolInput
	createResult  *CarpoolMutationResult
	joinResult    *CarpoolMutationResult
	confirmRes    *CarpoolMutationResult
	unconfirmRes  *CarpoolMutationResult
	launchResult  *CarpoolMutationResult
	pendingLaunch []CarpoolPendingLaunch
}

func (s *launchFlowRepoStub) List(ctx context.Context, userID int64) ([]Carpool, error) {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) GetByID(ctx context.Context, carpoolID, userID int64) (*Carpool, error) {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) GetByInvite(ctx context.Context, userID int64, tokenHash string) (*Carpool, error) {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) Create(ctx context.Context, ownerUserID int64, input CreateCarpoolInput, inviteHash, inviteHint string) (*CarpoolMutationResult, error) {
	s.createInput = &input
	return s.createResult, nil
}
func (s *launchFlowRepoStub) CreateInvite(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool, inviteHash, inviteHint string) error {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) Join(ctx context.Context, carpoolID, userID int64, declaredWeeklyQuotaUSD float64, joinedWechatGroup bool, inviteHash *string) (*CarpoolMutationResult, error) {
	return s.joinResult, nil
}
func (s *launchFlowRepoStub) Leave(ctx context.Context, carpoolID, userID int64) (*CarpoolMutationResult, error) {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) Confirm(ctx context.Context, carpoolID, ownerUserID int64) (*CarpoolMutationResult, error) {
	return s.confirmRes, nil
}
func (s *launchFlowRepoStub) Unconfirm(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) (*CarpoolMutationResult, error) {
	return s.unconfirmRes, nil
}
func (s *launchFlowRepoStub) ListPendingLaunch(ctx context.Context) ([]CarpoolPendingLaunch, error) {
	return s.pendingLaunch, nil
}
func (s *launchFlowRepoStub) Launch(ctx context.Context, carpoolID, actorUserID int64, isAdmin, force bool) (*CarpoolMutationResult, error) {
	return s.launchResult, nil
}
func (s *launchFlowRepoStub) Cancel(ctx context.Context, carpoolID, actorUserID int64, isAdmin bool) error {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) SetJoinLocked(ctx context.Context, carpoolID, actorUserID int64, locked bool) error {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) GetGroupQRCode(ctx context.Context, carpoolID int64) ([]byte, string, error) {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) ListSettlementMembers(ctx context.Context, carpoolID int64) ([]CarpoolSettlementMemberRow, error) {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) PersistSettlement(ctx context.Context, carpoolID, actorUserID int64, members []CarpoolSettlementMember) error {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) ClearSettlement(ctx context.Context, carpoolID, actorUserID int64) error {
	panic("unexpected call")
}
func (s *launchFlowRepoStub) GetRecentWeeklyUsageStats(ctx context.Context, userID int64) (float64, int, error) {
	panic("unexpected call")
}

// recordingSender 记录发送尝试，可配置为永远失败以验证优雅降级。
type recordingSender struct {
	fail    bool
	subject []string
	to      []string
	body    []string
}

func (s *recordingSender) SendEmail(ctx context.Context, to, subject, body string) error {
	s.to = append(s.to, to)
	s.subject = append(s.subject, subject)
	s.body = append(s.body, body)
	if s.fail {
		return errors.New("smtp not configured")
	}
	return nil
}

type stubUserDirectory struct {
	users  map[int64]*User
	admins []User
}

func (d *stubUserDirectory) GetByID(ctx context.Context, id int64) (*User, error) {
	if u, ok := d.users[id]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (d *stubUserDirectory) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	return d.admins, &pagination.PaginationResult{Total: int64(len(d.admins))}, nil
}

func newLaunchFlowService(repo CarpoolRepository, sender CarpoolEmailSender, dir CarpoolUserDirectory) *CarpoolService {
	return &CarpoolService{repo: repo, emailSender: sender, userDirectory: dir}
}

func TestParseCarpoolGroupQRCode(t *testing.T) {
	// data URL
	data, ct, err := parseCarpoolGroupQRCode("data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNGBytes))
	require.NoError(t, err)
	require.Equal(t, "image/png", ct)
	require.Equal(t, testPNGBytes, data)

	// 纯 base64（content-type 由内容 sniff）
	data, ct, err = parseCarpoolGroupQRCode(base64.StdEncoding.EncodeToString(testJPEGBytes))
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", ct)
	require.Equal(t, testJPEGBytes, data)

	// 空串 → required
	_, _, err = parseCarpoolGroupQRCode("  ")
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeRequired)

	// 非法 base64
	_, _, err = parseCarpoolGroupQRCode("!!!not-base64!!!")
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeInvalid)

	// GIF 不在允许类型内
	_, _, err = parseCarpoolGroupQRCode(base64.StdEncoding.EncodeToString(testGIFBytes))
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeInvalid)

	// 超过 2MB
	_, _, err = parseCarpoolGroupQRCode(base64.StdEncoding.EncodeToString(make([]byte, CarpoolGroupQRCodeMaxBytes+1)))
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeInvalid)

	// data URL 缺逗号
	_, _, err = parseCarpoolGroupQRCode("data:image/png;base64")
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeInvalid)
}

func TestCreateRequiresAddedAdminWechat(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := newLaunchFlowService(repo, nil, nil)
	input := CreateCarpoolInput{
		Name: "weekend-car", Visibility: CarpoolVisibilityPublic,
		GroupQRCode: "data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNGBytes),
	}
	_, err := svc.Create(context.Background(), 11, false, input)
	require.ErrorIs(t, err, ErrCarpoolContactConfirmRequired)
	require.Nil(t, repo.createInput, "确认缺失时不应触达 repo 层")
}

func TestCreateRequiresValidGroupQRCode(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := newLaunchFlowService(repo, nil, nil)
	base := CreateCarpoolInput{
		Name: "weekend-car", Visibility: CarpoolVisibilityPublic, AddedAdminWechat: true,
	}

	// 缺二维码 → required
	_, err := svc.Create(context.Background(), 11, false, base)
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeRequired)

	// 二维码类型不支持 → invalid
	bad := base
	bad.GroupQRCode = base64.StdEncoding.EncodeToString(testGIFBytes)
	_, err = svc.Create(context.Background(), 11, false, bad)
	require.ErrorIs(t, err, ErrCarpoolGroupQRCodeInvalid)
	require.Nil(t, repo.createInput)
}

func TestCreateStoresParsedQRCode(t *testing.T) {
	owner := int64(11)
	repo := &launchFlowRepoStub{createResult: &CarpoolMutationResult{Carpool: &Carpool{ID: 7, OwnerUserID: &owner, WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05}}}
	svc := newLaunchFlowService(repo, nil, nil)
	input := CreateCarpoolInput{
		Name: "weekend-car", Visibility: CarpoolVisibilityPublic, AddedAdminWechat: true,
		GroupQRCode: "data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNGBytes),
	}
	result, err := svc.Create(context.Background(), 11, false, input)
	require.NoError(t, err)
	require.NotNil(t, repo.createInput)
	require.True(t, repo.createInput.AddedAdminWechat)
	require.Equal(t, testPNGBytes, repo.createInput.GroupQRCodeBytes)
	require.Equal(t, "image/png", repo.createInput.GroupQRCodeContentType)
	require.NotEmpty(t, result.InviteToken)
	require.Equal(t, CarpoolAdminWechatID, result.Carpool.AdminWechat)
}

func TestJoinRequiresWechatGroupConfirmation(t *testing.T) {
	repo := &launchFlowRepoStub{}
	svc := newLaunchFlowService(repo, nil, nil)
	_, err := svc.Join(context.Background(), 7, 12, 100, false)
	require.ErrorIs(t, err, ErrCarpoolGroupJoinRequired)
	_, err = svc.JoinByInvite(context.Background(), "token", 12, 100, false)
	require.ErrorIs(t, err, ErrCarpoolGroupJoinRequired)
}

// 进区间通知：Join 使 Σ 进入发车区间后给车主发邮件；邮件失败（SMTP 未配置）不影响主流程。
func TestJoinNotifiesOwnerWhenBandEnteredWithEmailDegradation(t *testing.T) {
	owner := int64(11)
	repo := &launchFlowRepoStub{joinResult: &CarpoolMutationResult{
		LaunchBandEntered: true,
		Carpool: &Carpool{
			ID: 7, Name: "weekend-car", OwnerUserID: &owner,
			WeeklyLimitUSD: 2400, LaunchMinRatio: 0.95, LaunchMaxRatio: 1.05, DeclaredTotalUSD: 2350,
		},
	}}
	sender := &recordingSender{fail: true}
	dir := &stubUserDirectory{users: map[int64]*User{11: {ID: 11, Email: "owner@example.com"}}}
	svc := newLaunchFlowService(repo, sender, dir)

	result, err := svc.Join(context.Background(), 7, 12, 300, true)
	require.NoError(t, err, "邮件失败不得中断上车流程")
	require.True(t, result.LaunchBandEntered)
	require.Equal(t, []string{"owner@example.com"}, sender.to)
	require.Len(t, sender.subject, 1)
	require.Contains(t, sender.subject[0], "发车区间")
	require.Equal(t, CarpoolAdminWechatID, result.Carpool.AdminWechat)

	// 未进区间 → 不发邮件
	repo.joinResult.LaunchBandEntered = false
	sender.to, sender.subject = nil, nil
	_, err = svc.Join(context.Background(), 7, 13, 100, true)
	require.NoError(t, err)
	require.Empty(t, sender.to)
}

// 车主确认后给所有 admin 发邮件；邮件失败不影响确认结果。
func TestConfirmNotifiesAdminsWithEmailDegradation(t *testing.T) {
	owner := int64(11)
	repo := &launchFlowRepoStub{confirmRes: &CarpoolMutationResult{
		Carpool: &Carpool{ID: 7, Name: "weekend-car", OwnerUserID: &owner, Status: "confirmed", DeclaredTotalUSD: 2350, WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05},
	}}
	sender := &recordingSender{fail: true}
	dir := &stubUserDirectory{admins: []User{{ID: 1, Email: "a1@example.com"}, {ID: 2, Email: "a2@example.com"}}}
	svc := newLaunchFlowService(repo, sender, dir)

	result, err := svc.Confirm(context.Background(), 7, 11)
	require.NoError(t, err, "邮件失败不得中断确认流程")
	require.NotNil(t, result.Carpool)
	require.ElementsMatch(t, []string{"a1@example.com", "a2@example.com"}, sender.to)
	for _, subject := range sender.subject {
		require.True(t, strings.Contains(subject, "24 小时"), "主题应提示 24 小时内启动: %s", subject)
	}
}

// 启动成功后给每位成员发邮件；邮件失败不影响发车结果。
func TestLaunchNotifiesMembersWithEmailDegradation(t *testing.T) {
	owner := int64(11)
	repo := &launchFlowRepoStub{launchResult: &CarpoolMutationResult{
		ActivatedUserIDs: []int64{11, 12},
		ActivatedGroupID: 91,
		Carpool:          &Carpool{ID: 7, Name: "weekend-car", OwnerUserID: &owner, Status: "active", WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05},
	}}
	sender := &recordingSender{fail: true}
	dir := &stubUserDirectory{users: map[int64]*User{
		11: {ID: 11, Email: "owner@example.com"},
		12: {ID: 12, Email: "member@example.com"},
	}}
	svc := newLaunchFlowService(repo, sender, dir)

	result, err := svc.Launch(context.Background(), 7, 99, true, false)
	require.NoError(t, err, "邮件失败不得中断发车流程")
	require.NotNil(t, result.Carpool)
	require.ElementsMatch(t, []string{"owner@example.com", "member@example.com"}, sender.to)
	for _, subject := range sender.subject {
		require.Contains(t, subject, "已发车")
	}
}

// 响应展示字段：派生指标 + 硬编码管理员微信号。
func TestFillCarpoolPresentationSetsAdminWechat(t *testing.T) {
	c := &Carpool{WeeklyLimitUSD: 2400, LaunchMaxRatio: 1.05, DeclaredTotalUSD: 2400}
	fillCarpoolPresentation(c)
	require.Equal(t, "Charlemartingale", c.AdminWechat)
	require.InDelta(t, 20, c.PlusEquivalents, 1e-9)
}
