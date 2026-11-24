package idv

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	personaDefaultEndpoint = "https://api.withpersona.com/api/v1"
	personaHostedBaseURL   = "https://inquiry.withpersona.com/verify"
	personaProviderName    = "persona"
)

// PersonaVerifier implements IdentityVerifier for Persona Inquiries API v1
// (Hosted Flow).
//
// Contract reviewed against the official docs (OPEN-API-75 P5, 2026-11-14):
//   - Create inquiry: POST https://api.withpersona.com/api/v1/inquiries with
//     Authorization: Bearer <api key> and JSON:API body
//     {"data":{"attributes":{"inquiry-template-id":"itmpl_...",
//     "reference-id":"usr_...","fields":{...}}}} → {"data":{"id":"inq_..."}}
//     https://docs.withpersona.com/2025-12-08/tutorial-hosted-flow-unique-api
//   - Hosted verification link: https://inquiry.withpersona.com/verify?inquiry-id=<id>
//   - Fetch result: GET /api/v1/inquiries/{id} → data.attributes.status
//     ("approved" / "rejected" are terminal; review states stay pending).
//   - Webhooks: the persona-signature header carries one or more
//     "t=<unix>,v1=<hex>" groups; v1 is the hex HMAC-SHA256 of
//     "<t>.<raw body>" keyed with the webhook secret.
//     https://docs.withpersona.com/2025-10-27/quickstart-webhooks
type PersonaVerifier struct {
	APIKey        string
	WebhookSecret string
	TemplateID    string
	BaseURL       string
	HostedBaseURL string
	client        *http.Client
}

var _ IdentityVerifier = (*PersonaVerifier)(nil)

func NewPersonaVerifier(apiKey, webhookSecret, templateID, endpoint string) *PersonaVerifier {
	baseURL := strings.TrimSpace(endpoint)
	if baseURL == "" {
		baseURL = personaDefaultEndpoint
	}
	return &PersonaVerifier{
		APIKey:        strings.TrimSpace(apiKey),
		WebhookSecret: strings.TrimSpace(webhookSecret),
		TemplateID:    strings.TrimSpace(templateID),
		BaseURL:       strings.TrimRight(baseURL, "/"),
		HostedBaseURL: personaHostedBaseURL,
		client:        safeClient(),
	}
}

func (v *PersonaVerifier) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	req, err := newJSONRequest(ctx, method, v.BaseURL+path, body)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "persona: build request")
	}
	req.Header.Set("Authorization", "Bearer "+v.APIKey)
	return req, nil
}

func (v *PersonaVerifier) InitiateVerification(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	if v.TemplateID == "" {
		return nil, apperrors.NewInvalidArgument("persona: inquiryTemplateId is not configured for this provider")
	}
	attributes := map[string]any{
		"inquiry-template-id": v.TemplateID,
	}
	if opts.UserID != "" {
		attributes["reference-id"] = opts.UserID
	}
	if first, last := splitFullName(opts.FullName); first != "" {
		fields := map[string]string{"name_first": first}
		if last != "" {
			fields["name_last"] = last
		}
		attributes["fields"] = fields
	}
	body, err := json.Marshal(map[string]any{
		"data": map[string]any{"attributes": attributes},
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "persona: marshal inquiry")
	}
	req, err := v.newRequest(ctx, http.MethodPost, "/inquiries", body)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "persona: create inquiry failed")
	}
	defer resp.Body.Close()
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(personaProviderName, resp.StatusCode)
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := decodeJSON(personaProviderName, raw, &created); err != nil {
		return nil, err
	}
	if created.Data.ID == "" {
		return nil, apperrors.NewUnavailable("persona: inquiry response is missing data.id")
	}
	return &InitiateResult{
		VerificationID: created.Data.ID,
		RedirectURL:    v.HostedBaseURL + "?inquiry-id=" + created.Data.ID,
		SessionToken:   created.Data.ID,
	}, nil
}

func (v *PersonaVerifier) GetVerificationResult(ctx context.Context, verificationID string) (*VerificationResult, error) {
	req, err := v.newRequest(ctx, http.MethodGet, "/inquiries/"+verificationID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "persona: result request failed")
	}
	defer resp.Body.Close()
	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(personaProviderName, resp.StatusCode)
	}
	var inquiry struct {
		Data struct {
			Attributes struct {
				Status string `json:"status"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := decodeJSON(personaProviderName, raw, &inquiry); err != nil {
		return nil, err
	}
	return &VerificationResult{
		VerificationID: verificationID,
		Status:         mapPersonaStatus(inquiry.Data.Attributes.Status),
		ProviderName:   personaProviderName,
	}, nil
}

func (v *PersonaVerifier) VerifyCallback(_ context.Context, payload []byte, headers map[string]string) (*VerificationResult, error) {
	if v.WebhookSecret == "" {
		// Fail closed: an unconfigured webhook secret must never degrade to "accept".
		return nil, apperrors.NewForbidden("persona: webhook secret is not configured")
	}
	signature := headerValue(headers, "Persona-Signature")
	if signature == "" {
		return nil, apperrors.NewUnauthorized("persona: webhook signature header is missing")
	}
	if !verifyPersonaSignature(payload, signature, v.WebhookSecret) {
		return nil, apperrors.NewUnauthorized("persona: webhook signature verification failed")
	}
	var event struct {
		Data struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
			Relationships struct {
				Inquiry struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"inquiry"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := decodeJSON(personaProviderName, payload, &event); err != nil {
		return nil, err
	}
	if event.Data.Relationships.Inquiry.Data.ID == "" {
		return nil, apperrors.NewInvalidArgument("persona: webhook event is missing the inquiry id")
	}
	return &VerificationResult{
		VerificationID: event.Data.Relationships.Inquiry.Data.ID,
		Status:         mapPersonaEvent(event.Data.Attributes.Name),
		ProviderName:   personaProviderName,
	}, nil
}

// Probe fetches a random inquiry id. 200/404 both prove the API key was
// accepted; 401/403 are surfaced as auth failures.
func (v *PersonaVerifier) Probe(ctx context.Context) error {
	probeID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "persona: generate probe id")
	}
	req, err := v.newRequest(ctx, http.MethodGet, "/inquiries/inq_"+probeID, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "persona: probe request failed")
	}
	defer resp.Body.Close()
	_, _ = readLimited(resp)
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNotFound:
		return nil
	default:
		return mapProviderHTTPError(personaProviderName, resp.StatusCode)
	}
}

// mapPersonaStatus maps an inquiry status onto the domain status. Only
// "approved"/"rejected" are terminal; review and in-progress states keep the
// row pending so polling continues.
func mapPersonaStatus(status string) VerificationStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "approved":
		return StatusApproved
	case "rejected":
		return StatusRejected
	default:
		return StatusPending
	}
}

// mapPersonaEvent maps a webhook event name (inquiry.approved,
// inquiry.failed, ...) onto the domain status.
func mapPersonaEvent(name string) VerificationStatus {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "inquiry.approved":
		return StatusApproved
	case "inquiry.failed", "inquiry.rejected":
		return StatusRejected
	default:
		return StatusPending
	}
}

// verifyPersonaSignature validates the persona-signature header: one or more
// space-separated "t=<unix>,v1=<hex>" groups, where v1 is the hex HMAC-SHA256
// of "<t>.<raw body>" keyed with the webhook secret. A signature is accepted
// when any group matches.
func verifyPersonaSignature(payload []byte, signature, secret string) bool {
	for _, group := range strings.Fields(signature) {
		var ts string
		var digest string
		for _, part := range strings.Split(group, ",") {
			key, value, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			switch strings.TrimSpace(key) {
			case "t":
				ts = strings.TrimSpace(value)
			case "v1":
				digest = strings.TrimSpace(value)
			}
		}
		if ts == "" || digest == "" {
			continue
		}
		if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
			continue
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(ts + "."))
		mac.Write(payload)
		expected := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(digest), []byte(expected)) {
			return true
		}
	}
	return false
}
