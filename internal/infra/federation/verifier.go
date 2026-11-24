// Package federation 实现 ADR-028 Federation Trust 的外部 JWT 验证。
//
// 职责：给定 subject JWT + FederationTrust（issuer / jwks_uri / audience），
// 从远程 JWKS 拉 RSA 公钥、验签、验 exp/nbf/iss/aud，返回 claims。
//
// 风格与 infra/jwt/issuer.go 一致 — 纯 crypto/rsa + encoding/json + base64，
// 零第三方 JWT 库依赖。支持的算法：RS256 only（与 ValidateSigningAlgorithm 平台策略对齐）。
package federation

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	domainworkload "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

// joseHeader 是外部 JWT 受保护头的最小字段集。
type joseHeader struct {
	Alg  string `json:"alg"`
	Kid  string `json:"kid"`
	Typ  string `json:"typ"`
	Crit any    `json:"crit"`
}

// Verifier 验证 Federation Trust 场景下的外部 JWT。
type Verifier struct {
	remoteJWKS *platformjwks.RemoteJWKSCache
}

// NewVerifier 构造 Verifier。RemoteJWKSCache 由调用方注入（全局单例 + 后台刷新）。
func NewVerifier(remoteJWKS *platformjwks.RemoteJWKSCache) *Verifier {
	return &Verifier{remoteJWKS: remoteJWKS}
}

// Verify 验证 subject JWT 并返回 claims。
// trust 指定了信任边界（issuer + jwks_uri + audience）。
//
// 验证步骤（与 infra/jwt/issuer.go ValidateTokenWithConfig 同构）：
//  1. 解码 JOSE header → alg + kid
//  2. alg 必须是 RS256（Reject HS* 和 none）
//  3. 从 trust.JWKSURI 拉公钥（RemoteJWKSCache）→ 按 kid 取
//  4. rsa.VerifyPKCS1v15 (RS256)
//  5. 验 exp、nbf
//  6. 验 iss == trust.Issuer
//  7. 验 aud 至少包含 trust.Audience 里的一个值
func (v *Verifier) Verify(tokenString string, trust *domainworkload.FederationTrust) (map[string]interface{}, error) {
	if trust == nil {
		return nil, apperrors.NewInvalidArgument("federation trust is nil")
	}
	if trust.Status != domainworkload.TrustStatusActive {
		return nil, apperrors.NewInvalidArgument("federation trust is not active")
	}
	if strings.TrimSpace(trust.Issuer) == "" {
		return nil, apperrors.NewInvalidArgument("federation trust issuer is empty")
	}
	if len(trust.Audience) == 0 {
		return nil, apperrors.NewInvalidArgument("federation trust audience is empty")
	}
	for _, audience := range trust.Audience {
		if strings.TrimSpace(audience) == "" {
			return nil, apperrors.NewInvalidArgument("federation trust audience is empty")
		}
	}
	if v.remoteJWKS == nil {
		return nil, apperrors.NewInternal("remote JWKS cache is not configured")
	}

	// 1. 拆 JWT → header.payload.signature
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, apperrors.NewInvalidArgument("invalid JWT format")
	}

	// 2. 解码 header → alg + kid
	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid JWT header encoding")
	}
	var header joseHeader
	if err := json.NewDecoder(bytes.NewReader(rawHeader)).Decode(&header); err != nil {
		return nil, apperrors.NewInvalidArgument("invalid JWT header")
	}

	if header.Crit != nil {
		return nil, apperrors.NewInvalidArgument("JWT header contains unsupported critical parameters")
	}

	if strings.TrimSpace(header.Alg) != "RS256" {
		return nil, apperrors.NewInvalidArgument("federation trust only supports RS256-signed JWT")
	}

	if strings.TrimSpace(header.Kid) == "" {
		return nil, apperrors.NewInvalidArgument("JWT header missing kid")
	}

	// 3. 拉公钥
	pub, err := v.remoteJWKS.GetPublicKey(context.Background(), trust.JWKSURI, header.Kid)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "fetch verification key")
	}

	// 4. 验签
	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode JWT signature")
	}
	hashed := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], sigBytes); err != nil {
		return nil, apperrors.NewUnauthorized("JWT signature verification failed")
	}

	// 5. 解码 payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode JWT payload")
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse JWT claims")
	}

	// 6. 验 exp / nbf。exp 是必须声明的短期凭据边界；nbf 如存在则必须是合法 NumericDate。
	now := time.Now().UTC()
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, apperrors.NewUnauthorized("JWT missing exp claim")
	}
	if !time.Unix(int64(exp), 0).After(now) {
		return nil, apperrors.NewUnauthorized("JWT has expired")
	}
	if rawNBF, exists := claims["nbf"]; exists {
		nbf, ok := rawNBF.(float64)
		if !ok {
			return nil, apperrors.NewUnauthorized("JWT nbf claim is invalid")
		}
		if time.Unix(int64(nbf), 0).After(now) {
			return nil, apperrors.NewUnauthorized("JWT not yet valid")
		}
	}

	// 7. 验 iss
	if iss, ok := claims["iss"].(string); !ok || iss != trust.Issuer {
		return nil, apperrors.NewUnauthorized("JWT issuer does not match federation trust")
	}

	// 8. 验 aud — 必须包含 trust.Audience 里的**至少一个**；空 trust audience 已在入口拒绝。
	tokenAuds := extractAuds(claims)
	if !audienceIntersects(tokenAuds, trust.Audience) {
		return nil, apperrors.NewUnauthorized("JWT audience does not match federation trust")
	}

	return claims, nil
}

// extractAuds 把 claims["aud"] 提取成 []string（aud 可能是 string 或 []interface{}）。
func extractAuds(claims map[string]interface{}) []string {
	audRaw, ok := claims["aud"]
	if !ok {
		return nil
	}
	switch aud := audRaw.(type) {
	case string:
		return []string{aud}
	case []interface{}:
		out := make([]string, 0, len(aud))
		for _, a := range aud {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// audienceIntersects 检查两个 aud 集合是否有交集。
func audienceIntersects(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := set[s]; ok {
			return true
		}
	}
	return false
}
