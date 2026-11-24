package ldapsync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-B-10 regression: a lookup failure (e.g. database outage) must NOT fall
// through to the create branch — that would mask the outage as duplicate-key
// noise. The entry is counted as failed and no Create is attempted.
func TestSyncUsers_LookupDBError_DoesNotCreate_CountsFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := usersmock.NewMockRepository(ctrl)
	userRepo.EXPECT().
		GetByUsername(gomock.Any(), "org-1", "alice").
		Return(nil, apperrors.NewInternal("db is down"))
	// No Create expectation: gomock fails the test if the create branch runs.

	svc := NewService(nil, nil, userRepo)
	result := &ldaps.LdapSyncResult{}
	svc.syncUsers(context.Background(), &ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"},
		[]*ldapEntry{{Username: "alice"}}, result)

	assert.Equal(t, 1, result.UsersSynced)
	assert.Equal(t, 1, result.UsersFailed)
	assert.Equal(t, 0, result.UsersCreated)
	assert.Equal(t, 0, result.UsersUpdated)
}

// Only the platform NotFound code opens the create branch.
func TestSyncUsers_NotFound_CreatesUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := usersmock.NewMockRepository(ctrl)
	userRepo.EXPECT().
		GetByUsername(gomock.Any(), "org-1", "alice").
		Return(nil, apperrors.NewNotFound("user not found"))
	userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	svc := NewService(nil, nil, userRepo)
	result := &ldaps.LdapSyncResult{}
	svc.syncUsers(context.Background(), &ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"},
		[]*ldapEntry{{Username: "alice", DisplayName: "Alice"}}, result)

	assert.Equal(t, 1, result.UsersCreated)
	assert.Equal(t, 0, result.UsersFailed)
}

// A failed create is a failure, not a silent success.
func TestSyncUsers_CreateError_CountsFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := usersmock.NewMockRepository(ctrl)
	userRepo.EXPECT().
		GetByUsername(gomock.Any(), "org-1", "alice").
		Return(nil, apperrors.NewNotFound("user not found"))
	userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperrors.NewInternal("write failed"))

	svc := NewService(nil, nil, userRepo)
	result := &ldaps.LdapSyncResult{}
	svc.syncUsers(context.Background(), &ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"},
		[]*ldapEntry{{Username: "alice"}}, result)

	assert.Equal(t, 0, result.UsersCreated)
	assert.Equal(t, 1, result.UsersFailed)
}

// BE-B-10 regression: an update persistence failure must count as UsersFailed
// and must not be reported as a successful update.
func TestSyncUsers_UpdateError_CountsFailed_NotUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := &users.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        users.UserProfile{UserID: "user-1", DisplayName: "Old Name"},
	}
	userRepo := usersmock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").Return(existing, nil)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).Return(apperrors.NewInternal("write failed"))

	svc := NewService(nil, nil, userRepo)
	result := &ldaps.LdapSyncResult{}
	svc.syncUsers(context.Background(), &ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"},
		[]*ldapEntry{{Username: "alice", DisplayName: "New Name"}}, result)

	assert.Equal(t, 1, result.UsersSynced)
	assert.Equal(t, 1, result.UsersFailed)
	assert.Equal(t, 0, result.UsersUpdated)
}

// A successful profile change counts exactly one update and no failure.
func TestSyncUsers_UpdateSuccess_CountsUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := &users.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        users.UserProfile{UserID: "user-1", DisplayName: "Old Name"},
	}
	userRepo := usersmock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").Return(existing, nil)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *users.UserProfile) error {
			assert.Equal(t, "New Name", p.DisplayName)
			return nil
		})

	svc := NewService(nil, nil, userRepo)
	result := &ldaps.LdapSyncResult{}
	svc.syncUsers(context.Background(), &ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"},
		[]*ldapEntry{{Username: "alice", DisplayName: "New Name"}}, result)

	assert.Equal(t, 1, result.UsersUpdated)
	assert.Equal(t, 0, result.UsersFailed)
}

// An unchanged entry performs no write and inflates no counter.
func TestSyncUsers_NoChange_NoWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := &users.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Profile:        users.UserProfile{UserID: "user-1", DisplayName: "Same"},
	}
	userRepo := usersmock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").Return(existing, nil)
	// No UpdateProfile expectation.

	svc := NewService(nil, nil, userRepo)
	result := &ldaps.LdapSyncResult{}
	svc.syncUsers(context.Background(), &ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"},
		[]*ldapEntry{{Username: "alice", DisplayName: "Same"}}, result)

	assert.Equal(t, 1, result.UsersSynced)
	assert.Equal(t, 0, result.UsersUpdated)
	assert.Equal(t, 0, result.UsersFailed)
	assert.Equal(t, 0, result.UsersCreated)
}

// BE-B-18 pins: the sync path bounds the dial itself (SetTimeout only applies
// after the connection is established) and caps one search so SizeLimit=0
// (unlimited) can never reach the directory.
func TestSyncLDAPBoundsPinned(t *testing.T) {
	require.Equal(t, 10*time.Second, ldapDialTimeout)
	require.Equal(t, 10000, ldapSearchSizeLimit)
	assert.Greater(t, ldapSearchSizeLimit, 0)
}
