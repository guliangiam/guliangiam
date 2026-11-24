# ADR-032: 前端页面 Controller / View 分离

- 状态：[Current]
- 日期：2026-10-06

## 背景

`web/src/pages` 下的资源列表页（ResourceListCard 模式，覆盖 Sessions / Tokens / Resources / Permissions 等域）普遍超过 200 行，**状态管理 / API 调用 / 事件处理** 与 **JSX 渲染** 混在一个函数体内。直接问题：

1. 认知负荷高——理解一个页面需要同时消化 `useQuery` 参数、`useMutation` 回调、`useState` 初始值和 JSX 树
2. 复用困难——单独消费状态逻辑（例如在另一个组件里读 `sessionsQuery.data`）必须 import 整个 Page，而 Page 默认导出 JSX 组件，无法单独 import 逻辑
3. 测试粒度粗——对逻辑分支（pagination reset、mutation onSuccess、filter change）做单测必须挂载完整组件并 mock 所有依赖

量化基线（拆分前）：

| 指标 | 值 |
|------|----|
| ResourceListCard 页面数 | 31 |
| 其中 ≥ 200 行 | 27 |
| 其中 ≥ 300 行 | 14 |
| 用户页（UsersPage.tsx） | 462 行 |

## 决策

采用三文件模式，按 **逻辑与渲染分离** 重新组织 Page 目录：

```
src/pages/[page-name]/
├── use[PageName]Controller.tsx   ← 新增：逻辑层（hook）
├── [PageName]PageView.tsx        ← 新增：渲染层（纯组件）
└── [PageName]Page.tsx            ← 保留但重写为薄封装（~10 行）
```

### Controller（逻辑层）

- **文件**：`use[PageName]Controller.tsx`
- **形式**：自定义 hook，命名遵循 React hook 约定（`use` 前缀）
- **职责**：
  - 声明所有 `useState` / `useCallback` / `useMemo`
  - 调用 `useQuery` / `useMutation` / `useInfiniteQuery`（TanStack Query 5）
  - 调用 Zustand store selector（`useSessionStore(state => state.xxx)`）
  - 定义事件处理函数（`handleSearch` / `handleSortChange` / `handleOrgChange` / `onConfirm` 等）
  - 定义与列表页 toolbar 相关的 JSX 变量（`const toolbar = (<Space>...</Space>)`）
  - **返回一个 `useMemo` 或 plain object**，包含 View 层需要消费的所有值
- **签名**：`export function useXxxController(): XxxController` + `export type XxxController = ReturnType<typeof useXxxController>`
- **允许在文件内定义 JSX 变量**（原因见「理由与后果」）

```tsx
// src/pages/sessions/useSessionsController.tsx
import { useState, useCallback } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { Space, Popconfirm, Button } from "antd";
import { getSessionsPaginated, revokeOtherSessions } from "@/shared/api/iam/session";

export function useSessionsController() {
  const { t } = useTranslation("common");
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);
  const [pagination, setPagination] = useState({ page: 1, pageSize: 50, total: 0 });

  const sessionsQuery = useQuery({
    queryKey: ["sessions", orgFilter, pagination.page],
    queryFn: async () => {
      const result = await getSessionsPaginated({ page: pagination.page, pageSize: pagination.pageSize });
      setPagination((p) => ({ ...p, total: result.total }));
      return result;
    },
    enabled: organization !== null,
    select: (data) => data.items,
  });

  const revokeOthersMutation = useMutation({
    mutationFn: () => revokeOtherSessions(),
    onSuccess: async () => {
      queryClient.invalidateQueries({ queryKey: ["sessions"] });
    },
  });

  // toolbar JSX 留在 Controller（不是错误）
  const toolbar = (
    <Popconfirm title={t("logoutOtherDevices")} onConfirm={() => revokeOthersMutation.mutate()}>
      <Button danger>{t("logoutOtherDevices")}</Button>
    </Popconfirm>
  );

  return {
    pagination,
    setPagination,
    sessionsQuery,
    revokeOthersMutation,
    toolbar,
  };
}

export type SessionsController = ReturnType<typeof useSessionsController>;
```

### View（渲染层）

- **文件**：`[PageName]PageView.tsx`
- **形式**：普通函数组件，接收一个 `ctrl` 参数
- **职责**：
  - 从 `ctrl` 对象解构出所有需要的状态、handler 和 JSX 变量
  - 纯粹负责 JSX 结构：Layout / Card / Table columns / Toolbar slot / Empty state
  - 可以包含纯函数（`formatDateTime` / `renderSessionState` 等不依赖 hook 的辅助函数）
- **签名**：`export function XxxPageView(ctrl: XxxController): React.JSX.Element`

```tsx
// src/pages/sessions/SessionsPageView.tsx
import type { SessionsController } from "./useSessionsController";
import { Card, Descriptions, Typography } from "antd";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";

export function SessionsPageView(ctrl: SessionsController) {
  const { pagination, sessionsQuery, toolbar } = ctrl;

  return (
    <div className="gl-resource-page">
      <Card title="当前设备会话" extra={toolbar}>
        {/* 会话详情 Descriptions */}
      </Card>

      <ResourceListCard
        title="会话列表"
        toolbar={toolbar}
        serverPagination={pagination}
        query={sessionsQuery}
        columns={[
          { title: "用户名", dataIndex: "username" },
          { title: "IP", dataIndex: "ipAddress" },
        ]}
      />
    </div>
  );
}
```

### Page（薄封装）

- **文件**：`[PageName]Page.tsx`
- **形式**：10 行以内的薄封装组件
- **职责**：调用 Controller → 传给 View。唯一默认导出，保持路由注册点不变（React Router / 路由配置无需改动）
- **约束**：**不得**在此文件添加任何业务逻辑、state 或 JSX 细节，所有实现都应下沉到 Controller 或 View

```tsx
// src/pages/sessions/SessionsPage.tsx
import { useSessionsController } from "./useSessionsController";
import { SessionsPageView } from "./SessionsPageView";

export default function SessionsPage() {
  const ctrl = useSessionsController();
  return <SessionsPageView ctrl={ctrl} />;
}
```

### 依赖方向

```
Page.tsx  ──import──▶  useXxxController.tsx   ──import──▶  shared/api/*
   │                                                           shared/hooks/*
   └──import──▶  XxxPageView.tsx  ──import──▶  antd
                                            shared/ui/*
```

**严格单向**：Controller 不 import View，View 通过 `import type` 引入 Controller 的类型定义（运行时不产生依赖），Page 是唯一同时 import 两者的地方。

### Controller 返回对象的筛选原则

Controller 的 `return { ... }` 并非要返回函数体内声明的所有 `const`，需要有意识地排除通用变量：

| 类别 | 排除原因 |
|------|----------|
| `t` / `tr` / `tc`（翻译函数） | View 也可以自己 `useTranslation`，不强制从 ctrl 解构 |
| `message`（App.useApp()） | 仅 Controller 内 mutation callback 使用 |
| `queryClient` | 仅 Controller 内 invalidate 使用 |
| `account` / `organization` | 如果 View 不消费，则不返回；如果 View 需要（如权限守卫），则返回 |
| `navigate` / `useNavigate` | 同上 |

经验法则：**JSX 中出现的变量必须在 return 里**（脚本已通过 `{xxx}` 引用扫描自动补全）；仅 Controller 内部使用的不要返回。

### Controller 中的 JSX 变量规范

Controller **不是纯逻辑层**——它允许包含 JSX 变量，但仅限以下场景。本节逐条定义每种模式的归属、约束和反例。

#### 模式一：Toolbar / Header / Footer JSX 变量（MUST 留在 Controller）

特征：一个 `const` 赋值表达式，右侧是 JSX 根元素，内部引用 Controller 闭包里的 state / handler / mutation。

```tsx
// ✅ MUST 留在 Controller
const toolbar = (
  <Space>
    <Popconfirm
      onConfirm={() => revokeOthersMutation.mutate()}   // 引用 mutation handler
      disabled={!currentSession || revokeOthersMutation.isPending}  // 引用 state
    >
      <Button danger>{t("logoutOtherDevices")}</Button>
    </Popconfirm>
  </Space>
);
```

**约束**：

- JSX 内部引用的所有 `const`（mutation handler / state / t 函数）必须是 Controller 闭包内已声明的，不允许引用 View 层才有的变量
- 变量名必须能让 View 一目了然其用途（`toolbar` / `header` / `footer` / `actions` / `bulkActions`），不允许含糊命名如 `ui` / `slot1`
- 必须出现在 `return { ... }` 中

**反例（MUST NOT）**：

```tsx
// ❌ toolbar 内部引用了 pagination，但 pagination 不在 Controller 闭包里
const toolbar = (
  <Button onClick={() => setPagination({ page: 1 })}>   // setPagination 未定义
    回到第一页
  </Button>
);
```

如果 toolbar 需要某个 Controller 没有的变量，**先把那个变量加到 Controller 的 state/handler 里**，再引用。不要为了凑 toolbar 而让它跨层引用。

#### 模式二：纯 render 辅助函数（优选 View，可留 Controller）

特征：`function renderXxx(...args): JSX.Element`，内部 **不引用** 任何 hook 返回值，仅根据传入参数渲染。典型例子：状态标签、徽章、图标映射。

```tsx
// ✅ 纯函数，留在 Controller 也 OK
function renderTokenStatus(status: string) {
  const colorMap = { active: "green", expired: "orange", revoked: "red" };
  return <Tag color={colorMap[status] ?? "default"}>{status}</Tag>;
}

// ✅ 或移到 View（两者等价，优先就近放）
```

**约束**：

- 函数体内不得调用 `useXxx` hooks（React Hook 规则本身禁止）
- 不得引用 Controller 闭包中的 `t` / `message` / `account` 等——如果需要翻译，把 `t` 作为参数传入
- 如果函数只被 View 使用（如 columns 的 `render` 回调），**优先放 View**；如果 Controller 内部也需要（如 toolbar 里要渲染一个状态 Badge），则放 Controller

#### 模式三：Table Columns 数组（MUST 留在 View）

特征：`columns` 数组定义 Table 的列，包含 `title` / `dataIndex` / `render` / `width` 等。**这是 View 的职责**，因为：

1. Column 的视觉布局（宽度、对齐、标题）是渲染关注点
2. `render` 回调里经常直接引用 `t()` 做列标题，属于 View 的翻译上下文

```tsx
// ❌ MUST NOT 放在 Controller
const columns = [
  { title: t("username"), dataIndex: "username", width: 120 },
  { title: t("status"), dataIndex: "status", render: (v) => <Tag>{v}</Tag> },
];

// ✅ 放在 View
export function ResourcesPageView(ctrl: ResourcesController) {
  const { resourcesQuery, handleDelete } = ctrl;
  return (
    <ResourceListCard
      columns={[
        { title: tc("name"), dataIndex: "name" },
        { title: tc("actions"), key: "actions", render: (_, r) => (
          <Button onClick={() => handleDelete(r.id)}>{tc("delete")}</Button>
        )},
      ]}
    />
  );
}
```

**约束**：

- 如果 column 的 `render` 回调需要调用 Controller 的 handler（如 `handleDelete(record.id)`），**通过闭包捕获 View 解构出来的 handler 引用**，不要让 Controller 预先构造好 columns 数组再 return
- 唯一例外：columns 数组不包含 JSX / 不引用 hooks / 不引用 handler，只是纯 `{ title, dataIndex }` 描述——这种情况下可视为配置常量，放 Controller 或独立 `columns.ts` 均可

#### 模式四：常量数组含 JSX 字面量（可留 Controller）

特征：`const xxx: ItemType[] = [{ key, icon: <IconOutlined />, path }]`，用于快捷入口 / 导航菜单等。JSX 仅作为 `icon` 属性值，不引用任何 hook state。

```tsx
// ✅ 允许留在 Controller（或独立 constants 文件）
const SHORT_CUTS: ShortcutItem[] = [
  { key: "users", icon: <UserOutlined />, path: "/users" },
  { key: "apps",  icon: <AppstoreOutlined />, path: "/applications" },
];
```

**约束**：

- 数组里的 JSX 元素不得引用 Controller 的 state / handler / mutation
- 如果后续需要根据权限过滤（如某个 shortcut 仅 admin 可见），**过滤逻辑留在 Controller**，return 过滤后的数组：
  ```tsx
  const visibleShortCuts = SHORT_CUTS.filter(item => account?.isAdmin || item.key !== "users");
  return { visibleShortCuts };
  ```

#### 决策速查表

| JSX 变量类型 | 归属 | 核心理由 |
|-------------|------|----------|
| toolbar / header / footer / bulkActions（含闭包 state） | **Controller** | 引用 Controller 私有 state/mutation，跨层暴露代价大 |
| 纯 render 辅助函数（无 hook 依赖） | **View 优先** | 就近消费；Controller 内部也要用则放 Controller |
| Table columns 数组 | **View** | 视觉布局是 View 职责；render 回调通过闭包捕获 handler |
| 常量数组含 JSX icon（无 state 引用） | **Controller 或独立文件** | 纯配置，过滤逻辑如有则留在 Controller |
| 组件内联的条件渲染 JSX（`{loading && <Spin />}`） | **View** | 纯结构 |

#### 拆分脚本辅助规则

自动拆分脚本按以下优先级处理函数体内的 `const xxx =` 变量：

1. **JSX 变量（含 `<` 标签）** → 一律留在 Controller、进 return（不尝试移到 View）
2. **`columns` / `tableColumns` 命名** → 不进 Controller return（脚本报 WARN，人工确认后移到 View）
3. **纯函数 `function renderXxx` / `const renderXxx = () =>`** → 留在原位置（Controller 或 View 都 OK）
4. **其他 `const`** → 按通用变量筛选原则决定是否进 return

### 文件扩展名

Controller 文件使用 **`.tsx` 而非 `.ts`**，原因：部分 Controller（含 `toolbar` JSX 变量、Table columns 的 `render: () => <Tag>...</Tag>`）包含 JSX 语法，`.ts` 扩展名不允许解析 JSX。即使当前某些 Controller 不含 JSX，统一用 `.tsx` 避免后续加 JSX 时还要改扩展名。

## 理由与后果

**为什么 Controller 可以包含 JSX 变量（toolbar）？**

纯渲染层分离方案要求把 `toolbar` 移到 View，再从 Controller 返回它依赖的原始值（`revokeOthersMutation`、`currentSession` 等）。这导致：

1. Controller return 对象膨胀——为了让 View 能重建 toolbar，不得不把 toolbar 内部引用的所有 handler 和 state 都暴露出来
2. toolbar 内部逻辑（判断 `currentSession` 是否存在才允许点击）分散在 Controller （mutation）和 View （JSX 条件）两处
3. 拆分脚本实现复杂度剧增——需要识别哪些 const 是「纯逻辑」哪些是「含 JSX 的 UI 变量」，并分别处理

**允许 Controller 直接 return JSX 变量** 的代价极小（Controller 文件扩展名从 `.ts` 改为 `.tsx`），收益是：

- Controller 封装了「数据 + 展示组件入口」，View 只负责 Layout 和 Table columns
- 拆分脚本实现简化到「函数体里所有 const 都进 return」，不需要识别 JSX 边界
- 运行时行为完全不变（React hook 返回 JSX 是合法操作，调用方只是把它嵌到 JSX 树里）

**为什么 View 不自己调用 hooks？**

两种可行方案：
- Controller 统一调用 `useTranslation` / `useSessionStore`，return `t` / `account` 给 View → View 零 hook
- View 自己调用 hooks，Controller 只 return 业务 state 和 handler

本次实现采用前者（Controller 统一），理由：View 文件更纯粹（只解构 + JSX），且翻译 key 选择、store selector 初始化位置集中在一个文件，便于统一调整。但这不是硬约束——如果某个 View 有独立的翻译 namespace 需求，**可以**在 View 里单独 `useTranslation("xxx-namespace")`，再把 `t` 传给需要的地方。

**潜在代价与缓解**：

| 代价 | 缓解 |
|------|------|
| Controller 里同时有逻辑 + JSX 变量，违反「Controller 纯逻辑」直觉 | 在注释中标注哪些是 UI 变量（toolbar / header 等）；真正要做纯逻辑分离时再把这些 JSX 变量抽成独立的 `render-utils.tsx` |
| View 与 Controller 的 import 块重复（各自 import 一遍 antd / hooks / api） | TypeScript/ESBuild 会自动去重，bundle 体积无影响 |
| Controller return 对象的 shape 演进缺乏约束 | Controller 类型通过 `ReturnType<typeof useXxxController>` 自动推导，View 解构时 TS 会提示缺失字段；未来可引入 `satisfies` 显式约束 |

## 替代方案

- **自定义 hook + 纯 View，但 JSX 变量强制移到 View**：Controller 变成纯逻辑，View 自己调用 hooks + 自己定义 toolbar。放弃原因：需要 Controller 把 toolbar 依赖的原始值全部暴露，return 对象膨胀且 JSX 内聚性差；实现复杂度高到 v8/v9/v10 三版脚本都没稳定。
- **useReducer + useReducer provider（Context）**：全局 reducer 分发 state action。放弃原因：ResourceListCard 各页面独立，跨页共享极少；reducer 样板代码远高于当前 useState + useCallback 组合。
- **RTK Query / Zustand Query 统一数据流**：放弃原因：TanStack Query 已在使用，替换成本高且收益不明确。
- **不拆，保持单文件**：放弃原因：27 个页面超过 200 行，UsersPage 462 行，认知负荷和复用瓶颈真实存在。

## 验证证据

- **Current 量化**（拆分验收时点快照，2026-10；现状计数以命令输出为准，不手工维护）：
  - Controller/View 对覆盖已迁移页面（对数以 `find web/src/pages -name '*Controller*'` 及对应 View 文件输出为准）
  - 构建通过：`vite build` 2.47s，零 rolldown 错误
  - 类型通过：`tsc --noEmit` 零 Controller/View 相关错误
  - Controller 行数范围：29~147 L（平均 ~95 L）
  - View 行数范围：56~349 L（平均 ~130 L）
  - Page 薄封装行数：均 ≤ 12 L

- **拆分迭代中踩过的坑（用于防止回归）**：
  | 版本 | 问题 | 根因 | 修复 |
  |------|------|------|------|
  | v8 | 多行 import 截断（`import { deleteToken,` 丢失前导 `import {`） | 逐行过滤跳过非首行 | 整体提取到最后一个分号 |
  | v9 | Controller 里 JSX 变量（toolbar）被单独移动后，依赖的 handler 没一起暴露 | 未考虑 JSX 变量的闭包依赖 | 放弃移动，允许 Controller return JSX |
  | v10 | Controller `.ts` 文件内 `<Tag>` 语法报错 | `.ts` 扩展名禁止 JSX | 改用 `.tsx` |
  | v10 | 之前就存在的 auth Controller（git 追踪的）被误删 | `find -delete` 没区分新旧文件 | 改用 `git restore` 恢复 git 追踪文件 |

## 复审条件

以下情况触发复审本 ADR：

1. 需要将 Controller 里的 JSX 变量彻底分离（如追求纯逻辑层），并引入第三文件 `render-utils.tsx`
2. 引入新的状态管理方案替代 TanStack Query + Zustand（如切换到 Redux Toolkit Query）
3. 某个页面规模膨胀到 Controller/View 各自超过 200 L，需要进一步拆分（如把 Table columns 抽出到单独文件）
4. 新增页面默认模板需要更新（如 ESLint rule、scaffold script 变更）
