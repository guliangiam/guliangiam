package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// defaultReCAPTCHAV3ScoreThreshold is the score below which a v3 token is
// rejected. Google's own guidance treats 0.5 as "likely human"; a deployment
// that wants a stricter bar sets RECAPTCHA_V3_SCORE_THRESHOLD.
const defaultReCAPTCHAV3ScoreThreshold = 0.5

// reCAPTCHAV3VerifyURL is the same siteverify endpoint as v2; the v3 response
// additionally carries score (0.0-1.0, higher = more human-like) and action
// (https://developers.google.com/recaptcha/docs/v3, verified 2026-11).
const reCAPTCHAV3VerifyURL = "https://www.google.com/recaptcha/api/siteverify"

// ReCAPTCHAV3Provider verifies Google reCAPTCHA v3 tokens.
//
// v3 never renders a challenge: the client calls grecaptcha.execute() and the
// token carries a risk score. `success` alone therefore proves only that Google
// recognised the token, not that a human produced it, so this provider also
// compares `score` with a threshold and, when configured, checks the action
// label the client submitted.
type ReCAPTCHAV3Provider struct {
	SecretKey      string
	ScoreThreshold float64
	ExpectedAction string
	client         *http.Client
}

// NewReCAPTCHAV3Provider creates a reCAPTCHA v3 provider. A threshold outside
// (0, 1] falls back to the default so an unset or malformed environment value
// cannot turn verification into an unconditional pass or a permanent reject.
func NewReCAPTCHAV3Provider(secretKey string, scoreThreshold float64, expectedAction string) *ReCAPTCHAV3Provider {
	if scoreThreshold <= 0 || scoreThreshold > 1 {
		scoreThreshold = defaultReCAPTCHAV3ScoreThreshold
	}
	return &ReCAPTCHAV3Provider{
		SecretKey:      secretKey,
		ScoreThreshold: scoreThreshold,
		ExpectedAction: strings.TrimSpace(expectedAction),
		client:         ssrf.DefaultHTTPClient,
	}
}

func (p *ReCAPTCHAV3Provider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	data := url.Values{}
	data.Set("secret", p.SecretKey)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reCAPTCHAV3VerifyURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, apperrors.WrapInternalf(err, "recaptcha v3 verify request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, apperrors.WrapUnavailablef(err, "recaptcha v3 verify")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, apperrors.NewUnavailablef("recaptcha v3 verify: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Success    bool     `json:"success"`
		Score      float64  `json:"score"`
		Action     string   `json:"action"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, apperrors.WrapUnavailablef(err, "recaptcha v3 decode response")
	}

	return p.accept(result.Success, result.Score, result.Action), nil
}

// accept applies the v3 verdict policy: Google's success flag, the configured
// action (when set) and the score threshold. Google returns score 0 on a failed
// verification, so the threshold check also rejects a success=false response.
func (p *ReCAPTCHAV3Provider) accept(success bool, score float64, action string) bool {
	if !success {
		return false
	}
	if p.ExpectedAction != "" && action != p.ExpectedAction {
		return false
	}
	return score >= p.ScoreThreshold
}
