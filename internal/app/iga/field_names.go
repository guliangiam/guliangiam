package iga

import domain "github.com/guliangiam/guliangiam/internal/domain/iga"

// 审批人角色标识（与审批链配置一致）。
const (
	approverRoleManager = "manager"
	approverRoleSAP     = "sap"
)

// approvalStages / terminalLifecycleStatuses 是校验闭集的单一源：
// switch 分支与 422 的 `allowed` 元数据都从这里读取。
var approvalStages = []string{approverRoleManager, approverRoleSAP}

var terminalLifecycleStatuses = []string{string(domain.LifecycleRevoked), string(domain.LifecycleExpired)}

// 审计 details 里复用的键名（审计检索与前端展示按同名键取值）。
const (
	keyExpiresAt   = "expiresAt"
	keyLifecycleID = "lifecycleId"
	keyPolicyID    = "policyId"
	keyStartsAt    = "startsAt"
	keyTicketID    = "ticketId"
	keyPrivileged  = "privileged"
)
