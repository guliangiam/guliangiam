package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// plivoDefaultBaseURL is the Plivo Message API host.
const plivoDefaultBaseURL = "https://api.plivo.com"

// PlivoSmsSender sends SMS through the Plivo Message API
// (POST /v1/Account/{auth_id}/Message/ with HTTP Basic auth).
type PlivoSmsSender struct {
	AuthID    string
	AuthToken string
	Src       string
	BaseURL   string
	client    *http.Client
}

// NewPlivoSmsSender creates a Plivo SMS sender.
func NewPlivoSmsSender(authID, authToken, src string) *PlivoSmsSender {
	return &PlivoSmsSender{
		AuthID:    authID,
		AuthToken: authToken,
		Src:       src,
		BaseURL:   plivoDefaultBaseURL,
		client:    ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send posts a verification SMS to the Plivo Message API.
func (s *PlivoSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.AuthID == "" || s.AuthToken == "" {
		return apperrors.NewInvalidArgument("plivo authId and authToken are required")
	}

	payload := map[string]any{
		"src":     s.Src,
		"dst":     []string{phone},
		fieldText: "Your verification code is: " + code,
	}
	body, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "plivo: marshal")
	}

	apiURL := s.BaseURL + "/v1/Account/" + url.PathEscape(s.AuthID) + "/Message/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, jsonReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "plivo: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(s.AuthID, s.AuthToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "plivo: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "plivo: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("plivo: rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("plivo: API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		ErrorCode    *int     `json:"error_code"`
		ErrorMessage string   `json:"error_message"`
		MessageUUID  []string `json:"message_uuid"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return apperrors.WrapInternalf(err, "plivo: decode response")
	}
	if result.ErrorCode != nil && *result.ErrorCode != 0 {
		return apperrors.NewInternalf("plivo: send failed (code %d, %s)", *result.ErrorCode, result.ErrorMessage)
	}
	return nil
}

var _ SmsSender = (*PlivoSmsSender)(nil)
