package auth

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// ImpersonationSession is the trusted session projection of an opaque
// impersonation token. The transport layer turns it into a SessionContext so
// impersonation requests reach the protected chain (OPEN-API-58).
type ImpersonationSession struct {
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	// ImpersonatedBy is the admin user ID that minted the token
	// (metadata.impersonated_by). The transport layer binds it against the
	// cookie session before letting the imp identity take priority.
	ImpersonatedBy string
}

// ResolveImpersonationSession validates an opaque impersonation token:
//   - only tokens minted with TokenPrefix "imp" and metadata impersonated=true;
//   - the token must be unexpired and not revoked;
//   - scopes must contain "impersonate";
//   - the target user and organization must still exist.
//
// Any failure is NotFound so callers cannot enumerate tokens.
func (s *Service) ResolveImpersonationSession(ctx context.Context, rawToken string) (*ImpersonationSession, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, apperrors.NewNotFound("impersonation token not found")
	}
	var session *ImpersonationSession
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		token, err := tx.Tokens().GetByTokenHash(ctx, platformcrypto.HashTokenValue(rawToken))
		if err != nil {
			return err
		}
		if token.TokenPrefix != "imp" || token.RevokedAt != nil || !token.ExpiresAt.After(s.Clock.Now()) {
			return apperrors.NewNotFound("impersonation token not found")
		}
		if impersonated, _ := token.Metadata["impersonated"].(bool); !impersonated {
			return apperrors.NewNotFound("impersonation token not found")
		}
		hasScope := false
		for _, scope := range token.Scopes {
			if scope == "impersonate" {
				hasScope = true
				break
			}
		}
		if !hasScope {
			return apperrors.NewNotFound("impersonation token not found")
		}
		user, err := tx.Users().GetByID(ctx, token.UserID)
		if err != nil {
			return err
		}
		organization, err := tx.Organizations().GetByID(ctx, user.OrganizationID)
		if err != nil {
			return err
		}
		impersonatedBy, _ := token.Metadata["impersonated_by"].(string)
		session = &ImpersonationSession{
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ImpersonatedBy:   strings.TrimSpace(impersonatedBy),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

var _ = time.Now
