package platformadmins

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appplatformadmins "github.com/guliangiam/guliangiam/internal/app/platform_admins"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

// newTestRouter 组装带可信会话注入的路由树（模拟 SessionContextMiddleware）。
func newTestRouter(stub *stubService, withSession bool) chi.Router {
	router := chi.NewRouter()
	if withSession {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := authtransport.WithSessionContext(r.Context(), authtransport.SessionContext{
					UserID: testOperatorUserID,
				})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
	}
	RegisterChiRoutes(router, NewHandler(stub))
	return router
}

func doJSON(router chi.Router, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// 高敏审计字段防伪造：body 携带 createdBy/approvedBy/approverId 必须 400
// （DisallowUnknownFields），而不是被静默采纳。
func TestChi_RejectsForgedAuditFields(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"create createdBy", http.MethodPost, "/api/v1/platform-admins", `{"userId":"` + testTargetUserID + `","createdBy":"attacker"}`},
		{"patch createdBy", http.MethodPatch, "/api/v1/platform-admins/" + testAdminID, `{"createdBy":"attacker"}`},
		{"create scope approvedBy", http.MethodPost, "/api/v1/platform-admins/" + testAdminID + "/scopes", `{"scopeType":"all","expiresAt":"2027-01-01T00:00:00Z","approvedBy":"attacker"}`},
		{"approve approverId", http.MethodPost, "/api/v1/platform-admin-scopes/" + testScopeID + ":approve", `{"approverId":"attacker"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := newTestRouter(activeOperatorStub(), true)
			rec := doJSON(router, tc.method, tc.target, tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestChi_CreateSuccess201(t *testing.T) {
	stub := activeOperatorStub()
	stub.admin = &appplatformadmins.PlatformAdminDetail{ID: testAdminID, UserID: testTargetUserID}
	router := newTestRouter(stub, true)
	rec := doJSON(router, http.MethodPost, "/api/v1/platform-admins", `{"userId":"`+testTargetUserID+`"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, testOperatorUserID, stub.gotCreate.CreatedBy)
	assert.Contains(t, rec.Body.String(), `"id":"`+testAdminID+`"`)
}

func TestChi_CreateWithoutSession401(t *testing.T) {
	router := newTestRouter(activeOperatorStub(), false)
	rec := doJSON(router, http.MethodPost, "/api/v1/platform-admins", `{"userId":"`+testTargetUserID+`"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestChi_Unwired501(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})
	rec := doJSON(router, http.MethodPost, "/api/v1/platform-admins", `{"userId":"`+testTargetUserID+`"}`)
	assert.Equal(t, http.StatusNotImplemented, rec.Code, rec.Body.String())
	rec = doJSON(router, http.MethodGet, "/api/v1/platform-admins", "")
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestChi_SetActive(t *testing.T) {
	stub := activeOperatorStub()
	router := newTestRouter(stub, true)

	// 缺 active 字段 → 400。
	rec := doJSON(router, http.MethodPost, "/api/v1/platform-admins/"+testAdminID+":set-active", `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = doJSON(router, http.MethodPost, "/api/v1/platform-admins/"+testAdminID+":set-active", `{"active":false}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, testAdminID, stub.gotSetActive.ID)
	assert.False(t, stub.gotSetActive.Active)
}

func TestChi_ApproveSuccess(t *testing.T) {
	stub := activeOperatorStub()
	router := newTestRouter(stub, true)
	rec := doJSON(router, http.MethodPost, "/api/v1/platform-admin-scopes/"+testScopeID+":approve", `{}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, testOperatorUserID, stub.gotApprove.ApproverID)
}

func TestChi_ListParsesQuery(t *testing.T) {
	stub := activeOperatorStub()
	router := newTestRouter(stub, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform-admins?page=2&pageSize=5&active=true&keyword=abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 2, stub.gotListQuery.Page)
	assert.Equal(t, 5, stub.gotListQuery.PageSize)
	require.NotNil(t, stub.gotListQuery.Active)
	assert.True(t, *stub.gotListQuery.Active)
	assert.Equal(t, "abc", stub.gotListQuery.Keyword)
	// 空列表必须序列化 [] 而不是 null。
	assert.Contains(t, rec.Body.String(), `"items":[]`)
}

func TestChi_GetInvalidID400(t *testing.T) {
	router := newTestRouter(activeOperatorStub(), true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform-admins/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestChi_DeleteSuccess(t *testing.T) {
	stub := activeOperatorStub()
	router := newTestRouter(stub, true)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/platform-admins/"+testAdminID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, testAdminID, stub.gotDeleteID)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/platform-admin-scopes/"+testScopeID, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, testScopeID, stub.gotDeleteScope)
}

func TestChi_CreateScopeSuccess201(t *testing.T) {
	stub := activeOperatorStub()
	stub.scope = &appplatformadmins.ScopeDetail{ID: testScopeID, PlatformAdminID: testAdminID}
	router := newTestRouter(stub, true)
	rec := doJSON(router, http.MethodPost, "/api/v1/platform-admins/"+testAdminID+"/scopes",
		`{"scopeType":"targeted","targetTenants":["tenant-1"],"expiresAt":"2027-01-01T00:00:00Z"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.NotNil(t, stub.gotScopeCreate.ApprovedBy)
	assert.Equal(t, testOperatorUserID, *stub.gotScopeCreate.ApprovedBy)
	assert.Equal(t, testOperatorUserID, stub.gotScopeCreate.CreatedBy)
}

func TestChi_ListScopesParsesOnlyActive(t *testing.T) {
	stub := activeOperatorStub()
	router := newTestRouter(stub, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform-admins/"+testAdminID+"/scopes?onlyActive=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, stub.gotScopesQuery.OnlyActive)
}
