package tokens

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppkg "github.com/guliangiam/guliangiam/internal/app/tokens"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainapplications "github.com/guliangiam/guliangiam/internal/domain/applications"
	applicationsmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	domainorganizations "github.com/guliangiam/guliangiam/internal/domain/organizations"
	organizationsmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	domaintokens "github.com/guliangiam/guliangiam/internal/domain/tokens"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
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

func (forgedOrgUUID) NewString() (string, error) { return "generated-token-id", nil }

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

// forgedOrgRouter wires a token service whose repositories expose the tenant
// each use case actually resolves.
func forgedOrgRouter(ctrl *gomock.Controller) (chi.Router, *tokensmock.MockRepository, *appuowmock.MockUnitOfWork) {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := tokensmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Tokens().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
	return router, repo, uow
}

// TestRevokeMany_EmptyBodyRejectedEvenWithTenant pins that the trusted tenant
// does not become the implicit filter: an empty body stays a 400.
func TestRevokeMany_EmptyBodyRejectedEvenWithTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/revoke", strings.NewReader(`{}`)))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestListTokens_ForgedQueryOrganizationIDIgnored pins that the query value
// only requests a tenant; the authenticated scope wins.
func TestListTokens_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domaintokens.ListQuery) ([]*domaintokens.Token, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/tokens?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateToken_ForgedBodyOrganizationIDIgnored pins that a regular caller
// cannot mint a token for another tenant through the JSON body: the user and
// application lookups and the persisted row all use the trusted tenant.
func TestCreateToken_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, uow := forgedOrgRouter(ctrl)

	organizations := organizationsmock.NewMockRepository(ctrl)
	organizations.EXPECT().GetByID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, id string) (*domainorganizations.Organization, error) {
			assert.Equal(t, forgedOrgSessionID, id)
			return &domainorganizations.Organization{ID: forgedOrgSessionID, Slug: "trusted"}, nil
		},
	)
	uow.EXPECT().Organizations().Return(organizations).AnyTimes()

	users := usersmock.NewMockRepository(ctrl)
	users.EXPECT().GetByID(gomock.Any(), "user-1").Return(
		&domainusers.User{ID: "user-1", OrganizationID: forgedOrgSessionID, Username: "user"}, nil)
	uow.EXPECT().Users().Return(users).AnyTimes()

	applications := applicationsmock.NewMockRepository(ctrl)
	applications.EXPECT().GetByID(gomock.Any(), "app-1").Return(
		&domainapplications.Application{ID: "app-1", OrganizationID: forgedOrgSessionID, Slug: "app"}, nil)
	uow.EXPECT().Applications().Return(applications).AnyTimes()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, token *domaintokens.Token) error {
			assert.Equal(t, forgedOrgSessionID, token.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","userId":"user-1","applicationId":"app-1","scopes":["read"],"expiresInSeconds":3600}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
}

func forgedOrgAccessToken(orgID string) *domaintokens.Token {
	now := time.Unix(0, 0).UTC()
	return &domaintokens.Token{
		ID:               "tok-1",
		UserID:           "user-1",
		Username:         "user",
		OrganizationID:   orgID,
		OrganizationSlug: "org",
		ApplicationID:    "app-1",
		ApplicationSlug:  "app",
		TokenHash:        "hash",
		Scopes:           []string{"read"},
		Audience:         []string{"app-1"},
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func forgedOrgRefreshToken(orgID string) *domaintokens.RefreshToken {
	now := time.Unix(0, 0).UTC()
	return &domaintokens.RefreshToken{
		ID:               "rt-1",
		AccessTokenID:    "tok-1",
		UserID:           "user-1",
		Username:         "user",
		OrganizationID:   orgID,
		OrganizationSlug: "org",
		ApplicationID:    "app-1",
		ApplicationSlug:  "app",
		RefreshTokenHash: "hash",
		Scopes:           []string{"read"},
		Audience:         []string{"app-1"},
		ExpiresAt:        now.Add(24 * time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// TestRevokeTokens_ForgedBodyOrganizationIDIgnored pins that the batch revoke
// endpoint resolves the tenant from the authenticated scope, not from the
// caller-supplied JSON body.
func TestRevokeTokens_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domaintokens.RevokeQuery, _ time.Time) (int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/revoke",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","userId":"user-1"}`)))
	assert.Equal(t, http.StatusOK, recorder.Code)
}

// TestRevokeTokens_TrustedTenantScopeAccepted keeps the same-tenant path:
// explicitly naming the caller's own tenant still revokes within that tenant.
func TestRevokeTokens_TrustedTenantScopeAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domaintokens.RevokeQuery, _ time.Time) (int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return 1, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/revoke",
		strings.NewReader(`{"organizationId":"`+forgedOrgSessionID+`","userId":"user-1"}`)))
	assert.Equal(t, http.StatusOK, recorder.Code)
}

// TestIntrospectToken_ForgedBodyOrganizationIDRejected pins that the
// introspect wire contract still has no organizationId field: the forged
// value is a decode error before any token lookup happens.
func TestIntrospectToken_ForgedBodyOrganizationIDRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/introspect",
		strings.NewReader(`{"token":"gl_pat_foreign","organizationId":"`+forgedOrgRequestID+`"}`)))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestIntrospectToken_ForeignTenantTokenInactive pins that a regular caller
// cannot learn whether another tenant's token is live.
func TestIntrospectToken_ForeignTenantTokenInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(
		forgedOrgAccessToken(forgedOrgRequestID), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/introspect",
		strings.NewReader(`{"token":"gl_pat_foreign"}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"active":false`)
}

// TestIntrospectToken_SameTenantTokenActive keeps the same-tenant path.
func TestIntrospectToken_SameTenantTokenActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(
		forgedOrgAccessToken(forgedOrgSessionID), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/introspect",
		strings.NewReader(`{"token":"gl_pat_own"}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"active":true`)
}

// TestRefreshToken_ForgedBodyOrganizationIDRejected mirrors introspect: the
// refresh wire contract has no organizationId field.
func TestRefreshToken_ForgedBodyOrganizationIDRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/refresh",
		strings.NewReader(`{"refreshToken":"gl_prt_foreign","organizationId":"`+forgedOrgRequestID+`"}`)))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestRefreshToken_ForeignTenantTokenInvalid pins that a regular caller
// cannot rotate another tenant's refresh token.
func TestRefreshToken_ForeignTenantTokenInvalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, uow := forgedOrgRouter(ctrl)
	repo.EXPECT().GetRefreshByTokenHash(gomock.Any(), gomock.Any()).Return(
		forgedOrgRefreshToken(forgedOrgRequestID), nil)

	// Allow the rotation side effects to run so the assertion is about the
	// cross-tenant decision rather than an unexpected mock call.
	repo.EXPECT().GetByID(gomock.Any(), "tok-1").Return(
		forgedOrgAccessToken(forgedOrgRequestID), nil).AnyTimes()
	organizations := organizationsmock.NewMockRepository(ctrl)
	organizations.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(
		&domainorganizations.Organization{ID: forgedOrgRequestID, Slug: "foreign"}, nil).AnyTimes()
	uow.EXPECT().Organizations().Return(organizations).AnyTimes()
	users := usersmock.NewMockRepository(ctrl)
	users.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(
		&domainusers.User{ID: "user-1", OrganizationID: forgedOrgRequestID, Username: "user"}, nil).AnyTimes()
	uow.EXPECT().Users().Return(users).AnyTimes()
	applications := applicationsmock.NewMockRepository(ctrl)
	applications.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(
		&domainapplications.Application{ID: "app-1", OrganizationID: forgedOrgRequestID, Slug: "app"}, nil).AnyTimes()
	uow.EXPECT().Applications().Return(applications).AnyTimes()
	repo.EXPECT().Revoke(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	repo.EXPECT().RevokeRefresh(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	repo.EXPECT().CreateRefresh(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/refresh",
		strings.NewReader(`{"refreshToken":"gl_prt_foreign"}`)))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestRefreshToken_SameTenantRotates keeps the same-tenant rotation path.
func TestRefreshToken_SameTenantRotates(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, uow := forgedOrgRouter(ctrl)
	repo.EXPECT().GetRefreshByTokenHash(gomock.Any(), gomock.Any()).Return(
		forgedOrgRefreshToken(forgedOrgSessionID), nil)
	repo.EXPECT().GetByID(gomock.Any(), "tok-1").Return(
		forgedOrgAccessToken(forgedOrgSessionID), nil)
	organizations := organizationsmock.NewMockRepository(ctrl)
	organizations.EXPECT().GetByID(gomock.Any(), forgedOrgSessionID).Return(
		&domainorganizations.Organization{ID: forgedOrgSessionID, Slug: "trusted"}, nil)
	uow.EXPECT().Organizations().Return(organizations).AnyTimes()
	users := usersmock.NewMockRepository(ctrl)
	users.EXPECT().GetByID(gomock.Any(), "user-1").Return(
		&domainusers.User{ID: "user-1", OrganizationID: forgedOrgSessionID, Username: "user"}, nil)
	uow.EXPECT().Users().Return(users).AnyTimes()
	applications := applicationsmock.NewMockRepository(ctrl)
	applications.EXPECT().GetByID(gomock.Any(), "app-1").Return(
		&domainapplications.Application{ID: "app-1", OrganizationID: forgedOrgSessionID, Slug: "app"}, nil)
	uow.EXPECT().Applications().Return(applications).AnyTimes()
	repo.EXPECT().Revoke(gomock.Any(), "tok-1", gomock.Any()).Return(nil)
	repo.EXPECT().RevokeRefresh(gomock.Any(), "rt-1", gomock.Any()).Return(nil)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, token *domaintokens.Token) error {
			assert.Equal(t, forgedOrgSessionID, token.OrganizationID)
			return nil
		})
	repo.EXPECT().CreateRefresh(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, token *domaintokens.RefreshToken) error {
			assert.Equal(t, forgedOrgSessionID, token.OrganizationID)
			return nil
		})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/refresh",
		strings.NewReader(`{"refreshToken":"gl_prt_own"}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"refreshed"`)
}

// TestGetToken_ForeignTenantNotFound pins the by-ID read path: the repository
// scoped read is the ownership gate, and a foreign id is indistinguishable from
// a missing one.
func TestGetToken_ForeignTenantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "tok-1", forgedOrgSessionID).Return(
		nil, apperrors.NewNotFound("token not found"))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/tokens/tok-1", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// TestGetToken_SameTenantFound keeps the by-ID read path for the owning tenant.
func TestGetToken_SameTenantFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "tok-1", forgedOrgSessionID).Return(
		forgedOrgAccessToken(forgedOrgSessionID), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/tokens/tok-1", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"organizationId":"`+forgedOrgSessionID+`"`)
}

// TestGetToken_ForeignRowFromScopedReadStillNotFound pins the transport
// assertion: even if the scoped read regresses and returns a foreign row, the
// response stays a 404.
func TestGetToken_ForeignRowFromScopedReadStillNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "tok-1", forgedOrgSessionID).Return(
		forgedOrgAccessToken(forgedOrgRequestID), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/tokens/tok-1", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// TestRevokeToken_ForeignTenantNotFound pins that the by-ID revoke resolves the
// token inside the trusted tenant before mutating.
func TestRevokeToken_ForeignTenantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "tok-1", forgedOrgSessionID).Return(
		nil, apperrors.NewNotFound("token not found"))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/tok-1/revoke", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// TestRevokeToken_SameTenantRevoked keeps the by-ID revoke path.
func TestRevokeToken_SameTenantRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "tok-1", forgedOrgSessionID).Return(
		forgedOrgAccessToken(forgedOrgSessionID), nil)
	repo.EXPECT().Revoke(gomock.Any(), "tok-1", gomock.Any()).Return(nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tokens/tok-1/revoke", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"revoked"`)
}

// TestDeleteToken_ForeignTenantNotFound pins that the by-ID delete resolves the
// token inside the trusted tenant before deleting.
func TestDeleteToken_ForeignTenantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "tok-1", forgedOrgSessionID).Return(
		nil, apperrors.NewNotFound("token not found"))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodDelete, "/api/v1/tokens/tok-1", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// TestDeleteToken_SameTenantDeleted keeps the by-ID delete path.
func TestDeleteToken_SameTenantDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo, _ := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "tok-1", forgedOrgSessionID).Return(
		forgedOrgAccessToken(forgedOrgSessionID), nil)
	repo.EXPECT().Delete(gomock.Any(), "tok-1").Return(nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodDelete, "/api/v1/tokens/tok-1", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"deleted"`)
}

// TestTokens_ByIDWithoutTenantScopeRejected pins the fail-fast guard for the
// by-ID routes.
func TestTokens_ByIDWithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})

	for _, tc := range []struct {
		name   string
		method string
		target string
	}{
		{name: "get", method: http.MethodGet, target: "/api/v1/tokens/tok-1"},
		{name: "revoke", method: http.MethodPost, target: "/api/v1/tokens/tok-1/revoke"},
		{name: "delete", method: http.MethodDelete, target: "/api/v1/tokens/tok-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

// TestTokens_WithoutTenantScopeRejected pins the fail-fast guard.
func TestTokens_WithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "list", method: http.MethodGet, target: "/api/v1/tokens?organizationId=" + forgedOrgRequestID},
		{name: "create", method: http.MethodPost, target: "/api/v1/tokens",
			body: `{"organizationId":"` + forgedOrgRequestID + `","userId":"user-1","applicationId":"app-1","scopes":["read"],"expiresInSeconds":3600}`},
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
