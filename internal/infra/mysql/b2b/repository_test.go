package b2b

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/b2b"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fixedNow  = time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	testID    = "00000000-0000-0000-0000-0000000000a1"
	testOrgID = "00000000-0000-0000-0000-0000000000b2"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func domainColumns() []string {
	return []string{
		"id", "organization_id", "domain", "verification_token",
		"status", "verified_at", "created_at", "updated_at",
		"attempts", "last_error", "next_retry_at", "last_checked_at",
	}
}

func domainRow(status string, verifiedAt driver.Value) []driver.Value {
	return []driver.Value{
		testID, testOrgID, "corp.example.com", "token-1",
		status, verifiedAt, fixedNow, fixedNow,
		0, nil, fixedNow, nil,
	}
}

func newDomain(status string) *domain.OrganizationDomain {
	return &domain.OrganizationDomain{
		ID:                testID,
		OrganizationID:    testOrgID,
		Domain:            "corp.example.com",
		VerificationToken: "token-1",
		Status:            status,
		CreatedAt:         fixedNow,
		UpdatedAt:         fixedNow,
	}
}

// ── GetByID ─────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrganizationDomainByID)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows(domainColumns()).AddRow(domainRow("pending", nil)...))

	d, err := NewRepository(db).GetByID(context.Background(), testID)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, testID, d.ID)
	assert.Equal(t, "pending", d.Status)
	assert.True(t, d.VerifiedAt.IsZero())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_VerifiedAtScanned(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrganizationDomainByID)).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows(domainColumns()).AddRow(domainRow("verified", fixedNow)...))

	d, err := NewRepository(db).GetByID(context.Background(), testID)
	require.NoError(t, err)
	assert.Equal(t, "verified", d.Status)
	assert.Equal(t, fixedNow, d.VerifiedAt)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrganizationDomainByID)).
		WithArgs(testID).
		WillReturnError(sql.ErrNoRows)

	_, err := NewRepository(db).GetByID(context.Background(), testID)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRepository_GetByID_DriverError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	boom := errors.New("driver exploded")
	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrganizationDomainByID)).
		WithArgs(testID).
		WillReturnError(boom)

	_, err := NewRepository(db).GetByID(context.Background(), testID)
	assert.ErrorIs(t, err, boom)
}

func TestRepository_GetByIDForOrganization_Scoped(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrganizationDomainByIDForOrganization)).
		WithArgs(testID, testOrgID).
		WillReturnRows(sqlmock.NewRows(domainColumns()).AddRow(domainRow("pending", nil)...))

	d, err := NewRepository(db).GetByIDForOrganization(context.Background(), testID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, testID, d.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByDomain ────────────────────────────────────────────────

func TestRepository_GetByDomain_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrganizationDomainByName)).
		WithArgs(testOrgID, "corp.example.com").
		WillReturnRows(sqlmock.NewRows(domainColumns()).AddRow(domainRow("pending", nil)...))

	d, err := NewRepository(db).GetByDomain(context.Background(), testOrgID, "corp.example.com")
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "corp.example.com", d.Domain)
}

func TestRepository_GetByDomain_NotClaimedReturnsNil(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetOrganizationDomainByName)).
		WithArgs(testOrgID, "free.example.com").
		WillReturnError(sql.ErrNoRows)

	d, err := NewRepository(db).GetByDomain(context.Background(), testOrgID, "free.example.com")
	require.NoError(t, err)
	assert.Nil(t, d)
}

// ── FindVerifiedByDomain ───────────────────────────────────────

func TestRepository_FindVerifiedByDomain_Single(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryFindVerifiedOrganizationDomain)).
		WithArgs("corp.example.com").
		WillReturnRows(sqlmock.NewRows(domainColumns()).AddRow(domainRow("verified", fixedNow)...))

	d, err := NewRepository(db).FindVerifiedByDomain(context.Background(), "corp.example.com")
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "verified", d.Status)
}

func TestRepository_FindVerifiedByDomain_NoneReturnsNil(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryFindVerifiedOrganizationDomain)).
		WithArgs("unknown.example.com").
		WillReturnRows(sqlmock.NewRows(domainColumns()))

	d, err := NewRepository(db).FindVerifiedByDomain(context.Background(), "unknown.example.com")
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestRepository_FindVerifiedByDomain_MultipleIsConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryFindVerifiedOrganizationDomain)).
		WithArgs("corp.example.com").
		WillReturnRows(sqlmock.NewRows(domainColumns()).
			AddRow(domainRow("verified", fixedNow)...).
			AddRow(domainRow("verified", fixedNow)...))

	_, err := NewRepository(db).FindVerifiedByDomain(context.Background(), "corp.example.com")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
}

// ── List ───────────────────────────────────────────────────────

func TestRepository_List_WithStatusFilter(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryCountOrganizationDomains)).
		WithArgs(testOrgID, "pending", "pending").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(queryListOrganizationDomains)).
		WithArgs(testOrgID, "pending", "pending", 20, 0).
		WillReturnRows(sqlmock.NewRows(domainColumns()).AddRow(domainRow("pending", nil)...))

	items, total, err := NewRepository(db).List(context.Background(), domain.ListQuery{
		Page:           1,
		PageSize:       20,
		OrganizationID: testOrgID,
		Status:         "pending",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "pending", items[0].Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_NoStatusFilterPagination(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryCountOrganizationDomains)).
		WithArgs(testOrgID, "", "").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(queryListOrganizationDomains)).
		WithArgs(testOrgID, "", "", 20, 20).
		WillReturnRows(sqlmock.NewRows(domainColumns()))

	items, total, err := NewRepository(db).List(context.Background(), domain.ListQuery{
		Page:           2,
		PageSize:       20,
		OrganizationID: testOrgID,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, items)
	assert.NotNil(t, items)
}

func TestRepository_List_CountErrorAborts(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	boom := errors.New("count failed")
	mock.ExpectQuery(regexp.QuoteMeta(queryCountOrganizationDomains)).
		WithArgs(testOrgID, "", "").
		WillReturnError(boom)

	_, _, err := NewRepository(db).List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
		PageSize:       20,
	})
	assert.ErrorIs(t, err, boom)
}

// ── ClaimPending ───────────────────────────────────────────────

func TestRepository_ClaimPending(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryClaimPendingIDs)).
		WithArgs(50).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testID))

	inClause := "(?)"
	mock.ExpectExec(regexp.QuoteMeta(strings.Replace(queryClaimPendingLease, "__PICKED_IDS__", inClause, 1))).
		WithArgs(300, testID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(strings.Replace(queryClaimPendingSelect, "__PICKED_IDS__", inClause, 1))).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows(domainColumns()).AddRow(domainRow("pending", nil)...))

	items, err := NewRepository(db).ClaimPending(context.Background(), 50, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, testID, items[0].ID)
	assert.Equal(t, fixedNow, items[0].NextRetryAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ClaimPending_EmptyBatch(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryClaimPendingIDs)).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	items, err := NewRepository(db).ClaimPending(context.Background(), 10, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ClaimPending_LimitsToSingleDigit(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryClaimPendingIDs)).
		WithArgs(1).
		WillReturnError(errors.New("pick failed"))

	_, err := NewRepository(db).ClaimPending(context.Background(), 1, time.Minute)
	assert.Error(t, err)
}

// ── Create / Update / Delete ───────────────────────────────────

func TestRepository_Create_ZeroVerifiedAtPassesNil(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertOrganizationDomain)).
		WithArgs(testID, testOrgID, "corp.example.com", "token-1", "pending", nil, fixedNow, fixedNow).
		WillReturnResult(sqlmock.NewResult(1, 1))

	assert.NoError(t, NewRepository(db).Create(context.Background(), newDomain("pending")))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Create_DuplicateSurfacesDriverError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	dup := errors.New("Error 1062: Duplicate entry for key 'uq_org_domain'")
	mock.ExpectExec(regexp.QuoteMeta(queryInsertOrganizationDomain)).
		WithArgs(testID, testOrgID, "corp.example.com", "token-1", "pending", nil, fixedNow, fixedNow).
		WillReturnError(dup)

	err := NewRepository(db).Create(context.Background(), newDomain("pending"))
	assert.ErrorIs(t, err, dup)
}

func TestRepository_Update_PendingAffected(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	d := newDomain("verified")
	d.VerifiedAt = fixedNow
	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePendingOrganizationDomain)).
		WithArgs("corp.example.com", "token-1", "verified", fixedNow, fixedNow, 0, "", nil, nil, testID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, NewRepository(db).Update(context.Background(), d))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_WritesRetryBudgetFields(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	d := newDomain("pending")
	d.Attempts = 1
	d.LastError = "token_mismatch: ..."
	d.NextRetryAt = fixedNow.Add(time.Minute)
	d.LastCheckedAt = fixedNow
	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePendingOrganizationDomain)).
		WithArgs("corp.example.com", "token-1", "pending", nil, fixedNow, 1,
			"token_mismatch: ...", fixedNow.Add(time.Minute), fixedNow, testID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, NewRepository(db).Update(context.Background(), d))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_NoRowsIsConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdatePendingOrganizationDomain)).
		WithArgs("corp.example.com", "token-1", "pending", nil, fixedNow, 0, "", nil, nil, testID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := NewRepository(db).Update(context.Background(), newDomain("pending"))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
}

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteOrganizationDomain)).
		WithArgs(testID, testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, NewRepository(db).Delete(context.Background(), testID, testOrgID))
}

func TestRepository_Delete_ForeignTenantIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteOrganizationDomain)).
		WithArgs(testID, testOrgID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := NewRepository(db).Delete(context.Background(), testID, testOrgID)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}
