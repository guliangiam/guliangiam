package bootstrap

import (
	"context"

	appprivacy "github.com/guliangiam/guliangiam/internal/app/privacy"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainprivacy "github.com/guliangiam/guliangiam/internal/domain/privacy"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// privacyTx adapts the transaction handle to the privacy use cases: the DSAR
// repository is bound to the same transaction, and the audit record + outbox
// event are embedded in that same transaction.
type privacyTx struct {
	tx appuow.UnitOfWork
}

func (t privacyTx) Privacy() domainprivacy.DsarRepository {
	provider, ok := t.tx.(interface {
		Privacy() domainprivacy.DsarRepository
	})
	if !ok {
		return nil
	}
	return provider.Privacy()
}

func (t privacyTx) Consents() domainprivacy.ConsentRepository {
	provider, ok := t.tx.(interface {
		PrivacyConsents() domainprivacy.ConsentRepository
	})
	if !ok {
		return nil
	}
	return provider.PrivacyConsents()
}

func (t privacyTx) EmbedAudit(ctx context.Context, event appprivacy.AuditEvent) error {
	return audit.EmbedAuditEvent(
		clock.SystemClock{}, t.tx, ctx,
		"", "", "", "", "",
		event.OrganizationID, event.ActorID, event.Action,
		event.ResourceType, event.ResourceID, event.AggregateType, event.EventType,
		event.Payload,
	)
}

type privacyRunner struct {
	uow appuow.UnitOfWork
}

func newPrivacyRunner(uow appuow.UnitOfWork) appprivacy.TransactionRunner {
	return privacyRunner{uow: uow}
}

func (r privacyRunner) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appprivacy.Tx) error) error {
	return r.uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return fn(ctx, privacyTx{tx: tx})
	})
}
