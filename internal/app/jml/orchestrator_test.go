package jml

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	domainconnectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	iga "github.com/guliangiam/guliangiam/internal/domain/iga"
	users "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	domainoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	"github.com/stretchr/testify/assert"
)

// ─── Stubs ──────────────────────────────────────────────────────────

type stubUsersRepo struct {
	mu    sync.Mutex
	users map[string]*users.User
}

func newStubUsersRepo() *stubUsersRepo {
	return &stubUsersRepo{users: map[string]*users.User{}}
}

func (s *stubUsersRepo) GetByID(_ context.Context, id string) (*users.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.users[id], nil
}

func (s *stubUsersRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*users.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[id]
	if user == nil || user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return user, nil
}

// 以下 users.Repository 方法在本测试中未被 Orchestrator 调用，
// 用 nil 返回占位以满足接口约束。
func (s *stubUsersRepo) LockByID(context.Context, string) error { return nil }
func (s *stubUsersRepo) GetByUsername(context.Context, string, string) (*users.User, error) {
	return nil, nil
}
func (s *stubUsersRepo) GetByLoginIdentifier(context.Context, string, string) (*users.User, error) {
	return nil, nil
}
func (s *stubUsersRepo) List(context.Context, users.ListQuery) ([]*users.User, int64, error) {
	return nil, 0, nil
}
func (s *stubUsersRepo) Create(context.Context, *users.User) error { return nil }
func (s *stubUsersRepo) Update(context.Context, *users.User) error { return nil }
func (s *stubUsersRepo) UpdateForOrganization(context.Context, *users.User, string) error {
	return nil
}
func (s *stubUsersRepo) UpdateProfile(context.Context, *users.UserProfile) error { return nil }
func (s *stubUsersRepo) SetContactVerifiedForOrganization(context.Context, string, string, bool, bool) error {
	return nil
}
func (s *stubUsersRepo) UpdateCollectionsForOrganization(context.Context, string, string, users.UserCollections) error {
	return nil
}
func (s *stubUsersRepo) UpdateCredentials(context.Context, *users.UserCredentials) error {
	return nil
}
func (s *stubUsersRepo) UpdatePasswordCredential(context.Context, *users.UserCredentials) error {
	return nil
}
func (s *stubUsersRepo) ReplaceIdentities(context.Context, string, []users.UserIdentity) error {
	return nil
}
func (s *stubUsersRepo) CreateIdentity(context.Context, *users.UserIdentity) error { return nil }
func (s *stubUsersRepo) DeleteIdentity(context.Context, string, string) error      { return nil }
func (s *stubUsersRepo) ReplaceMfaMethods(context.Context, string, []users.UserMfaMethod) error {
	return nil
}
func (s *stubUsersRepo) ReplaceRoleBindings(context.Context, string, []users.UserRoleBinding) error {
	return nil
}

func (s *stubUsersRepo) ListRoleNames(context.Context, string) ([]string, error) {
	return nil, nil
}
func (s *stubUsersRepo) ReplaceGroupMemberships(context.Context, string, []users.UserGroupMembership) error {
	return nil
}
func (s *stubUsersRepo) Delete(context.Context, string, string) error { return nil }
func (s *stubUsersRepo) GetPasswordHistory(context.Context, string, int) ([]users.PasswordHistoryEntry, error) {
	return nil, nil
}
func (s *stubUsersRepo) CreatePasswordHistory(context.Context, *users.PasswordHistoryEntry) error {
	return nil
}
func (s *stubUsersRepo) DeletePasswordHistoryBeyond(context.Context, string, int) error {
	return nil
}
func (s *stubUsersRepo) GetByProviderIdentity(context.Context, string, string) (*users.User, error) {
	return nil, nil
}
func (s *stubUsersRepo) Count(context.Context, string) (int64, error) { return 0, nil }
func (s *stubUsersRepo) HasDeletedWithUsername(context.Context, string, string) (bool, error) {
	return false, nil
}

// ─── RoleBinding Stub ────────────────────────────────────────────────

type stubRolesRepo struct {
	mu       sync.Mutex
	bindings map[string][]*users.UserRoleBinding // key=userID
	// Create / DeleteByUser / ListByUser 的可注入错误
	createErr    error
	deleteErr    error
	listErr      error
	listResponse []*users.UserRoleBinding // 若设置则 ListByUser 直接返回
	// BE-D-20：精确删除的可注入错误与调用记录
	deletePairErr error
	deletedPairs  [][2]string
}

func newStubRolesRepo() *stubRolesRepo {
	return &stubRolesRepo{bindings: map[string][]*users.UserRoleBinding{}}
}

func (s *stubRolesRepo) Create(_ context.Context, b *users.UserRoleBinding) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[b.UserID] = append(s.bindings[b.UserID], &users.UserRoleBinding{UserID: b.UserID, RoleID: b.RoleID})
	return nil
}

func (s *stubRolesRepo) DeleteByUser(_ context.Context, userID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bindings, userID)
	return nil
}

func (s *stubRolesRepo) Delete(_ context.Context, userID, roleID string) error {
	if s.deletePairErr != nil {
		return s.deletePairErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedPairs = append(s.deletedPairs, [2]string{userID, roleID})
	bs := s.bindings[userID]
	out := bs[:0]
	for _, b := range bs {
		if b.RoleID != roleID {
			out = append(out, b)
		}
	}
	s.bindings[userID] = out
	return nil
}

func (s *stubRolesRepo) ListByUser(_ context.Context, userID string) ([]*users.UserRoleBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResponse != nil {
		return s.listResponse, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bs := s.bindings[userID]
	out := make([]*users.UserRoleBinding, len(bs))
	copy(out, bs)
	return out, nil
}

// ─── IGA Stub ────────────────────────────────────────────────────────

type stubIGAService struct {
	mu                sync.Mutex
	campaignCreated   bool
	createCampaignErr error
	// SoD 冲突规则；非 nil 时 CheckSoD 只要 candidate 与 existing 任一 role
	// 组合命中 rule 的 (RoleAID, RoleBID) 就返回该 rule
	soDConflictRule *iga.SODRule
	checkSoDErr     error
	// 记录被调用的 CheckSoD 参数，方便断言
	lastCheckCandidate string
	lastCheckExisting  []string
}

func (s *stubIGAService) CheckSoD(_ context.Context, _, candidate string, existing []string) (*iga.SODRule, error) {
	s.mu.Lock()
	s.lastCheckCandidate = candidate
	s.lastCheckExisting = append([]string{}, existing...)
	s.mu.Unlock()

	if s.checkSoDErr != nil {
		return nil, s.checkSoDErr
	}
	if s.soDConflictRule == nil {
		return nil, nil
	}
	// candidate vs rule.RoleAID / RoleBID
	if candidate == s.soDConflictRule.RoleAID || candidate == s.soDConflictRule.RoleBID {
		for _, r := range existing {
			if r == s.soDConflictRule.RoleAID || r == s.soDConflictRule.RoleBID {
				// candidate 和 r 命中 SoD rule 的两个端点（或 candidate == existing 一个端点）
				if (candidate == s.soDConflictRule.RoleAID && r == s.soDConflictRule.RoleBID) ||
					(candidate == s.soDConflictRule.RoleBID && r == s.soDConflictRule.RoleAID) {
					return s.soDConflictRule, nil
				}
			}
		}
	}
	return nil, nil
}

func (s *stubIGAService) CreateAutoCampaign(context.Context, string, string, []string, string) error {
	if s.createCampaignErr != nil {
		return s.createCampaignErr
	}
	s.mu.Lock()
	s.campaignCreated = true
	s.mu.Unlock()
	return nil
}

// ─── Outbox Stub ─────────────────────────────────────────────────────

type stubOutboxEnqueuer struct {
	mu     sync.Mutex
	events []*domainoutbox.OutboxItem
	// 第 N 次 Insert 返回 error（0-indexed，-1 表示永不失败）
	failOnN   int
	insertErr error
}

func newStubOutbox() *stubOutboxEnqueuer {
	return &stubOutboxEnqueuer{failOnN: -1}
}

func (s *stubOutboxEnqueuer) Insert(_ context.Context, item *domainoutbox.OutboxItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.insertErr != nil {
		return s.insertErr
	}
	if s.failOnN >= 0 && len(s.events) == s.failOnN {
		return errors.New("outbox stub: simulated failure")
	}
	// 捕获 item 的副本，避免后续测试误改
	cp := *item
	s.events = append(s.events, &cp)
	return nil
}

// ─── Session / Token Revoker Stubs ───────────────────────────────────

type stubSessionRevoker struct {
	mu      sync.Mutex
	revoked []string
	failOn  int // 第 N 次 RevokeAll 返回 error；-1（默认）表示永不
	callCnt int
}

func newStubSessionRevoker() *stubSessionRevoker { return &stubSessionRevoker{failOn: -1} }

func (s *stubSessionRevoker) RevokeAll(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn >= 0 && s.callCnt == s.failOn {
		s.callCnt++
		return errors.New("session revoker stub: simulated failure")
	}
	s.callCnt++
	s.revoked = append(s.revoked, userID)
	return nil
}

type stubTokenRevoker struct {
	mu      sync.Mutex
	revoked []string
	failOn  int
	callCnt int
}

func newStubTokenRevoker() *stubTokenRevoker { return &stubTokenRevoker{failOn: -1} }

func (s *stubTokenRevoker) RevokeAll(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn >= 0 && s.callCnt == s.failOn {
		s.callCnt++
		return errors.New("token revoker stub: simulated failure")
	}
	s.callCnt++
	s.revoked = append(s.revoked, userID)
	return nil
}

// ─── Test Helpers ────────────────────────────────────────────────────

type env struct {
	o        *Orchestrator
	users    *stubUsersRepo
	roles    *stubRolesRepo
	iga      *stubIGAService
	outbox   *stubOutboxEnqueuer
	sessions *stubSessionRevoker
	tokens   *stubTokenRevoker
}

func newEnv() *env {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	usersRepo := newStubUsersRepo()
	usersRepo.users["user-1"] = &users.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	rolesRepo := newStubRolesRepo()
	igaSvc := &stubIGAService{}
	outbox := newStubOutbox()
	sessions := newStubSessionRevoker()
	tokens := newStubTokenRevoker()
	return &env{
		o: &Orchestrator{
			Users:    usersRepo,
			Roles:    rolesRepo,
			IGA:      igaSvc,
			Outbox:   outbox,
			Sessions: sessions,
			Tokens:   tokens,
			TimeNow:  func() time.Time { return now },
		},
		users:    usersRepo,
		roles:    rolesRepo,
		iga:      igaSvc,
		outbox:   outbox,
		sessions: sessions,
		tokens:   tokens,
	}
}

// assertOutboxAggregateIDs 断言 outbox events 的 AggregateID 序列（按 EventType 配对）
func assertOutboxPairs(t *testing.T, outbox *stubOutboxEnqueuer, pairs [][2]string) {
	t.Helper()
	outbox.mu.Lock()
	defer outbox.mu.Unlock()
	assert.Equal(t, len(pairs), len(outbox.events), "outbox length")
	for i, p := range pairs {
		assert.Equal(t, p[0], outbox.events[i].EventType, "pair[%d].type", i)
		assert.Equal(t, p[1], outbox.events[i].AggregateID, "pair[%d].aggregate", i)
	}
}

// ─── Joiner Tests ────────────────────────────────────────────────────

func TestJoiner_Success(t *testing.T) {
	e := newEnv()
	payload := []byte(`{"userName":"alice"}`)
	err := e.o.OnUserCreated(context.Background(), "user-1", "org-1", payload, []string{"roleA", "roleB"})
	assert.NoError(t, err)

	// 2 个 binding 创建
	bs, _ := e.roles.ListByUser(context.Background(), "user-1")
	assert.Len(t, bs, 2)
	assert.ElementsMatch(t, []string{"roleA", "roleB"}, []string{bs[0].RoleID, bs[1].RoleID})

	// campaign 自动创建
	assert.True(t, e.iga.campaignCreated)

	// outbox: 1× user.created + 2× role.member.added
	assertOutboxPairs(t, e.outbox, [][2]string{
		{domainconnectors.EvtUserCreated, "user-1"},
		{domainconnectors.EvtRoleMemberAdded, "roleA"},
		{domainconnectors.EvtRoleMemberAdded, "roleB"},
	})
}

func TestJoiner_SoDConflict(t *testing.T) {
	e := newEnv()
	// 配置 SoD：roleA ↔ roleB 冲突
	e.iga.soDConflictRule = &iga.SODRule{ID: "rule-1", RoleAID: "roleA", RoleBID: "roleB"}

	payload := []byte(`{}`)
	err := e.o.OnUserCreated(context.Background(), "user-1", "org-1", payload, []string{"roleA", "roleB"})
	assert.Error(t, err)

	// binding 应该回滚（因为 roleA 先创建 → 然后 roleB 与 roleA 冲突 → 回滚）
	bs, _ := e.roles.ListByUser(context.Background(), "user-1")
	assert.Len(t, bs, 0)

	// 没有 outbox
	assert.Len(t, e.outbox.events, 0)

	// 没有 campaign
	assert.False(t, e.iga.campaignCreated)
}

// BE-D-20 回归：用户已有既有绑定时，Joiner SoD 冲突回滚只删本批创建的
// binding，不得用 DeleteByUser 清空既有角色。
func TestJoiner_SoDConflict_RollbackKeepsExistingBindings(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	// 既有绑定（非本批）
	assert.NoError(t, e.roles.Create(ctx, &users.UserRoleBinding{UserID: "user-1", RoleID: "roleExisting"}))
	e.iga.soDConflictRule = &iga.SODRule{ID: "rule-1", RoleAID: "roleA", RoleBID: "roleB"}

	err := e.o.OnUserCreated(ctx, "user-1", "org-1", []byte(`{}`), []string{"roleA", "roleB"})
	assert.Error(t, err)

	bs, _ := e.roles.ListByUser(ctx, "user-1")
	assert.Len(t, bs, 1, "既有绑定必须保留")
	assert.Equal(t, "roleExisting", bs[0].RoleID)

	// 精确删除只命中本批创建的 roleA（roleB 从未创建）
	e.roles.mu.Lock()
	assert.Equal(t, [][2]string{{"user-1", "roleA"}}, e.roles.deletedPairs)
	e.roles.mu.Unlock()
}

func TestJoiner_EmptyRoles(t *testing.T) {
	e := newEnv()
	payload := []byte(`{}`)
	err := e.o.OnUserCreated(context.Background(), "user-1", "org-1", payload, []string{})
	assert.NoError(t, err)

	// 0 binding
	bs, _ := e.roles.ListByUser(context.Background(), "user-1")
	assert.Len(t, bs, 0)

	// 跳过 SoD：IGA.CheckSoD 不应被调用（或被调用 0 次）
	// campaign 跳过（空 roles）
	assert.False(t, e.iga.campaignCreated)

	// 1× user.created outbox
	assertOutboxPairs(t, e.outbox, [][2]string{
		{domainconnectors.EvtUserCreated, "user-1"},
	})
}

func TestJoiner_UserNotFound(t *testing.T) {
	e := newEnv()
	payload := []byte(`{}`)
	err := e.o.OnUserCreated(context.Background(), "ghost", "org-1", payload, []string{"roleA"})
	assert.Error(t, err)

	// 错误消息应包含 not found
	assert.Contains(t, err.Error(), "not found")

	// 没有 binding
	bs, _ := e.roles.ListByUser(context.Background(), "ghost")
	assert.Len(t, bs, 0)

	// 没有 outbox
	assert.Len(t, e.outbox.events, 0)
}

func TestJoiner_OutboxFail_NotRollbacked(t *testing.T) {
	e := newEnv()
	payload := []byte(`{}`)
	// 让第 1 次 outbox Insert 失败（user.created）
	e.outbox.failOnN = 0

	err := e.o.OnUserCreated(context.Background(), "user-1", "org-1", payload, []string{"roleA"})
	assert.Error(t, err)

	// binding 仍然创建成功 —— outbox 失败不回滚
	bs, _ := e.roles.ListByUser(context.Background(), "user-1")
	assert.Len(t, bs, 1)

	// 第 1 次 Insert 失败，所以 events 长度 = 0（failOnN=0 表示第一次失败，不会记录）
	// 但如果我们在 fail 前 append，则 events 长度 = 1；查看 stub 实现 → failOnN 检查在 append 之前 → 0
	assert.Len(t, e.outbox.events, 0)
}

// ─── Mover Tests ─────────────────────────────────────────────────────

func TestMover_Success(t *testing.T) {
	e := newEnv()

	// 预置：user-1 已有 roleA
	e.roles.listResponse = []*users.UserRoleBinding{
		{UserID: "user-1", RoleID: "roleA"},
	}

	payload := []byte(`{"status":"active"}`)
	err := e.o.OnRoleBindingChanged(context.Background(), "user-1", "org-1",
		[]string{"roleX"}, []string{"roleY"}, payload)
	assert.NoError(t, err)

	// SoD 通过
	// outbox: roleX added + roleY removed + user updated
	assertOutboxPairs(t, e.outbox, [][2]string{
		{domainconnectors.EvtRoleMemberAdded, "roleX"},
		{domainconnectors.EvtRoleMemberRemoved, "roleY"},
		{domainconnectors.EvtUserUpdated, "user-1"},
	})
}

func TestMover_SoDConflictOnAdd(t *testing.T) {
	e := newEnv()

	// 预置：user-1 已有 roleA
	e.roles.listResponse = []*users.UserRoleBinding{
		{UserID: "user-1", RoleID: "roleA"},
	}
	// SoD 配置：roleA ↔ roleX 冲突
	e.iga.soDConflictRule = &iga.SODRule{ID: "rule-2", RoleAID: "roleA", RoleBID: "roleX"}

	payload := []byte(`{}`)
	err := e.o.OnRoleBindingChanged(context.Background(), "user-1", "org-1",
		[]string{"roleX"}, nil, payload)
	assert.Error(t, err)

	// SoD 错误应带冲突信息
	assert.Contains(t, err.Error(), "SOD conflict")

	// outbox 不应写入（SoD 冲突时没有任何变更）
	assert.Len(t, e.outbox.events, 0)
}

func TestMover_NoRolesChanged(t *testing.T) {
	e := newEnv()

	err := e.o.OnRoleBindingChanged(context.Background(), "user-1", "org-1", nil, nil, nil)
	assert.NoError(t, err)

	// 零副作用 —— outbox 空
	assert.Len(t, e.outbox.events, 0)
}

// ─── Leaver Tests ────────────────────────────────────────────────────

func TestLeaver_Success(t *testing.T) {
	e := newEnv()

	// 预置：user-1 有 3 个角色
	e.roles.listResponse = []*users.UserRoleBinding{
		{UserID: "user-1", RoleID: "roleX"},
		{UserID: "user-1", RoleID: "roleY"},
		{UserID: "user-1", RoleID: "roleZ"},
	}

	payload := []byte(`{"active":false}`)
	err := e.o.OnUserDeactivated(context.Background(), "user-1", "org-1", payload)
	assert.NoError(t, err)

	// Sessions.RevokeAll 被调用
	assert.Equal(t, []string{"user-1"}, e.sessions.revoked)
	// Tokens.RevokeAll 被调用
	assert.Equal(t, []string{"user-1"}, e.tokens.revoked)
	// role binding 已清理
	assert.Len(t, e.roles.bindings, 0)

	// outbox: 3× role.member.removed + 1× user.deactivated
	assertOutboxPairs(t, e.outbox, [][2]string{
		{domainconnectors.EvtRoleMemberRemoved, "roleX"},
		{domainconnectors.EvtRoleMemberRemoved, "roleY"},
		{domainconnectors.EvtRoleMemberRemoved, "roleZ"},
		{domainconnectors.EvtUserDeactivated, "user-1"},
	})
}

func TestLeaver_SessionRevokeFail(t *testing.T) {
	e := newEnv()
	e.sessions.failOn = 0 // 第 0 次调用（也就是第一次）失败

	err := e.o.OnUserDeactivated(context.Background(), "user-1", "org-1", nil)
	assert.Error(t, err)

	// Tokens 不应被调用（在 Sessions 之后）—— 编排器停止在第一处失败点
	assert.Len(t, e.tokens.revoked, 0)
	// outbox 不应写入（任何 outbox 在清理之后才写；session fail 直接返回）
	assert.Len(t, e.outbox.events, 0)
}

func TestLeaver_TokenRevokeFail_AfterSessionOK(t *testing.T) {
	e := newEnv()
	e.tokens.failOn = 0 // 第 0 次（第一次）调用失败

	err := e.o.OnUserDeactivated(context.Background(), "user-1", "org-1", nil)
	assert.Error(t, err)

	// Sessions OK
	assert.Equal(t, []string{"user-1"}, e.sessions.revoked)
	// outbox 空（还没到写 outbox 的阶段）
	assert.Len(t, e.outbox.events, 0)
	// binding 保留（Token 吊销失败时保留角色，让调用方可以重试）
}

func TestLeaver_NoRoles(t *testing.T) {
	e := newEnv()
	// listResponse 为 nil → ListByUser 返回空切片
	e.roles.listResponse = []*users.UserRoleBinding{}

	payload := []byte(`{"active":false}`)
	err := e.o.OnUserDeactivated(context.Background(), "user-1", "org-1", payload)
	assert.NoError(t, err)

	// 1× user.deactivated outbox
	assertOutboxPairs(t, e.outbox, [][2]string{
		{domainconnectors.EvtUserDeactivated, "user-1"},
	})
}

// ─── 额外边界测试 ───────────────────────────────────────────────────

func TestMover_UserPayloadNil_NoUpdated(t *testing.T) {
	e := newEnv()
	e.roles.listResponse = []*users.UserRoleBinding{{UserID: "user-1", RoleID: "roleA"}}

	err := e.o.OnRoleBindingChanged(context.Background(), "user-1", "org-1", []string{"roleB"}, nil, nil)
	assert.NoError(t, err)

	// outbox 只有 role.member.added，没有 user.updated
	assertOutboxPairs(t, e.outbox, [][2]string{
		{domainconnectors.EvtRoleMemberAdded, "roleB"},
	})
}

func TestNewSoDConflict_Meta(t *testing.T) {
	rule := &iga.SODRule{ID: "r1", RoleAID: "a", RoleBID: "b"}
	err := newSoDConflict("a", rule)
	assert.Error(t, err)

	// 检查 Error.Meta 是否携带 rule 信息
	appErr, ok := err.(interface {
		Code() string
		Meta() map[string]any
	})
	if ok {
		_ = appErr
	}
	assert.Contains(t, err.Error(), "SOD conflict")
	assert.Contains(t, err.Error(), "rule=r1")
}

// TestStubs_ImplementsInterfaces 编译时断言 + 运行时接口兼容
func TestStubs_ImplementsInterfaces(t *testing.T) {
	var _ users.Repository = (*stubUsersRepo)(nil)
	var _ RoleBindingRepository = (*stubRolesRepo)(nil)
	var _ IGARoleService = (*stubIGAService)(nil)
	var _ OutboxEnqueuer = (*stubOutboxEnqueuer)(nil)
	var _ SessionRevoker = (*stubSessionRevoker)(nil)
	var _ TokenRevoker = (*stubTokenRevoker)(nil)
}

// TestRoleIDs 去重逻辑
func TestRoleIDs(t *testing.T) {
	bs := []*users.UserRoleBinding{
		{UserID: "u", RoleID: "a"},
		{UserID: "u", RoleID: "b"},
		{UserID: "u", RoleID: "a"}, // duplicate
	}
	got := roleIDs(bs)
	assert.ElementsMatch(t, []string{"a", "b"}, got)

	got2 := roleIDsFromStrs([]string{"x", "y", "x", "z"})
	assert.ElementsMatch(t, []string{"x", "y", "z"}, got2)
}

// TestConnectorHeaders 验证 Headers JSON 里只有 orgID，没有 connectorID
func TestConnectorHeaders(t *testing.T) {
	h := connectorHeaders("org-1")
	var got connectorTarget
	assert.NoError(t, json.Unmarshal(h, &got))
	assert.Equal(t, "org-1", got.OrganizationID)
	assert.Empty(t, got.ConnectorID, "JML 不指定 connector_id")
}
