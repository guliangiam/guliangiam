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

// Sinch Conversation API endpoints. The product authenticates with OAuth2
// client credentials (key id + key secret) and sends SMS through the project
// scoped messages endpoint.
const (
	sinchDefaultTokenURL = "https://auth.sinch.com/oauth/token"
	sinchDefaultBaseURL  = "https://conversation.api.sinch.com"
)

// SinchSmsSender sends SMS through the Sinch Conversation API (transport=sms).
type SinchSmsSender struct {
	KeyID     string
	KeySecret string
	ProjectID string
	From      string
	TokenURL  string
	BaseURL   string
	client    *http.Client
}

// NewSinchSmsSender creates a Sinch Conversation API sender.
func NewSinchSmsSender(keyID, keySecret, projectID, from, baseURL string) *SinchSmsSender {
	return &SinchSmsSender{
		KeyID:     keyID,
		KeySecret: keySecret,
		ProjectID: projectID,
		From:      from,
		TokenURL:  sinchDefaultTokenURL,
		BaseURL:   strings.TrimRight(baseURL, "/"),
		client:    ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// fetchToken exchanges the access key for a short-lived OAuth2 bearer token.
func (s *SinchSmsSender) fetchToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", s.KeyID)
	form.Set("client_secret", s.KeySecret)
	form.Set("scope", "sms")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "sinch: create token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "sinch: token request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "sinch: read token response")
	}
	if resp.StatusCode >= 400 {
		return "", apperrors.NewInternalf("sinch: token error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &token); err != nil {
		return "", apperrors.WrapInternalf(err, "sinch: decode token response")
	}
	if token.AccessToken == "" {
		return "", apperrors.NewInternalf("sinch: empty access token")
	}
	return token.AccessToken, nil
}

// Send exchanges credentials and dispatches one verification SMS.
func (s *SinchSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.KeyID == "" || s.KeySecret == "" || s.ProjectID == "" {
		return apperrors.NewInvalidArgument("sinch keyId, keySecret and projectId are required")
	}
	base := s.BaseURL
	if base == "" {
		base = sinchDefaultBaseURL
	}

	accessToken, err := s.fetchToken(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		fieldFrom:   s.From,
		"to":        phone,
		fieldText:   map[string]any{fieldText: "Your verification code is: " + code},
		"transport": "sms",
	}
	body, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "sinch: marshal")
	}

	apiURL := base + "/v1/projects/" + url.PathEscape(s.ProjectID) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, jsonReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "sinch: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "sinch: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "sinch: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("sinch: rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("sinch: API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		Messages []struct {
			MessageID string `json:"messageId"`
		} `json:"messages"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return apperrors.WrapInternalf(err, "sinch: decode response")
	}
	if result.Error != nil {
		return apperrors.NewInternalf("sinch: send failed (code %d, %s)", result.Error.Code, result.Error.Message)
	}
	if len(result.Messages) == 0 {
		return apperrors.NewInternalf("sinch: empty response")
	}
	return nil
}

var _ SmsSender = (*SinchSmsSender)(nil)
