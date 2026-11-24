package crypto

import (
	"crypto/sha256"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 审计链哈希算法是 records.chain_algo 的封闭取值。新增算法必须同时修改
// contracts/schema/60_audit.yaml 的 ck_records_chain_algo 与这里的分发。
const (
	ChainAlgoSHA256 = "sha256"
	ChainAlgoSM3    = "sm3"
)

// NormalizeChainAlgorithm resolves a persisted chain_algo value for read paths.
//
// This is read compatibility for rows written before chain_algo was populated,
// NOT a default that accepts arbitrary values: "" maps to the historical
// SHA-256 baseline, the registered algorithms pass through unchanged, and
// anything else is rejected. Callers must not swallow that error and fall back
// to a default, because a typo selecting the wrong digest would rewrite the
// chain or mask a mixed-algorithm chain. Registering a new algorithm means
// adding it here and in HashByChainAlgorithm.
func NormalizeChainAlgorithm(algo string) (string, error) {
	if algo == "" {
		return ChainAlgoSHA256, nil
	}
	switch algo {
	case ChainAlgoSHA256, ChainAlgoSM3:
		return algo, nil
	default:
		return "", unsupportedChainAlgorithm(algo)
	}
}

// HashByChainAlgorithm computes the audit-chain digest for the given algorithm.
// Unknown or empty values are rejected: a silent fallback would let a typo
// rewrite the chain with the wrong algorithm and hide a mixed-algorithm chain.
func HashByChainAlgorithm(algo string, data []byte) ([]byte, error) {
	switch algo {
	case ChainAlgoSHA256:
		sum := sha256.Sum256(data)
		return sum[:], nil
	case ChainAlgoSM3:
		return SM3Sum(data), nil
	default:
		return nil, unsupportedChainAlgorithm(algo)
	}
}

// unsupportedChainAlgorithm keeps one diagnostic text for every read path
// (VerifyChainRecords, risk audit-integrity, hash computation).
func unsupportedChainAlgorithm(algo string) error {
	return apperrors.NewInvalidArgumentf(
		"unsupported audit chain algorithm %q (allowed: %s, %s)", algo, ChainAlgoSHA256, ChainAlgoSM3)
}

// DefaultChainAlgorithm selects the algorithm for the first entry of a new
// chain: the gm crypto profile writes SM3, every other profile keeps the
// historical SHA-256 baseline.
func DefaultChainAlgorithm() string {
	if strings.EqualFold(config.CryptoProfile(), string(ProfileGM)) {
		return ChainAlgoSM3
	}
	return ChainAlgoSHA256
}
