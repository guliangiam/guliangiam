# ADR-027: 自适应认证与持续访问评估（规则引擎先行 + 风险分 + step-up + CAEP/SSF 下推）

- 状态：[Current]/[Target]（风险规则引擎、风险决策存储/服务与 API 已落地；CAEP/SSF 下推、分级阈值定标与持续评估演练仍为 [Target]）
- 日期：2026-09-14

## 背景

SRS RISK-001~016 要求：风险信号采集（设备指纹/IP 信誉/地理速度/撞库模式）、分级决策、step-up/MFA 联动、持续访问评估、CAEP/SSF 事件下推与哈希链审计保全。现状仅有静态防护面：账号锁定（8.18，`lockout_attempts`）、IP 黑白名单（`ip_rules`）、验证码与进程内限流（8.19）——全部为 `[Current]`；风险引擎已实现：`internal/domain/risk/`（决策模型/端口）、`internal/app/risk/`（服务）、`internal/infra/{postgres,mysql}/risk/`（存储）、HTTP 路由 `internal/transport/http/api/v1/risk/`；风险决策落 `risk_decisions` 并可审计。CAEP/SSF 下推与 step-up（acr/amr）联动仍为 `[Target]`。目标表 `risk_decisions` / `ssf_receivers`（[3.2](../architecture/data-model.md#32-repository-约定)；`risk_decisions` 已随生成式 V1 基线声明，`ssf_receivers` 已随 V5 增量迁移声明（组织 RLS + 唯一命名 + 状态 CHECK））。

## 决策

1. **规则引擎先行**：分级决策由确定性规则 + 阈值实现（RISK-006），规则版本化、可灰度、可回滚、变更有审计；ML 评分仅以可插拔接口预留（输入信号 schema 稳定、输出分值归一），本阶段不交付训练/推理管线（SRS §3.3 修订口径）。
2. **决策可解释可审计**：`risk_decisions` 落 `score` / `level`（low/medium/high/critical）/ `hit_signals`（脱敏，不含信号原文）/ `action` / `rule_version`；四级映射放行 / step-up / MFA 加固 / 拒绝+通知（RISK-005，medium 默认 step-up 走 AUTH-105 acr/amr/auth_time 语义）；阈值租户可配；信号输入缺失按保守默认决策（fail-close 同源，8.8）。
3. **决策点接入**：登录链（6.9）、敏感操作前置（AUTH-105 step-up）、管理面高危 API；风险分级作为 6.4 授权决策顺序的前置环境属性输入（REBAC-008），不作为独立授权引擎。
4. **持续访问评估**（RISK-009）：会话生命周期内周期/事件驱动重估（风险信号、权限变更、设备状态变化触发），重估结果可降级会话（要求 step-up / 吊销）；被踢会话与 refresh family 即时吊销（8.17 族撤销联动）。
5. **CAEP/SSF 事件下推**（RISK-010/012）：以 RFC 8935（推送）/8936（轮询）向 `ssf_receivers` 注册的接收方下发会话/令牌失效类事件；消费上游 RISC/SSF 事件联动本系统吊销；签名、重试与死信走 Outbox/Worker 语义（ADR-008）。
6. **静态防护叠加不替换**：`lockout_attempts`（8.18）、`ip_rules`、验证码/限流（8.19）保持现有语义，风险信号与之叠加。

## 理由与后果

规则先行保证决策**可解释**（RISK-001 验收硬要求：分值与命中信号清单可审计）且冷启动可用；ML 接口预留避免未来推翻管线。持续评估把安全边界从「登录时一次性判定」推进到「会话全程」。代价：信号源（设备指纹哈希、地理库）引入版本化维护成本；SSF 接收方注册与签名密钥管理是新的治理面；吊销传播时延需要独立指标监控。

## 替代方案

- **纯 ML 模型起步**：决策不可解释（无法满足 RISK-001 验收的命中信号清单）、冷启动无标注数据、误报不可定位；放弃本阶段交付，保留可插拔接口。
- **第三方风控 SaaS**：设备/IP/行为信号属个人相关数据，出境违反数据驻留约束（PRIV-010）；规则资产沉淀在供应商侧，阈值调优不可自主；放弃。
- **仅登录时一次性评估、不做持续评估**：凭据被盗后会话持续有效，无法满足 RISK-009 与 CAEP 下推的商业对标项；放弃（持续评估可分期，方向不裁剪）。

## 验证证据

- Current：风险规则引擎/决策落点已实现（`internal/domain/risk/`、`internal/app/risk/`、`internal/infra/{postgres,mysql}/risk/`、`internal/transport/http/api/v1/risk/`）；静态防护（`lockout_attempts` / `ip_rules`）继续叠加。CAEP/SSF 下推：`ssf_receivers` 表（已随生成式 V1 基线声明，结构事实源 `contracts/schema/`）与 PostgreSQL 注册仓储（Create/Get/List/Delete，租户事务内 RLS 上下文）已落地；RFC 8417 SET 声明构造与签名已落地（`internal/platform/ssf.BuildClaims/SignEvent`，`secevent+jwt`，CAEP/RISC 事件类型常量，单测覆盖）；Outbox 投递 Handler 已注册（`ssf_event`：加载 receiver、订阅过滤、禁用跳过、SET 签名 POST、非 2xx 交由 Worker 重试/死信；signer 经 holder 后置注入 JWKS）；事件生产端已落地：`RevokeSession`（单会话）、`RevokeOtherSessions`（其余会话）、用户级 `ForceLogoutUser`（按受影响组织逐个）与组织级 `ForceLogoutOrganization` 在同一事务内写 `ssf_event`（CAEP session-revoked，subject 按场景含 sub/session_id/org_id，reason 区分单会话/其余会话/组织强退），投递 Handler 在无 receiver_id 时向组织内所有 active 订阅方扇出；认证面登出（`auth.Logout`）在撤销成功后 best-effort 写 `ssf_event`（reason=logout；事件队列失败不阻塞登出，已记录告警）；风险决策入队、上游 RISC 消费与 step-up 联动仍为 `[Target]`。
- Target 出口条件：①四级决策路径同输入同输出可复现（RISK-001 验收 1）；②规则版本化灰度回滚演练留痕；③吊销传播时延指标可查（RISK-012 验收 3）；④`ssf_receivers` 签名投递与上游 RISC 事件消费端到端测试通过；⑤`records` 哈希链校验工具可发现单点篡改（RISK-013，落点见 ADR-031）。待取证项：分级阈值与信号源清单定标、地理库版本化更新机制。

## 复审条件

出口条件满足后转 `[Current]`；ML 评分管线启动交付、或风险信号新增出境类来源时，须复审本 ADR 的接口与驻留约束。
