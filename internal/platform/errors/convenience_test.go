package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTooManyRequests(t *testing.T) {
	err := NewTooManyRequests("rate limited")
	assert.Equal(t, CodeTooManyRequests, err.Code)
	assert.Equal(t, "rate limited", err.Message)
	assert.Nil(t, err.Cause)
}

func TestNewAccountLocked(t *testing.T) {
	err := NewAccountLocked("account frozen")
	assert.Equal(t, CodeAccountLocked, err.Code)
	assert.Equal(t, "account frozen", err.Message)
}

func TestNewPasswordExpired(t *testing.T) {
	err := NewPasswordExpired("password too old")
	assert.Equal(t, CodePasswordExpired, err.Code)
	assert.Equal(t, "password too old", err.Message)
}

func TestNewMfaRequired(t *testing.T) {
	err := NewMfaRequired("mfa needed")
	assert.Equal(t, CodeMfaRequired, err.Code)
	assert.Equal(t, "mfa needed", err.Message)
}

func TestNewInvitationUsed(t *testing.T) {
	err := NewInvitationUsed("already used")
	assert.Equal(t, CodeInvitationUsed, err.Code)
	assert.Equal(t, "already used", err.Message)
}

func TestNewInvitationExpired(t *testing.T) {
	err := NewInvitationExpired("invitation expired")
	assert.Equal(t, CodeInvitationExpired, err.Code)
	assert.Equal(t, "invitation expired", err.Message)
}

func TestNewGone(t *testing.T) {
	err := NewGone("resource gone")
	assert.Equal(t, CodeGone, err.Code)
	assert.Equal(t, "resource gone", err.Message)
}

func TestNewInvalidArgument(t *testing.T) {
	err := NewInvalidArgument("bad input")
	assert.Equal(t, CodeInvalidArgument, err.Code)
	assert.Equal(t, "bad input", err.Message)
}

func TestNewNotFound(t *testing.T) {
	err := NewNotFound("not here")
	assert.Equal(t, CodeNotFound, err.Code)
	assert.Equal(t, "not here", err.Message)
}

func TestNewUnauthorized(t *testing.T) {
	err := NewUnauthorized("no auth")
	assert.Equal(t, CodeUnauthorized, err.Code)
	assert.Equal(t, "no auth", err.Message)
}

func TestNewForbidden(t *testing.T) {
	err := NewForbidden("no access")
	assert.Equal(t, CodeForbidden, err.Code)
	assert.Equal(t, "no access", err.Message)
}

func TestNewConflict(t *testing.T) {
	err := NewConflict("duplicate")
	assert.Equal(t, CodeConflict, err.Code)
	assert.Equal(t, "duplicate", err.Message)
}

func TestNewAlreadyExists(t *testing.T) {
	err := NewAlreadyExists("already there")
	assert.Equal(t, CodeAlreadyExists, err.Code)
	assert.Equal(t, "already there", err.Message)
}

func TestNewInternal(t *testing.T) {
	err := NewInternal("server error")
	assert.Equal(t, CodeInternal, err.Code)
	assert.Equal(t, "server error", err.Message)
}

func TestNewUnavailable(t *testing.T) {
	err := NewUnavailable("service down")
	assert.Equal(t, CodeUnavailable, err.Code)
	assert.Equal(t, "service down", err.Message)
}

func TestNewUnimplemented(t *testing.T) {
	err := NewUnimplemented("not done yet")
	assert.Equal(t, CodeUnimplemented, err.Code)
	assert.Equal(t, "not done yet", err.Message)
}

// Formatted convenience constructors

func TestNewInvalidArgumentf(t *testing.T) {
	err := NewInvalidArgumentf("field %s is required", "email")
	assert.Equal(t, CodeInvalidArgument, err.Code)
	assert.Equal(t, "field email is required", err.Message)
}

func TestNewNotFoundf(t *testing.T) {
	err := NewNotFoundf("resource %s not found", "user-123")
	assert.Equal(t, CodeNotFound, err.Code)
	assert.Equal(t, "resource user-123 not found", err.Message)
}

func TestNewUnauthorizedf(t *testing.T) {
	err := NewUnauthorizedf("token %s expired", "abc")
	assert.Equal(t, CodeUnauthorized, err.Code)
	assert.Equal(t, "token abc expired", err.Message)
}

func TestNewForbiddenf(t *testing.T) {
	err := NewForbiddenf("role %s denied", "guest")
	assert.Equal(t, CodeForbidden, err.Code)
	assert.Equal(t, "role guest denied", err.Message)
}

func TestNewConflictf(t *testing.T) {
	err := NewConflictf("item %s exists", "foo")
	assert.Equal(t, CodeConflict, err.Code)
	assert.Equal(t, "item foo exists", err.Message)
}

func TestNewAlreadyExistsf(t *testing.T) {
	err := NewAlreadyExistsf("user %s exists", "admin")
	assert.Equal(t, CodeAlreadyExists, err.Code)
	assert.Equal(t, "user admin exists", err.Message)
}

func TestNewInternalf(t *testing.T) {
	err := NewInternalf("db error: %s", "timeout")
	assert.Equal(t, CodeInternal, err.Code)
	assert.Equal(t, "db error: timeout", err.Message)
}

func TestNewUnavailablef(t *testing.T) {
	err := NewUnavailablef("service %s down", "auth")
	assert.Equal(t, CodeUnavailable, err.Code)
	assert.Equal(t, "service auth down", err.Message)
}

func TestNewUnimplementedf(t *testing.T) {
	err := NewUnimplementedf("feature %s not ready", "sso")
	assert.Equal(t, CodeUnimplemented, err.Code)
	assert.Equal(t, "feature sso not ready", err.Message)
}

func TestNewTooManyRequestsf(t *testing.T) {
	err := NewTooManyRequestsf("limit %d exceeded", 100)
	assert.Equal(t, CodeTooManyRequests, err.Code)
	assert.Equal(t, "limit 100 exceeded", err.Message)
}

// Wrap convenience functions

func TestWrapInternal(t *testing.T) {
	inner := errors.New("db conn failed")
	err := WrapInternal(inner, "database error")
	assert.Equal(t, CodeInternal, err.Code)
	assert.Equal(t, "database error", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapInternalf(t *testing.T) {
	inner := errors.New("timeout")
	err := WrapInternalf(inner, "service %s failed", "auth")
	assert.Equal(t, CodeInternal, err.Code)
	assert.Equal(t, "service auth failed", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapInvalidArgument(t *testing.T) {
	inner := errors.New("parse error")
	err := WrapInvalidArgument(inner, "invalid input")
	assert.Equal(t, CodeInvalidArgument, err.Code)
	assert.Equal(t, "invalid input", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapInvalidArgumentf(t *testing.T) {
	inner := errors.New("bad format")
	err := WrapInvalidArgumentf(inner, "field %s invalid", "email")
	assert.Equal(t, CodeInvalidArgument, err.Code)
	assert.Equal(t, "field email invalid", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapNotFound(t *testing.T) {
	inner := errors.New("query returned 0 rows")
	err := WrapNotFound(inner, "user not found")
	assert.Equal(t, CodeNotFound, err.Code)
	assert.Equal(t, "user not found", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapNotFoundf(t *testing.T) {
	inner := errors.New("no rows")
	err := WrapNotFoundf(inner, "resource %s missing", "org-1")
	assert.Equal(t, CodeNotFound, err.Code)
	assert.Equal(t, "resource org-1 missing", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapUnauthorized(t *testing.T) {
	inner := errors.New("token invalid")
	err := WrapUnauthorized(inner, "auth failed")
	assert.Equal(t, CodeUnauthorized, err.Code)
	assert.Equal(t, "auth failed", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapUnauthorizedf(t *testing.T) {
	inner := errors.New("expired token")
	err := WrapUnauthorizedf(inner, "token %s expired", "abc123")
	assert.Equal(t, CodeUnauthorized, err.Code)
	assert.Equal(t, "token abc123 expired", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapForbidden(t *testing.T) {
	inner := errors.New("access denied")
	err := WrapForbidden(inner, "no permission")
	assert.Equal(t, CodeForbidden, err.Code)
	assert.Equal(t, "no permission", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapForbiddenf(t *testing.T) {
	inner := errors.New("role insufficient")
	err := WrapForbiddenf(inner, "role %s insufficient", "viewer")
	assert.Equal(t, CodeForbidden, err.Code)
	assert.Equal(t, "role viewer insufficient", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapConflict(t *testing.T) {
	inner := errors.New("duplicate key")
	err := WrapConflict(inner, "duplicate entry")
	assert.Equal(t, CodeConflict, err.Code)
	assert.Equal(t, "duplicate entry", err.Message)
	assert.Equal(t, inner, err.Cause)
}

func TestWrapConflictf(t *testing.T) {
	inner := errors.New("unique violation")
	err := WrapConflictf(inner, "item %s exists", "user-1")
	assert.Equal(t, CodeConflict, err.Code)
	assert.Equal(t, "item user-1 exists", err.Message)
	assert.Equal(t, inner, err.Cause)
}

// HTTPStatus for special codes

func TestHTTPStatus_AccountLocked(t *testing.T) {
	err := NewAccountLocked("locked")
	assert.Equal(t, 423, err.HTTPStatus())
}

func TestHTTPStatus_PasswordExpired(t *testing.T) {
	err := NewPasswordExpired("expired")
	assert.Equal(t, 403, err.HTTPStatus())
}

func TestHTTPStatus_MfaRequired(t *testing.T) {
	err := NewMfaRequired("mfa needed")
	assert.Equal(t, 403, err.HTTPStatus())
}

func TestHTTPStatus_InvitationUsed(t *testing.T) {
	err := NewInvitationUsed("used")
	assert.Equal(t, 409, err.HTTPStatus())
}

func TestHTTPStatus_InvitationExpired(t *testing.T) {
	err := NewInvitationExpired("expired")
	assert.Equal(t, 410, err.HTTPStatus())
}

func TestHTTPStatus_Gone(t *testing.T) {
	err := NewGone("gone")
	assert.Equal(t, 410, err.HTTPStatus())
}
