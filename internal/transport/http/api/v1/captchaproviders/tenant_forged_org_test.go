package captchaproviders

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppkg "github.com/guliangiam/guliangiam/internal/app/captchaproviders"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/captchaproviders/mock"
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

func (forgedOrgUUID) NewString() (string, error) { return "generated-captcha-provider-id", nil }

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
	uow.EXPECT().CaptchaProviders().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router, repo
}

// TestListCaptchaProviders_ForgedQueryOrganizationIDIgnored pins that the query
// value only requests a tenant; the authenticated scope wins.
func TestListCaptchaProviders_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.CaptchaProvider, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/captcha-providers?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateCaptchaProvider_ForgedBodyOrganizationIDIgnored pins that a regular
// caller cannot move a provider write into another tenant through the JSON body.
func TestCreateCaptchaProvider_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, provider *domain.CaptchaProvider) error {
			assert.Equal(t, forgedOrgSessionID, provider.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/captcha-providers",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"turnstile","providerType":"turnstile"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
}

// TestCaptchaProviders_WithoutTenantScopeRejected pins the fail-fast guard.
func TestCaptchaProviders_WithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "list", method: http.MethodGet, target: "/api/v1/captcha-providers?organizationId=" + forgedOrgRequestID},
		{name: "create", method: http.MethodPost, target: "/api/v1/captcha-providers",
			body: `{"organizationId":"` + forgedOrgRequestID + `","name":"turnstile","providerType":"turnstile"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.target, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
