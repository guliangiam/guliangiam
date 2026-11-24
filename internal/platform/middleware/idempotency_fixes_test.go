package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/idempotency"
)

// memoryIdempotencyRepo is a scope-keyed in-memory Repository used to prove
// cross-user isolation and terminal-state capture without a database.
type memoryIdempotencyRepo struct {
	mu           sync.Mutex
	rows         map[string]*idempotency.Record // scope|key → record
	completeErr  error                          // ctx.Err() observed at Complete call time
	completeSeen bool
	completeID   string
	completeSt   idempotency.Status
	completes    int
}

func newMemoryIdempotencyRepo() *memoryIdempotencyRepo {
	return &memoryIdempotencyRepo{rows: map[string]*idempotency.Record{}}
}

func (m *memoryIdempotencyRepo) GetByScopeAndKey(_ context.Context, scope, key string) (*idempotency.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rows[scope+"|"+key], nil
}

func (m *memoryIdempotencyRepo) TryAcquire(_ context.Context, rec *idempotency.Record) (*idempotency.Record, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := rec.Scope + "|" + rec.Key
	if existing, ok := m.rows[k]; ok {
		return existing, false, nil
	}
	stored := *rec
	stored.ID = fmt.Sprintf("rec-%d", len(m.rows)+1)
	stored.Status = idempotency.StatusInProgress
	rec.ID = stored.ID // mirror the real repo: TryAcquire stamps the caller's record
	m.rows[k] = &stored
	return &stored, true, nil
}

func (m *memoryIdempotencyRepo) Complete(ctx context.Context, id string, status idempotency.Status, respStatus int, respBody, respContentType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completes++
	m.completeSeen = true
	m.completeErr = ctx.Err()
	m.completeID = id
	m.completeSt = status
	for _, row := range m.rows {
		if row.ID == id {
			row.Status = status
			row.ResponseStatus = &respStatus
			row.ResponseBody = &respBody
			if respContentType != "" {
				ct := respContentType
				row.ContentType = &ct
			} else {
				row.ContentType = nil
			}
		}
	}
	return ctx.Err()
}

// TestIdempotencyMiddleware_CrossUserSameKeyExecutesIndependently pins
// BE-E-01: user B reusing user A's Idempotency-Key on the same path must
// execute fresh, never replay A's stored response body.
func TestIdempotencyMiddleware_CrossUserSameKeyExecutesIndependently(t *testing.T) {
	repo := newMemoryIdempotencyRepo()
	var executions []string
	handler := IdempotencyMiddleware(repo, func(ctx context.Context) (string, string) {
		return "acme", ctx.Value(userKey{}).(string)
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(userKey{}).(string)
		executions = append(executions, user)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created-for-" + user))
	}))

	serve := func(user string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{"sku":"x"}`))
		req = req.WithContext(context.WithValue(req.Context(), userKey{}, user))
		req.Header.Set(IdempotencyKeyHeader, "shared-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	recA := serve("user-a")
	recB := serve("user-b")

	if len(executions) != 2 || executions[0] != "user-a" || executions[1] != "user-b" {
		t.Fatalf("executions = %v, want both users to execute independently", executions)
	}
	if recB.Header().Get(IdempotencyReplayedHeader) != "" {
		t.Fatal("user B was served a replayed response")
	}
	if recB.Body.String() != "created-for-user-b" {
		t.Fatalf("user B body = %q, want its own response", recB.Body.String())
	}
	// Same user repeating the key still replays (idempotency preserved).
	recA2 := serve("user-a")
	if recA2.Header().Get(IdempotencyReplayedHeader) != "true" || recA2.Body.String() != "created-for-user-a" {
		t.Fatalf("same-user replay broken: replayed=%q body=%q",
			recA2.Header().Get(IdempotencyReplayedHeader), recA2.Body.String())
	}
	_ = recA
}

type userKey struct{}

// TestIdempotencyMiddleware_OversizedBodyPassesThroughIntact pins BE-E-05:
// a keyed request larger than the 1 MiB hashing limit must reach the handler
// with its full body (no truncation, no replacement) and skip idempotency.
func TestIdempotencyMiddleware_OversizedBodyPassesThroughIntact(t *testing.T) {
	repo := newMemoryIdempotencyRepo()
	payload := bytes.Repeat([]byte("a"), (1<<20)+4096)
	var got []byte
	handler := IdempotencyMiddleware(repo, func(context.Context) (string, string) { return "acme", "u" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			got, err = io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("downstream body read: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/upload-csv", bytes.NewReader(payload))
	req.Header.Set(IdempotencyKeyHeader, "big-upload")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !bytes.Equal(got, payload) {
		t.Fatalf("downstream body length = %d, want intact %d bytes", len(got), len(payload))
	}
	if len(repo.rows) != 0 || repo.completes != 0 {
		t.Fatalf("oversized request must skip idempotency recording, got %d rows / %d completes", len(repo.rows), repo.completes)
	}
}

// TestIdempotencyMiddleware_CompleteSurvivesClientDisconnect pins BE-E-06:
// Complete must run on a context detached from the cancelled request context
// so the record does not strand in in_progress after a client disconnect.
func TestIdempotencyMiddleware_CompleteSurvivesClientDisconnect(t *testing.T) {
	repo := newMemoryIdempotencyRepo()
	ctx, cancel := context.WithCancel(context.Background())
	handler := IdempotencyMiddleware(repo, func(context.Context) (string, string) { return "acme", "u" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true}`))
			cancel() // client disconnects mid-request
		}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{"sku":"x"}`)).WithContext(ctx)
	req.Header.Set(IdempotencyKeyHeader, "disconnect-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if repo.completes != 1 {
		t.Fatalf("Complete calls = %d, want 1 despite request cancellation", repo.completes)
	}
	if !repo.completeSeen || repo.completeErr != nil {
		t.Fatalf("Complete context cancelled at call time: %v", repo.completeErr)
	}
	if repo.completeSt != idempotency.StatusSucceeded {
		t.Fatalf("Complete status = %s, want succeeded", repo.completeSt)
	}
}

// TestIdempotencyMiddleware_FailedRecordRetryIsCaptured pins BE-E-12 (middleware
// side): a same-hash retry of a failed record re-executes AND records the new
// terminal state instead of passing through untracked.
func TestIdempotencyMiddleware_FailedRecordRetryIsCaptured(t *testing.T) {
	body := []byte(`{"sku":"x"}`)
	repo := &stubIdempotencyRepository{
		existing: &idempotency.Record{
			ID:          "rec-failed-1",
			Status:      idempotency.StatusFailed,
			RequestHash: hashBody(body),
		},
		acquired: false,
	}
	var executed bool
	handler := IdempotencyMiddleware(repo, func(context.Context) (string, string) { return "acme", "u" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executed = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set(IdempotencyKeyHeader, "retry-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !executed {
		t.Fatal("failed-record retry did not execute")
	}
	if !repo.completed {
		t.Fatal("failed-record retry outcome was not captured via Complete")
	}
	if repo.lastRecord == nil || repo.lastRecord.ID != "rec-failed-1" {
		t.Fatalf("Complete must target the existing failed row, got %+v", repo.lastRecord)
	}
}

// TestIdempotencyMiddleware_ContentTypeCapturedAndReplayed pins BE-E-12③:
// the response Content-Type set by the handler must be stored at Complete
// time and restored on replay, so JSON replays keep their Content-Type header.
func TestIdempotencyMiddleware_ContentTypeCapturedAndReplayed(t *testing.T) {
	repo := newMemoryIdempotencyRepo()
	var executions int
	handler := IdempotencyMiddleware(repo, func(context.Context) (string, string) { return "acme", "u" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executions++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))

	serve := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{"sku":"x"}`))
		req.Header.Set(IdempotencyKeyHeader, "ct-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	first := serve()
	if got := first.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("first response Content-Type = %q, want application/json", got)
	}
	replay := serve()
	if executions != 1 {
		t.Fatalf("executions = %d, want 1 (second request must replay)", executions)
	}
	if replay.Header().Get(IdempotencyReplayedHeader) != "true" {
		t.Fatal("second request was not replayed")
	}
	if got := replay.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("replayed Content-Type = %q, want stored application/json", got)
	}
	if replay.Code != http.StatusCreated || replay.Body.String() != `{"ok":true}` {
		t.Fatalf("replay status/body = %d/%q, want 201/{\"ok\":true}", replay.Code, replay.Body.String())
	}
}

// TestIdempotencyMiddleware_ContentTypeNullFallsBack pins BE-E-12③ compatibility:
// rows without a stored Content-Type (legacy NULL rows, or handlers that never
// set the header) replay without an explicit Content-Type — the pre-fix behavior.
func TestIdempotencyMiddleware_ContentTypeNullFallsBack(t *testing.T) {
	body := []byte(`{"sku":"x"}`)
	repo := newMemoryIdempotencyRepo()
	// Legacy succeeded row: response captured before content_type existed.
	repo.rows["acme:u:/api/v1/orders|legacy-key"] = &idempotency.Record{
		ID:             "rec-legacy",
		Scope:          "acme:u:/api/v1/orders",
		Key:            "legacy-key",
		RequestHash:    hashBody(body),
		Status:         idempotency.StatusSucceeded,
		ResponseStatus: func() *int { s := http.StatusOK; return &s }(),
		ResponseBody:   func() *string { s := `{"ok":true}`; return &s }(),
		ContentType:    nil,
	}
	handler := IdempotencyMiddleware(repo, func(context.Context) (string, string) { return "acme", "u" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("legacy succeeded row must replay, not execute")
		}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set(IdempotencyKeyHeader, "legacy-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get(IdempotencyReplayedHeader) != "true" {
		t.Fatal("legacy row was not replayed")
	}
	if got := rec.Header().Get("Content-Type"); got != "" {
		t.Fatalf("NULL content_type replay set explicit Content-Type %q, want none", got)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("legacy replay body = %q, want stored body", rec.Body.String())
	}
}

// TestIdempotencyMiddleware_UnsetContentTypeStoredNull: when the handler never
// sets Content-Type, Complete must store an empty value (→ NULL), not a sniffed
// guess, so replay keeps the default behavior.
func TestIdempotencyMiddleware_UnsetContentTypeStoredNull(t *testing.T) {
	repo := newMemoryIdempotencyRepo()
	handler := IdempotencyMiddleware(repo, func(context.Context) (string, string) { return "acme", "u" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("plain-body"))
		}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{"sku":"x"}`))
	req.Header.Set(IdempotencyKeyHeader, "no-ct-key")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if repo.completes != 1 {
		t.Fatalf("Complete calls = %d, want 1", repo.completes)
	}
	for _, row := range repo.rows {
		if row.ContentType != nil {
			t.Fatalf("stored ContentType = %q, want nil when handler set none", *row.ContentType)
		}
	}
}
