package aiupstream

import (
	"context"
	"errors"
	"fmt"
	"net"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	transportFailureClass  = "transport failure"
	transportTimeoutClass  = "request timed out"
	transportCanceledClass = "request canceled"
)

// sanitizedTransportCause is an error whose rendered text is credential-free
// while its Unwrap chain still points at the original transport error. This is
// the safe replacement for fmt.Errorf("...: %w", err): the chain keeps
// errors.Is/errors.As semantics (context deadline/cancel, *url.Error), but the
// message never echoes the request URL — Gemini carries the plaintext API key
// in the ?key= query string, and *url.Error.Error() includes that URL.
type sanitizedTransportCause struct {
	class string
	err   error
}

func (e *sanitizedTransportCause) Error() string { return e.class }
func (e *sanitizedTransportCause) Unwrap() error { return e.err }

// sanitizeTransportError converts a transport-layer failure (typically a
// *url.Error from http.Client.Do) into a CodeUnavailable platform error whose
// message only names the family, operation and error class. Status mapping is
// unchanged: the caller still sees a 503.
func sanitizeTransportError(family, operation string, err error) error {
	if err == nil {
		return nil
	}
	class := transportFailureClass
	switch {
	case errors.Is(err, context.Canceled):
		class = transportCanceledClass
	case errors.Is(err, context.DeadlineExceeded):
		class = transportTimeoutClass
	default:
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			class = transportTimeoutClass
		}
	}
	cause := &sanitizedTransportCause{class: class, err: err}
	return apperrors.Wrap(apperrors.CodeUnavailable, cause, fmt.Sprintf("%s: %s", family, operation))
}
