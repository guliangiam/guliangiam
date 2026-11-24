package notification

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BarkNotifier sends push notifications via Bark (iOS).
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：推送 URL 形态
// GET https://api.day.app/{key}/{body}：https://github.com/FinlayChan/Bark#readme
type BarkNotifier struct {
	HTTPClient *http.Client
}

// NewBarkNotifier creates a new Bark notifier.
func NewBarkNotifier() *BarkNotifier {
	return &BarkNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

func (n *BarkNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("bark notification: device key is required")
	}

	// Bark API URL format: https://api.day.app/{key}/{body}; path segments
	// must be percent-encoded so spaces/newlines/slashes survive.
	apiURL := fmt.Sprintf("https://api.day.app/%s/%s", url.PathEscape(notif.Recipient), url.PathEscape(notif.Content))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "bark notification: create request")
	}

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "bark notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("bark notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
