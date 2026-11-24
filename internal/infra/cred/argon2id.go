package cred

import "github.com/alexedwards/argon2id"

// Argon2idProvider implements HashProvider using Argon2id.
type Argon2idProvider struct{}

// NewArgon2idProvider creates a new Argon2id hash provider.
func NewArgon2idProvider() *Argon2idProvider {
	return &Argon2idProvider{}
}

// Hash generates an Argon2id hash from the password.
func (p *Argon2idProvider) Hash(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return hash, nil
}

// Verify checks if the password matches the Argon2id hash.
func (p *Argon2idProvider) Verify(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

// Algorithm returns "argon2id".
func (p *Argon2idProvider) Algorithm() string {
	return AlgoArgon2id
}

// Ensure Argon2idProvider implements HashProvider.
var _ HashProvider = (*Argon2idProvider)(nil)
