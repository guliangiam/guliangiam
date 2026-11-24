package idempotency

import (
	"context"
	stderrors "errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domainidem "github.com/guliangiam/guliangiam/internal/domain/idempotency"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// scriptedDBTX drives tryAcquireDB through its conflict/reclaim/reacquire
// branches without PostgreSQL: each SQL statement is answered by a scripted
// handler keyed on a distinctive fragment of the query text.
type scriptedDBTX struct {
	t            *testing.T
	insertCalls  int
	insertRows   []pgx.Row // successive answers for QueryTryAcquire
	deleteTag    pgconn.CommandTag
	getRow       pgx.Row
	reacquireRow pgx.Row
}

func (s *scriptedDBTX) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "DELETE FROM") && strings.Contains(sql, "expires_at <= NOW()") {
		return s.deleteTag, nil
	}
	s.t.Fatalf("unexpected Exec: %s", sql)
	return pgconn.CommandTag{}, nil
}

func (s *scriptedDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) {
	s.t.Fatal("unexpected Query")
	return nil, nil
}

func (s *scriptedDBTX) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "INSERT INTO"):
		idx := s.insertCalls
		s.insertCalls++
		if idx < len(s.insertRows) {
			return s.insertRows[idx]
		}
		return errRow{pgx.ErrNoRows}
	case strings.Contains(sql, "SET status = 'in_progress'"):
		if s.reacquireRow == nil {
			return errRow{pgx.ErrNoRows}
		}
		return s.reacquireRow
	case strings.Contains(sql, "SELECT id"):
		if s.getRow == nil {
			return errRow{pgx.ErrNoRows}
		}
		return s.getRow
	}
	s.t.Fatalf("unexpected QueryRow: %s", sql)
	return errRow{pgx.ErrNoRows}
}

type errRow struct{ err error }

func (e errRow) Scan(...any) error { return e.err }

// recordRow answers scanRecord's 14-column scan from a domain Record.
type recordRow struct{ rec domainidem.Record }

func (r recordRow) Scan(dest ...any) error {
	rec := r.rec
	vals := []any{
		rec.ID, rec.OrganizationID, rec.Scope, rec.Key,
		rec.RequestMethod, rec.RequestPath, rec.RequestHash,
		rec.Status, rec.ResponseStatus, rec.ResponseBody, rec.ContentType,
		rec.CompletedAt, &rec.ExpiresAt, &rec.CreatedAt,
	}
	for i, d := range dest {
		dv := reflect.ValueOf(d).Elem()
		v := reflect.ValueOf(vals[i])
		if v.Type() != dv.Type() {
			v = v.Convert(dv.Type())
		}
		dv.Set(v)
	}
	return nil
}

func newTestRecord(hash string) *domainidem.Record {
	return &domainidem.Record{
		ID:            "01900000-0000-7000-8000-000000000001",
		Scope:         "acme:u-1:/api/v1/orders",
		Key:           "k-1",
		RequestMethod: "POST",
		RequestPath:   "/api/v1/orders",
		RequestHash:   hash,
	}
}

// TestTryAcquire_ExpiredRowPreempted pins BE-E-12①: an expired-but-uncleaned
// row must be reclaimed by key and the INSERT retried, not 409 until the TTL
// cleaner runs.
func TestTryAcquire_ExpiredRowPreempted(t *testing.T) {
	rec := newTestRecord("h1")
	db := &scriptedDBTX{
		t:          t,
		insertRows: []pgx.Row{errRow{pgx.ErrNoRows}, recordRow{domainidem.Record{ID: rec.ID, Status: domainidem.StatusInProgress}}},
		deleteTag:  pgconn.NewCommandTag("DELETE 1"),
	}
	repo := NewRepository(db, nil)

	out, acquired, err := repo.TryAcquire(context.Background(), rec)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if !acquired || out == nil {
		t.Fatalf("acquired=%v out=%v, want preempted acquisition", acquired, out)
	}
	if db.insertCalls != 2 {
		t.Fatalf("insert attempts = %d, want 2 (initial + post-reclaim)", db.insertCalls)
	}
}

// TestTryAcquire_ExpiredReclaimLostRaceFallsBackToRead: when a concurrent
// request already reclaimed the row (DELETE 0), the existing live row is
// returned for the middleware to adjudicate.
func TestTryAcquire_ExpiredReclaimLostRaceFallsBackToRead(t *testing.T) {
	db := &scriptedDBTX{
		t:         t,
		deleteTag: pgconn.NewCommandTag("DELETE 0"),
		getRow: recordRow{domainidem.Record{
			ID: "existing", Status: domainidem.StatusInProgress, RequestHash: "h1",
		}},
	}
	repo := NewRepository(db, nil)

	out, acquired, err := repo.TryAcquire(context.Background(), newTestRecord("h1"))
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if acquired || out == nil || out.ID != "existing" {
		t.Fatalf("acquired=%v out=%+v, want existing in_progress row", acquired, out)
	}
}

// TestTryAcquire_FailedSameHashReacquired pins BE-E-12②: a failed terminal row
// retried with the same body hash flips back to in_progress so the retry's
// outcome is captured.
func TestTryAcquire_FailedSameHashReacquired(t *testing.T) {
	db := &scriptedDBTX{
		t:         t,
		deleteTag: pgconn.NewCommandTag("DELETE 0"),
		getRow: recordRow{domainidem.Record{
			ID: "failed-row", Status: domainidem.StatusFailed, RequestHash: "h1",
		}},
		reacquireRow: recordRow{domainidem.Record{
			ID: "failed-row", Status: domainidem.StatusInProgress, RequestHash: "h1",
		}},
	}
	repo := NewRepository(db, nil)

	out, acquired, err := repo.TryAcquire(context.Background(), newTestRecord("h1"))
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if !acquired || out == nil || out.ID != "failed-row" || out.Status != domainidem.StatusInProgress {
		t.Fatalf("acquired=%v out=%+v, want failed row reacquired as in_progress", acquired, out)
	}
}

// TestTryAcquire_FailedDifferentHashStaysConflictInput: a different body hash
// must NOT reacquire; the middleware turns the returned record into 409.
func TestTryAcquire_FailedDifferentHashStaysConflictInput(t *testing.T) {
	db := &scriptedDBTX{
		t:         t,
		deleteTag: pgconn.NewCommandTag("DELETE 0"),
		getRow: recordRow{domainidem.Record{
			ID: "failed-row", Status: domainidem.StatusFailed, RequestHash: "other-hash",
		}},
	}
	repo := NewRepository(db, nil)

	out, acquired, err := repo.TryAcquire(context.Background(), newTestRecord("h1"))
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if acquired || out == nil || out.RequestHash != "other-hash" {
		t.Fatalf("acquired=%v out=%+v, want non-acquired existing record for 409", acquired, out)
	}
}

// TestTryAcquire_ConflictUnreadableStillConflict: insert conflicts, nothing to
// reclaim and nothing readable (row vanished) keeps the explicit conflict.
func TestTryAcquire_ConflictUnreadableStillConflict(t *testing.T) {
	db := &scriptedDBTX{t: t, deleteTag: pgconn.NewCommandTag("DELETE 0")}
	repo := NewRepository(db, nil)

	_, _, err := repo.TryAcquire(context.Background(), newTestRecord("h1"))
	if err == nil {
		t.Fatal("want conflict error")
	}
	var appErr *apperrors.Error
	if !stderrors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("err = %v, want conflict code", err)
	}
}

// completeCaptureDBTX records the single UPDATE issued by Complete so tests
// can pin the content_type argument shape without PostgreSQL.
type completeCaptureDBTX struct {
	t    *testing.T
	sql  string
	args []any
}

func (c *completeCaptureDBTX) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.sql = sql
	c.args = args
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (c *completeCaptureDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) {
	c.t.Fatal("unexpected Query")
	return nil, nil
}

func (c *completeCaptureDBTX) QueryRow(context.Context, string, ...any) pgx.Row {
	c.t.Fatal("unexpected QueryRow")
	return errRow{pgx.ErrNoRows}
}

// TestComplete_StoresContentType pins BE-E-12③ (infra side): Complete must
// write the captured Content-Type into the content_type column.
func TestComplete_StoresContentType(t *testing.T) {
	db := &completeCaptureDBTX{t: t}
	repo := NewRepository(db, nil)

	err := repo.Complete(context.Background(), "rec-1", domainidem.StatusSucceeded, 201, `{"ok":true}`, "application/json")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(db.sql, "content_type = $4") {
		t.Fatalf("Complete SQL missing content_type assignment: %s", db.sql)
	}
	if len(db.args) != 5 || db.args[3] != "application/json" || db.args[4] != "rec-1" {
		t.Fatalf("Complete args = %v, want content_type at $4 and id at $5", db.args)
	}
}

// TestComplete_EmptyContentTypeStoresNull: an unset Content-Type must be
// stored as NULL so legacy replay behavior is preserved.
func TestComplete_EmptyContentTypeStoresNull(t *testing.T) {
	db := &completeCaptureDBTX{t: t}
	repo := NewRepository(db, nil)

	if err := repo.Complete(context.Background(), "rec-1", domainidem.StatusSucceeded, 200, "body", ""); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(db.args) != 5 || db.args[3] != nil {
		t.Fatalf("Complete args = %v, want nil content_type for empty header", db.args)
	}
}

// TestScanRecord_ContentTypeRoundTripAndNullFallback: scanRecord must surface
// a stored content_type and keep nil for legacy NULL rows.
func TestScanRecord_ContentTypeRoundTripAndNullFallback(t *testing.T) {
	ct := "application/json"
	rec, err := scanRecord(recordRow{domainidem.Record{ID: "r1", ContentType: &ct}})
	if err != nil {
		t.Fatalf("scanRecord: %v", err)
	}
	if rec.ContentType == nil || *rec.ContentType != "application/json" {
		t.Fatalf("ContentType = %v, want application/json", rec.ContentType)
	}

	legacy, err := scanRecord(recordRow{domainidem.Record{ID: "r2", ContentType: nil}})
	if err != nil {
		t.Fatalf("scanRecord legacy: %v", err)
	}
	if legacy.ContentType != nil {
		t.Fatalf("legacy ContentType = %q, want nil", *legacy.ContentType)
	}
}
