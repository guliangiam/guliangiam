package workload

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_ListTrustsWithoutResolverFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(qCountTrusts)).
		WithArgs("", "", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(qListTrusts)).
		WithArgs("", "", "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(trustCols()).AddRow(newTrustRow()...))
	items, total, err := NewRepository(db).ListTrusts(context.Background(), domain.ListTrustsQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindActiveTrustsByIssuer(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(qFindActiveTrustsByIssuer)).
		WithArgs("https://corp.example.com").
		WillReturnRows(sqlmock.NewRows(trustCols()).AddRow(newTrustRow()...))
	items, err := NewRepository(db).FindActiveTrustsByIssuer(context.Background(), "https://corp.example.com")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, []string{"repo:org/*"}, items[0].Subjects)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetAccountByClientID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(qGetAccountByClientID)).
		WithArgs("ci-bot-client-id").
		WillReturnRows(sqlmock.NewRows(accountCols()).AddRow(newAccountRow()...))
	account, err := NewRepository(db).GetAccountByClientID(context.Background(), "ci-bot-client-id")
	require.NoError(t, err)
	assert.Equal(t, "sa-001", account.ID)
	assert.Equal(t, "hash-of-secret", account.ClientSecretHash)
	assert.Equal(t, []string{"orders:read"}, account.Scopes)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetAccountByName(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(qGetAccountByName)).
		WithArgs(testOrgID, "ci-bot").
		WillReturnRows(sqlmock.NewRows(accountCols()).AddRow(newAccountRow()...))
	account, err := NewRepository(db).GetAccountByName(context.Background(), testOrgID, "ci-bot")
	require.NoError(t, err)
	assert.Equal(t, "sa-001", account.ID)
	assert.Equal(t, testOrgID, account.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_SetAccountStatus(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(qUpdateAccountStatus)).
		WithArgs("disabled", "sa-001", testOrgID, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := NewRepository(db).SetAccountStatus(context.Background(), "sa-001", testOrgID, domain.SAStatusDisabled, 3)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_SetAccountStatusRevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(qUpdateAccountStatus)).
		WithArgs("active", "sa-001", testOrgID, int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := NewRepository(db).SetAccountStatus(context.Background(), "sa-001", testOrgID, domain.SAStatusActive, 3)
	assert.ErrorContains(t, err, "revision conflict")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_RotateAccountSecret(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	rotatedAt := fixedNow.Add(time.Hour)
	mock.ExpectExec(regexp.QuoteMeta(qRotateAccountSecret)).
		WithArgs("new-hash", rotatedAt, "sa-001", testOrgID, int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := NewRepository(db).RotateAccountSecret(context.Background(), "sa-001", testOrgID, "new-hash", rotatedAt, 4)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_DeleteAccountRequiresDisabledRevision(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(qDeleteAccount)).
		WithArgs("sa-001", testOrgID, int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := NewRepository(db).DeleteAccount(context.Background(), "sa-001", testOrgID, 5)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_TrustStatusAndDelete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(qUpdateTrustStatus)).
		WithArgs("disabled", "trust-001", testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(qDeleteTrust)).
		WithArgs("trust-001", testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	repository := NewRepository(db)
	require.NoError(t, repository.SetTrustStatus(context.Background(), "trust-001", testOrgID, domain.TrustStatusDisabled))
	require.NoError(t, repository.DeleteTrust(context.Background(), "trust-001", testOrgID))
	assert.NoError(t, mock.ExpectationsWereMet())
}
