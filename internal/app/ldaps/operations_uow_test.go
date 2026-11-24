package ldaps

import (
	"context"
	"fmt"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	ldapsmock "github.com/guliangiam/guliangiam/internal/domain/ldaps/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_TestConnection_WithClient_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockClient := ldapsmock.NewMockLDAPTestClient(ctrl)

	svc := Service{
		Clock:      clock.SystemClock{},
		UOW:        mockUOW,
		LDAPClient: mockClient,
	}

	ldap := &domain.Ldap{ID: "ldap1", Host: "localhost", Port: 389, BindDN: "cn=admin", BindPassword: "pass"}
	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockClient.EXPECT().TestConnection("localhost", 389, "cn=admin", "pass", false).Return(nil)

	err := svc.TestConnection(context.Background(), "ldap1", "org1")
	require.NoError(t, err)
}

func TestService_TestConnection_WithClient_TLS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockClient := ldapsmock.NewMockLDAPTestClient(ctrl)

	svc := Service{
		Clock:      clock.SystemClock{},
		UOW:        mockUOW,
		LDAPClient: mockClient,
	}

	ldap := &domain.Ldap{ID: "ldap1", Host: "localhost", Port: 636, BindDN: "cn=admin", BindPassword: "pass"}
	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockClient.EXPECT().TestConnection("localhost", 636, "cn=admin", "pass", true).Return(nil)

	err := svc.TestConnection(context.Background(), "ldap1", "org1")
	require.NoError(t, err)
}

func TestService_TestConnection_WithClient_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockClient := ldapsmock.NewMockLDAPTestClient(ctrl)

	svc := Service{
		Clock:      clock.SystemClock{},
		UOW:        mockUOW,
		LDAPClient: mockClient,
	}

	ldap := &domain.Ldap{ID: "ldap1", Host: "localhost", Port: 636, BindDN: "cn=admin", BindPassword: "pass"}
	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockClient.EXPECT().TestConnection("localhost", 636, "cn=admin", "pass", true).Return(assert.AnError)

	err := svc.TestConnection(context.Background(), "ldap1", "org1")
	assert.Error(t, err)
}

func TestService_PreviewUsers_WithSyncer_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockSyncer := ldapsmock.NewMockLDAPSyncer(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	svc := Service{
		Clock:      clock.SystemClock{},
		UOW:        mockUOW,
		LDAPSyncer: mockSyncer,
	}

	ldap := &domain.Ldap{ID: "ldap1", OrganizationID: "org1", Host: "localhost", Port: 389}
	previewUsers := []*domain.LDAPPreviewUser{
		{UUID: "uuid1", Username: "user1"},
		{UUID: "uuid2", Username: "user2"},
	}

	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockSyncer.EXPECT().PreviewUsersDomain(gomock.Any(), gomock.Any()).Return(previewUsers, nil)
	mockUOW.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*userdomain.User{}, int64(0), nil)

	result, err := svc.PreviewUsers(context.Background(), "ldap1", "org1")
	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalCount)
	assert.Equal(t, 0, result.ExistCount)
}

func TestService_PreviewUsers_WithSyncer_ExistingUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockSyncer := ldapsmock.NewMockLDAPSyncer(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	svc := Service{
		Clock:      clock.SystemClock{},
		UOW:        mockUOW,
		LDAPSyncer: mockSyncer,
	}

	ldap := &domain.Ldap{ID: "ldap1", OrganizationID: "org1", Host: "localhost", Port: 389}
	previewUsers := []*domain.LDAPPreviewUser{
		{UUID: "uuid1", Username: "user1"},
	}
	existingUser := &userdomain.User{
		ID: "u1",
		Identities: []userdomain.UserIdentity{
			{IdentityType: "ldap", Subject: "uuid1"},
		},
	}

	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockSyncer.EXPECT().PreviewUsersDomain(gomock.Any(), gomock.Any()).Return(previewUsers, nil)
	mockUOW.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*userdomain.User{existingUser}, int64(1), nil)

	result, err := svc.PreviewUsers(context.Background(), "ldap1", "org1")
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, 1, result.ExistCount)
	assert.True(t, result.Users[0].ExistsLocal)
}

func TestService_SyncLdap_DisabledLdap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockSyncer := ldapsmock.NewMockLDAPSyncer(ctrl)

	svc := Service{
		Clock:      clock.SystemClock{},
		UOW:        mockUOW,
		LDAPSyncer: mockSyncer,
	}

	ldap := &domain.Ldap{ID: "ldap1", OrganizationID: "org1", IsEnabled: false}
	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)

	_, err := svc.SyncLdap(context.Background(), "ldap1", nil, "org1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestService_SyncLdap_GetMappedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockSyncer := ldapsmock.NewMockLDAPSyncer(ctrl)

	svc := Service{
		Clock:      clock.SystemClock{},
		UOW:        mockUOW,
		LDAPSyncer: mockSyncer,
	}

	ldap := &domain.Ldap{ID: "ldap1", OrganizationID: "org1", IsEnabled: true, Host: "localhost", Port: 389}
	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockSyncer.EXPECT().GetMappedUsersDomain(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	_, err := svc.SyncLdap(context.Background(), "ldap1", nil, "org1")
	assert.Error(t, err)
}

func TestService_SyncLdap_EmptyUsername(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockSyncer := ldapsmock.NewMockLDAPSyncer(ctrl)

	svc := Service{
		Clock:      fakeClock{fixedTime: time.Now()},
		UUID:       fakeUUIDGenerator{fixedID: "new-id"},
		UOW:        mockUOW,
		LDAPSyncer: mockSyncer,
	}

	ldap := &domain.Ldap{ID: "ldap1", OrganizationID: "org1", IsEnabled: true, Host: "localhost", Port: 389}
	mappedUsers := []*domain.LDAPMappedUser{
		{Username: "", DN: "cn=nouser"},
	}

	mockUOW.EXPECT().Ldaps().Return(ldapRepo).AnyTimes()
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockSyncer.EXPECT().GetMappedUsersDomain(gomock.Any(), gomock.Any(), gomock.Any()).Return(mappedUsers, nil)
	mockSyncer.EXPECT().GetMappedGroupsDomain(gomock.Any(), gomock.Any()).Return([]*domain.LDAPMappedGroup{}, nil)
	// Sync resolves each entry against the LDAP identity index, which lists
	// the organization's users (empty here).
	userRepo := usersmock.NewMockRepository(ctrl)
	mockUOW.EXPECT().Users().Return(userRepo).AnyTimes()
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	ldapRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	result, err := svc.SyncLdap(context.Background(), "ldap1", nil, "org1")
	require.NoError(t, err)
	assert.Equal(t, 1, result.FailedUsers)
	assert.Equal(t, 0, result.AddedUsers)
}

func TestLDAPPreviewUserResult_Fields(t *testing.T) {
	r := LDAPPreviewUserResult{
		Users:      []*domain.LDAPPreviewUser{{UUID: "u1"}},
		ExistUUIDs: []string{"u1"},
		TotalCount: 1,
		ExistCount: 1,
	}
	assert.Equal(t, 1, r.TotalCount)
}

// pagedUsers returns a repository expectation that serves the user slice page by
// page, mirroring the postgres repository's clamp-and-slice behaviour.
func pagedUsers(
	t *testing.T,
	repo *usersmock.MockRepository,
	users []*userdomain.User,
) {
	t.Helper()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
			assert.Equal(t, platformpagination.MaxPageSize, query.PageSize)
			offset := platformpagination.Offset(query.Page, query.PageSize)
			if offset >= len(users) {
				return nil, int64(len(users)), nil
			}
			end := offset + query.PageSize
			if end > len(users) {
				end = len(users)
			}
			return users[offset:end], int64(len(users)), nil
		}).Times(3)
}

func ldapUsers(count int) []*userdomain.User {
	users := make([]*userdomain.User, count)
	for i := range users {
		users[i] = &userdomain.User{
			ID: fmt.Sprintf("user-%03d", i),
			Identities: []userdomain.UserIdentity{{
				IdentityType: identityTypeLDAP,
				Subject:      fmt.Sprintf("uuid-%03d", i),
			}},
		}
	}
	return users
}

// Preview must see the LDAP identity on the third page: the previous
// PageSize:1000 read was clamped to MaxPageSize, so uuid-200 looked new.
func TestService_FindExistingUUIDs_ScansBeyondTheFirstPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := usersmock.NewMockRepository(ctrl)
	uow.EXPECT().Users().Return(repo).AnyTimes()

	users := ldapUsers(2*platformpagination.MaxPageSize + 1)
	pagedUsers(t, repo, users)

	got := (Service{UOW: uow}).findExistingUUIDs(
		context.Background(),
		"org1",
		[]*domain.LDAPPreviewUser{{UUID: "uuid-200"}},
	)

	assert.Equal(t, []string{"uuid-200"}, got)
}

// Sync must match the LDAP identity on the third page instead of creating a
// duplicate because the first two pages did not contain it.
func TestService_FindUserByLDAPIdentity_ScansBeyondTheFirstPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := usersmock.NewMockRepository(ctrl)
	uow.EXPECT().Users().Return(repo).AnyTimes()

	users := ldapUsers(2*platformpagination.MaxPageSize + 1)
	pagedUsers(t, repo, users)
	repo.EXPECT().GetByID(gomock.Any(), "user-200").Return(users[200], nil)

	got := (Service{UOW: uow}).findUserByLDAPIdentity(context.Background(), "org1", "uuid-200")

	require.NotNil(t, got)
	assert.Equal(t, "user-200", got.ID)
}

func TestLDAPSyncResult_Fields(t *testing.T) {
	r := LDAPSyncResult{
		TotalUsers:   10,
		AddedUsers:   5,
		UpdatedUsers: 3,
		FailedUsers:  2,
		TotalGroups:  4,
		AddedGroups:  3,
		FailedGroups: 1,
		Errors:       []string{"err1"},
	}
	assert.Equal(t, 10, r.TotalUsers)
	assert.Equal(t, 1, r.FailedGroups)
}
