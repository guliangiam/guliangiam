package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	backchannelLogoutTokenTTL    = 5 * time.Minute
	backchannelLogoutHTTPTimeout = 10 * time.Second
)

// backchannelHTTPClient is the package-level HTTP client used by
// postBackchannelLogoutToken. Tests override it with http.DefaultClient
// so httptest.NewServer URLs work (SafeHTTPClient blocks loopback).
var backchannelHTTPClient = ssrf.SafeHTTPClient(backchannelLogoutHTTPTimeout)

// SendBackchannelLogoutCommand contains parameters for sending a back-channel logout.
type SendBackchannelLogoutCommand struct {
	UserID         string
	OrganizationID string
	ActorID        string
	ActorType      string
	SessionID      string
	ClientID       string // The application to send the logout token to
	// IssuerURL is the issuer the logout token must declare, resolved by the
	// transport from the same source as discovery (设计报告 14.8).
	IssuerURL string
}

// BackchannelLogoutResult contains the result of a back-channel logout.
type BackchannelLogoutResult struct {
	LogoutToken string `json:"logoutToken"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

// SendBackchannelLogout generates a logout token per OpenID Connect Back-Channel Logout 1.0
// and sends it to the application's back-channel logout URI via HTTP POST.
func (s *Service) SendBackchannelLogout(ctx context.Context, command SendBackchannelLogoutCommand) (*BackchannelLogoutResult, error) {
	if strings.TrimSpace(command.UserID) == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.ClientID) == "" {
		return nil, apperrors.NewInvalidArgument("clientId is required")
	}
	actorID := strings.TrimSpace(command.ActorID)
	if actorID == "" {
		return nil, apperrors.NewUnauthorized("authentication required")
	}
	if command.ActorType != "platform_admin" && actorID != strings.TrimSpace(command.UserID) {
		return nil, apperrors.NewForbidden("cannot revoke another user's tokens")
	}
	if strings.TrimSpace(command.OrganizationID) == "" {
		return nil, apperrors.NewForbidden("organization context is required")
	}

	var result *BackchannelLogoutResult
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByClientID(ctx, strings.TrimSpace(command.ClientID))
		if err != nil {
			return apperrors.NewNotFound("application not found")
		}
		if application.OrganizationID != strings.TrimSpace(command.OrganizationID) {
			return apperrors.NewForbidden("application does not belong to the actor's organization")
		}

		// The logout token is a token of this application, so it must use the
		// application's token_signing_method. Rejecting an out-of-enum value here
		// keeps a misconfigured application from silently falling back to RS256.
		signingAlgorithm, err := logoutTokenSigningAlgorithm(application.TokenSigningMethod)
		if err != nil {
			return err
		}

		// Generate the logout token JWT.
		now := s.Clock.Now().UTC()
		jti, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate logout token jti")
		}

		// Build logout token claims per OIDC Back-Channel Logout spec.
		// Per spec: iss MUST be the OP issuer identifier, aud MUST be the client_id.
		// The issuer is the deployment's own, not a literal: a hardcoded value made
		// every relying party reject the token on any other domain.
		claims := map[string]any{
			"iss":    s.resolveIssuer(command.IssuerURL),
			"aud":    application.ClientID,
			"iat":    now.Unix(),
			"jti":    jti,
			"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": struct{}{}},
		}

		if strings.TrimSpace(command.UserID) != "" {
			claims["sub"] = strings.TrimSpace(command.UserID)
		}
		// The claim is the OIDC session identifier, not the internal primary key
		// the caller passed in. `sessions.id` is the session cookie's value and a
		// CAS ticket component; putting it in a token handed to a relying party
		// both leaks that handle and gives the RP a `sid` that matches nothing in
		// the ID token (设计报告 14.12.3).
		if sid := sessionSidFor(ctx, tx, command.SessionID); sid != "" {
			claims["sid"] = sid
		}

		// Sign the logout token using the platform JWKS key if available.
		logoutToken, err := signLogoutTokenWithJWKS(claims, s.JWKSManager, signingAlgorithm)
		if err != nil {
			return apperrors.NewInternalf("sign logout token: %s", err.Error())
		}

		// Send the logout token to the application's back-channel logout URI.
		logoutURI := strings.TrimSpace(application.BackchannelLogoutUri)
		if logoutURI != "" {
			sendErr := postBackchannelLogoutToken(ctx, logoutURI, logoutToken)
			if sendErr != nil {
				// Log the error but don't fail the entire operation.
				// Back-channel logout is best-effort per OIDC spec.
				result = &BackchannelLogoutResult{
					LogoutToken: logoutToken,
					Status:      "delivered_with_error",
					Error:       sendErr.Error(),
				}
			} else {
				result = &BackchannelLogoutResult{
					LogoutToken: logoutToken,
					Status:      "delivered",
				}
			}
		} else {
			// No backchannel_logout_uri configured: token generated but not delivered.
			result = &BackchannelLogoutResult{
				LogoutToken: logoutToken,
				Status:      "generated",
			}
		}

		// Revoke all tokens for the user in this application.
		_, revokeErr := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{
			OrganizationID: application.OrganizationID,
			UserID:         strings.TrimSpace(command.UserID),
			ApplicationID:  application.ID,
		}, now)
		if revokeErr != nil {
			return revokeErr
		}

		return nil
	})
	if err != nil {
		var appErr *apperrors.Error
		if errors.As(err, &appErr) &&
			(appErr.Code == apperrors.CodeForbidden || appErr.Code == apperrors.CodeInvalidArgument) {
			return nil, appErr
		}
		return nil, apperrors.WrapInternal(err, "send backchannel logout transaction")
	}

	return result, nil
}

// sendBackchannelLogoutNotifications sends back-channel logout tokens to all applications
// that the user has active sessions with. This is called during the Logout flow.
func (s *Service) sendBackchannelLogoutNotifications(ctx context.Context, userID, requestIssuer string) {
	// Find all active sessions for the user to determine which applications to notify.
	activeOnly := true
	sessions, _, err := s.UOW.Sessions().List(ctx, sessiondomain.ListQuery{
		UserID:     userID,
		ActiveOnly: &activeOnly,
		PageSize:   100,
	})
	if err != nil {
		return
	}

	// Collapse the sessions to the distinct applications they belong to before
	// touching the database. A user can hold many sessions against the same
	// application (one per device/refresh family), so iterating the sessions
	// would issue one application read per session — and re-read the same
	// application for every session whenever it has no back-channel URI to
	// notify.
	//
	// The collapse also has to keep one session per application: the logout token
	// carries that session's `sid`, and a single shared value would tell every
	// relying party to destroy a session it never had (设计报告 14.12.3).
	seen := make(map[string]bool, len(sessions))
	applicationIDs := make([]string, 0, len(sessions))
	sidByApplication := make(map[string]string, len(sessions))
	for _, sess := range sessions {
		if sess.ApplicationID == "" {
			continue
		}
		if _, ok := sidByApplication[sess.ApplicationID]; !ok && sess.Sid != "" {
			sidByApplication[sess.ApplicationID] = sess.Sid
		}
		if seen[sess.ApplicationID] {
			continue
		}
		seen[sess.ApplicationID] = true
		applicationIDs = append(applicationIDs, sess.ApplicationID)
	}

	for _, applicationID := range applicationIDs {
		application, err := s.UOW.Applications().GetByID(ctx, applicationID)
		if err != nil {
			continue
		}

		if strings.TrimSpace(application.BackchannelLogoutUri) == "" {
			continue
		}

		signingAlgorithm, algErr := logoutTokenSigningAlgorithm(application.TokenSigningMethod)
		if algErr != nil {
			// Best-effort fan-out: an application with an unusable
			// token_signing_method must not receive a token signed with another
			// algorithm, so skip it rather than downgrade to RS256.
			continue
		}

		// Generate and send the logout token.
		now := s.Clock.Now().UTC()
		jti, jtiErr := s.UUIDGenerator.NewString()
		if jtiErr != nil {
			continue
		}

		claims := map[string]any{
			"iss":    s.resolveIssuer(requestIssuer),
			"aud":    application.ClientID,
			"iat":    now.Unix(),
			"jti":    jti,
			"sub":    userID,
			"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": struct{}{}},
		}
		// Per-application sid, so the RP destroys the session this event is about
		// rather than whatever session the caller happened to be holding.
		if sid := sidByApplication[applicationID]; sid != "" {
			claims["sid"] = sid
		}

		logoutToken, signErr := signLogoutTokenWithJWKS(claims, s.JWKSManager, signingAlgorithm)
		if signErr != nil {
			continue
		}

		// Fire-and-forget: best effort per OIDC spec.
		_ = postBackchannelLogoutToken(ctx, application.BackchannelLogoutUri, logoutToken)
	}
}

// postBackchannelLogoutToken sends the logout token to the application's
// back-channel logout URI via HTTP POST with application/x-www-form-urlencoded body.
// Per OIDC Back-Channel Logout spec, the token is sent as the "logout_token" form parameter.
func postBackchannelLogoutToken(ctx context.Context, logoutURI, logoutToken string) error {
	formData := "logout_token=" + urlEncode(logoutToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, logoutURI, bytes.NewBufferString(formData))
	if err != nil {
		return apperrors.WrapInternal(err, "create backchannel logout request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := backchannelHTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternal(err, "send backchannel logout")
	}
	defer resp.Body.Close()

	// Per OIDC spec: 200 OK means the RP accepted the logout notification.
	// Any other status is an error.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("backchannel logout rejected: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// urlEncode performs URL encoding for form values.
func urlEncode(s string) string {
	var buf bytes.Buffer
	for _, b := range []byte(s) {
		switch {
		case isUnreservedByte(b):
			buf.WriteByte(b)
		default:
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

func isUnreservedByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '.' || b == '_' || b == '~'
}

// signLogoutTokenWithJWKS signs a logout token using the platform's JWKS key
// with the algorithm resolved from the application's token_signing_method.
func signLogoutTokenWithJWKS(
	claims map[string]any,
	jwksManager domainauth.JWKSProvider,
	algorithm string,
) (string, error) {
	if jwksManager == nil {
		return "", apperrors.NewInternal("JWKS manager not configured")
	}

	token, err := jwksManager.SignTokenWithAlgorithm(claims, "logout+jwt", algorithm)
	if err != nil {
		return "", err
	}
	return token, nil
}
