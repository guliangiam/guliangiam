# 页面设计与交互规范

> 设计锚点：[DESIGN:frontend/interaction]

> 目标态规范「页面设计与交互规范」；写作约束见 [docs/README.md](../../README.md)。

---

## 页面布局模板

### 列表页模板

```tsx
// 标准列表页结构
<PageLayout>
  <PageHeader title={t("users:title")} extra={<Button type="primary">新建</Button>} />
  <CrudListPage config={listConfig} />
</PageLayout>
```

### 详情页模板

```tsx
// 标准详情页结构（嵌套路由）
<PageLayout>
  <PageHeader
    title={user.displayName}
    breadcrumb={buildBreadcrumbs(pathname)}
    extra={<Space><Button>编辑</Button><PopconfirmModal>删除</PopconfirmModal></Space>}
  />
  <Tabs
    items={[
      { key: "profile", label: "基本信息", children: <ProfileTab /> },
      { key: "permissions", label: "权限", children: <PermissionsTab /> },
      { key: "sessions", label: "会话", children: <SessionsTab /> },
    ]}
  />
</PageLayout>
```

### 表单页模板

创建/编辑/详情表单统一由共享壳 [ResourceFormPage](../../../web/src/shared/ui/ResourceFormPage.tsx) 承载，页面只提供标题、字段与提交函数：

```tsx
<ResourceFormPage
  title={isEdit ? "编辑用户" : "新建用户"}
  description="..."
  loading={detailQuery.isLoading}
  saving={updateMutation.isPending}
  form={form}
  onCancel={() => navigate(-1)}
  // 必须返回 Promise（通常是 mutateAsync），壳才能在请求结束前保持提交锁并回写字段错误。
  onFinish={async (values) => {
    await updateMutation.mutateAsync(values);
  }}
  enableDraft={isEdit}
  draftEntityId={id}
  extra={isEdit ? <DeleteButton /> : undefined}
>
  <FormSection title="基本信息">
    <Form.Item name="name" label="用户名" rules={[{ required: true }]}>
      <Input />
    </Form.Item>
  </FormSection>
</ResourceFormPage>
```

壳提供的强制交互契约（页面不得绕过）：

| 能力 | 行为 |
|---|---|
| 首错定位 | 客户端校验失败时不调用 API，滚动到第一个错误项；`onFinishFailed` 复用同一路径 |
| 提交锁 | `onFinish` 返回的 Promise 未结束前，主操作保持 loading 且拒绝重复提交；`mutate` 火并忘式保存视为缺陷 |
| 服务端字段错误 | 按 `HttpError.meta.field` 回写到对应 `Form.Item`，未映射错误只显示一条全局错误 |
| 失败保留 | 提交失败时保留全部输入与草稿，不清空表单 |
| 成功收尾 | 成功清草稿并失效相关 query；Save 留在本页，Save & Exit 成功后返回来源列表 |
| 未保存保护 | 草稿脏时离开路由触发既有守卫 |
| 宽度档 | 加载骨架与成品同宽，统一 `.gl-content__inner--form`（1200px），避免主区宽度跳变 |

## 子菜单与卡片规范

### 一层卡片

- 一个标签页 / 分区对应一张卡片：页级 `Tabs` 使用 `gl-tabs-card`（tab pane 即卡片）；详情 / 配置页分区使用 `.gl-section-anchor`（分区锚点即卡片）；组件级可用 `shared/ui/TabCard.tsx`。
- 卡片令牌统一：背景 `--gl-color-bg-elevated`、边框 1px `--gl-color-border`、圆角 `--gl-radius-lg`、阴影 `--gl-shadow-sm`、内边距 `--gl-space-5 --gl-space-6`。
- 禁止双层卡片：卡片作用域内的 `FormSection` 由 CSS 扁平化为纯分区（去背景 / 边框 / 阴影 / 内边距），作用域内不得再出现 `Card`。门禁：`python3 scripts/check-card-patterns.py`。

### 卡内分区

- 功能单元之间使用 1px `--gl-color-border` 细横线切分，上下间距 `--gl-space-5`（`.gl-form-section + .gl-form-section`、`.gl-app-ia__section + .gl-app-ia__section`）。
- 非 `FormSection` 的内容区块之间使用 `<TabCardDivider />`。
- 卡片与卡片之间间距 `--gl-space-6`，不由容器额外叠加 gap。

## 字段帮助提示规范

- 位置：字段名称、开关或独立功能点名称之后，间距 `--gl-space-1`，全系统同一图标与提示框。
- 组件：`shared/ui/FieldHelp.tsx` 的 `HelpTip` / `HelpLabel`；`GlFormRow` 支持 `help`（显式文本）与 `helpKey`（`help` 命名空间键）。
- 页级 `Tabs` 标签包一层 `HelpLabel`，键为 `tabs.<页面>.<标签>`（如 `tabs.iga.campaigns`）；标签帮助文案不得与标签文字相同，需说明该视图承载的功能范围。
- 全局兜底：`FieldHelpInjector` 在 AppShell 挂载一次，为全部表单 label 与带 `data-gl-help-key` 的元素注入图标，动态挂载（Tabs / Drawer）同样覆盖。
- 文案：`help` 命名空间，键为 `<路由首段 camelCase>.<字段名>`；与业务命名空间同名冲突时追加 `Help` 后缀（如 `account.emailHelp`）。
- 概念层：同名语义字段（`name`、`status`、`description`、`endpoint` 等）统一落到 `concept.<字段名>`，一条真实说明在全系统复用；解析顺序为 `<scope>.<field>` → `<scope>.<field>Help` → 概念层 → 按标签文本推导的概念键。
- 说明源文件：`web/src/shared/i18n/help/descriptions.json`（唯一人工维护源，含 `en` / `zh-CN` / `zh-TW`），运行 `python3 scripts/gen-help-locales.py` 合并进 14 个 `help.json`；未翻译语言暂用英文并在审计中计为未覆盖。
- 兜底：未登记字段使用 `help:fallback.field`，不允许出现无说明的裸图标；注入器会在 `span.gl-help-slot` 上写入 `data-gl-help-resolved-key` 与 `data-gl-help-fallback="1"`，供巡检脚本统计真实覆盖。
- 交互与可访问性：悬停与键盘 focus 均可触发，`button` + `aria-label`，`placement="topLeft"`，`mouseEnterDelay={0.15}`，提示框最大宽 320。
- 安全：提示文本只来自 i18n 资源，禁止 `dangerouslySetInnerHTML`；后端错误文案不得直接回显，统一经 `localizedError`。

## 多语言覆盖与审计规范

- 覆盖口径：UI 静态文案、枚举/状态标签、按钮/占位符/标题、前端提示与错误消息计入；用户或系统录入的业务数据（通知正文、审计详情、自定义字段值）不计入。
- 事实源：`web/src/shared/i18n/locales/<lang>/{common,auth,resource,help}.json`；四语言包键树必须完全一致，由 `python3 scripts/check-i18n-keys.py` 保证。
- 覆盖度测量：`python3 scripts/audit-i18n-coverage.py` 输出每语言/命名空间的 `sameAsEnglish`、`multiwordEnglish`、`referenced`、`referencedUntranslated`；`--json/--md` 供报告引用，`--list <lang>` 输出明细。
- 术语白名单：品牌与协议名（`API`、`JWT`、`OIDC`、`SAML`、`AWS S3`、`Azure AD`、`reCAPTCHA`、`Cloudflare Turnstile` 等）与格式片段（` · {{type}}`、`{"key": "value"}`）保留原文，不计为未覆盖。
- 错误文案：后端响应不直接展示；HTTP 状态/错误码经 `serverErrorMessage` 映射，前端本地 `Error` 保留其已本地化 message，未知值回退调用方 key。
- 内联兜底 `t("key", "English")` 只允许作为缺失兜底，新增项计入 R4 基线；补齐语言包后必须移除内联字面量。

## 页面级验收 rubric（10 项，全站逐页）

页面清单只取 [registry.md](registry.md) 的生成表（复现：`python3 scripts/gen_frontend_registry.py --check`），每批不超过 8 个页面文件，禁止手抄清单。每个页面按下表逐项留证；合格项写 PASS + 证据，缺陷项必须修复并附回归用例。

| # | 检查项 | 判定标准 | 证据方式 |
|---|---|---|---|
| R1 | 骨架与主操作 | create/edit/detail 使用 `ResourceFormPage` 或 `noForm` 壳；标题、说明、返回、单一主操作区；header 与 footer 不同时出现 Save | 静态检索 + 页面走查 |
| R2 | 视觉异常 | 无重叠、空容器、错位、异常留白、文本截断；浅/深配色下对比度达标 | 路由扫描截图 + 人工复核 |
| R3 | 宽度与响应式 | 1600/1200/960 档位正确；390/768/1024/1440/1920 无主区横向溢出 | `route-sweep` 五档断言 |
| R4 | 表单设计 | 字段分组使用 `FormSection`；两栏只在区块内部生效；必填标识、帮助文案、控件类型合理 | 静态检索 + 页面走查 |
| R5 | 双向绑定与重置 | 受控值正确；记录切换无残留；草稿不覆盖新值；`Form.useWatch` 不传字段数组 | `check-form-detail-patterns.py` |
| R6 | 校验与错误面 | Zod ↔ Ant rules ↔ 后端枚举一致；首错定位；服务端字段错误回写；全局错误唯一 | 共享壳契约 + 单测 |
| R7 | 异步与状态 | 提交锁由 Promise 驱动；失败保留输入；成功后失效 query 并回显；loading/empty/error/403 齐备 | 逐页 `onFinish` 检索 + E2E |
| R8 | 权限与入口 | `*:write` 守卫；只读角色无写入口；越权 API 返回统一 403 | `PermissionGuard` 检索 |
| R9 | a11y 与 i18n | label 与控件关联、键盘可达、无硬编码文案、`check-i18n-keys.py` 通过 | a11y 套件 + i18n 门禁 |
| R10 | 契约 | 使用 `paths.ts`；请求/响应字段与 OpenAPI/DTO 一致；闭集字段无自由输入 | `check-api-params.py`、`check-ui-wiring.py` |

**提交语义红线**：用户可见保存一律走 `onFinish`（内部 `mutateAsync`），返回 Promise 让共享壳保持提交锁；`onSubmit` 仅是兼容入口，禁止在 `onSubmit` 内写 `mutation.mutate(...)` 后立即返回。删除确认、导出、轮询等非表单保存动作可使用 `mutate`，但必须有 pending 态与错误提示。

## 响应式设计规范

### 断点定义

| 断点 | 值 |
|------|-----|
| xs | 480px |
| sm | 768px |
| md | 1024px |
| lg | 1280px |
| xl | 1536px |
| 2xl | 1920px |

### 内容宽度三档（唯一事实源）

宽度由 [web/src/styles.css](../../../web/src/styles.css) 的三个语义类统一提供，页面不得自行发明第四种宽度：

| 档位 | 选择器 | 上限 | 适用页面 |
|---|---|---|---|
| 页面框架 | `.gl-content__inner` | 1600px，`margin: 0 auto`；≤1024px 用可用全宽并保留安全间距 | 列表工作台、仪表盘、多 Tab 工作台 |
| 表单/详情 | `.gl-content__inner--form` | 1200px | 创建/编辑/详情表单（含加载骨架，与成品同宽） |
| 阅读型设置 | `.gl-content__inner--reading` | 960px | 账户、安全策略等阅读/设置面板 |

三档都在页面框架之内，即「内容阅读宽度」小于「页面框架宽度」；认证面板、支付结果、扫码设备等独立卡片可使用 `max-width + calc(100vw - Npx)` 的局部上限，但必须随视口收缩、不得产生横向滚动。

### 布局适配策略

| 断点 | 布局模式 | 侧边栏 | 顶栏 | 内容区 |
|------|---------|--------|------|--------|
| ≤768px (sm) | 抽屉模式 | 隐藏，汉堡菜单触发 | 紧凑 | 单列，全宽 |
| 769-1024px (md) | 抽屉模式 | 隐藏，汉堡菜单触发 | 标准 | 单列，带边距，框架宽度不再封顶 |
| 1025-1280px (lg) | 侧边栏模式 | 固定 240px | 标准 | 自适应，按上表档位封顶 |
| ≥1281px (xl) | 侧边栏模式 | 固定 240px，可折叠 | 标准 | 自适应，按上表档位封顶 |

表格列宽显式声明并允许容器内横向滚动；固定操作列不得被裁切。响应式断言由 `web/e2e/responsive.spec.ts` 在 390/768/1024/1440/1920 五档视口执行（`npx playwright test --project=viewport-*`）。

### Container Queries（组件级响应式）

```css
.card-container { container-type: inline-size; }
@container (max-width: 400px) { .card-content { flex-direction: column; } }
```

## 前端专项：字段显示规范（强制写入）

### 原则

页面关联字段以**用户可读名称**为准（组织 ID 显示组织名称，用户 ID 显示用户名）；内部关联与索引仍用 ID。

**三个约定层级**（优先级从高到低）：
1. **后端主路径**：后端列表/详情响应同时返回 `id` + `displayName`（或 `*Name` 语义字段），前端直接消费，不单独发 API
2. **后端配合缺失时**：使用 React Hook 做补充查询（仅用于审计详情、操作日志等已有 ID 但后端未联带名称的场景）
3. **前端本地缓存**：对已有全局上下文的实体（当前组织、当前用户）从全局状态取，不走 Hook

### 后端 DTO 约定

后端所有列表/详情响应 DTO **必须同时返回**关联实体的 `id` 与 `displayName`：

```typescript
// 后端响应 DTO（统一约定）
interface UserListItem {
  id: string;
  username: string;
  displayName: string;
  email: string;
  organizationId: string;
  organizationName: string;        // ← 前端直接显示组织名称，不发第二次请求
  // ...
}

interface AuditRecordDTO {
  id: string;
  actorId: string;
  actorName: string;               // ← 审计记录同时返回主体名称
  resourceId: string;
  resourceType: string;
  resourceDisplayName: string;     // ← 审计记录同时返回资源显示名（字段名固定）
  operation: string;
  // ...
}
```

**审计记录特殊约定**：审计事件（`records` 表）在写入时同时存入 `resource_id` 与 `resource_display_name` 两列。`resource_display_name` 由业务层在审计写入时一并传递，避免事后 join 查询或名称变更后历史记录"漂移"。后端查询审计时直接返回 `resource_display_name`，前端无需做 ID→名称转换。

### 前端 React Hook 完整签名

当后端响应中未联带名称时（例如审计详情页只有主体 ID），以下 Hook 作为补充路径。**主路径始终是后端联带返回名称**——Hook 只在已有 ID 但缺名称时使用。

```typescript
// ===== 四个标准 Hook 的统一签名 =====

// useOrgName — 组织名称
// 场景：审计详情页 actor 是组织；操作日志关联组织
export function useOrgName(
  orgId: string | undefined
): string | undefined {
  const { data } = useQuery({
    queryKey: queryKeys.organizations.detail(orgId!),
    queryFn: () => getOrganization(orgId!),
    enabled: !!orgId,
    staleTime: 5 * 60 * 1000,     // 5 min 内不复取（组织名极少变）
    select: (org) => org?.displayName ?? org?.name,
  });
  return data;
}

// useUserName — 用户名称
// 场景：审计详情页 actor 是用户但后端未联带 displayName；全局用户选择器补充
export function useUserName(
  userId: string | undefined,
  organizationId?: string
): string | undefined {
  const { data } = useQuery({
    queryKey: queryKeys.users.detail(orgId, userId!),
    queryFn: () => getUser(userId!, { orgId }),
    enabled: !!userId,
    staleTime: 2 * 60 * 1000,     // 2 min（用户名偶变）
    select: (u) => u?.displayName ?? u?.username,
  });
  return data;
}

// useAppName — 应用名称
// 场景：OAuth 授权页显示应用名；审计记录 resource 是 application
export function useAppName(
  appId: string | undefined,
  organizationId: string
): string | undefined {
  const { data } = useQuery({
    queryKey: queryKeys.apps.detail(organizationId, appId!),
    queryFn: () => getApplication(organizationId, appId!),
    enabled: !!appId && !!organizationId,
    staleTime: 5 * 60 * 1000,
    select: (a) => a?.displayName ?? a?.name,
  });
  return data;
}

// useRoleName — 角色名称
// 场景：权限详情页 roleId → roleName；审计记录角色变更
export function useRoleName(
  roleId: string | undefined,
  organizationId: string
): string | undefined {
  const { data } = useQuery({
    queryKey: queryKeys.roles.detail(organizationId, roleId!),
    queryFn: () => getRole(organizationId, roleId!),
    enabled: !!roleId && !!organizationId,
    staleTime: 5 * 60 * 1000,
    select: (r) => r?.displayName ?? r?.name,
  });
  return data;
}
```

**Hook 设计红线**：
- 所有 Hook 内部自动处理 `undefined | '' | null` 输入（`enabled: !!id`）
- 所有 Hook 返回类型为 `string | undefined`（调用方无需判空，直接用作 `<span>{useOrgName(id) ?? '-'}</span>`）
- `organizationId` 作为独立参数（非隐藏从全局状态取），便于测试与跨组织查询场景

## 前端专项：交互简化（强制写入）

### 原则

操作表单中**凡可通过关联选择实现的字段，一律不得使用自由输入框**。关联字段（orgId / userId / appId / roleId / permission 列表）本质是"从已存在资源中选一个/多个"，让用户输入 UUID 既不可行也易出错。

### 禁止自由输入的场景清单

| 禁止场景 | 正确组件 | 反模式（禁止） |
|----------|----------|----------------|
| 选择所属组织 | `OrganizationSelect` | `<Input placeholder="请输入组织ID">` |
| 选择目标用户 | `UserSelect` | `<Input placeholder="请输入用户ID">` |
| 选择目标应用 | `ApplicationSelect` | `<Input placeholder="请输入应用ID">` |
| 分配角色给用户 | `RolePaginateSelect`（单选或多选） | `<Input placeholder="请输入角色ID，多个用逗号分隔">` |
| 选择分组/树形资源 | `GroupTreeSelect` | `<TextArea placeholder="请输入ID列表，每行一个">` |
| 选择父组织 | `OrganizationSelect`（排除自己 + 自己的子孙） | `<Input placeholder="请输入父组织ID">` |
| 选择策略应用主体 | `UserSelect + RolePaginateSelect + OrganizationSelect` 联动 | `<TextArea placeholder="subject: 每行一个">` |
| 认证源（IdP）选择 | 下拉列表（从 `/api/v1/providers` 加载） | `<Input placeholder="请输入 IdP issuer">` |
| 审计查询的"操作主体"筛选 | `UserSelect`（可选） | `<Input placeholder="请输入操作主体ID">` |
| 审计查询的"资源类型"筛选 | 固定枚举下拉（`user`/`organization`/`application`/`role`/...） | `<Input placeholder="请输入资源类型">` |

### 共享选择器组件族

关联选择一律复用 `web/src/shared/ui/` 的共享选择器组件族（目录为唯一事实源，Props 以各组件源码为准，本文不复制签名）：

| 组件 | 用途 |
|------|------|
| `OrganizationSelect.tsx` | 组织选择（支持排除自己/子孙） |
| `UserSelect.tsx` / `UserPaginateSelect.tsx` | 用户选择（按组织过滤；分页搜索形态） |
| `ApplicationSelect.tsx` | 应用选择 |
| `RolePaginateSelect.tsx` | 角色选择（分页搜索） |
| `GroupTreeSelect.tsx` / `GroupPaginateSelect.tsx` | 分组树/分页选择 |
| `AsyncSearchSelect.tsx` / `PaginateSelect.tsx` | 通用异步搜索/分页选择基座 |
| 其余域选择器（Provider/Plan/Pricing/Product/Order/Region/CountryCode 等） | 同目录枚举 |

新增关联字段禁止自由输入，也禁止在页面内另造内联选择器；无现成组件时先扩展共享组件族。

### 组件行为红线

| 红线 | 说明 |
|------|------|
| 所有 Select 组件**强制**从后端加载选项（不允许前端硬编码选项） | 选项来源必须是 `GET /api/v1/*` 列表接口 |
| 所有带 `organizationId` 参数的组件**严格按组织过滤** | 不得展示跨组织资源（RLS 后端二次兜底，但前端也要正确传递 orgId） |
| 搜索框是 Select 组件的**内置属性** | 不允许在组件外再放一个 `<Input>` 当搜索框 |
| 多选组件返回 `string[]` | 不返回逗号拼接字符串，避免后端二次解析 |
| `excludeIds` 必须生效于**选项列表**而非仅隐藏已选项 | 已排除的 ID 在列表中不出现；但当前 `value` 若命中 excludeIds 仍显示（编辑场景保留已有值） |

## CrudListPage 列表页抽象

> 列表页抽象的唯一实现见 `web/src/shared/ui/CrudListPage.tsx`；本节只给架构要点。

### 三层架构

```
CrudListPage (组件级封装 / 入口 A)
├─ config 对象 → useCrudList → CrudListView
├─ 适合：标准 CRUD 列表页（拿不到 list 结果）
└─ 例：OrganizationsPage
CrudListView (纯渲染层 / 入口 B)
├─ (UI config, CrudListResult) → ResourceListCard
├─ 适合：页面壳自己要持有 useCrudList 结果的场景
└─ 例：UsersPage（需要 bulkDeleteMutation）
useCrudList (hook)
└─ 封装 state + query + mutations + handlers
```

### 三驾马车（必填）

| 字段 | 类型 | 说明 |
|------|------|------|
| `buildQueryKey` | `(p: TParams) => readonly unknown[]` | TanStack Query Key 构造器，必须覆盖所有变化维度 |
| `buildParams` | `({page, pageSize, search, filters, sortField, sortOrder}) => TParams` | 把 hook 内部 state 转换成后端 API 参数 |
| `paginatedFn` | `(params: TParams) => Promise<{items: TItem[], total: number}>` | 分页查询 API |

### 两种使用模式

| 模式 | 适用场景 | 示例 |
|------|---------|------|
| **CrudListPage config 直套** | 标准 CRUD，页面壳不需要 list 的 setter/mutation | OrganizationsPage / RolesPage |
| **useCrudList + CrudListView** | 需要 `rowSelectable` 保护谓词 / resetAll / 独立 mutations | UsersPage（保护用户 + 4 个独立 mutations） |

### 高级场景

- **闭包引用外层 state**（如 orgFilter）：`buildParams` 是普通闭包，可引用组件作用域任何 state
- **Reset All 按钮**：先走 `useCrudList` 拿到 crud 对象，暴露的 4 个 setter（setSearch / setFilters / setSortField / setSortOrder）组合实现
- **sortOrder 映射**：Ant Design `"ascend"/"descend"` → 后端 `"asc"/"desc"` 在 `buildParams` 里统一映射
- **独立 mutations**：CrudListPage **不接管** export / impersonate / optimistic update——留在页面壳

### 页面模式清单

| 页面 | 模式 | 特殊点 |
|------|------|--------|
| OrganizationsPage | CrudListPage config 直套 | 最简模板 |
| RolesPage | CrudListPage config | PermissionGuard + importModal 壳 |
| GroupsPage | CrudListPage config | JSON + Excel import |
| ApplicationsPage | CrudListPage config | 异步 delete summary Popconfirm + Duplicate/Export/Import 独立 mutations（行内 Duplicate/Export + 工具栏 Import Upload） |
| `UsersPage` | useCrudList + CrudListView | **`rowSelectable` 保护自己与 built-in admin + 4 个独立 mutations + resetAll + orgFilter 闭包** |

### 选型决策

```
新页面从这里开始:
├─ 有标准 CRUD query + delete + filter + sort？
│   ├─ YES → CrudListPage config 直套    (参考 OrganizationsPage)
│   └─ NO  → 页面壳自己要 crud 的 setter / mutation / query？
│            └─ useCrudList + CrudListView (参考 UsersPage)
└─ 两者都不满足（Kanban / 日历 / 复杂表单）→ 不用 CrudListPage
```

### 消除的 boilerplate

reducer + 6 个 useState + 1 个 useMemo + 4 个 handler + 1 个 deleteMutation + 1 个 bulkDeleteMutation ≈ **150-200 行**。

### 批量选择与批量操作

列表页的批量能力只由共享层提供，页面配置开关即可，禁止在页面里另写一套批量删除。

**适用范围**

| 列表类型 | 是否启用 | 说明 |
|----------|----------|------|
| 标准 CRUD 列表（有 per-row `deleteFn`） | 启用 | 复选框列 + 默认「批量删除」 |
| 行删除实际是"吊销/下线/标记已读"等非删除语义 | 按语义启用或不启用 | 启用时确认文案必须说明真实后果 |
| 只读列表（审计记录、登录日志、风险信号、工作台只读 Tab、用量/健康） | 不启用 | 没有删除 API，不提供无意义的复选框 |
| 依赖前置校验的删除（如删除摘要预检、内置对象保护） | 启用 | 用 `rowSelectable` 把不可删行置灰，而不是在批量动作里过滤 |
| 详情页内嵌表格、以单选/向导为目的的选择列（对账批次选择、LDAP 同步预览） | 不属于列表页批量能力 | 这些表格的选中语义是"选择对象"而非"批量操作对象"，不套用批量删除 |

**配置契约**

| 字段 | 语义 |
|------|------|
| `enableBulkDelete` | 渲染复选框列与默认「批量删除」。复选框列**始终渲染**，不依赖"已有选中项"才出现；表头支持全选/半选。 |
| `rowSelectable` | 返回 `false` 的行复选框置灰（当前登录人、built-in 组织/管理员、受保护资源）。全选不会选中置灰行。 |
| `bulkDeleteDescription` | 批量删除确认弹窗的补充说明，用于表达级联/吊销等额外后果。 |
| `bulkDeleteLabel` / `bulkDeleteConfirmTitle` | 行删除实际是"标记已读/吊销"等非删除语义时，改写默认动作的按钮文案与确认标题，禁止让按钮说谎。 |
| `bulkActions` | **只放额外的批量管理动作**（批量启用/停用/导出等），与默认删除并排；不得再放删除按钮。回调同时提供选中 keys 与当前页 `records`，便于按行携带 revision 等并发控制字段。 |

**行为红线**

1. **选择作用域**：选择只对当前视图有效——换页、搜索、筛选、排序后清空选择，避免批量动作作用到用户看不见的行。
2. **部分失败**：批量删除逐条调用资源删除接口，单条失败不得中断其余行；结果必须按「成功 N / 失败 M」如实提示，失败行保留选中以便重试，禁止整体回滚或谎报全部成功。
3. **并发上限**：批量删除按固定批次并发（共享层常量），避免一次选中把服务端打满；不在前端引入批量删除端点，服务端仍是权限与审计的唯一权威。
4. **权限**：批量动作同样受资源写权限点约束；无权限时按钮不渲染。默认删除不绕过单行删除的鉴权与审计链路。
5. **确认**：破坏性批量动作必须二次确认，确认弹窗展示选中数量；非破坏性管理动作可以直接执行。
6. **文案**：复用 `common` 命名空间的 `batchDelete` / `confirmBulkDelete` / `bulkDeletePartial` / `bulkDeleteSuccess`，页面不得复制一套同义文案。

**共享层职责划分**

- `useCrudList`：维护选中集合、按批次执行删除、汇总 `succeeded/failed/failedKeys`、失败行保留选中、视图变化时清空选择。
- `CrudListView`：始终渲染选择列、组装默认删除动作与页面扩展动作、提供 `rowSelectable` 置灰。
- `ResourceListCard`：只负责批量操作栏布局（已选计数 + 动作 + 取消选择）。
- 页面：声明开关、保护谓词与额外管理动作，不实现删除语义。

**测试口径**

共享层单测必须覆盖：零选中时复选框列已存在、`rowSelectable` 行置灰、批量删除按批次执行、部分失败保留失败行、换页/搜索清空选择。页面不再为批量删除重复写测试。


---

### 路由与导航（原 navigation.md，全文保留）

> 目标态规范「路由设计规范」；写作约束见 [docs/README.md](../../README.md)。

---

### 路由结构（按域模块化）

```typescript
// src/app/router/index.ts
import { authRoutes } from "./auth";
import { userRoutes } from "./users";
import { orgRoutes } from "./organizations";
import { appRoutes } from "./applications";
// ... 其他域路由

export const router = createBrowserRouter([
  ...authRoutes,  // 公开路由
  {
    path: "/",
    element: <AppShell />,
    errorElement: <RouterErrorElement />,
    children: [
      { index: true, loader: dashboardLoader, element: <DashboardPage /> },
      ...userRoutes,
      ...orgRoutes,
      ...appRoutes,
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
```

### 域路由模块模板

```typescript
// src/app/router/users.ts
import { lazy } from "react";

const UsersPage = lazy(() => import("@/domains/users/views/UsersListPage"));
const UserDetailPage = lazy(() => import("@/domains/users/views/UserDetailPage"));
const UserCreatePage = lazy(() => import("@/domains/users/views/UserCreatePage"));

/** 路由级数据预取 */
async function userDetailLoader({ params }: { params: { id: string } }) {
  const queryClient = getQueryClient();
  return queryClient.ensureQueryData({
    queryKey: queryKeys.users.detail(params.id),
    queryFn: () => getUser(params.id),
  });
}

export const userRoutes: RouteObject[] = [
  {
    path: "users",
    element: <RequirePermission permission="users:read"><UsersPage /></RequirePermission>,
  },
  {
    path: "users/new",
    element: <RequirePermission permission="users:create"><UserCreatePage /></RequirePermission>,
  },
  {
    path: "users/:id",
    loader: userDetailLoader,
    element: <RequirePermission permission="users:read"><UserDetailPage /></RequirePermission>,
    errorElement: <RouteErrorElement />,
  },
];
```

### 路由守卫

```typescript
// src/app/router/RequirePermission.tsx
export function RequirePermission({ permission, children }) {
  const { permissions, isLoading } = usePermissions();
  if (isLoading) return <PageSkeleton />;
  if (!permissions.actions.includes(permission)) return <ForbiddenPage />;
  return <>{children}</>;
}
```

后端 PEP 判定与前端路由守卫**独立双保险**——前端守卫是 UX 优化（不让无权限用户看到页面），后端 Enforcer 是安全边界。

### 懒加载策略

- 所有页面组件使用 `lazy(() => import(...))` 动态导入
- 每个域路由模块独立，按域自动代码分割
- vendor chunk 分层：Tier 0（React core）、Tier 1（antd/i18n/data）、Tier 2（recharts/monaco 等）
- 路由切换用 `<Suspense fallback={<PageSkeleton />}>` 包裹

### 面包屑自动生成

从路由路径 + 菜单定义自动生成：

```typescript
function buildBreadcrumbs(pathname: string, menuGroups: MenuGroup[]): BreadcrumbItem[] {
  const crumbs: BreadcrumbItem[] = [];
  for (const group of menuGroups) {
    for (const item of group.items) {
      if (pathname.startsWith(item.key)) {
        crumbs.push({ label: group.label, path: group.key });
        crumbs.push({ label: item.label, path: item.key });
      }
    }
  }
  return crumbs;
}
```

### 公开路由

| 路由 | 说明 |
|------|------|
| `/login` | 登录页 |
| `/saml/sso/{appSlug}` | SAML SP 入口（外部 IdP 发起） |
| `/saml/acs` | SAML SP 断言消费 |
| `/auth/qrcode` | 扫码登录页 |
| `/captcha/test` | CAPTCHA 测试（开发） |

### 与后端路由的关系

前端路由是浏览器级导航；后端路由（`internal/transport/http/**/routes.go`）是 API 端点。二者独立但命名有对应关系——前端 `/users` 对应后端 `/api/v1/users`。


---

### 数据层（原 data-layer.md，全文保留）

> 目标态规范「状态管理与数据层」；写作约束见 [docs/README.md](../../README.md)。

---

### 状态分层

| 状态类型 | 管理方案 | 示例 |
|---------|---------|------|
| 服务器数据 | TanStack Query | 用户列表、组织详情、权限定义 |
| 全局客户端状态 | Zustand store | 当前会话、主题配置、布局偏好 |
| URL 状态 | React Router params | 当前路由、资源 ID、过滤参数 |
| 局部 UI 状态 | useState / useReducer | 弹窗开关、选中行、表单步骤 |
| 表单状态 | react-hook-form | 表单值、校验错误、脏检查 |
| 认证流程状态 | React 组件状态 + sessionStorage | Passkey ceremony 中间态、OAuth PKCE verifier |

**禁止混用**：服务器数据（远端、会变、多人共享）和客户端状态（本地、个人偏好）的管理方案**必须分离**。不要把用户列表存 Zustand 里当客户端状态管理，也不要把 theme 存在 TanStack Query 里。

### TanStack Query 使用规范

#### Query Key 统一构造器

所有 query key 通过 `queryKeys` 对象构造（`src/shared/api/query-keys.ts`），禁止页面各处手写字符串：

```typescript
export const queryKeys = {
  users: {
    all: ["users"] as const,
    paginated: () => [...queryKeys.users.all, "paginated"] as const,
    paginatedWithParams: (p: UserListParams) => [...queryKeys.users.paginated(), p] as const,
    detail: (id: string) => [...queryKeys.users.all, id] as const,
  },
  organizations: { /* ... */ },
  // ... 其他域
};
```

#### 标准查询模式

```typescript
function useUserDetail(userId: string) {
  return useQuery({
    queryKey: queryKeys.users.detail(userId),
    queryFn: () => getUser(userId),
    staleTime: 5 * 60_000, // 5 分钟
  });
}
```

#### 乐观更新模式

```typescript
function useOptimisticUserStatus() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateUserStatus,
    onMutate: async ({ userId, enabled }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.users.all });
      const previous = queryClient.getQueryData(queryKeys.users.paginated());
      queryClient.setQueryData<UserListItem[]>(queryKeys.users.paginated(), (old) =>
        old?.map(u => u.id === userId ? { ...u, isEnabled: enabled } : u)
      );
      return { previous };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(queryKeys.users.paginated(), ctx.previous);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
    },
  });
}
```

#### 失效策略

| 场景 | 策略 |
|------|------|
| 创建/更新/删除 | `invalidateQueries` 失效相关 query key |
| 乐观更新 | `onMutate` 手动 setQueryData → `onSettled` invalidate |
| 独立 mutations | 页面壳自己管理（CrudListPage 不接管 export/impersonate 等） |

### Zustand Store 定义模板

```typescript
// src/features/session/session-store.ts
import { create } from "zustand";
import { persist } from "zustand/middleware";

type SessionState = {
  account: SessionAccount | null;
  organization: SessionOrganization | null;
  impersonatedUser: { id: string; username: string; displayName: string } | null;

  setSession: (payload: { account: SessionAccount | null; organization: SessionOrganization | null }) => void;
  setImpersonatedUser: (user: SessionState["impersonatedUser"]) => void;
  reset: () => void;
};

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      account: null,
      organization: null,
      impersonatedUser: null,
      setSession: ({ account, organization }) => set({ account, organization }),
      setImpersonatedUser: (user) => set({ impersonatedUser: user }),
      reset: () => set({ account: null, organization: null, impersonatedUser: null }),
    }),
    {
      name: "gl-session",
      partialize: (state) => ({ /* 仅持久化必要字段 */ }),
    },
  ),
);
```

### HTTP 客户端

```typescript
// src/shared/api/http.ts
/** 响应信封 schema */
const responseEnvelopeSchema = z.object({
  status: z.string(),
  msg: z.string().optional(),
  data: z.unknown().optional(),
  data2: z.unknown().optional(),
});

/** HTTP 错误类 */
export class HttpError extends Error {
  status: number;
  code: string;
  body: unknown;
  meta: Record<string, unknown> | undefined;
  retryAfterSeconds: number | undefined;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Request failed with status ${status}`);
    this.status = status;
    this.code = errorCodeFromStatus(status);
    this.body = body;
    this.name = "HttpError";
  }
}
```

错误分类：按 HTTP status → `error_code` 映射 → 展示给用户或静默处理。

### 与 security.md 的关系

会话 Cookie、CSRF 等安全相关规则见《design/frontend/security.md》。

### 生成型类型消费

前端不手写状态字面量枚举、API 三面联合类型、错误码联合类型——一律从生成器产出的 `web/src/generated/` 目录 import：

| 生成物 | 生成器 | 消费方式 |
|--------|--------|---------|
| `error-codes.ts` | `scripts/gen_error_codes.py` | `import type { AppErrorCode } from '@/generated/error-codes'` |
| `state-machines.ts` | `scripts/gen_domain_constants.py` | `import type { OrdersState, AuthFlowState } from '@/generated/state-machines'` |
| `api-contracts.ts` | `scripts/gen_api_contracts.py` | `import type { ApiSurface } from '@/generated/api-contracts'` |

三个文件均为零差异门禁生成物（`check-generated-zero-diff.sh`），禁止手改。新增状态机或变更 API 三面分类后，必须重新运行生成器并提交。

## Header 品牌区设计规范

### 组织品牌组件（`.gl-header__brand`）

Header 品牌区由 Avatar（logo）+ 组织名称组成，**始终可点击**（Wrap 在 antd `Dropdown` 内）。

#### 数据来源

| 元素 | 数据 | 后端字段 | 前端 store |
|------|------|----------|-----------|
| Avatar src | organization.logoUrl | `organizations.logo_url` | `session.organization.logoUrl` |
| Avatar 字母 fallback | organization.displayName.name 首字母 | — | 同左 |
| 组织名称文本 | organization.displayName \|\| organization.name | — | 同左 |
| Favicon | organization.faviconUrl | `organizations.favicon_url` | useEffect 动态替换 `<link rel=icon>` |
| 浏览器标题 | organization.displayName | — | useEffect 设置 `document.title` |

后端 `SessionOrganization` DTO 有 `LogoUrl` / `FaviconUrl` 字段；前端 `sessionOrganizationSchema`（Zod）必须声明这两个 `.optional().nullable()` 字段，否则 Zod parse 默认行为会**静默丢弃未声明字段**。

#### 组织切换选择器

组织切换的唯一 UI 是 Header 的组织 Select（事实源：`web/src/app/layout/AppShell.tsx` 的 `switchOrgMutation` 与 `orgSelectOptions`）：

- 选项来自 `GET /api/v1/organizations` 列表（TanStack Query 缓存）；每项显示组织 logo Avatar + displayName，当前组织带标识。
- 切换为前端会话上下文替换：拉取目标组织详情 → 替换 session store 的 organization（只保留菜单/品牌所需字段）→ 失效 session/组织/用户等依赖组织上下文的查询缓存 → 页面按新组织重拉。不存在独立的组织上下文切换后端端点；服务端租户裁决仍以可信会话上下文为准（见 `docs/design/iam/permissions.md` §6）。
- 非 super-admin 用户的 `/api/v1/organizations` 响应被 RLS 过滤只剩自己所属组织 → 选择器仍正常，但只有 1 项。

### 断点适配

| 断点 | Avatar | 组织名称 | Caret |
|------|--------|---------|-------|
| ≥768px | 40px | 完整显示 max-width 160px | 显示 |
| 640px | 32px | 截断 | 显示 |
| ≤480px | 28px | **隐藏** (display:none) | 隐藏 |

## Dashboard 设计规范

### 卡片网格（StatCard）

- 3 行 × 4 列 CSS Grid (`grid-template-columns: repeat(4, minmax(0, 1fr))`)
- 单卡 `min-height: 80px`，紧凑 padding
- 每张卡可点击（`gl-metric-card--clickable`）→ hover 上浮 2px + 主色边框高亮
- 结构：`[图标徽章 28px] + [标签]` 在上，`[数值]` 在下
- 每个数值接入独立后端端点（见下表），**禁止** mock / 静态数据

#### 统计卡片数据来源

| 卡片 | 后端端点 | 字段 | 前端路由 |
|------|---------|------|---------|
| 组织总量 | `GET /api/v1/dashboard/stats` | totalOrganizations | /organizations |
| 用户总量 | 同上 | totalUsers | /users |
| 应用总量 | 同上 | totalApplications | /applications |
| 身份提供商 | `GET /api/v1/dashboard/resources` | totalProviders | /providers |
| 总角色数 | `GET /api/v1/dashboard/stats` | totalRoles | /roles |
| 总组数 | 同上 | totalGroups | /groups |
| 总权限数 | 同上 | totalPermissions | /permissions |
| 活跃会话 | `GET /api/v1/dashboard/auth` | activeSessions | /sessions |
| 今日新增用户 | `GET /api/v1/dashboard/users` | newUsersToday | /users |
| 过去 7 天新增 | 同上 | newUsers7Days | /users |
| 过去 30 天新增 | 同上 | newUsers30Days | /users |
| 令牌总数 | `GET /api/v1/dashboard/auth` | totalTokens | /tokens |

### 图表区

| 模块 | 组件 | 端点 |
|------|------|------|
| GitHub 风格活动热力图 | `<ActivityHeatmap>` | `GET /api/v1/dashboard/activity-heatmap` |
| 24h 登录分布（Bar） | `<BarChart>` | `GET /api/v1/dashboard/login-heatmap` |
| IdP 提供商分布（Pie） | `<PieChart>` | `GET /api/v1/dashboard/provider-distribution` |
| 30 天趋势（Line） | `<LineChart>` | `GET /api/v1/dashboard/trends` |
| 组织用户分布（Pie） | `<PieChart>` | `GET /api/v1/dashboard/org-user-distribution` |
| Runtime Metrics（Go runtime） | antd `<Statistic>` | `GET /api/v1/system/metrics` |
| MFA 覆盖率 | antd `<Progress>` + 文本 | `GET /api/v1/dashboard/mfa-coverage` |

### 新增卡片 checklist

1. 后端 dashboard service 新增查询方法 + SQL 端点
2. 前端 `web/src/shared/api/iam/dashboard.ts` 加 type + fetch 函数
3. `DashboardPage.tsx` 里加 `useQuery` + `<StatCard>`（必须带 icon + onClick navigate）
4. 后端 SQL 必须支持 `organization_id` 参数（super-admin bypass RLS）
5. **禁止** inline 硬编码中文标题 — 必须加 i18n 键到全部 locale 文件（目录枚举）
6. curl 实证后端端点返回数据

### Layout 约束

- Dashboard 不设 hero 大标题区与"系统状态"装饰卡片，首屏即数据卡片网格。
- 退出登录入口唯一：用户头像 Dropdown 内，不在页面正文放置独立退出按钮。

## 独立认证页的页面家族

控制台页面统一走 `.gl-content__inner`。登录、注册、邀请注册、忘记密码、SAML 回调、
设备码、Telegram 登录等**不装配 AppShell 的独立页面**使用下列文档化壳层，新增页面
必须从中选择一个，不得使用无类名的内联容器：

| 壳层选择器 | 适用页面 | 宽度策略 |
|---|---|---|
| `.gl-content__inner` | 控制台列表 / 仪表盘 / 多 Tab 工作台 | 1600 / 1200 / 960 三档（见上表） |
| `.gl-auth-background` + `.gl-auth-panel` | 登录、注册、邀请、忘记密码、认证结果 | 面板 `max-width` + `calc(100vw - 32px)` |
| `.login-content` + `.login-panel` | 登录（含 `/login/oauth/authorize`）、Telegram 登录 | 分栏/紧凑两态，窄视口单列 |
| `.gl-panel` | SAML 回调等独立卡片 | 卡片 `max-width: 520px` |
| `.gl-standalone-page` | 设备码验证等居中单卡片页面 | 容器内卡片 `width: 460px; max-width: calc(100vw - 32px)` |

路由扫描 [route-sweep.spec.ts](../../../web/e2e/route-sweep.spec.ts) 以 registry 为唯一页面
清单，逐路由断言「命中上述任一壳层 + 无横向溢出 + 表格容器不越界 + `console.error=0`」；
新增页面漏选壳层会被五档视口扫描直接拦下，而不是靠人工走查。

## 全局交互与状态同步

本节是表单、列表、详情三类页面共同遵守的交互纪律，新增页面不再各自发明实现。

### 1. 表单受控属性必须透传到真实控件

`GlFormRow` 是 `Form.Item` 的直接子元素，antd 通过 `cloneElement` 注入
`value / onChange / id / aria-*`；它必须继续透传给内部控件。缺失透传会同时导致
「输入不进表单」「label 的 `for` 指向不存在」「错误文案重复」三类问题（回归用例
`GlFormRow.test.tsx`）。禁止在 `Form.Item` 与控件之间再插入不透传 props 的包装组件。

### 2. 提交锁与导航

- 提交期间按钮 `loading` 与 `isSubmitting` 同源，禁止重复提交。
- 提交成功后放行导航（不触发未保存保护弹窗）；提交失败保留输入并展示字段级错误。
- 破坏性写操作（删除、轮换凭据、禁用）走二次确认，确认框内展示对象名称而非 UUID。

### 3. 请求键包含租户与筛选维度

查询键（`queryKey`）必须包含租户标识与全部筛选参数；切换租户或筛选时以失效旧键
实现刷新，禁止用局部 `setState` 伪刷新。租户标识必须取会话里的 `organization.id`
（UUID），禁止把 slug、`"me"` 等占位符传给要求 UUID 的端点；否则后端 uuid 列直接失败。

### 4. 错误面统一

| 状态码 | 展示 |
|---|---|
| 400 / 422 | 行内字段错误或筛选条件错误，不整页报错 |
| 401 | 清空会话并跳登录，不回显上一个用户的缓存数据 |
| 403 | 「无权限」面，隐藏写入口 |
| 404 | 详情页「对象不存在」面，提供返回列表 |
| 503 `unavailable` | 「当前部署不支持该能力」面（MySQL 下 dashboard / LDAP Sync） |

### 5. 登出与缓存纪律

认证面 GET 响应由服务端声明 `Cache-Control: no-store`（`etag.go`），浏览器不得回放。
前端登出必须同时清空会话 store 与查询缓存；「登出后仍回到控制台」属于 P0 缺陷，
不得以加长缓存或轮询掩盖。

### 6. 列表失效

任何写操作成功后统一失效对应列表查询键；跨页面的派生数据（统计卡片、Tab 计数）
一并失效，避免「详情已改、列表仍旧」的状态不同步。

## 页面与 API 契约硬约束

| 约束 | 规范 | 验收锚点 |
|---|---|---|
| 端点必须已声明 | 页面请求的「方法 + 路径」必须命中 `openapi/openapi.yaml` 已声明路由；禁止拼接未声明路径或使用历史别名 | AC-FEGOV-008、AC-API-009 |
| 助手族匹配 | 前端 API helper 的路径族必须与调用点一致（auth/users/apps…），禁止跨族复用 | AC-API-010 |
| 筛选与排序键 | 筛选下拉的键必须是后端实际读取的参数；排序字段必须命中服务端白名单 | AC-FEGOV-011、AC-API-007 |
| 列表控件可达 | 分页、筛选、排序控件必须触发服务端请求并体现在请求参数中 | AC-FEGOV-010 |
| 422 可操作 | 校验失败必须映射到字段级错误并聚焦对应字段，不得只弹全局提示 | AC-FEGOV-008 |
| 权限 | 页面入口、行操作与批量操作均受权限点保护；无权时不渲染而非渲染后 403 | permissions.md §9 |
| 生成物纪律 | 路由/页面/菜单表由 `registry.md` 生成块承载；新增页面必须同时更新菜单或登记 NON_NAV 理由 | `check-frontend-coverage.py` |
