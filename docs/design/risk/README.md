# RISK 设计（身份风险与持续访问评估）

> 设计锚点：[DESIGN:risk/README]
> 决策：[ADR-027](../../adr/ADR-027-adaptive-auth-continuous-eval.md)；架构级八环节见 [security-model §3](../../architecture/security-model.md)。

## 1. 定位

风险引擎按规则先行、ML 可插拔的口径输出分级决策（low/medium/high/critical），并把决策映射为会话治理动作（monitor/step_up/revoke）。决策只做**加强**，不得降低既有静态策略强度；全部登录通道（密码/QR/Device Flow/Passkey 等）使用同一决策入口。

## 2. 数据模型

| 表 | 作用 | 隔离 |
|---|---|---|
| `risk_signals` | 风险信号（登录/设备/位置/行为） | 租户表（RLS） |
| `risk_decisions` | 分级决策与申诉 | 租户表（RLS） |
| `session_governance` | 会话治理动作记录 | 租户表（RLS） |

列与约束以 `contracts/schema/80_governance.yaml` 为唯一事实源（复现：`grep -n 'risk_signals\|risk_decisions\|session_governance' contracts/schema/80_governance.yaml`）；租户级策略开关经 `organization_security_policies` 承载。

## 3. 流程

1. **采集**：登录与关键操作产出信号（IP/ASN、设备指纹、失败次数、时间异常）。
2. **评分**：规则集求值 → 分数 → 分级；规则缺失时按默认分值，不因缺信号放行。
3. **决策**：low 放行；medium 触发 step-up（`AC-AUTH-015`）；high 要求完整 MFA；critical 拒绝并同时通知用户与管理员。
4. **治理**：决策落成会话动作（monitor/step_up/revoke），revoke 撤销会话及其派生令牌族。
5. **申诉**：用户可发起申诉，处理结论留痕；误报不影响既有静态策略。

落点：`internal/domain/risk/risk.go`（分级与规则端口）、`internal/app/risk/service.go`（编排、审计、告警）。

## 4. API 与页面

- API 家族：`/api/v1/risk/signals`、`/api/v1/risk/decisions`、`/api/v1/risk/decisions/{id}`、`/api/v1/risk/decisions/{id}/review`、`/api/v1/risk/decisions/{id}/appeal`、`/api/v1/risk/session-governance`、`/api/v1/risk/audit-integrity`；权限点 `risk:read` / `risk:write`（以 `routes.go` 与 `openapi/openapi.yaml` 为准）。
- 页面：`/risk`（列表工作台 + 决策复核），路由见 `docs/design/frontend/registry.md`。

## 5. 审计

信号、决策、治理动作、复核与申诉全部写 `records`（同事务），事件命名空间 `risk.*`；事件名唯一事实源为 `contracts/registries/events.yaml` 的 `risk` 命名空间。

## 6. 验收锚点

- `AC-RISK-001`：medium/high/critical 的分级动作、全通道一致、不降低静态策略、申诉留痕。
- `AC-RISK-002`：会话并发策略与强制下线。
- `AC-RISK-003`：审计完整性校验接口。

## 7. 边界与未决

- 规则求值不依赖外部服务；外部情报源不可用时按「无额外信号」计分，并在决策中标注信号缺失，不得降低既有策略。
- ML 评分器为可插拔接口，接入前规则引擎是唯一路径。
