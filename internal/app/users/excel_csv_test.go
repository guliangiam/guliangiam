package users

import (
	"testing"
)

func TestSafeGet(t *testing.T) {
	row := []string{"a", "b", "c"}
	if v := safeGet(row, 0); v != "a" {
		t.Errorf("got %q, want a", v)
	}
	if v := safeGet(row, 2); v != "c" {
		t.Errorf("got %q, want c", v)
	}
	if v := safeGet(row, 3); v != "" {
		t.Errorf("got %q, want empty for out of range", v)
	}
	if v := safeGet([]string{}, 0); v != "" {
		t.Errorf("got %q, want empty for empty row", v)
	}
}

func TestResolveCSVField(t *testing.T) {
	rowMap := map[string]string{"name": "Alice", "email": "alice@example.com"}
	mapping := map[string]string{"Full Name": "name", "Email Address": "email"}

	if v := resolveCSVField(rowMap, mapping, "name"); v != "Alice" {
		t.Errorf("got %q, want Alice", v)
	}
	if v := resolveCSVField(rowMap, mapping, "email"); v != "alice@example.com" {
		t.Errorf("got %q, want alice@example.com", v)
	}
	if v := resolveCSVField(rowMap, mapping, "unknown"); v != "" {
		t.Errorf("got %q, want empty", v)
	}

	rowMap2 := map[string]string{"username": "bob"}
	if v := resolveCSVField(rowMap2, nil, "username"); v != "bob" {
		t.Errorf("got %q, want bob (fallback)", v)
	}
}
