package keys

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-key-id", nil }

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

// TestListKeys_ForgedQueryOrganizationIDIgnored pins that the query value only
// requests a tenant; the authenticated scope decides the repository filter.
func TestListKeys_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().Keys().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Key, int64, error) {
			assert.Equal(t, forgedOrgSessionID, q.OrganizationID)
			return nil, 0, nil
		},
	)

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appkeys.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/keys?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestGetKey_ForeignTenantIDIsNotReturned pins that a by-ID read never returns
// a key owned by another tenant. The repository mirror has no RLS, so the
// tenant predicate has to live in the lookup itself.
func TestGetKey_ForeignTenantIDIsNotReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Keys().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "foreign-key", forgedOrgSessionID).
		Return(nil, apperrors.NewNotFound("key not found"))

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appkeys.NewService(forgedOrgClock{}, forgedOrgUUID{}, mockUOW)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/keys/foreign-key?organizationId="+forgedOrgRequestID, nil))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "publicKey")
	assert.NotContains(t, recorder.Body.String(), "privateKey")
}

func TestPatchKey_ForeignTenantIDIsNotReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Keys().Return(repo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "foreign-key", forgedOrgSessionID).
		Return(nil, apperrors.NewNotFound("key not found"))

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appkeys.NewService(forgedOrgClock{}, forgedOrgUUID{}, mockUOW)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPatch,
		"/api/v1/keys/foreign-key?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"displayName":"hijacked"}`)))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "publicKey")
	assert.NotContains(t, recorder.Body.String(), "privateKey")
}

func TestDeleteKey_ForeignTenantIDIsNotReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Keys().Return(repo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "foreign-key", forgedOrgSessionID).
		Return(nil, apperrors.NewNotFound("key not found"))

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appkeys.NewService(forgedOrgClock{}, forgedOrgUUID{}, mockUOW)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodDelete,
		"/api/v1/keys/foreign-key?organizationId="+forgedOrgRequestID, nil))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "publicKey")
	assert.NotContains(t, recorder.Body.String(), "privateKey")
}

func TestKeyByIDRoutes_WithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})

	requests := map[string]*http.Request{
		"get":    httptest.NewRequest(http.MethodGet, "/api/v1/keys/key-id", nil),
		"patch":  httptest.NewRequest(http.MethodPatch, "/api/v1/keys/key-id", strings.NewReader(`{}`)),
		"delete": httptest.NewRequest(http.MethodDelete, "/api/v1/keys/key-id", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "organizationId is required")
		})
	}
}

// TestPatchKey_MissingTenantRejectedBeforeBodyDecode pins the BIND order: the
// tenant decision happens before the JSON body is parsed, so a malformed body
// cannot be used to probe the parser on an unbound request.
func TestPatchKey_MissingTenantRejectedBeforeBodyDecode(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/keys/key-id", strings.NewReader(`{"displayName":`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "organizationId is required")
	assert.NotContains(t, recorder.Body.String(), "invalid JSON")
}

func TestGetKey_OwnTenantReturnsDetail(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)

	own := &domain.Key{
		ID: "own-key", OrganizationID: forgedOrgSessionID, Name: "own",
		PublicKey: "own-public-key", PrivateKey: "own-private-key",
	}
	mockUOW.EXPECT().Keys().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "own-key", forgedOrgSessionID).Return(own, nil)

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appkeys.NewService(forgedOrgClock{}, forgedOrgUUID{}, mockUOW)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/keys/own-key", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "own-public-key")
	assert.NotContains(t, recorder.Body.String(), "own-private-key")
}

func TestPatchKey_OwnTenantUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)

	own := &domain.Key{ID: "own-key", OrganizationID: forgedOrgSessionID, Revision: 1}
	mockUOW.EXPECT().Keys().Return(repo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "own-key", forgedOrgSessionID).Return(own, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, k *domain.Key) error {
			// The existing write semantics still accept private_key updates;
			// encryption happens in the infra repository.
			assert.Equal(t, "PATCH-PRIVATE-MATERIAL", k.PrivateKey)
			return nil
		},
	)

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appkeys.NewService(forgedOrgClock{}, forgedOrgUUID{}, mockUOW)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPatch, "/api/v1/keys/own-key",
		strings.NewReader(`{"displayName":"renamed","revision":1,"privateKey":"PATCH-PRIVATE-MATERIAL"}`)))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "PATCH-PRIVATE-MATERIAL")
	assert.NotContains(t, recorder.Body.String(), "privateKey")
}

func TestDeleteKey_OwnTenantDeletes(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)

	own := &domain.Key{ID: "own-key", OrganizationID: forgedOrgSessionID, Revision: 1}
	mockUOW.EXPECT().Keys().Return(repo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "own-key", forgedOrgSessionID).Return(own, nil)
	repo.EXPECT().Delete(gomock.Any(), "own-key", forgedOrgSessionID).Return(nil)

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appkeys.NewService(forgedOrgClock{}, forgedOrgUUID{}, mockUOW)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodDelete, "/api/v1/keys/own-key", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateKey_WithoutTenantScopeRejected pins the write fail-fast: a forged
// body organizationId must not let the request reach the repository.
func TestCreateKey_WithoutTenantScopeRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"key","keyType":"signing"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
