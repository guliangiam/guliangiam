package permissions

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	enforcermock "github.com/guliangiam/guliangiam/internal/domain/enforcers/mock"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	modelmock "github.com/guliangiam/guliangiam/internal/domain/models/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/permissions/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-permission-id", nil }

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

func forgedOrgRouter(ctrl *gomock.Controller) (chi.Router, *domainmock.MockRepository) {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Permissions().Return(repo).AnyTimes()
	enforcerRepo := enforcermock.NewMockRepository(ctrl)
	modelRepo := modelmock.NewMockRepository(ctrl)
	uow.EXPECT().Enforcers().Return(enforcerRepo).AnyTimes()
	uow.EXPECT().Models().Return(modelRepo).AnyTimes()
	enforcerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&enforcerdomain.Enforcer{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", OrganizationID: forgedOrgSessionID, ModelID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}, nil).AnyTimes()
	modelRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", gomock.Any()).
		Return(&modeldomain.Model{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", ModelText: "[request_definition] r = sub, obj, act"}, nil).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppermissions.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router, repo
}

// TestListPermissions_ForgedQueryOrganizationIDIgnored pins that the query
// value only requests a tenant; the authenticated scope wins.
func TestListPermissions_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Permission, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/permissions?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreatePermission_ForgedBodyOrganizationIDIgnored pins that a regular
// caller cannot move a permission write into another tenant through the body.
func TestCreatePermission_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, permission *domain.Permission) error {
			assert.Equal(t, forgedOrgSessionID, permission.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/permissions",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"policy","displayName":"Policy","resourceType":"Role"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
}

// TestBulkImportPermissions_ForgedRowOrganizationIDIgnored pins that both the
// outer and per-row organizationId are ignored by the import route.
func TestBulkImportPermissions_ForgedRowOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, permission *domain.Permission) error {
			assert.Equal(t, forgedOrgSessionID, permission.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/permissions/bulk-import",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","permissions":[{"name":"policy","displayName":"Policy","resourceType":"Role","organizationId":"`+forgedOrgRequestID+`"}]}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestUploadPermissions_ForgedQueryOrganizationIDIgnored pins that the Excel
// import route ignores the query value and writes into the trusted tenant.
func TestUploadPermissions_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	book := excelize.NewFile()
	cells := map[string]string{
		"A2": "policy", "B2": "Policy", "C2": "Role", "D2": "Allow", "E2": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "F2": "read",
	}
	for cell, value := range cells {
		require.NoError(t, book.SetCellValue("Sheet1", cell, value))
	}
	buf, err := book.WriteToBuffer()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "permissions.xlsx")
	require.NoError(t, err)
	_, err = part.Write(buf.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, permission *domain.Permission) error {
			assert.Equal(t, forgedOrgSessionID, permission.OrganizationID)
			return nil
		},
	)

	req := forgedOrgRequest(http.MethodPost,
		"/api/v1/permissions/upload?organizationId="+forgedOrgRequestID, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestPermissionsWriteWithoutTenantScopeRejected pins the fail-fast guard.
func TestPermissionsWriteWithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppermissions.Service{}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/permissions",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"policy","displayName":"Policy","resourceType":"Role"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
