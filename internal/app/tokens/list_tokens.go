package tokens

import (
	"context"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
)

type ListTokensQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	UserID         string
	ApplicationID  string
	ActiveOnly     *bool
	Keyword        string
}

func (s *Service) ListTokens(ctx context.Context, query ListTokensQuery) ([]TokenListItem, int64, error) {
	tokens, total, err := s.UOW.Tokens().List(ctx, domain.ListQuery(query))
	if err != nil {
		slog.ErrorContext(ctx, "tokens.ListTokens failed",
			"organization_id", query.OrganizationID,
			"user_id", query.UserID,
			"application_id", query.ApplicationID,
			"error", err,
		)
		return nil, 0, err
	}

	items := make([]TokenListItem, 0, len(tokens))
	for _, token := range tokens {
		items = append(items, toListItem(token))
	}

	return items, total, nil
}
