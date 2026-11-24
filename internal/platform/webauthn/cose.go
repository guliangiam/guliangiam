package webauthn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/binary"
	"math/big"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// COSE_Key 解析（RFC 8152 §13.1.1 / RFC 9052）。
//
// 为什么不用字节模式扫描：旧实现沿「0x58 0x20（32 字节 bstr）+ 32 字节」这个模式在整段
// 字节里扫出前两个坐标。CBOR 的编码顺序并不固定——同一把公钥可以写成 {1,3,-1,-2,-3}、
// {-2,-3,1,3,-1} 或带上额外标签——启发式扫描在非规范顺序下会取错字段（把别处的 32
// 字节当坐标），把合法认证器判成无效、也可能接受一个结构上不成立的键。这里按 CBOR 语法
// 逐字段解析：只认 EC2/P-256/ES256，坐标必须是 32 字节 bstr，其余一律拒绝。
const (
	coseKeyKty = 1
	coseKeyAlg = 3
	coseKeyCrv = -1
	coseKeyX   = -2
	coseKeyY   = -3

	coseKtyEC2   = 2
	coseAlgES256 = -7
	coseCrvP256  = 1

	coseCoordLen = 32
)

// ParsePublicKey 对外暴露 COSE 公钥解析，返回 *ecdsa.PublicKey 供地址推导等跨包调用。
// 内部走 parseCOSEPublicKey，兼容 COSE_Key 与 PKIX/DER 两种存量格式。
func ParsePublicKey(stored []byte) (*ecdsa.PublicKey, error) {
	return parseCOSEPublicKey(stored)
}

// parseCOSEPublicKey decodes a stored public key into an ECDSA key.
// 兼容两种存量格式：COSE_Key（协议规定的形态）与 PKIX/DER（早期实现的兼容路径）。
func parseCOSEPublicKey(stored []byte) (*ecdsa.PublicKey, error) {
	if pubKey, err := x509.ParsePKIXPublicKey(stored); err == nil {
		if ecdsaKey, ok := pubKey.(*ecdsa.PublicKey); ok {
			return ecdsaKey, nil
		}
		return nil, apperrors.NewInternal("public key is not ECDSA")
	}

	reader := &cborReader{buf: stored}
	entries, err := reader.readMapHeader()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse COSE key map")
	}

	var (
		kty, alg, crv                           int64
		xBytes, yBytes                          []byte
		haveKty, haveAlg, haveCrv, haveX, haveY bool
	)

	for i := 0; i < entries; i++ {
		label, err := reader.readInt()
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "parse COSE key label")
		}

		switch label {
		case coseKeyKty:
			kty, err = reader.readInt()
			haveKty = true
		case coseKeyAlg:
			alg, err = reader.readInt()
			haveAlg = true
		case coseKeyCrv:
			crv, err = reader.readInt()
			haveCrv = true
		case coseKeyX:
			xBytes, err = reader.readByteString()
			haveX = true
		case coseKeyY:
			yBytes, err = reader.readByteString()
			haveY = true
		default:
			// 未使用的标签（kid、key_ops 等）必须能被跳过，否则合法密钥会被拒。
			err = reader.skip()
		}
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "parse COSE key field %d", label)
		}
	}

	if !haveKty || kty != coseKtyEC2 {
		return nil, apperrors.NewInternalf("COSE key kty must be EC2 (%d)", coseKtyEC2)
	}
	if !haveCrv || crv != coseCrvP256 {
		return nil, apperrors.NewInternalf("COSE key crv must be P-256 (%d)", coseCrvP256)
	}
	if !haveAlg || alg != coseAlgES256 {
		// 只支持 ES256：把 -7 之外的值（含 RS256/EdDSA/不存在）明确拒绝，
		// 而不是按 P-256 去验签一个来自别的算法的键。
		return nil, apperrors.NewInternalf("COSE key alg must be ES256 (%d), got %d", coseAlgES256, alg)
	}
	if !haveX || !haveY {
		return nil, apperrors.NewInternal("COSE key is missing the x/y coordinates")
	}
	if len(xBytes) != coseCoordLen || len(yBytes) != coseCoordLen {
		return nil, apperrors.NewInternalf("COSE key coordinates must be %d bytes (x=%d, y=%d)", coseCoordLen, len(xBytes), len(yBytes))
	}

	// 允许前导零（CBOR 的 bstr 长度固定 32，因此不会出现短写），直接按大端整数解释。
	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)

	pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	if !pubKey.Curve.IsOnCurve(x, y) {
		return nil, apperrors.NewInternal("public key point is not on P-256 curve")
	}
	return pubKey, nil
}

// cborReader 是解析 COSE_Key 所需的最小 CBOR 读取器：只覆盖 definite-length map、
// 整数（含负整数）与 byte string 三种主类型，够用且不留「宽松解析」的空间。
type cborReader struct {
	buf []byte
	pos int
}

func (r *cborReader) readByte() (byte, error) {
	if r.pos >= len(r.buf) {
		return 0, apperrors.NewInternal("unexpected end of CBOR input")
	}
	b := r.buf[r.pos]
	r.pos++
	return b, nil
}

// readLength 读取主类型低 5 位的附加信息与随后的长度/值。
func (r *cborReader) readLength(info byte) (uint64, error) {
	switch {
	case info < 24:
		return uint64(info), nil
	case info == 24:
		b, err := r.readByte()
		return uint64(b), err
	case info == 25:
		var raw [2]byte
		for i := range raw {
			b, err := r.readByte()
			if err != nil {
				return 0, err
			}
			raw[i] = b
		}
		return uint64(binary.BigEndian.Uint16(raw[:])), nil
	case info == 26:
		var raw [4]byte
		for i := range raw {
			b, err := r.readByte()
			if err != nil {
				return 0, err
			}
			raw[i] = b
		}
		return uint64(binary.BigEndian.Uint32(raw[:])), nil
	case info == 27:
		var raw [8]byte
		for i := range raw {
			b, err := r.readByte()
			if err != nil {
				return 0, err
			}
			raw[i] = b
		}
		return binary.BigEndian.Uint64(raw[:]), nil
	default:
		// 28-30 保留，31 表示 indefinite length —— COSE_Key 不使用，直接拒绝。
		return 0, apperrors.NewInternalf("unsupported CBOR additional info %d", info)
	}
}

// readMapHeader 读取 map 主类型并返回条目数。
func (r *cborReader) readMapHeader() (int, error) {
	initial, err := r.readByte()
	if err != nil {
		return 0, err
	}
	if initial>>5 != 5 { // major type 5 = map
		return 0, apperrors.NewInternalf("COSE key must be a CBOR map, got major type %d", initial>>5)
	}
	entries, err := r.readLength(initial & 0x1f)
	if err != nil {
		return 0, err
	}
	if entries > 32 {
		return 0, apperrors.NewInternalf("COSE key map has too many entries (%d)", entries)
	}
	return int(entries), nil
}

// readInt 读取一个 CBOR 整数（含负整数）。
func (r *cborReader) readInt() (int64, error) {
	initial, err := r.readByte()
	if err != nil {
		return 0, err
	}
	major := initial >> 5
	if major != 0 && major != 1 { // 0 = unsigned, 1 = negative
		return 0, apperrors.NewInternalf("expected CBOR integer, got major type %d", major)
	}
	value, err := r.readLength(initial & 0x1f)
	if err != nil {
		return 0, err
	}
	if major == 0 {
		if value > 1<<62 {
			return 0, apperrors.NewInternal("CBOR integer out of range")
		}
		return int64(value), nil
	}
	// 负整数编码为 -1 - n。
	if value > 1<<62 {
		return 0, apperrors.NewInternal("CBOR negative integer out of range")
	}
	return -1 - int64(value), nil
}

// readByteString 读取一个 definite-length 的字节串。
func (r *cborReader) readByteString() ([]byte, error) {
	initial, err := r.readByte()
	if err != nil {
		return nil, err
	}
	if initial>>5 != 2 { // major type 2 = byte string
		return nil, apperrors.NewInternalf("expected CBOR byte string, got major type %d", initial>>5)
	}
	length, err := r.readLength(initial & 0x1f)
	if err != nil {
		return nil, err
	}
	if length > uint64(len(r.buf)-r.pos) {
		return nil, apperrors.NewInternal("CBOR byte string exceeds input length")
	}
	out := make([]byte, length)
	copy(out, r.buf[r.pos:r.pos+int(length)])
	r.pos += int(length)
	return out, nil
}

// skip 跳过一个未使用的字段（整数、字节串或文本串），保证额外标签不会让合法密钥被拒。
func (r *cborReader) skip() error {
	initial, err := r.readByte()
	if err != nil {
		return err
	}
	switch initial >> 5 {
	case 0, 1:
		_, err = r.readLength(initial & 0x1f)
		return err
	case 2, 3:
		length, err := r.readLength(initial & 0x1f)
		if err != nil {
			return err
		}
		if length > uint64(len(r.buf)-r.pos) {
			return apperrors.NewInternal("CBOR string exceeds input length")
		}
		r.pos += int(length)
		return nil
	case 4, 5: // array / map: 跳过嵌套结构，例如 attestationObject 的 attStmt
		entries, err := r.readLength(initial & 0x1f)
		if err != nil {
			return err
		}
		// 每个元素至少占 1 字节，条目数不可能超过剩余长度（也避免 2*entries 溢出）。
		if entries > uint64(len(r.buf)-r.pos) {
			return apperrors.NewInternal("CBOR container exceeds input length")
		}
		count := entries
		if initial>>5 == 5 {
			count *= 2 // map 的每个条目由键值两个元素组成
		}
		for i := uint64(0); i < count; i++ {
			if err := r.skip(); err != nil {
				return err
			}
		}
		return nil
	default:
		return apperrors.NewInternalf("unsupported CBOR field with major type %d", initial>>5)
	}
}
