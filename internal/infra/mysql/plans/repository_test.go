package plans

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/plans"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow4  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID4 = "00000000-0000-0000-0000-000000000001"
	testPlanID = "00000000-0000-0000-0000-000000000002"
)

func newPlanMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// planCols: 16 列
func planCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description", "price", "currency", "period",
		"product", "payment_providers", "state", "is_exclusive", "role", "created_at", "updated_at", "revision",
	}
}

func newPlanRow() []driver.Value {
	return []driver.Value{
		testPlanID, testOrgID4, "pro-plan", "Pro Plan", "desc",
		float64(99.99), "CNY", "monthly", "default", `["alipay","wechat"]`,
		"active", true, "default", fixedNow4, fixedNow4, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPlanByID)).
		WithArgs(testPlanID, testOrgID4).
		WillReturnRows(sqlmock.NewRows(planCols()).AddRow(newPlanRow()...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testPlanID, testOrgID4)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testPlanID, p.ID)
	assert.Equal(t, "active", p.State)
	assert.True(t, p.IsExclusive)
	assert.Equal(t, []string{"alipay", "wechat"}, p.PaymentProviders)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPlanByID)).
		WithArgs("bad", "org-1").
		WillReturnRows(sqlmock.NewRows(planCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), "bad", "org-1")
	assert.Error(t, err)
	assert.Nil(t, p)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + state×2 + period×2 + is_exclusive×2 = 11
// listArgs: 11 + limit + offset = 13
func TestRepository_List_WithExclusive(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	exclusive := true
	countArgs := []driver.Value{testOrgID4, testOrgID4, "kw", "kw", "kw", "active", "active", "monthly", "monthly", exclusive, exclusive}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountPlans)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListPlans)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(planCols()).AddRow(newPlanRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID4,
		Keyword:        "kw",
		State:          "active",
		Period:         "monthly",
		IsExclusive:    &exclusive,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_NilExclusive(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountPlans)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListPlans)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(planCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertPlan)).
		WithArgs(
			testPlanID, testOrgID4, "pro-plan", "Pro Plan", "desc",
			float64(99.99), "CNY", "monthly", "default", []byte(`["alipay","wechat"]`),
			"active", true, "default", fixedNow4, fixedNow4, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Plan{
		ID:               testPlanID,
		OrganizationID:   testOrgID4,
		Name:             "pro-plan",
		DisplayName:      "Pro Plan",
		Description:      "desc",
		Price:            float64(99.99),
		Currency:         "CNY",
		Period:           "monthly",
		Product:          "default",
		PaymentProviders: []string{"alipay", "wechat"},
		State:            "active",
		IsExclusive:      true,
		Role:             "default",
		CreatedAt:        fixedNow4,
		UpdatedAt:        fixedNow4,
		Revision:         int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePlan)).
		WithArgs(
			"Updated", "new-desc", float64(199.99), "USD", "yearly", "pro", []byte(`["stripe"]`),
			"archived", false, "admin", fixedNow4, int64(2), testPlanID, testOrgID4,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Plan{
		ID:               testPlanID,
		OrganizationID:   testOrgID4,
		DisplayName:      "Updated",
		Description:      "new-desc",
		Price:            float64(199.99),
		Currency:         "USD",
		Period:           "yearly",
		Product:          "pro",
		PaymentProviders: []string{"stripe"},
		State:            "archived",
		IsExclusive:      false,
		Role:             "admin",
		UpdatedAt:        fixedNow4,
		Revision:         int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePlan)).
		WithArgs(testPlanID, testOrgID4).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPlanID, testOrgID4)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 租户谓词 0 行受影响 ─────────────────────────────────────────

func TestRepository_Update_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePlan)).
		WithArgs(
			"Updated", "new-desc", float64(199.99), "USD", "yearly", "pro", []byte(`["stripe"]`),
			"archived", false, "admin", fixedNow4, int64(2), testPlanID, testOrgID4,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Plan{
		ID: testPlanID, OrganizationID: testOrgID4, DisplayName: "Updated", Description: "new-desc",
		Price: float64(199.99), Currency: "USD", Period: "yearly", Product: "pro",
		PaymentProviders: []string{"stripe"}, State: "archived", Role: "admin",
		UpdatedAt: fixedNow4, Revision: int64(2),
	})
	assertNotFoundCode(t, err)
}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newPlanMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePlan)).
		WithArgs(testPlanID, testOrgID4).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPlanID, testOrgID4)
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
