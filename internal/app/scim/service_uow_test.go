package scim

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domaingroups "github.com/guliangiam/guliangiam/internal/domain/groups"
	groupmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const scimTestUserID = "01900000-0000-7000-8000-000000000043"

var scimTestNow = time.Date(2026, time.November, 3, 9, 0, 0, 0, time.UTC)

type scimFixedClock struct{ now time.Time }

func (c scimFixedClock) Now() time.Time { return c.now }

type scimFixedUUID struct {
	id  string
	err error
}

func (g scimFixedUUID) NewString() (string, error) { return g.id, g.err }

func setupScimServiceTest(t *testing.T) (*gomock.Controller, *mock.MockUnitOfWork, Service) {
	ctrl := gomock.NewController(t)
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewService(scimFixedClock{now: scimTestNow}, scimFixedUUID{id: scimTestUserID}, uowMock)
	return ctrl, uowMock, svc
}

// expectScimTransaction makes the mock run the callback against itself so the
// transaction-scoped repository calls stay observable.
func expectScimTransaction(uowMock *mock.MockUnitOfWork) {
	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		})
}

func TestNewScimService(t *testing.T) {
	ctrl, _, _ := setupScimServiceTest(t)
	ctrl.Finish()
}

func TestService_ListUsers_DefaultPagination(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainusers.User{}, int64(0), nil)

	resp, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{StartIndex: 0, Count: 0})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.TotalResults)
}

func TestService_ListUsers_WithFilter(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainusers.User{}, int64(0), nil)

	resp, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{
		StartIndex: 1, Count: 10, Filter: "userName eq \"alice\"",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_ListUsers_InvalidFilter(t *testing.T) {
	ctrl, _, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	_, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{Filter: "((((("})
	assert.Error(t, err)
}

func TestService_GetUser(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{ID: "user-1", OrganizationID: "org-1"}, nil)

	user, err := svc.GetUser(context.Background(), "org-1", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, user)
}

func TestService_GetUser_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{ID: "user-1", OrganizationID: "org-2"}, nil)

	_, err := svc.GetUser(context.Background(), "org-1", "user-1")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

// TestService_GetUser_ForeignTenantNotFoundFromRepository pins the normal
// cross-tenant outcome: the repository tenant predicate reports a foreign row
// as not found, which stays the same SCIM 404 as a missing user.
func TestService_GetUser_ForeignTenantNotFoundFromRepository(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").
		Return(nil, apperrors.NewNotFound("user not found"))

	_, err := svc.GetUser(context.Background(), "org-1", "user-1")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_DeleteUser(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	expectScimTransaction(uowMock)
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{ID: "user-1", OrganizationID: "org-1", Revision: 1}, nil)
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").Return(nil)
	sessionsRepo := sessionsmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Sessions().Return(sessionsRepo)
	sessionsRepo.EXPECT().RevokeByUserExcept(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	tokensRepo := tokensmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Tokens().Return(tokensRepo)
	tokensRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)

	err := svc.DeleteUser(context.Background(), "org-1", "user-1")
	require.NoError(t, err)
}

func TestService_CreateUser_MissingUserName(t *testing.T) {
	ctrl, _, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	_, err := svc.CreateUser(context.Background(), "org-1", ScimUser{})
	assert.Error(t, err)
}

func TestService_ListGroups_DefaultPagination(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domaingroups.Group{}, int64(0), nil)

	resp, err := svc.ListGroups(context.Background(), "org-1", ScimListQuery{StartIndex: 0, Count: 0})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.TotalResults)
}

func TestService_GetGroup(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{ID: "group-1", OrganizationID: "org-1"}, nil)

	group, err := svc.GetGroup(context.Background(), "org-1", "group-1")
	require.NoError(t, err)
	assert.NotNil(t, group)
}

func TestService_GetGroup_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{ID: "group-1", OrganizationID: "org-2"}, nil)

	_, err := svc.GetGroup(context.Background(), "org-1", "group-1")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

// TestService_GetGroup_ForeignTenantNotFoundFromRepository pins the normal
// cross-tenant outcome at the repository boundary for groups as well.
func TestService_GetGroup_ForeignTenantNotFoundFromRepository(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").
		Return(nil, apperrors.NewNotFound("group not found"))

	_, err := svc.GetGroup(context.Background(), "org-1", "group-1")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_DeleteGroup(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo).Times(2)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{ID: "group-1", OrganizationID: "org-1"}, nil)
	groupRepo.EXPECT().Delete(gomock.Any(), "group-1", "org-1").Return(nil)

	err := svc.DeleteGroup(context.Background(), "org-1", "group-1")
	require.NoError(t, err)
}

func requireAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}

// captureUserWrite returns a gomock action that records the aggregate handed to
// the repository, so the tests assert the wire-level write, not only the mock call.
func captureUserWrite(target **domainusers.User) func(context.Context, *domainusers.User, string) error {
	return func(_ context.Context, user *domainusers.User, _ string) error {
		*target = user
		return nil
	}
}

func scimWriteFixture() *domainusers.User {
	return &domainusers.User{
		ID:                 "user-1",
		OrganizationID:     "org-1",
		Username:           "alice",
		Revision:           5,
		Balance:            42.5,
		BalanceCredit:      7.25,
		BalanceCurrency:    "CNY",
		PreferredMfaType:   "totp",
		MfaPushEnabled:     true,
		RecoveryCodeHashes: []string{"recovery-1"},
		Custom1:            "keep-1",
		Custom7:            "keep-7",
		Addresses:          []domainusers.Address{{Region: "CN", Street: "main"}},
		ApplicationScopes: []domainusers.ConsentRecord{
			{Application: "app-1", Scopes: []string{"openid"}},
		},
	}
}

// TestService_ReplaceUser_BumpsRevisionOnFullAggregate pins the write contract:
// the loaded Revision is incremented before the optimistic-locked UPDATE, and
// the whole aggregate (balance/MFA/collections/custom) is handed back verbatim.
func TestService_ReplaceUser_BumpsRevisionOnFullAggregate(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	loaded := scimWriteFixture()
	expectScimTransaction(uowMock)
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(loaded, nil)

	var written *domainusers.User
	var writtenProfile *domainusers.UserProfile
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").DoAndReturn(captureUserWrite(&written))
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, profile *domainusers.UserProfile) error {
		writtenProfile = profile
		return nil
	})

	resp, err := svc.ReplaceUser(context.Background(), "org-1", "user-1", ScimUser{
		UserName:    "alice-updated",
		DisplayName: "Alice Updated",
		Active:      true,
	})
	require.NoError(t, err)
	require.NotNil(t, written)
	require.NotNil(t, writtenProfile)
	assert.Equal(t, int64(6), written.Revision)
	assert.False(t, written.UpdatedAt.IsZero())
	assert.Equal(t, "alice-updated", written.Username)
	assert.Equal(t, 42.5, written.Balance)
	assert.Equal(t, "keep-1", written.Custom1)
	assert.Equal(t, "totp", written.PreferredMfaType)
	require.Len(t, written.Addresses, 1)
	assert.Equal(t, "user-1", writtenProfile.UserID)
	assert.Equal(t, "Alice Updated", writtenProfile.DisplayName)
	assert.NotNil(t, resp)
}

func TestService_PatchUser_BumpsRevisionOnFullAggregate(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	loaded := scimWriteFixture()
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(loaded, nil)

	var written *domainusers.User
	var writtenProfile *domainusers.UserProfile
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").DoAndReturn(captureUserWrite(&written))
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, profile *domainusers.UserProfile) error {
		writtenProfile = profile
		return nil
	})

	_, err := svc.PatchUser(context.Background(), "org-1", "user-1", ScimPatchOp{
		Operations: []ScimOperation{{Op: "replace", Path: "active", Value: false}},
	})
	require.NoError(t, err)
	require.NotNil(t, written)
	require.NotNil(t, writtenProfile)
	assert.Equal(t, int64(6), written.Revision)
	assert.True(t, written.IsForbidden)
	assert.Equal(t, 42.5, written.Balance)
	assert.Equal(t, "keep-7", written.Custom7)
	require.Len(t, written.ApplicationScopes, 1)
	assert.Equal(t, "user-1", writtenProfile.UserID)
}

// TestService_DeleteUser_DisablesAggregateWithoutZeroingSubtables is the
// regression for the previous partial User{ID, IsForbidden, Revision}: the
// disable write must carry the loaded child values so upsertUserSubtables
// cannot overwrite balance/MFA/collections/custom attributes with zero values.
func TestService_DeleteUser_DisablesAggregateWithoutZeroingSubtables(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	loaded := scimWriteFixture()
	expectScimTransaction(uowMock)
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(loaded, nil)

	var written *domainusers.User
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").DoAndReturn(captureUserWrite(&written))
	sessionsRepo := sessionsmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Sessions().Return(sessionsRepo)
	sessionsRepo.EXPECT().RevokeByUserExcept(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	tokensRepo := tokensmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Tokens().Return(tokensRepo)
	tokensRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)

	require.NoError(t, svc.DeleteUser(context.Background(), "org-1", "user-1"))
	require.NotNil(t, written)
	assert.True(t, written.IsForbidden)
	assert.Equal(t, int64(6), written.Revision)
	assert.Equal(t, 42.5, written.Balance)
	assert.Equal(t, 7.25, written.BalanceCredit)
	require.Equal(t, []string{"recovery-1"}, written.RecoveryCodeHashes)
	assert.True(t, written.MfaPushEnabled)
	assert.Equal(t, "keep-1", written.Custom1)
	require.Len(t, written.Addresses, 1)
	require.Len(t, written.ApplicationScopes, 1)
}

// TestService_ReplaceUser_StaleRevisionIsConflict pins that the repository's
// optimistic-lock conflict survives the app layer as a SCIM 409 instead of
// being re-wrapped into a 500.
func TestService_ReplaceUser_StaleRevisionIsConflict(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(scimWriteFixture(), nil)
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").
		Return(apperrors.NewConflict("user revision conflict"))
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.ReplaceUser(context.Background(), "org-1", "user-1", ScimUser{UserName: "alice", Active: true})
	requireAppErrorCode(t, err, apperrors.CodeConflict)
}

// TestService_ReplaceUser_CrossTenantIsNotFoundAndWritesNothing pins the
// zero-write path: the scoped load returns NotFound before any mutation call.
func TestService_ReplaceUser_CrossTenantIsNotFoundAndWritesNothing(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").
		Return(nil, apperrors.NewNotFound("user not found"))
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.ReplaceUser(context.Background(), "org-1", "user-1", ScimUser{UserName: "alice"})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_DeleteUser_CrossTenantIsNotFoundAndWritesNothing(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	expectScimTransaction(uowMock)
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").
		Return(nil, apperrors.NewNotFound("user not found"))
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	requireAppErrorCode(t, svc.DeleteUser(context.Background(), "org-1", "user-1"), apperrors.CodeNotFound)
}

// TestService_UserWrites_EmptyOrganizationIsInvalidArgument keeps the 400
// contract at the use-case boundary, not only in the HTTP handler.
func TestService_UserWrites_EmptyOrganizationIsInvalidArgument(t *testing.T) {
	ctrl, _, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	_, err := svc.ReplaceUser(context.Background(), "  ", "user-1", ScimUser{UserName: "alice"})
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	_, err = svc.PatchUser(context.Background(), "", "user-1", ScimPatchOp{})
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	requireAppErrorCode(t, svc.DeleteUser(context.Background(), "", "user-1"), apperrors.CodeInvalidArgument)
}
