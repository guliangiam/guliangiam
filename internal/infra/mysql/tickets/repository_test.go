package tickets

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID    = "00000000-0000-0000-0000-000000000001"
	testTickID   = "00000000-0000-0000-0000-000000000002"
	testAssignee = "00000000-0000-0000-0000-000000000003"
	testCreator  = "00000000-0000-0000-0000-000000000004"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ticketCols: 15 列
func ticketCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "type", "status", "priority", "assignee", "creator",
		"description", "attachment", "resolved_at", "created_at", "updated_at", "revision",
	}
}

func newTicketRow() []driver.Value {
	return []driver.Value{
		testTickID, testOrgID, "my-ticket", "My Ticket", "support", "open", "P1", testAssignee, testCreator,
		"need help", "", fixedNow,
		fixedNow, fixedNow, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testTickID, testOrgID).
		WillReturnRows(sqlmock.NewRows(ticketCols()).AddRow(newTicketRow()...))

	r := NewRepository(db)
	tk, err := r.GetByID(context.Background(), testTickID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, tk)
	assert.Equal(t, testTickID, tk.ID)
	assert.Equal(t, "open", tk.Status)
	assert.Equal(t, int64(1), tk.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs("bad", testOrgID).
		WillReturnRows(sqlmock.NewRows(ticketCols()))

	r := NewRepository(db)
	tk, err := r.GetByID(context.Background(), "bad", testOrgID)
	assert.Error(t, err)
	assert.Nil(t, tk)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org×2 + keyword×3 + status×2 + assignee×2 + creator×2 + priority×2 + type×2 = 15
// listArgs: countArgs + 20 + 0 = 17
func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	status := "open"
	countArgs := []driver.Value{
		testOrgID, testOrgID,
		"kw", "kw", "kw",
		status, status,
		testAssignee, testAssignee,
		testCreator, testCreator,
		"P1", "P1",
		"vendor_feedback", "vendor_feedback",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountTickets)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListTickets)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(ticketCols()).AddRow(newTicketRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
		Keyword:        "kw",
		Status:         &status,
		Assignee:       testAssignee,
		Creator:        testCreator,
		Priority:       "P1",
		Type:           "vendor_feedback",
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_EmptyFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", "", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountTickets)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListTickets)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(ticketCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertTicket)).
		WithArgs(
			testTickID, testOrgID, "my-ticket", "My Ticket",
			"support", "open", "P1", testAssignee, testCreator,
			"need help", "", fixedNow,
			fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Ticket{
		ID:             testTickID,
		OrganizationID: testOrgID,
		Name:           "my-ticket",
		DisplayName:    "My Ticket",
		Type:           "support",
		Status:         "open",
		Priority:       "P1",
		Assignee:       testAssignee,
		Creator:        testCreator,
		Description:    "need help",
		ResolvedAt:     &fixedNow,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateTicket)).
		WithArgs("Updated Ticket", "bug", "resolved", "P2", testAssignee,
			"fixed", "", fixedNow, fixedNow, int64(2), testTickID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Ticket{
		ID:             testTickID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated Ticket",
		Type:           "bug",
		Status:         "resolved",
		Priority:       "P2",
		Assignee:       testAssignee,
		Description:    "fixed",
		ResolvedAt:     &fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       2,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateTicket)).
		WithArgs("Updated Ticket", "bug", "resolved", "P2", testAssignee,
			"fixed", "", fixedNow, fixedNow, int64(2), testTickID, testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Ticket{
		ID:             testTickID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated Ticket",
		Type:           "bug",
		Status:         "resolved",
		Priority:       "P2",
		Assignee:       testAssignee,
		Description:    "fixed",
		ResolvedAt:     &fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       2,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteTicket)).
		WithArgs(testTickID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testTickID, testOrgID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteTicket)).
		WithArgs(testTickID, testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testTickID, testOrgID)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
