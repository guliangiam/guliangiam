package web3

import (
	"encoding/hex"
	"fmt"
	"strings"

	platformerrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
	"golang.org/x/crypto/sha3"
)

const web3DerivationSalt = "guliangiam-web3-v1"

// DeriveAddressFromCOSEPubkey derives a deterministic EVM address from a WebAuthn
// secp256r1 public key stored in COSE format.
//
// Math:
//
//	compressed_pubkey = 0x02/03 || x_padded_to_32_bytes  (y even → 0x02, y odd → 0x03)
//	seed              = keccak256(compressed_pubkey || "guliangiam-web3-v1")
//	address_bytes     = seed[12:]                       (last 20 bytes)
//	address           = "0x" + hex(address_bytes)       (42 chars, lowercase)
//
// The address is NOT a secp256k1 derivation and does not correspond to any known
// private key. Its purpose is to bind a stable Web3 identity label to this passkey
// user — not to control on-chain funds. This is a pure compute mapping, no chain ops.
func DeriveAddressFromCOSEPubkey(cosePublicKey []byte) (address string, derivationSeedHex string, err error) {
	pubKey, err := platformwebauthn.ParsePublicKey(cosePublicKey)
	if err != nil {
		return "", "", platformerrors.WrapInvalidArgumentf(err, "parse COSE public key")
	}

	// Pad big.Int coords to 32-byte big-endian (left-padded with zeros).
	xBytes := padLeft(pubKey.X.Bytes(), 32)
	if len(xBytes) != 32 {
		return "", "", platformerrors.NewInvalidArgument("P-256 x coordinate exceeds 32 bytes")
	}

	// Compressed prefix: 0x02 if Y even, 0x03 if Y odd.
	var prefix byte
	if pubKey.Y.Sign() == 0 {
		prefix = 0x02
	} else if pubKey.Y.Bit(0) == 0 {
		prefix = 0x02
	} else {
		prefix = 0x03
	}

	compressed := make([]byte, 0, 33)
	compressed = append(compressed, prefix)
	compressed = append(compressed, xBytes...)

	seed := keccak256(append(compressed, []byte(web3DerivationSalt)...))
	address = fmt.Sprintf("0x%s", strings.ToLower(hex.EncodeToString(seed[12:])))
	derivationSeedHex = strings.ToLower(hex.EncodeToString(seed))

	return address, derivationSeedHex, nil
}

// DeriveAddressFromCompressedPubkey derives an EVM address directly from a 33-byte
// compressed secp256r1 public key. Useful when the caller already has compressed
// form (e.g. from a future RIP-7212 flow).
func DeriveAddressFromCompressedPubkey(compressedPubkey []byte) (address string, derivationSeedHex string, err error) {
	if len(compressedPubkey) != 33 {
		return "", "", platformerrors.NewInvalidArgumentf("compressed pubkey must be 33 bytes, got %d", len(compressedPubkey))
	}
	if compressedPubkey[0] != 0x02 && compressedPubkey[0] != 0x03 {
		return "", "", platformerrors.NewInvalidArgumentf("invalid compressed pubkey prefix: 0x%02x (expect 0x02 or 0x03)", compressedPubkey[0])
	}

	seed := keccak256(append(compressedPubkey, []byte(web3DerivationSalt)...))
	address = fmt.Sprintf("0x%s", strings.ToLower(hex.EncodeToString(seed[12:])))
	derivationSeedHex = strings.ToLower(hex.EncodeToString(seed))

	return address, derivationSeedHex, nil
}

// keccak256 — thin wrapper around sha3.NewLegacyKeccak256 for readability.
func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

// padLeft pads a byte slice to n bytes by inserting leading zeros.
func padLeft(b []byte, n int) []byte {
	if len(b) >= n {
		return b
	}
	out := make([]byte, n)
	copy(out[n-len(b):], b)
	return out
}
