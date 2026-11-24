package enforcers

import (
	"database/sql"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
)

func TestRowToDomain(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	row := enforcerRow{
		ID:             "enf-1",
		OrganizationID: "org-1",
		Name:           "rbac-enforcer",
		DisplayName:    "RBAC Enforcer",
		Description:    "Role-based access control",
		ModelID:        sql.NullString{String: "model-1", Valid: true},
		AdapterID:      sql.NullString{String: "adapter-1", Valid: true},
		IsEnabled:      true,
		Owner:          "admin",
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	result := rowToDomain(row)
	if result == nil {
		t.Fatal("rowToDomain() returned nil")
	}
	if result.ID != "enf-1" {
		t.Errorf("ID = %q, want enf-1", result.ID)
	}
	if result.Name != "rbac-enforcer" {
		t.Errorf("Name = %q, want rbac-enforcer", result.Name)
	}
	if result.ModelID != "model-1" {
		t.Errorf("ModelID = %q, want model-1", result.ModelID)
	}
	if !result.IsEnabled {
		t.Error("IsEnabled = false, want true")
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		v, f, want int
	}{
		{0, 10, 10},
		{-1, 5, 5},
		{20, 10, 20},
	}
	for _, tt := range tests {
		if got := max(tt.v, tt.f); got != tt.want {
			t.Errorf("max(%d, %d) = %d, want %d", tt.v, tt.f, got, tt.want)
		}
	}
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	if repo.db != nil {
		t.Error("expected nil db")
	}
}

func TestDomainEnforcerFields(t *testing.T) {
	e := &domain.Enforcer{
		ID:        "test",
		Name:      "test-enforcer",
		IsEnabled: true,
	}
	if e.ID != "test" {
		t.Errorf("ID = %q, want test", e.ID)
	}
}
