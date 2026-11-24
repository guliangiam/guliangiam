package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	responseTypeCode = fieldCode
	responseTypeSAML = "saml"
	responseTypeCAS  = "cas"
)

// handleLoggedIn generates the appropriate post-login response based on the
// application type and response_type specified in the login command.
//
// sessionSid is the sid of the session this login just created. It is a
// parameter rather than a field on command because command is built from client
// input: the OIDC session identifier must not be something a caller can supply
// (设计报告 14.12.3).
func (s *Service) handleLoggedIn(
	ctx context.Context,
	tx appuow.UnitOfWork,
	command LoginCommand,
	organization *orgdomain.Organization,
	application *applicationdomain.Application,
	user *userdomain.User,
	sessionSid string,
	now time.Time,
) (*LoginResponse, error) {
	responseType := strings.TrimSpace(command.ResponseType)

	// If no response_type, return a simple success response with account context.
	if responseType == "" {
		resp := toSuccessfulLoginResponse(toAccountContext(organization, user, ""))
		return &resp, nil
	}

	switch responseType {
	case responseTypeCode:
		return s.handleLoggedInCode(ctx, tx, command, organization, application, user, sessionSid, now)
	case responseTypeSAML:
		return s.handleLoggedInSAML(ctx, command, organization, application, user)
	case responseTypeCAS:
		return s.handleLoggedInCAS(ctx, command, organization, user)
	default:
		// The implicit response types (`token`, `id_token`) were retired with the
		// rest of the implicit flow (设计报告 14.6.1「禁用 response_type=token」). They are answered
		// with the RFC 6749 §4.1.2.1 code instead of being silently downgraded to
		// the code flow, which would hand the client a grant it did not ask for.
		return nil, newUnsupportedResponseTypeError(responseType)
	}
}

// handleLoggedInCode generates an authorization code and redirect URL (authorization code flow).
func (s *Service) handleLoggedInCode(
	ctx context.Context,
	tx appuow.UnitOfWork,
	command LoginCommand,
	organization *orgdomain.Organization,
	application *applicationdomain.Application,
	user *userdomain.User,
	sessionSid string,
	now time.Time,
) (*LoginResponse, error) {
	redirectURI := strings.TrimSpace(command.RedirectURI)
	if redirectURI == "" && len(application.RedirectURIs) > 0 {
		redirectURI = application.RedirectURIs[0]
	}
	if redirectURI == "" {
		return nil, apperrors.NewInvalidArgument("redirectUri is required for authorization code flow")
	}

	// Exact-match against the registered set, through the same matcher the
	// authorize endpoint uses. The fallback above is safe because it picks a
	// registered value, never a reflected query parameter.
	if !applicationdomain.MatchRedirectURI(redirectURI, application.RedirectURIs) {
		return nil, apperrors.NewInvalidArgument("redirectUri is not registered for this application")
	}

	scopes := normalizeScopes(command.Scope)
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", contactEmail}
	}

	// Same constructor as the authorize endpoint, so this path binds the same
	// columns — including the PKCE challenge and the RFC 8707 resource — and uses
	// the same 256 bit entropy source (设计报告 14.6.4).
	authorizationCode, err := s.newAuthorizationCode(authorizationCodeIssuance{
		OrganizationID:   organization.ID,
		OrganizationSlug: organization.Slug,
		UserID:           user.ID,
		Username:         user.Username,
		ApplicationID:    application.ID,
		ApplicationSlug:  application.Slug,
		RedirectURI:      redirectURI,
		Scopes:           scopes,
		State:            command.State,
		Nonce:            command.Nonce,
		CodeChallenge:    command.CodeChallenge,
		ChallengeMethod:  command.ChallengeMethod,
		Resource:         command.Resource,
		// The code carries the session's sid so the token endpoint can put the same
		// value in the ID token, matching the logout token.
		Sid: sessionSid,
	}, now)
	if err != nil {
		return nil, err
	}

	if err := tx.AuthorizationCodes().Create(ctx, authorizationCode); err != nil {
		return nil, err
	}

	redirectTo := buildAuthorizationRedirectURL(redirectURI, authorizationCode.Code, command.State)

	resp := toSuccessfulLoginResponse(toAccountContext(organization, user, ""))
	resp.Code = authorizationCode.Code
	resp.RedirectTo = redirectTo
	return &resp, nil
}

// handleLoggedInSAML generates a signed SAML response after login. The ACS
// destination goes through the same redirect whitelist as the standalone ACS
// endpoint, and signing fails closed without a configured key.
func (s *Service) handleLoggedInSAML(
	ctx context.Context,
	command LoginCommand,
	organization *orgdomain.Organization,
	application *applicationdomain.Application,
	user *userdomain.User,
) (*LoginResponse, error) {
	samlRequest := strings.TrimSpace(command.SAMLRequest)
	if samlRequest == "" {
		return nil, apperrors.NewInvalidArgument("SAMLRequest is required for SAML flow")
	}

	requestInfo, err := s.ParseSamlRequest(ctx, samlRequest)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid SAML AuthnRequest")
	}
	if requestInfo.ID == "" {
		return nil, apperrors.NewInvalidArgument("SAML AuthnRequest has no ID")
	}

	acsURL, err := resolveSAMLACSURL(requestInfo.ACSURL, application)
	if err != nil {
		if apperrors.FromError(err).Code == apperrors.CodeForbidden {
			s.recordSAMLResponseRejection(ctx, organization.ID, user.ID, application.ID, "acs_url_not_registered")
		}
		return nil, err
	}

	// IdP role: RelayState was supplied by the external SP and must be returned
	// unchanged (SAML 2.0 §3.4). Signature verification belongs to the SP-side
	// ACS path, never to the IdP echo path.
	signedXML, err := s.GenerateSamlResponse(ctx, application, user, requestInfo.ID, acsURL)
	if err != nil {
		return nil, err
	}

	resp := toSuccessfulLoginResponse(toAccountContext(organization, user, ""))
	resp.SAMLResponse = base64.StdEncoding.EncodeToString([]byte(signedXML))
	resp.RelayState = command.RelayState
	resp.RedirectTo = acsURL
	return &resp, nil
}

// handleLoggedInCAS generates a CAS service ticket after login.
func (s *Service) handleLoggedInCAS(
	ctx context.Context,
	command LoginCommand,
	organization *orgdomain.Organization,
	user *userdomain.User,
) (*LoginResponse, error) {
	service := strings.TrimSpace(command.CASService)
	if service == "" {
		return nil, apperrors.NewInvalidArgument("service is required for CAS flow")
	}

	ticket, err := s.generateCASTicket(ctx, user.ID, user.Username, service)
	if err != nil {
		return nil, err
	}

	resp := toSuccessfulLoginResponse(toAccountContext(
		organization,
		user,
		"",
	))
	resp.CASTicket = ticket
	return &resp, nil
}

// idTokenIssuance carries everything the ID token must reflect. It is a struct
// rather than a parameter list because the claim rules below are easy to get
// wrong one argument at a time — notably the difference between the issuer, the
// authentication time, and the granted scopes.
type idTokenIssuance struct {
	Issuer      string
	AuthTime    time.Time
	Nonce       string
	Scopes      []string
	AccessToken string
	// SessionID becomes the `sid` claim. The logout token carries the same value,
	// and that equality is the whole point: without it a relying party cannot tell
	// which of its sessions a logout event refers to (设计报告 14.12.3).
	SessionID string
}

// buildIDToken creates a signed OIDC ID token using the JWKS Manager.
//
// Claims follow OIDC Core §3.1.3.3 and the scope gating of §5.4:
//   - `iss` is the same value discovery reports — never a hardcoded constant
//
// (设计报告 14.8);
//   - `auth_time` is when the user actually authenticated, not when the code was
//
// minted;
//   - profile/email/phone claims appear only when the matching scope was granted,
//
// and are omitted rather than blanked when it was not;
//   - `at_hash` binds the ID token to the access token issued alongside it
//
// .
func (s *Service) buildIDToken(
	application *applicationdomain.Application,
	user *userdomain.User,
	organization *orgdomain.Organization,
	issuance idTokenIssuance,
) (string, error) {
	if s.TokenIssuer == nil {
		return "", apperrors.NewInternal("token issuer is not configured for ID token signing")
	}

	granted := newScopeSet(issuance.Scopes)
	hasProfile := granted.has("profile")
	hasEmail := granted.has(contactEmail)
	hasPhone := granted.has(contactPhone)
	hasAddress := granted.has("address")

	signingAlgorithm := tokenSigningAlgorithmForApplication(application.TokenSigningMethod)
	params := tokendomain.IssueIDTokenParams{
		Issuer:             issuance.Issuer,
		Algorithm:          signingAlgorithm,
		Subject:            user.ID,
		Audience:           []string{application.ClientID},
		Nonce:              strings.TrimSpace(issuance.Nonce),
		AuthenticationTime: issuance.AuthTime,
		SessionID:          strings.TrimSpace(issuance.SessionID),
	}
	if params.Issuer == "" {
		return "", apperrors.NewInternal("issuer is not configured for ID token signing")
	}
	if params.AuthenticationTime.IsZero() {
		return "", apperrors.NewInternal("authentication time is required for ID token signing")
	}

	// `openid` alone guarantees only `sub`. Every other profile claim is gated on
	// the scope that owns it.
	if hasProfile {
		params.Name = user.Profile.DisplayName
		params.Username = user.Username
		params.GivenName = user.Profile.FirstName
		params.FamilyName = user.Profile.LastName
		params.Picture = user.Profile.AvatarURL
	}
	if hasEmail {
		emailVerified := user.Profile.EmailVerified
		params.Email = user.Profile.Email
		params.EmailVerified = &emailVerified
	}
	if hasPhone {
		phoneVerified := user.Profile.PhoneVerified
		params.Phone = user.Profile.Phone
		params.PhoneVerified = &phoneVerified
	}
	if hasProfile || hasAddress {
		params.Organization = organization.Slug
	}

	if issuance.AccessToken != "" {
		params.AccessTokenHash = accessTokenHash(issuance.AccessToken, signingAlgorithm)
	}

	idToken, err := s.TokenIssuer.IssueIDToken(params)
	if err != nil {
		return "", apperrors.NewInternal("sign ID token")
	}

	return idToken, nil
}

// accessTokenHash computes the `at_hash` claim: BASE64URL of the left half of
// the hash of the access token, with the hash chosen to match the JOSE `alg`
// (OIDC Core §3.1.3.6): SHA-256 for RS256, SM3 for SM2-SM3.
func accessTokenHash(accessToken, algorithm string) string {
	var digest []byte
	if algorithm == "SM2-SM3" {
		digest = platformcrypto.SM3Sum([]byte(accessToken))
	} else {
		sum := sha256.Sum256([]byte(accessToken))
		digest = sum[:]
	}
	return base64.RawURLEncoding.EncodeToString(digest[:len(digest)/2])
}

// scopeSet answers membership questions for a granted scope list.
type scopeSet map[string]struct{}

func newScopeSet(scopes []string) scopeSet {
	set := make(scopeSet, len(scopes))
	for _, scope := range scopes {
		if trimmed := strings.TrimSpace(scope); trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	return set
}

func (s scopeSet) has(scope string) bool {
	_, ok := s[scope]
	return ok
}
