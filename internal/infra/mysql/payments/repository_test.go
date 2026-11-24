package payments

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow6  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID6 = "00000000-0000-0000-0000-000000000001"
	testPayID  = "00000000-0000-0000-0000-000000000002"
	testUserID = "00000000-0000-0000-0000-000000000003"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// payCols: 32 列
func payCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "provider", "type",
		"products", "product_name", "product_display_name", "detail",
		"currency", "price", "user_id", "person_name", "person_id_card",
		"person_email", "person_phone", "invoice_type", "invoice_title",
		"invoice_tax_id", "invoice_remark", "invoice_url", "order_id",
		"out_order_id", "pay_url", "success_url", "status", "message",
		"idempotency_key", "created_at", "updated_at", "revision",
	}
}

func newPayRow() []driver.Value {
	productsJSON, _ := json.Marshal([]string{"plan-pro", "addon-storage"})
	return []driver.Value{
		testPayID, testOrgID6, "my-pay", "My Payment", "stripe", "subscription",
		string(productsJSON), "Pro Plan", "Pro Plan Display", "Monthly subscription",
		"USD", 29.99, testUserID, "张三", "ID123456",
		"zhangsan@example.com", "+8613800138000", "personal", "张三",
		"TAX001", "no remark", "", "order-abc",
		"out-xyz", "https://pay.example.com", "https://success.example.com",
		"created", "", "idem-key-001", fixedNow6, fixedNow6, int64(1),
	}
}

// newPayRowNoIdem 没有 idempotency_key 的行
func newPayRowNoIdem() []driver.Value {
	productsJSON, _ := json.Marshal([]string{"plan-pro"})
	return []driver.Value{
		testPayID, testOrgID6, "my-pay", "My Payment", "stripe", "subscription",
		string(productsJSON), "Pro Plan", "Pro Plan Display", "",
		"USD", 29.99, testUserID, "张三", "",
		"", "", "", "",
		"", "", "", "order-abc",
		"", "", "", "created", "",
		sql.NullString{Valid: false}, fixedNow6, fixedNow6, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPaymentByID)).
		WithArgs(testPayID).
		WillReturnRows(sqlmock.NewRows(payCols()).AddRow(newPayRow()...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testPayID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testPayID, p.ID)
	assert.Equal(t, "stripe", p.Provider)
	assert.Equal(t, []string{"plan-pro", "addon-storage"}, p.Products)
	assert.Equal(t, "idem-key-001", p.IdempotencyKey)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_FoundNoIdem(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPaymentByID)).
		WithArgs(testPayID).
		WillReturnRows(sqlmock.NewRows(payCols()).AddRow(newPayRowNoIdem()...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testPayID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Empty(t, p.IdempotencyKey)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPaymentByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(payCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, p)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + user_id×2 + state×2 = 9
// listArgs: 9 + limit + offset = 11
func TestRepository_List_AllFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	state := "created"
	countArgs := []driver.Value{testOrgID6, testOrgID6, "kw", "kw", "kw", testUserID, testUserID, state, state}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountPayments)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListPayments)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(payCols()).AddRow(newPayRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID6,
		Keyword:        "kw",
		User:           testUserID,
		State:          &state,
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

	countArgs := []driver.Value{"", "", "", "", "", "", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountPayments)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListPayments)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(payCols()))

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

	productsJSON, _ := json.Marshal([]string{"plan-pro"})

	mock.ExpectExec(regexp.QuoteMeta(queryInsertPayment)).
		WithArgs(
			testPayID, testOrgID6, "my-pay", "My Payment", "stripe", "subscription",
			string(productsJSON), "Pro Plan", "Pro Plan Display", "Monthly subscription",
			"USD", 29.99, testUserID, "张三", "ID123456",
			"zhangsan@example.com", "+8613800138000", "personal", "张三",
			"TAX001", "no remark", "", "order-abc",
			"out-xyz", "https://pay.example.com", "https://success.example.com",
			"created", "", sql.NullString{String: "idem-key-001", Valid: true},
			fixedNow6, fixedNow6, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Payment{
		ID:                 testPayID,
		OrganizationID:     testOrgID6,
		Name:               "my-pay",
		DisplayName:        "My Payment",
		Provider:           "stripe",
		Type:               "subscription",
		Products:           []string{"plan-pro"},
		ProductName:        "Pro Plan",
		ProductDisplayName: "Pro Plan Display",
		Detail:             "Monthly subscription",
		Currency:           "USD",
		Price:              29.99,
		User:               testUserID,
		PersonName:         "张三",
		PersonIdCard:       "ID123456",
		PersonEmail:        "zhangsan@example.com",
		PersonPhone:        "+8613800138000",
		InvoiceType:        "personal",
		InvoiceTitle:       "张三",
		InvoiceTaxId:       "TAX001",
		InvoiceRemark:      "no remark",
		Order:              "order-abc",
		OutOrderId:         "out-xyz",
		PayUrl:             "https://pay.example.com",
		SuccessUrl:         "https://success.example.com",
		State:              "created",
		IdempotencyKey:     "idem-key-001",
		CreatedAt:          fixedNow6,
		UpdatedAt:          fixedNow6,
		Revision:           int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Create_NoIdem(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	productsJSON, _ := json.Marshal([]string{"plan-pro"})

	mock.ExpectExec(regexp.QuoteMeta(queryInsertPayment)).
		WithArgs(
			testPayID, testOrgID6, "my-pay", "My Payment", "stripe", "subscription",
			string(productsJSON), "Pro Plan", "", "",
			"USD", 29.99, testUserID, "张三", "",
			"", "", "", "",
			"", "", "", "order-abc",
			"", "", "", "created", "",
			sql.NullString{Valid: false}, fixedNow6, fixedNow6, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Payment{
		ID:             testPayID,
		OrganizationID: testOrgID6,
		Name:           "my-pay",
		DisplayName:    "My Payment",
		Provider:       "stripe",
		Type:           "subscription",
		Products:       []string{"plan-pro"},
		ProductName:    "Pro Plan",
		Currency:       "USD",
		Price:          29.99,
		User:           testUserID,
		PersonName:     "张三",
		Order:          "order-abc",
		State:          "created",
		CreatedAt:      fixedNow6,
		UpdatedAt:      fixedNow6,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePayment)).
		WithArgs(
			"Updated", "alipay", "one-time", "succeeded", "ok",
			"https://new-pay.example.com", "https://new-success.example.com",
			"new-out-xyz", "", sql.NullString{String: "new-idem", Valid: true},
			fixedNow6, int64(2), testPayID, testOrgID6,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Payment{
		ID:             testPayID,
		OrganizationID: testOrgID6,
		DisplayName:    "Updated",
		Provider:       "alipay",
		Type:           "one-time",
		State:          "succeeded",
		Message:        "ok",
		PayUrl:         "https://new-pay.example.com",
		SuccessUrl:     "https://new-success.example.com",
		OutOrderId:     "new-out-xyz",
		IdempotencyKey: "new-idem",
		UpdatedAt:      fixedNow6,
		Revision:       int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePayment)).
		WithArgs(testPayID, testOrgID6).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPayID, testOrgID6)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ForeignOrganizationNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePayment)).
		WithArgs(testPayID, "foreign-org").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPayID, "foreign-org")
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_GetByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPaymentByIDForOrganization)).
		WithArgs(testPayID, testOrgID6).
		WillReturnRows(sqlmock.NewRows(payCols()).AddRow(newPayRow()...))

	r := NewRepository(db)
	p, err := r.GetByIDForOrganization(context.Background(), testPayID, testOrgID6)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testPayID, p.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPaymentByIDForOrganization)).
		WithArgs(testPayID, "foreign-org").
		WillReturnRows(sqlmock.NewRows(payCols()))

	r := NewRepository(db)
	p, err := r.GetByIDForOrganization(context.Background(), testPayID, "foreign-org")
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestRepository_ByIDQueriesCarryTenantPredicate(t *testing.T) {
	assert.Contains(t, queryGetPaymentByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, queryUpdatePayment, "WHERE id=? AND organization_id=?")
	assert.Contains(t, queryDeletePayment, "WHERE id = ? AND organization_id = ?")
}

// ── Channel Configs ─────────────────────────────────────────────

func TestChannelConfigsRepository_CreateIncludesProviderID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryChannelConfigInsert)).
		WithArgs(
			testPayID, testOrgID6, testPayID, "alipay", "production",
			"sealed", "fingerprint", nil, 1, "draft",
			nil, nil, nil, fixedNow6, fixedNow6, nil, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewChannelConfigsRepository(db)
	err := r.Create(context.Background(), &domain.PayChannelConfig{
		ID: testPayID, OrganizationID: testOrgID6, ProviderID: testPayID,
		Channel: "alipay", Environment: "production", ConfigSealed: "sealed",
		ConfigFingerprint: "fingerprint", VersionNo: 1, Status: "draft",
		CreatedAt: fixedNow6, UpdatedAt: fixedNow6, Revision: 1,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelConfigsRepository_GetByIDReadsProviderID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	columns := []string{
		"id", "organization_id", "provider_id", "channel", "environment",
		"config_sealed", "config_fingerprint", "config_set", "version_no", "status",
		"test_passed_at", "activated_at", "disabled_at", "created_at", "updated_at", "deleted_at", "revision",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryChannelConfigGetByID)).
		WithArgs(testPayID).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			testPayID, testOrgID6, testPayID, "alipay", "production",
			"sealed", "fingerprint", nil, 1, "draft",
			nil, nil, nil, fixedNow6, fixedNow6, nil, int64(1),
		))

	r := NewChannelConfigsRepository(db)
	config, err := r.GetByID(context.Background(), testPayID)
	assert.NoError(t, err)
	assert.Equal(t, testPayID, config.ProviderID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
