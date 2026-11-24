package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeNotFound, "resource not found")
	if err.Code != CodeNotFound {
		t.Errorf("Code = %q, want %q", err.Code, CodeNotFound)
	}
	if err.Message != "resource not found" {
		t.Errorf("Message = %q, want %q", err.Message, "resource not found")
	}
	if err.Cause != nil {
		t.Errorf("Cause = %v, want nil", err.Cause)
	}
}

func TestNewf(t *testing.T) {
	err := Newf(CodeInvalidArgument, "field %s is required", "email")
	if err.Code != CodeInvalidArgument {
		t.Errorf("Code = %q, want %q", err.Code, CodeInvalidArgument)
	}
	if err.Message != "field email is required" {
		t.Errorf("Message = %q, want %q", err.Message, "field email is required")
	}
}

func TestNewWithMeta(t *testing.T) {
	meta := map[string]any{"key": "value"}
	err := NewWithMeta(CodeConflict, "duplicate entry", meta)
	if err.Code != CodeConflict {
		t.Errorf("Code = %q, want %q", err.Code, CodeConflict)
	}
	if err.Meta["key"] != "value" {
		t.Errorf("Meta[key] = %v, want %v", err.Meta["key"], "value")
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("connection refused")
	err := Wrap(CodeInternal, inner, "database error")
	if err.Code != CodeInternal {
		t.Errorf("Code = %q, want %q", err.Code, CodeInternal)
	}
	if err.Cause != inner {
		t.Errorf("Cause = %v, want %v", err.Cause, inner)
	}
	if err.Error() != "database error: connection refused" {
		t.Errorf("Error() = %q, want %q", err.Error(), "database error: connection refused")
	}
}

func TestWrapf(t *testing.T) {
	inner := errors.New("timeout")
	err := Wrapf(CodeUnavailable, inner, "service %s", "auth")
	if err.Code != CodeUnavailable {
		t.Errorf("Code = %q, want %q", err.Code, CodeUnavailable)
	}
	if err.Message != "service auth" {
		t.Errorf("Message = %q, want %q", err.Message, "service auth")
	}
}

func TestError_ErrorMethod(t *testing.T) {
	err := &Error{Code: CodeNotFound, Message: "not found"}
	if err.Error() != "not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "not found")
	}

	errWithCause := &Error{Code: CodeInternal, Message: "internal", Cause: NewInternal("db error")}
	if errWithCause.Error() != "internal: db error" {
		t.Errorf("Error() = %q, want %q", errWithCause.Error(), "internal: db error")
	}
}

func TestError_ErrorMethod_Nil(t *testing.T) {
	var err *Error
	if err.Error() != "" {
		t.Errorf("nil Error.Error() = %q, want empty", err.Error())
	}
}

func TestError_Unwrap(t *testing.T) {
	inner := errors.New("root cause")
	err := Wrap(CodeInternal, inner, "wrapper")
	if unwrapped := err.Unwrap(); unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}

	var nilErr *Error
	if nilErr.Unwrap() != nil {
		t.Error("nil Error.Unwrap() should return nil")
	}
}

func TestError_HTTPStatus(t *testing.T) {
	tests := []struct {
		code     Code
		expected int
	}{
		{CodeInvalidArgument, http.StatusBadRequest},
		{CodeNotFound, http.StatusNotFound},
		{CodeConflict, http.StatusConflict},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeInternal, http.StatusInternalServerError},
		{CodeUnimplemented, http.StatusNotImplemented},
		{CodeUnavailable, http.StatusServiceUnavailable},
		{CodeTooManyRequests, http.StatusTooManyRequests},
	}
	for _, tc := range tests {
		err := New(tc.code, "test")
		if got := err.HTTPStatus(); got != tc.expected {
			t.Errorf("HTTPStatus(%q) = %d, want %d", tc.code, got, tc.expected)
		}
	}
}

func TestError_HTTPStatus_Nil(t *testing.T) {
	var err *Error
	if got := err.HTTPStatus(); got != http.StatusInternalServerError {
		t.Errorf("nil HTTPStatus() = %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestError_HTTPStatus_UnknownCode(t *testing.T) {
	err := New(Code("custom_code"), "test")
	if got := err.HTTPStatus(); got != http.StatusInternalServerError {
		t.Errorf("unknown code HTTPStatus() = %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestFromError(t *testing.T) {
	appErr := New(CodeNotFound, "not found")
	result := FromError(appErr)
	if result != appErr {
		t.Error("FromError should return same error for *Error input")
	}
}

func TestFromError_StdError(t *testing.T) {
	stdErr := errors.New("standard error")
	result := FromError(stdErr)
	if result.Code != CodeInternal {
		t.Errorf("Code = %q, want %q", result.Code, CodeInternal)
	}
	if result.Cause != stdErr {
		t.Errorf("Cause = %v, want %v", result.Cause, stdErr)
	}
}

func TestFromError_Nil(t *testing.T) {
	result := FromError(nil)
	if result != nil {
		t.Errorf("FromError(nil) = %v, want nil", result)
	}
}

func TestRegisterCode(t *testing.T) {
	customCode := Code("custom_418")
	RegisterCode(customCode, http.StatusTeapot)
	err := New(customCode, "I'm a teapot")
	if got := err.HTTPStatus(); got != http.StatusTeapot {
		t.Errorf("HTTPStatus(custom) = %d, want %d", got, http.StatusTeapot)
	}
}

func TestErrors_As(t *testing.T) {
	err := New(CodeForbidden, "access denied")
	var appErr *Error
	if !errors.As(err, &appErr) {
		t.Error("errors.As should match *Error")
	}
	if appErr.Code != CodeForbidden {
		t.Errorf("Code = %q, want %q", appErr.Code, CodeForbidden)
	}
}

func TestErrors_Is(t *testing.T) {
	inner := errors.New("base")
	wrapped := Wrap(CodeInternal, inner, "wrapper")
	if !errors.Is(wrapped, inner) {
		t.Error("errors.Is should match wrapped cause")
	}
}
