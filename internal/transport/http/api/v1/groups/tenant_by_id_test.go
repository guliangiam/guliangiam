package groups

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const foreignGroupID = "foreign-group"

type groupByIDClock struct{}

func (groupByIDClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type groupByIDUUID struct{}

func (groupByIDUUID) NewString() (string, error) { return "generated-group-id", nil }

// groupByIDRepo is a tenant-agnostic in-memory repository shaped like the
// current production port; it mutates on Update/Delete so a missing tenant
// check is observable as a real cross-tenant write.
type groupByIDRepo struct {
	groups    []*domain.Group
	updateCnt int
	deleteCnt int
}

func (r *groupByIDRepo) GetByID(_ context.Context, id string) (*domain.Group, error) {
	for _, group := range r.groups {
		if group.ID == id {
			return group, nil
		}
	}
	return nil, apperrors.NewNotFound("group not found")
}

func (r *groupByIDRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Group, error) {
	for _, group := range r.groups {
		if group.ID == id && group.OrganizationID == organizationID {
			return group, nil
		}
	}
	return nil, apperrors.NewNotFound("group not found")
}

func (r *groupByIDRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Group, int64, error) {
	return r.groups, int64(len(r.groups)), nil
}

func (r *groupByIDRepo) Create(_ context.Context, group *domain.Group) error {
	r.groups = append(r.groups, group)
	return nil
}

func (r *groupByIDRepo) Update(_ context.Context, group *domain.Group) error {
	r.updateCnt++
	for i, existing := range r.groups {
		if existing.ID == group.ID {
			r.groups[i] = group
			return nil
		}
	}
	return apperrors.NewNotFound("group not found")
}

func (r *groupByIDRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, existing := range r.groups {
		if existing.ID == id && existing.OrganizationID == organizationID {
			r.groups = append(r.groups[:i], r.groups[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("group not found")
}

func (r *groupByIDRepo) CountChildrenByParentID(_ context.Context, organizationID, parentID string) (int64, error) {
	var count int64
	for _, group := range r.groups {
		if group.ParentID == parentID && group.OrganizationID == organizationID {
			count++
		}
	}
	return count, nil
}

// groupByIDRecords records the audit rows embedded by the app service.
type groupByIDRecords struct {
	created []*recordsdomain.Record
}

func (r *groupByIDRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *groupByIDRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *groupByIDRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *groupByIDRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *groupByIDRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *groupByIDRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *groupByIDRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type groupByIDUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *groupByIDRecords
}

func (u *groupByIDUOW) Groups() domain.Repository         { return u.repo }
func (u *groupByIDUOW) Records() recordsdomain.Repository { return u.records }
func (u *groupByIDUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *groupByIDUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newGroupByIDService(repo *groupByIDRepo) (appgroups.Service, *groupByIDRecords) {
	records := &groupByIDRecords{}
	return appgroups.Service{
		Clock: groupByIDClock{},
		UUID:  groupByIDUUID{},
		UOW:   &groupByIDUOW{repo: repo, records: records},
	}, records
}

func groupByIDRouter(service appgroups.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveGroupByIDRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func foreignGroup() *domain.Group {
	return &domain.Group{
		ID:             foreignGroupID,
		OrganizationID: forgedOrgRequestID,
		Name:           "foreign-group",
		DisplayName:    "Foreign Group",
		IsEnabled:      true,
		Revision:       1,
	}
}

// TestGroupByID_ForeignTenantRejected pins that a foreign group id looks
// exactly like a missing one: 404, no update, no delete, no audit record.
func TestGroupByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/groups/" + foreignGroupID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/groups/" + foreignGroupID + "?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/groups/" + foreignGroupID + "?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &groupByIDRepo{groups: []*domain.Group{foreignGroup()}}
			service, records := newGroupByIDService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveGroupByIDRequest(t, groupByIDRouter(service),
				forgedOrgRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the group")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the group")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			require.Len(t, repo.groups, 1, "foreign group row must remain untouched")
		})
	}
}

// TestGroupByID_WithoutTenantScopeRejected pins the fail-fast: no trusted
// tenant means 400 before the repository is touched (MySQL has no RLS).
func TestGroupByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &groupByIDRepo{groups: []*domain.Group{{
		ID: "own-group", OrganizationID: forgedOrgSessionID, Name: "own", IsEnabled: true, Revision: 1,
	}}}
	service, _ := newGroupByIDService(repo)
	router := groupByIDRouter(service)

	requests := map[string]*http.Request{
		"get":    httptest.NewRequest(http.MethodGet, "/api/v1/groups/own-group", nil),
		"patch":  httptest.NewRequest(http.MethodPatch, "/api/v1/groups/own-group", strings.NewReader(`{}`)),
		"delete": httptest.NewRequest(http.MethodDelete, "/api/v1/groups/own-group", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveGroupByIDRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
}

// TestGroupByID_OwnTenantSucceeds pins that the same routes still work inside
// the authenticated tenant and that delete audits the group's organization.
func TestGroupByID_OwnTenantSucceeds(t *testing.T) {
	const ownGroupID = "own-group"

	t.Run("get", func(t *testing.T) {
		repo := &groupByIDRepo{groups: []*domain.Group{{
			ID: ownGroupID, OrganizationID: forgedOrgSessionID, Name: "own", DisplayName: "Own",
			IsEnabled: true, Revision: 1,
		}}}
		service, _ := newGroupByIDService(repo)
		recorder := serveGroupByIDRequest(t, groupByIDRouter(service),
			forgedOrgRequest(http.MethodGet, "/api/v1/groups/"+ownGroupID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+forgedOrgSessionID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &groupByIDRepo{groups: []*domain.Group{{
			ID: ownGroupID, OrganizationID: forgedOrgSessionID, Name: "own", DisplayName: "Own",
			IsEnabled: true, Revision: 1,
		}}}
		service, records := newGroupByIDService(repo)
		recorder := serveGroupByIDRequest(t, groupByIDRouter(service),
			forgedOrgRequest(http.MethodPatch, "/api/v1/groups/"+ownGroupID,
				strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &groupByIDRepo{groups: []*domain.Group{{
			ID: ownGroupID, OrganizationID: forgedOrgSessionID, Name: "own", IsEnabled: true, Revision: 1,
		}}}
		service, records := newGroupByIDService(repo)
		recorder := serveGroupByIDRequest(t, groupByIDRouter(service),
			forgedOrgRequest(http.MethodDelete, "/api/v1/groups/"+ownGroupID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID,
			"delete audit must carry the group organization")
	})
}

// TestGroupByID_PlatformAdminCanCrossTenant pins the single legitimate override.
func TestGroupByID_PlatformAdminCanCrossTenant(t *testing.T) {
	repo := &groupByIDRepo{groups: []*domain.Group{foreignGroup()}}
	service, _ := newGroupByIDService(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/groups/"+foreignGroupID+"?organizationId="+forgedOrgRequestID, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterSuperAdmin)
	req = req.WithContext(ctx)

	recorder := serveGroupByIDRequest(t, groupByIDRouter(service), req)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), forgedOrgRequestID)
}
