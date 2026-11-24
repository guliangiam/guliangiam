package sessions

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func tenantSessionRepo(sessionOrg string) *mockSessionRepo {
	return &mockSessionRepo{sessions: []*domain.Session{{
		ID:             "sess-tenant",
		UserID:         "user-tenant",
		OrganizationID: sessionOrg,
		SessionState:   domain.StateActive,
		ExpiresAt:      time.Now().Add(time.Hour),
		Revision:       1,
	}}}
}

func requireErrorCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v, got nil", want)
	}
	if got := apperrors.FromError(err).Code; got != want {
		t.Fatalf("error code = %v, want %v (err=%v)", got, want, err)
	}
}

func TestGetSessionForOrganization_EmptyOrgRejected(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	svc := Service{UOW: testUnitOfWork{sessionRepo: repo}}

	_, err := svc.GetSessionForOrganization(context.Background(), "sess-tenant", "")
	requireErrorCode(t, err, apperrors.CodeInvalidArgument)
}

func TestGetSessionForOrganization_ForeignTenantNotFound(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	svc := Service{UOW: testUnitOfWork{sessionRepo: repo}}

	_, err := svc.GetSessionForOrganization(context.Background(), "sess-tenant", "org-2")
	requireErrorCode(t, err, apperrors.CodeNotFound)
}

func TestGetSessionForOrganization_OwnTenantSucceeds(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	svc := Service{UOW: testUnitOfWork{sessionRepo: repo}}

	detail, err := svc.GetSessionForOrganization(context.Background(), "sess-tenant", "org-1")
	if err != nil {
		t.Fatalf("GetSessionForOrganization() error = %v", err)
	}
	if detail.OrganizationID != "org-1" {
		t.Fatalf("OrganizationID = %q, want org-1", detail.OrganizationID)
	}
}

func TestDeleteSessionForOrganization_ForeignTenantRejected(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	records := &mockRecordsRepo{}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{sessionRepo: repo, recordsRepo: records}}

	err := svc.DeleteSessionForOrganization(context.Background(), "sess-tenant", "org-2")
	requireErrorCode(t, err, apperrors.CodeNotFound)
	if len(repo.sessions) != 1 {
		t.Fatalf("foreign delete must leave the session intact, len = %d", len(repo.sessions))
	}
	if records.createCalls != 0 {
		t.Fatalf("foreign delete must not audit, calls = %d", records.createCalls)
	}
}

func TestDeleteSessionForOrganization_OwnTenantAuditsEntityOrganization(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	records := &mockRecordsRepo{}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{sessionRepo: repo, recordsRepo: records}}

	if err := svc.DeleteSessionForOrganization(context.Background(), "sess-tenant", "org-1"); err != nil {
		t.Fatalf("DeleteSessionForOrganization() error = %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("own-tenant delete must remove the session, len = %d", len(repo.sessions))
	}
	if records.createCalls != 1 {
		t.Fatalf("audit Records.Create calls = %d, want 1", records.createCalls)
	}
	if records.records[0].OrganizationID != "org-1" {
		t.Fatalf("audit OrganizationID = %q, want org-1", records.records[0].OrganizationID)
	}
}

func TestRevokeSessionForOrganization_ForeignTenantRejected(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	records := &mockRecordsRepo{}
	outbox := &mockOutboxRepo{}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{sessionRepo: repo, recordsRepo: records, outboxRepo: outbox}}

	err := svc.RevokeSessionForOrganization(context.Background(), "sess-tenant", "org-2")
	requireErrorCode(t, err, apperrors.CodeNotFound)
	if repo.sessions[0].RevokedAt != nil {
		t.Fatal("foreign revoke must leave the session active")
	}
	if records.createCalls != 0 || outbox.enqueueCalls != 0 {
		t.Fatalf("foreign revoke must not emit audit/outbox, records=%d outbox=%d", records.createCalls, outbox.enqueueCalls)
	}
}

func TestRevokeSessionForOrganization_OwnTenantAuditsEntityOrganization(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	records := &mockRecordsRepo{}
	outbox := &mockOutboxRepo{}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{sessionRepo: repo, recordsRepo: records, outboxRepo: outbox}}

	if err := svc.RevokeSessionForOrganization(context.Background(), "sess-tenant", "org-1"); err != nil {
		t.Fatalf("RevokeSessionForOrganization() error = %v", err)
	}
	if repo.sessions[0].RevokedAt == nil {
		t.Fatal("own-tenant revoke must set RevokedAt")
	}
	if records.createCalls != 1 {
		t.Fatalf("audit Records.Create calls = %d, want 1", records.createCalls)
	}
	if records.records[0].OrganizationID != "org-1" {
		t.Fatalf("audit OrganizationID = %q, want org-1", records.records[0].OrganizationID)
	}
}

func TestUpdateSessionForOrganization_ForeignTenantRejected(t *testing.T) {
	repo := tenantSessionRepo("org-1")
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UOW: testUnitOfWork{sessionRepo: repo}}
	state := string(domain.StateIdle)

	err := svc.UpdateSessionForOrganization(context.Background(), UpdateSessionCommand{
		ID: "sess-tenant", SessionState: &state, Revision: 1,
	}, "org-2")
	requireErrorCode(t, err, apperrors.CodeNotFound)
	if repo.sessions[0].SessionState != domain.StateActive {
		t.Fatalf("foreign update must not change state, got %q", repo.sessions[0].SessionState)
	}
}
