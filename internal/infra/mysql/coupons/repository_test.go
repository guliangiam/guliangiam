package coupons

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testID    = "00000000-0000-0000-0000-000000000010"
	testOrgID = "00000000-0000-0000-0000-000000000001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// couponFullCols: 23 列 (22 scan + revision 最后)
func couponFullCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description", "code",
		"discount_type", "discount", "max_discount", "scope", "products", "users",
		"quantity", "used_count", "max_usage_per_user", "start_time", "expire_time",
		"min_order_amount", "currency", "state", "created_at", "updated_at", "revision",
	}
}

func newCouponRow() []driver.Value {
	return []driver.Value{
		testID, testOrgID, "Welcome", "Welcome Coupon", "New user discount", "WELCOME10",
		"percentage", 10.0, 50.0, "all", `["product-1"]`, `["user-1"]`,
		100, 5, 1, "2026-09-01", "2026-12-31",
		0.0, "USD", "active", fixedNow, fixedNow, int64(1),
	}
}

// couponListCols: 10 列
func couponListCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "code",
		"discount_type", "discount", "state", "created_at", "updated_at",
	}
}

// ── GetByID ─────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows(couponFullCols()).AddRow(newCouponRow()...))

	r := NewRepository(db)
	c, err := r.GetByID(context.Background(), testID)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, testID, c.ID)
	assert.Equal(t, "Welcome Coupon", c.DisplayName)
	assert.Equal(t, []string{"product-1"}, c.Products)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(couponFullCols()))

	r := NewRepository(db)
	_, err := r.GetByID(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── List ─────────────────────────────────────────────────────────

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	q := domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Keyword:        "",
		DiscountType:   "",
	}

	// List 参数：org×2(2), keyword×4(4), state×2(2), discount_type×2(2), limit, offset(2) = 12
	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs(
			testOrgID, testOrgID,
			"", "", "", "",
			nil, nil,
			"", "",
			20, 0,
		).
		WillReturnRows(sqlmock.NewRows(couponListCols()).AddRow(
			testID, testOrgID, "Welcome", "Welcome Coupon", "WELCOME10",
			"percentage", 10.0, "active", fixedNow, fixedNow,
		))

	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs(
			testOrgID, testOrgID,
			"", "", "", "",
			nil, nil,
			"", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_WithState(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	state := "active"
	q := domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Keyword:        "",
		State:          &state,
		DiscountType:   "",
	}

	// state="active" 时 stateArg = "active" (出现 2 次)
	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs(
			testOrgID, testOrgID,
			"", "", "", "",
			"active", "active",
			"", "",
			20, 0,
		).
		WillReturnRows(sqlmock.NewRows(couponListCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs(
			testOrgID, testOrgID,
			"", "", "", "",
			"active", "active",
			"", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, items, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 23 个参数
	mock.ExpectExec(regexp.QuoteMeta(queryInsert)).
		WithArgs(
			testID, testOrgID, "Welcome", "Welcome Coupon",
			"New user discount", "WELCOME10",
			"percentage", 10.0, 50.0, "all",
			`["product-1"]`, `["user-1"]`,
			100, 0, 1, "2026-09-01", "2026-12-31",
			0.0, "USD", "active",
			fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Coupon{
		ID:              testID,
		OrganizationID:  testOrgID,
		Name:            "Welcome",
		DisplayName:     "Welcome Coupon",
		Description:     "New user discount",
		Code:            "WELCOME10",
		DiscountType:    "percentage",
		Discount:        10.0,
		MaxDiscount:     50.0,
		Scope:           "all",
		Products:        []string{"product-1"},
		Users:           []string{"user-1"},
		Quantity:        100,
		MaxUsagePerUser: 1,
		StartTime:       "2026-09-01",
		ExpireTime:      "2026-12-31",
		Currency:        "USD",
		State:           "active",
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
		Revision:        1,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 16 SET + 1 id = 17
	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(
			"Welcome Coupon", "desc", 10.0, 50.0, "all",
			`["product-1"]`, `["user-1"]`,
			100, 1, "2026-09-01", "2026-12-31",
			0.0, "USD", "active", fixedNow, int64(2),
			testID, testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Coupon{
		ID:              testID,
		OrganizationID:  testOrgID,
		DisplayName:     "Welcome Coupon",
		Description:     "desc",
		Discount:        10.0,
		MaxDiscount:     50.0,
		Scope:           "all",
		Products:        []string{"product-1"},
		Users:           []string{"user-1"},
		Quantity:        100,
		MaxUsagePerUser: 1,
		StartTime:       "2026-09-01",
		ExpireTime:      "2026-12-31",
		MinOrderAmount:  0.0,
		Currency:        "USD",
		State:           "active",
		UpdatedAt:       fixedNow,
		Revision:        2,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDelete)).
		WithArgs(testID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testID, testOrgID)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── CouponUsageRepository: Create ───────────────────────────────

var (
	usageID      = "00000000-0000-0000-0000-000000000020"
	testUserID   = "00000000-0000-0000-0000-000000000002"
	usageOrderID = "00000000-0000-0000-0000-000000000030"
)

func TestUsageRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUsageInsert)).
		WithArgs(usageID, testID, testUserID, usageOrderID, fixedNow, fixedNow).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewUsageRepository(db)
	err := r.Create(context.Background(), &domain.CouponUsage{
		ID:        usageID,
		CouponID:  testID,
		UserID:    testUserID,
		OrderID:   usageOrderID,
		UsedAt:    fixedNow,
		CreatedAt: fixedNow,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageRepository_ListByCoupon(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryUsageListByCoupon)).
		WithArgs(testID, 20, 0).
		WillReturnRows(sqlmock.NewRows(couponUsageCols()).AddRow(
			usageID, testID, testUserID, usageOrderID, fixedNow, fixedNow,
		))

	mock.ExpectQuery(regexp.QuoteMeta(queryUsageCountByCoupon)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	r := NewUsageRepository(db)
	items, total, err := r.ListByCoupon(context.Background(), testID, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageRepository_ListByCoupon_HostilePaginationKeepsOffsetNonNegative(t *testing.T) {
	cases := []struct {
		name           string
		page, pageSize int
		wantLimit      int
		wantOffset     int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			wantLimit: 20, wantOffset: (platformpagination.MaxPage - 1) * 20,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt,
			wantLimit: 500, wantOffset: 500,
		},
		{
			name: "normal page",
			page: 3, pageSize: 20,
			wantLimit: 20, wantOffset: 40,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			defer db.Close()

			mock.ExpectQuery(regexp.QuoteMeta(queryUsageListByCoupon)).
				WithArgs(testID, tc.wantLimit, tc.wantOffset).
				WillReturnRows(sqlmock.NewRows(couponUsageCols()))

			mock.ExpectQuery(regexp.QuoteMeta(queryUsageCountByCoupon)).
				WithArgs(testID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

			items, total, err := NewUsageRepository(db).ListByCoupon(context.Background(), testID, tc.page, tc.pageSize)
			require.NoError(t, err)
			assert.Empty(t, items)
			assert.Equal(t, int64(0), total)
			assert.GreaterOrEqual(t, tc.wantOffset, 0)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUsageRepository_CountByCouponAndUser(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryUsageCountByUser)).
		WithArgs(testID, testUserID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))

	r := NewUsageRepository(db)
	count, err := r.CountByCouponAndUser(context.Background(), testID, testUserID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var (
	_ domain.Repository            = Repository{}
	_ domain.CouponUsageRepository = UsageRepository{}
)

func TestRepository_GetByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByIDForOrganization)).
		WithArgs(testID, testOrgID).
		WillReturnRows(sqlmock.NewRows(couponFullCols()).AddRow(newCouponRow()...))

	r := NewRepository(db)
	c, err := r.GetByIDForOrganization(context.Background(), testID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, testOrgID, c.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_ForeignTenantIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByIDForOrganization)).
		WithArgs(testID, "foreign-org").
		WillReturnRows(sqlmock.NewRows(couponFullCols()))

	r := NewRepository(db)
	_, err := r.GetByIDForOrganization(context.Background(), testID, "foreign-org")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDelete)).
		WithArgs(testID, testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testID, testOrgID)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
