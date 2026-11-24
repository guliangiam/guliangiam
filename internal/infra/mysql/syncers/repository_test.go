package syncers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testSyncerID = "00000000-0000-0000-0000-000000000001"
	testOrgID    = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

func syncerCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "type",
		"database_type",
		"host", "port", "database", "table_name", "username",
		"password",
		"ssl_mode",
		"ssh_type", "ssh_host", "ssh_port", "ssh_username", "ssh_password", "ssh_cert_id",
		"is_enabled", "sync_interval", "field_mapping",
		"column_mapping",
		"is_read_only", "affinity", "affiliation_table", "avatar_base_url", "error_text",
		"last_sync_at", "created_at", "updated_at", "revision",
	}
}

func newSyncerRow() []driver.Value {
	encPwd, _ := syncerEncryptedMapper.EncryptColumn(syncerAAD(testSyncerID, syncerColumnPassword), "db-password")
	encSsh, _ := syncerEncryptedMapper.EncryptColumn(syncerAAD(testSyncerID, syncerColumnSSHPassword), "ssh-password")
	return []driver.Value{
		testSyncerID, testOrgID, "db-syncer-1", "DB Syncer 1", "mysql",
		"MySQL",
		"db.example.com", 3306, "appdb", "users_table", "admin",
		encPwd,
		"disable",
		"", "", 22, "", encSsh, "",
		true, 3600, "default",
		[]byte(`[]`),
		false, "", "", "", "",
		(*time.Time)(nil), fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_Syncers(t *testing.T) {
	assert.Equal(t, len(syncerCols()), len(newSyncerRow()), "cols vs row count mismatch")
}

func TestRepository_GetByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	assert.Contains(t, queryGetSyncerByIDForOrganization, "organization_id")
	mock.ExpectQuery(regexp.QuoteMeta(queryGetSyncerByIDForOrganization)).
		WithArgs(testSyncerID, testOrgID).
		WillReturnRows(sqlmock.NewRows(syncerCols()).AddRow(newSyncerRow()...))

	r := NewRepository(db)
	s, err := r.GetByIDForOrganization(context.Background(), testSyncerID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testSyncerID, s.ID)
	assert.Equal(t, "db-password", s.Password)     // 解密后
	assert.Equal(t, "ssh-password", s.SshPassword) // 解密后
	assert.Equal(t, int64(1), s.Revision)
	assert.Equal(t, testOrgID, s.OrganizationID)
}

func TestRepository_GetByIDForOrganization_ForeignOrganizationIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	assert.Contains(t, queryGetSyncerByIDForOrganization, "organization_id")
	mock.ExpectQuery(regexp.QuoteMeta(queryGetSyncerByIDForOrganization)).
		WithArgs(testSyncerID, "other-org").
		WillReturnRows(sqlmock.NewRows(syncerCols()))

	r := NewRepository(db)
	s, err := r.GetByIDForOrganization(context.Background(), testSyncerID, "other-org")
	assert.Error(t, err)
	assert.Nil(t, s)
}

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListSyncers)).
		WithArgs("", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(syncerCols()).AddRow(newSyncerRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountSyncers)).
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

	mock.ExpectExec(regexp.QuoteMeta(queryInsertSyncer)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	s := &domain.Syncer{
		ID: testSyncerID, OrganizationID: testOrgID, Name: "test", DisplayName: "Test",
		Type: "mysql", Host: "localhost", Port: 3306, Password: "db-password", SshPassword: "ssh-password",
		CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1,
	}
	err := r.Create(context.Background(), s)
	assert.NoError(t, err)
}

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateSyncer)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testSyncerID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testSyncerID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	s := &domain.Syncer{ID: testSyncerID, OrganizationID: testOrgID, DisplayName: "Updated", Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), s)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), s.Revision)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateSyncer)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testSyncerID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	s := &domain.Syncer{ID: testSyncerID, OrganizationID: testOrgID, Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteSyncer)).
		WithArgs(testSyncerID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testSyncerID, testOrgID)
	assert.NoError(t, err)
}

func TestRepository_Delete_ForeignOrganizationIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteSyncer)).
		WithArgs(testSyncerID, "other-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testSyncerID, "other-org")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

var _ domain.Repository = Repository{}
