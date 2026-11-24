package errors

import (
	fmtstd "fmt"
	"net/http"
	"runtime"
	"sync"
)

type Code string

const (
	CodeInvalidArgument Code = "invalid_argument"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeUnauthorized    Code = "unauthorized"
	CodeForbidden       Code = "forbidden"
	CodeInternal        Code = "internal"
	CodeUnimplemented   Code = "unimplemented"
	CodeAlreadyExists   Code = "already_exists"
	CodeUnavailable     Code = "unavailable"
	CodeTooManyRequests Code = "too_many_requests"
	// CodeQuotaExceeded is the tenant-quota hard limit (429, distinct from
	// request-rate limiting so clients can surface quota details and stop
	// retrying until an operator raises the limit).
	CodeQuotaExceeded   Code = "quota_exceeded"
	CodeAccountLocked   Code = "account_locked"
	CodePasswordExpired Code = "password_expired"
	// CodePasswordChangeRequired separates "an administrator forced a password
	// change (initial/reset password)" from "the password exceeded the
	// organization's max age". Both answer 403, but clients must show different
	// copy and must not treat them as interchangeable.
	CodePasswordChangeRequired Code = "password_change_required"
	CodeMfaRequired            Code = "mfa_required"
	CodeInvitationUsed         Code = "invitation_used"
	CodeInvitationExpired      Code = "invitation_expired"
	CodeGone                   Code = "gone"
	// CodeUnprocessableEntity separates "the request is malformed" from "the
	// request is well-formed but semantically invalid" (手册 4.14.1). Without it
	// both collapsed onto 400, so a client could not tell a retry-is-pointless
	// envelope error from a change-one-field-and-succeed validation error, and the
	// manual's 422 table (4.6.1 时间格式、4.7.1 pageSize>100、4.8.1 非法排序字段、
	// 4.14.3 枚举闭集) had nowhere to land.
	CodeUnprocessableEntity Code = "unprocessable_entity"
	// CodeRequestEntityTooLarge maps to 413 Content Too Large (RFC 9110 §15.5.14).
	// The upload contract (手册 4.17.2) requires a body over STORAGE_UPLOAD_MAX_BYTES
	// to be answered with 413; without a code the net/http *http.MaxBytesError would
	// fall through to CodeInternal and surface as a 500.
	CodeRequestEntityTooLarge Code = "request_entity_too_large"
	// CodeUnsupportedMediaType maps to 415 Unsupported Media Type (RFC 9110
	// §15.5.16). The protocol faces that carry a non-JSON body (OTLP protobuf)
	// need the 415 status distinct from 400: the body may be well-formed, only
	// its media type is not accepted. WriteError derives the status from the
	// code, so without this entry the only way to answer 415 was a hand-written
	// text body, which broke the unified envelope.
	CodeUnsupportedMediaType Code = "unsupported_media_type"
	// CodePreconditionFailed maps to 412 Precondition Failed (RFC 9110
	// §15.5.13): the If-Match/revision precondition does not match the current
	// version (规范 4.10). Meta carries the current revision so the client can
	// refetch and retry; 409 stays reserved for non-version conflicts.
	CodePreconditionFailed Code = "precondition_failed"
	// CodePreconditionRequired maps to 428 Precondition Required (RFC 6585 §3):
	// the write path requires an optimistic-lock precondition but the request
	// carried neither If-Match nor a body revision (规范 4.10). Last-write-wins
	// is not an allowed default on editable resources.
	CodePreconditionRequired Code = "precondition_required"
)

const (
	// 支付域错误码：见 design/commerce/payment.md §12.1 与错误码注册表 application 节。
	CodePaymentReceiptStorageFailed Code = "payment_receipt_storage_failed"
	CodePaymentVerificationFailed   Code = "payment_verification_failed"
	CodePaymentRefundExceedsBalance Code = "payment_refund_exceeds_balance"
	CodePaymentRefundRequired       Code = "payment_refund_required"
	CodePaymentChannelUnavailable   Code = "payment_channel_unavailable"
	CodePaymentChannelConfigInvalid Code = "payment_channel_config_invalid"
	CodePaymentChannelNotActivated  Code = "payment_channel_not_activated"
	CodePaymentConsistencyMismatch  Code = "payment_consistency_mismatch"
	CodePaymentInvalidTransition    Code = "payment_invalid_transition"
)

var (
	codeToHTTPStatus = map[Code]int{
		CodeInvalidArgument:             http.StatusBadRequest,
		CodeNotFound:                    http.StatusNotFound,
		CodeConflict:                    http.StatusConflict,
		CodeUnauthorized:                http.StatusUnauthorized,
		CodeForbidden:                   http.StatusForbidden,
		CodeInternal:                    http.StatusInternalServerError,
		CodeUnimplemented:               http.StatusNotImplemented,
		CodeAlreadyExists:               http.StatusConflict,
		CodeUnavailable:                 http.StatusServiceUnavailable,
		CodeTooManyRequests:             http.StatusTooManyRequests,
		CodeQuotaExceeded:               http.StatusTooManyRequests,
		CodeAccountLocked:               423, // Locked (RFC 4918)
		CodePasswordExpired:             http.StatusForbidden,
		CodePasswordChangeRequired:      http.StatusForbidden,
		CodeMfaRequired:                 http.StatusForbidden,
		CodeInvitationUsed:              http.StatusConflict,
		CodeInvitationExpired:           410, // Gone
		CodeGone:                        410, // Gone
		CodeUnprocessableEntity:         http.StatusUnprocessableEntity,
		CodeRequestEntityTooLarge:       http.StatusRequestEntityTooLarge,
		CodeUnsupportedMediaType:        http.StatusUnsupportedMediaType,
		CodePreconditionFailed:          http.StatusPreconditionFailed,
		CodePreconditionRequired:        428, // Precondition Required (RFC 6585 §3)
		CodePaymentReceiptStorageFailed: http.StatusInternalServerError,
		CodePaymentVerificationFailed:   http.StatusBadRequest,
		CodePaymentRefundExceedsBalance: http.StatusUnprocessableEntity,
		CodePaymentRefundRequired:       http.StatusUnprocessableEntity,
		CodePaymentChannelUnavailable:   http.StatusServiceUnavailable,
		CodePaymentChannelConfigInvalid: http.StatusBadRequest,
		CodePaymentChannelNotActivated:  http.StatusUnprocessableEntity,
		CodePaymentConsistencyMismatch:  http.StatusBadRequest,
		CodePaymentInvalidTransition:    http.StatusBadRequest,
	}
	codeMu sync.RWMutex
)

func RegisterCode(code Code, httpStatus int) {
	codeMu.Lock()
	defer codeMu.Unlock()
	codeToHTTPStatus[code] = httpStatus
}

type Error struct {
	// Code 是稳定 i18n key（invalid_argument / not_found / mfa_required …）。
	// 前端用 Code 查翻译表；Presenter 序列化为 JSON error.code。
	Code Code
	// Message 是英文 fallback（调试友好）。生产前端应优先用 Code 翻译，
	// 仅当翻译表缺失该 Code 时展示 Message。
	Message string
	// MessageKey 可选：比 Code 更细粒度的 i18n key。
	// 当同一个 Code 下需要区分多种用户可见提示时用它。
	// Presenter 序列化为 JSON error.messageKey；前端翻译优先级 messageKey > Code > Message。
	MessageKey string
	Cause      error
	Meta       map[string]any
	Stack      string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *Error) HTTPStatus() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	codeMu.RLock()
	status, ok := codeToHTTPStatus[e.Code]
	codeMu.RUnlock()
	if ok {
		return status
	}
	return http.StatusInternalServerError
}

func New(code Code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

func Newf(code Code, format string, args ...interface{}) *Error {
	return &Error{
		Code:    code,
		Message: fmtstd.Sprintf(format, args...),
	}
}

func NewWithMeta(code Code, message string, meta map[string]any) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Meta:    meta,
	}
}

func Wrap(code Code, err error, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

func Wrapf(code Code, err error, format string, args ...interface{}) *Error {
	return &Error{
		Code:    code,
		Message: fmtstd.Sprintf(format, args...),
		Cause:   err,
	}
}

func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return &Error{
		Code:    CodeInternal,
		Message: err.Error(),
		Cause:   err,
	}
}

func WithStack(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		if e.Stack == "" {
			e.Stack = captureStack(2)
		}
		return e
	}
	return &Error{
		Code:    CodeInternal,
		Message: err.Error(),
		Cause:   err,
		Stack:   captureStack(2),
	}
}

// ProtocolCodeError is implemented by errors that must be rendered with a code
// from the protocol namespace (错误码注册表 §2, e.g. `unsupported_response_type`)
// instead of an application code (错误码注册表 §1). The interface lives here, in the
// lowest layer both sides already depend on, so an application-layer service can
// signal a protocol outcome without importing the HTTP package that writes it.
//
// The writer on the protocol face duck-types this interface; the application
// namespace is untouched, and an error that does not implement it keeps its
// regular application code.
type ProtocolCodeError interface {
	error
	// ProtocolCode returns the RFC-defined code string.
	ProtocolCode() string
}

func captureStack(skip int) string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+1, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	var buf []byte
	for {
		frame, more := frames.Next()
		if buf != nil {
			buf = append(buf, '\n')
		}
		buf = append(buf, fmtstd.Sprintf("%s\n\t%s:%d", frame.Function, frame.File, frame.Line)...)
		if !more {
			break
		}
	}
	return string(buf)
}
