package castickets

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/castickets"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow       = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedExpiry    = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	fixedPGTExpiry = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	testTicketID   = "ST-abc123"
	testPGTIOU     = "PGTIOU-xyz789"
	testUserID     = "00000000-0000-0000-0000-000000000001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ticketCols: 11 列
func ticketCols() []string {
	return []string{
		"id", "ticket_id", "ticket_type", "user_id", "username", "service", "expires_at", "created_at",
		"pgt_iou", "pgt_iou_expires_at", "pgt_iou_used",
	}
}

func newTicketRow() []driver.Value {
	return []driver.Value{
		"00000000-0000-0000-0000-000000000010", testTicketID, "service", testUserID, "alice",
		"https://app.example.com/callback", fixedExpiry, fixedNow,
		nil, nil, false,
	}
}

func newPGTTicketRow() []driver.Value {
	return []driver.Value{
		"00000000-0000-0000-0000-000000000010", "PGT-abc", "proxy_granting", testUserID, "alice",
		"https://app.example.com/callback", fixedExpiry, fixedNow,
		testPGTIOU, fixedPGTExpiry, false,
	}
}

// ── GetByTicketID ────────────────────────────────────────────────

func TestRepository_GetByTicketID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByTicketID)).
		WithArgs(testTicketID).
		WillReturnRows(sqlmock.NewRows(ticketCols()).AddRow(newTicketRow()...))

	r := NewRepository(db)
	tk, err := r.GetByTicketID(context.Background(), testTicketID)
	assert.NoError(t, err)
	assert.NotNil(t, tk)
	assert.Equal(t, testTicketID, tk.TicketID)
	assert.Equal(t, false, tk.PGTIOUUsed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByTicketID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByTicketID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(ticketCols()))

	r := NewRepository(db)
	tk, err := r.GetByTicketID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, tk)
}

// ── GetByPGTIOU ──────────────────────────────────────────────────

func TestRepository_GetByPGTIOU_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByPGTIOU)).
		WithArgs(testPGTIOU).
		WillReturnRows(sqlmock.NewRows(ticketCols()).AddRow(newPGTTicketRow()...))

	r := NewRepository(db)
	tk, err := r.GetByPGTIOU(context.Background(), testPGTIOU)
	assert.NoError(t, err)
	assert.NotNil(t, tk)
	assert.Equal(t, "PGT-abc", tk.TicketID)
	assert.Equal(t, testPGTIOU, tk.PGTIOU)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByPGTIOU_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByPGTIOU)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(ticketCols()))

	r := NewRepository(db)
	tk, err := r.GetByPGTIOU(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, tk)
}

// ── MarkPGTIOUUsed ───────────────────────────────────────────────

func TestRepository_MarkPGTIOUUsed_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryMarkPGTIOUUsed)).
		WithArgs(testPGTIOU).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectPGTTicketID)).
		WithArgs(testPGTIOU).
		WillReturnRows(sqlmock.NewRows([]string{"ticket_id"}).AddRow("PGT-abc"))

	r := NewRepository(db)
	ticketID, err := r.MarkPGTIOUUsed(context.Background(), testPGTIOU)
	assert.NoError(t, err)
	assert.Equal(t, "PGT-abc", ticketID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_MarkPGTIOUUsed_Invalid(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryMarkPGTIOUUsed)).
		WithArgs("bad").
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → not found

	r := NewRepository(db)
	_, err := r.MarkPGTIOUUsed(context.Background(), "bad")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryCreate)).
		WithArgs(
			"00000000-0000-0000-0000-000000000010", testTicketID, "service", testUserID, "alice",
			"https://app.example.com/callback", fixedExpiry, fixedNow,
			nil, nil, false,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Ticket{
		ID:         "00000000-0000-0000-0000-000000000010",
		TicketID:   testTicketID,
		TicketType: "service",
		UserID:     testUserID,
		Username:   "alice",
		Service:    "https://app.example.com/callback",
		ExpiresAt:  fixedExpiry,
		CreatedAt:  fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteByTicketID ──────────────────────────────────────────────

func TestRepository_DeleteByTicketID_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 先 SELECT，再 DELETE
	mock.ExpectQuery(regexp.QuoteMeta(queryGetByTicketID)).
		WithArgs(testTicketID).
		WillReturnRows(sqlmock.NewRows(ticketCols()).AddRow(newTicketRow()...))

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteByTicketID)).
		WithArgs(testTicketID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	tk, err := r.DeleteByTicketID(context.Background(), testTicketID)
	assert.NoError(t, err)
	assert.NotNil(t, tk)
	assert.Equal(t, testTicketID, tk.TicketID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_DeleteByTicketID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByTicketID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(ticketCols()))

	r := NewRepository(db)
	tk, err := r.DeleteByTicketID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, tk)
}

// ── DeleteExpired ──────────────────────────────────────────────────

func TestRepository_DeleteExpired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteExpired)).
		WillReturnResult(sqlmock.NewResult(3, 3))

	r := NewRepository(db)
	n, err := r.DeleteExpired(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
