# api-surfaces — API 三面 + SurfaceFor + 路由覆盖

> 目标态规范「api-surfaces — API 三面 + SurfaceFor + 路由覆盖」；写作约束见 [docs/README.md](../README.md)。

## 1. 三面定义

在单体内部，对外 HTTP 接口按**契约与消费者**分为三个面。分类依据不是「是否需认证」（管理面与认证面都需要认证），而是消费者、契约来源与限流策略。

| 面 | 路径前缀 | 消费者 | 契约来源 |
|---|---|---|---|
| **管理面** `management` | `/api/v1/*`（排除下方两面） | 控制台 / 管理员 API | 本项目自定 REST |
| **认证面** `authentication` | `/api/v1/auth/*` 中的页面与会话类端点 | 本系统登录页 / SPA | 本项目自定 REST |
| **协议面** `protocol` | **多个前缀家族**（见下；家族清单唯一事实源为 `contracts/registries/surfaces.yaml`） | 外部标准客户端（RP/SP/IdP/SCIM/DCR） | RFC / OASIS 规定 |

协议面的前缀家族（完整清单与判据以 `contracts/registries/surfaces.yaml` 为唯一事实源；本节只写分类依据）：

1. 根级 `/oauth2/*`
2. 根级 `/.well-known/*`
3. 根级 `/cas/{organizationSlug}/{applicationSlug}/*`
4. 根级 `/scim/v2/*`
5. `/api/v1/auth/*` 内的协议端点
6. `/api/v1/saml/*`
7. `/api/v1/ai/faces/*`（AI 网关对外的 OpenAI / Anthropic / Gemini 兼容协议面）

> **关键事实**：协议端点没有统一的顶层前缀，而是分散在多个前缀家族。简单的路径前缀正则无法可靠判定面归属，因此面归属由路由声明显式携带，分类器仅作启动自检。

## 2. SurfaceFor 分类器

**三面分类的唯一事实源**：`contracts/registries/surfaces.yaml` 登记所有声明面归属的路由条目（含 pattern、Surface 枚举、挂载端点族）。`route.Definition.Surface` 是运行时面归属的**唯一来源**（显式声明，禁止推断）。

`SurfaceFor(pattern)` 函数**仅用于启动自检断言**（断言 `route.Definition.Surface` 的声明值 == `SurfaceFor(pattern)` 的推断值），**不参与运行时判定**；推断值只用于辅助发现「忘记声明 Surface」或「声明与 pattern 明显矛盾」的情况。`internal/bootstrap/surface_contract_test.go` 遍历全部路由声明，断言 `Surface` 非空且等于 `SurfaceFor(pattern)`，任何不一致即 CI 阻塞。

## 3. 三面运行时差异

| 面 | 挂载点 | 经过的中间件链 | 面内特殊节点 |
|---|--------|--------------|------------|
| 管理面 | `RegisterManagementRoutes`（`route.WithSurface(SurfaceManagement)` 子组） | A 全局 + B 受保护全链 | 全局组限流；`surface` 指标标签钉为 `management` |
| 认证面 | `auth.RegisterChiRoutes` 的 authentication 子组 | A + B 全链 | 组内 `mw.AuthRateLimit`；注册/密码重置局部限流器叠加；`surface` 钉为 `authentication` |
| 协议面 | `RegisterProtocolRoutes`（根级族）+ `auth/chi_protocol.go`（auth 内协议段） | 两者均走 A + B 全链 | 协议端点限流按端点族独立拆分（`oauth2/token`、`oauth2/introspect`、`oauth2/revoke` 各自限流器，参数见 `contracts/registries/limits.yaml`）；OTLP 局部限流器；`surface` 钉为 `protocol`；错误体走 RFC 6749/6750/7644 写入器 |
| 公开链 | `RegisterPublicRoutes`（`i18n`、`/healthz`、`/uploads/*` 等） | 仅 A 全局链 | `/healthz`、`/metrics` 为基础设施路由，在路由契约门禁中登记豁免 |

**错误格式分离**：`route.WriteError` 按请求上下文的 `surface` 分派——协议面输出 RFC 6749/6750 错误体（SCIM 走 RFC 7644 的 `application/scim+json`），管理面/认证面输出应用错误体。

## 4. 协议面单权威前缀（目标态）

CAS 唯一入口为根级 `/cas/{organizationSlug}/{applicationSlug}/*`（标准 CAS 2.0 URL 形态），SAML 唯一入口为 `/api/v1/saml/*`（元数据/SSO/SLO）。**从零建设无存量消费方，不设兼容别名、不设双前缀过渡**。路由声明与运行时注册双向差集门禁 + OpenAPI 生成保证「声明即实现」。

## 5. OpenAPI 路由覆盖

全部 transport 资源包均以 `routes.go` 定义 `route.Definition` 条目（Method/Pattern/OperationID/Summary/Permission/Surface），同时服务三个消费方：

1. Casbin 策略执行（`route.Definition.Permission` 驱动权限点注册表）
2. OpenAPI 生成（`python3 scripts/gen_openapi.py` 从 `routes.go` 生成 `openapi/openapi.yaml`，生成结果与工作区文件零差异为 CI 阻塞门禁，ADR-012）
3. 路由契约测试（声明集与 chi 运行时注册集双向差集为空，由 `internal/bootstrap/route_contract_test.go` 阻塞校验）

**引用 contracts/registries/surfaces.yaml 作为 API 契约唯一口径**——具体路由清单不在本文书写，复现命令：`python3 scripts/gen_openapi.py && diff openapi/openapi.yaml <(git show HEAD:openapi/openapi.yaml)`。

> **定位说明**：三面的权威表述在本节；[docs/design/](../design/README.md) 的对应章节是同一主题在开发设计视角下的对应章节。

## 6. RESTful 合规性审查（管理面 / 认证面）

本节定义 GuliangIAM 管理面与认证面（协议面遵循各协议规范，不受此约束）的 RESTful 实现约束。**所有管理面路由在 `routes.go` 中声明时必须满足下列规则**；声明集与运行时注册集的一致性由 `internal/bootstrap/route_contract_test.go` 阻塞校验，RESTful 形态约束由设计评审与本节判据把关。

### 6.1 资源命名 = 名词复数

| 规则 | 合法示例 | 禁止示例 |
|------|----------|----------|
| 顶层资源：名词复数、kebab-case | `/api/v1/users`、`/api/v1/organizations`、`/api/v1/applications` | `/api/v1/user`、`/api/v1/createUser`、`/api/v1/userService` |
| 子资源：父资源 + 单资源 ID + 子资源复数 | `/api/v1/organizations/{orgId}/users` | `/api/v1/org/{orgId}/userList` |
| 资源属性嵌入 | `/api/v1/users/{userId}/profile`（profile 为名词） | `/api/v1/users/{userId}/update` |
| 协议面例外 | `/oauth2/token`、`/scim/v2/Users`、`/.well-known/openid-configuration` | — |

### 6.2 HTTP 方法语义表

| 方法 | 语义 | 幂等 | 安全 | 管理面使用约束 | 典型示例 |
|------|------|------|------|----------------|----------|
| **GET** | 获取资源表示 | ✅ | ✅ | 只读操作一律 GET；GET 请求不得修改服务端状态 | `GET /api/v1/users`（列表）、`GET /api/v1/users/{id}`（详情） |
| **POST** | 创建资源 | ❌ | ❌ | 新建集合成员；触发有副作用的动作；POST 必须返回 **201 Created + Location header**（集合新建）或 200（动作） | `POST /api/v1/users` → 201 + `Location: /api/v1/users/{newId}`；`POST /api/v1/users/{id}:impersonate` → 200 |
| **PUT** | 整体替换资源 | ✅ | ❌ | 客户端提供完整资源表示（所有必填字段）；存在 → 替换；不存在 → 创建 | `PUT /api/v1/users/{id}` |
| **PATCH** | 部分更新资源 | ✅ | ❌ | JSON Patch (RFC 6902) 或 Merge Patch (RFC 7396)；仅更新请求中出现的字段 | `PATCH /api/v1/users/{id}`（仅更新 `status` 字段） |
| **DELETE** | 删除资源 | ✅ | ❌ | 支持软删除（`DELETE → status=deleted`）；幂等删除返回 **204 No Content**；不存在返回 **404** | `DELETE /api/v1/users/{id}` |
| **HEAD** | 获取资源元信息 | ✅ | ✅ | 镜像 GET 响应头、无响应体 | `HEAD /api/v1/users/{id}` |
| **OPTIONS** | 列出允许的方法 | ✅ | ✅ | CORS 预检自动处理 | 浏览器自动调用 |

> **禁止** 在管理面使用自定义动词作为路径段（`/users/{id}/activate` 应改为 `PATCH /users/{id}` + `status=active`）。仅在语义无法用 HTTP 方法表达时允许动作端点（命名为 `资源:动作` 如 `/users/{id}:impersonate`）。

### 6.3 集合 vs 单资源

| 维度 | 集合（Collection） | 单资源（Item） |
|------|--------------------|----------------|
| 路径形态 | `/resources`（无 ID 段） | `/resources/{id}` |
| GET | 列表 + 分页 + 过滤 + 排序 | 详情 |
| POST | 新建成员 → 201 + Location | 动作（如审计查询触发） |
| PUT | 整体替换集合（禁止） | 整体替换单资源 |
| PATCH | 批量更新（`/resources:batch-update` 动作） | 部分更新单资源 |
| DELETE | 批量删除（`/resources:batch-delete` 动作） | 删除单资源 → 204 |

### 6.4 分页规范

管理面列表接口**默认 offset-based**（简单、广泛兼容），大表（≥ 10,000 行）**必须提供 cursor-based** 作为高性能回退。协议面（SCIM）遵循各协议规范。

#### Offset-based（默认）

| 参数 | 类型 | 默认 | 约束 |
|------|------|------|------|
| `page` | int | 1 | ≥ 1 |
| `pageSize` | int | 20 | 1-100；> 100 返回 422 |
| `sortField` | string | 主键 | 必须在白名单中；不在白名单返回 422 |
| `sortOrder` | string | `asc` | enum: `asc`, `desc` |

响应体：`{ items: [...], page: { page, pageSize, total } }`（对齐 `openapi.yaml` `PageMeta`）。

#### Cursor-based（大表强制）

| 参数 | 类型 | 默认 | 约束 |
|------|------|------|------|
| `cursor` | string | 空（首页） | opaque，仅由服务器签发 |
| `limit` | int | 50 | 1-500 |

响应体：`{ items: [...], nextCursor: string | null }`；`nextCursor` 为空表示无下一页；游标基于 `(sortField, id)` 稳定排序，**禁止 offset + cursor 混用**。

> **红线**：所有 ORDER BY 必须以 `id` 作为 tiebreaker，防止分页漂移（ADR-012 OpenAPI 生成器已内置）。

### 6.5 过滤 / 排序 / 搜索查询参数规范

| 类别 | 参数 | 格式 | 约束 |
|------|------|------|------|
| 过滤 | `organizationId` | UUID | 租户隔离参数；不用于权限控制（由 RLS 强制） |
| 过滤 | `status` | string | enum 值；多值用逗号分隔（`status=active,pending`） |
| 过滤 | `keyword` | string | 全文字段统一关键字搜索（用户名 + 邮箱 + 姓名，等）；**参数名端到端固定为 `keyword`**（参考 `openapi.yaml` 参数定义） |
| 过滤 | `filter` | SCIM filter (RFC 7644 §3.4.2.2) | 仅 `/scim/v2/*` 端点接受；禁止直接拼 SQL |
| 排序 | `sortField` | string | camelCase 字段名；白名单机制（不在白名单返回 422） |
| 排序 | `sortOrder` | string | enum: `asc`, `desc` |

> **禁止** 自由格式过滤表达式（`field=value,op=eq` 风格）进入 SQL。所有过滤条件在应用层映射到预编译 SQL 参数化查询。

## 7. 统一请求/响应信封规范

### 7.1 与 `openapi.yaml` ErrorResponse 对齐

管理面与认证面**统一采用「应用错误体三件套 + 顶层关联标识」**（协议面遵循各协议规范：RFC 6749/6750、RFC 7644 SCIM）：

```typescript
// openapi.yaml components.schemas.ErrorResponse；
// 与《开发规范》4.1.2「错误体契约（唯一事实源）」逐字段一致
interface ErrorResponseEnvelope {
  error: {
    /** 稳定错误码（小写 snake_case）；取值空间见 contracts/registries/errors.yaml */
    code: string;
    /** 面向调用方的可读描述（英文；控制台以 code 作 i18n 键） */
    message: string;
    /** 可选细粒度 i18n 键（messageKey > code > message） */
    messageKey?: string;
    /** 键值对象；字段级原因以 field 键标识，其余键为稳定机器可读参数 */
    details?: Record<string, unknown>;
    /** @deprecated 与 details 同值的兼容别名，不得作为新契约读取 */
    meta?: Record<string, unknown>;
  };
  /** 关联请求的唯一标识；从请求头 `X-Request-Id` 回显（无则服务端生成），必须贯通全链路 */
  requestId: string;
  /** W3C Trace Context 的 trace 标识，用于跨链路关联 */
  traceId: string;
}
```

| 字段 | 必填 | 类型 | 来源 |
|------|------|------|------|
| `error.code` | ✅ | string | `contracts/registries/errors.yaml`（唯一事实源） |
| `error.message` | ✅ | string | 面向调用方的可读文本（英文）；控制台以 `error.code` 作 i18n 键 |
| `error.messageKey` | ❌ | string | 可选细粒度 i18n 键；优先级 `messageKey > code > message` |
| `error.details` | ❌ | object | 键值对象；字段级原因以 `field` 键标识，仅可定位到具体字段时填充 |
| `error.meta` | ❌ | object | deprecated 兼容别名，与 `error.details` 同值；新契约不得读取 |
| `requestId` | ✅ | string（UUID） | 请求头 `X-Request-Id` 回显（无则服务端生成）；服务间必须传递 |
| `traceId` | ✅ | string | W3C Trace Context，与 `requestId` 一同用于跨链路关联 |

> 字段名与层级以《开发规范》4.1.2 的错误体契约为准；生成物以 `openapi/openapi.yaml` 的 `ErrorResponse` 为准（复现：`python3 scripts/gen_openapi.py --check`）。

### 7.2 组织上下文的唯一裁决者

请求里出现 `organizationId`（请求体、query 或 `X-Organization-Id` 头）时，**它不是可信输入**。租户由认证会话与 `TenantFilterMiddleware` 写入的 typed context 决定，handler 必须经 `auth.TrustedOrganizationID` 取值：

| 场景 | 取值规则 |
|---|---|
| 普通会话 | 一律使用会话租户；请求体/query/header 中的 `organizationId` 被忽略（伪造值不得把写操作搬到别的租户） |
| 平台管理员（作用域被提升为 `super_admin` / `system` 的显式跨租户操作） | 允许用请求中的目标租户，且必须经 ScopeAuthorizer 授权并留审计 |
| 无会话且无租户作用域 | 返回空值并 fail-close（不落库、不查询） |
| query `organizationId`（列表端点） | 与请求体同等对待：中间件将其解析为显式目标租户；非平台管理员用他租户 UUID 由 ScopeAuthorizer 拒绝，用他租户 slug 直接 403 |
| 本租户 UUID/slug | 不标记为跨租户目标；普通用户携带自身租户参数不会触发跨租户拒绝 |

响应 DTO 仍返回 `organizationId`，但它表达的是服务端最终采用的租户，而非调用方声明的租户。

### 7.3 能力不支持的错误语义

当某能力在当前方言下不可用（例如 MySQL 下的审计哈希链、平台管理员跨租户仓储），接口必须返回明确的「能力不支持」错误码（501/409 语义 + `contracts/registries/errors.yaml` 中的稳定 code），**不得返回 500 或空成功**。调用方据此降级或提示，而不是把不可用误判为服务故障。

### 7.4 成功响应信封

| 资源类型 | 信封结构 | 示例 |
|----------|----------|------|
| 单资源详情 | `{ ...resourceFields }` | `GET /api/v1/users/{id}` → 用户字段直接展开 |
| 集合列表（offset） | `{ items: [...], page: { page, pageSize, total } }`（`page` 从 1 起；`pageSize` 默认 20、上限 100，越界 422） | `GET /api/v1/users` |
| 集合列表（cursor） | `{ items: [...], nextCursor: string \| null }`（请求参数 `cursor` + `limit`；`nextCursor` 为空表示无下一页；不承诺 `total`） | `GET /api/v1/audit-events?limit=50&cursor=...` |
| 创建成功 | `{ ...resourceFields }` + HTTP 201 + `Location: /api/v1/{resources}/{newId}` | `POST /api/v1/users` |
| 批量操作（逐项独立） | HTTP **207 Multi-Status**；每项 `{ index, status, code, body?, error? }`（`operations` 数组上限 200，超限 422） | `POST /api/v1/batch/users` |
| 原子批量 | 全成功 200；任一失败整体回滚返回 4xx；不得在 207 中混用「声称原子」 | 独立端点或 `atomic:true` 语义 |
| 无返回值（DELETE/PATCH） | HTTP 204 No Content | `DELETE /api/v1/users/{id}` |

> 分页参数命名与游标纪律的唯一口径见《开发规范》4.7 / 4.8；批量操作协议见《开发规范》4.11 与 §8.2 端点族登记。

### 7.5 错误码分类

| HTTP 状态 | 语义与场景 |
|-----------|-----------|
| 400 | 报文畸形 / 不可解析 |
| 401 | 未认证 / Token 缺失或过期 |
| 403 | 无权限 / 需先改密 / 需完成 MFA |
| 404 | 资源不存在或跨租户不可见 |
| 409 | 唯一性冲突 / 幂等键同键异体 / 邀请已被使用 |
| 410 | 已下线 / 邀请过期 |
| 412 | 前提失败（版本冲突）：响应体返回当前 `revision`；`If-Match` 不匹配；**仅**用于版本冲突，其他冲突仍 409 |
| 413 | 请求体超过配置上限 |
| 422 | 语法合法但语义校验失败（`details` 填字段级原因） |
| 423 | 账户锁定 |
| 428 | 前提缺失：写操作未提供 `If-Match`；与 412 配对 |
| 429 | 触发限流器（**禁止**用 503 表达限流） |
| 500 | 未预期的服务端错误 |
| 501 | 能力不支持（Not Implemented） |
| 503 | 依赖不可用 / 授权引擎 fail-close（附 `Retry-After`） |

> 各状态对应的 `error.code` 取值、category 与场景以 `contracts/registries/errors.yaml` 为唯一来源，渲染层 `docs/generated/errors-registry.md` 按 HTTP 状态列出（复现：`python3 scripts/gen_registry_md.py`）。

> **红线**：错误响应**不得**包含 SQL 片段、堆栈跟踪、密钥或敏感业务数据；`contracts/registries/errors.yaml` 是唯一错误码来源，`scripts/check-errors.py` 校验 Go 代码中的错误码常量与注册表零差异。

## 8. 统一限流熔断规范

GuliangIAM 所有限流策略的**唯一事实源**为 `contracts/registries/limits.yaml`（参数）与 `contracts/registries/slo.yaml`（熔断阈值 + 错误预算）。本节提供引用索引，详细数值以 YAML 为准。

### 8.1 限流策略引用（limits.yaml）

速率、Burst、窗口与响应头参数为可生成事实，本文不复写；唯一事实源 `contracts/registries/limits.yaml`，渲染层 [limit-registry](../generated/limit-registry.md)（生成命令 `python3 scripts/gen_registry_md.py`）。本节仅保留「限流器 → 维度 → 挂载面 → policy_id」索引：

| 限流器 | 维度 | 挂载点 | 面 | policy_id |
|--------|------|--------|----|-----------|
| `DefaultAPIRateLimit` | per-client-ip | global | 全部 | `global-ip` |
| `AuthRateLimit` | per-ip | auth_surface | authentication | `auth-ip` |
| `SignupRateLimit` | per-ip | auth_signup_reset | authentication | `signup-ip` |
| `PasswordResetRateLimit` | per-ip | auth_password_reset | authentication | `password-reset-ip` |
| `OAuth2TokenRateLimit` | per-ip | `POST /oauth2/token` | protocol | `oauth2-token-ip` |
| `TokenEndpointRateLimit` | per-ip | protocol_surface | protocol | `token-endpoint-ip` |
| `OAuth2IntrospectRateLimit` | per-client-id | `POST /oauth2/introspect` | protocol | `oauth2-introspect-client` |
| `OAuth2RevokeRateLimit` | per-ip | `POST /oauth2/revoke` | protocol | `oauth2-revoke-ip` |
| `OtlpRateLimit` | per-ip | entries | protocol | `otlp-ip` |
| `VerificationBusinessLimits` | per_recipient_and_ip | 业务层（非中间件） | 全部 | — |

**限流响应头**（全部以 RFC 规范 + `limits.yaml response_headers` 为准）：

| Header | 语义 | 出现时机 |
|--------|------|----------|
| `RateLimit-Limit` | 窗口内可见上限 | 每次响应 |
| `RateLimit-Remaining` | 当前剩余额度 | 每次响应 |
| `RateLimit-Reset` | 距额度恢复的秒数（整数） | 每次响应 |
| `RateLimit-Policy` | 机器可读策略 id（comment 参数） | 每次响应 |
| `Retry-After` | 秒数（整数，禁止 Go 时长字符串） | 仅 429 |

**熔断原则**：

- Redis 分布式令牌桶不可用时，**低风险只读降级到进程内限额**，**高风险写操作保守拒绝（fail-close）**（`limits.yaml distributed.failure_mode`）
- 多实例部署下各节点进程内限额独立，总速率按部署规模折算；单实例 rate 不得直接放大（`limits.yaml distributed.rate_conversion_note`）
- 限流**只用于客户端过载防护**；系统级过载熔断依赖 SLO 错误预算（见下）

### 8.2 SLO 熔断阈值引用（slo.yaml）

可用性目标、错误预算窗口、Burn Rate 阈值与告警参数为可生成事实，本文不复写；唯一事实源 `contracts/registries/slo.yaml`，渲染层 [slo-registry](../generated/slo-registry.md)（生成命令 `python3 scripts/gen_registry_md.py`）。本节仅保留「面 → SLI」索引：

| 面 | SLI ID |
|----|--------|
| management | SLI-1 |
| authentication | SLI-2 |
| authentication（业务成功率） | SLI-2a |
| protocol | SLI-3 |

**错误预算阈值动作**（`slo.yaml budget_actions`）：按已用预算比例分级触发「预警 → 收紧（冻结非修复类发布）→ 冻结（feature freeze，豁免须双人批准）」；各档阈值与动作全文以注册表为准，本文不复写百分比。

**告警触发信号**（`slo.yaml alert_levels`）：分「严重 / 高 / 中」三级（认领与响应时限、触发信号清单以注册表为准）。典型信号类别：SLO 快速燃烧、token 重用检测、审计落盘链路中断（严重）；延迟劣化、登录错误率、授权执行失败（高）；容量水位、加密回退、密钥轮换超时（中）。

> **熔断 vs 限流**：限流是**客户端过载防护**（429），熔断是**系统级自保护**（503 + 错误预算耗尽 + 自动降级）。两者独立触发、独立治理。熔断目标见 ADR-019（分布式限流目标态）与 ADR-020（SLO + 错误预算）。

## 9. 前端 API 路径唯一事实源

### 设计原则

前端 API client 若各自硬编码端点字符串，拼写错误无法在编译期捕获，重构端点需逐文件搜索替换，违反唯一事实源原则。

**`web/src/shared/api/paths.ts` 是路径常量的唯一来源**：

- 路径常量与动态生成器按资源域组织（Applications / Auth / Users / Organizations / Casbin / Gateway ...）
- 根路径常量示例：`USERS = "/api/v1/users"`
- 动态路径生成器示例：`USER_BY_ID(id)`、`USER_WEBAUTHN_CREDENTIAL_BY_ID(userId, credentialId)`
- **Query 字符串由各 client 自行构造**（涉及各自参数逻辑），paths.ts 只固定 URL path 本体

### 门禁

| 判据 | 验证 |
| --- | --- |
| 生产代码硬编码 `/api/v1/` 为 0 | CI `paths.ts gate`（`.github/workflows/ci.yml`）阻塞合并；复现命令见本节末 |
| 路径常量导出数 | `grep -c '^export ' web/src/shared/api/paths.ts` |
| 关联字段一律用 Select 组件 | `name="*Id"` 为裸 `<Input>` 的命中数为 0 |

```bash
grep -rnE '"/api/v1/|`/api/v1/' web/src --include='*.ts' --include='*.tsx' \
  | grep -v '\.test\.' | grep -v '/paths\.ts' | grep -v '/generated/' | grep -v '/e2e/'
```

### 与 env.go 的同构关系

后端 `platform/config/env.go` 是环境变量的唯一事实源（ADR-021 决策 5），前端 `shared/api/paths.ts` 是 API 路径的唯一事实源——两者遵循同一模式：只定义路径本体与变量名而不构造查询参数、按领域分节便于检索、以门禁命令验证约束不被违反。

### 前端组件化与路径收敛要求

> 唯一事实源与交互简化在前端侧由三层闭合保证：`paths.ts`（路径常量）、Select 组件族（关联字段交互）、CI gate（防回退）。

| 组件 | value 口径 | 应用场景 |
|------|-----------|---------|
| OrganizationSelect | id | 组织引用 |
| UserSelect / UserPaginateSelect | id | 用户引用 |
| RoleSelect / RolePaginateSelect | id | 角色选择 |
| GroupPaginateSelect / GroupTreeSelect | id | 用户组选择 |
| ApplicationSelect | id | 应用引用 |
| ResourceSelect | id | 资源引用 |
| PlanSelect / PricingSelect / ProductSelect / OrderSelect | id | 商业域引用 |
| AffiliationSelect | 多种 | 成员隶属 |
| CountryCodeSelect / RegionSelect | code | 地区 |

组件清单以目录枚举为准（复现：`ls web/src/shared/ui/*Select.tsx`）。
**豁免边界**（合法的硬编码例外）：
- `.test.ts` 文件（测试契约）、`generated/`（生成物）、`e2e/`（Playwright 测试）
- `bootstrap/chi.go` 路由注册 + middleware 限流键（反向键名，不是 client API 调用）
- runtime.go / schema_sql / seed_sql（嵌入 schema）

## 10. 系统信息端点响应契约

`/api/v1/system/version`、`/info`、`/health`、`/ready` 的响应体是前端 `/system-info` 页面的直接契约。OpenAPI 对这些管理面 GET 统一使用泛化 `ResourceDetail`，因此**字段契约由 handler 响应结构与 handler 测试共同锁定**；字段清单不在此复写（复现：`grep -nE 'type (VersionInfo|SystemInfo|HealthInfo|MetricsInfo) struct' internal/transport/http/api/v1/system/handler.go`）：

| 端点 | 契约测试 |
|------|----------|
| `GET /api/v1/system/version` | `chi_test.go::TestSystemVersion_ResponseContract` |
| `GET /api/v1/system/info` | `TestSystemInfo_ResponseContract` |
| `GET /api/v1/system/health` | `TestSystemHealth_ResponseContract` |
| `GET /api/v1/system/ready` | 同上；nil 池退化为 liveness（`TestSystemReady_NilPoolDegradesToLiveness`） |

`/api/v1/system/*` 与 `/healthz` / `/livez` / `/readyz` 的分层：`/api/v1/system/*` 挂在 ops 组（`MetricsTokenMiddleware`，与 `/metrics` 同一 `METRICS_SCRAPE_TOKEN`）；`/healthz`（liveness）额外暴露加密能力状态（结构见 `internal/platform/crypto/suite.go` 的 `HealthStatus`）：`CRYPTO_PROFILE=gm` 且必需能力缺失时进程在监听前 fail-fast（端点不可达，不产生运行期 503），仅 `gm_tls` 缺失时返回 200 `status=degraded` 并披露 `degraded_reasons`；`status=unavailable`→503 仅覆盖已发布报告的防御/注入用例。`/livez`（liveness）与 `/readyz`（readiness，当前驱动 DB Ping）由 `internal/bootstrap/chi.go` 直接挂载。

约束：前端 Zod schema（`web/src/shared/api/iam/system.ts`）与 handler 响应结构必须同时修改；任何字段重命名都会在两侧测试之一失败。
