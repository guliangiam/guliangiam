package casbin

import (
	"context"
	"fmt"
	"strings"

	"github.com/casbin/casbin/v3/model"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// LoadPolicy 加载非 JIT 的 p/g 规则到内存模型。
func (s Store) LoadPolicy(m model.Model) error {
	rows, err := s.db.QueryContext(context.Background(), qLoadPolicy, s.enforcerID)
	if err != nil {
		return apperrors.WrapInternalf(err, "query casbin rules")
	}
	defer rows.Close()

	for rows.Next() {
		var ptype, v0, v1, v2, v3, v4, v5 string
		if err := rows.Scan(&ptype, &v0, &v1, &v2, &v3, &v4, &v5); err != nil {
			return apperrors.WrapInternalf(err, "scan casbin rule")
		}
		sec := "p"
		rule := nonEmpty(v0, v1, v2, v3, v4, v5)
		if ptype != "" && ptype[0] == 'g' {
			sec = "g"
			// g 行按列位取 v0..v5（保留中间空列），不能先 nonEmpty 压缩：
			// 压缩会把 v2 的 domain 顶到 v1 的 role 位，生成错误的角色链接。
			rule = padGroupingRule(m, ptype, []string{v0, v1, v2, v3, v4, v5})
		}
		if err := m.AddPolicy(sec, ptype, rule); err != nil {
			return apperrors.WrapInternalf(err, "add policy to casbin model")
		}
	}
	return rows.Err()
}

// SavePolicy 在事务内清空该 enforcer 的规则并全量写入模型内容。
func (s Store) SavePolicy(m model.Model) error {
	beginner, ok := s.db.(mysqlshared.Beginner)
	if !ok {
		return apperrors.NewInternal("casbin policy store requires a transaction-capable handle")
	}
	ctx := context.Background()
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.WrapInternal(err, "begin casbin save transaction")
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, qDeleteAllRules, s.enforcerID); err != nil {
		return apperrors.WrapInternal(err, "clear casbin rules")
	}
	for _, assertionMap := range m {
		for ptype, ast := range assertionMap {
			for _, policy := range ast.Policy {
				if err := s.insertRuleTx(ctx, tx, ptype, policy); err != nil {
					return err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return apperrors.WrapInternal(err, "commit casbin save transaction")
	}
	return nil
}

// AddPolicy 追加一条规则。
func (s Store) AddPolicy(_, ptype string, rule []string) error {
	id, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "generate casbin rule id")
	}
	_, err = s.db.ExecContext(context.Background(), qInsertRule,
		id, s.enforcerID, ptype,
		field(rule, 0), field(rule, 1), field(rule, 2),
		field(rule, 3), field(rule, 4), field(rule, 5))
	if err != nil {
		return apperrors.WrapInternal(err, "insert casbin rule")
	}
	return nil
}

// RemovePolicy 删除一条精确规则（空字段视为通配，与 PG 版一致）。
func (s Store) RemovePolicy(_, ptype string, rule []string) error {
	v0, v1, v2, v3, v4, v5 := field(rule, 0), field(rule, 1), field(rule, 2), field(rule, 3), field(rule, 4), field(rule, 5)
	_, err := s.db.ExecContext(context.Background(), qDeleteRule,
		s.enforcerID, ptype,
		v0, v1,
		v2, v2, v3, v3, v4, v4, v5, v5)
	if err != nil {
		return apperrors.WrapInternal(err, "delete casbin rule")
	}
	return nil
}

// RemoveFilteredPolicy 按字段前缀过滤删除。
func (s Store) RemoveFilteredPolicy(_, ptype string, fieldIndex int, fieldValues ...string) error {
	query := qDeleteFilteredBase
	args := []any{s.enforcerID, ptype}
	fields := [6]string{}
	for i := range fields {
		if i >= fieldIndex && i < fieldIndex+len(fieldValues) {
			fields[i] = fieldValues[i-fieldIndex]
		}
	}
	for i, v := range fields {
		if v == "" {
			continue
		}
		query += fmt.Sprintf(" AND v%d = ?", i)
		args = append(args, v)
	}
	_, err := s.db.ExecContext(context.Background(), query, args...)
	if err != nil {
		return apperrors.WrapInternal(err, "delete filtered casbin rules")
	}
	return nil
}

func (s Store) insertRuleTx(ctx context.Context, tx mysqlshared.DBTX, ptype string, rule []string) error {
	id, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "generate casbin rule id")
	}
	if _, err := tx.ExecContext(ctx, qInsertRule,
		id, s.enforcerID, ptype,
		field(rule, 0), field(rule, 1), field(rule, 2),
		field(rule, 3), field(rule, 4), field(rule, 5)); err != nil {
		return apperrors.WrapInternal(err, "insert casbin rule")
	}
	return nil
}

func field(rule []string, idx int) string {
	if idx < len(rule) {
		return rule[idx]
	}
	return ""
}

func nonEmpty(vals ...string) []string {
	result := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

// 与 PG 版一致：casbin v3.11 的 buildRoleLinks 拒绝短于角色定义元数的 g 行，
// 历史无 domain 的 2 元 g 行补空 domain 后再入模型，避免整库 LoadPolicy 失败。
// vals 按列位排列（含中间空列），按模型元数补空/截断，不得压缩空列。
func padGroupingRule(m model.Model, ptype string, vals []string) []string {
	ast, ok := m["g"][ptype]
	if !ok {
		return nonEmpty(vals...)
	}
	arity := strings.Count(ast.Value, "_")
	rule := make([]string, arity)
	for i := range rule {
		if i < len(vals) {
			rule[i] = vals[i]
		}
	}
	return rule
}
