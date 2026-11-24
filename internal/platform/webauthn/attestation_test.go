package webauthn

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func textString(s string) []byte {
	out := append([]byte(nil), encodeHead(3, uint64(len(s)))...)
	return append(out, []byte(s)...)
}

// buildRegistrationAuthData assembles rpIdHash+flags+signCount+aaguid+credLen+credentialId+COSE.
func buildRegistrationAuthData(t *testing.T, credentialID, coseKey []byte, withCOSE bool) []byte {
	t.Helper()
	out := make([]byte, 0, 37+16+2+len(credentialID)+len(coseKey))
	out = append(out, make([]byte, 32)...) // rpIdHash (contents irrelevant here)
	out = append(out, 0x41)                // UP + AT
	out = append(out, 0, 0, 0, 0)          // signCount
	out = append(out, make([]byte, 16)...) // AAGUID
	out = append(out, byte(len(credentialID)>>8), byte(len(credentialID)))
	out = append(out, credentialID...)
	if withCOSE {
		out = append(out, coseKey...)
	}
	return out
}

func buildAttestationObject(authData []byte) []byte {
	out := []byte{0xa3} // map(3)
	out = append(out, textString("fmt")...)
	out = append(out, textString("none")...)
	out = append(out, textString("attStmt")...)
	out = append(out, 0xa0) // empty map
	out = append(out, textString("authData")...)
	out = append(out, byteString(authData)...)
	return out
}

func TestExtractRegistrationMaterial_HappyPath(t *testing.T) {
	key := mustGenerateP256(t)
	cose := buildCOSE(es256Fields(key))
	// credentialId 超过 255 字节时需要三字节长度字段；这里取 20 字节即可。
	credentialID := []byte("credential-id-20bytes")

	authData := buildRegistrationAuthData(t, credentialID, cose, true)
	derivedID, publicKey, err := ExtractRegistrationMaterial(base64.RawURLEncoding.EncodeToString(authData))
	require.NoError(t, err)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(credentialID), derivedID)
	assert.Equal(t, cose, publicKey)
}

func TestExtractRegistrationMaterial_TruncatedBuffer(t *testing.T) {
	key := mustGenerateP256(t)
	cose := buildCOSE(es256Fields(key))
	authData := buildRegistrationAuthData(t, []byte("cred"), cose, true)

	_, _, err := ExtractRegistrationMaterial(base64.RawURLEncoding.EncodeToString(authData[:len(authData)-1]))
	require.Error(t, err)

	_, _, err = ExtractRegistrationMaterial(base64.RawURLEncoding.EncodeToString(authData[:50]))
	require.Error(t, err)
}

func TestExtractRegistrationMaterial_MissingCOSEKey(t *testing.T) {
	authData := buildRegistrationAuthData(t, []byte("cred"), nil, false)
	_, _, err := ExtractRegistrationMaterial(base64.RawURLEncoding.EncodeToString(authData))
	require.Error(t, err)
}

func TestExtractRegistrationMaterial_InvalidBase64(t *testing.T) {
	_, _, err := ExtractRegistrationMaterial("!!!not-base64!!!")
	require.Error(t, err)
}

func TestExtractAttestationFormat_HappyPath(t *testing.T) {
	authData := buildRegistrationAuthData(t, []byte("cred"), buildCOSE(es256Fields(mustGenerateP256(t))), true)
	attestationObject := buildAttestationObject(authData)

	format, err := ExtractAttestationFormat(base64.RawURLEncoding.EncodeToString(attestationObject))
	require.NoError(t, err)
	assert.Equal(t, "none", format)

	extracted, err := ExtractAttestationAuthData(base64.RawURLEncoding.EncodeToString(attestationObject))
	require.NoError(t, err)
	assert.Equal(t, authData, extracted)
}

func TestExtractAttestationFormat_NonMapInput(t *testing.T) {
	_, err := ExtractAttestationFormat(base64.RawURLEncoding.EncodeToString(textString("not-a-map")))
	require.Error(t, err)
}

func TestExtractAttestationFormat_MissingFmt(t *testing.T) {
	authData := buildRegistrationAuthData(t, []byte("cred"), buildCOSE(es256Fields(mustGenerateP256(t))), true)
	object := []byte{0xa1} // map(1)
	object = append(object, textString("authData")...)
	object = append(object, byteString(authData)...)

	_, err := ExtractAttestationFormat(base64.RawURLEncoding.EncodeToString(object))
	require.Error(t, err)
}
