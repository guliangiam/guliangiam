package payments

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// NotifyPayment transitions payment state based on external provider callback
func (s Service) NotifyPayment(ctx context.Context, id, state, message, outOrderId string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		p, err := tx.Payments().GetByID(ctx, id)
		if err != nil {
			return err
		}
		if p.State != domain.StateCreated {
			return apperrors.NewInvalidArgument("payment is not in created state")
		}
		if err := p.TransitionTo(state); err != nil {
			return err
		}
		p.Message = message
		p.OutOrderId = outOrderId
		p.UpdatedAt = s.Clock.Now()
		p.Revision++
		if err := tx.Payments().Update(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			p.OrganizationID,
			p.User,
			"payment.notified",
			p.ID,
			"payment.notified",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
