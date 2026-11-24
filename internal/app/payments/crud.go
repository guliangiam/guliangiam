package payments

import (
	"context"
	"strings"

	"github.com/google/uuid"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func (s Service) ListPayments(ctx context.Context, q ListPaymentsQuery) ([]PaymentListItem, int64, error) {
	items, total, err := s.UOW.Payments().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]PaymentListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetPayment(ctx context.Context, organizationID, id string) (*PaymentDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	p, err := s.UOW.Payments().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(p)
	return &d, nil
}
func (s Service) CreatePayment(ctx context.Context, c CreatePaymentCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	state := domain.StateCreated
	if c.State != "" {
		state = c.State
	}
	p := &domain.Payment{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Provider: c.Provider, Type: c.Type, Products: append([]string(nil), c.Products...),
		ProductName: c.ProductName, ProductDisplayName: c.ProductDisplayName, Detail: c.Detail,
		Currency: c.Currency, Price: c.Price, User: c.User, PersonName: c.PersonName,
		PersonIdCard: c.PersonIdCard, PersonEmail: c.PersonEmail, PersonPhone: c.PersonPhone,
		InvoiceType: c.InvoiceType, InvoiceTitle: c.InvoiceTitle, InvoiceTaxId: c.InvoiceTaxId,
		InvoiceRemark: c.InvoiceRemark, InvoiceUrl: c.InvoiceUrl, Order: c.Order,
		OutOrderId: c.OutOrderId, PayUrl: c.PayUrl, SuccessUrl: c.SuccessUrl, State: state,
		Message: c.Message, CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err := p.Validate(); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Payments().Create(ctx, p); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			p.OrganizationID,
			p.User,
			"payment.created",
			p.ID,
			"payment.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchPayment(ctx context.Context, organizationID string, c PatchPaymentCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		p, err := tx.Payments().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && p.Revision != c.Revision {
			return apperrors.NewConflict("payment revision mismatch")
		}
		if c.DisplayName != nil {
			p.DisplayName = *c.DisplayName
		}
		if c.Provider != nil {
			p.Provider = *c.Provider
		}
		if c.Type != nil {
			p.Type = *c.Type
		}
		if c.State != nil {
			if err := p.TransitionTo(*c.State); err != nil {
				return err
			}
		}
		if c.Message != nil {
			p.Message = *c.Message
		}
		if c.PayUrl != nil {
			p.PayUrl = *c.PayUrl
		}
		if c.SuccessUrl != nil {
			p.SuccessUrl = *c.SuccessUrl
		}
		if c.OutOrderId != nil {
			p.OutOrderId = *c.OutOrderId
		}
		if c.InvoiceUrl != nil {
			p.InvoiceUrl = *c.InvoiceUrl
		}
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
			"payment.updated",
			p.ID,
			"payment.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			p.OrganizationID,
			p.User,
			"payment.updated",
			p.ID,
			"payment.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeletePayment(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		p, err := tx.Payments().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.Payments().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			p.OrganizationID,
			p.User,
			"payment.deleted",
			id,
			"payment.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// InvoicePayment updates the invoice information for a payment.
func (s Service) InvoicePayment(ctx context.Context, organizationID string, cmd InvoicePaymentCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		p, err := tx.Payments().GetByIDForOrganization(ctx, cmd.ID, organizationID)
		if err != nil {
			return err
		}
		if cmd.Revision > 0 && p.Revision != cmd.Revision {
			return apperrors.NewConflict("payment revision mismatch")
		}
		p.InvoiceType = cmd.InvoiceType
		p.InvoiceTitle = cmd.InvoiceTitle
		p.InvoiceTaxId = cmd.InvoiceTaxId
		p.InvoiceRemark = cmd.InvoiceRemark
		p.PersonName = cmd.PersonName
		p.PersonIdCard = cmd.PersonIdCard
		p.PersonEmail = cmd.PersonEmail
		p.PersonPhone = cmd.PersonPhone
		p.UpdatedAt = s.Clock.Now()
		p.Revision++
		return tx.Payments().Update(ctx, p)
	})
}

// GetUserPayments lists payments for a specific user.
//
// The caller identity is passed explicitly by the transport layer (app must not
// import transport). A caller may always read its own payments; reading another
// user's payments requires a platform administrator scope that covers the
// tenant. Without such a scope the request fails closed with 403 instead of
// returning another user's data (same-tenant horizontal privilege escalation).
func (s Service) GetUserPayments(ctx context.Context, orgID, user, actorUserID string, page, pageSize int) ([]PaymentListItem, int64, error) {
	if err := authorizeCrossUserRead(ctx, orgID, user, actorUserID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.UOW.Payments().List(ctx, domain.ListQuery{
		Page: page, PageSize: pageSize, OrganizationID: orgID, User: user,
	})
	if err != nil {
		return nil, 0, err
	}
	r := make([]PaymentListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}

// authorizeCrossUserRead enforces ownership on user-scoped read endpoints.
//
// Self reads always pass. Cross-user reads require a platform administrator
// scope covering the trusted tenant; that classification is resolved by
// ScopeAuthorizerMiddleware and reused here (no new permission point). Any
// unresolvable identity or tenant denies (fail-closed), so a denied caller
// never reaches the repository.
func authorizeCrossUserRead(ctx context.Context, organizationID, targetUserID, actorUserID string) error {
	actor := strings.TrimSpace(actorUserID)
	if actor == "" {
		return apperrors.NewForbidden("caller identity is required to read user-scoped commerce data")
	}
	if actor == targetUserID {
		return nil
	}
	// Mode==System is only reachable from trusted internal callers (Worker/Cron
	// entry points wrap ctx with TenantFilterSystem). Since OPEN-API-42 C1 an
	// authorized explicit cross-tenant request injects TenantFilterCross plus
	// the admin scope classification, so platform-admin reads are honoured by
	// the scope.Covers check below instead of a filter-mode shortcut.
	if tenantctx.TenantScopeFromContext(ctx).Mode == tenantctx.TenantFilterSystem {
		return nil
	}
	tenantID, err := uuid.Parse(strings.TrimSpace(organizationID))
	if err != nil {
		return apperrors.NewForbidden("cross-user access requires a resolvable tenant")
	}
	scope, ok := scopeauthorizer.AdminScopeFromContext(ctx)
	if ok && scope.Covers(tenantID) {
		return nil
	}
	return apperrors.NewForbidden("cross-user access requires a platform administrator scope covering the tenant")
}

// GetPaymentResult returns a user-facing payment result summary.
func (s Service) GetPaymentResult(ctx context.Context, organizationID, id string) (*PaymentResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	p, err := s.UOW.Payments().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	return &PaymentResult{
		ID:           p.ID,
		State:        p.State,
		Price:        p.Price,
		Currency:     p.Currency,
		ProductName:  p.ProductName,
		ProviderName: p.Provider,
		PayUrl:       p.PayUrl,
		InvoiceUrl:   p.InvoiceUrl,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}, nil
}
