package forms

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/forms"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow3  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID3 = "00000000-0000-0000-0000-000000000001"
	testFormID = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// formCols: 11 列
func formCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description", "type",
		"properties", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newFormRow() []driver.Value {
	return []driver.Value{
		testFormID, testOrgID3, "my-form", "My Form", "desc", "signup",
		`{"fields":[]}`, true, fixedNow3, fixedNow3, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetFormByID)).
		WithArgs(testFormID, testOrgID3).
		WillReturnRows(sqlmock.NewRows(formCols()).AddRow(newFormRow()...))

	r := NewRepository(db)
	f, err := r.GetByID(context.Background(), testFormID, testOrgID3)
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, testFormID, f.ID)
	assert.Equal(t, "signup", f.Type)
	assert.True(t, f.IsEnabled)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetFormByID)).
		WithArgs("bad", testOrgID3).
		WillReturnRows(sqlmock.NewRows(formCols()))

	r := NewRepository(db)
	f, err := r.GetByID(context.Background(), "bad", testOrgID3)
	assert.Error(t, err)
	assert.Nil(t, f)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + is_enabled×2 + type×2 = 9
// listArgs: 9 + limit + offset = 11
func TestRepository_List_WithEnabled(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgID3, testOrgID3, "kw", "kw", "kw", enabled, enabled, "signup", "signup"}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountForms)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListForms)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(formCols()).AddRow(newFormRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID3,
		Keyword:        "kw",
		IsEnabled:      &enabled,
		Type:           "signup",
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_NilEnabled(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// IsEnabled == nil → 两处传 nil
	countArgs := []driver.Value{"", "", "", "", "", nil, nil, "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountForms)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListForms)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(formCols()))

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

	mock.ExpectExec(regexp.QuoteMeta(queryInsertForm)).
		WithArgs(
			testFormID, testOrgID3, "my-form", "My Form", "desc", "signup",
			`{"fields":[]}`, true, fixedNow3, fixedNow3, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Form{
		ID:             testFormID,
		OrganizationID: testOrgID3,
		Name:           "my-form",
		DisplayName:    "My Form",
		Description:    "desc",
		Type:           "signup",
		Properties:     `{"fields":[]}`,
		IsEnabled:      true,
		CreatedAt:      fixedNow3,
		UpdatedAt:      fixedNow3,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateForm)).
		WithArgs("Updated", "new-desc", "login", `{"x":1}`, false, fixedNow3, int64(2), testFormID, testOrgID3).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Form{
		ID:             testFormID,
		OrganizationID: testOrgID3,
		DisplayName:    "Updated",
		Description:    "new-desc",
		Type:           "login",
		Properties:     `{"x":1}`,
		IsEnabled:      false,
		UpdatedAt:      fixedNow3,
		Revision:       int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteForm)).
		WithArgs(testFormID, testOrgID3).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testFormID, testOrgID3)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteForm)).
		WithArgs(testFormID, testOrgID3).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testFormID, testOrgID3)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
