package idv

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	appidv "github.com/guliangiam/guliangiam/internal/app/idv"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainidv "github.com/guliangiam/guliangiam/internal/domain/idv"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	infraidv "github.com/guliangiam/guliangiam/internal/infra/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// callbackRepo is a minimal in-memory IDV repository for the callback tests.
type callbackRepo struct {
	records map[string]*domainidv.Verification
}

func newCallbackRepo() *callbackRepo {
	return &callbackRepo{records: map[string]*domainidv.Verification{}}
}

func (r *callbackRepo) Create(_ context.Context, v *domainidv.Verification) error {
	r.records[v.ID] = v
	return nil
}

func (r *callbackRepo) Update(_ context.Context, v *domainidv.Verification) error {
	r.records[v.ID] = v
	return nil
}

func (r *callbackRepo) GetByID(_ context.Context, id, organizationID string) (*domainidv.Verification, error) {
	record, ok := r.records[id]
	if !ok || record.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("verification not found")
	}
	return record, nil
}

func (r *callbackRepo) GetByProviderRef(_ context.Context, provider, providerRef string) (*domainidv.Verification, error) {
	for _, record := range r.records {
		if record.Provider == provider && record.ProviderRef != nil && *record.ProviderRef == providerRef {
			return record, nil
		}
	}
	return nil, apperrors.NewNotFound("verification not found")
}

func (r *callbackRepo) List(context.Context, domainidv.ListQuery) ([]*domainidv.Verification, int64, error) {
	return nil, 0, nil
}

func (r *callbackRepo) Delete(context.Context, string, string) error { return nil }

// callbackUOW satisfies appuow.UnitOfWork for the callback path only. The
// embedded nil interface covers the unused repositories; Records/Outbox are
// explicitly nil so the audit embedder skips persistence in this test.
type callbackUOW struct {
	appuow.UnitOfWork
	repo *callbackRepo
}

func (u callbackUOW) IDV() domainidv.Repository         { return u.repo }
func (u callbackUOW) Records() domainrecords.Repository { return nil }
func (u callbackUOW) Outbox() domainoutbox.Repository   { return nil }
func (u callbackUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type callbackResolver struct {
	credentials domainidv.ProviderCredentials
}

func (r callbackResolver) ResolveIDVProvider(context.Context, string, string) (*domainidv.ProviderCredentials, error) {
	credentials := r.credentials
	return &credentials, nil
}

// newCallbackService wires the callback path to the real verifier factory with
// in-memory persistence, so a signed callback reaches 200 instead of the
// zero-value Service's 503 "provider is not configured".
func newCallbackService(repo *callbackRepo, credentials domainidv.ProviderCredentials) appidv.Service {
	return appidv.Service{
		UOW:       callbackUOW{repo: repo},
		Clock:     func() time.Time { return time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC) },
		Providers: callbackResolver{credentials: credentials},
		Factory:   infraidv.NewFactory(),
	}
}

func newCallbackServer(t *testing.T, repo *callbackRepo, credentials domainidv.ProviderCredentials) *httptest.Server {
	t.Helper()
	router := chi.NewRouter()
	RegisterCallbackChiRoutes(router, Handler{Service: newCallbackService(repo, credentials)})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func signed(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func pendingRecord(id, provider, reference string) *domainidv.Verification {
	ref := reference
	now := time.Now()
	return &domainidv.Verification{
		ID: id, OrganizationID: "org-1", Provider: provider,
		Status: domainidv.StatusPending, ProviderRef: &ref,
		InitiatedAt: now, CreatedAt: now, UpdatedAt: now,
	}
}

func postCallback(t *testing.T, server *httptest.Server, path string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("POST %s -> %d", path, response.StatusCode)
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestCallbackE2E_JumioAcceptsForgedSignature(t *testing.T) {
	repo := newCallbackRepo()
	record := pendingRecord("ver-1", "jumio", "scan-1")
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	server := newCallbackServer(t, repo, domainidv.ProviderCredentials{
		Provider: "jumio", APIKey: "token", APISecret: "secret", WebhookSecret: "whsec",
	})
	payload := []byte(`{"jumioIdScanReference":"scan-1","status":"APPROVED_VERIFIED"}`)

	response := postCallback(t, server, "/api/v1/idv/callback/jumio", payload, map[string]string{
		"X-Jumio-Signature": signed("whsec", payload),
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got := repo.records["ver-1"].Status; got != domainidv.StatusApproved {
		t.Fatalf("status = %s, want approved", got)
	}
}

func TestCallbackE2E_JumioRejectsBadSignature(t *testing.T) {
	repo := newCallbackRepo()
	if err := repo.Create(context.Background(), pendingRecord("ver-2", "jumio", "scan-2")); err != nil {
		t.Fatal(err)
	}
	server := newCallbackServer(t, repo, domainidv.ProviderCredentials{
		Provider: "jumio", APIKey: "token", APISecret: "secret", WebhookSecret: "whsec",
	})
	payload := []byte(`{"jumioIdScanReference":"scan-2","status":"APPROVED_VERIFIED"}`)

	response := postCallback(t, server, "/api/v1/idv/callback/jumio", payload, map[string]string{
		"X-Jumio-Signature": "deadbeef",
	})
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
	if got := repo.records["ver-2"].Status; got != domainidv.StatusPending {
		t.Fatalf("status must stay pending, got %s", got)
	}
}

func TestCallbackE2E_JumioRejectsMissingHeaderAndSecret(t *testing.T) {
	repo := newCallbackRepo()
	if err := repo.Create(context.Background(), pendingRecord("ver-3", "jumio", "scan-3")); err != nil {
		t.Fatal(err)
	}
	server := newCallbackServer(t, repo, domainidv.ProviderCredentials{
		Provider: "jumio", APIKey: "token", APISecret: "secret", WebhookSecret: "whsec",
	})
	payload := []byte(`{"jumioIdScanReference":"scan-3","status":"DENIED"}`)

	if response := postCallback(t, server, "/api/v1/idv/callback/jumio", payload, nil); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing header status = %d, want 401", response.StatusCode)
	}

	unconfigured := newCallbackServer(t, repo, domainidv.ProviderCredentials{
		Provider: "jumio", APIKey: "token", APISecret: "secret",
	})
	if response := postCallback(t, unconfigured, "/api/v1/idv/callback/jumio", payload, map[string]string{
		"X-Jumio-Signature": "deadbeef",
	}); response.StatusCode != http.StatusForbidden {
		t.Fatalf("unconfigured secret status = %d, want 403", response.StatusCode)
	}
}

func TestCallbackE2E_AliyunAcceptsForgedSignature(t *testing.T) {
	repo := newCallbackRepo()
	if err := repo.Create(context.Background(), pendingRecord("ver-4", "aliyun_idv", "cert-1")); err != nil {
		t.Fatal(err)
	}
	server := newCallbackServer(t, repo, domainidv.ProviderCredentials{
		Provider: "aliyun_idv", APIKey: "ak", APISecret: "sk", WebhookSecret: "whsec",
	})
	payload := []byte(`{"CertifyId":"cert-1","Passed":"T"}`)

	response := postCallback(t, server, "/api/v1/idv/callback/aliyun_idv", payload, map[string]string{
		"X-Acs-Signature": signed("whsec", payload),
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got := repo.records["ver-4"].Status; got != domainidv.StatusApproved {
		t.Fatalf("status = %s, want approved", got)
	}
}

func TestCallbackE2E_UnknownProviderIsNotFound(t *testing.T) {
	repo := newCallbackRepo()
	server := newCallbackServer(t, repo, domainidv.ProviderCredentials{Provider: "jumio"})

	response := postCallback(t, server, "/api/v1/idv/callback/faceid", []byte(`{}`), nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.StatusCode)
	}
}
