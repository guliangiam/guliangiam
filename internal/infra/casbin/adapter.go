package casbin

import (
	"context"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/casbin/casbin/v3/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const qEnforceJITPolicy = `
	SELECT EXISTS (
		SELECT 1 FROM "guliangiam".casbin_rules
		WHERE enforcer_id = $1 AND ptype = 'p'
		  AND v0 = $2 AND v1 = $3
		  AND (v2 = '*' OR v2 = $4)
		  AND (v3 = '*' OR v3 = $5)
		  AND CASE WHEN v5 LIKE '%"source":"iga_jit"%' THEN v4::timestamptz END > $6
		  AND CASE WHEN v5 LIKE '%"source":"iga_jit"%' THEN (v5::jsonb ->> 'startsAt')::timestamptz END <= $6
		  AND CASE WHEN v5 LIKE '%"source":"iga_jit"%' THEN v5::jsonb ->> 'lifecycleId' END = $7
	)`

const qLoadPolicy = `
	SELECT ptype, v0, v1, v2, v3, v4, v5
	FROM "guliangiam".casbin_rules
	WHERE enforcer_id = $1 AND (v5 IS NULL OR v5 NOT LIKE '%"source":"iga_jit"%')
	ORDER BY id`

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type policyQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Adapter implements casbin persist.Adapter using pgx, storing Casbin policies
// in the guliangiam.casbin_rules table scoped by enforcer_id.
type Adapter struct {
	pool       *pgxpool.Pool
	enforcerID string
}

// NewAdapter creates a new pgx-based Casbin adapter.
func NewAdapter(pool *pgxpool.Pool) Adapter {
	return Adapter{pool: pool}
}

// WithEnforcerID returns a copy of the adapter scoped to the given enforcer ID.
func (a Adapter) WithEnforcerID(enforcerID string) Adapter {
	return Adapter{pool: a.pool, enforcerID: enforcerID}
}

// Scoped 实现 PolicyStore：返回同一实现的 enforcer 作用域副本。
func (a Adapter) Scoped(enforcerID string) PolicyStore {
	return a.WithEnforcerID(enforcerID)
}

// ResolveEnforcerID 把组织 ID 解析成该组织启用的 enforcer 记录 ID。
//
// casbin_rules.enforcer_id 上有指向 enforcers(id) 的外键，且种子数据就是按
// enforcer 记录 ID 存的；直接用 orgID 写策略会触发 SQLSTATE 23503，
// 因此 org 维度的策略同步必须先做这一步解析。
func (a Adapter) ResolveEnforcerID(ctx context.Context, orgID string) (string, error) {
	if strings.TrimSpace(orgID) == "" {
		return "", apperrors.NewInvalidArgument("organization id is required")
	}
	// enforcers 表启用了 RLS，裸 pool 没有租户上下文时一行都读不到，
	// 因此在事务内显式注入 app.current_organization_id 再查询。
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_organization_id', $1, true)`, orgID); err != nil {
		return "", err
	}

	var enforcerID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM "guliangiam".enforcers
		WHERE organization_id = $1::uuid AND is_enabled = true
		ORDER BY created_at
		LIMIT 1`, orgID).Scan(&enforcerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", apperrors.NewInvalidArgument("organization has no casbin enforcer configured")
		}
		return "", err
	}
	return enforcerID, nil
}

// ResolveEnforcerUUID 供授权中间件使用：此时还没有任何租户 GUC，RLS 会 fail-closed，
// 因此在同一连接上先注入 super_admin 角色再查询（Exec 与 QueryRow 必须同连接）。
func (a Adapter) ResolveEnforcerUUID(ctx context.Context, orgUUID string) (string, error) {
	if a.pool == nil {
		return "", apperrors.NewUnavailable("casbin policy store is not configured")
	}
	conn, err := a.pool.Acquire(ctx)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "acquire connection for enforcer resolve")
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', false)`); err != nil {
		return "", apperrors.WrapInternalf(err, "set super_admin role for enforcer resolve")
	}

	var enforcerUUID string
	err = conn.QueryRow(ctx, `
		SELECT id FROM "guliangiam".enforcers
		WHERE organization_id = $1::uuid AND is_enabled = true
		ORDER BY created_at ASC
		LIMIT 1
	`, orgUUID).Scan(&enforcerUUID)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "resolve enforcer for organization %s", orgUUID)
	}
	return enforcerUUID, nil
}

// EnforceJIT evaluates time-windowed policies at request time. JIT rows are
// deliberately excluded from the cached Casbin model so expiration remains a
// hard database gate even when the reclaim worker is stopped.
func (a Adapter) EnforceJIT(ctx context.Context, enforcerID, lifecycleID string, rvals ...interface{}) (bool, error) {
	if a.pool == nil {
		return false, nil
	}
	return enforceJIT(ctx, a.pool, enforcerID, lifecycleID, time.Now().UTC(), rvals...)
}

func enforceJIT(ctx context.Context, db queryRower, enforcerID, lifecycleID string, at time.Time, rvals ...interface{}) (bool, error) {
	if strings.TrimSpace(lifecycleID) == "" || len(rvals) < 4 {
		return false, nil
	}
	vals := [4]string{}
	for i := range vals {
		value, ok := rvals[i].(string)
		if !ok {
			return false, nil
		}
		vals[i] = value
	}
	var allowed bool
	err := db.QueryRow(ctx, qEnforceJITPolicy,
		enforcerID, vals[0], vals[1], vals[2], vals[3], at, lifecycleID).Scan(&allowed)
	if err != nil {
		return false, apperrors.WrapInternalf(err, "enforce JIT policy")
	}
	return allowed, nil
}

func (a Adapter) LoadPolicy(m model.Model) error {
	return loadPolicy(context.Background(), a.pool, a.enforcerID, m)
}

func loadPolicy(ctx context.Context, db policyQuerier, enforcerID string, m model.Model) error {
	rows, err := db.Query(ctx, qLoadPolicy, enforcerID)
	if err != nil {
		return apperrors.WrapInternalf(err, "query casbin rules")
	}
	defer rows.Close()

	for rows.Next() {
		var ptype, v0, v1, v2, v3, v4 string
		var v5 pgtype.Text
		if err := rows.Scan(&ptype, &v0, &v1, &v2, &v3, &v4, &v5); err != nil {
			return apperrors.WrapInternalf(err, "scan casbin rule")
		}
		rule := nonEmpty(v0, v1, v2, v3, v4, v5.String)
		sec := "p"
		if strings.HasPrefix(ptype, "g") {
			sec = "g"
			// g 行按列位取 v0..v5（保留中间空列），不能先 nonEmpty 压缩：
			// 压缩会把 v2 的 domain 顶到 v1 的 role 位，生成错误的角色链接。
			rule = padGroupingRule(m, ptype, []string{v0, v1, v2, v3, v4, v5.String})
		}
		if err := m.AddPolicy(sec, ptype, rule); err != nil {
			return apperrors.WrapInternalf(err, "add policy to casbin model")
		}
	}
	return rows.Err()
}

func (a Adapter) SavePolicy(m model.Model) error {
	ctx := context.Background()
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Clear existing rules for this enforcer.
	if _, err := tx.Exec(ctx, `DELETE FROM "guliangiam".casbin_rules WHERE enforcer_id = $1`, a.enforcerID); err != nil {
		return err
	}

	// Insert all policies from the model.
	for _, assertionMap := range m {
		for ptype, ast := range assertionMap {
			for _, policy := range ast.Policy {
				if err := a.insertRule(ctx, tx, ptype, policy); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit(ctx)
}

func (a Adapter) AddPolicy(sec, ptype string, rule []string) error {
	ctx := context.Background()
	_, err := a.pool.Exec(ctx, `
		INSERT INTO "guliangiam".casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8)`,
		a.enforcerID, ptype,
		field(rule, 0), field(rule, 1), field(rule, 2),
		field(rule, 3), field(rule, 4), field(rule, 5))
	return err
}

func (a Adapter) RemovePolicy(sec, ptype string, rule []string) error {
	ctx := context.Background()
	_, err := a.pool.Exec(ctx, `
		DELETE FROM "guliangiam".casbin_rules
		WHERE enforcer_id = $1 AND ptype = $2 AND v0 = $3 AND v1 = $4
		  AND ($5 = '' OR v2 = $5) AND ($6 = '' OR v3 = $6)
		  AND ($7 = '' OR v4 = $7) AND ($8 = '' OR v5 = $8)`,
		a.enforcerID, ptype,
		field(rule, 0), field(rule, 1), field(rule, 2),
		field(rule, 3), field(rule, 4), field(rule, 5))
	return err
}

func (a Adapter) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	ctx := context.Background()
	query := `DELETE FROM "guliangiam".casbin_rules WHERE enforcer_id = $1 AND ptype = $2`
	args := []interface{}{a.enforcerID, ptype}
	argIdx := 3

	fields := [6]string{}
	for i := range fields {
		if i >= fieldIndex && i < fieldIndex+len(fieldValues) {
			fields[i] = fieldValues[i-fieldIndex]
		}
	}

	for i, v := range fields {
		if v != "" {
			query += fmt.Sprintf(" AND v%d = $%d", i, argIdx)
			args = append(args, v)
			argIdx++
		}
	}

	_, err := a.pool.Exec(ctx, query, args...)
	return err
}

func (a Adapter) insertRule(ctx context.Context, db pgx.Tx, ptype string, rule []string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO "guliangiam".casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8)`,
		a.enforcerID, ptype,
		field(rule, 0), field(rule, 1), field(rule, 2),
		field(rule, 3), field(rule, 4), field(rule, 5))
	return err
}

// ReplaceUserGroupings rewrites the role-inheritance rows for one user.
//
// The request-time model is `g = _, _, _` with `g(r.sub, p.sub, r.org)`, so a
// row is (username, role name, organization slug). Replacing instead of
// appending makes revocations effective immediately and keeps the table
// idempotent when role bindings are saved repeatedly.
func (a Adapter) ReplaceUserGroupings(ctx context.Context, username, orgSlug string, roleNames []string) error {
	if a.pool == nil || strings.TrimSpace(username) == "" || strings.TrimSpace(orgSlug) == "" {
		return nil
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return apperrors.WrapInternal(err, "begin casbin grouping transaction")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		DELETE FROM "guliangiam".casbin_rules
		WHERE enforcer_id = $1 AND ptype = 'g' AND v0 = $2`, a.enforcerID, username); err != nil {
		return apperrors.WrapInternal(err, "delete casbin groupings")
	}

	for _, roleName := range roleNames {
		roleName = strings.TrimSpace(roleName)
		if roleName == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO "guliangiam".casbin_rules (id, enforcer_id, ptype, v0, v1, v2)
			VALUES (gen_random_uuid(), $1, 'g', $2, $3, $4)
			ON CONFLICT (enforcer_id, ptype, v0, v1, v2, v3, v4, v5) DO NOTHING`,
			a.enforcerID, username, roleName, orgSlug); err != nil {
			return apperrors.WrapInternal(err, "insert casbin grouping")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return apperrors.WrapInternal(err, "commit casbin groupings")
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

// casbin v3.11 起 buildRoleLinks 对元素数少于角色定义元数的 g 行直接报错，
// 而历史 SyncRole 经 2 元 org 模型写入过无 domain 的 g 行；同一张表又被
// 3 元请求模型（g = _, _, _）加载，短行会让整库 LoadPolicy 失败 → 全租户 503。
// vals 必须是按列位排列的 v0..v5（含中间空列）：按模型元数补空/截断，
// 空 domain 链接对 g(sub, role, org) 匹配器不可见，与 v3.10 裁决语义一致；
// 2 元模型加载时截断回 2 元，超出元数的列（如误写入 v3 的 domain）被丢弃。
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
