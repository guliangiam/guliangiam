package invitations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	postInvitationTenantOrg  = "11111111-1111-1111-1111-111111111111"
	postInvitationForeignOrg = "22222222-2222-2222-2222-222222222222"
	postInvitationOwnID      = "33333333-3333-3333-3333-333333333333"
	postInvitationForeignID  = "44444444-4444-4444-4444-444444444444"
)

type postInvitationClock struct{}

func (postInvitationClock) Now() time.Time { return time.Unix(0, 0).UTC() }

// postInvitationRepo is a tenant-aware in-memory repository. The system-scoped
// read still returns the row so the pre-fix exploit is observable.
type postInvitationRepo struct {
	domain.Repository
	items   map[string]*domain.Invitation
	gets    int
	updates int
	updateO string
}

func (r *postInvitationRepo) GetByID(_ context.Context, id string) (*domain.Invitation, error) {
	r.gets++
	if item, ok := r.items[id]; ok {
		return item, nil
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *postInvitationRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Invitation, error) {
	r.gets++
	if item, ok := r.items[id]; ok && item.OrganizationID == organizationID {
		return item, nil
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *postInvitationRepo) Update(_ context.Context, item *domain.Invitation) error {
	r.updates++
	r.updateO = item.OrganizationID
	r.items[item.ID] = item
	return nil
}

type postInvitationRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *postInvitationRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type postInvitationOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (o *postInvitationOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, event)
	return nil
}

type postInvitationUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *postInvitationRecords
	outbox  *postInvitationOutbox
}

func (u *postInvitationUOW) Invitations() domain.Repository    { return u.repo }
func (u *postInvitationUOW) Records() recordsdomain.Repository { return u.records }
func (u *postInvitationUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *postInvitationUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type postInvitationHarness struct {
	repo    *postInvitationRepo
	records *postInvitationRecords
	outbox  *postInvitationOutbox
	router  chi.Router
}

func newPostInvitationHarness(items ...*domain.Invitation) *postInvitationHarness {
	inventory := make(map[string]*domain.Invitation, len(items))
	for _, item := range items {
		inventory[item.ID] = item
	}
	repo := &postInvitationRepo{items: inventory}
	records := &postInvitationRecords{}
	outbox := &postInvitationOutbox{}
	service := appinvitations.Service{
		Clock: postInvitationClock{},
		UOW:   &postInvitationUOW{repo: repo, records: records, outbox: outbox},
	}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return &postInvitationHarness{repo: repo, records: records, outbox: outbox, router: router}
}

func postInvitationRequest(method, target string, organizationID string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if organizationID != "" {
		ctx := tenantctx.WithTenantScope(req.Context(), organizationID, "session-org", tenantctx.TenantFilterStrict)
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
			UserID: "55555555-5555-5555-5555-555555555555", OrganizationID: organizationID,
		})
		req = req.WithContext(ctx)
	}
	return req
}

func postInvitationPlatformRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, nil)
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func servePostInvitation(t *testing.T, harness *postInvitationHarness, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	harness.router.ServeHTTP(recorder, req)
	return recorder
}

func postInvitationItem(id, organizationID string) *domain.Invitation {
	now := time.Unix(0, 0).UTC()
	return &domain.Invitation{
		ID: id, OrganizationID: organizationID, Email: "user@example.com", Code: "code-" + id,
		Status: domain.StatusPending, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
}

var postInvitationActions = []string{"send", "revoke", "resend"}

// TestInvitationPostAction_ForeignTenantRejected pins that send/revoke/resend
// are tenant-bound: a foreign id is a 404 and never mutates. send is read-only
// and stays silent; revoke/resend deliberately write a rejection audit trail
// (invitationTxReject → recordInvitationRejectAudit) in their own system
// transaction, attributed to the caller's trusted tenant — never the foreign
// organization, so probing another tenant leaves a trail without leaking it.
func TestInvitationPostAction_ForeignTenantRejected(t *testing.T) {
	for _, action := range postInvitationActions {
		t.Run(action, func(t *testing.T) {
			harness := newPostInvitationHarness(postInvitationItem(postInvitationForeignID, postInvitationForeignOrg))

			recorder := servePostInvitation(t, harness, postInvitationRequest(http.MethodPost,
				"/api/v1/invitations/"+postInvitationForeignID+"/"+action+"?organizationId="+postInvitationForeignOrg,
				postInvitationTenantOrg))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign invitation must look missing")
			assert.Zero(t, harness.repo.updates, "foreign tenant must not mutate the invitation")
			if action == "send" {
				assert.Empty(t, harness.records.created, "send rejection must not produce an audit record")
				assert.Empty(t, harness.outbox.events, "send rejection must not produce an outbox event")
				return
			}
			require.Len(t, harness.records.created, 1, "rejection must leave exactly one audit record")
			assert.Equal(t, "invitations.rejected", harness.records.created[0].Action)
			assert.Equal(t, postInvitationTenantOrg, harness.records.created[0].OrganizationID,
				"the rejection audit must never point at the foreign organization")
			require.Len(t, harness.outbox.events, 1, "rejection must leave exactly one outbox event")
			require.NotNil(t, harness.outbox.events[0].OrganizationID)
			assert.Equal(t, postInvitationTenantOrg, *harness.outbox.events[0].OrganizationID,
				"the rejection outbox event must never point at the foreign organization")
		})
	}
}

// TestInvitationPostAction_WithoutTenantScopeRejected pins the fail-fast: no
// trusted tenant yields 400 before the repository is touched.
func TestInvitationPostAction_WithoutTenantScopeRejected(t *testing.T) {
	for _, action := range postInvitationActions {
		t.Run(action, func(t *testing.T) {
			harness := newPostInvitationHarness(postInvitationItem(postInvitationOwnID, postInvitationTenantOrg))

			recorder := servePostInvitation(t, harness, postInvitationRequest(http.MethodPost,
				"/api/v1/invitations/"+postInvitationOwnID+"/"+action, ""))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, harness.repo.gets, "an empty tenant must not reach the repository")
			assert.Zero(t, harness.repo.updates)
			assert.Empty(t, harness.records.created)
		})
	}
}

// TestInvitationPostAction_OwnTenantSucceeds pins the same-tenant path and that
// the write audit carries the invitation's real organization.
func TestInvitationPostAction_OwnTenantSucceeds(t *testing.T) {
	for _, action := range postInvitationActions {
		t.Run(action, func(t *testing.T) {
			harness := newPostInvitationHarness(postInvitationItem(postInvitationOwnID, postInvitationTenantOrg))

			recorder := servePostInvitation(t, harness, postInvitationRequest(http.MethodPost,
				"/api/v1/invitations/"+postInvitationOwnID+"/"+action, postInvitationTenantOrg))

			require.Equal(t, http.StatusOK, recorder.Code)
			if action == "send" {
				assert.Zero(t, harness.repo.updates)
				return
			}
			assert.Equal(t, 1, harness.repo.updates)
			assert.Equal(t, postInvitationTenantOrg, harness.repo.updateO,
				"the write must use the invitation's real organization")
			require.Len(t, harness.records.created, 1)
			assert.Equal(t, postInvitationTenantOrg, harness.records.created[0].OrganizationID)
			require.Len(t, harness.outbox.events, 1)
			require.NotNil(t, harness.outbox.events[0].OrganizationID)
			assert.Equal(t, postInvitationTenantOrg, *harness.outbox.events[0].OrganizationID)
		})
	}
}

// TestInvitationPostAction_PlatformAdminCanTargetTenant pins the single legit
// override: super_admin/system scopes may name another tenant explicitly.
func TestInvitationPostAction_PlatformAdminCanTargetTenant(t *testing.T) {
	harness := newPostInvitationHarness(postInvitationItem(postInvitationForeignID, postInvitationForeignOrg))

	recorder := servePostInvitation(t, harness, postInvitationPlatformRequest(
		"/api/v1/invitations/"+postInvitationForeignID+"/revoke?organizationId="+postInvitationForeignOrg))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, harness.repo.updates)
	assert.Equal(t, postInvitationForeignOrg, harness.repo.updateO)
}
