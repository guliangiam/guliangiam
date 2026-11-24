package idempotency

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	domainidem "github.com/guliangiam/guliangiam/internal/domain/idempotency"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

func TestRepository_DeleteExpired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	cutoff := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM idempotency_keys")).
		WithArgs(cutoff, 1000).
		WillReturnResult(sqlmock.NewResult(0, 3))

	repo := NewRepository(db)
	n, err := repo.DeleteExpired(context.Background(), cutoff, 1000)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_Complete_ContentType pins BE-E-12③ (MySQL mirror): Complete
// writes the captured Content-Type into the content_type column.
func TestRepository_Complete_ContentType(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(QueryComplete)).
		WithArgs("succeeded", 201, `{"ok":true}`, "application/json", "rec-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewRepository(db)
	err := repo.Complete(context.Background(), "rec-1", domainidem.StatusSucceeded, 201, `{"ok":true}`, "application/json")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_Complete_EmptyContentTypeStoresNull: an unset Content-Type
// must be stored as NULL so legacy replay behavior is preserved.
func TestRepository_Complete_EmptyContentTypeStoresNull(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(QueryComplete)).
		WithArgs("succeeded", 200, "body", nil, "rec-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewRepository(db)
	err := repo.Complete(context.Background(), "rec-1", domainidem.StatusSucceeded, 200, "body", "")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_GetByScopeAndKey_ContentTypeShape pins the 14-column scan
// (content_type after response_body) and the NULL fallback for legacy rows.
func TestRepository_GetByScopeAndKey_ContentTypeShape(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	cols := []string{
		"id", "organization_id", "scope", "idempotency_key",
		"request_method", "request_path", "request_hash",
		"status", "response_status", "response_body", "content_type",
		"completed_at", "expires_at", "created_at",
	}
	now := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(QueryGetByScopeKey)).
		WithArgs("acme:u:/api/v1/orders", "k-1").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"rec-1", nil, "acme:u:/api/v1/orders", "k-1",
			"POST", "/api/v1/orders", "h1",
			"succeeded", 201, `{"ok":true}`, "application/json",
			now, now.Add(24*time.Hour), now,
		))

	repo := NewRepository(db)
	rec, err := repo.GetByScopeAndKey(context.Background(), "acme:u:/api/v1/orders", "k-1")
	assert.NoError(t, err)
	assert.NotNil(t, rec)
	assert.NotNil(t, rec.ContentType)
	assert.Equal(t, "application/json", *rec.ContentType)

	// Second scan of the same query: legacy NULL row keeps ContentType nil.
	mock.ExpectQuery(regexp.QuoteMeta(QueryGetByScopeKey)).
		WithArgs("acme:u:/api/v1/orders", "k-legacy").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"rec-legacy", nil, "acme:u:/api/v1/orders", "k-legacy",
			"POST", "/api/v1/orders", "h1",
			"succeeded", 200, `{"ok":true}`, nil,
			now, now.Add(24*time.Hour), now,
		))
	legacy, err := repo.GetByScopeAndKey(context.Background(), "acme:u:/api/v1/orders", "k-legacy")
	assert.NoError(t, err)
	assert.NotNil(t, legacy)
	assert.Nil(t, legacy.ContentType)
	assert.NoError(t, mock.ExpectationsWereMet())
}
