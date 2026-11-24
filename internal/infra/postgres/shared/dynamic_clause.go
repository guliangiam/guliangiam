// Package shared holds infra-layer helpers shared across postgres repositories.
//
// AppendParamCondition 是所有 `extra += fmt.Sprintf("AND col = $N", len(args))`
// 模式的统一出口。guard 会校验 col 不包含注入关键词——未来维护者如果
// 把 DTO 字段直接塞进来，guard 会在 log 模式下记日志、panic 模式下直接炸掉。
package shared

import (
	"fmt"

	"github.com/guliangiam/guliangiam/internal/platform/sqlguard"
)

// AppendParamCondition 安全地追加一个 `AND col = $N` 条件到 extra。
// col 必须是调用方传进来的编译期常量（不是字符串拼接）。
//
// 用法:
//
//	extra, args = shared.AppendParamCondition(extra, "status", string(*q.Status), args)
//	// extra 变成 " AND status = $5"，args 变成 [..., "active"]
//
// 返回值: (新 extra, 新 args)
// 列名校验失败时降级（extra 和 args 不变）——不会 panic，符合现有
// extra 模式的"条件不满足就不追加"语义。
func AppendParamCondition(extra, col string, val any, args []any) (clauseOut string, argsOut []any) {
	if err := sqlguard.ValidColumnIdentifier(col); err != nil {
		return extra, args // 降级，行为同"条件为 nil"
	}
	clause := fmt.Sprintf(" AND %s = $%d", col, len(args)+1)
	if err := sqlguard.ValidAndClause(clause); err != nil {
		return extra, args // guard 失败，不追加
	}
	args = append(args, val)
	return extra + clause, args
}
