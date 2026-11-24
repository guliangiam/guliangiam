package forms

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appforms "github.com/guliangiam/guliangiam/internal/app/forms"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/forms"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tenantOrgID    = "11111111-1111-1111-1111-111111111111"
	foreignOrgID   = "22222222-2222-2222-2222-222222222222"
	ownFormID      = "own-form"
	foreignFormID  = "foreign-form"
	tenantUserID   = "33333333-3333-3333-3333-333333333333"
	tenantSessionO = "session-org"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-form-id", nil }

func tenantRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), tenantOrgID, tenantSessionO, tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{UserID: tenantUserID, OrganizationID: tenantOrgID})
	return req.WithContext(ctx)
}

func platformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), tenantOrgID, tenantSessionO, tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func bareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

type idorFormRepo struct {
	forms     []*domain.Form
	getOrgIDs []string
	updateCnt int
	deleteCnt int
}

func (r *idorFormRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Form, error) {
	r.getOrgIDs = append(r.getOrgIDs, organizationID)
	for _, f := range r.forms {
		if f.ID == id && f.OrganizationID == organizationID {
			return f, nil
		}
	}
	return nil, apperrors.NewNotFound("form not found")
}

func (r *idorFormRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Form, int64, error) {
	return r.forms, int64(len(r.forms)), nil
}

func (r *idorFormRepo) Create(_ context.Context, form *domain.Form) error {
	r.forms = append(r.forms, form)
	return nil
}

func (r *idorFormRepo) Update(_ context.Context, form *domain.Form) error {
	r.updateCnt++
	for i, f := range r.forms {
		if f.ID == form.ID && f.OrganizationID == form.OrganizationID {
			r.forms[i] = form
			return nil
		}
	}
	return apperrors.NewNotFound("form not found")
}

func (r *idorFormRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, f := range r.forms {
		if f.ID == id && f.OrganizationID == organizationID {
			r.forms = append(r.forms[:i], r.forms[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("form not found")
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

func (u *idorUOW) Forms() domain.Repository { return u.repo }
func (u *idorUOW) Records() recordsdomain.Repository {
	return u.records
}
func (u *idorUOW) Outbox() domainoutbox.Repository { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newTenantService(repo domain.Repository) (appforms.Service, *idorRecords) {
	records := &idorRecords{}
	return appforms.Service{UOW: &idorUOW{repo: repo, records: records}, Clock: idorClock{}, UUID: idorUUID{}}, records
}

func formFixture(id, orgID string) *domain.Form {
	return &domain.Form{ID: id, OrganizationID: orgID, Name: "tenant-form", IsEnabled: true, Revision: 1}
}

func tenantRouter(service appforms.Service) chi.Router {
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

func TestFormByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/forms/" + foreignFormID + "?organizationId=" + foreignOrgID, ""},
		{"patch", http.MethodPatch, "/api/v1/forms/" + foreignFormID + "?organizationId=" + foreignOrgID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/forms/" + foreignFormID + "?organizationId=" + foreignOrgID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorFormRepo{forms: []*domain.Form{formFixture(foreignFormID, foreignOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), tenantRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign form must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the form")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the form")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, []string{tenantOrgID}, repo.getOrgIDs, "repository must receive the trusted session tenant")
			require.Len(t, repo.forms, 1, "foreign form row must remain untouched")
		})
	}
}

func TestFormByID_WithoutTenantScopeRejected(t *testing.T) {
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
			repo := &idorFormRepo{forms: []*domain.Form{formFixture(ownFormID, tenantOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), bareRequest(tt.method, "/api/v1/forms/"+ownFormID, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.getOrgIDs, "an empty tenant must not reach the repository")
		})
	}
}

func TestFormByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorFormRepo{forms: []*domain.Form{formFixture(ownFormID, tenantOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodGet, "/api/v1/forms/"+ownFormID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+tenantOrgID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorFormRepo{forms: []*domain.Form{formFixture(ownFormID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			tenantRequest(http.MethodPatch, "/api/v1/forms/"+ownFormID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorFormRepo{forms: []*domain.Form{formFixture(ownFormID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodDelete, "/api/v1/forms/"+ownFormID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.forms)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID, "delete audit must carry the form organization")
	})
}

func TestFormByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorFormRepo{forms: []*domain.Form{formFixture(foreignFormID, foreignOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodGet, "/api/v1/forms/"+foreignFormID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), foreignOrgID)
	})

	t.Run("delete audits the form tenant", func(t *testing.T) {
		repo := &idorFormRepo{forms: []*domain.Form{formFixture(foreignFormID, foreignOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodDelete, "/api/v1/forms/"+foreignFormID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, foreignOrgID, records.created[0].OrganizationID)
	})
}
