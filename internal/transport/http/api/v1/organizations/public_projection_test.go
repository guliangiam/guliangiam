package organizations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authv1 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sentinels planted on every credential-bearing organization column. They must
// never appear in a response served without a session.
const (
	secretMasterPassword   = "SENTINEL-master-password"
	secretVerificationCode = "SENTINEL-master-verification-code"
	secretDefaultPassword  = "SENTINEL-default-password"
	secretPasswordSalt     = "SENTINEL-password-salt"
	secretObfuscatorKey    = "SENTINEL-password-obfuscator-key"
	secretKerberosKeytab   = "SENTINEL-kerberos-keytab"
)

var forbiddenPublicKeys = []string{
	"masterPassword",
	"masterVerificationCode",
	"defaultPassword",
	"passwordSalt",
	"passwordObfuscatorKey",
	"kerberosKeytab",
}

var forbiddenPublicValues = []string{
	secretMasterPassword,
	secretVerificationCode,
	secretDefaultPassword,
	secretPasswordSalt,
	secretObfuscatorKey,
	secretKerberosKeytab,
}

// publicProjectionUOW only implements Organizations(); the embedded nil
// interface supplies the remaining IdentityUoW methods. A test that reaches an
// unimplemented repository panics, which is the signal we want.
type publicProjectionUOW struct {
	appuow.IdentityUoW
	orgRepo orgdomain.Repository
}

func (u publicProjectionUOW) Organizations() orgdomain.Repository { return u.orgRepo }

// publicProjectionOrgRepo returns a full organization aggregate, exactly as the
// real repositories do. The projection under test is the transport/app contract,
// not the SQL column list.
type publicProjectionOrgRepo struct {
	orgdomain.Repository
	org *orgdomain.Organization
}

func (r publicProjectionOrgRepo) GetByID(_ context.Context, _ string) (*orgdomain.Organization, error) {
	return r.org, nil
}

func (r publicProjectionOrgRepo) GetBySlug(_ context.Context, _ string) (*orgdomain.Organization, error) {
	return r.org, nil
}

func (r publicProjectionOrgRepo) GetBySlugPublic(_ context.Context, _ string) (*orgdomain.Organization, error) {
	return r.org, nil
}

func publicProjectionFixture() *orgdomain.Organization {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	return &orgdomain.Organization{
		ID:                     "org-public-1",
		Slug:                   "acme",
		DisplayName:            "Acme",
		WebsiteURL:             "https://acme.example.com",
		LogoURL:                "https://acme.example.com/logo.png",
		LogoDarkURL:            "https://acme.example.com/logo-dark.png",
		FaviconURL:             "https://acme.example.com/favicon.ico",
		DefaultAvatar:          "https://acme.example.com/avatar.png",
		Status:                 "active",
		PasswordType:           "argon2id",
		DisableSignin:          false,
		InitiateLoginURL:       "https://acme.example.com/login",
		Settings:               orgdomain.OrganizationSettings{DefaultApplicationID: "app-1"},
		Theme:                  orgdomain.OrganizationThemeSettings{ThemeName: "acme", Tokens: map[string]any{"primary": "#123456"}, CustomCSS: "body{}", Assets: map[string]any{"logo": "x"}},
		MasterPassword:         secretMasterPassword,
		MasterVerificationCode: secretVerificationCode,
		DefaultPassword:        secretDefaultPassword,
		PasswordSalt:           secretPasswordSalt,
		PasswordObfuscatorKey:  secretObfuscatorKey,
		KerberosKeytab:         secretKerberosKeytab,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

func publicProjectionRouter(org *orgdomain.Organization) http.Handler {
	service := apporganizations.Service{
		UOW: publicProjectionUOW{orgRepo: publicProjectionOrgRepo{org: org}},
	}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func anonymousGet(t *testing.T, router http.Handler, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "anonymous GET %s failed: %s", path, rec.Body.String())
	return rec.Code, rec.Body.String()
}

func assertNoCredentialFields(t *testing.T, body string) {
	t.Helper()
	for _, key := range forbiddenPublicKeys {
		assert.NotContains(t, body, `"`+key+`"`, "public response exposes credential field %q", key)
	}
	for _, value := range forbiddenPublicValues {
		assert.NotContains(t, body, value, "public response exposes credential value %q", value)
	}
}

func assertPublicProjectionKeys(t *testing.T, body string) {
	t.Helper()
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(body), &payload))

	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	assert.ElementsMatch(t, []string{
		"id",
		"slug",
		"displayName",
		"websiteUrl",
		"logoUrl",
		"logoDarkUrl",
		"faviconUrl",
		"defaultAvatar",
		"defaultApplicationId",
		"status",
		"passwordType",
		"disableSignin",
		"initiateLoginUrl",
		"theme",
		"createdAt",
		"updatedAt",
	}, keys, "public organization projection must stay minimal")
	assert.Contains(t, body, `"displayName":"Acme"`)
	assert.Contains(t, body, `"defaultApplicationId":"app-1"`)
}

// TestPublicOrganizationProjection_BySlugAndResolve is the P0 regression for
// the anonymous /by-slug and /resolve discovery endpoints. Both share
// GetBySlugPublic, so both must serve the same minimal projection; a fix that
// only covers one route leaves the other leaking.
func TestPublicOrganizationProjection_BySlugAndResolve(t *testing.T) {
	router := publicProjectionRouter(publicProjectionFixture())

	t.Run("by-slug", func(t *testing.T) {
		_, body := anonymousGet(t, router, "/api/v1/organizations/by-slug/acme")
		assertNoCredentialFields(t, body)
		assertPublicProjectionKeys(t, body)
	})

	t.Run("resolve", func(t *testing.T) {
		_, body := anonymousGet(t, router, "/api/v1/organizations/resolve?slug=acme")
		assertNoCredentialFields(t, body)
		assertPublicProjectionKeys(t, body)
	})
}

// TestPublicOrganizationProjection_PathsAreAnonymousWhitelisted pins the
// premise of the regression: if these paths ever lose their anonymous
// exemption, the leak disappears but the pre-login discovery contract breaks.
func TestPublicOrganizationProjection_PathsAreAnonymousWhitelisted(t *testing.T) {
	assert.True(t, authv1.IsTrulyPublicEndpoint("/api/v1/organizations/by-slug/acme"))
	assert.True(t, authv1.IsTrulyPublicEndpoint("/api/v1/organizations/resolve"))
	// Negative control: the management by-ID route must stay authenticated.
	assert.False(t, authv1.IsTrulyPublicEndpoint("/api/v1/organizations/org-public-1"))
}

// TestOrganizationDetailReads_AreNoStore pins that the detail-level organization
// reads (by ID and by slug) never fall through to the ETag middleware's
// `private, no-cache` branch: they carry masked credential fields and a save
// followed by a refresh must observe fresh data.
func TestOrganizationDetailReads_AreNoStore(t *testing.T) {
	org := publicProjectionFixture()
	cfg := httpmw.DefaultETagConfig()

	t.Run("by-id", func(t *testing.T) {
		router := httpmw.ETagCacheMiddleware(cfg)(publicProjectionRouter(org))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+org.ID, nil)
		req = req.WithContext(tenantctx.WithTenantScope(req.Context(), org.ID, org.Slug, tenantctx.TenantFilterStrict))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		assert.Empty(t, rec.Header().Get("ETag"))
	})

	t.Run("by-slug", func(t *testing.T) {
		router := httpmw.ETagCacheMiddleware(cfg)(publicProjectionRouter(org))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/organizations/by-slug/"+org.Slug, nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		assert.Empty(t, rec.Header().Get("ETag"))
	})
}
