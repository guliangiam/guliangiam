package auth

import (
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateAccountQuery(query AccountQuery) error {
	if strings.TrimSpace(query.OrganizationSlug) == "" {
		return apperrors.NewUnauthorized("organization context is required")
	}
	if strings.TrimSpace(query.Username) == "" {
		return apperrors.NewUnauthorized("user context is required")
	}
	return nil
}

func validateAuthorizeCommand(command AuthorizeCommand) error {
	if strings.TrimSpace(command.OrganizationSlug) == "" {
		return apperrors.NewUnauthorized("organization context is required")
	}
	if strings.TrimSpace(command.Username) == "" {
		return apperrors.NewUnauthorized("user context is required")
	}
	if strings.TrimSpace(command.ClientID) == "" {
		return apperrors.NewInvalidArgument("clientId is required")
	}
	if strings.TrimSpace(command.RedirectURI) == "" {
		return apperrors.NewInvalidArgument("redirectUri is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(command.RedirectURI))
	if err != nil || !parsed.IsAbs() {
		return apperrors.NewInvalidArgument("redirectUri must be a valid absolute URL")
	}
	// Only the authorization-code response type is served. The implicit response
	// types are retired (设计报告 14.6.1「禁用 response_type=token」): the protocol face answers
	// `unsupported_response_type` rather than downgrading the request.
	if responseType := strings.TrimSpace(command.ResponseType); responseType != "" && responseType != fieldCode {
		return newUnsupportedResponseTypeError(responseType)
	}
	challengeMethod := strings.TrimSpace(command.ChallengeMethod)
	if challengeMethod != "" && challengeMethod != "S256" {
		return apperrors.NewInvalidArgument("codeChallengeMethod must be S256 when provided")
	}
	if strings.TrimSpace(command.CodeChallenge) == "" && challengeMethod != "" {
		return apperrors.NewInvalidArgument("codeChallenge is required when codeChallengeMethod is provided")
	}
	if strings.TrimSpace(command.CodeChallenge) != "" && challengeMethod == "" {
		return apperrors.NewInvalidArgument("codeChallengeMethod is required when codeChallenge is provided")
	}
	return nil
}

func validateExchangeAuthorizationCodeCommand(command ExchangeAuthorizationCodeCommand) error {
	grantType := strings.TrimSpace(command.GrantType)
	if grantType == "" {
		grantType = grantAuthorizationCode
	}
	if grantType != grantAuthorizationCode {
		return apperrors.NewInvalidArgument("grantType must be authorization_code")
	}
	if strings.TrimSpace(command.ClientID) == "" {
		return apperrors.NewInvalidArgument("clientId is required")
	}
	if strings.TrimSpace(command.Code) == "" {
		return apperrors.NewInvalidArgument("code is required")
	}
	if strings.TrimSpace(command.RedirectURI) == "" {
		return apperrors.NewInvalidArgument("redirectUri is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(command.RedirectURI))
	if err != nil || !parsed.IsAbs() {
		return apperrors.NewInvalidArgument("redirectUri must be a valid absolute URL")
	}
	return nil
}
