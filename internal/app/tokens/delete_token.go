package tokens

import (
	"context"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type DeleteTokenCommand struct {
	ID string
	// OrganizationID is the trusted tenant scope resolved by transport.
	OrganizationID string
}

func (s *Service) DeleteToken(ctx context.Context, command DeleteTokenCommand) error {
	// by-ID 读取在仓储层绑定 organization_id；跨租户与不存在同为 not found，
	// 且在 WithinTransaction 之前就拒绝，写路径不会触碰他租户行。
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if strings.TrimSpace(command.ID) == "" {
		return apperrors.NewInvalidArgument("token id is required")
	}

	token, err := s.UOW.Tokens().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
	if err != nil {
		slog.ErrorContext(ctx, "tokens.DeleteToken failed",
			"row_id", command.ID,
			"organization_id", command.OrganizationID,
			"error", err,
		)
		return err
	}

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Tokens().Delete(ctx, command.ID); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			token.OrganizationID,
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
		slog.ErrorContext(ctx, "tokens.DeleteToken failed",
			"organization_id", token.OrganizationID,
			"row_id", command.ID,
			"error", err,
		)
		return err
	}

	slog.InfoContext(ctx, "tokens.DeleteToken ok",
		"organization_id", token.OrganizationID,
		"row_id", command.ID,
	)
	return nil
}
