package applications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apprepoMock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// token_signing_method is NOT NULL DEFAULT 'RS256': the HTTP surface must never
// hand the repository an empty value, and import must not accept an out-of-enum
// method (GM N8a).

// The smallest accepted create body omits tokenSigningMethod; the persisted
// entity must still carry the RS256 default.
func TestCreateApplication_MinimalBodyDefaultsTokenSigningMethodToRS256(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := apprepoMock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByOrganizationAndSlug(gomock.Any(), tenantRegressionSessionOrg, "n8a-minimal").
		Return(nil, apperrors.NewNotFound("application not found"))
	repo.EXPECT().HasDeletedWithOrgAndSlug(gomock.Any(), tenantRegressionSessionOrg, "n8a-minimal").
		Return(false, nil)

	var persisted *domain.Application
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, application *domain.Application) error {
			persisted = application
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	tenantRegressionRouter(uow).ServeHTTP(recorder, tenantRegressionRequest(
		http.MethodPost, "/api/v1/applications",
		`{"slug":"n8a-minimal","displayName":"N8A Minimal","status":"active",`+
			`"grantTypes":["authorization_code"]}`))

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotNil(t, persisted)
	assert.Equal(t, domain.TokenSigningMethodRS256, persisted.TokenSigningMethod)
}

// An import whose export document omits/blank the method must land as RS256.
func TestImportApplication_BlankTokenSigningMethodPersistsRS256(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := apprepoMock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*domain.Application(nil), int64(0), nil)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)

	var persisted *domain.Application
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, application *domain.Application) error {
			persisted = application
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	tenantRegressionRouter(uow).ServeHTTP(recorder, tenantRegressionRequest(
		http.MethodPost, "/api/v1/applications/import",
		`{"version":"1.1","export":{"slug":"n8a-import","displayName":"N8A Import",`+
			`"status":"active","grantTypes":["authorization_code"],"tokenSigningMethod":"  "}}`))

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotNil(t, persisted)
	assert.Equal(t, domain.TokenSigningMethodRS256, persisted.TokenSigningMethod)
}

// An out-of-enum method is rejected before the repository is touched, with the
// same 422 field metadata as create/patch.
func TestImportApplication_RejectsOutOfEnumTokenSigningMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := apprepoMock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	// No List/Create expectation: the request must fail before persistence.

	recorder := httptest.NewRecorder()
	tenantRegressionRouter(uow).ServeHTTP(recorder, tenantRegressionRequest(
		http.MethodPost, "/api/v1/applications/import",
		`{"version":"1.1","export":{"slug":"n8a-import","displayName":"N8A Import",`+
			`"status":"active","tokenSigningMethod":"HS256"}}`))

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
}
