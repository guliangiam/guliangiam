package jml

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainconnectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokensdomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	usersdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// ── 事务 UoW 与仓储 fake（只覆盖适配器触达的方法）──────────────────────

type adapterTxUow struct {
	appuow.UnitOfWork
	users    usersdomain.Repository
	outbox   domainoutbox.Repository
	sessions sessionsdomain.Repository
	tokens   tokensdomain.Repository
}

func (u adapterTxUow) Users() usersdomain.Repository       { return u.users }
func (u adapterTxUow) Outbox() domainoutbox.Repository     { return u.outbox }
func (u adapterTxUow) Sessions() sessionsdomain.Repository { return u.sessions }
func (u adapterTxUow) Tokens() tokensdomain.Repository     { return u.tokens }

type plainUsersRepo struct{ usersdomain.Repository }

type bindingUsersRepo struct {
	usersdomain.Repository
	created  []*usersdomain.UserRoleBinding
	deleted  [][2]string
	purged   []string
	bindings []usersdomain.UserRoleBinding
}

func (r *bindingUsersRepo) CreateRoleBinding(_ context.Context, b *usersdomain.UserRoleBinding) error {
	r.created = append(r.created, b)
	return nil
}
func (r *bindingUsersRepo) DeleteRoleBinding(_ context.Context, userID, roleID string) error {
	r.deleted = append(r.deleted, [2]string{userID, roleID})
	return nil
}
func (r *bindingUsersRepo) DeleteRoleBindingsByUser(_ context.Context, userID string) error {
	r.purged = append(r.purged, userID)
	return nil
}
func (r *bindingUsersRepo) ListRoleBindingsByUser(_ context.Context, _ string) ([]usersdomain.UserRoleBinding, error) {
	return r.bindings, nil
}

type captureOutboxRepo struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (r *captureOutboxRepo) Enqueue(_ context.Context, ev *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, ev)
	return nil
}

type captureSessionsRepo struct {
	sessionsdomain.Repository
	revoked []string
}

func (r *captureSessionsRepo) RevokeByUser(_ context.Context, userID string, _ time.Time) (int64, error) {
	r.revoked = append(r.revoked, userID)
	return 1, nil
}

type captureTokensRepo struct {
	tokensdomain.Repository
	refreshRevoked []string
	queryRevoked   []string
}

func (r *captureTokensRepo) RevokeRefreshByUser(_ context.Context, userID string, _ time.Time) (int64, error) {
	r.refreshRevoked = append(r.refreshRevoked, userID)
	return 1, nil
}
func (r *captureTokensRepo) RevokeByQuery(_ context.Context, q tokensdomain.RevokeQuery, _ time.Time) (int64, error) {
	r.queryRevoked = append(r.queryRevoked, q.UserID)
	return 1, nil
}

// ── SafeUserPayload ─────────────────────────────────────────────────────

func TestSafeUserPayload_MetadataOnly(t *testing.T) {
	u := &usersdomain.User{
		ID:                 "u-1",
		Username:           "alice",
		OrganizationID:     "org-1",
		ExternalID:         "ext-1",
		IsForbidden:        false,
		RecoveryCodeHashes: []string{"recovery-hash-secret"},
		TotpSecret:         "JBSWY3DPEHPK3PXP",
	}
	raw := SafeUserPayload(u)
	require.NotNil(t, raw)
	assert.NotContains(t, string(raw), "recovery-hash-secret")
	assert.NotContains(t, string(raw), "JBSWY3DPEHPK3PXP")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Equal(t, map[string]any{
		"user_id":         "u-1",
		"username":        "alice",
		"organization_id": "org-1",
		"external_id":     "ext-1",
		"active":          true,
	}, payload)
}

func TestSafeUserPayload_NilUser(t *testing.T) {
	assert.Nil(t, SafeUserPayload(nil))
}

func TestSafeUserPayload_ForbiddenIsInactive(t *testing.T) {
	raw := SafeUserPayload(&usersdomain.User{ID: "u-1", IsForbidden: true})
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Equal(t, false, payload["active"])
}

// ── roleBindingAdapter 能力断言 ─────────────────────────────────────────

func TestNewRoleBindingAdapter_CapabilityMissing(t *testing.T) {
	_, err := newRoleBindingAdapter(plainUsersRepo{})
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnavailable, typed.Code)
}

func TestRoleBindingAdapter_Delegates(t *testing.T) {
	repo := &bindingUsersRepo{bindings: []usersdomain.UserRoleBinding{
		{UserID: "u-1", RoleID: "r-1"},
		{UserID: "u-1", RoleID: "r-2"},
	}}
	adapter, err := newRoleBindingAdapter(repo)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, adapter.Create(ctx, &usersdomain.UserRoleBinding{UserID: "u-1", RoleID: "r-3"}))
	require.NoError(t, adapter.Delete(ctx, "u-1", "r-1"))
	require.NoError(t, adapter.DeleteByUser(ctx, "u-1"))
	list, err := adapter.ListByUser(ctx, "u-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "r-1", list[0].RoleID)

	assert.Len(t, repo.created, 1)
	assert.Equal(t, [][2]string{{"u-1", "r-1"}}, repo.deleted)
	assert.Equal(t, []string{"u-1"}, repo.purged)
}

// ── outbox / sessions / tokens 适配器 ───────────────────────────────────

func TestOutboxEnqueueAdapter_NilRepo(t *testing.T) {
	err := outboxEnqueueAdapter{}.Insert(context.Background(), &platformoutbox.OutboxItem{})
	require.Error(t, err)
}

func TestOutboxEnqueueAdapter_MapsFields(t *testing.T) {
	repo := &captureOutboxRepo{}
	orgID := "org-1"
	item := &platformoutbox.OutboxItem{
		ID:             "ev-1",
		OrganizationID: &orgID,
		AggregateType:  domainconnectors.AggregateType,
		AggregateID:    "u-1",
		EventType:      domainconnectors.EvtUserDeactivated,
		EventVersion:   1,
		Payload:        []byte(`{"user_id":"u-1"}`),
		MaxAttempts:    5,
	}
	require.NoError(t, (outboxEnqueueAdapter{repo: repo}).Insert(context.Background(), item))
	require.Len(t, repo.events, 1)
	ev := repo.events[0]
	assert.Equal(t, "ev-1", ev.ID)
	assert.Equal(t, domainconnectors.EvtUserDeactivated, ev.EventType)
	assert.Equal(t, "u-1", ev.AggregateID)
	assert.Equal(t, item.Payload, ev.Payload)
	assert.Equal(t, 5, ev.MaxAttempts)
}

func TestSessionRevokerAdapter_NilRepo(t *testing.T) {
	require.Error(t, sessionRevokerAdapter{now: time.Now}.RevokeAll(context.Background(), "u-1"))
}

func TestSessionRevokerAdapter_Delegates(t *testing.T) {
	repo := &captureSessionsRepo{}
	require.NoError(t, (sessionRevokerAdapter{repo: repo, now: time.Now}).RevokeAll(context.Background(), "u-1"))
	assert.Equal(t, []string{"u-1"}, repo.revoked)
}

func TestTokenRevokerAdapter_NilRepo(t *testing.T) {
	require.Error(t, tokenRevokerAdapter{now: time.Now}.RevokeAll(context.Background(), "u-1"))
}

func TestTokenRevokerAdapter_RevokesRefreshAndAccess(t *testing.T) {
	repo := &captureTokensRepo{}
	require.NoError(t, (tokenRevokerAdapter{repo: repo, now: time.Now}).RevokeAll(context.Background(), "u-1"))
	assert.Equal(t, []string{"u-1"}, repo.refreshRevoked)
	assert.Equal(t, []string{"u-1"}, repo.queryRevoked)
}

// ── UoWLifecycle 事务编排 ───────────────────────────────────────────────

func TestUoWLifecycle_OnUserDeactivated_OrchestratesLeaver(t *testing.T) {
	users := &bindingUsersRepo{bindings: []usersdomain.UserRoleBinding{
		{UserID: "u-1", RoleID: "r-1"},
		{UserID: "u-1", RoleID: "r-2"},
	}}
	outbox := &captureOutboxRepo{}
	sessions := &captureSessionsRepo{}
	tokens := &captureTokensRepo{}
	tx := adapterTxUow{users: users, outbox: outbox, sessions: sessions, tokens: tokens}

	lifecycle := NewUoWLifecycle(nil)
	payload := SafeUserPayload(&usersdomain.User{ID: "u-1", OrganizationID: "org-1", IsForbidden: true})
	require.NoError(t, lifecycle.OnUserDeactivated(context.Background(), tx, "u-1", "org-1", payload))

	assert.Equal(t, []string{"u-1"}, sessions.revoked)
	assert.Equal(t, []string{"u-1"}, tokens.refreshRevoked)
	assert.Equal(t, []string{"u-1"}, tokens.queryRevoked)
	assert.Equal(t, []string{"u-1"}, users.purged)

	// 出站顺序：先 role.member.removed×2，再 user.deactivated。
	require.Len(t, outbox.events, 3)
	assert.Equal(t, domainconnectors.EvtRoleMemberRemoved, outbox.events[0].EventType)
	assert.Equal(t, "r-1", outbox.events[0].AggregateID)
	assert.Equal(t, domainconnectors.EvtRoleMemberRemoved, outbox.events[1].EventType)
	assert.Equal(t, "r-2", outbox.events[1].AggregateID)
	assert.Equal(t, domainconnectors.EvtUserDeactivated, outbox.events[2].EventType)
	assert.Equal(t, "u-1", outbox.events[2].AggregateID)
	assert.Equal(t, payload, outbox.events[2].Payload)
}

func TestUoWLifecycle_CapabilityMissingFailsClosed(t *testing.T) {
	tx := adapterTxUow{users: plainUsersRepo{}}
	lifecycle := NewUoWLifecycle(nil)
	err := lifecycle.OnUserDeactivated(context.Background(), tx, "u-1", "org-1", nil)
	require.Error(t, err)
	var typed *apperrors.Error
	require.ErrorAs(t, err, &typed)
	assert.Equal(t, apperrors.CodeUnavailable, typed.Code)
}
