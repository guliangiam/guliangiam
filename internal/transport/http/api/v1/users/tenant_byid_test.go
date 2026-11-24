package users

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	byIDUserID = "44444444-4444-4444-4444-444444444444"
	byIDMfaID  = "55555555-5555-5555-5555-555555555555"
)

type byIDClock struct{}

func (byIDClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type byIDUUID struct{}

func (byIDUUID) NewString() (string, error) { return "66666666-6666-6666-6666-666666666666", nil }

// byIDRepo is a tenant-aware in-memory repository. A scoped read only matches
// the owning organization; a foreign user behaves exactly like a missing one.
type byIDRepo struct {
	userdomain.Repository
	user                   *userdomain.User
	scopedOrgs             []string
	scopedGetCalls         int
	updateCalls            int
	updateProfileCalls     int
	updateCredentialsCalls int
	replaceMfaCalls        int
}

// GetByID models the pre-fix system-scoped SQL (`WHERE id = $1`): it returns
// the row regardless of tenant, so the baseline leaks a foreign user.
func (r *byIDRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *byIDRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*userdomain.User, error) {
	r.scopedGetCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if r.user == nil || r.user.ID != id || r.user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *byIDRepo) Update(ctx context.Context, user *userdomain.User) error {
	r.updateCalls++
	return nil
}

func (r *byIDRepo) UpdateForOrganization(_ context.Context, user *userdomain.User, organizationID string) error {
	r.updateCalls++
	if user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func (r *byIDRepo) UpdateProfile(context.Context, *userdomain.UserProfile) error {
	r.updateProfileCalls++
	return nil
}

func (r *byIDRepo) UpdateCredentials(context.Context, *userdomain.UserCredentials) error {
	r.updateCredentialsCalls++
	return nil
}

func (r *byIDRepo) ReplaceMfaMethods(context.Context, string, []userdomain.UserMfaMethod) error {
	r.replaceMfaCalls++
	return nil
}

type byIDOrgRepo struct {
	orgdomain.Repository
	org *orgdomain.Organization
}

func (r *byIDOrgRepo) GetByID(_ context.Context, id string) (*orgdomain.Organization, error) {
	if r.org == nil || r.org.ID != id {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return r.org, nil
}

type byIDRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *byIDRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type byIDOutbox struct {
	domainoutbox.Repository
	enqueued []*domainoutbox.OutboxEvent
}

func (r *byIDOutbox) Enqueue(_ context.Context, item *domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, item)
	return nil
}

type byIDSessions struct {
	sessiondomain.Repository
	revokeCalls int
}

func (r *byIDSessions) RevokeByUserExcept(context.Context, string, string, time.Time) (int64, error) {
	r.revokeCalls++
	return 0, nil
}

type byIDTokens struct {
	tokendomain.Repository
	revokeCalls int
}

func (r *byIDTokens) RevokeByQuery(context.Context, tokendomain.RevokeQuery, time.Time) (int64, error) {
	r.revokeCalls++
	return 0, nil
}

type byIDUnitOfWork struct {
	appuow.UnitOfWork
	repo     *byIDRepo
	orgs     *byIDOrgRepo
	records  *byIDRecords
	outbox   *byIDOutbox
	sessions *byIDSessions
	tokens   *byIDTokens
}

func (u *byIDUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *byIDUnitOfWork) Users() userdomain.Repository        { return u.repo }
func (u *byIDUnitOfWork) Organizations() orgdomain.Repository { return u.orgs }
func (u *byIDUnitOfWork) Records() recordsdomain.Repository   { return u.records }
func (u *byIDUnitOfWork) Outbox() domainoutbox.Repository     { return u.outbox }
func (u *byIDUnitOfWork) Sessions() sessiondomain.Repository  { return u.sessions }
func (u *byIDUnitOfWork) Tokens() tokendomain.Repository      { return u.tokens }

// GetUser 回填 webauthnCredentialCount；零值 stub 返回空列表。
func (u *byIDUnitOfWork) WebAuthnCredentials() webauthndomain.Repository { return &delGetWebAuthn{} }

// byIDPlatformAdmins 让 DeleteUser 的平台管理员守卫放行：目标非平台管理员
// （GetByUserID 返回 nil, nil）。
type byIDPlatformAdmins struct {
	platformadmins.PlatformAdminRepository
}

func (byIDPlatformAdmins) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return nil, nil
}

func (u *byIDUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository {
	return byIDPlatformAdmins{}
}

type byIDHarness struct {
	repo     *byIDRepo
	orgs     *byIDOrgRepo
	records  *byIDRecords
	outbox   *byIDOutbox
	sessions *byIDSessions
	tokens   *byIDTokens
	router   chi.Router
}

func newByIDHarness(ownerOrg string) *byIDHarness {
	h := &byIDHarness{
		repo:     &byIDRepo{user: byIDTenantUser(ownerOrg)},
		orgs:     &byIDOrgRepo{org: &orgdomain.Organization{ID: ownerOrg, Slug: "tenant-a"}},
		records:  &byIDRecords{},
		outbox:   &byIDOutbox{},
		sessions: &byIDSessions{},
		tokens:   &byIDTokens{},
	}
	uow := &byIDUnitOfWork{
		repo: h.repo, orgs: h.orgs, records: h.records,
		outbox: h.outbox, sessions: h.sessions, tokens: h.tokens,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: appusers.Service{
		Clock: byIDClock{}, UUID: byIDUUID{}, UOW: uow,
	}})
	return h
}

func byIDTenantUser(organizationID string) *userdomain.User {
	return &userdomain.User{
		ID: byIDUserID, OrganizationID: organizationID,
		Username: "tenant-user", UserType: "normal", Status: "active", Revision: 1,
		PreferredMfaType: "totp",
		MfaMethods: []userdomain.UserMfaMethod{
			{ID: byIDMfaID, UserID: byIDUserID, MethodType: "totp", Enabled: true, Verified: true},
		},
		Profile: userdomain.UserProfile{
			UserID: byIDUserID, DisplayName: "Tenant User",
			Email: "alice@example.com", Phone: "13800138000",
		},
	}
}

func byIDRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "77777777-7777-7777-7777-777777777777", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

func byIDSuperAdminRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func byIDServe(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

var byIDRoutes = []struct {
	name          string
	method        string
	path          string
	body          string
	write         bool
	auditExpected int
}{
	{"get", http.MethodGet, "/api/v1/users/" + byIDUserID, "", false, 0},
	{"patch", http.MethodPatch, "/api/v1/users/" + byIDUserID, `{"tag":"hijacked","revision":1}`, true, 1},
	{"delete", http.MethodDelete, "/api/v1/users/" + byIDUserID, "", true, 1},
	{"profile", http.MethodPut, "/api/v1/users/" + byIDUserID + "/profile", `{"displayName":"hijacked"}`, true, 1},
	{"credentials", http.MethodPut, "/api/v1/users/" + byIDUserID + "/credentials", `{"needUpdatePassword":false}`, true, 1},
	{"preferred-mfa", http.MethodPut, "/api/v1/users/" + byIDUserID + "/preferred-mfa", `{"mfaType":"totp","revision":1}`, true, 1},
	{"mfa-methods", http.MethodPut, "/api/v1/users/" + byIDUserID + "/mfa-methods", `{"revision":1,"items":[{"id":"` + byIDMfaID + `","methodType":"totp","enabled":true}]}`, true, 1},
	{"masked-contact", http.MethodGet, "/api/v1/users/" + byIDUserID + "/masked-contact", "", false, 0},
}

// TestUserByID_ForeignTenantRejected pins that every core by-ID route resolves
// the authenticated tenant, not the caller-supplied organizationId, and that a
// foreign user is indistinguishable from a missing one: 404, no write, no audit.
func TestUserByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range byIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newByIDHarness(forgedOrgRequestID)

			recorder := byIDServe(h.router, byIDRequest(tt.method, tt.path+"?organizationId="+forgedOrgRequestID, tt.body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign user must look missing")
			assert.NotContains(t, recorder.Body.String(), "alice@example.com")
			assert.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.repo.updateCalls, "foreign tenant must not write the user")
			assert.Zero(t, h.repo.updateProfileCalls, "foreign tenant must not write the profile")
			assert.Zero(t, h.repo.updateCredentialsCalls, "foreign tenant must not write credentials")
			assert.Zero(t, h.repo.replaceMfaCalls, "foreign tenant must not replace MFA methods")
			assert.Empty(t, h.records.created, "foreign tenant must not produce an audit event")
			assert.Empty(t, h.outbox.enqueued, "foreign tenant must not produce an outbox event")
			assert.Zero(t, h.sessions.revokeCalls)
			assert.Zero(t, h.tokens.revokeCalls)
		})
	}
}

// TestUserByID_WithoutTenantScopeRejected pins the fail-fast: when neither the
// session nor a platform-admin target provides a tenant, every route is rejected
// before touching the repository.
func TestUserByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range byIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newByIDHarness(forgedOrgSessionID)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := byIDServe(h.router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.scopedGetCalls)
			assert.Zero(t, h.repo.updateCalls)
			assert.Empty(t, h.records.created)
			assert.Empty(t, h.outbox.enqueued)
		})
	}
}

// TestUserByID_OwnTenantSucceeds pins that the eight routes still work inside
// the authenticated tenant even when the query tries to name another tenant,
// and that every write audits the real organization.
func TestUserByID_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range byIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newByIDHarness(forgedOrgSessionID)

			recorder := byIDServe(h.router, byIDRequest(tt.method, tt.path+"?organizationId="+forgedOrgRequestID, tt.body))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.NotEmpty(t, h.repo.scopedOrgs)
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			if tt.write {
				assert.Equal(t, 1, h.repo.updateCalls, "own tenant must persist the user once")
			} else {
				assert.Zero(t, h.repo.updateCalls)
			}
			require.Len(t, h.records.created, tt.auditExpected)
			require.Len(t, h.outbox.enqueued, tt.auditExpected)
			if tt.auditExpected == 1 {
				assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID,
					"audit must carry the entity organization, never the built-in UUID")
			}

			switch tt.name {
			case "profile":
				assert.Equal(t, 1, h.repo.updateProfileCalls)
			case "credentials":
				assert.Equal(t, 1, h.repo.updateCredentialsCalls)
			case "mfa-methods":
				assert.Equal(t, 1, h.repo.replaceMfaCalls)
			case "delete":
				assert.Equal(t, 1, h.sessions.revokeCalls)
				assert.Equal(t, 1, h.tokens.revokeCalls)
			case "masked-contact":
				assert.Contains(t, recorder.Body.String(), "a***@example.com")
				assert.NotContains(t, recorder.Body.String(), "alice@example.com")
			}
		})
	}
}

// TestUserByID_PlatformAdminCanTargetTenant pins that a platform scope keeps the
// explicit cross-tenant target; the requested organization becomes the predicate.
func TestUserByID_PlatformAdminCanTargetTenant(t *testing.T) {
	h := newByIDHarness(forgedOrgRequestID)

	recorder := byIDServe(h.router, byIDSuperAdminRequest(http.MethodGet,
		"/api/v1/users/"+byIDUserID+"?organizationId="+forgedOrgRequestID))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotEmpty(t, h.repo.scopedOrgs)
	assert.Equal(t, forgedOrgRequestID, h.repo.scopedOrgs[0])
}

// TestUserDetailRead_ExposesContactVerifiedFlags pins that GET /users/{id}
// surfaces the profile verification flags under `profile` so the detail page
// can render verified/unverified without a second round-trip.
func TestUserDetailRead_ExposesContactVerifiedFlags(t *testing.T) {
	h := newByIDHarness(forgedOrgSessionID)
	h.repo.user.Profile.EmailVerified = true
	h.repo.user.Profile.PhoneVerified = false

	recorder := byIDServe(h.router, byIDRequest(http.MethodGet, "/api/v1/users/"+byIDUserID, ""))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var payload struct {
		Profile map[string]any `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, true, payload.Profile["emailVerified"])
	assert.Equal(t, false, payload.Profile["phoneVerified"])
}

// TestUserDetailRead_IsNoStore pins that GET /users/{id} never falls through to
// the ETag middleware's `private, no-cache` branch: the payload exposes masked
// credential placeholders and must reflect a save immediately.
func TestUserDetailRead_IsNoStore(t *testing.T) {
	h := newByIDHarness(forgedOrgSessionID)
	handler := httpmw.ETagCacheMiddleware(httpmw.DefaultETagConfig())(h.router)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, byIDRequest(http.MethodGet, "/api/v1/users/"+byIDUserID, ""))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Empty(t, recorder.Header().Get("ETag"))
}
