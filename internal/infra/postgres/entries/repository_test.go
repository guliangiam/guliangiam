package entries

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
)

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	if repo.db != nil {
		t.Error("expected nil db")
	}
}

func TestDomainEntryFields(t *testing.T) {
	e := &domain.Entry{
		ID:             "entry-1",
		OrganizationID: "org-1",
		Name:           "login-event",
		Type:           "auth",
	}
	if e.ID != "entry-1" {
		t.Errorf("ID = %q, want entry-1", e.ID)
	}
}
