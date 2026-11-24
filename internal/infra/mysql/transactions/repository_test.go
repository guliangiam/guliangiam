package transactions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow    = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID   = "00000000-0000-0000-0000-000000000001"
	testTxID    = "00000000-0000-0000-0000-000000000002"
	testUserID  = "00000000-0000-0000-0000-000000000003"
	testOrderID = "00000000-0000-0000-0000-000000000004"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// txCols: 15 列
func txCols() []string {
	return []string{
		"id", "organization_id", "user_id", "order_id", "payment_id", "transaction_type",
		"amount", "currency", "status", "provider_type", "provider_id", "metadata",
		"created_at", "updated_at", "revision",
	}
}

func newTxRow() []driver.Value {
	return []driver.Value{
		testTxID, testOrgID, testUserID, testOrderID, nil, "payment",
		99.99, "CNY", "pending", "alipay", nil, `{}`,
		fixedNow, fixedNow, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testTxID).
		WillReturnRows(sqlmock.NewRows(txCols()).AddRow(newTxRow()...))

	r := NewRepository(db)
	tr, err := r.GetByID(context.Background(), testTxID)
	assert.NoError(t, err)
	assert.NotNil(t, tr)
	assert.Equal(t, testTxID, tr.ID)
	assert.Equal(t, "payment", tr.TransactionType)
	assert.Equal(t, "pending", tr.Status)
	assert.Equal(t, float64(99.99), tr.Amount)
	assert.Equal(t, int64(1), tr.Revision)
	assert.Equal(t, testUserID, tr.UserID)
	assert.Equal(t, testOrderID, tr.OrderID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(txCols()))

	r := NewRepository(db)
	tr, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, tr)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org×2 + keyword×3 + user×2 + order×2 + status×2 + type×2 = 13
// listArgs: countArgs + 20 + 0 = 15
func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	status := "pending"
	typeVal := "payment"
	countArgs := []driver.Value{
		testOrgID, testOrgID,
		"kw", "kw", "kw",
		testUserID, testUserID,
		testOrderID, testOrderID,
		status, status,
		typeVal, typeVal,
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountTransactions)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListTransactions)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(txCols()).AddRow(newTxRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID:  testOrgID,
		Keyword:         "kw",
		UserID:          testUserID,
		OrderID:         testOrderID,
		Status:          &status,
		TransactionType: &typeVal,
		Page:            1,
		PageSize:        20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_EmptyFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountTransactions)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListTransactions)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(txCols()))

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

	meta := json.RawMessage(`{"order":"ABC"}`)
	mock.ExpectExec(regexp.QuoteMeta(queryInsertTransaction)).
		WithArgs(
			testTxID, testOrgID, testUserID, testOrderID, nil,
			"payment", 99.99, "CNY", "pending", "alipay",
			nil, `{"order":"ABC"}`,
			fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Transaction{
		ID:              testTxID,
		OrganizationID:  testOrgID,
		UserID:          testUserID,
		OrderID:         testOrderID,
		TransactionType: "payment",
		Amount:          99.99,
		Currency:        "CNY",
		Status:          "pending",
		ProviderType:    "alipay",
		Metadata:        meta,
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
		Revision:        int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateTransaction)).
		WithArgs("completed", `{}`, fixedNow, testTxID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Transaction{
		ID:             testTxID,
		OrganizationID: testOrgID,
		Status:         "completed",
		UpdatedAt:      fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteTransaction)).
		WithArgs(testTxID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testTxID, testOrgID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ForeignOrganizationIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteTransaction)).
		WithArgs(testTxID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testTxID, testOrgID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
