package bootstrap

import (
	"context"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainpayments "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// channelConfigResolver resolves the active channel configuration a charge may
// use. Payment records do not carry an environment column; production is the
// default channel environment (channel configs default to production), so the
// resolver pins it explicitly and reports any missing/inactive config to the
// ChargeOrchestrator as payment_channel_not_activated.
type channelConfigResolver struct {
	repo domainpayments.ChannelConfigsRepo
}

func (r channelConfigResolver) ResolveActive(ctx context.Context, organizationID, channel string) (*domainpayments.PayChannelConfig, error) {
	return r.repo.GetActive(ctx, organizationID, channel, "production")
}

var _ interface {
	ResolveActive(ctx context.Context, organizationID, channel string) (*domainpayments.PayChannelConfig, error)
} = channelConfigResolver{}

// chargeTx binds the payment repository and the audit writer to one transaction.
type chargeTx struct {
	tx appuow.UnitOfWork
}

func (c chargeTx) PaymentRepo() domainpayments.Repository {
	provider, ok := c.tx.(interface {
		Payments() domainpayments.Repository
	})
	if !ok {
		return nil
	}
	return provider.Payments()
}

func (c chargeTx) EmbedAudit(ctx context.Context, event apppayments.ChargeAuditEvent) error {
	return audit.EmbedAuditEvent(
		clock.SystemClock{}, c.tx, ctx,
		"", "", "", "", "",
		event.OrganizationID, event.ActorID, event.Action,
		event.ResourceType, event.ResourceID, event.AggregateType, event.EventType,
		event.Payload,
	)
}

type chargeRunner struct {
	uow appuow.UnitOfWork
}

func (r chargeRunner) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx apppayments.ChargeTx) error) error {
	return r.uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return fn(ctx, chargeTx{tx: tx})
	})
}
