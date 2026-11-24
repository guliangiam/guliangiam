// Package authorization_audit 审计哈希链 domain 接口 + 实体。
//
// 对应 V5 authorization_audit_records + V6 authorization_chain_state 两张表。
// 所有操作走 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）。
//
// 哈希链设计（Phase C-Step 4）：
//
//	BEGIN tx
//	  SELECT last_hash FROM authorization_chain_state WHERE id = 1 FOR UPDATE  — 锁单行
//	  prev_hash = 上一条 hash（链头为 NULL）
//	  hash = sha256(prev_hash|created_at|actor_id|actor_type|...|reason)     — 固定字段顺序
//	  INSERT INTO authorization_audit_records(..., prev_hash, hash)
//	  UPDATE authorization_chain_state SET last_hash = hash WHERE id = 1
//	COMMIT
package authorization_audit

import (
	"time"

	"github.com/google/uuid"
)

// AuditRecord 审计记录实体（对应 authorization_audit_records 表）。
type AuditRecord struct {
	ID           uuid.UUID  `json:"id"`
	ActorID      uuid.UUID  `json:"actorId"`
	ActorType    string     `json:"actorType"`
	SourceTenant *uuid.UUID `json:"sourceTenant"`
	TargetTenant *uuid.UUID `json:"targetTenant"`

	ResourceType string `json:"resourceType"`
	Action       string `json:"action"`
	RiskLevel    string `json:"riskLevel"`

	ScopeID  *uuid.UUID `json:"scopeId"`
	TicketID *uuid.UUID `json:"ticketId"`

	Decision string  `json:"decision"`
	Reason   *string `json:"reason"`

	// 哈希链字段 — 由仓储层负责写入（prev_hash 从 chain_state 读取）
	PrevHash *string `json:"prevHash"`
	Hash     string  `json:"hash"`

	TraceID   *string   `json:"traceId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ChainRecord 审计链查询结果（用于 validate chain 验证完整性）。
type ChainRecord struct {
	ActorID      uuid.UUID  `json:"actorId"`
	ActorType    string     `json:"actorType"`
	SourceTenant *uuid.UUID `json:"sourceTenant"`
	TargetTenant *uuid.UUID `json:"targetTenant"`
	ResourceType string     `json:"resourceType"`
	Action       string     `json:"action"`
	RiskLevel    string     `json:"riskLevel"`
	Decision     string     `json:"decision"`
	Reason       *string    `json:"reason"`

	PrevHash  *string   `json:"prevHash"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"createdAt"`
}
