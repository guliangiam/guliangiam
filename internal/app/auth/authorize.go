package auth

import (
	"context"
	"net/url"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const authorizationCodeTTL = 5 * time.Minute

var defaultOAuthScopes = map[string]struct{}{
	scopeOpenID:        {},
	scopeProfile:       {},
	scopeEmail:         {},
	scopePhone:         {},
	"address":          {},
	scopeOfflineAccess: {},
}

func (s *Service) Authorize(ctx context.Context, command AuthorizeCommand) (*AuthorizeResponse, error) {
	if err := validateAuthorizeCommand(command); err != nil {
		return nil, err
	}

	organization, user, application, scopes, err := s.resolveAuthorizeContext(ctx, command)
	if err != nil {
		return nil, err
	}

	// PKCE is mandatory for every client (OAuth 2.1, ADR-004). The per-application
	// pkceRequired flag no longer narrows this rule.
	if err := validatePKCEBinding(command.CodeChallenge, command.ChallengeMethod); err != nil {
		return nil, err
	}

	// Only the authorization-code response type is served: the implicit flow was
	// retired (设计报告 14.6.1「禁用 response_type=token」), so `token` and `id_token` are answered with
	// the RFC 6749 §4.1.2.1 unsupported_response_type code instead of being
	// silently downgraded. validateAuthorizeCommand rejects the value before the
	// service is reached on the HTTP face; this guard covers direct callers.
	responseType := strings.TrimSpace(command.ResponseType)
	if responseType == "" {
		responseType = fieldCode
	}
	if responseType != fieldCode {
		return nil, newUnsupportedResponseTypeError(responseType)
	}

	grantedScopes, err := s.getActiveGrantedScopes(ctx, user.ID, application.ID)
	if err != nil {
		return nil, err
	}
	consentableScopes := filterConsentableScopes(scopes, application.CustomScopes)
	missingScopes := differenceScopes(consentableScopes, grantedScopes)
	if len(missingScopes) > 0 {
		return &AuthorizeResponse{
			Status: "consent_required",
			Evaluation: &ConsentEvaluationDetail{
				ApplicationID:     application.ID,
				ApplicationSlug:   application.Slug,
				RequestedScopes:   scopes,
				ConsentableScopes: consentableScopes,
				GrantedScopes:     grantedScopes,
				MissingScopes:     missingScopes,
				Required:          len(consentableScopes) > 0,
			},
		}, nil
	}

	now := s.Clock.Now().UTC()

	// Authorization code flow. The code is built by the shared constructor, so
	// this path and the post-login path cannot diverge on entropy, TTL, or the
	// set of bound fields (state/nonce/PKCE/resource).
	var response *AuthorizeResponse
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		authorizationCode, err := s.newAuthorizationCode(authorizationCodeIssuance{
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			UserID:           user.ID,
			Username:         user.Username,
			ApplicationID:    application.ID,
			ApplicationSlug:  application.Slug,
			RedirectURI:      command.RedirectURI,
			Scopes:           scopes,
			State:            command.State,
			Nonce:            command.Nonce,
			CodeChallenge:    command.CodeChallenge,
			ChallengeMethod:  command.ChallengeMethod,
			Resource:         command.Resource,
			Sid:              command.SessionID,
			AuthTime:         command.AuthTime,
		}, now)
		if err != nil {
			return err
		}

		if err := tx.AuthorizationCodes().Create(ctx, authorizationCode); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"protocol",
			organization.ID,
			user.ID,
			"authorization_code.created",
			"authorization_code",
			authorizationCode.ID,
			"authorization_code",
			"authorization_code.created",
			map[string]any{},
		); err != nil {
			return err
		}
		expiresAt := authorizationCode.ExpiresAt
		response = &AuthorizeResponse{
			Status: "authorized",
			Code: &AuthorizationCodeDetail{
				Code:        authorizationCode.Code,
				RedirectURI: authorizationCode.RedirectURI,
				RedirectTo:  buildAuthorizationRedirectURL(authorizationCode.RedirectURI, authorizationCode.Code, authorizationCode.State),
				State:       authorizationCode.State,
				ExpiresAt:   &expiresAt,
			},
		}
		return nil
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "authorize transaction")
	}

	return response, nil
}

func (s *Service) resolveAuthorizeContext(ctx context.Context, command AuthorizeCommand) (*orgdomain.Organization, *userdomain.User, *applicationdomain.Application, []string, error) {
	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, nil, nil, nil, apperrors.WrapInternalf(err, "lookup organization by slug %q", command.OrganizationSlug)
	}
	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		return nil, nil, nil, nil, apperrors.WrapInternalf(err, "lookup user %q in organization %s", command.Username, organization.ID)
	}
	application, err := s.UOW.Applications().GetByClientID(ctx, command.ClientID)
	if err != nil {
		return nil, nil, nil, nil, apperrors.WrapInternalf(err, "lookup application by client_id %q", command.ClientID)
	}
	if application.OrganizationID != organization.ID {
		return nil, nil, nil, nil, apperrors.NewInvalidArgument("application does not belong to organization")
	}
	if !applicationdomain.MatchRedirectURI(command.RedirectURI, application.RedirectURIs) {
		return nil, nil, nil, nil, apperrors.NewInvalidArgument("redirectUri is not registered for the application")
	}
	if !containsString(application.GrantTypes, "authorization_code") {
		return nil, nil, nil, nil, apperrors.NewInvalidArgument("application does not allow authorization_code grant")
	}

	scopes, err := validateAuthorizeScopes(command.RequestedScopes, application.CustomScopes)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return organization, user, application, scopes, nil
}

func validateAuthorizeScopes(requestedScopes []string, customScopes []applicationdomain.ApplicationCustomScope) ([]string, error) {
	normalized := normalizeScopes(requestedScopes)
	if len(normalized) == 0 {
		return []string{}, nil
	}

	customScopeSet := make(map[string]struct{}, len(customScopes))
	for _, scope := range customScopes {
		name := strings.TrimSpace(scope.Scope)
		if name == "" {
			continue
		}
		customScopeSet[name] = struct{}{}
	}

	invalidScopes := make([]string, 0)
	for _, scope := range normalized {
		if _, ok := defaultOAuthScopes[scope]; ok {
			continue
		}
		if _, ok := customScopeSet[scope]; ok {
			continue
		}
		invalidScopes = append(invalidScopes, scope)
	}
	if len(invalidScopes) > 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "requestedScopes contains unsupported scope", map[string]any{"invalidScopes": invalidScopes})
	}

	return normalized, nil
}

func containsString(values []string, expected string) bool {
	needle := strings.TrimSpace(expected)
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}

func buildAuthorizationRedirectURL(redirectURI, code, state string) string {
	parsed, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil {
		return ""
	}
	query := parsed.Query()
	query.Set(fieldCode, code)
	if strings.TrimSpace(state) != "" {
		query.Set("state", strings.TrimSpace(state))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
