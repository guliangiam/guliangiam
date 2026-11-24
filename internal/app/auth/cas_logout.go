package auth

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CasLogoutAction is the audit action recorded by CAS single logout. The naming
// follows the `resource.verb` shape already used by `user.login` / `user.logout`.
const CasLogoutAction = "user.cas_logout"

// casLogoutApplicationPageSize bounds the per-organization application scan used
// to assemble the redirect whitelist. It is a page size, not a cap: the scan
// keeps paging until the organization's applications are exhausted.
const casLogoutApplicationPageSize = 100

// CasLogoutCommand drives CAS single logout (设计报告 28.10.5 FE-014).
type CasLogoutCommand struct {
	SessionID string
	IssuerURL string
	// Service is the URI the caller asked to be redirected back to. It is only
	// ever echoed back when it matches the redirect URIs registered by the
	// applications of the session's organization, so the endpoint cannot be used
	// as an open redirect.
	Service   string
	IPAddress string
	UserAgent string
	// Surface / RequestID / TraceID come from the transport layer's request
	// metadata (手册 12.5) and are never client supplied.
	Surface   string
	RequestID string
	TraceID   string
}

// CasLogoutResult reports what the call actually did, so the caller never has to
// infer an outcome from an empty field.
type CasLogoutResult struct {
	// RevokedSessions counts the SSO sessions terminated by this call. Zero means
	// the subject had no live session, which is the idempotent repeat-visit case.
	RevokedSessions int
	// RedirectURI is non-empty only when the requested service matched a
	// registered redirect URI.
	RedirectURI string
	// RedirectRejected is true when the caller supplied a service that is not a
	// registered redirect URI. The redirect is refused and the logout still
	// stands; the two are reported separately so no caller has to guess which of
	// "no service", "unmatched service" and "logout failed" it is looking at.
	RedirectRejected bool
}

// CasLogout terminates the subject's SSO sessions and decides whether the caller
// may be sent back to the service it named.
//
// The order matters: the redirect target is resolved before any session is
// revoked, so a refused redirect never leaves a half-applied logout behind.
// The two revocations and the audit record then commit in one transaction, so
// the audit can never describe a logout that did not happen (BE-08). The call
// is idempotent (28.10.3 FE-014): an absent cookie, an expired session and a
// repeat visit all return the same success shape.
func (s *Service) CasLogout(ctx context.Context, command CasLogoutCommand) (*CasLogoutResult, error) {
	result := &CasLogoutResult{}

	sessionID := strings.TrimSpace(command.SessionID)
	if sessionID == "" {
		return result, nil
	}

	session, err := s.UOW.Sessions().GetByID(ctx, sessionID)
	if err != nil {
		if isIgnorableLogoutError(err) {
			return result, nil
		}
		return nil, apperrors.WrapInternalf(err, "lookup session %s for CAS logout", sessionID)
	}

	result.RedirectURI, result.RedirectRejected, err = s.resolveCasServiceRedirect(ctx, session.OrganizationID, command.Service)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "resolve CAS service redirect for organization %s", session.OrganizationID)
	}

	now := s.Clock.Now().UTC()

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if revokeErr := tx.Sessions().Revoke(ctx, sessionID, now); revokeErr != nil && !isIgnorableLogoutError(revokeErr) {
			return apperrors.WrapInternalf(revokeErr, "revoke session %s for CAS logout", sessionID)
		}
		result.RevokedSessions = 1

		// SLO fan-out (28.10.5): single logout ends the subject's sessions in this
		// application *and* in the other applications sharing the organization. The
		// per-application alternative (RevokeByUserAndApp) would leave the subject
		// signed in elsewhere, which is exactly what CAS SLO exists to prevent.
		revoked, err := tx.Sessions().RevokeByUserExcept(ctx, session.UserID, sessionID, now)
		if err != nil {
			return apperrors.WrapInternalf(err, "revoke other sessions for user %s except %s", session.UserID, sessionID)
		}
		result.RevokedSessions += int(revoked)

		if err := s.writeCasLogoutAudit(ctx, tx, session, command, result); err != nil {
			return apperrors.WrapInternal(err, "CAS logout audit write")
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Back-channel notifications contact external applications; they run after
	// the transaction commits and never fail the logout (best-effort, S-12).
	if s.JWKSManager != nil {
		s.sendBackchannelLogoutNotifications(ctx, session.UserID, command.IssuerURL)
	}

	return result, nil
}

// resolveCasServiceRedirect applies the redirect whitelist: the requested value
// is echoed back only when applications.MatchRedirectURI accepts it against the
// union of the organization's registered redirect URIs. An empty request yields
// no redirect and no rejection — the caller simply asked for none.
func (s *Service) resolveCasServiceRedirect(ctx context.Context, organizationID, service string) (
	redirectURI string, rejected bool, err error,
) {
	service = strings.TrimSpace(service)
	if service == "" {
		return "", false, nil
	}

	registered, err := s.registeredRedirectURIs(ctx, organizationID)
	if err != nil {
		return "", false, err
	}

	if !applications.MatchRedirectURI(service, registered) {
		return "", true, nil
	}

	return service, false, nil
}

// registeredRedirectURIs collects the redirect URIs of every application in the
// organization. The union is the whitelist the CAS `service` parameter is
// matched against; it is never supplied by the caller.
func (s *Service) registeredRedirectURIs(ctx context.Context, organizationID string) ([]string, error) {
	if organizationID == "" {
		return nil, nil
	}

	uris := make([]string, 0, casLogoutApplicationPageSize)
	for page := 1; ; page++ {
		list, total, err := s.UOW.Applications().List(ctx, applications.ListQuery{
			Page:           page,
			PageSize:       casLogoutApplicationPageSize,
			OrganizationID: organizationID,
		})
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "list applications page %d for organization %s", page, organizationID)
		}

		for _, application := range list {
			if application == nil {
				continue
			}
			uris = append(uris, application.RedirectURIs...)
		}

		if len(list) == 0 || int64(page*casLogoutApplicationPageSize) >= total {
			return uris, nil
		}
	}
}

// writeCasLogoutAudit records the logout on the append-only audit face (41 章)
// inside the same transaction as the revocations. The request metadata is
// taken from the transport, never from the body.
func (s *Service) writeCasLogoutAudit(
	ctx context.Context,
	tx appuow.UnitOfWork,
	session *sessiondomain.Session,
	command CasLogoutCommand,
	result *CasLogoutResult,
) error {
	id, err := s.UUIDGenerator.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "generate cas logout record id")
	}

	record := &domainrecords.Record{
		ID:             id,
		OrganizationID: session.OrganizationID,
		ActorID:        session.UserID,
		Action:         CasLogoutAction,
		ResourceType:   "user",
		ResourceID:     session.UserID,
		IP:             command.IPAddress,
		UserAgent:      command.UserAgent,
		Surface:        domainrecords.NormalizeSurface(command.Surface),
		RequestID:      command.RequestID,
		TraceID:        command.TraceID,
		// The requested `service` value is deliberately absent: it is
		// caller-controlled and a rejected value would otherwise be stored in the
		// audit face and surfaced by whatever renders it.
		Metadata: map[string]any{
			"revokedSessions":  result.RevokedSessions,
			"redirectRejected": result.RedirectRejected,
		},
		CreatedAt: s.Clock.Now().UTC(),
	}

	if err := tx.Records().Create(ctx, record); err != nil {
		return apperrors.WrapInternal(err, "create CAS logout audit record")
	}
	return nil
}
