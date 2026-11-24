package users

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// JML (ADR-030) 精细化角色绑定能力：domain users.Repository 只有全量
// ReplaceRoleBindings，Joiner 逐条创建 / Leaver 清理需要单条语义。
// 这些方法不进入 domain 接口，由 app/jml 适配器做能力断言（MySQL 镜像
// 未实现，OPEN-API-37）。

func (r Repository) CreateRoleBinding(ctx context.Context, binding *domain.UserRoleBinding) error {
	_, err := r.db.Exec(ctx, queryInsertUserRoleBindingIgnore, binding.UserID, binding.RoleID)
	return err
}

func (r Repository) DeleteRoleBinding(ctx context.Context, userID, roleID string) error {
	_, err := r.db.Exec(ctx, queryDeleteSingleUserRoleBinding, userID, roleID)
	return err
}

func (r Repository) DeleteRoleBindingsByUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, queryDeleteUserRoleBindings, userID)
	return err
}

func (r Repository) ListRoleBindingsByUser(ctx context.Context, userID string) ([]domain.UserRoleBinding, error) {
	rows, err := r.db.Query(ctx, queryListRoleBindingsByUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]domain.UserRoleBinding, 0)
	for rows.Next() {
		var uid, roleID string
		if err := rows.Scan(&uid, &roleID); err != nil {
			return nil, err
		}
		bindings = append(bindings, domain.UserRoleBinding{UserID: uid, RoleID: roleID})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return bindings, nil
}
