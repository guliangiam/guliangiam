package scim

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appscim "github.com/guliangiam/guliangiam/internal/app/scim"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestResolveOrgID_FromTrustedScope(t *testing.T) {
	r := httptest.NewRequest("GET", "/scim/v2/Users?organizationId=forged-query", nil)
	r.Header.Set("X-Organization-Id", "forged-header")
	r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-trusted", "", tenantctx.TenantFilterStrict))
	assert.Equal(t, "org-trusted", resolveOrgID(r))
}

func TestResolveOrgID_ForgedQueryIgnored(t *testing.T) {
	r := httptest.NewRequest("GET", "/scim/v2/Users?organizationId=org-1", nil)
	assert.Equal(t, "", resolveOrgID(r))
}

func TestResolveOrgID_ForgedHeaderIgnored(t *testing.T) {
	r := httptest.NewRequest("GET", "/scim/v2/Users", nil)
	r.Header.Set("X-Organization-Id", "org-2")
	assert.Equal(t, "", resolveOrgID(r))
}

func TestResolveOrgID_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/scim/v2/Users", nil)
	assert.Equal(t, "", resolveOrgID(r))
}

// TestListUsers_TrustedScopeOverridesForgedQuery pins the GAP-011/014 rule at the
// handler boundary: the organizationId query value is request input, so the app
// service must receive the scope injected by BearerTokenAuth instead.
func TestListUsers_TrustedScopeOverridesForgedQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)
	uow.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			assert.Equal(t, "org-trusted", query.OrganizationID)
			return nil, 0, nil
		})

	h := &Handler{ScimService: appscim.Service{UOW: uow}}
	r := httptest.NewRequest("GET", "/scim/v2/Users?organizationId=org-forged", nil)
	r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-trusted", "", tenantctx.TenantFilterStrict))
	w := httptest.NewRecorder()
	h.ListUsers(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsBadRequest(t *testing.T) {
	err := apperrors.NewInvalidArgument("bad request")
	assert.True(t, isBadRequest(err))
}

func TestIsBadRequest_NotBadRequest(t *testing.T) {
	err := apperrors.NewNotFound("not found")
	assert.False(t, isBadRequest(err))
}

func TestIsBadRequest_NonAppError(t *testing.T) {
	err := errors.New("generic error")
	assert.False(t, isBadRequest(err))
}

func TestIsConflict(t *testing.T) {
	err := apperrors.NewConflict("conflict")
	assert.True(t, isConflict(err))
}

func TestIsConflict_AlreadyExists(t *testing.T) {
	err := apperrors.NewAlreadyExists("already exists")
	assert.True(t, isConflict(err))
}

func TestIsConflict_NotConflict(t *testing.T) {
	err := apperrors.NewInvalidArgument("bad request")
	assert.False(t, isConflict(err))
}

func TestIsNotFound(t *testing.T) {
	err := apperrors.NewNotFound("not found")
	assert.True(t, isNotFound(err))
}

func TestIsNotFound_NotNotFoundError(t *testing.T) {
	err := apperrors.NewInvalidArgument("bad request")
	assert.False(t, isNotFound(err))
}

func TestScimStatusCodeFromError_Nil(t *testing.T) {
	assert.Equal(t, http.StatusOK, scimStatusCodeFromError(nil))
}

func TestScimStatusCodeFromError_InvalidArgument(t *testing.T) {
	err := apperrors.NewInvalidArgument("bad request")
	assert.Equal(t, http.StatusBadRequest, scimStatusCodeFromError(err))
}

func TestScimStatusCodeFromError_NotFound(t *testing.T) {
	err := apperrors.NewNotFound("not found")
	assert.Equal(t, http.StatusNotFound, scimStatusCodeFromError(err))
}

func TestScimStatusCodeFromError_GenericError(t *testing.T) {
	err := errors.New("generic error")
	assert.Equal(t, http.StatusInternalServerError, scimStatusCodeFromError(err))
}

func TestWriteSCIMJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	writeSCIMJSON(w, http.StatusOK, data)

	assert.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]string
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestWriteSCIMError(t *testing.T) {
	w := httptest.NewRecorder()
	writeSCIMError(w, http.StatusBadRequest, "test error")

	assert.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var scimErr ScimError
	err := json.NewDecoder(w.Body).Decode(&scimErr)
	require.NoError(t, err)
	assert.Equal(t, "test error", scimErr.Detail)
	assert.Equal(t, "400", scimErr.Status)
	assert.Contains(t, scimErr.Schemas[0], "Error")
}

func TestListUsers_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("GET", "/scim/v2/Users", nil)
	w := httptest.NewRecorder()
	h.ListUsers(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetUser_MissingID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("GET", "/scim/v2/Users/", nil)
	w := httptest.NewRecorder()

	// Need chi context for URL params
	rctx := chiRouteCtx(r, "id", "")
	h.GetUser(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetUser_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("GET", "/scim/v2/Users/user-1", nil)
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "user-1")
	h.GetUser(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateUser_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("POST", "/scim/v2/Users", strings.NewReader(`{"userName":"alice"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateUser(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateUser_InvalidJSON(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("POST", "/scim/v2/Users", strings.NewReader("invalid json"))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-1", "", tenantctx.TenantFilterStrict))
	w := httptest.NewRecorder()
	h.CreateUser(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchUser_MissingID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("PATCH", "/scim/v2/Users/", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "")
	h.PatchUser(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchUser_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("PATCH", "/scim/v2/Users/user-1", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "user-1")
	h.PatchUser(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteUser_MissingID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("DELETE", "/scim/v2/Users/", nil)
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "")
	h.DeleteUser(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteUser_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("DELETE", "/scim/v2/Users/user-1", nil)
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "user-1")
	h.DeleteUser(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListGroups_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("GET", "/scim/v2/Groups", nil)
	w := httptest.NewRecorder()
	h.ListGroups(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetGroup_MissingID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("GET", "/scim/v2/Groups/", nil)
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "")
	h.GetGroup(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetGroup_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("GET", "/scim/v2/Groups/group-1", nil)
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "group-1")
	h.GetGroup(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateGroup_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("POST", "/scim/v2/Groups", strings.NewReader(`{"displayName":"Admins"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGroup(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateGroup_InvalidJSON(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("POST", "/scim/v2/Groups", strings.NewReader("invalid json"))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-1", "", tenantctx.TenantFilterStrict))
	w := httptest.NewRecorder()
	h.CreateGroup(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchGroup_MissingID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("PATCH", "/scim/v2/Groups/", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "")
	h.PatchGroup(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchGroup_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("PATCH", "/scim/v2/Groups/group-1", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "group-1")
	h.PatchGroup(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteGroup_MissingID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("DELETE", "/scim/v2/Groups/", nil)
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "")
	h.DeleteGroup(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteGroup_MissingOrgID(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("DELETE", "/scim/v2/Groups/group-1", nil)
	w := httptest.NewRecorder()

	rctx := chiRouteCtx(r, "id", "group-1")
	h.DeleteGroup(w, rctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewHandler(t *testing.T) {
	h := &Handler{}
	assert.NotNil(t, h)
}

// chiRouteCtx creates a request with chi URL params set
func chiRouteCtx(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestListUsers_SortParamsReachService(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)
	uow.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			assert.Equal(t, "updatedAt", query.SortField)
			assert.Equal(t, "desc", query.SortOrder)
			return nil, 0, nil
		})

	h := &Handler{ScimService: appscim.Service{UOW: uow}}
	r := httptest.NewRequest("GET", "/scim/v2/Users?sortBy=meta.lastModified&sortOrder=descending", nil)
	r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-trusted", "", tenantctx.TenantFilterStrict))
	w := httptest.NewRecorder()
	h.ListUsers(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListUsers_CountTooManyEnvelope(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	h := &Handler{ScimService: appscim.Service{UOW: uow}}
	r := httptest.NewRequest("GET", "/scim/v2/Users?count=101", nil)
	r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-trusted", "", tenantctx.TenantFilterStrict))
	w := httptest.NewRecorder()
	h.ListUsers(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var scimErr ScimError
	require.NoError(t, json.NewDecoder(w.Body).Decode(&scimErr))
	assert.Equal(t, "tooMany", scimErr.ScimType)
}

func TestListUsers_UnknownSortByIsInvalidValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	h := &Handler{ScimService: appscim.Service{UOW: uow}}
	r := httptest.NewRequest("GET", "/scim/v2/Users?sortBy=password", nil)
	r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-trusted", "", tenantctx.TenantFilterStrict))
	w := httptest.NewRecorder()
	h.ListUsers(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var scimErr ScimError
	require.NoError(t, json.NewDecoder(w.Body).Decode(&scimErr))
	assert.Equal(t, "invalidValue", scimErr.ScimType)
}

func TestWriteSCIMListError_MetaPassthroughAndFallback(t *testing.T) {
	w := httptest.NewRecorder()
	writeSCIMListError(w, apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "count exceeds the maximum of 100", map[string]any{"scimType": "tooMany"}))
	var scimErr ScimError
	require.NoError(t, json.NewDecoder(w.Body).Decode(&scimErr))
	assert.Equal(t, "tooMany", scimErr.ScimType)

	w = httptest.NewRecorder()
	writeSCIMListError(w, apperrors.NewConflict("conflict"))
	require.NoError(t, json.NewDecoder(w.Body).Decode(&scimErr))
	assert.Equal(t, "uniqueness", scimErr.ScimType, "无 Meta 时回落状态码映射")
}
