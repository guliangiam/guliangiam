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
	onfidoDefaultEndpoint = "https://api.eu.onfido.com/v3.6"
	onfidoProviderName    = "onfido"
)

// OnfidoVerifier implements IdentityVerifier for Onfido (Entrust Identity
// Verification) v3.6 Studio workflow runs.
//
// Contract reviewed against the official API reference (OPEN-API-75 P5,
// 2026-11-14):
//   - Auth: Authorization: Token token=<api_sandbox...|api_live...>
//   - Create applicant: POST /v3.6/applicants {first_name,last_name,...}
//   - Start workflow run: POST /v3.6/workflow_runs {workflow_id, applicant_id}
//     → 201 {id, status, sdk_token, ...}
//   - Fetch result: GET /v3.6/workflow_runs/{id} → status ("approved" in the
//     documented example; the terminal verdict is carried in the status
//     attribute)
//   - Webhooks: HMAC-SHA256 hex digest of the raw body keyed with the webhook
//     signing token, delivered in the Signatures / X-Signature header
//     (verified against the official onfido-python WebhookEventVerifier).
//     https://documentation.identity.entrust.com/api/latest/
//
// The capture flow runs inside the Onfido SDK, so InitiateVerification returns
// the workflow-run id plus the sdk_token as the session token instead of a
// hosted redirect URL.
type OnfidoVerifier struct {
	APIToken      string
	WebhookSecret string
	WorkflowID    string
	BaseURL       string
	client        *http.Client
}

var _ IdentityVerifier = (*OnfidoVerifier)(nil)

func NewOnfidoVerifier(apiToken, webhookSecret, workflowID, endpoint string) *OnfidoVerifier {
	baseURL := strings.TrimSpace(endpoint)
	if baseURL == "" {
		baseURL = onfidoDefaultEndpoint
	}
	return &OnfidoVerifier{
		APIToken:      strings.TrimSpace(apiToken),
		WebhookSecret: strings.TrimSpace(webhookSecret),
		WorkflowID:    strings.TrimSpace(workflowID),
		BaseURL:       strings.TrimRight(baseURL, "/"),
		client:        safeClient(),
	}
}

func (v *OnfidoVerifier) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	req, err := newJSONRequest(ctx, method, v.BaseURL+path, body)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "onfido: build request")
	}
	req.Header.Set("Authorization", "Token token="+v.APIToken)
	return req, nil
}

func (v *OnfidoVerifier) InitiateVerification(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	if v.WorkflowID == "" {
		return nil, apperrors.NewInvalidArgument("onfido: workflowId is not configured for this provider")
	}
	first, last := splitFullName(opts.FullName)
	if first == "" || last == "" {
		return nil, apperrors.NewInvalidArgument("onfido: applicant first and last name are required")
	}
	applicantPayload := map[string]any{
		"first_name": first,
		"last_name":  last,
	}
	body, err := json.Marshal(applicantPayload)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "onfido: marshal applicant")
	}
	req, err := v.newRequest(ctx, http.MethodPost, "/applicants", body)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "onfido: create applicant failed")
	}
	defer resp.Body.Close()
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(onfidoProviderName, resp.StatusCode)
	}
	var applicant struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(onfidoProviderName, raw, &applicant); err != nil {
		return nil, err
	}
	if applicant.ID == "" {
		return nil, apperrors.NewUnavailable("onfido: applicant response is missing id")
	}

	runPayload := map[string]any{
		"applicant_id": applicant.ID,
		"workflow_id":  v.WorkflowID,
	}
	if opts.Locale != "" {
		runPayload["locale"] = opts.Locale
	}
	runBody, err := json.Marshal(runPayload)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "onfido: marshal workflow run")
	}
	req, err = v.newRequest(ctx, http.MethodPost, "/workflow_runs", runBody)
	if err != nil {
		return nil, err
	}
	resp2, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "onfido: start workflow run failed")
	}
	defer resp2.Body.Close()
	raw, err = readLimited(resp2)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp2.StatusCode) {
		return nil, mapProviderHTTPError(onfidoProviderName, resp2.StatusCode)
	}
	var run struct {
		ID       string `json:"id"`
		SDKToken string `json:"sdk_token"`
		Status   string `json:"status"`
	}
	if err := decodeJSON(onfidoProviderName, raw, &run); err != nil {
		return nil, err
	}
	if run.ID == "" {
		return nil, apperrors.NewUnavailable("onfido: workflow run response is missing id")
	}
	return &InitiateResult{
		VerificationID: run.ID,
		SessionToken:   run.SDKToken,
	}, nil
}

func (v *OnfidoVerifier) GetVerificationResult(ctx context.Context, verificationID string) (*VerificationResult, error) {
	req, err := v.newRequest(ctx, http.MethodGet, "/workflow_runs/"+verificationID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "onfido: result request failed")
	}
	defer resp.Body.Close()
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(onfidoProviderName, resp.StatusCode)
	}
	var run struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(onfidoProviderName, raw, &run); err != nil {
		return nil, err
	}
	out := &VerificationResult{
		VerificationID: verificationID,
		Status:         mapOnfidoStatus(run.Status),
		ProviderName:   onfidoProviderName,
	}
	return out, nil
}

func (v *OnfidoVerifier) VerifyCallback(_ context.Context, payload []byte, headers map[string]string) (*VerificationResult, error) {
	if v.WebhookSecret == "" {
		// Fail closed: an unconfigured signing token must never degrade to "accept".
		return nil, apperrors.NewForbidden("onfido: webhook signing token is not configured")
	}
	signature := headerValue(headers, "Signatures")
	if signature == "" {
		signature = headerValue(headers, "X-Signature")
	}
	if signature == "" {
		return nil, apperrors.NewUnauthorized("onfido: webhook signature header is missing")
	}
	if !verifyOnfidoSignature(payload, signature, v.WebhookSecret) {
		return nil, apperrors.NewUnauthorized("onfido: webhook signature verification failed")
	}
	var event struct {
		Action string `json:"action"`
		Data   struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := decodeJSON(onfidoProviderName, payload, &event); err != nil {
		return nil, err
	}
	if event.Data.ID == "" {
		return nil, apperrors.NewInvalidArgument("onfido: webhook event is missing data.id")
	}
	return &VerificationResult{
		VerificationID: event.Data.ID,
		Status:         mapOnfidoStatus(event.Data.Status),
		ProviderName:   onfidoProviderName,
	}, nil
}

// Probe performs a read-only lookup of a random workflow-run id. 200/404 both
// prove the token was accepted; 401/403 are surfaced as auth failures.
func (v *OnfidoVerifier) Probe(ctx context.Context) error {
	probeID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "onfido: generate probe id")
	}
	req, err := v.newRequest(ctx, http.MethodGet, "/workflow_runs/"+probeID, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "onfido: probe request failed")
	}
	defer resp.Body.Close()
	_, _ = readLimited(resp)
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNotFound:
		return nil
	default:
		return mapProviderHTTPError(onfidoProviderName, resp.StatusCode)
	}
}

// mapOnfidoStatus maps a Studio workflow-run status onto the domain status.
// "approved" is the documented terminal success value; unknown statuses stay
// pending so polling continues.
func mapOnfidoStatus(status string) VerificationStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "approved":
		return StatusApproved
	case "declined", "rejected":
		return StatusRejected
	case "abandoned", "expired":
		return StatusExpired
	case "error":
		return StatusError
	case "in_progress", "processing":
		return StatusInProgress
	default:
		return StatusPending
	}
}

// verifyOnfidoSignature checks the documented hex HMAC-SHA256 of the raw body
// keyed with the webhook signing token. The current API exposes the digest in
// the Signatures header as one or more "t=<ts>,signature=<hex>" pairs; the
// legacy spelling is a bare hex digest in X-Signature. Both shapes are
// accepted so a provider rollout does not silently break callbacks.
func verifyOnfidoSignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, part := range strings.FieldsFunc(signature, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		value := part
		if idx := strings.Index(value, "="); idx >= 0 {
			key := strings.ToLower(strings.TrimSpace(value[:idx]))
			value = strings.TrimSpace(value[idx+1:])
			if key != "signature" {
				continue
			}
		}
		if hmac.Equal([]byte(strings.TrimSpace(value)), []byte(expected)) {
			return true
		}
	}
	return false
}

func splitFullName(full string) (string, string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	if idx := strings.IndexAny(full, " "); idx > 0 {
		return strings.TrimSpace(full[:idx]), strings.TrimSpace(full[idx+1:])
	}
	return full, full
}
