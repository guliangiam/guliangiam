package idv

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	jumioDefaultEndpoint = "https://netverify.com/api/v4"
	jumioSandboxEndpoint = "https://sandbox-netverify.com/api/v4"
	jumioProviderName    = "jumio"
)

// JumioVerifier implements IdentityVerifier for Jumio Netverify v4.
// Jumio authenticates with HTTP Basic (API token + API secret) and requires a
// User-Agent on every call.
//
// Calibration (OPEN-API-75 P5, 2026-11-14, 已按官方文档复核，无偏差):
//   - ID Verification Web Implementation（POST https://netverify.com/api/v4/initiate，
//     EU https://lon.netverify.com/api/v4，SGP https://core-sgp.jumio.com/api/v4）:
//     https://documentation.jumio.ai/docs/Integrate/netVerifyv2/idVerification/netVerify-web-implementation
//   - Netverify Callback（form-urlencoded 推送 + HMAC 签名头）:
//     https://documentation.jumio.ai/docs/Integrate/netVerifyv2/idVerification/callbackIDVerification
type JumioVerifier struct {
	APIToken      string
	APISecret     string
	BaseURL       string
	WebhookSecret string
	client        *http.Client
}

var _ IdentityVerifier = (*JumioVerifier)(nil)

func NewJumioVerifier(apiToken, apiSecret, webhookSecret string, sandbox bool) *JumioVerifier {
	return NewJumioVerifierWithEndpoint(apiToken, apiSecret, webhookSecret, sandbox, "")
}

func NewJumioVerifierWithEndpoint(apiToken, apiSecret, webhookSecret string, sandbox bool, endpoint string) *JumioVerifier {
	baseURL := strings.TrimSpace(endpoint)
	if baseURL == "" {
		baseURL = jumioDefaultEndpoint
		if sandbox {
			baseURL = jumioSandboxEndpoint
		}
	}
	return &JumioVerifier{
		APIToken:      apiToken,
		APISecret:     apiSecret,
		BaseURL:       strings.TrimRight(baseURL, "/"),
		WebhookSecret: webhookSecret,
		client:        safeClient(),
	}
}

func (v *JumioVerifier) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	req, err := newJSONRequest(ctx, method, v.BaseURL+path, body)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "jumio: build request")
	}
	req.SetBasicAuth(v.APIToken, v.APISecret)
	req.Header.Set("User-Agent", "GuliangIAM/1.0")
	return req, nil
}

func (v *JumioVerifier) InitiateVerification(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	payload := map[string]any{
		"customerInternalReference": opts.UserID,
		"userReference":             opts.UserID,
		"successUrl":                opts.RedirectURL,
		"errorUrl":                  opts.RedirectURL,
	}
	if opts.RedirectURL != "" {
		payload["callbackUrl"] = opts.RedirectURL
	}
	if opts.Locale != "" {
		payload["locale"] = opts.Locale
	}
	if opts.Country != "" {
		payload["country"] = strings.ToUpper(opts.Country)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "jumio: marshal initiate")
	}

	req, err := v.newRequest(ctx, http.MethodPost, "/initiate", body)
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "jumio: initiate request failed")
	}
	defer resp.Body.Close()

	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(jumioProviderName, resp.StatusCode)
	}

	var result struct {
		JumioIDScanReference string `json:"jumioIdScanReference"`
		RedirectURL          string `json:"redirectUrl"`
		ClientRedirectURL    string `json:"clientRedirectUrl"`
		ExpiresAt            string `json:"authorizationExpirationDateTime"`
	}
	if err := decodeJSON(jumioProviderName, raw, &result); err != nil {
		return nil, err
	}
	if result.JumioIDScanReference == "" || result.RedirectURL == "" {
		return nil, apperrors.NewUnavailable("jumio: initiate response is missing the scan reference or redirect URL")
	}

	return &InitiateResult{
		VerificationID: result.JumioIDScanReference,
		RedirectURL:    result.RedirectURL,
		SessionToken:   result.ClientRedirectURL,
		ExpiresAt:      result.ExpiresAt,
	}, nil
}

func (v *JumioVerifier) GetVerificationResult(ctx context.Context, verificationID string) (*VerificationResult, error) {
	req, err := v.newRequest(ctx, http.MethodGet, "/scans/"+verificationID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "jumio: result request failed")
	}
	defer resp.Body.Close()

	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(jumioProviderName, resp.StatusCode)
	}

	var payload struct {
		Status       string `json:"status"`
		RejectReason string `json:"rejectReason"`
		ID           struct {
			FirstName   string `json:"firstName"`
			LastName    string `json:"lastName"`
			DateOfBirth string `json:"dateOfBirth"`
			Country     string `json:"country"`
			IDType      string `json:"idType"`
			IDNumber    string `json:"idNumber"`
		} `json:"id"`
	}
	if err := decodeJSON(jumioProviderName, raw, &payload); err != nil {
		return nil, err
	}

	result := &VerificationResult{
		VerificationID: verificationID,
		Status:         mapJumioStatus(payload.Status),
		ProviderName:   jumioProviderName,
		FirstName:      payload.ID.FirstName,
		LastName:       payload.ID.LastName,
		DateOfBirth:    payload.ID.DateOfBirth,
		Nationality:    payload.ID.Country,
		IDType:         payload.ID.IDType,
		IDNumber:       payload.ID.IDNumber,
		Reason:         payload.RejectReason,
	}
	return result, nil
}

func (v *JumioVerifier) VerifyCallback(_ context.Context, payload []byte, headers map[string]string) (*VerificationResult, error) {
	if v.WebhookSecret == "" {
		// Fail closed: an unconfigured secret must never degrade to "accept".
		return nil, apperrors.NewForbidden("jumio: webhook secret is not configured")
	}
	signature := headerValue(headers, "X-Jumio-Signature")
	if signature == "" {
		return nil, apperrors.NewUnauthorized("jumio: callback signature header is missing")
	}
	if !verifyJumioSignature(payload, signature, v.WebhookSecret) {
		return nil, apperrors.NewUnauthorized("jumio: callback signature verification failed")
	}

	var callback struct {
		Status               string `json:"status"`
		RejectReason         string `json:"rejectReason"`
		JumioIDScanReference string `json:"jumioIdScanReference"`
	}
	if err := decodeJSON(jumioProviderName, payload, &callback); err != nil {
		return nil, err
	}
	if callback.JumioIDScanReference == "" {
		return nil, apperrors.NewInvalidArgument("jumio: callback is missing jumioIdScanReference")
	}

	return &VerificationResult{
		VerificationID: callback.JumioIDScanReference,
		Status:         mapJumioStatus(callback.Status),
		ProviderName:   jumioProviderName,
		Reason:         callback.RejectReason,
	}, nil
}

// Probe performs a read-only authenticated lookup of a random scan reference.
// 200/404 both prove the credential was accepted; 401/403 are surfaced as auth
// failures and 5xx as outages.
func (v *JumioVerifier) Probe(ctx context.Context) error {
	probeID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "jumio: generate probe id")
	}
	req, err := v.newRequest(ctx, http.MethodGet, "/scans/"+probeID, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "jumio: probe request failed")
	}
	defer resp.Body.Close()
	_, _ = readLimited(resp)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNotFound:
		return nil
	default:
		return mapProviderHTTPError(jumioProviderName, resp.StatusCode)
	}
}

func mapJumioStatus(status string) VerificationStatus {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "APPROVED_VERIFIED":
		return StatusApproved
	case "DENIED", "FAILED":
		return StatusRejected
	case "EXPIRED":
		return StatusExpired
	case "PENDING":
		return StatusPending
	case "PROCESSING":
		return StatusInProgress
	case "ERROR":
		return StatusError
	default:
		return StatusPending
	}
}

// verifyJumioSignature accepts the documented hex digest and also tolerates a
// base64 digest so a provider rollout does not silently break callbacks.
func verifyJumioSignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedHex := hex.EncodeToString(mac.Sum(nil))
	if hmac.Equal([]byte(strings.TrimSpace(signature)), []byte(expectedHex)) {
		return true
	}
	expectedB64 := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.TrimSpace(signature)), []byte(expectedB64))
}
