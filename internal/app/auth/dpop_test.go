package auth

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDPoPTestSvc() Service {
	return Service{
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
	}
}

// ─── dpopProofInvalidError 覆盖 ──────────────────────────────────────────

func TestDPoPProofInvalidError_DefaultMessage(t *testing.T) {
	e := newDPoPProofInvalidError("").(*dpopProofInvalidError)
	assert.Equal(t, "DPoP proof is invalid", e.Error())
	assert.Equal(t, "invalid_dpop_proof", e.ProtocolCode())
	assert.Equal(t, 401, e.HTTPStatus())
}

func TestDPoPProofInvalidError_CustomDetail(t *testing.T) {
	e := newDPoPProofInvalidError("bad algo: PS256").(*dpopProofInvalidError)
	assert.Equal(t, "bad algo: PS256", e.Error())
	assert.Equal(t, "invalid_dpop_proof", e.ProtocolCode())
	assert.Equal(t, 401, e.HTTPStatus())
}

// ─── ComputeJWKThumbprint (Service wrapper, 0% → 100%) ──────────────────

func TestService_ComputeJWKThumbprint_ValidRSA(t *testing.T) {
	svc := newDPoPTestSvc()
	// Minimal RSA JWK per RFC 7638 example
	jwk := `{"e":"AQAB","kty":"RSA","n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwuRKKG07hdUuE-4RT-PeDFgwjkgFNJ1UJTgIq5px41suGcHIoQW2RlQopoFKY0nvapaNcGAuWEgWOt61TKZ3FjdtuSnmweBikGWvpE6dKXaFinDqz8Qzt0QHID9Vt4H0Rjq3FgK00zcSim__MaHNJQG2g0Foe1waoHOgr7D0XwYCUEu7ahk7xM5BTI7MtFPqq1vvI6i18q-Jtb8guLXQc1ArBLQLnWcgV2irdoclDGjTKVn4Hwjfn5sYIFD4FAosEg3DNnh8xgXWvOgfsRQR2axCq2LYQbbTlWXCGx2vAMM3hQ"}`
	jkt, err := svc.ComputeJWKThumbprint(jwk)
	require.NoError(t, err)
	assert.NotEmpty(t, jkt)
	assert.Len(t, jkt, 43) // base64url(SHA-256) 固定 43 字符
}

func TestService_ComputeJWKThumbprint_InvalidJSON(t *testing.T) {
	svc := newDPoPTestSvc()
	_, err := svc.ComputeJWKThumbprint(`not-json`)
	require.Error(t, err)
}

func TestService_ComputeJWKThumbprint_Empty(t *testing.T) {
	svc := newDPoPTestSvc()
	_, err := svc.ComputeJWKThumbprint(``)
	require.Error(t, err)
}

// ─── BindTokenToDPoP ────────────────────────────────────────────────────

func TestService_BindTokenToDPoP_AddsCnfAndBinding(t *testing.T) {
	svc := newDPoPTestSvc()
	meta := map[string]any{"scope": "openid"}
	result := svc.BindTokenToDPoP(meta, "thumbprint-xyz")
	assert.Equal(t, "openid", result["scope"]) // 原字段保留
	assert.Equal(t, "DPoP", result["tokenBinding"])
	cnf, ok := result["cnf"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "thumbprint-xyz", cnf["jkt"])
}

func TestService_BindTokenToDPoP_NilInput_CreatesMap(t *testing.T) {
	svc := newDPoPTestSvc()
	result := svc.BindTokenToDPoP(nil, "thumb-abc")
	require.NotNil(t, result)
	cnf, ok := result["cnf"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "thumb-abc", cnf["jkt"])
}

// ─── GenerateNonce store nil 分支 ──────────────────────────────────────

func TestService_GenerateNonce_DefaultValidatorStore_Succeeds(t *testing.T) {
	// defaultDPoPValidator 构造时必定注入 Nonces store（见 platformdpop.NewDPoPValidator）
	svc := newDPoPTestSvc()
	result, err := svc.GenerateNonce(context.Background(), GenerateNonceCommand{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Nonce)
	assert.Greater(t, result.ExpiresIn, int64(0))
}

// ─── ValidateDPoPProof (早返回路径) ────────────────────────────────────

func TestService_ValidateDPoPProof_EmptyHeader_ReturnsError(t *testing.T) {
	svc := newDPoPTestSvc()
	_, err := svc.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{DPoPHeader: ""})
	require.Error(t, err)
}

func TestService_ValidateDPoPProof_WhitespaceOnlyHeader_ReturnsError(t *testing.T) {
	svc := newDPoPTestSvc()
	_, err := svc.ValidateDPoPProof(context.Background(), ValidateDPoPProofCommand{DPoPHeader: "   "})
	require.Error(t, err)
}
