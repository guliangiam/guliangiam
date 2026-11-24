package aigateway

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

var quotaPriceTenantAt = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func TestQuotaPoliciesRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectQuery(regexp.QuoteMeta(queryQuotaPolicyGetByIDForOrganization)).
		WithArgs("q-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "organization_id", "scope", "scope_ref", "limit_type", "limit_value",
			"reset_cycle", "status", "created_at", "updated_at", "deleted_at", "revision",
		}).AddRow(
			"q-1", "org-1", "organization", nil, "rpm", 60, "daily", "active",
			quotaPriceTenantAt, quotaPriceTenantAt, nil, 1,
		))

	repo := NewQuotaPoliciesRepository(db)
	q, err := repo.GetByIDForOrganization(context.Background(), "org-1", "q-1")
	require.NoError(t, err)
	require.Equal(t, "q-1", q.ID)
	require.Equal(t, "org-1", q.OrganizationID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuotaPoliciesRepository_GetByIDForOrganization_NoRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectQuery(regexp.QuoteMeta(queryQuotaPolicyGetByIDForOrganization)).
		WithArgs("q-foreign", "org-1").
		WillReturnError(sql.ErrNoRows)

	repo := NewQuotaPoliciesRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "q-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuotaPoliciesRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryQuotaPolicyUpdateForOrganization)).
		WithArgs(
			"organization", nil, "rpm", float64(60), "daily", "active",
			quotaPriceTenantAt, nil, "q-1", 2, "org-1",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewQuotaPoliciesRepository(db)
	q := &domain.LlmQuotaPolicy{
		ID: "q-1", OrganizationID: "org-1", Scope: "organization",
		LimitType: "rpm", LimitValue: 60, ResetCycle: "daily", Status: "active",
		Revision: 2, UpdatedAt: quotaPriceTenantAt,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), q, "org-1"))
	require.Equal(t, int64(3), q.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuotaPoliciesRepository_UpdateForOrganization_ZeroRowsIsConflict(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryQuotaPolicyUpdateForOrganization)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewQuotaPoliciesRepository(db)
	err := repo.UpdateForOrganization(context.Background(),
		&domain.LlmQuotaPolicy{ID: "q-foreign", Revision: 1}, "org-1")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuotaPoliciesRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryQuotaPolicyDeleteForOrganization)).
		WithArgs("q-foreign", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewQuotaPoliciesRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "q-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPriceVersionsRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectQuery(regexp.QuoteMeta(queryPriceVersionGetByIDForOrganization)).
		WithArgs("pv-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "organization_id", "model_id", "version_no",
			"input_price_per_1m", "output_price_per_1m", "currency", "effective_from",
			"created_at", "updated_at", "deleted_at", "revision",
		}).AddRow(
			"pv-1", "org-1", "m-1", 1, 5, 15, "USD",
			quotaPriceTenantAt, quotaPriceTenantAt, quotaPriceTenantAt, nil, 0,
		))

	repo := NewPriceVersionsRepository(db)
	v, err := repo.GetByIDForOrganization(context.Background(), "org-1", "pv-1")
	require.NoError(t, err)
	require.Equal(t, "pv-1", v.ID)
	require.Equal(t, "org-1", v.OrganizationID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPriceVersionsRepository_GetByIDForOrganization_NoRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectQuery(regexp.QuoteMeta(queryPriceVersionGetByIDForOrganization)).
		WithArgs("pv-foreign", "org-1").
		WillReturnError(sql.ErrNoRows)

	repo := NewPriceVersionsRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "pv-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPriceVersionsRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryPriceVersionUpdateForOrganization)).
		WithArgs(1, float64(5), float64(15), "USD", quotaPriceTenantAt, quotaPriceTenantAt, "pv-1", 0, "org-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewPriceVersionsRepository(db)
	v := &domain.LlmPriceVersion{
		ID: "pv-1", OrganizationID: "org-1", ModelID: "m-1", VersionNo: 1,
		InputPricePer1M: 5, OutputPricePer1M: 15, Currency: "USD",
		EffectiveFrom: quotaPriceTenantAt, UpdatedAt: quotaPriceTenantAt, Revision: 0,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), v, "org-1"))
	require.Equal(t, int64(1), v.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPriceVersionsRepository_UpdateForOrganization_ZeroRowsIsConflict(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryPriceVersionUpdateForOrganization)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewPriceVersionsRepository(db)
	err := repo.UpdateForOrganization(context.Background(),
		&domain.LlmPriceVersion{ID: "pv-foreign"}, "org-1")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPriceVersionsRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryPriceVersionDeleteForOrganization)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "pv-foreign", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewPriceVersionsRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "pv-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
