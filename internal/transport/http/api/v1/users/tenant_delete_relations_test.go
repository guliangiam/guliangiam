package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	delRelSessionID        = "session-1"
	delRelOtherSessionID   = "session-2"
	delRelGroupID          = "group-1"
	delRelOtherGroupID     = "group-2"
	delRelIdentityID       = "identity-1"
	delRelOtherIdentityID  = "identity-2"
	delRelRoleID           = "role-1"
	delRelOtherRoleID      = "role-2"
	delRelConsentID        = "app-1"
	delRelOtherConsentID   = "app-foreign"
	delRelRemainingGroupID = "group-remaining"
	delRelRemainingRoleID  = "role-remaining"
)

type delRelUserRepo struct {
	userdomain.Repository
	user *userdomain.User

	getByIDCalls       int
	scopedGetCalls     int
	scopedOrgs         []string
	updateCalls        int
	updateForOrgCalls  int
	deleteIdentity     int
	replaceGroups      int
	replaceRoles       int
	remainingGroups    []string
	remainingRoles     []string
	deletedIdentityIDs []string
}

func (r *delRelUserRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	r.getByIDCalls++
	if r.user == nil || r.user.ID != id {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *delRelUserRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*userdomain.User, error) {
	r.scopedGetCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if r.user == nil || r.user.ID != id || r.user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *delRelUserRepo) Update(context.Context, *userdomain.User) error {
	r.updateCalls++
	return nil
}

func (r *delRelUserRepo) UpdateForOrganization(_ context.Context, user *userdomain.User, organizationID string) error {
	r.updateForOrgCalls++
	if user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func (r *delRelUserRepo) DeleteIdentity(_ context.Context, userID, identityID string) error {
	r.deleteIdentity++
	r.deletedIdentityIDs = append(r.deletedIdentityIDs, identityID)
	return nil
}

func (r *delRelUserRepo) ReplaceGroupMemberships(_ context.Context, _ string, memberships []userdomain.UserGroupMembership) error {
	r.replaceGroups++
	r.remainingGroups = r.remainingGroups[:0]
	for _, membership := range memberships {
		r.remainingGroups = append(r.remainingGroups, membership.GroupID)
	}
	return nil
}

func (r *delRelUserRepo) ReplaceRoleBindings(_ context.Context, _ string, bindings []userdomain.UserRoleBinding) error {
	r.replaceRoles++
	r.remainingRoles = r.remainingRoles[:0]
	for _, binding := range bindings {
		r.remainingRoles = append(r.remainingRoles, binding.RoleID)
	}
	return nil
}

type delRelSessions struct {
	sessiondomain.Repository
	sessions []*sessiondomain.Session

	listCalls         int
	listQuery         sessiondomain.ListQuery
	getScopedCalls    int
	revokeScopedCalls int
	revokeCalls       int
	revokeByUserCalls int
	revokedIDs        []string
}

func (r *delRelSessions) find(id string) *sessiondomain.Session {
	for _, session := range r.sessions {
		if session.ID == id {
			return session
		}
	}
	return nil
}

func (r *delRelSessions) GetByID(_ context.Context, id string) (*sessiondomain.Session, error) {
	if session := r.find(id); session != nil {
		return session, nil
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *delRelSessions) GetByIDForOrganization(_ context.Context, id, organizationID string) (*sessiondomain.Session, error) {
	r.getScopedCalls++
	session := r.find(id)
	if session == nil || session.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("session not found")
	}
	return session, nil
}

func (r *delRelSessions) List(_ context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	r.listCalls++
	r.listQuery = query
	var result []*sessiondomain.Session
	for _, session := range r.sessions {
		if query.UserID == "" || session.UserID == query.UserID {
			result = append(result, session)
		}
	}
	return result, int64(len(result)), nil
}

func (r *delRelSessions) Revoke(_ context.Context, id string, revokedAt time.Time) error {
	r.revokeCalls++
	r.revokedIDs = append(r.revokedIDs, id)
	if session := r.find(id); session != nil {
		session.RevokedAt = &revokedAt
	}
	return nil
}

func (r *delRelSessions) RevokeForOrganization(_ context.Context, id, organizationID string, revokedAt time.Time) error {
	r.revokeScopedCalls++
	r.revokedIDs = append(r.revokedIDs, id)
	session := r.find(id)
	if session == nil || session.OrganizationID != organizationID {
		return apperrors.NewNotFound("session not found")
	}
	session.RevokedAt = &revokedAt
	return nil
}

func (r *delRelSessions) RevokeByUser(_ context.Context, userID string, revokedAt time.Time) (int64, error) {
	r.revokeByUserCalls++
	var count int64
	for _, session := range r.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = &revokedAt
			count++
		}
	}
	return count, nil
}

type delRelTokens struct {
	tokendomain.Repository
	refreshCalls int
	revokeCalls  int
}

func (r *delRelTokens) RevokeRefreshByUser(context.Context, string, time.Time) (int64, error) {
	r.refreshCalls++
	return 1, nil
}

func (r *delRelTokens) RevokeByQuery(context.Context, tokendomain.RevokeQuery, time.Time) (int64, error) {
	r.revokeCalls++
	return 1, nil
}

type delRelUnitOfWork struct {
	appuow.UnitOfWork
	users    *delRelUserRepo
	sessions *delRelSessions
	tokens   *delRelTokens
	records  *postByIDRecords
	outbox   *postByIDOutbox
}

func (u *delRelUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *delRelUnitOfWork) Users() userdomain.Repository       { return u.users }
func (u *delRelUnitOfWork) Sessions() sessiondomain.Repository { return u.sessions }
func (u *delRelUnitOfWork) Tokens() tokendomain.Repository     { return u.tokens }
func (u *delRelUnitOfWork) Records() recordsdomain.Repository  { return u.records }
func (u *delRelUnitOfWork) Outbox() domainoutbox.Repository    { return u.outbox }

type delRelHarness struct {
	repo     *delRelUserRepo
	sessions *delRelSessions
	tokens   *delRelTokens
	records  *postByIDRecords
	outbox   *postByIDOutbox
	router   chi.Router
}

func newDelRelHarness(ownerOrg string) *delRelHarness {
	h := &delRelHarness{
		repo: &delRelUserRepo{user: delRelTargetUser(ownerOrg)},
		sessions: &delRelSessions{sessions: []*sessiondomain.Session{{
			ID: delRelSessionID, UserID: postByIDUserID, OrganizationID: ownerOrg,
			Username: "target-user", SessionState: "active",
			CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
		}}},
		tokens:  &delRelTokens{},
		records: &postByIDRecords{},
		outbox:  &postByIDOutbox{},
	}
	uow := &delRelUnitOfWork{
		users: h.repo, sessions: h.sessions, tokens: h.tokens,
		records: h.records, outbox: h.outbox,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{
		Service: appusers.Service{
			Clock: postByIDClock{}, UUID: postByIDUUID{}, UOW: uow,
		},
		SessionService: appsessions.Service{Clock: postByIDClock{}, UOW: uow},
	})
	return h
}

func delRelTargetUser(organizationID string) *userdomain.User {
	return &userdomain.User{
		ID: postByIDUserID, OrganizationID: organizationID,
		Username: "target-user", UserType: "normal", Status: "active", Revision: 3,
		GroupMemberships: []userdomain.UserGroupMembership{
			{UserID: postByIDUserID, GroupID: delRelGroupID},
			{UserID: postByIDUserID, GroupID: delRelRemainingGroupID},
		},
		Identities: []userdomain.UserIdentity{
			{ID: delRelIdentityID, UserID: postByIDUserID, ProviderID: "google", Subject: "google-sub"},
		},
		RoleBindings: []userdomain.UserRoleBinding{
			{UserID: postByIDUserID, RoleID: delRelRoleID},
			{UserID: postByIDUserID, RoleID: delRelRemainingRoleID},
		},
		ApplicationScopes: []userdomain.ConsentRecord{
			{Application: delRelConsentID, Scopes: []string{"read"}},
			{Application: "app-remaining", Scopes: []string{"write"}},
		},
	}
}

func delRelPlatformRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		ID: "platform-session", UserID: postByIDCallerID, Username: postByIDCallerName,
		OrganizationID: forgedOrgSessionID, OrganizationSlug: "built-in",
	})
	return req.WithContext(ctx)
}

type delRelRoute struct {
	name            string
	path            string
	otherID         string
	wantJSON        string
	assertOwnWrites func(t *testing.T, h *delRelHarness)
	assertNoWrites  func(t *testing.T, h *delRelHarness)
}

func delRelBasePath() string {
	return "/api/v1/users/" + postByIDUserID
}

var delRelRoutes = []delRelRoute{
	{
		name:     "session",
		path:     delRelBasePath() + "/sessions/" + delRelSessionID,
		otherID:  delRelOtherSessionID,
		wantJSON: `{"status":"revoked","msg":"resource revoked"}`,
		assertOwnWrites: func(t *testing.T, h *delRelHarness) {
			assert.Equal(t, 1, h.sessions.getScopedCalls)
			assert.Equal(t, 1, h.sessions.revokeScopedCalls)
			assert.Equal(t, []string{delRelSessionID}, h.sessions.revokedIDs)
		},
		assertNoWrites: func(t *testing.T, h *delRelHarness) {
			assert.Zero(t, h.sessions.revokeScopedCalls)
			assert.Zero(t, h.sessions.revokeCalls)
		},
	},
	{
		name:     "sessions",
		path:     delRelBasePath() + "/sessions",
		wantJSON: `"status":"revoked"`,
		assertOwnWrites: func(t *testing.T, h *delRelHarness) {
			assert.Equal(t, 1, h.sessions.listCalls)
			assert.Equal(t, forgedOrgSessionID, h.sessions.listQuery.OrganizationID)
			assert.Equal(t, postByIDUserID, h.sessions.listQuery.UserID)
			assert.Equal(t, 1, h.sessions.revokeByUserCalls)
			assert.Equal(t, 1, h.tokens.refreshCalls)
			assert.Equal(t, 1, h.tokens.revokeCalls)
		},
		assertNoWrites: func(t *testing.T, h *delRelHarness) {
			assert.Zero(t, h.sessions.listCalls)
			assert.Zero(t, h.sessions.revokeByUserCalls)
			assert.Zero(t, h.tokens.refreshCalls)
			assert.Zero(t, h.tokens.revokeCalls)
		},
	},
	{
		name:     "consent",
		path:     delRelBasePath() + "/consents/" + delRelConsentID,
		otherID:  delRelOtherConsentID,
		wantJSON: `{"status":"revoked","msg":"resource revoked"}`,
		assertOwnWrites: func(t *testing.T, h *delRelHarness) {
			assert.Equal(t, 1, h.repo.updateForOrgCalls)
			assert.Empty(t, h.repo.updateCalls)
		},
		assertNoWrites: func(t *testing.T, h *delRelHarness) {
			assert.Zero(t, h.repo.updateForOrgCalls)
			assert.Zero(t, h.repo.updateCalls)
		},
	},
	{
		name:     "group",
		path:     delRelBasePath() + "/groups/" + delRelGroupID,
		otherID:  delRelOtherGroupID,
		wantJSON: `{"status":"deleted","msg":"resource deleted"}`,
		assertOwnWrites: func(t *testing.T, h *delRelHarness) {
			assert.Equal(t, 1, h.repo.updateForOrgCalls)
			assert.Equal(t, 1, h.repo.replaceGroups)
			assert.NotContains(t, h.repo.remainingGroups, delRelGroupID)
			assert.Contains(t, h.repo.remainingGroups, delRelRemainingGroupID)
		},
		assertNoWrites: func(t *testing.T, h *delRelHarness) {
			assert.Zero(t, h.repo.updateForOrgCalls)
			assert.Zero(t, h.repo.replaceGroups)
		},
	},
	{
		name:     "identity",
		path:     delRelBasePath() + "/identities/" + delRelIdentityID,
		otherID:  delRelOtherIdentityID,
		wantJSON: `{"status":"deleted","msg":"resource deleted"}`,
		assertOwnWrites: func(t *testing.T, h *delRelHarness) {
			assert.Equal(t, 1, h.repo.deleteIdentity)
			assert.Equal(t, []string{delRelIdentityID}, h.repo.deletedIdentityIDs)
		},
		assertNoWrites: func(t *testing.T, h *delRelHarness) {
			assert.Zero(t, h.repo.deleteIdentity)
		},
	},
	{
		name:     "role-binding",
		path:     delRelBasePath() + "/role-bindings/" + delRelRoleID,
		otherID:  delRelOtherRoleID,
		wantJSON: `{"status":"deleted","msg":"resource deleted"}`,
		assertOwnWrites: func(t *testing.T, h *delRelHarness) {
			assert.Equal(t, 1, h.repo.updateForOrgCalls)
			assert.Equal(t, 1, h.repo.replaceRoles)
			assert.NotContains(t, h.repo.remainingRoles, delRelRoleID)
			assert.Contains(t, h.repo.remainingRoles, delRelRemainingRoleID)
		},
		assertNoWrites: func(t *testing.T, h *delRelHarness) {
			assert.Zero(t, h.repo.updateForOrgCalls)
			assert.Zero(t, h.repo.replaceRoles)
		},
	},
}

func assertDelRelNoAudit(t *testing.T, h *delRelHarness) {
	t.Helper()
	assert.Empty(t, h.records.created)
	assert.Empty(t, h.outbox.enqueued)
}

func delRelCrossTenantPath(tt delRelRoute) string {
	if tt.otherID == "" {
		return tt.path
	}
	switch tt.name {
	case "session":
		return delRelBasePath() + "/sessions/" + tt.otherID
	case "consent":
		return delRelBasePath() + "/consents/" + tt.otherID
	case "group":
		return delRelBasePath() + "/groups/" + tt.otherID
	case "identity":
		return delRelBasePath() + "/identities/" + tt.otherID
	case "role-binding":
		return delRelBasePath() + "/role-bindings/" + tt.otherID
	default:
		return tt.path
	}
}

// TestUserDeleteRelations_ForeignTenantRejected pins that all six DELETE
// routes resolve {id} through the trusted tenant: a foreign user is 404 and no
// write/audit side effect runs.
func TestUserDeleteRelations_ForeignTenantRejected(t *testing.T) {
	for _, tt := range delRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelRelHarness(forgedOrgRequestID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodDelete,
				tt.path+"?organizationId="+forgedOrgRequestID, "", postByIDCallerID))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign user must look missing")
			assert.Zero(t, h.repo.getByIDCalls, "delete routes must never use the system-scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			tt.assertNoWrites(t, h)
			assertDelRelNoAudit(t, h)
		})
	}
}

// TestUserDeleteRelations_WithoutTenantScopeRejected pins the fail-fast: an
// empty trusted tenant is a 400 before any repository read.
func TestUserDeleteRelations_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range delRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelRelHarness(forgedOrgSessionID)

			recorder := byIDServe(h.router, httptest.NewRequest(http.MethodDelete, tt.path, nil))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getByIDCalls)
			assert.Zero(t, h.repo.scopedGetCalls)
			tt.assertNoWrites(t, h)
			assertDelRelNoAudit(t, h)
		})
	}
}

// TestUserDeleteRelations_OwnTenantSucceeds pins the success path, the scoped
// write, and that audit carries the entity organization.
func TestUserDeleteRelations_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range delRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelRelHarness(forgedOrgSessionID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodDelete,
				tt.path+"?organizationId="+forgedOrgRequestID, "", postByIDCallerID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), tt.wantJSON)
			assert.Zero(t, h.repo.getByIDCalls, "management writes must use the scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs)
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}

			tt.assertOwnWrites(t, h)
			require.Len(t, h.records.created, 1)
			assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID,
				"audit must carry the entity organization, never the built-in tenant")
		})
	}
}

// TestUserDeleteRelations_ForeignChildRejected pins the child binding: a
// session/consent/group/identity/role that does not belong to {id} in the same
// tenant stays invisible and is never written.
func TestUserDeleteRelations_ForeignChildRejected(t *testing.T) {
	for _, tt := range delRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			if tt.otherID == "" {
				t.Skip("route has no child id to cross")
			}
			h := newDelRelHarness(forgedOrgSessionID)
			switch tt.name {
			case "session":
				h.sessions.sessions = append(h.sessions.sessions, &sessiondomain.Session{
					ID: delRelOtherSessionID, UserID: postByIDOtherUserID, OrganizationID: forgedOrgSessionID,
					Username: "other-user", SessionState: "active",
					CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
				})
			}

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodDelete,
				delRelCrossTenantPath(tt), "", postByIDCallerID))

			assert.Equal(t, http.StatusNotFound, recorder.Code,
				"a child outside {id} must look missing")
			assert.Equal(t, 1, h.repo.scopedGetCalls, "the parent is resolved before the child check")
			tt.assertNoWrites(t, h)
			assertDelRelNoAudit(t, h)
		})
	}
}

// TestUserDeleteRelations_PlatformAdminCanTargetTenant pins that a platform
// scope keeps the explicit cross-tenant target as the predicate.
func TestUserDeleteRelations_PlatformAdminCanTargetTenant(t *testing.T) {
	for _, tt := range delRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelRelHarness(forgedOrgRequestID)

			recorder := byIDServe(h.router, delRelPlatformRequest(http.MethodDelete,
				tt.path+"?organizationId="+forgedOrgRequestID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.NotEmpty(t, h.repo.scopedOrgs)
			assert.Equal(t, forgedOrgRequestID, h.repo.scopedOrgs[0],
				"platform admin's explicit organization is the predicate")
			require.Len(t, h.records.created, 1)
			assert.Equal(t, forgedOrgRequestID, h.records.created[0].OrganizationID)
		})
	}
}

// Guard compile-time drift.
var (
	_ appuow.UnitOfWork = (*delRelUnitOfWork)(nil)
	_ appuow.AuthUoW    = (*delRelUnitOfWork)(nil)
)
