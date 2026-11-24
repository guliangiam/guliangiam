package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/gateway/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-gateway-id", nil }

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

func forgedOrgRouter(ctrl *gomock.Controller, repo *domainmock.MockRepository) chi.Router {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Gateway().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appgateway.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router
}

// TestListServers_ForgedQueryOrganizationIDIgnored pins the server list route.
func TestListServers_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := domainmock.NewMockRepository(ctrl)
	repo.EXPECT().ListServers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListServersQuery) ([]*domain.Server, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	forgedOrgRouter(ctrl, repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/gateway/servers?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateServer_ForgedBodyOrganizationIDIgnored pins the server create route.
func TestCreateServer_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := domainmock.NewMockRepository(ctrl)
	repo.EXPECT().CreateServer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, server *domain.Server) error {
			assert.Equal(t, forgedOrgSessionID, server.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	forgedOrgRouter(ctrl, repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/gateway/servers",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"server","endpoint":"https://gw.example.com","protocol":"http","status":"active"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "/api/v1/gateway/servers/generated-gateway-id", recorder.Header().Get("Location"))
	assert.JSONEq(t, `{"id":"generated-gateway-id","status":"created"}`, recorder.Body.String())
}

// TestListSites_ForgedQueryOrganizationIDIgnored pins the site list route.
func TestListSites_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := domainmock.NewMockRepository(ctrl)
	repo.EXPECT().ListSites(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListSitesQuery) ([]*domain.Site, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	forgedOrgRouter(ctrl, repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/gateway/sites?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateSite_ForgedBodyOrganizationIDIgnored pins the site create route.
func TestCreateSite_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := domainmock.NewMockRepository(ctrl)
	repo.EXPECT().CreateSite(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, site *domain.Site) error {
			assert.Equal(t, forgedOrgSessionID, site.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	forgedOrgRouter(ctrl, repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/gateway/sites",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"site","host":"site.example.com","status":"active"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "/api/v1/gateway/sites/generated-gateway-id", recorder.Header().Get("Location"))
	assert.JSONEq(t, `{"id":"generated-gateway-id","status":"created"}`, recorder.Body.String())
}

// TestCreateServer_WithoutTenantScopeRejected pins the fail-fast guard.
func TestCreateServer_WithoutTenantScopeRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/servers",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"server","endpoint":"https://gw.example.com","protocol":"http","status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appgateway.Service{}})
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
