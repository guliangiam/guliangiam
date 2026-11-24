package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ReCAPTCHAEnterpriseProvider verifies Google reCAPTCHA Enterprise tokens via
// the assessments REST API.
//
// Official contract (verified 2026-11 against
// https://cloud.google.com/recaptcha-enterprise/docs/create-assessment):
//   - POST https://recaptchaenterprise.googleapis.com/v1/projects/{project}/assessments?key={API_KEY}
//     with a JSON body {"event":{"token","siteKey","expectedAction"}}.
//   - Response {"score": 0.0-1.0, "tokenProperties": {"valid": bool, "action":
//     string}}. tokenProperties.valid=false rejects the token outright (expired,
//     replayed or forged); the score is then compared with the configured
//     threshold, and the action label is checked when an expected action is set
//     — the same verdict policy as ReCAPTCHAV3Provider.
//
// Three credentials are required (site key, API key, project id); the two-slot
// mapping keeps the site key and API key, and the project id travels in the
// provider config (see NewProviderFromRow).
type ReCAPTCHAEnterpriseProvider struct {
	SiteKey        string
	APIKey         string
	ProjectID      string
	ScoreThreshold float64
	ExpectedAction string

	Endpoint string
	client   *http.Client
}

// NewReCAPTCHAEnterpriseProvider creates a reCAPTCHA Enterprise provider. A
// threshold outside (0, 1] falls back to the v3 default so a malformed config
// value cannot turn verification into an unconditional pass or reject.
func NewReCAPTCHAEnterpriseProvider(siteKey, apiKey, projectID string, scoreThreshold float64, expectedAction string) *ReCAPTCHAEnterpriseProvider {
	if scoreThreshold <= 0 || scoreThreshold > 1 {
		scoreThreshold = defaultReCAPTCHAV3ScoreThreshold
	}
	return &ReCAPTCHAEnterpriseProvider{
		SiteKey:        siteKey,
		APIKey:         apiKey,
		ProjectID:      projectID,
		ScoreThreshold: scoreThreshold,
		ExpectedAction: strings.TrimSpace(expectedAction),
		Endpoint:       "https://recaptchaenterprise.googleapis.com",
		client:         ssrf.DefaultHTTPClient,
	}
}

// Verify creates an assessment for the token and applies the verdict policy.
func (p *ReCAPTCHAEnterpriseProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if p.SiteKey == "" || p.APIKey == "" || p.ProjectID == "" {
		return false, apperrors.NewUnavailable("recaptcha enterprise: credentials not configured")
	}
	if token == "" {
		return false, apperrors.NewInvalidArgument("recaptcha enterprise: token is required")
	}

	event := map[string]any{"token": token, "siteKey": p.SiteKey}
	if p.ExpectedAction != "" {
		event["expectedAction"] = p.ExpectedAction
	}
	if remoteIP != "" {
		event["userIp"] = remoteIP
	}
	payload, err := json.Marshal(map[string]any{"event": event})
	if err != nil {
		return false, apperrors.WrapInternalf(err, "recaptcha enterprise: marshal request")
	}

	endpoint := fmt.Sprintf("%s/v1/projects/%s/assessments?key=%s",
		strings.TrimRight(p.Endpoint, "/"), url.PathEscape(p.ProjectID), p.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "recaptcha enterprise: build request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "recaptcha enterprise verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("recaptcha enterprise verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Score           float64 `json:"score"`
		TokenProperties *struct {
			Valid  bool   `json:"valid"`
			Action string `json:"action"`
		} `json:"tokenProperties"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "recaptcha enterprise: decode response")
	}
	props := result.TokenProperties
	if props == nil || !props.Valid {
		return false, nil
	}
	if p.ExpectedAction != "" && props.Action != p.ExpectedAction {
		return false, nil
	}
	return result.Score >= p.ScoreThreshold, nil
}

var _ Provider = (*ReCAPTCHAEnterpriseProvider)(nil)
