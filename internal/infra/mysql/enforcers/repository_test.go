package enforcers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

var (
	fixedNow       = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testEnforcerID = "00000000-0000-0000-0000-000000000001"
	testOrgID      = "00000000-0000-0000-0000-000000000002"
	testModelID    = "model-001"
	testAdapterID  = "adapter-001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

func enforcerCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description",
		"model_id", "adapter_id", "is_enabled", "owner",
		"created_at", "updated_at", "revision",
	}
}

func newEnforcerRow() []driver.Value {
	return []driver.Value{
		testEnforcerID, testOrgID, "default-enforcer", "Default Enforcer", "Main enforcer",
		testModelID, testAdapterID, true, "owner1",
		fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_Enforcers(t *testing.T) {
	assert.Equal(t, len(enforcerCols()), len(newEnforcerRow()), "cols vs row count mismatch")
}

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetEnforcerByID)).
		WithArgs(testEnforcerID).
		WillReturnRows(sqlmock.NewRows(enforcerCols()).AddRow(newEnforcerRow()...))

	r := NewRepository(db)
	e, err := r.GetByID(context.Background(), testEnforcerID)
	assert.NoError(t, err)
	assert.NotNil(t, e)
	assert.Equal(t, testEnforcerID, e.ID)
	assert.Equal(t, testModelID, e.ModelID)
	assert.Equal(t, testAdapterID, e.AdapterID)
	assert.Equal(t, int64(1), e.Revision)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetEnforcerByID)).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(enforcerCols()))

	r := NewRepository(db)
	e, err := r.GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, e)
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetEnforcerByIDForOrganization)).
		WithArgs(testEnforcerID, testOrgID).
		WillReturnRows(sqlmock.NewRows(enforcerCols()).AddRow(newEnforcerRow()...))

	r := NewRepository(db)
	e, err := r.GetByIDForOrganization(context.Background(), testEnforcerID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, e)
	assert.Equal(t, testOrgID, e.OrganizationID)
}

func TestRepository_GetByIDForOrganization_ForeignTenantNotFound(t *testing.T) {
	assert.Contains(t, queryGetEnforcerByIDForOrganization, "organization_id = ?")
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetEnforcerByIDForOrganization)).
		WithArgs(testEnforcerID, "other-org").
		WillReturnRows(sqlmock.NewRows(enforcerCols()))

	r := NewRepository(db)
	e, err := r.GetByIDForOrganization(context.Background(), testEnforcerID, "other-org")
	assert.Error(t, err)
	assert.Nil(t, e)
}

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListEnforcers)).
		WithArgs("", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(enforcerCols()).AddRow(newEnforcerRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountEnforcers)).
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

	mock.ExpectExec(regexp.QuoteMeta(queryInsertEnforcer)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	e := &domain.Enforcer{
		ID: testEnforcerID, OrganizationID: testOrgID, Name: "default", DisplayName: "Default",
		ModelID: testModelID, AdapterID: testAdapterID, IsEnabled: true,
		CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1,
	}
	err := r.Create(context.Background(), e)
	assert.NoError(t, err)
}

func TestRepository_Update(t *testing.T) {
	assert.Contains(t, queryUpdateEnforcer, "organization_id = ?", "update must carry the entity organization predicate")
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateEnforcer)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testEnforcerID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testEnforcerID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	e := &domain.Enforcer{ID: testEnforcerID, OrganizationID: testOrgID, DisplayName: "Updated", Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), e)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), e.Revision)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateEnforcer)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testEnforcerID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	e := &domain.Enforcer{ID: testEnforcerID, OrganizationID: testOrgID, Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), e)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

func TestRepository_Delete(t *testing.T) {
	assert.Contains(t, queryDeleteEnforcer, "organization_id = ?", "delete must carry the trusted tenant predicate")
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteEnforcer)).
		WithArgs(testEnforcerID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testEnforcerID, testOrgID)
	assert.NoError(t, err)
}

func TestRepository_Delete_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteEnforcer)).
		WithArgs(testEnforcerID, "other-org").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testEnforcerID, "other-org")
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

var _ domain.Repository = Repository{}
