package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	gwmock "github.com/guliangiam/guliangiam/internal/domain/gateway/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// 健康状态落库失败必须上抛：不能返回“检查成功但未持久化”的假结果。
func TestCheckHealth_UpdateSiteFailureReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).Times(2)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").Return(&domain.Site{
		ID: "site-1", OrganizationID: "org-1", Name: "site", Host: "http://127.0.0.1:1",
	}, nil)
	// 127.0.0.1 会被 SSRF DialControl 拦截 → 进入 unhealthy 分支 → 必须落库；
	// 这里让落库失败并断言错误上抛。
	repo.EXPECT().UpdateSite(gomock.Any(), gomock.Any()).Return(errors.New("db down"))

	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: uow}
	result, err := svc.CheckHealth(context.Background(), "org-1", "site-1")

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "update unhealthy site health status")
}
