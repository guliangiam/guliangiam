// preflight.go — MySQL users 仓储与生成的 schema 的启动期一致性探针。
//
// 背景（OPEN-API-37）：MySQL 镜像仓储的 SQL 曾按「users 扁平列」编写，
// 而 contracts/schema 生成的 MySQL DDL 采用与 PostgreSQL 相同的规范化表
// （user_signin_tracking / user_mfa_config / user_financials / user_collections /
// user_custom_attributes）。列不一致在编译期不可见，首个身份请求会得到
// Unknown column 的 500。这里在启动时执行一次真实查询，把运行期 500
// 前移成可控的启动失败。
package users

import (
	"context"
	"errors"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// Preflight 执行基础读查询（空结果集），用于验证仓储 SQL 与当前 schema 兼容。
// 查询本身不带副作用；列缺失时返回驱动错误。
func Preflight(ctx context.Context, db mysqlshared.DBTX) error {
	rows, err := db.QueryContext(ctx, queryGetUserByID, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		// 空结果集正常；这里不消费行内容。
	}
	return rows.Err()
}

// PreflightWrite 在事务内对既有用户的 5 张子表执行 upsert，然后回滚，用于验证
// 写路径 SQL 与生成 DDL 一致。它要求 db 是 *sql.DB（Beginner）；调用方必须提供
// 一个已存在的 user_id（FK 指向 users）。不会留下任何持久化变更。
func PreflightWrite(ctx context.Context, db mysqlshared.DBTX, userID string) error {
	beginner, ok := db.(mysqlshared.Beginner)
	if !ok {
		return errors.New("PreflightWrite requires a transaction-capable handle (*sql.DB)")
	}
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, queryUpsertUserSigninTracking, userID, 0, nil, nil, ""); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertUserMfaConfig, userID, "", []byte("[]"), "", "", "", "", "", false, nil); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertUserFinancials, userID, 0, 0, "USD"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertUserCollections, userID,
		[]byte("[]"), []byte("[]"), []byte("[]"), []byte("[]"), []byte("[]"), []byte("[]"), []byte("[]")); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertUserCustomAttributes, userID,
		"", "", "", "", "", "", "", "", "", ""); err != nil {
		return err
	}
	return nil
}
