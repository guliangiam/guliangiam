package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appcerts "github.com/guliangiam/guliangiam/internal/app/certs"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/certs/mock"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// tenantByIDRepo is a tenant-aware in-memory cert repository used by the
// HTTP-level regression tests. The transaction recorder below deliberately
// avoids gomock so audit writes are observable (gomock UoW skips audit).
type tenantByIDRepo struct {
	certs       []*domain.Cert
	getCalls    int
	updateCalls int
	deleteCalls int
}

func (r *tenantByIDRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Cert, error) {
	r.getCalls++
	for _, c := range r.certs {
		if c.ID == id && c.OrganizationID == organizationID {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("cert not found")
}

func (r *tenantByIDRepo) List(context.Context, domain.ListQuery) ([]*domain.Cert, int64, error) {
	return r.certs, int64(len(r.certs)), nil
}

func (r *tenantByIDRepo) Create(_ context.Context, cert *domain.Cert) error {
	r.certs = append(r.certs, cert)
	return nil
}

func (r *tenantByIDRepo) Update(_ context.Context, cert *domain.Cert) error {
	r.updateCalls++
	for i, c := range r.certs {
		if c.ID == cert.ID {
			r.certs[i] = cert
			return nil
		}
	}
	return apperrors.NewNotFound("cert not found")
}

func (r *tenantByIDRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, c := range r.certs {
		if c.ID == id && c.OrganizationID == organizationID {
			r.certs = append(r.certs[:i], r.certs[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("cert not found")
}

type recordingRecords struct {
	created []*recordsdomain.Record
}

func (r *recordingRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *recordingRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *recordingRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *recordingRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *recordingRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *recordingRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *recordingRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

// certScopeUOW exposes only the repos used by the cert service; the embedded
// UnitOfWork carries the remaining interface surface without implementation.
type certScopeUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *recordingRecords
}

func (u *certScopeUOW) Certs() domain.Repository          { return u.repo }
func (u *certScopeUOW) Records() recordsdomain.Repository { return u.records }
func (u *certScopeUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *certScopeUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newCertScopeService(repo domain.Repository) (appcerts.Service, *recordingRecords) {
	records := &recordingRecords{}
	return appcerts.NewService(forgedOrgClock{}, forgedOrgUUID{}, &certScopeUOW{repo: repo, records: records}), records
}

func tenantCert(certID, orgID string, certificate string) *domain.Cert {
	return &domain.Cert{
		ID:             certID,
		OrganizationID: orgID,
		Name:           "tenant-cert",
		DisplayName:    "Tenant Cert",
		Scope:          "SSL",
		Type:           "SSL",
		Certificate:    certificate,
		PrivateKey:     "super-secret-private-key",
		AccessSecret:   "super-secret-access-secret",
		Revision:       1,
	}
}

func selfSignedPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tenant.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func certRouter(service appcerts.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveCertRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestCertByID_RepositoryReceivesTrustedOrganization pins the exact tenant
// argument seen by the repository: the authenticated scope wins over the
// caller-supplied organizationId for reads and deletes.
func TestCertByID_RepositoryReceivesTrustedOrganization(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uow := appuowmock.NewMockUnitOfWork(ctrl)
		repo := domainmock.NewMockRepository(ctrl)
		uow.EXPECT().Certs().Return(repo).AnyTimes()
		repo.EXPECT().GetByID(gomock.Any(), "foreign-cert", forgedOrgSessionID).
			Return(nil, apperrors.NewNotFound("cert not found"))

		router := chi.NewRouter()
		RegisterChiRoutes(router, Handler{Service: appcerts.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
		recorder := serveCertRequest(t, router, forgedOrgRequest(http.MethodGet,
			"/api/v1/certs/foreign-cert?organizationId="+forgedOrgRequestID, nil))
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uow := appuowmock.NewMockUnitOfWork(ctrl)
		repo := domainmock.NewMockRepository(ctrl)
		uow.EXPECT().Certs().Return(repo).AnyTimes()
		uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
				return fn(ctx, uow)
			},
		)
		repo.EXPECT().GetByID(gomock.Any(), "foreign-cert", forgedOrgSessionID).
			Return(nil, apperrors.NewNotFound("cert not found"))
		// No Delete expectation: a foreign id must never reach the delete path.

		router := chi.NewRouter()
		RegisterChiRoutes(router, Handler{Service: appcerts.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
		recorder := serveCertRequest(t, router, forgedOrgRequest(http.MethodDelete,
			"/api/v1/certs/foreign-cert?organizationId="+forgedOrgRequestID, nil))
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("delete own tenant", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uow := appuowmock.NewMockUnitOfWork(ctrl)
		repo := domainmock.NewMockRepository(ctrl)
		own := &domain.Cert{ID: "own-cert", OrganizationID: forgedOrgSessionID, Revision: 1}
		uow.EXPECT().Certs().Return(repo).AnyTimes()
		uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
				return fn(ctx, uow)
			},
		)
		repo.EXPECT().GetByID(gomock.Any(), "own-cert", forgedOrgSessionID).Return(own, nil)
		repo.EXPECT().Delete(gomock.Any(), "own-cert", forgedOrgSessionID).Return(nil)

		router := chi.NewRouter()
		RegisterChiRoutes(router, Handler{Service: appcerts.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
		recorder := serveCertRequest(t, router, forgedOrgRequest(http.MethodDelete, "/api/v1/certs/own-cert", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
	})
}

// TestCertByID_ForeignTenantRejected pins that every by-ID operation resolves
// the authenticated tenant, never the caller-supplied organizationId, and that
// a foreign certificate looks exactly like a missing one.
func TestCertByID_ForeignTenantRejected(t *testing.T) {
	const foreignCertID = "foreign-cert"
	certificate := selfSignedPEM(t)

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/certs/" + foreignCertID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/certs/" + foreignCertID + "?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/certs/" + foreignCertID + "?organizationId=" + forgedOrgRequestID, ""},
		{"update-domain-expire", http.MethodPost, "/api/v1/certs/" + foreignCertID + "/update-domain-expire?organizationId=" + forgedOrgRequestID, `{"domainExpireTime":"2027-01-01T00:00:00Z"}`},
		{"check-domain-expiration", http.MethodPost, "/api/v1/certs/" + foreignCertID + "/check-domain-expiration?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &tenantByIDRepo{certs: []*domain.Cert{tenantCert(foreignCertID, forgedOrgRequestID, certificate)}}
			service, records := newCertScopeService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := forgedOrgRequest(tt.method, tt.target, body)
			recorder := serveCertRequest(t, certRouter(service), req)

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign cert must be indistinguishable from missing")
			assert.Zero(t, repo.updateCalls, "foreign tenant must not update the certificate")
			assert.Zero(t, repo.deleteCalls, "foreign tenant must not delete the certificate")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.NotContains(t, recorder.Body.String(), "super-secret-private-key")
			assert.NotContains(t, recorder.Body.String(), "super-secret-access-secret")
		})
	}
}

// TestCertByID_OwnTenantSucceeds pins that the same five operations still work
// for the authenticated tenant and that deletion audits the real organization.
func TestCertByID_OwnTenantSucceeds(t *testing.T) {
	const ownCertID = "own-cert"
	certificate := selfSignedPEM(t)

	t.Run("get", func(t *testing.T) {
		repo := &tenantByIDRepo{certs: []*domain.Cert{tenantCert(ownCertID, forgedOrgSessionID, certificate)}}
		service, _ := newCertScopeService(repo)
		recorder := serveCertRequest(t, certRouter(service), forgedOrgRequest(http.MethodGet, "/api/v1/certs/"+ownCertID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "privateKeyMasked")
		assert.Contains(t, recorder.Body.String(), "accessSecretMasked")
		assert.NotContains(t, recorder.Body.String(), "super-secret-private-key")
		assert.NotContains(t, recorder.Body.String(), "super-secret-access-secret")
	})

	t.Run("patch", func(t *testing.T) {
		repo := &tenantByIDRepo{certs: []*domain.Cert{tenantCert(ownCertID, forgedOrgSessionID, certificate)}}
		service, records := newCertScopeService(repo)
		recorder := serveCertRequest(t, certRouter(service), forgedOrgRequest(http.MethodPatch,
			"/api/v1/certs/"+ownCertID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &tenantByIDRepo{certs: []*domain.Cert{tenantCert(ownCertID, forgedOrgSessionID, certificate)}}
		service, records := newCertScopeService(repo)
		recorder := serveCertRequest(t, certRouter(service), forgedOrgRequest(http.MethodDelete, "/api/v1/certs/"+ownCertID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID, "delete audit must carry the certificate organization")
	})

	t.Run("update-domain-expire", func(t *testing.T) {
		repo := &tenantByIDRepo{certs: []*domain.Cert{tenantCert(ownCertID, forgedOrgSessionID, certificate)}}
		service, records := newCertScopeService(repo)
		recorder := serveCertRequest(t, certRouter(service), forgedOrgRequest(http.MethodPost,
			"/api/v1/certs/"+ownCertID+"/update-domain-expire",
			strings.NewReader(`{"domainExpireTime":"2027-01-01T00:00:00Z"}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("check-domain-expiration", func(t *testing.T) {
		repo := &tenantByIDRepo{certs: []*domain.Cert{tenantCert(ownCertID, forgedOrgSessionID, certificate)}}
		service, _ := newCertScopeService(repo)
		recorder := serveCertRequest(t, certRouter(service), forgedOrgRequest(http.MethodPost,
			"/api/v1/certs/"+ownCertID+"/check-domain-expiration", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "daysRemaining")
	})
}

// TestCertByID_WithoutTenantScopeRejected pins the fail-fast: when neither the
// session nor a platform-admin target provides a tenant, every by-ID route is
// rejected before touching the repository.
func TestCertByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &tenantByIDRepo{certs: []*domain.Cert{tenantCert("any-cert", forgedOrgSessionID, selfSignedPEM(t))}}
	service, _ := newCertScopeService(repo)
	router := certRouter(service)

	requests := map[string]*http.Request{
		"get":                     httptest.NewRequest(http.MethodGet, "/api/v1/certs/any-cert", nil),
		"patch":                   httptest.NewRequest(http.MethodPatch, "/api/v1/certs/any-cert", strings.NewReader(`{}`)),
		"delete":                  httptest.NewRequest(http.MethodDelete, "/api/v1/certs/any-cert", nil),
		"update-domain-expire":    httptest.NewRequest(http.MethodPost, "/api/v1/certs/any-cert/update-domain-expire", strings.NewReader(`{}`)),
		"check-domain-expiration": httptest.NewRequest(http.MethodPost, "/api/v1/certs/any-cert/check-domain-expiration", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveCertRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
