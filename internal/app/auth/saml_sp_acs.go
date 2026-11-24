package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	// samlSPStateTTL is the maximum age of a server-issued browser ACS state.
	// It matches infra/saml.RelayStateSigner's TTL; the extra replay lifetime
	// covers the signer's clock-skew allowance so a late replay still finds the
	// reservation and fails closed.
	samlSPStateTTL          = 10 * time.Minute
	samlSPStateReplayTTL    = samlSPStateTTL + time.Minute
	samlSPStateReplayPrefix = "saml-sp-acs"
	samlSPStateMaxNonceLen  = 128
)

// CompleteSAMLSocialLoginCommand is the browser ACS input. OrganizationSlug and
// ProviderID come from the route path; SAMLResponse and RelayState come from the
// IdP's form POST.
type CompleteSAMLSocialLoginCommand struct {
	OrganizationSlug  string
	ProviderID        string
	SAMLResponse      string
	RelayState        string
	IPAddress         string
	UserAgent         string
	PreviousSessionID string
}

// samlSPStatePayload is the signed RelayState payload. Short keys keep the
// browser-visible envelope small; the whole payload is covered by the HMAC.
type samlSPStatePayload struct {
	OrganizationSlug string `json:"o"`
	ProviderID       string `json:"p"`
	Nonce            string `json:"s"`
}

// CompleteSAMLSocialLogin validates the provider against the path organization
// and then reuses the social SAML verification chain (loginWithProviderInTx).
// The shared login body consumes the server-issued state exactly once, so the
// browser ACS and the legacy JSON social-login endpoint enforce the same rule.
func (s *Service) CompleteSAMLSocialLogin(ctx context.Context, command CompleteSAMLSocialLoginCommand) (*SocialLoginResult, error) {
	orgSlug := strings.TrimSpace(command.OrganizationSlug)
	providerID := strings.TrimSpace(command.ProviderID)
	if orgSlug == "" || providerID == "" {
		return nil, apperrors.NewInvalidArgument("organizationSlug and providerId are required")
	}
	if !platformuuid.IsValid(providerID) {
		// A malformed id can only be a probe; answer exactly like a foreign id.
		return nil, apperrors.NewNotFound("SAML provider not found")
	}
	if strings.TrimSpace(command.SAMLResponse) == "" || strings.TrimSpace(command.RelayState) == "" {
		return nil, apperrors.NewInvalidArgument("SAMLResponse and RelayState are required")
	}

	// The ACS is unauthenticated (the session does not exist yet). Resolution
	// uses the public organization lookup; the provider row check below is the
	// tenant boundary, and the path slug is never treated as authorization.
	org, err := s.UOW.Organizations().GetBySlugPublic(ctx, orgSlug)
	if err != nil {
		if isNotFoundError(err) {
			return nil, apperrors.NewNotFound("SAML provider not found")
		}
		return nil, apperrors.WrapInternal(err, "load organization for SAML ACS")
	}

	// Stamp the resolved tenant onto the context so the unit of work applies the
	// same RLS scope a request-scoped handler would have. The value is derived
	// from the path + provider check, never from a request-supplied organizationId.
	ctx = tenantctx.WithTenantScope(ctx, org.ID, org.Slug, tenantctx.TenantFilterOptional)

	var result *SocialLoginResult
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := tx.Providers().GetByIDForOrganization(ctx, providerID, org.ID)
		if err != nil {
			if isNotFoundError(err) {
				return apperrors.NewNotFound("SAML provider not found")
			}
			return apperrors.WrapInternal(err, "load SAML provider")
		}
		if !isSAMLProvider(provider) {
			return apperrors.NewInvalidArgument("provider is not a SAML provider")
		}
		if err := ensureProviderUsable(provider); err != nil {
			return err
		}

		loginCommand := SocialLoginCommand{
			ProviderID:        provider.ID,
			Code:              command.SAMLResponse,
			State:             command.RelayState,
			OrganizationSlug:  org.Slug,
			ApplicationID:     org.Slug, // resolves the default app-<orgSlug>
			IPAddress:         command.IPAddress,
			UserAgent:         command.UserAgent,
			PreviousSessionID: command.PreviousSessionID,
		}
		loginResult, err := s.loginWithProviderInTx(ctx, tx, loginCommand)
		if err != nil {
			return err
		}
		result = loginResult
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.registeredUser != nil {
		s.dispatchPostRegistration(ctx, result.registeredUser.OrganizationID, result.registeredUser.ID,
			result.registeredUser.Username, result.registeredApplicationID, "Social Auto Register")
	}
	return result, nil
}

// signSAMLSPState generates the server-side state used both as the AuthnRequest
// ID and as RelayState. Caller-supplied state is never accepted: only this
// function's output carries a valid HMAC and a fresh nonce.
func (s *Service) signSAMLSPState(providerID, organizationSlug string) (string, error) {
	if s.RelayStateSigner == nil {
		return "", apperrors.NewInternal("relay state signer not configured")
	}
	nonce, err := generateSocialState()
	if err != nil {
		return "", apperrors.NewInternal("generate SAML state")
	}
	payload := encodeSAMLSPStatePayload(organizationSlug, providerID, nonce)
	signed, err := s.RelayStateSigner.Sign(payload, organizationSlug, providerID)
	if err != nil {
		return "", apperrors.WrapInternal(err, "sign SAML state")
	}
	return signed, nil
}

// consumeSAMLProviderState is the SAML guard shared by the browser ACS and the
// legacy JSON /social/login endpoint. It resolves the provider's organization
// (or rejects a caller-supplied slug that names another organization), then
// verifies the server-issued state, enforces its 10-minute lifetime and
// atomically reserves the nonce. Non-SAML providers never reach this function.
func (s *Service) consumeSAMLProviderState(ctx context.Context, tx appuow.UnitOfWork, provider *providerdomain.Provider, command SocialLoginCommand) error {
	if !isSAMLProvider(provider) {
		return nil
	}

	invalidState := apperrors.NewUnauthorized("SAML state is missing, expired, or already used")
	orgSlug := strings.TrimSpace(command.OrganizationSlug)
	if orgSlug == "" {
		org, err := tx.Organizations().GetByID(ctx, provider.OrganizationID)
		if err != nil {
			if isNotFoundError(err) {
				return invalidState
			}
			return apperrors.WrapInternal(err, "load organization for SAML login")
		}
		orgSlug = org.Slug
	} else {
		org, err := tx.Organizations().GetBySlug(ctx, orgSlug)
		if err != nil {
			if isNotFoundError(err) {
				return invalidState
			}
			return apperrors.WrapInternal(err, "load organization for SAML login")
		}
		if org.ID != provider.OrganizationID {
			return invalidState
		}
	}

	return s.consumeSAMLSPState(ctx, command.State, orgSlug, provider.ID, provider.OrganizationID)
}

// consumeSAMLSPState verifies the RelayState issued by GetSocialRedirectURL,
// binds it to the expected organization/provider, enforces the 10-minute lifetime
// and atomically reserves the nonce. Every failure is one generic 401 so the
// response never reveals which check failed.
func (s *Service) consumeSAMLSPState(ctx context.Context, relayState, organizationSlug, providerID, organizationID string) error {
	invalidState := apperrors.NewUnauthorized("SAML state is missing, expired, or already used")
	if s.RelayStateSigner == nil {
		return apperrors.NewInternal("relay state signer not configured")
	}

	payload, err := s.RelayStateSigner.Verify(relayState, organizationSlug, providerID)
	if err != nil {
		return invalidState
	}
	state, ok := decodeSAMLSPStatePayload(payload)
	if !ok || state.OrganizationSlug != organizationSlug || state.ProviderID != providerID {
		return invalidState
	}

	// The signer tolerates a small clock skew; enforce the documented 10-minute
	// lifetime here so a late assertion cannot be paired with a state that is
	// still inside the signer's skew window.
	if issuedAt, ok := signedSAMLSPStateIssuedAt(relayState); ok {
		if s.Clock.Now().UTC().Sub(issuedAt) > samlSPStateTTL {
			return invalidState
		}
	}

	if s.SAMLReplayStore == nil {
		return apperrors.NewInternal("SAML state store is not configured")
	}
	reserved, err := s.SAMLReplayStore.Reserve(ctx, samlSPStateReplayKey(organizationID, providerID, state.Nonce), samlSPStateReplayTTL)
	if err != nil {
		return apperrors.WrapInternal(err, "reserve SAML state")
	}
	if !reserved {
		return invalidState
	}
	return nil
}

func samlSPStateReplayKey(organizationID, providerID, nonce string) string {
	return strings.Join([]string{samlSPStateReplayPrefix, organizationID, providerID, nonce}, "|")
}

func encodeSAMLSPStatePayload(organizationSlug, providerID, nonce string) string {
	raw, _ := json.Marshal(samlSPStatePayload{
		OrganizationSlug: organizationSlug,
		ProviderID:       providerID,
		Nonce:            nonce,
	})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeSAMLSPStatePayload(payload string) (samlSPStatePayload, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(payload)
	}
	if err != nil {
		return samlSPStatePayload{}, false
	}

	var state samlSPStatePayload
	if err := json.Unmarshal(raw, &state); err != nil {
		return samlSPStatePayload{}, false
	}
	state.OrganizationSlug = strings.TrimSpace(state.OrganizationSlug)
	state.ProviderID = strings.TrimSpace(state.ProviderID)
	state.Nonce = strings.TrimSpace(state.Nonce)
	if state.OrganizationSlug == "" || state.ProviderID == "" || state.Nonce == "" || len(state.Nonce) > samlSPStateMaxNonceLen {
		return samlSPStatePayload{}, false
	}
	return state, true
}

// signedSAMLSPStateIssuedAt reads the issue time of the v1 RelayState envelope.
// A custom signer in tests may use another format; in that case the signer's own
// TTL remains the authority and the second check is skipped.
func signedSAMLSPStateIssuedAt(signed string) (time.Time, bool) {
	parts := strings.SplitN(signed, ".", 4)
	if len(parts) != 4 || parts[0] != "v1" {
		return time.Time{}, false
	}
	issuedAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(issuedAt, 0).UTC(), true
}

func isSAMLProvider(provider *providerdomain.Provider) bool {
	if provider == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(provider.Category)), string(providerdomain.CategorySAML)) ||
		strings.EqualFold(strings.TrimSpace(string(provider.ProviderType)), string(providerdomain.ProviderSAML)) ||
		strings.EqualFold(strings.TrimSpace(string(provider.Kind)), "saml")
}

// samlProviderOrganizationSlug resolves the organization slug used to bind the
// signed state. A caller-supplied slug is only accepted when it names the
// provider's own organization.
func (s *Service) samlProviderOrganizationSlug(ctx context.Context, provider *providerdomain.Provider, requestedSlug string) (string, error) {
	if provider == nil || strings.TrimSpace(provider.OrganizationID) == "" {
		return "", apperrors.NewInternal("SAML provider is missing organization")
	}
	if slug := strings.TrimSpace(requestedSlug); slug != "" {
		org, err := s.UOW.Organizations().GetBySlugPublic(ctx, slug)
		if err != nil {
			return "", apperrors.WrapInternal(err, "load organization for SAML state")
		}
		if org.ID != provider.OrganizationID {
			return "", apperrors.NewInvalidArgument("provider does not belong to the requested organization")
		}
		return org.Slug, nil
	}
	org, err := s.UOW.Organizations().GetByID(ctx, provider.OrganizationID)
	if err != nil {
		return "", apperrors.WrapInternal(err, "load provider organization for SAML state")
	}
	return org.Slug, nil
}

// socialAuthenticationLevel maps the SAML AuthnContextClassRef onto the session
// authentication_level vocabulary. Unknown classes fall back to "password",
// which is the same baseline the SAML adapter requests.
func socialAuthenticationLevel(authnContext string) string {
	normalized := strings.ToLower(strings.TrimSpace(authnContext))
	switch {
	case normalized == "":
		return loginMethodPassword
	case strings.Contains(normalized, "fido"), strings.Contains(normalized, "webauthn"), strings.Contains(normalized, "passkey"):
		return loginMethodFido2
	case strings.Contains(normalized, "multi"), strings.Contains(normalized, "mfa"),
		strings.Contains(normalized, "twofactor"), strings.Contains(normalized, "timesync"),
		strings.Contains(normalized, "smartcard"), strings.Contains(normalized, "x509"):
		return loginMethodMFA
	default:
		return loginMethodPassword
	}
}
