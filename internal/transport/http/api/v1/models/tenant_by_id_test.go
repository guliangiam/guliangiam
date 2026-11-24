package models

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appmodels "github.com/guliangiam/guliangiam/internal/app/models"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/models"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tenantOrgID     = "11111111-1111-1111-1111-111111111111"
	foreignOrgID    = "22222222-2222-2222-2222-222222222222"
	ownModelID      = "own-model"
	foreignModelID  = "foreign-model"
	tenantUserID    = "33333333-3333-3333-3333-333333333333"
	tenantSessionOr = "session-org"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-model-id", nil }

func tenantRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), tenantOrgID, tenantSessionOr, tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{UserID: tenantUserID, OrganizationID: tenantOrgID})
	return req.WithContext(ctx)
}

func platformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), tenantOrgID, tenantSessionOr, tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func bareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

type idorModelRepo struct {
	models    []*domain.Model
	getOrgIDs []string
	updateCnt int
	deleteCnt int
}

func (r *idorModelRepo) GetByID(_ context.Context, id string) (*domain.Model, error) {
	for _, m := range r.models {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, apperrors.NewNotFound("model not found")
}

func (r *idorModelRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Model, error) {
	r.getOrgIDs = append(r.getOrgIDs, organizationID)
	for _, m := range r.models {
		if m.ID == id && m.OrganizationID == organizationID {
			return m, nil
		}
	}
	return nil, apperrors.NewNotFound("model not found")
}

func (r *idorModelRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Model, int64, error) {
	return r.models, int64(len(r.models)), nil
}

func (r *idorModelRepo) Create(_ context.Context, model *domain.Model) error {
	r.models = append(r.models, model)
	return nil
}

func (r *idorModelRepo) Update(_ context.Context, model *domain.Model) error {
	r.updateCnt++
	for i, m := range r.models {
		if m.ID == model.ID && m.OrganizationID == model.OrganizationID {
			r.models[i] = model
			return nil
		}
	}
	return apperrors.NewNotFound("model not found")
}

func (r *idorModelRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, m := range r.models {
		if m.ID == id && m.OrganizationID == organizationID {
			r.models = append(r.models[:i], r.models[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("model not found")
}

type idorRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorRecords
}

func (u *idorUOW) Models() domain.Repository { return u.repo }
func (u *idorUOW) Records() recordsdomain.Repository {
	return u.records
}
func (u *idorUOW) Outbox() domainoutbox.Repository { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newTenantService(repo domain.Repository) (appmodels.Service, *idorRecords) {
	records := &idorRecords{}
	return appmodels.Service{UOW: &idorUOW{repo: repo, records: records}, Clock: idorClock{}, UUID: idorUUID{}}, records
}

func modelFixture(id, orgID string) *domain.Model {
	return &domain.Model{ID: id, OrganizationID: orgID, Name: "tenant-model", IsEnabled: true, Revision: 1}
}

func tenantRouter(service appmodels.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveTenant(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestModelByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/models/" + foreignModelID + "?organizationId=" + foreignOrgID, ""},
		{"patch", http.MethodPatch, "/api/v1/models/" + foreignModelID + "?organizationId=" + foreignOrgID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/models/" + foreignModelID + "?organizationId=" + foreignOrgID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorModelRepo{models: []*domain.Model{modelFixture(foreignModelID, foreignOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), tenantRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign model must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the model")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the model")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, []string{tenantOrgID}, repo.getOrgIDs, "repository must receive the trusted session tenant")
			require.Len(t, repo.models, 1, "foreign model row must remain untouched")
		})
	}
}

func TestModelByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{"get", http.MethodGet, ""},
		{"patch", http.MethodPatch, `{}`},
		{"delete", http.MethodDelete, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorModelRepo{models: []*domain.Model{modelFixture(ownModelID, tenantOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), bareRequest(tt.method, "/api/v1/models/"+ownModelID, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.getOrgIDs, "an empty tenant must not reach the repository")
		})
	}
}

func TestModelByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorModelRepo{models: []*domain.Model{modelFixture(ownModelID, tenantOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodGet, "/api/v1/models/"+ownModelID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+tenantOrgID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorModelRepo{models: []*domain.Model{modelFixture(ownModelID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			tenantRequest(http.MethodPatch, "/api/v1/models/"+ownModelID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorModelRepo{models: []*domain.Model{modelFixture(ownModelID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodDelete, "/api/v1/models/"+ownModelID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.models)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID, "delete audit must carry the model organization")
	})
}

func TestModelByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorModelRepo{models: []*domain.Model{modelFixture(foreignModelID, foreignOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodGet, "/api/v1/models/"+foreignModelID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), foreignOrgID)
	})

	t.Run("delete audits the model tenant", func(t *testing.T) {
		repo := &idorModelRepo{models: []*domain.Model{modelFixture(foreignModelID, foreignOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodDelete, "/api/v1/models/"+foreignModelID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, foreignOrgID, records.created[0].OrganizationID)
	})
}
