package iga_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appiga "github.com/guliangiam/guliangiam/internal/app/iga"
	domainiga "github.com/guliangiam/guliangiam/internal/domain/iga"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/require"
)

func newPAMFixture() (*workflowRepo, *workflowSessions) {
	request := newWorkflowRequest(domainiga.RequestProvisioned, 2)
	request.RequesterID = "user-1"
	request.LifecycleID = "lifecycle-1"
	lifecycle := &domainiga.AccessLifecycle{
		ID: "lifecycle-1", OrganizationID: "org-uuid", UserID: "user-1", RoleID: "role-1",
		PolicyID: "policy-1", Status: domainiga.LifecycleActive,
		StartsAt: workflowNow.Add(-time.Hour), ExpiresAt: workflowNow.Add(30 * time.Minute),
	}
	session := &domainsessions.Session{
		ID: "session-1", UserID: "user-1", OrganizationID: "org-uuid",
		SessionState: domainsessions.StateActive, ExpiresAt: workflowNow.Add(2 * time.Hour),
		CreatedAt: workflowNow.Add(-time.Hour), LastSeenAt: workflowNow, Revision: 1,
	}
	return &workflowRepo{request: request, lifecycle: lifecycle}, &workflowSessions{session: session}
}

func TestStartPrivilegedSession_ElevatesAndClampsExpiry(t *testing.T) {
	repo, sessions := newPAMFixture()
	service, uow := newWorkflowService(repo, sessions)

	result, err := service.StartPrivilegedSession(context.Background(), actionCommand("user-1", 2), "session-1")
	require.NoError(t, err)
	require.True(t, result.Privileged)
	require.Equal(t, "lifecycle-1", result.LifecycleID)
	require.Equal(t, "ticket-1", result.TicketID)
	require.Equal(t, repo.lifecycle.ExpiresAt, result.ExpiresAt)
	require.Equal(t, 1, sessions.updates)
	require.True(t, sessions.session.IsPrivileged)
	require.Equal(t, repo.lifecycle.ExpiresAt, sessions.session.ExpiresAt)
	require.Len(t, uow.records.items, 1)
	require.Equal(t, "iga.pam.session_started", uow.records.items[0].Action)
	require.Len(t, uow.outbox.items, 1)
	require.Equal(t, "iga.pam.session_started", uow.outbox.items[0].EventType)
}

func TestStartPrivilegedSession_RejectsExpiredAndFutureLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domainiga.AccessLifecycle)
	}{
		{name: "expired", mutate: func(l *domainiga.AccessLifecycle) { l.ExpiresAt = workflowNow }},
		{name: "future", mutate: func(l *domainiga.AccessLifecycle) {
			l.StartsAt = workflowNow.Add(time.Hour)
			l.ExpiresAt = workflowNow.Add(2 * time.Hour)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, sessions := newPAMFixture()
			tt.mutate(repo.lifecycle)
			service, uow := newWorkflowService(repo, sessions)

			result, err := service.StartPrivilegedSession(context.Background(), actionCommand("user-1", 2), "session-1")
			require.Error(t, err)
			require.Nil(t, result)
			require.Zero(t, sessions.updates)
			require.False(t, sessions.session.IsPrivileged)
			require.Empty(t, uow.records.items)
			require.Empty(t, uow.outbox.items)
		})
	}
}

func TestAuditPrivilegedOperation_RechecksLifecycleWindow(t *testing.T) {
	repo, sessions := newPAMFixture()
	sessions.session.IsPrivileged = true
	sessions.session.LifecycleID = "lifecycle-1"
	sessions.session.TicketID = "ticket-1"
	service, uow := newWorkflowService(repo, sessions)

	err := service.AuditPrivilegedOperation(context.Background(), "org-uuid", "session-1", "user-1", "get", "orders")
	require.NoError(t, err)
	require.Len(t, uow.records.items, 1)
	require.Equal(t, "iga.pam.operation", uow.records.items[0].Action)
	require.Len(t, uow.outbox.items, 1)
	require.Equal(t, "iga.pam.operation", uow.outbox.items[0].EventType)

	repo.lifecycle.ExpiresAt = workflowNow
	err = service.AuditPrivilegedOperation(context.Background(), "org-uuid", "session-1", "user-1", "get", "orders")
	require.Error(t, err)
	require.ErrorContains(t, err, "outside its time window")
	require.Len(t, uow.records.items, 1, "expired privileged operations must not emit a success audit")
	require.Len(t, uow.outbox.items, 1)
}

func TestListPrivilegedSessions_UsesTenantTransaction(t *testing.T) {
	repo, sessions := newPAMFixture()
	sessions.session.IsPrivileged = true
	sessions.session.LifecycleID = "lifecycle-1"
	service, uow := newWorkflowService(repo, sessions)
	ctx := tenantctx.WithTenantScope(context.Background(), "org-uuid", "acme", tenantctx.TenantFilterStrict)

	result, err := service.ListPrivilegedSessions(ctx, "org-uuid", "user-1")
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "lifecycle-1", result[0].LifecycleID)
	require.True(t, sessions.listInTx)
	require.Len(t, uow.transactionScopes, 1)
	require.Equal(t, tenantctx.TenantFilterStrict, uow.transactionScopes[0].Mode)
	require.Equal(t, "org-uuid", uow.transactionScopes[0].OrganizationID)
}

func TestPrivilegedSessionDTO_JSONFieldNames(t *testing.T) {
	dto := appiga.PrivilegedSessionDTO{
		SessionID:    "session-1",
		LifecycleID:  "lifecycle-1",
		TicketID:     "ticket-1",
		Privileged:   true,
		SessionState: "active",
		ExpiresAt:    time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(dto)
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	for _, key := range []string{"sessionId", "lifecycleId", "ticketId", "privileged", "sessionState", "expiresAt"} {
		require.Contains(t, fields, key)
	}
	for _, legacy := range []string{"SessionID", "LifecycleID", "TicketID", "SessionState", "ExpiresAt"} {
		require.NotContains(t, fields, legacy)
	}
}
