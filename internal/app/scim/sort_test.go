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

func TestResolveScimSort_UserPaths(t *testing.T) {
	cases := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantField string
		wantOrder string
		wantErr   bool
	}{
		{"default id ascending", "", "", "id", "asc", false},
		{"default order with explicit field", "userName", "", "username", "asc", false},
		{"case-insensitive attribute", "USERNAME", "DESCENDING", "username", "desc", false},
		{"meta.created alias", "meta.created", "ascending", "createdAt", "asc", false},
		{"meta.lastModified alias", "Meta.LastModified", "descending", "updatedAt", "desc", false},
		{"whitelist field", "lastSigninAt", "descending", "lastSigninAt", "desc", false},
		{"id explicit", "id", "descending", "id", "desc", false},
		{"unknown path", "password", "", "", "", true},
		{"group-only path on users", "displayName", "", "", "", true},
		{"bad order", "userName", "sideways", "", "", true},
		{"bad order without field", "", "desc", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field, order, err := resolveScimSort(tc.sortBy, tc.sortOrder, scimUserSortPaths)
			if tc.wantErr {
				require.Error(t, err)
				var appErr *apperrors.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantField, field)
			assert.Equal(t, tc.wantOrder, order)
		})
	}
}

func TestResolveScimSort_GroupPaths(t *testing.T) {
	field, order, err := resolveScimSort("displayName", "descending", scimGroupSortPaths)
	require.NoError(t, err)
	assert.Equal(t, "displayName", field)
	assert.Equal(t, "desc", order)

	_, _, err = resolveScimSort("userName", "", scimGroupSortPaths)
	require.Error(t, err, "userName 不在 Group 白名单")
}

func TestNormalizeScimCount(t *testing.T) {
	count, err := normalizeScimCount(100)
	require.NoError(t, err)
	assert.Equal(t, 100, count)

	count, err = normalizeScimCount(0)
	require.NoError(t, err)
	assert.Equal(t, 100, count, "count<1 保持归一为上限")

	_, err = normalizeScimCount(101)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
	assert.Equal(t, "tooMany", appErr.Meta["scimType"])
}

// count 超上限必须在触达 UOW 之前拒绝（RFC 7644 §3.4.2.4 tooMany）。
func TestService_ListUsers_CountTooMany(t *testing.T) {
	ctrl, _, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	_, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{Count: 101})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "tooMany", appErr.Meta["scimType"])
}

func TestService_ListGroups_CountTooMany(t *testing.T) {
	ctrl, _, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	_, err := svc.ListGroups(context.Background(), "org-1", ScimListQuery{Count: 101})
	require.Error(t, err)
}

func TestService_ListUsers_SortPassthrough(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			assert.Equal(t, "updatedAt", q.SortField)
			assert.Equal(t, "desc", q.SortOrder)
			return []*domainusers.User{}, int64(0), nil
		})

	_, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{
		SortBy: "meta.lastModified", SortOrder: "descending",
	})
	require.NoError(t, err)
}

func TestService_ListUsers_DefaultSortIsID(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			assert.Equal(t, "id", q.SortField)
			assert.Equal(t, "asc", q.SortOrder)
			return []*domainusers.User{}, int64(0), nil
		})

	_, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{})
	require.NoError(t, err)
}

func TestService_ListGroups_SortPassthrough(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domaingroups.ListQuery) ([]*domaingroups.Group, int64, error) {
			assert.Equal(t, "displayName", q.SortField)
			assert.Equal(t, "asc", q.SortOrder)
			return []*domaingroups.Group{}, int64(0), nil
		})

	_, err := svc.ListGroups(context.Background(), "org-1", ScimListQuery{SortBy: "displayName"})
	require.NoError(t, err)
}

func TestService_ListUsers_InvalidSortRejectedBeforeUOW(t *testing.T) {
	ctrl, _, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	_, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{SortBy: "password"})
	require.Error(t, err)

	_, err = svc.ListUsers(context.Background(), "org-1", ScimListQuery{SortOrder: "sideways"})
	require.Error(t, err)
}
