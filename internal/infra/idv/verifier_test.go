package idv

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

func allowLoopback(client *JumioVerifier) *JumioVerifier {
	client.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return client
}

func allowLoopbackAliyun(client *AliyunIDVVerifier) *AliyunIDVVerifier {
	client.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return client
}

func TestJumio_InitiateSuccess(t *testing.T) {
	var gotAuth string
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUserAgent = r.Header.Get("User-Agent")
		if r.URL.Path != "/initiate" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"jumioIdScanReference":"scan-1","redirectUrl":"https://jumio.test/redirect","authorizationExpirationDateTime":"2026-11-05T10:00:00Z"}`)
	}))
	defer server.Close()

	verifier := allowLoopback(NewJumioVerifierWithEndpoint("token", "secret", "whsec", false, server.URL))
	result, err := verifier.InitiateVerification(context.Background(), InitiateOptions{UserID: "u-1", RedirectURL: "https://iam.test/return"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.VerificationID != "scan-1" || result.RedirectURL != "https://jumio.test/redirect" {
		t.Errorf("unexpected result: %+v", result)
	}
	if gotAuth == "" || !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("expected basic auth, got %q", gotAuth)
	}
	if gotUserAgent == "" {
		t.Error("jumio requires a User-Agent header")
	}
}

func TestJumio_InitiateAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	verifier := allowLoopback(NewJumioVerifierWithEndpoint("bad", "bad", "", false, server.URL))
	_, err := verifier.InitiateVerification(context.Background(), InitiateOptions{UserID: "u-1"})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected Unauthorized, got %v", err)
	}
}

func TestJumio_InitiateInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"jumioIdScanReference":`)
	}))
	defer server.Close()

	verifier := allowLoopback(NewJumioVerifierWithEndpoint("token", "secret", "", false, server.URL))
	if _, err := verifier.InitiateVerification(context.Background(), InitiateOptions{UserID: "u-1"}); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestJumio_InitiateTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	verifier := NewJumioVerifierWithEndpoint("token", "secret", "", false, server.URL)
	verifier.client = ssrf.SafeHTTPClientWithAllowlist(40*time.Millisecond, []string{"127.0.0.1/32"})
	if _, err := verifier.InitiateVerification(context.Background(), InitiateOptions{UserID: "u-1"}); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestJumio_GetResultMapsStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/scans/scan-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"status":"APPROVED_VERIFIED","id":{"firstName":"Zhang","lastName":"San","idNumber":"110101199001011234"}}`)
	}))
	defer server.Close()

	verifier := allowLoopback(NewJumioVerifierWithEndpoint("token", "secret", "", false, server.URL))
	result, err := verifier.GetVerificationResult(context.Background(), "scan-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApproved {
		t.Errorf("status = %s, want approved", result.Status)
	}
	if result.FirstName != "Zhang" {
		t.Errorf("firstName = %q", result.FirstName)
	}

	if _, err := verifier.GetVerificationResult(context.Background(), "missing"); err == nil {
		t.Error("expected not-found error")
	}
}

func TestJumio_CallbackSignature(t *testing.T) {
	verifier := &JumioVerifier{WebhookSecret: "whsec"}
	payload := []byte(`{"jumioIdScanReference":"scan-1","status":"DENIED","rejectReason":"DOCUMENT_INVALID"}`)

	mac := hmac.New(sha256.New, []byte("whsec"))
	mac.Write(payload)
	valid := hex.EncodeToString(mac.Sum(nil))

	result, err := verifier.VerifyCallback(context.Background(), payload, map[string]string{"x-jumio-signature": valid})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusRejected || result.VerificationID != "scan-1" {
		t.Errorf("unexpected result: %+v", result)
	}

	if _, err := verifier.VerifyCallback(context.Background(), payload, map[string]string{"X-Jumio-Signature": "deadbeef"}); err == nil {
		t.Error("expected signature rejection")
	}
	if _, err := verifier.VerifyCallback(context.Background(), payload, nil); err == nil {
		t.Error("expected missing-signature rejection")
	}
}

func TestJumio_Probe(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	verifier := allowLoopback(NewJumioVerifierWithEndpoint("token", "secret", "", false, server.URL))
	if err := verifier.Probe(context.Background()); err != nil {
		t.Fatalf("404 must prove the credential was accepted, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected one read-only probe call, got %d", calls)
	}

	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer unauthorized.Close()
	bad := allowLoopback(NewJumioVerifierWithEndpoint("bad", "bad", "", false, unauthorized.URL))
	if err := bad.Probe(context.Background()); err == nil {
		t.Error("expected auth failure")
	}
}

func TestAliyun_InitiateParsesResultObject(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		form = r.PostForm
		_, _ = io.WriteString(w, `{"Code":"200","RequestId":"req-1","ResultObject":"{\"certifyId\":\"cert-1\",\"certifyUrl\":\"https://aliyun.test/certify\"}"}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "scene-1", "", ""))
	result, err := verifier.InitiateVerification(context.Background(), InitiateOptions{
		UserID: "u-1", FullName: "Zhang San", IDNumber: "110101199001011234", IDType: "ID_CARD",
		BizSeq: "0123456789abcdef0123456789abcdef", MetaInfo: `{"deviceType":"web"}`,
		RedirectURL: "https://iam.test/return",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.VerificationID != "cert-1" || result.RedirectURL != "https://aliyun.test/certify" {
		t.Errorf("unexpected result: %+v", result)
	}
	if result.ProductCode != "ID_PRO" {
		t.Errorf("productCode = %q, want ID_PRO", result.ProductCode)
	}
	if form.Get("Action") != "InitFaceVerify" || form.Get("Signature") == "" {
		t.Errorf("expected signed InitFaceVerify form, got %v", form)
	}
	if form.Get("CertName") != "Zhang San" || form.Get("CertNo") != "110101199001011234" {
		t.Error("claimed identity was not forwarded to CloudAuth")
	}
	if form.Get("OuterOrderNo") != "0123456789abcdef0123456789abcdef" {
		t.Errorf("OuterOrderNo = %q, want the server-generated bizSeq", form.Get("OuterOrderNo"))
	}
	if form.Get("MetaInfo") != `{"deviceType":"web"}` || form.Get("CertifyUrlType") != "WEB" {
		t.Errorf("expected real client MetaInfo and derived CertifyUrlType, got %v", form)
	}
	if form.Get("SignatureMethod") != "HMAC-SHA1" || form.Get("SignatureVersion") != "1.0" {
		t.Error("expected RPC V1 signing parameters")
	}
}

func TestAliyun_ErrorCodeMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Code":"SignatureDoesNotMatch","Message":"bad signature"}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "scene-1", "", ""))
	_, err := verifier.InitiateVerification(context.Background(), InitiateOptions{
		UserID: "u-1", BizSeq: "0123456789abcdef0123456789abcdef",
		MetaInfo: `{"deviceType":"web"}`, RedirectURL: "https://iam.test/return",
	})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected Unauthorized for SignatureDoesNotMatch, got %v", err)
	}
}

func TestAliyun_ProbeAcceptsBusinessErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.PostForm.Get("SceneId"); got != "scene-1" {
			t.Errorf("SceneId = %q, want scene-1", got)
		}
		_, _ = io.WriteString(w, `{"Code":"InvalidParameter","Message":"certifyId not found"}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "scene-1", "", ""))
	if err := verifier.Probe(context.Background()); err != nil {
		t.Fatalf("business-level rejection still proves auth was accepted, got %v", err)
	}

	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Code":"InvalidAccessKeyId.NotFound","Message":"unknown key"}`)
	}))
	defer unauthorized.Close()
	bad := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("bad", "bad", "cn-shanghai", "", unauthorized.URL, "scene-1", "", ""))
	if err := bad.Probe(context.Background()); err == nil {
		t.Error("expected auth failure from probe")
	}
}

func TestAliyun_ProbeRequiresSceneID(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "", "", ""))
	err := verifier.Probe(context.Background())
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid argument for missing sceneId, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("missing sceneId must fail before the upstream request, got %d calls", calls)
	}
}

func TestAliyun_GetVerificationResultIncludesSceneID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.PostForm.Get("SceneId"); got != "scene-1" {
			t.Errorf("SceneId = %q, want scene-1", got)
		}
		_, _ = io.WriteString(w, `{"Code":"200","ResultObject":"{\"passed\":\"T\"}"}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "scene-1", "", ""))
	result, err := verifier.GetVerificationResult(context.Background(), "cert-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApproved {
		t.Fatalf("status = %s, want approved", result.Status)
	}
}

func TestAliyun_CallbackRequiresSignature(t *testing.T) {
	verifier := &AliyunIDVVerifier{WebhookSecret: "whsec"}
	payload := []byte(`{"CertifyId":"cert-1","Passed":"T"}`)
	if _, err := verifier.VerifyCallback(context.Background(), payload, nil); err == nil {
		t.Error("expected rejection without a signature")
	}

	result, err := verifier.VerifyCallback(context.Background(), payload, map[string]string{
		"X-Acs-Signature": "deadbeef",
	})
	if err == nil || result != nil {
		t.Error("expected invalid signature rejection")
	}

	mac := hmac.New(sha256.New, []byte("whsec"))
	mac.Write(payload)
	valid, err := verifier.VerifyCallback(context.Background(), payload, map[string]string{
		"X-Acs-Signature": hex.EncodeToString(mac.Sum(nil)),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid.Status != StatusApproved || valid.VerificationID != "cert-1" {
		t.Errorf("unexpected result: %+v", valid)
	}
}

func TestAliyun_SignatureIsDeterministicForSortedParams(t *testing.T) {
	params := map[string]string{"Action": "DescribeFaceVerify", "CertifyId": "abc", "Format": "JSON"}
	first := aliyunRPCSignature(http.MethodPost, params, "secret")
	second := aliyunRPCSignature(http.MethodPost, params, "secret")
	if first != second || first == "" {
		t.Fatalf("signature must be deterministic: %q vs %q", first, second)
	}
	mutated := map[string]string{"Action": "DescribeFaceVerify", "CertifyId": "abd", "Format": "JSON"}
	if aliyunRPCSignature(http.MethodPost, mutated, "secret") == first {
		t.Error("signature must change when a business parameter changes")
	}
}

func TestFactoryRejectsFakeOrUnknownProviders(t *testing.T) {
	factory := NewFactory()
	if _, err := factory.Build(ProviderCredentials{Provider: "jumio", APIKey: "", APISecret: ""}); err == nil {
		t.Error("expected incomplete jumio credentials to be rejected")
	}
	if _, err := factory.Build(ProviderCredentials{Provider: "aliyun_idv", APIKey: "ak"}); err == nil {
		t.Error("expected incomplete aliyun credentials to be rejected")
	}
	if _, err := factory.Build(ProviderCredentials{Provider: "faceid"}); err == nil {
		t.Error("expected unsupported provider to be rejected")
	}
	verifier, err := factory.Build(ProviderCredentials{Provider: "jumio", APIKey: "t", APISecret: "s"})
	if err != nil || verifier == nil {
		t.Fatalf("expected a real jumio verifier, got %v", err)
	}
}

func TestMapJumioStatus(t *testing.T) {
	cases := map[string]VerificationStatus{
		"APPROVED_VERIFIED": StatusApproved,
		"DENIED":            StatusRejected,
		"FAILED":            StatusRejected,
		"EXPIRED":           StatusExpired,
		"PENDING":           StatusPending,
		"PROCESSING":        StatusInProgress,
	}
	for input, want := range cases {
		if got := mapJumioStatus(input); got != want {
			t.Errorf("mapJumioStatus(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestReadLimitedRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := json.RawMessage(`{"padding":"` + strings.Repeat("a", maxResponseBytes) + `"}`)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	verifier := allowLoopback(NewJumioVerifierWithEndpoint("token", "secret", "", false, server.URL))
	if _, err := verifier.GetVerificationResult(context.Background(), "scan-1"); err == nil {
		t.Fatal("expected oversized response rejection")
	}
}

func TestJumio_CallbackWithoutSecretIsRejected(t *testing.T) {
	verifier := &JumioVerifier{}
	_, err := verifier.VerifyCallback(context.Background(), []byte(`{"jumioIdScanReference":"scan-1"}`), map[string]string{
		"X-Jumio-Signature": "deadbeef",
	})
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("an unconfigured webhook secret must fail closed (403), got %v", err)
	}
}

func TestAliyun_CallbackWithoutSecretIsRejected(t *testing.T) {
	verifier := &AliyunIDVVerifier{}
	_, err := verifier.VerifyCallback(context.Background(), []byte(`{"CertifyId":"cert-1"}`), map[string]string{
		"X-Acs-Signature": "deadbeef",
	})
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("an unconfigured webhook secret must fail closed (403), got %v", err)
	}
}

func TestFactory_CallbackReference(t *testing.T) {
	factory := NewFactory()

	ref, err := factory.CallbackReference("jumio", []byte(`{"jumioIdScanReference":"scan-1"}`))
	if err != nil || ref != "scan-1" {
		t.Fatalf("jumio reference = %q, err = %v", ref, err)
	}
	ref, err = factory.CallbackReference("aliyun_idv", []byte(`{"CertifyId":"cert-1"}`))
	if err != nil || ref != "cert-1" {
		t.Fatalf("aliyun reference = %q, err = %v", ref, err)
	}
	if _, err := factory.CallbackReference("faceid", []byte(`{}`)); err == nil {
		t.Error("unknown provider must be rejected")
	}
	if _, err := factory.CallbackReference("jumio", []byte(`{"status":"PENDING"}`)); err == nil {
		t.Error("a body without a reference must be rejected")
	}
}
