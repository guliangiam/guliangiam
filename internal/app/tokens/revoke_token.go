package tokens

import (
	"context"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/errors"
)

type RevokeTokenCommand struct {
	ID string
	// OrganizationID is the trusted tenant scope resolved by transport.
	OrganizationID string
}

func (s *Service) RevokeToken(ctx context.Context, command RevokeTokenCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if strings.TrimSpace(command.ID) == "" {
		return errors.NewInvalidArgument("token id is required")
	}

	token, err := s.UOW.Tokens().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
	if err != nil {
		slog.ErrorContext(ctx, "tokens.RevokeToken failed",
			"row_id", command.ID,
			"organization_id", command.OrganizationID,
			"error", err,
		)
		return err
	}

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Tokens().Revoke(ctx, command.ID, s.Clock.Now()); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			token.OrganizationID,
			// BE-D-13：actor 是操作者（ctx 回落），token 主体只进 payload。
			"",
			"token.revoked",
			"token",
			command.ID,
			"token",
			"token.revoked",
			map[string]any{"tokenId": command.ID, auditPayloadKeyOrgID: token.OrganizationID, "userId": token.UserID},
		)
	})
	if err != nil {
		slog.ErrorContext(ctx, "tokens.RevokeToken failed",
			"organization_id", token.OrganizationID,
			"user_id", token.UserID,
			"row_id", command.ID,
			"error", err,
		)
		return err
	}

	slog.InfoContext(ctx, "tokens.RevokeToken ok",
		"organization_id", token.OrganizationID,
		"user_id", token.UserID,
		"row_id", command.ID,
	)
	return nil
}
