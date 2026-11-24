package rules

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apprules "github.com/guliangiam/guliangiam/internal/app/rules"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tenantOrgID    = "11111111-1111-1111-1111-111111111111"
	foreignOrgID   = "22222222-2222-2222-2222-222222222222"
	ownRuleID      = "own-rule"
	foreignRuleID  = "foreign-rule"
	tenantUserID   = "33333333-3333-3333-3333-333333333333"
	tenantSessionO = "session-org"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-rule-id", nil }

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

type idorRuleRepo struct {
	rules     []*domain.Rule
	getOrgIDs []string
	updateCnt int
	deleteCnt int
}

func (r *idorRuleRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Rule, error) {
	r.getOrgIDs = append(r.getOrgIDs, organizationID)
	for _, rule := range r.rules {
		if rule.ID == id && rule.OrganizationID == organizationID {
			return rule, nil
		}
	}
	return nil, apperrors.NewNotFound("rule not found")
}

func (r *idorRuleRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Rule, int64, error) {
	return r.rules, int64(len(r.rules)), nil
}

func (r *idorRuleRepo) Create(_ context.Context, rule *domain.Rule) error {
	r.rules = append(r.rules, rule)
	return nil
}

func (r *idorRuleRepo) Update(_ context.Context, rule *domain.Rule) error {
	r.updateCnt++
	for i, rl := range r.rules {
		if rl.ID == rule.ID && rl.OrganizationID == rule.OrganizationID {
			r.rules[i] = rule
			return nil
		}
	}
	return apperrors.NewNotFound("rule not found")
}

func (r *idorRuleRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, rule := range r.rules {
		if rule.ID == id && rule.OrganizationID == organizationID {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("rule not found")
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

func (u *idorUOW) Rules() domain.Repository { return u.repo }
func (u *idorUOW) Records() recordsdomain.Repository {
	return u.records
}
func (u *idorUOW) Outbox() domainoutbox.Repository { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newTenantService(repo domain.Repository) (apprules.Service, *idorRecords) {
	records := &idorRecords{}
	return apprules.Service{UOW: &idorUOW{repo: repo, records: records}, Clock: idorClock{}, UUID: idorUUID{}}, records
}

func ruleFixture(id, orgID string) *domain.Rule {
	// BE-D-22 后写路径整体校验条件：IP 规则必须携带合法表达式，
	// 无条件 fixture 是不可能经 API 创建的非法存量行。
	return &domain.Rule{ID: id, OrganizationID: orgID, Name: "tenant-rule", Scope: "org", RuleType: "IP", Action: "Allow", Revision: 1,
		Conditions: []map[string]any{{"operator": "equals", "value": "192.168.1.1"}}}
}

func tenantRouter(service apprules.Service) chi.Router {
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

func TestRuleByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/rules/" + foreignRuleID + "?organizationId=" + foreignOrgID, ""},
		{"patch", http.MethodPatch, "/api/v1/rules/" + foreignRuleID + "?organizationId=" + foreignOrgID, `{"name":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/rules/" + foreignRuleID + "?organizationId=" + foreignOrgID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorRuleRepo{rules: []*domain.Rule{ruleFixture(foreignRuleID, foreignOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), tenantRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign rule must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the rule")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the rule")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, []string{tenantOrgID}, repo.getOrgIDs, "repository must receive the trusted session tenant")
			require.Len(t, repo.rules, 1, "foreign rule row must remain untouched")
		})
	}
}

func TestRuleByID_WithoutTenantScopeRejected(t *testing.T) {
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
			repo := &idorRuleRepo{rules: []*domain.Rule{ruleFixture(ownRuleID, tenantOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), bareRequest(tt.method, "/api/v1/rules/"+ownRuleID, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.getOrgIDs, "an empty tenant must not reach the repository")
		})
	}
}

func TestRuleByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorRuleRepo{rules: []*domain.Rule{ruleFixture(ownRuleID, tenantOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodGet, "/api/v1/rules/"+ownRuleID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+tenantOrgID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorRuleRepo{rules: []*domain.Rule{ruleFixture(ownRuleID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			tenantRequest(http.MethodPatch, "/api/v1/rules/"+ownRuleID, strings.NewReader(`{"name":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorRuleRepo{rules: []*domain.Rule{ruleFixture(ownRuleID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodDelete, "/api/v1/rules/"+ownRuleID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.rules)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID, "delete audit must carry the rule organization")
	})
}

func TestRuleByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorRuleRepo{rules: []*domain.Rule{ruleFixture(foreignRuleID, foreignOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodGet, "/api/v1/rules/"+foreignRuleID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), foreignOrgID)
	})

	t.Run("delete audits the rule tenant", func(t *testing.T) {
		repo := &idorRuleRepo{rules: []*domain.Rule{ruleFixture(foreignRuleID, foreignOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodDelete, "/api/v1/rules/"+foreignRuleID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, foreignOrgID, records.created[0].OrganizationID)
	})
}
