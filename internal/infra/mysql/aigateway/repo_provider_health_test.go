package aigateway

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestProviderHealthRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryProviderHealthGetByIDForOrganization)).
		WithArgs("h-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "provider_id", "state", "consecutive_failures",
			"open_until", "last_checked_at", "last_latency_ms", "updated_at",
		}).AddRow("h-1", "p-1", "healthy", 0, nil, nil, nil, time.Now()))

	repo := NewProviderHealthRepository(db)
	h, err := repo.GetByIDForOrganization(context.Background(), "org-1", "h-1")
	require.NoError(t, err)
	require.Equal(t, "h-1", h.ID)
	require.Equal(t, "p-1", h.ProviderID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProviderHealthRepository_GetByIDForOrganization_NoRowsIsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryProviderHealthGetByIDForOrganization)).
		WithArgs("h-foreign", "org-1").
		WillReturnError(sql.ErrNoRows)

	repo := NewProviderHealthRepository(db)
	_, err = repo.GetByIDForOrganization(context.Background(), "org-1", "h-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestProviderHealthRepository_ListByOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryProviderHealthListByOrganization)).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "provider_id", "state", "consecutive_failures",
			"open_until", "last_checked_at", "last_latency_ms", "updated_at",
		}).AddRow("h-1", "p-1", "open", 5, time.Now(), nil, 120, time.Now()))

	repo := NewProviderHealthRepository(db)
	out, err := repo.ListByOrganization(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "h-1", out[0].ID)
	require.Equal(t, "p-1", out[0].ProviderID)
	require.Equal(t, "open", out[0].State)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProviderHealthQueries_ScopeThroughProvidersTable(t *testing.T) {
	for name, query := range map[string]string{
		"get":  queryProviderHealthGetByIDForOrganization,
		"list": queryProviderHealthListByOrganization,
	} {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, query, "EXISTS", "ownership must be resolved through llm_providers")
			require.Contains(t, query, "`llm_providers`")
			require.Contains(t, query, "organization_id = ?")
		})
	}
}

func TestProvidersRepository_GetByIDForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryProviderGetByIDForOrganization)).
		WithArgs("p-foreign", "org-1").
		WillReturnError(sql.ErrNoRows)

	repo := NewProvidersRepository(db)
	_, err = repo.GetByIDForOrganization(context.Background(), "org-1", "p-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProvidersRepository_UpdateForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	updatedAt := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(queryProviderUpdateForOrganization)).
		WithArgs(
			"provider", "provider", "openai", "null", "openai",
			"https://api.example.com", "sealed", "fp", "active",
			5000, 1, 10, 100, nil, nil, updatedAt, nil,
			"p-1", 2, "org-1",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewProvidersRepository(db)
	p := &domain.LlmProvider{
		ID: "p-1", OrganizationID: "org-1", Name: "provider", DisplayName: "provider",
		ProviderType: "openai", PreferredProtocol: "openai", BaseURL: "https://api.example.com",
		APIKeySealed: "sealed", APIKeyFingerprint: "fp", Status: "active",
		RequestTimeoutMs: 5000, MaxRetries: 1, Priority: 10, Weight: 100, Revision: 2,
		UpdatedAt: updatedAt,
	}
	require.NoError(t, repo.UpdateForOrganization(context.Background(), p, "org-1"))
	require.Equal(t, int64(3), p.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProvidersRepository_UpdateForOrganization_ZeroRowsIsConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryProviderUpdateForOrganization)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewProvidersRepository(db)
	err = repo.UpdateForOrganization(context.Background(), &domain.LlmProvider{ID: "p-foreign", Revision: 1}, "org-1")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProvidersRepository_DeleteForOrganization_BindsTenantPredicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryProviderDeleteForOrganization)).
		WithArgs("p-1", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewProvidersRepository(db)
	require.NoError(t, repo.DeleteForOrganization(context.Background(), "org-1", "p-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProvidersRepository_DeleteForOrganization_ZeroRowsIsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryProviderDeleteForOrganization)).
		WithArgs("p-foreign", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewProvidersRepository(db)
	err = repo.DeleteForOrganization(context.Background(), "org-1", "p-foreign")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
