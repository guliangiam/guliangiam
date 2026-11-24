# GuliangIAM 开发规范手册

**文档版本**：V4.398（版本沿革见仓库根目录 [`CHANGELOG.md`](../CHANGELOG.md)）
**基线日期**：2026-09-17
**适用团队**：全体开发 / 测试 / DBA / DevOps / 安全
**文档状态**：正式发布
**审批人**：架构评审委员会

> **定位声明**：本手册描述 GuliangIAM 的**目标态开发规范**。正文全部条目均为必须遵守的规范性要求，不以「实施进度」为叙述对象；正文禁止出现实施进度标注、偏差登记、手写计数与核验日期。规范与架构基准的对应关系以 [architecture/README.md](architecture/README.md) 与 [ADR 索引](adr/README.md)（ADR-007 / ADR-015 / ADR-016 / ADR-021 / ADR-022）为准。
>
> **变更记录约定**：本手册**只保存规范正文**，禁止在本文件内维护任何形式的历史（版本表、修订记录、变更日志、勘误史一律不得写入）。全部版本沿革唯一维护在仓库根目录的 [`CHANGELOG.md`](../CHANGELOG.md)。发现规范正文与代码 / DDL / OpenAPI 不一致时，**直接修正正文**，并在同一次变更中写入 `CHANGELOG.md`。

---

1. 编程规范
2. 前端开发规范
3. 数据库设计规范
4. 接口与 API 规范
5. 架构与工程规范
6. Git 版本控制规范
7. 代码评审规范
8. 测试规范
9. 安全规范
10. 项目交付与文档规范
11. 性能与 SLO 规范
12. 可观测性规范
13. 部署与运维规范
14. 密钥与加密规范
15. 无障碍（A11y）规范
16. 国际化（i18n）规范
17. CI/CD 与质量门禁规范
18. 多实例与分布式一致性规范
19. 数据库变更与门禁

附录 A. 分层违规自动化检查脚本
附录 B. API 端点索引
附录 C. 错误码全量清单
附录 D. 环境变量完整清单
附录 E. 术语与缩写对照表
附录 F. 文档地图与维护责任
附录 G. 违规处理与豁免流程
附录 H. 推荐 IDE 配置与插件
附录 I. 自动化检查清单
附录 J. 错误、性能与国际化规范

---

## 0. 前置说明：权威技术栈基线

> **重要**：本章是全部规范的技术基线，所有其他章节引用版本号时以本章为准。若发现其他章节版本号与本章冲突，以本章为准。

### 0.1 技术栈基线表

| 层次 | 技术 | 事实源 | 备注 |
|------|------|--------|------|
| 后端语言 | Go | 见 `go.mod` + 复现：`grep '^go ' go.mod` | 兼容上一个 minor 版本 |
| HTTP 框架 | chi/v5 | 见 `go.mod` | 基于 net/http 标准库 |
| 数据库驱动 | pgx/v5 | 见 `go.mod` | 原生驱动，非 database/sql |
| 数据库 | PostgreSQL | CI 验证 18；生产版本见迁移/CI 支持矩阵 | 以迁移和 CI 支持矩阵为准 |
| 授权引擎 | Casbin | 见 `go.mod` | 不支持 v2 API |
| 密码哈希 | alexedwards/argon2id | 见 `go.mod` | 默认算法 |
| 前端框架 | React | 见 `web/package.json` | — |
| UI 组件库 | Ant Design | 见 `web/package.json` | 视觉令牌统一在 `web/src/styles.css` 的 `--gl-*` 变量中声明 |
| 服务端状态 | TanStack Query | 见 `web/package.json` | — |
| 客户端状态 | Zustand | 见 `web/package.json` | persist 中间件 |
| 路由 | React Router | 见 `web/package.json` | 声明式 + lazy |
| 构建 | Vite + Rolldown | 见 `web/package.json` | — |
| 类型系统 | TypeScript | 见 `web/package.json` | strict 模式 |
| 参数校验 | Zod | 见 `web/package.json` | — |
| i18n | i18next / react-i18next | 见 `web/package.json` | 语言包位于 `web/src/shared/i18n/locales/`，清单以该目录为准（复现：`ls web/src/shared/i18n/locales/`） |
| E2E | Cypress（主）/ Playwright（辅） | 见 `web/package.json`（`dependencies` + `devDependencies`，锁定主版本/精确版本，**禁止 `latest`**） | 关键流程覆盖；主次口径见 8.2 |
| 容器基础镜像 | Alpine | Dockerfile 声明 | 锁定版本，不得使用 `latest` 标签 |
| 包管理器（前端） | pnpm | 见 `web/package.json` 的 `packageManager` 字段 | — |

### 0.2 版本升级原则

- **Patch 版本**：可通过 `go get -u=patch` / `pnpm update` 自由升级，需通过 CI。
- **Minor 版本**：需在 PR 描述中说明变更点，架构师审批。
- **Major 版本**：必须走 RFC 流程，禁止在 feature 分支直接升级。

---

## 1. 编程规范

### 1.1 Go 编程规范

#### 1.1.1 包/模块命名

| 规则 | 示例 | 说明 |
|------|------|------|
| 全小写，无下划线 | `users`, `auth`, `casbin` | 符合 Go 惯例 |
| 域模块名用复数 | `users`, `organizations`, `applications` | 与数据库表名一致 |
| 避免通用名 | 禁止 `common`, `util`, `helper` | 包名应体现领域含义 |
| 内部包用 `internal` | `internal/domain/users` | 防止外部依赖 |
| 缩写保持统一 | `id` 不写 `Id`，`url` 不写 `URL` 作为字段后缀时除外 | 与 pgx 扫描保持一致 |

**包命名映射**：

```
Domain 层: internal/domain/{module}/    → 包名: {module}
App 层:   internal/app/{module}/        → 包名: app{Module}（避免与 domain 冲突）
Infra 层: internal/infra/postgres/{module}/ → 包名: {module}（PostgreSQL 唯一基线）
Transport: internal/transport/http/api/v1/{module}/ → 包名: {module}
Platform: internal/platform/{module}/   → 包名: {module}
```

#### 1.1.2 类/接口/结构体命名

| 规则 | 示例 | 说明 |
|------|------|------|
| 领域实体用 PascalCase | `User`, `Organization`, `Application` | 导出类型 |
| 仓储接口加 `Repository` 后缀 | `Repository`（在域包内）| 在域包内无需前缀 |
| Service 在 App 层 | `Service`（在 app 包内）| 构造函数: `NewService()` |
| Handler 在 Transport 层 | `Handler`（在 api 包内）| 构造函数: `NewHandler()` |
| 接口不加 `I` 前缀 | `Repository`, `Clock`, `Generator` | Go 惯例 |
| 请求/响应 DTO 加后缀 | `LoginRequest`, `UserResponse` | 在 transport 层定义 |
| 端口接口加能力后缀 | `JWTIssuer`, `PolicyEnforcer`, `StorageProvider` | 描述能力而非实现 |
| 适配器加 `Impl` 后缀 | `LDAPAuthenticatorImpl` | 仅当同包存在接口时 |

**领域模型命名约定**：

```go
// 实体
type User struct { ... }

// 值对象（嵌在实体内）
type UserProfile struct { ... }
type UserCredentials struct { ... }

// 集合元素
type UserIdentity struct { ... }
type UserMfaMethod struct { ... }

// 查询结构
type ListQuery struct { ... }

// 状态机（示例与 `internal/domain/orders/order.go` 同形：常量 PascalCase，取值全小写 snake_case，
// 闭集与迁移边唯一登记于 contracts/registries/state-machines.yaml）
const (
    StateCreated        = "created"
    StatePaymentPending = "payment_pending"
    StatePaid           = "paid"
    StateCompleted      = "completed"
    StateFailed         = "failed"
    StateCancelled      = "cancelled"
    StateRefunded       = "refunded"
    StateClosed         = "closed"
)
func CanTransition(from, to string) bool { ... }
```

#### 1.1.3 方法/变量/常量命名

| 规则 | 示例 | 说明 |
|------|------|------|
| 导出方法 PascalCase | `GetByID()`, `Create()`, `List()` | 仓储接口方法 |
| 未导出方法 camelCase | `loadAggregate()`, `toDomain()` | 内部辅助 |
| 布尔变量用 is/has 前缀 | `IsAdmin`, `HasPassword`, `IsForbidden` | 语义清晰 |
| 常量 PascalCase | `DefaultPageSize`, `MaxRetryCount` | 不用全大写+下划线 |
| 错误变量 `Err` 前缀 | `ErrNotFound`, `ErrUnauthorized` | 标准错误值 |
| 构造函数 `New` 前缀 | `NewService()`, `NewHandler()` | Go 惯例 |
| 上下文参数必须为 `ctx` | 所有仓储/服务方法第一个参数 | 强制 |

**仓储方法命名约定**：

```go
// 标准 CRUD
GetByID(ctx, id) (*Entity, error)
GetBySlug(ctx, orgID, slug) (*Entity, error)
List(ctx, query) ([]*Entity, int64, error)    // 返回列表+总数
Create(ctx, entity) error
Update(ctx, entity) error
Delete(ctx, id) error                          // 软删除优先

// 子实体操作
UpdateProfile(ctx, profile) error
ReplaceIdentities(ctx, userID, identities) error  // delete-all-then-insert
DeleteByFamilyID(ctx, familyID) (int64, error)    // 批量原子删除

// 业务查询
GetPasswordHistory(ctx, userID, limit) ([]Entry, error)
ConsumeByNonce(ctx, nonce) (*Entity, error)       // 原子消费（DELETE RETURNING）

// 统计查询（Dashboard StatsRepository 模式）
CountUsers(ctx, organizationID) (*UserCountResult, error)      // COUNT + FILTER
CountActiveSessions(ctx, organizationID) (*SessionCountResult, error)
GetLoginHeatmap(ctx, organizationID) ([]HeatmapPoint, error)   // 聚合查询
```

#### 1.1.4 代码格式

| 规则 | 说明 |
|------|------|
| 缩进 | Tab（Go 标准） |
| 行长度 | 不设 CI 硬限制；评审时建议 < 120 字符，长 SQL / 日志模板可超（原 `lll` 规则已停用，理由见 1.1.9） |
| 括号对齐 | K&R 风格（`gofmt` 标准） |
| 导入排序 | 标准库 → 第三方 → 项目内，空行分隔（`goimports` 标准） |
| 结构体字段对齐 | 使用 `gofmt` 自动对齐 |
| 强制方式 | `gofmt -l` 与 `goimports` 由门禁阻塞（1.1.9） |

**导入排序示例**：

```go
import (
    "context"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/guliangiam/guliangiam/internal/domain/users"
    "github.com/guliangiam/guliangiam/internal/platform/uuid"
)
```

#### 1.1.5 注释规范

**文件头注释**：不需要（Go 包文档通过 `doc.go` 提供）

**导出类型注释**（godoc 格式）：

```go
// User represents a user account in the system.
// A User belongs to exactly one Organization and can have
// multiple Identities, MfaMethods, RoleBindings, and GroupMemberships.
type User struct {
    ID             string    // UUIDv7 primary key
    OrganizationID string    // FK to organizations
    Username       string    // unique within organization
    IsAdmin        bool      // organization-level admin flag
    IsForbidden    bool      // if true, user cannot login
    // ...
}
```

**方法注释**：

```go
// GetByID retrieves a user by their unique identifier.
// Returns ErrNotFound if the user does not exist.
func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
```

**行内注释原则**：

- 注释"为什么"而非"做什么"
- 复杂业务逻辑必须注释
- 安全敏感代码必须注释决策理由

```go
// Fail-close: if Casbin fails to evaluate, deny the request rather
// than allowing unauthenticated access. This prioritizes security
// over availability during authorization service outages.
```

#### 1.1.6 错误处理与日志

**错误处理规则**：

| 规则 | 示例 | 说明 |
|------|------|------|
| 不吞错误 | 禁止 `_ = doSomething()` | 所有错误必须处理 |
| 错误向上传播 | `return fmt.Errorf("get user: %w", err)` | 使用 `%w` 包装 |
| 统一错误码 | `internal/platform/errors/errors.go` | 使用预定义错误码 |
| 业务错误 vs 系统错误 | `ErrNotFound` vs `fmt.Errorf("db query: %w", err)` | 区分处理 |
| `fmt.Errorf` 仅用于包装 | 禁止 `fmt.Errorf("invalid user")` | 无 `%w` 的参数化错误应用 `errors.New*` |

**统一错误码定义**：

```go
// internal/platform/errors/errors.go（节选；完整集合见 contracts/registries/errors.yaml 与 docs/generated/errors-registry.md）
type Code string

const (
    CodeInvalidArgument   Code = "invalid_argument"
    CodeNotFound          Code = "not_found"
    CodeConflict          Code = "conflict"
    CodeUnauthorized      Code = "unauthorized"
    CodeForbidden         Code = "forbidden"
    CodeInternal          Code = "internal"
    CodeUnimplemented     Code = "unimplemented"
    CodeAlreadyExists     Code = "already_exists"
    CodeUnavailable       Code = "unavailable"
    CodeTooManyRequests   Code = "too_many_requests"
    CodeAccountLocked     Code = "account_locked"
    CodePasswordExpired   Code = "password_expired"
    CodeMfaRequired       Code = "mfa_required"
    CodeInvitationUsed    Code = "invitation_used"
    CodeInvitationExpired Code = "invitation_expired"
    CodeGone              Code = "gone"
)
```

构造错误一律使用该包提供的构造器（`errors.NewInvalidArgument`、`errors.WrapInternalf` 等），HTTP 状态由 `codeToHTTPStatus` 统一映射，**禁止在 handler 内手写状态码或错误 JSON**。

**日志规范**：

| 规则 | 说明 |
|------|------|
| 结构化日志 | 使用 `slog`（JSON 格式） |
| 日志级别 | DEBUG / INFO / WARN / ERROR |
| 请求追踪 | 必含 `request_id`、`trace_id`、`span_id` |
| 不记录敏感信息 | 禁止记录密码、Token、密钥、`idCard` |
| 错误日志包含上下文 | 包含操作、参数摘要、错误原因 |
| 禁止使用 `log.*` 标准库 | 统一使用 `slog`，由 `platform/logging` 提供 handler |

```go
// 正确
slog.ErrorContext(ctx, "failed to create user",
    "organization_id", orgID,
    "username", username,
    "error", err,
)

// 错误
log.Printf("error: %v", err)
```

#### 1.1.7 通用最佳实践

| 实践 | 说明 |
|------|------|
| 使用 `context.Context` | 所有仓储/服务方法第一个参数必须是 `ctx context.Context` |
| 接口优先 | 依赖接口而非实现，构造函数注入 |
| 避免 `init()` | 不使用 init 函数，显式初始化（如 `InitLockoutCache()`） |
| 避免 `panic` | 业务逻辑中使用 error 返回，仅在不可恢复时 panic |
| 使用 `pgx/v5` 参数绑定 | 禁止字符串拼接 SQL |
| 使用 `UUIDv7` | 所有主键使用 `platformuuid.V7Generator` |
| 时钟抽象 | 使用 `clock.Clock` 接口，测试注入 `FakeClock` |
| 并发安全 | 共享状态使用 `sync.RWMutex`，写操作加写锁 |
| 乐观锁 | 所有 Update 使用 `WHERE id=$1 AND revision=$2`，由 `shared.CheckAffectedRows` 验证 |
| 软删除 | 使用 `deleted_at` 列，查询自动过滤 `deleted_at IS NULL` |
| 事务管理 | 通过 Unit of Work 统一管理，不在 Repository 层单独开事务 |

### 1.2 TypeScript 编程规范

#### 1.2.1 命名规范

| 类型 | 规则 | 示例 |
|------|------|------|
| 文件名 | kebab-case | `auth-api.ts`, `user-profile.tsx` |
| 组件名 | PascalCase | `LoginPage`, `UserDetailPage` |
| Hook 名 | use 前缀 | `useSessionBootstrap`, `usePermissions` |
| Store 名 | xxxStore | `sessionStore`, `authFlowStore` |
| 函数名 | camelCase | `iamList`, `iamCreate`, `buildQueryString` |
| 类型/接口 | PascalCase | `User`, `LoginRequest`, `SessionAccount` |
| Zod schema | xxxSchema | `userSchema`, `loginRequestSchema` |
| 常量 | SCREAMING_SNAKE | `DEFAULT_PAGE_SIZE`, `API_BASE_URL` |
| CSS 类名 | gl- 前缀 | `gl-empty-state`, `gl-sidebar` |
| QueryKey 工厂 | camelCase + 层级后缀 | `queryKeys.users.list`, `queryKeys.users.detail` |

#### 1.2.2 代码格式

| 规则 | 工具 | 说明 |
|------|------|------|
| 格式化 | oxfmt | 自动格式化，不可配置 |
| 代码检查 | oxlint | 规则配置在 `.oxlintrc.json` |
| 缩进 | 2 空格 | oxfmt 自动处理 |
| 行尾分号 | 无分号 | TypeScript 标准 |
| 引号 | 单引号 | oxfmt 自动处理 |
| 尾随逗号 | 有 | 多行结构加尾随逗号 |
| 类型导入 | 使用 `import type` | 避免运行时导入 |
| 路径别名 | `@/` 前缀映射 `src/` | 不使用相对路径的 `../../..` |

---

## 2. 前端开发规范

### 2.1 组件结构规范

**页面组件结构**（每个页面模块）：

```
pages/{module}/
├── {Module}Page.tsx          # 列表/主页面
├── {Module}CreatePage.tsx    # 创建页面（如需要）
├── {Module}DetailPage.tsx    # 详情/编辑页面
└── [可选] 子组件.tsx
```

**组件编写规则**：

| 规则 | 说明 |
|------|------|
| 函数组件 | 使用函数式组件 + Hooks，禁止 class 组件 |
| 懒加载 | 所有页面路由使用 `React.lazy()` 懒加载 |
| Error Boundary | 懒加载路由包裹 `RouteErrorBoundary` |
| Props 类型 | 使用 TypeScript 接口定义 Props |
| 默认导出 | 页面组件使用 `export default` |
| 命名导出 | 工具函数、Hooks、共享组件使用命名导出 |
| 大组件拆分 | 主文件 ≤ 300 行；超出拆分为子组件 + Hook |

**页面组件模板**：

```tsx
import { useQuery } from "@tanstack/react-query";
import { iamList } from "@/shared/api/iam/{module}";
import { queryKeys } from "@/shared/api/query-keys";

export default function {Module}Page() {
  const { data, isLoading } = useQuery({
    queryKey: queryKeys.{modules}.list(),
    queryFn: () => iamList<{Module}>({Module}Schema),
  });

  return <div className="gl-page">{/* 页面内容 */}</div>;
}
```

### 2.2 样式管理

| 规则 | 说明 |
|------|------|
| CSS 类前缀 | `gl-` 前缀避免冲突，禁止新增无前缀类名 |
| 暗色模式 | 使用 `data-theme="dark"` 属性选择器，浅色/深色共用同一套语义令牌名 |
| 设计令牌 | 统一在 `web/src/styles.css` 声明 `--gl-*` 变量；antd `ConfigProvider` 的 `token` 必须引用同一组取值 |
| 禁止内联样式 | 复杂样式使用 CSS 文件或 CSS-in-JS；**禁止在 TSX 中写行内颜色** |
| 全局样式 | `styles.css` 定义全局变量、`@layer` 顺序与基础样式 |
| 对比度 | 正文文本 ≥ 4.5:1，大号文本与图标 ≥ 3:1；禁止仅靠颜色传递状态（详见第 15 章） |
| 动效 | 必须尊重 `@media (prefers-reduced-motion: reduce)` |
| 禁用硬编码颜色 | 使用 CSS 变量（如 `--gl-color-text-secondary`） |

**令牌族（权威定义见 `web/src/styles.css`）**：

| 令牌族 | 前缀 | 说明 |
|---|---|---|
| 颜色 | `--gl-color-*` | `primary`/`primary-hover`/`primary-active`/`primary-bg`、`success`/`warning`/`error`（含 `-bg`/`-text`）、`text`/`text-secondary`/`text-tertiary`/`text-inverse`、`bg`/`bg-elevated`/`bg-code`、`border`/`border-strong` |
| 间距 | `--gl-space-*` | 4px 基准栅格（`--gl-space-1` = 4px） |
| 圆角 | `--gl-radius-*` | 卡片、输入、按钮、标签 |
| 字体/行高 | `--gl-font-*`、`--gl-line-*` | 正文默认 `"IBM Plex Sans", "Segoe UI", sans-serif` |
| 阴影 | `--gl-shadow-*` | 浮层、抽屉、模态层级 |
| 过渡 | `--gl-transition-*` | 统一过渡时长 |
| 布局 | `--gl-page-*`、`--gl-shell-*`、`--gl-panel-*`、`--gl-card-*`、`--gl-sidebar-*` | 页面容器、外壳、面板、卡片与侧边栏尺寸 |

**令牌红线**：新增视觉常量必须先加令牌再引用；不得引入临时色值、魔法像素或未登记的过渡时长。是否新增令牌在评审中必须明确回答"为什么现成令牌不可用"。

**文本颜色变量使用规范**：

```css
/* 正确 */
color: var(--gl-color-text-secondary);

/* 错误 */
color: #666;
```

### 2.3 状态管理约定

| 状态类型 | 方案 | 持久化 | 示例 |
|----------|------|--------|------|
| 服务端状态 | TanStack Query | 否（缓存 30s） | 用户列表、组织详情 |
| 全局 UI 状态 | Zustand | localStorage | 主题、资源 UI 状态 |
| 认证流程状态 | React 组件状态 + sessionStorage | 会话级（sessionStorage） | Passkey ceremony 中间态、OAuth PKCE verifier |
| 会话状态 | Zustand | 否 | 当前账户、当前组织 |

**Zustand Store 模板**：

```ts
import { create } from "zustand";
import { persist } from "zustand/middleware";

interface {Module}State {
  // 状态定义
}

export const use{Module}Store = create<{Module}State>()(
  persist(
    (set) => ({
      // 初始状态和 actions
    }),
    {
      name: "guliangiam-{module}", // localStorage key
      partialize: (state) => ({ /* 仅持久化非过期数据 */ }),
    }
  )
);
```

**TanStack Query 约定**：

```ts
// 默认配置（在 AppProviders 中）
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,          // 30s 缓存
      refetchOnWindowFocus: false, // 禁用窗口聚焦重取
      retry: (failureCount, error) => {
        if (error instanceof HttpError && error.status < 500) return false;
        return failureCount < 2;
      },
    },
  },
});
```

**QueryKey 工厂模式（强制）**：

```ts
// shared/api/query-keys.ts
export const queryKeys = {
  users: {
    all: ["iam", "users"] as const,
    lists: () => [...queryKeys.users.all, "list"] as const,
    list: (params?: UserListParams) => [...queryKeys.users.lists(), params] as const,
    detail: (id: string) => [...queryKeys.users.all, "detail", id] as const,
  },
  // ...
};
```

**缓存策略预设**：

| 预设 | staleTime | gcTime | retry | 适用场景 |
|------|-----------|--------|-------|---------|
| CACHE_AUTH | 5min | 30min | 0 | session、permissions |
| CACHE_LIST | 1min | 5min | 2 | users、tokens、orders |
| CACHE_DETAIL | 2min | 10min | 2 | settings |
| CACHE_CONFIG | 5min | 30min | 2 | organizations、roles、providers |
| CACHE_REALTIME | 10s | 1min | 1 | dashboard |

### 2.4 服务端分页与列表页规范

#### 2.4.1 ServerPagination 状态模式

列表页面必须使用服务端分页，禁止一次性加载全部数据后前端分页。

**状态定义**（`web/src/shared/api/iam/client.ts`）：

```ts
interface ServerPagination {
  page: number
  pageSize: number
}

interface PaginatedResponse<T> {
  items: T[]
  page: { page: number; pageSize: number; total: number }
}
```

**页面使用模式**：

```tsx
const [pagination, setPagination] = useState<ServerPagination>({ page: 1, pageSize: 50 })
const [search, setSearch] = useState("")

const { data, isLoading } = useQuery({
  queryKey: queryKeys.{modules}.list({ ...pagination, search }),
  queryFn: () => get{Modules}Paginated({ ...pagination, search }),
})

return (
  <ResourceListCard
    serverPagination={pagination}
    onServerPaginationChange={setPagination}
    onSearch={setSearch}
    dataSource={data?.items}
    total={data?.page?.total}
  />
)
```

#### 2.4.2 API 函数约定

| 函数 | 说明 |
|------|------|
| `getXxxPaginated(params)` | 带分页参数的列表查询，返回 `PaginatedResponse<T>` |
| `searchXxxPaginated(params)` | 带搜索+分页的列表查询，用于审计日志等需要全文搜索的场景 |
| `iamListWithPagination(url, params)` | 通用分页请求函数，适用于简单资源列表 |

#### 2.4.3 列表页渲染约定

| 规则 | 说明 |
|------|------|
| 状态标签 | 使用 Ant Design `Tag` 组件，按语义着色（成功=green, 警告=orange, 错误=red） |
| 时间格式 | 使用 `formatDateTime()` 统一格式化，禁止直接显示 ISO 字符串 |
| 搜索 | 通过 `onSearch` 回调更新 search state，由 queryKey 触发重新请求 |
| 空状态 | 使用 `ResourceListCard` 内置的空状态提示 |
| 排序 | 通过 `defaultSortField` / `onSortChange` 传递到 API |

### 2.5 无障碍与语义化

> 详见第 15 章。所有组件必须满足 WCAG 2.1 AA 标准。

### 2.6 共享 UI 组件规范

#### 2.6.1 组件文件组织

共享组件放置于 `web/src/shared/ui/` 目录，每个组件一个文件，文件名使用 kebab-case：

```
shared/ui/
├── image-crop-upload.tsx
├── json-editor.tsx
├── redirect-uri-table.tsx
├── webhook-header-table.tsx
├── token-attribute-table.tsx
├── saml-attribute-table.tsx
├── signin-method-table.tsx
├── FormSection.tsx
├── ResourceListCard.tsx
├── PermissionGuard.tsx
├── ConditionalField.tsx
├── StickyFormFooter.tsx
├── charts/
│   └── ActivityHeatmap.tsx
└── ...
```

#### 2.6.2 组件编写约定

| 规则 | 说明 |
|------|------|
| 命名导出 | 共享组件使用命名导出，不用 `export default` |
| Props 接口 | 组件内定义 `interface XxxProps`，导出类型 |
| Zod 校验 | 表单提交前使用 Zod schema 校验 |
| i18n | 所有用户可见文本使用 `tr()` 函数 |
| gl- 前缀 | 自定义 CSS 类名使用 `gl-` 前缀 |
| 无障碍 | 交互元素必须支持键盘，详见第 15 章 |

#### 2.6.3 全局样式约定

| CSS 类 | 用途 | 说明 |
|--------|------|------|
| `.gl-page` | 页面容器 | 标准页面内边距 |
| `.gl-empty-state` | 空状态 | 居中布局，含图标+标题+描述变体 |
| `.gl-form-section` | 表单分区 | 含标题+描述+分隔线 |
| `.gl-sidebar` | 侧边栏 | 可折叠，含组织名称+切换按钮 |
| `.gl-skip-link` | 跳过导航链接 | 键盘焦点可见，详见第 15 章 |
| `.gl-auth-*` | 认证页面设计系统 | Signup/ForgotPassword/AuthResult |

### 2.7 Storage Provider（前端视角）

存储 Provider 的**后端**抽象（`storage.Provider` 接口）、各后端实现（集合以 `internal/infra/storage/factory.go` 注册为准）、配置环境变量与上传端点契约，属后端 / 架构规范，已归位至 **5.5 存储 Provider 抽象规范（后端）**，本章不重复定义。

前端只消费上传契约：

- 端点：`POST /api/v1/upload-resource`（multipart/form-data）
- 返回：`{"data": "<url>"}`（`ImageCropUpload` 组件契约）
- 大小上限：32MB；Local 模式下静态文件经 `/uploads/*` 提供

### 2.8 列表页交互规范

#### 2.8.1 列级搜索

- ResourceListCard 支持 `searchableColumns` 属性，指定可搜索的列
- 搜索图标显示在列头右侧，点击展开搜索输入框
- 输入框 debounce 300ms 后触发 `onColumnSearch` 回调
- 搜索激活时列头高亮，清空输入自动取消搜索

#### 2.8.2 服务端排序

- ResourceListCard 支持 `defaultSortField` / `defaultSortOrder` / `onSortChange` 属性
- 可排序列设置 `sorter: true`，Ant Design 渲染排序指示器
- 排序变更通过 `onSortChange(field, order)` 回调父组件
- 父组件通过 `sortField` / `sortOrder` 查询参数传递给 API
- 后端 `buildOrderBy` 使用白名单防注入
- **传 `onSortChange` 就必须声明可排序列**：该回调只在 antd `Table.onChange` 中触发，而它**仅对声明了 `sorter` 的列**触发。只传回调而不给列加 `sorter`，回调永不运行，排序接线就是承诺了页面并不提供的能力。若该模块后端尚未登记 `sorting.Spec` 白名单，**不得**加 `sorter: true`——服务端不解析 `sortField` 时，点击表头只会造出新的静默无效。

> **参数名口径**：线上参数是 **`sortField`**（handler 读取的实际名称），不是 `sortBy`。`sortBy` 仅为前端 `IamListParams` 的内部属性名，`buildQueryString` 会将其映射为 `sortField`。

#### 2.8.3 过滤器下拉

- ResourceListCard `filters` 属性定义过滤项
- 每个过滤项包含 `key`、`label`、`options`
- 过滤值通过 `filterValues` 通知父组件
- **父组件必须把 `filterValues` 交给请求**（服务端模式下）：约定形式为 `filters: filterValues`，由 `buildQueryString` 展开为查询参数。仅把 `filterValues` 传给组件的 `filterValues` prop 是**不完整**的——组件只负责渲染与回传，不会代为请求
- 过滤项 `key` 必须是**后端 handler 实际读取**的参数名；后端无对应列/参数的筛选项应移除而非保留
- 多值过滤使用 `mode="multiple"`

### 2.9 Excel 导入导出规范

- 导出：调用 `GET /api/v1/users/export`，接收 streaming Excel 响应
- 导入：调用 `POST /api/v1/users/import`，上传 multipart Excel 文件
- 预览：导入前调用 `POST /api/v1/users/upload/preview` 解析 Excel 显示前 10 行数据供确认
- 模板下载：前端生成标准模板（含必填列标题）
- 错误处理：导入失败行汇总返回，前端展示错误详情
- 3 步流程：上传 → 预览 → 确认导入

### 2.10 模拟登录安全规范

- 仅超级管理员可执行模拟登录
- 目标用户必须未禁用（`isForbidden=false`）
- Impersonate token 有效期 1 小时
- 所有模拟登录操作写入审计日志（标记 `impersonated_by`）
- 模拟会话中 UI 显示醒目的"模拟登录"横幅
- 退出模拟登录使用唯一权威路径 `POST /api/v1/auth/exit-impersonate`（`auth/chi.go`，`auth/routes.go`）；`/api/v1/users/{id}:exit-impersonate` 等旧路径一律下线或改为重定向，不得并存；该端点必须鉴权且写审计

### 2.11 导航与侧边栏规范

菜单是**唯一**的导航事实源，集中在 `web/src/features/navigation/menu.tsx` 的 `menuGroups` 中定义；面包屑由同文件的 `getBreadcrumbs()` 派生。

| 规则 | 说明 |
|------|------|
| 单一事实源 | 新增页面必须先在 `menuGroups` 注册，再写路由与页面；禁止在页面内自建导航或面包屑 |
| 分组结构 | `MenuGroup = { key, label, icon, routeKeys, children }`；`MenuChild = { key, label, icon, matchPaths? }`；`key` 即路由路径 |
| 可见性判据 | 分组声明 `routeKeys`（该组覆盖的路由集合）；`getVisibleRouteKeys(account, organization)` 计算可见集合；`buildMenuItems(account, organization, t)` 过滤分组 |
| 无权限不渲染 | 无权限菜单项**不渲染**，禁止渲染为禁用态（禁用态会泄露系统结构信息） |
| 分组自动隐藏 | 组内子项全部不可见时，该组自动隐藏（由 `routeKeys` 交集判定成立） |
| 平铺降级 | 子项总数 ≤ `FLAT_MENU_THRESHOLD` 时降级为平铺列表，避免少量菜单出现无意义层级 |
| 服务端裁决 | 前端隐藏只是体验优化；服务端必须独立鉴权，越权请求一律 403 |
| i18n | 标签一律使用 i18n 键（如 `nav.home`、`nav.identity`），禁止硬编码中文/英文文案 |

**菜单分组**（变更需同步本节、[design/frontend/interaction.md](design/frontend/interaction.md) 与 `menu.test.ts`）：

| 分组 `key` | 分组标签 | `routeKeys` 覆盖路由 |
|---|---|---|
| `home` | `nav.home` | `/`、`/shortcuts`、`/apps`、`/oidc-discovery` |
| `identity` | `nav.identity` | `/organizations`、`/settings`、`/users`、`/users/import`、`/groups`、`/invitations`、`/applications`、`/providers` |
| `authz` | `nav.authorization` | `/roles`、`/permissions`、`/models`、`/adapters`、`/enforcers`、`/rebac`、`/actions` |
| `audit` | `nav.audit` | `/sessions`、`/records`、`/notifications`、`/tokens`、`/verifications`、`/entries` |
| `business` | `nav.business` | `/store`、`/products`、`/plans`、`/coupons`、`/cart`、`/orders`、`/payments`、`/pricings`、`/subscriptions`、`/transactions`、`/metering` |
| `integration` | `nav.integration` | `/syncers`、`/migration`、`/webhooks`、`/webhooks/:webhookId/events`、`/ldaps`、`/servers`、`/sites` |
| `llm` | `nav.llmAi` | `/ai/providers`、`/ai/providers/new`、`/ai/providers/:id`、`/ai/providers/:id/test`、`/ai/models`、`/ai/aliases`、`/ai/routes`、`/ai/api-keys`、`/ai/quotas`、`/ai/prices`、`/ai/usage`、`/ai/usage/stats`、`/ai/health`、`/ai/agents`、`/ai/agents/new`、`/ai/agents/:id`、`/ai/mcp-servers`、`/ai/mcp-servers/new`、`/ai/mcp-servers/:id`、`/ai/mcp-servers/store` |
| `security` | `nav.security` | `/resources`、`/certs`、`/keys`、`/captcha-providers`、`/captcha/test`、`/face-id-providers`、`/notification-providers`、`/radius-configs`、`/radius-accountings`、`/rules`、`/risk`、`/iga`、`/idv`、`/identity-pools` |
| `admin` | `nav.admin` | `/system-info`、`/account`、`/theme-editor`、`/forms`、`/tickets`、`/swagger`、`/i18n/languages`、`/i18n/translations` |

**交互约定**：分组可折叠且状态持久化；当前项高亮且父分组自动展开；支持键盘方向键移动 + `Enter` 进入；移动端收起为抽屉；`MenuSearch.tsx` 提供跨分组快捷搜索；`AuditSectionNav.tsx` / `AuthzSectionNav.tsx` / `BusinessSectionNav.tsx` 提供段内二级导航。搜索词与折叠状态为纯 UI 状态，不写入服务端。

**一致性校验**：`menu.test.ts` 与 CI 必须验证（a）菜单路由存在于路由表；（b）`routeKeys` 覆盖该组全部子项路径；（c）i18n 键在所有语言包中存在。三者任一缺失即门禁失败。

### 2.12 详情页与表单控件模式规范

> 以下规范由门禁 `scripts/check-form-detail-patterns.py` 强制校验。

#### 2.12.1 Zod schema 数组字段 — 必须 `.catch([])`，禁止 `.default([])`

Go `json.Marshal(nil_slice)` 输出 JSON `null`（不是 `[]`）。Zod `.default([])` 仅处理 `undefined` 当字段缺失的情形，无法处理 API 返回 `null` — 整个 schema 将 throw。必须用 `.catch([])` 同时兜底 `undefined` 和 `null`。

```typescript
// ❌ 禁止 — null 会导致 schema throw
z.array(z.string()).default([])

// ✅ 必须 — null 和 undefined 都兜底为空数组
z.array(z.string()).catch([])

// ❌ 禁止 — 公共 flexibleObjectArraySchema 无 catch
export const flexibleObjectArraySchema = z.array(flexibleObjectSchema);

// ✅ 必须 — 两处都加 catch
export const flexibleObjectSchema = z.record(z.string(), z.unknown()).catch({});
export const flexibleObjectArraySchema = z.array(flexibleObjectSchema).catch([]);
```

**门禁**：`scripts/check-form-detail-patterns.py` 每次 commit 必须通过 Pattern A 检查。

#### 2.12.2 SPA navigate 下 debounce/timeout 必须在 entity-key sync 块清理

`useEffect(() => () => clearTimeout(...), [])` 仅在**完整 unmount** 时执行。SPA navigate 不 unmount 组件 — 仅 re-render 且 deps 变化。Entity A 触发的 setTimeout 在导航到 Entity B 后仍然 fire，会用旧 entity 的 state setter 污染新 entity 页面。

```typescript
// ❌ 禁止 — entity-key sync 里缺 clearTimeout
const dirtyDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
const entityKey = isNew ? "new" : (detail?.id ?? "");
if (prevEntityKey !== entityKey) {
  setEntityKey(entityKey);
  // ❌ 缺 clearTimeout(dirtyDebounceRef.current)
  setIsDirty(false);
}

// ✅ 必须 — entity-key sync 里先清 timer，再重置状态
if (prevEntityKey !== entityKey) {
  setEntityKey(entityKey);
  if (dirtyDebounceRef.current) {
    clearTimeout(dirtyDebounceRef.current);
    dirtyDebounceRef.current = null;
  }
  setIsDirty(false);
}
```

#### 2.12.3 Router param 条件渲染必须显式守卫 `!isNew`

路由 `/applications/new` 中 `useParams()` 返回字符串 `"new"`（truthy）。任何按 `entityId ? <Tab /> : null` 条件渲染的 Tab / Detail 组件，在 CreatePage 场景会错误渲染并查询不存在的 entity ID。必须加 `!isNew &&` 守卫。

```tsx
// ❌ 禁止 — "new" 是 truthy 字符串
children: applicationId ? <SigninLogTab applicationId={applicationId} /> : null

// ✅ 必须
children: !isNew && applicationId ? <SigninLogTab applicationId={applicationId} /> : null
```

#### 2.12.4 Switch Form.Item — 必须声明 `valuePropName="checked"`

AntD Switch 组件以 `checked` prop 控制开关状态，但 Form.Item 默认把 Form 值传给 `value` prop（Switch 不识别）。缺少 `valuePropName="checked"` 时 Switch 状态永远错误，用户操作也不会写回 Form。

```tsx
// ❌ 禁止
<Form.Item name="pkceRequired" hidden>
  <Switch />
</Form.Item>

// ✅ 必须
<Form.Item name="pkceRequired" hidden valuePropName="checked">
  <Switch />
</Form.Item>
```

#### 2.12.5 handleReset — 必须同步更新 dirty 基线 ref

`configForm.resetFields()` 将 Form 恢复到最近一次 `setFieldsValue` 的快照，但 `initialConfigRef.current` 可能还停留在更早的版本（保存前 / SPA 导航前）。dirty 检测下次对比会用旧基线，导致已重置好的状态错误被标记为 dirty。

```typescript
// ❌ 禁止
const handleReset = useCallback(() => {
  configForm.resetFields();
  setIsDirty(false);
}, [configForm]);

// ✅ 必须 — reset 基线 ref 同步
const handleReset = useCallback(() => {
  configForm.resetFields();
  initialConfigRef.current = normalizeConfigSnapshot(tr, configForm.getFieldsValue(true));
  setIsDirty(false);
}, [configForm, tr]);
```

#### 2.12.6 onSuccess — 基线更新必须先于 `setIsDirty(false)`

`onSuccess` 中 `setIsDirty(false)` 硬重置脏状态后，`initialConfigRef.current` 要等 `invalidateQueries` → React Query refetch → useEffect 依赖变化才会更新。这个时间窗口内用户任何修改又撤销，dirty 检测基线错位。必须在 `setIsDirty(false)` 之前先更新基线。

```typescript
// ❌ 禁止 — 基线更新滞后
onSuccess: async () => {
  setIsDirty(false);                        // ❌ 先硬重置
  await invalidateQueries(...);             // 基线 ref 等这里才更新
  message.success(tr("saved"));
}

// ✅ 必须 — 基线更新在前
onSuccess: async () => {
  initialConfigRef.current = normalizeConfigSnapshot(tr, configForm.getFieldsValue(true));
  setIsDirty(false);
  await invalidateQueries(...);
  message.success(tr("saved"));
}
```

#### 2.12.7 Tabs URL sync — 静态常量解耦，禁止 useMemo 引用尚未声明的变量

React StrictMode 下 `useMemo` / `useCallback` / `useEffect` 的 factory 函数在**首次渲染时同步执行**。如果 URL sync 块写在 `const formTabs = [...]` 声明之前且引用 `formTabs`，会触发 TDZ（Temporal Dead Zone）白屏。

```typescript
// ❌ 禁止 — validTabKeys 在 formTabs 声明之前引用它
const validTabKeys = useMemo(() => new Set(formTabs.map(t => t.key)), []);
const formTabs = [...]; // ← formTabs 在这里声明！Runtime: ReferenceError

// ✅ 必须 — 静态常量解耦
const ALL_TAB_KEYS = ["overview", "basic", "auth", ...]; // 文件级常量
const validTabKeys = useMemo(() => {
  const hasOverview = !isNew && !!detail;
  return new Set(ALL_TAB_KEYS.filter(k => k !== "overview" || hasOverview));
}, [isNew, detail]);
```

#### 2.12.8 审计门禁

以下命令在 commit 前必须手动运行或由 CI 门禁执行：

```bash
# 审计脚本（Pattern A-F 必须全绿；Pattern G 已知误报，按说明过滤）
python3 scripts/check-form-detail-patterns.py

# TypeScript 编译（零错误）
cd web && npx tsc --noEmit

# Go 编译（零错误）
go build ./...
```

---

## 3. 数据库设计规范

> 本章为数据库设计的**唯一事实源**，与 [ADR-007](adr/ADR-007-uuidv7.md) / [ADR-016](adr/ADR-016-rls-tenant-isolation.md) / [ADR-022](adr/ADR-022-declarative-schema-versioned-migration.md) 共同构成数据库规范基准。`scripts/check-db-conventions.sh` 是本章的强制门禁，已接入 CI `gates` 作业；提交前必须本地执行并全部通过。

### 3.0 唯一运行期基线：PostgreSQL 18+（PostgreSQL-first）

GuliangIAM 的**唯一运行期数据库基线是 PostgreSQL 18+**（ADR-023）。RLS、`SKIP LOCKED`、部分索引、`ON CONFLICT`/`RETURNING`、GIN/JSONB、数组、`DISTINCT ON`、`ILIKE`、:: 转型、`INET`/`CIDR`、生成列、事务内 DDL 均直接依赖，不做运行期兼容层。

MySQL 为 **infra 镜像方言**（ADR-023 多方言边界）：仅用于 DDL 与仓储兼容性验证（`db/mysql/`、`internal/infra/mysql/`），运行期缺口由 `OPEN-API-37` 跟踪，`DB_DRIVER=mysql` 启动探针 fail-fast；不得在没有真实 MySQL 重放证据时宣称 MySQL 可用。方言边界与开放项状态以 `contracts/registries/open-items.yaml` 为准。

### 3.1 设计总则与 DDL 维护约定

- **声明层唯一入口**（ADR-022）：表、列、约束、索引的结构变更只改 `contracts/schema/*.yaml`，再运行 `python3 scripts/gen_schema.py` 生成 DDL、版本化迁移与配对回滚；`db/postgresql/`、`db/rollback/`、`internal/bootstrap/schema_sql/` 均为生成物，**禁止手改**（门禁 `check-generated-zero-diff.sh`、`check-migrations.sh`）。
- **结构链纯度**：结构链（`db/postgresql/V{N}__*.sql`）只承载 DDL 与结构必要的元数据语句，不得混入业务数据 `INSERT`；内置组织/管理员/目录等数据一律走数据链 `db/seeds/`（见 3.4）。
- **数据链纯度**：数据链只允许 DML，禁止任何 DDL（见 3.4.1「类别纯度」）；需要新结构时先新增结构迁移，再在种子中引用。
- **事务与一致性**：业务写路径必须经 `UnitOfWork` 事务边界；审计记录与 Outbox 事件与业务事实同事务落库（ADR-008）。
- **租户隔离**：业务表必须带 `organization_id` 并启用 RLS `ENABLE + FORCE`（ADR-016）；仓储显式租户过滤，MySQL 镜像无 RLS，过滤责任在仓储 SQL。
### 3.2 命名约定

> 本章分两层：本节主表与 3.2.1 规定**表/列/索引/约束/API/事件/错误码/文件/目录/符号**的命名形态；同一概念跨层拼写的**唯一裁决**（JSON/SQL/URL/claim）登记于 `contracts/registries/naming.yaml`（渲染层 `docs/generated/naming-registry.md`，N1–N15 裁决 + `disallowed_spellings`），任何一层不得自造同义词或缩写。

| 对象 | 规则 | 示例 |
|------|------|------|
| 表名 | 小写、下划线、**复数**、语义化业务名（不使用 `gateway_`/`iam_` 等历史前缀） | `users`, `user_profiles`, `webhook_events`, `sites` |
| 列名 | 小写、下划线、单数 | `organization_id`, `display_name`, `created_at` |
| 主键 | 统一 `id`（逻辑类型 `UUID`，UUIDv7 由应用层生成，列定义无默认值） | `id` |
| 外键列 | `{被引用表单数}_id` | `organization_id`, `application_id`, `provider_id` |
| 普通索引 | `idx_{表名}_{列名...}` | `idx_orders_state`, `idx_users_org_created_at_desc` |
| 唯一索引/唯一约束 | `uq_{表名}_{列名...}`，**禁止 `uk_` 前缀** | `uq_users_org_username_active` |
| 部分唯一索引 | `uq_{表名}_{列名}_active`；谓词为 `WHERE deleted_at IS NULL`（PostgreSQL 部分索引） | `uq_organizations_slug_active` |
| 外键约束 | `fk_{表名}_{引用表}`（仅在需要显式命名时） | `fk_orders_users` |
| 检查约束 | `ck_{表名}_{语义}` | `ck_orders_amount_non_negative` |
| 序列 | 不使用 | UUIDv7 主键无需序列 |
| Schema/库前缀 | 所有表位于 `guliangiam` schema，`guliangiam.` 前缀保留 | 不新增其他 schema |
| 表级唯一约束 | **必须显式命名**，禁止裸 `UNIQUE (...)` | `CONSTRAINT uq_roles_org_name UNIQUE (organization_id, name)` |
| 时间列 | `*_at` 后缀，逻辑 `TIMESTAMP_TZ`，UTC | `created_at`, `updated_at`, `deleted_at`, `expires_at` |
| 金额列 | `{业务}_amount` 命名并配独立 `currency` 列，逻辑 `DECIMAL(19,4)` | `paid_amount` + `currency` |
| 布尔列 | `is_` / `has_` / `can_` 前缀，禁止 `disable_*` / `enable_*` 反义命名 | `is_admin`, `has_mfa` |
| 状态列 | 统一列名 `status`（状态机状态与启用/禁用语义一律用 `status`；`state` 仅存量例外，禁止新增，见命名裁决 N15），取值全小写 snake_case，DDL `CHECK` 约束；状态机迁移规则唯一登记于状态机注册表 | `status = 'pending'` |
| API 路径 | 资源名复数、路径小写+连字符、`/api/v1/` 前缀（见 4.1.1） | `/api/v1/captcha-providers` |
| API 字段 | 管理面 / 认证面 camelCase；协议面按 RFC 标准 snake_case（见 4.5） | `organizationId` / `start_index` |
| 事件名 | 领域事件 `{域}.{对象过去式}`，复合事件 `{域}.{对象}_{过去式动作}`（命名裁决 N13）全小写，事件域与事件名唯一登记于 `contracts/registries/events.yaml`（口径见 12.5） | `organization.created` |
| 错误码 | 应用错误码与协议错误码唯一登记于附录 C（见 4.1.2 映射） | `not_found` |

#### 3.2.1 文件、目录、符号与生成物命名

| 场景 | 规则 | 示例 | 强制手段 |
|------|------|------|----------|
| Go 包/模块 | 全小写、无下划线、域模块用复数、禁止 `common`/`util`/`helper` | `users`、`casbin` | 1.1.1 + `check-layers.sh`（目录与依赖方向） |
| Go 源文件 | 小写+下划线，按层职责命名（一文件一职责） | `repository.go`、`sql.go`、`handler.go`、`service.go`、`mapper.go` | 目录约定 + 评审 |
| Go 生成文件 | 后缀 `*_generated.go`，头注释标明生成器与事实源，禁止手改 | `state_generated.go` | `check-generated-zero-diff.sh` |
| Go 测试文件 | `*_test.go` | `repository_test.go` | `go test ./...` |
| Go 符号（包/结构体/接口/方法/变量/常量） | 导出 PascalCase、未导出 camelCase、常量 PascalCase、错误值 `Err` 前缀、布尔 `is/has/can` 前缀、上下文参数固定 `ctx` | `GetByID`、`loadAggregate`、`ErrNotFound`、`IsAdmin` | 1.1.2/1.1.3 + `go vet` + `check-lint.sh` |
| 前端文件 | kebab-case；组件 PascalCase；页面 `*Page.tsx`；测试 `*.test.ts(x)` | `user-profile.tsx`、`UsersPage.tsx` | `check-frontend-architecture.py` + 评审 |
| 前端生成物 | `web/src/generated/{kebab}.ts`，头注释标明生成器，禁止手改 | `state-machines.ts`、`error-codes.ts` | `check-generated-zero-diff.sh` |
| 生成器脚本 | `gen_{对象}.py`（下划线），只读事实源、只写生成物 | `gen_openapi.py`、`gen_registry_md.py` | `check-generated-zero-diff.sh` 逐一调用 |
| 门禁脚本 | `check-{对象}.{py,sh}`（连字符），失败即非零退出 | `check-doc-links.py`、`check-migrations.sh` | `verify-docs.sh` 调度 + 附录 I |
| 结构声明 | `contracts/schema/{NN}_{域}.yaml`（两位分组前缀，表族拆分） | `20_identity.yaml`、`75_ai_gateway.yaml` | `gen_schema.py` / `check-active-schema-history` |
| 注册表与渲染层 | `contracts/registries/{kebab}.yaml` → `docs/generated/{kebab}-registry.md` | `known-red.yaml` → `known-red-registry.md` | `gen_registry_md.py --check` |
| 迁移脚本 | 前滚 `db/postgresql/V{N}__{主题}.sql`、配对回滚 `db/rollback/V{N}__{主题}.down.sql`（均为生成物，禁手改）；版本在链内自 1 起连续；V1 为生成式全量基线，V2+ 由生成器 diff 产出 | `V15__idempotency_keys.sql` | `check-migrations.sh` |
| ADR | `ADR-{三位序号}-{kebab}.md`，序号只增不复用 | `ADR-022-declarative-schema-versioned-migration.md` | `check-doc-anchors.py` + `adr/README` 索引 |
| 文档 | 主题 kebab-case；根目录入口固定四文件（README/开发规范/验收条件/追溯矩阵） | `data-model.md`、`api-surfaces.md` | `check-doc-links.py` + `verify-docs.sh` 文件存在性 |
| 环境变量 | 全大写 snake_case，必须登记 `contracts/registries/env.yaml` 且有唯一读取点 | `RATE_LIMIT_GLOBAL_RATE` | `check-registry-sync.py` env 读取点断言 |
| 事件/指标 | 事件 `{域}.{对象过去式}`（N13）；指标 `guliangiam_{域}_{对象}_{单位}`（N14） | `organization.created`、`guliangiam_http_requests_total` | `events.yaml` / `slo.yaml` + `check-registry-sync.py` |
| API 标识 | 路径/字段见 3.2 主表与 4.1；`operationId` 全仓唯一 | `organizationId`、`ListUsers` | `gen_openapi.py` 唯一性断言 + `check-openapi-contract.sh` |

**覆盖边界**：本表覆盖**文件/目录/符号/生成物**层面的命名形态；**同一概念的跨层拼写裁决**（如 `organizationId`/`organization_id`/`org_id`）唯一登记于 `contracts/registries/naming.yaml`。新增命名场景必须归入本表或该注册表之一，禁止在设计文档中自造同义词。

### 3.3 字段设计规范

> 下表左右两列为**逻辑类型**与**类型语义**，**不是**可以直接写进 DDL 的物理类型。物理类型即为 PostgreSQL 18+ 的对应声明。

| 规范 | 说明 |
|------|------|
| 时间 | 逻辑类型 `TIMESTAMP_TZ`，统一存 UTC，禁止无时区时间；默认 `now()` |
| 金额 | 逻辑类型 `DECIMAL(19,4)`，**禁止** `DECIMAL(10,2)` 与浮点类型（支持多币种与分账精度） |
| 币种 | 逻辑类型 `VARCHAR(3)`（ISO 4217 三字母码），禁止超过 3 位；**全库统一 `VARCHAR(3)`，不使用 `CHAR(3)`** |
| 标识符引用 | 引用 `users`/`organizations`/`applications` 等核心实体的列一律逻辑 `UUID`，禁止 `VARCHAR(255)`/`TEXT` 存 ID |
| 标识符文本 | 业务标识符（`slug`、`username`、`client_id`）用逻辑 `VARCHAR(n)` 并登记上限，禁止无界 `TEXT` |
| 布尔 | 逻辑 `BOOLEAN`，命名以 `is_` / `has_` / `can_` 开头，禁止 `disable_*` / `enable_*` 反义命名，必须有明确 `DEFAULT` |
| 状态 | 统一列名 `status`（业务生命周期与状态机状态同用 `status`；`state` 仅存量例外，禁止新增，见命名裁决 N15），取值全小写 snake_case |
| 字符串 | 逻辑 `TEXT` 为默认；需要长度约束时用 `VARCHAR(n)` 并在规范中登记上限；URL 类见下一行 |
| URL 列长度 | 逻辑 `VARCHAR(n)`，按**是否承载查询参数**取档：短链接 / 发票 / 支付跳转结果页用 `VARCHAR(500)`（如 `payment_url`、`invoice_url`）；OAuth 回调、`success_url` 等携带 query/state 参数的用 `VARCHAR(2000)`；个别带中等长度参数的用 `VARCHAR(1000)`。新增列就近取档并在 PR 说明，禁止再写 `VARCHAR(500~2000)` 这种无判定规则的区间表述 |
| 数组 | 逻辑 `ARRAY<TEXT>`，默认 `'{}'`。**PostgreSQL 以外方言不支持时降级为子表或 JSON 数组**（见 3.3.2） |
| JSON | 逻辑 `JSON`，默认 `'{}'` / `'[]'`。PostgreSQL 用 `JSONB`，MySQL/OceanBase 用 `JSON`，达梦用 `CLOB` + 应用层校验 |
| 二进制 | 逻辑 `BLOB`（如 WebAuthn 公钥、证书 DER） |
| 网络地址 | 逻辑 `IP`；PostgreSQL 用 `INET`，其他方言用 `VARCHAR(45)` + 应用层校验（禁止依赖列类型做 IP 校验） |
| 敏感字段 | 密文列以 `_hash` / `_enc` / `_ciphertext` 结尾；禁止明文存储口令、TOTP 种子、私钥、验证码（见第 14 章） |
| 必备字段 | 每表必须有 `id`, `created_at` |
| 可更新表 | 必须有 `updated_at TIMESTAMP_TZ NOT NULL DEFAULT now()` 与 `revision BIGINT NOT NULL DEFAULT 1` |
| 软删除表 | 追加 `deleted_at TIMESTAMP_TZ NULL` |

#### 3.3.1 受限 SQL 子集

所有方言**必须**支持下列语句与函数；子集外的一律视为方言专有构造，必须由 `internal/infra/dialect/` 生成：

| 类别 | 允许（子集内） | 禁止直接书写（必须经 Dialect） |
|------|------|------|
| 基本 DML | `SELECT`（显式列）/ `INSERT` / `UPDATE` / `DELETE`，参数占位符由 Dialect 生成 | 字符串拼接、手写 `?` 或 `$n` 混淆 |
| UPSERT | 语义调用 `Dialect.Upsert(...)` | `ON CONFLICT ... DO ...`、`ON DUPLICATE KEY UPDATE`、`MERGE` |
| 插入回填 | 语义调用 `Dialect.InsertReturning(...)`，不支持时回退为「先备 ID 再插入」 | `RETURNING`、`OUTPUT`、达梦 `RETURNING INTO` |
| 类型转换 | 参数按逻辑类型由 Dialect 绑定 | `$1::uuid`、`CAST(... AS jsonb)` 等方言化写法 |
| 分页 | `Limit` / `Offset` 语义参数 | `LIMIT ... OFFSET` 手写、`TOP n`、`ROWNUM` |
| 锁 | `Dialect.LockForUpdate(...)` / `Dialect.DequeueBatch(...)` | `FOR UPDATE SKIP LOCKED`、`NOWAIT` 直接书写 |
| 时间 | 传入 UTC `time.Time`，SQL 形如 `created_at >= $1` | `now()` 与 Go 时间混用、`DATE_ADD`/`INTERVAL` 方言写法 |
| 索引 DDL | 普通 `CREATE INDEX`、`CREATE UNIQUE INDEX`（用户代码不经手，由 DDL 生成器产出） | `CONCURRENTLY`、`ALGORITHM=INPLACE`、`ONLINE=ON` 手写 |
| 部分索引 | 仅在 PostgreSQL/兼容方言启用（方言增强） | 作为其他方言的**必要条件** |
| JSON 查询 | JSON 列整存整取 + 应用层解析 | `->>`、`#>>`、`JSON_EXTRACT` 直接书写 |
| 聚合过滤 | 普通 `COUNT/SUM/MAX/MIN` | `FILTER (WHERE ...)`、`DISTINCT ON` |
| 大小写匹配 | 应用层统一归一化后等值比较 | `ILIKE`、`LOWER()` 包裹索引列 |

#### 3.3.2 逻辑类型 → 方言物理类型映射

| 逻辑类型 | PostgreSQL | MySQL / MariaDB / TiDB / OceanBase / PolarDB | 人大金仓 / GaussDB | 达梦 DM8（参考） |
|---|---|---|---|---|
| `UUID` | `UUID` | `CHAR(36)`（大写无连字符则为 `CHAR(32)`，由 Dialect 统一） | `UUID` | `CHAR(36)` |
| `TEXT` | `TEXT` | `LONGTEXT` | `TEXT` | `CLOB` |
| `VARCHAR(n)` | `VARCHAR(n)` | `VARCHAR(n)` | `VARCHAR(n)` | `VARCHAR(n)` |
| `VARCHAR(3)`（币种 ISO 4217） | `VARCHAR(3)` | `VARCHAR(3)` | `VARCHAR(3)` | `VARCHAR(3)` |
| `BOOLEAN` | `BOOLEAN` | `TINYINT(1)` | `BOOLEAN` | `TINYINT(1)` |
| `INTEGER` | `INTEGER` | `INT` | `INTEGER` | `INT` |
| `BIGINT` | `BIGINT` | `BIGINT` | `BIGINT` | `BIGINT` |
| `DECIMAL(p,s)` | `DECIMAL(p,s)` | `DECIMAL(p,s)` | `DECIMAL(p,s)` | `DECIMAL(p,s)` |
| `TIMESTAMP_TZ` | `TIMESTAMPTZ` | `DATETIME(6)`（连接固定 `time_zone='+00:00'`） | `TIMESTAMPTZ` | `TIMESTAMP(6) WITH TIME ZONE` |
| `JSON` | `JSONB` | `JSON` | `JSONB` | `CLOB` + 应用层校验 |
| `BLOB` | `BYTEA` | `LONGBLOB` | `BYTEA` | `BLOB` |
| `IP` | `INET` | `VARCHAR(45)` | `INET` | `VARCHAR(45)` |
| `ARRAY<TEXT>` | `TEXT[]` | 不支持：改子表或 `JSON` 数组 | `TEXT[]` | 不支持：改子表或 `CLOB` 内 JSON 数组 |

说明：

- 映射表是**生成 DDL 与绑定参数**的唯一依据；业务代码只声明逻辑类型。
- 不支持 `ARRAY` 的方言，需为对应列提供子表或 JSON 数组的等价建模，且**必须在 PR 中说明 API 契约不变**。
- 时间列在所有方言下都视为 UTC；MySQL 连接必须显式 `time_zone='+00:00'`，否则 `DATETIME(6)` 会丢失时区语义。

#### 3.3.3 标准表模板

所有新建表必须按以下模板组织列序与必备列（方言物理类型由生成器按 3.3.2 映射产出）：

```sql
CREATE TABLE IF NOT EXISTS guliangiam.<table> (
    id              UUID NOT NULL PRIMARY KEY,          -- UUIDv7 由应用层生成，无库函数默认值（ADR-007 / ADR-022）
    organization_id UUID NOT NULL,                      -- 租户列；全局表逐表在隔离分级注册表声明例外（ADR-016）
    status          VARCHAR(32) NOT NULL,               -- 状态列，取值全小写 snake_case
    revision        BIGINT  NOT NULL DEFAULT 1,         -- 乐观锁；纯追加表可省略本列与 updated_at
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ NULL,                   -- 软删除表才有；业务唯一键走 `_active` 部分唯一索引
    -- 业务列……
    CONSTRAINT ck_<table>_status CHECK (status IN ('active', 'disabled' /* …… */))
);
```

模板红线：

1. `id` 无数据库默认值，禁止方言函数（`uuidv7()` 等）；主键值一律由应用层 `platform/uuid` 生成。
2. 状态列必须带 DDL `CHECK`，合法取值与状态机注册表一致。
3. 多租户表必须带 `organization_id UUID NOT NULL` 并启用 RLS FORCE（3.6.1）。
4. 可更新表必须带 `revision`；软删除表必须带 `deleted_at`，且活跃唯一性按 3.4 在库内强制。

### 3.4 软删除与唯一约束

软删除表的「业务唯一键」**不得**使用表级 `UNIQUE` 约束，否则被删除行会永久占用取值空间。统一做法：

```sql
-- 业务键的活跃唯一性：使用部分唯一索引
CREATE UNIQUE INDEX IF NOT EXISTS uq_organizations_slug_active
    ON guliangiam.organizations (slug)
    WHERE deleted_at IS NULL;
```

| 规则 | 说明 |
|------|------|
| 活跃唯一 | 软删除表的唯一键一律使用 `CREATE UNIQUE INDEX ... WHERE deleted_at IS NULL` |
| 命名 | `uq_{表名}_{列名...}_active` |
| 非软删除表 | 可直接使用 `CONSTRAINT uq_... UNIQUE (...)` |
| 已确立的活跃唯一键 | `organizations.slug`、`users(organization_id, username)`、`sites.host`、`user_profiles.email` |
| 非软删除业务唯一键 | `groups(organization_id, name)`、`consents(user_id, application_id)`、`provider_configs(provider_id, config_type)`：均为普通 `CONSTRAINT uq_... UNIQUE (...)` |
| **UPSERT 仲裁** | 部分唯一索引**不能**被不带谓词的 `ON CONFLICT (cols)` 命中；仲裁目标必须原样重复索引谓词 |

`ON CONFLICT` 与部分唯一索引必须成对书写，否则 PostgreSQL 在解析阶段报 `there is no unique or exclusion constraint matching the ON CONFLICT specification`：

```sql
-- 正确：仲裁目标带上与部分唯一索引一致的谓词
INSERT INTO guliangiam.organizations (slug, display_name)
VALUES ('built-in', 'Built-in Organization')
ON CONFLICT (slug) WHERE deleted_at IS NULL DO NOTHING;

-- 正确：DO UPDATE 同样需要谓词
INSERT INTO guliangiam.users (id, organization_id, username, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (organization_id, username) WHERE deleted_at IS NULL
DO UPDATE SET status = EXCLUDED.status, revision = users.revision + 1;

-- 错误：缺少 WHERE deleted_at IS NULL，解析阶段直接失败
-- ON CONFLICT (organization_id, username) DO NOTHING;
```

同样，仲裁列必须与唯一约束/索引的列集合**完全一致**（既不能少也不能多）。例如 `casbin_rules` 的唯一键为 `(enforcer_id, ptype, v0, v1, v2, v3, v4, v5)`，仲裁目标也必须写全 8 列。

#### 3.4.1 跨方言「活跃唯一」方案

部分唯一索引是 PostgreSQL（及兼容方言）的**方言增强**，不是必备能力。同一语义在各方言下的等价实现：

| 方言 | 实现 | 说明 |
|------|------|------|
| PostgreSQL / KingbaseES / GaussDB（支持部分索引） | `CREATE UNIQUE INDEX ... WHERE deleted_at IS NULL` | 首选，强度最高 |
| MySQL / MariaDB / TiDB / PolarDB / OceanBase | 生成列 + 普通唯一索引 | 见下方示例；OceanBase 与 TiDB 需实测 STORED 生成列支持度 |
| 达梦 DM8 / 神舟通用 | 生成列不支持时改用**未用值填充列**（如 `active_key` 固定为 `'1'`，软删除时置 `NULL`）+ `UNIQUE` | 必须由 Dialect 生成，不得应用层自行约定 |

MySQL 方案示例（由 DDL 生成器、而非人手书写）：

```sql
-- 软删除行 置 NULL，活跃行 置 1；NULL 不参与唯一性判定（MySQL 与 PostgreSQL 语义一致）
alter table `guliangiam`.`users`
    add column `is_active` tinyint generated always as (if(`deleted_at` is null, 1, null)) stored;

create unique index `uq_users_org_username_active`
    on `guliangiam`.`users` (`organization_id`, `username`, `is_active`);
```

红线：

1. **唯一性必须在库内保证**，不得只靠应用层先查后写（并发下必失败）。
2. 降级方案必须保持与 PostgreSQL 方案**完全相同**的业务语义（包括撤回/恢复时重新占用键）。
3. 能力探测失败时，启动日志必须输出警告并暴露到 `/system-info`；若缺少降级路径，**必须拒绝启动**。

#### 3.4.2 UPSERT 跨方言对应

| 方言 | UPSERT | 仲裁目标 |
|------|------|------|
| PostgreSQL | `ON CONFLICT (cols) [WHERE pred] DO ...` | 唯一索引/约束，部分索引必须原样重复谓词 |
| MySQL / MariaDB / TiDB / PolarDB / OceanBase | `ON DUPLICATE KEY UPDATE` | 由唯一索引决定，**无谓词概念** |
| 达梦 / 神舟通用 | `MERGE INTO ... USING ...` | 由唯一索引 + 等值条件表达，需自行处理并发重试 |

应用代码**只写语义**，由 `Dialect.Upsert(...)` 生成对应语句；由于 MySQL 无谓词，上表 3.4.1 的生成列方案是 MySQL 下 UPSERT 能正确命中活跃唯一键的前提条件。

> `ON CONFLICT` 仲裁目标可解析且存在唯一约束，由 CI 结构门禁强制（见第 19 章）；各方言须提供等价断言。

### 3.5 乐观锁

| 规则 | 说明 |
|------|------|
| 列定义 | `revision BIGINT NOT NULL DEFAULT 1`（**禁止 `DEFAULT 0`**） |
| 更新 | `UPDATE ... SET ..., revision = revision + 1, updated_at = now() WHERE id = $1 AND revision = $2` |
| 冲突 | 影响行数为 0 → 返回 `409 Conflict`（错误码见附录 C） |
| 例外 | 纯追加表（如审计、事件）可不加 `revision`，但也**不应**定义 `updated_at` |
| 密钥版本 | `provider_secrets.version` 表示密钥版本（轮换用），与 `revision` 语义不同，两者并存 |

### 3.6 索引与约束

| 规则 | 说明 |
|------|------|
| 查询驱动 | 仅对真实高频查询建索引，新增索引需在 PR 中附 `EXPLAIN (ANALYZE)` 证据 |
| 外键索引 | 所有外键列必须有索引 |
| 唯一约束即索引 | 引用了 UPSERT 的表，其仲裁列集合必须有对应的唯一约束/索引；不得依赖「应用层看起来唯一」而缺失约束 |
| 状态过滤 | 高频过滤状态列建索引，软删除表优先建部分索引 |
| 联合索引 | 遵循最左前缀原则；已有 `(a, b)` 不再单独建 `(a)` |
| 禁止重复 | 同一文件内禁止重复 `CREATE INDEX` 同名定义，禁止 `DROP INDEX` + 重建的写法 |
| 默认类型 | 等值/范围查询使用 B-tree；JSON 查询在支持 GIN 的方言下可用 GIN（方言增强，MySQL 用生成列 + 索引或全文索引替代） |
| 部分索引 | 方言增强：PostgreSQL/兼容方言用 `WHERE deleted_at IS NULL` / `WHERE revoked_at IS NULL`；其他方言用 3.4.1 的等价方案 |
| 外键策略 | **核心强引用**（users/organizations/applications/orders 等）必须在 DDL 层建立 FK；无法用单表 FK 表达的引用（多态聚合、跨系统标识）降级为逻辑引用，但必须在 `contracts/registries/referential-integrity.yaml` 登记分类与理由，由 `python3 scripts/check-fk-coverage.py` 阻塞门禁。凡声明 FK，必须**显式**书写 `ON DELETE` 行为，禁止依赖默认值（NO ACTION 隐式行为） |
| 删除行为 | 三选一：**归属关系** `ON DELETE CASCADE`；**可选引用** `ON DELETE SET NULL`（列须可空）；**核心事实表** `ON DELETE RESTRICT`（存在事实引用时禁止删除主档） |
| 外键索引 | 所有外键列必须建索引；复合外键索引须覆盖最左前缀 |
| 依赖顺序 | DDL 生成器按依赖拓扑排序产出建表顺序，**禁止前向引用**（引用表不得早于被引用表创建） |
| 分区 | 大表（`records`、`entries`、`webhook_events`、`access_tokens`、`sessions`）按时间分区；分区决策必须附带 `EXPLAIN (ANALYZE, BUFFERS)` 证据与容量指标，未取证不分区。各方言分区语法差异极大，分区 DDL 只允许由 DDL 生成器产出（PostgreSQL 声明式分区 / MySQL RANGE 分区 / 达梦表空间分区） |
| 行级安全 | 多租户表必须启用 RLS 并 `FORCE`（见 ADR-016 与 3.6.1 模板）。**RLS 为 PostgreSQL 特有**；其他方言必须由应用层 `TenantFilter` + 仓储强制 `organization_id` 条件补齐，不得声称「已有 RLS 保护」 |
| 索引并发创建 | `CREATE INDEX CONCURRENTLY`（PostgreSQL）与 `ALGORITHM=INPLACE`（MySQL）均属方言语法，仅允许出现在 DDL 生成器中 |

#### 3.6.1 租户隔离与 RLS 模板

多租户表（含 `organization_id` 的表）必须启用 RLS 并 `FORCE`；全局表与间接隔离表逐表在**隔离分级注册表**中声明（直接租户列 / 间接隔离 / 全局，见 ADR-016）：

```sql
-- 建表迁移同事务提交：启用并 FORCE，防止表属主绕过策略
ALTER TABLE guliangiam.<table> ENABLE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.<table> FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_<table> ON guliangiam.<table>
    USING (organization_id = current_setting('app.current_organization_id')::uuid)
    WITH CHECK (organization_id = current_setting('app.current_organization_id')::uuid);
```

会话侧约定：

1. 每个事务开始时执行 `SELECT set_config('app.current_organization_id', $1, true)`（第三参数 `true` = 事务级，事务结束自动失效）。
2. 连接归还连接池前必须 `RESET ALL`，防止租户上下文跨请求残留（ADR-016）。
3. RLS 是**第二道防线**：应用层 `TenantFilter` 严格模式（租户上下文缺失即拒绝请求）仍是第一道防线，二者缺一不可。

### 3.7 宽表与子表

单表列数超过约 30 列，或存在「读写频率差异大 / 敏感度差异大 / 可选性差异大」的列组时，必须拆分子表。

| 主表 | 子表 | 拆分依据 |
|------|------|----------|
| `users` | `user_profiles` | 展示类字段，读多写少 |
| `users` | `user_credentials` | 凭证类字段，敏感 + 写少 |
| `users` | `user_password_history` | 追加型历史 |
| `organizations` | `organization_settings` / `organization_theme_settings` / `organization_security_policies` | 配置类字段按域拆分 |
| `providers` | `provider_configs` / `provider_secrets` | 配置与密钥分离，密钥独立轮换 |
| `webhook_events` | `outbox` | 事件（事实）留 `webhook_events`；投递（可重试状态）归 `outbox`（`aggregate_type='webhook'`） |

子表统一约定：以 `{主表}_{域}` 命名；主表主键作为子表主键（1:1）或外键（1:N）；子表不重复存储主表已有字段。

**宽表拆分（`applications`）**：`applications` 是全库最宽的表，列组按域切分为子表；**子表集合与列以 `contracts/schema/20_identity.yaml` 申报为准**（复现：`grep -n 'app_' contracts/schema/20_identity.yaml`），当前包括：

| 主表 | 拆分子表 | 拆分依据 |
|------|----------|----------|
| `applications` | `app_oauth_config` / `app_client_secrets` / `app_client_jwks` / `app_client_jwks_uris` / `app_client_certificates` | 客户端凭据与密钥材料，敏感度独立、独立轮换与审计 |
| `applications` | `app_saml_config` | SAML 协议配置，仅 SAML 应用使用 |
| `applications` | `app_network_config` | 网络/回调类配置 |
| `applications` | `app_signin_config` / `app_form_config` | 登录页/表单/品牌类展示配置，读多写少 |

拆分不改动 API 契约（对外仍是 application 资源），由 App 层在一个事务内写多表。

**契约保持设计（拆分实施时必须逐条满足）**：

| 维度 | 要求 | 理由 |
|---|---|---|
| 资源标识 | 子表以 `application_id` 作为主键/外键，**不**新增外部可见 ID | 避免同一资源出现两套 ID，OpenAPI 的 `applicationId` 路径参数不得变化 |
| 读取路径 | 仓储对外仍返回完整 `Application` 聚合；子表读取由仓储内部 `LEFT JOIN` 或按需二次查询完成，**不**暴露给 App 层拼装 | 保持 `internal/domain/applications` 聚合边界不变 |
| 写入路径 | 创建/更新在同一事务内写多表；子表行缺失时按默认值 upsert | 避免部分列写入后留下残缺资源 |
| 默认值语义 | 子表列的 `NOT NULL DEFAULT` 必须与原宽表逐列一致 | 否则历史行回填后行为漂移 |
| 版本控制 | `revision` 保留在主表，子表不各自维护乐观锁；子表更新必须同事务频增主表 `revision` | 保证 `If-Match`/`ETag` 语义不变 |
| 迁移路径 | 首次上线时在主表建与子表同名的 `VIEW` 供存量代码过渡，再逐步切换；VIEW 在全部消费者切换后删除 | 允许不停机切换；VIEW 为**过渡产物**，不得长期保留 |
| 数据回填 | 回填必须幂等（`ON CONFLICT DO NOTHING`），并以子表行数为断言 | 重放安全 |
| 验收钩子 | 拆分前后 `openapi/openapi.yaml` 零差异；且新增一条验收条件断言「同一请求在拆分前后返回相同资源体」 | 使「不改 API 契约」可机检 |

> 本项与 ADR-023 的“契约先于方言”原则一致：拆分是**内部**物理重构，不得成为修改对外契约的理由。

### 3.7.1 平台内核表

以下三张表不属于任何业务聚合，是跨模块的**平台内核**，任何新功能都不得绕过它们另建同类表：

| 表 | 作用 |
|---|---|
| `guliangiam.outbox` | 事务性发件箱：与业务事实同事务写入，由 Worker 异步投递 |
| `guliangiam.idempotency_keys` | 统一幂等：写操作去重与响应重放 |
| `guliangiam.encryption_keys` | 密钥版本元数据：按版本解包、轮换与退役 |

> 三张表的列、索引、约束与 CHECK 取值以 `contracts/schema/00_platform.yaml` 为唯一源；复现与校验：`python3 scripts/gen_schema.py --check`。本节只约束它们的平台内核归属与使用规则，不复制结构清单。

约定：

- `outbox` 是**唯一**允许承载跨模块异步副作用的表；禁止在业务表内写 `retry_count`/`delivery_status` 之类的投递字段。
- **Webhook 投递不另建表**：投递统一走 `outbox`（`aggregate_type='webhook'`、`aggregate_id=webhook_id`），禁止保留或新建 `webhook_deliveries` 类投递表；`http_status_code` 记入 `outbox.headers`、`response_body` 截断记入 `outbox.last_error`，不得为此新增列污染 `outbox` 通用语义；`outbox.max_attempts` 默认值为 12（对齐 design/infrastructure/outbox.md 的 `max_attempts` 唯一口径）。
- `idempotency_keys` 的 `scope` 必须包含租户与主体（如 `orders:{organization_id}:{user_id}`），防止跨租户键碰撞。
- `encryption_keys` **只存密钥引用与指纹，禁止存密钥材料**；`key_material_ref` 指向 KMS/Vault 或 Secret 名。

### 3.8 DDL 检查清单（提交前逐项自检）

> **口径说明**：3.1–3.7 描述的物理类型与写法是**默认基线方言（PostgreSQL 18+）**的规范。3.8 分两组：**A 组**是 PostgreSQL 基线检查项；**B 组**是面向多数据库（ADR-023）的**约束性红线**，由 `scripts/check-sql-portability.sh` 保证。两组都要过，不要用 B 组去否定 A 组。

#### A 组：PostgreSQL 基线（必须满足）

- [ ] 表名复数、语义化，无 `iam_` / `gateway_` 等历史前缀
- [ ] `id UUID NOT NULL PRIMARY KEY`（**无数据库默认值**，UUIDv7 由应用层生成，见 3.3.3）
- [ ] `created_at` / `updated_at` 为 `TIMESTAMPTZ NOT NULL DEFAULT now()`
- [ ] 可更新表含 `revision BIGINT NOT NULL DEFAULT 1`，且**无** `DEFAULT 0`
- [ ] 软删除表含 `deleted_at TIMESTAMPTZ NULL`，业务唯一键改用 `_active` 部分唯一索引，且不得并存 `is_deleted` 等布尔删除标记
- [ ] 金额 `DECIMAL(19,4)`、币种 `VARCHAR(3)`、ID 引用 `UUID`（全库仅允许 `DECIMAL(19,4)`）
- [ ] 布尔列 `is_*` / `has_*` / `can_*`，且带明确默认值
- [ ] 状态列统一为 `status`（禁用 `state` 列名，见命名裁决 N15），取值全小写 snake_case，DDL `CHECK` 约束与状态机注册表一致
- [ ] 多租户表含 `organization_id UUID NOT NULL`，并已提交 `ENABLE ROW LEVEL SECURITY` + `FORCE` 与租户策略（见 3.6.1）
- [ ] 所有 `CREATE` 语句带 `IF NOT EXISTS`
- [ ] 无 `DROP SCHEMA/TABLE/INDEX`、无 `TRUNCATE`
- [ ] 无 FK 前向引用（建表顺序由 DDL 生成器按依赖拓扑排序）；每个 FK 显式声明 `ON DELETE` 三则之一（CASCADE / RESTRICT / SET NULL），且外键列已建索引；FK 完整性由 `python3 scripts/check-schema-parity.py --fk` 阻塞门禁
- [ ] 每个 UUID 类型的 `*_id` 列要么有物理 FK，要么在 `contracts/registries/referential-integrity.yaml` 登记为 `logical_references` / `pending_physical_fks`（后者必须指向开放项）；`python3 scripts/check-fk-coverage.py` 通过
- [ ] 所有 `CHECK` 约束命名 `ck_{表名}_{语义}`，表达式引用列均为该表自有列（禁止跨表 `CHECK`，由应用层保障）；枚举类 `CHECK` 取值与状态机注册表一致，由 `scripts/check-enums.py` + `scripts/check-schema-parity.py --check` 双重校验
- [ ] 无冗余索引：任何显式索引不得被唯一约束/唯一索引的最左前缀完全覆盖；同一表上不得出现 `(a, b)` 与 `(a)` 共存（后者被前者覆盖），由 `scripts/check-schema-parity.py --redundant-index` 阻塞门禁
- [ ] 高增长表的清理/归档路径已定义（见第 19 章），且 `expires_at` 类列带索引
- [ ] 新增的跨模块异步副作用走 `outbox`，未在业务表内新增投递状态列
- [ ] 写操作若可重试，已在 `idempotency_keys` 语义下定义 `scope` 与 `request_hash`
- [ ] 新增密钥用途时已在 `encryption_keys.purpose` 登记，且禁止落明文密钥材料
- [ ] 敏感数据已加密或哈希（见第 14 章）；机密列接入字段加密，加密密钥在所有环境必填，缺失即拒绝启动
- [ ] 已执行 `bash scripts/check-db-conventions.sh` 且通过
- [ ] 已执行 `bash scripts/check-sql-portability.sh` 且通过（无子集外方言语法）
- [ ] 已同步 `internal/bootstrap/schema_sql/` 下的同名副本（SHA-256 一致）
- [ ] 已通过 CI 结构门禁十项（空库全量建表、上一版本升级、回滚、FK/索引/约束/CHECK 可解析、ON CONFLICT 仲裁目标、引用列类型一致、软删除唯一性、租户表 RLS、生成物零差异、方言能力探测，见第 19 章，ADR-022）

#### B 组：多数据库约束与红线（ADR-023）

- [ ] **（红线）** 列类型已用 3.3.2 中的逻辑类型声明，未直接写 `JSONB`/`TEXT[]`/`INET`/`BYTEA` 等方言物理类型作为规范
- [ ] **（红线）** 软删除表的活跃唯一键已有 3.4.1 中各方言的等价方案，未把部分唯一索引当作必要条件
- [ ] **（红线）** 未在 Transport/App/Domain/普通 Repository 中写 `ON CONFLICT` / `RETURNING` / `::` 转换 / `SKIP LOCKED` / `MERGE`
- [ ] **（红线）** 主键由应用层 `internal/platform/uuid` 生成，`id UUID NOT NULL`（**无数据库默认值**），不依赖 `guliangiam.uuidv7()` 等方言函数
- [ ] **（红线）** 各方言 DDL 与迁移产物由声明式 Schema 经 DDL 生成器产出（ADR-022），方言专有构造仅存在于 `internal/infra/dialect/{postgres,mysql,...}`
- [ ] **（红线）** CI 中存在 PostgreSQL 18 与 MySQL 8.4 LTS+ 两个方言结构门禁作业（见 17.1 与第 19 章）


### 3.9 SQL 编写规范

| 规则 | 说明 |
|------|------|
| 方言隔离（硬约束） | 除 `internal/infra/dialect/<dialect>/` 外，任何 Go 代码中不得出现 `ON CONFLICT`、`RETURNING`、`::` 类型转换、`FOR UPDATE SKIP LOCKED`、`MERGE`、`->>` 等方言构造；违者门禁直接失败 |
| 参数化查询 | 使用参数绑定（**禁止**字符串拼接）；占位符形态由 Dialect 产出（PostgreSQL `$1`、MySQL `?`） |
| 类型转换 | 参数与列类型的绑定由 Dialect 按逻辑类型解决；禁止在业务 SQL 中书写 `$1::uuid`、`CAST(... AS jsonb)` 等方言化转换 |
| SELECT 明确列 | 禁止 `SELECT *`，明确列出所需字段 |
| 分页查询 | 使用 `Limit`/`Offset` 语义参数或 keyset 分页；不得手写方言分页语法 |
| 批量操作 | 批量写入由 Dialect 产出：PostgreSQL 可用 `UNNEST($1::text[])`；其他方言降级为分批多值 `INSERT` |
| 时间参数 | 统一传 UTC `time.Time`；禁止在 SQL 内混用 `now()` 与 Go 时间做业务比较 |
| 事务管理 | 通过 Unit of Work 统一管理，不在 Repository 层单独开事务 |
| 聚合优化 | 避免 `COUNT(*)` 全表扫描，使用预估或缓存 |
| 锁策略 | 优先使用乐观锁（`revision`），避免 `SELECT FOR UPDATE`；批量取件必须经 `Dialect.DequeueBatch(...)` |
| 更新语句 | 必须同时递增 `revision`、刷新 `updated_at`，并以 `revision` 作为 `WHERE` 条件 |
| UPSERT | 只写语义，由 `Dialect.Upsert(...)` 生成；PostgreSQL 下仲裁列必须与唯一约束/索引列集合一致，部分唯一索引必须原样重复谓词（见 3.4.2） |
| 软删除过滤 | 查询默认追加 `deleted_at IS NULL`（或走 `_active` 活跃唯一键） |
| 敏感字段加密 | 见第 14 章 |

### 3.10 种子数据规范（数据链）

> **范围**：本节约束 `db/seeds/V{N}__{主题}_{动作}.sql` 及其嵌入副本 `internal/bootstrap/seed_sql/`。数据链与结构链是两条独立版本链，账本主键 `(kind, version)`（ADR-022 决策 6 / 附录 A.2.2）。

**形态：人工编写的唯一事实源，无声明层、无生成器。** 结构链是"声明 → 生成物"，因为结构不变量（外键目标、索引列、RLS 列、回滚逆拓扑、方言一致性）可由声明静态推导；种子的不变量全部是执行期语义，声明层提供不了额外蕴含，只会引入第二份表示与第二个漂移面。因此 `db/seeds/` 下**只允许**版本化 `.sql`，出现 `*.yaml`/`*.yml`/`*.json` 等声明文件即门禁失败（`scripts/check-migrations.sh` 第 3 项）。

| 规则 | 说明 |
|------|------|
| 命名与版本 | `V{N}__{主题}_{动作}.sql`，`^V[0-9]+__[a-z0-9_]+\.sql$`；版本号在数据链内自 1 起连续，无空洞；与结构链版本号互不约束 |
| 头部契约行 | 脚本头部必须有且仅有一行 `-- @seed-chain: authored file={本文件名}`；`file=` 与实际文件名不一致即失败（防止改名后残留旧文件名） |
| 幂等 | 每条 `INSERT` 必须带 `ON CONFLICT … DO NOTHING`，仲裁列必须与真实唯一约束/索引列集合一致（部分唯一索引必须原样重复谓词，见 3.4.2）；`UPDATE` 必须自带收敛性 `WHERE`，重放第二次不得再改任何行 |
| 类别纯度 | 数据链只允许 DML（`INSERT`/`UPDATE`/`DELETE`），禁止任何 DDL；结构链反之（见 3.1） |
| 引用方式 | 主键为显式 UUIDv7 字面量（ADR-022 §7，主键无库函数默认值）；外键必须用子查询关联（`(SELECT id FROM … WHERE …)`），不得硬编码另一行的 UUID，也不得依赖 `RETURNING` 串联 |
| 副本一致 | `db/seeds/<name>.sql` 与 `internal/bootstrap/seed_sql/<name>.sql` 必须逐字节一致（SHA-256 门禁，第 5 项）；改动只允许改 `db/seeds/` 下的源文件，然后同步副本 |
| 执行与账本 | 数据链由 `AUTO_SEED`（或服务启动参数 `-seed`）控制，在结构链之后执行，记录写入 `guliangiam.schema_migrations` 的 `(kind='data', version=N)`；`checksum` 与镜像不一致即拒绝启动 |
| 不复用结构树 | 种子脚本不得声明表、索引、约束或 RLS；需要新结构时必须新增结构迁移，再在种子中引用 |
| 安全约束 | 种子不得包含生产密钥、真实凭据、有效证书、真实个人信息或任何**生产**引导口令；引导账号凭据由数据链 V1 写入开发哨兵 Argon2id 哈希，播种执行器在提供 `BOOTSTRAP_ADMIN_PASSWORD` 时以环境口令替换它；两种来源都必须携带强制首登改密标记（`user_credentials.need_update_password = true`），用户自定义的非空哈希不得被重放覆盖。允许在数据链 V1 内嵌「开发哨兵」Argon2id 哈希：仅用于本地/测试环境，必须同时满足（a）`need_update_password = true`；（b）`APP_ENV=production` 时执行器检测到哨兵即 fail-close，除非环境口令覆盖；（c）环境口令优先级高于哨兵；（d）哨兵明文只记录在 `AGENTS.md` 与部署/运维文档，不写入日志/响应。规则与实现见 `internal/bootstrap/migrate.go::SeedBootstrapAdminHash`，回归用例 `internal/bootstrap/seed_credential_test.go` |
| 镜像同步 | 改动只允许改 `db/seeds/` 下的源文件，随后运行 `bash scripts/sync-seed-mirror.sh` 同步 `internal/bootstrap/seed_sql/`；逐字节一致性由该脚本 cmp 与 `scripts/check-migrations.sh` 第 5 项双重校验 |
| 变更流程 | 内容变更直接改 `db/seeds/V{N}__*.sql`（禁止新增增量种子脚本改变历史语义），随后必须通过：`bash scripts/check-migrations.sh`，以及 `ALLOW_SCHEMA_RESET=1 DATABASE_URL=… BOOTSTRAP_ADMIN_PASSWORD=… bash scripts/dev/validate_schema_replay.sh`（真实 PostgreSQL 上连续重放两次证明幂等，并逐个解析全部 `ON CONFLICT` 仲裁目标） |

### 3.11 前瞻目标表规范：ReBAC / 计量 / 审批 / 风险 / 联邦

> **范围**：本节约束 [architecture/data-model.md §6「目标态数据模型声明」](architecture/data-model.md#6-目标态数据模型声明) 已申报的目标表。表名与列名以 `contracts/schema/*.yaml` 为唯一源（查询口径：`rg '^table_name:' contracts/schema/*.yaml`；复现与校验：`python3 scripts/gen_schema.py --check`），本节不复制表清单。前瞻目标域表在建成并迁移上线前，**禁止在 API/OpenAPI/文档中宣称其存在**，表名不得在 Schema 申报之外发明（与 [data-model §6](architecture/data-model.md#6-目标态数据模型声明) 红线一致）。公共列（`id`/`organization_id`/`revision`/时间列）沿用 3.3.3 与 3.1 约定。

| 维度 | 规范（建表时强制） | 锚点 |
|------|------------------|------|
| 租户列 | 除控制面注册表 `deployment_regions`（区域元数据，不承载个人数据）外，全部表必须含 `organization_id UUID NOT NULL` 并以租户列打头建复合索引；列名以 Schema 申报为准 | [data-model §6](architecture/data-model.md#6-目标态数据模型声明)；[outbox.md](design/infrastructure/outbox.md) |
| RLS | 建表 DDL 同事务提交 `ENABLE ROW LEVEL SECURITY` + `FORCE` + 策略模板 `USING (organization_id = current_setting('app.current_organization_id')::uuid)`（见 3.6.1）；前置条件 = `TenantFilter` 严格模式与事务级租户上下文（ADR-016）；RLS 为第二道防线，不替代应用层谓词 | [permissions.md §6.2 GUC 契约](design/iam/permissions.md#62-guc-契约)；ADR-016 |
| 分区 | `metering_events`（追加型高增长）初始即按**月**时间分区；`relation_tuples` 分区方向登记为**按 org 哈希**，启用前必须取得证据门槛（`EXPLAIN (ANALYZE, BUFFERS)` + 容量指标），未取证不分区 | [ADR-024](adr/ADR-024-rebac-relation-tuple-authz.md)；3.6 |
| 归档与 TTL | 追加型明细表（计量事件、执行流水与风险信号原文类列）建表时必须同时登记保留期 TTL 与归档表：先归档后删除，删除分批满足 19.4 清理约束；归档表结构 = 原表 + 归档时间戳，不参与业务查询主路径 | 19.4；[audit.md §数据模型](design/infrastructure/audit.md#数据模型) |
| 索引最小集 | 每表索引 = 主键 + 租户打头查询索引 + 唯一约束仲裁列，禁止超出 DESIGN 各章已申报最小集新增冗余索引；索引名与列集合以 `contracts/schema/*.yaml` 为准 | [ADR-024](adr/ADR-024-rebac-relation-tuple-authz.md)/[metering.md §数据模型](design/commerce/metering.md#数据模型)/[audit.md §数据模型](design/infrastructure/audit.md#数据模型)；3.6 |
| 外键与引用 | 核心强引用（身份池 → 联邦信任、动作版本 → 动作定义）建 FK；弱引用/跨域引用应用层校验（7.1 外键策略同口径）；身份池与联邦信任的删除预检必须覆盖池级授权与在途令牌级联 | [data-model §6](architecture/data-model.md#6-目标态数据模型声明)；[data-model §3.4 引用完整性](architecture/data-model.md#34-引用完整性外键-vs-逻辑引用) |
| 审计 | 策略/信任/配额/审批类表的写路径必须落 `records`（事件名以 `contracts/registries/events.yaml` 登记为准，如 `policy.changed` / `credential.*` / `org.settings.updated`，12.5）；`relation_tuples` 与业务行**同事务**写入并落审计（REBAC-010） | 12.5；[permissions.md §5 审计与追溯机制](design/iam/permissions.md#5-审计与追溯机制) |

---

## 4. 接口与 API 规范

> **本章小节索引**
>
> - **4.1** RESTful 设计规范：4.1.1 URL 与 HTTP 方法 · 4.1.2 统一响应与错误码 · 4.1.3 API 面划分（管理面 / 认证面 / 协议面） · 4.1.4 统一契约裁决表（跨节汇总）
> - **4.2** 接口版本化与弃用策略（`/api/vN`、`Deprecation`/`Sunset`、公告期）
> - **4.3** 接口文档要求（OpenAPI 3.1）
> - **4.4** 限流响应头（IETF `RateLimit-*` 与 `Retry-After`）
> - **4.5** JSON 命名与 DTO 映射权威规则
> - **4.6** 时间契约（RFC3339 / UTC / 非法格式 422）
> - **4.7** 分页契约（`page`/`pageSize` 与 HMAC 不透明游标）
> - **4.8** 排序与过滤统一语法（白名单、422、稳定排序）
> - **4.9** `Idempotency-Key` 幂等键规范
> - **4.10** ETag / `If-Match` 与 `revision` 乐观锁
> - **4.11** 批量操作（逐项状态与 207 Multi-Status）
> - **4.12** 异步任务（202 + `Location` + `Retry-After`）
> - **4.13** PATCH merge 语义（`null` 置空 / 缺省不变 / 数组替换）
> - **4.14** 422 与 400 分工、枚举开放扩展规则
> - **4.15** 错误消息 i18n（机器码 / `Accept-Language` / field 级错误）
> - **4.16** CORS 跨域规范
> - **4.17** CSP 与文件上传契约
>
> 4.5–4.17 为**通用 API 契约**细则，管理面与认证面必须遵守；协议面凡 RFC/OASIS 标准另有强制规定者（如 SCIM 的 `startIndex`/`count`、OAuth2 的 snake_case 字段名），以标准为准并在各节注明例外。每节均按「规则 → 示例 → 反例」组织。

### 4.1 RESTful 设计规范

#### 4.1.1 URL 与 HTTP 方法

| 操作 | HTTP 方法 | URL 模式 | 示例 |
|------|----------|---------|------|
| 列表 | GET | `/api/v1/{resources}` | `GET /api/v1/users` |
| 创建 | POST | `/api/v1/{resources}` | `POST /api/v1/users` |
| 获取单个 | GET | `/api/v1/{resources}/{id}` | `GET /api/v1/users/019abc` |
| 更新 | PATCH | `/api/v1/{resources}/{id}` | `PATCH /api/v1/users/019abc` |
| 替换 | PUT | `/api/v1/{resources}/{id}` | `PUT /api/v1/users/019abc/role-bindings` |
| 删除 | DELETE | `/api/v1/{resources}/{id}` | `DELETE /api/v1/users/019abc` |
| 自定义操作 | POST | `/api/v1/{resources}/{id}:{action}` | `POST /api/v1/orders/019abc:cancel` |
| 子资源 | GET | `/api/v1/{resources}/{id}/{sub}` | `GET /api/v1/webhooks/019abc/events` |

**URL 命名规则**：

| 规则 | 示例 | 说明 |
|------|------|------|
| 资源名用复数 | `/users`, `/organizations` | REST 标准 |
| 路径用小写+连字符 | `/captcha-providers` | 多词用连字符 |
| 嵌套资源不超过2层 | `/webhooks/{id}/events` | 超过则用查询参数 |
| 自定义操作用冒号 | `/orders/{id}:cancel` | Google API 风格 |
| 查询参数用 camelCase | `?organizationId=xxx&pageSize=50` | 前端友好 |


#### 4.1.1.1 路由字符串统一管理与防漂移

路由路径字符串若同时出现在注册、白名单与前端调用三处，必然漂移。因此必须满足以下三条纪律：

1. **声明式注册**（唯一事实源）：每个 api/v1/* 模块在 `routes.go` 里声明 `route.Definition.Pattern`。这是该端点路径字符串的唯一权威。chi.go 里直接 `router.Get("/api/v1/applications/resolve", …)` 这种硬编码**仅允许**用于根路由（`/.well-known/`、`/healthz`、`/uploads/*`）；其他所有 v1 端点都必须经 `routes.go` 声明、`RegisterChiRoutes` 消费。

2. **公开端点白名单必须用同一份字符串**：`internal/transport/http/api/v1/auth/auth_paths.go` 里的 `isTrulyPublicEndpoint` 白名单**必须与** `routes.go` 里的 `Pattern` **逐字符一致**。禁止用 `:resolve` 这种手写变体。该函数对外导出 `IsTrulyPublicEndpoint`，供合约测试消费。

3. **前端路径字符串禁止独立维护**：前端（`web/`）中凡是对 GuliangIAM 后端 REST 端点的引用，必须通过 `web/src/shared/api/paths.ts`（**前端唯一路径源**）获取常量或动态函数。禁止手写任何 `/api/v1/...` 字面量——既包括纯静态路径（如 `"/api/v1/payments"` → `PAYMENTS`），也包括带变量的动态路径（如 `` `/api/v1/orders/${id}` `` → `` `${ORDER_BY_ID(id)}` ``）。CI `paths.ts gate`（`.github/workflows/ci.yml`）阻塞合并时扫描全 `web/src` 生产代码（豁免 `.test.*` / `generated/` / `e2e/` / `paths.ts` 自身），发现任何硬编码即失败。

   判据：全 `web/src` 生产代码硬编码 `/api/v1/` 为 0；关联字段一律使用对应 Select 组件（组件清单以 `ls web/src/shared/ui/*Select.tsx` 为准），交互简化违规数为 0。

**防漂移合约测试**（`internal/bootstrap/public_whitelist_contract_test.go`）：

- `TestPublicWhitelist_NoRouteDrift` 从所有 `declaredRouteModules.routes()` 里枚举 Pattern，反向断言白名单能覆盖所有"看起来应该公开"的候选端点；同时反向检查白名单里声明的每个 truly public 路径在 routes.go 里真有声明。未来任何 routes.go 改动或白名单改动，两边不一致时此测试直接炸。
- `TestApplicationsResolve_IsTrulyPublic` 定向护栏：断言 `/api/v1/applications/resolve` 必须是 truly public，冒号变体 `/api/v1/applications:resolve` 必须返回 false。

**slug→UUID 类型安全**：数据库中 `organization_id` 是 `uuid` 列，但业务入口常以 slug 形式传入。禁止在 SQL 里直接 `$1::uuid` 强转（会触发 `invalid input syntax for type uuid: "built-in"`，SQLSTATE 22P02）。正确做法：
- 若参数是 slug → 调用 `guliangiam.resolve_org_id($1)` 函数（返回 uuid 或 NULL）；
- 若调用方已经有 UUID（service 层先解析好）→ 可以用 `$1::uuid`；
- 禁止让前端/HTTP handler 直接传 slug 字符串到 sql.go 的 `::uuid` 参数位。

见第 19 章 §5 的 SQL 参数绑定规则。

**`applicationId` 三态契约**：请求体的 `application` / `applicationId` 可能是**应用名称、slug 或 UUID**（登录页、SDK、SAML/OAuth 回调均如此）；而 `applications.sql.go` 的 `GetByID` 只接受 UUID。任何把原始引用直接传入 `WHERE id = $1::uuid` 的写法都会在 PG 侧抛 22P02，并可能把原始 SQL 错误泄漏为 `internal`。

**规则**：auth 应用层**禁止**把请求体里的 `applicationId` / `application` 直接传给 `Applications().GetByID`。凡接收该字段的**登录类**流程（登录、MFA challenge、社交登录、同意授权、Passkey/WebAuthn）都必须走 `resolveApplicationReference`：

| 引用形态 | 解析路径 |
|---|---|
| 合法 UUID（`platform/uuid.IsValid`） | `GetByID` |
| 应用 slug | `GetByOrganizationAndSlug(org.ID, slug)` |
| 组织 slug（如 `built-in`） | 回退该组织默认应用 `app-<orgSlug>` |
| 其余 | 400 `invalid_argument`（**绝不**把 PG 原文冒泡成 500） |

新增此类流程时必须复用该解析器，并补 `application_reference_test.go` 风格的用例。注意 `sessions.application_id` 与 `access_tokens.application_id` 都是 `uuid` 列——引用必须先解析成 UUID 再落库。无组织上下文的路径（如 SLO、IdP metadata）传 `nil` org，解析器自动退化为"仅接受 UUID"，非 UUID 直接 400。

> **单一入口**：`resolveApplicationReference` 是 auth 应用层解析应用引用的唯一入口；新增登录类流程或新仓储写入路径时，必须复用该解析器并补充 `application_reference_test.go` 风格的用例，禁止在流程内重复实现 slug/组织 slug 回退逻辑。

#### 4.1.2 统一响应与错误码

**成功响应**：

```json
// 单个资源
{ "id": "019abc...", "name": "example" }

// 列表
{ "items": [...], "page": { "page": 1, "pageSize": 50, "total": 100 } }

// 操作结果
{ "status": "ok", "msg": "operation succeeded" }
```

**错误响应**（唯一形态）：

```json
{
  "error": {
    "code": "not_found",
    "message": "resource not found",
    "details": { "field": "id", "reason": "invalid_uuid" }
  },
  "requestId": "019abc...",
  "traceId": "4bf92f3577b34da6..."
}
```

> **错误体契约（唯一事实源）**：字段名 = 顶层 `requestId`/`traceId`（camelCase，与 `X-Request-Id` 一致）；错误对象内为 `code`/`message`/`details`。`details` 是**键值对象**（可选，仅可定位到具体字段时返回）；`meta` 是 deprecated 同值别名，不得作为新契约读取。错误呈现层的实现必须与本契约逐字段一致。

**错误码清单**：唯一事实源为 `contracts/registries/errors.yaml`；渲染层为 `docs/generated/errors-registry.md`（复现与校验：`python3 scripts/gen_registry_md.py --check`）。本节不复制码表；新增码必须先在 `internal/platform/errors/errors.go` 定义常量，再在注册表登记。

> **HTTP 422 的定位**：422 `unprocessable_entity` 保留给「请求体语法合法但语义校验失败」（字段间逻辑矛盾、跨字段约束、状态机不允许的转移），与 400 `invalid_argument`「参数格式/类型错误」严格区分；两码的 HTTP 映射与使用场景以 `contracts/registries/errors.yaml` 为准。422/400 的完整判定边界与时间/分页/排序/枚举各场景的归属见 4.14。

#### 4.1.3 API 面划分（管理面 / 认证面 / 协议面）

本系统将 HTTP 接口按**服务对象与契约来源**分为三个面。三面共用同一进程、同一数据库、同一 OpenAPI 文件，**不拆进程、不拆域名、不拆库**；拆分的目的是让路由分组、中间件链、错误格式、限流组、指标标签、审计字段和 OpenAPI tag 可以**分别配置和验收**。

| 面 | 路径前缀 | 服务对象 | 契约来源 | 错误格式 | 限流组 | SLO（可用性） |
|---|---|---|---|---|---|---|
| **管理面** management | `/api/v1/*`，**排除**下方两面的路径 | 控制台与管理员 API 调用方 | 本项目自定 REST | `{error:{code,message,details},requestId,traceId}`（附录 C） | `RATE_LIMIT_GLOBAL_RATE/BURST`（默认 100/200） | 99.5% |
| **认证面** authentication | `/api/v1/auth/*` 中的**页面与会话类**端点（登录、注册、密码重置、MFA、验证码、会话、社交/Passkey 登录） | 本系统自带登录页与 SPA | 本项目自定 REST | 同上 | `RATE_LIMIT_AUTH_RATE/BURST`（默认 10/5，分钟）+ 防枚举（登录/注册/密码重置不得因账号存在与否返回不同状态码或时延） | 99.9% |
| **协议面** protocol | **多个前缀家族**： ① 根级 `/oauth2/*`（`/oauth2/register`、`/oauth2/register/{clientID}`、`/oauth2/userinfo`）；② 根级 `/.well-known/*`；③ 根级 `/cas/{organizationSlug}/{applicationSlug}/*`；④ 根级 `/scim/v2/*`；⑤ `/api/v1/auth/*` 中的**协议端点**（`authorize`、`tokens:exchange`、`device/authorize`、`device/token`、`oauth2/revoke`、`oauth2/introspect`、`webauthn/*`、`cas/*`、`saml/*`）；⑥ `/api/v1/saml/*`；⑦ `/api/v1/ai/faces/*`（AI 网关 OpenAI / Anthropic / Gemini 协议兼容面） | 外部标准客户端（RP、SP、IdP、SCIM 客户端、RFC 7591 DCR 客户端、LLM 工具客户端） | RFC / OASIS 标准**规定**的响应体 | 按标准：OAuth2 用 `error`/`error_description`/`error_uri`；SCIM 用 `schemas`/`detail`/`status`；其余面同管理面 | `RATE_LIMIT_TOKEN_RATE/BURST`（默认 30/10，分钟） | 99.9% |

**判读要点**：

1. 面的划分依据是**契约与消费者**，不是「是否需认证」。管理面与认证面都需要认证，差异在于消费者和限流策略。
2. **协议面没有统一的 `/oauth/*` 顶层前缀**（这是常见的对标误解）。协议端点分布在**多个前缀家族**中：

   | 家族 | 路径 | 注册位置 |
   |---|---|---|
   | ① 根级 `/oauth2/*` | `POST /oauth2/register`、`GET /oauth2/register/{clientID}`、`GET /oauth2/userinfo` | `dcr/chi.go`、`wellknown/chi.go` |
   | ② 根级 `/.well-known/*` | `openid-configuration`、`{application}/openid-configuration`、`jwks.json`、`{application}/jwks.json`、`webfinger`、`oauth-authorization-server`、`oauth-protected-resource`、`{application}/oauth-protected-resource` | `wellknown/chi.go` |
   | ③ 根级 `/cas/{organizationSlug}/{applicationSlug}/*` | `validate`、`serviceValidate`、`proxyValidate`、`proxy`、`p3/serviceValidate`、`p3/proxyValidate`、`samlValidate` | `cas/chi.go`（经 `bootstrap.RegisterProtocolRoutes` 注册于根路由） |
   | ④ 根级 `/scim/v2/*` | `Users`、`Users/{id}`、`Groups`、`Groups/{id}`、`ServiceProviderConfig`、`ResourceTypes`、`Schemas` | `scim/routes.go`；`bootstrap.RegisterProtocolRoutes` 的 `router.Mount("/", …)` |
   | ⑤ `/api/v1/auth/*` 内的协议端点 | `authorize`、`tokens:exchange`、`device/authorize`、`device/token`、`oauth2/revoke`、`oauth2/introspect`、`webauthn/*`、`cas/{login,serviceValidate,proxyValidate,proxy,logout}`、`saml/{redirect,metadata,acs}`、`kerberos/login` | `auth/chi_protocol.go` 的 `registerProtocolRoutes`、`webauthn/chi.go`、`saml/chi.go`、`kerberos/chi.go` |
   | ⑥ `/api/v1/saml/*` | `metadata/{applicationId}`、`sso/{applicationId}`（GET/POST）、`slo/{applicationId}`、`slo`（GET/POST） | `saml/chi.go`（经 `bootstrap.RegisterProtocolRoutes` 注册于根路由） |
   | ⑦ `/api/v1/ai/faces/*` | `{openai,anthropic,gemini}/chat/completions`、`{openai,anthropic,gemini}/embeddings`、`{openai,anthropic,gemini}/models` | `aigateway/chi.go`（协议面 API Key 鉴权，不走受保护会话链） |

   即**协议端点被拆在多个前缀家族中**，这正是需要「面」这一抽象层的原因——简单的路径前缀区间正则无法可靠判定面归属。若将来把协议端点搬到统一的 `/oauth/*` 顶层前缀，属**破坏性变更**，必须走 4.2 的版本策略。
   > **分类器唯一事实源**：`internal/platform/registry/surface.go` 的 `SurfaceFor(pattern)` 实现上述家族的判定（`internal/transport/http/route` 以类型别名桥接），并被 `internal/bootstrap/surface_contract_test.go` 的已知路径用例钉死；本节家族表、该函数、`route.Definition.Surface` 三者中的任何一处漂移都会让门禁变红。
3. 三面的分离是**显式契约**：路径前缀分组（三组注册）、按面绑定的中间件与限流组（`route.WithSurface`）、指标 `surface` 标签、审计 `records.surface` 列、OpenAPI tag 面前缀、以及本节这张对应表。错误格式按请求上下文的 `surface` 分派：协议面输出 RFC 6749/6750（OAuth2）与 RFC 7644（SCIM）规定的错误体，管理面/认证面输出应用错误体。逐项落地要求见下表。
4. 限流维度上，`RATE_LIMIT_SIGNUP_*`、`RATE_LIMIT_PASSWORD_RESET_*`、`RATE_LIMIT_OTLP_*`、`RATE_LIMIT_PER_IP_*` 是**路径级/子系统级覆盖**，与三面限流组叠加生效，不得相互替代。

**三面契约落地项（必须逐项满足）**：

| # | 落地项 | 位置 | 规范要求 |
|---|---|---|---|
| 1 | 路由三组注册（`r.Group` 或等价包装），不在同一 `RegisterAPIRoutes` 平铺 | `internal/bootstrap/chi.go` | 三组注册：`RegisterProtocolRoutes(protocol, …)`（根级协议族）、`authentication := protected.Group(nil)`（认证面）、`RegisterManagementRoutes(management, …)`（管理面，`route.WithSurface(SurfaceManagement)`）；`auth` 包内部按面拆为 `registerAuthenticationRoutes` 与 `registerProtocolRoutes`，两组各自绑限流策略 |
| 2 | 中间件链按面绑定 | `bootstrap/chi.go` + `transport/http/route/metadata.go` | `route.WithSurface(surface)` 中间件把面**显式**写入请求上下文，三组各自挂载；协议面绑 `mw.TokenEndpointRateLimit`（30/10 分钟）、认证面绑 `mw.AuthRateLimit`（10/5 分钟），与同表限流组列一致；`route.RequestMetadataMiddleware()` 兜底为根级协议路由按路径派生面 |
| 3 | 错误格式：协议面使用符合 RFC 6749 / RFC 7644 的专用写入器，不复用 `route.WriteError` | `internal/transport/http/route/oauth_error.go` | `WriteOAuthError` / `WriteProtocolError`（RFC 6749 §5.2、RFC 6750 §3：401 带 `WWW-Authenticate`、429 带 `Retry-After`、5xx 不回显内部消息）与 `WriteSCIMError`（RFC 7644 §3.12，含 `application/scim+json`）；`WriteError` 按请求上下文的面自动分派；应用错误码→协议错误码映射集中在 `oauthCodeByAppCode` |
| 4 | `route.Definition` 声明 `Surface` 字段 | `internal/transport/http/route/route.go` | 每条路由声明显式标注面，门禁 `surface_contract_test.go` 强制其合法且等于 `SurfaceFor(pattern)` |
| 5 | 指标带 `surface="management\|authentication\|protocol"` 标签 | 12.2 / `PrometheusMiddleware` | `http_requests_total` 标签为 `{method,path,status,surface}`，`http_request_duration_seconds` 为 `{method,path,surface}`；取值由 `route.SurfaceFor(routePattern)` 从**路由模板**派生，与三面契约门禁同源，标签基数受控 |
| 6 | 审计写入带 `surface` | `records` 表 + `infra/postgres/records` + `mapper.go` | `records` 表含 `surface`、`request_id`、`trace_id` 三列（结构以 `contracts/schema/60_audit.yaml` 为准，见 12.12）；写入链路 `transport（handler 取请求元数据）→ app（命令）→ domain（`Record`）→ infra` 全部承载三字段，`surface` 由服务端元数据填充、**不接受客户端传参**，`domain.NormalizeSurface` 兜底为 `management` |
| 7 | OpenAPI tag 前缀 `management:` / `authentication:` / `protocol:` | `scripts/gen_openapi.py` 的 `get_tag_name` / `TAG_MAP` | `gen_openapi.py` 解析声明中的 `Surface:` 字段并输出 `management: <module>` 形式；一个模块跨面时产生多个 tag（如 `auth` → `authentication: auth` + `protocol: auth`），tag 名含冒号故加引号 |
| 8 | 协议前缀单一化：CAS/SAML 权威族 = 根级家族 | `auth/chi.go`、`cas/chi.go`、`saml/chi.go` | CAS 权威前缀为根级 `/cas/{organizationSlug}/{applicationSlug}/*`；SAML 同口径。`/api/v1/auth/cas/*`、`/api/v1/saml/*` 等兼容前缀仅允许作为兼容期薄适配（弃用日志 + `Deprecation` 头），并按 4.2 弃用流程公告后停用；公开白名单必须与权威前缀一致 |

**不建议做的事**（保持与 [system-overview.md §3 物理部署视图](architecture/system-overview.md#3-物理部署视图)「第一阶段保持单体部署」一致）：不拆进程、不拆域名、不拆数据库、不拆 OpenAPI 文件。

#### 4.1.4 统一契约裁决表（跨节汇总）

下表汇总第 4 章通用 API 契约的唯一裁决，供三面（4.1.3）统一对照。**本表只做汇总，不重复展开细则**：单一展开位置仍为各细则小节；若本表与细则节或代码不一致，按 `generated/naming-registry.md` §4 维护规则以注册表裁决为准，并在同一变更中修正细则。

> 裁决详情见 `generated/naming-registry.md` §3（错误体三命名空间）与 §2（N11 限流头、N12 400/422 分工）。

| 契约项 | 统一裁决 |
|---|---|
| 错误体 | 三命名空间：管理面/认证面=`{error:{code,message,details},requestId,traceId}`；协议面=RFC 6749/6750 `{error,error_description}`、SCIM=RFC 7644（外部标准合规例外，不予合并）。细则见 4.1.2 / 4.1.3 |
| 分页 | 管理面/认证面统一 `page`（从 1 起）/`pageSize`（默认 20、最大 100、超限 422 不静默截断）；SCIM 按 RFC 7644 保留 `startIndex`/`count`；游标分页统一 `cursor`（不透明 + HMAC 签名 + TTL）/`limit`；所有 `ORDER BY` 追加 `id ASC` tiebreaker。细则见 4.7 / 4.8 |
| 幂等 | 写接口统一 `Idempotency-Key` 头：同键同体重放首响应并附 `Idempotency-Replayed: true`；同键异体 409；处理中 409 + `Retry-After`；幂等判定优先于乐观锁；渠道回调幂等（payments 通知等）为外部幂等语义，独立保留。强制范围、键作用域与 TTL 细则见 4.9 |
| 乐观并发 | 所有可编辑资源 GET 返回 `ETag: "<revision>"`；写操作强制 `If-Match`（缺失 428）；版本不符 412 并在响应体返回当前 `revision`。细则见 4.10 |
| 限流头 | 仅 IETF `RateLimit-Limit`/`RateLimit-Remaining`/`RateLimit-Reset`/`RateLimit-Policy`；`Retry-After` 在限流语境仅 429/503（异步任务 202、幂等处理中 409 的重试指引分别见 4.12 / 4.9）；废弃 `X-RateLimit-*`。细则见 4.4 <!-- naming-allow：本行仅声明该字面量已废弃，不得在代码或契约中使用 --> |
| 400/422 | 400 `invalid_argument`=报文畸形不可解析；422 `unprocessable_entity`=语法可解析但语义校验失败（`details.field` 携带字段级原因）。细则见 4.14 |
| 审计 | 所有写操作成功响应附 `auditId`；所有拒绝附 `requestId` + `traceId`。登记见 `generated/naming-registry.md` §1/§3 |

### 4.2 接口版本化与弃用策略

#### 4.2.1 版本规则

| 规则 | 说明 |
|------|------|
| 大版本路径前缀 | 管理面/认证面统一 `/api/vN/`（当前为 `/api/v1/`）；协议面路径由对应 RFC/OASIS 标准约束，不随业务大版本改名 |
| 同版本向后兼容 | 新增可选请求参数、新增响应字段、新增端点、新增枚举值（接收侧，见 4.14.3）不算破坏性变更，不得因此升版 |
| 破坏性变更 | 删除/重命名字段或端点、收紧校验、改变字段语义或错误码归属，必须发布新大版本 `/api/v{N+1}/`，新旧版本并行 |
| 版本生命周期 | `active` → `deprecated`（发 `Deprecation` 头/OpenAPI 标记）→ `sunset`（发 `Sunset` 头）→ 到期下线；**禁止无公告、无并行期的直接下线** |
| 契约标注 | 弃用端点必须在 OpenAPI 3.1 中加 `deprecated: true`，并在 `x-deprecated-at` / `x-sunset-at` / `x-successor` 扩展中写明日期与替代端点（扩展由 `scripts/gen_openapi.py` 透传） |
| 文档一致性 | 本手册禁止任何「零公告、零并行窗口即可下线旧版 API」的规则 |

#### 4.2.2 弃用公告头

弃用期内，**每个**被弃用端点的响应必须携带以下头：

| 响应头 | 规范来源 | 形态与示例 | 含义 |
|--------|---------|-----------|------|
| `Deprecation` | RFC 8594 + RFC 8941 结构化字段日期 | `Deprecation: @1798761600`（Unix 秒，sf-date） | 端点自该日起进入弃用期 |
| `Sunset` | RFC 8594 | `Sunset: Sat, 31 Mar 2027 23:59:59 GMT`（IMF-fixdate） | 端点预定下线时刻 |
| `Link` | RFC 8288 / RFC 9525 | `Link: <https://iam.example.com/api/v2/users>; rel="successor-version"` | 替代版本地址；无替代时给迁移文档 `rel="deprecation"` |

**公告期红线**：

1. 自**首次**发出 `Deprecation` 头（或等效公告）到 `Sunset` 日期，最短间隔 **≥ 6 个月**；涉及外部集成方（协议面、Webhook 消费方）的端点建议 ≥ 12 个月。
2. 下线前必须出具**遥测调用方清单**：按访问日志/网关指标统计过去 90 天仍调用该端点的 client_id、租户、用户代理与调用量；清单中仍有活跃调用方时不得下线，须逐户迁移确认。
3. 到期下线的处置顺序：先在预发环境灰度 410 `gone`（保留 1 个发布周期）→ 生产端点头按 `Sunset` 日期切换 410 与 JSON 错误体（`code:"gone"`，错误码注册表 §1 已有该码）→ 至少再保留 30 天 410 响应后才可移除路由。
4. 公告期内不得对被弃用端点做除安全修复外的行为变更。

**示例（弃用期响应头）**：

```http
HTTP/1.1 200 OK
Deprecation: @1798761600
Sunset: Sat, 31 Mar 2027 23:59:59 GMT
Link: <https://iam.example.com/api/v2/users>; rel="successor-version"
```

**反例**：

- ❌ v1 某字段「下个版本直接删掉」，无 `Deprecation`/`Sunset`、无并行 v2 窗口；
- ❌ 公告 2 周后即下线（短于 6 个月红线）；
- ❌ 仍有租户在调用，凭「已发公告」直接 404（正确：到期前 410 而非 404，且须先完成调用方清单确认）；
- ❌ 仅在 changelog 里宣布，响应头与 OpenAPI 标记均缺失。

#### 4.2.3 落地要求

- 4.2.2 全部头与流程必须落地；新增大版本时旧版本路由整体挂载于独立 router group，按面（4.1.3）分别公告。
- 弃用端点响应中三头齐备、公告期 ≥ 180 天可由配置/审计记录证明；调用方清单是下线 PR 的必备附件。

### 4.3 接口文档要求

| 要求 | 说明 |
|------|------|
| OpenAPI 3.1 | 所有 API 必须有 OpenAPI 规范文件 |
| 自动生成 | 通过 `scripts/gen_openapi.py` 从 `routes.go` 自动生成，确保 100% 对齐 |
| 嵌入式文档 | `GET /openapi/openapi.yaml` 和 `/openapi/openapi.json` |
| ETag 缓存 | OpenAPI 端点支持 ETag/If-None-Match 304 |
| 请求示例 | 每个端点必须有请求/响应示例 |
| 错误码文档 | 每个端点列出可能的错误码 |
| 鉴权标注 | 明确标注公开/需认证/需特定权限 |
| `x-permission` | 每个操作标注所需的 Casbin 权限字符串 |

### 4.4 速率限制响应头

#### 4.4.1 规范（IETF 标准头）

限流响应头以 IETF `draft-ietf-httpapi-ratelimit-headers`（`RateLimit-*` 结构化字段）为权威形态，`Retry-After` 遵循 RFC 9110：

| 响应头 | 形态 | 出现时机 | 说明 |
|--------|------|---------|------|
| `RateLimit-Limit` | 如 `RateLimit-Limit: 100`（结构化字段 integer/decimal；配额名策略见 `RateLimit-Policy`） | 成功与被限响应均应携带 | 当前窗口配额上限 |
| `RateLimit-Remaining` | `RateLimit-Remaining: 73` | 同上 | 窗口剩余额度 |
| `RateLimit-Reset` | `RateLimit-Reset: 42`（非负整数秒；或结构化字段 `@date`） | 同上 | 配额重置的相对秒数/绝对时刻，二选一并在 OpenAPI 固定 |
| `RateLimit-Policy` | `RateLimit-Policy: 100;w=60;burst=200;comment="global-ip"`（`w` 窗口秒数；`burst` 等于配额时可省略；`comment` 承载策略 id，见表下说明） | 同上 | 声明所引用的配额策略与维度 |
| `Retry-After` | `Retry-After: 60`（delta-seconds 纯整数） | **仅 429/503 响应** | 建议重试等待秒数；禁止 Go duration 串（`"1m0s"`）与 HTTP-date 之外的自定义格式 |

被限流时状态码**必须为 429 Too Many Requests**（错误码 `too_many_requests`，错误码注册表 §1），错误体仍走 4.1.2 统一结构；**禁止用 503 表达限流**（503 仅用于依赖故障/维护）。

**示例**：

```http
HTTP/1.1 429 Too Many Requests
RateLimit-Limit: 10
RateLimit-Remaining: 0
RateLimit-Reset: 38
RateLimit-Policy: 10;w=60
Retry-After: 38
Content-Type: application/json; charset=utf-8

{"error":{"code":"too_many_requests","message":"rate limit exceeded"},"requestId":"019abc..."}
```

**反例**：

- ❌ 限流被拒返回 `503 unavailable`（客户端会误判为服务宕路并熔断）；
- ❌ `Retry-After: 1m0s`（非 delta-seconds，标准客户端无法解析）；
- ❌ 限流头仅在被拒时出现（成功响应也应携带，客户端才能自适应退避）；
- ❌ 自定义 `X-Quota-Left` 等私有头而不提供标准头。

**多维度限流**：限流维度至少包含 IP / 租户 / 用户 / client（OAuth client_id）/ 端点族五层，各层独立计数并在 `RateLimit-Policy` 中用策略 id 区分；维度划分、默认配额、叠加仲裁与分布式存储选型见 18.5；环境变量清单见环境变量注册表 D.4，三面限流组见 4.1.3。

**策略 id 与 RFC 9331 的偏差**：RFC 9331 将 `comment` 定义为人类可读备注，本系统将其用作机器可读的维度标识（如 `global-ip`、`auth-ip`、`service-ip:{路由模板}`），以便多维叠加时能归因到具体某一维；严格按 RFC 解析的客户端忽略该参数不影响 `quota`/`w`/`burst` 的解析。策略 id 字符集限 `[a-z0-9.:/-]`，**禁止**写入用户 ID/租户 ID/IP 等高基数值。策略 id 全量清单见 `generated/limit-registry.md` §3。

#### 4.4.2 实现要求

限流必须由**单一**中间件家族实现：全部维度统一输出 4.4.1 标准头；被拒状态码一律 429；`Retry-After` 一律 delta-seconds 整数；成功响应同样携带配额头。**头的写入点唯一**：所有限流中间件必须调用 `internal/platform/ratelimit`（`Dimension.Policy()` / `Write()` / `WriteRetryAfter()`），禁止在中间件内自行 `Header().Set("RateLimit-…")`——字段名、单位与折算逻辑多写一份就多一处漂移。低于 1 req/s 的配额必须折算到整分钟窗口呈现（`quota;w=60`）：按秒截断会把 `10 req/min` 报成 `0`，客户端读到的语义是「不允许任何请求」。`X-RateLimit-*` 一律不得出现（命名裁决 N11 已废弃，**不设过渡别名期**）。配额参数（`RATE_LIMIT_*`）维持环境变量注册表 D.4 单一登记口径，不得新增未经附录 D 登记的变量。 <!-- naming-allow：本行仅声明该字面量已废弃，不得在代码或契约中使用 -->

### 4.5 JSON 命名与 DTO 映射权威规则

#### 4.5.1 规则

1. **管理面 / 认证面**的请求体、响应体、查询参数与 JSON 字段名**一律 lower camelCase**（如 `organizationId`、`pageSize`、`lastSigninAt`、`isAdmin`）；查询参数命名与 4.1.1 一致。
2. **边界映射唯一权威**：数据库 `snake_case` 列名**不得**出现在 API JSON 中；转换只允许发生在 infra 仓储 mapper 层（`internal/infra/postgres/*/mapper.go`）。domain/app 层为 Go PascalCase 字段，transport DTO 必须用**显式 `json` tag** 声明线上名称，禁止依赖编码库默认字段名推导。
3. **协议面例外（标准优先）**：协议面字段名以 RFC/OASIS 为准，不适用 camelCase 规则——OAuth2 令牌/吊销请求用 snake_case（`grant_type`、`client_id`、`token_type_hint`），OIDC discovery 文档用 snake_case（`authorization_endpoint`、`scopes_supported`），SCIM (RFC 7644) 用其自定的 camelCase（`startIndex`、`itemsPerPage`）。
4. **字段形态约定**：

| 字段类别 | API 形态 | 说明 |
|----------|---------|------|
| ID / 外键 | UUIDv7 字符串 | 主键、租户、外键均为 UUID 字符串，禁止数字自增 ID（3.2/ADR-007） |
| 时间 | RFC3339 字符串 | 细则见 4.6 |
| 枚举 | 小写或领域固定大小写的**字符串** | 值集闭集登记，未知值策略见 4.14.3 |
| 金额 | **十进制字符串**（如 `"19.9900"`）或最小货币单位整数 + 币种字段 | 禁止 JSON 浮点 number；库内 `DECIMAL(19,4)` + `VARCHAR(3)` 币种（3.3） |
| 布尔 | `isXxx` / `hasXxx` camelCase | 与 DTO 命名一致 |
| 列表包装 | `{ "items": [...], "page": { "page", "pageSize", "total" } }` | 见 4.7 |
| 空值 | `null` 仅用于显式表达「无值」；可空字段加 `omitempty` | PATCH 语义见 4.13 |

#### 4.5.2 示例与反例

```json
// ✅ 正确：camelCase、UUID 字符串、RFC3339、金额字符串
{
  "id": "0193a7e2-7a3b-700c-9f2a-1b9c0a4d7e11",
  "organizationId": "0193a7e2-0000-0000-0000-000000000001",
  "displayName": "示例企业",
  "isAdmin": false,
  "lastSigninAt": "2026-09-14T08:30:00Z",
  "balance": { "amount": "1024.5000", "currency": "CNY" }
}
```

```json
// ❌ 反例：DB 列名泄漏（snake_case）、金额用浮点、ID 用自增数字
{ "organization_id": 1, "is_admin": false, "last_signin_at": "2026/9/14 8:30", "balance": 1024.5 }
```

#### 4.5.3 落地要求

- **金额字段**：API 上必须为十进制字符串（如 `"19.9900"`）或最小货币单位整数，并附 `currency` 字段；**禁止 JSON 浮点 number**（库内 `DECIMAL(19,4)` + `VARCHAR(3)` 币种，3.3）。金额改型属破坏性变更，须按 4.2 走 `/api/v2` 或显式响应协商，不得在 v1 直接改型。
- 新增端点的 DTO 评审默认套用本节字段形态表；OpenAPI 生成器对 snake_case 管理面字段、number 型金额字段报错（门禁见附录 I）。

#### 4.5.4 前端 Zod schema 必须覆盖后端所有声明字段

后端 DTO 与前端 Zod schema 构成**契约两面**：后端是权威，前端必须完整镜像。

**Zod `.object()` 默认行为是剥离未声明的未知字段**（不是拒绝），因此前端 schema 缺失字段时会发生**静默数据丢失**——后端正常返回、前端 parse 正常通过、但数据字段被悄悄丢弃。

**触发条件**：后端 mapper.go 已正确映射 DB 字段到 DTO（如 `organization.logo_url` → `LogoUrl`）、DTO 有正确的 `json:"logoUrl"` tag、但前端 `z.object()` 里没声明这个 key。

**强制规则**：

1. 后端 DTO 每新增一个对外 JSON 字段，前端对应的 Zod schema（`web/src/shared/**/*.ts`）必须在**同一 commit**里补齐对应 `.zodType` 声明
2. 新增字段一律 `.optional().nullable()`（允许后端遗漏时不致 parse 失败）
3. **禁止**用 `(payload as Record<string, unknown>)["fieldName"]` hack 从 parse 后的对象里"救"被剥离的字段——这是 schema 未对齐的症状，修 schema 才是根治
4. 每次跨域接口（auth/account、organizations、users 等）变更后端字段后，**必须 curl 实证**前端 parse 结果包含新字段
5. `pnpm tsc --noEmit` 通过不代表 schema 对齐——TypeScript 不检查 Zod 运行时行为

**典型反例**：

后端 `SessionOrganization` DTO 一直有 `LogoUrl` / `FaviconUrl`，mapper 也正确填充；但前端 `authSessionOrganizationSchema` 未声明这两个 key → Zod parse 剥离 → 前端 store 永远拿到 `logoUrl: null` → Header brand Avatar 一直用首字母 fallback，favicon useEffect 永远不触发。此类缺口编译期不可见，只能靠同 commit 补齐 schema + curl 实证拦截。

### 4.6 时间契约

#### 4.6.1 规则

1. **出参**：所有时间字段输出 **RFC 3339 / ISO 8601 字符串，统一 UTC（尾缀 `Z`）**，如 `2026-09-14T08:30:00Z`；不输出 Unix 秒/毫秒、不输出 `2026/9/14`、不带本地时区缩写。
2. **入参**：接受 RFC3339 字符串（允许带偏移，如 `2026-09-14T16:30:00+08:00`，服务端归一化为 UTC 后写入 `TIMESTAMPTZ`）。
3. **非法格式 / 不可解析 / 偏移错误 → 422**；时间区间 `start > end` 等跨字段矛盾同为 422。
4. 纯日期（如生日）用 `YYYY-MM-DD`，纯无时区时间（如每日 02:00 定时）须在端点文档中单独声明；除此之外一律带时区。
5. 服务端禁止信任客户端传入的「当前时间」，当前时刻只取自 `clock.Clock`（1.1.7）。

```http
# ✅ 正确
GET /api/v1/records?startAt=2026-09-01T00:00:00Z&endAt=2026-09-14T00:00:00Z
# ❌ 反例：Unix 秒、本地化无偏移格式
GET /api/v1/records?startAt=1756656000&endAt=2026-09-14 08:00
```

#### 4.6.2 落地要求

- 响应在 presenter 层统一 `UTC().Format(time.RFC3339)` 输出并写入 OpenAPI 契约（固定秒级精度，去掉 Nano 尾数），全部出参样例以 `Z` 结尾，不依赖数据库会话时区。
- 非法时间入参返回 422 且有 field 级错误；入参解析统一使用 `time.Parse(time.RFC3339, ...)`。

### 4.7 分页契约

#### 4.7.1 页码分页（管理面默认）

| 规则 | 约定 |
|------|------|
| 参数 | `page`（从 1 开始，默认 1）、`pageSize`（默认 **20**，最大 **100**） |
| `page < 1` 或 `pageSize < 1` / 非整数 | **422**：`PaginationMiddleware` 在受保护组上统一校验，**参数存在即必须可用**；缺省不报错（那是文档默认值） |
| `pageSize > 100` | **422**，不得静默截断。响应体在 `details` 中带 `{field, value, max}`，便于前端定位到具体输入 |
| `page` 超出总页数 | **200 + 空 `items`**，`page.total` 照常返回（不 404，便于并发数据变更下的稳定翻页） |
| 响应体 | `{"items":[...],"page":{"page":1,"pageSize":20,"total":100}}` |
| 服务端取值 | 仓储层一律经 `shared.ClampPageSize` / `shared.PageOffset`（唯一实现 `internal/platform/pagination`）：`≤0` → 20、`>100` → 100，且 **LIMIT 与 OFFSET 用同一个值**；禁止各仓储自行实现页参数裁剪（否则会出现「地板不是天花板」与 `pageSize ∈ [1,19]` 时相邻页窗口重叠） |
| 深翻页 | `page*pageSize > 10000` 时建议（非强制）改用游标分页，响应中可带 `page.hasNext` |

```http
# ✅
GET /api/v1/users?page=2&pageSize=50
# ❌ 反例：snake_case 参数名、无上限页大小
GET /api/v1/users?page_num=2&page_size=99999
```

#### 4.7.2 游标分页（大流量列表）

| 规则 | 约定 |
|------|------|
| 参数 | `cursor`（不透明字符串，首次请求不传）+ `limit`（上限同为 100） |
| 响应 | `{"items":[...],"nextCursor":"...","hasMore":true}`；不承诺返回 `total` |
| 不透明性 | 游标为服务端签发的 **HMAC-SHA256 签名 + Base64URL** 令牌，客户端**不得解析、构造或持久化跨会话复用** |
| 签名密钥 | `CURSOR_HMAC_KEY`（环境变量注册表 D.1），生产**必填**，缺失即拒绝启动；禁止内置默认值回落；多实例共享同一密钥以支持跨实例重放游标 |
| 篡改 / 签名失效 / 载荷非法 / 过期 | **422** |
| 有效期 | 默认 **1 小时**，经 `CURSOR_TTL_SECONDS` 配置（环境变量注册表 D.12） |
| 稳定性 | 游标载荷含排序键值 + `id`，翻页等价于 keyset pagination，天然避免 OFFSET 漂移 |
| 接线范围 | 大流量列表（审计、事件、Webhook 投递记录）必须提供游标分页 |

```http
GET /api/v1/audit-events?limit=50&cursor=eyJwayI6...<HMAC>
```

#### 4.7.3 落地要求

- 页码分页参数为 camelCase `page`/`pageSize`，响应包装与本节同形；游标原语唯一实现位于 `internal/platform/pagination`。
- 协议面例外：SCIM 端点使用 `startIndex`/`count`，属 RFC 7644 强制口径。

### 4.8 排序与过滤统一语法

#### 4.8.1 规则

1. 排序参数为 **`sortField` + `sortOrder`**：`sortOrder ∈ {asc, desc}`，缺省 `asc`；多字段排序支持逗号列表（`sortField=createdAt,id&sortOrder=desc,asc`）。
2. **白名单制**：每个列表端点显式登记「API 字段名（camelCase）→ 物理列」映射；**任何不在白名单的字段 → 422**，禁止把查询值拼进 SQL（SQL 注入红线，也见 3.9）。
3. **稳定排序**：任何 `ORDER BY` 末尾必须追加唯一键 tiebreaker（`ORDER BY <field> <dir>, id ASC`），保证分页/重试时行序确定。
4. 过滤：等值过滤直接用参数（`?status=active`），多值用逗号分隔（`?status=active,pending`）；自由文本用 `keyword`/`search`；复杂表达式只在协议面按 SCIM RQL `filter` 表达（已有解析器 `internal/app/scim/filter.go`）。
5. 非法枚举值、越界数值、非法排序方向 → 422。

```http
# ✅
GET /api/v1/users?sortField=createdAt&sortOrder=desc&status=active
# ❌ 反例：未白名单字段（且指向敏感列）、自造排序语法
GET /api/v1/users?sortField=password_hash&sortOrder=rand()
```

#### 4.8.2 实现要求

- 白名单规格唯一实现于 `internal/platform/sorting.Spec`（**API camelCase → 物理列**），每个列表端点登记一份；禁止 value 恒等于 key 的伪映射。
- 白名单外字段与非法方向 → **422**，错误体在 `details` 中列出允许集合；禁止静默回退 `ORDER BY created_at DESC`。
- 所有列表 `ORDER BY` 末尾强制追加 `, id ASC` tiebreaker；列表语句的 ORDER BY 由 `{ORDER_BY}` 槽位承载，不得硬编码。
- **SQL 注入红线**：插值进 SQL 的只有白名单映射出的列名与 `ASC`/`DESC` 两个字面量，请求值从不进入语句文本（`Spec.Resolve` 拒绝一切白名单外取值）。槽位用**命名 token** 而非 `%s`：含 ILIKE 通配符等字面 `%` 的语句经 `fmt.Sprintf` 会把 `%` 当动词并破坏模式。

### 4.9 `Idempotency-Key` 幂等键规范

#### 4.9.1 规则

1. **全部写方法统一**（POST/PUT/PATCH/DELETE，ADR-018「统一幂等」口径）：写请求客户端发送请求头 `Idempotency-Key: <客户端生成的 UUIDv7/ULID>`；服务端对全部写端点接受并按语义处理——非天然幂等端点（创建订单、发起支付、批量写、触发 Webhook 等）**强制要求**（缺失返回 428 Precondition Required）；天然幂等端点（按完整字段的 PUT、条件 DELETE）接受但可选。GET 请求不使用。
2. **键作用域**：唯一键为 `(scope, idempotency_key)`，`scope` **必须含租户 + 主体**——用户令牌为 `{面或资源}:{organizationId}:{userId}`，客户端凭据为 `{面或资源}:{organizationId}:{clientId}`（与 3.7.1 及 DDL `uq_idempotency_keys_scope_key` 一致），防止跨租户/跨主体碰撞。
3. **TTL = 24 小时**（与 `idempotency_keys.expires_at` 默认 `now() + INTERVAL '24 hours'` 一致；19.4 规定到期物理删除）；过期后同键视为新请求。
4. **请求指纹**：服务端对规范化后的请求体计算 `request_hash`；判定流程：

| 情形 | 响应 |
|------|------|
| 首次请求 | 插入 `in_progress` 记录（唯一约束仲裁并发），处理完成后回写 `succeeded` + `response_status` + `response_body` |
| 同键**首次请求仍在处理中**（并发/快速重试） | **409 Conflict** + `Retry-After: <建议秒数>`，错误码 `conflict` |
| 同键且 `request_hash` **相同**、已完成 | **重放原响应**：原封返回首次的状态码、响应体与 `Content-Type`（不重新执行业务） |
| 同键但 `request_hash` **不同** | **409**（键被复用为不同请求，同键异体，错误码 `conflict`） |
| 键格式非法（非 UUID/超长） | 422 |

5. **与 `If-Match` 的判定顺序**：认证鉴权 → 幂等键判定（命中已完成直接重放、处理中 409、键体冲突 409）→ `If-Match`/`revision` 乐观锁校验（4.10）→ 业务执行。即幂等重放优先短路，不再重复校验版本前提。
6. 幂等键只保护**请求级**副作用；业务列上的 `idempotencyKey`（订单/支付/Webhook 事件）是**外部渠道流水号去重**，两者不互相替代。

```http
POST /api/v1/orders
Idempotency-Key: 0193a7e2-7a3b-700c-9f2a-1b9c0a4d7e99
Content-Type: application/json
```

```
# ❌ 反例：服务端用 key 做「存在即返回成功但不重放原响应」；跨租户共享同一键空间；把幂等键当业务唯一约束在应用表上做 upsert
```

#### 4.9.2 落地要求

- 统一幂等机制 = 幂等中间件 + `idempotency_keys` 仓储（UPSERT 仲裁遵循 3.4.2），完整落实 4.9.1 第 4 条状态机；验收＝并发同键首请求返回 409+`Retry-After`、重放响应字节与首次一致、键同体异返回 409、跨租户键不可互见。
- 物理表 `guliangiam.idempotency_keys`（见 3.7.1）与业务列 `idempotencyKey` 的分工见 4.9.1 第 6 条。
- 到期记录由定时任务每小时分批物理删除（清理行数与耗时暴露为指标），清理失败不阻塞请求路径。

### 4.10 ETag / `If-Match` 与 `revision` 乐观锁

#### 4.10.1 规则

| 场景 | 状态码 / 行为 |
|------|--------------|
| `GET` 成功 | 返回 `ETag`（强验证器，带引号）；条件 `If-None-Match` 命中返回 **304** |
| `PUT`/`PATCH`/`DELETE` 携带 `If-Match: "<etag>"`（或端点约定的 `revision`）且匹配 | 正常执行，返回 200/204 与**新** `ETag`/`revision` |
| 写请求**要求**乐观锁但未携带前提头 | **428 Precondition Required** |
| `If-Match` 与当前版本不符 | **412 Precondition Failed**，不执行业务；响应体指明当前 `revision` |
| 唯一约束等非版本冲突 | 仍为 **409**（412 只用于可由客户端前提避免的版本冲突） |
| **弱 ETag** `W/"..."` | 仅表示语义等价，可用于 `If-None-Match` 缓存，**禁止**用于 `If-Match` 并发控制 |

`revision`（`BIGINT`，从 1 起，每次更新 +1）是乐观锁事实源；ETag 是它的 HTTP 表达。读响应同时给 `ETag` 与 JSON `"revision"`，写请求二选一提交（推荐 `If-Match`），服务端统一折算为 `WHERE id=$1 AND revision=$2 ... RETURNING revision`。

```http
PATCH /api/v1/users/0193...
If-Match: "3"
```

```
# ❌ 反例：用弱 ETag 做 If-Match；版本冲突返回 409 让客户端无法区分「可重试的前提失败」；无 revision 时默认「最后写入获胜」
```

#### 4.10.2 落地要求

- GET 侧强 ETag 缓存中间件（`internal/transport/http/middleware/etag.go`）：对 2xx body 做 HMAC-SHA256，输出**带引号的强 ETag**，`If-None-Match` 命中改 304；仅作用于 GET、跳过写路径；`ETAG_HMAC_KEY` 生产强制显式配置（多实例共享），禁止缺省随机回落。
- 乐观锁 SQL 口径：`WHERE id=$n AND revision=$m RETURNING revision`；零行更新统一转为 **409** `conflict`（`infra/postgres/shared/optimistic_lock.go`）。
- 写前提头 412/428 与 SQL 乐观锁接通；ETag 与 `revision` 建立确定性映射，使 GET ETag 可直接用于写前提。

### 4.11 批量操作

#### 4.11.1 规则

1. 批量端点接收 `operations` 数组，**单次上限 200 项**；超上限返回 422；空数组返回 200/207 且 `results` 为空数组。
2. **逐项独立**：默认每项独立成功/失败、互不共享事务；响应使用 **207 Multi-Status**，每项至少含 `{index/status/code/body|error}`，单项错误不影响其他项。
3. 需要原子性的场景必须由**另一个显式端点**提供（请求体带 `atomic:true` 语义或独立资源），全成功返 200、任一失败整体回滚返 4xx，不得在 207 中混用「声称原子」。
4. 批量端点必须支持 `Idempotency-Key`（4.9）；端点自身写操作仍逐项走乐观锁（4.10）。
5. 并发度由服务端控制（保护下游），请求中不接受「并发度」参数。

```json
// ✅ 响应（HTTP 207 Multi-Status）
{ "results": [
  { "index": 0, "status": 201, "body": { "id": "0193...a" } },
  { "index": 1, "status": 409, "error": { "code": "already_exists", "message": "..." } }
] }
```

```json
// ❌ 反例：整体恒 200，失败项只用字符串 "status":"error"，无逐项 HTTP 状态码
```

#### 4.11.2 落地要求

- 批量端点（`POST /api/v1/batch/{users,organizations,applications}`，`internal/transport/http/api/v1/batch/`）必须满足本节全部规则：上限 200、207 Multi-Status、逐项标准 HTTP 状态码与统一错误体（4.1.2）、原子批量走独立端点；既有响应形态调整属破坏性变更时按 4.2 公告。
- Excel 导入类上传（`groups`/`roles`/`permissions`，multipart）是另一类批量入口，其结果报告同样适用逐项状态要求。

### 4.12 异步任务契约

#### 4.12.1 规则

1. 不能在请求生命周期内完成的操作（导入导出、批量同步、报表生成、LDAP 同步）返回 **202 Accepted**：

```http
HTTP/1.1 202 Accepted
Location: /api/v1/jobs/0193a7e2-7a3b-700c-9f2a-1b9c0a4d7e21
Retry-After: 5
Content-Type: application/json

{ "jobId": "0193a7e2-...", "status": "queued" }
```

2. 客户端按 `Retry-After` 轮询 `Location`：运行中返回 **200**（可继续带 `Retry-After`），体为 `{status: queued|running|succeeded|failed, progress?, result?, error?}`；终态保留 200；任务过期清理后返 404（或按 4.2 公告期返 410）。
3. **禁止**：无 `Location` 的 202（客户端无法追踪）；202 但实际已同步执行完业务；用 200 + 「稍后处理」消息代替 202。
4. 任务终态与产物保留期限由 19.4 数据生命周期规定；长任务必须可幂等重试（接 4.9）。

#### 4.12.2 落地要求

- 异步作业资源模型（`/api/v1/jobs/{id}`）、统一 202 头契约、进度与终态查询必须实现；验收＝任一长任务端点满足「202 + Location + Retry-After」三件套且可轮询到终态。
- 埋点接收类端点（如 Web Vitals 上报）不适用 202 契约，按普通 2xx 处理。

### 4.13 PATCH merge 语义

#### 4.13.1 规则

1. `PATCH` 请求体按 **JSON Merge Patch（RFC 7396）语义**解释，`Content-Type` 为 `application/merge-patch+json`（现行兼容 `application/json`）；类型不符返 415。
2. **字段缺省 = 不变；显式 `null` = 置空**（仅限该字段在模型上可空）；对不可空字段置 `null` 返 422。
3. **数组 = 整体替换**（不做按下标/按 id 的增量合并）；嵌套对象按 key 合并；空对象 `{}` 为无变更，返 200/204。
4. 未声明字段拒绝（当前 `DisallowUnknownFields`，见下）；PATCH 必须配合 `revision`/`If-Match`（4.10）。

```json
// 对 { "displayName":"A", "tag":"vip", "scopes":["read"] }
{ "tag": null, "scopes": ["read","write"] }
// 结果：displayName 不变；tag 被置空；scopes 整体替换为两个元素（不是追加）
```

```json
// ❌ 反例：客户端漏传 scopes 却被服务端当成「清空数组」；或传 null 想表达「不改」
```

#### 4.13.2 落地要求

- PATCH DTO 必须能区分「字段缺省」「显式 `null`」「赋值」三态（在场标记，如 `json.RawMessage`、可空包装类型或 RFC 7396 合并器）；禁止以「指针 + `omitempty`」表达 PATCH 语义（`nil` 指针无法区分缺省与置空）。
- JSON 解码侧 `route.DecodeJSON` 启用 `DisallowUnknownFields`（`internal/transport/http/route/http.go`）；未知字段与不可空字段置空返回 422。
- 数组整体替换有领域先例（角色/身份绑定 delete-all-then-insert，见 1.1.3 仓储方法约定）。

### 4.14 422 与 400 分工、枚举开放扩展规则

#### 4.14.1 状态码分工

| 状态码 | 语义 | 典型场景 |
|--------|------|---------|
| **400** Bad Request | 请求**畸形或不可解析**，服务端无法据以形成字段级判断 | JSON 截断/括号不匹配、类型根本无法解码（字符串传到了期望结构的位置）、`Content-Type` 与体不符 |
| **415** | 不支持的媒体类型 | PATCH/POST 非 JSON 媒体类型（4.13） |
| **422** Unprocessable Entity | 语法可解析、结构可绑定，但**语义校验失败** | email/UUID/RFC3339 格式非法（4.6）、`pageSize>100`/游标篡改过期（4.7）、非法排序字段（4.8）、枚举闭集外取值、跨字段矛盾（起止时间倒置）、状态机非法转移 |
| 409 / 412 / 428 | 冲突与前提 | 版本前提见 4.10；幂等键同键异体 409 见 4.9 |

> **422 落地口径**：`unprocessable_entity → 422` 已在 `internal/platform/errors` 与错误码注册表 §1 登记。右栏场景按 422 返回：分页越界/非整数（`PaginationMiddleware`）、排序字段不在白名单或方向非法（`platform/sorting`）、provider 类型闭集外取值（`internal/platform/enumset`）；其余业务枚举的闭集校验与跨字段矛盾按 4.14.3 落地。

#### 4.14.2 示例与反例

```http
# 400：体根本不是 JSON
POST /api/v1/users
Content-Type: application/json
{"username": "abc",

# 422：可解析但语义非法
POST /api/v1/users?pageSize=99999
{"userType": "not-a-real-type", "status": "active"}
```

- ❌ 反例：把所有校验失败一律 400，客户端无法区分「重试无用的畸形报文」与「改个字段就能成功的语义错误」；
- ❌ 反例：服务端对非法字段静默回退默认值——分页越界与非白名单排序字段必须返回 422，并在 `details` 中给出字段名与允许集合。

#### 4.14.3 枚举的开放扩展规则

1. **请求方向（入参闭集）**：枚举字段只接受服务端登记值集内的取值，**未知值拒绝（422）**，不得静默落库、不得悄悄归一化为默认值。
2. **响应方向（出参开放）**：服务端可以随时**新增**枚举值（属向后兼容变更，不升版本，见 4.2）；客户端必须对未知枚举值容错（走「未知」兜底展示而非白屏/崩溃），Web 端 Zod/switch 必须有 default 分支。
3. 协议面枚举扩展受对应 RFC 约束（如 OAuth2 `error` 码、SCIM `schema`），以标准为准。
4. **闭集校验共享实现**：统一使用 `internal/platform/enumset`（`Set` 值对象：`Validate` / `ValidateRequired` / `ValidateOptional` / `ValidateOptionalRequired`，未知值一律 **422** 并携带 `{field,value,allowed,set}`，**不做大小写折叠**——静默归一化会让错误写入变得不可见）。所有业务枚举（含 `users`/`applications`/`organizations` 与通用 `providers` 表的 `(category, providerType)` 复合值域）必须接线 `enumset`；值集必须是运行时工厂的注册键（`internal/bootstrap/runtime.go`）；复合值域采用 `map[category]Set` 设计，分支判定使用精确匹配，禁止 `strings.ToLower` 归一化后再分支。枚举值集的唯一事实源为 `contracts/registries/states/*.yaml`（按域拆分，如 `states/users.yaml`、`states/applications.yaml`），`enumset.NewSet` 必须从该注册表同步生成，禁止代码硬编码值集。未在服务端注册的构造器能力不得纳入闭集（纳入等于承诺服务端做不到的能力）。
5. **跨语言一致性**：`scripts/check-enums.py` 阻塞校验——服务端 `NewSet` 与控制台 `@enumset` 清单必须**逐值双向相等**；每个服务端闭集要么有带 `@enumset` 标注的前端清单，要么在脚本的 `HEADLESS_SETS` 中登记理由，一个未被消费的声明本身就是缺陷。

### 4.15 错误消息 i18n

#### 4.15.1 规则

1. **`error.code` 是稳定的英文蛇形机器码**（唯一事实源为 `contracts/registries/errors.yaml`，渲染层见附录 C）：禁止本地化、禁止改变已有码含义、禁止用 code 携带句子或参数；面向机器的分支判断只依赖 code。
2. **`error.message` 按 `Accept-Language` 本地化**：服务端从请求头匹配第 16 章的 14 个 locale，匹配不上回落 `en`；无 `Accept-Language` 时对外 API 默认英文、控制台同源场景按会话语言偏好。message 面向人类排障，允许含参数但不得含密码/Token 等敏感信息（1.1.6 日志红线同样适用）。
3. **field 级错误详情对象**：语义校验失败在 `error.details` 中以键值对象返回；字段级原因用 `field` 键标识目标字段，其余键（如 `reason`/`value`/`max`/`allowed`）为稳定机器码或原始约束值：

```json
{ "error": {
    "code": "invalid_argument",
    "message": "请求参数校验失败",
    "details": { "field": "pageSize", "reason": "out_of_range", "max": 100 }
  },
  "requestId": "0193a7e2...", "traceId": "..." }
```

4. `reason` 同样是稳定英文机器码（`required`/`invalid_email`/`out_of_range`/`enum_not_allowed`/...）；前端 i18n 用 `code`/`details.field`/`details.reason` 自行查表（第 16 章 `resource:` 命名空间），不直接渲染服务端 message 作为唯一文案来源。
5. 与第 16 章互链：locale 集合（16.1）、键树与引用规则（16.3）、CI 门禁（16.5）对后端消息包同样生效；后端嵌入式消息包位于 `internal/platform/i18n/locales/*/data.json`。

#### 4.15.2 落地要求

- 机器码体系见错误码注册表 §1；后端翻译器（`internal/platform/i18n`，嵌入式多语言、en 回落）必须接入错误呈现链路：错误呈现层按 `Accept-Language` locale 查译、415/422 等新状态码消息同步入包、多语言键一致性纳入附录 I 门禁。
- 错误体字段名以 4.1.2 错误体契约为准（`details`），`scripts/gen_openapi.py` schema 同步；契约变更按 4.2 处理。

### 4.16 CORS 跨域规范

#### 4.16.1 规则

1. **允许来源显式枚举**：`Access-Control-Allow-Origin` 只能回显**白名单中与请求 `Origin` 精确匹配**的单个来源，并随响应带 `Vary: Origin`；白名单由环境变量配置（建议 `CORS_ALLOWED_ORIGINS`，登记环境变量注册表 D.12）。
2. **禁止 `Access-Control-Allow-Origin: *` 与 `Access-Control-Allow-Credentials: true` 同用**（浏览器会直接拒绝；也是安全红线）。需要凭证（Cookie/Authorization）时必须精确来源。
3. 预检（OPTIONS）：仅回放白名单方法/头；`Access-Control-Max-Age: 600`；`Access-Control-Allow-Headers` 取服务端固定清单（至少含 `Authorization`、`Content-Type`、`Idempotency-Key`、`If-Match`），**不得反射**请求的 `Request-Headers`。
4. **`Access-Control-Expose-Headers`**：跨域需读取的头必须显式暴露——`RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset, RateLimit-Policy, Retry-After, ETag, X-Request-Id`（`X-RateLimit-*` 已按 `generated/naming-registry.md` N11 废弃，**不保留任何过渡别名**）。`ETag` 不可省：4.10 的条件请求要求前端携带 `If-None-Match`，读不到 `ETag` 时跨域形态会静默退化为每次全量响应。
5. **通配子域审批条件**：仅允许 `https`；`https://*.example.com` 形态必须经安全评审、登记父域后缀并做精确后缀匹配（防 `evilexample.com` 类绕过），禁止 `http://*` 与裸 `*`；能精确列举就不用通配。
6. **分层澄清**：OAuth 客户端配置 `corsAllowedOrigins`（应用的登录回调/前端来源，落库于 `app_signin_config.cors_allowed_origins`，见 `internal/transport/http/api/v1/applications/request.go` 的 `CorsAllowedOrigins` 字段）是**授权流程的客户端来源校验**，不替代 API 网关层 CORS 中间件；两者白名单各自维护、最小授权。

```
# ✅ 预检
Origin: https://console.example.com →
Access-Control-Allow-Origin: https://console.example.com
Access-Control-Allow-Credentials: true
Vary: Origin
Access-Control-Max-Age: 600
# ❌ 反例：Allow-Origin: * + Credentials: true；Allow-Headers 原样反射任意头
```

#### 4.16.2 落地要求

- 后端必须提供可配置 CORS 中间件：默认同源不放行、白名单显式登记、子域通配走安全评审；本地开发同源代理（Vite）不豁免该要求。
- 与 OAuth 客户端来源校验（4.16.1 第 6 条）、4.4 暴露头清单联动验收。

**实现位置**（`internal/transport/http/middleware/cors.go`，挂载于 `internal/bootstrap/chi.go`）：

| 要求 | 实现 | 校验 |
|---|---|---|
| 来源精确回显 | `CORSConfig.matchOrigin` 只回显命中的单条来源，未命中不回显任何 CORS 头（且不报错） | `cors_test.go` `TestCORS_UnlistedOriginIsNotEchoed` |
| `Vary: Origin` | 命中与未命中**都**追加；同字段去重后并入单个 `Vary` | `TestCORS_ExactOriginIsEchoedWithCredentials` / `TestCORS_UnlistedOriginIsNotEchoed` |
| 通配子域点边界 | `https://*.父域` 要求非空子域 + 点边界，`evilexample.org` 与裸父域均不匹配；`http://` 与 `*` 在启动阶段即被 `Validate()` 拒绝 | `TestCORS_WildcardSubdomainRequiresNonEmptyLabelAndDotBoundary` / `TestCORSConfig_Validate` |
| 不反射请求头 | `Access-Control-Allow-Headers` 取服务端固定清单 | `TestCORS_PreflightDoesNotReflectRequestedHeaders` |
| 预检终结 | OPTIONS 预检返回 204 且不下发业务链（否则一次真实请求会被计两次限流/审计） | `cors_test.go` `serveCORS` 断言 `nextCalled == false`；`TestCORS_PreflightDoesNotReflectRequestedHeaders` |
| 暴露头 | `Access-Control-Expose-Headers` 含 `RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset, RateLimit-Policy, Retry-After, ETag, X-Request-Id`，`RateLimit-*` 取值取自 `internal/platform/ratelimit` 常量 | `TestCORS_ExposesRateLimitAndRequestIDHeaders` |
| 配置校验时机 | 非法白名单在 `NewRuntime` 装配阶段返回错误（进程启动失败），不是首个跨域请求时静默不放行 | `bootstrap/runtime.go`；`CORSConfig.Validate` |

**中间件链次序**：CORS 必须排在可观测性、限流与安全头**之前**（`bootstrap/chi.go`）。两条理由：①预检请求由 CORS 终结，不应消耗限流额度、不产生 span、不写审计；②被限流或被 WAF 拒绝的响应也必须带跨域头，否则浏览器只报「跨域失败」，把 429 的退避指引连同真实原因一起藏掉。

### 4.17 CSP 与文件上传契约

#### 4.17.1 Content-Security-Policy 目标基线

所有提供 HTML 的响应（控制台、错误页、嵌入式登录页）必须下发 CSP 响应头（不依赖 `<meta http-equiv>`），目标基线：

```http
Content-Security-Policy: default-src 'self';
  script-src 'self' 'nonce-{每请求随机}';
  style-src 'self' 'nonce-{每请求随机}';
  img-src 'self' data: blob:;
  font-src 'self';
  connect-src 'self';
  object-src 'none';
  base-uri 'self';
  frame-ancestors 'none';
  form-action 'self';
  report-to csp-endpoint
```

| 规则 | 说明 |
|------|------|
| `default-src 'self'` | 默认只允许同源；按需逐项放开，禁止整体 `*` |
| `script-src` nonce | 每请求一次性随机 nonce（HTML 响应 `Cache-Control: private, no-store` 或另行做非 nonce 静态分片）；**禁止 `'unsafe-inline'`/`'unsafe-eval'`**（审计例外须登记） |
| `object-src 'none'` | 禁插件（Flash/Java/Applet 残留面） |
| `frame-ancestors 'none'` | 防点击劫持（与现有 `X-Frame-Options: DENY` 双发过渡） |
| `base-uri 'self'` / `form-action 'self'` | 防 `<base>` 劫持与表单外发 |
| `report-to` | 违规上报端点先行 `Report-Only` 观察一个发布周期再切强制；报告只入内部端点，禁止上报到第三方 |

**反例**：脚本/样式 `unsafe-inline` 长期无例外登记；CSP 只在生产发、测试环境不发（问题无法提前暴露）；用 meta 标签发 CSP 但遗漏 `frame-ancestors`（meta 不支持该指令）。

#### 4.17.2 文件上传契约

| 规则 | 约定 |
|------|------|
| 入口 | `multipart/form-data`，字段名 `file`；路径必须纳入 `/api/v1/` 版本化 |
| **大小上限** | 由环境变量控制（`STORAGE_UPLOAD_MAX_BYTES`，默认 `33554432` 即 32 MiB，登记环境变量注册表 D.12）；超限返 **413 Content Too Large** |
| **MIME 双校验（强制）** | ① 读取前 512 字节用 `http.DetectContentType` 做**魔数嗅探**；② 扩展名白名单校验；③ 两者一致且与客户端声明的 `Content-Type` 一致才接受；任一不符 **422**。白名单由 `UPLOAD_ALLOWED_MIME_TYPES` 配置（环境变量注册表 D.12） |
| 活跃内容 | 默认拒绝 `text/html`、`image/svg+xml`、`application/javascript`、`application/x-msdownload` 等可执行/可脚本类型；业务确需 SVG 时必须经 SVG 净化器处理并**仅以附件方式**提供 |
| 下载防护 | 用户内容响应固定 `Content-Disposition: attachment; filename*=UTF-8''<编码文件名>`（RFC 5987）+ 已有的 `X-Content-Type-Options: nosniff`；确需内联的图片仅限魔数校验通过的位图类型，并规划独立内容域名 |
| 对象键 | 服务端以 UUIDv7 + 白名单扩展名生成对象键；**禁止**直接用 `header.Filename` 或客户端传入路径拼存储键 |
| 隔离存储 | 专用桶/前缀、默认私有、与业务数据库备份域隔离；跨域上传采用**预签名浏览器直传**（POST/PUT 预签名 + 服务端先授权后签发） |
| Range | 支持 `Range` 请求（大文件/视频播放）：本地 `/uploads/*` 经标准库 `http.FileServer` 服务，原生支持条件请求与 **206 Partial Content**；云存储侧经预签名 URL 由对象存储支持；契约须在 OpenAPI 声明 `200/206` 与 `Accept-Ranges: bytes` |
| 路径穿越 | 所有 provider 的对象键在**应用服务层**统一做穿越/绝对路径校验，不允许仅个别 provider 自防 |

```http
# ❌ 反例：只相信 multipart 中客户端自报的 Content-Type（可把 HTML 改名 .png）；
# 32 MiB 上限写死在代码；上传后用 /uploads/xxx.svg 直接内联访问（存储型 XSS）；
# 接收 fullFilePath=../../etc/passwd 风格键并原样传给对象存储。
```

#### 4.17.3 落地要求

- 安全头中间件（`internal/transport/http/middleware/security_headers.go`）必须全局挂载，CSP 按 4.17.1 目标基线逐指令落地（nonce、`object-src`、`base-uri`、`form-action`、`report-to`）；nonce 迁移完成前 `style-src` 的 `'unsafe-inline'` 属须登记的审计例外；CSP 同时适用于测试环境；禁止用 `<meta>` 承载 CSP。
- 上传端点路径为 `/api/v1/...` 版本化形态；大小上限由 `STORAGE_UPLOAD_MAX_BYTES` 控制，不得硬编码。
- 所有 provider 的对象键统一走应用服务层穿越/绝对路径校验（如 `safePath`），云 provider 不得豁免。
- 下载响应固定 `Content-Disposition: attachment`（RFC 5987）+ `X-Content-Type-Options: nosniff`；收敛验收＝恶意伪装文件（HTML 改名 PNG、SVG 含 `<script>`、路径穿越键）在契约测试中全部 422/拒绝，上传响应不产生可内联执行的活跃内容。

---

## 5. 架构与工程规范

### 5.1 模块与分层约束

> **决策依据**：[ADR-005 端口接口隔离基础设施](adr/ADR-005-port-interfaces.md)（依赖方向冻结、端口接口归属 Domain/Platform、方言与外部系统实现一律落 Infra），并由 `scripts/check-layers.sh` 门禁把关。

**严格依赖规则**（违反即 PR 拒绝）：

| 层 | 允许依赖 | 禁止依赖 |
|----|---------|---------|
| Transport | App, Platform | Domain, Infra |
| App | Domain, Platform | Transport, Infra（直接） |
| Domain | Platform (clock/uuid) | App, Transport, Infra |
| Infra | Domain (实现接口), Platform | App, Transport |
| Platform | 无依赖 | 所有其他层 |
| Bootstrap | All | - |

**关键约束**：

- Domain 层**绝不**导入 Infra 包
- Domain 层**绝不**导入 Transport 包
- App 层**绝不**导入 Transport 包
- App 层**绝不**直接导入 Infra 包（通过 Domain 层端口接口解耦）
- Infra 实现 Domain 定义的 Repository 接口
- Transport 调用 App Service，不直接调用 Repository

**验证脚本**：见附录 A。

**端口接口解耦模式**（App → Infra 违规修复）：

| 模式 | 说明 | 示例 |
|------|------|------|
| Domain 端口接口 | 在 Domain 层定义接口，App 层依赖接口而非 Infra 实现 | `domain/verifications.SenderFactory`、`domain/resources.StorageProvider`、`domain/roles.PolicyManager`、`domain/certs.CertificateRenewer`、`domain/tokens.TokenIssuer`、`domain/auth.JWTIssuer`、`domain/auth.JWKSProvider`、`domain/ldaps.LDAPAuthenticator`、`domain/permissions.PolicyEnforcer`、`domain/webhooks.DeliverySender` |
| Type Alias 桥接 | Infra 层用 `type X = domain.X` 将具体类型桥接到 Domain 类型 | `infra/messaging.EmailSender = domain/verifications.EmailSender`、`infra/storage.ObjectInfo = domain/resources.ObjectInfo` |
| 函数注入 | 中间件通过函数参数注入依赖提取逻辑，而非直接读取 HTTP header | `RBACAuthz(manager, cfg, extractRole)` 替代 `r.Header.Get("X-User-Role")` |

### 5.2 依赖管理

| 规则 | 说明 |
|------|------|
| Go 模块 | `go.mod` 统一管理，`go mod tidy` 清理 |
| 前端依赖 | `pnpm` 管理，`package.json` 统一版本 |
| 版本统一 | 同一库只能有一个版本 |
| 禁止循环依赖 | Go 编译器会阻止，但需代码评审时注意 |
| 间接依赖 | `go.sum` / `pnpm-lock.yaml` 提交到版本控制 |
| 安全更新 | 定期运行 **`govulncheck`**（`go run golang.org/x/vuln/cmd/govulncheck@latest`）/ `pnpm audit` / `gosec ./...`；注意不存在 `go audit` 或 `go vuln` 这两个命令 |
| Patch 自动 | CI 每周自动检测 patch 更新并创建 PR |

### 5.3 配置管理

| 规则 | 说明 |
|------|------|
| 环境变量 | 所有配置通过环境变量注入 |
| .env 文件 | `.env` 提交模板，`.env.local` 不提交 |
| godotenv | `run.go` 仅在**非生产**环境加载 `.env.local` → `.env`；`APP_ENV=production` 一律跳过，环境变量只能来自编排层（k8s Secret / systemd `Environment=` / 容器 env）。原因：dotenv 会把 `.env` 里未设置的键补进进程环境，而 DSN 优先级是 `DB_DSN > POSTGRES_DSN > DATABASE_URL`，若部署只注入 `DATABASE_URL` 而同目录 `.env` 含 `POSTGRES_DSN`，进程会静默连错库。启动日志 `postgres DSN source env_key=…` 只打印来源键名、不打印值，用于自证 |
| 敏感配置 | 禁止硬编码密钥/密码，使用环境变量 |
| 默认值 | 非敏感配置可有合理默认值 |
| 生产禁用 | `AUTO_MIGRATE`、`AUTO_SEED` 默认 `false`，仅显式 `true`/`1`/`yes`/`on` 才启用；生产**不得启用**。服务启动可用 `-migrate`/`-seed` 显式开启（命令行优先于环境变量，`-seed` 隐含 `-migrate`） |
| 环境识别 | **唯一 `APP_ENV`**（ADR-021）：`internal/platform/config/env_core.go` 的 `IsProduction()` 为唯一判定入口；`IS_PRODUCTION` 仅作为只读同义词，与 `APP_ENV` 冲突即拒绝启动；禁止引入其他环境标识变量（`GULIANGIAM_ENV` / `GINULIANGIAM_ENV` 一律无效） |

**环境变量完整清单**：见附录 D。

### 5.4 文件与目录结构约定

**后端新增域模块检查清单**：

1. `internal/domain/{module}/` - 实体 + 仓储接口
2. `internal/app/{module}/` - 应用服务
3. `internal/infra/{postgres|mysql}/{module}/` - 仓储实现（双方言镜像，ADR-023；双方言目录保持一致文件结构：models.go, repository.go, sql.go, mapper.go）
4. `internal/transport/http/api/v1/{module}/` - Handler（chi.go, handler.go, request.go, response.go, routes.go, new.go）
5. `internal/bootstrap/runtime.go` - 注册服务和 Handler
6. `internal/bootstrap/chi.go` - 注册路由组
7. `openapi/openapi.yaml` - 通过 `scripts/gen_openapi.py` 自动生成

**统计类模块变体**（如 Dashboard）：当模块仅需聚合查询而非 CRUD 时，接口定义在 `internal/app/{module}/repository.go`（而非 domain 层），Infra 层直接实现该接口，避免创建无实体的 domain 包。

**前端新增页面检查清单**：

1. `web/src/pages/{module}/` - 页面组件
2. `web/src/shared/api/iam/{module}.ts` - API 模块
3. `web/src/shared/api/query-keys.ts` - 注册 queryKey 工厂
4. `web/src/app/router/router.tsx` - 注册路由（懒加载）
5. `web/src/features/navigation/menu.tsx` - 添加菜单项
6. `web/src/shared/i18n/locales/{lang}/resource.json` - 添加 i18n 键（14 语言）

### 5.5 存储 Provider 抽象规范（后端）

> 本节由原「2.7 Storage Provider 规范」归位而来（存储抽象是后端能力，不属于前端章节）；前端消费视角见 2.7。

#### 5.5.1 后端 Storage 抽象

`storage.Provider` 接口定义文件操作抽象：

```go
type Provider interface {
    Put(ctx, key, contentType, content) error
    PutStream(ctx, key, reader, contentType) error
    Get(ctx, key) (content, contentType, error)
    GetURL(ctx, key) (string, error)
    Delete(ctx, key) error
    List(ctx, prefix) ([]string, error)
}
```

#### 5.5.2 配置约定

`STORAGE_TYPE` 决定装配实现，Provider 构造参数通过环境变量注入；变量完整清单以**环境变量注册表 D.5** 为唯一事实源（`STORAGE_TYPE` 默认 `local`，另有 `STORAGE_LOCAL_BASE_DIR` / `STORAGE_LOCAL_BASE_URL` / `STORAGE_ACCESS_KEY` / `STORAGE_SECRET_KEY` / `STORAGE_REGION` / `STORAGE_ENDPOINT` / `STORAGE_BUCKET` 及 Azure 专用变量）。

#### 5.5.3 上传接口约定

- 端点：`POST /api/v1/upload-resource`（multipart/form-data）
- 返回格式：`{"data": "url"}`（前端 ImageCropUpload 组件契约）
- 文件大小限制：由 `STORAGE_UPLOAD_MAX_BYTES` 控制（默认 32 MiB，见 4.17.2）
- 静态文件服务：Local 模式下通过 `/uploads/*` 路径提供

#### 5.5.4 支持的存储后端

存储后端集合以 `internal/infra/storage/factory.go` 的注册为唯一事实源（实现文件 `internal/infra/storage/{backend}_storage.go`；复现：`ls internal/infra/storage/*_storage.go`）。当前覆盖：本地文件系统（Local，默认）、S3 与 S3 兼容系（S3 / MinIO / R2 / B2 / Spaces）、国内云（阿里云 OSS、腾讯 COS、华为 OBS、七牛、天翼 CUCloud）、海外公有云（Google GCS、Azure Blob）与 NAS（Synology FileStation）。新增后端必须实现 `storage.Provider` 接口并在 factory 注册，禁止在业务代码中散落分支判断。

### 5.6 策略即代码规范（Policy-as-Code）

> 授权策略以**策略包（GitOps 仓库）**为唯一发布来源；`guliangiam.casbin_rules`（`internal/infra/casbin/adapter.go`）是运行时加载与应急带外通道的存储面，不作为策略编辑入口。本节为 ReBAC/ABAC 策略资产（[permissions](design/iam/permissions.md)，ADR-024）的强制规范。

**策略包目录结构**：

```text
policies/
├── rebac/<namespace>/          # 关系型策略：一命名空间一目录
│   ├── namespace.yaml          # relation_defs 展开规则（对应 relation_namespaces）
│   └── version.yaml            # 语义化版本 + 变更说明 + 评审记录引用
├── abac/<domain>/              # 属性型策略：属性目录与环境属性输入（REBAC-008）
│   └── attributes.yaml
├── actions/<action-name>/      # 流程扩展点声明（求值与授权正交，见 5.7）
│   ├── action.yaml             # 触发点 / 失败策略 / 出站白名单声明
│   └── src/                    # 源码；构建产物以 artifact_ref 指纹登记
└── tests/                      # 策略用例：正向 + 负向 + 边界，随包提交
```

**工程要求**：

| 项 | 规范 |
|----|------|
| 评审 | 双人评审强制：作者 + 独立评审人各 1 人；安全相关策略（deny 类规则、SoD 规则、出站白名单）第二人必须为安全角色（与 7.1 安全评审同节点） |
| 版本化 | 策略版本轴有两个且**不得混用**：① 授权策略 = `relation_namespaces.version` 整型递增 + draft/active/deprecated 状态机（[permissions](design/iam/permissions.md)）；② 流程扩展 = `identity_action_versions`（版本不可变，发布后只读）。Actions 不承担授权判定（ADR-029 决策 5），策略包引用 Action 时必须锁定具体版本号 |
| 静态检查 | CI 必检项：① 引用完整性——relation 类型/属性名引用必须存在于已注册目录；② 边界——展开深度上限与结果集分页上限声明必填；③ 必填字段——`organization_id` 作用域、`version`、失败策略，缺一即失败；④ 禁止策略体内联机密（Secret 扫描 9.9 同源）；⑤ 用例覆盖——每条规则至少 1 正 1 负 |
| GitOps 发布 | `PR → CI 校验（静态检查 + 策略用例）→ 双人合入 → 灰度（租户子集 / 流量百分比）→ 全量`；灰度期间新旧版本并存、同会话钉版不混用（[ADR-029](adr/ADR-029-identity-actions-sandbox.md) 同纪律）；回滚 = 激活版本指针回退并即时生效；生产变更留 `records`（`policy.changed`，12.5） |
| 门禁挂接 | 策略包校验（目录 lint + 静态检查 + 用例执行）为 PR 触发的独立门禁作业，失败阻塞合并；落地时同步更新 17.1 与 8.9 表（同款三条件纪律），未接线前只能表述为「本机手工执行」 |
| 应急带外通道 | 管理面 API 直接写 `casbin_rules` 的路径仅作为应急带外通道（常规发布路径为 GitOps），仅限故障恢复；操作必须留 `authz.*` 审计并在 24h 内回填策略包 PR，否则视为违规 |

### 5.7 Actions 开发规范（Identity Actions）

> 本节随 [ADR-029](adr/ADR-029-identity-actions-sandbox.md) 落地强制执行；webhooks 为出站通知，不承担请求路径同步扩展点职责。

| 项 | 规范 | 锚点 |
|----|------|------|
| 触发点契约 | 触发点目录唯一权威 = [ADR-029](adr/ADR-029-identity-actions-sandbox.md) 五类表（pre-login / pre-token / post-registration / consent / MFA challenge），可返回指令 `continue/deny/modify` 逐点声明；**禁止 Action 内自造触发点或旁路挂载**；新增触发点必须先修订 [ADR-029](adr/ADR-029-identity-actions-sandbox.md) 并过 ADR | [ADR-029](adr/ADR-029-identity-actions-sandbox.md) |
| 隔离红线 | ① 网络：出站**一律经白名单代理**，默认拒绝、显式放行（域名/网段），越界阻断并审计，执行 [outbox SSRF 出口管控](design/infrastructure/outbox.md#ssrf-出口管控覆盖所有出站-http) 四条（禁内网段 / 限重定向 / DNS rebinding 防护 / scheme+端口白名单）；② 密钥：经宿主**注入端口**按 action 粒度注入，**禁止直读进程 env、禁止落配置明文与日志**，密钥引用走信封格式（第 14 章 14.2）；③ 能力：无文件系统访问、无任意进程、无时钟原语（WASM 优先，受限 JS 备选须复审） | [ADR-029](adr/ADR-029-identity-actions-sandbox.md) |
| 超时与配额 | 默认配额：CPU 50ms/次、内存 64MiB、墙钟 200ms/次（默认值待 L2 基线校准后修订）；任一触顶即终止并记 `outcome=timeout|quota_exceeded`；失败策略按 [ADR-029](adr/ADR-029-identity-actions-sandbox.md) 逐触发点执行——安全类（pre-login / pre-token / consent / MFA）**fail-close**，post-registration 可声明 fail-open 但必须留失败审计与告警 | [ADR-029](adr/ADR-029-identity-actions-sandbox.md) |
| 可观测 | `identity_action_executions` 快照**必打字段**：`action_id`、`action_version_id`、`duration`、`outcome`（continue/deny/modify/timeout/quota_exceeded）、输入摘要（8.9 脱敏，禁原文）；执行失败**不产生计量事件**（METER-002 口径） | [ADR-029](adr/ADR-029-identity-actions-sandbox.md)；12.5 |
| 审核与发布 | 上架审核 = 代码评审（第 7 章）+ 沙箱逃逸用例集通过（17.4）+ 出站白名单逐条评审 → 灰度（`canary_percent` 按「租户 × 应用 × 流量百分比」钉版）→ 全量；回滚 = `active_version` 指针切回即时生效；版本不可变，artifact 保留至在途执行排空 | [ADR-029](adr/ADR-029-identity-actions-sandbox.md) |

---

## 6. Git 版本控制规范

### 6.1 分支模型与策略

采用简化的 Git Flow：

| 分支 | 用途 | 生命周期 |
|------|------|---------|
| `main` | 生产就绪代码 | 永久 |
| `develop` | 开发集成 | 永久 |
| `feature/{module}-{desc}` | 功能开发 | 合并后删除 |
| `bugfix/{module}-{desc}` | 缺陷修复 | 合并后删除 |
| `hotfix/{desc}` | 生产紧急修复 | 合并后删除 |
| `release/v{version}` | 发布准备 | 合并后删除 |

### 6.2 分支命名规范

```
feature/auth-qrcode-login
feature/commerce-order-lifecycle
bugfix/users-password-history-check
hotfix/session-cookie-hmac-key
release/v1.2.0
```

### 6.3 提交信息规范（约定式提交）

```
<type>(<scope>): <subject>

<body>

<footer>
```

**类型**：

| 类型 | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | 缺陷修复 |
| `refactor` | 重构（不改变行为） |
| `docs` | 文档变更 |
| `test` | 测试相关 |
| `chore` | 构建/工具/依赖变更 |
| `perf` | 性能优化 |
| `style` | 代码格式（不影响逻辑） |
| `security` | 安全修复（新增类型） |

**示例**：

```
feat(auth): implement QR code scan login

Add 5 API endpoints for QR code login flow:
- POST /auth/qrcode (create)
- POST /auth/qrcode/scan (scan)
- POST /auth/qrcode/confirm (confirm)
- GET /auth/qrcode/{codeId}/status (poll)
- POST /auth/qrcode/exchange (exchange)

Closes #123
```

### 6.4 代码合并与 MR 流程

1. 功能分支开发完成，自测通过
2. 创建 Merge Request 到 `develop`
3. 至少 1 人 Code Review 通过
4. CI 通过（lint + test + build + security scan）
5. Squash Merge 到 `develop`
6. 删除功能分支

---

## 7. 代码评审规范

### 7.1 评审流程与责任人

| 阶段 | 责任人 | 说明 |
|------|--------|------|
| 自审 | 提交者 | 提交前自查 |
| 指派评审 | 团队成员 | 至少 1 人通过 |
| 架构评审 | 架构师 | 跨模块/架构级变更 |
| 安全评审 | 安全负责人 | 涉及认证/授权/加密变更 |

### 7.2 评审检查清单

**通用检查**：

- [ ] 代码是否符合命名规范
- [ ] 是否遵循分层依赖规则（运行附录 A 脚本）
- [ ] 是否有未处理的错误（`_ = xxx` 出现即拒绝）
- [ ] 是否有硬编码的密钥/密码
- [ ] 日志是否包含必要上下文且不含敏感信息
- [ ] 是否使用了 `slog` 而非 `log.*`
- [ ] `fmt.Errorf` 是否仅用于 `%w` 包装

**后端检查**：

- [ ] Domain 层是否未导入 Infra 包
- [ ] SQL 是否使用参数绑定
- [ ] 仓储方法是否遵循命名约定
- [ ] 新增域模块是否在 Bootstrap 中注册
- [ ] Context 是否正确传递
- [ ] 并发安全：共享状态是否有锁保护
- [ ] 乐观锁：Update SQL 是否包含 `revision` 条件
- [ ] 软删除：查询是否过滤 `deleted_at IS NULL`
- [ ] 敏感字段是否加密
- [ ] 速率限制是否配置

**前端检查**：

- [ ] 路由是否懒加载
- [ ] API 调用是否使用 TanStack Query
- [ ] queryKey 是否来自工厂
- [ ] Zod schema 是否定义
- [ ] i18n key 是否添加（14 语言）
- [ ] 权限控制是否配置（usePermissions）
- [ ] 交互元素是否支持键盘（A11y）
- [ ] 文本对比度是否达标
- [ ] 是否使用 CSS 变量而非硬编码颜色

### 7.3 评审意见管理

| 标签 | 含义 | 处理方式 |
|------|------|---------|
| `MUST` | 必须修改 | 阻塞合并 |
| `SHOULD` | 建议修改 | 尽量修改 |
| `NIT` | 小问题 | 可选修改 |
| `QUESTION` | 疑问 | 需回复 |

---

## 8. 测试规范

### 8.1 单元测试要求

| 层 | 覆盖率目标 | 测试重点 |
|----|-----------|---------|
| Domain | > 80% | 实体验证、值对象不可变性、状态机 |
| App | > 70% | 业务逻辑编排、输入校验、错误分支 |
| Transport | > 50% | 请求解析、响应序列化、中间件 |
| Infra | > 50% | SQL 映射、事务管理 |
| **整体** | **> 30%** | **CI 门禁** |

**Go 测试约定**：

```go
// 文件名: service_test.go
func TestUserService_Create(t *testing.T) {
    // 使用 fake 依赖
    clock := clock.FakeClock{FixedTime: time.Now()}
    uuid := uuid.FakeGenerator{FixedID: "019abc"}
    uow := &mockUnitOfWork{}
    
    svc := NewService(clock, uuid, uow)
    
    user, err := svc.Create(ctx, CreateUserCommand{...})
    require.NoError(t, err)
    assert.Equal(t, "019abc", user.ID)
}
```

**前端测试约定**：

```ts
// 文件名: usePermissions.test.ts
import { renderHook } from "@testing-library/react";
import { usePermissions } from "./usePermissions";

describe("usePermissions", () => {
  it("should return allowed actions for admin", () => {
    const { result } = renderHook(() => usePermissions("admin"));
    expect(result.current.canCreate).toBe(true);
  });
});
```

### 8.2 集成测试与端到端测试

| 类型 | 工具 | 范围 |
|------|------|------|
| API 集成测试 | Go `testing` + test DB | 完整请求→数据库→响应 |
| 前端 E2E | Cypress（主）/ Playwright（辅） | 关键业务流程（登录、MFA、CRUD） |
| Bootstrap 测试 | `internal/bootstrap/*_test.go` | 路由注册、API 分组 |
| 性能基准 | Go `testing.B` + `-benchmem` | 核心热路径 |

> **E2E 栈口径（唯一表述）**：**Cypress 为主、Playwright 为辅**。Cypress（`web/cypress.config.ts`）为 `pnpm e2e` 主入口；Playwright（`@playwright/test`）为补充入口 `pnpm e2e:pw`，用于少量场景与 trace 取证。两者均须按 8.9 门禁表接入 CI（17.1「集成测试」行）。0.1 技术栈表与 [architecture/layers.md §2 技术栈](architecture/layers.md#2-技术栈) 同源。

### 8.3 测试数据规范

| 规则 | 说明 |
|------|------|
| 隔离性 | 测试间不共享数据，每个测试创建和清理 |
| 幂等性 | 测试可重复执行，结果一致 |
| 真实性 | 测试数据贴近生产格式（UUIDv7 格式、有效邮箱等） |
| 敏感性 | 测试数据不含真实用户信息 |
| 状态机 | 状态转换测试覆盖所有合法/非法转换 |

### 8.4 Mock 与 Stub 使用原则

| 场景 | 推荐方式 |
|------|---------|
| 仓储接口 | 接口 Mock（如 `mockRepository`） |
| 外部 API | HTTP Mock Server |
| 时钟 | `clock.FakeClock` |
| UUID | `uuid.FakeGenerator` |
| 数据库 | 真实测试数据库（Docker） |

**原则**：

- Domain 层测试：Mock 仓储接口
- App 层测试：Mock 仓储 + 使用 Fake Clock/UUID
- Infra 层测试：使用真实测试数据库
- Transport 层测试：Mock App Service

### 8.5 覆盖率门槛分域

**分域阈值表**（CI 覆盖率门禁的**权威口径**；8.1 各分层行继续作为开发自检目标）：

| 域 | 包 / 范围 | 阈值 | 口径 |
|----|----------|------|------|
| 整体 | `go test ./...` 汇总（按排除清单扣除） | 阶梯：**50% → 60% → 70%**（每档达成并稳定一个迭代后方可升档） | 语句 |
| 安全关键域 | `internal/app/auth`、`internal/app/tokens`、`internal/platform/crypto`、`internal/platform/jwks`、`internal/platform/dpop`、`internal/app/permissions`、`internal/infra/casbin` | **≥ 85%** | 语句 |
| 分支口径 | 整体域 + 安全关键域，阈值与语句口径相同 | 同上 | 分支（分支口径工具选型结论登记附录 I） |
| 分层自检 | Domain / App / Transport / Infra | 维持 8.1 分层表 | 语句（开发自检口径，不进 CI 门禁） |

**排除生成代码清单**（覆盖率统计排除，实现方式为 CI coverage 步骤以 `-coverpkg` 白名单或 profile 过滤，禁止用「报告人工扣除」方式）：

| 排除对象 | 依据 |
|---------|------|
| go.uber.org/mock 生成物（`internal/domain/*/mock/`、`internal/app/uow/mock/` 等） | 文件头 `Code generated ... DO NOT EDIT` |
| 测试文件自身（`*_test.go`） | Go 覆盖率默认不计入 |

**基线纪律**：整体与分域阈值生效前须完成一次全量 `go test -coverprofile` 测量并把结果归档为基线制品；未完成测量前，禁止在任何文档或汇报中引用具体覆盖率数字。

### 8.6 集成测试真实依赖（testcontainers）与契约测试

**testcontainers**：集成测试的真实依赖由 `testcontainers-go`（含 `modules/postgres`）提供；`internal/infra/postgres/testutil/container.go` 是唯一的**容器装配入口**，各仓储集成测试（`organizations` / `roles` / `groups` / `permissions` / `sessions` / `users` / `applications` / `webhooks` / `tokens` 的 `repository_integration_test.go`）统一经它装配。

| 规则 | 要求 |
|------|------|
| 真实数据库 | Infra 层集成测试必须通过 `testutil/container.go` 起真实 PostgreSQL 容器，禁止以内存假仓储替代集成验证（单测中的 Mock 分工不变，见 8.4） |
| 装配唯一入口 | testcontainers 装配只允许发生在 `testutil` 包；业务测试不得直接 import testcontainers |
| Redis 真实容器 | 涉 Redis 的集成测试必须引入 `modules/redis` 真实容器补齐（8.9 联动表） |
| CI 可用性 | CI 具备 PG service 容器，集成测试随 `./internal/...` 执行；CI runner 上 testcontainers 的 Docker 可用性须通过专项验证 |

**契约测试**：

| 层 | 定位 | 要求 |
|----|------|------|
| 文档级契约门禁 | `scripts/check-openapi-contract.sh`（17.1 / 附录 I.1）：管「OpenAPI 文档与代码签名一致」 | `ci.yml` 的 `gates` 作业逐条执行，本机命令与 CI 命令一致 |
| 运行时 schema 断言 | 集成测试中加载 `/openapi/openapi.yaml` schema 并对响应做结构与类型校验，管「运行时行为与 schema 一致」 | 必须落地（8.9 门禁表） |
| 消费者驱动契约（CDC） | 前端/第三方作为消费者断言契约 | 可选演进项，不进门禁 |

两契约层互不替代：schema 断言落地时必须复用 `check-openapi-contract.sh` 的同一份 OpenAPI 源，禁止出现第二份契约事实源。

### 8.7 测试数据工厂与脱敏集、性能回归挂钩

**测试数据工厂**（8.3 定义数据**性质**要求，本节定义**构造方式**要求；工厂是实现 8.3 的标准手段）：

| 规则 | 说明 |
|------|------|
| 工厂函数 | 每个聚合根包提供 `testutil` 工厂（`NewTestUser(t, opts...)` 风格），集中管理合法默认值与变体；禁止在测试体内散落字面量组装聚合 |
| 合法 / 非法分离 | 工厂必须显式区分「合法基线」与「非法变体」；非法变体供负向用例（9.11 越权矩阵）与 fuzz 语料（9.12）复用同一构造源 |
| 状态机覆盖 | 工厂提供状态迁移辅助方法，支撑 8.3「状态机」要求 |

**脱敏集与真实数据禁令**：

| 规则 | 说明 |
|------|------|
| 真实数据禁入 | 生产库快照、备份、导出文件**不得**作为任何层级的测试数据源（8.3 敏感性 + 19.4 生产红线延伸）；E2E 使用的 staging 数据必须先经脱敏集处理 |
| 脱敏集 | 对 D2/D3 字段（9.16 分级）提供**确定性**脱敏变换（保持格式与唯一性约束的假名化，保证可重复回放）；脱敏规则与脚本入库，并在 9.16 PII 资产清单登记引用 |

**性能回归挂钩**：Benchmark 类测试的回归判定执行 [architecture/observability.md §2「性能基线（L0-L3 分层）」](architecture/observability.md#2-性能基线l0-l3-分层) 的 L0 使用红线：同一硬件 / Go 版本下相对历史基线变化在 **±10%** 以内视为无回归；超过 ±10% 且统计显著（p<0.05）按该节门禁阻塞并接线 CI。本节只挂钩不复制其数值与结论；L0 数字**不得**用于宣称 SLO（11.2 口径）。

### 8.8 前端组件测试、视觉回归与测试金字塔

**组件测试与视觉回归**：前端单元测试由 CI `gates` 作业强制执行（`pnpm run test`，失败即阻塞合并）；E2E（Cypress 主 + Playwright 辅）与视觉回归按 8.9 接入门禁。视觉回归要求：

| 项 | 要求 |
|----|------|
| 主路径 | Playwright screenshot（`toHaveScreenshot`）优先：先覆盖共享 UI 组件（2.6 `web/src/shared/ui/`）与管理面核心页面快照 |
| 替代形态 | Storybook（story → 快照）可作为共享组件的替代实现，与主路径**二选一**，不做双轨维护 |
| 基线管理 | 快照基线入库；diff 超阈值阻塞合并（阈值随 CI job 定义，见 8.9） |
| 维度控制 | 按「主题（含暗色）× 抽样语言（非全 14 语言）」扩展快照维度，防止组合爆炸 |

**测试金字塔比例指引**：单元 : 组件/集成 : E2E ≈ **70 : 20 : 10**（以用例数为口径的**管理指引**，不作硬门禁）。前端以 vitest 单测为主体、E2E 双栈并存；后端由 Go 单测、仓储集成测试与 Bootstrap 测试构成，不强套该比例。禁止以堆叠 E2E 用例替代单元 / 组件层覆盖。

### 8.9 测试类型与第 17 章质量门禁联动

各测试类型进入质量门禁的 CI job 归属清单如下：

| 测试类型 | 定义节 | 目标 CI job | 门禁动作 |
|---------|--------|------------|---------|
| Go 单测 + 覆盖率 | 8.1 / 8.5 | `ci.yml` backend | 覆盖率低于阈值阻塞；分域阈值见 8.5 |
| testcontainers 集成测试 | 8.6 | 并入 backend job（PG service 已具备） | 失败阻塞；runner Docker 可用性核验为前置 |
| 契约测试（schema 断言） | 8.6 | backend job 新增 step | 失败阻塞 |
| Go fuzz 短时门禁 | 9.12 | 独立 fuzz job（或并入 `ci.yml` security 作业） | 崩溃阻塞 |
| 越权矩阵负向用例 | 9.11 | 并入 backend job；负向用例计数与 `openapi/openapi.yaml` 的管理面 operation 数对账 | 对账失败阻塞 |
| 前端单测 | 8.1 | frontend job `pnpm test`（vitest run 模式） | 失败阻塞 |
| 视觉回归 | 8.8 | 独立 visual-regression job（PR 触发） | diff 超阈值阻塞 |
| E2E（Cypress / Playwright） | 8.2 | 17.3 第 4 步烟雾测试执行环境 | 失败阻塞发布 |
| 协议合规测试 | 9.13 | E2E 层级 + 17.3 第 4 步 | 失败阻塞发布 |

**进入门禁的通用规则**：任何测试类型宣称「已接入质量门禁」必须同时满足：① 有可复现的本地执行命令（随包登记）；② 有本表对应的 CI job 归属；③ 有明确失败处置级别（阻塞 / 警告）。三者缺一，只能表述为「存在测试资产」，不得表述为「已被门禁保护」。

### 8.10 测试执行记录与缺陷分级模板

测试执行的记录字段与缺陷分级为固定模板（本规范唯一口径，禁止另立）：

- **缺陷分级**：Blocker / Critical / Major / Minor / Trivial，各级分别统计「新增 / 已修复 / 遗留」三列。
- **执行记录字段**（每行一条用例，逐次运行逐行落库）：

| 用例编号 | 版本 | 执行时间 | 执行人 | 环境 | 结果 | 证据 | 缺陷 | 备注 |
|---------|------|---------|--------|------|------|------|------|------|

> 单次运行数据落在测试管理系统；本规范只固定字段与分级口径，不承载运行数据，也不构成能力声明。

---

## 9. 安全规范

### 9.1 输入校验与防注入

| 规则 | 说明 |
|------|------|
| SQL 注入 | 使用 pgx/v5 参数绑定，**严禁**字符串拼接 SQL；SCIM 过滤器属性必须白名单映射，未知属性返回空结果（`1=0`）而非直接作为列名 |
| XSS | 前端使用 DOMPurify 清洗 HTML，后端响应设置 `Content-Type`；SecurityHeaders 中间件设置 `X-XSS-Protection: 0`（现代浏览器依赖 CSP） |
| CSRF | Double Submit Cookie（`CSRFTokenMiddleware`） |
| 参数校验 | 后端 Zod（前端）/ 应用层校验（后端） |
| 文件上传 | 校验文件类型、大小，存储到隔离目录 |
| 速率限制 | 全局 100 req/s，登录端点 10 req/min，验证码 3 次/码，10 次/小时/IP |
| 安全响应头 | SecurityHeaders 中间件强制设置：Content-Security-Policy、X-Content-Type-Options: nosniff、X-Frame-Options: DENY、Strict-Transport-Security（TLS 连接时）、Referrer-Policy: strict-origin-when-cross-origin |
| URL 重定向 | 使用 `isSafeRedirectUrl` 校验，拒绝 `//evil.com`、`javascript:`、`data:` |
| SAML RelayState | 使用 HMAC-SHA256 签名 + 常量时间比较 |

### 9.2 认证与授权

| 规则 | 说明 |
|------|------|
| 默认拒绝 | 所有 API 默认需要认证；豁免项必须按「**方法 + 路径模板**」逐条枚举，禁止前缀通配 |
| 公开白名单 | 白名单必须以「**方法 + 路径模板**」逐条枚举并与 `route.Definition` 同源，**全仓唯一声明**；启动自检必须断言「管理面路径不得命中任何豁免项」。**不得**使用 `/api/v1/auth/` 之类的整前缀豁免——该前缀下 `GET account`、`GET permissions`、`POST exit-impersonate`、`login-challenges/{challengeId}:approve\|reject`、`consents:grant\|revoke`、`client:authenticate`、`dpop:validate` 等端点必须鉴权。白名单事实源为 `contracts/registries/surfaces.yaml` 的 `public_whitelist` 键，由路由注册时自动加载校验 |
| Fail-Close | Casbin 评估失败时返回 503（服务不可用），策略拒绝返回 403 |
| 最小权限 | Casbin RBAC 细粒度控制，3 层角色 |
| 域隔离 | 每个组织独立策略空间 |
| 会话管理 | HttpOnly + Secure + SameSite=Lax Cookie |
| Token 安全 | PKCE (S256 only) + DPoP + Token Rotation + 重用检测 + Family Revocation |
| 密码安全 | Argon2id 默认 + 密码历史 + 过期策略 + 泄露检查 |
| 账号锁定 | 5 次失败触发 / 锁定 30 分钟（`account_lockout.go` 默认值），先判锁定再哈希；计数窗口为 15 分钟滑动窗口（计数仅锁定期满清零），持久化于 `lockout_attempts` 表 |
| JWT 算法白名单 | 校验白名单与禁用表以 `internal/platform/jwks/allowed_algorithms.go` 为准（`SM2-SM3` 与 RS/ES/PS 族并列）；禁止 `none`/`HS256`/`HS384`/`HS512`；ID Token 宣告集合见 `internal/platform/jwks/id_token_algorithms.go` |

> **公开白名单纪律**：豁免声明在全仓**只能存在一份**（方法 + 路径模板，与 `route.Definition` 同源派生）；以 `strings.HasPrefix` 做前缀豁免、或存在多份互不一致的白名单声明，均属违规。`/metrics` 挂载于受保护组（requires authentication），不在白名单内。白名单变更触发 9.8 威胁建模准入门禁（AC-PERM-004）。

### 9.3 实名认证安全规范

| 规则 | 说明 |
|------|------|
| 前置条件 | 验证前必须填写 realName + idCardType + idCard，否则返回 `CodeInvalidArgument` |
| 幂等性 | 已验证用户重复调用返回成功（no-op），不产生副作用 |
| 字段锁定 | isVerified=true 后，realName/idCardType/idCard/idCardImages 不可修改 |
| 后端校验 | UpdateProfile 接口应检查 isVerified 状态，拒绝修改锁定字段 |
| 前端锁定 | isVerified=true 时，前端 Real Name 标签页的实名字段全部 disabled |
| 审计追踪 | 实名验证操作写入审计日志 |

### 9.4 组织级字段配置规范（AccountItems）

| 规则 | 说明 |
|------|------|
| 存储位置 | accountItems 存储在 organization_settings 表的 JSONB 列 |
| 配置维度 | 每项包含 name/visible/viewRule/modifyRule/tab/regex 六个维度 |
| 预定义项 | 预定义 AccountItem 集合以 `web/src/pages/organizations/organization-payload.ts` 的 `DEFAULT_ACCOUNT_ITEMS` 与 `db/seeds/V1__guliangiam_seed_init.sql` 的 `organization_settings.account_items` 为事实源，覆盖用户字段 |
| 可见性控制 | visible=false 时字段不渲染；viewRule 控制可见角色（Public/Self/Admin） |
| 修改控制 | modifyRule 控制可编辑性：Self（本人可改）、Admin（仅管理员）、Immutable（不可改） |
| 正则校验 | regex 字段仅对特定字段类型显示 |
| 条件联动 | visible=false 时 viewRule/modifyRule 列不渲染；viewRule=Admin 或 name=Is admin 时 modifyRule 仅显示 Admin/Immutable |
| 前端集成 | UserDetailPage 加载组织 accountItems 配置，通过 useAccountItems hook 计算字段权限 |

### 9.5 敏感数据保护与脱敏

| 数据类型 | 存储方式 | 脱敏规则 |
|----------|---------|---------|
| 密码 | Argon2id 哈希 | 永不返回 |
| Client Secret | **Argon2id 哈希存储**（`client_secret_hash` 列，仅可校验、不可逆；实现见 `internal/platform/crypto/client_secret.go`，与 14.2 / 14.2.1 一致） | 创建时明文仅返回一次；API 响应中掩码 |
| Access Token | SHA-256 哈希存储 | 永不返回原文 |
| Refresh Token | SHA-256 哈希存储 | 永不返回原文 |
| Access Secret | SHA-256 哈希存储 | 创建时返回一次 |
| 私钥 | AES-256-GCM 字段加密（`certs.private_key`、`keys.private_key`，见 14.2） | 永不返回 |
| Webhook Secret | AES-256-GCM 加密 | API 响应中掩码；用于 HMAC-SHA256 签名 |
| 手机号 | 原文存储 | 列表查询时掩码中间 4 位 |
| 身份证号 | AES-256-GCM 字段加密（`user_profiles.id_card_sealed`/`passport_no_sealed`/`driver_license_no_sealed`（V19）、`identity_verifications.id_number_sealed`，注册表 `internal/platform/crypto/legacy_sealed_columns.go`；明文孪生列双写过渡，清除属另行审批的破坏性迁移） | 永不返回原文（响应仅 `XxxSet`/`XxxMasked`） |

**API 响应过滤（以 `internal/platform/redact/redact.go` 的 `IsSensitiveField` 为唯一事实源）**：`password`、`passwordHash`、`passwordSalt`、`accessSecret`、`accessSecretHash`、`accessToken`、`refreshToken`、`clientSecret`、`bindPassword`、`totpSecret`、`recoveryCodes`、`mfaPushReceiver`、`masterPassword`、`kerberosKeytab`、`privateKey`、`token`、`idCard`、`passportNo`、`driverLicenseNo`、`cookie`、`setCookie`、`authorization`、`apiKey`、`credential`、`sessionSecret`、`secret`。匹配按**分词边界**（`tokenize` 后逐段拼接比对，含复数/hash/salt 等变体后缀），故 `access_token`/`X-Api-Key` 均命中，而 `tokenizer` 这类仅包含敏感词的字段名不误伤。**覆盖纪律**：清单必须覆盖全部 D2/D3 响应字段（9.16 分级）——不在清单且分词不命中的字段不会被掩码，必须补入 `sensitiveFields` 或以 `XxxSet`/`XxxMasked` 显式脱敏收敛（证件号族走后者，门禁 `scripts/check-credential-masking.py` 同时扫描 read-side struct json tag 与 `map[string]any` 字面量键）。

**日志脱敏（以 `internal/platform/logging/sanitizer.go` 为唯一事实源）**：sanitizer 直接消费 `redact.IsSensitiveField`（与 API 响应过滤共用 `redact.go` 清单，不存在第二份列表）；命中即以 `redact.Placeholder`（`***`）替换。**覆盖纪律**：清单必须覆盖全部 D2/D3 日志字段；新增敏感 key 时同步补入 `redact.go`，不得依赖子串碰巧命中（如 `masterVerificationCode` 无任何分词命中，必须显式登记）。

### 9.6 日志安全与审计追踪

| 规则 | 说明 |
|------|------|
| 不记录敏感信息 | 禁止日志中包含密码、Token、密钥 |
| 审计追踪 | 关键操作写入 `records` 表 |
| 审计事件 | 登录、登出、Token 签发、密码修改、权限变更、MFA 操作、模拟登录、策略变更、Secret 轮换 |
| 日志格式 | 结构化 JSON，包含 request_id, trace_id |
| 日志保留 | 审计记录按合规要求长期保留（最短 1 年）；应用日志与运行日志的保留期以 19.4 为唯一事实源 |
| 防篡改 | 审计记录只增不改 |

### 9.7 密钥与加密

> 详见第 14 章。

### 9.8 STRIDE 威胁建模准入门禁

**触发条件**（满足任一即必须在方案评审前完成威胁建模，禁止先写代码后补）：

| # | 触发场景 | 说明 |
|---|---------|------|
| 1 | 新业务模块 | 新增 `/api/v1/<module>` 资源面（新增 `route.Definition` 所在模块） |
| 2 | 新协议端点 | 4.1.3 六前缀家族（`/oauth2/*`、`/.well-known/*`、`/cas/*`、`/scim/v2/*`、`/api/v1/auth/*` 协议端点、`/api/v1/saml/*`）任何新增 |
| 3 | 新外部集成 | 新增 Provider 类型、存储后端、支付/消息/验证码渠道、LDAP/Kerberos 上游等 |
| 4 | 敏感数据处理变化 | 新增 D3 级数据字段（见 9.16 分级）、新增导出通道、新增审计消费方 |
| 5 | 公开白名单变更 | 9.2 白名单新增任何「方法 + 路径模板」条目（AC-PERM-004） |

**产物模板——威胁清单表**（每模块/端点族一份，文件随模块 PR 提交，威胁编号 `TH-{module}-{seq}`）：

| 列 | 填写要求 |
|----|---------|
| ID | `TH-{module}-{seq}`，如 `TH-scim-001` |
| STRIDE 类别 | Spoofing / Tampering / Repudiation / Information Disclosure / Denial of Service / Elevation of Privilege（六选一，可多标） |
| 资产 | 被威胁的数据/功能（引用 9.16 资产清单条目或表名） |
| 攻击路径 | 入口端点 + 前置条件 + 攻击步骤（一句话可复现） |
| 控制 | 已有/计划中的缓解（中间件、限流组、AC 编号、DB 编号） |
| 残余风险 | 控制实施后剩余风险与接受理由 |
| 状态 | `mitigated` / `accepted`（须附豁免单，见附录 G）/ `open` |

**评审挂钩**：

1. PR 模板新增勾选项「本变更触发 9.8 威胁建模：是/否；清单链接：___」。
2. 7.1 的「安全评审」阶段必须核对：触发条件命中而未附威胁清单的 PR 一律 `MUST`（阻塞合并，见 7.3）。
3. 启动自检断言「管理面路径不得命中任何豁免项」（9.2）通过后，新增白名单条目必须同时附对应威胁清单行。

**与 [architecture/security-model.md](architecture/security-model.md) 联动**：该文档是**系统级**安全模型与加固项清单；本节产物是**模块级**增量。新模块威胁清单必须先对照该文档已登记的安全控制（§6/§7），新增的、系统级未覆盖的威胁**必须回写该文档**，保持两处口径一致。

### 9.9 SAST / DAST / Secret 扫描 / SBOM / 容器与 IaC 扫描

> `ci.yml` 的 `gates` 作业接线本手册的门禁脚本（分层、DB 约定、SQL 可移植性、OpenAPI 契约）+ 路由/三面契约测试与 `verify-docs.sh`，`docker` 作业以其为前置。下表为各安全能力的 CI job 归属与阻断级别。

| 能力 | 工具 | 固定版本策略 | CI job | 阻断级别 |
|------|------|-------------|--------|---------|
| SAST（Go） | gosec | **禁止 `@master` 浮动**：固定 release tag（如 `v2.22.x`）并在附录 I 登记版本；升级走 PR | `ci.yml` security-gosec | Error（High 级以上）阻断合并 |
| 依赖漏洞（Go） | govulncheck | 固定 `golang.org/x/vuln/cmd/govulncheck@<tag>`；不得使用无效命令或旧模块路径 | `ci.yml` security 作业统一口径 | 有**调用链**的可利用漏洞（Error 级）阻断 |
| 依赖审计（前端） | pnpm audit | 锁定 pnpm 版本 | `ci.yml` Frontend CI「Dependency audit」步骤：`pnpm audit --audit-level=high`（advisory 端点固定官方 npm registry） | High 及以上阻断，**不得**以 `\|\| true` 吞错 |
| DAST | OWASP ZAP | 固定镜像 tag + baseline 脚本入库 | 每夜对 staging 跑 baseline scan；全量 scan 每迭代 | baseline 阻断（误报人工分诊后移入豁免单） |
| Secret 扫描 | gitleaks | 固定版本 + 全仓 `.gitleaks.toml` | PR diff 扫描 + 每周全量扫描 | Error 阻断；历史误报以 allowlist 显式登记 |
| SBOM | Syft | 固定版本，输出 SPDX JSON | 随 release 产出并归档（制品清单见 10.1） | 非阻断（产物完整性由 release job 断言） |
| 容器/镜像扫描 | Trivy（image） | 固定版本 + 漏洞库快照日期 | release 前扫描 `guliangiam:v{version}` | Critical/High 的**可修复**（有补丁版本）漏洞阻断 |
| IaC 扫描 | Trivy（config）或等价 | 固定版本 | 扫描 `Dockerfile`、`k8s.yaml`（13.2/13.3 清单） | Error 阻断 |

**版本锁定红线**：所有 GitHub Action 必须以固定 tag 或 commit SHA 引用（`actions/checkout@v4` 类主版本 tag 允许，`@master`/`@main` 禁止）；工具版本变更必须同步附录 I 登记表；工作流中不得存在无效命令与吞错后缀。

### 9.10 漏洞修复 SLA

| 级别 | 修复时限 | 时钟起点 | 逾期处理 |
|------|---------|---------|---------|
| Critical | **7 天** | 漏洞确认之日（依赖类＝有调用链的可利用确认） | 直接进入发布冻结例外流程（下述第 4 条） |
| High | **30 天** | 同上 | 迭代评审追责，纳入下个迭代首项 |
| Medium | **90 天** | 同上 | 迭代评审跟踪 |
| Low | **下个迭代** | 同上 | 常规 backlog |

补充规则：

1. **修复即验证**：修复 PR 必须附复现用例（测试或扫描前后对比），并注明对应的扫描器输出（gosec/GZ-xxx、govulncheck GO-xxxx-xxxx、ZAP alert id）。
2. **豁免机制**：确因业务不可修复（如遗留协议兼容）时，按附录 G 豁免单格式申请（条款/理由/替代补偿/有效期/审批人），**Critical 不允许豁免**；豁免单必须登记责任人（个人而非团队）与到期复审日期，到期未复审视为违规（附录 G 处理级别）；豁免清单在本节维护，逾期豁免自动失效。
3. **依赖升级窗口**：patch 类安全升级随时可合（5.2 patch 原则）；minor 类安全升级按季度窗口集中处理；major 类走 RFC。**版本冻结期（release 流程 17.3 第 1 步起）内，安全类升级不受冻结限制**，走 hotfix 分支。
4. **与门禁联动**：17.2「安全扫描 0 Critical/High」为强制门禁；SLA 逾期未修复也未豁免的漏洞视为门禁失败，阻塞下一次 release（17.3 第 6 步安全复审不通过）。

### 9.11 越权测试矩阵（水平 / 垂直 / 跨租户）

**规则**：**每个管理面端点（4.1.3 management 面）必须有负向用例**；负向用例按下列三维矩阵组织，缺失任一维度的端点视为测试覆盖不完整（门禁联动见 8.9 与 17 章）。

**矩阵模板**（预期码一律引用错误码注册表 §1 的错误码→HTTP 映射，不得自造）：

| 端点 | 操作 | 同租户他用户（水平 IDOR） | 他租户主体（跨租户 IDOR） | 无权限角色（垂直） | 预期码 |
|------|------|--------------------------|--------------------------|-------------------|--------|
| `/api/v1/users/{id}` | GET | 用户 A 读取同组织用户 B 详情 | 租户 X 的管理员读取租户 Y 的用户 | 已登录但无 `users:read` 的角色 | 水平/跨租户：404 `not_found`（「不属于当前租户」归 404，错误码注册表 §1）；垂直：403 `forbidden` |
| `/api/v1/roles/{id}` | DELETE | 同租户无角色管理权限的用户删除 | 租户 X 管理员删除租户 Y 的角色 | — | 跨租户：404 `not_found`；垂直：403 `forbidden` |

填写约定：

1. 「同租户他用户」维验证**对象级**授权（拥有功能权限但非资源属主）；「他租户」维验证**租户隔离**（`organization_id` 归属校验与 `TenantFilter` 严格模式）；「无权限角色」维验证**功能级**授权（Casbin 策略，fail-close 见 AC-PERM-001）。
2. 协议面端点不适用本矩阵（其鉴权模型由 RFC/OASIS 规定），但**必须**有对应协议负向用例（9.13）。
3. 矩阵与 `route.Definition.Permission` 声明同源生成（由 `scripts/gen_openapi.py` 扩展导出矩阵骨架），禁止手工另维护一份端点清单。
4. 与验收标准衔接：**AC-PERM-001**（fail-close）、**AC-PERM-003**（受保护路由必须声明权限）、**AC-PERM-004**（公开白名单精确到方法+路径模板）、**AC-TENANT-001**（租户隔离）、**AC-TENANT-002**（跨租户资源）。
5. 矩阵条目数与 `openapi/openapi.yaml` 的管理面 operation 数对账一致；CI 汇总负向用例计数并可按模块下钻（8.9 门禁表）。

### 9.12 Go Fuzz 测试范围与门禁

**范围**（下列入口均为不可信输入解析点，必须建立 fuzz 目标）：

| # | fuzz 目标 | 入口域 | 风险 |
|---|----------|--------|------|
| 1 | SCIM filter 表达式解析 | `internal/infra/postgres/scim`（filter → SQL 映射，9.1 白名单规则） | 注入、panic、越权列访问 |
| 2 | 排序字段解析 | 2.8.2 `buildOrderBy` 白名单链路 | 白名单绕过 → SQL 注入 |
| 3 | 协议参数 | `redirect_uri` 校验（`isSafeRedirectUrl`）、DPoP proof 解析（RFC 9449，ADR-014） | 开放重定向、JWT/proof 结构崩溃 |
| 4 | CSV/JSON 解析器 | 用户导入导出（2.9 Excel/CSV、附录 B `users/import`） | 畸形输入 panic、公式注入（CSV 导出） |

**命名与组织**：目标函数命名 `Fuzz<被测函数>`，置于被测包内 `fuzz_test.go`；种子语料提交 `testdata/fuzz/<FuzzName>/`；发现的崩溃语料必须固化为回归用例后才能修复关闭。

**运行门禁**：PR 触发短时 fuzz（每目标 ≥ 30s，仅对触碰包）；每日定时长 fuzz（每目标 ≥ 5min）；CI job 归属见 8.9 联动表。崩溃即阻塞合并。

### 9.13 协议合规测试

IAM 的协议面（4.1.3 六前缀家族）以标准符合性为交付物的一部分，**协议合规测试纳入 E2E 层级**（8.2 表；随 17.3 发布流程第 4 步烟雾测试/第 5 步性能测试一同执行，作为发布门槛的组成部分）：

| 协议 | 合规目标 | 用例来源 | 衔接 |
|------|---------|---------|------|
| OIDC | openid.net 官方认证套件通过并申请 **certified** | OpenID Conformance Suite（Basic / Config / Form Post 客户端认证路径 + discovery + JWKS） | AC-OAUTH-001、AC-TOKEN-001/002 |
| SAML | 与主流 IdP 互操作矩阵（至少覆盖 Okta / Microsoft Entra ID / ADFS 三类元数据形态；CI 内以开源 IdP 容器常驻 2 家） | SP 发起与 IdP 发起、ACS/SLO、签名与 RelayState（9.1 HMAC 规则） | AC-PROTO-001 |
| SCIM | RFC 7644 用例集（filter、`startIndex`/`count` 分页、PATCH op、`/Schemas` discovery、Enterprise 扩展） | RFC 7644 附录 + 服务商自检清单 | AC-PROTO-002 |
| CAS | 票据用例（ST/PT/P3 属性、`proxyValidate`、`samlValidate`） | CAS 协议 3.0 规范 | AC-PROTO-003 |

执行约定：

1. **权威前缀**：合规用例按 4.1.3 定义的权威前缀编写（CAS/SAML 权威族 = 根级家族）；旧兼容前缀的合规用例转为重定向断言。
2. 合规测试发现的偏差按 9.10 SLA 定级（协议漏洞默认 High 起）。
3. OIDC certified 认证属商业化交付项，其结果（通过/未通过 + 套件版本）登记入 10.1 制品清单。

### 9.14 安全响应头全集

> **与 4.17 的分工**：CSP 的目标基线与 nonce 机制**只**在 4.17.1 维护，本节不重复 CSP 内容；本节登记 CSP **之外**的响应头基线，两者共同构成安全响应头全集。

| 响应头 | 目标值 | 适用范围 | 说明 |
|--------|--------|---------|------|
| `Cross-Origin-Opener-Policy` | `same-origin-allow-popups` | 全部 HTML 响应 | OAuth/SAML 弹窗流程需要 popups 互通；隔离 window.opener 引用 |
| `Cross-Origin-Embedder-Policy` | `credentialless`（经评估后可降为不发送） | 全部 HTML 响应 | 启用后跨域子资源不再携带凭证；启用前必须完成第三方子资源清单评估 |
| `Cross-Origin-Resource-Policy` | `same-origin`（用户上传内容按 4.17.2 附件策略） | 全部资源响应 | 防 `no-cors` 跨域嵌入敏感资源（Spectre 类侧信道缓解） |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=(), payment=()` | 全部 HTML 响应 | IAM 控制台无需上述能力；WebAuthn 使用 `publickey-credentials-*` 时按需放行并在 PR 说明 |
| `Cache-Control: no-store, no-transform` | — | **令牌端点、个人数据端点、授权端点响应**（access/refresh token 签发与内省、用户详情含 D2/D3 字段、`/api/v1/auth/*` 会话端点） | 防中间层/浏览器缓存令牌与 PII；`no-transform` 防代理改写体破坏签名/HMAC |
| `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Strict-Transport-Security`、`Referrer-Policy` | 维持 9.1 既有值 | 全部响应 | 由 SecurityHeaders 中间件统一实现 |

**落地要求**：SecurityHeaders 中间件统一扩展上述头；令牌/个人数据端点以 route 元数据声明「敏感响应」并自动追加 `Cache-Control: no-store, no-transform`。验收＝对 `/oauth2/userinfo`、`tokens:exchange`、任一含 `idCard` 字段的端点验证五类新头与 no-store 齐备；HTML 响应带 COOP/Permissions-Policy。

### 9.15 TLS 基线与证书到期监控

| 项 | 基线 | 说明 |
|----|------|------|
| 最低版本 | **TLS 1.2**；**TLS 1.3 优先**（配置为 `MinVersion: TLS1.2` 且优先协商 1.3） | 终止点在 Ingress（13.3：cert-manager + nginx 支持 TLS 1.3）；应用内直连场景（如自托管部署）同基线 |
| 套件白名单（1.3） | `TLS_AES_128_GCM_SHA256`、`TLS_AES_256_GCM_SHA384`、`TLS_CHACHA20_POLY1305_SHA256` | 1.3 套件本就受限，白名单为断言用途 |
| 套件白名单（1.2） | 仅 `ECDHE` 密钥交换 + AEAD（`AES_128_GCM`/`AES_256_GCM`/`CHACHA20_POLY1305`） | **禁止** CBC、3DES、RC4、静态 RSA、SHA-1 签名套件 |
| HSTS | 由 SecurityHeaders 下发（9.1）；`max-age` ≥ 15552000 且先做 1 周灰度 | 防 SSL 剥离 |
| 证书签发 | cert-manager 自动（13.7：60 天周期）+ `ACME_ENABLED` 自托管路径（环境变量注册表 D.9） | 自动化不豁免监控（见下行） |
| 到期监控链 | **30 天（信息级）/ 14 天（中）/ 7 天（高）/ 1 天（严重）** 四级告警，接入 12.6 告警四级词汇与通知矩阵 | 必须监控**全部**终止点证书（Ingress、自托管 TLS、数据库客户端证书）；自动续期失败是静默故障，监控链是兜底 |
| 传输层 mTLS（内网服务间） | 服务间调用（含数据库/Redis/出站 HTTP 下游）启用双向 TLS，内部 CA 签发，短期证书（≤ 24h）轮换 | 见下方 mTLS 澄清第 (a) 条 |

**mTLS 两层澄清（评审红线）**：

1. **传输层 mTLS**（本节）：内网服务间/南北向管理员通道的双向 TLS，属传输加密基线。
2. **令牌绑定 mTLS**（RFC 8705）：Access Token 以 `cnf.x5t#S256` 绑定客户端证书，属**令牌发送方约束**，按 **ADR-014**（`docs/adr/ADR-014-token-binding.md`）落地。ADR-014 红线：`applications.ClientCert` 字段是应用联邦 SSL/SAML 配置属性，**不是**令牌绑定，**不得据此声称 mTLS 已支持**。两者不得混用表述；外部文档引用 mTLS 时必须写明指哪一层。

### 9.16 PII 资产清单、数据分级与 DPIA

> **分级符号说明**：本节数据密级采用 **D0–D3**。第 11.2 节的 **L0–L3 是性能测量层级**（纯函数/数据库/HTTP/多实例），与本节语义无关，两套符号不得互串。

**数据分级定义**：

| 密级 | 定义 | 本仓示例 | 存储与传输要求 |
|------|------|---------|---------------|
| D0 公开 | 可对外发布 | 产品名、i18n 文案 | 无特殊要求 |
| D1 内部 | 泄露无害但不宜公开 | 组织配置、菜单结构 | 内部访问控制 |
| D2 个人数据（PII） | 可识别自然人的数据 | 手机号、邮箱、姓名、IP、user_agent、登录记录 | 9.5 脱敏规则 + 19.4 保留期 + 审计 |
| D3 高敏 | 凭据、密钥、生物特征、证件号 | 密码哈希、TOTP 种子、私钥、`idCard`、MFA Secret | 第 14 章加密/哈希 + 密钥管理红线（14.4） |

**PII 资产清单**（模板 + 已知锚点；新增 D2/D3 处理点必须登记，威胁建模（9.8）引用本清单）：

| 资产（表/列） | 密级 | 处理规则锚点 |
|--------------|------|-------------|
| `users` / `user_profiles`（姓名、邮箱、手机号） | D2 | 9.5 掩码；`idCard` 必须在 `SecretFields` 清单显式覆盖（9.5） |
| `user_credentials`（密码哈希） | D3 | Argon2id（14.5）；永不返回 |
| 字段级加密列（如 `user_mfa_config.totp_secret`、`certs.private_key`、`keys.private_key`、`organization_auth_config.master_password`；完整清单以 `internal/platform/crypto/reencrypt_job.go` 的 `DefaultEncryptedColumns` 与 `contracts/registries/crypto.yaml` 为准） | D3 | 第 14 章字段加密（AES-256-GCM 信封，14.2）；备份/副本按同密级管理 |
| `records`（ip、user_agent、metadata、surface、request_id、trace_id） | D2 | 12.5 审计字段；保留期 19.4（最短 1 年）；`surface`/`request_id`/`trace_id` 列可按列直查（12.11/12.12） |
| `entries`（登录条目） | D2 | 19.4：默认 180 天分批删除 |
| API 响应中透出的 D2/D3 字段 | D2/D3 | 9.5 SecretFields 清单与日志脱敏清单为唯一事实源 |

**DPIA（数据保护影响评估）流程**：

| 环节 | 要求 |
|------|------|
| 触发条件 | ① 新增 D3 数据类别或新增 D2 处理用途；② 新增向第三方/跨境传输 D2 的通道（新 Provider/同步器/Webhook 消费方）；③ 数据保留期变更；④ 新增对 D2 的批量导出（如 Excel 导出、SCIM 全量读取） |
| 评估模板要点 | 处理目的与合法基础 → 数据流图（入口/存储/出口）→ 对 9.8 威胁清单的引用 → 风险评估（泄露/越权/留存超期）→ 缓解措施（脱敏、加密、保留期、最小化）→ 残余风险与接受人 |
| 审批 | 安全负责人 + 架构评审委员会（与 7.1 安全评审同节点）；未通过不得开工 |
| 复审 | 每年例行复审一次；触发条件再次命中时即时复审；豁免类缓解措施随豁免单到期复审（9.10 第 2 条） |

与既有章节的互链：脱敏执行细则见 **9.5**；生命周期与隐私擦除（30 天内完成）见 **19.4**；审计字段与事件脱敏红线见 **12.5**。

### 9.17 合规映射工程条款

> 本节把商业化合规要求落为工程验收口径。保留期类参数落地时必须回写 19.4 生命周期表（两侧同步纪律）；任何保留期变更触发 DPIA 复审（9.16）。

| 合规项 | 工程要求 | 状态与锚点 |
|--------|---------|-----------|
| 审计保留期 | `records` 商业合规 **≥ 3 年** + 分级归档：热（库内可查）→ 温（对象存储归档，签名 + 清单校验和）→ 冷（WORM 链段归档，归档不改写链列，链段闭环前禁止物理删除）；本体不设定期删除、默认最短 1 年（19.4），保留期参数落地时回写 19.4 | 19.4；[observability §1.4](architecture/observability.md#14-audit审计) |
| 安全日志留存 | 安全类事件（12.5 中高/严重级：`auth.login.failed`、`token.reuse_detected`、`authz.casbin.enforce_error`、`authz.cross_tenant_denied`、`key.*` / `credential.*` 等）对应日志留存 **≥ 180 天**；审计事实以 `records` 为准（不可关闭），日志为辅助面，脱敏按 12.4/9.5 | 12.5 |
| 密码模块 | 国密能力位（[security-model §2.21](architecture/security-model.md#221-国密能力位与密码学边界adr-031)，ADR-031 决策 4）：SM2（GB/T 32918，签名/证书）、SM3（GB/T 32905，摘要）、SM4（GB/T 32907，字段加密，与 AES-256-GCM 同级信封口径）注册入 8.6 算法白名单与 `FieldEncryptor` 加密器抽象，**能力缺失时拒绝启用（硬失败），禁止静默降级**；SM4 信封的独立验证以 GM/T 标准向量或第三方实现离线比对为出口证据，不设第二实现冒烟（国密实现仅 `gmsm`）；FIPS 模式作为可选运行模式，与能力位同面披露（`/health` 能力矩阵） | [security-model §2.21](architecture/security-model.md#221-国密能力位与密码学边界adr-031)；ADR-031；8.6 |
| 数据出境/驻留 | 租户驻留绑定 `organizations.region_code` → `deployment_regions` 注册表（控制面仅元数据，个人数据与派生数据不出区域，PRIV-010）；**批量导出通道（Excel 导出、SCIM 全量读取、证据包导出）必须内置导出前检查钩子**：目标接收方区域与租户 `region_code` 不一致即拒绝并留审计；新增导出通道触发 DPIA（9.16 触发条件 ④） | [multi-region §1](architecture/multi-region.md#1-区域桶模型dr-001adr-031)；9.16 |
| 参数登记 | 本节新增配置项（保留期、留存天数、FIPS 模式开关、驻留检查钩子开关等）实现时必须登记环境变量注册表 D.12，禁止散落未登记变量（与 14.7 红线 5 同纪律）；未经登记与评审的保留期默认值不得上线 | 环境变量注册表 D.12；14.7 |

### 9.18 风险/信号数据隐私边界

> 风险管线（[security-model §3](architecture/security-model.md#3-风险引擎与持续访问评估adr-027)，ADR-027）引入新的个人数据处理面；本节划定隐私边界。数据密级沿用 9.16 的 **D0–D3**（与 11.2 的 L0–L3 性能测量层级无关，两套符号不得互串）。

| 边界 | 规范 | 锚点 |
|------|------|------|
| 密级归类 | 风险信号输入（IP、设备指纹、地理速度、ASN、登录模式）为 **D2**；`risk_decisions.hit_signals` 必须为脱敏 JSON（信号类别 + 命中与否），**不含信号原文**（RISK-001 验收） | 9.16；[security-model §3](architecture/security-model.md#3-风险引擎与持续访问评估adr-027) |
| 哈希化存储 | 设备指纹与 IP 落库一律先**不可逆哈希**（指纹哈希不可逆为风险信号采集环节红线）；展示与导出走 9.5 脱敏规则 | [security-model §3](architecture/security-model.md#3-风险引擎与持续访问评估adr-027)；9.5 |
| 保留期 | 风险信号原文/原始指纹保留 **≤ 90 天**（到期分批物理删除，满足 19.4 清理约束）；`risk_decisions` 决策记录（可解释性需要）按 19.4 审计口径保留；落地时同步登记 19.4 新行 | [security-model §3](architecture/security-model.md#3-风险引擎与持续访问评估adr-027)；19.4 |
| 用户查询权（DSAR） | 隐私导出必须包含**风险画像摘要**：最近决策级别、时间、命中信号类别清单；不含信号原文、不含可逆向评分引擎的权重细节；用户隐私擦除（30 天，19.4）联动 `risk_decisions` 匿名化（`subject_ref` 置空 + 保留决策骨架） | 19.4；9.16 |

---

## 10. 项目交付与文档规范

### 10.1 必交付制品清单

| 制品 | 路径 | 说明 |
|------|------|------|
| Go 二进制 | `guliangiam-server` | 主服务可执行文件 |
| 前端静态资源 | `web/dist/` | SPA 构建产物 |
| Docker 镜像 | `Dockerfile` | 多阶段构建；pnpm 版本以 `web/package.json` 为准 |
| K8s 配置 | `k8s.yaml` | 部署清单（含 HPA + PDB） |
| OpenAPI 规范 | `openapi/openapi.yaml` | API 文档（自动生成） |
| Schema SQL | `db/postgresql/V*.sql`（版本链以目录为准，复现：`ls db/postgresql/V*.sql`） | 数据库初始化与增量迁移 |
| 环境配置模板 | `.env.example` | 配置说明 |
| 需求规格 | `docs/requirements/functional.md` | 需求文档 |
| 设计说明 | `docs/design/README.md` | 设计文档 |
| 开发规范 | `docs/GuliangIAM-开发规范.md` | 规范文档（本文件） |
| 架构说明 | `docs/architecture/README.md` | 架构文档 |
| 部署指南 | `docs/operations/README.md` | 部署文档 |
| 密钥轮换 | `docs/architecture/security-model.md` | 密钥管理 |
| 无障碍规范 | 本手册第 15 章 | 开发与评审规则 |
| 性能基线 | `docs/architecture/observability.md` | 性能测量方法 |
| 发布验证 | CI 工作流与发布记录 | 当前提交验证结果 |

### 10.2 制品版本命名规则

| 制品 | 规则 | 示例 |
|------|------|------|
| Git Tag | `v{major}.{minor}.{patch}` | `v1.2.0` |
| Docker Image | `guliangiam:v{version}` | `guliangiam:v1.2.0` |
| Release | `release/v{version}` 分支 | `release/v1.2.0` |

### 10.3 文档存放与维护位置

| 文档 | 位置 | 维护者 |
|------|------|--------|
| 需求规格 | `docs/requirements/functional.md` | 产品/架构 |
| 设计说明 | `docs/design/README.md` | 架构 |
| 开发规范 | `docs/GuliangIAM-开发规范.md` | 架构 |
| 架构说明 | `docs/architecture/README.md` | 架构 |
| 部署指南 | `docs/operations/README.md` | DevOps |
| 密钥轮换 | `docs/architecture/security-model.md` | 安全 |
| API 参考 | `openapi/openapi.yaml` | 后端（自动生成） |
| README | `README.md` | 全体 |
| 安全策略 | `SECURITY.md` | 安全 |

---

## 11. 性能与 SLO 规范

### 11.1 服务等级目标（SLO）

| 指标 | 目标值 | 测量方式 | 违反后果 |
|------|--------|---------|---------|
| API P50 响应时间 | < 100ms | Prometheus `http_request_duration_seconds` | 告警 |
| API P99 响应时间 | < 500ms | Prometheus P99 | 严重告警 |
| 登录认证响应 | < 100ms（认证面 P99，不含外部 IdP） | OpenTelemetry Trace | 告警 |
| 并发用户数 | ≥ 1000 | k6 阶梯加压 | 容量评估 |
| 列表查询（50条/页） | < 200ms | Prometheus | 告警 |
| Session 恢复 | < 20ms | OpenTelemetry | 告警 |
| JWT 签发 | < 1ms（P99） | OpenTelemetry | 告警 |
| Casbin Enforce | < 2ms（P99） | Casbin 内部计时 | 告警 |
| 可用性（**管理面** management） | **99.5%（月度）** | 分面健康检查 / SLI：`/api/v1/*`（排除认证面、协议面）5xx 率与成功率 | 严重告警 |
| 可用性（**认证面** authentication） | **99.9%（月度）** | 分面 SLI：`/api/v1/auth/*` 页面与会话类端点成功率 | 严重告警 |
| 可用性（**协议面** protocol） | **99.9%（月度）** | 分面 SLI：协议面端点（家族清单见 `contracts/registries/surfaces.yaml`，判据见 4.1.3）成功率 | 严重告警 |
| 错误率（5xx） | < 0.1% | Prometheus（按 `surface` 标签分面统计） | 告警 |

> **可用性口径唯一权威为 generated/slo-registry.md**：管理面 99.5%、认证面 99.9%、协议面 99.9%（与 4.1.3 三面表一致；本表数值为该注册表的展示投影，冲突以注册表为准）；月度统计、健康检查/SLI 必须按面分别计算，不得再用笼统的单一「99.9%」概括全部接口。他处引用可用性指标时统一引用 generated/slo-registry.md。按面分桶直接用 `surface="management"` 等选择器实现（见 4.1.3 落地清单第 5 项与 12.2）。

> 上表为 **SLO 目标**，必须由 **L1/L2**（真实数据库 / 真实 HTTP）测量支撑；L0 纯函数基准**不得**用于宣称 SLO 达标（见 11.2 与《architecture/observability.md》）。

### 11.2 性能基线（本地 benchmark 结果）

> **重要口径（见《architecture/observability.md》第 1 章）**：下表全部为 **L0（纯函数 / 纯内存 Benchmark）** 数据，来源是 `go test -bench`，**不得**代表 API 端到端延迟。以 `19ns` 这类 L0 数值去对照 `< 20ms` 的 API 级 SLO 是无效比较。
>
> L1（真实数据库）与 L2（真实 HTTP）测量在基线文档中单列，SLO 结论**只允许**引用 L1/L2。
>
> **层级补全（L0/L1/L2/L3）**：**L3 = 多实例 / 故障注入 / 恢复 / 峰值流量场景级测量**（k6 阶梯/峰值加压 + 故障注入，如杀副本、断库、网络分区），用于验证多实例下的分布式一致性与故障恢复，并作为发布门槛；定义与方法以《architecture/observability.md》为准（附录 J 亦引用 L0/L1/L2/L3 四级）。11.3 的负载/压力/浸泡测试是 L3 的组成部分。

| 操作（L0 纯函数/内存） | 实测值 | 备注 |
|------|--------|---------|
| Session **结构体创建** | 19.62ns | `BenchmarkSessionCreation`；**不是** Session 恢复链路，也不构成 API 级 SLO |
| JWT 签发 | 1.19ms | L0 回归项，**不得**与 API 级 SLO 比较（SLO 结论只引 L1/L2/L3） |
| JWT 验证 | 37μs | - |
| Casbin Enforce（缓存命中） | 539ns | - |
| Casbin Enforce（缓存未命中） | 494ns | - |
| SHA-256 Token 哈希 | 256ns | - |
| AES-256-GCM 加密 | 525ns | - |
| AES-256-GCM 解密 | 181ns | - |
| UUIDv7 生成 | 399ns | - |

### 11.3 性能测试要求

| 测试类型 | 工具 | 频率 | 说明 |
|----------|------|------|------|
| 单元基准 | Go `testing.B` + `-benchmem` | 每次 PR | 覆盖热路径 |
| 集成基准 | k6 | 每周 | 真实 DB 环境 |
| 负载测试 | k6 | 每次发布前 | 1000 并发 |
| 压力测试 | k6 | 每季度 | 找容量拐点 |
| 浸泡测试 | k6 | 每半年 | 24 小时稳定运行 |

### 11.4 性能优化红线

以下情况必须优化后才能合并：

| 红线 | 检查方式 |
|------|---------|
| 热路径（认证/授权/JWT）新增 > 10ms | Benchmark 对比 |
| 新增 N+1 查询 | Code Review |
| 无索引的全表扫描 | EXPLAIN 分析 |
| 内存分配 > 100 allocs/op（热路径） | `-benchmem` |
| 新增 > 10MB 内存驻留 | 内存 profile |

### 11.5 数据库性能规范

| 规则 | 说明 |
|------|------|
| 慢查询阈值 | > 100ms 记录到日志 |
| EXPLAIN 分析 | 新增复杂查询必须附 EXPLAIN 输出 |
| 索引覆盖 | 高频查询必须有覆盖索引 |
| 分区表 | 大表按时间分区（如 `access_tokens`） |
| 连接池 | pgxpool min/max 配置合理，避免连接泄漏 |
| 批量操作 | 使用 `UNNEST` 替代循环 |
| 预估计数 | Dashboard 避免 `COUNT(*)` 全表扫描 |

### 11.6 SLI 定义与统计窗口

> 本节为 11.1 分面可用性目标的 **SLI 精确公式**：SLO 数值不改（管理面 99.5% / 认证面 99.9% / 协议面 99.9%，4.1.3 三面表与 ADR-020 合同目标）；本节只回答「怎么算」。分母选择器直接用 `surface="management"` 等；公式与窗口不变。

**统计窗口**：全部 SLI 统一采用 **28 天滚动窗口**（28×24×60 = 40320 分钟），与 [deployment.md §2.3](operations/deployment.md#23-k8s-生产清单-8-项上线门槛) ServiceMonitor「保留 28d 滚动窗口」同口径。11.1 表中「月度」视为**对外汇报周期**（展示层）；SLO 判定与错误预算消耗一律以 28 天滚动窗口计算（与 ADR-020 合同目标不冲突：数值口径 99.5/99.9/99.9 不变，仅统计窗口精确化）。

**采样口径**：全部 SLI 以**服务端遥测**为准（Prometheus 指标 + 服务端访问日志），不采客户端 SDK 数据；前端体验另行度量（12.13 RUM，不参与 SLO 判定）。指标按 `surface`/路由模板分面，不得跨面混合计算。

**SLI 分子/分母定义表已迁至 `generated/slo-registry.md`（SLI-1 / SLI-2 / SLI-2a / SLI-3），本节不重复列表。** 延迟类指标（P50/P99、Session 恢复、JWT 签发等）沿用 11.1 表「测量方式」列的口径，不重复定义。

### 11.7 分面 SLO 单表（引用）

分面 SLO / 错误预算唯一口径已迁至 `generated/slo-registry.md`；本节仅保留指向。三面 SLO 数值（management 99.5% / authentication 99.9% / protocol 99.9%）、SLI 编号对应、28 天滚动窗口错误预算精确换算（`预算分钟数 = 40320 × (1 − SLO)`）与度量来源，均以该注册表单表为准。任何文档引用错误预算分钟数时必须以注册表为准，不得使用按自然月（30d/43200min）或其他窗口换算的数字。

### 11.8 错误预算策略

**预算追踪**：按 `generated/slo-registry.md` 单表逐面追踪 28 天滚动错误预算的剩余量；预算消耗事件（分钟数、面、事故链接、恢复时间）记入错误预算台账。

**阈值动作**：各级预算剩余（< 50% 预警 / < 25% 收紧 / ≤ 0 耗尽冻结）对应的动作、冻结范围与豁免批准要求见 `generated/slo-registry.md`「错误预算阈值动作」表。

**发布流程引用点（17.3 表）**：第 1 步「版本冻结」时发布经理必须核对各分面错误预算余量，耗尽面不得纳入非修复类变更；第 7 步「生产发布」前 DevOps 复核发布期间预算余量未回退到冻结线以下。

**恢复条件**：预算随 28 天滚动窗口自然回补（历史坏时段滑出窗口即回补），**禁止人工清零**；若因持续事故导致窗口内无法自然恢复，由周会评审临时上调错误预算消耗容忍（登记决策与期限，最长 4 周）。

**豁免**：安全应急（漏洞修复、`token.reuse_detected` 处置）与合规强制变更不受冻结限制，但须事后 1 个工作日内在台账补登记。

---

## 12. 可观测性规范

### 12.1 三支柱规范

| 支柱 | 技术 | 规范 |
|------|------|------|
| 日志 | `slog` + JSON | 结构化、含 trace_id、敏感字段脱敏 |
| 指标 | Prometheus | 命名规范、标签规范、见 12.2 |
| 追踪 | OpenTelemetry | 采样策略、Span 命名、见 12.3 |

### 12.2 指标命名与标签规范

**命名规范**：`{namespace}_{subsystem}_{name}_{unit}`

| 指标 | 类型 | 说明 |
|------|------|------|
| `guliangiam_http_requests_total` | Counter | 请求总数 |
| `guliangiam_http_request_duration_seconds` | Histogram | 请求耗时 |
| `guliangiam_http_requests_in_flight` | Gauge | 并发请求 |
| `guliangiam_auth_login_total` | Counter | 登录计数 |
| `guliangiam_auth_login_errors_total` | Counter | 登录错误 |
| `guliangiam_db_query_duration_seconds` | Histogram | DB 查询耗时 |
| `guliangiam_casbin_enforce_duration_seconds` | Histogram | 授权耗时 |
| `guliangiam_token_issued_total` | Counter | Token 签发 |
| `guliangiam_secret_rotation_total` | Counter | Secret 轮换 |
| `guliangiam_encryption_fallback_total` | Counter | 加密回退（密钥轮换期间） |

**标签规范**：

- 禁止高基数标签（user_id、session_id 等）
- 推荐标签：`method`、`path`、`status`、`org_id`（基数可控时）、`surface`（取值仅限 `management` / `authentication` / `protocol`，见 4.1.3）
- 路径标签使用路由模板（`/api/v1/users/{id}`）而非实际路径

> **`surface` 标签**：`http_requests_total` 的标签集为 `{method, path, status, surface}`，`http_request_duration_seconds` 为 `{method, path, surface}`（`internal/platform/middleware/prometheus.go`）。取值由 `route.SurfaceFor(routePattern)` 从**路由模板**派生——与 `internal/bootstrap/surface_contract_test.go` 钉死 `route.Definition` 的同一分类器同源，因此标签不可能与三面契约不一致。`surface` 只有三个取值，基数可控。

### 12.3 追踪规范

| 规则 | 说明 |
|------|------|
| 采样 | 生产环境 1% 基础采样，错误 100% 采样 |
| Span 命名 | `{http.method} {route_template}` 或 `{package}.{operation}` |
| 关键 Span | HTTP 入口、DB 查询、外部调用、Casbin Enforce、JWT 签发 |
| 属性注入 | `user.id`、`org.id`、`session.id`、`error.type` |
| 上下文传播 | 使用 W3C Trace Context 标准头 |

### 12.4 日志规范

| 字段 | 必填 | 说明 |
|------|------|------|
| `timestamp` | 是 | ISO 8601 |
| `level` | 是 | DEBUG/INFO/WARN/ERROR |
| `message` | 是 | 人类可读描述 |
| `request_id` | 是（HTTP 请求） | 请求唯一 ID |
| `trace_id` | 是（HTTP 请求） | 追踪 ID |
| `span_id` | 否 | Span ID |
| `error` | 否 | 错误详情 |
| 业务字段 | 否 | 结构化上下文 |

**禁止**：记录密码、Token、密钥、`idCard`、完整请求体（含敏感字段）。

### 12.5 IAM 安全事件分类

**目的**：把 IAM 关键动作归入固定的安全分类，每类使用统一事件名、必填字段和固定的告警响应级别。**事件域与事件名的唯一登记处是 `contracts/registries/events.yaml`**（渲染层 `docs/generated/event-registry.md`）；下表仅为告警分类视角的示例投影，事件名集合以注册表为准。新增任何与身份/授权/密钥相关的关键路径时，必须先在 `events.yaml` 登记事件名（命名遵循 3.2 与命名裁决 N13）再使用，**不得**发明临时事件名。

| 安全分类 | 已登记事件示例（以 events.yaml 为准） | 固定严重级别 | 主要写入位置 |
|---|---|---|---|
| 认证事件 | `auth.login.success`、`auth.login.failed`、`auth.challenge.issued`、`auth.step_up.completed`、`auth.logout`、`session.created`、`session.hijack_suspected` | 信息（`auth.login.failed`/`session.hijack_suspected` 为高） | `records` + 登录指标 |
| 令牌与协议事件 | `token.created`、`token.revoked`、`protocol.token.issued`、`protocol.token.revoked`、`protocol.authorize.denied`；refresh token 重用检测事件 `token.reuse_detected`（代码锚点 `internal/app/tokens/refresh_token.go`） | `token.reuse_detected` 为**严重**；其余中/高 | `records` + `guliangiam_token_issued_total` |
| 授权与策略事件 | `authz.allow`、`authz.deny`、`authz.casbin.enforce_error`（fail-close）、`authz.casbin.init_fail`、`authz.cross_tenant_denied`、`policy.changed`、`control.policy_changed` | `authz.casbin.enforce_error` / `authz.cross_tenant_denied` 为高 | `records` + `guliangiam_casbin_enforce_duration_seconds` |
| 身份治理事件 | `organization.created`、`organization.deleted`、`user.created`、`user.deleted`、`user.role_binding.removed`、`user.group_membership.removed`、`user.impersonation.started`、`user.impersonation.ended` | `user.impersonation.*` 与 `user.deleted` 为高 | `records` |
| 密钥与凭据事件 | `key.created`、`key.updated`、`key.disabled`、`key.deleted`、`credential.created`、`credential.rotated`、`credential.revoked`、`secret.rotated`、`application.client_secret.rotated` | 密钥轮换超时/加密回退告警为中（指标驱动，见 12.6） | `records` + `guliangiam_secret_rotation_total` / `guliangiam_encryption_fallback_total` |
| 管理与运维事件 | `org.settings.updated`、`webhook.created`、`webhook_event.failed`、`actions.execution.failed` | 中（认证失败风暴为高） | `records` + 告警 |

**必填字段（所有类共通）**：

| 字段 | 必填 | 说明 |
|---|---|---|
| `occurred_at` | 是 | UTC 时间（`occurred_at` / 表列 `created_at`） |
| `surface` | 是 | `management` / `authentication` / `protocol`（见 4.1.3；`records.surface` 列承载，值由服务端从请求元数据填充，不接受客户端传参） |
| `event_type` | 是 | `contracts/registries/events.yaml` 登记的事件名（如 `auth.login.failed`） |
| `outcome` | 是 | `success` / `failure`/`denied` |
| `org_id` | 是（全局事件除外） | 租户范围 |
| `actor_id` | 否 | 操作主体；系统触发时为空 |
| `reason_code` | 失败时必填 | 使用附录 C 的错误码，不得自造 |
| `request_id` / `trace_id` | HTTP 路径必填 | 关联日志与追踪 |
| `ip` / `user_agent` | 是 | 审计取证需要 |

**红线**：

1. 事件名一经使用即为契约，不得静默改名；新增与废弃必须在 `contracts/registries/events.yaml` 登记，变更历史记录于 `CHANGELOG.md`。
2. 安全事件必须**同时**写入 `records` 审计表（不可关闭）与日志/指标（可采样）；不得仅依赖日志。
3. `auth.login.failed` 与 refresh token 重用检测事件 `token.reuse_detected`（代码锚点 `internal/app/tokens/refresh_token.go`）必须拉上 `org_id` 与 `ip`，否则无法定位枚举/撞库行为。
4. 不得把敏感值（Token、密码、MFA Secret、恢复码、Client Secret 明文）写入事件属性；`reason_code` 与 `actor_id` 是允许的最大粒度。

### 12.6 告警规则

| 事件 | 告警条件 | 级别 | 通知 |
|------|---------|------|------|
| 5xx 比率 | > 5% 持续 5 分钟 | 严重 | 值班 + 邮件 |
| P99 延迟 | > 2s 持续 5 分钟 | 高 | 值班 |
| 登录错误率 | > 10% 持续 5 分钟 | 高 | 值班 |
| 并发请求 | > 10000 持续 1 分钟 | 中 | 邮件 |
| Token 重用检测 | 已撤销 Token 被使用 | 严重 | 安全组 |
| 暴力破解 | 5 分钟内 10+ 次失败 | 高 | 安全组 |
| Casbin Fail-Close | 503 出现 | 高 | 架构组 |
| 加密回退 | `encryption_fallback_total` > 0 | 中 | 安全组 |
| 密钥轮换超时 | `HasPreviousKey()==true` 持续 > 24h | 中 | 安全组 |
| 审计日志落盘延迟 | > 1 分钟 | 高 | 合规组 |

> 告警事件名引用 12.5 的事件名（如 `token.reuse_detected`、`authz.casbin.enforce_error`；加密回退按 `guliangiam_encryption_fallback_total` 指标判定），不得另起别名。
>
> **告警级别术语闭合（与 12.5 统一）**：全手册告警/事件严重级别只允许使用四级——**信息 / 中 / 高 / 严重**，不得出现 `warning`、`critical`、`紧急`、`低` 等等价或自创级别。12.5 的「固定严重级别」与 12.6 的「级别」列同属此词汇表；12.6 表内当前**无信息级告警行**（信息级只记录、不触发通知，不入告警表），这是有意为之而非遗漏。各级别的响应时限、通知渠道、升级路径与静默抑制细则见 12.8；on-call 值班排班与豁免见 12.9。

### 12.7 可观测性初始化清单

新增服务/模块时必须：

1. 关键操作打点（Counter/Histogram）
2. HTTP handler 使用 `PrometheusMiddleware` 自动指标
3. DB 查询通过 `pgx` 钩子自动打点
4. 外部调用使用 OpenTelemetry client instrumentation
5. 错误路径记录结构化日志（含 `trace_id`）
6. 关键路径增加 Span

### 12.8 告警分级与响应升级

> 本节定义**响应时限 / 升级路径 / 静默抑制**细则；告警级别词汇为四级（信息 / 中 / 高 / 严重）。

**四级响应矩阵**：各级别（严重 / 高 / 中 / 信息）的触发场景、响应时限、通知渠道与升级路径见 `generated/slo-registry.md`「告警四级响应矩阵」表；burn rate 快速燃烧告警阈值与 503 防抖窗口见下方联动与防抖细则。

**错误预算联动（burn rate 告警）**：分面错误预算（`generated/slo-registry.md`）消耗速率达到「1 小时烧掉 2% 预算」时触发严重级快速燃烧告警（2% 预算/1h ≈ 28d 窗口内 burn rate 13.4x）；达到「6 小时烧掉 5% 预算」触发高级。预算耗尽本身的冻结动作见 11.8，不由告警系统自动执行发布冻结。

**「503 即告警」的防抖窗口（补强 12.6 Casbin Fail-Close 行）**：单次 503 出现**只记录事件（信息级），不页面**；聚合判定 `503 率 > 1% 持续 2 分钟`（分母同 11.6 SLI 分母口径）才升级为严重级页面。目的：隔离单实例抖动/滚动重启瞬间的孤立 503，防告警疲劳。安全类事件（`token.reuse_detected`）**不做速率防抖**，单次即页面。

**静默抑制规则**：

1. **同源聚合**：同一故障源（同一 deployment 副本集、同一 DB 实例、同一 trace_id/request_id 关联的告警群）在 5 分钟窗口内触发的多条告警聚合为一次通知，父告警携带子告警列表；派生告警（如 DB 主库故障引发的登录错误率、会话恢复延迟）被根因告警抑制。
2. **维护窗口**：已登记的变更窗口（17.3 发布流程执行期）内由变更产生的告警自动静默并标注变更单号，窗口结束自动恢复；变更超时未结束须人工延长，不得静默漂移。
3. **静默登记**：任何人工作静默（silence）必须登记（操作人、起始/结束时间、理由、关联事件），有效期最长 24h，超时须值班经理批准续期；静默清单纳入 on-call 交接（12.9）。

### 12.9 On-call 值班排班

| 事项 | 规则 |
|---|---|
| 主班 / 备班 | 主班 7×24 首响（响应时限按 12.8 四级矩阵，见 `generated/slo-registry.md`）；备班待命，在主班 15 分钟未认领严重级页面时自动接手 |
| 轮换周期 | 主班以周为单位轮换；轮换交接后第一周为新任主班责任周 |
| 交接 | 每周交接时点（建议工作日上午 10:00，本地时区 Asia/Shanghai）；交接清单：未闭环事件、静默清单（12.8）、各分面错误预算余量（`generated/slo-registry.md`）、在途变更窗口；交接双方在台账确认 |
| 豁免（light week） | 每 6–8 周安排一次 light week：该周仅保留严重级页面响应，高/中级顺延至下一工作日处理，主班职责由备班代理；light week 用于偿还技术债、告警规则治理与本节排班规则迭代 |
| 与升级路径联动 | 升级链：主 on-call → 备 on-call → 值班经理 + 架构组 → 安全组（仅安全类事件，12.6 通知列）；light week 期间升级链跳过主班直达备班 |
| 豁免审批 | 因休假/健康等个人原因短期卸任主班，须提前 1 个工作日与备班换班并在台账登记；未登记的缺位按升级链自动推进 |

### 12.10 结构化日志强化（资源属性 / 采样 / 大小上限）

> 补强 12.4（字段表与脱敏禁令不变）；本节定义资源属性强制集、采样策略与单条上限。

**资源属性强制集**（每条日志必须携带）：

| 属性 | 说明 | 来源 |
|---|---|---|
| `service.name` | 固定 `guliangiam` | `internal/bootstrap/telemetry.go` |
| `service.version` | git sha / 发布版本 | 与 `SENTRY_RELEASE` 同源（环境变量注册表 D.4），构建注入 |
| `env` | 环境标识（dev/test/staging/production） | `APP_ENV`（环境变量注册表 D.1） |
| `caller` | 调用方标识：产生日志的模块/包名（如 `platform.middleware`） | slog handler 注入 |

**采样策略**：

| 类别 | 采样 |
|---|---|
| ERROR / WARN | **100%** |
| 慢请求（响应 > 500ms，对齐 11.1 P99 红线） | **100%**（含请求级 INFO 访问日志） |
| 正常请求 INFO 访问日志 | **10% 目标抽样**（固定比率，按 trace_id 哈希抽样保证同 trace 完整） |
| 健康检查 / 探活端点 | 0%（成功时） |
| 12.5 安全事件对应日志 | 100%（审计本体在 `records` 表，不可采样，12.5 红线 2） |

**单条大小上限**：单条日志序列化后 **32KB** 上限；超限截断（字段级优先保留资源属性与错误字段），并追加 `log.truncated=true` 与 `log.original_bytes` 标记。禁止以截断为理由把敏感字段（12.4 禁令）留在日志中。

### 12.11 Trace 上下文传播与三 ID 关联

> 补强 12.3（采样/Span 命名规则不变）。

**传播规范**：

| 项 | 规范 |
|---|---|
| 入站提取 | 启动时注册 W3C `TraceContext + Baggage` propagator；`TracingMiddleware` 从请求头提取 `traceparent`，缺失时新建根 Span |
| 出站 HTTP 注入 | 出站调用经 `otelhttp.NewTransport`（或显式 `propagator.Inject`）向下游注入 `traceparent` |
| 队列注入 | Webhook/消息类出站载荷在消息头注入 `traceparent`，消费端提取接续同一 trace |
| Baggage 键白名单 | 仅允许 `org_id`、`tenant_tier`、`slo_surface`；**禁止**任何 PII / 凭据进 Baggage（白名单外键在出口处丢弃并计数） |
| Span 命名收敛 | 与 12.3 一致改用路由模板（`{http.method} {route_template}`），消除高基数 Span 名 |

**三 ID 关联（强制格式）**：

| ID | 生成点 | 关联规则 |
|---|---|---|
| `request_id` | chi `middleware.RequestID`（`internal/bootstrap/chi.go` 全局首层挂载）每请求生成；透传 `X-Request-Id` 时沿用 | 日志（12.4）与错误响应必带 |
| `trace_id` | OTel Span（入站提取或新建） | 日志必带（12.4）；与 `request_id` 同请求一一对应 |
| `audit_id` | **`records.id`（UUIDv7）**，生成点为审计记录创建时的 UUID 生成（`internal/app/records/create_record.go` 的 `s.UUID.NewString()`）；对外展示层可加 `rec_` 前缀，存储为原生 UUID | 审计行与日志/Trace 的关联靠 `records.request_id` / `records.trace_id` 列（12.12） |

三 ID 在同一 HTTP 请求内必须可互查：日志按 `request_id` 直查、审计按 `records.request_id` 直查（见 12.12）。

### 12.12 审计事件 schema 对齐 `records`

> 12.5 必填字段表中 `surface` / `request_id` / `trace_id` 的语义与写入链路见本节；`records` 表结构（列名、类型、索引与约束）以声明式 Schema `contracts/schema/60_audit.yaml` 为唯一事实源，DDL 由生成器产出（ADR-022）。

**Schema 与写入链路**：

1. `records` 表在基础审计列之上承载 `surface` / `request_id` / `trace_id` 三列及其查询索引；列名、类型、索引与约束以 `contracts/schema/60_audit.yaml` 为准。复现与校验：`python3 scripts/gen_schema.py --check`。
2. 写入链路：`Handler.Create`（`internal/transport/http/api/v1/records/handler.go`）从 `route.RequestMetadataFrom(ctx)` 取值 → `apprecords.CreateRecordCommand`（`internal/app/records/create_record.go`）→ `domain.Record`（`internal/domain/records/record.go`）→ `infra/postgres/records`（`sql.go` 的 INSERT/SELECT 列、`repository.go` 的 Exec/Scan、`mapper.go`）。`surface` / `request_id` / `trace_id` **不在 `CreateRecordRequest` 中**，客户端无法伪造面归属；列表/搜索响应通过 `RecordResponse` 暴露三字段。REST 主写入路径为 `POST /api/v1/records`；旧路径 `POST /api/v1/audit/records` 以 307 兼容保留（`internal/transport/http/api/v1/records/routes.go`）；12.5 红线 2「安全事件必须同时写入 `records`」的自动化落点由审计事件发射器承担（见第 3 条）。
3. 审计事件发射器：`OtelEmitter`（`internal/infra/audit/otel_emitter.go`）负责把安全事件写入 slog/OTel 并联动 `records` 写入；`NoopEmitter` 仅允许在测试中使用，生产装配禁止空实现。

- `surface` 取值仅限 `management` / `authentication` / `protocol`（4.1.3）；应用层新写入必须显式传值，Schema 默认值仅服务存量行迁移。
- `request_id` / `trace_id` 以部分索引支撑查询（空串不建索引），与 12.11 三 ID 关联口径一致；查询**必须**走列索引。

### 12.13 前端 RUM 与 Web Vitals

> 前端真实用户监控口径，与《architecture/observability.md》§13（前端性能基线，CWV 预算）对齐。

**指标与预算（PERF §13 对齐，不改值）**：`LCP < 2.5s`、`INP < 200ms`、`CLS < 0.1`；超预算即「坏会话」。

**采样边界**：

| 会话类别 | 上报采样 |
|---|---|
| 错误会话（发生 JS 错误 / 任一 CWV 超预算） | **100%** |
| 正常会话 | **1–10% 随机抽样**（默认 5%，按环境变量可调） |
| 上报字段 | metric name / value / rating、页面路径、session 标识（脱敏、无 PII）、`request_id`（关联后端 Trace，12.11） |

**落地要求**：上报端点 `POST /api/v1/analytics/web-vitals`（`internal/bootstrap/chi.go` 的 `RegisterPublicRoutes`）必须落库并进入 RUM 指标管道，不得止步于空壳应答。

**口径红线**：RUM/Web Vitals 数据**不得**用于 SLI/SLO 判定（SLO 只认服务端遥测，11.6 采样口径；SLO 数值与错误预算见 `generated/slo-registry.md`）；RUM 仅用于前端体验监控与 PERF §13 预算验证。

### 12.14 OTLP 基线

> Trace/Metrics/Logs 出口的技术选型与 OTLP 基线（配置下限）。

**`/metrics` 鉴权纪律**：`/metrics` 不得无鉴权裸奔——收敛为单一暴露点（受保护或独立端口），并由 ServiceMonitor 以合规通道抓取（[deployment.md §2.3](operations/deployment.md#23-k8s-生产清单-8-项上线门槛)）；禁止多处重复暴露 `promhttp.Handler()`。

**OTLP 基线（配置下限）**：

| 项 | 基线 |
|---|---|
| endpoint | `OTEL_EXPORTER_OTLP_ENDPOINT`（gRPC :4317 或 HTTP :4318），指向自建 OTel Collector/网关，不直连厂商 SaaS；网络放行边界按 OPS §5 表 1 |
| 资源属性 | `service.name=guliangiam`、`service.version`（与 `SENTRY_RELEASE` 同源 git sha）、`deployment.environment=APP_ENV`、`service.caller`（surface，可选）——与 12.10 日志资源属性同集 |
| 采样器 | 生产 `ParentBased(TraceIDRatioBased(1%))`（对齐 12.3）；错误 100%（异常 Span 强制保留）；开发环境 `AlwaysSample` |
| 可靠性 | exporter 失败本地缓冲 + 丢弃计数暴露为自监控指标；Collector 不可达不得阻塞请求路径 |
| 变量登记 | `OTEL_*` 环境变量落地时登记环境变量注册表 D.4（不在本节展开） |

---

## 13. 部署与运维规范

### 13.1 环境分级

| 环境 | 用途 | 数据 | 变更 |
|------|------|------|------|
| dev | 本地开发 | 模拟数据 | 自由 |
| test | 集成测试 | 测试数据 | 自动 |
| staging | 预生产 | 脱敏生产数据 | 手动审批 |
| production | 生产 | 生产数据 | 严格审批 |

### 13.2 容器化规范

**Dockerfile 要求**：

| 规则 | 说明 |
|------|------|
| 多阶段构建 | 构建阶段 + 运行阶段分离 |
| 基础镜像锁定 | `alpine:3.22`（禁止 `latest`） |
| 包管理器锁定 | `corepack prepare pnpm@<packageManager 版本> --activate`（版本以 `web/package.json` 的 `packageManager` 字段为准，当前 `pnpm@12.18.0`） |
| HTTPS 仓库 | Alpine 仓库必须 HTTPS |
| 非 root 用户 | 运行阶段使用非 root |
| 最小权限 | 仅暴露必要端口 |
| 健康检查 | `HEALTHCHECK` 指令 |

**.dockerignore 必含**：

```
.git/
node_modules/
web/dist/
*.test
*.exe
guliangiam-server
.idea/
.vscode/
```

### 13.3 Kubernetes 部署规范

| 资源 | 要求 |
|------|------|
| Deployment | `replicas: 2+`，RollingUpdate，`maxUnavailable: 0` |
| HPA | CPU 70% / Memory 80%，`minReplicas: 2`，`maxReplicas: 10` |
| PDB | `minAvailable: 1` |
| Liveness | `GET /healthz`，`initialDelaySeconds: 10` |
| Readiness | `GET /readyz`（需 `X-Metrics-Token`），`initialDelaySeconds: 5` |
| Resources | `requests: 100m/128Mi`，`limits: 500m/512Mi` |
| Ingress | cert-manager 自动 TLS，nginx 支持 TLS 1.3 |
| ConfigMap | 非敏感配置 |
| Secret | 敏感配置（POSTGRES_DSN / ISSUER_URL / ENCRYPTION_KEY_CURRENT） |

### 13.4 部署顺序

1. **数据库迁移**（如需要）：`psql -f db/postgresql/V{N}__*.sql`（迁移文件唯一目录为 `db/postgresql/`，与第 19 章一致；不存在 `sql/` 目录）
2. **配置注入**：更新 ConfigMap / Secret
3. **镜像滚动更新**：`kubectl rollout` 或 HPA 自动
4. **健康检查验证**：`curl /healthz`（存活探针，`bootstrap.RegisterPublicRoutes`）；就绪探针使用 `curl -H "X-Metrics-Token: $METRICS_SCRAPE_TOKEN" /readyz`；需要业务版本信息时使用 `curl /api/v1/system/version`（`system/chi.go` 实际注册；`/api/v1/system/*` 与 `/metrics` 同组，均需 metrics token；`/api/v1/system/health` 仅 legacy liveness）
5. **烟雾测试**：核心端点可用

### 13.5 回滚规范

| 场景 | 操作 |
|------|------|
| 应用回滚 | `kubectl rollout undo deployment/guliangiam` |
| 配置回滚 | 还原 ConfigMap/Secret，滚动重启 |
| 数据库回滚 | 从备份恢复（PITR） |
| 完整回滚 | 还原 Deployment + ConfigMap + Secret + DB |

### 13.6 备份与恢复

备份类型、频率、保留代数与恢复验证演练**以 [backup-dr.md §6](operations/backup-dr.md) 为唯一口径**（物理基备 + WAL 归档 + 逻辑备份的保留策略在该文档维护，本节不复制数值）。

**恢复 RTO/RPO**：三档场景（L1 应用层 / L2 数据库主库 / L3 数据库损坏）的 RTO/RPO 阈值、依赖机制与演练频率**以 `contracts/registries/dr.yaml` 为唯一事实源**（人类可读说明见 [backup-dr.md §7](operations/backup-dr.md#7-rtorpo-三档与恢复演练)；需求口径见 [requirements/non-functional.md §6.2](requirements/non-functional.md)）。

### 13.7 密钥与配置轮换

| 配置 | 轮换周期 | 操作 |
|------|---------|------|
| `ENCRYPTION_KEY` | 90 天 | 双密钥过渡，见第 14 章 |
| SAML RelayState 签名密钥 | 随 `ENCRYPTION_KEY_CURRENT` 轮换 | 由后端以 `ENCRYPTION_KEY_CURRENT` 做域分离派生（标签 `guliangiam:saml-relay-state:v1`），无独立前端密钥；轮换会使在途 RelayState 失效（TTL 10 分钟）。**规范**：前端构建产物中不得注入任何保密值；RelayState 由后端 `infra/saml.RelayStateSigner` 签发/校验，IdP 角色按 SAML 2.0 §3.4 原样回传；详见 D.10 |
| 数据库密码 | 90 天 | 数据库端 + Secret 端同步 |
| TLS 证书 | 60 天（cert-manager 自动） | 无人工操作 |

---

## 14. 密钥与加密规范

### 14.1 加密层次

| 层次 | 技术 | 用途 |
|------|------|------|
| 传输层 | TLS 1.3 | 网络传输加密 |
| 字段级 | AES-256-GCM | 敏感字段存储加密 |
| 密码 | Argon2id | 用户密码哈希 |
| Token | SHA-256 | Token 哈希存储 |
| 签名 | RS256/ES256 | JWT 签名 |

### 14.2 字段级加密规范

**加密算法**：AES-256-GCM

**密钥长度**：32 字节（256 bit），Base64 编码

**加密字段清单**：唯一事实源为代码注册表 `internal/platform/crypto/reencrypt_job.go` 中的 `DefaultEncryptedColumns`（含 provider 回填、sealed payload 与 PII legacy 映射）；复现与校验：`python3 scripts/check-encrypted-columns.py`。本节只定义判定规则，不复制列清单。

> 实现要求：注册表登记的字段一律经字段加密器加解密，并遵守 14.3 的双密钥过渡与存量回填；**不能**只改文档不改代码，也不能只改代码不回填存量数据。任何「需要还原明文」的 D3 字段不得以明文 `TEXT` 列存储。

> `applications` 的 Client Secret **不属于本清单**：它以 Argon2id 单向哈希存储于 `applications.client_secret_hash`（`internal/platform/crypto/client_secret.go`），无需也无法解密，应作为哈希校验而非字段加密实现。

#### 14.2.1 判定边界：何时用「哈希」、何时用「字段加密」

本节只定义**为什么**某些字段不属于哈希，不复制字段清单。判定规则如下，任何新增敏感字段必须按下表归类，并在 `DefaultEncryptedColumns` 登记（或显式说明为哈希）：

| 判定问题 | 答案为「是」→ 哈希（Argon2id，单向） | 答案为「是」→ 字段加密（AES-256-GCM，可逆） |
|---|---|---|
| 系统是否需要**还原明文**使用？ | 否 | **是** |
| 校验方式 | 重新计算摘要后**比对**（`Verify(hash, input)`） | 解密后**使用**（如作为出站请求的签名材料、上游连接的认证凭据） |
| 存储位置与实现 | `*_hash` 命名列；`internal/platform/crypto/client_secret.go`（Client Secret）/ `internal/infra/cred/`（用户密码） | 密文列；`internal/platform/crypto/field_encryption.go`（`Encryptor`） |
| 锚点案例 | `applications.client_secret_hash`：仅参与 OAuth 客户端认证比对，从不被还原 | `webhooks.secret`：`internal/infra/webhook/http_sender.go:104` 以 `hmac.New(sha256.New, []byte(secret))` 对出站请求体签名，**必须先解密拿到明文**，故为字段加密而非哈希 |

**红线**：

1. 凡「需要还原明文」的字段，**不得**以哈希存储（哈希后无法使用）；凡「只需比对」的字段，**不得**以字段加密替代哈希（可逆存储扩大了泄露面）。
2. `applications.client_secret_hash` 与 `webhooks.secret` 是同一类「共享密钥」但归属不同机制的两个锚点案例，评审时不得混用；新增同类字段必须明确归属并在 `DefaultEncryptedColumns` 登记。
3. 字段加密的密钥轮换、双密钥过渡与回滚流程见 14.3；哈希字段不参与密钥轮换（仅当算法参数升级时才需要重算，走用户下次登录或强制重置路径）。

### 14.3 密钥轮换

**双密钥过渡方案**：

| 环境变量 | 用途 |
|----------|------|
| `ENCRYPTION_KEY_CURRENT` | 当前活跃密钥，新写入使用 |
| `ENCRYPTION_KEY_PREVIOUS` | 前一个密钥，仅解密回退 |
| `ENCRYPTION_KEY` | 旧版变量，向后兼容（fallback） |

**解析优先级**：

```
当前密钥: ENCRYPTION_KEY_CURRENT > ENCRYPTION_KEY
前一个密钥: ENCRYPTION_KEY_PREVIOUS（独立，无 fallback）
```

**轮换步骤**：

1. 生成新密钥：`openssl rand -base64 32`
2. 配置双密钥：`ENCRYPTION_KEY_CURRENT=$NEW`，`ENCRYPTION_KEY_PREVIOUS=$OLD`
3. 重启服务（新加密用新密钥，解密回退到旧密钥）
4. 批量重加密：调用 `RotateEncryptedData()` 对所有加密字段重加密
5. 验证后移除 `ENCRYPTION_KEY_PREVIOUS`，重启服务

**回滚步骤**：交换密钥，`ENCRYPTION_KEY_CURRENT=$OLD`，`ENCRYPTION_KEY_PREVIOUS=$NEW`，重启。

**监控**：`HasPreviousKey()==true` 持续 > 24h 触发告警。

**详细操作步骤**见 `docs/architecture/security-model.md`。

### 14.4 密钥管理红线

| 红线 | 说明 |
|------|------|
| 禁止硬编码 | 密钥不得出现在代码、日志、配置文件 |
| 禁止跨环境复用 | dev/test/staging/production 使用不同密钥 |
| 禁止通过非加密通道传输 | 密钥传输必须使用加密通道 |
| 禁止在内存中长期驻留 | 密钥在使用后应尽快释放引用 |
| 禁止记录到日志 | 日志脱敏必须覆盖所有密钥类型 |
| 必须使用密钥管理服务（强制） | **生产环境必须**接入 KMS（Vault / AWS KMS / Azure Key Vault 等任一），不得仅以环境变量明文承载 `ENCRYPTION_KEY_CURRENT`；开发/测试环境允许降级为环境变量，但必须在启动日志中标注运行环境 |

### 14.5 密码哈希规范

| 算法 | 用途 | 参数 |
|------|------|------|
| Argon2id（用户口令） | 默认，新账户 | alexedwards `DefaultParams`：memory=64MiB, iterations=1, parallelism=NumCPU, salt=16B, hash=32B（事实源 `internal/infra/cred/argon2id.go`） |
| Argon2id（Client Secret） | 应用客户端密钥哈希 | memory=64MiB, iterations=3, parallelism=2, salt=16B, hash=32B（事实源 `internal/platform/crypto/client_secret.go`） |
| bcrypt | 遗留兼容 | cost=10 |
| PBKDF2-SHA256 | 遗留兼容 | 600,000 iterations |
| PBKDF2-SHA512 | 遗留兼容 | - |
| SHA-256 | 遗留兼容（仅验证） | - |
| SHA-512 | 遗留兼容（仅验证） | - |
| MD5 | 仅迁移验证 | - |
| Plain | 仅测试 | - |

**自动升级**：登录时若检测到遗留算法（非 Argon2id），自动以 Argon2id 重新哈希并持久化。

### 14.6 Secret 轮换审计

所有 Secret 轮换操作写入 `secret_rotation_audit` 表：

| 字段 | 说明 |
|------|------|
| `actor_id` | 操作人 |
| `resource_type` | 资源类型（provider/application/agent） |
| `resource_id` | 资源 ID |
| `secret_type` | Secret 类型（client_secret/webhook_secret 等） |
| `rotated_at` | 轮换时间 |
| `ip` | 操作 IP |
| `user_agent` | 用户代理 |

### 14.7 轮换执行工程红线（配套 14.3–14.4）

> 轮换能力以静态双密钥（14.3）为底线，生产必须接入 KMS（14.4）；本节红线在实现与运维中强制执行：

| # | 红线 | 说明 |
|---|------|------|
| 1 | **KMS 调用必须在事务外** | 事务内禁止任何网络调用（[outbox 依赖失败的处置原则](design/infrastructure/outbox.md#依赖失败的处置原则)）；DEK 解包在事务前完成，信封重写与游标推进（[14.3 密钥轮换](#143-密钥轮换)）在同一事务内持久化 |
| 2 | **KMS 时延预算 ≤50ms，降级 DEK 本地缓存 ≤60s** | 超预算/不可用时降级缓存，缓存失效即拒绝敏感读写（14.2 明文降级拒绝同款），禁止回退明文 |
| 3 | **再加密批次 500 行 + `FOR UPDATE SKIP LOCKED` 行锁** | 与 Outbox 领取同款模式（[outbox Worker 参数与投递语义](design/infrastructure/outbox.md#worker-参数与投递语义)）；禁止 OFFSET 翻页；游标只在数据提交后前移 |
| 4 | **production 材料删除必须双人审批** | 第二批准人写 `records`；缺审批的删除端点必须 `403`，该开关在 production 禁止关闭 |
| 5 | **参数命名与登记** | 本节新增变量（`KMS_LATENCY_BUDGET_MS`、`KMS_DEK_CACHE_TTL_SECONDS`、`KEYS_REENCRYPT_BATCH`、`KEYS_DELETION_DUAL_APPROVAL`）实现时必须登记环境变量注册表 D.12，禁止散落未登记变量；Provider 接口放 domain/port，实现放 infra，方言 SQL 禁止混入 app 层（`bash scripts/check-layers.sh` 把关） |

**验证命令**：`go test ./internal/platform/crypto/... ./internal/infra/postgres/keys/...`；DDL 变更走第 19 章结构门禁；文档一致性 `bash scripts/verify-docs.sh`。

### 14.8 Workload 凭据规范

> 工作负载凭据的目标模型为服务账号及其凭据子表（表名/列名以 `contracts/schema/*.yaml` 申报为准；未申报前不得在 API/OpenAPI/文档中宣称其存在，[security-model §4](architecture/security-model.md#4-workload-身份平面adr-026)，ADR-026）；凭据子表落地前，client_credentials 以 `applications.client_secret_hash`（Argon2id）承载。协议面 `POST /api/v1/auth/tokens:exchange` 承载联邦交换语义。本节随 [security-model §4](architecture/security-model.md#4-workload-身份平面adr-026) 落地强制执行。

| 环节 | 规范 | 锚点 |
|------|------|------|
| 禁止长效 secret | 凭据类型 `federated` 优先；`static_key` 仅在不可联邦场景经审批保留并强制轮换；**凭据一律短期（上限 24h，联邦换发默认 ≤ 1h），禁止新增长效 secret 用例**——静态检查 + 门禁拦截（17.4），豁免须双人审批留痕 | [security-model §4.4](architecture/security-model.md#44-短期凭据与凭据主权)；[security-model §4](architecture/security-model.md#4-workload-身份平面adr-026) |
| 存储形态 | 静态凭据仅存哈希（Argon2id，与 client secret 哈希同算法口径；列名以 Schema 申报为准），永不返回明文；`federation_trusts.jwks_uri` 等信任面字段不做机密处理，但 `signing_key_ref` 类引用一律走信封/引用列 | [security-model §4.1](architecture/security-model.md#41-主体模型)；14.2.1 |
| 签发 | Token Exchange（RFC 8693，[security-model §4.3](architecture/security-model.md#43-与令牌通道的合流)）：`subject_token` 必须同时满足 issuer 精确命中 / `aud` 白名单 / `subject` 白名单（受控通配）/ JWKS 验签（算法白名单 8.6 同源），任一不满足拒绝并审计；产出平台令牌的 `scope` 相对身份池 `scope_ceiling` **只能收敛不能放大**；两条路径（client_credentials 直连接凭证与联邦换发）产出同一 claim 框架、actor/subject 可区分 | [security-model §4.2](architecture/security-model.md#42-联邦信任与身份池)/[security-model §4.3](architecture/security-model.md#43-与令牌通道的合流)/[security-model §4.4](architecture/security-model.md#44-短期凭据与凭据主权) |
| 轮转 | 联邦令牌到期前**自动换发，刷新周期 ≤ 1h**，短期令牌不落库存明文；`static_key` 以 `last_rotated_at` 强制轮换周期，到期进入回收扫描（[login](design/auth/login.md)）；轮换审计走 `secret_rotation_audit`（V1 已建，14.6） | [security-model §4.4](architecture/security-model.md#44-短期凭据与凭据主权)/[login](design/auth/login.md)；14.6 |
| 吊销 | `federation_trusts` 删除**级联**：身份池即时失效 → 池内签发的在途令牌吊销（令牌撤销传播语义同 [login](design/auth/login.md)）→ 信任变更/删除留 `records`；jti 一次性缓存防重放；JWKS 缓存版本化失效，轮换平滑 | [security-model §4.2](architecture/security-model.md#42-联邦信任与身份池)/[login](design/auth/login.md)；[security-model §4](architecture/security-model.md#4-workload-身份平面adr-026) |
| 审计 | 签发 / 使用 / 吊销**全链**落 `records`（`token.created` / `token.revoked` / `credential.*` 事件域，12.5）；凭据使用记录（who / where / when）可查询（WLI-003 验收 4）；SA 审计身份与人类用户可区分、禁止登录交互式 UI（WLI-001），删除前预检关联凭据与授权 | 12.5；[security-model §4](architecture/security-model.md#4-workload-身份平面adr-026) |

---

## 15. 无障碍（A11y）规范

> 目标：WCAG 2.1 Level AA 合规。

### 15.1 强制要求

| 类别 | 要求 | 检查方式 |
|------|------|---------|
| 非文本内容 | 所有 `<svg>` 提供 `role="img"` + `aria-label` 或 `<title>` | Code Review + axe-core |
| 名称/角色/值 | 所有交互元素提供可访问名称；div 模拟按钮必须 `role="button"` + `tabIndex={0}` + `onKeyDown` | Code Review |
| 键盘可达 | 所有功能可通过键盘完成；Card onClick 必须支持 Enter/Space | 手动测试 |
| 跳转链接 | 页面顶部必须有 "Skip to Main Content" | Code Review |
| 焦点可见 | `:focus-visible` 样式必须存在且清晰 | 手动测试 |
| 颜色对比 | 普通文本 ≥ 4.5:1；大文本 ≥ 3:1；图形 ≥ 3:1 | 工具验证 |
| 标题层级 | 不得跳级（h1 → h2 → h3，不跳 h4） | Code Review |
| 语义标签 | 使用 `<main>` / `<nav>` / `<section>` / `<article>` | Code Review |
| 语言属性 | `<html lang="zh-CN">` 正确设置 | Code Review |
| 减弱运动 | `prefers-reduced-motion` 时禁用非必要动画 | Code Review |

### 15.2 颜色规范

| 用途 | 变量 | 对比度 |
|------|------|--------|
| 主要文本 | `--gl-color-text` | ≥ 7:1（AAA） |
| 次要文本 | `--gl-color-text-secondary` | ≥ 4.5:1 |
| 三级文本 | `--gl-color-text-tertiary` | ≥ 4.5:1（≥ 14px） |
| 禁用文本 | `--gl-color-text-disabled` | 仅装饰性 |
| 交互图形 | `--gl-color-primary` | ≥ 3:1 |

**禁止**：硬编码 `#999`、`#666`、`#94A3B8` 等未经验证的颜色。

### 15.3 组件级要求

| 组件类型 | 要求 |
|----------|------|
| 按钮 | 使用 `<Button>` 或 `<a>`；禁止 `<div onClick>` |
| Card 可点击 | 必须 `tabIndex={0}` + `role="link/button"` + `onKeyDown` |
| 模态框 | 焦点陷阱 + Esc 关闭 + 关闭后焦点归还 |
| 表格 | 行可选时必须支持空格键选中 |
| 下拉菜单 | 支持上下箭头 + Enter 选择 |
| 图标按钮 | 必须有 `aria-label` |
| 表单控件 | 必须有 `<label>` 或 `aria-label` |
| 错误提示 | 使用 `role="alert"` 或 `aria-live="polite"` |
| 加载状态 | 使用 `role="status"` + `aria-live="polite"` |

### 15.4 审计工具链

| 工具 | 用途 | 频率 |
|------|------|------|
| `@axe-core/react` | 开发环境自动检测 | 持续 |
| Playwright + axe-core | E2E 无障碍测试 | 每次 PR |
| NVDA / VoiceOver | 手动屏幕阅读器测试 | 每季度 |
| WCAG Color Contrast Checker | 对比度验证 | 每次 UI 变更 |
| 静态审计脚本 | 源码模式匹配 | 每次 PR |

### 15.5 A11y 修复优先级

| 级别 | 含义 | 处理时限 |
|------|------|---------|
| Critical | 完全阻断键盘/屏幕阅读器用户 | 24 小时 |
| Serious | 显著影响特定用户群体 | 1 周 |
| Moderate | 影响体验但有替代路径 | 1 个月 |
| Minor | 最佳实践 | 3 个月 |

---

## 16. 国际化（i18n）规范

### 16.1 语言覆盖

必须支持 14 种语言：

| Locale | Language |
|--------|----------|
| zh-CN | 简体中文 |
| zh-TW | 繁体中文 |
| en | English |
| ja | 日本語 |
| ko | 한국어 |
| de | Deutsch |
| fr | Français |
| es | Español |
| pt | Português |
| pl | Polski |
| ru | Русский |
| tr | Türkçe |
| uk | Українська |
| vi | Tiếng Việt |

### 16.2 命名空间

| 命名空间 | 内容 |
|----------|------|
| `common` | 跨页面复用的 UI 标签、按钮、消息、货币名称、枚举值文案（`enumValues.*`） |
| `auth` | 登录、MFA、Passkey、QR code、SAML、consent |
| `resource` | 实体特定标签、占位符（`*.placeholder.*`）和验证消息（`*.validation.*`） |

命名空间集合只有一处事实源：`web/src/shared/i18n/index.ts` 的 `ns: [...]`。新增命名空间必须先改该数组，再补齐全部语言包（清单以 `web/src/shared/i18n/locales/` 目录为准）；`useTranslation("device")` 这类未注册命名空间会让整页渲染原始键名。

**别名约定**：同一文件需要多个命名空间时，按固定别名解构，避免同一文件出现第二个 `t`：

| 别名 | 绑定命名空间 | 写法 |
|------|--------------|------|
| `t` | 该文件的主命名空间 | `const { t } = useTranslation("resource")` |
| `tc` | `common` | `const { t: tc } = useTranslation("common")` |
| `tr` | `resource` | `const { t: tr } = useTranslation("resource")` |
| `ta` | `auth` | `const { t: ta } = useTranslation("auth")` |

未使用的别名绑定会被 `oxlint` 的 `no-unused-vars` 判为 error，删除调用点时必须同步删除绑定。

### 16.3 键树与引用规则

语言包是**嵌套结构的 JSON**，i18next 把 `.` 当路径分隔符、把 `:` 当命名空间分隔符。键树形状与调用点语义必须完全一致，否则子树会被无声吞掉。

**结构规则（硬失败）**

| 规则 | 内容 | 反例 |
|------|------|------|
| R1 | 键名自身不得含 `.` 或 `:`；层级只能用嵌套对象表达 | `"login.qrcode.expiresIn": "..."` → 应为 `{"login": {"qrcode": {"expiresIn": "..."}}}` |
| R2 | 顶层组名不得与命名空间同名 | `common.json` 里出现 `"common": {...}`（会让 `t("common.foo")` 解析到命名空间内部的 `common.foo`） |
| R3 | 同一路径不得既是叶子又是组；前缀一旦是字符串，其子树不可达 | `"settings": "设置"` 与 `"settings.passwordMinLength": "..."` 共存 |

**引用规则（R4）**

| 规则 | 说明 |
|------|------|
| 命名空间只能用 `:` 表达 | `t("auth:signIn")`；`t("common.saved")` 是错的 |
| 同命名空间内直接用路径 | 文件绑定 `common` 时写 `t("form.invalidFormat")` |
| 跨命名空间必须显式加前缀 | 文件绑定 `resource` 时引用登录标题写 `t("auth:signIn")` |
| 解析范围 = 该 translator 的绑定命名空间 | `t` 的绑定由 `useTranslation(ns)` 决定；`tc` / `tr` / `ta` 各自只能解析自己的命名空间 |
| 内联兜底不得掩盖缺失键 | `t("auth.fieldRequired", "Required")` 会永久渲染英文兜底，该键永远不会被翻译；应补齐语言包或改用真实键 |

**键归属规则（R5）**：同一键路径不得同时存在于两个命名空间——那是两个事实源，必然漂移。必须收敛到单一命名空间。注意 R5 判定的是**完整键路径**，因此叶子名相同但路径不同（`resource.user.idCard` 与 `resource.account.idCard`）不算重复归属；真正的重复是同一路径出现在两个命名空间。

**命名规范**

| 规则 | 示例 |
|------|------|
| 层级用嵌套对象，分组名用 lowerCamelCase 或小写 | `form.invalidFormat`、`provider.types.oauth.github` |
| 动态值用插值占位符 | `welcome: "欢迎，{{name}}"` |
| 货币名称放在 `common` 的 `currency.<CODE>` | `t("currency.CNY")` |
| Provider 类型放在 `resource` 的 `provider.types.<category>.<type>` | `t("provider.types.oauth.github")` |

### 16.4 使用规范

| 规则 | 说明 |
|------|------|
| 零硬编码 | 所有用户可见文本必须通过 `t()` / `tr()` 引用 |
| 选项列表也走键 | 语言下拉框等选项名称必须来自语言包，不得在组件内写死本地名称 |
| 14 语言同步 | 新增键必须**一次性**补齐全部语言包，否则 parity 门禁失败 |
| 复数处理 | 使用 i18next 复数语法（`_one` / `_other`） |
| 日期时间 | 使用 `formatDateTime()` 统一格式化 |
| 数字/货币 | 使用 `Intl.NumberFormat` 或统一工具函数 |
| RTL 支持 | 当前语言列表无 RTL，未来若添加需 CSS 适配 |

### 16.5 CI 校验

唯一门禁：`python3 scripts/check-i18n-keys.py`（已接线入 `scripts/verify-docs.sh`）。

| 检查 | 失败级别 |
|------|---------|
| 命名空间契约（`useTranslation("x")` 与 `"x:key"` 前缀必须已在 `web/src/shared/i18n/index.ts` 的 `ns` 数组中注册） | 硬失败 |
| 键树结构 R1–R3 | 硬失败 |
| 语言包 parity（全部语言的键集合完全一致） | 硬失败 |
| 键解析（无内联兜底的 `t("key")` 必须可解析） | 基线记账（`scripts/i18n-key-baseline.txt`），只减不增 |
| 内联兜底掩盖的缺失键（R4） | 同上 |
| 键重复归属（R5） | 基线记账（`scripts/i18n-key-ownership-baseline.txt`），只减不增 |
| 基线条目过期（已修复未移除） | 硬失败 |

修复后必须执行 `python3 scripts/check-i18n-keys.py --update-baseline` 收敛基线。

硬编码中文检测：`grep -rPn "[\x{4e00}-\x{9fff}]" web/src/ --exclude-dir=i18n --exclude-dir=node_modules` 必须为空。

---

## 17. CI/CD 与质量门禁规范

> 本章为 CI/CD 流水线与质量门禁规范：`ci.yml`（`backend` / `frontend` / `gates` / `linter` / `security` / `security-gosec` / `docker` / `perf` 作业）承载下述检查，`docker` 作业 `needs` 含 `backend`/`frontend`/`gates`；未接线项按 8.9 门禁表接入。

### 17.1 CI 流水线阶段

| 阶段 | 目标检查 | 失败处理 |
|------|---------|---------|
| 1. Lint（前端） | `oxlint` + `oxfmt`：`pnpm lint` = `oxlint -c .oxlintrc.json src`；`pnpm format:check` = `oxfmt -c oxfmt.json --check .`；prettier 降级为 `format:prettier` / `format:prettier:check` | 阻塞 |
| 1b. Lint（后端） | `golangci-lint` | 阻塞 |
| 2. 类型 | `go build ./...` + `npx tsc --noEmit` | 阻塞 |
| 3. 分层检查 | `scripts/check-layers.sh`（CI `gates` 作业阻塞步骤；规则排除 `*_test.go`） | 阻塞 |
| 4. 单元测试 | `go test ./...` + `pnpm test`：`backend` 作业跑 `go test -cover -race -count=1 -timeout=120s ./internal/...`；`gates` 作业经 `RUN_CODE_CHECKS=1 verify-docs.sh` 执行 | 阻塞 |
| 5. 覆盖率 | `go test -cover`，阈值见 8.5 | 阻塞 |
| 6. 安全扫描 | `gosec`（`ci.yml` security-gosec）+ `govulncheck`（`ci.yml` security）+ `actions/dependency-review-action@v4`（`ci.yml` security）；`pnpm audit` 已接线 `ci.yml` Frontend CI「Dependency audit」步骤（`--audit-level=high`，不得以 `\|\| true` 吞错，见 8.9） | 阻塞（Critical/High） |
| 7. A11y 扫描 | axe-core | 阻塞 |
| 8. i18n 校验 | 硬编码检测 + 键一致性 | 阻塞 |
| 9. Docker 构建 | `docker build -t guliangiam:ci .` + 健康检查 | 阻塞 |
| 10. 集成测试 | Playwright E2E | 阻塞发布 |
| 11. 文档与契约门禁 | `verify-docs.sh` / `check-db-conventions.sh` / `check-sql-portability.sh` / `check-openapi-contract.sh`（`gates` 作业阻塞步骤） | 阻塞 |


### 17.2 质量门禁（Quality Gate）

> 下表为**强制阈值**：「编译」「`go vet`」「单元测试」「覆盖率」「Lint」「分层违规」「硬编码中文」「标准库 `log`」「`fmt.Errorf` 无 `%w`」「前端 lint/format/build/test」由 `ci.yml` 的 `backend` 与 `gates` 作业阻塞执行；安全扫描在 `ci.yml` 的 `security`（govulncheck + dependency-review）与 `security-gosec`（gosec）执行；A11y、E2E、方言矩阵按 8.9 门禁表接入。

| 门禁 | 阈值 | 说明 |
|------|------|------|
| 编译 | 0 错误 | 强制 |
| `go vet` | 0 告警 | 强制 |
| Lint | 0 Error | 强制 |
| 单元测试 | 100% 通过 | 强制 |
| 覆盖率 | ≥ 30%（整体），≥ 70%（新增文件） | 强制 |
| 分层违规 | 0 | 强制 |
| 硬编码中文 | 0 | 强制 |
| 标准库 `log` | 0 引用 | 强制 |
| `fmt.Errorf` 无 `%w` | 0 | 强制 |
| 安全扫描 | 0 Critical/High | 强制 |
| A11y | 0 Critical | 强制 |
| Docker 镜像大小 | < 200MB | 警告 |

### 17.3 发布流程

| 阶段 | 操作 | 责任 |
|------|------|------|
| 1. 版本冻结 | 从 develop 创建 `release/vX.Y.Z` | 发布经理 |
| 2. RC 构建 | CI 构建 RC 镜像 | CI |
| 3. Staging 部署 | 部署到 staging | DevOps |
| 4. 烟雾测试 | 核心流程验证 | QA |
| 5. 性能测试 | k6 负载测试 | QA |
| 6. 安全复审 | 安全扫描 + 人工复审 | 安全 |
| 7. 生产发布 | 滚动更新 | DevOps |
| 8. 验证 | 监控 30 分钟 | DevOps |
| 9. 合并回 develop | `release` → `develop` + `main` | 发布经理 |
| 10. Tag | `vX.Y.Z` | 发布经理 |

### 17.4 新能力测试类型与质量门禁

> 下表为新增测试类型的**准入标准与 CI 挂接点**，纪律与 8.9 相同：宣称「已接入门禁」必须同时满足 ① 可复现本地命令、② CI job 归属、③ 失败处置级别三条件，否则只能表述为「存在测试资产」。

| 测试类型 | 设计锚点 | 准入标准（最低用例集） | CI 挂接点 | 门禁动作 |
|---------|---------|----------------------|----------|---------|
| 关系元组测试（一致性 / 并发写） | [ADR-024](adr/ADR-024-rebac-relation-tuple-authz.md) | ① 写后读一致（Zookie 显式回传，写后读路径断言）；② 并发写同元组：唯一键仲裁收敛（同向去重、异向以 `revision` 乐观锁拒绝）；③ 元组写入与业务行 / `records` 审计**同事务**（拆事务即用例失败）；④ 展开深度超限与环检测拒绝；⑤ 跨租户对象引用写入预检拒绝并审计 | backend job（testcontainers PG，复用 8.6 设施） | 失败阻塞；随 ReBAC 落地回写 8.9 表 |
| 沙箱逃逸测试（网络 / 文件 / 时钟 / 资源） | [ADR-029](adr/ADR-029-identity-actions-sandbox.md) | 四域用例集：① 网络逃逸——白名单外域名、内网段直连、DNS rebinding → 全部阻断 + 审计；② 文件逃逸——读 / 写宿主文件系统 → 无原语可用；③ 时钟 / 随机逃逸 → 确定性受控；④ 资源耗尽——CPU / 内存 / 墙钟触顶 → 终止且 `outcome=quota_exceeded` 快照落库。**任一逃逸成立即 0 容忍缺陷** | 独立 `actions-sandbox` job（PR 触发） | 逃逸未阻断即阻塞；配额默认值 L2 校准前资源类用例为警告级 |
| 联邦互操作测试（Token Exchange / SSF） | [security-model §4.2](architecture/security-model.md#42-联邦信任与身份池)/[security-model §4.3](architecture/security-model.md#43-与令牌通道的合流)、[ADR-026](adr/ADR-026-workload-identity-federation.md) | ① RFC 8693 矩阵：issuer / aud / subject 白名单命中 × 未命中（白名单外必须拒绝 + 审计，**不得换发成功**）；② JWKS 轮换：旧 key 失效、新 key 生效平滑；③ jti 重放拒绝；④ `scope` 不越 `scope_ceiling`；⑤ SSF push（RFC 8935）/ poll（RFC 8936）投递签名验证、重试与死信路径 | backend job（契约层，8.6）+ E2E（17.3 第 4 步烟雾测试） | 失败阻塞；「白名单外换发成功」按阻塞级安全缺陷处置（9.10 SLA 起算） |
| 国密向量测试（SM2 / SM3 / SM4） | [security-model §2.21](architecture/security-model.md#221-国密能力位与密码学边界adr-031)；ADR-031 | ① SM2（GB/T 32918）、SM3（GB/T 32905）、SM4（GB/T 32907）官方标准测试向量**全量通过**；② SM4 信封 GM/T 标准向量 + 独立离线比对（国密实现仅 `gmsm`）；③ 能力缺失时启用国密租户策略 → 硬失败（非静默降级）用例 | `ci.yml` backend 并入或独立 `crypto-compat` job | 标准向量失败阻塞；国密能力位未通过探测时作业显式 skipped 并注明原因，禁止静默跳过 |

---

## 18. 多实例与分布式一致性规范

### 18.1 多实例状态基线

| 组件 | 存储 | 多实例一致性 |
|------|------|-------------|
| Session | PostgreSQL | ✅ 一致 |
| Token | PostgreSQL | ✅ 一致 |
| Account Lockout | PostgreSQL | ✅ 一致 |
| Passkey Challenge（`webauthn_challenges`） | PostgreSQL | ✅ 一致 |
| QR Code Session | PostgreSQL | ✅ 一致 |
| Device Auth Code | PostgreSQL | ✅ 一致 |
| Face ID Session | PostgreSQL | ✅ 一致 |
| CAS Ticket | PostgreSQL | ✅ 一致 |
| Kerberos Ticket | PostgreSQL | ✅ 一致 |
| Casbin Enforcer Cache | Redis（18.2 方案 A） | ✅ 共享 |
| JWKS | 内存（确定性生成） | ✅ 一致 |

### 18.2 进程内状态收敛（Casbin Enforcer Cache）

**Casbin Enforcer Cache** 不得长期以进程内状态存在，按方案 A 收敛为 Redis 共享缓存（PostgreSQL 为策略源）：

| 方案 | 优点 | 缺点 | 推荐 |
|------|------|------|------|
| A. Redis 缓存 | 高性能，共享 | 引入 Redis 依赖 | ✅ 推荐 |
| B. PostgreSQL 共享 + 定期刷新 | 无新依赖 | 性能略低 | 备选 |
| C. 广播失效（NATS） | 实时 | 复杂度高 | 未来 |

**推荐方案 A**：使用 Redis 作为 Casbin 策略缓存，配合 PostgreSQL 策略源。

### 18.3 分布式一致性红线

| 红线 | 说明 |
|------|------|
| 禁止使用进程内状态存储用户会话 | 必须使用 PostgreSQL |
| 禁止使用进程内计数器做限流 | 使用 Redis 或 PostgreSQL |
| 禁止在多个实例间共享密钥（除配置注入） | 使用 KMS/Vault |
| 定时任务必须有分布式锁 | 使用 PostgreSQL advisory lock 或 Redis lock |
| 缓存失效必须广播 | 使用 Redis Pub/Sub 或 NATS |

### 18.4 分布式锁规范

| 场景 | 实现 | 说明 |
|------|------|------|
| Schema 迁移 | PostgreSQL advisory lock | 防止多实例并发迁移 |
| 定时任务 | PostgreSQL advisory lock | 如 LDAP 同步 |
| 缓存刷新 | Redis lock | 防止缓存击穿 |
| Secret 轮换 | PostgreSQL advisory lock | 防止并发轮换 |

**advisory lock 使用示例**：

```go
// 获取锁
_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockID)
// 事务结束自动释放
```

> **锁实现纪律**：app 层只依赖 `LockProvider` 接口（`internal/app/ldapsync/lock.go`，含单副本/测试用 `noopLockProvider`），PostgreSQL 实现位于 `internal/infra/postgres/advisory_lock.go`——用**会话级** `pg_try_advisory_lock`/`pg_advisory_unlock`（键由锁名 FNV-1a 折叠为 int64），连接池按 `Acquire`/`Release` 配对；被保护的同步工作跨多个事务，故不用事务级锁。装配点：`internal/bootstrap/runtime.go`（`Service.WithLockProvider(postgres.NewAdvisoryLockProvider(pool))`）。定时任务按**每 config 一把锁**执行：取锁成功后由 `runConfigLocked()` 完成 `MarkSyncRunning` → `executeSync` → 结果落库，拿不到锁的实例直接跳过本次调度（`slog.Info` 记录）；锁释放使用独立的 5s 超时 context，避免调用方 context 取消导致锁泄漏。

**方言隔离**：`pg_advisory_xact_lock` 仅 PostgreSQL 可用，跨方言不得把方言 SQL 写进 app 层（呼应第 2 章分层红线）。规范：

| 方言 | 方案 | 关键约束与风险 |
|------|------|---------------|
| PostgreSQL | `pg_advisory_xact_lock`（事务级） | 事务结束自动释放；现有首选 |
| MySQL 8.4+ | `GET_LOCK`（**会话级**） | 连接归还池前**必须** `RELEASE_LOCK`（连接池钩子强制）；风险：忘释放 → 连接复用后死锁/互斥失效；`GET_LOCK` 持元锁可阻塞同命名锁的 DDL；长事务持锁放大竞争。优先评估改用应用层锁表 |
| 达梦（DM） | 无原生 advisory lock → **应用层锁表** | 独立锁表（`lock_name` 唯一 + `locked_by`/`locked_at` + 租约回收），与 Outbox 租约回收（[outbox Worker 参数与投递语义](design/infrastructure/outbox.md#worker-参数与投递语义)）同款语义；锁行清理任务随 TTL 清理同批 |

统一要求：app 层只依赖 `infra` 层 `LockProvider` 接口（`Acquire/Release/Refresh`），方言差异全部封装在 `internal/infra/`；`bash scripts/check-layers.sh` 负责把关 app 层无方言 SQL。

### 18.5 分布式机制参数红线

八类机制的参数默认值表与失败场景见 [webauthn](design/auth/webauthn.md)/[permissions §6](design/iam/permissions.md#6-数据隔离-rls-策略)/[outbox](design/infrastructure/outbox.md)/[mfa](design/auth/mfa.md)；本节是工程侧红线（新增参数、分层、验证命令）：

| 机制 | 红线 | 验证命令 |
|------|------|---------|
| Outbox Worker | 轮询/批大小/租约/max_attempts 全部走 `OUTBOX_*` 环境变量（实现时登记环境变量注册表 D.12）；`SKIP LOCKED` 领取必须短事务；死信必须迁表 + `records` 审计 + 告警 | `go test ./internal/platform/outbox/...`；第 19 章结构门禁 |
| Webhook | 签名头 `t=,v1=` 双段格式与 ±5min 防重放不得简化；SSRF 出口管控为共享出站 Client，五个出口（Webhook/pgtUrl/LDAP/Syncer/SCIM）禁止各自绕过 | `go test ./internal/app/webhooks/... ./internal/platform/ssrf/...` |
| 统一幂等 | `request_hash` 规范化算法与 `scope` 取值以 [outbox 统一幂等](design/infrastructure/outbox.md#统一幂等) 为唯一口径；业务 `idempotency_key` 列进入废弃收敛（新旧并行），禁止新端点再加业务幂等列 | `go test ./internal/platform/middleware/...`；`bash scripts/verify-docs.sh` |
| 限流 | 参数**只允许**附录 D 已登记变量（D.4 现有 + D.12 新增），禁止散落新变量；Redis 不可用时认证面 fail-close（本地桶减半）、管理面只读 fail-open | `go test ./internal/transport/http/middleware/... ./internal/platform/middleware/...` |
| 缓存 | TTL 必须带 ±10% 抖动；同实例回源必须 single-flight；缓存键必须含租户维度；用户/组织缓存失效必须事件驱动 + 30s TTL 上界双保险 | `go test ./internal/infra/cache/...` |
| 密钥轮换 | 见 14.7 五条红线（事务边界/时延预算/批次/双人审批/命名登记） | `go test ./internal/platform/crypto/...` |
| RLS | `FORCE ROW LEVEL SECURITY` + 非 owner 应用连接**双保险**缺一不可；`set_config` 必须同事务且连接归还 `RESET`；受控表增量必须同批登记 | RLS 集成测试（[permissions §6](design/iam/permissions.md#6-数据隔离-rls-策略) 出口条件三断言） |
| 时钟 | 生产节点必须 chrony + 4 内网源 + 公网 `noselect` 备用；漂移 >200ms 告警；协议容忍值以 [mfa 生命周期参数](design/auth/mfa.md#生命周期参数) 表为唯一口径（TOTP ±90s） | 部署基线检查（OPS）；监控规则含 `chronyc tracking` 采样项 |
| 分布式锁 | 见 18.4 方言隔离：app 层只依赖 `LockProvider` 接口 | `bash scripts/check-layers.sh` |
| Redis 技术栈 | 引入前必须完成 ADR（Redis 8.1 + go-redis v9 + Sentinel 起步）；`REDIS_*` 变量登记附录 D；禁止在 ADR 前散落引入任何 Redis 客户端依赖 | `grep -r "redis" go.mod`（ADR 前应为 0 命中）；`bash scripts/verify-docs.sh` |

---

## 19. 数据库变更与门禁

> 本章对齐 **ADR-022**：**声明式 Schema 是数据库结构的唯一事实源**，各方言 DDL 与版本化迁移、回滚脚本由生成器产出（已取代旧的「单一幂等 DDL 重放」策略）。

### 19.1 迁移与回滚

| 规则 | 说明 |
|------|------|
| 唯一事实源 | 声明式 Schema 唯一定义结构；各方言 DDL、版本化迁移、回滚脚本与嵌入副本均为生成产物，禁止手写维护 |
| 变更表达 | 任何结构变更（新增、删除、改类型、重命名、约束收紧）均以版本化迁移表达，每个版本必须附带对应回滚脚本；禁止散装 SQL、人工 DDL 与一次性运维语句入库 |
| 破坏性语句 | 迁移产物中禁止手写 `DROP SCHEMA/TABLE/INDEX`、`TRUNCATE`；开发库重置统一走 `scripts/dev/reset_schema.sql`（仅限本地与 CI，禁止在生产执行） |
| 生产纪律 | `AUTO_MIGRATE` / `AUTO_SEED` 默认关闭，生产不得启用；本地服务启动用 `-migrate`/`-seed` 显式开启；生产初始化与升级由运维执行生成器产出的迁移 |
| 产物一致性 | 生成产物与嵌入副本（`internal/bootstrap/schema_sql/`）必须逐字节一致，`scripts/check-db-conventions.sh` 按 SHA-256 校验 |
| 备份与窗口 | 执行破坏性变更前必须完成全量备份并校验恢复；变更在低峰期执行，执行前 `SET lock_timeout = '5s'` 避免长事务阻塞 |
| 应用发布顺序 | 先发布兼容新旧两种结构的应用版本，再执行破坏性变更 |
| 大表例外 | 对 > 100 万行表的变更须附 `EXPLAIN (ANALYZE, BUFFERS)` 证据与数据量说明：建索引用 `CREATE INDEX CONCURRENTLY`（由 DDL 生成器/运维路径产出，不进入常规迁移）；改列类型走「新建列 → 双读双写 → 回填 → 切换 → 删旧列」；数据回填分批（每批 1 万行）提交、记录游标、可断点续跑，避免长事务 |

### 19.2 CI 结构门禁十项

`gates` 作业对每次数据库变更执行以下结构门禁，全部通过方可合并（任一失败即阻塞）：

| # | 门禁 | 验证内容 |
|---|------|---------|
| 1 | 空库全量建表 | 从空库执行全量建表产物，结构完整可建 |
| 2 | 增量迁移前滚 | 存在 V{N}+ 生成式增量时，从 V{N-1} 结构执行增量迁移，收敛到当前声明式结构；增量版本链以 `db/postgresql/` 目录与迁移账本为准（复现：`ls db/postgresql/V*.sql`） |
| 3 | 回滚 | 存在 V{N}+ 增量时，其配对回滚脚本可执行且回到上一版本结构（V1 全量基线的回滚由部署流水线重建/备份恢复承担） |
| 4 | 外键/索引/约束/CHECK 可解析 | 迁移产物中全部 FK、索引、约束、CHECK 语句可被方言解析 |
| 5 | ON CONFLICT 仲裁目标存在唯一约束 | UPSERT 仲裁目标必须存在匹配的唯一约束/索引（部分唯一索引须原样重复谓词，见 3.4.2） |
| 6 | 引用列类型一致 | 外键列与被引用列类型一致 |
| 7 | 软删除唯一性数据库强制 | 软删除表业务唯一键以 `_active` 部分唯一索引或方言等价方案在库内强制（见 3.4.1） |
| 8 | 租户表 RLS 强制 | 多租户表提交 `ENABLE ROW LEVEL SECURITY` + `FORCE` 与租户策略（见 3.6.1） |
| 9 | 生成物零差异 | 生成器产出的 DDL、版本化迁移、回滚脚本、OpenAPI、文档表格与工作区文件比对，任一不一致即失败（ADR-022） |
| 10 | 方言能力探测 | 镜像方言（MySQL，ADR-023）对 RLS、部分唯一索引、`SKIP LOCKED`、JSON、数组能力的探测；缺失即落地等价方案（登记 ADR）或拒绝启动，禁止静默降级安全能力 |

### 19.3 Seed 数据规范

| 规则 | 说明 |
|------|------|
| 幂等 | Seed 脚本可重复执行 |
| 使用 `ON CONFLICT DO NOTHING` / `DO UPDATE` | 避免重复插入；仲裁目标必须有匹配的唯一约束或部分唯一索引 |
| 不包含生产数据 | Seed 仅含初始配置 |
| `AUTO_SEED=false` | 生产禁止自动 Seed |

### 19.4 数据生命周期规范

> **一致性约定**：本表与 [requirements/data.md §3「数据生命周期与归档策略」](requirements/data.md#3-数据生命周期与归档策略) 均是 `contracts/registries/lifecycle.yaml` 的投影，数值以注册表为准。本表是工程实现口径（含清理任务约束），需求 §3 是需求口径；任一侧修改必须同步另一侧并复核注册表。
>
> `entries` 登录条目的 **180 天**是运维默认值，可在保留策略配置中调整，但不得在本表与需求 §3 之间出现两个默认值。

| 数据类别 | 保留策略 | 处置方式 |
|------|------|------|
| 审计记录（`records`） | 按合规要求长期保留（本体不设定期删除，默认最短 1 年） | 归档到对象存储，`records` 本体只在显式策略下删除；严禁应用层 UPDATE |
| 登录条目（`entries`） | 默认 180 天 | 分批物理删除，保留聚合统计 |
| Webhook 事件与投递（`webhook_events` / `outbox`） | 默认 90 天 | 按 `created_at` 分区/清理，死信保留至人工处理完成 |
| 令牌与会话（`access_tokens` / `refresh_tokens` / `sessions`） | 过期即物理删除 | 清理任务按 `expires_at` 分批删除，保留 `revoked_at` 用于安全分析的时间窗 |
| 幂等键（`idempotency_keys`） | 24 小时 | 按 `expires_at` 物理删除 |
| Remember-me 家族与轮换令牌（`remember_me_families` / `remember_me_tokens`） | 30 天（代码常量 `RememberMeTTL`）；到期即失效 | 到期行由 session-cleanup 每小时分批物理删除（批次 1000、单轮最多 10 批，见 lifecycle.yaml cleanup_cron）；读路径按 `expires_at` / `revoked_at` / `consumed_at` 过滤语义不变；随租户/用户/家族级联删除 |
| 信任设备（`trusted_devices`） | 30 天（代码常量 `trustedDeviceTrustDuration`）；到期即不再受信 | 到期行由 session-cleanup 每小时分批物理删除（口径同上）；登录侧按 `revoked_at` / `expires_at` 过滤，列表标记 `expired`；随租户/用户级联删除 |
| Outbox 事件（`outbox`） | `succeeded` 保留 7 天，`dead` 保留至人工处置完成（最短 30 天） | 分批删除已投递事件；`dead` 未处置前不得删除 |
| 审计链锚点（`audit_chain_anchors`） | 与 `records` 同生命周期，链段闭环前禁止物理删除 | 随 `records` 归档链路冷归档；锚定失败只补链不覆盖 |
| 业务主数据软删除（带 `deleted_at` 的业务表） | 软删除保留 365 天；交易/订单/支付类商业合规 ≥ 3 年 | 按 `deleted_at` 分批物理删除，删除前经隐私擦除检查，恢复窗口内支持 undelete |
| 一次性凭据与短期授权（`authorization_codes` / `device_auth_codes` / `cas_tickets` / `kerberos_tickets` / `login_challenges` / `webauthn_challenges` / `web3_nonces` / `verifications` / `invitations` / `access_lifecycle` / `qr_code_sessions` / `face_id_sessions`） | 过期即物理删除；TTL 以 `short-lived-state.yaml` 与代码常量为唯一口径 | 按 `expires_at` 分批删除；一次性凭据使用后立即失效，禁止仅依赖 TTL，且不得进入缓存 |
| 密钥材料（`keys` / `llm_api_keys` / `provider_secrets`） | 轮换重叠窗口 60 天；retired 后宽限期 60 天 | 宽限期结束才物理删除；提前删除会使旧令牌/旧密文永久不可解（ADR-015） |
| 安全证据日志（`dpop_replay` / `lockout_attempts` / `user_password_history` / `secret_rotation_audit` / `encryption_reencrypt_progress` / `authorization_audit_records`） | 默认 180 天（重放防护类不得短于 nonce/proof 最大时效） | 按 `created_at` 分批物理删除 |
| 隐私权利请求证据（`dsar_events` / `consent_records`） | 3 年（合规证据） | 先归档证据包再分批物理删除；撤回记录在留存期内不得删除 |
| 运行日志（`execution_logs`） | 默认 90 天 | 按 `created_at` 分批删除，保留聚合指标 |
| 计量与计费证据（`metering_events` / `llm_usage_logs` / `llm_metering_events`） | 3 年（计费对账与商业合规证据） | 先归档后分批删除；聚合统计长期保留 |
| 支付尝试证据（`pay_attempts`） | 3 年（对账与争议处理证据） | 先归档后分批物理删除，禁止无 `LIMIT` 的 `DELETE` |
| 临时角色绑定（`user_role_bindings` 的 `expires_at`） | 到期即丧失授权；数据保留 1 年 | 按 `expires_at` 分批删除过期绑定，保留期用于提权追溯 |
| 跨租户操作单（`cross_tenant_operations` 的 `expires_at`） | 到期即授权失效；数据保留 1 年 | 按 `expires_at` 分批删除，保留期用于平台侧审计 |
| 用户隐私擦除 | 用户请求后 30 天内完成 | 匿名化个人字段并保留审计骨架，会话/令牌/凭证全部物理删除 |

**覆盖口径（机检）**：凡带 `deleted_at` 或 `expires_at`/`*_expires_at` 列的表、以及追加型明细/证据表，必须在 `contracts/registries/lifecycle.yaml` 有显式条目或策略组 `applies_to` 覆盖；未命中的业务主数据/配置数据按 `coverage.default_policy`（随租户生命周期保留 + 级联删除 + 备份策略）处理。门禁：`python3 scripts/check-lifecycle-coverage.py`（由 `verify-docs.sh` 调度）。新增表若引入上述列或命名落入追加型模式，必须同批补生命周期条目并在本节登记。

清理任务必须是**分批、可断点续跑、幂等**的，并输出删除行数到指标；禁止无 `LIMIT` 的批量 `DELETE`。

## 附录 A. 分层违规自动化检查脚本

**文件**：`scripts/check-layers.sh`

```bash
#!/bin/bash
# 分层违规检查脚本
# 用法: bash scripts/check-layers.sh

set -e
EXIT_CODE=0

echo "=== 检查 1: app 层禁止导入 infra 层 ==="
if grep -rn "internal/infra" internal/app/ 2>/dev/null; then
    echo "❌ FAIL: app 层存在 infra 导入"
    EXIT_CODE=1
else
    echo "✅ PASS: app 层无 infra 导入"
fi

echo "=== 检查 2: domain 层禁止导入 app/transport/infra ==="
if grep -rn "internal/\(app\|transport\|infra\)/" internal/domain/ 2>/dev/null; then
    echo "❌ FAIL: domain 层存在上层导入"
    EXIT_CODE=1
else
    echo "✅ PASS: domain 层无上层导入"
fi

echo "=== 检查 3: platform 层禁止导入其他层 ==="
if grep -rn "internal/\(app\|transport\|infra\|domain\|bootstrap\)/" internal/platform/ 2>/dev/null; then
    echo "❌ FAIL: platform 层存在上层导入"
    EXIT_CODE=1
else
    echo "✅ PASS: platform 层无上层导入"
fi

echo "=== 检查 4: 禁止标准库 log 包 ==="
if grep -rn '^import.*"log"' internal/ 2>/dev/null | grep -v "_test.go"; then
    echo "❌ FAIL: 存在标准库 log 导入"
    EXIT_CODE=1
else
    echo "✅ PASS: 无标准库 log 导入"
fi

echo "=== 检查 5: 禁止无 %w 的 fmt.Errorf ==="
if grep -rn 'fmt\.Errorf("[^"]*")' internal/ 2>/dev/null | grep -v "%w"; then
    echo "⚠️  WARN: 存在无 %w 的 fmt.Errorf（应使用 errors.New*）"
fi

echo "=== 检查 6: 禁止 SELECT * ==="
if grep -rn 'SELECT \*' internal/infra/postgres/ 2>/dev/null; then
    echo "❌ FAIL: 存在 SELECT * 语句"
    EXIT_CODE=1
else
    echo "✅ PASS: 无 SELECT * 语句"
fi

echo "=== 检查 7: 前端硬编码中文 ==="
if grep -rPn "[\x{4e00}-\x{9fff}]" web/src/ --exclude-dir=i18n --exclude-dir=node_modules 2>/dev/null; then
    echo "⚠️  WARN: 前端存在硬编码中文"
fi

echo "=== 检查 8: SQL 字符串拼接 ==="
if grep -rn 'Exec(ctx, "[^"]*" +' internal/infra/postgres/ 2>/dev/null; then
    echo "❌ FAIL: 存在 SQL 字符串拼接"
    EXIT_CODE=1
else
    echo "✅ PASS: 无 SQL 字符串拼接"
fi

exit $EXIT_CODE
```

**CI 集成**：`ci.yml` 的 `gates` 作业将本脚本作为**独立阻塞步骤**执行；本机复现命令与 CI 命令完全一致。

---

## 附录 B. API 端点索引

> **事实源与口径**：端点清单、operation 数与模块归属不在本手册手写。声明源为 `internal/transport/http/api/v1/*/routes.go` 的 `route.Definition`（根级协议路由的注册声明见 `internal/transport/http` 各 `chi.go`）；机读生成物为 `openapi/openapi.yaml`，由 `python3 scripts/gen_openapi.py` 生成。复现与校验：`python3 scripts/gen_openapi.py --check`；路由声明与 `chi.go` 实际注册的双向对齐由 `internal/bootstrap/route_contract_test.go` 阻塞校验，OpenAPI 契约由 `scripts/check-openapi-contract.sh` 校验。任何端点数量、模块归属或路径前缀以生成器输出与上述门禁为准。

**按 API 面分布**：`route.Definition.Surface` 显式标注面归属（管理 / 认证 / 协议，见 4.1.3），分布由 `internal/bootstrap/surface_contract_test.go::TestSurfaceContract` 断言；OpenAPI tag 带面前缀（`management: <module>` 等）。各面的路径家族见 4.1.3，不在本附录复写。

**根级与内联注册**：根级协议路由（`/.well-known/*`、`/cas/*`、`/oauth2/register`、`/healthz`、`/metrics` 等）的注册位置与家族归属见 4.1.3 的家族表；其存在性与路径以 `openapi/openapi.yaml` 与 `internal/bootstrap/route_contract_test.go` 为准。

**总计口径**：`openapi/openapi.yaml` 的 operation / path / `$ref` 总数由生成器产出，并由 `check-openapi-contract.sh` 对账；悬空 `$ref` 必须为 0。

---

## 附录 C. 错误码全量清单（引用）

应用与协议错误码唯一事实源为 `contracts/registries/errors.yaml`，渲染层为 `docs/generated/errors-registry.md`；本章仅保留指向。应用错误码（`internal/platform/errors` 命名空间）与 OAuth / OIDC 协议错误码（RFC 6749 / 6750 / 9449 命名空间）两套清单及其 HTTP 映射、语义与使用场景，均以该注册表为准；代码常量事实源为 `internal/platform/errors/errors.go`（`codeToHTTPStatus`）。复现与校验：`python3 scripts/gen_registry_md.py --check`。

- **不得新增未登记码**：新增错误码必须改动 `errors.go` 常量并在注册表登记（变更顺序：代码 → 注册表 → `CHANGELOG.md`）。
- **协议错误走协议写入器**：全部错误响应必须经 `route.WriteError` / `WriteOAuthError` / `WriteSCIMError` 写出，协议码从应用层到达协议面唯一经 `errors.ProtocolCodeError` 桥接；实施纪律（前端回退对齐、登记例外）见注册表第 3 节。

---

## 附录 D. 环境变量完整清单（引用）

环境变量唯一口径已迁至 `generated/env-registry.md`；本章仅保留指向。分组结构（D.1 环境标识与密钥、D.2 数据库与连接池、D.3 多数据库方言与驱动、D.4 HTTP/服务标识/限流与可观测性、D.5 对象存储、D.6 邮件与短信渠道、D.7 推送通知/社交与消息渠道、D.8 验证码/身份核验/支付与地理位置、D.9 协议服务与集成、D.10 前端构建变量、D.11 内置默认值回落纪律、D.12 通用 API 契约新增变量）与每个变量的名称、必填性、默认值、语义、生产约束，均以该注册表为准。

- 代码事实源为读取点枚举（`os.Getenv` / `envOrEmpty` / `envInt` 口径，见 `internal/bootstrap/runtime.go`）；`.env.example` 是示例覆盖值而非代码默认值；生产必填项以注册表「生产必需 / 必填」标记为准。
- **使用规则**：新增环境变量必须先在注册表登记，否则视为未定义配置，严禁先写代码后补登记；内置默认值回落纪律（`CURSOR_HMAC_KEY`、`ETAG_HMAC_KEY` 仅限非生产；`ENCRYPTION_KEY_CURRENT` 全环境必填）见注册表 D.11。

---

## 附录 E. 术语与缩写对照表

| 术语/缩写 | 定义 |
|-----------|------|
| IAM | Identity and Access Management，身份认证与访问管理 |
| OIDC | OpenID Connect，基于 OAuth 2.0 的身份层协议 |
| SAML | Security Assertion Markup Language，安全断言标记语言 |
| CAS | Central Authentication Service，中央认证服务 |
| RBAC | Role-Based Access Control，基于角色的访问控制 |
| ABAC | Attribute-Based Access Control，基于属性的访问控制 |
| MFA | Multi-Factor Authentication，多因子认证 |
| TOTP | Time-based One-Time Password，基于时间的一次性密码 |
| PKCE | Proof Key for Code Exchange，授权码交换证明密钥 |
| DPoP | Demonstrating Proof-of-Possession，令牌绑定证明 |
| DCR | Dynamic Client Registration，动态客户端注册 |
| SCIM | System for Cross-domain Identity Management，跨域身份管理 |
| RADIUS | Remote Authentication Dial-In User Service |
| WebAuthn | Web Authentication，Web 认证 API |
| IdP | Identity Provider，身份提供者 |
| SP | Service Provider，服务提供者 |
| ACS | Assertion Consumer Service，断言消费服务 |
| UUIDv7 | 基于时间戳的通用唯一标识符版本 7 |
| UoW | Unit of Work，工作单元 |
| SLO | Service Level Objective，服务等级目标 |
| RTO | Recovery Time Objective，恢复时间目标 |
| RPO | Recovery Point Objective，恢复点目标 |
| HPA | Horizontal Pod Autoscaler，水平 Pod 自动扩缩 |
| PDB | Pod Disruption Budget，Pod 中断预算 |
| A11y | Accessibility，无障碍 |
| WCAG | Web Content Accessibility Guidelines |
| i18n | Internationalization，国际化 |
| l10n | Localization，本地化 |
| PostgreSQL-first | 本项目对多数据库策略的命名：**PostgreSQL 18+ 是唯一受支持的运行期数据库**，`first` 表示部署、安全能力与迁移版本链均以 PostgreSQL 为准；MySQL 仅为 infra 层镜像方言，运行期可用性受 `OPEN-API-37` 约束（ADR-023） |
| 管理面 / 认证面 / 协议面 | 见 4.1.3。按**契约与消费者**划分的三类 HTTP 接口：管理面（`/api/v1/*`，控制台与管理员）、认证面（`/api/v1/auth/*` 页面与会话类、自带登录页）、协议面（RFC/OASIS 标准规定响应体的端点） |
| `surface` | 三面标识，取值仅限 `management` / `authentication` / `protocol`；用于指标标签、审计列与 OpenAPI tag 前缀（见 4.1.3、12.2、12.5） |
| PEP | Policy Enforcement Point，策略执行点。本项目中指 `internal/transport/http/middleware.APIPermissionMiddleware`（由 `auth.APIAuthorizationMiddleware` 装配进 `bootstrap/chi.go` 的 `protected` 链） |
| PDP | Policy Decision Point，策略决策点。本项目中指 `internal/infra/casbin.Manager`（`Enforce` / `GetOrCreateEnforcer`，模型来自 `models` 表、适配器来自 `adapters` 表） |
| PIP | Policy Information Point，策略信息点。本项目中指策略与属性的来源集合：`internal/domain/permissions.CasbinPolicyManager`、`internal/app/permissions`、`casbin_rules` 表、`middleware.RBACAuthz` 的角色提取与 `TenantFilterMiddleware` 的租户属性 |
| Identity Warehouse | 身份仓库，指系统内承载身份事实的存储层集合（用户与档案、凭据、组织与授权、会话与令牌、外部身份映射），见 [system-overview.md §2](architecture/system-overview.md) |
| BFF | Backend for Frontend，为前端服务的后端层；要求浏览器端只与同源 BFF 交互，令牌不下发到 `localStorage`，长期刷新令牌仅存服务端（见 [security-model.md §1 安全红线](architecture/security-model.md#1-安全红线66)） |
| Passkey | 可同步的 FIDO2/WebAuthn 凭据（保存在平台或跨设备认证器中），用户以生物特征或设备 PIN 完成无密码登录；需求见 SRS AUTH-099~102，设计见 design/auth/webauthn.md |
| ReBAC | Relationship-Based Access Control，基于关系的访问控制：以关系元组 `(user, relation, object)` 与命名空间展开规则求值（Google Zanzibar 模型），见 [requirements/functional.md §12](requirements/functional.md)、[security-model.md §2 ReBAC](architecture/security-model.md)、ADR-024 |
| Zanzibar | Google 提出的全局关系型授权系统模型，核心概念为关系元组、命名空间展开规则与一致性令牌（Zookie）；本项目 ReBAC 目标态对标该模型（REBAC-001~008、REBAC-010） |
| IGA | Identity Governance and Administration，身份治理与管理：访问认证（Campaign）、访问请求与多级审批、JIT/JEA 到期回收、SoD 互斥与角色挖掘，见 [requirements/functional.md §13](requirements/functional.md)、ADR-025 |
| PAM | Privileged Access Management，特权访问管理：特权凭据保险柜、审批式提权与特权会话录制；与支持/排查用途的受控模拟（Impersonate）分域，见 SRS IGA-008~011 |
| JIT | Just-in-Time Access，即时访问：授权仅在声明的生效时间窗内有效、到期自动回收（IGA-003/004） |
| JEA | Just-Enough-Administration，刚好够用管理：按任务声明最小权限范围，超出范围的申请被拒绝；与 JIT 共同构成临时特权机制 |
| SoD | Segregation of Duties，职责分离：以互斥矩阵阻止同一人持有不相容角色/权限，申请时与角色变更时（含 SCIM/批量导入路径）双时机校验（IGA-005） |
| Workload Identity | 工作负载身份：赋予服务/进程/自动化任务的一等身份（service account、短期凭据、联邦信任），区别于人类用户；见 [requirements/functional.md §16](requirements/functional.md)、[security-model.md §4 Workload 身份平面](architecture/security-model.md#4-workload-身份平面adr-026)、ADR-026 |
| SPIFFE | Secure Production Identity Framework for Everyone，工作负载身份的开源标准（SPIFFE ID + SVID 可验证凭据）；本项目作为 Workload 联邦的方向锚点（WLI-006） |
| CAEP | Continuous Access Evaluation Profile，持续访问评估事件 profile（OpenID Shared Signals 家族）：定义会话撤销、令牌声明变更等事件的标准结构，见 RISK-010 |
| SSF | Shared Signals Framework（RFC 8935 推送 / RFC 8936 轮询），共享信号框架：风险与会话事件向已注册接收方的标准化投递机制，见 RISK-010 |
| RISC | Risk Incident Sharing and Cooperation，OpenID 风险事件共享规范：消费上游 IdP 风险/会话失效事件以联动本系统吊销，见 RISK-012 |
| PAR | Pushed Authorization Requests（RFC 9126），推送授权请求：客户端先把授权参数推送到授权服务器换取一次性 `request_uri`，再发起授权请求；见 design/protocol/oauth2.md §14.18 |
| JAR | JWT-Secured Authorization Request（RFC 9101），JWT 签名授权请求：授权参数经签名的 request 对象承载以防篡改，可与 PAR 组合；见 design/protocol/oauth2.md §14.19 |
| JARM | JWT-Secured Authorization Response Mode，JWT 安全授权响应模式：授权响应以服务端签名（可选加密）的 JWT 经 redirect_uri 返回；见 design/protocol/oauth2.md §14.20 |
| CIBA | Client Initiated Backchannel Authentication（OpenID CIBA 1.0），客户端发起的后通道认证：用户在认证设备上确认后，客户端以 ping/poll 模式取得令牌；见 design/protocol/oauth2.md §14.21 |
| RAR | Rich Authorization Requests（RFC 9396），富授权请求：以结构化 `authorization_details` 表达细粒度授权范围；见 design/protocol/oauth2.md §14.22 |
| FAPI | Financial-grade API，金融级 API 安全 profile（FAPI 2.0）：对客户端认证、令牌绑定与协议流程提出更高安全要求；见 design/protocol/oauth2.md §14.29 |
| mTLS | Mutual TLS（RFC 8705），双向 TLS 客户端认证与证书绑定访问令牌（`cnf.x5t#S256`）；本项目按 ADR-014 落地，见 design/protocol/oauth2.md §14.26 |
| PEP / PDP / PIP 三者关系 | 策略执行点 PEP 拦截请求并执行决策、策略决策点 PDP 给出 allow/deny、策略信息点 PIP 提供决策所需角色与属性；调用链为「PEP → PDP ← PIP」；本项目实现映射见 [layers.md §2](architecture/layers.md)（PEP/PDP/PIP 单列定义见本附录） |
| DSAR | Data Subject Access Request，数据主体权利请求（查阅/更正/删除/可携带等）；合规工作台目标见 [requirements/non-functional.md §5.2](requirements/non-functional.md)（PRIV 域），风险画像摘要口径见本手册 9.18 |
| BYOK | Bring Your Own Key，客户自持密钥：租户提供密钥材料、平台在 KMS 内信封托管且租户可轮换/吊销；更进一步的 HYOK 指密钥全程不离开租户侧 KMS/HSM；见 DR-003、[multi-region.md §5.1 BYOK/HYOK](architecture/multi-region.md#51-byokhyokdr-003) |
| 数据驻留 | Data Residency：租户个人数据的存储与处理在其声明区域内完成，跨区域访问受控并审计，区域间不复制个人数据（去标识聚合统计除外）；见 PRIV-010、DR-001、[multi-region.md](architecture/multi-region.md) |
| Identity Actions（Hook） | 身份事件挂钩机制：在 pre-login / pre-token / post-registration / consent / MFA 等触发点运行租户自定义逻辑，返回继续/修改/拒绝指令；沙箱化执行，见 EXT-001、[system-overview.md §4 生产级 IAM 对齐能力](architecture/system-overview.md#4-生产级-iam-对齐能力)、本手册 5.7、ADR-029 |
| 关系元组（relation tuple） | ReBAC 的最小授权事实单元 `(user, relation, object)`，对象带类型命名空间与租户归属（如 `folder:org1/f12`）；持久化于目标表 `relation_tuples`，见 REBAC-001、ADR-024 |
| AAGUID | Authenticator Attestation GUID，认证器型号唯一标识；可据此实施认证器白/黑名单与同步 passkey 策略（AUTH-101、design/auth/webauthn.md） |
| step-up authentication | 增强认证（步进认证）：访问敏感资源或风险升高时，在既有会话上要求更强的额外认证因素（MFA/WebAuthn 二次验证）；见 AUTH-105、RISK-005、AC-AUTH-015 |
| tenant tier（租户分级） | 按 shared / bridged / dedicated 三档把租户与限流配额、隔离强度、备份频率和可选数据驻留挂钩的商业化分层模型；见 ORG-016/019/020、[multi-region.md §6.3 租户隔离分级套餐](architecture/multi-region.md#63-租户隔离分级套餐) |

---

## 附录 F. 文档地图与维护责任

| 文档 | 位置 | 维护者 | 更新频率 |
|------|------|--------|---------|
| 开发规范（本文件） | `docs/GuliangIAM-开发规范.md` | 架构组 | 规范变更时 |
| 需求规格 | `docs/requirements/functional.md` | 产品 | 需求变更时 |
| 设计说明 | `docs/design/README.md` | 架构 | 架构变更时 |
| 架构说明 | `docs/architecture/README.md` | 架构 | 架构变更时 |
| 部署指南 | `docs/operations/README.md` | DevOps | 部署变更时 |
| 密钥轮换 | `docs/architecture/security-model.md` | 安全 | 密钥流程变更时 |
| 无障碍规范 | 本手册第 15 章 | 前端 | 组件规范变更时 |
| 性能基线 | `docs/architecture/observability.md` | 性能组 | 测量方法变更时 |
| 发布验证 | CI 工作流与发布记录 | QA | 每次发布 |
| 问题与风险清单 | `contracts/registries/known-red.yaml` | 架构 | 风险变更时 |
| OpenAPI 规范 | `openapi/openapi.yaml` | 后端（自动生成） | 每次 API 变更 |
| README | `README.md` | 全体 | 按需 |
| 安全策略 | `SECURITY.md` | 安全 | 每半年 |

---

## 附录 G. 违规处理与豁免流程

| 级别 | 处理方式 |
|------|---------|
| 严重违规（安全/数据泄露） | 立即修复，代码回滚 |
| 一般违规（命名/格式/分层） | PR 评审指出，修改后合并 |
| 特殊豁免 | 提交豁免申请，架构师审批，代码中注释说明豁免理由和有效期 |

**豁免申请格式**：

```
豁免申请: [规范条款编号]
理由: [为什么必须豁免]
替代方案: [如何补偿]
有效期: [何时复审]
审批人: [架构师签名]
```

---

## 附录 H. 推荐 IDE 配置与插件

**Go 开发**：

- Go 扩展（官方）
- gopls（语言服务器）
- goimports（导入排序）
- golangci-lint（本地 lint）
- Delve（调试）

**前端开发**：

- VS Code + Oxlint 扩展
- TypeScript 扩展
- axe-core 浏览器扩展
- React Developer Tools

**通用**：

- EditorConfig
- GitLens
- Error Lens
- Docker 扩展
- Kubernetes 扩展

---

## 附录 I. 自动化检查清单

### I.1 门禁脚本清单

> `ci.yml` 的阻断型 **`gates` 作业**逐项执行下表脚本（`docker` 作业 `needs` 含 `gates`，任一失败即不进入镜像构建）；统一本地复现入口：`RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh`（依次执行分层检查 → 数据库规范 → SQL 可移植性 → 编译 → vet → test → OpenAPI 零差异 → OpenAPI 契约校验 → 前端 lint/build/test）。

| 检查 | 命令 | 检查内容 | CI 阻塞 |
|---|---|---|---|
| 文档一致性 | `bash scripts/verify-docs.sh` | 文档与代码签名一致（统一复现入口，内含 build/vet/test 与前端 lint/build/test） | 是 |
| 文档链接完整性 | `python3 scripts/check-doc-links.py` | `docs/` 与根目录正式文档的相对链接全部可解析（跳过围栏代码块/行内代码/外部 URL）；事实源搬迁后指向旧位置的断链、逃逸仓库根的链接均阻塞 | 是 |
| 数据生命周期覆盖 | `python3 scripts/check-lifecycle-coverage.py` | `contracts/schema/*.yaml` 中命中 `deleted_at`、`expires_at`/`*_expires_at`、追加型命名的表必须被 `lifecycle.yaml` 显式条目或策略组 `applies_to` 覆盖；幽灵条目/悬空 `applies_to` 即失败 | 是 |
| 门禁接线覆盖 | `python3 scripts/check-gate-coverage.py` | 每个 `scripts/check-*` 必须被 `verify-docs.sh`/CI/Makefile 引用，或在 `RETIRED_GATES` 登记退役理由；`run_gate`/CI 引用的脚本必须存在；文档中反引号内的脚本路径与仓库相对路径必须可解析（禁止幽灵门禁与路径漂移；目标态产物须改为引用 `contracts/registries/open-items.yaml` 的表述） | 是 |
| 表间引用完整性 | `python3 scripts/check-fk-coverage.py` | UUID 类型 `*_id` 列必须被 `fk_constraints` 覆盖，或在 `contracts/registries/referential-integrity.yaml` 登记 `logical_references` / `pending_physical_fks`（后者须指向开放项）；悬空/重复/指向不存在列的登记即失败 | 是 |
| RLS 强制隔离 | `bash scripts/check-rls.sh` | 含 `organization_id` 的租户表必须启用 RLS（按 YAML 语义解析，兼容 `rls: true` 与 `rls: {force: true}`）；生成 SQL 必须 ENABLE+FORCE；`guliangiam_app` 应用角色存在且被授权（非 owner） | 是 |
| 目标域表完整性 | `bash scripts/check-target-schema.sh` | ADR-024~029 的目标域表必须同时存在于声明式 Schema 与生成 SQL，且两者零漂移 | 是 |
| 缓存注册表一致性 | `bash scripts/check-cache-registry.sh` | 缓存分层/TTL/失效策略/租户键前缀/四条红线必须与 `cache.yaml` 渲染层一致 | 是 |
| 数据库规范（9 项） | `bash scripts/check-db-conventions.sh` | 命名、必备列、约束、索引等 DDL 约定（3.8） | 是 |
| 仓储 SQL 列漂移 | `python3 scripts/check-sql-columns.py` | 全部 `sql.go` 与 `repository.go` 中表/列引用可解析；静态检查 INSERT 列表、UPDATE SET 目标、`alias.column` 引用、占位符 cast 与目标列类型；不覆盖无 FROM 的拼接片段（由下一行覆盖） | 是 |
| 仓储 SQL 语句级契约 | `SQL_CHECK_DSN=… python3 scripts/check-sql-runtime.py` | 全部语句可在真实 PostgreSQL 中 `PREPARE`——发现类型不匹配（`uuid = text`）、未使用参数序号（`$2` 悬空）、多表 UPDATE 同名列歧义、指向不存在的表；每次运行先重置库再重放建表产物（DSN 取自 `SQL_CHECK_DSN` 专用 scratch 库时自动重置；取自 `POSTGRES_DSN`/`DATABASE_URL` 必须显式 `ALLOW_SCHEMA_RESET=1`，否则拒绝执行） | 是（CI `gates` 提供 PostgreSQL service，以 `REQUIRE_SQL_RUNTIME_CHECK=1` 禁止静默跳过） |
| 前后端查询参数契约 | `python3 scripts/check-api-params.py` | 六条规则：① web 发出的查询参数全部被 transport 层读取；② OpenAPI 声明的查询参数全部被某 handler 读取；③ `internal/infra/postgres` 内无分页「地板」写法且无包内函数接收 page size；④ `sortField`/`sortBy` 字面量必须落在后端白名单并集内；⑤ `pageSize` 字面量不得大于 `MaxPageSize`（阈值由 Go 常量解析而非复制）；⑥ `filters` 内每个 key 必须是某 handler 读取的参数 | 是 |
| 数据访问调用点参数个数 | `python3 scripts/check-sql-args.py` | 每个 `db.Query`/`QueryRow`/`Exec` 的参数个数必须等于语句绑定的最大 `$N`（无 `$N` 的语句必须不传参）——补足 `PREPARE` 看不到调用点的门禁空洞 | 是 |
| 仓储 SELECT/Scan 列数 | `python3 scripts/check-scan-columns.py` | 链式 `Query/QueryRow(...).Scan(...)` 与 `rows, _ := Query(...); rows.Scan(...)` 两种写法（函数作用域匹配）的 SELECT 列数必须等于 Scan 目标数；防止运行时 Scan 参数数量错误（WebAuthn 凭据读取同类缺陷） | 是 |
| goroutine 安全 | `bash scripts/check-goroutine-safety.sh` | `internal/app` / `internal/platform` / `internal/infra` 中长期运行的后台 goroutine 必须由 `recover.SafeGo` 包裹；一次性 goroutine 在脚本豁免清单登记理由 | 是 |
| 错误体写入器 | `python3 scripts/check-errors.py` | ① 全部错误响应经 `route.WriteError` / `WriteOAuthError` / `WriteSCIMError`；② handler 包内无「有 `Error()` 但无状态」的类型；例外经登记（`payments/notify.go`，响应体由支付网关规定） | 是 |
| 手写错误响应 | `bash scripts/check-error-response.sh` | handler 不得直接 `w.WriteHeader(4xx/5xx)+w.Write` 绕过写入器；协议端点 XML/二进制等特例在脚本豁免清单内登记理由 | 是 |
| 枚举闭集跨语言一致性 | `python3 scripts/check-enums.py` | ① 每个 `enumset.NewSet` 良构且名称唯一；② 前端 `@enumset` 清单与服务端同名集合**双向逐值相等**；③ 每个服务端集合要么有带标注的前端清单，要么在脚本 `HEADLESS_SETS` 登记理由 | 是 |
| HTTP 方法与路径契约 | `python3 scripts/check-http-methods.py` | 控制台发出的每个请求必须命中一条已声明的 `(Method, Pattern)`，且请求助手族与路径族必须匹配（方法不匹配不是软失败：请求将得不到响应） | 是 |
| 前端控件可达性 | `python3 scripts/check-ui-wiring.py` | ① 持有 `filterValues` 状态必须交给请求（`filters: filterValues`）；② 传 `onSortChange` 必须存在 `sorter:` 列 | 是 |
| 首屏 antd 入口导入 | `python3 scripts/check-app-barrel-imports.py` | `web/src/app/**` 与 `main.tsx` 禁止从 `antd` barrel 导入（必须 `antd/es/<component>`）；barrel 会把仅懒加载页使用的 Form/Table/DatePicker/Upload 拖进入口静态图，首屏 antd 从 470 KB gzip 起步；懒加载页面不受限 | 是 |
| 前端架构（HTTP 入口） | `python3 scripts/check-frontend-architecture.py` | `web/src/` 禁止直调 `fetch`/`XMLHttpRequest`/`axios`，全部请求必须经 `shared/api/http.ts`（否则绕过 credentials/CSRF/Accept-Language/错误映射/401 登出） | 是 |
| SQL 可移植性（4 项） | `bash scripts/check-sql-portability.sh` | 子集外方言语法检查与 DDL 方言构造普查（输出 `ON CONFLICT`/`RETURNING`/`JSONB`/`TEXT[]`/`INET`/`uuidv7`/`PARTITION BY`/`ROW LEVEL SECURITY` 等清单） | 是 |
| Schema 结构门禁 | CI 结构门禁十项（见 19.2） | 空库全量建表、上一版本升级、回滚、FK/索引/约束/CHECK 可解析、`ON CONFLICT` 仲裁目标、引用列类型一致、软删除唯一性、租户表 RLS、生成物零差异、方言能力探测（ADR-022/023） | 是 |
| 编译 | `go build ./...` | 编译通过 | 是 |
| 静态检查 | `go vet ./...` | 0 告警 | 是 |
| 后端测试 | `go test ./...` | 全部通过（`backend` 作业为 `go test -cover -race -count=1 -timeout=120s ./internal/...`） | 是 |
| 分层检查 | `bash scripts/check-layers.sh` | 依赖方向红线（第 5 章）；规则排除 `*_test.go` | 是（`gates` 独立阻塞步骤；`verify-docs.sh` 内为告警） |
| OpenAPI 零差异 | `python3 scripts/gen_openapi.py --check`（由 `RUN_CODE_CHECKS=1` 内部执行） | 生成结果与提交的 `openapi/openapi.yaml` 一致（不与 `git HEAD` 比较）；生成器对每个 `routes.go` 断言「`Method` 字面量数 = 解析出的声明数」——缺 Summary、字段顺序不同、包别名不同等任何形态导致漏解析即硬失败，禁止静默丢端点；`path + method` 重复声明硬失败；`operationId` 全局唯一 | 是 |
| OpenAPI 契约自检 | `bash scripts/check-openapi-contract.sh` | OpenAPI `3.1.0`、`info.version`、YAML 重复键、路径占位符已声明且 `required: true`、`operationId` 存在且唯一、状态码仅数字或 `default`、必有 2xx、悬空 `$ref` 为 0 | 是 |
| 生成物零差异（全部） | `bash scripts/check-generated-zero-diff.sh` | OpenAPI / 四方言 DDL / 注册表渲染（数量以 `gen_registry_md.py --check` 输出为准）/ 前端注册表 / 领域常量（Go+TS）/ 错误码 TS / 前端契约 TS / `.env.example` / 追溯矩阵逐项与生成器一致；生成器或产物缺失即 FAIL（禁止静默跳过） | 是 |
| 路由与三面契约测试 | `go test ./internal/bootstrap/ -run 'TestRouteContractMatchesRuntimeRouter\|TestSurfaceContract\|TestSurfaceForKnownPaths'` | `routes.go` 声明与运行时注册双向对齐；三面分布与 `SurfaceFor` 分类一致（4.1.3） | 是 |
| 前端 lint | `cd web && pnpm run lint`（= `oxlint -c .oxlintrc.json src`） | `correctness` 类 0 error（阻塞）；`suspicious` 类为非阻塞告警基线（见 I.1.1） | 是 |
| 前端构建 | `cd web && pnpm run build` | 构建通过 | 是 |
| 前端测试 | `cd web && pnpm run test` | 全部通过（vitest run 模式；`src/test-setup.ts` 在 `afterAll` 提供宏任务泄空，抵消 `@rc-component/util` `useDelayState` 卸载不清理定时器导致的偶发非零退出） | 是 |
| i18n 键完整性 | `python3 scripts/check-i18n-keys.py` | 命名空间契约、键树结构 R1–R3、语言包 parity 为硬失败；键解析与内联兜底掩盖的缺失键（R4）、键重复归属（R5）走基线记账、只减不增；基线条目过期亦失败。规则见第 16 章，事实源见 docs/design/frontend/i18n.md | 是 |
| 三面分类注册表完整性 | `python3 scripts/check-surface-registry.py` | `contracts/registries/surfaces.yaml`：三面/协议家族/显式端点/公开豁免自洽；公开豁免不得命中管理面；`gates` 引用的脚本必须真实存在 | 是 |
| 短期状态矩阵登记 | `python3 scripts/check-short-lived-state.py` | `contracts/registries/short-lived-state.yaml` 每条须登记 存储/多区域策略/失败姿态/清理 四要素，取值落在闭集内 | 是 |
| 索引经济性（D9） | `python3 scripts/check-redundant-index.py` | 普通（非部分）索引列被某唯一约束/唯一索引的最左前缀完全覆盖即报冗余；防止写放大与存储浪费 | 是 |
| 冗余索引门禁 | 同上 | 目标态：0 处冗余；当前存量以 known-red 基线记账，逐表裁决后可删则删、确需保留者加说明性白名单 | 是 |

> **known-red 基线机制**：`contracts/registries/known-red.yaml` 登记已知红项的 `rule_id` 与基线失败数。`verify-docs.sh` 的 `run_gate` 在门禁失败时查询该表——已登记且基线 > 0 降级为 `WARN`（非阻断），未登记则硬 `FAIL`。**崩溃不受基线豁免**：门禁输出 `Traceback`/`panic`/命令不存在（exit 127）时一律硬 `FAIL`——崩溃说明校验根本没有执行，把它当成「已知债务」等于用一条 WARN 换取一条永不运行的门禁。基线只增不减；任何基线调整须在同一变更内写入 `CHANGELOG.md`。目标态缺口同时登记 `contracts/registries/open-items.yaml`。

#### I.1.1 前端 oxlint 基线

工具链版本以 `web/package.json` 为唯一事实源（当前 `oxlint ^1.94.0` + `oxfmt ^0.75.0`；复现：`grep -E '"oxlint"|"oxfmt"' web/package.json`）；`eslint` 保留为 `lint:eslint`（待上游支持 TypeScript 7 后恢复）。

复现命令：`cd web && pnpm lint`（JSON 口径：`npx oxlint -c .oxlintrc.json --format=json src`）。

| 类别 | 阈值 | 处置 |
|---|---:|---|
| `correctness` = error（阻塞） | **0** | 阻塞；禁止以 disable 规避，必须重构修复（派生值/渲染期状态调整/`useSyncExternalStore`、`useMemo`/`useCallback`、原生元素替换、文件级 override 仅限例外登记内的场景） |
| `suspicious` = warn（非阻塞告警基线） | 非阻塞 | 告警基线，作为后续收敛对象登记；不阻塞门禁 |

**例外登记（有理由、经评审，非静默关闭）**：

| 规则 | 范围 | 理由 |
|---|---|---|
| `jsx-a11y/prefer-tag-over-role` | `src/features/navigation/MenuSearch.tsx`（**文件级 override**） | 命令面板式搜索无法用原生 `<select>` 表达（富行、分组、命中高亮）；`role="listbox"`/`role="option"` 是 WCAG 4.1.2 与 APG Combobox 的推荐替代 |
| `jsx-a11y/prefer-tag-over-role`、`react/incompatible-library` | `src/shared/ui/VirtualTable/VirtualTable.tsx`（**文件级 override**） | 虚拟滚动表格由 `@tanstack/react-virtual` 自行渲染行标记，`role="row"` 为该集成的硬要求，无法使用浏览器原生表格布局 |
| `jsx-a11y/prefer-tag-over-role` | `src/shared/ui/{ActivityHeatmap,charts/ActivityHeatmap,ProviderIcon/ProviderIcon}.tsx`（**文件级 override**） | `<svg role="img">` 是 WCAG 1.1.1（非文本内容）的有意修法，不可移除 |

### I.2 门禁出口条件

I.1 登记已被 CI 调度的门禁；门禁的验收方式与出口条件由对应 `scripts/check-*` 的失败消息与 `contracts/registries/open-items.yaml` 的 `close_condition` 定义。出口条件以注册表为唯一事实源，本文不复制台账。纪律：判定方式与出口条件先写清，脚本落地前不得创建空壳占位文件。

## 附录 J. 错误、性能与国际化规范

- 错误统一使用稳定错误码、字段级详情、HTTP 状态和 `request_id`；不得返回 SQL、堆栈或 Secret。
- 外部依赖错误必须区分可重试/不可重试，设置超时、指数退避、最大重试和死信；认证/授权不确定时 fail-close。
- 性能结论按 L0 纯函数、L1 数据库、L2 HTTP、L3 多实例/故障分层；纯函数 Benchmark 不得代替 API SLO。
- 缓存键必须包含租户范围，必须有 TTL 和失效策略；禁止用缓存绕过授权。
- 用户可见文本必须使用 i18n 键，日期、时区、数字和货币必须经过本地化格式化，新增键至少同步 zh-CN/en。

---

**文档结束**

> 本规范自基线日期（2026-09-17）起生效。所有新增代码和 PR 必须符合本规范；存量代码在功能迭代时逐步合规，不在单独的重构 PR 中处理。
>
> 规范修订由架构评审委员会负责，任何团队成员可通过 RFC 流程提出修订建议。