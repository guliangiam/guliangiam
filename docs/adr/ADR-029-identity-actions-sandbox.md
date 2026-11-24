# ADR-029: Identity Actions 登录管线沙箱（同步扩展点 + WASM 优先隔离 + 出站白名单）

- 状态：[Current]/[Target] — 同步扩展点调度、WASM 沙箱、出站代理与 API 已落地；五触发点全量集成与沙箱逃逸验收为 Target（出口条件：全量集成 + 逃逸验收取证）
- 日期：2026-09-14

## 背景

SRS EXT-001~004 与《架构设计》6.13 要求租户可编程扩展点挂在认证/协议面关键节点：pre-login / pre-token / post-registration / consent / MFA 五类触发点，可返回 continue / deny / modify 指令。Actions 运行时已落地：`internal/domain/actions/` 定义五类触发点（`pre_login`/`pre_token`/`post_registration`/`consent`/`mfa`），`internal/app/actions/` 提供调度器/沙箱/出站代理与 WASM 执行器（`dispatcher.go`、`sandbox.go`、`wasm_executor.go`、`proxy.go`），HTTP 路由在 `internal/transport/http/api/v1/actions/` 注册；既有 webhooks 仍为出站通知，二者定位不同。目标表 `identity_actions` / `identity_action_versions` / `identity_action_executions`（[3.2](../architecture/data-model.md#32-repository-约定)；对应生成式 V1 表为 `identity_actions` / `action_versions` / `execution_logs`，均已声明）。沙箱运行时选型原为未决项，本 ADR 给出决策。

## 决策

1. **同步扩展点**：五类触发点按《架构设计》6.13 表挂载（pre-login 在凭据校验后、pre-token 在协议面令牌端点签发前、consent 写入时、MFA 挑战构造/校验时、post-registration 事务提交后）；**安全类触发点默认 fail-close**（pre-login / pre-token / consent / MFA），非安全类（post-registration）可声明 fail-open 但必须留执行失败审计与告警（EXT-004）。
2. **隔离模型：WASM 优先，受限 JS 解释器为备选形态**：WASM 提供内存安全与确定性资源限额（CPU/内存/执行时长配额，超限按触发点失败策略处置），允许多租户 Action 共用宿主；受限 JS（受沙箱化解释器约束）作为生态兼容备选。**放弃原生插件（动态库）**——内存安全与租户隔离不可保障、宿主崩溃面不可接受。无论何种形态统一约束：无本地文件系统访问、无任意进程/网络原语（6.13 沙箱执行模型）。
3. **出站白名单**：Action 网络调用一律经出站代理，目标白名单（域名/网段）默认拒绝、显式放行，越界阻断并审计（EXT-003）；出站目标/大小/协议/超时受 6.6 SSRF 红线约束；密钥经运行时注入，不落配置明文不进日志（8.9/8.10 覆盖），轮换后新密钥生效。
4. **版本与灰度**：Action 代码/配置版本化于 `identity_actions` / `identity_action_versions`（`canary_percent` / `timeout_policy` / `failure_strategy`），按租户/应用/流量百分比灰度与快速回滚；执行快照（输入摘要/输出指令/耗时/错误）落 `identity_action_executions` 供追溯。
5. **与授权正交**（6.13 架构定位）：Actions 处理流程定制（风控联动/属性增强/外部审批联动），不承担授权判定——授权仍由 Casbin（6.4.1）与 ReBAC（ADR-024）按 6.4.2 顺序决策。
6. **容量联动**：Action 预算计入认证面时延 SLO（6.1 认证面 99.9%）；Action 执行失败不产生计量事件（METER-002 口径以实际业务事实为准）。

## 理由与后果

同步扩展点是 Okta/Auth0 类平台的商业必备（客户风控、属性增强无法经异步 webhook 满足时延要求）；同步性要求把「第三方代码在认证链路上执行」的风险显式化，因此隔离形态、配额、白名单、失败策略四件套必须同时落地。WASM 在内存安全与多租户密度间取得最优平衡。代价：WASM 运行时引入宿主依赖与冷启动开销；受限 JS 备选形态需要独立维护沙箱逃逸测试；同步超时预算挤压认证面 SLO。

## 替代方案

- **原生插件（Go plugin / 动态库）**：与宿主同地址空间，无内存安全边界，崩溃/逃逸即全局事故，且无法按 Action 粒度限额；放弃。
- **独立短进程/容器 per 执行**：隔离最强，但进程冷启动与运维密度使认证链路时延与资源成本不可接受（6.13 候选三选一中保留为高隔离需求演进方向，非首选）；本阶段放弃。
- **HTTP 回调外部服务（webhook 同步化）**：时延与可用性耦合外部系统，fail-close 语义下外部抖动直接放大为登录不可用；放弃。

## 验证证据

- Current：`find internal -type d \( -name 'actions' -o -name 'hooks' \)` 命中 `internal/app/actions`、`internal/infra/postgres/actions`、`internal/infra/mysql/actions`、`internal/domain/actions`、`internal/transport/http/api/v1/actions`；WASM 沙箱（wazero）、超时/内存限额、出站代理与灰度字段均已实现；`identity_actions` / `action_versions` / `execution_logs` 已随生成式 V1 基线声明。
- Target 出口条件：①五类触发点在真实认证/协议流程命中并有集成测试（EXT-001 验收）；②沙箱逃逸与资源耗尽用例被阻断（EXT-002 验收 3，随附逃逸测试方案为本 ADR 出口条件）；③白名单外出站被阻断并留痕（EXT-003 验收 2）；④灰度百分比与回滚演练留痕；⑤超时按 6.13 失败策略表逐触发点执行。待取证项：WASM 运行时选型结论（含逃逸测试方案）、Action 超时预算对认证面 SLO 的影响实测。

## 复审条件

出口条件满足后转 `[Current]`；备选形态（受限 JS）启用、或引入独立进程隔离形态时，须复审隔离模型并更新本 ADR。
