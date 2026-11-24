package webauthn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// COSE_Key 解析的用例。旧实现按「0x58 0x20 + 32 字节」的字节模式扫描坐标，
// 对 CBOR 编码顺序脆弱；这些用例把「按语法解析」的边界钉死。

func mustGenerateP256(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func coords(key *ecdsa.PrivateKey) (x, y []byte) {
	return key.PublicKey.X.FillBytes(make([]byte, 32)), key.PublicKey.Y.FillBytes(make([]byte, 32))
}

type coseField struct {
	label int
	value []byte
}

func encodeHead(major byte, value uint64) []byte {
	switch {
	case value < 24:
		return []byte{major<<5 | byte(value)}
	case value <= 0xff:
		return []byte{major<<5 | 24, byte(value)}
	case value <= 0xffff:
		return []byte{major<<5 | 25, byte(value >> 8), byte(value)}
	default:
		return []byte{major<<5 | 26, byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
	}
}

func encodeInt(v int) []byte {
	if v >= 0 {
		return encodeHead(0, uint64(v))
	}
	return encodeHead(1, uint64(-1-v))
}

func byteString(b []byte) []byte {
	return append(encodeHead(2, uint64(len(b))), b...)
}

// buildCOSE 按给定的字段顺序拼一个 COSE_Key map（短编码）。
func buildCOSE(fields []coseField) []byte {
	out := []byte{0xa0 | byte(len(fields))}
	for _, f := range fields {
		out = append(out, encodeInt(f.label)...)
		out = append(out, f.value...)
	}
	return out
}

func es256Fields(key *ecdsa.PrivateKey) []coseField {
	x, y := coords(key)
	return []coseField{
		{label: 1, value: encodeInt(2)},   // kty: EC2
		{label: 3, value: encodeInt(-7)},  // alg: ES256
		{label: -1, value: encodeInt(1)},  // crv: P-256
		{label: -2, value: byteString(x)}, // x
		{label: -3, value: byteString(y)}, // y
	}
}

func TestParseCOSEPublicKey_AcceptsES256InAnyFieldOrder(t *testing.T) {
	key := mustGenerateP256(t)
	x, y := coords(key)

	canonical := buildCOSE(es256Fields(key))
	reordered := buildCOSE([]coseField{
		{label: -3, value: byteString(y)},
		{label: -2, value: byteString(x)},
		{label: -1, value: encodeInt(1)},
		{label: 1, value: encodeInt(2)},
		{label: 3, value: encodeInt(-7)},
	})

	for name, raw := range map[string][]byte{"canonical": canonical, "reordered": reordered} {
		t.Run(name, func(t *testing.T) {
			parsed, err := parseCOSEPublicKey(raw)
			require.NoError(t, err)
			assert.Equal(t, 0, parsed.X.Cmp(key.PublicKey.X))
			assert.Equal(t, 0, parsed.Y.Cmp(key.PublicKey.Y))
		})
	}
}

func TestParseCOSEPublicKey_SkipsUnusedLabels(t *testing.T) {
	key := mustGenerateP256(t)
	fields := append([]coseField{
		{label: 2, value: byteString([]byte("key-id-1"))}, // kid
	}, es256Fields(key)...)

	parsed, err := parseCOSEPublicKey(buildCOSE(fields))
	require.NoError(t, err)
	assert.Equal(t, 0, parsed.X.Cmp(key.PublicKey.X))
}

func TestParseCOSEPublicKey_RejectsUnsupportedKeys(t *testing.T) {
	key := mustGenerateP256(t)
	x, y := coords(key)

	replace := func(label int, value []byte) []byte {
		fields := es256Fields(key)
		for i := range fields {
			if fields[i].label == label {
				fields[i].value = value
			}
		}
		return buildCOSE(fields)
	}

	tests := []struct {
		name string
		raw  []byte
	}{
		{"not a map", []byte{0x01}},
		{"wrong kty (RSA)", replace(1, encodeInt(3))},
		{"wrong curve (P-384)", replace(-1, encodeInt(2))},
		{"wrong algorithm (EdDSA)", replace(3, encodeInt(-8))},
		{"missing alg", buildCOSE([]coseField{
			{label: 1, value: encodeInt(2)},
			{label: -1, value: encodeInt(1)},
			{label: -2, value: byteString(x)},
			{label: -3, value: byteString(y)},
		})},
		{"missing coordinates", buildCOSE([]coseField{
			{label: 1, value: encodeInt(2)},
			{label: 3, value: encodeInt(-7)},
			{label: -1, value: encodeInt(1)},
		})},
		{"short coordinate", replace(-2, byteString(x[:16]))},
		{"truncated input", buildCOSE(es256Fields(key))[:10]},
		{"not on curve", replace(-3, byteString(make([]byte, 32)))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseCOSEPublicKey(tt.raw)
			require.Error(t, err, "非法 COSE_Key 必须被拒绝，而不是按 P-256 去掉验签")
			assert.Nil(t, parsed)
		})
	}
}
