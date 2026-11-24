package webhooks

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
)

func TestPatchWebhook_Success(t *testing.T) {
	now := time.Now()
	repo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-1", Name: "test", DisplayName: "Old", URL: "http://old", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	newDisplayName := "Updated"
	newURL := "https://new.example.com"
	err := svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{
		ID:          "wh-1",
		DisplayName: &newDisplayName,
		URL:         &newURL,
		Revision:    1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.webhooks[0].DisplayName != "Updated" {
		t.Errorf("DisplayName = %q, want %q", repo.webhooks[0].DisplayName, "Updated")
	}
	if repo.webhooks[0].URL != "https://new.example.com" {
		t.Errorf("URL = %q, want %q", repo.webhooks[0].URL, "https://new.example.com")
	}
	if repo.webhooks[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.webhooks[0].Revision)
	}
}

func TestPatchWebhook_PartialUpdate(t *testing.T) {
	now := time.Now()
	repo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-1", Name: "test", DisplayName: "Original", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	err := svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{
		ID:       "wh-1",
		Revision: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.webhooks[0].DisplayName != "Original" {
		t.Errorf("DisplayName should stay unchanged, got %q", repo.webhooks[0].DisplayName)
	}
	if repo.webhooks[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.webhooks[0].Revision)
	}
}

func TestPatchWebhook_EventsUpdate(t *testing.T) {
	now := time.Now()
	repo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-1", Name: "test", Events: []string{"old.event"}, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	err := svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{
		ID:       "wh-1",
		Events:   []string{"new.event1", "new.event2"},
		Revision: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.webhooks[0].Events) != 2 {
		t.Errorf("len(Events) = %d, want 2", len(repo.webhooks[0].Events))
	}
	if repo.webhooks[0].Events[0] != "new.event1" {
		t.Errorf("Events[0] = %q, want %q", repo.webhooks[0].Events[0], "new.event1")
	}
}

func TestCreateWebhook_IsEnabledDefault(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "wh-id"},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	_, err := svc.CreateWebhook(context.Background(), CreateWebhookCommand{
		OrganizationID: "org-1",
		Name:           "test",
		URL:            "https://example.com/hook",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.webhooks[0].IsEnabled {
		t.Error("IsEnabled should default to true")
	}
}

func TestCreateWebhook_IsEnabledFalse(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "wh-id"},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	isEnabled := false
	_, err := svc.CreateWebhook(context.Background(), CreateWebhookCommand{
		OrganizationID: "org-1",
		Name:           "test",
		URL:            "https://example.com/hook",
		IsEnabled:      &isEnabled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.webhooks[0].IsEnabled {
		t.Error("IsEnabled should be false when explicitly set")
	}
}

func TestPatchWebhook_NotFound(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	err := svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{ID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent webhook")
	}
}

// ---------------------------------------------------------------------------
// By-ID tenant binding (IDOR P1): cross-tenant reads and writes must behave
// like a missing row, without touching the row or writing an audit record.
// ---------------------------------------------------------------------------

const (
	crudTenantOrgID = "01a0ea76-0c00-7000-8000-000000000601"
	crudOtherOrgID  = "01a0ea76-0c00-7000-8000-000000000602"
	crudWebhookID   = "01a0ea76-0c00-7000-8000-000000000603"
)

func newCRUDTenantFixture(ownerOrgID string) (*mockWebhookRepo, Service, *int) {
	auditCalls := 0
	repo := &mockWebhookRepo{webhooks: []*domain.Webhook{
		{ID: crudWebhookID, OrganizationID: ownerOrgID, Name: "hook", DisplayName: "Original", Revision: 1},
	}}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW: testUnitOfWork{
			webhookRepo: repo, eventRepo: &mockEventRepo{}, auditCalls: &auditCalls,
		},
	}
	return repo, svc, &auditCalls
}

func TestGetWebhook_CrossTenantIsNotFound(t *testing.T) {
	_, svc, _ := newCRUDTenantFixture(crudOtherOrgID)

	detail, err := svc.GetWebhook(context.Background(), crudWebhookID, crudTenantOrgID)
	assertNotFound(t, err)
	if detail != nil {
		t.Errorf("detail = %+v, want nil", detail)
	}
}

func TestGetWebhook_SameTenantReturnsDetail(t *testing.T) {
	_, svc, _ := newCRUDTenantFixture(crudTenantOrgID)

	detail, err := svc.GetWebhook(context.Background(), crudWebhookID, crudTenantOrgID)
	if err != nil {
		t.Fatalf("GetWebhook() error = %v", err)
	}
	if detail.ID != crudWebhookID {
		t.Errorf("ID = %q, want %q", detail.ID, crudWebhookID)
	}
}

func TestGetWebhook_PlatformScopeCrossesTenant(t *testing.T) {
	_, svc, _ := newCRUDTenantFixture(crudOtherOrgID)

	detail, err := svc.GetWebhook(platformScopeContext(), crudWebhookID, "")
	if err != nil {
		t.Fatalf("platform-scope GetWebhook() error = %v", err)
	}
	if detail.ID != crudWebhookID {
		t.Errorf("ID = %q, want %q", detail.ID, crudWebhookID)
	}
}

func TestGetWebhook_EmptyTenantWithoutPlatformScopeRejected(t *testing.T) {
	_, svc, _ := newCRUDTenantFixture(crudOtherOrgID)

	detail, err := svc.GetWebhook(context.Background(), crudWebhookID, "")
	assertInvalidArgument(t, err)
	if detail != nil {
		t.Errorf("detail = %+v, want nil", detail)
	}
}

func TestPatchWebhook_CrossTenantIsNotFoundBeforeWrite(t *testing.T) {
	repo, svc, auditCalls := newCRUDTenantFixture(crudOtherOrgID)
	hijacked := "Hijacked"

	err := svc.PatchWebhook(context.Background(), crudTenantOrgID, PatchWebhookCommand{
		ID: crudWebhookID, DisplayName: &hijacked,
	})
	assertNotFound(t, err)
	if repo.updates != 0 {
		t.Errorf("updates = %d, want 0", repo.updates)
	}
	if *auditCalls != 0 {
		t.Errorf("audit repo calls = %d, want 0", *auditCalls)
	}
	if repo.webhooks[0].DisplayName != "Original" {
		t.Errorf("DisplayName = %q, want %q", repo.webhooks[0].DisplayName, "Original")
	}
	if repo.webhooks[0].Revision != 1 {
		t.Errorf("Revision = %d, want 1", repo.webhooks[0].Revision)
	}
}

func TestPatchWebhook_SameTenantUpdatesAndAudits(t *testing.T) {
	repo, svc, auditCalls := newCRUDTenantFixture(crudTenantOrgID)
	updated := "Updated"

	err := svc.PatchWebhook(context.Background(), crudTenantOrgID, PatchWebhookCommand{
		ID: crudWebhookID, DisplayName: &updated, Revision: 1,
	})
	if err != nil {
		t.Fatalf("PatchWebhook() error = %v", err)
	}
	if repo.updates != 1 {
		t.Errorf("updates = %d, want 1", repo.updates)
	}
	if *auditCalls != 2 {
		t.Errorf("audit repo calls = %d, want 2 (record + outbox)", *auditCalls)
	}
	if repo.webhooks[0].DisplayName != "Updated" {
		t.Errorf("DisplayName = %q, want %q", repo.webhooks[0].DisplayName, "Updated")
	}
}

func TestPatchWebhook_PlatformScopeCrossesTenant(t *testing.T) {
	repo, svc, auditCalls := newCRUDTenantFixture(crudOtherOrgID)
	updated := "PlatformUpdated"

	err := svc.PatchWebhook(platformScopeContext(), "", PatchWebhookCommand{
		ID: crudWebhookID, DisplayName: &updated, Revision: 1,
	})
	if err != nil {
		t.Fatalf("platform-scope PatchWebhook() error = %v", err)
	}
	if repo.updates != 1 {
		t.Errorf("updates = %d, want 1", repo.updates)
	}
	if *auditCalls != 2 {
		t.Errorf("audit repo calls = %d, want 2 (record + outbox)", *auditCalls)
	}
}

func TestDeleteWebhook_CrossTenantIsNotFoundBeforeDelete(t *testing.T) {
	repo, svc, auditCalls := newCRUDTenantFixture(crudOtherOrgID)

	err := svc.DeleteWebhook(context.Background(), crudWebhookID, crudTenantOrgID)
	assertNotFound(t, err)
	if repo.deletes != 0 {
		t.Errorf("deletes = %d, want 0", repo.deletes)
	}
	if *auditCalls != 0 {
		t.Errorf("audit repo calls = %d, want 0", *auditCalls)
	}
	if len(repo.webhooks) != 1 {
		t.Errorf("len(webhooks) = %d, want 1", len(repo.webhooks))
	}
}

func TestDeleteWebhook_SameTenantDeletesAndAudits(t *testing.T) {
	repo, svc, auditCalls := newCRUDTenantFixture(crudTenantOrgID)

	err := svc.DeleteWebhook(context.Background(), crudWebhookID, crudTenantOrgID)
	if err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}
	if repo.deletes != 1 {
		t.Errorf("deletes = %d, want 1", repo.deletes)
	}
	if *auditCalls != 2 {
		t.Errorf("audit repo calls = %d, want 2 (record + outbox)", *auditCalls)
	}
	if len(repo.webhooks) != 0 {
		t.Errorf("len(webhooks) = %d, want 0", len(repo.webhooks))
	}
}

func TestDeleteWebhook_PlatformScopeCrossesTenant(t *testing.T) {
	repo, svc, auditCalls := newCRUDTenantFixture(crudOtherOrgID)

	err := svc.DeleteWebhook(platformScopeContext(), crudWebhookID, "")
	if err != nil {
		t.Fatalf("platform-scope DeleteWebhook() error = %v", err)
	}
	if repo.deletes != 1 {
		t.Errorf("deletes = %d, want 1", repo.deletes)
	}
	if *auditCalls != 2 {
		t.Errorf("audit repo calls = %d, want 2 (record + outbox)", *auditCalls)
	}
}

func TestCreateWebhook_GeneratesSigningSecretOnce(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "wh-id"},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	secret, err := svc.CreateWebhook(context.Background(), CreateWebhookCommand{
		OrganizationID: "org-1",
		Name:           "test",
		URL:            "https://example.com/hook",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(secret, "whsec_") || len(secret) != len("whsec_")+64 {
		t.Fatalf("secret shape = %q, want whsec_ + 64 hex", secret)
	}
	if repo.webhooks[0].Secret != secret {
		t.Fatalf("persisted secret mismatch: %q vs %q", repo.webhooks[0].Secret, secret)
	}

	// 读模型不得回传 secret：toDetail 只映射非敏感字段。
	detail := toDetail(repo.webhooks[0])
	if strings.Contains(fmt.Sprintf("%+v", detail), secret) {
		t.Fatal("webhook detail must not expose the signing secret")
	}
}
