package transactions

import (
	"context"
	"encoding/json"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s Service) ListTransactions(ctx context.Context, q ListTransactionsQuery) ([]TransactionListItem, int64, error) {
	items, total, err := s.UOW.Transactions().List(ctx, domain.ListQuery{
		Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID,
		Keyword: q.Keyword, UserID: q.UserID, OrderID: q.OrderID,
		Status: q.Status, TransactionType: q.TransactionType,
	})
	if err != nil {
		return nil, 0, err
	}
	r := make([]TransactionListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}

func (s Service) GetTransaction(ctx context.Context, organizationID, id string) (*TransactionDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	t, err := s.UOW.Transactions().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if err := ensureTransactionTenantScope(t, organizationID); err != nil {
		return nil, err
	}
	d := toDetail(t)
	return &d, nil
}

func (s Service) CreateTransaction(ctx context.Context, c CreateTransactionCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.TransactionType) == "" {
		return apperrors.NewInvalidArgument("transactionType is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	status := "pending"
	if c.Status != "" {
		status = c.Status
	}
	var metadata json.RawMessage
	if c.Metadata != nil {
		metadata = c.Metadata
	} else {
		metadata = json.RawMessage(`{}`)
	}
	t := &domain.Transaction{
		ID: id, OrganizationID: c.OrganizationID, UserID: c.UserID,
		OrderID: c.OrderID, PaymentID: c.PaymentID,
		TransactionType: c.TransactionType, Amount: c.Amount,
		Currency: c.Currency, Status: status,
		ProviderType: c.ProviderType, ProviderID: c.ProviderID,
		Metadata: metadata, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	if err := t.Validate(); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Transactions().Create(ctx, t); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			t.UserID,
			"transaction.created",
			t.ID,
			"transaction.created",
			map[string]any{

				keyTransactionID: t.ID,

				keyName: t.TransactionType,

				keyOrgID: t.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}

func (s Service) PatchTransaction(ctx context.Context, organizationID string, c PatchTransactionCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation, so a cross-tenant request
		// leaves no write and no audit record behind.
		t, err := tx.Transactions().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if err := ensureTransactionTenantScope(t, organizationID); err != nil {
			return err
		}
		if c.Revision > 0 && t.Revision != c.Revision {
			return apperrors.NewConflict("transaction revision mismatch")
		}
		if c.Status != nil {
			if err := t.TransitionTo(*c.Status); err != nil {
				return err
			}
		}
		if c.Metadata != nil {
			t.Metadata = c.Metadata
		}
		t.UpdatedAt = s.Clock.Now()
		t.Revision++

		if err := tx.Transactions().Update(ctx, t); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			t.UserID,
			"transaction.updated",
			t.ID,
			"transaction.updated",
			map[string]any{

				keyTransactionID: t.ID,

				keyName: t.TransactionType,

				keyOrgID: t.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}

func (s Service) DeleteTransaction(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the audit
		// row carries the entity's real organization.
		t, err := tx.Transactions().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := ensureTransactionTenantScope(t, organizationID); err != nil {
			return err
		}

		if err := tx.Transactions().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			t.UserID,
			"transaction.deleted",
			t.ID,
			"transaction.deleted",
			map[string]any{

				keyTransactionID: t.ID,

				keyName: t.TransactionType,

				keyOrgID: t.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}
