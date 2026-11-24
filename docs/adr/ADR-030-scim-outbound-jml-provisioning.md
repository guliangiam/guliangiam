# ADR-030: SCIM 出站与 JML 身份生命周期编排（复用 Outbox 栈）

- 状态：[Current]/[Target]（入站 SCIM 服务端、出站 HTTP 客户端/DeliveryHandler 与 outbox Worker 已落地；调和漂移报告、死信/重放演练与 JML 全链路回归仍为 [Target]）
- 日期：2026-09-14

## 背景

SRS PROV-010/011（§5.5.1）要求 SCIM 出站 Provisioning：连接器框架（配置/凭据/字段映射模板/事件订阅）与调和（reconciliation，期望态 diff + 漂移报告）。现状三件事实：①**入站 SCIM 服务端已有**（AUTH-072，`[Current]`：`internal/app/scim/service.go`、`internal/transport/http/api/v1/scim/`、`internal/infra/postgres/scim/`，`grep -rli scim internal/ --include='*.go'` 25 文件命中）；②**上游拉取同步器已有**（`internal/domain/syncers/syncer.go:19` 定义 `SyncerTypeSCIM`，方向为从上游目录拉取进平台）；③**出站 Client 已实现**：`internal/outbound/scim/client.go`（SCIM v2 HTTP 客户端、限流/重试）与 `handler.go`（DeliveryHandler）；④**Outbox Worker 已落地**：`internal/bootstrap/wire_outbox.go` 装配 `platform/outbox.Worker`，webhook 投递处理器见 `internal/app/webhooks/delivery_outbox.go`（outbox 单轨，ADR-008 已同步复核）。

## 决策

1. **出站投递复用 Outbox/Worker 统一栈**（25.4、INFRA-022）：SCIM 出站事件以 Outbox 领域事件为唯一事件源（8.5 事件清单：user.created/updated/deactivated、group.member.added/removed 等），经 Worker 领约投递，失败指数退避重试、并发与下游速率控制、死信与人工重放；**禁止请求路径内同步推送下游**（ADR-008 已有结论）。
2. **连接器框架**（PROV-010）：连接器配置（base URL、Bearer 凭据**信封加密存储**走 8.7/ADR-015 信封）、字段映射模板 CRUD、事件订阅按需勾选；SSRF 出口管控与 Webhook 同口径（6.6，参数化）。
3. **调和**（PROV-011）：定期/手动拉取下游 User/Group 列表与本地期望态 diff，产出漂移报告（缺失/多余/属性不一致），一键修复或人工逐项处置；报告留审计；下游持续不可用退避并告警。

   **已落地（引擎 + 下游拉取）**：`ReconcileUsers` / `ReconcileGroups` 对 expected/remote `UserRecord`/`GroupRecord` 做确定性 diff（用户：missing/extra/attributeMismatch/stateMismatch；组：missing/extra/memberMismatch，成员按集合排序），externalId 优先、大小写不敏感兜底，带 JSON 标签与单测；`ListRemoteUsers` / `ListRemoteGroups` 按 SCIM ListResponse 分页（startIndex/count/totalResults，边界上限防死循环），`FetchAndReconcile` 串联拉取与双报告，httptest 分页用例覆盖。报告持久化已落地：`scim_reconciliation_reports` 已随生成式 V1 基线声明（结构事实源 `contracts/schema/80_governance.yaml`；组织隔离 + RLS，用户/组报告 JSONB、状态与错误列），PostgreSQL 仓储 `NewReconciliationReportRepository` 提供追加与按组织分页查询；`cmd/scim-reconcile` 提供手动执行入口（expected-state 文件 → 下游分页拉取 → 双报告落库，漂移/失败非零退出）与 `-list` 分页查询已持久化报告；仓储在租户事务内显式设置 RLS GUC（`set_tenant_context`），因此无 HTTP 会话的 CLI 也能安全落库/查询。管理面另提供 `GET /api/v1/reconciliation/scim-reports`（会话组织范围 + 分页）供控制台读取报告。审计事件、一键修复、退避告警与定时调度仍为 [Target]。
4. **JML 编排**（Joiner-Mover-Leaver）：建号/转岗/离职以领域事件统一驱动——Joiner 触发建号与授权申请（IGA-002，ADR-025）、Mover 触发角色变更重估与 SoD 复检（`sod_rules` 双时机校验）、Leaver 触发会话/令牌吊销（8.17）与下游 SCIM 停用投递；JML 与出站事件**同源同语义**（同一 Outbox 事件流），不建第二套事件管线。

## 理由与后果

入站（上游同步、SCIM 服务端）与出站（下游 provisioning）是两个方向，复用面仅在投递设施与事件源：出站天然需要 Outbox 的至少一次投递 + 幂等 + 死信语义，自建第二套重试/队列等于复制 ADR-008。凭据信封加密复用既有 `EncryptedFieldMapper` 模式（8.7 三个仓储已验证的接入方式）。代价：调和任务与投递 Worker 的调度参数（周期/批大小/速率）需要参数化；JML 全链路横跨 SCIM/IGA/会话吊销三域，回归面大。

## 替代方案

- **每应用独立轮询同步器**：时效受轮询周期限制（分钟级事件时效退化为小时级）、每个连接器重复实现重试/幂等/映射/速率控制，运维与缺陷面线性放大；放弃。
- **第三方 provisioning 中间件**：多一跳有状态运维组件，字段映射逻辑仍在本地维护（收益不成比例），且下游用户数据流经第三方存在驻留约束（PRIV-010）风险；放弃。
- **请求路径内同步推送下游**：第三方时延直接拖垮管理面 API（ADR-008 已有同类结论），失败语义与请求事务耦合；放弃。

## 验证证据

- Current：入站 SCIM 服务端（`internal/app/scim/`、`internal/transport/http/api/v1/scim/`、`internal/infra/{postgres,mysql}/scim/`）与上游 syncer（`SyncerTypeSCIM`）已落地；出站 Client/DeliveryHandler 已实现（`internal/outbound/scim/`）；Outbox Worker 已落地（`internal/bootstrap/wire_outbox.go`、`internal/platform/outbox/`、`internal/infra/webhook/`）。
- Target 出口条件：①连接器端到端投递测试（create/update/deactivate 三类事件 + Bearer 凭据密文落库）；②调和漂移报告三类差异正确且修复留痕（PROV-011 验收）；③死信与人工重放演练留痕（INFRA-022 语义）；④JML 全链路回归（joiner→授权申请→mover→SoD 复检→leaver→下游停用+会话吊销）。待取证项：投递 Worker 落地（ADR-008 收敛）后的批大小/速率参数、字段映射模板默认集。

## 复审条件

出口条件满足后转 `[Current]`；若事件源脱离 Outbox 领域事件（如引入外部事件总线直连下游）或新增非 SCIM 出站协议族（LDAP 出站等），须复审本 ADR 并评估是否扩展事件清单。
