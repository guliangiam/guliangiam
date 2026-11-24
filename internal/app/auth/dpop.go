package auth

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"

	platformdpop "github.com/guliangiam/guliangiam/internal/platform/dpop"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// defaultDPoPValidator backs Service.DPoP when bootstrap did not inject one. It
// is the same value the constructor would produce, so the algorithm whitelist and
// the replay cache stay active either way.
var defaultDPoPValidator = platformdpop.NewDPoPValidator()

// dpopProofInvalidError is RFC 9449 §5 `invalid_dpop_proof` (401). The proof
// failures used to surface as `apperrors.NewUnauthorized("DPoP proof validation
// failed")`, which threw away the cause and made the protocol code
// `OAuthErrInvalidDPoPProof` a constant with no production user.
type dpopProofInvalidError struct {
	detail string
}

func (e *dpopProofInvalidError) Error() string {
	if e.detail == "" {
		return "DPoP proof is invalid"
	}
	return e.detail
}

func (e *dpopProofInvalidError) ProtocolCode() string { return "invalid_dpop_proof" }

func (e *dpopProofInvalidError) HTTPStatus() int { return http.StatusUnauthorized }

func newDPoPProofInvalidError(detail string) error {
	return &dpopProofInvalidError{detail: detail}
}

// ValidateDPoPProofCommand contains parameters for DPoP proof validation.
type ValidateDPoPProofCommand struct {
	DPoPHeader string // The raw DPoP proof JWT from the HTTP header
	Method     string // The HTTP method of the request
	URL        string // The HTTP URL of the request
}

// DPoPProofResult contains the result of a DPoP proof validation.
type DPoPProofResult struct {
	JKT string // The JWK thumbprint of the DPoP proof key
	JTI string // The JTI claim from the proof
}

// ValidateDPoPProof validates a DPoP proof JWT from the HTTP header.
// It verifies the signature, checks required claims, applies the algorithm
// whitelist and the replay cache, and returns the JWK thumbprint.
//
// The window is the RFC 9449 §11.1 ±60s (platformdpop.ProofClockSkew). This used
// to pass a locally defined 5 minute skew, which made the effective window five
// times wider than the design report's 口径 (14.27.2).
func (s *Service) ValidateDPoPProof(ctx context.Context, command ValidateDPoPProofCommand) (*DPoPProofResult, error) {
	if strings.TrimSpace(command.DPoPHeader) == "" {
		return nil, apperrors.NewInvalidArgument("DPoP proof header is required")
	}

	validator := s.DPoP
	if validator == nil {
		validator = defaultDPoPValidator
	}

	// The validator (not the bare platform function) is the only path that also
	// enforces the configured whitelist and rejects a replayed jti.
	proof, err := validator.Validate(
		ctx,
		strings.TrimSpace(command.DPoPHeader),
		command.Method,
		command.URL,
	)
	if err != nil {
		var appErr *apperrors.Error
		if stderrors.As(err, &appErr) && appErr.Code == apperrors.CodeUnavailable {
			return nil, err
		}
		return nil, newDPoPProofInvalidError(err.Error())
	}

	// Compute the JWK thumbprint.
	thumbprint, err := platformdpop.ComputeJWKThumbprint(proof.JWKJSON)
	if err != nil {
		return nil, apperrors.NewInternalf("compute JWK thumbprint: %s", err.Error())
	}

	return &DPoPProofResult{
		JKT: thumbprint,
		JTI: proof.Claims.JTI,
	}, nil
}

// ComputeJWKThumbprint computes the SHA-256 JWK thumbprint per RFC 7638.
func (s *Service) ComputeJWKThumbprint(jwkJSON string) (string, error) {
	return platformdpop.ComputeJWKThumbprint(jwkJSON)
}

// BindTokenToDPoP marks a token as DPoP-bound with the given JWK thumbprint.
func (s *Service) BindTokenToDPoP(metadata map[string]any, jwkThumbprint string) map[string]any {
	return platformdpop.BindTokenToDPoP(metadata, jwkThumbprint)
}

// ─── Nonce Challenge (RFC 9449 §4.2) ──────────────────────────────────────

// GenerateNonceCommand carries no parameters — the server is the sole issuer.
type GenerateNonceCommand struct{}

// NonceResult wraps a freshly-issued nonce with its TTL in seconds so the
// transport layer can serialize it to the client.
type NonceResult struct {
	Nonce     string
	ExpiresIn int64 // seconds
}

// GenerateNonce returns a fresh DPoP nonce that the client MUST echo back in
// the `nonce` claim of its next proof. Returning an error here would mean the
// random generator failed — a real failure is indistinguishable from a bug, so
// we propagate it verbatim.
func (s *Service) GenerateNonce(ctx context.Context, _ GenerateNonceCommand) (*NonceResult, error) {
	store := defaultDPoPValidator.Nonces
	if s.DPoP != nil && s.DPoP.Nonces != nil {
		store = s.DPoP.Nonces
	}
	if store == nil {
		// Should never happen: defaultDPoPValidator always wires one.
		return nil, apperrors.NewInternal("DPoP nonce store not configured")
	}

	nonce, ttl, err := store.Generate(ctx)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate DPoP nonce")
	}
	return &NonceResult{
		Nonce:     nonce,
		ExpiresIn: int64(ttl.Seconds()),
	}, nil
}
