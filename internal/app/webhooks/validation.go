package webhooks

import (
	"encoding/json"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// validateWebhookURL enforces a non-empty absolute http(s) URL. The delivery
// worker only speaks HTTP(S); any other scheme (or a relative/empty value)
// would be persisted and then fail every delivery silently.
func validateWebhookURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return apperrors.NewInvalidArgument("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return apperrors.WrapInvalidArgument(err, "url is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "url scheme must be http or https", map[string]any{
			"field": "url", "value": u.Scheme, "allowed": []string{"http", "https"},
		})
	}
	if u.Hostname() == "" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "url must include a host", map[string]any{
			"field": "url", "value": raw, "expected": "absolute URL with a host",
		})
	}
	return nil
}

// validateWebhookHeaders accepts an empty string (no custom headers) or a
// JSON object with string values. The delivery worker decodes headers into
// map[string]string before delivery; arrays, scalars or non-string values
// would be persisted and then silently dropped on every delivery.
func validateWebhookHeaders(headers string) error {
	if strings.TrimSpace(headers) == "" {
		return nil
	}
	// json.Unmarshal accepts `null` into a nil map; require an actual object.
	if !strings.HasPrefix(strings.TrimSpace(headers), "{") {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "headers must be a JSON object with string values", map[string]any{
			"field": "headers", "format": "JSON object with string values",
		})
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(headers), &decoded); err != nil {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "headers must be a JSON object with string values", map[string]any{
			"field": "headers", "format": "JSON object with string values",
		})
	}
	return nil
}
