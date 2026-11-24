# ReBAC 设计（关系元组授权）

> 设计锚点：[DESIGN:rebac/README]
> 决策：[ADR-024](../../adr/ADR-024-rebac-relation-tuple-authz.md)；与 Casbin 的求值分工见 [permissions.md](../iam/permissions.md)。

## 1. 定位

ReBAC 承载**对象实例级**细粒度授权：授权事实是「主体-关系-对象」元组，而不是角色-资源-动作三元组。与 Casbin 的分工固定为：Casbin 判定域/资源/动作的粗粒度权限，ReBAC 判定对象实例级关系；两者串联，任一不可用时按 fail-close 拒绝。对象级授权不替代 RLS，元组读写仍受租户隔离约束。

## 2. 数据模型

| 表 | 作用 | 隔离 |
|---|---|---|
| `relation_namespaces` | 命名空间与关系/主体类型定义 | 租户表（RLS） |
| `relation_tuples` | 关系元组（对象-关系-主体） | 租户表（RLS） |

列清单与约束以 `contracts/schema/30_authz.yaml` 为唯一事实源（复现：`grep -n 'relation_namespaces\|relation_tuples' contracts/schema/30_authz.yaml`）；写入使用 `revision` 乐观锁，唯一约束由声明层给出。元组写入与读模型失效在同一事务内完成。

## 3. 求值流程

1. **入参**：`check(subject, relation, object)`；未注册的命名空间、关系或主体类型一律 400 `invalid_argument` 并写审计。
2. **求值**：直接关系命中 → 命名空间配置的继承关系展开 → 组/主体集展开 → 返回决策；求值路径可经 Why 接口回溯，输出「哪条关系导致放行」。
3. **一致性**：写操作返回 `zookie`，读请求可携带 `zookie` 要求写后读一致性；撤销元组后，依赖该关系的决策在承诺时延内失效。
4. **批量面**：`batch check` / `expand` / `list-objects` 复用同一求值器，禁止另写一套判定逻辑。

落点：`internal/domain/rebac/evaluator.go`（求值）、`internal/app/rebac/service.go`（编排、事务、审计）。

## 4. API 与页面

- API 家族：`/api/v1/rebac/namespaces`（命名空间与关系定义）、`/api/v1/rebac/namespaces/{id}`、`/api/v1/rebac/tuples/{id}`（元组写入/删除）。权限点与归属面以 `internal/transport/http/api/v1/rebac/routes.go` 与 `openapi/openapi.yaml` 为准。
- 页面：`/rebac` 列表工作台（菜单 `authz` 分组），路由与页面文件以 `docs/design/frontend/registry.md` 为准。
- 端点口径：需求旅程使用实际 OpenAPI 路径 `/api/v1/rebac/namespaces/{namespaceId}/tuples` 写入元组；`/api/v1/rebac/tuples/{id}` 仅用于删除。
- 求值前校验：`check`/`expand`/`batch check` 校验命名空间存在、relation 属于 `relation_defs.roles|permissions`、subject 类型（当声明 `subjectTypes` 时）属于白名单；未登记一律 400 `invalid_argument`。运行时证据见 `internal/app/rebac/rebac_runtime_pg_test.go`。

## 5. 审计

元组写入、命名空间变更与求值拒绝均写 `records`（同事务），事件命名空间 `rebac.*`；事件名唯一事实源为 `contracts/registries/events.yaml` 的 `rebac` 命名空间。

## 6. 验收锚点

- `AC-REBAC-001`：check 正确性 + 未注册命名空间/关系/主体类型拒绝（400 `invalid_argument`）+ P99 < 50ms（L2 压测口径）。
- `AC-REBAC-002`：Zookie 写后读一致性、对象共享撤销、审计留痕。

## 7. 边界与未决

- 不做全量图遍历；继承深度与批量上限由命名空间配置给出，超限拒绝而非降级。
- 对象级授权不得绕过 RLS；跨租户元组一律视为不存在（404 防枚举）。
