package ldaps

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testLdapID = "00000000-0000-0000-0000-000000000001"
	testOrgID  = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

func ldapCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name",
		"server_name", "host", "port", "admin",
		"bind_dn", "bind_password", "base_dn",
		"type", "attribute_name", "password_type",
		"filter", "filter_fields",
		"enable_ssl", "allow_self_signed_cert",
		"enable_groups", "default_groups",
		"custom_attributes",
		"is_enabled", "sync_interval", "last_sync_at",
		"created_at", "updated_at", "revision",
	}
}

func newLdapRow() []driver.Value {
	encPwd, _ := ldapEncryptedMapper.EncryptColumn(ldapAAD(testLdapID), "bind-secret")
	return []driver.Value{
		testLdapID, testOrgID, "corporate-ldap", "Corporate LDAP",
		"ldap.example.com", "ldap.example.com", 389, "admin",
		"cn=admin,dc=example,dc=com", encPwd, "dc=example,dc=com",
		"Active Directory", "sAMAccountName", "Plain",
		"(&(objectClass=user))", []byte(`["mail"]`),
		true, false,
		true, []byte(`["Domain Users"]`),
		[]byte(`{"attr1":"val1"}`),
		true, 3600, (*time.Time)(nil),
		fixedNow, fixedNow, int64(1),
	}
}

func Test_colCounts_Ldaps(t *testing.T) {
	assert.Equal(t, len(ldapCols()), len(newLdapRow()), "cols vs row count mismatch")
}

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetLdapByID)).
		WithArgs(testLdapID).
		WillReturnRows(sqlmock.NewRows(ldapCols()).AddRow(newLdapRow()...))

	r := NewRepository(db)
	l, err := r.GetByID(context.Background(), testLdapID)
	assert.NoError(t, err)
	assert.NotNil(t, l)
	assert.Equal(t, testLdapID, l.ID)
	assert.Equal(t, "bind-secret", l.BindPassword) // 解密后
	assert.Equal(t, []string{"mail"}, l.FilterFields)
	assert.Equal(t, map[string]string{"attr1": "val1"}, l.CustomAttributes)
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetLdapByIDForOrganization)).
		WithArgs(testLdapID, testOrgID).
		WillReturnRows(sqlmock.NewRows(ldapCols()).AddRow(newLdapRow()...))

	r := NewRepository(db)
	l, err := r.GetByIDForOrganization(context.Background(), testLdapID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, l)
	assert.Equal(t, testOrgID, l.OrganizationID)
	assert.Equal(t, "bind-secret", l.BindPassword) // 解密后
}

func TestRepository_GetByIDForOrganization_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetLdapByIDForOrganization)).
		WithArgs(testLdapID, "other-org").
		WillReturnRows(sqlmock.NewRows(ldapCols()))

	r := NewRepository(db)
	l, err := r.GetByIDForOrganization(context.Background(), testLdapID, "other-org")
	assert.Error(t, err)
	assert.Nil(t, l)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetLdapByID)).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(ldapCols()))

	r := NewRepository(db)
	l, err := r.GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, l)
}

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListLdaps)).
		WithArgs("", "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(ldapCols()).AddRow(newLdapRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCountLdaps)).
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

	mock.ExpectExec(regexp.QuoteMeta(queryInsertLdap)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	l := &domain.Ldap{
		ID: testLdapID, OrganizationID: testOrgID, Name: "test", DisplayName: "Test",
		Host: "ldap.example.com", Port: 389, BindDN: "cn=admin", BindPassword: "bind-secret",
		BaseDN: "dc=example,dc=com", IsEnabled: true, FilterFields: []string{"mail"},
		DefaultGroups: []string{"Users"}, CustomAttributes: map[string]string{},
		CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1,
	}
	err := r.Create(context.Background(), l)
	assert.NoError(t, err)
}

func TestRepository_Update(t *testing.T) {
	assert.Contains(t, queryUpdateLdap, "organization_id = ?", "update must carry the entity organization predicate")
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateLdap)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testLdapID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevision)).
		WithArgs(testLdapID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	l := &domain.Ldap{ID: testLdapID, OrganizationID: testOrgID, DisplayName: "Updated", Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), l)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), l.Revision)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateLdap)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testLdapID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	l := &domain.Ldap{ID: testLdapID, OrganizationID: testOrgID, Revision: 1, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), l)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

func TestRepository_Delete(t *testing.T) {
	assert.Contains(t, queryDeleteLdap, "organization_id = ?", "delete must carry the trusted tenant predicate")
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteLdap)).
		WithArgs(testLdapID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testLdapID, testOrgID)
	assert.NoError(t, err)
}

func TestRepository_Delete_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteLdap)).
		WithArgs(testLdapID, "other-org").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testLdapID, "other-org")
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

var _ domain.Repository = Repository{}
