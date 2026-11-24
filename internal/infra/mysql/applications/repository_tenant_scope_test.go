package applications

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestRepository_GetByIDForOrganization_Predicate pins the MySQL tenant
// predicate (there is no RLS on this dialect) and the argument order.
func TestRepository_GetByIDForOrganization_Predicate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("WHERE apps.id = ? AND apps.organization_id = ?")).
		WithArgs("app-1", "org-1").
		WillReturnRows(sqlmock.NewRows(rowCols()))

	_, err := NewRepository(db).GetByIDForOrganization(context.Background(), "app-1", "org-1")

	var notFound *apperrors.Error
	require.True(t, errors.As(err, &notFound))
	assert.Equal(t, apperrors.CodeNotFound, notFound.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_Delete_CrossTenant_NoChildCleanup pins that a scoped soft
// delete that matches no row returns NotFound before deleting provider bindings
// or consents, so a foreign id cannot destroy child rows.
func TestRepository_Delete_CrossTenant_NoChildCleanup(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `applications` SET deleted_at = NOW()")).
		WithArgs("app-1", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := NewRepository(db).Delete(context.Background(), "app-1", "org-1")

	var notFound *apperrors.Error
	require.True(t, errors.As(err, &notFound))
	assert.Equal(t, apperrors.CodeNotFound, notFound.Code)

	// sqlmock fails the test on any statement beyond BEGIN + scoped UPDATE +
	// ROLLBACK, so a provider-binding/consent delete would surface here.
	assert.NoError(t, mock.ExpectationsWereMet())
}
