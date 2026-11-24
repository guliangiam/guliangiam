package cred

import "golang.org/x/crypto/bcrypt"

// BcryptProvider implements HashProvider using bcrypt.
type BcryptProvider struct {
	Cost int
}

// NewBcryptProvider creates a new bcrypt hash provider with default cost 10.
func NewBcryptProvider() *BcryptProvider {
	return &BcryptProvider{Cost: 10}
}

// Hash generates a bcrypt hash from the password.
func (p *BcryptProvider) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), p.Cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Verify checks if the password matches the bcrypt hash.
// A mismatched hash or password is reported as (false, nil): only unrecoverable
// hashing failures would be returned as an error, and bcrypt does not expose any.
func (p *BcryptProvider) Verify(password, hash string) (bool, error) {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil, nil
}

// Algorithm returns "bcrypt".
func (p *BcryptProvider) Algorithm() string {
	return AlgoBcrypt
}

// Ensure BcryptProvider implements HashProvider.
var _ HashProvider = (*BcryptProvider)(nil)
