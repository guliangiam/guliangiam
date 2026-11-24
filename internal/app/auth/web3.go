package auth

import (
	"math/big"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// normalizeEthereumAddress normalizes an Ethereum address to lowercase with 0x prefix.
func normalizeEthereumAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if !strings.HasPrefix(addr, "0x") && !strings.HasPrefix(addr, "0X") {
		addr = "0x" + addr
	}
	return strings.ToLower(addr)
}

// ParseChainID parses a chain ID string to a big.Int.
func ParseChainID(chainID string) (*big.Int, error) {
	if chainID == "" {
		return big.NewInt(1), nil
	}
	id := new(big.Int)
	if _, ok := id.SetString(chainID, 10); !ok {
		return nil, apperrors.NewInvalidArgumentf("invalid chain ID: %s", chainID)
	}
	return id, nil
}
