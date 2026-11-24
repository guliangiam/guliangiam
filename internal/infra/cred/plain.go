package cred

// PlainProvider implements HashProvider using plaintext comparison.
// WARNING: This exists ONLY for verifying legacy plaintext passwords during
// migration. Never use it for new passwords.
type PlainProvider struct{}

// NewPlainProvider creates a new plaintext hash provider.
func NewPlainProvider() *PlainProvider {
	return &PlainProvider{}
}

// Hash returns the password as-is. This should NOT be used for new passwords.
func (p *PlainProvider) Hash(password string) (string, error) {
	return password, nil
}

// Verify checks if the password matches the stored plaintext.
func (p *PlainProvider) Verify(password, hash string) (bool, error) {
	return password == hash, nil
}

// Algorithm returns "plain".
func (p *PlainProvider) Algorithm() string {
	return AlgoPlain
}

// Ensure PlainProvider implements HashProvider.
var _ HashProvider = (*PlainProvider)(nil)
