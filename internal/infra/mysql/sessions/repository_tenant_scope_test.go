package sessions

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRepository_GetByIDForOrganization_Query(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("AND u.organization_id = ?")).
		WithArgs("sess-1", "org-1").
		WillReturnRows(sqlmock.NewRows(sessionCols).AddRow(newTestSessionRow("sess-1", "user-1", "alice", "my-app")...))

	r := NewRepository(db)
	session, err := r.GetByIDForOrganization(context.Background(), "sess-1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, "sess-1", session.ID)
	assert.Equal(t, "org-1", session.OrganizationID)
}

func TestRepository_GetByIDForOrganization_ForeignRowNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("AND u.organization_id = ?")).
		WithArgs("sess-1", "org-2").
		WillReturnRows(sqlmock.NewRows(sessionCols))

	r := NewRepository(db)
	_, err := r.GetByIDForOrganization(context.Background(), "sess-1", "org-2")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRepository_DeleteForOrganization_Query(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("AND user_id IN (SELECT id FROM `users` WHERE organization_id = ?)")).
		WithArgs("sess-1", "org-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	require.NoError(t, r.DeleteForOrganization(context.Background(), "sess-1", "org-1"))
}

func TestRepository_DeleteForOrganization_ZeroRowsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("AND user_id IN (SELECT id FROM `users` WHERE organization_id = ?)")).
		WithArgs("sess-1", "org-2").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.DeleteForOrganization(context.Background(), "sess-1", "org-2")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRepository_RevokeForOrganization_Query(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := time.Unix(0, 0).UTC()
	mock.ExpectExec(regexp.QuoteMeta("AND user_id IN (SELECT id FROM `users` WHERE organization_id = ?)")).
		WithArgs(revokedAt, revokedAt, "sess-1", "org-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	require.NoError(t, r.RevokeForOrganization(context.Background(), "sess-1", "org-1", revokedAt))
}

func TestRepository_RevokeForOrganization_ForeignRowNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	revokedAt := time.Unix(0, 0).UTC()
	mock.ExpectExec(regexp.QuoteMeta("AND user_id IN (SELECT id FROM `users` WHERE organization_id = ?)")).
		WithArgs(revokedAt, revokedAt, "sess-1", "org-2").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("AND u.organization_id = ?")).
		WithArgs("sess-1", "org-2").
		WillReturnRows(sqlmock.NewRows(sessionCols))

	r := NewRepository(db)
	err := r.RevokeForOrganization(context.Background(), "sess-1", "org-2", revokedAt)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}
