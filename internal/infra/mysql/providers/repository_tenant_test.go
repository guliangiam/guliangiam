package providers

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func mysqlProviderTenantContext(orgID string) context.Context {
	return tenantctx.WithTenantScope(context.Background(), orgID, "", tenantctx.TenantFilterStrict)
}

func requireMySQLProviderNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestRepository_GetByID_TenantScopedMySQL is the MySQL counterpart of the
// PostgreSQL test: this dialect has no RLS, so the organization predicate is
// the only storage-level isolation for by-ID reads.
func TestRepository_GetByID_TenantScopedMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectQuery(regexp.QuoteMeta("FROM `providers` WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("prov-1", "org-1", "org-1").
		WillReturnRows(sqlmock.NewRows(providerRowCols(45)))

	_, err := repo.GetByID(mysqlProviderTenantContext("org-1"), "prov-1")
	requireMySQLProviderNotFound(t, err)
}

// TestRepository_GetByIDForOrganization_TenantPredicateMySQL pins the explicit
// organization_id predicate on the management by-ID read; MySQL has no RLS.
func TestRepository_GetByIDForOrganization_TenantPredicateMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetProviderByIDForOrganization)).
		WithArgs("prov-1", "org-1").
		WillReturnRows(sqlmock.NewRows(providerRowCols(45)))

	_, err := repo.GetByIDForOrganization(context.Background(), "prov-1", "org-1")
	requireMySQLProviderNotFound(t, err)
}

func TestRepository_Update_TenantScopedMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	args := providerWriteArgs(48)
	args[44] = "prov-1"
	args[45] = int64(1)
	args[46] = "org-1"
	args[47] = "org-1"
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateProvider)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT revision FROM `providers` WHERE id = ?")).
		WithArgs("prov-1").
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(3)))

	err := repo.Update(mysqlProviderTenantContext("org-1"), &domain.Provider{
		ID: "prov-1", Name: "renamed", Revision: 2,
		Family: domain.FamilyPayment, Kind: domain.Kind("alipay"), Variant: domain.VariantSandbox,
	})
	require.NoError(t, err)
}

func TestRepository_Delete_TenantScopedMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE `providers` SET deleted_at = NOW(), updated_at = NOW() WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("prov-1", "org-1", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Delete(mysqlProviderTenantContext("org-1"), "prov-1"))
}

func TestRepository_Delete_ZeroRowsIsNotFoundMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE `providers` SET deleted_at = NOW(), updated_at = NOW() WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("prov-1", "org-1", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(mysqlProviderTenantContext("org-1"), "prov-1")
	requireMySQLProviderNotFound(t, err)
}

func TestRepository_GetConfig_TenantScopedMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectQuery(regexp.QuoteMeta("FROM `provider_configs` pc WHERE pc.provider_id = ?")).
		WithArgs("prov-1", "org-1", "org-1").
		WillReturnRows(sqlmock.NewRows(providerRowCols(4)))

	_, err := repo.GetConfig(mysqlProviderTenantContext("org-1"), "prov-1")
	require.NoError(t, err)
}

func TestRepository_ListSecretStatuses_TenantScopedMySQL(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectQuery(regexp.QuoteMeta("FROM `provider_secrets` ps WHERE ps.provider_id = ?")).
		WithArgs("prov-1", "org-1", "org-1").
		WillReturnRows(sqlmock.NewRows(providerRowCols(8)))

	_, err := repo.ListSecretStatuses(mysqlProviderTenantContext("org-1"), "prov-1")
	require.NoError(t, err)
}
