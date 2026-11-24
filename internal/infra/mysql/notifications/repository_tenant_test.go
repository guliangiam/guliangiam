package notifications

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRepository_GetByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationByIDForOrganization)).
		WithArgs(testID, testOrgID).
		WillReturnRows(sqlmock.NewRows(notificationCols()).AddRow(newNotificationRow()...))

	r := NewRepository(db)
	n, err := r.GetByIDForOrganization(context.Background(), testID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, n)
	assert.Equal(t, testOrgID, n.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationByIDForOrganization)).
		WithArgs(testID, "other-org").
		WillReturnRows(sqlmock.NewRows(notificationCols()))

	r := NewRepository(db)
	_, err := r.GetByIDForOrganization(context.Background(), testID, "other-org")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_MarkSentForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryMarkSentForOrganization)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), testID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.MarkSentForOrganization(context.Background(), testID, testOrgID)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_MarkSentForOrganization_ZeroRowsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryMarkSentForOrganization)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), testID, "other-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.MarkSentForOrganization(context.Background(), testID, "other-org")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationTenantQueriesShapeMySQL(t *testing.T) {
	assert.NotContains(t, queryGetNotificationByID, "WHERE id = ? AND organization_id")
	assert.Contains(t, queryGetNotificationByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.NotContains(t, queryMarkSent, "organization_id")
	assert.Contains(t, queryMarkSentForOrganization, "WHERE id = ? AND organization_id = ?")
}
