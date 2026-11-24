package users

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	customattributes "github.com/guliangiam/guliangiam/internal/domain/customattributes"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Custom attribute fakes
// ---------------------------------------------------------------------------

type fakeDefinitionRepo struct {
	existing  *customattributes.Definition
	created   []*customattributes.Definition
	updated   []*customattributes.Definition
	deleted   []string
	getErr    error
	createErr error
	updateErr error
	deleteErr error
}

func (r *fakeDefinitionRepo) ListByOrganization(_ context.Context, _ string, _ bool) ([]*customattributes.Definition, error) {
	return nil, nil
}

func (r *fakeDefinitionRepo) GetByOrganizationAndKey(_ context.Context, _, _ string) (*customattributes.Definition, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.existing == nil {
		return nil, apperrors.NewNotFound("custom attribute not found")
	}
	return r.existing, nil
}

func (r *fakeDefinitionRepo) Create(_ context.Context, definition *customattributes.Definition) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, definition)
	return nil
}

func (r *fakeDefinitionRepo) Update(_ context.Context, definition *customattributes.Definition) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updated = append(r.updated, definition)
	return nil
}

func (r *fakeDefinitionRepo) Delete(_ context.Context, _, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deleted = append(r.deleted, id)
	return nil
}

type captureOutbox struct {
	domainoutbox.Repository

	events []*domainoutbox.OutboxEvent
}

func (o *captureOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, event)
	return nil
}

// customAttributeTestUnitOfWork adds the custom attribute port (and an outbox
// capture) that svcTestUnitOfWork does not provide. It overrides
// WithinTransaction so the callback receives the widest unit of work.
type customAttributeTestUnitOfWork struct {
	svcTestUnitOfWork
	definitions customattributes.Repository
	outbox      domainoutbox.Repository
}

func (u customAttributeTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u customAttributeTestUnitOfWork) CustomAttributeDefinitions() customattributes.Repository {
	return u.definitions
}

func (u customAttributeTestUnitOfWork) Outbox() domainoutbox.Repository { return u.outbox }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func requireSingleAudit(t *testing.T, records *fakeRecordsRepo, action, actorID, resourceType, resourceID string) {
	t.Helper()
	if len(records.created) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(records.created))
	}
	record := records.created[0]
	if record.Action != action {
		t.Fatalf("audit action = %q, want %q", record.Action, action)
	}
	if record.ActorID == "" || record.ActorID != actorID {
		t.Fatalf("audit actor = %q, want %q (non-empty)", record.ActorID, actorID)
	}
	if record.ResourceType != resourceType || record.ResourceID != resourceID {
		t.Fatalf("audit resource = %q/%q, want %q/%q", record.ResourceType, record.ResourceID, resourceType, resourceID)
	}
}

// ---------------------------------------------------------------------------
// ExitImpersonate
// ---------------------------------------------------------------------------

func TestService_ExitImpersonate_RevokesOnlyImpersonationTokens(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	impToken := &tokendomain.Token{ID: "tok-imp", UserID: "user-1", OrganizationID: "org-1", TokenPrefix: "imp"}
	normalToken := &tokendomain.Token{ID: "tok-normal", UserID: "user-1", OrganizationID: "org-1", TokenPrefix: "at"}

	var captured tokendomain.RevokeQuery
	tokenRepo := &mockTokenRepo{
		revokeByQueryFn: func(_ context.Context, query tokendomain.RevokeQuery, revokedAt time.Time) (int64, error) {
			captured = query
			var revoked int64
			for _, token := range []*tokendomain.Token{impToken, normalToken} {
				if query.OrganizationID != "" && token.OrganizationID != query.OrganizationID {
					continue
				}
				if query.UserID != "" && token.UserID != query.UserID {
					continue
				}
				if query.TokenPrefix != "" && token.TokenPrefix != query.TokenPrefix {
					continue
				}
				at := revokedAt
				token.RevokedAt = &at
				revoked++
			}
			return revoked, nil
		},
	}
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, organizationID string) (*userdomain.User, error) {
			return &userdomain.User{ID: id, OrganizationID: organizationID}, nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   testUnitOfWork{users: userRepo, tokens: tokenRepo, records: &fakeRecordsRepo{}},
	}

	err := svc.ExitImpersonate(context.Background(), ExitImpersonateCommand{
		OrganizationID: "org-1", UserID: "user-1", ActorID: "admin-1",
	})
	if err != nil {
		t.Fatalf("ExitImpersonate() unexpected error: %v", err)
	}

	if captured.TokenPrefix != "imp" {
		t.Fatalf("revoke query token prefix = %q, want %q", captured.TokenPrefix, "imp")
	}
	if impToken.RevokedAt == nil {
		t.Fatal("impersonation token was not revoked")
	}
	if normalToken.RevokedAt != nil {
		t.Fatal("regular token must not be revoked by exit-impersonate")
	}
}

func TestService_ExitImpersonate_EmitsAudit(t *testing.T) {
	records := &fakeRecordsRepo{}
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, organizationID string) (*userdomain.User, error) {
			return &userdomain.User{ID: id, OrganizationID: organizationID}, nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{users: userRepo, tokens: &mockTokenRepo{}, records: records},
	}

	err := svc.ExitImpersonate(context.Background(), ExitImpersonateCommand{
		OrganizationID: "org-1", UserID: "user-1", ActorID: "admin-1",
	})
	if err != nil {
		t.Fatalf("ExitImpersonate() unexpected error: %v", err)
	}

	requireSingleAudit(t, records, "user.impersonation.ended", "admin-1", "User", "user-1")
	if got := records.created[0].OrganizationID; got != "org-1" {
		t.Fatalf("audit organization = %q, want %q", got, "org-1")
	}
}

// ---------------------------------------------------------------------------
// RemoveUserFromGroup
// ---------------------------------------------------------------------------

func TestService_RemoveUserFromGroup_EmitsAudit(t *testing.T) {
	records := &fakeRecordsRepo{}
	var remaining []userdomain.UserGroupMembership
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, organizationID string) (*userdomain.User, error) {
			return &userdomain.User{
				ID: id, OrganizationID: organizationID,
				GroupMemberships: []userdomain.UserGroupMembership{
					{UserID: id, GroupID: "group-1"},
					{UserID: id, GroupID: "group-2"},
				},
			}, nil
		},
		replaceGroupMembershipsFn: func(_ context.Context, _ string, memberships []userdomain.UserGroupMembership) error {
			remaining = memberships
			return nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{users: userRepo, records: records},
	}

	err := svc.RemoveUserFromGroup(context.Background(), RemoveFromGroupCommand{
		OrganizationID: "org-1", UserID: "user-1", GroupID: "group-1", ActorID: "admin-1",
	})
	if err != nil {
		t.Fatalf("RemoveUserFromGroup() unexpected error: %v", err)
	}

	if len(remaining) != 1 || remaining[0].GroupID != "group-2" {
		t.Fatalf("remaining memberships = %+v, want only group-2", remaining)
	}
	requireSingleAudit(t, records, "user.group_membership.removed", "admin-1", "User", "user-1")
}

// ---------------------------------------------------------------------------
// Custom attributes
// ---------------------------------------------------------------------------

func TestService_CreateCustomAttribute_EmitsAudit(t *testing.T) {
	records := &fakeRecordsRepo{}
	outbox := &captureOutbox{}
	definitions := &fakeDefinitionRepo{}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UUID:  svcFakeUUID{fixedID: "def-1"},
		UOW: customAttributeTestUnitOfWork{
			svcTestUnitOfWork: svcTestUnitOfWork{records: records},
			definitions:       definitions,
			outbox:            outbox,
		},
	}

	definition, err := svc.CreateCustomAttribute(context.Background(), CreateCustomAttributeCommand{
		OrganizationID: "org-1", ActorID: "admin-1",
		Key: "nickname", DataType: "string", Visibility: "public",
	})
	if err != nil {
		t.Fatalf("CreateCustomAttribute() unexpected error: %v", err)
	}
	if len(definitions.created) != 1 || definitions.created[0].ID != definition.ID {
		t.Fatalf("created definitions = %d, want the new definition", len(definitions.created))
	}

	requireSingleAudit(t, records, "user.custom_attribute.created", "admin-1", "CustomAttribute", definition.ID)
	if len(outbox.events) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(outbox.events))
	}
	payload := decodePayload(t, outbox.events[0])
	if payload["key"] != "nickname" {
		t.Fatalf("audit payload key = %v, want nickname", payload["key"])
	}
	requireNoPropertyValues(t, payload)
}

func TestService_UpdateCustomAttribute_EmitsAudit(t *testing.T) {
	records := &fakeRecordsRepo{}
	outbox := &captureOutbox{}
	definitions := &fakeDefinitionRepo{existing: &customattributes.Definition{
		ID: "def-1", OrganizationID: "org-1", Key: "nickname",
		DataType: customattributes.DataTypeString, Visibility: customattributes.VisibilityPrivate,
		Enabled: true, Writable: true, Revision: 3,
	}}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UUID:  svcFakeUUID{fixedID: "unused"},
		UOW: customAttributeTestUnitOfWork{
			svcTestUnitOfWork: svcTestUnitOfWork{records: records},
			definitions:       definitions,
			outbox:            outbox,
		},
	}

	definition, err := svc.UpdateCustomAttribute(context.Background(), UpdateCustomAttributeCommand{
		OrganizationID: "org-1", ActorID: "admin-1",
		Key: "nickname", DataType: "number", Visibility: "public", Required: true,
	})
	if err != nil {
		t.Fatalf("UpdateCustomAttribute() unexpected error: %v", err)
	}
	if len(definitions.updated) != 1 {
		t.Fatalf("updated definitions = %d, want 1", len(definitions.updated))
	}

	requireSingleAudit(t, records, "user.custom_attribute.updated", "admin-1", "CustomAttribute", definition.ID)
	if len(outbox.events) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(outbox.events))
	}
	payload := decodePayload(t, outbox.events[0])
	if payload["key"] != "nickname" {
		t.Fatalf("audit payload key = %v, want nickname", payload["key"])
	}
	if payload["revision"] != float64(4) {
		t.Fatalf("audit payload revision = %v, want 4", payload["revision"])
	}
	requireNoPropertyValues(t, payload)
}

func TestService_DeleteCustomAttribute_EmitsAudit(t *testing.T) {
	records := &fakeRecordsRepo{}
	definitions := &fakeDefinitionRepo{}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UOW: customAttributeTestUnitOfWork{
			svcTestUnitOfWork: svcTestUnitOfWork{records: records},
			definitions:       definitions,
		},
	}

	if err := svc.DeleteCustomAttribute(context.Background(), "org-1", "def-1", "admin-1"); err != nil {
		t.Fatalf("DeleteCustomAttribute() unexpected error: %v", err)
	}

	if len(definitions.deleted) != 1 || definitions.deleted[0] != "def-1" {
		t.Fatalf("deleted definitions = %+v, want def-1", definitions.deleted)
	}
	requireSingleAudit(t, records, "user.custom_attribute.deleted", "admin-1", "CustomAttribute", "def-1")
}

func decodePayload(t *testing.T, event *domainoutbox.OutboxEvent) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode outbox payload: %v", err)
	}
	return payload
}

// requireNoPropertyValues guards the PII rule: custom attribute audits carry
// the definition only, never user-supplied property values.
func requireNoPropertyValues(t *testing.T, payload map[string]any) {
	t.Helper()
	for _, forbidden := range []string{"value", "properties", "propertyValues"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("audit payload must not contain %q: %v", forbidden, payload)
		}
	}
}
