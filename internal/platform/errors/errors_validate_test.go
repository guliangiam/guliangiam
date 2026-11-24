package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithStack(t *testing.T) {
	t.Run("with_app_error", func(t *testing.T) {
		err := New(CodeNotFound, "not found")
		stackErr := WithStack(err)
		assert.NotNil(t, stackErr)
		assert.Equal(t, CodeNotFound, stackErr.Code)
		assert.NotEmpty(t, stackErr.Stack, "stack should be captured")
	})

	t.Run("with_std_error", func(t *testing.T) {
		stdErr := errors.New("standard error")
		stackErr := WithStack(stdErr)
		assert.NotNil(t, stackErr)
		assert.Equal(t, CodeInternal, stackErr.Code)
		assert.NotEmpty(t, stackErr.Stack)
	})

	t.Run("nil_error", func(t *testing.T) {
		stackErr := WithStack(nil)
		assert.Nil(t, stackErr)
	})

	t.Run("idempotent_stack", func(t *testing.T) {
		err := New(CodeInternal, "error")
		stackErr1 := WithStack(err)
		stackErr2 := WithStack(err)
		assert.Equal(t, stackErr1.Stack, stackErr2.Stack, "calling WithStack twice should not overwrite")
	})
}

func TestError_Chain(t *testing.T) {
	t.Run("three_level_chain", func(t *testing.T) {
		base := errors.New("base error")
		middle := Wrap(CodeInternal, base, "middle error")
		top := Wrap(CodeUnavailable, middle, "top error")

		// Unwrap chain
		assert.ErrorIs(t, top, base)
		assert.ErrorIs(t, top, middle)

		// Error message format
		assert.Contains(t, top.Error(), "top error")
		assert.Contains(t, top.Error(), "middle error")
		assert.Contains(t, top.Error(), "base error")
	})

	t.Run("errors_as_works", func(t *testing.T) {
		err := New(CodeForbidden, "access denied")
		var appErr *Error
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, CodeForbidden, appErr.Code)
	})

	t.Run("wrapped_errors_as_works", func(t *testing.T) {
		appErr := New(CodeNotFound, "resource missing")
		stdErr := errors.New("wrapper")
		combined := Wrap(CodeInternal, stdErr, "wrapped: "+appErr.Error())
		var target *Error
		assert.True(t, errors.As(combined, &target))
	})
}

func TestError_Meta(t *testing.T) {
	t.Run("meta_preserved", func(t *testing.T) {
		meta := map[string]any{
			"userId": "user-001",
			"ip":     "192.168.1.1",
		}
		err := NewWithMeta(CodeForbidden, "access denied", meta)
		assert.Equal(t, "user-001", err.Meta["userId"])
		assert.Equal(t, "192.168.1.1", err.Meta["ip"])
	})

	t.Run("nil_meta", func(t *testing.T) {
		err := New(CodeNotFound, "not found")
		assert.Nil(t, err.Meta)
	})
}

func TestHTTPStatus_AllCodes(t *testing.T) {
	expectedMappings := map[Code]int{
		CodeInvalidArgument: http.StatusBadRequest,
		CodeNotFound:        http.StatusNotFound,
		CodeConflict:        http.StatusConflict,
		CodeAlreadyExists:   http.StatusConflict,
		CodeUnauthorized:    http.StatusUnauthorized,
		CodeForbidden:       http.StatusForbidden,
		CodeInternal:        http.StatusInternalServerError,
		CodeUnimplemented:   http.StatusNotImplemented,
		CodeUnavailable:     http.StatusServiceUnavailable,
		CodeTooManyRequests: http.StatusTooManyRequests,
	}

	for code, expectedStatus := range expectedMappings {
		t.Run(string(code), func(t *testing.T) {
			err := New(code, "test")
			assert.Equal(t, expectedStatus, err.HTTPStatus())
		})
	}
}

func TestFromError_NilInput(t *testing.T) {
	result := FromError(nil)
	assert.Nil(t, result)
}

func TestError_NilReceiver(t *testing.T) {
	var err *Error
	assert.Equal(t, "", err.Error())
	assert.Nil(t, err.Unwrap())
	assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus())
}
