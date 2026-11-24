package products

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow6  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID6 = "00000000-0000-0000-0000-000000000001"
	testProdID = "00000000-0000-0000-0000-000000000004"
)

func newProductMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// productCols: 21 列
func productCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "image", "detail", "description",
		"tag", "currency", "price", "quantity", "sold", "is_recharge", "recharge_options",
		"disable_custom_recharge", "providers", "success_url", "status", "created_at", "updated_at", "revision",
	}
}

func newProductRow() []driver.Value {
	return []driver.Value{
		testProdID, testOrgID6, "gold-card", "Gold Card", "img.png", "detail", "desc",
		"tag", "CNY", float64(999.00), 100, 0, true, `[100,200,500]`,
		false, `["alipay","wechat"]`, "https://success.example.com", "active", fixedNow6, fixedNow6, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetProductByID)).
		WithArgs(testProdID).
		WillReturnRows(sqlmock.NewRows(productCols()).AddRow(newProductRow()...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testProdID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testProdID, p.ID)
	assert.True(t, p.IsRecharge)
	assert.Equal(t, []float64{100, 200, 500}, p.RechargeOptions)
	assert.Equal(t, []string{"alipay", "wechat"}, p.Providers)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetProductByIDForOrganization)).
		WithArgs(testProdID, testOrgID6).
		WillReturnRows(sqlmock.NewRows(productCols()).AddRow(newProductRow()...))

	r := NewRepository(db)
	p, err := r.GetByIDForOrganization(context.Background(), testProdID, testOrgID6)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testProdID, p.ID)
	assert.True(t, p.IsRecharge)
	assert.Equal(t, []float64{100, 200, 500}, p.RechargeOptions)
	assert.Equal(t, []string{"alipay", "wechat"}, p.Providers)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetProductByIDForOrganization)).
		WithArgs("bad", "org-1").
		WillReturnRows(sqlmock.NewRows(productCols()))

	r := NewRepository(db)
	p, err := r.GetByIDForOrganization(context.Background(), "bad", "org-1")
	assert.Error(t, err)
	assert.Nil(t, p)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + state×2 + is_recharge×2 = 9
// listArgs: 9 + limit + offset = 11
func TestRepository_List_WithStateAndRecharge(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	state := "active"
	recharge := true
	countArgs := []driver.Value{testOrgID6, testOrgID6, "kw", "kw", "kw", state, state, recharge, recharge}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountProducts)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListProducts)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(productCols()).AddRow(newProductRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID6,
		Keyword:        "kw",
		State:          &state,
		IsRecharge:     &recharge,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_NilStateAndRecharge(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", nil, nil, nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountProducts)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListProducts)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(productCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertProduct)).
		WithArgs(
			testProdID, testOrgID6, "gold-card", "Gold Card", "img.png", "detail", "desc",
			"tag", "CNY", float64(999.00), 100, 0, true, []byte(`[100,200,500]`),
			false, []byte(`["alipay","wechat"]`), "https://success.example.com", "active",
			fixedNow6, fixedNow6, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Product{
		ID:                    testProdID,
		OrganizationID:        testOrgID6,
		Name:                  "gold-card",
		DisplayName:           "Gold Card",
		Image:                 "img.png",
		Detail:                "detail",
		Description:           "desc",
		Tag:                   "tag",
		Currency:              "CNY",
		Price:                 float64(999.00),
		Quantity:              100,
		Sold:                  0,
		IsRecharge:            true,
		RechargeOptions:       []float64{100, 200, 500},
		DisableCustomRecharge: false,
		Providers:             []string{"alipay", "wechat"},
		SuccessUrl:            "https://success.example.com",
		State:                 "active",
		CreatedAt:             fixedNow6,
		UpdatedAt:             fixedNow6,
		Revision:              int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateProduct)).
		WithArgs(
			"Updated", "new-img", "new-detail", "new-desc", "new-tag", "USD", float64(1999.00),
			50, 10, false, []byte(`[1000]`), true, []byte(`["stripe"]`), "https://new-success.com",
			"discontinued", fixedNow6, int64(2), testProdID, testOrgID6,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Product{
		ID:                    testProdID,
		OrganizationID:        testOrgID6,
		DisplayName:           "Updated",
		Image:                 "new-img",
		Detail:                "new-detail",
		Description:           "new-desc",
		Tag:                   "new-tag",
		Currency:              "USD",
		Price:                 float64(1999.00),
		Quantity:              50,
		Sold:                  10,
		IsRecharge:            false,
		RechargeOptions:       []float64{1000},
		DisableCustomRecharge: true,
		Providers:             []string{"stripe"},
		SuccessUrl:            "https://new-success.com",
		State:                 "discontinued",
		UpdatedAt:             fixedNow6,
		Revision:              int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteProduct)).
		WithArgs(testProdID, testOrgID6).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testProdID, testOrgID6)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 租户谓词 0 行受影响 ─────────────────────────────────────────

func TestRepository_Update_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateProduct)).
		WithArgs(
			"Updated", "new-img", "new-detail", "new-desc", "new-tag", "USD", float64(1999.00),
			50, 10, false, []byte(`[1000]`), true, []byte(`["stripe"]`), "https://new-success.com",
			"discontinued", fixedNow6, int64(2), testProdID, testOrgID6,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Product{
		ID: testProdID, OrganizationID: testOrgID6, DisplayName: "Updated", Image: "new-img",
		Detail: "new-detail", Description: "new-desc", Tag: "new-tag", Currency: "USD",
		Price: float64(1999.00), Quantity: 50, Sold: 10, RechargeOptions: []float64{1000},
		DisableCustomRecharge: true, Providers: []string{"stripe"}, SuccessUrl: "https://new-success.com",
		State: "discontinued", UpdatedAt: fixedNow6, Revision: int64(2),
	})
	assertNotFoundCode(t, err)
}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newProductMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteProduct)).
		WithArgs(testProdID, testOrgID6).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testProdID, testOrgID6)
	assertNotFoundCode(t, err)
}

func assertNotFoundCode(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %v", err)
	}
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// 编译期接口断言
var _ domain.Repository = Repository{}
