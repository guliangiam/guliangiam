package identitypools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appw "github.com/guliangiam/guliangiam/internal/app/workload"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	tokenmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
	forgedOrgPoolID    = "44444444-4444-4444-4444-444444444444"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

// forgedOrgRepo is a narrow workload repository: only the methods exercised by
// the write routes are implemented, the embedded nil interface covers the rest.
type forgedOrgRepo struct {
	appw.Repository
	pool     *appw.Pool
	trust    *appw.FederationTrust
	account  *appw.ServiceAccount
	lastOrg  string
	lastOp   string
	lastID   string
	lastPool string
	now      time.Time
}

func (r *forgedOrgRepo) GetPool(_ context.Context, _, orgID string) (*appw.Pool, error) {
	r.lastOrg = orgID
	if r.pool == nil || r.pool.OrganizationID != orgID {
		return nil, nil
	}
	return r.pool, nil
}

func (r *forgedOrgRepo) CreatePool(_ context.Context, pool *appw.Pool) error {
	r.lastOrg, r.lastOp, r.lastID = pool.OrganizationID, "CreatePool", pool.ID
	r.now = pool.CreatedAt
	return nil
}

func (r *forgedOrgRepo) ListPools(_ context.Context, q appw.ListPoolsQuery) ([]*appw.Pool, int64, error) {
	r.lastOrg, r.lastOp = q.OrganizationID, "ListPools"
	return nil, 0, nil
}

func (r *forgedOrgRepo) GetTrust(_ context.Context, id, orgID string) (*appw.FederationTrust, error) {
	r.lastOrg = orgID
	if r.trust == nil || r.trust.ID != id || r.trust.OrganizationID != orgID {
		return nil, nil
	}
	return r.trust, nil
}

func (r *forgedOrgRepo) CreateTrust(_ context.Context, trust *appw.FederationTrust) error {
	r.lastOrg, r.lastOp, r.lastID, r.lastPool = trust.OrganizationID, "CreateTrust", trust.ID, trust.PoolID
	r.now = trust.CreatedAt
	return nil
}

func (r *forgedOrgRepo) SetTrustStatus(_ context.Context, id, orgID string, _ appw.FederationTrustStatus) error {
	r.lastOrg, r.lastOp, r.lastID = orgID, "SetTrustStatus", id
	return nil
}

func (r *forgedOrgRepo) DeleteTrust(_ context.Context, id, orgID string) error {
	r.lastOrg, r.lastOp, r.lastID = orgID, "DeleteTrust", id
	return nil
}

func (r *forgedOrgRepo) GetAccount(_ context.Context, id, orgID string) (*appw.ServiceAccount, error) {
	r.lastOrg = orgID
	if r.account == nil || r.account.ID != id || r.account.OrganizationID != orgID {
		return nil, nil
	}
	return r.account, nil
}

func (r *forgedOrgRepo) CreateAccount(_ context.Context, account *appw.ServiceAccount) error {
	r.lastOrg, r.lastOp, r.lastID, r.lastPool = account.OrganizationID, "CreateAccount", account.ID, account.PoolID
	r.now = account.CreatedAt
	return nil
}

func (r *forgedOrgRepo) SetAccountStatus(_ context.Context, id, orgID string, _ appw.ServiceAccountStatus, _ int64) error {
	r.lastOrg, r.lastOp, r.lastID = orgID, "SetAccountStatus", id
	return nil
}

func (r *forgedOrgRepo) RotateAccountSecret(_ context.Context, id, orgID, _ string, _ time.Time, _ int64) error {
	r.lastOrg, r.lastOp, r.lastID = orgID, "RotateAccountSecret", id
	return nil
}

func (r *forgedOrgRepo) DeleteAccount(_ context.Context, id, orgID string, _ int64) error {
	r.lastOrg, r.lastOp, r.lastID = orgID, "DeleteAccount", id
	return nil
}

func forgedOrgRouter(ctrl *gomock.Controller, repo *forgedOrgRepo) (chi.Router, *appuowmock.MockUnitOfWork, *tokenmock.MockRepository) {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	tokenRepo := tokenmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Workload().Return(repo).AnyTimes()
	uow.EXPECT().Tokens().Return(tokenRepo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appw.Service{UOW: uow, Clock: forgedOrgClock{}}})
	return router, uow, tokenRepo
}

func newWorkloadPool() *appw.Pool {
	return &appw.Pool{ID: forgedOrgPoolID, OrganizationID: forgedOrgSessionID, Name: "pool", CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(), Revision: 1}
}

func assertForgedOrgIgnored(t *testing.T, repo *forgedOrgRepo, wantOp, wantID string) {
	t.Helper()
	assert.Equal(t, wantOp, repo.lastOp)
	assert.Equal(t, forgedOrgSessionID, repo.lastOrg)
	if wantID != "" {
		assert.Equal(t, wantID, repo.lastID)
	}
}

// assertCreatedLocation pins the 201 contract of the create routes: the body
// keeps its resource representation and Location names the same resource under
// the collection path the caller used.
func assertCreatedLocation(t *testing.T, recorder *httptest.ResponseRecorder, collectionPath string) {
	t.Helper()
	var body struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
	require.NotEmpty(t, body.ID, "the created representation must carry its id")
	assert.Equal(t, collectionPath+body.ID, recorder.Header().Get("Location"))
}

func TestListPools_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := &forgedOrgRepo{}
	router, _, _ := forgedOrgRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/identity-pools?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assertForgedOrgIgnored(t, repo, "ListPools", "")
}

func TestCreatePool_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	repo := &forgedOrgRepo{}
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
		"/api/v1/identity-pools?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"name":"pool","displayName":"Pool"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
	assertForgedOrgIgnored(t, repo, "CreatePool", "")
	assertCreatedLocation(t, recorder, "/api/v1/identity-pools/")
}

func TestCreateTrust_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	repo := &forgedOrgRepo{pool: newWorkloadPool()}
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
		"/api/v1/identity-pools/"+forgedOrgPoolID+"/federation-trusts?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"name":"trust","issuer":"https://issuer.example.com","jwksUri":"https://issuer.example.com/jwks.json","audience":["api"],"subjects":["repo:org/*"]}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
	assertForgedOrgIgnored(t, repo, "CreateTrust", "")
	assertCreatedLocation(t, recorder, "/api/v1/identity-pools/"+forgedOrgPoolID+"/federation-trusts/")
}

func TestSetTrustStatus_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	trust := &appw.FederationTrust{ID: "trust-1", OrganizationID: forgedOrgSessionID, PoolID: forgedOrgPoolID, Status: appw.TrustStatusActive}
	repo := &forgedOrgRepo{trust: trust}
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPatch,
		"/api/v1/identity-pools/"+forgedOrgPoolID+"/federation-trusts/trust-1/status?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"status":"disabled"}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assertForgedOrgIgnored(t, repo, "SetTrustStatus", "trust-1")
}

func TestDeleteTrust_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	trust := &appw.FederationTrust{ID: "trust-1", OrganizationID: forgedOrgSessionID, PoolID: forgedOrgPoolID, Status: appw.TrustStatusActive}
	repo := &forgedOrgRepo{trust: trust}
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodDelete,
		"/api/v1/identity-pools/"+forgedOrgPoolID+"/federation-trusts/trust-1?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assertForgedOrgIgnored(t, repo, "DeleteTrust", "trust-1")
}

func TestCreateAccount_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	repo := &forgedOrgRepo{pool: newWorkloadPool()}
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
		"/api/v1/identity-pools/"+forgedOrgPoolID+"/service-accounts?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"name":"robot","description":"CI"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
	assertForgedOrgIgnored(t, repo, "CreateAccount", "")
	assertCreatedLocation(t, recorder, "/api/v1/identity-pools/"+forgedOrgPoolID+"/service-accounts/")
}

func TestSetAccountStatus_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	account := &appw.ServiceAccount{ID: "sa-1", OrganizationID: forgedOrgSessionID, PoolID: forgedOrgPoolID, Status: appw.SAStatusActive, Revision: 1}
	repo := &forgedOrgRepo{account: account}
	ctrl := gomock.NewController(t)
	router, _, _ := forgedOrgRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPatch,
		"/api/v1/identity-pools/"+forgedOrgPoolID+"/service-accounts/sa-1/status?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"status":"active","revision":1}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assertForgedOrgIgnored(t, repo, "SetAccountStatus", "sa-1")
}

func TestRotateAccountSecret_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	account := &appw.ServiceAccount{ID: "sa-1", OrganizationID: forgedOrgSessionID, PoolID: forgedOrgPoolID, Status: appw.SAStatusActive, Revision: 1}
	repo := &forgedOrgRepo{account: account}
	ctrl := gomock.NewController(t)
	router, _, tokenRepo := forgedOrgRouter(ctrl, repo)
	tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
		"/api/v1/identity-pools/"+forgedOrgPoolID+"/service-accounts/sa-1/rotate-secret?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"revision":1}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assertForgedOrgIgnored(t, repo, "RotateAccountSecret", "sa-1")
}

func TestDeleteAccount_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	account := &appw.ServiceAccount{ID: "sa-1", OrganizationID: forgedOrgSessionID, PoolID: forgedOrgPoolID, Status: appw.SAStatusActive, Revision: 1}
	repo := &forgedOrgRepo{account: account}
	ctrl := gomock.NewController(t)
	router, _, tokenRepo := forgedOrgRouter(ctrl, repo)
	tokenRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodDelete,
		"/api/v1/identity-pools/"+forgedOrgPoolID+"/service-accounts/sa-1?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"revision":1}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
	assertForgedOrgIgnored(t, repo, "DeleteAccount", "sa-1")
}

var _ tokendomain.Repository = (*tokenmock.MockRepository)(nil)
