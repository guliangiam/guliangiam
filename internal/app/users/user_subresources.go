package users

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/transactions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// UserTransactionItem is a lightweight DTO for a transaction in the user context.
type UserTransactionItem struct {
	ID              string  `json:"id"`
	TransactionType string  `json:"transactionType"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	Status          string  `json:"status"`
	ProviderType    string  `json:"providerType,omitempty"`
	CreatedAt       string  `json:"createdAt"`
}

// UserConsentItem is a lightweight DTO for a consent in the user context.
type UserConsentItem struct {
	ID              string   `json:"id"`
	ApplicationID   string   `json:"applicationId"`
	ApplicationSlug string   `json:"applicationSlug,omitempty"`
	GrantedScopes   []string `json:"grantedScopes"`
	CreatedAt       string   `json:"createdAt"`
	RevokedAt       string   `json:"revokedAt,omitempty"`
}

// ListUserTransactions returns transactions for a specific user.
func (s *Service) ListUserTransactions(ctx context.Context, organizationID, userID string, page, pageSize int) ([]UserTransactionItem, int64, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, 0, err
	}
	if userID == "" {
		return nil, 0, apperrors.NewInvalidArgument("userId is required")
	}

	var txns []*transactions.Transaction
	var total int64

	err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// The parent user is the tenant anchor; the child list also carries the
		// organization predicate as defence in depth.
		if _, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID); err != nil {
			return err
		}
		var qerr error
		txns, total, qerr = tx.Transactions().List(ctx, transactions.ListQuery{
			Page:           page,
			PageSize:       pageSize,
			OrganizationID: organizationID,
			UserID:         userID,
		})
		return qerr
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]UserTransactionItem, 0, len(txns))
	for _, txn := range txns {
		items = append(items, UserTransactionItem{
			ID:              txn.ID,
			TransactionType: txn.TransactionType,
			Amount:          txn.Amount,
			Currency:        txn.Currency,
			Status:          txn.Status,
			ProviderType:    txn.ProviderType,
			CreatedAt:       txn.CreatedAt.Format(time.RFC3339),
		})
	}

	return items, total, nil
}

// ListUserConsents returns consents for a specific user from the user's ApplicationScopes.
func (s *Service) ListUserConsents(ctx context.Context, organizationID, userID string) ([]UserConsentItem, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}

	// Consents live inside the user aggregate: the scoped read is both the
	// ownership proof and the data source.
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, organizationID)
	if err != nil {
		return nil, err
	}

	items := make([]UserConsentItem, 0, len(user.ApplicationScopes))
	for _, scope := range user.ApplicationScopes {
		items = append(items, UserConsentItem{
			ID:            scope.Application,
			ApplicationID: scope.Application,
			GrantedScopes: scope.Scopes,
			CreatedAt:     scope.GrantedAt,
		})
	}

	return items, nil
}

// RevokeConsentCommand is the command for revoking a user consent.
type RevokeConsentCommand struct {
	OrganizationID string
	UserID         string
	ConsentID      string
}

// RevokeUserConsent removes a consent (application scope) from a user.
// It filters out the consent matching the consentID (which is the application ID)
// and persists the updated application scopes.
func (s *Service) RevokeUserConsent(ctx context.Context, command RevokeConsentCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if command.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if command.ConsentID == "" {
		return apperrors.NewInvalidArgument("consentId is required")
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Consents live inside the user aggregate: the scoped read proves both
		// parent ownership and consent membership before the write.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		// Filter out the consent matching the consentID (application ID).
		updatedScopes := make([]userdomain.ConsentRecord, 0, len(user.ApplicationScopes))
		found := false
		for _, scope := range user.ApplicationScopes {
			if scope.Application == command.ConsentID {
				found = true
				continue
			}
			updatedScopes = append(updatedScopes, scope)
		}

		if !found {
			return apperrors.NewNotFound("consent not found")
		}

		user.ApplicationScopes = updatedScopes
		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, command.OrganizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.updated",
			"User",
			user.ID,
			"user",
			"user.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
