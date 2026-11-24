package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// birdDefaultBaseURL is the MessageBird (now Bird) Messages REST host.
const birdDefaultBaseURL = "https://rest.messagebird.com"

// BirdSmsSender sends SMS through the Bird / MessageBird Messages API
// (POST /messages with the legacy "AccessKey <key>" authorization header).
type BirdSmsSender struct {
	AccessKey string
	Origin    string
	BaseURL   string
	client    *http.Client
}

// NewBirdSmsSender creates a Bird (MessageBird) SMS sender.
func NewBirdSmsSender(accessKey, origin string) *BirdSmsSender {
	return &BirdSmsSender{
		AccessKey: accessKey,
		Origin:    origin,
		BaseURL:   birdDefaultBaseURL,
		client:    ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send posts a verification SMS to the Bird Messages API.
func (s *BirdSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.AccessKey == "" {
		return apperrors.NewInvalidArgument("bird accessKey is required")
	}

	payload := map[string]any{
		"origin":     s.Origin,
		"recipients": []string{phone},
		"body":       "Your verification code is: " + code,
	}
	body, err := jsonMarshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "bird: marshal")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/messages", jsonReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "bird: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "AccessKey "+s.AccessKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "bird: request")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapInternalf(err, "bird: read response")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("bird: rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("bird: API error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return apperrors.WrapInternalf(err, "bird: decode response")
	}
	if result.ID == "" {
		return apperrors.NewInternalf("bird: missing message id in response")
	}
	return nil
}

var _ SmsSender = (*BirdSmsSender)(nil)
