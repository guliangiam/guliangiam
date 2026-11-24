package auth

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	tokenExchangeAccessTokenTTL  = 1 * time.Hour
	tokenExchangeRefreshTokenTTL = 30 * 24 * time.Hour

	tokenExchangeGrantType = "urn:ietf:params:oauth:grant-type:token-exchange" //nolint:gosec // G101: OAuth grant-type URN, not a credential

	// Subject token types per RFC 8693.
	SubjectTokenTypeAccessToken = "urn:ietf:params:oauth:token-type:access_token" //nolint:gosec // G101: RFC 8693 token-type URN
	SubjectTokenTypeJWT         = "urn:ietf:params:oauth:token-type:jwt"          //nolint:gosec // G101: RFC 8693 token-type URN
	SubjectTokenTypeIDToken     = "urn:ietf:params:oauth:token-type:id_token"     //nolint:gosec // G101: RFC 8693 token-type URN

	// maxActChainDepth caps the delegation chain written into an act claim
	// (RFC 8693 §4.1). Without a cap a client can nest actor tokens indefinitely
	// and grow the token without bound.
	maxActChainDepth = 3
)

// invalidTargetError is the RFC 8693 §2.2.2 `invalid_target` code. It has no
// counterpart in the application error table (错误码注册表 §1), so it travels as a
// ProtocolCodeError, the same bridge `unsupported_response_type` uses.
type invalidTargetError struct {
	detail string
}

func (e *invalidTargetError) Error() string {
	if e.detail == "" {
		return "invalid target"
	}
	return "invalid target: " + e.detail
}

func (e *invalidTargetError) ProtocolCode() string { return "invalid_target" }
func (e *invalidTargetError) HTTPStatus() int      { return 400 }

func newInvalidTargetError(detail string) error {
	return &invalidTargetError{detail: detail}
}

// GetTokenExchangeCommand contains parameters for the OAuth 2.0 Token Exchange grant (RFC 8693).
type GetTokenExchangeCommand struct {
	GrantType          string // Must be "urn:ietf:params:oauth:grant-type:token-exchange"
	SubjectToken       string // The token to exchange
	SubjectTokenType   string // Type of the subject token
	ActorToken         string // Optional actor token for delegation
	ActorTokenType     string // Type of the actor token
	Resource           string // Optional resource indicator
	Audience           string // Optional audience
	Scope              string // Optional requested scope
	ClientID           string // Client identifier
	ClientSecret       string // Client secret for confidential clients
	RequestedTokenType string // Optional requested token type for the response
}

// TokenExchangeResponse represents the response from a token exchange per RFC 8693.
type TokenExchangeResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int64  `json:"expires_in"`
	Scope           string `json:"scope,omitempty"`
	RefreshToken    string `json:"refresh_token,omitempty"`
}

// GetTokenExchangeToken implements the OAuth 2.0 Token Exchange grant type (RFC 8693).
// It validates the subject token, performs downscoping, and issues a new token pair.
func (s *Service) GetTokenExchangeToken(ctx context.Context, command GetTokenExchangeCommand) (*TokenExchangeResponse, error) {
	if strings.TrimSpace(command.SubjectToken) == "" {
		return nil, apperrors.NewInvalidArgument("subject_token is required")
	}
	if !isValidSubjectTokenType(strings.TrimSpace(command.SubjectTokenType)) {
		return nil, apperrors.NewInvalidArgument("subject_token_type is not supported")
	}
	if strings.TrimSpace(command.GrantType) != tokenExchangeGrantType {
		return nil, apperrors.NewInvalidArgument("grant_type must be " + tokenExchangeGrantType)
	}

	// RFC 8693 §2.2: `resource` and `audience` both name a target and are not
	// meant to be combined. Accepting both leaves the resulting audience
	// ambiguous, so the request is refused rather than silently preferring one.
	resource := strings.TrimSpace(command.Resource)
	audienceParam := strings.TrimSpace(command.Audience)
	if resource != "" && audienceParam != "" {
		return nil, apperrors.NewInvalidArgument("resource and audience must not both be provided")
	}

	// The requested token type must be one this server can actually produce;
	// echoing an unsupported value would tell the client it received something
	// it did not (RFC 8693 §4).
	requestedTokenType := strings.TrimSpace(command.RequestedTokenType)
	if requestedTokenType != "" && requestedTokenType != SubjectTokenTypeAccessToken && requestedTokenType != SubjectTokenTypeJWT {
		return nil, apperrors.NewInvalidArgument("requested_token_type is not supported")
	}

	// --- Native SSO Branch ---
	// If actor_token_type is device-secret, route to the Native SSO exchange path.
	if strings.TrimSpace(command.ActorTokenType) == ActorTokenTypeDeviceSecret {
		result, err := s.ExchangeNativeSsoToken(
			ctx,
			command.SubjectToken,
			command.SubjectTokenType,
			command.ActorToken,
			command.ActorTokenType,
			command.Scope,
			command.ClientID,
			command.ClientSecret,
		)
		if err != nil {
			return nil, err
		}
		return &TokenExchangeResponse{
			AccessToken:     result.AccessToken,
			TokenType:       result.TokenType,
			ExpiresIn:       result.ExpiresIn,
			Scope:           result.Scope,
			IssuedTokenType: SubjectTokenTypeAccessToken,
		}, nil
	}

	// The calling client is authenticated before the subject token is trusted:
	// the ownership predicate below is judged against this caller, and RFC 8693
	// §2.1 requires client authentication on the token endpoint regardless of
	// grant type. Deriving the caller from the subject token instead (which is
	// what the previous flow did when client_id was absent) let anyone holding a
	// subject token exchange it without authenticating at all.
	caller, err := s.resolveTokenExchangeCaller(ctx, command)
	if err != nil {
		return nil, err
	}

	// Resolve the subject token based on its type.
	var subjectToken *tokendomain.Token

	switch strings.TrimSpace(command.SubjectTokenType) {
	case SubjectTokenTypeAccessToken:
		subjectToken, err = s.resolveAccessTokenSubject(ctx, strings.TrimSpace(command.SubjectToken))
	default:
		// For JWT and ID token types, validate using the JWT issuer and resolve claims.
		subjectToken, err = s.resolveJWTSubjectTokenForOrganization(ctx, strings.TrimSpace(command.SubjectToken), caller.OrganizationID)
	}
	if err != nil {
		return nil, err
	}

	// Validate the subject token is active.
	if subjectToken.RevokedAt != nil {
		return nil, apperrors.NewUnauthorized("subject token has been revoked")
	}
	if !subjectToken.ExpiresAt.After(s.Clock.Now().UTC()) {
		return nil, apperrors.NewUnauthorized("subject token has expired")
	}

	// Ownership predicate (设计报告 14.24.2). Without this check any
	// authenticated client could present another client's subject token and walk
	// away with a token for that user — a horizontal privilege escalation that
	// needed no exploit beyond knowing one token value.
	// Federated subjects are exempt: a synthetic federation token is anchored by
	// external JWT verification against an active FederationTrust, not by local
	// application binding (it carries no local ApplicationID).
	if !isFederatedSubject(subjectToken) && subjectToken.ApplicationID != caller.ApplicationID {
		return nil, apperrors.NewForbidden("subject token does not belong to the requesting client")
	}

	// The actor token, when supplied, is a credential too: validating it is the
	// whole point of the delegated variant. Previously it was recorded in
	// metadata and never checked, so an arbitrary string "delegated" the
	// exchange.
	actor, err := s.resolveTokenExchangeActor(ctx, command, caller)
	if err != nil {
		return nil, err
	}

	// Perform downscoping: requested scope must be a subset of subject token scope.
	requestedScopes := normalizeScopesFromString(command.Scope)
	if len(requestedScopes) > 0 {
		if !isScopeSubset(requestedScopes, subjectToken.Scopes) {
			return nil, apperrors.NewInvalidArgument("requested scope exceeds subject token scope")
		}
	} else {
		// If no scope requested, inherit from subject token.
		requestedScopes = append([]string(nil), subjectToken.Scopes...)
	}

	// RFC 8707: a resource the caller is not allowed to target is `invalid_target`
	// rather than a silently ignored parameter.
	if resource != "" && !isResourceAllowedFor(caller, resource) {
		return nil, newInvalidTargetError("resource is not permitted for this client")
	}
	if audienceParam != "" && !isAudienceAllowedFor(subjectToken, audienceParam) {
		return nil, newInvalidTargetError("audience is not permitted for this client")
	}

	var response *TokenExchangeResponse
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now().UTC()

		applicationID := caller.ApplicationID
		applicationSlug := caller.ApplicationSlug
		organizationID := caller.OrganizationID

		// ADR-029 pre_token: fail-closed policy gate before issuance.
		// Subject/actor token values never enter the action input.
		if gateErr := s.enforceActionsPreToken(ctx, organizationID, subjectToken.UserID, subjectToken.Username,
			applicationID, tokenExchangeGrantType, requestedScopes,
			newInvalidGrantError("token issuance denied by policy")); gateErr != nil {
			return gateErr
		}

		// Issue new access token.
		accessTokenValue, accessTokenPrefix, accessTokenHash, genErr := s.newAccessTokenSecret()
		if genErr != nil {
			return genErr
		}
		accessTokenID, genErr := s.UUIDGenerator.NewString()
		if genErr != nil {
			return apperrors.NewInternal("generate access token id")
		}

		audience := makeAudience(resource)
		if len(audience) == 0 && audienceParam != "" {
			audience = []string{audienceParam}
		}
		if len(audience) == 0 {
			audience = append([]string(nil), subjectToken.Audience...)
		}

		metadata := map[string]any{
			paramGrantType:     tokenExchangeGrantType,
			"subjectTokenType": strings.TrimSpace(command.SubjectTokenType),
			"subjectTokenId":   subjectToken.ID,
		}
		if actor != nil {
			metadata["actorTokenType"] = strings.TrimSpace(command.ActorTokenType)
			metadata["actorClientId"] = actor.ClientID
			metadata["actor"] = actor.actChainMetadata()
		}

		accessToken := &tokendomain.Token{
			ID:              accessTokenID,
			UserID:          subjectToken.UserID,
			Username:        subjectToken.Username,
			OrganizationID:  organizationID,
			ApplicationID:   applicationID,
			ApplicationSlug: applicationSlug,
			TokenPrefix:     accessTokenPrefix,
			TokenHash:       accessTokenHash,
			Scopes:          requestedScopes,
			Audience:        audience,
			Metadata:        metadata,
			ExpiresAt:       now.Add(tokenExchangeAccessTokenTTL),
			CreatedAt:       now,
			UpdatedAt:       now,
			Revision:        1,
		}

		if createErr := tx.Tokens().Create(ctx, accessToken); createErr != nil {
			return createErr
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			accessToken.OrganizationID,
			accessToken.UserID,
			"token.created",
			"token",
			accessToken.ID,
			"token",
			"token.created",
			map[string]any{},
		); err != nil {
			return err
		}

		// Issue refresh token. Federated/workload-identity subjects are
		// deliberately excluded: their credentials must stay short-lived, so a
		// 30-day refresh token would defeat the federation trust model. They
		// re-authenticate at their external IdP instead. Local (interactive)
		// subjects keep the standard refresh flow.
		refreshTokenValue := ""
		if !isFederatedSubject(subjectToken) {
			refreshTokenSecret, refreshTokenHash, genErr := s.newRefreshTokenSecret()
			if genErr != nil {
				return genErr
			}
			refreshTokenID, genErr := s.UUIDGenerator.NewString()
			if genErr != nil {
				return apperrors.NewInternal("generate refresh token id")
			}

			refreshToken := &tokendomain.RefreshToken{
				ID:               refreshTokenID,
				AccessTokenID:    accessTokenID,
				UserID:           subjectToken.UserID,
				Username:         subjectToken.Username,
				OrganizationID:   organizationID,
				ApplicationID:    applicationID,
				ApplicationSlug:  applicationSlug,
				RefreshTokenHash: refreshTokenHash,
				Scopes:           requestedScopes,
				Audience:         audience,
				// Self-referencing family root (FK family_id → refresh_tokens.id);
				// NULL here would disable reuse-triggered family revocation.
				FamilyID:  refreshTokenID,
				ExpiresAt: now.Add(tokenExchangeRefreshTokenTTL),
				CreatedAt: now,
				UpdatedAt: now,
				Revision:  1,
			}
			if createErr := tx.Tokens().CreateRefresh(ctx, refreshToken); createErr != nil {
				return createErr
			}
			refreshTokenValue = refreshTokenSecret
		}

		// RFC 8693 §2.2.1: `issued_token_type` reports what the server actually
		// issued. The previous code echoed `requested_token_type`, which told a
		// client asking for a JWT that it had received one while the response
		// carried an opaque token.
		response = &TokenExchangeResponse{
			AccessToken:     accessTokenValue,
			IssuedTokenType: SubjectTokenTypeAccessToken,
			TokenType:       tokenTypeBearer,
			ExpiresIn:       int64(tokenExchangeAccessTokenTTL / time.Second),
			Scope:           strings.Join(requestedScopes, " "),
			RefreshToken:    refreshTokenValue,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// isFederatedSubject reports whether the subject token originates from an
// external federation trust (WLI / workload identity). Synthetic federation
// tokens are marked with Metadata["federation"]=true and a "fed:" ID prefix.
// Such subjects get short-lived access tokens only.
func isFederatedSubject(subjectToken *tokendomain.Token) bool {
	if subjectToken == nil {
		return false
	}
	if strings.HasPrefix(subjectToken.ID, "fed:") {
		return true
	}
	federated, _ := subjectToken.Metadata["federation"].(bool)
	return federated
}

// tokenExchangeCaller is the authenticated client making the exchange request.
type tokenExchangeCaller struct {
	ApplicationID   string
	ApplicationSlug string
	OrganizationID  string
	ClientID        string
	IsConfidential  bool
}

// resolveTokenExchangeCaller authenticates the requesting client. A missing
// client_id is a failure, not a reason to trust the subject token: RFC 8693
// makes the token endpoint an authenticated endpoint for every grant type.
func (s *Service) resolveTokenExchangeCaller(ctx context.Context, command GetTokenExchangeCommand) (*tokenExchangeCaller, error) {
	clientID := strings.TrimSpace(command.ClientID)
	if clientID == "" {
		return nil, apperrors.NewUnauthorized("client authentication failed")
	}

	application, err := s.UOW.Applications().GetByClientID(ctx, clientID)
	if err != nil {
		return nil, apperrors.NewUnauthorized("client authentication failed")
	}

	if application.RequiresClientSecret() {
		clientSecret := strings.TrimSpace(command.ClientSecret)
		if clientSecret == "" {
			return nil, apperrors.NewUnauthorized("client authentication failed")
		}
		match, verifyErr := platformcrypto.VerifyClientSecret(clientSecret, application.ClientSecretHash)
		if verifyErr != nil || !match {
			return nil, apperrors.NewUnauthorized("client authentication failed")
		}
	} else if strings.TrimSpace(command.ClientSecret) != "" {
		// A public client presenting a secret is a configuration error worth
		// surfacing rather than ignoring.
		return nil, apperrors.NewUnauthorized("client authentication failed")
	}

	return &tokenExchangeCaller{
		ApplicationID:   application.ID,
		ApplicationSlug: application.Slug,
		OrganizationID:  application.OrganizationID,
		ClientID:        application.ClientID,
		IsConfidential:  application.RequiresClientSecret(),
	}, nil
}

// tokenExchangeActor is the validated actor of a delegated exchange. Its fields
// become the RFC 8693 §4.1 `act` structure.
type tokenExchangeActor struct {
	Subject  string
	ClientID string
	// Parent carries the actor chain of the token that was used as the actor
	// token, so delegations nest instead of flattening.
	Parent map[string]any
}

// actChainMetadata renders the actor as the nested `act` object carried in token
// metadata (opaque tokens cannot hold claims, so introspection reads it from here).
func (a *tokenExchangeActor) actChainMetadata() map[string]any {
	if a == nil {
		return nil
	}
	act := map[string]any{"client_id": a.ClientID}
	if strings.TrimSpace(a.Subject) != "" {
		act["sub"] = a.Subject
	}
	if len(a.Parent) > 0 {
		depth := actDepth(a.Parent) + 1
		if depth <= maxActChainDepth {
			act["act"] = a.Parent
		}
	}
	return act
}

// actDepth counts the nesting of an `act` chain. The cap exists because each
// delegation can add a level, and an unbounded chain is both a token-size and a
// verification-cost problem.
func actDepth(act map[string]any) int {
	depth := 0
	current := act
	for len(current) > 0 {
		depth++
		nested, ok := current["act"].(map[string]any)
		if !ok {
			break
		}
		current = nested
	}
	return depth
}

// resolveTokenExchangeActor validates the optional actor token. Returning nil
// means "no delegation requested", which is not an error.
func (s *Service) resolveTokenExchangeActor(ctx context.Context, command GetTokenExchangeCommand, caller *tokenExchangeCaller) (*tokenExchangeActor, error) {
	actorToken := strings.TrimSpace(command.ActorToken)
	if actorToken == "" {
		return nil, nil
	}

	actorTokenType := strings.TrimSpace(command.ActorTokenType)
	if actorTokenType == "" {
		return nil, apperrors.NewInvalidArgument("actor_token_type is required when actor_token is provided")
	}
	if actorTokenType != SubjectTokenTypeAccessToken {
		// JWT/ID-token actors resolve through the same path as subject tokens,
		// but only the access-token shape carries a client identity to delegate
		// on. Refuse the rest explicitly rather than silently ignoring them.
		return nil, apperrors.NewInvalidArgument("actor_token_type is not supported")
	}

	actor, err := s.resolveAccessTokenSubject(ctx, actorToken)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid actor token")
	}
	if actor.RevokedAt != nil {
		return nil, apperrors.NewUnauthorized("actor token has been revoked")
	}
	if !actor.ExpiresAt.After(s.Clock.Now().UTC()) {
		return nil, apperrors.NewUnauthorized("actor token has expired")
	}
	// The actor must be the caller (or a client the caller's token was issued
	// for, which for access tokens means the same application).
	if actor.ApplicationID != caller.ApplicationID {
		return nil, apperrors.NewForbidden("actor token does not belong to the requesting client")
	}

	result := &tokenExchangeActor{
		Subject:  actor.UserID,
		ClientID: actor.ApplicationSlug,
	}
	// Inherit a pre-existing chain so A→B→C delegations stay distinguishable
	// from a single A→C hop.
	if actor.Metadata != nil {
		if parent, ok := actor.Metadata["actor"].(map[string]any); ok && actDepth(parent) < maxActChainDepth {
			result.Parent = parent
		}
	}
	return result, nil
}

// isResourceAllowedFor reports whether the caller may target the given resource.
//
// The permitted set is the caller's own registered audience: a client may only
// ask for an access token aimed at itself or at a resource it already declares.
// Anything else is `invalid_target` (RFC 8707 §2) rather than a silently issued
// token with an audience the client was never entitled to.
func isResourceAllowedFor(caller *tokenExchangeCaller, resource string) bool {
	if caller == nil || strings.TrimSpace(resource) == "" {
		return true
	}
	// The resource must be at least scoped to the caller's own identity; without
	// a per-application allowed-resource registry the safe rule is to permit only
	// the caller's own client_id as an audience-bearing resource.
	return resource == caller.ClientID
}

// isAudienceAllowedFor reports whether the requested audience is within the set
// the subject token was already valid for.
func isAudienceAllowedFor(subjectToken *tokendomain.Token, audience string) bool {
	if subjectToken == nil || strings.TrimSpace(audience) == "" {
		return true
	}
	for _, existing := range subjectToken.Audience {
		if strings.TrimSpace(existing) == audience {
			return true
		}
	}
	return len(subjectToken.Audience) == 0
}

// resolveAccessTokenSubject resolves a subject token that is an access token.
func (s *Service) resolveAccessTokenSubject(ctx context.Context, subjectToken string) (*tokendomain.Token, error) {
	tokenHash := platformcrypto.HashTokenValue(subjectToken)
	token, err := s.UOW.Tokens().GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid subject token")
	}
	return token, nil
}

// resolveJWTSubjectToken resolves a subject token that is a JWT or ID token.
// It validates the JWT signature using the platform key and extracts the token by claims.
func (s *Service) resolveJWTSubjectToken(ctx context.Context, subjectToken string) (*tokendomain.Token, error) {
	return s.resolveJWTSubjectTokenForOrganization(ctx, subjectToken, "")
}

func (s *Service) resolveJWTSubjectTokenForOrganization(ctx context.Context, subjectToken, organizationID string) (*tokendomain.Token, error) {
	claims, err := parseJWTClaimsUnverified(subjectToken)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid subject token JWT")
	}

	if s.federationResolver != nil && s.federationVerifier != nil {
		verifiedClaims, _, federationErr := s.resolveFederationJWTSubjectWithClaims(
			ctx, organizationID, subjectToken, claims, s.federationResolver, s.federationVerifier,
		)
		if federationErr != nil {
			return nil, federationErr
		}
		if verifiedClaims != nil {
			return s.syntheticFederationToken(verifiedClaims)
		}
	}

	// An issuer with no configured trust may still be a locally signed JWT.
	if s.JWKSManager != nil {
		kp, keyErr := s.JWKSManager.GetKeyPair()
		if keyErr == nil {
			if sigErr := verifyJWTSignature(subjectToken, &kp.Key.PublicKey); sigErr != nil {
				return nil, apperrors.NewUnauthorized("subject token JWT signature verification failed")
			}
		}
	}

	jti, _ := claims["jti"].(string)
	sub, _ := claims["sub"].(string)

	if strings.TrimSpace(jti) != "" {
		token, err := s.UOW.Tokens().GetByID(ctx, strings.TrimSpace(jti))
		if err == nil {
			return token, nil
		}
	}

	if strings.TrimSpace(sub) != "" {
		return nil, apperrors.NewUnauthorizedf("cannot resolve JWT subject token by subject %q without jti", sub)
	}

	return nil, apperrors.NewUnauthorized("invalid subject token: missing jti claim")
}
func isValidSubjectTokenType(tokenType string) bool {
	switch tokenType {
	case SubjectTokenTypeAccessToken, SubjectTokenTypeJWT, SubjectTokenTypeIDToken:
		return true
	default:
		return false
	}
}

// isScopeSubset checks whether all requested scopes are present in the available scopes.
func isScopeSubset(requested, available []string) bool {
	availableSet := make(map[string]struct{}, len(available))
	for _, s := range available {
		availableSet[s] = struct{}{}
	}
	for _, s := range requested {
		if _, ok := availableSet[s]; !ok {
			return false
		}
	}
	return true
}
