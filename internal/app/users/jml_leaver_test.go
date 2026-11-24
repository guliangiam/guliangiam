package users

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/app/jml"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type fakeJMLPort struct {
	deactivated    []string
	lastPayload    []byte
	deactivateErr  error
	createdCalls   int
	bindingChanges int
}

func (f *fakeJMLPort) OnUserCreated(_ context.Context, _ appuow.UnitOfWork, _, _ string, _ []byte, _ []string) error {
	f.createdCalls++
	return nil
}

func (f *fakeJMLPort) OnRoleBindingChanged(_ context.Context, _ appuow.UnitOfWork, _, _ string, _, _ []string, _ []byte) error {
	f.bindingChanges++
	return nil
}

func (f *fakeJMLPort) OnUserDeactivated(_ context.Context, _ appuow.UnitOfWork, userID, _ string, payload []byte) error {
	f.deactivated = append(f.deactivated, userID)
	f.lastPayload = payload
	return f.deactivateErr
}

var _ jml.LifecyclePort = (*fakeJMLPort)(nil)

func leaverPatchFixture(t *testing.T, forbidden bool) (*Service, *fakeJMLPort) {
	t.Helper()
	testUser := newTestUser()
	testUser.IsForbidden = forbidden
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, _ string) (*userdomain.User, error) {
			if id != testUserID {
				return nil, apperrors.NewNotFound("user not found")
			}
			copy := *testUser
			return &copy, nil
		},
		updateForOrganizationFn: func(_ context.Context, _ *userdomain.User, _ string) error { return nil },
	}
	port := &fakeJMLPort{}
	svc := &Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)},
		UOW:   testUnitOfWork{users: userRepo},
		JML:   port,
	}
	return svc, port
}

func TestPatchUser_ForbiddenTransitionTriggersLeaver(t *testing.T) {
	svc, port := leaverPatchFixture(t, false)
	forbid := true
	require.NoError(t, svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		IsForbidden:    &forbid,
		Revision:       1, // 规范 4.10：写路径必须携带乐观锁前提
	}))
	require.Len(t, port.deactivated, 1)
	assert.Equal(t, testUserID, port.deactivated[0])
	var payload map[string]any
	require.NoError(t, json.Unmarshal(port.lastPayload, &payload))
	assert.Equal(t, testUserID, payload["user_id"])
	assert.Equal(t, false, payload["active"])
}

func TestPatchUser_AlreadyForbiddenNoLeaver(t *testing.T) {
	svc, port := leaverPatchFixture(t, true)
	forbid := true
	require.NoError(t, svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		IsForbidden:    &forbid,
		Revision:       1, // 规范 4.10：写路径必须携带乐观锁前提
	}))
	assert.Empty(t, port.deactivated)
}

func TestPatchUser_UnforbidNoLeaver(t *testing.T) {
	svc, port := leaverPatchFixture(t, true)
	forbid := false
	require.NoError(t, svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		IsForbidden:    &forbid,
		Revision:       1, // 规范 4.10：写路径必须携带乐观锁前提
	}))
	assert.Empty(t, port.deactivated)
}

func TestPatchUser_NilJMLKeepsLegacyBehavior(t *testing.T) {
	svc, _ := leaverPatchFixture(t, false)
	svc.JML = nil
	forbid := true
	require.NoError(t, svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		IsForbidden:    &forbid,
		Revision:       1, // 规范 4.10：写路径必须携带乐观锁前提
	}))
}

func leaverDeleteFixture(t *testing.T, port *fakeJMLPort, sessionsRevoked, tokensRevoked *bool) *Service {
	t.Helper()
	testUser := newTestUser()
	testOrg := newTestOrganization()
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, _ string) (*userdomain.User, error) {
			if id != testUserID {
				return nil, apperrors.NewNotFound("user not found")
			}
			copy := *testUser
			return &copy, nil
		},
		updateForOrganizationFn: func(_ context.Context, _ *userdomain.User, _ string) error { return nil },
	}
	orgRepo := &mockOrgRepo{
		getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
			if id != testOrganizationID {
				return nil, apperrors.NewNotFound("organization not found")
			}
			return testOrg, nil
		},
	}
	sessionRepo := &mockSessionRepo{
		revokeByUserExceptFn: func(_ context.Context, _, _ string, _ time.Time) (int64, error) {
			*sessionsRevoked = true
			return 0, nil
		},
	}
	tokenRepo := &mockTokenRepo{
		revokeByQueryFn: func(_ context.Context, _ tokendomain.RevokeQuery, _ time.Time) (int64, error) {
			*tokensRevoked = true
			return 0, nil
		},
	}
	return &Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)},
		UOW: testUnitOfWork{
			users:         userRepo,
			organizations: orgRepo,
			sessions:      sessionRepo,
			tokens:        tokenRepo,
			records:       &fakeRecordsRepo{},
		},
		JML: port,
	}
}

func TestDeleteUser_JMLLeaverReplacesLegacyRevocation(t *testing.T) {
	port := &fakeJMLPort{}
	sessionsRevoked, tokensRevoked := false, false
	svc := leaverDeleteFixture(t, port, &sessionsRevoked, &tokensRevoked)

	require.NoError(t, svc.DeleteUser(context.Background(), DeleteUserCommand{
		ID:             testUserID,
		OrganizationID: testOrganizationID,
	}))
	assert.Equal(t, []string{testUserID}, port.deactivated)
	assert.False(t, sessionsRevoked, "JML 注入时吊销由 Leaver 编排统一执行")
	assert.False(t, tokensRevoked, "JML 注入时吊销由 Leaver 编排统一执行")
}

func TestDeleteUser_NilJMLKeepsLegacyRevocation(t *testing.T) {
	sessionsRevoked, tokensRevoked := false, false
	svc := leaverDeleteFixture(t, nil, &sessionsRevoked, &tokensRevoked)
	svc.JML = nil

	require.NoError(t, svc.DeleteUser(context.Background(), DeleteUserCommand{
		ID:             testUserID,
		OrganizationID: testOrganizationID,
	}))
	assert.True(t, sessionsRevoked)
	assert.True(t, tokensRevoked)
}

func TestDeleteUser_JMLErrorPropagates(t *testing.T) {
	port := &fakeJMLPort{deactivateErr: apperrors.NewUnavailable("jml down")}
	sessionsRevoked, tokensRevoked := false, false
	svc := leaverDeleteFixture(t, port, &sessionsRevoked, &tokensRevoked)

	err := svc.DeleteUser(context.Background(), DeleteUserCommand{
		ID:             testUserID,
		OrganizationID: testOrganizationID,
	})
	require.Error(t, err)
}
