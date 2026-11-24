package adapters

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
)

func TestRowToDomain(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	port := 5432
	// DB-058: the confidential columns are persisted as ciphertext.
	encPassword, err := adapterEncryptedMapper.EncryptColumn(adapterAAD("adapter-1", adapterColumnPassword), "secret")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	encDbPassword, err := adapterEncryptedMapper.EncryptColumn(adapterAAD("adapter-1", adapterColumnDbPassword), "casbin-pass")
	if err != nil {
		t.Fatalf("encrypt db_password: %v", err)
	}
	row := adapterRow{
		ID:             "adapter-1",
		OrganizationID: "org-1",
		Name:           "casbin-adapter",
		DisplayName:    "Casbin Adapter",
		Description:    "Policy storage adapter",
		Type:           "casbin",
		Server:         "localhost",
		Database:       "guliangiam",
		TableName:      "casbin_rule",
		Username:       "admin",
		Password:       encPassword,
		Owner:          "system",
		UseSameDb:      true,
		DatabaseType:   "postgres",
		Host:           "db.example.com",
		Port:           &port,
		DbUser:         "casbin",
		DbPassword:     encDbPassword,
		DatabaseName:   "casbin_db",
		Policy:         "rbac",
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	result, err := rowToDomain(row)
	if err != nil {
		t.Fatalf("rowToDomain() error: %v", err)
	}
	if result == nil {
		t.Fatal("rowToDomain() returned nil")
	}
	if result.Password != "secret" {
		t.Errorf("Password = %q, want secret", result.Password)
	}
	if result.DbPassword != "casbin-pass" {
		t.Errorf("DbPassword = %q, want casbin-pass", result.DbPassword)
	}
	if result.ID != "adapter-1" {
		t.Errorf("ID = %q, want adapter-1", result.ID)
	}
	if result.Name != "casbin-adapter" {
		t.Errorf("Name = %q, want casbin-adapter", result.Name)
	}
	if result.Type != "casbin" {
		t.Errorf("Type = %q, want casbin", result.Type)
	}
	if !result.IsEnabled {
		t.Error("IsEnabled = false, want true")
	}
	if result.Host != "db.example.com" {
		t.Errorf("Host = %q, want db.example.com", result.Host)
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		v, f, want int
	}{
		{0, 10, 10},
		{-1, 5, 5},
		{20, 10, 20},
		{1, 0, 1},
	}
	for _, tt := range tests {
		if got := maxInt(tt.v, tt.f); got != tt.want {
			t.Errorf("maxInt(%d, %d) = %d, want %d", tt.v, tt.f, got, tt.want)
		}
	}
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	if repo.db != nil {
		t.Error("expected nil db")
	}
}

// Ensure domain.Adapter fields are accessible
func TestDomainAdapterFields(t *testing.T) {
	a := &domain.Adapter{
		ID:             "test",
		OrganizationID: "org-1",
		Name:           "test-adapter",
		Type:           "casbin",
		IsEnabled:      true,
	}
	if a.ID != "test" {
		t.Errorf("ID = %q, want test", a.ID)
	}
	if !a.IsEnabled {
		t.Error("IsEnabled should be true")
	}
}
