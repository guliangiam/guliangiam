package scim

import (
	"testing"
)

func TestNewScimUser(t *testing.T) {
	user, err := NewScimUser("org-1", "alice")
	if err != nil {
		t.Fatalf("NewScimUser() error = %v", err)
	}
	if user.ID == "" {
		t.Error("ID should not be empty")
	}
	if user.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", user.OrganizationID, "org-1")
	}
	if user.UserName != "alice" {
		t.Errorf("UserName = %q, want %q", user.UserName, "alice")
	}
	if user.Active == nil || *user.Active != true {
		t.Error("Active should be true")
	}
	if user.Meta == nil {
		t.Error("Meta should not be nil")
	}
	if user.Meta.ResourceType != "User" {
		t.Errorf("Meta.ResourceType = %q, want %q", user.Meta.ResourceType, "User")
	}
	if user.Revision != 1 {
		t.Errorf("Revision = %d, want 1", user.Revision)
	}
}

func TestNewScimGroup(t *testing.T) {
	group, err := NewScimGroup("org-1", "Engineering")
	if err != nil {
		t.Fatalf("NewScimGroup() error = %v", err)
	}
	if group.ID == "" {
		t.Error("ID should not be empty")
	}
	if group.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", group.OrganizationID, "org-1")
	}
	if group.DisplayName != "Engineering" {
		t.Errorf("DisplayName = %q, want %q", group.DisplayName, "Engineering")
	}
	if group.Meta == nil {
		t.Error("Meta should not be nil")
	}
	if group.Meta.ResourceType != "Group" {
		t.Errorf("Meta.ResourceType = %q, want %q", group.Meta.ResourceType, "Group")
	}
	if len(group.Members) != 0 {
		t.Errorf("len(Members) = %d, want 0", len(group.Members))
	}
	if group.Revision != 1 {
		t.Errorf("Revision = %d, want 1", group.Revision)
	}
}

func TestBoolPtr(t *testing.T) {
	ptr := boolPtr(true)
	if ptr == nil || *ptr != true {
		t.Error("boolPtr(true) should return *bool with value true")
	}
	ptr2 := boolPtr(false)
	if ptr2 == nil || *ptr2 != false {
		t.Error("boolPtr(false) should return *bool with value false")
	}
}

func TestScimUser_Fields(t *testing.T) {
	active := true
	user := ScimUser{
		ID:             "user-1",
		OrganizationID: "org-1",
		UserName:       "bob",
		Name:           &Name{GivenName: "Bob", FamilyName: "Smith", Formatted: "Bob Smith"},
		DisplayName:    "Bob Smith",
		Active:         &active,
		Emails:         []Email{{Value: "bob@example.com", Type: "work", Primary: true}},
		PhoneNumbers:   []PhoneNumber{{Value: "+1234567890", Type: "mobile"}},
		Groups:         []GroupRef{{Value: "group-1", Display: "Engineering"}},
	}
	if user.UserName != "bob" {
		t.Errorf("UserName = %q, want %q", user.UserName, "bob")
	}
	if user.Name == nil || user.Name.GivenName != "Bob" {
		t.Error("Name.GivenName should be Bob")
	}
	if len(user.Emails) != 1 {
		t.Errorf("len(Emails) = %d, want 1", len(user.Emails))
	}
	if !user.Emails[0].Primary {
		t.Error("Emails[0].Primary should be true")
	}
}

func TestScimGroup_Fields(t *testing.T) {
	group := ScimGroup{
		ID:             "group-1",
		OrganizationID: "org-1",
		DisplayName:    "Engineering",
		Members: []Member{
			{Value: "user-1", Display: "Alice", Type: "User"},
		},
	}
	if group.DisplayName != "Engineering" {
		t.Errorf("DisplayName = %q, want %q", group.DisplayName, "Engineering")
	}
	if len(group.Members) != 1 {
		t.Errorf("len(Members) = %d, want 1", len(group.Members))
	}
}

func TestAttribute_Fields(t *testing.T) {
	attr := Attribute{
		Name:       "userName",
		Type:       "string",
		Required:   true,
		Mutability: "readWrite",
		Returned:   "always",
		Uniqueness: "server",
	}
	if attr.Name != "userName" {
		t.Errorf("Name = %q, want %q", attr.Name, "userName")
	}
	if !attr.Required {
		t.Error("Required should be true")
	}
}

func TestPatchOperation_Fields(t *testing.T) {
	op := PatchOperation{Op: "replace", Path: "userName", Value: "newname"}
	if op.Op != "replace" {
		t.Errorf("Op = %q, want %q", op.Op, "replace")
	}
}

func TestSchemaConstants(t *testing.T) {
	if SchemaUser != "urn:ietf:params:scim:schemas:core:2.0:User" {
		t.Errorf("SchemaUser = %q, unexpected value", SchemaUser)
	}
	if SchemaGroup != "urn:ietf:params:scim:schemas:core:2.0:Group" {
		t.Errorf("SchemaGroup = %q, unexpected value", SchemaGroup)
	}
	if SchemaEnterpriseUser != "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User" {
		t.Errorf("SchemaEnterpriseUser = %q, unexpected value", SchemaEnterpriseUser)
	}
}
