package aiupstream

import (
	"context"
	"fmt"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Stream failure messages are fixed on purpose. The provider's raw error can
// echo request headers or a URL carrying an API key (Gemini's ?key=), so it
// stays out of both the SSE payload and the log attributes.
const (
	streamReadErrorMessage     = "upstream stream interrupted"
	streamReportedErrorMessage = "upstream stream error"
	streamErrorCodeUnavailable = string(apperrors.CodeUnavailable)
)

// streamErrorEvent builds the terminal event every adapter emits when the
// upstream stream cannot be read to completion.
func streamErrorEvent(message string) *domain.CanonicalStreamEvent {
	return &domain.CanonicalStreamEvent{
		Type: domain.StreamEventError,
		Error: &domain.CanonicalStreamError{
			Code:    streamErrorCodeUnavailable,
			Message: message,
		},
	}
}

// emitStreamError logs a sanitized diagnostic and sends the terminal error
// event. A cancelled request context means the caller is gone: no event, no
// log noise, and no blocking send.
func emitStreamError(
	ctx context.Context,
	ch chan<- *domain.CanonicalStreamEvent,
	provider, message string,
	cause error,
) {
	if ctx.Err() != nil {
		return
	}
	if cause != nil {
		// Only the concrete type is logged: err.Error() may embed credentials.
		slog.WarnContext(ctx, "ai upstream stream read failed", "provider", provider, "error_type", fmt.Sprintf("%T", cause))
	} else {
		slog.WarnContext(ctx, "ai upstream reported a stream error", "provider", provider)
	}

	select {
	case ch <- streamErrorEvent(message):
	case <-ctx.Done():
	}
}
