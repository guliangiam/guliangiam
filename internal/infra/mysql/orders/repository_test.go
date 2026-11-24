package orders

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrdID = "00000000-0000-0000-0000-000000000001"
	testOrgID = "00000000-0000-0000-0000-000000000002"
	testUser  = "00000000-0000-0000-0000-000000000003"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义（与 scanOrder 扫描顺序严格一致，共 21 列）────────

func orderCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "user_id",
		"products", "product_infos",
		"payment", "price", "currency", "state", "message",
		"payment_url", "trade_no", "coupon_name", "coupon_discount",
		"idempotency_key",
		"created_at", "updated_at", "paid_at", "revision",
	}
}

// newOrderRow 严格匹配 scanOrder 的 21 个 Scan 参数顺序。
func newOrderRow(withIdempotencyKey, withPaidAt bool) []driver.Value {
	row := []driver.Value{
		testOrdID, testOrgID, "order-test", "Test Order", testUser,
		[]byte(`["plan-1"]`),
		[]byte(`[{"name":"Plan 1","price":9.99}]`),
		"stripe", 9.99, "USD", "created", "",
		"", "trade-no-001", "", 0.0,
	}
	if withIdempotencyKey {
		row = append(row, "idem-001")
	} else {
		row = append(row, nil)
	}
	row = append(row, fixedNow, fixedNow)
	if withPaidAt {
		row = append(row, fixedNow)
	} else {
		row = append(row, nil)
	}
	row = append(row, int64(1))
	return row
}

// ── Test 列数 ────────────────────────────────────────────────────

func Test_colCounts_Orders(t *testing.T) {
	t.Logf("orderCols count: %d", len(orderCols()))
	t.Logf("orderRow count (no idem, no paid_at): %d", len(newOrderRow(false, false)))
	assert.Equal(t, len(orderCols()), len(newOrderRow(false, false)), "cols vs row count mismatch")
}

// ── GetByID ──────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrderByID)).
		WithArgs(testOrdID).
		WillReturnRows(sqlmock.NewRows(orderCols()).AddRow(newOrderRow(true, false)...))

	r := NewRepository(db)
	o, err := r.GetByID(context.Background(), testOrdID)
	assert.NoError(t, err)
	assert.NotNil(t, o)
	assert.Equal(t, testOrdID, o.ID)
	assert.Equal(t, "order-test", o.Name)
	assert.Equal(t, testUser, o.User)
	assert.Equal(t, []string{"plan-1"}, o.Products)
	assert.Equal(t, "idem-001", o.IdempotencyKey)
	assert.Nil(t, o.PaidAt)
	assert.Equal(t, int64(1), o.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrderByID)).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(orderCols()))

	r := NewRepository(db)
	o, err := r.GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, o)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByTradeNo ────────────────────────────────────────────────

func TestRepository_GetByTradeNo(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrderByTradeNo)).
		WithArgs("trade-no-001").
		WillReturnRows(sqlmock.NewRows(orderCols()).AddRow(newOrderRow(false, true)...))

	r := NewRepository(db)
	o, err := r.GetByTradeNo(context.Background(), "trade-no-001")
	assert.NoError(t, err)
	assert.NotNil(t, o)
	assert.Equal(t, "trade-no-001", o.TradeNo)
	assert.NotNil(t, o.PaidAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── List ────────────────────────────────────────────────────────
// List 参数（11 个）：org_id×2, keyword×3, user×2, state×2, limit, offset
// Count 参数（9 个）：org_id×2, keyword×3, user×2, state×2

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListOrders)).
		WithArgs(
			"", "", // org_id × 2
			"", "", "", // keyword × 3
			"", "", // user × 2
			nil, nil, // state × 2
			20, 0, // limit, offset
		).
		WillReturnRows(sqlmock.NewRows(orderCols()).AddRow(newOrderRow(false, false)...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountOrders)).
		WithArgs("", "", "", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testOrdID, items[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListOrders)).
		WithArgs("", "", "", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(orderCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountOrders)).
		WithArgs("", "", "", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 21 个 ? 参数
	mock.ExpectExec(regexp.QuoteMeta(queryInsertOrder)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	o := &domain.Order{
		ID:             testOrdID,
		OrganizationID: testOrgID,
		Name:           "order-test",
		DisplayName:    "Test Order",
		User:           testUser,
		Products:       []string{"plan-1"},
		Payment:        "stripe",
		Price:          9.99,
		Currency:       "USD",
		State:          "created",
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
	}
	err := r.Create(context.Background(), o)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ───────────────────────────────────────────────────────
// UPDATE: 17 个 ?（SET 14 列 + WHERE id + WHERE old_revision + WHERE organization_id）

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 17 个 ?
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateOrder)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testOrdID, int64(1), testOrgID, // id + old_revision + organization_id
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// SELECT revision 回读
	mock.ExpectQuery(regexp.QuoteMeta(querySelectOrderRevision)).
		WithArgs(testOrdID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	o := &domain.Order{
		ID:             testOrdID,
		OrganizationID: testOrgID,
		Revision:       2,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), o)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), o.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateOrder)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testOrdID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 rows affected = conflict

	r := NewRepository(db)
	o := &domain.Order{
		ID:             testOrdID,
		OrganizationID: testOrgID,
		Revision:       2,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), o)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteOrder)).
		WithArgs(testOrdID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testOrdID, testOrgID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
