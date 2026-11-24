package aigateway

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestQuotaPoliciesRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow(nil, pgx.ErrNoRows)

	repo := NewQuotaPoliciesRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "q-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"q-foreign", "org-1"}, call.args)
}

func TestQuotaPoliciesRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow([]any{int64(3)}, nil)

	repo := NewQuotaPoliciesRepository(db)
	q := &domain.LlmQuotaPolicy{
		ID: "q-1", OrganizationID: "org-1", Scope: "organization",
		LimitType: "rpm", LimitValue: 60, ResetCycle: "daily", Status: "active", Revision: 2,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), q, "org-1"))
	require.Equal(t, int64(3), q.Revision)

	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($11)")
	require.Len(t, call.args, 11)
	require.Equal(t, int64(2), call.args[8], "revision guard stays at $9")
	require.Equal(t, "q-1", call.args[9])
	require.Equal(t, "org-1", call.args[10], "trusted tenant is the last predicate")
}

func TestQuotaPoliciesRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, call := capturedExec(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewQuotaPoliciesRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "q-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"q-foreign", "org-1"}, call.args)
}

func TestPriceVersionsRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow(nil, pgx.ErrNoRows)

	repo := NewPriceVersionsRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "pv-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"pv-foreign", "org-1"}, call.args)
}

func TestPriceVersionsRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow([]any{int64(1)}, nil)

	repo := NewPriceVersionsRepository(db)
	v := &domain.LlmPriceVersion{
		ID: "pv-1", OrganizationID: "org-1", ModelID: "m-1", VersionNo: 1,
		InputPricePer1M: 5, OutputPricePer1M: 15, Currency: "USD", Revision: 0,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), v, "org-1"))
	require.Equal(t, int64(1), v.Revision)

	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($9)")
	require.Len(t, call.args, 9)
	require.Equal(t, int64(0), call.args[6], "revision guard stays at $7")
	require.Equal(t, "pv-1", call.args[7])
	require.Equal(t, "org-1", call.args[8], "trusted tenant is the last predicate")
}

func TestPriceVersionsRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, call := capturedExec(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewPriceVersionsRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "pv-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($4)")
	require.Len(t, call.args, 4)
	require.Equal(t, "pv-foreign", call.args[2])
	require.Equal(t, "org-1", call.args[3])
}
