// audit_outbox.go — 薄 wrapper 委托给 internal/platform/outbox/providers/audit（审计事件唯一写入提供者）。
// 共享实现（单一事实源）在 internal/platform/outbox/providers/audit。
package invitations

import (
	"context"
	"errors"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// embedAuditEvent 为 Service 在同一事务内写一条审计 Record + 一条
// OutboxEvent。内部委托给 platform/outbox/providers/audit.EmbedAuditEvent。
func (s *Service) embedAuditEvent(
	tx appuow.UnitOfWork, ctx context.Context,
	orgID, action, resourceID, eventType string,
	payload map[string]any,
) error {
	return audit.EmbedAuditEvent(
		s.Clock, tx, ctx,
		"", "", "", "", "",
		orgID, "", action, "invitations", resourceID,
		"invitations", eventType,
		payload,
	)
}

// invitationTxReject wraps a business-rejection error with audit metadata so
// the caller can consume it after the mutation transaction rolls back — a
// write inside a tx that is about to rollback is useless.
type invitationTxReject struct {
	EventType string
	OrgID     string
	InvID     string
	Payload   map[string]any
	Err       error
}

func (e *invitationTxReject) Error() string { return e.Err.Error() }
func (e *invitationTxReject) Unwrap() error { return e.Err }

// recordInvitationRejectAudit writes one audit record + outbox event in its
// own short-lived System() transaction — the only safe place for a rejection
// audit (the mutation tx that produced it has just rolled back).
func (s *Service) recordInvitationRejectAudit(
	ctx context.Context, orgID, invID, eventType string, payload map[string]any,
) {
	_ = s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.embedAuditEvent(
			tx, ctx,
			orgID,
			"invitations.rejected",
			invID,
			eventType,
			payload,
		)
	})
}

// unwrapInvitationTxReject checks whether err is an invitationTxReject and,
// if so, writes its audit trail (outside any transaction) before returning
// the wrapped business error to the caller.
func (s *Service) unwrapInvitationTxReject(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var rej *invitationTxReject
	if errors.As(err, &rej) {
		s.recordInvitationRejectAudit(ctx, rej.OrgID, rej.InvID, rej.EventType, rej.Payload)
		return rej.Err
	}
	return err
}
