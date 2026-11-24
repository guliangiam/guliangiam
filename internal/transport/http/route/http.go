package route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
	"github.com/guliangiam/guliangiam/internal/platform/logging"
	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

func WriteJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if value == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("http route: json encode error", "error", err)
	}
}

// WriteCreated writes the success response of a create endpoint: 201 plus the
// Location header naming the canonical path of the new resource (RFC 9110
// §15.3.2), then the resource representation through WriteJSON so the body and
// Content-Type are byte-identical to every other JSON response.
//
// location is the handler's instantiated resource path (for example
// "/api/v1/users/" + id). An empty location leaves the header unset: a create
// whose command returns no identifier must not advertise a URL that would 404
// on the follow-up GET. Callers build the path from the id they already hold;
// this helper is the only place that writes the header.
func WriteCreated(w http.ResponseWriter, location string, body any) {
	if location != "" {
		w.Header().Set("Location", location)
	}
	WriteJSON(w, http.StatusCreated, body)
}

func DecodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		// The bootstrap wall wraps r.Body in http.MaxBytesReader. A body over
		// that cap must be 413 (手册 4.17.2), not a generic 400; the raw error
		// would otherwise fall through to invalid_argument because this
		// function rewrites every decode failure as such.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperrors.NewRequestEntityTooLargef("request body exceeds the %d byte limit", maxErr.Limit)
		}
		return apperrors.NewInvalidArgument(err.Error())
	}

	// BE-E-22: reject trailing data — `{}{}` must not decode as `{}`.
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return apperrors.NewInvalidArgument("unexpected trailing data after JSON body")
	}

	// Post-decode validation: if the target implements Validatable, call Validate()
	// now. This is how transport-layer size/format bounds get enforced uniformly —
	// every handler that does route.DecodeJSON gets it for free, and Request types
	// that don't implement Validatable simply skip this check (progressive adoption).
	//
	// Design notes:
	//   - Validation errors bubble up as 400 Bad Request via apperrors.NewInvalidArgument
	//   - Only non-nil targets are validated (nil means the JSON body was empty/null)
	//   - This sits AFTER DisallowUnknownFields so Validate sees a fully-decoded struct
	if target != nil {
		if v, ok := target.(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// WriteError writes a structured JSON error response. The response carries the
// request id (set by chi's RequestID middleware) and a non-empty trace id:
// the active OTel span's W3C trace id when tracing recorded one, otherwise the
// request id so the field is never empty on the wire.
//
// On the protocol face the body shape is dictated by the standard instead of by
// this project (手册 4.1.3, 错误码注册表 §2), so the call is delegated to
// WriteProtocolError before anything rewraps the error: wrapping would hide the
// protocol code an error may carry (apperrors.ProtocolCodeError).
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	err = mapBodyTooLarge(err)
	statusCode := StatusCodeFromError(err)
	if statusCode >= http.StatusInternalServerError {
		logServerError(r, err, statusCode)
	}
	writeRetryAfter(w.Header(), statusCode, err)
	if RequestMetadataFrom(r.Context()).Surface == SurfaceProtocol {
		WriteProtocolError(w, r, err)
		return
	}
	err = apperrors.WithStack(err)
	requestID, traceID := requestCorrelationIDs(r.Context())
	body := presenter.PresentError(err, requestID, traceID)
	WriteJSON(w, statusCode, body)
}

// writeRetryAfter surfaces the upstream delay recorded by the error producer.
// 错误码注册表 §1（too_many_requests, 429）要求 429 必须同时返回 Retry-After；
// 503 沿用同一机制（依赖不可用时给客户端退避指引）。其余状态码不写该头，避免
// 把 4xx 的业务拒绝误当成限流。值只来自上游（errors.WithRetryAfterSeconds），
// 解析失败时生产者已丢弃，这里绝不补默认值。
func writeRetryAfter(header http.Header, statusCode int, err error) {
	if statusCode != http.StatusTooManyRequests && statusCode != http.StatusServiceUnavailable {
		return
	}
	seconds, ok := apperrors.RetryAfterSeconds(err)
	if !ok {
		return
	}
	header.Set(ratelimit.HeaderRetryAfter, strconv.Itoa(seconds))
}

// mapBodyTooLarge turns the net/http read error raised by the bootstrap
// http.MaxBytesReader wall into the registered 413 code. Handlers that parse
// multipart forms hand the raw error to WriteError, and without this mapping it
// would fall through to CodeInternal and surface as a 500 instead of the
// contract's 413 (手册 4.17.2).
func mapBodyTooLarge(err error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return apperrors.NewRequestEntityTooLargef("request body exceeds the %d byte limit", maxErr.Limit)
	}
	return err
}

// logServerError is the single 5xx error record for route.WriteError. The
// access log keeps its own request line (status/duration/bytes); this record
// carries the error code and type instead, so the two never duplicate the same
// detail. The raw error message is deliberately omitted: it may carry SQL,
// constraint names or internal paths (错误码注册表 §1).
func logServerError(r *http.Request, err error, statusCode int) {
	ids := logging.RequestIDsFrom(r.Context())
	if ids.TraceID == "" {
		ids.TraceID = ids.RequestID
	}
	routePattern := chi.RouteContext(r.Context()).RoutePattern()
	if routePattern == "" {
		routePattern = r.URL.Path
	}

	attrs := make([]any, 0, 9)
	if ids.RequestID != "" {
		attrs = append(attrs, "request_id", ids.RequestID)
	}
	if ids.TraceID != "" {
		attrs = append(attrs, "trace_id", ids.TraceID)
	}
	if ids.SpanID != "" {
		attrs = append(attrs, "span_id", ids.SpanID)
	}
	attrs = append(attrs,
		"http.method", r.Method,
		"http.route", routePattern,
		"http.status", statusCode,
		"error.type", fmt.Sprintf("%T", err),
	)
	if code := errorCode(err); code != "" {
		attrs = append(attrs, "error.code", code)
	}
	slog.ErrorContext(r.Context(), "http server error", attrs...)
}

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return string(appErr.Code)
	}
	var protocolErr apperrors.ProtocolCodeError
	if errors.As(err, &protocolErr) {
		return protocolErr.ProtocolCode()
	}
	if StatusCodeFromError(err) >= http.StatusInternalServerError {
		return string(apperrors.CodeInternal)
	}
	return ""
}

// requestCorrelationIDs resolves the identifiers stamped on the request
// context. RequestMetadataMiddleware already captured both after RequestID and
// Tracing ran; traceIDFromContext is the fallback for callers that wire the
// writer without the metadata middleware. When tracing is not recording (and
// in unit tests) the request id keeps traceId non-empty instead of silently
// omitting it, so the error contract's 顶层 traceId stays satisfied.
func requestCorrelationIDs(ctx context.Context) (requestID, traceID string) {
	meta := RequestMetadataFrom(ctx)
	requestID = middleware.GetReqID(ctx)
	if requestID == "" {
		requestID = meta.RequestID
	}
	traceID = meta.TraceID
	if traceID == "" {
		traceID = traceIDFromContext(ctx)
	}
	if traceID == "" {
		traceID = requestID
	}
	return requestID, traceID
}

// HandleError is an alias for WriteError.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, r, err)
}

// StatusCodeFromError maps a platform/errors.Error to the corresponding HTTP status code.
// It delegates to platform/http so middleware denials and handler errors share one
// mapping; that helper in turn uses apperrors.Error.HTTPStatus(), keeping the mapping
// in sync with the central codeToHTTPStatus table in platform/errors, covering all
// codes including account_locked (423), password_expired (403), password_change_required (403), mfa_required (403),
// invitation_used (409), invitation_expired (410), and gone (410).
func StatusCodeFromError(err error) int {
	return httperr.StatusCodeFromError(err)
}
