package roles

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	approles "github.com/guliangiam/guliangiam/internal/app/roles"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const foreignRoleID = "foreign-role"

type roleByIDClock struct{}

func (roleByIDClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type roleByIDUUID struct{}

func (roleByIDUUID) NewString() (string, error) { return "generated-role-id", nil }

// roleByIDRepo is a tenant-agnostic in-memory repository shaped like the
// current production port; it mutates on Update/Delete so a missing tenant
// check is observable as a real cross-tenant write.
type roleByIDRepo struct {
	roles     []*domain.Role
	updateCnt int
	deleteCnt int
}

func (r *roleByIDRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Role, error) {
	for _, role := range r.roles {
		if role.ID == id && role.OrganizationID == organizationID {
			return role, nil
		}
	}
	return nil, apperrors.NewNotFound("role not found")
}

func (r *roleByIDRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Role, int64, error) {
	return r.roles, int64(len(r.roles)), nil
}

func (r *roleByIDRepo) Create(_ context.Context, role *domain.Role) error {
	r.roles = append(r.roles, role)
	return nil
}

func (r *roleByIDRepo) Update(_ context.Context, role *domain.Role) error {
	r.updateCnt++
	for i, existing := range r.roles {
		if existing.ID == role.ID {
			r.roles[i] = role
			return nil
		}
	}
	return apperrors.NewNotFound("role not found")
}

func (r *roleByIDRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, existing := range r.roles {
		if existing.ID == id && existing.OrganizationID == organizationID {
			r.roles = append(r.roles[:i], r.roles[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("role not found")
}

// roleByIDRecords records the audit rows embedded by the app service.
type roleByIDRecords struct {
	created []*recordsdomain.Record
}

func (r *roleByIDRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *roleByIDRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *roleByIDRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *roleByIDRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *roleByIDRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *roleByIDRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *roleByIDRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type roleByIDUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *roleByIDRecords
}

func (u *roleByIDUOW) Roles() domain.Repository          { return u.repo }
func (u *roleByIDUOW) Records() recordsdomain.Repository { return u.records }
func (u *roleByIDUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *roleByIDUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newRoleByIDService(repo *roleByIDRepo) (approles.Service, *roleByIDRecords) {
	records := &roleByIDRecords{}
	return approles.Service{
		Clock: roleByIDClock{},
		UUID:  roleByIDUUID{},
		UOW:   &roleByIDUOW{repo: repo, records: records},
	}, records
}

func roleByIDRouter(service approles.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveRoleByIDRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func foreignRole() *domain.Role {
	return &domain.Role{
		ID:             foreignRoleID,
		OrganizationID: forgedOrgRequestID,
		Name:           "foreign-admin",
		DisplayName:    "Foreign Admin",
		IsEnabled:      true,
		Revision:       1,
	}
}

// TestRoleByID_ForeignTenantRejected pins that a foreign role id looks exactly
// like a missing one: 404, no update, no delete, no audit record, row untouched.
func TestRoleByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/roles/" + foreignRoleID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/roles/" + foreignRoleID + "?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/roles/" + foreignRoleID + "?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &roleByIDRepo{roles: []*domain.Role{foreignRole()}}
			service, records := newRoleByIDService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveRoleByIDRequest(t, roleByIDRouter(service),
				forgedOrgRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the role")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the role")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			require.Len(t, repo.roles, 1, "foreign role row must remain untouched")
		})
	}
}

// TestRoleByID_WithoutTenantScopeRejected pins the fail-fast: no trusted
// tenant means 400 before the repository is touched (MySQL has no RLS).
func TestRoleByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &roleByIDRepo{roles: []*domain.Role{{
		ID: "own-role", OrganizationID: forgedOrgSessionID, Name: "own", IsEnabled: true, Revision: 1,
	}}}
	service, _ := newRoleByIDService(repo)
	router := roleByIDRouter(service)

	requests := map[string]*http.Request{
		"get":    httptest.NewRequest(http.MethodGet, "/api/v1/roles/own-role", nil),
		"patch":  httptest.NewRequest(http.MethodPatch, "/api/v1/roles/own-role", strings.NewReader(`{}`)),
		"delete": httptest.NewRequest(http.MethodDelete, "/api/v1/roles/own-role", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveRoleByIDRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
}

// TestRoleByID_OwnTenantSucceeds pins that the same routes still work inside
// the authenticated tenant and that delete audits the role's organization.
func TestRoleByID_OwnTenantSucceeds(t *testing.T) {
	const ownRoleID = "own-role"

	t.Run("get", func(t *testing.T) {
		repo := &roleByIDRepo{roles: []*domain.Role{{
			ID: ownRoleID, OrganizationID: forgedOrgSessionID, Name: "own", DisplayName: "Own",
			IsEnabled: true, Revision: 1,
		}}}
		service, _ := newRoleByIDService(repo)
		recorder := serveRoleByIDRequest(t, roleByIDRouter(service),
			forgedOrgRequest(http.MethodGet, "/api/v1/roles/"+ownRoleID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+forgedOrgSessionID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &roleByIDRepo{roles: []*domain.Role{{
			ID: ownRoleID, OrganizationID: forgedOrgSessionID, Name: "own", DisplayName: "Own",
			IsEnabled: true, Revision: 1,
		}}}
		service, records := newRoleByIDService(repo)
		recorder := serveRoleByIDRequest(t, roleByIDRouter(service),
			forgedOrgRequest(http.MethodPatch, "/api/v1/roles/"+ownRoleID,
				strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &roleByIDRepo{roles: []*domain.Role{{
			ID: ownRoleID, OrganizationID: forgedOrgSessionID, Name: "own", IsEnabled: true, Revision: 1,
		}}}
		service, records := newRoleByIDService(repo)
		recorder := serveRoleByIDRequest(t, roleByIDRouter(service),
			forgedOrgRequest(http.MethodDelete, "/api/v1/roles/"+ownRoleID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID,
			"delete audit must carry the role organization")
	})
}

// TestRoleByID_PlatformAdminCanCrossTenant pins the single legitimate override.
func TestRoleByID_PlatformAdminCanCrossTenant(t *testing.T) {
	repo := &roleByIDRepo{roles: []*domain.Role{foreignRole()}}
	service, _ := newRoleByIDService(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/roles/"+foreignRoleID+"?organizationId="+forgedOrgRequestID, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterSuperAdmin)
	req = req.WithContext(ctx)

	recorder := serveRoleByIDRequest(t, roleByIDRouter(service), req)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), forgedOrgRequestID)
}
