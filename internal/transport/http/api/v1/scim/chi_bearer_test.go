package scim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainapplications "github.com/guliangiam/guliangiam/internal/domain/applications"
	applicationsmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const scimTestBearerToken = "scim-test-bearer-token"

// scimAuthService builds an application service whose validation lists the
// applications of orgID, returning one whose client secret matches
// scimTestBearerToken.
func scimAuthService(t *testing.T, orgID string) appapplications.Service {
	t.Helper()

	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := applicationsmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo)

	hash, err := platformcrypto.HashClientSecret(scimTestBearerToken)
	require.NoError(t, err)

	// New secret rows are empty in this fixture; verification falls back to the
	// legacy column, which the expectation below provides.
	repo.EXPECT().ListUsableClientSecrets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domainapplications.ListQuery) ([]*domainapplications.Application, int64, error) {
			assert.Equal(t, orgID, query.OrganizationID)
			return []*domainapplications.Application{{
				OrganizationID:   orgID,
				ClientSecretHash: hash,
			}}, 1, nil
		})
	return appapplications.Service{UOW: uow}
}

func scimBearerRequest(target, token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// scimAuthProbe records what the downstream handler sees on success.
type scimAuthProbe struct {
	called bool
	orgID  string
	mode   tenantctx.TenantFilterMode
}

func (p *scimAuthProbe) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.called = true
		p.orgID = resolveOrgID(r)
		p.mode = tenantctx.TenantScopeFromContext(r.Context()).Mode
		w.WriteHeader(http.StatusNoContent)
	})
}

// TestBearerTokenAuth_InjectsValidatedScope pins the GAP-011/014 rule for the
// SCIM middleware: handlers read the organization bound by the middleware, and
// a forged X-Organization-Id cannot redirect that binding.
func TestBearerTokenAuth_InjectsValidatedScope(t *testing.T) {
	service := scimAuthService(t, "org-validated")
	probe := &scimAuthProbe{}
	req := scimBearerRequest("/scim/v2/Users?organizationId=org-validated", scimTestBearerToken)
	req.Header.Set("X-Organization-Id", "org-forged")
	w := httptest.NewRecorder()

	BearerTokenAuth(service)(probe.handler()).ServeHTTP(w, req)

	require.True(t, probe.called)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "org-validated", probe.orgID)
	assert.Equal(t, tenantctx.TenantFilterStrict, probe.mode)
}

// TestBearerTokenAuth_InjectedScopeIsValidationTarget asserts the injected value
// is exactly the organization the token was validated against: the query value
// selects the validation target, and once it passes that same value is injected.
func TestBearerTokenAuth_InjectedScopeIsValidationTarget(t *testing.T) {
	service := scimAuthService(t, "org-query")
	probe := &scimAuthProbe{}
	req := scimBearerRequest("/scim/v2/Users?organizationId=org-query", scimTestBearerToken)
	req.Header.Set("X-Organization-Id", "org-header")
	w := httptest.NewRecorder()

	BearerTokenAuth(service)(probe.handler()).ServeHTTP(w, req)

	require.True(t, probe.called)
	assert.Equal(t, "org-query", probe.orgID)
}

// TestBearerTokenAuth_InvalidTokenDoesNotInject rejects an invalid token and
// leaves no tenant scope for the downstream handler to consume.
func TestBearerTokenAuth_InvalidTokenDoesNotInject(t *testing.T) {
	service := scimAuthService(t, "org-validated")
	probe := &scimAuthProbe{}
	req := scimBearerRequest("/scim/v2/Users?organizationId=org-validated", "wrong-token")
	w := httptest.NewRecorder()

	BearerTokenAuth(service)(probe.handler()).ServeHTTP(w, req)

	assert.False(t, probe.called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestBearerTokenAuth_MissingOrganizationDoesNotInject rejects requests without
// an organization before any validation or scope injection happens.
func TestBearerTokenAuth_MissingOrganizationDoesNotInject(t *testing.T) {
	probe := &scimAuthProbe{}
	req := scimBearerRequest("/scim/v2/Users", scimTestBearerToken)
	w := httptest.NewRecorder()

	BearerTokenAuth(appapplications.Service{})(probe.handler()).ServeHTTP(w, req)

	assert.False(t, probe.called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
