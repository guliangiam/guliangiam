package invitations

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/invitations"
	invitationsmock "github.com/guliangiam/guliangiam/internal/domain/invitations/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewService_invitations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock)
	assert.NotNil(t, svc)
	assert.Equal(t, uowMock, svc.UOW)
}

func TestListInvitations_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := invitationsmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().Invitations().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*invitations.Invitation{}, int64(0), nil)

	items, total, err := svc.ListInvitations(context.Background(), ListInvitationsQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestListInvitations_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	repo := invitationsmock.NewMockRepository(ctrl)
	svc := Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uowMock}

	uowMock.EXPECT().Invitations().Return(repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, _, err := svc.ListInvitations(context.Background(), ListInvitationsQuery{Page: 1, PageSize: 10})
	assert.Error(t, err)
}
