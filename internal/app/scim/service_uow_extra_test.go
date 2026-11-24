package scim

import (
	"context"
	"testing"

	domaingroups "github.com/guliangiam/guliangiam/internal/domain/groups"
	groupmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_CreateUser_Success(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").Return(nil, apperrors.NewNotFound("user not found"))
	userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.CreateUser(context.Background(), "org-1", ScimUser{
		UserName:     "alice",
		DisplayName:  "Alice",
		Active:       true,
		Name:         &ScimName{GivenName: "Alice", FamilyName: "Smith"},
		Emails:       []ScimEmail{{Value: "alice@example.com"}},
		PhoneNumbers: []ScimPhone{{Value: "+1234567890"}},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "alice", resp.UserName)
}

func TestService_CreateUser_Duplicate(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").Return(&domainusers.User{ID: "existing"}, nil)
	userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.CreateUser(context.Background(), "org-1", ScimUser{UserName: "alice"})
	requireAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestService_CreateUser_CreateError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").Return(nil, apperrors.NewNotFound("user not found"))
	userRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(assert.AnError)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.CreateUser(context.Background(), "org-1", ScimUser{UserName: "alice"})
	assert.Error(t, err)
}

func TestService_ReplaceUser_Success(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-1", Profile: domainusers.UserProfile{},
	}, nil)
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").Return(nil)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, profile *domainusers.UserProfile) error {
		assert.Equal(t, "user-1", profile.UserID)
		assert.Equal(t, "Alice Updated", profile.DisplayName)
		return nil
	})

	resp, err := svc.ReplaceUser(context.Background(), "org-1", "user-1", ScimUser{
		UserName:    "alice",
		DisplayName: "Alice Updated",
		Active:      true,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_ReplaceUser_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-2",
	}, nil)

	_, err := svc.ReplaceUser(context.Background(), "org-1", "user-1", ScimUser{UserName: "alice"})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_ReplaceUser_UpdateError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-1", Profile: domainusers.UserProfile{},
	}, nil)
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").Return(assert.AnError)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.ReplaceUser(context.Background(), "org-1", "user-1", ScimUser{UserName: "alice"})
	assert.Error(t, err)
}

func TestService_PatchUser_Success(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-1", Profile: domainusers.UserProfile{},
	}, nil)
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").Return(nil)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, profile *domainusers.UserProfile) error {
		assert.Equal(t, "user-1", profile.UserID)
		assert.Equal(t, "New Name", profile.DisplayName)
		return nil
	})

	resp, err := svc.PatchUser(context.Background(), "org-1", "user-1", ScimPatchOp{
		Operations: []ScimOperation{
			{Op: "replace", Path: "displayName", Value: "New Name"},
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_PatchUser_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-2",
	}, nil)

	_, err := svc.PatchUser(context.Background(), "org-1", "user-1", ScimPatchOp{})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_PatchUser_UnsupportedOp(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-1", Profile: domainusers.UserProfile{},
	}, nil)

	_, err := svc.PatchUser(context.Background(), "org-1", "user-1", ScimPatchOp{
		Operations: []ScimOperation{{Op: "invalid", Path: "x"}},
	})
	assert.Error(t, err)
}

func TestService_PatchUser_UpdateError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo).Times(2)
	expectScimTransaction(uowMock)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-1", Profile: domainusers.UserProfile{},
	}, nil)
	userRepo.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), "org-1").Return(assert.AnError)
	userRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.PatchUser(context.Background(), "org-1", "user-1", ScimPatchOp{
		Operations: []ScimOperation{{Op: "replace", Path: "displayName", Value: "New"}},
	})
	assert.Error(t, err)
}

func TestService_DeleteUser_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	expectScimTransaction(uowMock)
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(&domainusers.User{
		ID: "user-1", OrganizationID: "org-2",
	}, nil)

	err := svc.DeleteUser(context.Background(), "org-1", "user-1")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_CreateGroup_Success(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.CreateGroup(context.Background(), "org-1", ScimGroup{
		DisplayName: "Engineers",
		Members:     []ScimGroupMember{{Value: "user-1"}, {Value: "user-2"}},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_CreateGroup_MissingDisplayName(t *testing.T) {
	ctrl, _, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	_, err := svc.CreateGroup(context.Background(), "org-1", ScimGroup{})
	assert.Error(t, err)
}

func TestService_CreateGroup_CreateError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(assert.AnError)

	_, err := svc.CreateGroup(context.Background(), "org-1", ScimGroup{DisplayName: "Engineers"})
	assert.Error(t, err)
}

func TestService_ReplaceGroup_Success(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo).Times(2)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{
		ID: "group-1", OrganizationID: "org-1",
	}, nil)
	groupRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.ReplaceGroup(context.Background(), "org-1", "group-1", ScimGroup{
		DisplayName: "Updated Group",
		Members:     []ScimGroupMember{{Value: "user-1"}},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_ReplaceGroup_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{
		ID: "group-1", OrganizationID: "org-2",
	}, nil)

	_, err := svc.ReplaceGroup(context.Background(), "org-1", "group-1", ScimGroup{DisplayName: "X"})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_ReplaceGroup_NoMembers(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo).Times(2)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{
		ID: "group-1", OrganizationID: "org-1", Users: []string{"old-user"},
	}, nil)
	groupRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.ReplaceGroup(context.Background(), "org-1", "group-1", ScimGroup{
		DisplayName: "Updated Group",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_PatchGroup_Success(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo).Times(2)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{
		ID: "group-1", OrganizationID: "org-1",
	}, nil)
	groupRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.PatchGroup(context.Background(), "org-1", "group-1", ScimPatchOp{
		Operations: []ScimOperation{
			{Op: "replace", Path: "displayName", Value: "New Name"},
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_PatchGroup_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{
		ID: "group-1", OrganizationID: "org-2",
	}, nil)

	_, err := svc.PatchGroup(context.Background(), "org-1", "group-1", ScimPatchOp{})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_PatchGroup_UnsupportedOp(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{
		ID: "group-1", OrganizationID: "org-1",
	}, nil)

	_, err := svc.PatchGroup(context.Background(), "org-1", "group-1", ScimPatchOp{
		Operations: []ScimOperation{{Op: "invalid", Path: "x"}},
	})
	assert.Error(t, err)
}

func TestService_DeleteGroup_WrongOrg(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(&domaingroups.Group{
		ID: "group-1", OrganizationID: "org-2",
	}, nil)

	err := svc.DeleteGroup(context.Background(), "org-1", "group-1")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_ListUsers_WithResults(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainusers.User{
		{ID: "u1", Username: "alice", OrganizationID: "org-1", Profile: domainusers.UserProfile{DisplayName: "Alice"}},
		{ID: "u2", Username: "bob", OrganizationID: "org-1", Profile: domainusers.UserProfile{DisplayName: "Bob"}},
	}, int64(2), nil)

	resp, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{StartIndex: 1, Count: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.TotalResults)
	assert.Len(t, resp.Resources, 2)
}

func TestService_ListUsers_ListError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{StartIndex: 1, Count: 10})
	assert.Error(t, err)
}

func TestService_ListGroups_WithResults(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domaingroups.Group{
		{ID: "g1", Name: "admins", OrganizationID: "org-1"},
	}, int64(1), nil)

	resp, err := svc.ListGroups(context.Background(), "org-1", ScimListQuery{StartIndex: 1, Count: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.TotalResults)
}

func TestService_ListGroups_ListError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, err := svc.ListGroups(context.Background(), "org-1", ScimListQuery{StartIndex: 1, Count: 10})
	assert.Error(t, err)
}

func TestService_GetUser_RepoError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(nil, assert.AnError)

	_, err := svc.GetUser(context.Background(), "org-1", "user-1")
	assert.Error(t, err)
}

func TestService_DeleteUser_RepoError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	expectScimTransaction(uowMock)
	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").Return(nil, assert.AnError)

	err := svc.DeleteUser(context.Background(), "org-1", "user-1")
	assert.Error(t, err)
}

func TestService_DeleteGroup_RepoError(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", "org-1").Return(nil, assert.AnError)

	err := svc.DeleteGroup(context.Background(), "org-1", "group-1")
	assert.Error(t, err)
}
