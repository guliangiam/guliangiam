package iga

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/stretchr/testify/require"
)

func TestListAccessRequests_FiltersWorkflowState(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	status := domain.RequestPendingManager
	filter := " AND COALESCE(workflow_state, status) = ?"
	countSQL := "SELECT COUNT(*) FROM `tickets` WHERE organization_id = ? AND type = 'iga_access_request' AND deleted_at IS NULL" + filter
	listSQL := qListAccessRequestsBase + filter + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	mock.ExpectQuery(regexp.QuoteMeta(countSQL)).
		WithArgs(testOrgID, string(status)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(listSQL)).
		WithArgs(testOrgID, string(status), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	items, total, err := NewRepository(db).ListAccessRequests(context.Background(), domain.ListAccessRequestsQuery{
		OrganizationID: testOrgID, Status: &status, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	require.NotContains(t, countSQL, "AND status = ?")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateAccessTargets_ReturnsOrganizationSlug(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(qValidateAccessTargets)).
		WithArgs(testOrgID, "user-1", testOrgID, "role-1", "app-1", testOrgID, "app-1", testOrgID, "enforcer-1", testOrgID, "user-1", testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"user_valid", "role_valid", "application_valid", "enforcer_valid", "subject", "organization_slug"}).
			AddRow(true, true, true, true, "alice", "acme"))

	validity, err := NewRepository(db).ValidateAccessTargets(context.Background(), testOrgID, "user-1", "role-1", "app-1", "enforcer-1")
	require.NoError(t, err)
	require.True(t, validity.Valid())
	require.Equal(t, "alice", validity.Subject)
	require.Equal(t, "acme", validity.OrganizationSlug)
	require.Contains(t, qValidateAccessTargets, "SELECT slug FROM `organizations`")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateJITPolicy_WritesPPolicyWithOrganizationSlug(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	startsAt := time.Date(2026, 10, 27, 11, 0, 0, 0, time.UTC)
	expiresAt := startsAt.Add(time.Hour)
	metadata := `{"lifecycleId":"lifecycle-1","source":"iga_jit","startsAt":"2026-10-27T11:00:00Z"}`
	mock.ExpectExec(regexp.QuoteMeta(qInsertJITPolicy)).
		WithArgs("policy-1", "enforcer-1", "alice", "acme", "orders", "get", expiresAt.Format(time.RFC3339Nano), metadata).
		WillReturnResult(sqlmock.NewResult(1, 1))

	policy := &domain.JITPolicy{
		ID: "policy-1", EnforcerID: "enforcer-1", LifecycleID: "lifecycle-1",
		UserID: "user-1", Subject: "alice", OrganizationSlug: "acme",
		Resource: "orders", Action: "get", StartsAt: startsAt, ExpiresAt: expiresAt,
	}
	require.NoError(t, NewRepository(db).CreateJITPolicy(context.Background(), policy))
	require.Contains(t, qInsertJITPolicy, "'p', ?")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListExpiringLifecycle_DoesNotRequireBinding(t *testing.T) {
	before := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		orgID string
		query string
		args  []driver.Value
	}{
		{name: "global", query: qListExpiringLifecycle, args: []driver.Value{before, 25}},
		{name: "by organization", orgID: testOrgID, query: qListExpiringLifecycleByOrg, args: []driver.Value{testOrgID, before, 25}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			defer db.Close()
			mock.ExpectQuery(regexp.QuoteMeta(tt.query)).
				WithArgs(tt.args...).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))
			items, err := NewRepository(db).ListExpiringLifecycle(context.Background(), tt.orgID, before, 25)
			require.NoError(t, err)
			require.Empty(t, items)
			require.Contains(t, tt.query, "l.status = 'active'")
			require.Contains(t, tt.query, "l.expires_at <=")
			require.NotContains(t, tt.query, "user_role_bindings")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRevokeJITArtifacts_DeletesResidualBindingAndPPolicy(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(qDeleteJITBinding)).
		WithArgs("lifecycle-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(qDeleteJITPolicy)).
		WithArgs("policy-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, NewRepository(db).RevokeJITArtifacts(context.Background(), "lifecycle-1", "policy-1"))
	require.Contains(t, qDeleteJITPolicy, "ptype = 'p'")
	require.NoError(t, mock.ExpectationsWereMet())
}
