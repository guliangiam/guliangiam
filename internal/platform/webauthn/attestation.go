package webauthn

import (
	"encoding/base64"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// authenticatorData 的固定前缀：rpIdHash(32) + flags(1) + signCount(4) + AAGUID(16)。
const (
	authenticatorDataPrefixLen = 37
	aaguidLen                  = 16
	credentialIDLenFieldLen    = 2
)

// ExtractRegistrationMaterial derives the credential ID and the COSE public key
// from authenticatorData so no client-submitted public key is ever trusted.
//
// authenticatorData 的布局（WebAuthn Level 2 §6.1）：
//
//	rpIdHash(32) | flags(1) | signCount(4) | AAGUID(16) | credentialIdLength(2) | credentialId | COSE public key
//
// 只有 AT 标志位存在时才带 attested credential data；这里不检查标志位，
// 调用方必须先用 VerifyRegistration 校验 ceremony 语义。
func ExtractRegistrationMaterial(authenticatorDataB64 string) (credentialIDB64 string, publicKey []byte, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(authenticatorDataB64)
	if err != nil {
		return "", nil, apperrors.WrapInternal(err, "decode authenticatorData")
	}

	dataStart := authenticatorDataPrefixLen + aaguidLen
	if len(raw) < dataStart+credentialIDLenFieldLen {
		return "", nil, apperrors.NewInternal("authenticatorData is too short to contain attested credential data")
	}

	credentialIDLen := int(raw[dataStart])<<8 | int(raw[dataStart+1])
	if credentialIDLen == 0 {
		return "", nil, apperrors.NewInternal("authenticatorData has an empty credentialId")
	}
	credentialIDStart := dataStart + credentialIDLenFieldLen
	if credentialIDStart+credentialIDLen > len(raw) {
		return "", nil, apperrors.NewInternal("authenticatorData credentialId exceeds input length")
	}

	credentialID := raw[credentialIDStart : credentialIDStart+credentialIDLen]
	rest := raw[credentialIDStart+credentialIDLen:]
	if len(rest) == 0 {
		return "", nil, apperrors.NewInternal("authenticatorData is missing the COSE public key")
	}

	reader := &cborReader{buf: rest}
	if err := reader.skip(); err != nil {
		return "", nil, apperrors.WrapInternal(err, "parse COSE public key")
	}
	if reader.pos <= 0 || reader.pos > len(rest) {
		return "", nil, apperrors.NewInternal("COSE public key span is invalid")
	}

	return base64.RawURLEncoding.EncodeToString(credentialID), append([]byte(nil), rest[:reader.pos]...), nil
}

// RegistrationMaterial 是一次注册 ceremony 中由 attestationObject 派生的权威材料。
// 注册流程必须持久化这里的值；请求体里的同名参数只能用于一致性校验。
type RegistrationMaterial struct {
	AuthenticatorData    []byte
	AuthenticatorDataB64 string
	CredentialID         []byte
	CredentialIDB64      string
	PublicKey            []byte
	AttestationType      string
}

// DeriveRegistrationMaterial 从 attestationObject 一次性派生注册所需的全部字段：
// credentialId、COSE 公钥、authenticatorData 与 attestation 格式。
//
// 这是自服务注册与管理员代注册共用的唯一派生入口。客户端另传的 publicKey /
// credentialId / authenticatorData 不得作为存储来源，只能与这里的结果比对。
func DeriveRegistrationMaterial(attestationObjectB64 string) (*RegistrationMaterial, error) {
	material, err := parseAttestationObject(attestationObjectB64)
	if err != nil {
		return nil, err
	}
	if material.format == "" {
		return nil, apperrors.NewInternal("attestationObject is missing the fmt field")
	}
	if len(material.authData) == 0 {
		return nil, apperrors.NewInternal("attestationObject is missing the authData field")
	}

	authDataB64 := base64.RawURLEncoding.EncodeToString(material.authData)
	credentialIDB64, publicKey, err := ExtractRegistrationMaterial(authDataB64)
	if err != nil {
		return nil, err
	}
	credentialID, err := base64.RawURLEncoding.DecodeString(credentialIDB64)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "decode derived credentialId")
	}

	return &RegistrationMaterial{
		AuthenticatorData:    append([]byte(nil), material.authData...),
		AuthenticatorDataB64: authDataB64,
		CredentialID:         credentialID,
		CredentialIDB64:      credentialIDB64,
		PublicKey:            publicKey,
		AttestationType:      material.format,
	}, nil
}

// EncodeCredentialIDs 把原始 credentialId 编码成 base64url，供
// PublicKeyCredentialCreationOptions.excludeCredentials 使用。
func EncodeCredentialIDs(ids [][]byte) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, base64.RawURLEncoding.EncodeToString(id))
	}
	return out
}

// attestationMaterial 是 attestationObject 顶层 map 里服务端关心的两个字段。
type attestationMaterial struct {
	format   string
	authData []byte
}

// ExtractAttestationFormat 返回 attestationObject 顶层的 fmt 字段（none/packed/tpm…）。
func ExtractAttestationFormat(attestationObjectB64 string) (string, error) {
	material, err := parseAttestationObject(attestationObjectB64)
	if err != nil {
		return "", err
	}
	if material.format == "" {
		return "", apperrors.NewInternal("attestationObject is missing the fmt field")
	}
	return material.format, nil
}

// ExtractAttestationAuthData 返回 attestationObject 顶层的 authData（原始字节）。
// 客户端只提交 attestationObject；authenticatorData 由服务端从其中派生，
// 因此不存在「客户端自报签名数据」的通道。
func ExtractAttestationAuthData(attestationObjectB64 string) ([]byte, error) {
	material, err := parseAttestationObject(attestationObjectB64)
	if err != nil {
		return nil, err
	}
	if len(material.authData) == 0 {
		return nil, apperrors.NewInternal("attestationObject is missing the authData field")
	}
	return material.authData, nil
}

// parseAttestationObject 严格解析 attestationObject 的顶层 CBOR map。
// 只读取 fmt（文本串）与 authData（字节串），其余字段跳过。
func parseAttestationObject(attestationObjectB64 string) (*attestationMaterial, error) {
	raw, err := base64.RawURLEncoding.DecodeString(attestationObjectB64)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "decode attestationObject")
	}

	reader := &cborReader{buf: raw}
	entries, err := reader.readMapHeader()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "parse attestationObject map")
	}

	material := &attestationMaterial{}
	for i := 0; i < entries; i++ {
		key, err := reader.readTextString()
		if err != nil {
			return nil, apperrors.WrapInternal(err, "parse attestationObject key")
		}

		switch key {
		case "fmt":
			material.format, err = reader.readTextString()
		case "authData":
			material.authData, err = reader.readByteString()
		default:
			err = reader.skip()
		}
		if err != nil {
			return nil, apperrors.WrapInternal(err, "parse attestationObject field")
		}
	}

	return material, nil
}

// readTextString 读取一个 definite-length 的文本串（major type 3）。attestationObject
// 的键是文本串，COSE_Key/attestation 的值几乎都是整数、字节串或嵌套 map。
func (r *cborReader) readTextString() (string, error) {
	initial, err := r.readByte()
	if err != nil {
		return "", err
	}
	if initial>>5 != 3 { // major type 3 = text string
		return "", apperrors.NewInternalf("expected CBOR text string, got major type %d", initial>>5)
	}
	length, err := r.readLength(initial & 0x1f)
	if err != nil {
		return "", err
	}
	if length > uint64(len(r.buf)-r.pos) {
		return "", apperrors.NewInternal("CBOR text string exceeds input length")
	}
	value := string(r.buf[r.pos : r.pos+int(length)])
	r.pos += int(length)
	return value, nil
}
