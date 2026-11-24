package web3identity

import (
	"context"
	"errors"
	"time"
)

// Repository defines persistence for Passkey → EVM address mappings.
type Repository interface {
	// Create persists a new mapping. Wallet address uniqueness is enforced by
	// the DB unique constraint; caller should derive deterministically from
	// the same passkey pubkey (never accept arbitrary wallet_address from client).
	Create(ctx context.Context, mapping *Mapping) error

	// GetByCredentialID returns the mapping bound to a specific webauthn_credentials row.
	// Returns ErrNotFound when no mapping exists — this is the expected path for
	// a user who has a passkey but hasn't yet linked a Web3 identity.
	GetByCredentialID(ctx context.Context, credentialID string) (*Mapping, error)

	// GetByWalletAddress looks up by the derived EVM address. Used when the client
	// identifies itself via address.
	GetByWalletAddress(ctx context.Context, walletAddress string) (*Mapping, error)

	// TouchLastUsedAt bumps last_used_at after a successful login via this mapping.
	TouchLastUsedAt(ctx context.Context, id string, now time.Time) error
}

// ErrNotFound is returned when a mapping lookup yields no row.
var ErrNotFound = errors.New("web3identity: mapping not found")
