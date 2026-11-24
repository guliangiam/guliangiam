package gateway

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

func TestRepository_GetSiteByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetSiteByIDForOrganization)).
		WithArgs(testSiteID, testOrgID).
		WillReturnRows(sqlmock.NewRows(siteCols()).AddRow(newSiteRow()...))

	r := NewRepository(db)
	s, err := r.GetSiteByIDForOrganization(context.Background(), testSiteID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, testSiteID, s.ID)
	assert.Equal(t, testOrgID, s.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetSiteByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetSiteByIDForOrganization)).
		WithArgs(testSiteID, "other-org").
		WillReturnRows(sqlmock.NewRows(siteCols()))

	r := NewRepository(db)
	s, err := r.GetSiteByIDForOrganization(context.Background(), testSiteID, "other-org")
	require.Error(t, err)
	require.Nil(t, s)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetSiteByIDForOrganization_QueryError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetSiteByIDForOrganization)).
		WithArgs(testSiteID, testOrgID).
		WillReturnError(assert.AnError)

	r := NewRepository(db)
	_, err := r.GetSiteByIDForOrganization(context.Background(), testSiteID, testOrgID)
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetServerByIDForOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetServerByIDForOrganization)).
		WithArgs(testServerID, testOrgID).
		WillReturnRows(sqlmock.NewRows(serverCols()).AddRow(newServerRow()...))

	r := NewRepository(db)
	s, err := r.GetServerByIDForOrganization(context.Background(), testServerID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, testServerID, s.ID)
	assert.Equal(t, testOrgID, s.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetServerByIDForOrganization_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetServerByIDForOrganization)).
		WithArgs(testServerID, "other-org").
		WillReturnRows(sqlmock.NewRows(serverCols()))

	r := NewRepository(db)
	s, err := r.GetServerByIDForOrganization(context.Background(), testServerID, "other-org")
	require.Error(t, err)
	require.Nil(t, s)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestServerQueries_TenantPredicateShape(t *testing.T) {
	assert.Contains(t, queryGetServerByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.NotContains(t, queryGetServerByID, "WHERE id = ? AND organization_id")

	var _ domain.Repository = Repository{}
}

func TestSiteQueries_TenantPredicateShape(t *testing.T) {
	assert.Contains(t, queryGetSiteByIDForOrganization, "WHERE id = ? AND organization_id = ?")
	assert.NotContains(t, queryGetSiteByID, "WHERE id = ? AND organization_id")

	var _ domain.Repository = Repository{}
}
