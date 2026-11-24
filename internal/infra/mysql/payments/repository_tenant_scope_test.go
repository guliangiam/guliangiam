package payments

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_ScopedQueriesCarryTenantPredicate(t *testing.T) {
	assert.Contains(t, refundGetByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, refundListByPaymentForOrganization, "WHERE payment_id = ? AND organization_id = ?")
	assert.Contains(t, attemptGetByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, attemptListByPaymentForOrganization, "WHERE payment_id = ? AND organization_id = ?")
	assert.Contains(t, queryChannelConfigGetByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, queryChannelConfigUpdate, "WHERE id=? AND revision=? AND organization_id=?")
	assert.Contains(t, reconBatchGetByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, reconItemListForOrganization, "b.organization_id = ?")
	assert.Contains(t, reconItemResolveForOrganization, "b.organization_id = ?")
}

func TestChannelConfigsRepository_GetByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	columns := []string{
		"id", "organization_id", "provider_id", "channel", "environment",
		"config_sealed", "config_fingerprint", "config_set", "version_no", "status",
		"test_passed_at", "activated_at", "disabled_at", "created_at", "updated_at", "deleted_at", "revision",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryChannelConfigGetByIDForOrganization)).
		WithArgs(testPayID, testOrgID6).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			testPayID, testOrgID6, nil, "alipay", "production",
			"sealed", "fingerprint", nil, 1, "draft",
			nil, nil, nil, fixedNow6, fixedNow6, nil, int64(1),
		))

	r := NewChannelConfigsRepository(db)
	cfg, err := r.GetByIDForOrganization(context.Background(), testPayID, testOrgID6)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, testOrgID6, cfg.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReconRepository_GetBatchForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	columns := []string{
		"id", "organization_id", "channel", "batch_date", "status",
		"diff_count", "total_local", "total_channel", "created_at", "updated_at", "deleted_at", "revision",
	}
	mock.ExpectQuery(regexp.QuoteMeta(reconBatchGetByIDForOrganization)).
		WithArgs("batch-1", testOrgID6).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			"batch-1", testOrgID6, "alipay", fixedNow6, "pending",
			0, 0, 0, fixedNow6, fixedNow6, nil, int64(1),
		))

	r := NewReconRepository(db)
	b, err := r.GetBatchForOrganization(context.Background(), "batch-1", testOrgID6)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, testOrgID6, b.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReconRepository_ResolveItemForOrganization_ForeignTenantNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(reconItemResolveForOrganization)).
		WithArgs("ops", sqlmock.AnyArg(), sql.NullString{Valid: false}, "item-1", "foreign-org").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewReconRepository(db)
	err := r.ResolveItemForOrganization(context.Background(), "item-1", "foreign-org", "ops")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
