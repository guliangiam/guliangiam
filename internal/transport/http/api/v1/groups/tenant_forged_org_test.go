package groups

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
	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
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

func (forgedOrgUUID) NewString() (string, error) { return "generated-group-id", nil }

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
	uow.EXPECT().Groups().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appgroups.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router, repo
}

// TestListGroups_ForgedQueryOrganizationIDIgnored pins that the query value is
// only a target request; the authenticated tenant scope wins.
func TestListGroups_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Group, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/groups?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateGroup_ForgedBodyOrganizationIDIgnored pins that a regular caller
// cannot move a group write into another tenant through the JSON body.
func TestCreateGroup_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(nil, apperrors.NewNotFound("group not found"))
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, group *domain.Group) error {
			assert.Equal(t, forgedOrgSessionID, group.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/groups",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"developers","displayName":"Developers"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "/api/v1/groups/generated-group-id", recorder.Header().Get("Location"))
	assert.JSONEq(t, `{"id":"generated-group-id","status":"created"}`, recorder.Body.String())
}

// TestBulkImportGroups_ForgedRowOrganizationIDIgnored pins that both the outer
// and per-row organizationId are ignored by the import route.
func TestBulkImportGroups_ForgedRowOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(nil, apperrors.NewNotFound("group not found"))
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, group *domain.Group) error {
			assert.Equal(t, forgedOrgSessionID, group.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/groups/bulk-import",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","groups":[{"name":"developers","displayName":"Developers","organizationId":"`+forgedOrgRequestID+`"}]}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestUploadGroups_ForgedQueryOrganizationIDIgnored pins that the Excel import
// route ignores the query value and writes into the trusted tenant.
func TestUploadGroups_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	book := excelize.NewFile()
	require.NoError(t, book.SetCellValue("Sheet1", "A2", "developers"))
	require.NoError(t, book.SetCellValue("Sheet1", "B2", "Developers"))
	// G7-s 闭集：Excel 导入的 Type 列同样受枚举校验，夹具取合法值。
	require.NoError(t, book.SetCellValue("Sheet1", "C2", "department"))
	buf, err := book.WriteToBuffer()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "groups.xlsx")
	require.NoError(t, err)
	_, err = part.Write(buf.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(nil, apperrors.NewNotFound("group not found"))
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, group *domain.Group) error {
			assert.Equal(t, forgedOrgSessionID, group.OrganizationID)
			return nil
		},
	)

	req := forgedOrgRequest(http.MethodPost,
		"/api/v1/groups/upload?organizationId="+forgedOrgRequestID, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestGroupsWriteWithoutTenantScopeRejected pins the fail-fast guard.
func TestGroupsWriteWithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appgroups.Service{}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"developers","displayName":"Developers"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestGroupDetailRead_IsNoStore pins that GET /groups/{id} never falls through to
// the ETag middleware's `private, no-cache` branch.
func TestGroupDetailRead_IsNoStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "group-1", forgedOrgSessionID).Return(&domain.Group{
		ID:             "group-1",
		OrganizationID: forgedOrgSessionID,
		Name:           "engineering",
		DisplayName:    "Engineering",
		Type:           "normal",
		IsEnabled:      true,
		Revision:       1,
	}, nil)

	handler := httpmw.ETagCacheMiddleware(httpmw.DefaultETagConfig())(router)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/groups/group-1", nil))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Empty(t, recorder.Header().Get("ETag"))
}
