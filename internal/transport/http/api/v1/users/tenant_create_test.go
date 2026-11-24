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
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	createUserID = "88888888-8888-8888-8888-888888888888"
)

type createUserClock struct{}

func (createUserClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type createUserUUID struct{}

func (createUserUUID) NewString() (string, error) { return createUserID, nil }

// createUserRepo records the tenant every lookup and insert went through. A
// cross-tenant create is visible as a row whose OrganizationID is not the
// trusted tenant.
type createUserRepo struct {
	userdomain.Repository
	getByUsernameOrgs []string
	hasDeletedOrgs    []string
	created           []*userdomain.User
}

func (r *createUserRepo) GetByUsername(_ context.Context, organizationID, _ string) (*userdomain.User, error) {
	r.getByUsernameOrgs = append(r.getByUsernameOrgs, organizationID)
	return nil, apperrors.NewNotFound("user not found")
}

func (r *createUserRepo) HasDeletedWithUsername(_ context.Context, organizationID, _ string) (bool, error) {
	r.hasDeletedOrgs = append(r.hasDeletedOrgs, organizationID)
	return false, nil
}

func (r *createUserRepo) Create(_ context.Context, user *userdomain.User) error {
	r.created = append(r.created, user)
	return nil
}

type createUserRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *createUserRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type createUserOutbox struct {
	domainoutbox.Repository
	enqueued []*domainoutbox.OutboxEvent
}

func (r *createUserOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, event)
	return nil
}

type createUserUnitOfWork struct {
	appuow.UnitOfWork
	users   *createUserRepo
	records *createUserRecords
	outbox  *createUserOutbox
}

func (u *createUserUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *createUserUnitOfWork) Users() userdomain.Repository      { return u.users }
func (u *createUserUnitOfWork) Records() recordsdomain.Repository { return u.records }
func (u *createUserUnitOfWork) Outbox() domainoutbox.Repository   { return u.outbox }

type createUserHarness struct {
	router  chi.Router
	users   *createUserRepo
	records *createUserRecords
	outbox  *createUserOutbox
}

func newCreateUserHarness() *createUserHarness {
	h := &createUserHarness{
		users:   &createUserRepo{},
		records: &createUserRecords{},
		outbox:  &createUserOutbox{},
	}
	uow := &createUserUnitOfWork{users: h.users, records: h.records, outbox: h.outbox}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: appusers.Service{
		Clock: createUserClock{}, UUID: createUserUUID{}, UOW: uow,
	}})
	return h
}

func createUserTenantRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

func createUserPlatformRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
		OrganizationSlug: "built-in",
	})
	return req.WithContext(ctx)
}

func createUserBody(organizationID string) string {
	return `{"username":"alice","userType":"normal","status":"active",` +
		`"organizationId":"` + organizationID + `","profile":{"displayName":"Alice"}}`
}

func serveCreateUser(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestCreateUser_ForgedOrganizationIgnored pins the IDOR closure: a body
// organizationId cannot move the insert outside the authenticated tenant. The
// forged value is ignored exactly like /users/import clamps its rows; the write
// and the audit event land in the trusted tenant only.
func TestCreateUser_ForgedOrganizationIgnored(t *testing.T) {
	h := newCreateUserHarness()

	recorder := serveCreateUser(t, h.router, createUserTenantRequest(createUserBody(forgedOrgRequestID)))

	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	require.Len(t, h.users.created, 1)
	assert.Equal(t, forgedOrgSessionID, h.users.created[0].OrganizationID,
		"the forged organizationId must not move the insert")
	assert.NotEqual(t, forgedOrgRequestID, h.users.created[0].OrganizationID)
	for _, org := range h.users.getByUsernameOrgs {
		assert.Equal(t, forgedOrgSessionID, org, "uniqueness must be checked inside the trusted tenant")
	}
	for _, org := range h.users.hasDeletedOrgs {
		assert.Equal(t, forgedOrgSessionID, org)
	}
	require.Len(t, h.records.created, 1, "the create must audit")
	assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID)
	require.Len(t, h.outbox.enqueued, 1)
}

// TestCreateUser_WithoutTenantScopeRejected pins the empty-org fail-fast: no
// repository read and no write when the caller has no trusted tenant.
func TestCreateUser_WithoutTenantScopeRejected(t *testing.T) {
	h := newCreateUserHarness()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users",
		strings.NewReader(createUserBody(forgedOrgSessionID)))
	req.Header.Set("Content-Type", "application/json")
	recorder := serveCreateUser(t, h.router, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Empty(t, h.users.getByUsernameOrgs)
	assert.Empty(t, h.users.hasDeletedOrgs)
	assert.Empty(t, h.users.created)
	assert.Empty(t, h.records.created)
	assert.Empty(t, h.outbox.enqueued)
}

// TestCreateUser_OwnTenantSucceeds pins the normal tenant path.
func TestCreateUser_OwnTenantSucceeds(t *testing.T) {
	h := newCreateUserHarness()

	recorder := serveCreateUser(t, h.router, createUserTenantRequest(createUserBody(forgedOrgSessionID)))

	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	assert.JSONEq(t, `{"id":"`+createUserID+`","status":"created"}`, recorder.Body.String())
	assert.Equal(t, "/api/v1/users/"+createUserID, recorder.Header().Get("Location"),
		"the 201 must advertise the canonical path of the created user")
	require.Len(t, h.users.created, 1)
	assert.Equal(t, forgedOrgSessionID, h.users.created[0].OrganizationID)
	require.Len(t, h.records.created, 1)
	assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID)
}

// TestCreateUser_PlatformAdminTargetsExplicitTenant pins that a platform
// administrator keeps creating users in an explicitly named tenant.
func TestCreateUser_PlatformAdminTargetsExplicitTenant(t *testing.T) {
	h := newCreateUserHarness()

	recorder := serveCreateUser(t, h.router, createUserPlatformRequest(createUserBody(forgedOrgRequestID)))

	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	require.Len(t, h.users.created, 1)
	assert.Equal(t, forgedOrgRequestID, h.users.created[0].OrganizationID,
		"platform admin's explicit organization is the predicate")
	require.Len(t, h.records.created, 1)
	assert.Equal(t, forgedOrgRequestID, h.records.created[0].OrganizationID)
}
