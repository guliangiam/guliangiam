package mcpservers

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireMySQLNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_GetByIDForOrganization_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetMcpServerByIDForOrganization)).
		WithArgs(testMcpID, testOrgID5).
		WillReturnRows(sqlmock.NewRows(mcpCols()).AddRow(newMcpRow()...))

	r := NewRepository(db)
	s, err := r.GetByIDForOrganization(context.Background(), testMcpID, testOrgID5)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, testOrgID5, s.OrganizationID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByIDForOrganization_ForeignOrMissingIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetMcpServerByIDForOrganization)).
		WithArgs(testMcpID, "other-org").
		WillReturnRows(sqlmock.NewRows(mcpCols()))

	r := NewRepository(db)
	_, err := r.GetByIDForOrganization(context.Background(), testMcpID, "other-org")
	requireMySQLNotFound(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateMcpServer)).
		WithArgs(
			"Updated", "https://new-mcp.example.com",
			sealedMcpTokenArg{plaintext: "new-tok", serverID: testMcpID}, "new-app",
			"[]", false, fixedNow5, int64(2), testMcpID, testOrgID5,
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.McpServer{
		ID:             testMcpID,
		OrganizationID: testOrgID5,
		DisplayName:    "Updated",
		URL:            "https://new-mcp.example.com",
		Token:          "new-tok",
		Application:    "new-app",
		Tools:          []domain.Tool{},
		IsEnabled:      false,
		UpdatedAt:      fixedNow5,
		Revision:       int64(2),
	})
	requireMySQLNotFound(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer mock.ExpectationsWereMet()
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteMcpServer)).
		WithArgs(testMcpID, testOrgID5).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	requireMySQLNotFound(t, r.Delete(context.Background(), testMcpID, testOrgID5))
}
