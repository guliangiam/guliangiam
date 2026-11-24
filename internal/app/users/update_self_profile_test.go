package users

import (
	"context"
	"errors"
	"testing"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func selfProfileErrorCode(err error) apperrors.Code {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

// selfProfileTestUser is the stored aggregate the merge tests overlay onto.
// Contact values are synthetic; their verified flags must survive every edit.
func selfProfileTestUser() *userdomain.User {
	return &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "member",
		Revision:       3,
		Profile: userdomain.UserProfile{
			UserID:        "user-1",
			DisplayName:   "Old Name",
			FirstName:     "Old",
			LastName:      "Name",
			Bio:           "keep me",
			Location:      "Keep City",
			Email:         "member@example.com",
			EmailVerified: true,
			Phone:         "+8613800000000",
			PhoneVerified: true,
			Affiliation:   "Old Org",
			Properties: map[string]any{
				"affiliation": "Old Org",
				"homepage":    "https://old.example.com",
				"education":   "bachelor",
				"language":    "zh-CN",
				"idCardFront": "https://cdn.example.com/front.png",
				"custom":      "keep",
			},
		},
	}
}

func strPtr(value string) *string { return &value }

// selfProfileHarness wires the service with capture fakes for the profile
// write, the user write and the audit/outbox pair.
type selfProfileHarness struct {
	svc          Service
	savedUser    *userdomain.User
	savedProfile *userdomain.UserProfile
	records      *fakeRecordsRepo
	outbox       *captureOutbox
}

func newSelfProfileHarness(user *userdomain.User, getErr error) *selfProfileHarness {
	h := &selfProfileHarness{records: &fakeRecordsRepo{}, outbox: &captureOutbox{}}
	userRepo := &svcMockUserRepo{
		getByIDForOrganizationFn: func(_ context.Context, id, organizationID string) (*userdomain.User, error) {
			if getErr != nil {
				return nil, getErr
			}
			if id != user.ID || organizationID != user.OrganizationID {
				return nil, apperrors.NewNotFound("user not found")
			}
			return user, nil
		},
		updateForOrganizationFn: func(_ context.Context, saved *userdomain.User, _ string) error {
			h.savedUser = saved
			return nil
		},
		updateProfileFn: func(_ context.Context, profile *userdomain.UserProfile) error {
			h.savedProfile = profile
			return nil
		},
	}
	h.svc = Service{
		Clock: svcFakeClock{fixedTime: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)},
		UOW: customAttributeTestUnitOfWork{
			svcTestUnitOfWork: svcTestUnitOfWork{users: userRepo, records: h.records},
			outbox:            h.outbox,
		},
	}
	return h
}

// TestUpdateSelfProfile_MergesOnlySuppliedFields: nil pointers keep the stored
// values; email/phone and their verified flags are never touched.
func TestUpdateSelfProfile_MergesOnlySuppliedFields(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		DisplayName:    strPtr("New Name"),
		Bio:            strPtr("new bio"),
	})
	if err != nil {
		t.Fatalf("UpdateSelfProfile() error = %v", err)
	}
	if h.savedProfile == nil {
		t.Fatal("UpdateProfile was not called")
	}
	p := h.savedProfile
	if p.DisplayName != "New Name" || p.Bio != "new bio" {
		t.Fatalf("supplied fields not applied: %+v", p)
	}
	if p.FirstName != "Old" || p.LastName != "Name" || p.Location != "Keep City" {
		t.Fatalf("untouched fields were wiped: %+v", p)
	}
	if p.Email != "member@example.com" || !p.EmailVerified {
		t.Fatalf("email/verification must stay untouched: %+v", p)
	}
	if p.Phone != "+8613800000000" || !p.PhoneVerified {
		t.Fatalf("phone/verification must stay untouched: %+v", p)
	}
	if p.Properties["homepage"] != "https://old.example.com" || p.Properties["custom"] != "keep" {
		t.Fatalf("stored properties were wiped: %+v", p.Properties)
	}
	if p.Affiliation != "Old Org" {
		t.Fatalf("affiliation must survive an unrelated edit: %q", p.Affiliation)
	}
}

// TestUpdateSelfProfile_PropertiesMergeAndDelete: overlay keys win, explicit
// empty string / null delete the key, folded fields follow pointer semantics.
func TestUpdateSelfProfile_PropertiesMergeAndDelete(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		Title:          strPtr("Engineer"),
		Language:       strPtr(""), // explicit empty deletes the stored language
		IDCardFront:    strPtr(""), // explicit empty deletes the stored image key
		Affiliation:    strPtr("New Org"),
		Properties: map[string]any{
			"homepage":  "",       // delete
			"custom":    nil,      // null deletes
			"extra":     "kept",   // new key
			"education": "master", // overlay wins
		},
	})
	if err != nil {
		t.Fatalf("UpdateSelfProfile() error = %v", err)
	}
	props := h.savedProfile.Properties
	for _, deleted := range []string{"homepage", "language", "idCardFront", "custom"} {
		if _, ok := props[deleted]; ok {
			t.Fatalf("key %q must be deleted, got %+v", deleted, props)
		}
	}
	if props["extra"] != "kept" || props["education"] != "master" || props["title"] != "Engineer" {
		t.Fatalf("merged properties wrong: %+v", props)
	}
	if props["affiliation"] != "New Org" || h.savedProfile.Affiliation != "New Org" {
		t.Fatalf("affiliation column must follow the merged key: %+v", props)
	}
}

// TestUpdateSelfProfile_AffiliationDeleteClearsColumn: deleting the folded key
// also clears the derived column (与管理员 PUT 同口径).
func TestUpdateSelfProfile_AffiliationDeleteClearsColumn(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		Affiliation:    strPtr(""),
	})
	if err != nil {
		t.Fatalf("UpdateSelfProfile() error = %v", err)
	}
	if _, ok := h.savedProfile.Properties["affiliation"]; ok {
		t.Fatalf("affiliation key must be deleted: %+v", h.savedProfile.Properties)
	}
	if h.savedProfile.Affiliation != "" {
		t.Fatalf("affiliation column must be cleared, got %q", h.savedProfile.Affiliation)
	}
}

// TestUpdateSelfProfile_CrossTenantRejected: the scoped lookup fails and no
// write happens.
func TestUpdateSelfProfile_CrossTenantRejected(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-foreign",
		UserID:         "user-1",
		DisplayName:    strPtr("x"),
	})
	if selfProfileErrorCode(err) != apperrors.CodeNotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
	if h.savedProfile != nil || h.savedUser != nil {
		t.Fatal("cross-tenant request must not write")
	}
	if len(h.records.created) != 0 {
		t.Fatal("cross-tenant request must not audit a write")
	}
}

// TestUpdateSelfProfile_RevisionAndTimestamp: revision increments and
// UpdatedAt comes from the injected clock.
func TestUpdateSelfProfile_RevisionAndTimestamp(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		DisplayName:    strPtr("New Name"),
	})
	if err != nil {
		t.Fatalf("UpdateSelfProfile() error = %v", err)
	}
	if h.savedUser == nil {
		t.Fatal("UpdateForOrganization was not called")
	}
	if h.savedUser.Revision != 4 {
		t.Fatalf("revision = %d, want 4", h.savedUser.Revision)
	}
	if !h.savedUser.UpdatedAt.Equal(time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("UpdatedAt = %v, want the injected clock value", h.savedUser.UpdatedAt)
	}
}

// TestUpdateSelfProfile_AuditSelfServiceMarker: one user.updated audit whose
// outbox payload carries only the selfService marker — never PII plaintext.
func TestUpdateSelfProfile_AuditSelfServiceMarker(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		RealName:       strPtr("Synthetic Test Name"),
		IDCard:         strPtr("TEST-DOC-0001"),
	})
	if err != nil {
		t.Fatalf("UpdateSelfProfile() error = %v", err)
	}
	if len(h.records.created) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(h.records.created))
	}
	record := h.records.created[0]
	if record.Action != "user.updated" || record.ResourceID != "user-1" || record.ActorID != "user-1" {
		t.Fatalf("audit record wrong: %+v", record)
	}
	if len(h.outbox.events) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(h.outbox.events))
	}
	event := h.outbox.events[0]
	if event.EventType != "user.updated" {
		t.Fatalf("outbox event type = %q, want user.updated", event.EventType)
	}
	payload := decodePayload(t, event)
	if payload["selfService"] != true {
		t.Fatalf("audit payload must mark selfService: %v", payload)
	}
	for _, forbidden := range []string{"realName", "idCard", "email", "phone", "properties", "displayName"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("audit payload must not contain PII key %q: %v", forbidden, payload)
		}
	}
}

// TestUpdateSelfProfile_ValidationNotRelaxed: the merged profile is validated
// with the admin command's strictness (homepage must be an absolute URL).
func TestUpdateSelfProfile_ValidationNotRelaxed(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		Homepage:       strPtr("not-a-url"),
	})
	if err == nil {
		t.Fatal("invalid homepage must be rejected")
	}
	if h.savedProfile != nil {
		t.Fatal("invalid merge must not write")
	}
}

// TestUpdateSelfProfile_RequiresOrganization: an empty tenant fails fast before
// any repository work.
func TestUpdateSelfProfile_RequiresOrganization(t *testing.T) {
	h := newSelfProfileHarness(selfProfileTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		UserID:      "user-1",
		DisplayName: strPtr("x"),
	})
	if selfProfileErrorCode(err) != apperrors.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if h.savedProfile != nil {
		t.Fatal("missing tenant must not write")
	}
}
