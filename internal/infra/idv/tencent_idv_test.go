package idv

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

func allowLoopbackTencent(verifier *TencentIDVVerifier) *TencentIDVVerifier {
	verifier.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return verifier
}

func newTestTencentVerifier(endpoint string) *TencentIDVVerifier {
	return allowLoopbackTencent(NewTencentIDVVerifierWithOptions("AKIDtest", "sktest", "", endpoint, "rule-1", ""))
}

// verifyTencentTC3 recomputes the signature v3 Authorization from the received
// request exactly as a Tencent server would, so the contract test proves the
// header is consistent with the official algorithm rather than just present.
func verifyTencentTC3(t *testing.T, r *http.Request, body []byte, secretID, secretKey, service string) {
	t.Helper()
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "TC3-HMAC-SHA256 Credential="+secretID+"/") {
		t.Fatalf("unexpected authorization %q", auth)
	}
	signed := strings.Split(auth, "Signature=")
	if len(signed) != 2 {
		t.Fatalf("authorization missing signature part: %q", auth)
	}
	timestamp := r.Header.Get("X-TC-Timestamp")
	if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
		t.Fatalf("invalid X-TC-Timestamp %q", timestamp)
	}
	host := r.Host
	action := r.Header.Get("X-TC-Action")
	expected := tencentTC3Authorization(service, host, action, r.Header.Get("Content-Type"), mustParseInt(timestamp), body, secretID, secretKey)
	if auth != expected {
		t.Errorf("signature mismatch\n got %q\nwant %q", auth, expected)
	}
	// The credential scope date must be the UTC date of the timestamp.
	date := time.Unix(mustParseInt(timestamp), 0).UTC().Format("2006-01-02")
	if !strings.Contains(auth, "/"+date+"/"+service+"/tc3_request") {
		t.Errorf("credential scope must carry the UTC date %q: %q", date, auth)
	}
}

func mustParseInt(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func TestTencent_InitiateDetectAuth(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-TC-Action"); got != "DetectAuth" {
			t.Errorf("X-TC-Action = %q", got)
		}
		if got := r.Header.Get("X-TC-Version"); got != "2018-03-01" {
			t.Errorf("X-TC-Version = %q", got)
		}
		if got := r.Header.Get("X-TC-Region"); got != "" {
			t.Errorf("faceid is a region-less product, X-TC-Region = %q", got)
		}
		gotBody, _ = io.ReadAll(r.Body)
		verifyTencentTC3(t, r, gotBody, "AKIDtest", "sktest", "faceid")
		_, _ = io.WriteString(w, `{"Response":{"BizToken":"biz-1","Url":"https://faceid.test/h5?nonce=x","RequestId":"req-1"}}`)
	}))
	defer server.Close()

	verifier := newTestTencentVerifier(server.URL)
	result, err := verifier.InitiateVerification(context.Background(), InitiateOptions{
		UserID: "u-1", FullName: "张三", IDNumber: "110101199001011234", RedirectURL: "https://iam.test/return",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.VerificationID != "biz-1" || result.RedirectURL != "https://faceid.test/h5?nonce=x" || result.SessionToken != "biz-1" {
		t.Errorf("unexpected result: %+v", result)
	}
	body := string(gotBody)
	for _, want := range []string{`"RuleId":"rule-1"`, `"Name":"张三"`, `"IdCard":"110101199001011234"`, `"RedirectUrl":"https://iam.test/return"`, `"Extra":"u-1"`} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %s: %s", want, body)
		}
	}
}

func TestTencent_InitiateRequiresRuleID(t *testing.T) {
	verifier := newTestTencentVerifier("http://127.0.0.1:1")
	verifier.RuleID = ""
	_, err := verifier.InitiateVerification(context.Background(), InitiateOptions{UserID: "u-1"})
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid argument for missing ruleId, got %v", err)
	}
}

func TestTencent_GetVerificationResultMapsErrCodes(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		want    VerificationStatus
		wantKey string
	}{
		{"approved", `{"ErrCode":0,"ErrMsg":"成功","Sim":"98.5","Name":"张三","IdCard":"110101199001011234"}`, StatusApproved, ""},
		{"rejected", `{"ErrCode":2006,"ErrMsg":"姓名和身份证号不匹配"}`, StatusRejected, "姓名和身份证号不匹配"},
		{"expired", `{"ErrCode":15,"ErrMsg":"Token 过期，请重试"}`, StatusExpired, ""},
		{"pending-flow-incomplete", `{"ErrCode":17,"ErrMsg":"本次流程未完成"}`, StatusPending, ""},
		{"pending-empty-text", "", StatusPending, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				_, _ = io.WriteString(w, `{"Response":{"Text":`+strconv.Quote(tc.text)+`,"RequestId":"req-1"}}`)
			}))
			defer server.Close()

			verifier := newTestTencentVerifier(server.URL)
			result, err := verifier.GetVerificationResult(context.Background(), "biz-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status != tc.want {
				t.Errorf("status = %s, want %s", result.Status, tc.want)
			}
			if tc.wantKey != "" && !strings.Contains(result.Reason, tc.wantKey) {
				t.Errorf("reason = %q, want to contain %q", result.Reason, tc.wantKey)
			}
			if result.ProviderName != tencentIDVProviderName || result.VerificationID != "biz-1" {
				t.Errorf("unexpected result: %+v", result)
			}
			body := string(gotBody)
			if !strings.Contains(body, `"BizToken":"biz-1"`) || !strings.Contains(body, `"RuleId":"rule-1"`) {
				t.Errorf("request body missing BizToken/RuleId: %s", body)
			}
		})
	}
}

func TestTencent_BusinessErrorMapping(t *testing.T) {
	cases := map[string]apperrors.Code{
		`{"Response":{"Error":{"Code":"AuthFailure.SignatureFailure","Message":"The provided credentials resulted in an authentication error."},"RequestId":"r"}}`: apperrors.CodeUnauthorized,
		`{"Response":{"Error":{"Code":"UnauthorizedOperation.Nonactivated","Message":"服务未开通"},"RequestId":"r"}}`:                                                   apperrors.CodeForbidden,
		`{"Response":{"Error":{"Code":"InvalidParameterValue.BizTokenExpired","Message":"token expired"},"RequestId":"r"}}`:                                        apperrors.CodeInvalidArgument,
		`{"Response":{"Error":{"Code":"FailedOperation.UnKnowError","Message":"内部错误"},"RequestId":"r"}}`:                                                           apperrors.CodeUnavailable,
		`{"Response":{"Error":{"Code":"LimitExceeded.FreeRateLimit","Message":"qps exceeded"},"RequestId":"r"}}`:                                                   apperrors.CodeTooManyRequests,
	}
	for payload, want := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, payload)
		}))
		verifier := newTestTencentVerifier(server.URL)
		_, err := verifier.GetVerificationResult(context.Background(), "biz-1")
		server.Close()
		appErr, ok := err.(*apperrors.Error)
		if !ok || appErr.Code != want {
			t.Fatalf("payload %s: expected code %v, got %v", payload, want, err)
		}
	}
}

func TestTencent_CallbackSignature(t *testing.T) {
	verifier := &TencentIDVVerifier{WebhookSecret: "whsec"}
	payload := []byte(`{"BizToken":"biz-1","ErrCode":0,"ErrMsg":"成功"}`)

	if _, err := verifier.VerifyCallback(context.Background(), payload, nil); err == nil {
		t.Error("expected missing-signature rejection")
	}
	if _, err := verifier.VerifyCallback(context.Background(), payload, map[string]string{"X-Tc-Signature": "deadbeef"}); err == nil {
		t.Error("expected invalid signature rejection")
	}

	mac := hmac.New(sha256.New, []byte("whsec"))
	mac.Write(payload)
	result, err := verifier.VerifyCallback(context.Background(), payload, map[string]string{
		"X-Tc-Signature": hex.EncodeToString(mac.Sum(nil)),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApproved || result.VerificationID != "biz-1" || result.ProviderName != tencentIDVProviderName {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTencent_CallbackWithoutSecretIsRejected(t *testing.T) {
	verifier := &TencentIDVVerifier{}
	_, err := verifier.VerifyCallback(context.Background(), []byte(`{"BizToken":"biz-1"}`), map[string]string{
		"X-Tc-Signature": "deadbeef",
	})
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("an unconfigured webhook secret must fail closed (403), got %v", err)
	}
}

func TestTencent_ProbeAcceptsBusinessErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-TC-Action"); got != "GetDetectInfoEnhanced" {
			t.Errorf("probe action = %q", got)
		}
		_, _ = io.WriteString(w, `{"Response":{"Error":{"Code":"InvalidParameterValue.BizTokenExpired","Message":"token expired"},"RequestId":"r"}}`)
	}))
	defer server.Close()

	verifier := newTestTencentVerifier(server.URL)
	if err := verifier.Probe(context.Background()); err != nil {
		t.Fatalf("business-level rejection still proves auth was accepted, got %v", err)
	}

	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Response":{"Error":{"Code":"AuthFailure.SecretIdNotFound","Message":"The SecretId is not found"},"RequestId":"r"}}`)
	}))
	defer unauthorized.Close()
	bad := newTestTencentVerifier(unauthorized.URL)
	if err := bad.Probe(context.Background()); err == nil {
		t.Error("expected auth failure from probe")
	}
}

func TestTencentTC3AuthorizationIsDeterministic(t *testing.T) {
	payload := []byte(`{"RuleId":"rule-1"}`)
	first := tencentTC3Authorization("faceid", "faceid.tencentcloudapi.com", "DetectAuth", "application/json", 1700000000, payload, "AKID", "sk")
	second := tencentTC3Authorization("faceid", "faceid.tencentcloudapi.com", "DetectAuth", "application/json", 1700000000, payload, "AKID", "sk")
	if first != second || first == "" {
		t.Fatalf("signature must be deterministic: %q vs %q", first, second)
	}
	if !strings.Contains(first, "SignedHeaders=content-type;host;x-tc-action") {
		t.Errorf("unexpected signed headers set: %q", first)
	}
	mutated := tencentTC3Authorization("faceid", "faceid.tencentcloudapi.com", "DetectAuth", "application/json", 1700000000, []byte(`{"RuleId":"rule-2"}`), "AKID", "sk")
	if mutated == first {
		t.Error("signature must change when the payload changes")
	}
	otherKey := tencentTC3Authorization("faceid", "faceid.tencentcloudapi.com", "DetectAuth", "application/json", 1700000000, payload, "AKID", "other")
	if otherKey == first {
		t.Error("signature must change when the secret key changes")
	}
}

func TestFactoryTencentBranches(t *testing.T) {
	factory := NewFactory()
	verifier, err := factory.Build(ProviderCredentials{Provider: "tencent_idv", APIKey: "id", APISecret: "secret", SceneID: "rule-1"})
	if err != nil || verifier == nil {
		t.Fatalf("expected a real tencent verifier, got %v", err)
	}
	if _, err := factory.Build(ProviderCredentials{Provider: "tencent_idv", APIKey: "id"}); err == nil {
		t.Error("expected incomplete tencent credentials to be rejected")
	}
	recognizer, err := factory.BuildOCR(OCRConfig{Provider: "tencent_ocr", AccessKey: "id", SecretKey: "secret"})
	if err != nil || recognizer == nil {
		t.Fatalf("expected a real tencent OCR recognizer, got %v", err)
	}
	// The IDV provider row doubles as the OCR credential source.
	if _, err := factory.BuildOCR(OCRConfig{Provider: "tencent_idv", AccessKey: "id", SecretKey: "secret"}); err != nil {
		t.Fatalf("tencent_idv must map onto the OCR adapter, got %v", err)
	}
	if _, err := factory.BuildOCR(OCRConfig{Provider: "tencent_ocr", AccessKey: "id"}); err == nil {
		t.Error("expected incomplete tencent OCR credentials to be rejected")
	}
	ref, err := factory.CallbackReference("tencent_idv", []byte(`{"BizToken":"biz-1"}`))
	if err != nil || ref != "biz-1" {
		t.Fatalf("tencent reference = %q, err = %v", ref, err)
	}
	if _, err := factory.CallbackReference("tencent_idv", []byte(`{}`)); err == nil {
		t.Error("a body without BizToken must be rejected")
	}
}
