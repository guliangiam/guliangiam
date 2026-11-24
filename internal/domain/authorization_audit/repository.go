package authorization_audit

import "context"

// AuditRepository 审计哈希链读写。
//
// 关键设计：Append 内部自管事务（BEGIN → SELECT FOR UPDATE → 计算 hash → INSERT → UPDATE → COMMIT）。
// 哈希链完整性要求 prevHash（从 chain_state 读）和 hash（sha256(prevHash|createdAt|…)）必须
// 在同一个事务里产生 — 所以 hash 计算也放在 repository 里。
//
// 不走 UoW 事务：审计写入从 ScopeAuthorizer.Authorize 调用，不在业务事务内。
// 所有方法自动注入 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）。
type AuditRepository interface {
	// Append 完成一条审计记录的哈希链写入。
	//
	// 内部事务：
	//  1. BEGIN + SELECT last_hash FROM chain_state WHERE id=1 FOR UPDATE（锁单行）
	//  2. hash = sha256(prev_hash|created_at|actor_id|actor_type|...|reason) — 固定字段顺序管道符分隔
	//  3. INSERT INTO audit_records(..., prev_hash, hash, created_at)
	//  4. UPDATE chain_state SET last_hash = hash WHERE id=1
	//  5. COMMIT
	//
	// createdAt 由应用层传入（UTC Truncate time.Microsecond — 避免 DB DEFAULT 生成/验证不一致）。
	Append(ctx context.Context, rec *AuditRecord, createdAt string) error

	// ListRecords 按 created_at DESC 拉审计链记录（用于 validate chain 测试/验证）。
	ListRecords(ctx context.Context, limit int) ([]ChainRecord, error)
}
