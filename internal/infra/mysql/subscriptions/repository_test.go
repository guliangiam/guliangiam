package subscriptions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow      = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedFuture   = time.Date(2027, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID     = "00000000-0000-0000-0000-000000000001"
	testSubID     = "00000000-0000-0000-0000-000000000002"
	testUserID    = "00000000-0000-0000-0000-000000000003"
	testPlanID    = "00000000-0000-0000-0000-000000000004"
	testPricingID = "00000000-0000-0000-0000-000000000005"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// subCols: 14 列
func subCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "user_id", "plan_id", "pricing_id", "state",
		"start_at", "end_at", "expire_at", "created_at", "updated_at", "revision",
	}
}

func newSubRow() []driver.Value {
	return []driver.Value{
		testSubID, testOrgID, "my-sub", "My Subscription", testUserID, testPlanID, testPricingID, "active",
		fixedNow, fixedFuture, fixedFuture,
		fixedNow, fixedNow, int64(1),
	}
}

// ── GetByIDForOrganization ────────────────────────────────────────

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByIDForOrganization)).
		WithArgs(testSubID, testOrgID).
		WillReturnRows(sqlmock.NewRows(subCols()).AddRow(newSubRow()...))

	r := NewRepository(db)
	s, err := r.GetByIDForOrganization(context.Background(), testSubID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testSubID, s.ID)
	assert.Equal(t, "active", s.State)
	assert.Equal(t, int64(1), s.Revision)
	assert.NotNil(t, s.StartAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByIDForOrganization)).
		WithArgs(testSubID, testOrgID).
		WillReturnRows(sqlmock.NewRows(subCols()))

	r := NewRepository(db)
	s, err := r.GetByIDForOrganization(context.Background(), testSubID, testOrgID)
	assert.Error(t, err)
	assert.Nil(t, s)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org×2 + keyword×3 + user×2 + state×2 = 9
// listArgs: countArgs + 20 + 0 = 11
func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	state := "active"
	countArgs := []driver.Value{testOrgID, testOrgID, "kw", "kw", "kw", testUserID, testUserID, state, state}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountSubscriptions)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListSubscriptions)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(subCols()).AddRow(newSubRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
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

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountSubscriptions)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListSubscriptions)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(subCols()))

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

	mock.ExpectExec(regexp.QuoteMeta(queryInsertSubscription)).
		WithArgs(
			testSubID, testOrgID, "my-sub", "My Subscription",
			testUserID, testPlanID, testPricingID, "active",
			fixedNow, fixedFuture, fixedFuture,
			fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Subscription{
		ID:             testSubID,
		OrganizationID: testOrgID,
		Name:           "my-sub",
		DisplayName:    "My Subscription",
		User:           testUserID,
		Plan:           testPlanID,
		Pricing:        testPricingID,
		State:          "active",
		StartAt:        &fixedNow,
		EndAt:          &fixedFuture,
		ExpireAt:       &fixedFuture,
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

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateSubscription)).
		WithArgs("Updated Display", "cancelled", fixedNow, fixedFuture, fixedFuture, fixedNow, testSubID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Subscription{
		ID:             testSubID,
		OrganizationID: testOrgID,
		DisplayName:    "Updated Display",
		State:          "cancelled",
		StartAt:        &fixedNow,
		EndAt:          &fixedFuture,
		ExpireAt:       &fixedFuture,
		UpdatedAt:      fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteSubscription)).
		WithArgs(testSubID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testSubID, testOrgID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
