package records

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	apprecords "github.com/guliangiam/guliangiam/internal/app/records"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	regressionSessionOrg  = "11111111-1111-1111-1111-111111111111"
	regressionForgedOrg   = "22222222-2222-2222-2222-222222222222"
	regressionSessionUser = "33333333-3333-3333-3333-333333333333"
	regressionUserAgent   = "tenant-audit-regression/1.0"
	regressionRecordID    = "44444444-4444-4444-4444-444444444444"
)

type regressionClock struct{}

func (regressionClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type regressionUUID struct{}

func (regressionUUID) NewString() (string, error) { return regressionRecordID, nil }

// regressionRequest builds a request carrying an authenticated session, so the
// handler sees a trusted tenant/actor even when the payload carries another one.
func regressionRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", regressionUserAgent)
	return req.WithContext(authtransport.WithSessionContext(req.Context(), authtransport.SessionContext{
		UserID:         regressionSessionUser,
		OrganizationID: regressionSessionOrg,
	}))
}

func regressionRouter(uow *appuowmock.MockUnitOfWork) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apprecords.NewService(regressionClock{}, regressionUUID{}, uow)})
	return router
}

// TestListRecords_TenantScopeOverridesQueryOrganizationID pins GAP-011: a
// query-supplied organizationId must not become the repository filter when the
// session already carries an organization.
func TestListRecords_TenantScopeOverridesQueryOrganizationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	uow.EXPECT().Records().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Record, int64, error) {
			assert.Equal(t, regressionSessionOrg, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	regressionRouter(uow).ServeHTTP(recorder, regressionRequest(
		http.MethodGet, "/api/v1/records?organizationId="+regressionForgedOrg, ""))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestSearchRecords_TenantScopeOverridesBodyOrganizationID pins the search
// variant of GAP-011: the POST body is client-controlled just like the query.
func TestSearchRecords_TenantScopeOverridesBodyOrganizationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	uow.EXPECT().Records().Return(repo).AnyTimes()
	repo.EXPECT().Search(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, criteria domain.SearchCriteria) ([]*domain.Record, int64, error) {
			assert.Equal(t, regressionSessionOrg, criteria.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	regressionRouter(uow).ServeHTTP(recorder, regressionRequest(
		http.MethodPost, "/api/v1/records/search",
		`{"organizationId":"`+regressionForgedOrg+`","page":1,"pageSize":10}`))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestSearchRecords_DefaultsAreEchoed pins A3: page/pageSize 0 in the body mean
// "omitted", so the response must report the page the repository actually served
// (1/DefaultPageSize) instead of echoing the raw request.
func TestSearchRecords_DefaultsAreEchoed(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	uow.EXPECT().Records().Return(repo).AnyTimes()
	repo.EXPECT().Search(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)

	recorder := httptest.NewRecorder()
	regressionRouter(uow).ServeHTTP(recorder, regressionRequest(
		http.MethodPost, "/api/v1/records/search", `{"page":0,"pageSize":0}`))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body ListRecordsResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, 1, body.Page.Page)
	require.Equal(t, platformpagination.DefaultPageSize, body.Page.PageSize)
	require.Equal(t, int64(0), body.Page.Total)
	require.NotNil(t, body.Items, "empty search result must serialize as [] rather than null")
}

// TestCreateRecord_ActorIPUserAgentDerivedFromRequest pins GAP-012: the audit
// actor and its origin are evidence, so the body cannot supply them.
func TestCreateRecord_ActorIPUserAgentDerivedFromRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	uow.EXPECT().Records().Return(repo).AnyTimes()
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record *domain.Record) error {
			assert.Equal(t, regressionSessionOrg, record.OrganizationID)
			assert.Equal(t, regressionSessionUser, record.ActorID)
			// httptest.NewRequest defaults to RemoteAddr 192.0.2.1:1234; the
			// body's forged IP must never win over the observed connection.
			assert.Equal(t, "192.0.2.1", record.IP)
			assert.Equal(t, regressionUserAgent, record.UserAgent)
			assert.Equal(t, "record.regression", record.Action)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	regressionRouter(uow).ServeHTTP(recorder, regressionRequest(
		http.MethodPost, "/api/v1/records",
		`{"organizationId":"`+regressionForgedOrg+`","actorId":"99999999-9999-9999-9999-999999999999",`+
			`"ip":"203.0.113.9","userAgent":"forged-agent/1.0",`+
			`"action":"record.regression","resourceType":"record","resourceId":"`+regressionRecordID+`"}`))
	require.Equal(t, http.StatusCreated, recorder.Code)
}
