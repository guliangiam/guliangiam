# Identity Actions 设计（租户可编程扩展点）

> 设计锚点：[DESIGN:actions/README]
> 决策：[ADR-029](../../adr/ADR-029-identity-actions-sandbox.md)；运行时与注册中心见 [agent.md](../infrastructure/agent.md)。

## 1. 定位

Identity Actions 是挂在登录/注册/令牌等管线触发点上的**租户可编程扩展**。执行必须在沙箱内完成（WASM 优先），具备资源限额、密钥注入与出站白名单；扩展失败不得阻断认证主链路，除非动作显式声明为阻断型。

## 2. 数据模型

| 表 | 作用 | 隔离 |
|---|---|---|
| `identity_actions` | 动作定义（触发点、启用态、失败策略） | 租户表（RLS） |
| `action_versions` | 动作版本与制品引用 | 租户表（RLS） |
| `execution_logs` | 执行记录（结果、耗时、资源用量） | 租户表（RLS） |

列与约束以 `contracts/schema/80_governance.yaml` 为唯一事实源（复现：`grep -n 'identity_actions\|action_versions\|execution_logs' contracts/schema/80_governance.yaml`）。制品本身存对象存储，表内只存引用与校验和。

## 3. 流程

1. **注册**：动作声明触发点（登录前/登录后/注册、令牌签发前后等，目录以设计为准）与失败策略（阻断/旁路）。
2. **版本**：每次发布生成不可变版本与校验和；回滚即切换活跃版本。
3. **执行**：沙箱加载 → 注入声明式密钥（只注入绑定到该动作的密钥引用）→ 执行 → 记录结果与资源用量。
4. **限额**：CPU/内存/时长超限即终止，按失败策略处置；沙箱逃逸与资源耗尽必须被阻断。
5. **出站**：只允许白名单目标；协议限定 https（开发环境例外需显式开关并留日志），默认禁重定向，DNS 结果固定 IP 连接。

落点：`internal/app/actions/dispatcher.go`（调度与限额）、`internal/app/actions/proxy.go`（出站管控）、`internal/domain/actions/`（定义与端口）。

## 4. API 与页面

- API 家族：`/api/v1/actions`、`/api/v1/actions/{id}`、`/api/v1/actions/{id}/status`（以 `routes.go` 与 `openapi/openapi.yaml` 为准）。
- 页面：`/actions` 列表工作台（菜单 `authz` 分组），路由见 `docs/design/frontend/registry.md`。
- 端点口径：需求旅程使用实际 OpenAPI 路径 `/api/v1/actions`（创建/版本/状态/执行）；HTTP 路由用例见 `internal/transport/http/api/v1/actions/chi_test.go`。

## 5. 审计

动作注册、版本发布/回滚、执行失败与阻断、限额触发均写 `records`（同事务），事件命名空间 `actions.*`；执行日志只写脱敏字段（不含密钥与请求体原文）。事件名唯一事实源为 `contracts/registries/events.yaml` 的 `actions` 命名空间。

## 6. 验收锚点

- `AC-EXT-001`：触发点挂载、资源限额生效、超时/超内存按失败策略处理、逃逸与耗尽被阻断、版本可升级且升级有回归。
- `AC-EXT-002`：密钥注入最小化、出站白名单与 SSRF 防护、不得写入边界外路径。

## 7. 边界与未决

- 沙箱内不得直接访问数据库；需要数据时经声明式输入注入。
- 阻断型动作的失败必须 fail-close（拒绝登录），旁路型动作失败只告警。
