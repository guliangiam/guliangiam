# IGA/PAM 设计（身份治理与特权访问）

> 设计锚点：[DESIGN:iga/README]
> 决策：[ADR-025](../../adr/ADR-025-iga-jit-access-lifecycle.md)；页面范围见 [workbenches.md](../frontend/workbenches.md)。

## 1. 定位

IGA 承载访问活动（campaign）、职责分离（SoD）与 JIT/审批式提权：所有临时权限以「时间窗 + 声明范围 + 审批」为约束，到期由回收器自动回收，回收幂等、可重试、可审计。

## 2. 数据模型

| 表 | 作用 | 隔离 |
|---|---|---|
| `access_campaigns` | 访问复核活动与周期 | 租户表（RLS） |
| `sod_rules` | 职责分离规则（互斥角色/权限） | 租户表（RLS） |
| `access_lifecycle` | 临时授权生命周期（时间窗、状态） | 租户表（RLS） |

列与约束以 `contracts/schema/80_governance.yaml` 为唯一事实源（复现：`grep -n 'access_campaigns\|sod_rules\|access_lifecycle' contracts/schema/80_governance.yaml`）。授权态与角色绑定在**同一事务**内变更：临时角色绑定写 `user_role_bindings` 时带 `expires_at`，回收即删除或置失效。

## 3. 流程

1. **申请**：用户/管理员提交 JIT 申请（开始时间 + 时长 + 声明范围）。
2. **审批**：审批人决策，记录审批人、原因与时间；越权申请 403 `forbidden`。
3. **生效**：窗口内按声明范围放行，临时权限在令牌与审计中可区分；窗口外一律拒绝。
4. **回收**：到期回收器（周期扫描）移除角色/吊销临时凭据；回收幂等，失败重试并告警；提前回收即时生效。
5. **复核**：campaign 到期生成待复核清单，逾期未复核按策略自动回收并留痕。

落点：`internal/app/iga/jit.go`（JIT）、`internal/app/iga/access.go`（campaign/lifecycle）、`internal/domain/iga/`（状态与端口）。

## 4. API 与页面

- API 家族：`/api/v1/iga/access-campaigns`、`/api/v1/iga/access-campaigns/{id}`、`/api/v1/iga/access-campaigns/{id}/status`、`/api/v1/iga/access-lifecycle`；权限点以 `routes.go` 与 `openapi/openapi.yaml` 为准。
- 页面：IGA/PAM 工作台（路由与页面见 `docs/design/frontend/registry.md`；页面范围与验收锚点见 [workbenches.md](../frontend/workbenches.md)）。

## 5. 审计

申请、审批、生效、回收、拒绝与逾期均写 `records`（同事务），事件命名空间 `iga.*`；事件名唯一事实源为 `contracts/registries/events.yaml` 的 `iga` 命名空间。

## 6. 验收锚点

- `AC-IGA-001`：窗口内/外行为、超范围 403、到期自动回收（幂等 + 重试 + 告警）、提前回收即时生效。
- `AC-IGA-002`：审批式提权、特权会话隔离、审批与回收全程可审计。

## 7. 边界与未决

- 回收器不依赖外部调度：以进程内周期任务 + 租约去重实现，多实例下同一记录只被回收一次。
- SoD 规则求值的输入必须来自授权事实源，不在 IGA 侧另建角色快照。
