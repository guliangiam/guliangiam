package idp

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Web3Provider implements IdP authentication via Web3/MetaMask signature verification.
// Instead of a traditional OAuth redirect flow, it verifies an Ethereum personal-sign
// signature. This provider does not use standard OAuth2 endpoints.
//
// NOTE: The VerifySignature method requires the go-ethereum/crypto package for
// secp256k1 signature recovery. Add github.com/ethereum/go-ethereum to go.mod
// before using this provider in production.
type Web3Provider struct {
	BaseOAuthProvider
}

// NewWeb3Provider creates a new Web3 (MetaMask) provider.
func NewWeb3Provider(config *OAuthConfig) *Web3Provider {
	return &Web3Provider{
		BaseOAuthProvider: newBaseOAuth(config, "", "", ""),
	}
}

// GetRedirectURL is not applicable for Web3 authentication; returns an error.
func (p *Web3Provider) GetRedirectURL(state string) (string, error) {
	return "", apperrors.NewUnimplemented("web3 provider does not support redirect-based authentication")
}

// ExchangeCode is not applicable for Web3 authentication; returns an error.
func (p *Web3Provider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	return nil, apperrors.NewUnimplemented("web3 provider does not support code exchange; use VerifySignature instead")
}

// GetUserInfo is not applicable for Web3 authentication; returns an error.
func (p *Web3Provider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	return nil, apperrors.NewUnimplemented("web3 provider does not support token-based user info; use VerifySignature instead")
}

// GenerateNonce generates a random nonce for the Web3 signature challenge.
func (p *Web3Provider) GenerateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", apperrors.WrapInternalf(err, "generate nonce")
	}
	return hex.EncodeToString(b), nil
}

// VerifySignature verifies an Ethereum personal-sign signature and returns
// the user info with the recovered address as the user ID.
//
// Requires github.com/ethereum/go-ethereum/crypto for Keccak-256 and
// secp256k1 recovery. Implement with:
//
//	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
//	hash := crypto.Keccak256Hash([]byte(prefixedMessage))
//	pubKey, err := crypto.SigToPub(hash.Bytes(), sigBytes)
//	address := crypto.PubkeyToAddress(*pubKey).Hex()
func (p *Web3Provider) VerifySignature(message, signature string) (*UserInfo, error) {
	return nil, apperrors.NewUnauthorized("web3 VerifySignature requires go-ethereum dependency; add it to go.mod for production use")
}
