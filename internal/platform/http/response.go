package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ErrorBody is the canonical error object carried under the top-level "error"
// key. Both route.WriteError (management/authentication faces) and platform
// middleware write this shape; the protocol face keeps the shape dictated by
// its standard instead.
type ErrorBody struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	MessageKey string `json:"messageKey,omitempty"`
	// Details is the contract field (开发规范 4.1.2 / 4.1.4): structured,
	// machine-readable detail that lets a caller locate the offending field.
	Details map[string]any `json:"details,omitempty"`
	// Meta is a deprecated compatibility alias that always carries the same
	// value as Details. It stays on the wire until the announced window closes:
	// no first-party consumer reads error.meta (web/src), and external clients
	// have migrated to error.details. Removal is a breaking change and must be
	// registered in CHANGELOG.md before it happens.
	Meta map[string]any `json:"meta,omitempty"`
}

// ErrorResponse is the canonical error envelope. RequestID and TraceID are
// always populated by WriteError: when tracing recorded no span the request id
// keeps traceId non-empty.
type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"requestId,omitempty"`
	TraceID   string    `json:"traceId,omitempty"`
}

// InternalErrorMessage is the only message a 500 may carry. The code stays
// stable for i18n; driver text, SQL state, paths and constraint names stay in
// the server log (错误码注册表 §1).
const InternalErrorMessage = "an internal error occurred"

// PresentError builds the canonical error envelope. It lives in platform/http
// so middleware can emit the same shape without importing the transport layer;
// presenter.PresentError delegates here.
func PresentError(err error, requestID, traceID string) ErrorResponse {
	if err == nil {
		return ErrorResponse{}
	}

	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		// Details and Meta deliberately point at the same map: the canonical
		// field and its deprecated alias must never drift during the
		// compatibility window.
		body := ErrorBody{
			Code:       string(appErr.Code),
			Message:    appErr.Message,
			MessageKey: appErr.MessageKey,
			Details:    appErr.Meta,
			Meta:       appErr.Meta,
		}
		if appErr.Code == apperrors.CodeInternal {
			// A CodeInternal error still carries its code so callers can branch
			// on 500, but neither its message nor its structured detail may reach
			// the client: WrapInternalf and driver errors put those details in
			// Message.
			body.Message = InternalErrorMessage
			body.MessageKey = ""
			body.Details = nil
			body.Meta = nil
		}
		return ErrorResponse{
			Error:     body,
			RequestID: requestID,
			TraceID:   traceID,
		}
	}

	return ErrorResponse{
		Error: ErrorBody{
			Code:    string(apperrors.CodeInternal),
			Message: InternalErrorMessage,
		},
		RequestID: requestID,
		TraceID:   traceID,
	}
}

// CorrelationIDs resolves the identifiers stamped on the request context. The
// chi RequestID middleware supplies requestId; the active OTel span supplies
// traceId. When tracing is not recording the request id keeps traceId non-empty
// (same priority as route.WriteError, which additionally prefers the values
// RequestMetadataMiddleware captured from these two middlewares).
func CorrelationIDs(ctx context.Context) (requestID, traceID string) {
	requestID = chimw.GetReqID(ctx)
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	if traceID == "" {
		traceID = requestID
	}
	return requestID, traceID
}

// StatusCodeFromError maps an error to the HTTP status code. It mirrors the
// transport helper of the same name so middleware denials answer the same
// status as handler errors.
func StatusCodeFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if appErr, ok := err.(*apperrors.Error); ok {
		return appErr.HTTPStatus()
	}
	var withStatus interface{ HTTPStatus() int }
	if errors.As(err, &withStatus) {
		return withStatus.HTTPStatus()
	}
	return http.StatusInternalServerError
}

// WriteError writes the canonical JSON error envelope. Middleware uses it
// instead of writing an ad-hoc body, so middleware denials (CSRF 403,
// idempotency 400/409/503, rate-limit 429 …) carry the same fields as
// route.WriteError, including a non-empty traceId.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	err = apperrors.WithStack(err)
	requestID, traceID := CorrelationIDs(r.Context())
	writeJSON(w, StatusCodeFromError(err), PresentError(err, requestID, traceID))
}

// writeJSON is the platform-local encoder: platform may not import the
// transport writer (Rule 7), and the header/encoding contract is two lines.
func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if value == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("http: json encode error", "error", err)
	}
}
