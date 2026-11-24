package ssf

import (
	"sync"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	platformssf "github.com/guliangiam/guliangiam/internal/platform/ssf"
)

// SignerHolder lets the Outbox worker registry be constructed before the JWKS
// provider exists (runtime wiring order) while delivery reads the signer at
// request time.
type SignerHolder struct {
	mu     sync.RWMutex
	signer platformssf.Signer
}

// NewSignerHolder constructs an empty holder.
func NewSignerHolder() *SignerHolder {
	return &SignerHolder{}
}

// Set installs the JWKS-backed signer; safe for concurrent delivery.
func (h *SignerHolder) Set(signer platformssf.Signer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.signer = signer
}

// SignToken implements platform/ssf.Signer.
func (h *SignerHolder) SignToken(claims map[string]any, tokenType string) (string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.signer == nil {
		return "", apperrors.NewUnavailable("security event signer is not configured")
	}
	return h.signer.SignToken(claims, tokenType)
}
