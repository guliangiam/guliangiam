package pricings

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow5  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID5 = "00000000-0000-0000-0000-000000000001"
	testPriID  = "00000000-0000-0000-0000-000000000003"
)

func newPricingMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// pricingCols: 12 列
func pricingCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description", "plans",
		"is_enabled", "trial_duration", "application", "created_at", "updated_at", "revision",
	}
}

func newPricingRow() []driver.Value {
	return []driver.Value{
		testPriID, testOrgID5, "pro-pricing", "Pro Pricing", "desc",
		`["plan1","plan2"]`, true, 14, "web", fixedNow5, fixedNow5, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPricingByID)).
		WithArgs(testPriID, testOrgID5).
		WillReturnRows(sqlmock.NewRows(pricingCols()).AddRow(newPricingRow()...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testPriID, testOrgID5)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testPriID, p.ID)
	assert.True(t, p.IsEnabled)
	assert.Equal(t, []string{"plan1", "plan2"}, p.Plans)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetPricingByID)).
		WithArgs("bad", "org-1").
		WillReturnRows(sqlmock.NewRows(pricingCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), "bad", "org-1")
	assert.Error(t, err)
	assert.Nil(t, p)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 7 + limit + offset = 9
func TestRepository_List_WithEnabled(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgID5, testOrgID5, "kw", "kw", "kw", enabled, enabled}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountPricings)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListPricings)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(pricingCols()).AddRow(newPricingRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID5,
		Keyword:        "kw",
		IsEnabled:      &enabled,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_NilEnabled(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountPricings)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListPricings)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(pricingCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertPricing)).
		WithArgs(
			testPriID, testOrgID5, "pro-pricing", "Pro Pricing", "desc",
			[]byte(`["plan1","plan2"]`), true, 14, "web", fixedNow5, fixedNow5, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Pricing{
		ID:             testPriID,
		OrganizationID: testOrgID5,
		Name:           "pro-pricing",
		DisplayName:    "Pro Pricing",
		Description:    "desc",
		Plans:          []string{"plan1", "plan2"},
		IsEnabled:      true,
		TrialDuration:  14,
		Application:    "web",
		CreatedAt:      fixedNow5,
		UpdatedAt:      fixedNow5,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePricing)).
		WithArgs(
			"Updated", "new-desc", []byte(`["plan3"]`), false, 7, "mobile", fixedNow5, int64(2), testPriID, testOrgID5,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Pricing{
		ID:             testPriID,
		OrganizationID: testOrgID5,
		DisplayName:    "Updated",
		Description:    "new-desc",
		Plans:          []string{"plan3"},
		IsEnabled:      false,
		TrialDuration:  7,
		Application:    "mobile",
		UpdatedAt:      fixedNow5,
		Revision:       int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePricing)).
		WithArgs(testPriID, testOrgID5).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPriID, testOrgID5)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── 租户谓词 0 行受影响 ─────────────────────────────────────────

func TestRepository_Update_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePricing)).
		WithArgs(
			"Updated", "new-desc", []byte(`["plan3"]`), false, 7, "mobile", fixedNow5, int64(2), testPriID, testOrgID5,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Pricing{
		ID: testPriID, OrganizationID: testOrgID5, DisplayName: "Updated", Description: "new-desc",
		Plans: []string{"plan3"}, IsEnabled: false, TrialDuration: 7, Application: "mobile",
		UpdatedAt: fixedNow5, Revision: int64(2),
	})
	assertNotFoundCode(t, err)
}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newPricingMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeletePricing)).
		WithArgs(testPriID, testOrgID5).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testPriID, testOrgID5)
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
