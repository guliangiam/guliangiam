package applications

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// app_client_jwks 的边界值。它们同时是写入校验和验签读取的硬上限：
// 读取侧不做二次解析，因此超过上限的行永远不可能被创建。
const (
	// ClientJWKMaxKeys 是单个客户端允许登记的活跃公钥数量上限。多把密钥只用于
	// 无停机轮换（新旧 kid 并存一个窗口），不是密钥仓库。
	ClientJWKMaxKeys = 8
	// ClientJWKMaxDocumentBytes 限制一次提交的整个 JWKS 文档大小。
	ClientJWKMaxDocumentBytes = 32 * 1024
	// ClientJWKMaxKeyBytes 限制单个 JWK 成员的大小。
	ClientJWKMaxKeyBytes = 8 * 1024
	// ClientJWKKidMaxLength 与 app_client_jwks.kid 的 VARCHAR(128) 对齐。
	ClientJWKKidMaxLength = 128
	// ClientJWKMinRSABits 是 OWASP 对 RSA 验签密钥的最低要求。
	ClientJWKMinRSABits = 2048
	// ClientJWKMaxRSABits 防止超大模数带来的验签耗时放大。
	ClientJWKMaxRSABits = 8192
)

// ClientJWKSupportedAlgorithms 与 app_client_jwks.ck_app_client_jwks_alg 保持一致。
// private_key_jwt 首期只接受非对称签名算法；HS* / none 永远不在集合内。
var ClientJWKSupportedAlgorithms = []string{"RS256", "PS256", "ES256"}

// clientJWKPrivateMembers 是 JWK 私钥参数。任何一项出现都直接拒绝，绝不静默
// 丢弃：静默丢弃会让调用方误以为私钥已被服务器保存，实际却只能验签失败。
var clientJWKPrivateMembers = []string{"d", "p", "q", "dp", "dq", "qi", "oth", "k"}

// ClientJWKStatus 是 app_client_jwks.status 的域枚举。
type ClientJWKStatus string

const (
	ClientJWKStatusActive  ClientJWKStatus = "active"
	ClientJWKStatusRetired ClientJWKStatus = "retired"
)

// ClientJWK 是 app_client_jwks 的一行。
//
// JWK 只保存公有参数；私钥参数在 ParseClientJWKS 阶段已被拒绝，仓储层不会
// 收到含私钥的文档。
type ClientJWK struct {
	ID             string
	ApplicationID  string
	OrganizationID string
	Kid            string
	Alg            string
	JWK            string
	Status         ClientJWKStatus
	RetiredAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ParsedClientJWK 是通过校验、可直接落库的公有 JWK。
type ParsedClientJWK struct {
	Kid string
	Alg string
	// JWK 是重新序列化后的公有 JWK JSON（只含允许的成员）。
	JWK string
}

// ParseClientJWKS 解析并校验内嵌 JWKS 文档（RFC 7517 §5）。
//
// 规则：
//  1. 顶层必须是对象且含 keys 数组；空数组合法（用于把客户端切换离
//     private_key_jwt 时清空密钥），由调用方结合认证方式判断是否可用。
//  2. 每把密钥必须是 RSA 或 EC；alg 缺省时按 kty 推导（RSA→RS256，EC→ES256）。
//  3. 拒绝全部私钥参数（d/p/q/dp/dq/qi/oth/k）与对称密钥（k）。
//  4. RSA 模数至少 2048 位；EC 只接受 P-256（对应 ES256），且点必须在曲线上。
//  5. kid 必填、唯一、不超过列宽。
func ParseClientJWKS(raw []byte) ([]ParsedClientJWK, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, jwksInvalid("jwks must be a JWKS object with a keys array")
	}
	if len(trimmed) > ClientJWKMaxDocumentBytes {
		return nil, jwksInvalid("jwks exceeds the maximum document size")
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &document); err != nil {
		return nil, jwksInvalid("jwks must be a JSON object")
	}
	rawKeys, ok := document["keys"]
	if !ok {
		return nil, jwksInvalid("jwks must contain a keys array")
	}
	var keys []json.RawMessage
	if err := json.Unmarshal(rawKeys, &keys); err != nil {
		return nil, jwksInvalid("jwks.keys must be an array")
	}
	if len(keys) > ClientJWKMaxKeys {
		return nil, jwksInvalid("jwks must not contain more than the allowed number of keys")
	}

	parsed := make([]ParsedClientJWK, 0, len(keys))
	seenKids := make(map[string]struct{}, len(keys))
	for _, rawKey := range keys {
		if len(rawKey) > ClientJWKMaxKeyBytes {
			return nil, jwksInvalid("a JWK exceeds the maximum key size")
		}
		key, err := parseClientJWK(rawKey)
		if err != nil {
			return nil, err
		}
		if _, exists := seenKids[key.Kid]; exists {
			return nil, jwksInvalid("jwks must not contain duplicate kid values")
		}
		seenKids[key.Kid] = struct{}{}
		parsed = append(parsed, key)
	}
	return parsed, nil
}

func parseClientJWK(raw json.RawMessage) (ParsedClientJWK, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return ParsedClientJWK{}, jwksInvalid("each jwks.keys entry must be a JSON object")
	}
	for _, forbidden := range clientJWKPrivateMembers {
		if _, present := members[forbidden]; present {
			return ParsedClientJWK{}, jwksInvalid("jwks must contain public keys only: private member is not allowed")
		}
	}

	kty, err := jwkStringMember(members, "kty", true)
	if err != nil {
		return ParsedClientJWK{}, err
	}
	if kty != "RSA" && kty != "EC" {
		return ParsedClientJWK{}, jwksInvalid("jwks kty must be RSA or EC")
	}

	kid, err := jwkStringMember(members, "kid", true)
	if err != nil {
		return ParsedClientJWK{}, err
	}
	if kid == "" || len(kid) > ClientJWKKidMaxLength {
		return ParsedClientJWK{}, jwksInvalid("jwks kid must be a non-empty string no longer than 128 characters")
	}

	if use, err := jwkStringMember(members, "use", false); err != nil {
		return ParsedClientJWK{}, err
	} else if use != "" && use != "sig" {
		return ParsedClientJWK{}, jwksInvalid(`jwks use must be "sig" when present`)
	}
	if err := validateKeyOps(members); err != nil {
		return ParsedClientJWK{}, err
	}

	alg, err := jwkStringMember(members, "alg", false)
	if err != nil {
		return ParsedClientJWK{}, err
	}
	if alg == "" {
		if kty == "RSA" {
			alg = "RS256"
		} else {
			alg = "ES256"
		}
	}
	if !isSupportedClientJWKAlgorithm(alg) {
		return ParsedClientJWK{}, jwksInvalid("jwks alg must be one of RS256, PS256, ES256")
	}

	canonical := map[string]any{"kty": kty, "kid": kid, "alg": alg}
	switch kty {
	case "RSA":
		if alg != "RS256" && alg != "PS256" {
			return ParsedClientJWK{}, jwksInvalid("jwks alg is not valid for an RSA key")
		}
		n, err := jwkDecodedMember(members, "n", true)
		if err != nil {
			return ParsedClientJWK{}, err
		}
		e, err := jwkDecodedMember(members, "e", true)
		if err != nil {
			return ParsedClientJWK{}, err
		}
		nInt := new(big.Int).SetBytes(n)
		eInt := new(big.Int).SetBytes(e)
		if nInt.BitLen() < ClientJWKMinRSABits || nInt.BitLen() > ClientJWKMaxRSABits {
			return ParsedClientJWK{}, jwksInvalid("jwks RSA modulus must be between 2048 and 8192 bits")
		}
		if eInt.BitLen() == 0 || eInt.BitLen() > 64 || eInt.Bit(0) == 0 {
			return ParsedClientJWK{}, jwksInvalid("jwks RSA exponent is invalid")
		}
		canonical["n"] = base64.RawURLEncoding.EncodeToString(n)
		canonical["e"] = base64.RawURLEncoding.EncodeToString(e)

	case "EC":
		if alg != "ES256" {
			return ParsedClientJWK{}, jwksInvalid("jwks alg is not valid for an EC key")
		}
		crv, err := jwkStringMember(members, "crv", true)
		if err != nil {
			return ParsedClientJWK{}, err
		}
		if crv != "P-256" {
			return ParsedClientJWK{}, jwksInvalid("jwks EC curve must be P-256 for ES256")
		}
		xBytes, err := jwkDecodedMember(members, "x", true)
		if err != nil {
			return ParsedClientJWK{}, err
		}
		yBytes, err := jwkDecodedMember(members, "y", true)
		if err != nil {
			return ParsedClientJWK{}, err
		}
		curve := elliptic.P256()
		xInt := new(big.Int).SetBytes(xBytes)
		yInt := new(big.Int).SetBytes(yBytes)
		if !curve.IsOnCurve(xInt, yInt) {
			return ParsedClientJWK{}, jwksInvalid("jwks EC point is not on P-256")
		}
		canonical["crv"] = crv
		canonical["x"] = base64.RawURLEncoding.EncodeToString(xBytes)
		canonical["y"] = base64.RawURLEncoding.EncodeToString(yBytes)
	}

	if use := jwkMemberString(members, "use"); use != "" {
		canonical["use"] = use
	}
	if keyOps, ok := members["key_ops"]; ok {
		canonical["key_ops"] = json.RawMessage(keyOps)
	}

	encoded, err := json.Marshal(canonical)
	if err != nil {
		return ParsedClientJWK{}, jwksInvalid("jwks contains an unsupported member")
	}
	return ParsedClientJWK{Kid: kid, Alg: alg, JWK: string(encoded)}, nil
}

func isSupportedClientJWKAlgorithm(alg string) bool {
	for _, supported := range ClientJWKSupportedAlgorithms {
		if alg == supported {
			return true
		}
	}
	return false
}

func jwkMemberString(members map[string]json.RawMessage, name string) string {
	raw, ok := members[name]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func jwkStringMember(members map[string]json.RawMessage, name string, required bool) (string, error) {
	raw, ok := members[name]
	if !ok {
		if required {
			return "", jwksInvalid("jwks is missing the required " + name + " member")
		}
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return "", jwksInvalid("jwks member " + name + " must be a non-empty string")
	}
	return strings.TrimSpace(value), nil
}

func jwkDecodedMember(members map[string]json.RawMessage, name string, required bool) ([]byte, error) {
	value, err := jwkStringMember(members, name, required)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(value)
	if decodeErr != nil || len(decoded) == 0 {
		return nil, jwksInvalid("jwks member " + name + " must be base64url")
	}
	return decoded, nil
}

func validateKeyOps(members map[string]json.RawMessage) error {
	raw, ok := members["key_ops"]
	if !ok {
		return nil
	}
	var ops []string
	if err := json.Unmarshal(raw, &ops); err != nil || len(ops) == 0 {
		return jwksInvalid("jwks key_ops must be a non-empty array when present")
	}
	hasVerify := false
	for _, op := range ops {
		switch op {
		case "verify":
			hasVerify = true
		case "sign":
			// A registered client JWK is a public key: it can never sign.
			return jwksInvalid("jwks key_ops must not contain sign for a public key")
		default:
			return jwksInvalid("jwks key_ops contains an unsupported operation")
		}
	}
	if !hasVerify {
		return jwksInvalid(`jwks key_ops must contain "verify"`)
	}
	return nil
}

func jwksInvalid(detail string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		detail,
		map[string]any{"field": "jwks", "expected": "public_jwks"},
	)
}

// ClientJWKUsable 报告该行在验签时可用。
func (j ClientJWK) Usable() bool {
	return j.Status == ClientJWKStatusActive && strings.TrimSpace(j.JWK) != ""
}
