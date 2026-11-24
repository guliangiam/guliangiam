package models

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/models"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fixedNow    = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testModelID = "00000000-0000-0000-0000-000000000001"
	testOrgID   = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

func modelCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description",
		"model_text", "owner", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newModelRow() []driver.Value {
	return []driver.Value{
		testModelID, testOrgID, "default-model", "Default Model", "Casbin ABAC model",
		"[request_definition]...", "owner1", true,
		fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_Models(t *testing.T) {
	assert.Equal(t, len(modelCols()), len(newModelRow()), "cols vs row count mismatch")
}

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetModelByID)).
		WithArgs(testModelID).
		WillReturnRows(sqlmock.NewRows(modelCols()).AddRow(newModelRow()...))

	r := NewRepository(db)
	m, err := r.GetByID(context.Background(), testModelID)
	assert.NoError(t, err)
	assert.NotNil(t, m)
	assert.Equal(t, testModelID, m.ID)
	assert.Equal(t, int64(1), m.Revision)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetModelByID)).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(modelCols()))

	r := NewRepository(db)
	m, err := r.GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, m)
}

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListModels)).
		WithArgs("", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(modelCols()).AddRow(newModelRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountModels)).
		WithArgs("", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertModel)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	m := &domain.Model{
		ID: testModelID, OrganizationID: testOrgID, Name: "default", DisplayName: "Default",
		ModelText: "[request_definition]...", IsEnabled: true,
		CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1,
	}
	err := r.Create(context.Background(), m)
	assert.NoError(t, err)
}

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateModel)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), testModelID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testModelID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	m := &domain.Model{ID: testModelID, OrganizationID: testOrgID, DisplayName: "Updated", Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), m)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), m.Revision)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateModel)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), testModelID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	m := &domain.Model{ID: testModelID, OrganizationID: testOrgID, Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), m)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteModel)).
		WithArgs(testModelID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testModelID, testOrgID)
	assert.NoError(t, err)
}

var _ domain.Repository = Repository{}

func TestRepository_GetByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetModelByIDForOrganization)).
		WithArgs(testModelID, testOrgID).
		WillReturnRows(sqlmock.NewRows(modelCols()).AddRow(newModelRow()...))

	r := NewRepository(db)
	m, err := r.GetByIDForOrganization(context.Background(), testModelID, testOrgID)
	assert.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, testOrgID, m.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteModel)).
		WithArgs(testModelID, testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testModelID, testOrgID)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
