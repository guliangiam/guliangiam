package payment

import "testing"

func TestChannelMetadata_MatchesChannelNames(t *testing.T) {
	names := ChannelNames()
	metadata := ChannelMetadata()
	if len(metadata) != len(names) {
		t.Fatalf("metadata entries = %d, want %d", len(metadata), len(names))
	}

	seen := map[string]bool{}
	for i, meta := range metadata {
		if meta.Channel != names[i] {
			t.Errorf("metadata[%d].Channel = %q, want %q", i, meta.Channel, names[i])
		}
		if seen[meta.Channel] {
			t.Errorf("duplicate channel %q", meta.Channel)
		}
		seen[meta.Channel] = true
	}

	// The catalog must be independent of credentials: it exposes code-level
	// capability even for channels that are not configured at runtime.
	for _, name := range names {
		if !seen[name] {
			t.Errorf("channel %q missing from metadata", name)
		}
	}
}

func TestChannelCatalog_WeChatPayIsCanonical(t *testing.T) {
	for _, name := range ChannelNames() {
		if name == "wxpay" || name == "wechat" {
			t.Fatalf("channel catalog still exposes legacy key %q", name)
		}
	}
	meta := NewChannelAdapter("wechat_pay", nil).Metadata()
	if !meta.SupportsRefund {
		t.Error("wechat_pay should advertise refund support")
	}
}
