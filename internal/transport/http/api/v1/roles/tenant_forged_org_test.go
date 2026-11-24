package roles

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
	approles "github.com/guliangiam/guliangiam/internal/app/roles"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/roles/mock"
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

func (forgedOrgUUID) NewString() (string, error) { return "generated-role-id", nil }

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

func forgedOrgRouter(ctrl *gomock.Controller) (chi.Router, *appuowmock.MockUnitOfWork, *domainmock.MockRepository) {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Roles().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: approles.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router, uow, repo
}

// TestListRoles_ForgedQueryOrganizationIDIgnored pins that the query value is
// only a target request; the authenticated tenant scope wins.
func TestListRoles_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Role, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/roles?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateRole_ForgedBodyOrganizationIDIgnored pins that a regular caller
// cannot move a role write into another tenant through the JSON body.
func TestCreateRole_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, role *domain.Role) error {
			assert.Equal(t, forgedOrgSessionID, role.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/roles",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"admin","displayName":"Administrator"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
}

// TestBulkImportRoles_ForgedRowOrganizationIDIgnored pins that both the outer
// and per-row organizationId are ignored by the import route.
func TestBulkImportRoles_ForgedRowOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, role *domain.Role) error {
			assert.Equal(t, forgedOrgSessionID, role.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/roles/bulk-import",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","roles":[{"name":"admin","displayName":"Administrator","organizationId":"`+forgedOrgRequestID+`"}]}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestUploadRoles_ForgedQueryOrganizationIDIgnored pins that the Excel import
// route ignores the query value and writes into the trusted tenant.
func TestUploadRoles_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	book := excelize.NewFile()
	require.NoError(t, book.SetCellValue("Sheet1", "A2", "admin"))
	require.NoError(t, book.SetCellValue("Sheet1", "B2", "Administrator"))
	buf, err := book.WriteToBuffer()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "roles.xlsx")
	require.NoError(t, err)
	_, err = part.Write(buf.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	ctrl := gomock.NewController(t)
	router, _, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, role *domain.Role) error {
			assert.Equal(t, forgedOrgSessionID, role.OrganizationID)
			return nil
		},
	)

	req := forgedOrgRequest(http.MethodPost,
		"/api/v1/roles/upload?organizationId="+forgedOrgRequestID, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestRolesWriteWithoutTenantScopeRejected pins the fail-fast guard: without a
// trusted tenant the route must not fall through to the repository's
// empty-organization "all tenants" branch.
func TestRolesWriteWithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: approles.Service{}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/roles",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"admin","displayName":"Administrator"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
