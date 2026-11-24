package auth

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// socialOIDCStateTTL is the lifetime shared by the state, nonce and PKCE
	// verifier of one generic OIDC authorization request.
	socialOIDCStateTTL = 10 * time.Minute
	// socialOIDCRandomBytes yields a 43-character base64url value, which sits
	// inside the RFC 7636 verifier range (43..128).
	socialOIDCRandomBytes = 32
)

// socialOIDCState binds the values generated when the authorization URL is
// issued to the state the IdP echoes back. The record is consumed exactly once
// on callback, so a replayed state cannot complete a second login.
type socialOIDCState struct {
	State        string
	ProviderID   string
	Nonce        string
	CodeVerifier string
	ExpiresAt    time.Time
}

// socialOIDCStateStore is the in-process store for pending generic OIDC
// authorizations. It deliberately holds no persistence: pending values are
// short-lived, single-process, and never survive a restart (a restart invalidates
// the in-flight login and the user simply retries).
type socialOIDCStateStore struct {
	mu      sync.Mutex
	entries map[string]socialOIDCState
}

func newSocialOIDCStateStore() *socialOIDCStateStore {
	return &socialOIDCStateStore{entries: make(map[string]socialOIDCState)}
}

var defaultSocialOIDCStateStore = newSocialOIDCStateStore()

// save stores the entry under its state and opportunistically drops expired
// entries so the map cannot grow beyond the in-flight login window.
func (s *socialOIDCStateStore) save(entry socialOIDCState, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for state, existing := range s.entries {
		if !existing.ExpiresAt.After(now) {
			delete(s.entries, state)
		}
	}
	s.entries[entry.State] = entry
}

// consume returns the entry and deletes it in the same critical section. The
// deletion happens before the expiry check so an expired entry cannot be
// replayed either.
func (s *socialOIDCStateStore) consume(state string, now time.Time) (*socialOIDCState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[state]
	if !ok {
		return nil, apperrors.NewUnauthorized("OIDC state is missing, expired, or already used")
	}
	delete(s.entries, state)
	if !entry.ExpiresAt.After(now) {
		return nil, apperrors.NewUnauthorized("OIDC state is missing, expired, or already used")
	}
	return &entry, nil
}

func (s *Service) socialOIDCStateStore() *socialOIDCStateStore {
	if s.socialOIDCStates != nil {
		return s.socialOIDCStates
	}
	return defaultSocialOIDCStateStore
}

// isGenericOIDCProvider gates the nonce/PKCE binding to the generic OIDC
// adapter. Named OAuth2 providers keep their previous behaviour.
func isGenericOIDCProvider(provider *providerdomain.Provider) bool {
	if provider == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(provider.ProviderType)), string(providerdomain.ProviderOIDC))
}

// generateSocialOIDCValues produces the nonce and the PKCE verifier/challenge
// pair for one authorization request.
func generateSocialOIDCValues() (nonce, verifier, challenge string, err error) {
	nonceBytes := make([]byte, socialOIDCRandomBytes)
	if _, err = rand.Read(nonceBytes); err != nil {
		return "", "", "", err
	}
	verifierBytes := make([]byte, socialOIDCRandomBytes)
	if _, err = rand.Read(verifierBytes); err != nil {
		return "", "", "", err
	}
	nonce = base64.RawURLEncoding.EncodeToString(nonceBytes)
	verifier = base64.RawURLEncoding.EncodeToString(verifierBytes)
	challenge = buildPKCEChallenge(verifier)
	return nonce, verifier, challenge, nil
}

// bindSocialOIDCState generates and stores the nonce/PKCE values for an
// authorization request, then exposes only the nonce and challenge to the
// adapter. The verifier stays server-side until the callback.
func (s *Service) bindSocialOIDCState(provider *providerdomain.Provider, state string, config *SocialProviderConfig, now time.Time) error {
	nonce, verifier, challenge, err := generateSocialOIDCValues()
	if err != nil {
		return apperrors.NewInternal("generate OIDC nonce and PKCE verifier")
	}
	s.socialOIDCStateStore().save(socialOIDCState{
		State:        state,
		ProviderID:   provider.ID,
		Nonce:        nonce,
		CodeVerifier: verifier,
		ExpiresAt:    now.Add(socialOIDCStateTTL),
	}, now)
	config.Nonce = nonce
	config.CodeChallenge = challenge
	config.CodeChallengeMethod = pkceChallengeMethodS256
	return nil
}

// socialProviderConfigForLogin builds the adapter config for a callback. For
// generic OIDC it consumes the state exactly once and takes the nonce and PKCE
// verifier from that record instead of trusting request values.
func (s *Service) socialProviderConfigForLogin(provider *providerdomain.Provider, redirectURI string, command SocialLoginCommand, now time.Time) (SocialProviderConfig, error) {
	config := socialProviderConfig(provider, redirectURI)
	if !isGenericOIDCProvider(provider) {
		config.CodeVerifier = strings.TrimSpace(command.CodeVerifier)
		return config, nil
	}

	state := strings.TrimSpace(command.State)
	if state == "" {
		return config, apperrors.NewUnauthorized("OIDC state is required")
	}
	entry, err := s.socialOIDCStateStore().consume(state, now)
	if err != nil {
		return config, err
	}
	if entry.ProviderID != provider.ID {
		return config, apperrors.NewUnauthorized("OIDC state does not match the provider")
	}
	if strings.TrimSpace(entry.Nonce) == "" || strings.TrimSpace(entry.CodeVerifier) == "" {
		return config, apperrors.NewUnauthorized("OIDC state is missing nonce or PKCE verifier")
	}
	config.Nonce = entry.Nonce
	config.CodeVerifier = entry.CodeVerifier
	return config, nil
}
