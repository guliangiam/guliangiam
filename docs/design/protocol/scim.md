# SCIM 2.0 完整设计

> 设计锚点：[DESIGN:protocol/scim]

> 目标态规范「SCIM 2.0 完整设计」；写作约束见 [docs/README.md](../../README.md)。

---

## 端点与鉴权

### 端点清单（`internal/transport/http/api/v1/scim/routes.go`）

| 资源 | 方法与路径 | 实际鉴权 |
|------|-----------|---------|
| User | GET/POST `/scim/v2/Users`；GET/PUT/PATCH/DELETE `/scim/v2/Users/{id}` | BearerTokenAuth |
| Group | GET/POST `/scim/v2/Groups`；GET/PUT/PATCH/DELETE `/scim/v2/Groups/{id}` | 同上 |
| Discovery | GET `/scim/v2/ServiceProviderConfig`、`/scim/v2/ResourceTypes`、`/scim/v2/Schemas` | 同上 |

### BearerTokenAuth 语义

- `Authorization: Bearer <token>` 强制（不接受 Basic）
- 组织上下文必填（query `organizationId` 或 `X-Organization-Id`）
- token 校验对照该组织应用的 client secret
- **不产生用户会话、不进入主 authz 中间件链**——SCIM 面是纯 client-credential 协议面
- routes.go 中的 `Permission` 字段不参与运行时鉴权决策

## RFC 7643 Schema 映射

### User 资源映射

| SCIM 属性 | 映射目标 | 说明 |
|----------|---------|------|
| `id` | `users.id` | 域 UUID 直接回显 |
| `userName` | `users.username` | 必填；创建前查重（409 冲突） |
| `externalId` | `users.external_id` | 透传存储 |
| `name.givenName/familyName` | `user_profiles.first_name/last_name` | 单层映射 |
| `displayName` | `user_profiles.display_name` | 直映射 |
| `emails[]` | `user_profiles.email` | **取 `primary=true` 优先、其次 `type=work`**；其余扩展存储 |
| `phoneNumbers[]` | `user_profiles.phone` | 同上 |
| `active` | `!users.is_forbidden` | 语义对齐 |
| `groups`（只读） | `user_group_memberships`（读路径派生） | 只读，不支持写入 |
| `meta` | `ScimMeta{resourceType,created,lastModified,version,location}` | `version` = `"W/\"<revision>\""` |
| 其余核心属性 | — | JSON 解码宽容丢弃 |

### Group 资源映射

| SCIM 属性 | 映射目标 |
|----------|---------|
| `displayName` | `groups.display_name` |
| `members[].value` | `user_group_memberships`（读路径 JOIN） |

### 自定义扩展 Schema

命名空间：`urn:ietf:params:scim:schemas:extension:guliangiam:2.0:User`

- 属性白名单由 `scim_schemas.attributes`（JSONB）租户级配置
- 属性名进 filter/sortBy 前必须命中白名单
- 扩展属性落地于 `user_profiles` JSONB 扩展列，索引按需 GIN

## RFC 7644 协议语义

### filter（13 操作符 + 白名单解析）

| # | 输入形态 | 协议要求 |
|---|------|--------|
| 1 | 不可识别字符 | 不得静默跳过；返回 400 `scimType=invalidFilter` |
| 2 | 未闭合引号 | 不得静默取剩余串；返回 400 `invalidFilter` |
| 3 | 白名单外属性路径 | 统一返回 `invalidFilter` |
| 4 | Groups 列表 filter | 按 `displayName/members.value` 白名单翻译，不得整体忽略 |

### attributes / excludedAttributes / sortBy / sortOrder

sortBy 白名单 = User 白名单字段 ∪ `meta.created/meta.lastModified`，默认 `id`；sortOrder 仅 `ascending/descending`。attributes/excludedAttributes 按属性路径裁剪响应为规划态（运行期不裁剪，返回全部属性）。

**运行期语义**：传输层 ListUsers/ListGroups 透传 `sortBy/sortOrder`；`internal/app/scim/sort.go` `resolveScimSort` 做白名单与方向解析——属性路径大小写不敏感（RFC 7643 §2.1，`userName` 命中 `username`），`meta.created/meta.lastModified` 别名映射 `createdAt/updatedAt`；sortBy 缺省默认 `id` 升序，白名单外路径 400 `invalidValue`，sortOrder 非 `ascending/descending` 400。仓储按手册 4.8.1 白名单生成 ORDER BY（users 双方言 `userSortSpec` 含 `id`；groups 双方言 `groupSortSpec`，无排序参数保留 `created_at DESC` 历史默认，尾随 `id` tiebreaker）。ServiceProviderConfig 宣告 `sort.supported=true`（`internal/app/scim/discovery.go`），与「宣告约束」一致（`OPEN-API-87` 已关闭）。

### 分页：startIndex / count / ListResponse

运行期语义（`internal/app/scim/sort.go` `normalizeScimCount`，Users/Groups 共用）：`startIndex < 1` 归一为 1；`count < 1` 归一为 100（与 discovery `filter.maxResults=100` 一致）；`count > 100` 返回 `scimType=tooMany` 的 400（RFC 7644 §3.4.2.4，错误 Meta 经 `route.WriteSCIMErrorWithType` 透传），不再静默改写。目标态：`startIndex` 深翻页（>10000）走游标方案，登记于开放项 `OPEN-API-89`。

### CRUD 与 PATCH

- PUT 全量替换（未提供字段重置为空值，与 RFC 7644 一致）
- PATCH 支持 `add/replace/remove` 三 op；path 白名单约束；非法 path 返回 `scimType=invalidPath`
- DELETE `active=false` 语义：SCIM 面 DELETE 是「停用」（置 `users.is_forbidden=true`），不是物理删除；停用在同一事务内置态、revision++、写 records/outbox 审计，并吊销该用户会话与令牌（`internal/app/scim/service_user.go`、`audit_outbox.go`）

### Bulk（目标态）

> **规划态章节**：运行期无 `/scim/v2/Bulk` 路由（复现：`grep -n 'Bulk' internal/transport/http/api/v1/scim/routes.go`），ServiceProviderConfig 宣告 `bulk.supported=false / maxOperations=0 / maxPayloadSize=0`（`internal/app/scim/discovery.go`）。

目标态：`POST /scim/v2/Bulk`，`maxOperations=100`、`maxPayloadSize=1MB`；`failOnErrors` 按 RFC 7644 §3.7 非原子模式。

### ETag / If-Match（目标态）

`meta.version` 回显 `"W/\"<revision>\""`（运行期已回显，见 §User 资源映射 `meta` 行）。`If-Match` 强校验（412）为目标态：运行期 ServiceProviderConfig 宣告 `etag.supported=false`（`internal/app/scim/discovery.go`），PUT/PATCH/DELETE 不消费 `If-Match` 头。

### 错误体与 scimType

| scimType | 语义 | HTTP |
|----------|------|------|
| `invalidFilter` | 过滤表达式非法 | 400 |
| `tooMany` | count 超上限（>100，见 §分页） | 400 |
| `uniqueness` | 唯一约束冲突 | 409 |
| `invalidValue` | 属性值非法 | 400 |
| `invalidPath` | PATCH path 非法 | 400 |
| `noTarget` | path 目标不存在 | 400 |

**detail 泄露治理**：4xx 透传业务 detail，5xx 统一 detail="An internal error occurred" + correlation id。

### ServiceProviderConfig 宣告约束

ServiceProviderConfig 的每个宣告项必须与实际可执行能力一致：`sort` 仅在 `sortBy/sortOrder` 解析生效后宣告 `true`；`changePassword` 固定 `false`（密码管理走管理面 API）；`authenticationSchemes` 仅宣告 Bearer。宣告与能力的偏差必须登记 `contracts/registries/open-items.yaml` 后收敛，历史变更见 `CHANGELOG.md`。

## 入站同步作业

外部 IdP/HR 系统向 GuliangIAM 推送全量/增量同步：

1. **作业模型**：每次同步 = 计划 → 执行 → 报告；游标持久化（增量窗口允许重叠，靠 upsert 幂等）
2. **幂等 upsert 仲裁**：`userName`/`externalId` 双键冲突时 `externalId` 优先
3. **失败退避**：上游源拉取模式的 429/503 指数退避
4. **报告**：created/updated/deactivated/skipped/failed 计数，入审计

## 出站 SCIM Client（PROV-010/011）

GuliangIAM 作为 SCIM Client 向下游推送身份与组变更：

- **连接器框架**：连接器配置 CRUD；事件订阅按需勾选
- **投递通道**：复用《design/infrastructure/outbox.md》统一 Outbox/Worker，**不新建投递栈**
- **调和**：定期/手动拉取下游 User/Group 列表与本地 diff，产出漂移报告；引擎与下游分页拉取见（`ReconcileUsers`/`ReconcileGroups` + `ListRemoteUsers`/`ListRemoteGroups`/`FetchAndReconcile`），报告落库见（`scim_reconciliation_reports` + PostgreSQL 仓储），并提供 `cmd/scim-reconcile` 手动执行：读取 expected-state JSON（users/groups）→ 拉取下游 → `FetchAndReconcile` → 写入报告，漂移退出码 1、失败退出码 2；`-list` 分页查询持久化报告；管理面 `GET /api/v1/reconciliation/scim-reports` 按会话组织分页读取；审计事件、修复动作与定时调度按 [ADR-030](../../adr/ADR-030-scim-outbound-jml-provisioning.md) 的目标态义务执行，开放项见 `OPEN-API-43`
- **冲突解决**：下游 409 → 记死信 + 映射冲突原因；下游 404 → 按策略重建或跳过
- **JML 生命周期**：joiner（入站创建即推送）/ mover（部门变更）/ leaver（停用 → 下游 `active=false`）

## 覆盖矩阵

| # | 维度 | 锚点 |
|---|------|------|
| 1 | Schema 映射（User/Group/扩展） | 本文件 §Schema 映射 |
| 2 | filter 操作符与白名单解析 | §filter |
| 3 | 排序与分页 | §分页 |
| 4 | Bulk | §Bulk |
| 5 | ETag/If-Match | §ETag |
| 6 | 错误体与 scimType | §错误体 |
| 7 | Content-Type `application/scim+json` | handler 全端点统一 |
| 8 | 去激活语义（DELETE=disable） | §CRUD 与 PATCH |
| 9 | 入站同步作业 | §入站同步 |
| 10 | 出站 SCIM Client | §出站 |
