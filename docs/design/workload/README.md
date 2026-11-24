# Workload Identity 设计（工作负载身份）

> 设计锚点：[DESIGN:workload/README]
> 决策：[ADR-026](../../adr/ADR-026-workload-identity-federation.md)；主体模型与联邦信任见 [security-model §4](../../architecture/security-model.md)。

## 1. 定位

Service Account 是**一等主体**：可参与认证与授权、可被绑定角色/组，但与人类用户在审计身份上可区分。工作负载经 OIDC 联邦信任换取短期凭据，不得以静态长期密钥作为默认路径。

## 2. 数据模型

| 表 | 作用 | 隔离 |
|---|---|---|
| `identity_pools` | 身份池（信任边界与提供方） | 租户表（RLS） |
| `federation_trusts` | OIDC 联邦信任（issuer/audience/claim 条件） | 租户表（RLS） |
| `service_accounts` | Service Account 主体与生命周期状态 | 租户表（RLS） |

列与约束以 `contracts/schema/85_workload.yaml` 为唯一事实源（复现：`grep -n 'identity_pools\|federation_trusts\|service_accounts' contracts/schema/85_workload.yaml`）。SA 的凭据不落明文：短期令牌由签发路径生成，长期密钥只存哈希或引用。

## 3. 流程

1. **建池**：创建身份池并绑定信任提供方；信任条件（issuer/audience/subject claim）为必填。
2. **联邦**：外部工作负载提交 OIDC Token → 校验签名/issuer/audience/claim → 命中信任 → 签发短期凭据（TTL 与 scope 由信任声明限定）。
3. **使用**：SA 以短期凭据调用 API；SA 调用交互式登录端点一律 403 `forbidden`。
4. **生命周期**：创建/轮换/禁用/删除均带预检；删除前预检关联凭据与授权绑定，存在关联时拒绝并给出原因。

落点：`internal/domain/workload/subject.go`（主体与信任端口）、`internal/app/workload/service.go`（编排、审计）。

## 4. API 与页面

- API 家族：`/api/v1/identity-pools`、`/api/v1/identity-pools/{id}` 等（以 `routes.go` 与 `openapi/openapi.yaml` 为准）；SA 生命周期端点以同一 OpenAPI 声明为准。
- 页面：工作负载工作台（路由见 `docs/design/frontend/registry.md`；页面范围见 [workbenches.md](../frontend/workbenches.md)）。
- 端点口径：需求旅程使用实际 OpenAPI 路径 `POST /api/v1/identity-pools` 创建身份池、`POST /api/v1/identity-pools/{poolId}/service-accounts` 创建 Service Account；HTTP 路由用例见 `internal/transport/http/api/v1/identitypools/chi_test.go`。

## 5. 审计

池/信任/SA 的创建、轮换、禁用、删除与联邦签发失败均写 `records`（同事务），事件命名空间 `service_account.*` / `wli.*`；事件名唯一事实源为 `contracts/registries/events.yaml` 的 `service_account` / `wli` 命名空间。

## 6. 验收锚点

- `AC-WLI-001`：SA 一等主体、生命周期预检与审计、与人类用户可区分、不可登录 UI。
- `AC-WLI-002`：OIDC 联邦信任与短期凭据、错误 issuer/audience/claim 的拒绝路径。

## 7. 边界与未决

- 不引入第二套密钥管理：长期密钥的托管以 ADR-009 的 KMS/Vault 端口为前置。
- 联邦信任条件解析必须 fail-close：任一条件无法判定即拒绝。
