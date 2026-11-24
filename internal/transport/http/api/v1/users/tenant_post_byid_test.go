package users

import (
	"context"
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
	trusteddevicesdomain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	postByIDUserID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	postByIDCallerID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	postByIDCallerName  = "tenant-admin"
	postByIDGroupID     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	postByIDIdentityID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	postByIDOtherUserID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	postByIDAppID       = "99999999-9999-9999-9999-999999999999"
)

type postByIDClock struct{}

func (postByIDClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type postByIDUUID struct{}

func (postByIDUUID) NewString() (string, error) { return "ffffffff-ffff-ffff-ffff-ffffffffffff", nil }

// postByIDRepo models the pre-fix MySQL repository: GetByID has no tenant
// predicate and returns any row. Every scoped read records the organization it
// was asked for so a forged query parameter is visible in the assertion.
type postByIDRepo struct {
	userdomain.Repository
	user   *userdomain.User
	caller *userdomain.User

	getByIDCalls       int
	scopedGetCalls     int
	scopedOrgs         []string
	updateCalls        int
	updateProfileCalls int
	updateCredentials  int
	deleteIdentity     int
	replaceGroups      int
	remainingGroups    []string
	createdHistory     int
	updatePasswordCred int
}

func (r *postByIDRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	r.getByIDCalls++
	if r.user == nil || r.user.ID != id {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *postByIDRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*userdomain.User, error) {
	r.scopedGetCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if r.user == nil || r.user.ID != id || r.user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *postByIDRepo) GetByUsername(_ context.Context, _ string, username string) (*userdomain.User, error) {
	if r.caller == nil || r.caller.Username != username {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.caller, nil
}

func (r *postByIDRepo) Update(context.Context, *userdomain.User) error {
	r.updateCalls++
	return nil
}

func (r *postByIDRepo) UpdateForOrganization(_ context.Context, user *userdomain.User, organizationID string) error {
	r.updateCalls++
	if user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func (r *postByIDRepo) UpdateProfile(context.Context, *userdomain.UserProfile) error {
	r.updateProfileCalls++
	return nil
}

func (r *postByIDRepo) UpdateCredentials(context.Context, *userdomain.UserCredentials) error {
	r.updateCredentials++
	return nil
}

// UpdatePasswordCredential models the directed credential write used by the
// password-reset path (only password columns, never the access secret).
func (r *postByIDRepo) UpdatePasswordCredential(context.Context, *userdomain.UserCredentials) error {
	r.updatePasswordCred++
	return nil
}

func (r *postByIDRepo) DeleteIdentity(context.Context, string, string) error {
	r.deleteIdentity++
	return nil
}

func (r *postByIDRepo) ReplaceGroupMemberships(_ context.Context, _ string, memberships []userdomain.UserGroupMembership) error {
	r.replaceGroups++
	r.remainingGroups = r.remainingGroups[:0]
	for _, membership := range memberships {
		r.remainingGroups = append(r.remainingGroups, membership.GroupID)
	}
	return nil
}

func (r *postByIDRepo) GetPasswordHistory(context.Context, string, int) ([]userdomain.PasswordHistoryEntry, error) {
	return nil, nil
}

func (r *postByIDRepo) CreatePasswordHistory(context.Context, *userdomain.PasswordHistoryEntry) error {
	r.createdHistory++
	return nil
}

type postByIDOrgRepo struct {
	orgdomain.Repository
	org *orgdomain.Organization
}

func (r *postByIDOrgRepo) GetByID(_ context.Context, id string) (*orgdomain.Organization, error) {
	if r.org == nil || r.org.ID != id {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return r.org, nil
}

type postByIDRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *postByIDRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type postByIDOutbox struct {
	domainoutbox.Repository
	enqueued []*domainoutbox.OutboxEvent
}

func (r *postByIDOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, event)
	return nil
}

type postByIDSessions struct {
	sessiondomain.Repository
	revokeByUserCalls int
	revokeExceptCalls int
}

func (r *postByIDSessions) RevokeByUser(context.Context, string, time.Time) (int64, error) {
	r.revokeByUserCalls++
	return 1, nil
}

func (r *postByIDSessions) RevokeByUserExcept(context.Context, string, string, time.Time) (int64, error) {
	r.revokeExceptCalls++
	return 1, nil
}

type postByIDTokens struct {
	tokendomain.Repository
	createCalls   int
	createdTokens []*tokendomain.Token
	revokeCalls   int
	revokeQueries []tokendomain.RevokeQuery
	refreshCalls  int
}

func (r *postByIDTokens) Create(_ context.Context, token *tokendomain.Token) error {
	r.createCalls++
	r.createdTokens = append(r.createdTokens, token)
	return nil
}

func (r *postByIDTokens) RevokeByQuery(_ context.Context, query tokendomain.RevokeQuery, _ time.Time) (int64, error) {
	r.revokeCalls++
	r.revokeQueries = append(r.revokeQueries, query)
	return 1, nil
}

func (r *postByIDTokens) RevokeRefreshByUser(context.Context, string, time.Time) (int64, error) {
	r.refreshCalls++
	return 1, nil
}

type postByIDPlatformAdmins struct {
	platformadmins.PlatformAdminRepository
	admin  *platformadmins.PlatformAdmin
	scopes []*platformadmins.PlatformAdminScope
}

func (r *postByIDPlatformAdmins) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	if r.admin == nil {
		return nil, apperrors.NewNotFound("platform admin not found")
	}
	return r.admin, nil
}

func (r *postByIDPlatformAdmins) ListByPlatformAdmin(context.Context, string, platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return r.scopes, nil
}

type postByIDTrustedDevices struct {
	trusteddevicesdomain.Repository
	revokeByUserCalls int
}

func (r *postByIDTrustedDevices) RevokeByUser(context.Context, string, string, time.Time) (int64, error) {
	r.revokeByUserCalls++
	return 0, nil
}

type postByIDPlatformAdminScopes struct {
	platformadmins.PlatformAdminScopeRepository
}

func (postByIDPlatformAdminScopes) ListByPlatformAdmin(context.Context, string, platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return nil, nil
}

type postByIDUnitOfWork struct {
	appuow.UnitOfWork
	repo     *postByIDRepo
	orgs     *postByIDOrgRepo
	records  *postByIDRecords
	outbox   *postByIDOutbox
	sessions *postByIDSessions
	tokens   *postByIDTokens
	admins   *postByIDPlatformAdmins
	devices  *postByIDTrustedDevices
}

func (u *postByIDUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *postByIDUnitOfWork) Users() userdomain.Repository        { return u.repo }
func (u *postByIDUnitOfWork) Organizations() orgdomain.Repository { return u.orgs }
func (u *postByIDUnitOfWork) Records() recordsdomain.Repository   { return u.records }
func (u *postByIDUnitOfWork) Outbox() domainoutbox.Repository     { return u.outbox }
func (u *postByIDUnitOfWork) Sessions() sessiondomain.Repository  { return u.sessions }
func (u *postByIDUnitOfWork) Tokens() tokendomain.Repository      { return u.tokens }
func (u *postByIDUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository {
	return u.admins
}
func (u *postByIDUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return postByIDPlatformAdminScopes{}
}
func (u *postByIDUnitOfWork) TrustedDevices() trusteddevicesdomain.Repository { return u.devices }

type postByIDHarness struct {
	repo     *postByIDRepo
	orgs     *postByIDOrgRepo
	records  *postByIDRecords
	outbox   *postByIDOutbox
	sessions *postByIDSessions
	tokens   *postByIDTokens
	admins   *postByIDPlatformAdmins
	devices  *postByIDTrustedDevices
	router   chi.Router
}

func newPostByIDHarness(userOrg string) *postByIDHarness {
	h := &postByIDHarness{
		repo: &postByIDRepo{
			user:   postByIDTargetUser(userOrg),
			caller: &userdomain.User{ID: postByIDCallerID, OrganizationID: forgedOrgSessionID, Username: postByIDCallerName, IsAdmin: true},
		},
		orgs:     &postByIDOrgRepo{org: &orgdomain.Organization{ID: userOrg, Slug: "tenant-a", DisplayName: "Tenant A"}},
		records:  &postByIDRecords{},
		outbox:   &postByIDOutbox{},
		sessions: &postByIDSessions{},
		tokens:   &postByIDTokens{},
		admins:   &postByIDPlatformAdmins{},
		devices:  &postByIDTrustedDevices{},
	}
	uow := &postByIDUnitOfWork{
		repo: h.repo, orgs: h.orgs, records: h.records,
		outbox: h.outbox, sessions: h.sessions, tokens: h.tokens,
		admins: h.admins, devices: h.devices,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: appusers.Service{
		Clock: postByIDClock{}, UUID: postByIDUUID{}, UOW: uow,
		IdentityVerification: postByIDIDVGateway{},
	}})
	return h
}

func postByIDTargetUser(organizationID string) *userdomain.User {
	return &userdomain.User{
		ID:                  postByIDUserID,
		OrganizationID:      organizationID,
		Username:            "target-user",
		UserType:            "normal",
		Status:              "active",
		Revision:            1,
		SignupApplicationID: postByIDAppID,
		Profile: userdomain.UserProfile{
			UserID:      postByIDUserID,
			DisplayName: "Target User",
			Email:       "alice@example.com",
			Phone:       "13800138000",
			CountryCode: "CN",
			RealName:    "Zhang San",
			IDCardType:  "ID card",
			IDCard:      "110101199001011234",
		},
		Credentials: userdomain.UserCredentials{
			UserID:       postByIDUserID,
			HasPassword:  true,
			PasswordHash: "old-password-hash",
			PasswordAlgo: "argon2id",
		},
		GroupMemberships: []userdomain.UserGroupMembership{
			{UserID: postByIDUserID, GroupID: postByIDGroupID},
			{UserID: postByIDUserID, GroupID: "other-group"},
		},
		Identities: []userdomain.UserIdentity{
			{ID: postByIDIdentityID, UserID: postByIDUserID, ProviderID: "google", Subject: "google-sub"},
		},
	}
}

// postByIDRequest builds a tenant-scoped request. sessionUserID lets the
// exit-impersonate cases model "the caller is operating as {id}" or not.
func postByIDRequest(method, target, body, sessionUserID string) *http.Request {
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
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "tenant-a", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		ID: "session-id", UserID: sessionUserID, Username: postByIDCallerName,
		OrganizationID: forgedOrgSessionID, OrganizationSlug: "tenant-a",
	})
	return req.WithContext(ctx)
}

func postByIDPlatformRequest(target, body string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(http.MethodPost, target, nil)
	}
	ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		ID: "platform-session", UserID: postByIDCallerID, Username: postByIDCallerName,
		OrganizationID: forgedOrgSessionID, OrganizationSlug: "built-in",
	})
	return req.WithContext(ctx)
}

func postByIDServe(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// postByIDRoute is the batch-1 surface. sessionUserID is the identity the
// session is operating as; only exit-impersonate compares it with {id}.
// audit is the number of audit records the existing use case embeds; none of
// the eight routes gained or lost audit semantics in this change.
type postByIDRoute struct {
	name             string
	path             string
	body             string
	sessionUserID    string
	wantWrite        func(t *testing.T, h *postByIDHarness)
	wantAudit        int
	wantBodyContains string
}

var postByIDRoutes = []postByIDRoute{
	{
		name:          "remove-from-group",
		path:          "/api/v1/users/" + postByIDUserID + "/remove-from-group",
		body:          `{"groupId":"` + postByIDGroupID + `"}`,
		sessionUserID: postByIDCallerID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			assert.Equal(t, 1, h.repo.replaceGroups, "own tenant must replace memberships once")
			assert.NotContains(t, h.repo.remainingGroups, postByIDGroupID, "the target group must be removed")
			assert.Contains(t, h.repo.remainingGroups, "other-group")
		},
		wantAudit: 1,
	},
	{
		name:          "keys",
		path:          "/api/v1/users/" + postByIDUserID + "/keys",
		sessionUserID: postByIDCallerID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			assert.Equal(t, 1, h.repo.updateCalls, "own tenant must persist the user once")
			assert.Equal(t, 1, h.repo.updateCredentials, "own tenant must persist credentials once")
			// Access Secret 轮换带 30 分钟宽限期（access_secret_hash_prev），
			// 语义上不再级联撤销会话与访问令牌；撤销仍由密码变更路径负责。
			assert.Zero(t, h.sessions.revokeByUserCalls, "rotation with grace must not revoke sessions")
			assert.Zero(t, h.tokens.revokeCalls, "rotation with grace must not revoke access tokens")
		},
		wantAudit:        1,
		wantBodyContains: `"accessSecret"`,
	},
	{
		name:          "unlink",
		path:          "/api/v1/users/" + postByIDUserID + "/unlink",
		body:          `{"identityId":"` + postByIDIdentityID + `"}`,
		sessionUserID: postByIDCallerID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			assert.Equal(t, 1, h.repo.deleteIdentity, "own tenant must delete the identity once")
		},
	},
	{
		name:          "impersonate",
		path:          "/api/v1/users/" + postByIDUserID + "/impersonate",
		sessionUserID: postByIDCallerID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			assert.Equal(t, 1, h.tokens.createCalls, "own tenant must mint one impersonation token")
			require.Len(t, h.tokens.createdTokens, 1)
			assert.Equal(t, postByIDAppID, h.tokens.createdTokens[0].ApplicationID,
				"impersonation token must bind the target's signup application")
		},
		wantAudit:        1,
		wantBodyContains: `"token"`,
	},
	{
		name:          "exit-impersonate",
		path:          "/api/v1/users/" + postByIDUserID + "/exit-impersonate",
		sessionUserID: postByIDUserID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			require.Len(t, h.tokens.revokeQueries, 1)
			assert.Equal(t, forgedOrgSessionID, h.tokens.revokeQueries[0].OrganizationID)
			assert.Equal(t, postByIDUserID, h.tokens.revokeQueries[0].UserID)
			assert.Equal(t, "imp", h.tokens.revokeQueries[0].TokenPrefix,
				"exit-impersonate must only revoke impersonation tokens")
		},
		wantAudit: 1,
	},
	{
		name:          "verify-identification",
		path:          "/api/v1/users/" + postByIDUserID + "/verify-identification",
		sessionUserID: postByIDCallerID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			// Real verification only starts a provider session; the verified flag
			// is flipped after the provider approves, never self-attested.
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.updateProfileCalls)
		},
		wantAudit:        0,
		wantBodyContains: `"status":"pending"`,
	},
	{
		name:          "set-password",
		path:          "/api/v1/users/" + postByIDUserID + "/set-password",
		body:          `{"password":"Str0ng!Passw0rd"}`,
		sessionUserID: postByIDCallerID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			assert.Equal(t, 1, h.repo.updateCalls, "own tenant must persist the user once")
			assert.Equal(t, 1, h.repo.updatePasswordCred, "reset must write the password columns once")
			assert.Equal(t, 1, h.repo.createdHistory, "previous password must be archived once")
			assert.Equal(t, 1, h.sessions.revokeByUserCalls, "password reset cascades sessions")
			assert.Equal(t, 1, h.tokens.revokeCalls, "password reset cascades access tokens")
			assert.Equal(t, 1, h.devices.revokeByUserCalls, "password reset cascades trusted devices")
		},
		wantAudit: 1,
	},
	{
		name:          "mfa-setup-initiate",
		path:          "/api/v1/users/" + postByIDUserID + "/mfa-setup/initiate",
		body:          `{"mfaType":"totp"}`,
		sessionUserID: postByIDCallerID,
		wantWrite: func(t *testing.T, h *postByIDHarness) {
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.updateCredentials)
		},
		wantBodyContains: `"secret"`,
	},
}

// TestUserPostByID_ForeignTenantRejected pins that every batch-1 POST route
// resolves {id} through the trusted tenant: a foreign user is 404, the forged
// query organizationId is ignored, and no write/audit/outbox side effect runs.
func TestUserPostByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range postByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPostByIDHarness(forgedOrgRequestID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodPost,
				tt.path+"?organizationId="+forgedOrgRequestID, tt.body, tt.sessionUserID))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign user must look missing")
			assert.NotContains(t, recorder.Body.String(), "alice@example.com")
			assert.Zero(t, h.repo.getByIDCalls, "the by-id POST routes must never use the system-scoped read")
			assert.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.updateProfileCalls)
			assert.Zero(t, h.repo.updateCredentials)
			assert.Zero(t, h.repo.deleteIdentity)
			assert.Zero(t, h.repo.replaceGroups)
			assert.Zero(t, h.tokens.createCalls)
			assert.Zero(t, h.tokens.revokeCalls)
			assert.Zero(t, h.sessions.revokeByUserCalls)
			assert.Empty(t, h.records.created, "foreign tenant must not produce an audit event")
			assert.Empty(t, h.outbox.enqueued, "foreign tenant must not produce an outbox event")
		})
	}
}

// TestUserPostByID_WithoutTenantScopeRejected pins the fail-fast: no trusted
// organization means 400 before the repository is touched.
func TestUserPostByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range postByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPostByIDHarness(forgedOrgSessionID)

			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodPost, tt.path, nil)
			}
			recorder := postByIDServe(h.router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getByIDCalls)
			assert.Zero(t, h.repo.scopedGetCalls)
			assert.Zero(t, h.repo.updateCalls)
			assert.Empty(t, h.records.created)
			assert.Empty(t, h.outbox.enqueued)
		})
	}
}

// TestUserPostByID_OwnTenantSucceeds pins that the eight routes still work
// inside the authenticated tenant even when the query names another tenant,
// and reports the write/audit counts each use case actually performs.
func TestUserPostByID_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range postByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPostByIDHarness(forgedOrgSessionID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodPost,
				tt.path+"?organizationId="+forgedOrgRequestID, tt.body, tt.sessionUserID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Zero(t, h.repo.getByIDCalls, "management writes must use the scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs)
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			tt.wantWrite(t, h)
			require.Len(t, h.records.created, tt.wantAudit)
			require.Len(t, h.outbox.enqueued, tt.wantAudit)
			if tt.wantAudit == 1 {
				assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID,
					"audit must carry the entity organization, never the forged value")
			}
			if tt.wantBodyContains != "" {
				assert.Contains(t, recorder.Body.String(), tt.wantBodyContains)
			}
		})
	}
}

// TestUserPostByID_ExitImpersonateRejectsForeignIdentity pins the session
// binding: {id} must be the identity the caller is currently operating as.
func TestUserPostByID_ExitImpersonateRejectsForeignIdentity(t *testing.T) {
	h := newPostByIDHarness(forgedOrgSessionID)

	recorder := postByIDServe(h.router, postByIDRequest(http.MethodPost,
		"/api/v1/users/"+postByIDUserID+"/exit-impersonate", "", postByIDCallerID))

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Zero(t, h.repo.scopedGetCalls, "a mismatched identity must be rejected before any lookup")
	assert.Zero(t, h.tokens.revokeCalls)
	assert.Empty(t, h.records.created)
}

// TestUserPostByID_ExitImpersonateWithoutSessionIdentityRejected pins the
// fail-closed path when the session has no user id.
func TestUserPostByID_ExitImpersonateWithoutSessionIdentityRejected(t *testing.T) {
	h := newPostByIDHarness(forgedOrgSessionID)

	recorder := postByIDServe(h.router, postByIDRequest(http.MethodPost,
		"/api/v1/users/"+postByIDUserID+"/exit-impersonate", "", ""))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Zero(t, h.repo.scopedGetCalls)
	assert.Zero(t, h.tokens.revokeCalls)
}

// TestUserPostByID_PlatformAdminImpersonateExplicitTenant pins the documented
// exception: a platform scope may target an explicitly requested organization,
// and the target user is still resolved through that organization.
func TestUserPostByID_PlatformAdminImpersonateExplicitTenant(t *testing.T) {
	h := newPostByIDHarness(forgedOrgRequestID)

	recorder := postByIDServe(h.router, postByIDPlatformRequest(
		"/api/v1/users/"+postByIDUserID+"/impersonate?organizationId="+forgedOrgRequestID, ""))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotEmpty(t, h.repo.scopedOrgs)
	assert.Equal(t, forgedOrgRequestID, h.repo.scopedOrgs[0], "platform admin's explicit organization is the predicate")
	assert.Equal(t, 1, h.tokens.createCalls)
	require.Len(t, h.records.created, 1)
	assert.Equal(t, forgedOrgRequestID, h.records.created[0].OrganizationID)
}

// postByIDIDVGateway stands in for the real identity-verification service so
// the tenant-isolation assertion exercises the route, not a network provider.
type postByIDIDVGateway struct{}

func (postByIDIDVGateway) InitiateUserVerification(_ context.Context, request appusers.IdentityVerificationRequest) (appusers.IdentityVerificationTicket, error) {
	return appusers.IdentityVerificationTicket{
		VerificationID: "verification-test",
		Provider:       "jumio",
		RedirectURL:    "https://provider.test/flow",
	}, nil
}
