package aigateway

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type pgQueryCapture struct {
	sql  string
	args []any
}

func capturedQueryRow(values []any, err error) (*shared.MockDBTX, *pgQueryCapture) {
	c := &pgQueryCapture{}
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
		c.sql = sql
		c.args = append([]any(nil), args...)
		return shared.NewMockRow(values, err)
	}}
	return db, c
}

func capturedExec(tag pgconn.CommandTag) (*shared.MockDBTX, *pgQueryCapture) {
	c := &pgQueryCapture{}
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		c.sql = sql
		c.args = append([]any(nil), args...)
		return tag, nil
	}}
	return db, c
}

func TestModelsRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow(nil, pgx.ErrNoRows)

	repo := NewModelsRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "m-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"m-foreign", "org-1"}, call.args)
}

func TestModelsRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow([]any{int64(3)}, nil)

	repo := NewModelsRepository(db)
	m := &domain.LlmModel{
		ID: "m-1", OrganizationID: "org-1", ProviderID: "p-1",
		ProviderModelName: "gpt-4o", DisplayName: "gpt", Status: "active", Revision: 2,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), m, "org-1"))
	require.Equal(t, int64(3), m.Revision)

	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($11)")
	require.Len(t, call.args, 11)
	require.Equal(t, int64(1), call.args[8], "revision guard stays at $9")
	require.Equal(t, "m-1", call.args[9])
	require.Equal(t, "org-1", call.args[10], "trusted tenant is the last predicate")
}

func TestModelsRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, call := capturedExec(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewModelsRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "m-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"m-foreign", "org-1"}, call.args)
}

func TestAliasesRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow(nil, pgx.ErrNoRows)

	repo := NewAliasesRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "a-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"a-foreign", "org-1"}, call.args)
}

func TestAliasesRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, call := capturedQueryRow([]any{int64(3)}, nil)

	repo := NewAliasesRepository(db)
	a := &domain.LlmModelAlias{
		ID: "a-1", OrganizationID: "org-1", Alias: "fast",
		DisplayName: "fast", Status: "active", Revision: 2,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), a, "org-1"))
	require.Equal(t, int64(3), a.Revision)

	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($8)")
	require.Len(t, call.args, 8)
	require.Equal(t, int64(1), call.args[5], "revision guard stays at $6")
	require.Equal(t, "a-1", call.args[6])
	require.Equal(t, "org-1", call.args[7], "trusted tenant is the last predicate")
}

func TestAliasesRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, call := capturedExec(pgconn.NewCommandTag("UPDATE 0"))

	repo := NewAliasesRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "a-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Contains(t, call.sql, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"a-foreign", "org-1"}, call.args)
}
