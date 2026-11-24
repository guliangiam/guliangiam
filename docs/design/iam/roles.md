# 角色域详细设计

> 设计锚点：[DESIGN:iam/roles]

> 目标态规范「角色域详细设计」；写作约束见 [docs/README.md](../../README.md)。

---

## 定位

角色是权限的命名集合，通过用户-角色绑定实现 RBAC（基于角色的访问控制）。与 ReBAC（ADR-024）共存，求值分工见《architecture/security-model.md》§RBAC/ABAC/ReBAC 三分工。

## 角色属性

| 属性 | 说明 |
|------|------|
| id | UUIDv7 |
| organization_id | 租户边界 |
| name | 角色名（组织内唯一） |
| description | 说明 |
| permissions | 关联的权限标识集合 |

## 用户-角色绑定

绑定表 `user_role_bindings`（`user_id` + `role_id`，联合唯一）。可批量导入（JSON/Excel）。

## 角色与 ReBAC 的分工

| 模型 | 适用场景 | 存储 |
|------|---------|------|
| RBAC（角色） | 预定义权限模板（管理员、普通用户等） | Casbin policy 行 |
| ABAC | 属性动态决策 | Casbin matcher |
| ReBAC | 资源关系（owner/member/editor） | `relation_tuples` 表（OpenFGA 兼容） |

**求值分工**：端点级权限由 Casbin 承载（`route.Definition.Permission`）；资源关系授权由 ReBAC `relation_tuples` 承载（OpenFGA 兼容 API，路由见 `internal/transport/http/api/v1/rebac/routes.go`）；ABAC 作为 Casbin matcher 的属性条件。三者按《architecture/layers.md》§5 的决策顺序组合，ReBAC 不替代 Casbin。

## 角色编辑前端

角色管理前端由 `web/src/pages/roles/` 承载（RolesPage 列表、RoleCreatePage 创建、RoleDetailPage 详情与权限编辑，Controller/View 分离见 `useRoleDetailPageController.ts`）；角色 × 权限的勾选编辑在角色详情页内完成，组件结构以该目录为事实源，文档不复制 Props 清单。

## API 事实源

角色域 API 端点清单以 `internal/transport/http/api/v1/roles/routes.go` 为准。
