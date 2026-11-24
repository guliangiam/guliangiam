package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ═══════════════════════════════════════════════════════════════════════
// authorization_code.go unsupportedResponseTypeError.Error 0% → 100%
// ═══════════════════════════════════════════════════════════════════════

func TestUnsupportedResponseTypeError_EmptyResponseType(t *testing.T) {
	e := newUnsupportedResponseTypeError("").(*unsupportedResponseTypeError)
	assert.Equal(t, "unsupported response type", e.Error())
	assert.Equal(t, "unsupported_response_type", e.ProtocolCode())
	assert.Equal(t, 400, e.HTTPStatus())
}

func TestUnsupportedResponseTypeError_WithResponseType(t *testing.T) {
	e := newUnsupportedResponseTypeError("token").(*unsupportedResponseTypeError)
	assert.Equal(t, "unsupported response type: token", e.Error())
	assert.Equal(t, "unsupported_response_type", e.ProtocolCode())
	assert.Equal(t, 400, e.HTTPStatus())
}

func TestUnsupportedResponseTypeError_WhitespaceTrims(t *testing.T) {
	e := newUnsupportedResponseTypeError("  id_token  ").(*unsupportedResponseTypeError)
	assert.Equal(t, "unsupported response type: id_token", e.Error())
}

// ═══════════════════════════════════════════════════════════════════════
// device_flow.go deviceProtocolError.Error 0% → 100%
// ═══════════════════════════════════════════════════════════════════════

func TestDeviceProtocolError_EmptyDescription(t *testing.T) {
	e := newDeviceProtocolError("slow_down", "").(*deviceProtocolError)
	assert.Equal(t, "slow_down", e.Error())
	assert.Equal(t, "slow_down", e.ProtocolCode())
	assert.Equal(t, 400, e.HTTPStatus())
}

func TestDeviceProtocolError_WithDescription(t *testing.T) {
	e := newDeviceProtocolError("expired_token", "the user denied").(*deviceProtocolError)
	assert.Equal(t, "expired_token: the user denied", e.Error())
	assert.Equal(t, "expired_token", e.ProtocolCode())
	assert.Equal(t, 400, e.HTTPStatus())
}

// ═══════════════════════════════════════════════════════════════════════
// rfc8693_token_exchange.go invalidTargetError.Error 0% → 100%
// ═══════════════════════════════════════════════════════════════════════

func TestInvalidTargetError_EmptyDetail(t *testing.T) {
	e := newInvalidTargetError("").(*invalidTargetError)
	assert.Equal(t, "invalid target", e.Error())
	assert.Equal(t, "invalid_target", e.ProtocolCode())
	assert.Equal(t, 400, e.HTTPStatus())
}

func TestInvalidTargetError_WithDetail(t *testing.T) {
	e := newInvalidTargetError("audience not in trust list").(*invalidTargetError)
	assert.Equal(t, "invalid target: audience not in trust list", e.Error())
	assert.Equal(t, "invalid_target", e.ProtocolCode())
	assert.Equal(t, 400, e.HTTPStatus())
}
