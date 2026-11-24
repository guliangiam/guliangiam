package kerberostickets

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedExpiry  = time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	testTicketID = "KRBTGT-0001"
	testRowID    = "00000000-0000-0000-0000-000000000001"
	testUserID   = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义（8 列）───────────────────────────────────────────────

func ticketCols() []string {
	return []string{
		"id", "ticket_id", "user_id", "username",
		"service", "realm", "expires_at", "created_at",
	}
}

func newTicketRow() []driver.Value {
	return []driver.Value{
		testRowID, testTicketID, testUserID, "alice",
		"HTTP/host.example.com", "EXAMPLE.COM", fixedExpiry, fixedNow,
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
	t2, err := r.GetByTicketID(context.Background(), testTicketID)
	assert.NoError(t, err)
	assert.NotNil(t, t2)
	assert.Equal(t, testRowID, t2.ID)
	assert.Equal(t, testTicketID, t2.TicketID)
	assert.Equal(t, "EXAMPLE.COM", t2.Realm)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByTicketID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByTicketID)).
		WithArgs("unknown").
		WillReturnRows(sqlmock.NewRows(ticketCols()))

	r := NewRepository(db)
	t2, err := r.GetByTicketID(context.Background(), "unknown")
	assert.Error(t, err)
	assert.Nil(t, t2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryCreate)).
		WithArgs(
			testRowID, testTicketID, testUserID, "alice",
			"HTTP/host.example.com", "EXAMPLE.COM", fixedExpiry, fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Ticket{
		ID:        testRowID,
		TicketID:  testTicketID,
		UserID:    testUserID,
		Username:  "alice",
		Service:   "HTTP/host.example.com",
		Realm:     "EXAMPLE.COM",
		ExpiresAt: fixedExpiry,
		CreatedAt: fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteByTicketID ────────────────────────────────────────────

func TestRepository_DeleteByTicketID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteByTicketID)).
		WithArgs(testTicketID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteByTicketID(context.Background(), testTicketID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── DeleteExpired ────────────────────────────────────────────────

func TestRepository_DeleteExpired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteExpired)).
		WillReturnResult(sqlmock.NewResult(1, 5)) // 5 rows deleted

	r := NewRepository(db)
	count, err := r.DeleteExpired(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
