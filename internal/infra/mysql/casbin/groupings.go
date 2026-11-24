package casbin

import (
	"context"
	"strings"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ReplaceUserGroupings 在事务内按 (username, orgSlug) 重写角色继承的 g-规则：
// 先删该用户全部 g 行，再逐条插入，重复行由唯一键的 ON DUPLICATE KEY 兜住。
func (s Store) ReplaceUserGroupings(ctx context.Context, username, orgSlug string, roleNames []string) error {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(orgSlug) == "" {
		return nil
	}
	beginner, ok := s.db.(mysqlshared.Beginner)
	if !ok {
		return apperrors.NewInternal("casbin policy store requires a transaction-capable handle")
	}
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.WrapInternal(err, "begin casbin grouping transaction")
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, qDeleteGroupingsForUser, s.enforcerID, username); err != nil {
		return apperrors.WrapInternal(err, "delete casbin groupings")
	}
	for _, roleName := range roleNames {
		roleName = strings.TrimSpace(roleName)
		if roleName == "" {
			continue
		}
		id, err := platformuuid.V7Generator{}.NewString()
		if err != nil {
			return apperrors.WrapInternal(err, "generate casbin grouping id")
		}
		if _, err := tx.ExecContext(ctx, qInsertGrouping, id, s.enforcerID, username, roleName, orgSlug); err != nil {
			return apperrors.WrapInternal(err, "insert casbin grouping")
		}
	}
	if err := tx.Commit(); err != nil {
		return apperrors.WrapInternal(err, "commit casbin groupings")
	}
	return nil
}
