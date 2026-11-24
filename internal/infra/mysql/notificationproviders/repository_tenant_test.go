package notificationproviders

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestNP_GetByIDForOrganization(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationProviderByIDForOrganization)).
		WithArgs(testNPID, testOrgIDNP).
		WillReturnRows(sqlmock.NewRows(npCols()).AddRow(newNPRow(t)...))

	r := NewRepository(db)
	p, err := r.GetByIDForOrganization(context.Background(), testNPID, testOrgIDNP)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, testOrgIDNP, p.OrganizationID)
	assert.Equal(t, "token-123", p.AccessToken)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNP_GetByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationProviderByIDForOrganization)).
		WithArgs(testNPID, "other-org").
		WillReturnRows(sqlmock.NewRows(npCols()))

	r := NewRepository(db)
	_, err := r.GetByIDForOrganization(context.Background(), testNPID, "other-org")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationProviderTenantQueriesShapeMySQL(t *testing.T) {
	assert.NotContains(t, queryGetNotificationProviderByID, "AND organization_id")
	assert.Contains(t, queryGetNotificationProviderByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.Contains(t, queryUpdateNotificationProvider, "AND organization_id = ?")
	assert.Contains(t, queryDeleteNotificationProvider, "AND organization_id = ?")
	assert.Contains(t, querySelectRevisionNotificationProvider, "WHERE id = ? AND organization_id = ?")
}
