package privacy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appprivacy "github.com/guliangiam/guliangiam/internal/app/privacy"
	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDsarRepo struct {
	items         map[string]*domain.DsarRequest
	exportRecords []domain.ExportRecord
	exportRegion  string
}

func newFakeDsarRepo() *fakeDsarRepo { return &fakeDsarRepo{items: map[string]*domain.DsarRequest{}} }

func (f *fakeDsarRepo) Create(ctx context.Context, request *domain.DsarRequest, event *domain.DsarEvent) error {
	f.items[request.ID] = request
	return nil
}
func (f *fakeDsarRepo) CollectSubjectData(ctx context.Context, organizationID, subjectType, subjectID string) ([]domain.ExportRecord, string, error) {
	return f.exportRecords, f.exportRegion, nil
}

func (f *fakeDsarRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.DsarRequest, error) {
	request, ok := f.items[id]
	if !ok || request.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("dsar request not found")
	}
	return request, nil
}
func (f *fakeDsarRepo) UpdateWithEvent(ctx context.Context, request *domain.DsarRequest, event *domain.DsarEvent, expectedRevision int64) error {
	f.items[request.ID] = request
	return nil
}
func (f *fakeDsarRepo) ListOverdue(ctx context.Context, now time.Time, limit int) ([]*domain.DsarRequest, error) {
	return nil, nil
}
func (f *fakeDsarRepo) ListByOrganization(ctx context.Context, organizationID string, limit, offset int) ([]*domain.DsarRequest, int64, error) {
	items := make([]*domain.DsarRequest, 0, len(f.items))
	for _, request := range f.items {
		if request.OrganizationID == organizationID {
			items = append(items, request)
		}
	}
	return items, int64(len(items)), nil
}

func newPrivacyTestRouter(t *testing.T) (chi.Router, *fakeDsarRepo) {
	t.Helper()
	repo := newFakeDsarRepo()
	service := appprivacy.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, fakePrivacyRunner{repo: repo})
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router, repo
}

func doRequest(t *testing.T, router chi.Router, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(tenantctx.WithTenantScope(request.Context(), "org-1", "org", tenantctx.TenantFilterStrict))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestPrivacyRoutes_CreateListTransition(t *testing.T) {
	router, repo := newPrivacyTestRouter(t)

	created := doRequest(t, router, http.MethodPost, "/api/v1/privacy/dsar", `{"subjectType":"user","subjectId":"user-1","requestType":"access"}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var response DsarResponse
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &response))
	assert.Equal(t, "received", response.State)
	assert.Equal(t, "org-1", response.OrganizationID)
	t.Logf("created body: %s", created.Body.String())
	t.Logf("repo ids: %v", func() []string {
		ids := make([]string, 0, len(repo.items))
		for id := range repo.items {
			ids = append(ids, id)
		}
		return ids
	}())
	assert.Len(t, repo.items, 1)

	list := doRequest(t, router, http.MethodGet, "/api/v1/privacy/dsar?page=1&pageSize=10", "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())

	transition := doRequest(t, router, http.MethodPost, "/api/v1/privacy/dsar/"+response.ID+"/transition", `{"target":"verifying","revision":1}`)
	require.Equal(t, http.StatusOK, transition.Code, transition.Body.String())
	var updated DsarResponse
	require.NoError(t, json.Unmarshal(transition.Body.Bytes(), &updated))
	assert.Equal(t, "verifying", updated.State)
	assert.Equal(t, int64(2), updated.Revision)

	invalid := doRequest(t, router, http.MethodPost, "/api/v1/privacy/dsar/"+response.ID+"/transition", `{"target":"fulfilled"}`)
	require.Equal(t, http.StatusUnprocessableEntity, invalid.Code, invalid.Body.String())
}

func TestRouteDefinitionsMatchChiRoutes(t *testing.T) {
	router, _ := newPrivacyTestRouter(t)
	actual := make(map[string]bool)
	require.NoError(t, chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		actual[method+" "+pattern] = true
		return nil
	}))
	definitions := Routes()
	for _, definition := range definitions {
		assert.True(t, actual[definition.Method+" "+definition.Pattern], definition.Method+" "+definition.Pattern)
	}
}

type fakePrivacyTx struct{ repo *fakeDsarRepo }

func (t fakePrivacyTx) Privacy() domain.DsarRepository { return t.repo }

func (t fakePrivacyTx) Consents() domain.ConsentRepository { return nil }

func (t fakePrivacyTx) EmbedAudit(ctx context.Context, event appprivacy.AuditEvent) error { return nil }

type fakePrivacyRunner struct{ repo *fakeDsarRepo }

func (r fakePrivacyRunner) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appprivacy.Tx) error) error {
	return fn(ctx, fakePrivacyTx{repo: r.repo})
}

func TestPrivacyRoutes_ExportFulfilledAccessRequest(t *testing.T) {
	router, repo := newPrivacyTestRouter(t)
	repo.items["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
		State: domain.StateFulfilled, Revision: 2,
	}
	repo.exportRecords = []domain.ExportRecord{{Category: "user", SubjectID: "user-1", Payload: map[string]any{"username": "alice"}}}
	repo.exportRegion = "cn"

	recorder := doRequest(t, router, http.MethodGet, "/api/v1/privacy/exports/dsar-1", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Header().Get("Content-Type"), "application/x-ndjson")
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "privacy-export-dsar-1.jsonl")
	assert.Equal(t, "cn", recorder.Header().Get("X-Region"))
	assert.Equal(t, "1", recorder.Header().Get("X-Record-Count"))
	assert.NotEmpty(t, recorder.Header().Get("X-Checksum-Sha256"))
	assert.Contains(t, recorder.Body.String(), "\"category\":\"user\"")
}

func TestPrivacyRoutes_ExportRequiresFulfilledState(t *testing.T) {
	router, repo := newPrivacyTestRouter(t)
	repo.items["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
		State: domain.StateInProgress, Revision: 2,
	}

	recorder := doRequest(t, router, http.MethodGet, "/api/v1/privacy/exports/dsar-1", "")
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
}

func TestPrivacyRoutes_ReviewErasureScope(t *testing.T) {
	router, repo := newPrivacyTestRouter(t)
	repo.items["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeErase,
		State: domain.StateInProgress, Revision: 2,
	}
	body := `{"targets":[{"category":"profile","targetId":"user-1"},{"category":"audit","targetId":"user-1"}],"retentions":[{"category":"audit","legalBasis":"legal-obligation"}]}`
	recorder := doRequest(t, router, http.MethodPost, "/api/v1/privacy/dsar/dsar-1/erase-review", body)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"category":"profile"`)
	assert.Contains(t, recorder.Body.String(), `"legalBasis":"legal-obligation"`)
}

func TestPrivacyRoutes_ReviewErasureBlocked(t *testing.T) {
	router, repo := newPrivacyTestRouter(t)
	repo.items["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeErase,
		State: domain.StateInProgress, Revision: 2,
	}
	body := `{"targets":[{"category":"audit","targetId":"user-1"}],"retentions":[{"category":"audit","legalBasis":"legal-obligation","blocks":true}]}`
	recorder := doRequest(t, router, http.MethodPost, "/api/v1/privacy/dsar/dsar-1/erase-review", body)
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
}
