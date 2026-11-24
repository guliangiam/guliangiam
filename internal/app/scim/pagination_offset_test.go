package scim

import (
	"context"
	"testing"

	domaingroups "github.com/guliangiam/guliangiam/internal/domain/groups"
	groupmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// BE-E-07: SCIM startIndex is a 1-based row offset, not a page number.
// startIndex=2&count=100 must skip exactly one row (offset=1), not 100.
func TestService_ListUsers_StartIndexIsRowOffset(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			assert.Equal(t, 1, q.Offset, "startIndex=2 → offset 1")
			assert.Equal(t, 100, q.PageSize)
			return []*domainusers.User{}, int64(0), nil
		})

	resp, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{StartIndex: 2, Count: 100})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.StartIndex)
	assert.Equal(t, 100, resp.ItemsPerPage)
}

func TestService_ListUsers_DefaultStartIndexOffsetZero(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	userRepo := usermock.NewMockRepository(ctrl)
	uowMock.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			assert.Equal(t, 0, q.Offset)
			assert.Equal(t, 1, q.Page)
			return []*domainusers.User{}, int64(0), nil
		})

	_, err := svc.ListUsers(context.Background(), "org-1", ScimListQuery{})
	require.NoError(t, err)
}

func TestService_ListGroups_StartIndexIsRowOffset(t *testing.T) {
	ctrl, uowMock, svc := setupScimServiceTest(t)
	defer ctrl.Finish()

	groupRepo := groupmock.NewMockRepository(ctrl)
	uowMock.EXPECT().Groups().Return(groupRepo)
	groupRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domaingroups.ListQuery) ([]*domaingroups.Group, int64, error) {
			assert.Equal(t, 4, q.Offset, "startIndex=5 → offset 4")
			assert.Equal(t, 10, q.PageSize)
			return []*domaingroups.Group{}, int64(0), nil
		})

	resp, err := svc.ListGroups(context.Background(), "org-1", ScimListQuery{StartIndex: 5, Count: 10})
	require.NoError(t, err)
	assert.Equal(t, 5, resp.StartIndex)
}
