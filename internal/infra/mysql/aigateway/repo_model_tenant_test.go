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

var modelTenantUpdatedAt = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func newTenantModelMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestModelsRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectQuery(regexp.QuoteMeta(queryModelGetByIDForOrganization)).
		WithArgs("m-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "organization_id", "provider_id", "provider_model_name", "display_name",
			"capabilities", "max_context_tokens", "max_output_tokens", "status",
			"created_at", "updated_at", "deleted_at", "revision",
		}).AddRow(
			"m-1", "org-1", "p-1", "gpt-4o", "gpt-4o", "", nil, nil, "active",
			modelTenantUpdatedAt, modelTenantUpdatedAt, nil, 1,
		))

	repo := NewModelsRepository(db)
	m, err := repo.GetByIDForOrganization(context.Background(), "org-1", "m-1")
	require.NoError(t, err)
	require.Equal(t, "m-1", m.ID)
	require.Equal(t, "org-1", m.OrganizationID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelsRepository_GetByIDForOrganization_NoRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectQuery(regexp.QuoteMeta(queryModelGetByIDForOrganization)).
		WithArgs("m-foreign", "org-1").
		WillReturnError(sql.ErrNoRows)

	repo := NewModelsRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "m-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelsRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryModelUpdateForOrganization)).
		WithArgs(
			"gpt-4o", "gpt", "null", nil, nil, "active",
			modelTenantUpdatedAt, nil, "m-1", 2, "org-1",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewModelsRepository(db)
	m := &domain.LlmModel{
		ID: "m-1", OrganizationID: "org-1", ProviderID: "p-1",
		ProviderModelName: "gpt-4o", DisplayName: "gpt", Status: "active",
		Revision: 2, UpdatedAt: modelTenantUpdatedAt,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), m, "org-1"))
	require.Equal(t, int64(3), m.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelsRepository_UpdateForOrganization_ZeroRowsIsConflict(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryModelUpdateForOrganization)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewModelsRepository(db)
	err := repo.UpdateForOrganization(context.Background(),
		&domain.LlmModel{ID: "m-foreign", Revision: 1}, "org-1")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelsRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryModelDeleteForOrganization)).
		WithArgs("m-foreign", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewModelsRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "m-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAliasesRepository_GetByIDForOrganization_NoRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectQuery(regexp.QuoteMeta(queryAliasGetByIDForOrganization)).
		WithArgs("a-foreign", "org-1").
		WillReturnError(sql.ErrNoRows)

	repo := NewAliasesRepository(db)
	_, err := repo.GetByIDForOrganization(context.Background(), "org-1", "a-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAliasesRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryAliasUpdateForOrganization)).
		WithArgs("fast", "fast", "active", modelTenantUpdatedAt, nil, "a-1", 2, "org-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewAliasesRepository(db)
	a := &domain.LlmModelAlias{
		ID: "a-1", OrganizationID: "org-1", Alias: "fast",
		DisplayName: "fast", Status: "active", Revision: 2, UpdatedAt: modelTenantUpdatedAt,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), a, "org-1"))
	require.Equal(t, int64(3), a.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAliasesRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newTenantModelMock(t)

	mock.ExpectExec(regexp.QuoteMeta(queryAliasDeleteForOrganization)).
		WithArgs("a-foreign", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewAliasesRepository(db)
	err := repo.DeleteForOrganization(context.Background(), "org-1", "a-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
