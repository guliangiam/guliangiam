package sessions

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformssf "github.com/guliangiam/guliangiam/internal/platform/ssf"
)

// enqueueSSFSessionRevoked writes one CAEP session-revoked event for a single
// session in the same transaction as the revocation.
func (s *Service) enqueueSSFSessionRevoked(ctx context.Context, tx appuow.UnitOfWork, orgID, userID, sessionID string) error {
	return s.enqueueSSFRevoked(ctx, tx, orgID, userID, sessionID, "session_revoked")
}

// enqueueSSFRevoked writes one CAEP session-revoked event. Empty subject fields
// are omitted (bulk/administrative revocations may not identify one session).
func (s *Service) enqueueSSFRevoked(ctx context.Context, tx appuow.UnitOfWork, orgID, userID, sessionID, reason string) error {
	if tx.Outbox() == nil {
		return nil
	}
	issuer := config.ConfiguredIssuerURL()
	if issuer == "" {
		// Dev/test without ISSUER_URL: keep the revocation working; delivery is
		// not possible without an `iss` claim.
		return nil
	}
	event, err := platformssf.NewRevocationEvent(issuer, orgID, userID, sessionID, reason, time.Now().UTC())
	if err != nil {
		return err
	}
	return tx.Outbox().Enqueue(ctx, event)
}
