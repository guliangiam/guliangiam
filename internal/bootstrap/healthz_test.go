package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func resetCryptoHealth(t *testing.T) {
	t.Helper()
	platformcrypto.SetRuntimeReport(platformcrypto.ProfileReport{
		Profile:   string(platformcrypto.ProfileDefault),
		Available: true,
	})
	t.Cleanup(func() {
		platformcrypto.SetRuntimeReport(platformcrypto.ProfileReport{
			Profile:   string(platformcrypto.ProfileDefault),
			Available: true,
		})
	})
}

type healthzBody struct {
	Status   string   `json:"status"`
	Profile  string   `json:"profile"`
	Missing  []string `json:"missing"`
	Degraded []string `json:"degraded"`
}

func serveHealthz(t *testing.T) (*httptest.ResponseRecorder, healthzBody) {
	t.Helper()
	recorder := httptest.NewRecorder()
	healthzHandler(recorder, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var body healthzBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", recorder.Body.String(), err)
	}
	return recorder, body
}

func TestHealthzUnavailableWhenGMPRequiredCapabilityMissing(t *testing.T) {
	resetCryptoHealth(t)
	platformcrypto.SetRuntimeReport(platformcrypto.ProfileReport{
		Profile:   string(platformcrypto.ProfileGM),
		Available: false,
		Missing:   []string{platformcrypto.CapabilitySM3},
	})

	recorder, body := serveHealthz(t)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if body.Status != platformcrypto.HealthStatusUnavailable {
		t.Fatalf("status = %q, want unavailable", body.Status)
	}
	if body.Profile != string(platformcrypto.ProfileGM) {
		t.Fatalf("profile = %q, want gm", body.Profile)
	}
	if len(body.Missing) != 1 || body.Missing[0] != platformcrypto.CapabilitySM3 {
		t.Fatalf("missing = %v, want [sm3]", body.Missing)
	}
}

func TestHealthzOKForHealthyGMProfile(t *testing.T) {
	resetCryptoHealth(t)
	platformcrypto.SetRuntimeReport(platformcrypto.ProfileReport{
		Profile:   string(platformcrypto.ProfileGM),
		Available: true,
	})

	recorder, body := serveHealthz(t)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body.Status != platformcrypto.HealthStatusOK {
		t.Fatalf("status = %q, want ok", body.Status)
	}
	if body.Profile != string(platformcrypto.ProfileGM) {
		t.Fatalf("profile = %q, want gm", body.Profile)
	}
}

func TestHealthzDegradedGMTLSStaysAvailable(t *testing.T) {
	resetCryptoHealth(t)
	platformcrypto.SetRuntimeReport(platformcrypto.ProfileReport{
		Profile:   string(platformcrypto.ProfileGM),
		Available: true,
		Degraded:  []string{platformcrypto.CapabilityGMTLS},
	})

	recorder, body := serveHealthz(t)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d (gm_tls degrade must not be unavailable)", recorder.Code, http.StatusOK)
	}
	if body.Status != platformcrypto.HealthStatusDegraded {
		t.Fatalf("status = %q, want degraded", body.Status)
	}
	if len(body.Degraded) != 1 || body.Degraded[0] != platformcrypto.CapabilityGMTLS {
		t.Fatalf("degraded = %v, want [gm_tls]", body.Degraded)
	}
}

func TestHealthzOKForDefaultProfile(t *testing.T) {
	resetCryptoHealth(t)

	recorder, body := serveHealthz(t)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body.Status != platformcrypto.HealthStatusOK {
		t.Fatalf("status = %q, want ok", body.Status)
	}
	if body.Profile != string(platformcrypto.ProfileDefault) {
		t.Fatalf("profile = %q, want default", body.Profile)
	}
}

func TestApplyCryptoProfileSkipsPublishOnFailure(t *testing.T) {
	resetCryptoHealth(t)
	original := validateCryptoProfile
	t.Cleanup(func() { validateCryptoProfile = original })

	refused, refuseErr := platformcrypto.AssessProfile("gm", []platformcrypto.CapabilityStatus{
		{ID: platformcrypto.CapabilitySM2Sign, Available: true},
		{ID: platformcrypto.CapabilitySM2Encrypt, Available: true},
		{ID: platformcrypto.CapabilitySM3, Available: false},
		{ID: platformcrypto.CapabilitySM4, Available: true},
	})
	if refuseErr == nil {
		t.Fatal("fixture must be refused by AssessProfile")
	}
	if !strings.Contains(refuseErr.Error(), "refusing to start") {
		t.Fatalf("fixture error = %q, want fail-close wording", refuseErr.Error())
	}
	validateCryptoProfile = func() (platformcrypto.ProfileReport, error) {
		return refused, refuseErr
	}

	err := applyCryptoProfile()
	if err == nil {
		t.Fatal("applyCryptoProfile must return the startup-refusing error")
	}
	if err.Error() != refuseErr.Error() {
		t.Fatalf("err = %v, want the validation error propagated unchanged", err)
	}
	if got := platformcrypto.HealthReport(); got.Status != platformcrypto.HealthStatusOK ||
		got.Profile != string(platformcrypto.ProfileDefault) {
		t.Fatalf("failed gate published %+v; /healthz must keep the previous report", got)
	}
}

func TestApplyCryptoProfilePublishesReportOnSuccess(t *testing.T) {
	resetCryptoHealth(t)
	original := validateCryptoProfile
	t.Cleanup(func() { validateCryptoProfile = original })
	validateCryptoProfile = func() (platformcrypto.ProfileReport, error) {
		return platformcrypto.ProfileReport{
			Profile:   string(platformcrypto.ProfileGM),
			Available: true,
			Degraded:  []string{platformcrypto.CapabilityGMTLS},
		}, nil
	}

	if err := applyCryptoProfile(); err != nil {
		t.Fatalf("applyCryptoProfile: %v", err)
	}
	if got := platformcrypto.HealthReport(); got.Status != platformcrypto.HealthStatusDegraded ||
		got.Profile != string(platformcrypto.ProfileGM) {
		t.Fatalf("health = %+v, want degraded/gm", got)
	}
}
