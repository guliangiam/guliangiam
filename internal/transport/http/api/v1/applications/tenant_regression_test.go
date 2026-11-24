package applications

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apprepoMock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	tenantRegressionSessionOrg = "11111111-1111-1111-1111-111111111111"
	tenantRegressionForgedOrg  = "22222222-2222-2222-2222-222222222222"
	tenantRegressionUserAgent  = "tenant-regression/1.0"
)

type tenantRegressionClock struct{}

func (tenantRegressionClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type tenantRegressionUUID struct{}

func (tenantRegressionUUID) NewString() (string, error) { return "generated-id", nil }

func tenantRegressionRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", tenantRegressionUserAgent)
	return req.WithContext(authtransport.WithSessionContext(req.Context(), authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: tenantRegressionSessionOrg,
	}))
}

func tenantRegressionRouter(uow appuow.AuthUoW) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appapplications.NewService(tenantRegressionClock{}, tenantRegressionUUID{}, uow)})
	return router
}

// TestListApplications_TenantScopeOverridesQueryOrganizationID pins GAP-014 for
// the list route: the query value is a request, not a tenant authority.
func TestListApplications_TenantScopeOverridesQueryOrganizationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := apprepoMock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Application, int64, error) {
			assert.Equal(t, tenantRegressionSessionOrg, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	tenantRegressionRouter(uow).ServeHTTP(recorder, tenantRegressionRequest(
		http.MethodGet, "/api/v1/applications?organizationId="+tenantRegressionForgedOrg, ""))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateApplication_TenantScopeOverridesBodyOrganizationID pins GAP-014 for
// the create route: a regular caller cannot move the write into another tenant.
func TestCreateApplication_TenantScopeOverridesBodyOrganizationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := apprepoMock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByOrganizationAndSlug(gomock.Any(), tenantRegressionSessionOrg, "tenant-regression").
		Return(nil, apperrors.NewNotFound("application not found"))
	repo.EXPECT().HasDeletedWithOrgAndSlug(gomock.Any(), tenantRegressionSessionOrg, "tenant-regression").
		Return(false, nil)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, application *domain.Application) error {
			assert.Equal(t, tenantRegressionSessionOrg, application.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	tenantRegressionRouter(uow).ServeHTTP(recorder, tenantRegressionRequest(
		http.MethodPost, "/api/v1/applications",
		`{"organizationId":"`+tenantRegressionForgedOrg+`","slug":"tenant-regression",`+
			`"displayName":"Tenant Regression","status":"active",`+
			`"clientId":"tenant-regression-client","tokenEndpointAuthMethod":"none",`+
			`"grantTypes":["authorization_code"]}`))
	require.Equal(t, http.StatusCreated, recorder.Code)
}

// TestImportApplication_TenantScopeOverridesBodyOrganizationID pins the same
// rule for the import route, whose payload also carries an organizationId.
func TestImportApplication_TenantScopeOverridesBodyOrganizationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := apprepoMock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Application, int64, error) {
			assert.Equal(t, tenantRegressionSessionOrg, query.OrganizationID)
			return nil, 0, nil
		},
	)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, application *domain.Application) error {
			assert.Equal(t, tenantRegressionSessionOrg, application.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	tenantRegressionRouter(uow).ServeHTTP(recorder, tenantRegressionRequest(
		http.MethodPost, "/api/v1/applications/import",
		`{"organizationId":"`+tenantRegressionForgedOrg+`","version":"1.1",`+
			`"export":{"slug":"tenant-regression-import","displayName":"Imported",`+
			`"grantTypes":["authorization_code"]}}`))
	require.Equal(t, http.StatusCreated, recorder.Code)
}
