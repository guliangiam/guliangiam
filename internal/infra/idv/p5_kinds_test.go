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
	"strings"
	"testing"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

func allowLoopbackOnfido(v *OnfidoVerifier) *OnfidoVerifier {
	v.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return v
}

func allowLoopbackVeriff(v *VeriffVerifier) *VeriffVerifier {
	v.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return v
}

func allowLoopbackPersona(v *PersonaVerifier) *PersonaVerifier {
	v.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return v
}

// TestOnfidoContract pins the documented v3.6 workflow-run shape: Token token=
// auth header, POST /applicants → POST /workflow_runs, GET /workflow_runs/{id}
// and the webhook HMAC over the raw body.
func TestOnfidoContract(t *testing.T) {
	var applicantBody map[string]any
	var runBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/applicants", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token token=api_sandbox_test" {
			t.Errorf("unexpected authorization %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &applicantBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"apt_123"}`))
	})
	mux.HandleFunc("/workflow_runs", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &runBody)
		_, _ = w.Write([]byte(`{"id":"run_9","status":"awaiting_applicant","sdk_token":"sdk_tok"}`))
	})
	mux.HandleFunc("/workflow_runs/run_9", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"run_9","status":"approved"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	verifier := allowLoopbackOnfido(NewOnfidoVerifier("api_sandbox_test", "whk_secret", "wf_1", server.URL))
	initiated, err := verifier.InitiateVerification(context.Background(), InitiateOptions{
		UserID:   "user-1",
		FullName: "Alexander Sample",
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if initiated.VerificationID != "run_9" || initiated.SessionToken != "sdk_tok" {
		t.Fatalf("unexpected initiate result %+v", initiated)
	}
	if applicantBody["first_name"] != "Alexander" || applicantBody["last_name"] != "Sample" {
		t.Errorf("unexpected applicant payload %+v", applicantBody)
	}
	if runBody["applicant_id"] != "apt_123" || runBody["workflow_id"] != "wf_1" {
		t.Errorf("unexpected workflow run payload %+v", runBody)
	}

	result, err := verifier.GetVerificationResult(context.Background(), "run_9")
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if result.Status != StatusApproved || result.ProviderName != "onfido" {
		t.Fatalf("unexpected result %+v", result)
	}

	callbackBody := []byte(`{"action":"workflow_run.completed","data":{"id":"run_9","status":"declined"}}`)
	mac := hmac.New(sha256.New, []byte("whk_secret"))
	mac.Write(callbackBody)
	signature := hex.EncodeToString(mac.Sum(nil))
	callback, err := verifier.VerifyCallback(context.Background(), callbackBody, map[string]string{"Signatures": "t=1730000000,signature=" + signature})
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if callback.VerificationID != "run_9" || callback.Status != StatusRejected {
		t.Fatalf("unexpected callback result %+v", callback)
	}
	if _, err := verifier.VerifyCallback(context.Background(), callbackBody, map[string]string{"Signatures": "t=1730000000,signature=deadbeef"}); err == nil {
		t.Fatal("expected forged signature to be rejected")
	}
	if _, err := verifier.VerifyCallback(context.Background(), callbackBody, nil); err == nil {
		t.Fatal("expected missing signature to be rejected")
	}

	// Probe: an unregistered random run id answers 404, which proves the
	// token was accepted.
	if err := verifier.Probe(context.Background()); err != nil {
		t.Fatalf("probe: %v", err)
	}
}

// TestVeriffContract pins the Public API v1 session shape: X-AUTH-CLIENT auth,
// POST /v1/sessions → {id,url}, GET /v1/sessions/{id}/decision signed over the
// session id, and the webhook HMAC over the raw body.
func TestVeriffContract(t *testing.T) {
	var gotSessionSign string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-AUTH-CLIENT") != "api-key-1" {
			t.Errorf("missing X-AUTH-CLIENT")
		}
		_, _ = w.Write([]byte(`{"id":"ses_1","url":"https://met.veriff.test/v/ses_1"}`))
	})
	mux.HandleFunc("/v1/sessions/ses_1/decision", func(w http.ResponseWriter, r *http.Request) {
		gotSessionSign = r.Header.Get("X-HMAC-SIGNATURE")
		_, _ = w.Write([]byte(`{"decision":"approved"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	verifier := allowLoopbackVeriff(NewVeriffVerifier("api-key-1", "shared-secret", "", server.URL))
	initiated, err := verifier.InitiateVerification(context.Background(), InitiateOptions{UserID: "user-1"})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if initiated.VerificationID != "ses_1" || initiated.RedirectURL != "https://met.veriff.test/v/ses_1" {
		t.Fatalf("unexpected initiate result %+v", initiated)
	}

	result, err := verifier.GetVerificationResult(context.Background(), "ses_1")
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if result.Status != StatusApproved {
		t.Fatalf("unexpected result %+v", result)
	}
	mac := hmac.New(sha256.New, []byte("shared-secret"))
	mac.Write([]byte("ses_1"))
	if want := hex.EncodeToString(mac.Sum(nil)); gotSessionSign != want {
		t.Fatalf("decision GET signature mismatch: got %q want %q", gotSessionSign, want)
	}

	callbackBody := []byte(`{"id":"ses_1","decision":{"decision":"declined"}}`)
	hookMac := hmac.New(sha256.New, []byte("shared-secret"))
	hookMac.Write(callbackBody)
	callback, err := verifier.VerifyCallback(context.Background(), callbackBody, map[string]string{"X-Hmac-Signature": hex.EncodeToString(hookMac.Sum(nil))})
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if callback.VerificationID != "ses_1" || callback.Status != StatusRejected {
		t.Fatalf("unexpected callback result %+v", callback)
	}
	if _, err := verifier.VerifyCallback(context.Background(), callbackBody, map[string]string{"X-Hmac-Signature": "bad"}); err == nil {
		t.Fatal("expected forged signature to be rejected")
	}
}

// TestPersonaContract pins the Inquiries API v1 shape: Bearer auth, JSON:API
// create body, GET /inquiries/{id} status polling and the persona-signature
// webhook over "<t>.<body>".
func TestPersonaContract(t *testing.T) {
	var createBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/inquiries", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer pat_test" {
			t.Errorf("unexpected authorization %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &createBody)
		_, _ = w.Write([]byte(`{"data":{"id":"inq_1","attributes":{"status":"incomplete"}}}`))
	})
	mux.HandleFunc("/inquiries/inq_1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"id":"inq_1","attributes":{"status":"approved"}}}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	verifier := allowLoopbackPersona(NewPersonaVerifier("pat_test", "whsec_1", "itmpl_1", server.URL))
	initiated, err := verifier.InitiateVerification(context.Background(), InitiateOptions{
		UserID:   "usr_1",
		FullName: "Alexander Sample",
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if initiated.VerificationID != "inq_1" || !strings.Contains(initiated.RedirectURL, "inquiry-id=inq_1") {
		t.Fatalf("unexpected initiate result %+v", initiated)
	}
	data, _ := createBody["data"].(map[string]any)
	attributes, _ := data["attributes"].(map[string]any)
	if attributes["inquiry-template-id"] != "itmpl_1" || attributes["reference-id"] != "usr_1" {
		t.Errorf("unexpected create attributes %+v", attributes)
	}

	result, err := verifier.GetVerificationResult(context.Background(), "inq_1")
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if result.Status != StatusApproved {
		t.Fatalf("unexpected result %+v", result)
	}

	callbackBody := []byte(`{"data":{"type":"webhook_event","attributes":{"name":"inquiry.failed"},"relationships":{"inquiry":{"data":{"id":"inq_1"}}}}}`)
	ts := "1730000000"
	mac := hmac.New(sha256.New, []byte("whsec_1"))
	mac.Write([]byte(ts + "."))
	mac.Write(callbackBody)
	callback, err := verifier.VerifyCallback(context.Background(), callbackBody, map[string]string{"Persona-Signature": "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))})
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if callback.VerificationID != "inq_1" || callback.Status != StatusRejected {
		t.Fatalf("unexpected callback result %+v", callback)
	}
	if _, err := verifier.VerifyCallback(context.Background(), callbackBody, map[string]string{"Persona-Signature": "t=" + ts + ",v1=deadbeef"}); err == nil {
		t.Fatal("expected forged signature to be rejected")
	}
}
