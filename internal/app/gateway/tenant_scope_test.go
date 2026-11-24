package gateway

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	gwmock "github.com/guliangiam/guliangiam/internal/domain/gateway/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRequireOrganizationID(t *testing.T) {
	for _, organizationID := range []string{"", "  ", "\t"} {
		err := requireOrganizationID(organizationID)
		require.Error(t, err)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
	}
	require.NoError(t, requireOrganizationID("org-1"))
}

func TestProvisionCert_ScopesLookupToOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).Times(2)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").
		Return(&domain.Site{ID: "site-1", OrganizationID: "org-1", Host: "portal.example.com"}, nil)
	repo.EXPECT().UpdateSite(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	result, err := svc.ProvisionCert(context.Background(), "org-1", "site-1")
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestRenewCert_ScopesLookupToOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).Times(2)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").
		Return(&domain.Site{ID: "site-1", OrganizationID: "org-1", CertID: "cert-1"}, nil)
	repo.EXPECT().UpdateSite(gomock.Any(), gomock.Any()).Return(nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	result, err := svc.RenewCert(context.Background(), "org-1", "site-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "cert-1", result.CertID)
}

func TestCheckHealth_ScopesLookupBeforeProbe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").
		Return(&domain.Site{ID: "site-1", OrganizationID: "org-1"}, nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	result, err := svc.CheckHealth(context.Background(), "org-1", "site-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, healthStatusUnknown, result.Status, "empty host must short-circuit before any outbound probe")
}

func TestSiteActions_EmptyOrganizationRejectedBeforeLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}

	_, err := svc.ProvisionCert(context.Background(), "", "site-1")
	require.Error(t, err)
	_, err = svc.RenewCert(context.Background(), "", "site-1")
	require.Error(t, err)
	_, err = svc.CheckHealth(context.Background(), "", "site-1")
	require.Error(t, err)
}

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestByIDActions_RequireOrganization pins the non-HTTP fail-fast: the app
// layer rejects before touching the repository when no trusted tenant reaches
// it.
func TestByIDActions_RequireOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetSite(ctx, " ", "site-1")
	assertInvalidArgument(t, err)
	_, err = svc.GetServer(ctx, "", "server-1")
	assertInvalidArgument(t, err)
	_, err = svc.GetCertStatus(ctx, "", "site-1")
	assertInvalidArgument(t, err)
	_, err = svc.GetHealthHistory(ctx, "", "site-1")
	assertInvalidArgument(t, err)
	require.Error(t, svc.UpdateSite(ctx, "", UpdateSiteCommand{ID: "site-1"}))
	require.Error(t, svc.UpdateServer(ctx, "", UpdateServerCommand{ID: "server-1"}))
	require.Error(t, svc.DeleteSite(ctx, "", "site-1"))
	require.Error(t, svc.DeleteServer(ctx, "", "server-1"))
}

func TestByIDReads_ScopeLookupToOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).Times(4)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").
		Return(&domain.Site{ID: "site-1", OrganizationID: "org-1", Name: "site", Host: "site.example.com"}, nil)
	repo.EXPECT().GetServerByIDForOrganization(gomock.Any(), "server-1", "org-1").
		Return(&domain.Server{ID: "server-1", OrganizationID: "org-1", Name: "server"}, nil)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").
		Return(&domain.Site{ID: "site-1", OrganizationID: "org-1", SslMode: "off"}, nil)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").
		Return(&domain.Site{ID: "site-1", OrganizationID: "org-1", LastHealthStatus: healthStatusHealthy}, nil)

	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}

	site, err := svc.GetSite(context.Background(), "org-1", "site-1")
	require.NoError(t, err)
	assert.Equal(t, "site-1", site.ID)

	server, err := svc.GetServer(context.Background(), "org-1", "server-1")
	require.NoError(t, err)
	assert.Equal(t, "server-1", server.ID)

	certStatus, err := svc.GetCertStatus(context.Background(), "org-1", "site-1")
	require.NoError(t, err)
	assert.Equal(t, "off", certStatus.SslMode)

	health, err := svc.GetHealthHistory(context.Background(), "org-1", "site-1")
	require.NoError(t, err)
	assert.Equal(t, healthStatusHealthy, health.Status)
}

func TestByIDWrites_ForeignTenantStopsBeforeMutation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)

	uow.EXPECT().Gateway().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).Times(4)
	repo.EXPECT().GetSiteByIDForOrganization(gomock.Any(), "site-1", "org-1").
		Return(nil, apperrors.NewNotFound("site not found")).Times(2)
	repo.EXPECT().GetServerByIDForOrganization(gomock.Any(), "server-1", "org-1").
		Return(nil, apperrors.NewNotFound("server not found")).Times(2)
	// No Update/Delete expectations: a foreign id must never reach the write.

	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}

	err := svc.UpdateSite(context.Background(), "org-1", UpdateSiteCommand{ID: "site-1", Name: "hijacked"})
	assertNotFound(t, err)
	err = svc.UpdateServer(context.Background(), "org-1", UpdateServerCommand{ID: "server-1", Name: "hijacked"})
	assertNotFound(t, err)
	err = svc.DeleteSite(context.Background(), "org-1", "site-1")
	assertNotFound(t, err)
	err = svc.DeleteServer(context.Background(), "org-1", "server-1")
	assertNotFound(t, err)
}
