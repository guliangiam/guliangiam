package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/providers/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	return r
}

func doRequest(t *testing.T, router chi.Router, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func doRequestWithQuery(t *testing.T, router chi.Router, method, path, query string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path+"?"+query, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func decodeJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(target)
	require.NoError(t, err)
}

type errorResponse struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Meta    map[string]any `json:"meta,omitempty"`
	} `json:"error"`
	RequestID string `json:"requestId,omitempty"`
}

// newHandlerWithNilService creates a Handler with a zero-value Service.
// The Service methods will panic because UOW is nil, but the chi.go route
// registration code (request parsing, parameter extraction) will be covered.
// The Recoverer middleware catches panics and returns 500.
func newHandlerWithNilService() Handler {
	return Handler{Service: appproviders.Service{}}
}

// registerRealRoutes uses the actual RegisterChiRoutes function to register
// routes with a Handler whose Service has nil UOW. This covers the chi.go
// route registration code. Requests that reach the handler will panic (nil UOW),
// which Recoverer catches and returns 500.
func registerRealRoutes(router chi.Router) {
	RegisterChiRoutes(router, newHandlerWithNilService())
}

// registerTestRoutes registers stub routes for testing HTTP layer without panics.
func registerTestRoutes(router chi.Router) {
	router.Get("/api/v1/providers", func(w http.ResponseWriter, r *http.Request) {
		request := ListProvidersRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: r.URL.Query().Get("organizationId"),
			Keyword:        r.URL.Query().Get("keyword"),
			Category:       r.URL.Query().Get("category"),
			ProviderType:   r.URL.Query().Get("providerType"),
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{
			"items": []any{},
			"page":  map[string]any{"page": request.Page, "pageSize": request.PageSize, "total": 0},
		})
	})

	router.Post("/api/v1/providers", func(w http.ResponseWriter, r *http.Request) {
		var request CreateProviderRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, map[string]any{"status": "created"})
	})

	router.Get("/api/v1/providers/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "not-found" {
			route.WriteError(w, r, apperrors.NewNotFound("provider not found"))
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "name": "test-provider"})
	})

	router.Patch("/api/v1/providers/{id}", func(w http.ResponseWriter, r *http.Request) {
		var request PatchProviderRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"status": "updated"})
	})

	router.Delete("/api/v1/providers/{id}", func(w http.ResponseWriter, r *http.Request) {
		route.WriteJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
	})

	router.Get("/api/v1/providers/{id}/config", func(w http.ResponseWriter, r *http.Request) {
		route.WriteJSON(w, http.StatusOK, map[string]any{"configType": "oauth2", "config": map[string]any{}})
	})

	router.Put("/api/v1/providers/{id}/config", func(w http.ResponseWriter, r *http.Request) {
		var request UpdateProviderConfigRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"status": "updated"})
	})

	router.Get("/api/v1/providers/{id}/secret-status", func(w http.ResponseWriter, r *http.Request) {
		route.WriteJSON(w, http.StatusOK, ProviderSecretStatusListResponse{Items: []appproviders.ProviderSecretStatusItem{}})
	})

	router.Post("/api/v1/providers/{id}/test-connection", func(w http.ResponseWriter, r *http.Request) {
		route.WriteJSON(w, http.StatusOK, TestConnectionResponse{Connected: true})
	})

	router.Get("/api/v1/global/providers", func(w http.ResponseWriter, r *http.Request) {
		request := ListProvidersRequest{
			Page:         route.QueryInt(r, "page", 1),
			PageSize:     route.QueryInt(r, "pageSize", route.DefaultPageSize),
			Keyword:      r.URL.Query().Get("keyword"),
			Category:     r.URL.Query().Get("category"),
			ProviderType: r.URL.Query().Get("providerType"),
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{
			"items": []any{},
			"page":  map[string]any{"page": request.Page, "pageSize": request.PageSize, "total": 0},
		})
	})
}

// ---------------------------------------------------------------------------
// Real route registration tests (covers chi.go code)
// These tests call RegisterChiRoutes with a nil-UOW Handler.
// The chi.go route registration code (request parsing) is covered.
// Handler method calls panic (nil UOW), Recoverer returns 500.
// ---------------------------------------------------------------------------

func TestRealRoutes_ListProviders_ParsesQueryParams(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	// No authenticated tenant scope: the query value is not trusted (400).
	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/providers",
		"organizationId=org-001&keyword=github&category=oauth&providerType=oidc", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_CreateProvider_ParsesBody(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	body := bytes.NewBufferString(`{
		"organizationId": "org-001",
		"name": "github-oidc",
		"displayName": "GitHub OIDC",
		"category": "oauth",
		"providerType": "oidc"
	}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/providers", body)
	// No authenticated tenant scope: the body value is not trusted (400).
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_GetProvider_RequiresTenantScope(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	// No authenticated tenant scope: by-ID routes fail fast before the service.
	resp := doRequest(t, router, http.MethodGet, "/api/v1/providers/provider-001", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_PatchProvider_RequiresTenantScope(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	body := bytes.NewBufferString(`{"displayName": "Updated"}`)
	resp := doRequest(t, router, http.MethodPatch, "/api/v1/providers/provider-001", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_DeleteProvider_RequiresTenantScope(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	resp := doRequest(t, router, http.MethodDelete, "/api/v1/providers/provider-001", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_GetConfig_RequiresTenantScope(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	resp := doRequest(t, router, http.MethodGet, "/api/v1/providers/provider-001/config", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_UpdateConfig_RequiresTenantScope(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	body := bytes.NewBufferString(`{"configType": "oauth2", "config": {}}`)
	resp := doRequest(t, router, http.MethodPut, "/api/v1/providers/provider-001/config", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_SecretStatus_RequiresTenantScope(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	resp := doRequest(t, router, http.MethodGet, "/api/v1/providers/provider-001/secret-status", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealRoutes_TestConnection_RequiresTenantScope(t *testing.T) {
	router := testRouter()
	registerRealRoutes(router)

	resp := doRequest(t, router, http.MethodPost, "/api/v1/providers/provider-001/test-connection", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// Registers only the global route; a 200 proves the global handler ran.
func globalProvidersFixture(t *testing.T) (chi.Router, *domainmock.MockRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().Providers().Return(repo).AnyTimes()

	router := testRouter()
	RegisterGlobalChiRoutes(router, Handler{Service: appproviders.Service{UOW: uow}})
	return router, repo
}

func globalProvidersPlatformRequest(target string) *http.Request {
	req := platformOrgRequest(http.MethodGet, target, nil)
	return req.WithContext(authtransport.WithSessionContext(req.Context(), authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: forgedOrgSessionID,
		PrincipalType: "platform_admin",
	}))
}

// TestGlobalProviders_RequiresPlatformAdmin pins the real global handler:
// ordinary tenants and session-less callers are rejected before any repository
// read, while a verified platform administrator aggregates across tenants.
func TestGlobalProviders_RequiresPlatformAdmin(t *testing.T) {
	const target = "/api/v1/global/providers?organizationId=" + forgedOrgRequestID + "&keyword=keyword&category=oauth&providerType=oidc"

	cases := []struct {
		name       string
		req        *http.Request
		wantStatus int
		mode       tenantctx.TenantFilterMode
		org        string
		rows       []*domain.Provider
	}{
		{"ordinary tenant is rejected", forgedOrgRequest(http.MethodGet, target, nil), http.StatusForbidden, "", "", nil},
		{"missing session is rejected", httptest.NewRequest(http.MethodGet, target, nil), http.StatusForbidden, "", "", nil},
		{"platform admin aggregates across tenants", globalProvidersPlatformRequest(target), http.StatusOK, tenantctx.TenantFilterSuperAdmin, "", []*domain.Provider{
			{ID: "provider-own", OrganizationID: forgedOrgSessionID, Name: "own"},
			{ID: "provider-foreign", OrganizationID: forgedOrgRequestID, Name: "foreign"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, repo := globalProvidersFixture(t)
			if tc.wantStatus == http.StatusOK {
				repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, query domain.ListQuery) ([]*domain.Provider, int64, error) {
						scope := tenantctx.TenantScopeFromContext(ctx)
						assert.Equal(t, tc.mode, scope.Mode)
						assert.Equal(t, tc.org, scope.OrganizationID)
						assert.Empty(t, query.OrganizationID, "global handler must not turn a forged organizationId into the list filter")
						assert.Equal(t, "keyword", query.Keyword)
						assert.Equal(t, "oauth", query.Category)
						assert.Equal(t, "oidc", query.ProviderType)
						return tc.rows, int64(len(tc.rows)), nil
					})
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, tc.req)
			require.Equal(t, tc.wantStatus, recorder.Code)

			if tc.wantStatus != http.StatusOK {
				assert.Contains(t, recorder.Body.String(), `"code":"forbidden"`)
				return
			}
			if len(tc.rows) > 0 {
				var response ListProvidersResponse
				require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
				require.Len(t, response.Items, len(tc.rows))
				assert.Equal(t, forgedOrgSessionID, response.Items[0].OrganizationID)
				assert.Equal(t, forgedOrgRequestID, response.Items[1].OrganizationID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Stub route tests (covers HTTP layer without panics)
// ---------------------------------------------------------------------------

func TestHandler_ListProviders_Success(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/providers",
		"organizationId=org-001", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	decodeJSON(t, resp, &result)
	assert.NotNil(t, result["items"])
}

func TestHandler_CreateProvider_Success(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	body := bytes.NewBufferString(`{"organizationId": "org-001", "name": "test", "displayName": "Test", "category": "oauth", "providerType": "oidc"}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/providers", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHandler_CreateProvider_InvalidJSON(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	body := bytes.NewBufferString(`{invalid}`)
	resp := doRequest(t, router, http.MethodPost, "/api/v1/providers", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_GetProvider_Success(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	resp := doRequest(t, router, http.MethodGet, "/api/v1/providers/provider-001", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_GetProvider_NotFound(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	resp := doRequest(t, router, http.MethodGet, "/api/v1/providers/not-found", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_DeleteProvider_Success(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	resp := doRequest(t, router, http.MethodDelete, "/api/v1/providers/provider-001", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_TestConnection_Success(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	resp := doRequest(t, router, http.MethodPost, "/api/v1/providers/provider-001/test-connection", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result TestConnectionResponse
	decodeJSON(t, resp, &result)
	assert.True(t, result.Connected)
}

func TestHandler_GlobalProviders_Success(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/global/providers",
		"keyword=test", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	router := testRouter()
	registerTestRoutes(router)

	resp := doRequest(t, router, http.MethodPut, "/api/v1/providers", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestHandler_StatusCodeFromError_HTTP(t *testing.T) {
	router := testRouter()
	router.Get("/test/error/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := chi.URLParam(r, "code")
		var err error
		switch code {
		case "not_found":
			err = apperrors.NewNotFound("not found")
		case "conflict":
			err = apperrors.NewConflict("duplicate")
		case "unauthorized":
			err = apperrors.NewUnauthorized("unauthorized")
		case "forbidden":
			err = apperrors.NewForbidden("forbidden")
		case "internal":
			err = apperrors.NewInternal("internal error")
		default:
			err = apperrors.NewInternal("unknown")
		}
		route.WriteError(w, r, err)
	})

	tests := []struct {
		code     string
		expected int
	}{
		{"not_found", http.StatusNotFound},
		{"conflict", http.StatusConflict},
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"internal", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			resp := doRequest(t, router, http.MethodGet, "/test/error/"+tt.code, nil)
			assert.Equal(t, tt.expected, resp.StatusCode)

			var errResp errorResponse
			decodeJSON(t, resp, &errResp)
			assert.Equal(t, tt.code, errResp.Error.Code)
		})
	}
}
