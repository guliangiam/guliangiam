package risk

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apprisk "github.com/guliangiam/guliangiam/internal/app/risk"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	outboxdomain "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	httpOrgID        = "11111111-1111-4111-8111-111111111111"
	httpOtherOrgID   = "77777777-7777-4777-8777-777777777777"
	httpDecisionID   = "22222222-2222-4222-8222-222222222222"
	httpActorID      = "33333333-3333-4333-8333-333333333333"
	httpSessionID    = "44444444-4444-4444-8444-444444444444"
	httpGovernanceID = "55555555-5555-4555-8555-555555555555"
	httpNewGovID     = "66666666-6666-4666-8666-666666666666"
)

var httpTestNow = time.Date(2026, 10, 26, 11, 0, 0, 0, time.UTC)

type httpClock struct{}

func (httpClock) Now() time.Time { return httpTestNow }

type httpUUID struct{}

func (httpUUID) NewString() (string, error) { return httpNewGovID, nil }

type httpRiskRepository struct {
	domain.Repository
	decision           *domain.Decision
	governance         *domain.SessionGovernance
	auditChain         *domain.AuditChain
	lastOrganizationID string
	lastAuditQuery     domain.AuditIntegrityQuery
	appended           []domain.SessionGovernance
}

func (r *httpRiskRepository) AppendSignal(_ context.Context, signal domain.Signal) error {
	return nil
}

func (r *httpRiskRepository) CreateDecision(_ context.Context, decision domain.Decision) error {
	copy := decision
	r.decision = &copy
	return nil
}

func (r *httpRiskRepository) GetDecision(_ context.Context, id, organizationID string) (*domain.Decision, error) {
	return r.decisionFor(id, organizationID)
}

func (r *httpRiskRepository) GetDecisionForUpdate(_ context.Context, id, organizationID string) (*domain.Decision, error) {
	return r.decisionFor(id, organizationID)
}

func (r *httpRiskRepository) decisionFor(id, organizationID string) (*domain.Decision, error) {
	r.lastOrganizationID = organizationID
	if r.decision == nil || r.decision.ID != id || r.decision.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("risk decision not found")
	}
	copy := *r.decision
	return &copy, nil
}

func (r *httpRiskRepository) UpdateDecision(_ context.Context, cmd domain.UpdateDecisionCommand) error {
	if r.decision == nil || r.decision.ID != cmd.DecisionID || r.decision.OrganizationID != cmd.OrganizationID ||
		r.decision.Decision != cmd.ExpectedDecision || !bytes.Equal(r.decision.Factors, cmd.ExpectedFactors) {
		return apperrors.NewConflict("risk decision revision mismatch")
	}
	r.decision.Decision = cmd.NewDecision
	r.decision.Factors = cmd.NewFactors
	return nil
}

func (r *httpRiskRepository) GetGovernance(_ context.Context, id, organizationID string) (*domain.SessionGovernance, error) {
	return r.governanceFor(id, organizationID)
}

func (r *httpRiskRepository) GetGovernanceForTransition(_ context.Context, id, organizationID string) (*domain.SessionGovernance, error) {
	return r.governanceFor(id, organizationID)
}

func (r *httpRiskRepository) governanceFor(id, organizationID string) (*domain.SessionGovernance, error) {
	r.lastOrganizationID = organizationID
	if r.governance == nil || r.governance.ID != id || r.governance.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("session governance not found")
	}
	copy := *r.governance
	return &copy, nil
}

func (r *httpRiskRepository) AppendGovernance(_ context.Context, governance domain.SessionGovernance) error {
	r.appended = append(r.appended, governance)
	copy := governance
	r.governance = &copy
	return nil
}

func (r *httpRiskRepository) ListAuditChain(_ context.Context, query domain.AuditIntegrityQuery) (*domain.AuditChain, error) {
	r.lastOrganizationID = query.OrganizationID
	r.lastAuditQuery = query
	if r.auditChain == nil {
		return &domain.AuditChain{}, nil
	}
	return r.auditChain, nil
}

type httpRecordsRepository struct {
	recordsdomain.Repository
	records []*recordsdomain.Record
}

func (r *httpRecordsRepository) Create(_ context.Context, record *recordsdomain.Record) error {
	copy := *record
	r.records = append(r.records, &copy)
	return nil
}

type httpUnitOfWork struct {
	appuow.UnitOfWork
	risk    *httpRiskRepository
	records *httpRecordsRepository
	commits int
}

func (u *httpUnitOfWork) Risk() domain.Repository { return u.risk }

func (u *httpUnitOfWork) Records() recordsdomain.Repository { return u.records }

func (u *httpUnitOfWork) Outbox() outboxdomain.Repository { return nil }

func (u *httpUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	if err := fn(ctx, u); err != nil {
		return err
	}
	u.commits++
	return nil
}

type httpFixture struct {
	handler Handler
	repo    *httpRiskRepository
	records *httpRecordsRepository
	uow     *httpUnitOfWork
}

func newHTTPFixture() *httpFixture {
	repo := &httpRiskRepository{decision: &domain.Decision{
		ID: httpDecisionID, OrganizationID: httpOrgID, Decision: domain.DecisionChallenge,
		Score: 80, Factors: json.RawMessage(`[{"type":"new_device"}]`), CreatedAt: httpTestNow.Add(-time.Minute),
	}}
	records := &httpRecordsRepository{}
	uow := &httpUnitOfWork{risk: repo, records: records}
	return &httpFixture{
		handler: Handler{Service: apprisk.Service{Clock: httpClock{}, UUID: httpUUID{}, UOW: uow}},
		repo:    repo, records: records, uow: uow,
	}
}

func testRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	return router
}

func newRiskRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("User-Agent", "risk-http-test")
	request.RemoteAddr = "198.51.100.7:4242"
	ctx := tenantctx.WithTenantScope(request.Context(), httpOrgID, "", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{UserID: httpActorID, OrganizationID: httpOrgID})
	ctx = route.WithRequestMetadata(ctx, route.RequestMetadata{
		Surface: route.SurfaceManagement, RequestID: "request-risk-1", TraceID: "trace-risk-1",
	})
	return request.WithContext(ctx)
}

func serveRiskRequest(handler Handler, request *http.Request) *httptest.ResponseRecorder {
	router := testRouter()
	RegisterChiRoutes(router, handler)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertRiskError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code apperrors.Code) {
	t.Helper()
	assert.Equal(t, status, recorder.Code)
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, string(code), response.Error.Code)
}

func TestReviewRoute_UsesTenantSessionAndRequestMetadata(t *testing.T) {
	fixture := newHTTPFixture()
	request := newRiskRequest(http.MethodPost,
		"/api/v1/risk/decisions/"+httpDecisionID+"/review?organizationId="+httpOtherOrgID,
		`{"decision":"deny","reason":"confirmed risk","revision":0}`)
	request.Header.Set("X-Organization-Id", httpOtherOrgID)
	recorder := serveRiskRequest(fixture.handler, request)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, httpOrgID, fixture.repo.lastOrganizationID)
	assert.Equal(t, domain.DecisionDeny, fixture.repo.decision.Decision)
	assert.Equal(t, 1, fixture.uow.commits)
	require.Len(t, fixture.records.records, 1)
	audit := fixture.records.records[0]
	assert.Equal(t, "risk.decision.reviewed", audit.Action)
	assert.Equal(t, httpOrgID, audit.OrganizationID)
	assert.Equal(t, httpActorID, audit.ActorID)
	assert.Equal(t, httpDecisionID, audit.ResourceID)
	assert.Equal(t, "198.51.100.7", audit.IP)
	assert.Equal(t, "risk-http-test", audit.UserAgent)
	assert.Equal(t, "management", audit.Surface)
	assert.Equal(t, "request-risk-1", audit.RequestID)
	assert.Equal(t, "trace-risk-1", audit.TraceID)
}

func TestAppealAndResolutionRoutes(t *testing.T) {
	fixture := newHTTPFixture()
	fixture.repo.decision.Decision = domain.DecisionDeny

	appeal := serveRiskRequest(fixture.handler, newRiskRequest(http.MethodPost,
		"/api/v1/risk/decisions/"+httpDecisionID+"/appeal",
		`{"reason":"managed device was misclassified","revision":0}`))
	assert.Equal(t, http.StatusCreated, appeal.Code)

	resolution := serveRiskRequest(fixture.handler, newRiskRequest(http.MethodPost,
		"/api/v1/risk/decisions/"+httpDecisionID+"/appeal/resolution",
		`{"resolution":"accepted","reason":"device evidence verified","revision":1}`))
	assert.Equal(t, http.StatusCreated, resolution.Code)
	var response apprisk.DecisionDTO
	require.NoError(t, json.Unmarshal(resolution.Body.Bytes(), &response))
	assert.Equal(t, string(domain.DecisionDeny), response.Decision)
	assert.Equal(t, domain.AppealAccepted, response.Workflow.Appeal.Status)
	assert.Equal(t, int64(2), response.Revision)
	require.Len(t, fixture.records.records, 2)
	assert.Equal(t, "risk.decision.appeal.submitted", fixture.records.records[0].Action)
	assert.Equal(t, "risk.decision.appeal.accepted", fixture.records.records[1].Action)
}

func TestTransitionGovernanceRoute(t *testing.T) {
	fixture := newHTTPFixture()
	sessionID := httpSessionID
	fixture.repo.governance = &domain.SessionGovernance{
		ID: httpGovernanceID, OrganizationID: httpOrgID, SessionID: &sessionID,
		RiskScore: 90, Action: domain.GovernanceStepUp, CreatedAt: httpTestNow.Add(-time.Minute), Revision: 1,
	}
	recorder := serveRiskRequest(fixture.handler, newRiskRequest(http.MethodPost,
		"/api/v1/risk/session-governance/"+httpGovernanceID+"/transition",
		`{"action":"revoke","reason":"step-up failed","revision":1}`))

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, httpOrgID, fixture.repo.lastOrganizationID)
	require.Len(t, fixture.repo.appended, 1)
	assert.Equal(t, httpNewGovID, fixture.repo.appended[0].ID)
	assert.Equal(t, domain.GovernanceRevoke, fixture.repo.appended[0].Action)
	assert.Equal(t, int64(2), fixture.repo.appended[0].Revision)
	require.Len(t, fixture.records.records, 1)
	assert.Equal(t, "risk.session_governance.transitioned", fixture.records.records[0].Action)
}

func TestAuditIntegrityRoute_IsReadOnlyAndTenantScoped(t *testing.T) {
	fixture := newHTTPFixture()
	request := newRiskRequest(http.MethodGet,
		"/api/v1/risk/audit-integrity?organizationId="+httpOtherOrgID, "")
	request.Header.Set("X-Organization-Id", httpOtherOrgID)
	recorder := serveRiskRequest(fixture.handler, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, httpOrgID, fixture.repo.lastAuditQuery.OrganizationID)
	assert.Equal(t, httpOrgID, fixture.repo.lastAuditQuery.ChainID)
	assert.Zero(t, fixture.uow.commits)
	assert.Empty(t, fixture.records.records)
	var response apprisk.AuditIntegrityDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Valid)
	assert.Equal(t, "valid", response.Status)
	assert.Equal(t, "2026-10-26T11:00:00.000Z", response.VerifiedAt)
}

func TestDetailRoutes_UseTenantContext(t *testing.T) {
	fixture := newHTTPFixture()
	decision := serveRiskRequest(fixture.handler, newRiskRequest(http.MethodGet,
		"/api/v1/risk/decisions/"+httpDecisionID+"?organizationId="+httpOtherOrgID, ""))
	assert.Equal(t, http.StatusOK, decision.Code)
	assert.Equal(t, httpOrgID, fixture.repo.lastOrganizationID)

	sessionID := httpSessionID
	fixture.repo.governance = &domain.SessionGovernance{
		ID: httpGovernanceID, OrganizationID: httpOrgID, SessionID: &sessionID,
		Action: domain.GovernanceMonitor, CreatedAt: httpTestNow, Revision: 1,
	}
	governance := serveRiskRequest(fixture.handler, newRiskRequest(http.MethodGet,
		"/api/v1/risk/session-governance/"+httpGovernanceID+"?organizationId="+httpOtherOrgID, ""))
	assert.Equal(t, http.StatusOK, governance.Code)
	assert.Equal(t, httpOrgID, fixture.repo.lastOrganizationID)
}

func TestWorkflowRoutes_ReturnStructuredValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		code   apperrors.Code
	}{
		{name: "missing revision", method: http.MethodPost, path: "/api/v1/risk/decisions/" + httpDecisionID + "/review", body: `{"decision":"deny","reason":"reason"}`, status: http.StatusBadRequest, code: apperrors.CodeInvalidArgument},
		{name: "missing reason", method: http.MethodPost, path: "/api/v1/risk/decisions/" + httpDecisionID + "/appeal", body: `{"revision":0}`, status: http.StatusBadRequest, code: apperrors.CodeInvalidArgument},
		{name: "unknown field", method: http.MethodPost, path: "/api/v1/risk/decisions/" + httpDecisionID + "/review", body: `{"decision":"deny","reason":"reason","revision":0,"organizationId":"` + httpOtherOrgID + `"}`, status: http.StatusBadRequest, code: apperrors.CodeInvalidArgument},
		// The action parses and binds; the closed set rejects it — 422, not 400
		// (手册 4.14.1: the caller succeeds by changing one field).
		{name: "invalid governance action", method: http.MethodPost, path: "/api/v1/risk/session-governance/" + httpGovernanceID + "/transition", body: `{"action":"allow","reason":"reason","revision":1}`, status: http.StatusUnprocessableEntity, code: apperrors.CodeUnprocessableEntity},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHTTPFixture()
			recorder := serveRiskRequest(fixture.handler, newRiskRequest(test.method, test.path, test.body))
			assertRiskError(t, recorder, test.status, test.code)
			assert.Zero(t, fixture.uow.commits)
			assert.Empty(t, fixture.records.records)
		})
	}

	t.Run("missing tenant context", func(t *testing.T) {
		fixture := newHTTPFixture()
		request := newRiskRequest(http.MethodGet, "/api/v1/risk/audit-integrity", "")
		ctx := tenantctx.WithTenantScope(request.Context(), "", "", tenantctx.TenantFilterStrict)
		recorder := serveRiskRequest(fixture.handler, request.WithContext(ctx))
		assertRiskError(t, recorder, http.StatusBadRequest, apperrors.CodeInvalidArgument)
	})
}

func TestRouteDefinitionsMatchChiRoutes(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{})
	actual := make(map[string]bool)
	require.NoError(t, chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		actual[method+" "+pattern] = true
		return nil
	}))
	definitions := Routes()
	assert.Len(t, actual, len(definitions))
	for _, definition := range definitions {
		assert.True(t, actual[definition.Method+" "+definition.Pattern], definition.Method+" "+definition.Pattern)
	}
}
