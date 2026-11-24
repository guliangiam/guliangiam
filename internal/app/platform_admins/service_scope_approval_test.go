package platform_admins

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformerrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// approved_by 的语义回归：Schema FK 指向 users.id（contracts/schema/00_platform.yaml），
// 审批者必须按 users.id 解析（GetByUserID），自审批比较目标 admin 的 UserID。

type scopeAdminRepoStub struct {
	byID         map[string]*platformadmins.PlatformAdmin
	byUserID     map[string]*platformadmins.PlatformAdmin
	gotByUserID  []string
	gotByIDCalls []string
}

func (r *scopeAdminRepoStub) GetByID(_ context.Context, id string) (*platformadmins.PlatformAdmin, error) {
	r.gotByIDCalls = append(r.gotByIDCalls, id)
	if a, ok := r.byID[id]; ok {
		return a, nil
	}
	return nil, platformerrors.NewNotFound("platform admin not found")
}
func (r *scopeAdminRepoStub) GetByUserID(_ context.Context, userID string) (*platformadmins.PlatformAdmin, error) {
	r.gotByUserID = append(r.gotByUserID, userID)
	if a, ok := r.byUserID[userID]; ok {
		return a, nil
	}
	return nil, platformerrors.NewNotFound("platform admin not found")
}
func (r *scopeAdminRepoStub) List(context.Context, platformadmins.ListOptions) ([]*platformadmins.PlatformAdmin, int64, error) {
	return nil, 0, nil
}
func (r *scopeAdminRepoStub) Create(context.Context, *platformadmins.PlatformAdmin) error { return nil }
func (r *scopeAdminRepoStub) Update(context.Context, *platformadmins.PlatformAdmin) error { return nil }
func (r *scopeAdminRepoStub) SetActive(context.Context, string, bool) error               { return nil }
func (r *scopeAdminRepoStub) Delete(context.Context, string) error                        { return nil }

type scopeRepoStub struct {
	scope       *platformadmins.PlatformAdminScope
	created     *platformadmins.PlatformAdminScope
	approvedID  string
	approvedBy  string
	approveCall bool
}

func (r *scopeRepoStub) GetByID(context.Context, string) (*platformadmins.PlatformAdminScope, error) {
	if r.scope == nil {
		return nil, platformerrors.NewNotFound("scope not found")
	}
	return r.scope, nil
}
func (r *scopeRepoStub) ListByPlatformAdmin(context.Context, string, platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return nil, nil
}
func (r *scopeRepoStub) Create(_ context.Context, scope *platformadmins.PlatformAdminScope) error {
	r.created = scope
	return nil
}
func (r *scopeRepoStub) Approve(_ context.Context, scopeID, approverID string) error {
	r.approveCall = true
	r.approvedID = scopeID
	r.approvedBy = approverID
	return nil
}
func (r *scopeRepoStub) Delete(context.Context, string) error { return nil }
func (r *scopeRepoStub) HasActiveScopeForTenant(context.Context, string, string) (bool, error) {
	return false, nil
}

func newScopeTestService(t *testing.T, admins *scopeAdminRepoStub, scopes *scopeRepoStub) Service {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockUow := appuowmock.NewMockUnitOfWork(ctrl)
	mockUow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUow)
		}).AnyTimes()
	mockUow.EXPECT().PlatformAdmins().Return(admins).AnyTimes()
	mockUow.EXPECT().PlatformAdminScopes().Return(scopes).AnyTimes()
	return NewService(clock.SystemClock{}, platformuuid.V7Generator{}, mockUow, nil)
}

func scopeTestAdmins() *scopeAdminRepoStub {
	target := &platformadmins.PlatformAdmin{ID: "pa-1", UserID: "user-1", IsActive: true}
	approver := &platformadmins.PlatformAdmin{ID: "pa-2", UserID: "user-2", IsActive: true}
	return &scopeAdminRepoStub{
		byID:     map[string]*platformadmins.PlatformAdmin{"pa-1": target, "pa-2": approver},
		byUserID: map[string]*platformadmins.PlatformAdmin{"user-1": target, "user-2": approver},
	}
}

func TestCreateScope_ApproverResolvedByUserID(t *testing.T) {
	admins := scopeTestAdmins()
	scopes := &scopeRepoStub{}
	svc := newScopeTestService(t, admins, scopes)

	approverUserID := "user-2"
	created, err := svc.CreateScope(context.Background(), CreateScopeCommand{
		PlatformAdminID: "pa-1",
		ScopeType:       platformadmins.ScopeTypeAll,
		ApprovedBy:      &approverUserID,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	require.NotNil(t, scopes.created)
	assert.Equal(t, "user-2", *scopes.created.ApprovedBy)
	assert.Equal(t, "user-2", *created.ApprovedBy)
	assert.Contains(t, admins.gotByUserID, "user-2")
}

func TestCreateScope_SelfApprovalComparesUserID(t *testing.T) {
	admins := scopeTestAdmins()
	svc := newScopeTestService(t, admins, &scopeRepoStub{})

	selfUserID := "user-1"
	_, err := svc.CreateScope(context.Background(), CreateScopeCommand{
		PlatformAdminID: "pa-1",
		ScopeType:       platformadmins.ScopeTypeAll,
		ApprovedBy:      &selfUserID,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
	})
	require.Error(t, err)
	var pe *platformerrors.Error
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, platformerrors.CodeForbidden, pe.Code)
}

func TestCreateScope_ApproverMustBePlatformAdmin(t *testing.T) {
	admins := scopeTestAdmins()
	svc := newScopeTestService(t, admins, &scopeRepoStub{})

	outsider := "user-outsider"
	_, err := svc.CreateScope(context.Background(), CreateScopeCommand{
		PlatformAdminID: "pa-1",
		ScopeType:       platformadmins.ScopeTypeAll,
		ApprovedBy:      &outsider,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
	})
	require.Error(t, err)
	assert.Contains(t, admins.gotByUserID, "user-outsider")
}

func TestApproveScope_ApproverIsUserID(t *testing.T) {
	admins := scopeTestAdmins()
	scopes := &scopeRepoStub{scope: &platformadmins.PlatformAdminScope{
		ID: "sc-1", PlatformAdminID: "pa-1", ApprovedBy: nil,
	}}
	svc := newScopeTestService(t, admins, scopes)

	require.NoError(t, svc.ApproveScope(context.Background(), ApproveScopeCommand{
		ScopeID: "sc-1", ApproverID: "user-2",
	}))
	assert.True(t, scopes.approveCall)
	assert.Equal(t, "sc-1", scopes.approvedID)
	assert.Equal(t, "user-2", scopes.approvedBy)
	assert.Contains(t, admins.gotByUserID, "user-2")
}

func TestApproveScope_SelfApprovalComparesOwnerUserID(t *testing.T) {
	admins := scopeTestAdmins()
	scopes := &scopeRepoStub{scope: &platformadmins.PlatformAdminScope{
		ID: "sc-1", PlatformAdminID: "pa-1", ApprovedBy: nil,
	}}
	svc := newScopeTestService(t, admins, scopes)

	err := svc.ApproveScope(context.Background(), ApproveScopeCommand{
		ScopeID: "sc-1", ApproverID: "user-1",
	})
	require.Error(t, err)
	var pe *platformerrors.Error
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, platformerrors.CodeForbidden, pe.Code)
	assert.False(t, scopes.approveCall)
}
