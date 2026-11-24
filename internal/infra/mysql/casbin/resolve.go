package casbin

import (
	"context"
	"database/sql"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ResolveEnforcerID 把组织 ID 解析为该组织启用的 enforcer 记录 ID。
// MySQL 没有 RLS，无需注入租户上下文。
func (s Store) ResolveEnforcerID(ctx context.Context, orgID string) (string, error) {
	if strings.TrimSpace(orgID) == "" {
		return "", apperrors.NewInvalidArgument("organization id is required")
	}
	var enforcerID string
	err := s.db.QueryRowContext(ctx, qResolveEnforcer, orgID).Scan(&enforcerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", apperrors.NewInvalidArgument("organization has no casbin enforcer configured")
		}
		return "", apperrors.WrapInternalf(err, "resolve casbin enforcer")
	}
	return enforcerID, nil
}

// ResolveEnforcerUUID 与 ResolveEnforcerID 语义相同；授权中间件使用该名称。
func (s Store) ResolveEnforcerUUID(ctx context.Context, orgUUID string) (string, error) {
	var enforcerID string
	err := s.db.QueryRowContext(ctx, qResolveEnforcer, orgUUID).Scan(&enforcerID)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "resolve enforcer for organization %s", orgUUID)
	}
	return enforcerID, nil
}
