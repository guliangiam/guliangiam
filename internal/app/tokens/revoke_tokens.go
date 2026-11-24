package tokens

import (
	"context"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) RevokeTokens(ctx context.Context, command RevokeTokensCommand) (*RevokeTokensResult, error) {
	// 可信租户由 transport 的 trustedOrgID 解析并覆盖 body 的 organizationId；
	// body 仍必须至少包含一个过滤条件（organizationId/userId/applicationId），
	// 避免空过滤退化为「撤销本租户全部 token」。
	if err := validateRevokeTokensCommand(command); err != nil {
		return nil, err
	}

	var count int64
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		c, err := tx.Tokens().RevokeByQuery(ctx, domain.RevokeQuery{
			OrganizationID: command.OrganizationID,
			UserID:         command.UserID,
			ApplicationID:  command.ApplicationID,
		}, s.Clock.Now())
		if err != nil {
			return err
		}
		count = c

		return s.embedAuditEvent(
			tx,
			ctx,
			command.OrganizationID,
			// BE-D-13：actor 是操作者（ctx 回落），过滤主体只进 payload。
			"",
			"tokens.revoked.batch",
			"token",
			command.ApplicationID,
			"token",
			"tokens.revoked.batch",
			map[string]any{
				auditPayloadKeyOrgID: command.OrganizationID,
				"userId":             command.UserID,
				"appId":              command.ApplicationID,
				"revokedCount":       count,
			},
		)
	})
	if err != nil {
		slog.ErrorContext(ctx, "tokens.RevokeTokens failed",
			"organization_id", command.OrganizationID,
			"user_id", command.UserID,
			"application_id", command.ApplicationID,
			"error", err,
		)
		return nil, err
	}

	slog.InfoContext(ctx, "tokens.RevokeTokens ok",
		"organization_id", command.OrganizationID,
		"user_id", command.UserID,
		"application_id", command.ApplicationID,
		"count", count,
	)
	return &RevokeTokensResult{RevokedCount: count}, nil
}

func validateRevokeTokensCommand(command RevokeTokensCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" && strings.TrimSpace(command.UserID) == "" && strings.TrimSpace(command.ApplicationID) == "" {
		return apperrors.NewInvalidArgument("at least one filter is required to revoke tokens")
	}

	return nil
}
