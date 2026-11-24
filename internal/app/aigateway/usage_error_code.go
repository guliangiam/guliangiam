// usage_error_code.go — normalized error codes persisted for AI gateway usage.
package aigateway

import (
	"context"
	"errors"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// llm_usage_logs.error_code is VARCHAR(100).
const usageErrorCodeMaxLen = 100

// Persisted usage error codes. They reuse values already registered in
// contracts/registries/errors.yaml (AI gateway section) and the application
// codes in internal/platform/errors; no new registry entry is introduced.
const (
	usageErrorCodeUpstream        = "ai_upstream_error"
	usageErrorCodeUpstreamTimeout = "ai_upstream_timeout"
)

// normalizeUsageErrorCode reduces any error to a bounded, stable code. Raw
// error text is never persisted: upstream bodies can echo the Authorization
// header or Gemini's ?key= query, and llm_usage_logs / outbox payloads are
// readable through the management API.
func normalizeUsageErrorCode(err error) string {
	if err == nil {
		return ""
	}
	// Transport errors keep their cancellation/timeout chain, so the
	// distinction survives even though the rendered message is sanitized.
	if errors.Is(err, context.DeadlineExceeded) {
		return usageErrorCodeUpstreamTimeout
	}
	if errors.Is(err, context.Canceled) {
		return string(apperrors.CodeUnavailable)
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		if appErr.Code == apperrors.CodeUnavailable {
			return usageErrorCodeUpstream
		}
		return truncateUsageErrorCode(string(appErr.Code))
	}
	return string(apperrors.CodeInternal)
}

func truncateUsageErrorCode(code string) string {
	if len(code) <= usageErrorCodeMaxLen {
		return code
	}
	runes := []rune(code)
	if len(runes) <= usageErrorCodeMaxLen {
		return code
	}
	return string(runes[:usageErrorCodeMaxLen])
}
