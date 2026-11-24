package auth

import (
	"context"
	"net/url"
	"strings"

	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// FrontChannelTarget is one RP iframe to load during front-channel logout
// (OpenID Connect Front-Channel Logout 1.0 §2).
type FrontChannelTarget struct {
	ApplicationSlug string
	URL             string
}

// FrontChannelLogoutResult is the set of RP iframes for the user's active
// sessions plus the issuer used in their query strings.
type FrontChannelLogoutResult struct {
	Issuer  string
	Targets []FrontChannelTarget
}

// BuildFrontChannelLogout collects the frontchannel_logout_uri of every
// application the user has an active session with. Targets are built from the
// registered URI and carry iss (always) and sid (when the application requires
// session identification). The caller revokes the session and renders iframes.
func (s *Service) BuildFrontChannelLogout(ctx context.Context, sessionID, issuerURL string) (*FrontChannelLogoutResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, apperrors.NewUnauthorized("session id is required")
	}
	session, err := s.UOW.Sessions().GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	active := true
	sessions, _, err := s.UOW.Sessions().List(ctx, sessiondomain.ListQuery{
		UserID:     session.UserID,
		ActiveOnly: &active,
		PageSize:   100,
	})
	if err != nil {
		return nil, err
	}

	issuer := s.resolveIssuer(issuerURL)
	result := &FrontChannelLogoutResult{Issuer: issuer}
	seen := map[string]bool{}
	for _, activeSession := range sessions {
		if activeSession.ApplicationID == "" || seen[activeSession.ApplicationID] {
			continue
		}
		seen[activeSession.ApplicationID] = true
		application, err := s.UOW.Applications().GetByID(ctx, activeSession.ApplicationID)
		if err != nil {
			// A deleted/unreadable application must not block the logout of the
			// remaining RPs; skip it.
			continue
		}
		uri := strings.TrimSpace(application.FrontchannelLogoutUri)
		if uri == "" {
			continue
		}
		targetURL, err := buildFrontChannelURL(uri, issuer, activeSession.Sid, application.FrontchannelLogoutSessionRequired)
		if err != nil {
			continue // invalid registered URI: skip rather than break logout
		}
		result.Targets = append(result.Targets, FrontChannelTarget{
			ApplicationSlug: application.Slug,
			URL:             targetURL,
		})
	}
	return result, nil
}

func buildFrontChannelURL(raw, issuer, sid string, includeSid bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", apperrors.NewInvalidArgument("frontchannel_logout_uri must be absolute http(s)")
	}
	q := parsed.Query()
	if issuer != "" {
		q.Set("iss", issuer)
	}
	if includeSid && sid != "" {
		q.Set("sid", sid)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}
