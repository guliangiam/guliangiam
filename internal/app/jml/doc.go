// Package jml 实现 ADR-030 的 JML（Joiner / Mover / Leaver）入站编排器。
//
// 语义（IAM 生命周期标准模型）：
//
//   - Joiner：新用户入社 → 初始角色绑定 → IGA campaign 自动创建 → 出站 SCIM Provisioning
//     入口：Orchestrator.OnUserCreated
//   - Mover：用户角色变更 → SoD 双时机复检 → 出站 role.member.added/removed
//     入口：Orchestrator.OnRoleBindingChanged
//   - Leaver：用户离职/停用 → 吊销会话/令牌 → 清理 role binding → 出站 deactivated
//     入口：Orchestrator.OnUserDeactivated
//
// 与 ADR-025 的关系：
//
//	ADR-025（IGA 访问评审）规定 SoD 规则必须在两个时机被检查：
//	  1. Joiner 初始角色绑定时（本包 OnUserCreated）
//	  2. Mover 角色变更时（本包 OnRoleBindingChanged）
//	两个入口都调用 IGARoleService.CheckSoD，返回冲突时立即回滚/拒绝。
//
// 与 ADR-008 / ADR-030 的关系：
//
//	ADR-008 定义 Outbox 事务模式；ADR-030 定义出站 SCIM Provisioning。
//	本包所有出站事件写入 domain/outbox 仓储，事件类型常量取自
//	internal/domain/connectors.Evt*（与出站 handler 同源）。
//	Outbox Headers 只设置 organization_id，不指定 connector_id —— 让
//	Outbox Worker 遍历该租户下所有 enabled connector，实现零侵入多连接器。
//
// 零侵入架构：
//
//	Orchestrator 是独立包，不修改 app/scim/service.go 或任何 domain 层代码。
//	所有副作用依赖注入接口（deps.go），测试时全部 Stub。
//	事务由调用方（SCIM sync 层）管理，编排器只暴露 context 参数。
//
// 半安全策略：
//
//	Leaver 每一步失败都停止（会话 → 令牌 → binding 清理），避免不一致。
//	Joiner/Mover 的 outbox 写入失败不回滚已落库的 binding——绑定已生效，
//	outbox 有自身重试机制，回滚反而会造成用户"已被创建但无法登录"的不一致。
//
// 最小切片（本包职责边界）：
//
//	✅ SoD 检查 + 角色绑定编排 + Outbox 事件写入
//	❌ 不实现 role binding CRUD 底层存储（依赖 RoleBindingRepository）
//	❌ 不实现 IGA campaign 完整逻辑（依赖 IGARoleService）
//	❌ 不实现 outbox worker（由 platform/outbox 承担）
//	❌ 不直接触达任何 infra 层（全靠依赖注入）
package jml
