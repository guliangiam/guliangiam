package tokens

import (
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.AuthUoW
}

type CreateTokenCommand struct {
	OrganizationID          string
	UserID                  string
	ApplicationID           string
	Scopes                  []string
	Audience                []string
	Metadata                map[string]any
	ExpiresInSeconds        int64
	IssueRefreshToken       bool
	RefreshExpiresInSeconds int64
}

type CreateTokenResult struct {
	Token      *TokenDetail
	TokenValue string
}

type RevokeTokensCommand struct {
	OrganizationID string
	UserID         string
	ApplicationID  string
}

type RevokeTokensResult struct {
	RevokedCount int64
}

type IntrospectTokenCommand struct {
	// OrganizationID is the trusted tenant resolved by the transport layer.
	// Empty means the caller has no tenant binding (non-HTTP/internal use).
	OrganizationID string
	Token          string
	TokenTypeHint  string
}

type RefreshAccessTokenCommand struct {
	// OrganizationID is the trusted tenant resolved by the transport layer.
	// Empty means the caller has no tenant binding (non-HTTP/internal use).
	OrganizationID string
	// ApplicationID is the authenticated OAuth client. When set, the refresh
	// token and its access token must both belong to that application; the
	// protocol refresh_token grant passes it after client authentication.
	ApplicationID string
	RefreshToken  string
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.AuthUoW) Service {
	return Service{
		Clock: clk,
		UUID:  uuidGenerator,
		UOW:   unitOfWork,
	}
}

// tokenBelongsToTenant reports whether a token row is visible to the trusted
// tenant. Management HTTP paths always pass a non-empty tenant; an empty value
// only reaches here from non-HTTP/internal callers.
func tokenBelongsToTenant(trustedTenant, tokenTenant string) bool {
	return trustedTenant == "" || tokenTenant == trustedTenant
}

// requireOrganizationID fails fast when a by-ID management call reaches the app
// layer without a trusted tenant. HTTP handlers resolve this via
// TrustedOrganizationID; the guard keeps non-HTTP callers from degrading to a
// cross-tenant lookup.
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}
