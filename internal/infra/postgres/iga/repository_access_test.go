package iga

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestListAccessRequests_FiltersWorkflowState(t *testing.T) {
	status := domain.RequestPendingManager
	var countSQL, listSQL string
	var countArgs, listArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			countSQL = sql
			countArgs = args
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			listSQL = sql
			listArgs = args
			return shared.NewMockRows(nil), nil
		},
	}
	repo := NewRepository(db)
	items, total, err := repo.ListAccessRequests(context.Background(), domain.ListAccessRequestsQuery{
		OrganizationID: "org-1", Status: &status, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	require.Contains(t, countSQL, "AND COALESCE(workflow_state, status) = $2")
	require.Contains(t, listSQL, "AND COALESCE(workflow_state, status) = $2")
	require.NotContains(t, countSQL, "AND status =")
	require.Equal(t, []any{"org-1", string(status)}, countArgs)
	require.Equal(t, []any{"org-1", string(status), 20, 0}, listArgs)
}

func TestValidateAccessTargets_ReturnsOrganizationSlug(t *testing.T) {
	var gotSQL string
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
		gotSQL = sql
		require.Equal(t, []any{"org-1", "user-1", "role-1", "app-1", "enforcer-1"}, args)
		return shared.NewMockRow([]any{true, true, true, true, "alice", "acme"}, nil)
	}}
	validity, err := NewRepository(db).ValidateAccessTargets(context.Background(), "org-1", "user-1", "role-1", "app-1", "enforcer-1")
	require.NoError(t, err)
	require.True(t, validity.Valid())
	require.Equal(t, "alice", validity.Subject)
	require.Equal(t, "acme", validity.OrganizationSlug)
	require.Contains(t, gotSQL, "SELECT slug FROM guliangiam.organizations")
}

func TestCreateJITPolicy_WritesPPolicyWithOrganizationSlug(t *testing.T) {
	startsAt := time.Date(2026, 10, 27, 11, 0, 0, 0, time.UTC)
	expiresAt := startsAt.Add(time.Hour)
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		gotSQL = sql
		gotArgs = args
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}}
	policy := &domain.JITPolicy{
		ID: "policy-1", EnforcerID: "enforcer-1", LifecycleID: "lifecycle-1",
		UserID: "user-1", Subject: "alice", OrganizationSlug: "acme",
		Resource: "orders", Action: "get", StartsAt: startsAt, ExpiresAt: expiresAt,
	}
	require.NoError(t, NewRepository(db).CreateJITPolicy(context.Background(), policy))
	require.Equal(t, qInsertJITPolicy, gotSQL)
	require.Contains(t, gotSQL, "ptype, v0")
	require.Contains(t, gotSQL, "'p', $3")
	require.Len(t, gotArgs, 8)
	require.Equal(t, "alice", gotArgs[2])
	require.Equal(t, "acme", gotArgs[3], "Casbin v1 must be the runtime organization slug, not the UUID")
	require.Equal(t, "orders", gotArgs[4])
	require.Equal(t, "get", gotArgs[5])
	require.Equal(t, expiresAt.Format(time.RFC3339Nano), gotArgs[6])
	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotArgs[7].(string)), &metadata))
	require.Equal(t, "iga_jit", metadata["source"])
	require.Equal(t, "lifecycle-1", metadata["lifecycleId"])
	require.Equal(t, startsAt.Format(time.RFC3339Nano), metadata["startsAt"])
}

func TestListExpiringLifecycle_DoesNotRequireBinding(t *testing.T) {
	before := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		orgID string
		want  string
	}{
		{name: "global", want: qListExpiringLifecycle},
		{name: "by organization", orgID: "org-1", want: qListExpiringLifecycleByOrg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotSQL string
			db := &shared.MockDBTX{QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
				gotSQL = sql
				return shared.NewMockRows(nil), nil
			}}
			items, err := NewRepository(db).ListExpiringLifecycle(context.Background(), tt.orgID, before, 25)
			require.NoError(t, err)
			require.Empty(t, items)
			require.Equal(t, tt.want, gotSQL)
			require.Contains(t, gotSQL, "l.status = 'active'")
			require.Contains(t, gotSQL, "l.expires_at <=")
			require.NotContains(t, gotSQL, "user_role_bindings")
		})
	}
}

func TestRevokeJITArtifacts_DeletesResidualBindingAndPPolicy(t *testing.T) {
	var statements []string
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		statements = append(statements, sql)
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}
	require.NoError(t, NewRepository(db).RevokeJITArtifacts(context.Background(), "lifecycle-1", "policy-1"))
	require.Equal(t, []string{qDeleteJITBinding, qDeleteJITPolicy}, statements)
	require.Contains(t, qDeleteJITPolicy, "ptype = 'p'")
}
