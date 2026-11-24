package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/idempotency"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// stubIdempotencyRepository drives the middleware through the three denial
// branches without a database. TryAcquire never completes a record here: every
// test asserts a response the middleware writes before delegating downstream.
type stubIdempotencyRepository struct {
	existing     *idempotency.Record
	acquired     bool
	acquireCalls int
	lastRecord   *idempotency.Record
	completed    bool
}

func (r *stubIdempotencyRepository) GetByScopeAndKey(context.Context, string, string) (*idempotency.Record, error) {
	return nil, nil
}

func (r *stubIdempotencyRepository) TryAcquire(_ context.Context, rec *idempotency.Record) (*idempotency.Record, bool, error) {
	r.acquireCalls++
	r.lastRecord = rec
	return r.existing, r.acquired, nil
}

func (r *stubIdempotencyRepository) Complete(context.Context, string, idempotency.Status, int, string, string) error {
	r.completed = true
	return nil
}

// errReader fails on the first Read so io.ReadAll(io.LimitReader(...)) returns
// the same error the middleware turns into a 400.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func serveIdempotency(repo idempotency.Repository, body io.Reader, requestID string) *httptest.ResponseRecorder {
	handler := IdempotencyMiddleware(repo, func(context.Context) (string, string) { return "acme", "user-1" })(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("idempotency denial branches must not call the next handler")
		}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", body)
	request.Header.Set(IdempotencyKeyHeader, "idem-e1b3")
	return serveMiddlewareWithRequestID(handler, request, requestID)
}

func TestIdempotencyMiddleware_UnreadableBodyCanonicalEnvelope(t *testing.T) {
	repo := &stubIdempotencyRepository{}
	recorder := serveIdempotency(repo, errReader{err: errors.New("body read failed")}, "req-e1b3-idem-400")

	assertUnifiedErrorBody(t, recorder, http.StatusBadRequest,
		string(apperrors.CodeInvalidArgument), "idempotency: read request body", "req-e1b3-idem-400")
	if repo.acquireCalls != 0 {
		t.Errorf("TryAcquire calls = %d, want 0 when the body cannot be read", repo.acquireCalls)
	}
}

func TestIdempotencyMiddleware_KeyReuseConflictCanonicalEnvelope(t *testing.T) {
	const requestID = "req-e1b3-idem-409"
	repo := &stubIdempotencyRepository{
		existing: &idempotency.Record{
			Status:        idempotency.StatusSucceeded,
			RequestHash:   "hash-of-the-first-request",
			RequestMethod: http.MethodPost,
			RequestPath:   "/api/v1/users",
		},
		acquired: false,
	}

	recorder := serveIdempotency(repo, strings.NewReader(`{"email":"a@b.c"}`), requestID)

	assertUnifiedErrorBody(t, recorder, http.StatusConflict,
		string(apperrors.CodeConflict), "idempotency: key reused with different request body", requestID)

	// The key/scope derivation is unchanged: same tenant slug + path + header.
	if repo.lastRecord == nil {
		t.Fatal("TryAcquire was not called")
	}
	if repo.lastRecord.Scope != "acme:user-1:/api/v1/users" {
		t.Errorf("scope = %q, want acme:user-1:/api/v1/users", repo.lastRecord.Scope)
	}
	if repo.lastRecord.Key != "idem-e1b3" {
		t.Errorf("key = %q, want idem-e1b3", repo.lastRecord.Key)
	}
	if repo.lastRecord.RequestHash != hashBody([]byte(`{"email":"a@b.c"}`)) {
		t.Errorf("request hash = %q, want sha256 of the request body", repo.lastRecord.RequestHash)
	}
}

func TestIdempotencyMiddleware_InProgressUnavailableCanonicalEnvelope(t *testing.T) {
	const requestID = "req-e1b3-idem-503"
	body := []byte(`{"email":"a@b.c"}`)
	repo := &stubIdempotencyRepository{
		existing: &idempotency.Record{
			Status:        idempotency.StatusInProgress,
			RequestHash:   hashBody(body),
			RequestMethod: http.MethodPost,
			RequestPath:   "/api/v1/users",
		},
		acquired: false,
	}

	recorder := serveIdempotency(repo, strings.NewReader(string(body)), requestID)

	assertUnifiedErrorBody(t, recorder, http.StatusServiceUnavailable,
		string(apperrors.CodeUnavailable), "idempotency: request in progress, retry later", requestID)
	if retryAfter := recorder.Header().Get("Retry-After"); retryAfter != "2" {
		t.Errorf("Retry-After = %q, want 2 (unchanged retry semantics)", retryAfter)
	}
}
