package idv

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	veriffDefaultEndpoint = "https://api.veriff.com"
	veriffProviderName    = "veriff"
)

// VeriffVerifier implements IdentityVerifier for the Veriff Public API v1
// session flow.
//
// Contract reviewed against the official developer docs (OPEN-API-75 P5,
// 2026-11-14, https://devdocs.veriff.com/apidocs):
//   - Auth: X-AUTH-CLIENT carries the integration API key; X-HMAC-SIGNATURE
//     carries the hex HMAC-SHA256 of the request body (POST/PATCH) or of the
//     session id (GET/DELETE) keyed with the shared secret key. POST /sessions
//     is the documented exception that needs no X-HMAC-SIGNATURE.
//   - Create session: POST /v1/sessions {verification:{...}} → {id, url} where
//     url is the hosted verification link.
//   - Fetch decision: GET /v1/sessions/{sessionId}/decision → final decision
//     ("approved", "declined", "expired", "abandoned").
//   - Webhooks: decision events are pushed with the x-hmac-signature (or
//     VRF-HMAC-SIGNATURE) header, the hex HMAC-SHA256 of the raw body keyed
//     with the master shared secret key.
type VeriffVerifier struct {
	APIKey        string
	SharedSecret  string
	WebhookSecret string
	BaseURL       string
	client        *http.Client
}

var _ IdentityVerifier = (*VeriffVerifier)(nil)

func NewVeriffVerifier(apiKey, sharedSecret, webhookSecret, endpoint string) *VeriffVerifier {
	baseURL := strings.TrimSpace(endpoint)
	if baseURL == "" {
		baseURL = veriffDefaultEndpoint
	}
	if strings.TrimSpace(webhookSecret) == "" {
		// The decision webhook is keyed with the same master shared secret.
		webhookSecret = sharedSecret
	}
	return &VeriffVerifier{
		APIKey:        strings.TrimSpace(apiKey),
		SharedSecret:  strings.TrimSpace(sharedSecret),
		WebhookSecret: strings.TrimSpace(webhookSecret),
		BaseURL:       strings.TrimRight(baseURL, "/"),
		client:        safeClient(),
	}
}

func (v *VeriffVerifier) InitiateVerification(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	verification := map[string]any{}
	if opts.UserID != "" {
		verification["reference_id"] = opts.UserID
	}
	if opts.RedirectURL != "" {
		verification["callback_url"] = opts.RedirectURL
	}
	body, err := json.Marshal(map[string]any{"verification": verification})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "veriff: marshal session")
	}
	req, err := newJSONRequest(ctx, http.MethodPost, v.BaseURL+"/v1/sessions", body)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "veriff: build request")
	}
	req.Header.Set("X-AUTH-CLIENT", v.APIKey)
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "veriff: create session failed")
	}
	defer resp.Body.Close()
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(veriffProviderName, resp.StatusCode)
	}
	var session struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := decodeJSON(veriffProviderName, raw, &session); err != nil {
		return nil, err
	}
	if session.ID == "" {
		return nil, apperrors.NewUnavailable("veriff: session response is missing id")
	}
	return &InitiateResult{
		VerificationID: session.ID,
		RedirectURL:    session.URL,
		SessionToken:   session.ID,
	}, nil
}

func (v *VeriffVerifier) GetVerificationResult(ctx context.Context, verificationID string) (*VerificationResult, error) {
	raw, err := v.getDecision(ctx, verificationID)
	if err != nil {
		return nil, err
	}
	var decision struct {
		Decision any `json:"decision"`
	}
	if err := json.Unmarshal(raw, &decision); err != nil {
		return nil, apperrors.NewUnavailable("veriff: decision response is not valid JSON")
	}
	out := &VerificationResult{
		VerificationID: verificationID,
		Status:         mapVeriffDecision(decisionValue(decision.Decision)),
		ProviderName:   veriffProviderName,
	}
	return out, nil
}

// getDecision signs the session id (the documented GET recipe) and fetches
// /v1/sessions/{id}/decision.
func (v *VeriffVerifier) getDecision(ctx context.Context, sessionID string) ([]byte, error) {
	req, err := newJSONRequest(ctx, http.MethodGet, v.BaseURL+"/v1/sessions/"+sessionID+"/decision", nil)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "veriff: build request")
	}
	req.Header.Set("X-AUTH-CLIENT", v.APIKey)
	if v.SharedSecret != "" {
		mac := hmac.New(sha256.New, []byte(v.SharedSecret))
		mac.Write([]byte(sessionID))
		req.Header.Set("X-HMAC-SIGNATURE", hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "veriff: decision request failed")
	}
	defer resp.Body.Close()
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(veriffProviderName, resp.StatusCode)
	}
	return raw, nil
}

func (v *VeriffVerifier) VerifyCallback(_ context.Context, payload []byte, headers map[string]string) (*VerificationResult, error) {
	if v.WebhookSecret == "" {
		// Fail closed: an unconfigured shared secret must never degrade to "accept".
		return nil, apperrors.NewForbidden("veriff: webhook secret is not configured")
	}
	signature := headerValue(headers, "X-Hmac-Signature")
	if signature == "" {
		signature = headerValue(headers, "Vrf-Hmac-Signature")
	}
	if signature == "" {
		return nil, apperrors.NewUnauthorized("veriff: webhook signature header is missing")
	}
	mac := hmac.New(sha256.New, []byte(v.WebhookSecret))
	mac.Write(payload)
	if !hmac.Equal([]byte(strings.TrimSpace(signature)), []byte(hex.EncodeToString(mac.Sum(nil)))) {
		return nil, apperrors.NewUnauthorized("veriff: webhook signature verification failed")
	}
	// Decision events arrive as a JSON object; the session id and decision are
	// read leniently from both the top level and the nested "payload" object
	// used by older integrations.
	var callback struct {
		ID       string          `json:"id"`
		Decision json.RawMessage `json:"decision"`
		Payload  struct {
			ID       string          `json:"id"`
			Decision json.RawMessage `json:"decision"`
		} `json:"payload"`
	}
	if err := decodeJSON(veriffProviderName, payload, &callback); err != nil {
		return nil, err
	}
	verificationID := callback.ID
	decisionRaw := callback.Decision
	if verificationID == "" {
		verificationID = callback.Payload.ID
		decisionRaw = callback.Payload.Decision
	}
	if verificationID == "" {
		return nil, apperrors.NewInvalidArgument("veriff: callback is missing the session id")
	}
	var decisionPayload any
	if len(decisionRaw) > 0 {
		if err := json.Unmarshal(decisionRaw, &decisionPayload); err != nil {
			decisionPayload = string(decisionRaw)
		}
	}
	return &VerificationResult{
		VerificationID: verificationID,
		Status:         mapVeriffDecision(decisionValue(decisionPayload)),
		ProviderName:   veriffProviderName,
	}, nil
}

// Probe fetches the decision of a random session id. 401/403 disqualify the
// credential; 404/400 prove the API key was accepted.
func (v *VeriffVerifier) Probe(ctx context.Context) error {
	probeID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "veriff: generate probe id")
	}
	_, err = v.getDecision(ctx, probeID)
	if err == nil {
		return nil
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		return err
	}
	switch appErr.Code {
	case apperrors.CodeUnauthorized, apperrors.CodeForbidden:
		return err
	case apperrors.CodeNotFound, apperrors.CodeInvalidArgument:
		// The request was authenticated and understood; only transport or
		// credential failures disqualify.
		return nil
	default:
		return err
	}
}

// decisionValue unwraps the documented shapes of the decision field: either a
// bare string or an object carrying {decision:"approved"} plus reasons.
func decisionValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		if inner, ok := typed["decision"].(string); ok {
			return inner
		}
	}
	return ""
}

func mapVeriffDecision(decision string) VerificationStatus {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approved", "accepted":
		return StatusApproved
	case "declined", "rejected", "rejection":
		return StatusRejected
	case "expired":
		return StatusExpired
	case "abandoned":
		return StatusRejected
	case "pending", "resubmission":
		return StatusPending
	default:
		return StatusPending
	}
}
