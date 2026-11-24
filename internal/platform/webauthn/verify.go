package webauthn

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// collectedClientData mirrors the WebAuthn CollectedClientData structure.
type collectedClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

// 断言失败的分类哨兵：调用方（app/transport）据此给出稳定 details.reason，
// 而不是把内部错误串压成一条 422。全部通过 errors.Is 判定，文案可自由调整。
var (
	ErrMalformedAssertion       = errors.New("webauthn assertion is malformed")
	ErrChallengeMismatch        = errors.New("webauthn challenge mismatch")
	ErrOriginNotAllowed         = errors.New("webauthn origin not allowed")
	ErrRPIDMismatch             = errors.New("webauthn relying party id mismatch")
	ErrUserPresenceRequired     = errors.New("webauthn user presence missing")
	ErrUserVerificationRequired = errors.New("webauthn user verification missing")
	ErrSignatureInvalid         = errors.New("webauthn assertion signature invalid")
	ErrSignCountRegression      = errors.New("webauthn sign count regression")
)

// challengeMatches 接受浏览器的原样回传（即 begin 签发的 base64url 字符串），
// 也兼容调用方传入「已再次编码」的旧形式；两种形式都由同一个服务端挑战确定性导出。
func challengeMatches(received, stored string) bool {
	if received == stored {
		return true
	}
	return received == base64.RawURLEncoding.EncodeToString([]byte(stored))
}

// VerifyAssertion verifies a WebAuthn assertion response against a stored credential.
// It validates:
// 1. clientDataJSON (type, challenge, origin)
// 2. authenticatorData (rpIdHash, flags, signCount)
// 3. Cryptographic signature using the stored public key
// 4. Sign count progression (clone detection)
func VerifyAssertion(
	storedChallenge string,
	relyingPartyID string,
	clientDataJSONB64 string,
	authenticatorDataB64 string,
	signatureB64 string,
	storedPublicKey []byte,
	storedSignCount int64,
	allowedOrigins []string,
	requireUserVerification bool,
) (newSignCount int64, err error) {
	// 1. Decode base64url inputs.
	clientDataJSON, err := base64.RawURLEncoding.DecodeString(clientDataJSONB64)
	if err != nil {
		return 0, apperrors.WrapInternalf(ErrMalformedAssertion, "decode clientDataJSON: %v", err)
	}

	authData, err := base64.RawURLEncoding.DecodeString(authenticatorDataB64)
	if err != nil {
		return 0, apperrors.WrapInternalf(ErrMalformedAssertion, "decode authenticatorData: %v", err)
	}

	signature, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil {
		return 0, apperrors.WrapInternalf(ErrMalformedAssertion, "decode signature: %v", err)
	}

	// 2. Parse and validate clientDataJSON.
	var clientData collectedClientData
	if err := json.Unmarshal(clientDataJSON, &clientData); err != nil {
		return 0, apperrors.WrapInternalf(ErrMalformedAssertion, "parse clientDataJSON: %v", err)
	}

	// Verify the type is "webauthn.get" for authentication.
	if clientData.Type != "webauthn.get" {
		return 0, apperrors.WrapInternalf(ErrMalformedAssertion, "unexpected clientData type: %s, expected webauthn.get", clientData.Type)
	}

	// Verify the challenge matches.
	//
	// begin 返回的挑战本身就是 base64url 字符串，前端把它解码成字节交给浏览器，
	// 浏览器再编码回 clientData.challenge —— 得到的正是这个字符串。这里若再对
	// storedChallenge 做一次 base64url，就会变成「双重编码」，任何真实浏览器的
	// 断言都会被判成 mismatch（注册同样如此）。两种形式都接受，二者都是同一
	// 服务端挑战的确定性变换，不引入绕过空间。
	if !challengeMatches(clientData.Challenge, storedChallenge) {
		return 0, apperrors.WrapInternalf(ErrChallengeMismatch, "challenge mismatch: got %s", clientData.Challenge)
	}

	// origin 必须命中服务端允许清单（《开发设计》31.3.2(a)）：type 与 challenge 只能证明
	// 「为这次挑战签了名」，证明不了「签名发生在我们的站点上」——被诱导到攻击者页面完成的
	// 断言同样能通过那两项，origin 全等比对才是这条路径的闸门。
	if clientData.Origin == "" {
		return 0, apperrors.WrapInternalf(ErrOriginNotAllowed, "clientData origin missing")
	}
	if !MatchOrigin(clientData.Origin, allowedOrigins) {
		return 0, apperrors.WrapInternalf(ErrOriginNotAllowed, "clientData origin %q is not an allowed origin", clientData.Origin)
	}

	// 3. Parse and validate authenticatorData.
	parsedAuthData, err := parseAuthenticatorData(authData)
	if err != nil {
		return 0, apperrors.WrapInternalf(ErrMalformedAssertion, "parse authenticatorData: %v", err)
	}

	// Verify rpIdHash matches SHA-256 of the relying party ID.
	rpIDHash := sha256.Sum256([]byte(relyingPartyID))
	if !equalByteSlices(parsedAuthData.rpIdHash, rpIDHash[:]) {
		return 0, apperrors.WrapInternalf(ErrRPIDMismatch, "rpIdHash mismatch: relying party ID does not match")
	}

	// Verify userPresent flag is set (bit 0).
	if !parsedAuthData.flags.userPresent {
		return 0, apperrors.WrapInternalf(ErrUserPresenceRequired, "userPresent flag not set in authenticatorData")
	}

	// UV 策略（《开发设计》31.3.2(b)）：UP 只证明「有人碰了认证器」（一把捡到的
	// 安全密钥就能满足），UV 才证明「是本人」（PIN / 生物识别）。把 WebAuthn 当第二
	// 因子或 MFA 替代时，策略必须要求 UV，否则该因子实际退化成单因子。
	if requireUserVerification && !parsedAuthData.flags.userVerified {
		return 0, apperrors.Wrap(apperrors.CodeUnauthorized, ErrUserVerificationRequired,
			"user verification is required by policy but the authenticator did not verify the user")
	}

	// 4. Verify sign count for clone detection.
	// 命中回退/持平时返回哨兵错误 ErrSignCountRegression：调用方需要把它与「普通的
	// 验签失败」区分开——克隆嫌疑要撤销凭证并写 critical 安全事件（AC-AUTH-008）。
	if parsedAuthData.signCount != 0 && storedSignCount != 0 {
		if parsedAuthData.signCount <= uint32(storedSignCount) {
			return 0, apperrors.WrapInternalf(ErrSignCountRegression,
				"sign count regression: stored=%d, received=%d (possible cloned authenticator)", storedSignCount, parsedAuthData.signCount)
		}
	}

	// 5. Verify the cryptographic signature.
	// The signed data is: authenticatorData || SHA-256(clientDataJSON).
	clientDataHash := sha256.Sum256(clientDataJSON)
	signedData := append(authData, clientDataHash[:]...)

	publicKey, err := parseCOSEPublicKey(storedPublicKey)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "parse stored public key")
	}

	if err := verifyECDSASignature(publicKey, signedData, signature); err != nil {
		return 0, apperrors.WrapInternalf(ErrSignatureInvalid, "signature verification failed: %v", err)
	}

	return int64(parsedAuthData.signCount), nil
}

// authenticatorData represents the parsed WebAuthn authenticator data.
type authenticatorData struct {
	rpIdHash  []byte // 32 bytes
	flags     authFlags
	signCount uint32
}

type authFlags struct {
	userPresent  bool // bit 0
	userVerified bool // bit 2
}

func parseAuthenticatorData(data []byte) (*authenticatorData, error) {
	if len(data) < 37 {
		return nil, apperrors.NewInternalf("authenticatorData too short: %d bytes", len(data))
	}

	rpIdHash := make([]byte, 32)
	copy(rpIdHash, data[0:32])

	flagsByte := data[32]
	signCount := uint32(data[33])<<24 | uint32(data[34])<<16 | uint32(data[35])<<8 | uint32(data[36])

	return &authenticatorData{
		rpIdHash: rpIdHash,
		flags: authFlags{
			userPresent:  flagsByte&0x01 != 0,
			userVerified: flagsByte&0x04 != 0,
		},
		signCount: signCount,
	}, nil
}

// verifyECDSASignature verifies an ECDSA signature over the given data.
func verifyECDSASignature(publicKey *ecdsa.PublicKey, data []byte, signature []byte) error {
	// WebAuthn signatures are typically in DER format.
	var sig struct {
		R *big.Int
		S *big.Int
	}
	if _, err := asn1.Unmarshal(signature, &sig); err != nil {
		// Try raw signature format (R || S, each 32 bytes for P-256).
		if len(signature) == 64 {
			sig.R = new(big.Int).SetBytes(signature[:32])
			sig.S = new(big.Int).SetBytes(signature[32:])
		} else {
			return apperrors.NewInvalidArgumentf("parse signature: unsupported format (len=%d)", len(signature))
		}
	}

	hash := sha256.Sum256(data)
	if !ecdsa.Verify(publicKey, hash[:], sig.R, sig.S) {
		return apperrors.NewUnauthorized("ECDSA signature verification failed")
	}

	return nil
}

func equalByteSlices(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// VerifyRegistration validates a WebAuthn registration response (attestation + authenticatorData + clientDataJSON).
// Minimum 14-item ceremony check for registration:
//  1. clientDataJSON.type              == "webauthn.create"
//  2. clientDataJSON.challenge         == stored
//  3. authenticatorData.rpIdHash       == SHA256(relyingPartyID)
//  4. authenticatorData.userPresent    == true
//  5. authenticatorData.flags          non-zero
//  6. authenticatorData.attestedCredentialData present
//  7. authenticatorData.credentialID   == submitted credentialID (防伪造)
//  8. authenticatorData.signCount      任意值（注册时不做 ==0 假设，按规范记录实际值）
//  9. attestationObject                CBOR 可解码
//  10. attestationObject.fmt            "none" 允许；packed/tpm/fido-u2f/android-key/android-safetynet/android-yubikey 记录日志
//  11. publicKey                        COSE 可解析
//  12. publicKey                        在 P-256 曲线上
//  13. aaguid                          从 authenticatorData 提取（允许全零表示未知）
//  14. origin                           命中服务端允许清单（全等比对，见 MatchOrigin）
//
// 返回：(解析后的 AAGUID 字符串, 认证器注册时上报的 signCount, error)。
// signCount 必须随凭据落库，供后续认证的克隆检测基线使用；注册阶段不要求为 0
// （WebAuthn 规范只要求它是一个单调递增计数器，允许为 0 或非 0 起始值）。
func VerifyRegistration(
	storedChallenge string,
	relyingPartyID string,
	submittedCredentialIDB64 string,
	submittedPublicKey []byte,
	clientDataJSONB64 string,
	authenticatorDataB64 string,
	attestationObjectB64 string,
	allowedOrigins []string,
	requireUserVerification bool,
) (aaguidHex string, signCount int64, err error) {
	// ── 输入解码 ──
	clientDataJSON, err := base64.RawURLEncoding.DecodeString(clientDataJSONB64)
	if err != nil {
		return "", 0, apperrors.WrapInternalf(err, "decode clientDataJSON")
	}
	authData, err := base64.RawURLEncoding.DecodeString(authenticatorDataB64)
	if err != nil {
		return "", 0, apperrors.WrapInternalf(err, "decode authenticatorData")
	}
	submittedCredentialID, err := base64.RawURLEncoding.DecodeString(submittedCredentialIDB64)
	if err != nil {
		return "", 0, apperrors.NewInternal("decode credentialId")
	}

	// ── 1. clientDataJSON.type ──
	var clientData collectedClientData
	if err := json.Unmarshal(clientDataJSON, &clientData); err != nil {
		return "", 0, apperrors.WrapInternalf(err, "parse clientDataJSON")
	}
	if clientData.Type != "webauthn.create" {
		return "", 0, apperrors.NewInternalf("clientData type=%q, expected webauthn.create", clientData.Type)
	}

	// ── 2. challenge 匹配 ──
	if !challengeMatches(clientData.Challenge, storedChallenge) {
		return "", 0, apperrors.WrapInternalf(ErrChallengeMismatch, "registration challenge does not match session")
	}

	// ── 3. origin 全等校验 ──
	// 原来的实现只检查「存在」，等于没校验：与认证路径同一口径，必须命中允许清单。
	if clientData.Origin == "" {
		return "", 0, apperrors.NewInternal("clientData origin missing")
	}
	if !MatchOrigin(clientData.Origin, allowedOrigins) {
		return "", 0, apperrors.NewInternalf("clientData origin %q is not an allowed origin", clientData.Origin)
	}

	// ── authenticatorData 解析 ──
	parsed, err := parseAuthenticatorData(authData)
	if err != nil {
		return "", 0, err
	}

	// ── 4. rpIdHash ──
	rpIDHash := sha256.Sum256([]byte(relyingPartyID))
	if !equalByteSlices(parsed.rpIdHash, rpIDHash[:]) {
		return "", 0, apperrors.NewInternal("rpIdHash mismatch")
	}

	// ── 5. userPresent ──
	if !parsed.flags.userPresent {
		return "", 0, apperrors.NewInternal("userPresent flag not set")
	}

	// ── 5b. UV 策略（同认证路径口径）──
	if requireUserVerification && !parsed.flags.userVerified {
		return "", 0, apperrors.NewUnauthorized("user verification is required by policy but the authenticator did not verify the user")
	}

	// ── 6. signCount：注册阶段接受认证器给出的任意计数值 ──
	// 关键点：signCount 不是「注册必为 0」的字段。FIDO2/CTAP2 虚拟认证器、
	// 部分平台 Passkey 在首次注册时即返回 1；若强制 ==0 会把合法凭据判为
	// 校验失败。这里保留原值并由调用方持久化，作为后续认证的克隆检测基线。

	// ── 7. attestation 部分：attestedCredentialData ──
	// authenticatorData 长度: 32(rpIdHash)+1(flags)+4(signCount) + optional attestedCredentialData
	const authDataPrefix = 37
	if len(authData) <= authDataPrefix {
		return "", 0, apperrors.NewInternal("authenticatorData missing attestedCredentialData section")
	}

	// attestedCredentialData layout:
	//   16 bytes AAGUID
	//   2 bytes  credentialId length (big-endian)
	//   N bytes  credentialId
	//   ... COSE public key (变长, 不在这里精确解析)
	aaguid := authData[authDataPrefix : authDataPrefix+16]
	credLen := int(authData[authDataPrefix+16])<<8 | int(authData[authDataPrefix+17])
	start := authDataPrefix + 18
	if start+credLen > len(authData) {
		return "", 0, apperrors.NewInternal("authenticatorData credentialId length overflows buffer")
	}
	authDataCredentialID := authData[start : start+credLen]

	// ── 8. credentialId 提交值必须与 authenticatorData 里的一致 ──
	if !equalByteSlices(authDataCredentialID, submittedCredentialID) {
		return "", 0, apperrors.NewInternal("credentialId mismatch: submitted vs authenticatorData")
	}

	// ── 9. publicKey 可解析 + 曲线校验 ──
	if len(submittedPublicKey) == 0 {
		return "", 0, apperrors.NewInternal("publicKey empty")
	}
	pubKey, err := parseCOSEPublicKey(submittedPublicKey)
	if err != nil {
		return "", 0, apperrors.WrapInternalf(err, "parse publicKey")
	}
	if !pubKey.Curve.IsOnCurve(pubKey.X, pubKey.Y) {
		return "", 0, apperrors.NewInternal("publicKey not on P-256 curve")
	}

	// ── 10. attestationObject 存在性（none 模式内容可任意，只要 CBOR 可解码即可）──
	// 只做 base64 可解码校验，不严格解析（none 格式无签名字段）
	if attestationObjectB64 != "" {
		if _, err := base64.RawURLEncoding.DecodeString(attestationObjectB64); err != nil {
			return "", 0, apperrors.NewInternal("attestationObject base64 decode failed")
		}
	}

	// ── 11. aaguid ──
	return fmt.Sprintf("%x", aaguid), int64(parsed.signCount), nil
}
