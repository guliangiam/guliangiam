# ADR-025: IGA 与 JIT 访问生命周期（Campaign / 审批 / 到期自动回收 / SoD）

- 状态：[Current]/[Target] — IGA 服务、SoD 校验、到期回收 Worker 与 API 已落地；Campaign 逾期处置演练与证据导出校验和为 Target（出口条件：演练留痕 + 校验和独立验证）
- 日期：2026-09-14

## 背景

商业对标要求访问治理闭环：访问评审 Campaign（IGA-001/007）、审批矩阵（IGA-002）、JIT/JEA 时间窗授权（IGA-003）、到期自动回收（IGA-004）、SoD 冲突（IGA-005）、审计员只读视角与证据导出（IGA-007）。IGA 已落地服务与存储：`internal/domain/iga/`、`internal/app/iga/`（含 `sod_checker.go`、`worker.go`、`service.go` 的 `ExpireAccesses` 批量回收）、`internal/infra/{postgres,mysql}/iga/`、HTTP 路由 `internal/transport/http/api/v1/iga/`；`access_campaigns` / `access_campaign_items` / `sod_rules` 与 JIT 生命周期表均随声明式 Schema 落地。剩余证据是逾期处置/回收演练与审计员证据包验收（见验证证据）。回收类任务须遵守 §7.3 清理任务约束（幂等、分批）。

## 决策

1. **Campaign 驱动的访问评审**：`access_campaigns`（scope 模板、评审人、due_at、auto_action 逾期处置）+ `access_campaign_items`（逐项 approve/revoke/delegate）承载；Campaign 结果与 `records` 审计联动；审计员视角（IGA-007）只读可复核 Campaign 结果、授权链路与 SoD 豁免清单。
2. **JIT/JEA 时间窗授权**：授权（角色绑定/临时角色/访客账号/API 凭据）可带生效窗口；JIT 令牌/会话中身份声明可区分常态与临时权限（IGA-003 验收 3）；超范围申请被拒（最小权限）。
3. **到期自动回收器**（IGA-004）：统一扫描全部时间限定授权，回收动作（角色移除/凭据吊销/账号禁用）幂等、分批、失败重试并告警；回收清单可导出对账；实现为常驻 Worker 而非请求路径内联（与 ADR-008 Worker 语义一致）。
4. **SoD 双时机校验**：`sod_rules`（`rule_kind`=role_pair/permission_pair/custom，`exemption_policy`）在申请时与角色变更时双时机校验，**含 SCIM 出站/同步器写入路径**（自动化通道不得绕过 SoD）；豁免走双人审批且限期。
5. 审批流（IGA-002 矩阵）复用平台既有审批/通知设施，不另建独立审批引擎。

## 理由与后果

到期回收解决「存量不该有的权限」，Campaign 解决「应有权限的持续证明」——二者缺一即不构成可对外承诺的访问治理能力。SoD 双时机覆盖自动化写入路径，是 SCIM 出站（ADR-030）与同步器并存时的必要防线。代价：新增三表与一个常驻回收 Worker；审批与豁免引入管理成本；JIT 窗口语义要求令牌签发链路（6.9）携带可区分声明。

## 替代方案

- **全人工年度审计（表格/工单盘点）**：无持续证明能力、无到期自动回收、证据不可机读（IGA-007 证据包要求清单+校验和）；放弃。
- **外购 IGA SaaS**：个人与授权数据出境违反驻留约束（PRIV-010），权限判定出现外购引擎与 Casbin 双真值源，且成本随席位数放大；放弃。
- **只做到期回收、不做 Campaign**：回收只是「删存量」，无法回答「谁批准了什么、为何还在」的持续证明问题，商业对标验收（IGA-001/007）不达标；分期落地可（回收先行），但方向上不裁剪 Campaign。

## 验证证据

- Current：`grep -rniE 'access_campaign|sod_rule|identity_action' internal/ --include='*.go'` 有命中（行数以命令输出为准）——`internal/domain/iga/`、`internal/app/iga/`、`internal/infra/{postgres,mysql}/iga/` 与 `internal/transport/http/api/v1/iga/` 均已实现；`user_role_bindings` 仍存在，JIT/到期语义由 IGA 生命周期表承担。
- Target 出口条件：①到期授权在承诺时延内失效且重复扫描无重复副作用（IGA-004 验收 1/2，有集成测试）；②SoD 双时机校验覆盖 SCIM/同步器路径的负向用例；③Campaign 逾期处置（auto_action）演练留痕；④审计员证据包校验和可独立验证（IGA-007 验收 3，与 ADR-031 哈希链衔接）。待取证项：回收扫描周期与分批参数（参数化）、审批矩阵定义稿。

## 复审条件

出口条件满足后转 `[Current]`；若审批设施引入外部系统或回收 Worker 语义偏离 Outbox/Worker（ADR-008）约定，须复审并更新本 ADR。
