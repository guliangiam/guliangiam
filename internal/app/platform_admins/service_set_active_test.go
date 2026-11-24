package platform_admins

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
)

type stubPlatformAdminRepo struct {
	admin    *platformadmins.PlatformAdmin
	setCalls []bool
}

func (r *stubPlatformAdminRepo) GetByID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return r.admin, nil
}
func (r *stubPlatformAdminRepo) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return r.admin, nil
}
func (r *stubPlatformAdminRepo) List(context.Context, platformadmins.ListOptions) ([]*platformadmins.PlatformAdmin, int64, error) {
	return nil, 0, nil
}
func (r *stubPlatformAdminRepo) Create(context.Context, *platformadmins.PlatformAdmin) error {
	return nil
}
func (r *stubPlatformAdminRepo) Update(context.Context, *platformadmins.PlatformAdmin) error {
	return nil
}
func (r *stubPlatformAdminRepo) SetActive(_ context.Context, _ string, active bool) error {
	r.setCalls = append(r.setCalls, active)
	return nil
}
func (r *stubPlatformAdminRepo) Delete(context.Context, string) error { return nil }

type recordingInvalidator struct{ ids []string }

func (r *recordingInvalidator) Invalidate(userID string) { r.ids = append(r.ids, userID) }

func TestSetActiveDeactivateRevokesSessionsAndInvalidatesCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUow := appuowmock.NewMockUnitOfWork(ctrl)
	adminRepo := &stubPlatformAdminRepo{admin: &platformadmins.PlatformAdmin{ID: "pa-1", UserID: "user-1", IsActive: true}}
	sessionRepo := sessionsmock.NewMockRepository(ctrl)
	invalidator := &recordingInvalidator{}

	mockUow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUow)
		})
	mockUow.EXPECT().PlatformAdmins().Return(adminRepo).AnyTimes()
	mockUow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().RevokeByUserExcept(gomock.Any(), "user-1", "", gomock.Any()).Return(int64(2), nil)

	svc := NewService(clock.SystemClock{}, nil, mockUow, nil)
	svc.PrincipalCache = invalidator
	require.NoError(t, svc.SetActive(context.Background(), SetActiveCommand{ID: "pa-1", Active: false}))
	require.Equal(t, []bool{false}, adminRepo.setCalls)
	require.Equal(t, []string{"user-1"}, invalidator.ids)
	_ = time.Now
}

// BE-A-06：硬删除平台管理员必须同事务吊销其全部会话并失效 principal 缓存，
// 与 SetActive(false) 一致，避免被删管理员的既有会话继续以 platform_admin 通过 Gate。
func TestDeleteRevokesSessionsAndInvalidatesCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUow := appuowmock.NewMockUnitOfWork(ctrl)
	adminRepo := &stubPlatformAdminRepo{admin: &platformadmins.PlatformAdmin{ID: "pa-1", UserID: "user-1", IsActive: true}}
	sessionRepo := sessionsmock.NewMockRepository(ctrl)
	invalidator := &recordingInvalidator{}

	mockUow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUow)
		})
	mockUow.EXPECT().PlatformAdmins().Return(adminRepo).AnyTimes()
	mockUow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().RevokeByUserExcept(gomock.Any(), "user-1", "", gomock.Any()).Return(int64(3), nil)

	svc := NewService(clock.SystemClock{}, nil, mockUow, nil)
	svc.PrincipalCache = invalidator
	require.NoError(t, svc.DeletePlatformAdmin(context.Background(), DeletePlatformAdminCommand{ID: "pa-1"}))
	require.Equal(t, []string{"user-1"}, invalidator.ids)
}
