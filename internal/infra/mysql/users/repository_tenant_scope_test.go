package users

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetByIDForOrganization_ScopesQuery(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("WHERE u.id = ? AND u.organization_id = ?")).
		WithArgs("user-id", "org-id").
		WillReturnError(sql.ErrNoRows)

	r := NewRepository(db)
	_, err := r.GetByIDForOrganization(context.Background(), "user-id", "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateForOrganization_ForeignRowIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// SET 17 + id + revision + organization_id = 20 args.
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("AND organization_id = ? AND deleted_at IS NULL")).
		WithArgs(anyArgs(20)...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// The scoped probe uses the same organization predicate and finds nothing.
	mock.ExpectQuery(regexp.QuoteMeta("WHERE id = ? AND organization_id = ? AND deleted_at IS NULL")).
		WithArgs("user-id", "org-id").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	r := NewRepository(db)
	err := r.UpdateForOrganization(context.Background(), &domain.User{ID: "user-id", OrganizationID: "org-id", Revision: 1}, "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_UpdateForOrganization_StaleRevisionIsConflict pins the
// distinction the probe exists for: a same-tenant row that the UPDATE missed
// because `revision = ?` no longer matches must be a 409, while a foreign row
// stays a 404 (previous test). Nothing after the UPDATE may be written.
func TestRepository_UpdateForOrganization_StaleRevisionIsConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("revision = revision + 1")).
		WithArgs(anyArgs(20)...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE id = ? AND organization_id = ? AND deleted_at IS NULL")).
		WithArgs("user-id", "org-id").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectRollback()

	r := NewRepository(db)
	err := r.UpdateForOrganization(context.Background(), &domain.User{ID: "user-id", OrganizationID: "org-id", Revision: 3}, "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_UpdateForOrganization_ScopesRevisionGuard asserts the real SQL
// shape instead of only the error mapping: the optimistic lock compares the
// caller's Revision-1 (the value loaded from the row), and the tenant predicate
// plus deleted_at guard sit in the same WHERE clause.
func TestRepository_UpdateForOrganization_ScopesRevisionGuard(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	caller := &domain.User{ID: "user-id", OrganizationID: "org-id", Revision: 5}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("revision = revision + 1")).
		WithArgs(append(anyArgs(17), "user-id", int64(4), "org-id")...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, table := range []string{"user_signin_tracking", "user_mfa_config", "user_financials", "user_collections", "user_custom_attributes"} {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `" + table + "`")).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	r := NewRepository(db)
	require.NoError(t, r.UpdateForOrganization(context.Background(), caller, "org-id"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ForeignRowIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("organization_id = ?")).
		WithArgs("user-id", "org-id").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), "user-id", "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
