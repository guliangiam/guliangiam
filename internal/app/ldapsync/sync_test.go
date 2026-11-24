package ldapsync

import (
	"testing"
)

func TestDefaultAttributeMapping_ActiveDirectory(t *testing.T) {
	mapping := defaultAttributeMapping("ActiveDirectory")
	if mapping.Username != "sAMAccountName" {
		t.Errorf("AD Username = %q, want sAMAccountName", mapping.Username)
	}
	if mapping.DisplayName != "displayName" {
		t.Errorf("AD DisplayName = %q, want displayName", mapping.DisplayName)
	}
	if mapping.Email != "mail" {
		t.Errorf("AD Email = %q, want mail", mapping.Email)
	}
}

func TestDefaultAttributeMapping_OpenLDAP(t *testing.T) {
	mapping := defaultAttributeMapping("OpenLDAP")
	if mapping.Username != "uid" {
		t.Errorf("OpenLDAP Username = %q, want uid", mapping.Username)
	}
	if mapping.DisplayName != "cn" {
		t.Errorf("OpenLDAP DisplayName = %q, want cn", mapping.DisplayName)
	}
	if mapping.Email != "mail" {
		t.Errorf("OpenLDAP Email = %q, want mail", mapping.Email)
	}
}

func TestDefaultAttributeMapping_CaseInsensitive(t *testing.T) {
	mapping := defaultAttributeMapping("activedirectory")
	if mapping.Username != "sAMAccountName" {
		t.Errorf("case-insensitive AD Username = %q, want sAMAccountName", mapping.Username)
	}
}

func TestDefaultAttributeMapping_UnknownType(t *testing.T) {
	mapping := defaultAttributeMapping("FreeIPA")
	if mapping.Username != "uid" {
		t.Errorf("unknown type Username = %q, want uid (OpenLDAP default)", mapping.Username)
	}
}
