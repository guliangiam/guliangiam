package uuid

import (
	"testing"
)

func TestV7Generator_NewString(t *testing.T) {
	gen := V7Generator{}
	id, err := gen.NewString()
	if err != nil {
		t.Fatalf("NewString() error = %v", err)
	}
	if id == "" {
		t.Fatal("NewString() returned empty string")
	}
	if len(id) != 36 {
		t.Errorf("UUID length = %d, want 36", len(id))
	}
}

func TestV7Generator_NewString_Unique(t *testing.T) {
	gen := V7Generator{}
	id1, err := gen.NewString()
	if err != nil {
		t.Fatalf("NewString() error = %v", err)
	}
	id2, err := gen.NewString()
	if err != nil {
		t.Fatalf("NewString() error = %v", err)
	}
	if id1 == id2 {
		t.Error("two generated UUIDs should be unique")
	}
}

func TestV7Generator_NewString_Format(t *testing.T) {
	gen := V7Generator{}
	id, err := gen.NewString()
	if err != nil {
		t.Fatalf("NewString() error = %v", err)
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Errorf("UUID %q does not follow standard format", id)
	}
}

func TestV7Generator_NewString_Version7(t *testing.T) {
	gen := V7Generator{}
	id, err := gen.NewString()
	if err != nil {
		t.Fatalf("NewString() error = %v", err)
	}
	versionChar := id[14]
	if versionChar != '7' {
		t.Errorf("UUID version = %c, want '7'", versionChar)
	}
}

func TestV7Generator_ImplementsInterface(t *testing.T) {
	var _ Generator = V7Generator{}
}
