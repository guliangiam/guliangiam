package usernotifications

import (
	"testing"

	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
)

func TestMergeChannelTargetsPrefersExplicitTargets(t *testing.T) {
	merged := mergeChannelTargets(
		[]ChannelTarget{{Channel: "email", Recipient: "explicit@example.com"}},
		Contacts{
			Email: "profile@example.com",
			Phone: "+8613800000000",
			Bindings: map[string]string{
				"wecom_app":       "zhangsan",
				"wechat_official": "openid-1",
				"email":           "dup@example.com",
			},
		},
	)

	byChannel := make(map[string]string, len(merged))
	for _, target := range merged {
		if byChannel[target.Channel] != "" {
			t.Fatalf("duplicate channel %q in merged targets", target.Channel)
		}
		byChannel[target.Channel] = target.Recipient
	}

	if byChannel["email"] != "explicit@example.com" {
		t.Fatalf("explicit email target must win, got %q", byChannel["email"])
	}
	if byChannel["sms"] != "+8613800000000" {
		t.Fatalf("sms target = %q", byChannel["sms"])
	}
	if byChannel["wecom_app"] != "zhangsan" || byChannel["wechat_official"] != "openid-1" {
		t.Fatalf("binding targets missing: %#v", byChannel)
	}
	if len(merged) != 4 {
		t.Fatalf("merged targets = %d, want 4", len(merged))
	}
}

func TestMergeChannelTargetsSkipsEmptyValues(t *testing.T) {
	merged := mergeChannelTargets(nil, Contacts{Email: "  ", Phone: ""})
	if len(merged) != 0 {
		t.Fatalf("empty contacts produced targets: %#v", merged)
	}
}

func TestPickProviderForChannelMatchesExactProviderType(t *testing.T) {
	providers := []*domainnotificationproviders.NotificationProvider{
		{ID: "mail-1", ProviderType: "email"},
		{ID: "sms-1", ProviderType: "sms"},
		{ID: "wecom-robot", ProviderType: "wecom"},
		{ID: "wecom-app", ProviderType: "wecom_app"},
		{ID: "wechat-mp", ProviderType: "wechat_official"},
	}

	cases := map[string]string{
		"email":           "mail-1",
		"sms":             "sms-1",
		"wecom_app":       "wecom-app",
		"wechat_official": "wechat-mp",
		// 未知/未启用的渠道不匹配任何提供商。
		"unknown_channel": "",
	}
	for channel, wantID := range cases {
		selected := pickProviderForChannel(providers, channel)
		if wantID == "" {
			if selected != nil {
				t.Fatalf("channel %q selected %q, want none", channel, selected.ID)
			}
			continue
		}
		if selected == nil || selected.ID != wantID {
			t.Fatalf("channel %q selected %#v, want %q", channel, selected, wantID)
		}
	}
}
