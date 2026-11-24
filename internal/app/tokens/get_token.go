package tokens

import (
	"context"
	"log/slog"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type GetTokenQuery struct {
	ID string
	// OrganizationID is the trusted tenant scope resolved by transport.
	OrganizationID string
}

func (s *Service) GetToken(ctx context.Context, query GetTokenQuery) (*TokenDetail, error) {
	if err := requireOrganizationID(query.OrganizationID); err != nil {
		return nil, err
	}
	// id 来自 URL path；by-ID 读取在仓储层绑定 organization_id，跨租户与
	// 不存在同为 not found。
	if strings.TrimSpace(query.ID) == "" {
		return nil, apperrors.NewInvalidArgument("token id is required")
	}

	token, err := s.UOW.Tokens().GetByIDForOrganization(ctx, query.ID, query.OrganizationID)
	if err != nil {
		slog.ErrorContext(ctx, "tokens.GetToken failed",
			"row_id", query.ID,
			"organization_id", query.OrganizationID,
			"error", err,
		)
		return nil, err
	}

	return toDetail(token), nil
}
