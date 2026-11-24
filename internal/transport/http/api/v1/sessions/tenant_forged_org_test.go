package sessions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	tokensdomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
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

func trustedTenantRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

// TestListSessions_ForgedQueryOrganizationIDIgnored pins GET /sessions.
func TestListSessions_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := sessionsmock.NewMockRepository(ctrl)
	uow.EXPECT().Sessions().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Session, int64, error) {
			assert.Equal(t, forgedOrgSessionID, q.OrganizationID)
			assert.Equal(t, "user-1", q.UserID)
			return nil, 0, nil
		},
	)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: appsessions.Service{UOW: uow}})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, trustedTenantRequest(http.MethodGet,
		"/api/v1/sessions?organizationId="+forgedOrgRequestID+"&userId=user-1", ""))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestForceLogoutOrganization_ForgedBodyOrganizationIDIgnored pins the org-wide
// revocation write: the body's organizationId must not escape the trusted scope.
func TestForceLogoutOrganization_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	authUOW := appuowmock.NewMockAuthUoW(ctrl)
	fullUOW := appuowmock.NewMockUnitOfWork(ctrl)
	sessionRepo := sessionsmock.NewMockRepository(ctrl)
	tokenRepo := tokensmock.NewMockRepository(ctrl)

	fullUOW.EXPECT().Sessions().Return(sessionRepo).AnyTimes()
	fullUOW.EXPECT().Tokens().Return(tokenRepo).AnyTimes()
	fullUOW.EXPECT().Outbox().Return(nil).AnyTimes()
	authUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullUOW)
		},
	)
	sessionRepo.EXPECT().RevokeByOrganization(gomock.Any(), forgedOrgSessionID, gomock.Any()).
		Return(int64(1), nil)
	tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q tokensdomain.RevokeQuery, _ time.Time) (int64, error) {
			assert.Equal(t, forgedOrgSessionID, q.OrganizationID)
			return 1, nil
		},
	)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: appsessions.Service{Clock: forgedOrgClock{}, UOW: authUOW}})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, trustedTenantRequest(http.MethodPost,
		"/api/v1/sessions/force-logout-organization",
		`{"organizationId":"`+forgedOrgRequestID+`"}`))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestListSessionsWithoutTenantScopeRejected pins the empty-org fail-fast.
func TestListSessionsWithoutTenantScopeRejected(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
