package bootstrap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	appradiusaccountings "github.com/guliangiam/guliangiam/internal/app/radiusaccountings"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	radomain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	ramock "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// BE-B-13 regression: an accounting packet with "orgSlug/username" is resolved
// under System scope and persisted under the resolved tenant's strict scope.
func TestNewRadiusAccountingHandler_PersistsUnderResolvedTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	acctRepo := ramock.NewMockRepository(ctrl)

	resolveTx := uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			assert.Equal(t, tenantctx.TenantFilterSystem, tenantctx.TenantScopeFromContext(ctx).Mode)
			return fn(ctx, uowMock)
		})
	writeTx := uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			scope := tenantctx.TenantScopeFromContext(ctx)
			assert.Equal(t, tenantctx.TenantFilterStrict, scope.Mode)
			assert.Equal(t, "org-1", scope.OrganizationID)
			return fn(ctx, uowMock)
		})
	gomock.InOrder(resolveTx, writeTx)

	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "acme").Return(&orgdomain.Organization{ID: "org-1", Slug: "acme"}, nil)

	uowMock.EXPECT().RadiusAccountings().Return(acctRepo)
	acctRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ra *radomain.RadiusAccounting) error {
			assert.Equal(t, "org-1", ra.OrganizationID)
			assert.Equal(t, "alice", ra.Username)
			assert.Equal(t, "Start", ra.Name)
			assert.Equal(t, "sess-1", ra.AcctSessionID)
			assert.Equal(t, int64(100), ra.AcctInputTotal)
			assert.Equal(t, int64(200), ra.AcctOutputTotal)
			return nil
		})

	acctSvc := appradiusaccountings.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock)
	handler := newRadiusAccountingHandler(uowMock, acctSvc)
	handler(context.Background(), "acme/alice", "sess-1", "Start", 100, 200)
}

// BE-B-13: a packet without an org slug cannot be attributed to a tenant and is
// dropped before any repository access.
func TestNewRadiusAccountingHandler_DropsPacketWithoutOrgSlug(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl) // no expectations: any call fails the test
	acctSvc := appradiusaccountings.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock)
	handler := newRadiusAccountingHandler(uowMock, acctSvc)
	handler(context.Background(), "alice", "sess-1", "Stop", 1, 2)
}

// BE-B-13: an unresolvable slug drops the packet; nothing is written.
func TestNewRadiusAccountingHandler_DropsPacketOnUnknownSlug(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		})
	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "ghost").Return(nil, assert.AnError)

	acctSvc := appradiusaccountings.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock)
	handler := newRadiusAccountingHandler(uowMock, acctSvc)
	handler(context.Background(), "ghost/alice", "sess-1", "Start", 1, 2)
}
