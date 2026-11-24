package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

// TestAlibabaSMSSignatureMatchesOfficialVector uses the worked example from
// https://help.aliyun.com/zh/sdk/product-overview/rpc-mechanism (V2 RPC
// HMAC-SHA1): AccessKeySecret=testsecret produces 9NaGiOspFP5UPcwX8Iwt2YJXXuk=.
func TestAlibabaSMSSignatureMatchesOfficialVector(t *testing.T) {
	params := map[string]string{
		"AccessKeyId":      "testid",
		"Action":           "DescribeDedicatedHosts",
		"Format":           "JSON",
		"RegionId":         "cn-beijing",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   "edb2b34af0af9a6d14deaf7c1a5315eb",
		"SignatureVersion": "1.0",
		"Timestamp":        "2023-03-13T08:34:30Z",
		"Version":          "2014-05-26",
	}
	got := alibabaSMSSignature(http.MethodGet, params, "testsecret")
	want := "9NaGiOspFP5UPcwX8Iwt2YJXXuk="
	if got != want {
		t.Fatalf("signature mismatch: got %q want %q", got, want)
	}
}

func TestAlibabaSMSPercentEncode(t *testing.T) {
	cases := map[string]string{
		" ":     "%20",
		"*":     "%2A",
		"~":     "~",
		"a=b&c": "a%3Db%26c",
	}
	for input, want := range cases {
		if got := alibabaSMSPercentEncode(input); got != want {
			t.Fatalf("percentEncode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProbeAlibabaSMSAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Code":    "InvalidAccessKeyId.NotFound",
			"Message": "The specified access key ID does not exist.",
		})
	}))
	defer server.Close()

	provider := &domain.Provider{
		Category:     "SMS",
		ProviderType: "AlibabaSMS",
		Admin:        "invalid-access-key-id",
		Password:     "invalid-access-key-secret",
		Sender:       "Guliang",
		Template:     "SMS_000000001",
	}
	result, err := probeAlibabaSMS(context.Background(), server.Client(), server.URL, provider)
	if err != nil {
		t.Fatalf("probe returned error: %v", err)
	}
	if result.Connected {
		t.Fatalf("invalid credentials must not be reported as connected")
	}
	if !strings.Contains(result.Error, "InvalidAccessKeyId.NotFound") {
		t.Fatalf("expected mapped auth error, got %q", result.Error)
	}
}

func TestProbeAlibabaSMSApprovedSignAndTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("Action") {
		case "GetSmsSign":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Code":       "OK",
				"SignName":   "Guliang",
				"SignStatus": 1,
			})
		case "GetSmsTemplate":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Code":           "OK",
				"TemplateCode":   "SMS_000000001",
				"TemplateStatus": 1,
			})
		default:
			t.Errorf("unexpected action %q", r.URL.Query().Get("Action"))
		}
	}))
	defer server.Close()

	provider := &domain.Provider{
		Category:     "SMS",
		ProviderType: "AlibabaSMS",
		Admin:        "access-key-id",
		Password:     "access-key-secret",
		Sender:       "Guliang",
		Template:     "SMS_000000001",
	}
	result, err := probeAlibabaSMS(context.Background(), server.Client(), server.URL, provider)
	if err != nil {
		t.Fatalf("probe returned error: %v", err)
	}
	if !result.Connected {
		t.Fatalf("expected connected=true, got error %q", result.Error)
	}
}

func TestProbeAlibabaSMSRejectsUnapprovedSign(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Code":       "OK",
			"SignName":   "Guliang",
			"SignStatus": 2,
		})
	}))
	defer server.Close()

	provider := &domain.Provider{
		Category:     "SMS",
		ProviderType: "AlibabaSMS",
		Admin:        "access-key-id",
		Password:     "access-key-secret",
		Sender:       "Guliang",
		Template:     "SMS_000000001",
	}
	result, err := probeAlibabaSMS(context.Background(), server.Client(), server.URL, provider)
	if err != nil {
		t.Fatalf("probe returned error: %v", err)
	}
	if result.Connected {
		t.Fatalf("unapproved sign must not be reported as connected")
	}
	if !strings.Contains(result.Error, "not approved") {
		t.Fatalf("expected approval error, got %q", result.Error)
	}
}

func TestValidateSMSProviderConfig(t *testing.T) {
	alibaba := &domain.Provider{
		Category:     "SMS",
		ProviderType: "AlibabaSMS",
		Admin:        "ak",
		Password:     "sk",
		Sender:       "sign",
		Template:     "SMS_1",
	}
	if problem := validateSMSProviderConfig(alibaba); problem != "" {
		t.Fatalf("complete alibaba config rejected: %s", problem)
	}

	incomplete := &domain.Provider{Category: "SMS", ProviderType: "AlibabaSMS", Admin: "ak"}
	if problem := validateSMSProviderConfig(incomplete); !strings.Contains(problem, "accessKeySecret") {
		t.Fatalf("expected missing accessKeySecret, got %q", problem)
	}

	mock := &domain.Provider{Category: "SMS", ProviderType: "MockSMS"}
	if problem := validateSMSProviderConfig(mock); problem != "" {
		t.Fatalf("mock provider should not require credentials: %s", problem)
	}

	custom := &domain.Provider{Category: "SMS", ProviderType: "CustomHTTP", Host: "https://sms.example.com/send"}
	if problem := validateSMSProviderConfig(custom); problem != "" {
		t.Fatalf("complete custom HTTP config rejected: %s", problem)
	}
}
