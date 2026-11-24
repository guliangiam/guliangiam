package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	putRelRoleID          = "role-1"
	putRelOtherRoleID     = "role-foreign"
	putRelGroupID         = "group-1"
	putRelOtherGroupID    = "group-foreign"
	putRelIdentityID      = "identity-1"
	putRelOtherIdentityID = "identity-foreign"
)

type putRelUserRepo struct {
	userdomain.Repository
	user *userdomain.User

	getByIDCalls      int
	scopedGetCalls    int
	scopedOrgs        []string
	updateCalls       int
	updateForOrgCalls int
	replaceRoles      int
	replaceGroups     int
	replaceIdentities int
	roleIDs           []string
	groupIDs          []string
	identityIDs       []string
}

func (r *putRelUserRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	r.getByIDCalls++
	if r.user == nil || r.user.ID != id {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *putRelUserRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*userdomain.User, error) {
	r.scopedGetCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if r.user == nil || r.user.ID != id || r.user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *putRelUserRepo) Update(context.Context, *userdomain.User) error {
	r.updateCalls++
	return nil
}

func (r *putRelUserRepo) UpdateForOrganization(_ context.Context, user *userdomain.User, organizationID string) error {
	r.updateForOrgCalls++
	if user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func (r *putRelUserRepo) ReplaceRoleBindings(_ context.Context, _ string, bindings []userdomain.UserRoleBinding) error {
	r.replaceRoles++
	r.roleIDs = r.roleIDs[:0]
	for _, binding := range bindings {
		r.roleIDs = append(r.roleIDs, binding.RoleID)
	}
	return nil
}

func (r *putRelUserRepo) ReplaceGroupMemberships(_ context.Context, _ string, memberships []userdomain.UserGroupMembership) error {
	r.replaceGroups++
	r.groupIDs = r.groupIDs[:0]
	for _, membership := range memberships {
		r.groupIDs = append(r.groupIDs, membership.GroupID)
	}
	return nil
}

func (r *putRelUserRepo) ReplaceIdentities(_ context.Context, _ string, identities []userdomain.UserIdentity) error {
	r.replaceIdentities++
	r.identityIDs = r.identityIDs[:0]
	for _, identity := range identities {
		r.identityIDs = append(r.identityIDs, identity.ID)
	}
	return nil
}

type putRelRoles struct {
	roledomain.Repository
	organizationID string
	scopedCalls    int
	scopedOrgs     []string
	scopedIDs      []string
}

func (r *putRelRoles) GetByIDForOrganization(_ context.Context, id, organizationID string) (*roledomain.Role, error) {
	r.scopedCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	r.scopedIDs = append(r.scopedIDs, id)
	if id != putRelRoleID || organizationID != r.organizationID {
		return nil, apperrors.NewNotFound("role not found")
	}
	return &roledomain.Role{ID: id, OrganizationID: organizationID}, nil
}

type putRelGroups struct {
	groupsdomain.Repository
	organizationID string
	scopedCalls    int
	scopedOrgs     []string
	scopedIDs      []string
}

func (r *putRelGroups) GetByIDForOrganization(_ context.Context, id, organizationID string) (*groupsdomain.Group, error) {
	r.scopedCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	r.scopedIDs = append(r.scopedIDs, id)
	if id != putRelGroupID || organizationID != r.organizationID {
		return nil, apperrors.NewNotFound("group not found")
	}
	return &groupsdomain.Group{ID: id, OrganizationID: organizationID}, nil
}

type putRelUnitOfWork struct {
	appuow.UnitOfWork
	users   *putRelUserRepo
	roles   *putRelRoles
	groups  *putRelGroups
	records *postByIDRecords
	outbox  *postByIDOutbox
}

func (u *putRelUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *putRelUnitOfWork) Users() userdomain.Repository      { return u.users }
func (u *putRelUnitOfWork) Roles() roledomain.Repository      { return u.roles }
func (u *putRelUnitOfWork) Groups() groupsdomain.Repository   { return u.groups }
func (u *putRelUnitOfWork) Records() recordsdomain.Repository { return u.records }
func (u *putRelUnitOfWork) Outbox() domainoutbox.Repository   { return u.outbox }

type putRelHarness struct {
	repo    *putRelUserRepo
	roles   *putRelRoles
	groups  *putRelGroups
	records *postByIDRecords
	outbox  *postByIDOutbox
	router  chi.Router
}

func newPutRelHarness(ownerOrg string) *putRelHarness {
	user := postByIDTargetUser(ownerOrg)
	user.Identities = []userdomain.UserIdentity{{
		ID: putRelIdentityID, UserID: postByIDUserID, ProviderID: "google", Subject: "sub-1",
	}}
	h := &putRelHarness{
		repo:    &putRelUserRepo{user: user},
		roles:   &putRelRoles{organizationID: ownerOrg},
		groups:  &putRelGroups{organizationID: ownerOrg},
		records: &postByIDRecords{},
		outbox:  &postByIDOutbox{},
	}
	uow := &putRelUnitOfWork{
		users: h.repo, roles: h.roles, groups: h.groups,
		records: h.records, outbox: h.outbox,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: appusers.Service{
		Clock: postByIDClock{}, UUID: postByIDUUID{}, UOW: uow,
	}})
	return h
}

func putRelPlatformRequest(target, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPut, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		ID: "platform-session", UserID: postByIDCallerID, Username: postByIDCallerName,
		OrganizationID: forgedOrgSessionID, OrganizationSlug: "built-in",
	})
	return req.WithContext(ctx)
}

type putRelRoute struct {
	name              string
	path              string
	body              string
	otherBody         string
	checkOwn          func(t *testing.T, h *putRelHarness)
	checkForeignChild func(t *testing.T, h *putRelHarness)
	checkNoWrites     func(t *testing.T, h *putRelHarness)
}

func putRelBasePath() string {
	return "/api/v1/users/" + postByIDUserID
}

var putRelRoutes = []putRelRoute{
	{
		name:      "role-bindings",
		path:      putRelBasePath() + "/role-bindings",
		body:      `{"items":[{"roleId":"` + putRelRoleID + `"}]}`,
		otherBody: `{"items":[{"roleId":"` + putRelRoleID + `"},{"roleId":"` + putRelOtherRoleID + `"}]}`,
		checkOwn: func(t *testing.T, h *putRelHarness) {
			assert.Equal(t, 1, h.roles.scopedCalls)
			assert.Equal(t, forgedOrgSessionID, h.roles.scopedOrgs[0])
			assert.Equal(t, 1, h.repo.replaceRoles)
			assert.Equal(t, []string{putRelRoleID}, h.repo.roleIDs)
			assert.Zero(t, h.repo.replaceGroups)
			assert.Zero(t, h.repo.replaceIdentities)
		},
		checkForeignChild: func(t *testing.T, h *putRelHarness) {
			assert.GreaterOrEqual(t, h.roles.scopedCalls, 1)
			assert.Equal(t, []string{putRelRoleID, putRelOtherRoleID}, h.roles.scopedIDs)
		},
		checkNoWrites: func(t *testing.T, h *putRelHarness) {
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.updateForOrgCalls)
			assert.Zero(t, h.repo.replaceRoles)
		},
	},
	{
		name:      "group-memberships",
		path:      putRelBasePath() + "/group-memberships",
		body:      `{"items":[{"groupId":"` + putRelGroupID + `"}]}`,
		otherBody: `{"items":[{"groupId":"` + putRelGroupID + `"},{"groupId":"` + putRelOtherGroupID + `"}]}`,
		checkOwn: func(t *testing.T, h *putRelHarness) {
			assert.Equal(t, 1, h.groups.scopedCalls)
			assert.Equal(t, forgedOrgSessionID, h.groups.scopedOrgs[0])
			assert.Equal(t, 1, h.repo.replaceGroups)
			assert.Equal(t, []string{putRelGroupID}, h.repo.groupIDs)
			assert.Zero(t, h.repo.replaceRoles)
			assert.Zero(t, h.repo.replaceIdentities)
		},
		checkForeignChild: func(t *testing.T, h *putRelHarness) {
			assert.GreaterOrEqual(t, h.groups.scopedCalls, 1)
			assert.Equal(t, []string{putRelGroupID, putRelOtherGroupID}, h.groups.scopedIDs)
		},
		checkNoWrites: func(t *testing.T, h *putRelHarness) {
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.updateForOrgCalls)
			assert.Zero(t, h.repo.replaceGroups)
		},
	},
	{
		name:      "identities",
		path:      putRelBasePath() + "/identities",
		body:      `{"items":[{"id":"` + putRelIdentityID + `","providerId":"google","identityType":"oauth","subject":"sub-1"}]}`,
		otherBody: `{"items":[{"id":"` + putRelIdentityID + `","providerId":"google","identityType":"oauth","subject":"sub-1"},{"id":"` + putRelOtherIdentityID + `","providerId":"github","identityType":"oauth","subject":"sub-2"}]}`,
		checkOwn: func(t *testing.T, h *putRelHarness) {
			assert.Equal(t, 1, h.repo.replaceIdentities)
			assert.Equal(t, []string{putRelIdentityID}, h.repo.identityIDs)
			assert.Zero(t, h.repo.replaceRoles)
			assert.Zero(t, h.repo.replaceGroups)
		},
		checkForeignChild: func(t *testing.T, h *putRelHarness) {
			assert.Zero(t, h.repo.replaceIdentities)
		},
		checkNoWrites: func(t *testing.T, h *putRelHarness) {
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.updateForOrgCalls)
			assert.Zero(t, h.repo.replaceIdentities)
		},
	},
}

func assertPutRelNoAudit(t *testing.T, h *putRelHarness) {
	t.Helper()
	assert.Empty(t, h.records.created)
	assert.Empty(t, h.outbox.enqueued)
}

// TestUserPutRelations_ForeignTenantRejected pins that all three PUT routes
// resolve {id} through the trusted tenant before decoding/writing: a foreign
// user is 404 with zero writes and zero audit.
func TestUserPutRelations_ForeignTenantRejected(t *testing.T) {
	for _, tt := range putRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPutRelHarness(forgedOrgRequestID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodPut,
				tt.path+"?organizationId="+forgedOrgRequestID, tt.body, postByIDCallerID))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign user must look missing")
			assert.Zero(t, h.repo.getByIDCalls, "PUT routes must never use the system-scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			tt.checkNoWrites(t, h)
			assertPutRelNoAudit(t, h)
		})
	}
}

// TestUserPutRelations_WithoutTenantScopeRejected pins the fail-fast: an empty
// trusted tenant is a 400 before the body is decoded or any repository read.
func TestUserPutRelations_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range putRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPutRelHarness(forgedOrgSessionID)

			recorder := byIDServe(h.router, httptest.NewRequest(http.MethodPut, tt.path, strings.NewReader(tt.body)))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getByIDCalls)
			assert.Zero(t, h.repo.scopedGetCalls)
			tt.checkNoWrites(t, h)
			assertPutRelNoAudit(t, h)
		})
	}
}

// TestUserPutRelations_OwnTenantSucceeds pins the scoped write path and that
// audit carries the entity organization.
func TestUserPutRelations_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range putRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPutRelHarness(forgedOrgSessionID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodPut,
				tt.path+"?organizationId="+forgedOrgRequestID, tt.body, postByIDCallerID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.JSONEq(t, `{"status":"updated","msg":"resource updated"}`, recorder.Body.String())
			assert.Zero(t, h.repo.getByIDCalls, "management writes must use the scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs)
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}

			tt.checkOwn(t, h)
			require.Len(t, h.records.created, 1)
			assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID,
				"audit must carry the entity organization, never the built-in tenant")
		})
	}
}

// TestUserPutRelations_ForeignChildRejected pins that a body item outside the
// target user's tenant/aggregate is 404 and blocks the whole replacement:
// no partial write, no audit.
func TestUserPutRelations_ForeignChildRejected(t *testing.T) {
	for _, tt := range putRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPutRelHarness(forgedOrgSessionID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodPut,
				tt.path, tt.otherBody, postByIDCallerID))

			assert.Equal(t, http.StatusNotFound, recorder.Code,
				"a foreign child id must reject the whole replacement")
			assert.Equal(t, 1, h.repo.scopedGetCalls, "the parent is resolved before child checks")
			tt.checkForeignChild(t, h)
			tt.checkNoWrites(t, h)
			assertPutRelNoAudit(t, h)
		})
	}
}

// TestUserPutRelations_PlatformAdminCanTargetTenant pins that a platform scope
// keeps the explicit cross-tenant target as the predicate.
func TestUserPutRelations_PlatformAdminCanTargetTenant(t *testing.T) {
	for _, tt := range putRelRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newPutRelHarness(forgedOrgRequestID)

			recorder := byIDServe(h.router, putRelPlatformRequest(tt.path+"?organizationId="+forgedOrgRequestID, tt.body))

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
	_ appuow.UnitOfWork = (*putRelUnitOfWork)(nil)
)
