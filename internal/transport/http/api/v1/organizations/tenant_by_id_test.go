package organizations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	outboxdomain "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	tenantinstances "github.com/guliangiam/guliangiam/internal/domain/tenant_instances"
	"github.com/guliangiam/guliangiam/internal/domain/tenants"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	byIDOwnOrgID     = "11111111-1111-1111-1111-111111111111"
	byIDForeignOrgID = "22222222-2222-2222-2222-222222222222"
	byIDMissingOrgID = "33333333-3333-3333-3333-333333333333"
)

type byIDFixedClock struct{}

func (byIDFixedClock) Now() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }

// byIDOrgRepo simulates a store without row-level security (MySQL): it happily
// returns the foreign organization. Without a transport tenant gate every
// by-ID route succeeds against it, which is exactly the P1 exposure.
type byIDOrgRepo struct {
	orgdomain.Repository
	orgs          map[string]*orgdomain.Organization
	getCalls      []string
	updateCalls   int
	deleteCalls   int
	settingsCalls int
	themeCalls    int
	policyCalls   int
}

func (r *byIDOrgRepo) GetByID(_ context.Context, id string) (*orgdomain.Organization, error) {
	r.getCalls = append(r.getCalls, id)
	org, ok := r.orgs[id]
	if !ok {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return org, nil
}

func (r *byIDOrgRepo) Update(context.Context, *orgdomain.Organization) error {
	r.updateCalls++
	return nil
}

func (r *byIDOrgRepo) UpdateSettings(context.Context, *orgdomain.OrganizationSettings) error {
	r.settingsCalls++
	return nil
}

func (r *byIDOrgRepo) UpdateTheme(context.Context, *orgdomain.OrganizationThemeSettings) error {
	r.themeCalls++
	return nil
}

func (r *byIDOrgRepo) UpdateSecurityPolicy(context.Context, *orgdomain.OrganizationSecurityPolicy) error {
	r.policyCalls++
	return nil
}

func (r *byIDOrgRepo) Delete(context.Context, string) error {
	r.deleteCalls++
	return nil
}

// byIDAppRepo honours the query's OrganizationID, mirroring the MySQL List
// predicate. The exposure is not the repo: it is that the route feeds it a
// path-controlled organization id.
type byIDAppRepo struct {
	appdomain.Repository
	apps     []*appdomain.Application
	listOrgs []string
}

func (r *byIDAppRepo) List(_ context.Context, query appdomain.ListQuery) ([]*appdomain.Application, int64, error) {
	r.listOrgs = append(r.listOrgs, query.OrganizationID)
	items := make([]*appdomain.Application, 0, len(r.apps))
	for _, app := range r.apps {
		if query.OrganizationID == "" || app.OrganizationID == query.OrganizationID {
			items = append(items, app)
		}
	}
	return items, int64(len(items)), nil
}

// ListActiveClientJWKs returns no keys: this fake serves the default-application
// route, which projects the JWKS set but does not authenticate a client.
func (r *byIDAppRepo) ListActiveClientJWKs(_ context.Context, _, _ string) ([]*appdomain.ClientJWK, error) {
	return nil, nil
}

type byIDRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *byIDRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type byIDUOW struct {
	appuow.UnitOfWork
	orgRepo *byIDOrgRepo
	appRepo *byIDAppRepo
	records *byIDRecords
}

func (u *byIDUOW) Organizations() orgdomain.Repository { return u.orgRepo }
func (u *byIDUOW) Applications() appdomain.Repository  { return u.appRepo }
func (u *byIDUOW) Records() recordsdomain.Repository   { return u.records }
func (u *byIDUOW) Outbox() outboxdomain.Repository     { return nil }
func (u *byIDUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// byIDTenants is a no-op tenant port: this package only exercises deletion
// routing/authorization, not provisioning semantics.
type byIDTenants struct{}

func (byIDTenants) CreateIdentity(context.Context, *tenantidentities.TenantIdentity) error {
	return nil
}

func (byIDTenants) GetIdentity(context.Context, string) (*tenantidentities.TenantIdentity, error) {
	return nil, apperrors.NewNotFound("tenant identity not found")
}

func (byIDTenants) DecommissionIdentity(context.Context, string, time.Time) error { return nil }

func (byIDTenants) CreatePrimaryTopology(context.Context, *tenantinstances.TenantInstance) error {
	return nil
}

func (byIDTenants) GetPrimaryTopology(context.Context, string) (*tenantinstances.TenantInstance, error) {
	return nil, apperrors.NewNotFound("tenant primary topology not found")
}

func (byIDTenants) DecommissionTopologies(context.Context, string, time.Time) error { return nil }

func (byIDTenants) PrimaryRegion(context.Context) (*tenantinstances.Region, error) {
	return &tenantinstances.Region{ID: "region-1", RegionCode: "cn-north", CountryCode: "CN"}, nil
}

func (u *byIDUOW) Tenants() tenants.Repository { return byIDTenants{} }

func byIDOrganization(id, slug, displayName string) *orgdomain.Organization {
	return &orgdomain.Organization{
		ID:                     id,
		Slug:                   slug,
		DisplayName:            displayName,
		WebsiteURL:             "https://" + slug + ".example.com",
		Status:                 "active",
		PasswordType:           "argon2id",
		MasterPassword:         secretMasterPassword,
		MasterVerificationCode: secretVerificationCode,
		DefaultPassword:        secretDefaultPassword,
		PasswordSalt:           secretPasswordSalt,
		PasswordObfuscatorKey:  secretObfuscatorKey,
		KerberosKeytab:         secretKerberosKeytab,
		Revision:               1,
		Settings: orgdomain.OrganizationSettings{
			OrganizationID:       id,
			DefaultApplicationID: "app-" + slug,
		},
		Theme: orgdomain.OrganizationThemeSettings{
			OrganizationID: id,
			ThemeName:      slug,
			Tokens:         map[string]any{"primary": "#123456"},
		},
		Security: orgdomain.OrganizationSecurityPolicy{
			OrganizationID: id,
			PasswordPolicy: map[string]any{"minLength": float64(8)},
		},
		CreatedAt: byIDFixedClock{}.Now(),
		UpdatedAt: byIDFixedClock{}.Now(),
	}
}

func byIDApplication(id, orgID, slug string) *appdomain.Application {
	return &appdomain.Application{
		ID:             id,
		OrganizationID: orgID,
		Slug:           slug,
		DisplayName:    slug,
		Category:       "identity",
		AppType:        "web",
		Status:         "active",
		CreatedAt:      byIDFixedClock{}.Now(),
		UpdatedAt:      byIDFixedClock{}.Now(),
	}
}

type byIDFixture struct {
	router  chi.Router
	orgs    *byIDOrgRepo
	apps    *byIDAppRepo
	records *byIDRecords
}

func newByIDFixture() byIDFixture {
	orgRepo := &byIDOrgRepo{orgs: map[string]*orgdomain.Organization{
		byIDOwnOrgID:     byIDOrganization(byIDOwnOrgID, "own", "Own Corp"),
		byIDForeignOrgID: byIDOrganization(byIDForeignOrgID, "foreign", "Foreign Corp"),
	}}
	appRepo := &byIDAppRepo{apps: []*appdomain.Application{
		byIDApplication("app-own", byIDOwnOrgID, "own-app"),
		byIDApplication("app-foreign", byIDForeignOrgID, "foreign-app"),
	}}
	records := &byIDRecords{}
	unit := &byIDUOW{orgRepo: orgRepo, appRepo: appRepo, records: records}

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{
		Service:            apporganizations.Service{Clock: byIDFixedClock{}, UOW: unit},
		ApplicationService: appapplications.Service{UOW: unit},
	})
	return byIDFixture{router: router, orgs: orgRepo, apps: appRepo, records: records}
}

func byIDTenantRequest(method, target, body string) *http.Request {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, target, reader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	ctx := tenantctx.WithTenantScope(req.Context(), byIDOwnOrgID, "own", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID:           "55555555-5555-5555-5555-555555555555",
		OrganizationID:   byIDOwnOrgID,
		OrganizationSlug: "own",
		PrincipalType:    "user",
	})
	return req.WithContext(ctx)
}

func byIDPlatformAdminRequest(method, target, body string) *http.Request {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, target, reader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	ctx := tenantctx.WithTenantScope(req.Context(), byIDOwnOrgID, "own", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID:           "66666666-6666-6666-6666-666666666666",
		OrganizationID:   byIDOwnOrgID,
		OrganizationSlug: "own",
		PrincipalType:    "platform_admin",
	})
	return req.WithContext(ctx)
}

func serveByID(t *testing.T, fixture byIDFixture, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	fixture.router.ServeHTTP(recorder, req)
	return recorder
}

type byIDRoute struct {
	name   string
	method string
	suffix string
	body   string
}

// byIDRoutes covers all twelve organization by-ID routes in one table.
func byIDRoutes() []byIDRoute {
	return []byIDRoute{
		{name: "get", method: http.MethodGet, suffix: ""},
		{name: "patch", method: http.MethodPatch, suffix: "", body: `{"displayName":"renamed","revision":1}`},
		{name: "delete", method: http.MethodDelete, suffix: ""},
		{name: "put-settings", method: http.MethodPut, suffix: "/settings", body: `{}`},
		{name: "put-configuration", method: http.MethodPut, suffix: "/configuration", body: `{}`},
		{name: "put-theme", method: http.MethodPut, suffix: "/theme", body: `{"themeName":"acme"}`},
		{name: "put-security-policy", method: http.MethodPut, suffix: "/security-policy", body: `{}`},
		{name: "get-settings", method: http.MethodGet, suffix: "/settings"},
		{name: "get-theme", method: http.MethodGet, suffix: "/theme"},
		{name: "get-security-policy", method: http.MethodGet, suffix: "/security-policy"},
		{name: "get-applications", method: http.MethodGet, suffix: "/applications"},
		{name: "get-default-application", method: http.MethodGet, suffix: "/default-application"},
	}
}

func byIDPath(orgID, suffix string) string {
	return "/api/v1/organizations/" + orgID + suffix
}

// TestOrganizationByID_ForeignTenantRejected is the P1 regression: the store
// returns the foreign organization (no RLS), so only the transport tenant gate
// keeps it unreachable. A foreign row must look exactly like a missing one and
// must not be read, written, deleted, listed or audited.
func TestOrganizationByID_ForeignTenantRejected(t *testing.T) {
	for _, route := range byIDRoutes() {
		t.Run(route.name, func(t *testing.T) {
			fixture := newByIDFixture()

			foreign := serveByID(t, fixture, byIDTenantRequest(route.method,
				byIDPath(byIDForeignOrgID, route.suffix), route.body))
			missing := serveByID(t, fixture, byIDTenantRequest(route.method,
				byIDPath(byIDMissingOrgID, route.suffix), route.body))

			require.Equal(t, http.StatusNotFound, foreign.Code,
				"foreign organization must be rejected: %s", foreign.Body.String())
			require.Equal(t, http.StatusNotFound, missing.Code)
			assert.Equal(t, missing.Body.String(), foreign.Body.String(),
				"a foreign organization must be indistinguishable from a missing one")

			assert.NotContains(t, foreign.Body.String(), "Foreign Corp")
			assert.NotContains(t, foreign.Body.String(), "app-foreign")
			assert.NotContains(t, foreign.Body.String(), secretMasterPassword)
			assert.NotContains(t, foreign.Body.String(), secretKerberosKeytab)

			assert.Empty(t, fixture.orgs.getCalls, "foreign tenant must not reach the organization repository")
			assert.Zero(t, fixture.orgs.updateCalls)
			assert.Zero(t, fixture.orgs.deleteCalls)
			assert.Zero(t, fixture.orgs.settingsCalls)
			assert.Zero(t, fixture.orgs.themeCalls)
			assert.Zero(t, fixture.orgs.policyCalls)
			assert.Empty(t, fixture.apps.listOrgs, "foreign tenant must not list applications")
			assert.Empty(t, fixture.records.created, "foreign tenant must not produce an audit event")
		})
	}
}

// TestOrganizationByID_ForgedQueryCannotWidenScope pins that a query value
// cannot compensate for a foreign path id on these path-scoped routes.
func TestOrganizationByID_ForgedQueryCannotWidenScope(t *testing.T) {
	fixture := newByIDFixture()
	recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodGet,
		byIDPath(byIDForeignOrgID, "")+"?organizationId="+byIDOwnOrgID, ""))

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	assert.Empty(t, fixture.orgs.getCalls)
}

// TestOrganizationByID_MissingTenantRejected pins the fail-closed path when no
// authenticated tenant scope exists at all.
func TestOrganizationByID_MissingTenantRejected(t *testing.T) {
	for _, route := range byIDRoutes() {
		t.Run(route.name, func(t *testing.T) {
			fixture := newByIDFixture()

			var req *http.Request
			if route.body != "" {
				req = httptest.NewRequest(route.method, byIDPath(byIDOwnOrgID, route.suffix), strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(route.method, byIDPath(byIDOwnOrgID, route.suffix), nil)
			}

			recorder := serveByID(t, fixture, req)
			require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
			assert.Empty(t, fixture.orgs.getCalls)
			assert.Zero(t, fixture.orgs.updateCalls)
			assert.Zero(t, fixture.orgs.deleteCalls)
			assert.Empty(t, fixture.apps.listOrgs)
			assert.Empty(t, fixture.records.created)
		})
	}
}

// TestOrganizationByID_OwnTenantSucceeds pins that the authenticated tenant
// keeps every by-ID capability, that writes stay in its own row, and that the
// delete audit carries the real organization id (not the built-in fallback).
func TestOrganizationByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodGet, byIDPath(byIDOwnOrgID, ""), ""))

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Contains(t, recorder.Body.String(), `"displayName":"Own Corp"`)
		assert.Contains(t, recorder.Body.String(), `"masterPasswordMasked"`)
		// Management reads keep the masked credential shape but never the secret.
		assert.NotContains(t, recorder.Body.String(), secretMasterPassword)
		assert.NotContains(t, recorder.Body.String(), secretKerberosKeytab)
		require.Equal(t, []string{byIDOwnOrgID}, fixture.orgs.getCalls)
	})

	t.Run("get-uppercase-own-uuid", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodGet,
			byIDPath(strings.ToUpper(byIDOwnOrgID), ""), ""))

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		// The repository receives the trusted canonical id, not the path value.
		require.Equal(t, []string{byIDOwnOrgID}, fixture.orgs.getCalls)
	})

	t.Run("surface getters", func(t *testing.T) {
		for _, route := range []byIDRoute{
			{name: "get-settings", method: http.MethodGet, suffix: "/settings"},
			{name: "get-theme", method: http.MethodGet, suffix: "/theme"},
			{name: "get-security-policy", method: http.MethodGet, suffix: "/security-policy"},
		} {
			route := route
			t.Run(route.name, func(t *testing.T) {
				fixture := newByIDFixture()
				recorder := serveByID(t, fixture, byIDTenantRequest(route.method, byIDPath(byIDOwnOrgID, route.suffix), ""))
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				assert.NotContains(t, recorder.Body.String(), secretMasterPassword)
			})
		}
	})

	t.Run("patch", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodPatch, byIDPath(byIDOwnOrgID, ""),
			`{"displayName":"renamed","revision":1}`))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, fixture.orgs.updateCalls)
		require.NotEmpty(t, fixture.records.created)
		assert.Equal(t, byIDOwnOrgID, fixture.records.created[0].OrganizationID)
	})

	t.Run("put-settings", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodPut, byIDPath(byIDOwnOrgID, "/settings"), `{}`))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, fixture.orgs.settingsCalls)
	})

	t.Run("put-theme", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodPut,
			byIDPath(byIDOwnOrgID, "/theme"), `{"themeName":"acme"}`))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, fixture.orgs.themeCalls)
	})

	t.Run("put-security-policy", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodPut,
			byIDPath(byIDOwnOrgID, "/security-policy"), `{}`))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, fixture.orgs.policyCalls)
	})

	t.Run("delete", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodDelete, byIDPath(byIDOwnOrgID, ""), ""))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, 1, fixture.orgs.deleteCalls)
		require.Len(t, fixture.records.created, 1)
		assert.Equal(t, byIDOwnOrgID, fixture.records.created[0].OrganizationID,
			"delete audit must carry the deleted organization id")
	})

	t.Run("applications", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodGet, byIDPath(byIDOwnOrgID, "/applications"), ""))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Contains(t, recorder.Body.String(), "app-own")
		assert.NotContains(t, recorder.Body.String(), "app-foreign")
		require.Equal(t, []string{byIDOwnOrgID}, fixture.apps.listOrgs)
	})

	t.Run("default-application", func(t *testing.T) {
		fixture := newByIDFixture()
		recorder := serveByID(t, fixture, byIDTenantRequest(http.MethodGet,
			byIDPath(byIDOwnOrgID, "/default-application"), ""))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Contains(t, recorder.Body.String(), "app-own")
		require.Equal(t, []string{byIDOwnOrgID}, fixture.apps.listOrgs)
	})
}

// TestOrganizationByID_PlatformAdminCrossTenant pins that platform admins keep
// the explicit cross-tenant capability: TrustedOrganizationID honours the
// requested id for super_admin/system tenant scopes.
func TestOrganizationByID_PlatformAdminCrossTenant(t *testing.T) {
	for _, route := range []byIDRoute{
		{name: "get", method: http.MethodGet},
		{name: "patch", method: http.MethodPatch, body: `{"displayName":"renamed","revision":1}`},
		{name: "delete", method: http.MethodDelete},
		{name: "get-applications", method: http.MethodGet, suffix: "/applications"},
		{name: "get-default-application", method: http.MethodGet, suffix: "/default-application"},
	} {
		t.Run(route.name, func(t *testing.T) {
			fixture := newByIDFixture()
			recorder := serveByID(t, fixture, byIDPlatformAdminRequest(route.method,
				byIDPath(byIDForeignOrgID, route.suffix), route.body))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			if route.suffix == "/applications" || route.suffix == "/default-application" {
				require.Equal(t, []string{byIDForeignOrgID}, fixture.apps.listOrgs,
					"platform scope must honour the requested organization id")
				return
			}
			require.NotEmpty(t, fixture.orgs.getCalls)
			assert.Equal(t, byIDForeignOrgID, fixture.orgs.getCalls[0],
				"platform scope must honour the requested organization id")
		})
	}
}
