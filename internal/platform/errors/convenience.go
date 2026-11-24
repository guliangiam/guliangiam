package errors

import fmtstd "fmt"

// NewTooManyRequests creates a 429 Too Many Requests error.
func NewTooManyRequests(message string) *Error {
	return &Error{
		Code:    CodeTooManyRequests,
		Message: message,
	}
}

// NewAccountLocked creates a 423 Account Locked error.
func NewAccountLocked(message string) *Error {
	return &Error{
		Code:    CodeAccountLocked,
		Message: message,
	}
}

// NewPasswordExpired creates a 403 Password Expired error.
func NewPasswordExpired(message string) *Error {
	return &Error{
		Code:    CodePasswordExpired,
		Message: message,
	}
}

// NewMfaRequired creates a 403 MFA Required error.
func NewMfaRequired(message string) *Error {
	return &Error{
		Code:    CodeMfaRequired,
		Message: message,
	}
}

// NewInvitationUsed creates a 409 Invitation Used error.
func NewInvitationUsed(message string) *Error {
	return &Error{
		Code:    CodeInvitationUsed,
		Message: message,
	}
}

// NewInvitationExpired creates a 410 Invitation Expired error.
func NewInvitationExpired(message string) *Error {
	return &Error{
		Code:    CodeInvitationExpired,
		Message: message,
	}
}

// NewGone creates a 410 Gone error.
func NewGone(message string) *Error {
	return &Error{
		Code:    CodeGone,
		Message: message,
	}
}

// NewInvalidArgument creates a 400 Bad Request error.
func NewInvalidArgument(message string) *Error {
	return &Error{
		Code:    CodeInvalidArgument,
		Message: message,
	}
}

// NewNotFound creates a 404 Not Found error.
func NewNotFound(message string) *Error {
	return &Error{
		Code:    CodeNotFound,
		Message: message,
	}
}

// NewUnauthorized creates a 401 Unauthorized error.
func NewUnauthorized(message string) *Error {
	return &Error{
		Code:    CodeUnauthorized,
		Message: message,
	}
}

// NewForbidden creates a 403 Forbidden error.
func NewForbidden(message string) *Error {
	return &Error{
		Code:    CodeForbidden,
		Message: message,
	}
}

// NewConflict creates a 409 Conflict error.
func NewConflict(message string) *Error {
	return &Error{
		Code:    CodeConflict,
		Message: message,
	}
}

// NewPreconditionFailed creates a 412 Precondition Failed error (规范 4.10:
// If-Match/revision 与当前版本不符)。Callers that must surface the current
// revision use NewWithMeta(CodePreconditionFailed, …) instead.
func NewPreconditionFailed(message string) *Error {
	return &Error{
		Code:    CodePreconditionFailed,
		Message: message,
	}
}

// NewPreconditionRequired creates a 428 Precondition Required error (规范 4.10:
// 写路径要求乐观锁前提而请求未携带 If-Match/revision).
func NewPreconditionRequired(message string) *Error {
	return &Error{
		Code:    CodePreconditionRequired,
		Message: message,
	}
}

// NewAlreadyExists creates a 409 Already Exists error.
func NewAlreadyExists(message string) *Error {
	return &Error{
		Code:    CodeAlreadyExists,
		Message: message,
	}
}

// NewInternal creates a 500 Internal Server Error.
func NewInternal(message string) *Error {
	return &Error{
		Code:    CodeInternal,
		Message: message,
	}
}

// NewUnavailable creates a 503 Service Unavailable error.
func NewUnavailable(message string) *Error {
	return &Error{
		Code:    CodeUnavailable,
		Message: message,
	}
}

// NewUnimplemented creates a 501 Not Implemented error.
func NewUnimplemented(message string) *Error {
	return &Error{
		Code:    CodeUnimplemented,
		Message: message,
	}
}

// NewUnprocessableEntity creates a 422 Unprocessable Entity error: the request
// parsed and bound fine, but a rule rejected it (手册 4.14.1).
func NewUnprocessableEntity(message string) *Error {
	return &Error{
		Code:    CodeUnprocessableEntity,
		Message: message,
	}
}

// NewUnprocessableEntityf creates a 422 with format.
func NewUnprocessableEntityf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeUnprocessableEntity, Message: fmtstd.Sprintf(format, args...)}
}

// NewRequestEntityTooLarge creates a 413 Content Too Large error: the request
// body exceeded the configured cap (手册 4.17.2, STORAGE_UPLOAD_MAX_BYTES).
func NewRequestEntityTooLarge(message string) *Error {
	return &Error{Code: CodeRequestEntityTooLarge, Message: message}
}

// NewRequestEntityTooLargef creates a 413 with format.
func NewRequestEntityTooLargef(format string, args ...interface{}) *Error {
	return &Error{Code: CodeRequestEntityTooLarge, Message: fmtstd.Sprintf(format, args...)}
}

// NewUnsupportedMediaType creates a 415 Unsupported Media Type error. It is
// used by the OTLP protocol face, whose protobuf body cannot be parsed when the
// caller declares another media type.
func NewUnsupportedMediaType(message string) *Error {
	return &Error{Code: CodeUnsupportedMediaType, Message: message}
}

// --- Formatted convenience constructors (Newf) ---

// NewInvalidArgumentf creates a 400 Bad Request error with format.
func NewInvalidArgumentf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeInvalidArgument, Message: fmtstd.Sprintf(format, args...)}
}

// NewNotFoundf creates a 404 Not Found error with format.
func NewNotFoundf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeNotFound, Message: fmtstd.Sprintf(format, args...)}
}

// NewUnauthorizedf creates a 401 Unauthorized error with format.
func NewUnauthorizedf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeUnauthorized, Message: fmtstd.Sprintf(format, args...)}
}

// NewForbiddenf creates a 403 Forbidden error with format.
func NewForbiddenf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeForbidden, Message: fmtstd.Sprintf(format, args...)}
}

// NewConflictf creates a 409 Conflict error with format.
func NewConflictf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeConflict, Message: fmtstd.Sprintf(format, args...)}
}

// NewAlreadyExistsf creates a 409 Already Exists error with format.
func NewAlreadyExistsf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeAlreadyExists, Message: fmtstd.Sprintf(format, args...)}
}

// NewInternalf creates a 500 Internal Server Error with format.
func NewInternalf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeInternal, Message: fmtstd.Sprintf(format, args...)}
}

// NewUnavailablef creates a 503 Service Unavailable error with format.
func NewUnavailablef(format string, args ...interface{}) *Error {
	return &Error{Code: CodeUnavailable, Message: fmtstd.Sprintf(format, args...)}
}

// NewUnimplementedf creates a 501 Not Implemented error with format.
func NewUnimplementedf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeUnimplemented, Message: fmtstd.Sprintf(format, args...)}
}

// NewTooManyRequestsf creates a 429 Too Many Requests error with format.
func NewTooManyRequestsf(format string, args ...interface{}) *Error {
	return &Error{Code: CodeTooManyRequests, Message: fmtstd.Sprintf(format, args...)}
}

// --- Wrap convenience functions ---

// WrapInternal wraps an error with CodeInternal.
func WrapInternal(err error, message string) *Error {
	return &Error{Code: CodeInternal, Message: message, Cause: err}
}

// WrapInternalf wraps an error with CodeInternal and format.
func WrapInternalf(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeInternal, Message: fmtstd.Sprintf(format, args...), Cause: err}
}

// NewQuotaExceeded answers 429 with quota details (quota/limit/used/requested).
func NewQuotaExceeded(message string, meta map[string]any) *Error {
	return NewWithMeta(CodeQuotaExceeded, message, meta)
}

// WrapUnavailable wraps an error with CodeUnavailable (503 Service Unavailable).
// Use for upstream dependency failures (Redis ping, HTTP client Do, network errors).
func WrapUnavailable(err error, message string) *Error {
	return &Error{Code: CodeUnavailable, Message: message, Cause: err}
}

// WrapUnavailablef wraps an error with CodeUnavailable and format.
func WrapUnavailablef(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeUnavailable, Message: fmtstd.Sprintf(format, args...), Cause: err}
}

// WrapInvalidArgument wraps an error with CodeInvalidArgument.
func WrapInvalidArgument(err error, message string) *Error {
	return &Error{Code: CodeInvalidArgument, Message: message, Cause: err}
}

// WrapInvalidArgumentf wraps an error with CodeInvalidArgument and format.
func WrapInvalidArgumentf(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeInvalidArgument, Message: fmtstd.Sprintf(format, args...), Cause: err}
}

// WrapNotFound wraps an error with CodeNotFound.
func WrapNotFound(err error, message string) *Error {
	return &Error{Code: CodeNotFound, Message: message, Cause: err}
}

// WrapNotFoundf wraps an error with CodeNotFound and format.
func WrapNotFoundf(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeNotFound, Message: fmtstd.Sprintf(format, args...), Cause: err}
}

// WrapUnauthorized wraps an error with CodeUnauthorized.
func WrapUnauthorized(err error, message string) *Error {
	return &Error{Code: CodeUnauthorized, Message: message, Cause: err}
}

// WrapUnauthorizedf wraps an error with CodeUnauthorized and format.
func WrapUnauthorizedf(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeUnauthorized, Message: fmtstd.Sprintf(format, args...), Cause: err}
}

// WrapForbidden wraps an error with CodeForbidden.
func WrapForbidden(err error, message string) *Error {
	return &Error{Code: CodeForbidden, Message: message, Cause: err}
}

// WrapForbiddenf wraps an error with CodeForbidden and format.
func WrapForbiddenf(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeForbidden, Message: fmtstd.Sprintf(format, args...), Cause: err}
}

// WrapConflict wraps an error with CodeConflict.
func WrapConflict(err error, message string) *Error {
	return &Error{Code: CodeConflict, Message: message, Cause: err}
}

// WrapConflictf wraps an error with CodeConflict and format.
func WrapConflictf(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeConflict, Message: fmtstd.Sprintf(format, args...), Cause: err}
}

// ─────────────────────────────────────────────────────────────────────
// Validatable interface + Validate helper (P6 应用层 hardening)
// ─────────────────────────────────────────────────────────────────────

// Validatable is the contract for any value (Command, Request, DTO) that
// enforces its own business-level constraints. The interface intentionally
// lives in platform/errors because:
//
//   - Both transport-layer Request and app-layer Command use it
//   - Validation errors are always InvalidArgument (400) by default
//   - It's a zero-dependency contract — no import cycles possible
//
// Progressive adoption: types that don't implement Validatable simply skip
// validation. This lets us roll out validation incrementally — transport
// layer already triggers it in route.DecodeJSON, app layer can call
// Validate() explicitly in Service method entry points.
type Validatable interface {
	Validate() error
}

// ValidateCommand calls cmd.Validate() if cmd implements Validatable,
// otherwise returns nil silently. The returned error (if any) is expected
// to be a *Error with CodeInvalidArgument — ValidateCommand does not wrap
// or re-wrap it, because implementers should return the exact error they
// want the transport layer to surface.
//
// Usage in app-layer Service methods:
//
//	func (s Service) Login(ctx context.Context, cmd LoginCommand) (*LoginResponse, error) {
//	    if err := apperrors.ValidateCommand(cmd); err != nil {
//	        return nil, err
//	    }
//	    // ... business logic
//	}
func ValidateCommand(cmd any) error {
	if cmd == nil {
		return nil
	}
	if v, ok := cmd.(Validatable); ok {
		return v.Validate()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────
// MessageKey 变体（i18n 细粒度 key 支持）
// ─────────────────────────────────────────────────────────────────────

// NewWithMessageKey 创建带细粒度 i18n messageKey 的 Error。
// messageKey 比 Code 更具体（同一 Code 下多种用户可见提示时用）。
// 前端翻译优先级：messageKey > Code > Message。
func NewWithMessageKey(code Code, messageKey, message string) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		MessageKey: messageKey,
	}
}
