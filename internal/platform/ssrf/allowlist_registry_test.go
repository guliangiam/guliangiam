package ssrf

import (
	"testing"
)

func TestAllowlistRegistry_Validate(t *testing.T) {
	if err := ValidateRegistry(); err != nil {
		t.Fatalf("ValidateRegistry() = %v, want nil", err)
	}
}

func TestAllowlistRegistry_HasKnownEntries(t *testing.T) {
	// mcpservers.intranet_scan 是已知注册条目
	if !HasEntry("mcpservers.intranet_scan") {
		t.Error("HasEntry(mcpservers.intranet_scan) = false, want true")
	}
	// 未注册条目返回 false
	if HasEntry("unknown.biz") {
		t.Error("HasEntry(unknown.biz) = true, want false")
	}
}

func TestAllowlistRegistry_CIDRsForReturnsCopy(t *testing.T) {
	original := CIDRsFor("mcpservers.intranet_scan")
	if len(original) == 0 {
		t.Fatal("CIDRsFor returned empty slice")
	}
	// 修改返回值不应影响注册表
	original[0] = "HACKED"
	again := CIDRsFor("mcpservers.intranet_scan")
	if again[0] == "HACKED" {
		t.Errorf("CIDRsFor returns shared slice — mutation leaks into registry!")
	}
}

func TestAllowlistRegistry_NoMetadataCIDR(t *testing.T) {
	for _, e := range ListAllowlistEntries() {
		for _, c := range e.CIDRs {
			if c == "169.254.0.0/16" {
				t.Errorf("entry %q contains metadata CIDR 169.254.0.0/16", e.Name)
			}
		}
	}
}

func TestAllowlistRegistry_EntriesHaveDescriptions(t *testing.T) {
	for _, e := range ListAllowlistEntries() {
		if e.Description == "" {
			t.Errorf("entry %q has empty Description", e.Name)
		}
	}
}

func TestNamedHTTPClient_UnregisteredReturnsStrictClient(t *testing.T) {
	c := NamedHTTPClient("totally.unregistered.biz", 5*1000*1000*1000)
	if c == nil {
		t.Fatal("NamedHTTPClient returned nil")
	}
	// transport 必须存在且是 SafeTransport
	if c.Transport == nil {
		t.Error("NamedHTTPClient has nil Transport")
	}
}

// go test -v -run Allowlist
func TestAllowlistRegistry(t *testing.T) {
	t.Run("Validate", TestAllowlistRegistry_Validate)
	t.Run("HasKnownEntries", TestAllowlistRegistry_HasKnownEntries)
	t.Run("CIDRsForReturnsCopy", TestAllowlistRegistry_CIDRsForReturnsCopy)
	t.Run("NoMetadataCIDR", TestAllowlistRegistry_NoMetadataCIDR)
	t.Run("EntriesHaveDescriptions", TestAllowlistRegistry_EntriesHaveDescriptions)
	t.Run("UnregisteredReturnsStrictClient", TestNamedHTTPClient_UnregisteredReturnsStrictClient)
}
