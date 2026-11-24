package cred

import (
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// NewHashProvider creates a HashProvider based on the algorithm name.
func NewHashProvider(algorithm string) (HashProvider, error) {
	switch algorithm {
	case AlgoArgon2id:
		return NewArgon2idProvider(), nil
	case AlgoBcrypt:
		return NewBcryptProvider(), nil
	case AlgoPBKDF2:
		return NewPBKDF2Provider(), nil
	case AlgoSHA256:
		return NewSHA256Provider(), nil
	case AlgoSHA512:
		return NewSHA512Provider(), nil
	case AlgoMD5:
		return NewMD5Provider(), nil
	case AlgoPlain:
		return NewPlainProvider(), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported hash algorithm: %s", algorithm)
	}
}

// DetectAlgorithm detects the hash algorithm from the hash prefix.
func DetectAlgorithm(hash string) string {
	switch {
	case len(hash) > 10 && hash[:9] == "$argon2id":
		return AlgoArgon2id
	case len(hash) > 4 && (hash[:4] == "$2a$" || hash[:4] == "$2b$" || hash[:4] == "$2y$"):
		return AlgoBcrypt
	case len(hash) > 10 && hash[:9] == "pbkdf2-sh":
		return AlgoPBKDF2
	case len(hash) > 7 && hash[:7] == "{SHA256":
		return AlgoSHA256
	case len(hash) > 7 && hash[:7] == "{SHA512":
		return AlgoSHA512
	case len(hash) == 32:
		return AlgoMD5
	default:
		return AlgoArgon2id
	}
}
