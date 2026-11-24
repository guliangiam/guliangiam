# 测试规范

> 设计锚点：[DESIGN:frontend/testing]

> 目标态规范「测试规范」；写作约束见 [docs/README.md](../../README.md)。

---

## 测试分层

| 层 | 工具 | 覆盖范围 | 运行时机 |
|----|------|---------|---------|
| Unit | **Vitest** | 工具函数、hooks、组件渲染逻辑 | 每次 commit |
| Component | **React Testing Library** | 组件交互、表单校验、路由守卫 | 每次 commit |
| Integration | **Vitest + RTL** | 数据流（Controller + TanStack Query + API client，`vi.mock` 打桩） | 每次 commit |
| E2E | **Playwright** | 端到端用户流程（登录 → 创建用户 → 授权 → 登出） | CI nightly + 预发布 |
| Visual | **Playwright screenshot + 回归** | 视觉回归 | 手动或 nightly |

## Unit 测试（Vitest）

### 覆盖目标

- 工具函数：100% 覆盖率（pure function，输入 → 输出）
- 业务 hooks：关键路径覆盖（useCrudList 的查询/删除/失效/乐观更新）
- Zod schema：校验逻辑覆盖（必填、正则、跨字段约束）

### 示例

```typescript
// src/shared/utils/__tests__/sortOrderMapping.test.ts
import { describe, it, expect } from "vitest";

describe("sortOrder mapping", () => {
  it("maps 'ascend' → 'asc'", () => {
    expect(mapSortOrder("ascend")).toBe("asc");
  });
  it("maps null → undefined", () => {
    expect(mapSortOrder(null)).toBeUndefined();
  });
});
```

## Component 测试（RTL）

### 核心原则

- **从用户视角测试**（不是测试实现细节）
- 优先使用 `getByRole` / `getByLabelText` / `findByRole`（a11y 友好）
- 避免 `getByTestId`（除非无法通过 a11y 定位）
- 表单测试：填写 → 提交 → 断言错误或成功反馈

### 示例（表单校验）

```typescript
// src/shared/ui/__tests__/Form.test.tsx
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect } from "vitest";

describe("Form", () => {
  it("shows validation error when email is invalid", async () => {
    const onSubmit = vi.fn();
    render(<TestForm onSubmit={onSubmit} />);

    await userEvent.type(screen.getByLabelText(/邮箱/i), "invalid-email");
    await userEvent.click(screen.getByRole("button", { name: /保存/i }));

    await waitFor(() => {
      expect(screen.getByText(/有效邮箱/i)).toBeInTheDocument();
    });
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
```

### 示例（权限守卫）

```typescript
// src/app/router/__tests__/RequirePermission.test.tsx
it("shows ForbiddenPage when permission missing", () => {
  vi.mock("@/shared/hooks/usePermissions", () => ({
    usePermissions: () => ({ permissions: { actions: ["users:read"] }, isLoading: false }),
  }));
  render(<RequirePermission permission="users:write"><UserForm /></RequirePermission>);
  expect(screen.getByText(/无权限/i)).toBeInTheDocument();
});
```

## Integration 测试（Vitest + RTL）

### 数据层集成

测试 Controller + TanStack Query + API client 的端到端数据流。HTTP mock 统一在 `@/shared/api/**` 客户端模块边界以 `vi.mock` 打桩（仓库不引入 MSW），i18n 以 `vi.mock("react-i18next")` 提供键回显；示例见 `web/src/pages/users/UserCreatePage.test.tsx` 与 `web/src/features/auth/login-flow.test.tsx`。

断言口径：loading 态（Skeleton/progressbar）→ 数据渲染 → 错误态映射，与真实 API client 的错误分类保持一致。

## E2E 测试（Playwright）

### 关键流程

| 场景 | 覆盖步骤 |
|------|---------|
| 登录流程 | 用户名密码登录 → MFA（如有）→ 进入 Dashboard |
| 用户 CRUD | 创建用户 → 列表看到 → 编辑 → 删除 |
| 权限控制 | 无权限用户看不到菜单 → 有权限用户看到 |
| OAuth 授权码 | 第三方应用 → 重定向 GuliangIAM → 登录 → 同意 → 回调 |
| SAML 联邦 | 外部 IdP → GuliangIAM SP → 断言消费 → 登录 |
| 列表批量操作 | 零选中时复选框列已存在 → 受保护行复选框置灰 → 选中后批量栏出现且默认删除按钮可达 → 取消选择后批量栏消失 |

### 示例

```typescript
// e2e/auth/login.spec.ts
import { test, expect } from "@playwright/test";

test("user can login with valid credentials", async ({ page }) => {
  await page.goto("/login");
  await page.fill('input[name="username"]', "admin");
  await page.fill('input[name="password"]', "password123");
  await page.getByRole("button", { name: /登录/i }).click();

  await expect(page.getByText(/控制台/i)).toBeVisible();
  await expect(page).toHaveURL("/dashboard");
});
```

### 页面验收矩阵（全站逐页）

页面清单以 `docs/design/frontend/registry.md` 生成表为唯一事实源（复现：`python3 scripts/gen_frontend_registry.py --check`），
按域分批（每批 ≤8 文件）执行 [interaction.md](interaction.md) 的 R1–R10 rubric：

| 层级 | 验证 | 判据 |
|---|---|---|
| 静态 | `check-form-detail-patterns.py` / `check-ui-wiring.py` / `check-i18n-keys.py` / `check-frontend-coverage.py` | 0 新增失败 |
| 单元 | 本次变更涉及的 `*.test.tsx` | `pnpm --dir web test --run` 全通过 |
| 浏览器 | `route-sweep` 五档视口 × 本次变更页面 | 无横向溢出、壳层命中、`console.error=0` |
| 交互 | 本次变更页面至少 390 + 1440 两条断言（含刷新回显） | Playwright 通过 |
| 无障碍 | `pnpm --dir web e2e:a11y` 浅/深两套配色 | 0 failed（第三方嵌入页显式 skip） |

**逐页证据口径**：合格页记录「路由 + 文件 + rubric 结果 + E2E 用例名」；修复页额外记录缺陷 ID、
修改文件与回归用例。不得用「抽样通过」代替全量页面清单。

### L4 覆盖与「假跳过」纪律

E2E 的真实覆盖取决于三件事，缺一不可：

| 项 | 要求 | 事实源 |
|---|---|---|
| 会话 | `auth.setup.ts` 登录一次并落盘 `storageState`；需要会话的页面被重定向 `/login` 时判失败，不判 SKIP | `web/e2e/auth.setup.ts`、`route-sweep.spec.ts` |
| 夹具 | `fixtures.setup.ts` 从 `/api/v1/auth/account` 解析组织/用户标识并落盘 `.auth/fixtures.json`；动态路由统一读该文件 | `web/e2e/fixtures.ts` |
| 限流 | 测试后端显式覆盖 `RATE_LIMIT_PER_IP_REQUESTS` / `RATE_LIMIT_GLOBAL_RATE` / `RATE_LIMIT_AUTH_RATE`；生产默认值不变 | `scripts/verify-docs.sh` 之外的环境启动命令 |

纪律：`route-sweep` 只允许协议回调面（CAS/SAML/OAuth2/callback）与显式无夹具路径 SKIP；
其余 SKIP 必须附原因并在报告中汇总。逐页截图由 `page-audit.spec.ts` 落盘到 `reports/`（不入库）。

## 视觉回归（Playwright Screenshot）

- 基准截图存 `e2e/screenshots/`
- 每次手动触发或 nightly 对比
- 页面变更较大时手动更新基准

## 测试文件位置约定

```
web/src/
├── shared/
│   ├── utils/
│   │   └── __tests__/              — Unit 测试（vitest）
│   ├── hooks/
│   │   └── __tests__/              — Unit 测试
│   └── ui/
│       └── __tests__/              — Component 测试（rtl）
├── domains/users/
│   └── __tests__/                  — Component + Integration 测试
└── e2e/
    ├── auth/login.spec.ts          — E2E（playwright）
    ├── crud/users.spec.ts
    └── visual/                     — Visual regression
```

## Mock 策略

| 层级 | 方式 |
|------|------|
| Unit | `vi.mock()` 模拟依赖模块 |
| Component | `vi.mock()` + mock hooks/stores |
| Integration | `vi.mock()` 在 `@/shared/api/**` 客户端模块边界打桩 |
| E2E | **真实后端**（test 数据库 + seed 脚本） |

**反模式**：E2E 用前端层 mock——网络层 bug 漏测，E2E 必须打真实后端。


---

### i18n 治理（原 i18n.md，全文保留）

> 目标态规范「i18n 规范」；写作约束见 [docs/README.md](../../README.md)。

---

### 技术架构

```
src/shared/i18n/
├── index.ts                    — 初始化、语言检测、fallback 配置
├── locales/
│   ├── en/common.json          — 通用 UI 词汇（OK/Cancel/Save/错误提示等）
│   ├── en/auth.json            — 认证流程（登录/注册/找回密码/SSO）
│   ├── en/resource.json        — 业务资源（用户/应用/提供者/角色/证书等）
│   ├── zh-CN/                  — 简体中文（100% 覆盖）
│   ├── zh-TW/                  — 繁体中文
│   ├── ja/  ko/  de/  es/  fr/ — 日/韩/德/西/法
│   ├── pt/  ru/  tr/  uk/  pl/ vi/ — 葡/俄/土/乌/波/越
│   └── ... (语言目录由 `ls web/src/shared/i18n/locales` 枚举)
```

**命名空间同构**：每个语言目录与 en 目录严格同构。语言目录与命名空间清单以枚举为准（复现：`ls web/src/shared/i18n/locales`；`ns` 数组见 `web/src/shared/i18n/index.ts`）。

### 页面消费

```typescript
// 方式一：单 namespace
const { t } = useTranslation("common");
<Button>{t("save")}</Button>

// 方式二：多 namespace（带前缀）
const { t } = useTranslation();
<Button>{t("resource.newUser", { ns: "resource" })}</Button>
```

### 命名规范

| 类型 | 格式 | 示例 |
|------|------|------|
| 通用词汇 | `common.action.{verb}` | `common.action.save` → "保存" |
| 业务资源 | `resource.{resource}.{field}` | `resource.user.email` → "邮箱" |
| 认证流程 | `auth.{action}.{step}` | `auth.login.password` → "密码" |
| 页面标题 | `{domain}:title` | `users:title` → "用户管理"（命名空间前缀） |
| 错误消息 | `common.error.{type}` | `common.error.required` → "此项必填" |

### i18n 治理要求

| # | 要求 | 判据 |
| --- | --- | --- |
| G1 | 修改语言包或批量替换文案后，必须跑全量构建与 lint/parse 验证（TypeScript 编译器对部分 JSX 语法问题不如 oxc 严格） | CI `pnpm build` + `pnpm lint` |
| G2 | 批量插入 `useTranslation` 后必须扫描 hook 位置：hook 声明不得出现在组件顶层之外的缩进层 | 评审 + 静态扫描 |
| G3 | 批量替换采用「长模式优先于短模式」；行业通用英文术语按下方清单保留，不翻译 | 评审 |
| G4 | 键完整性、语言包 parity、键树结构由门禁强制；缺失键必须显式暴露（回落键名或默认语言），不得静默空白 | `python3 scripts/check-i18n-keys.py`（CI 阻塞） |
| G5 | 死键清理只针对「扁平化消费形式之外确实无引用」的键；嵌套键的死引用多为扁平化消费的正常形态，不得据此误删 | `check-i18n-keys.py --update-baseline` 收敛基线 |
| G6 | 新增用户可见文案必须同步全部语言包（缺失键在开发模式下显式暴露） | `AC-FEGOV-004` |

#### 术语保留清单
#### 技术术语不翻译清单

以下英文术语有意保留，翻译后反而降低可读性：

| 类别 | 词汇 |
|------|------|
| 标准协议 | OAuth、OIDC、SAML、CAS、JWT、WebAuthn |
| 网络认证 | RADIUS、LDAP、MFA、IdP、SP |
| 密码学 | RSA、ECDSA、AES、SHA、HMAC |
| 数据格式 | URL、HTTP、HTTPS、XML、JSON、HTML、CSV |
| 商业服务 | Web3、MetaMask、Telegram（品牌名） |

### 文件位置

```
src/shared/i18n/
├── index.ts                    — 初始化
└── locales/
    ├── en/common.json          — 通用 UI
    ├── en/auth.json            — 认证
    ├── en/resource.json        — 业务资源
    └── {lang}/...              — 与 en 同构（语言目录枚举）
```


---

### 性能预算（原 performance.md，全文保留）

> 目标态规范「性能规范」；写作约束见 [docs/README.md](../../README.md)。

---

### 首屏性能

#### 指标

| 指标 | 目标 |
|------|------|
| FCP（First Contentful Paint） | ≤ 1.5s |
| LCP（Largest Contentful Paint） | ≤ 2.5s |
| TTI（Time to Interactive） | ≤ 3.5s |
| CLS（Cumulative Layout Shift） | ≤ 0.1 |

#### 实现要点

- **骨架屏**：所有懒加载路由用 `<Suspense fallback={<PageSkeleton />}>` 包裹，避免白屏
- **关键 CSS 内联**：首屏必要 CSS inline，其余 async 加载
- **字体子集化**：IBM Plex Sans 子集化，按需加载
- **图片懒加载**：非首屏图片 `loading="lazy"` + `decoding="async"`
- **预连接**：预连接后端 API（`<link rel="preconnect" href="{apiHost}">`）

### 首屏字节预算（门禁强制）

`scripts/check-bundle-budget.py` 读取 web 构建产物 `index.html`（`web/dist/` 由构建命令生成，不随仓库提交）的首屏请求集合，按 gzip 逐字节累加，与脚本内 `FIRST_SCREEN_BUDGET_KB` 比较。预算为 ratchet 机制：只允许在记录理由后上调，不得静默放宽。预算值、实测字节与请求清单一律以门禁输出为准（复现：`pnpm --dir web build && python3 scripts/check-bundle-budget.py`），本文不复制数值。

**预算设计口径**：antd 按组件自然分割、AppShell 懒加载、@ant-design/icons 按需引入、loader 与 API client 动态化、非首屏对话框移出 shell。

**~300 KB 目标不作为性能目标（OPEN-PERF-01）**：React 19 + antd v6（emotion runtime 为强制依赖）技术栈下，框架层最小负载加必需应用逻辑与 antd 组件已超出该目标，物理不可达。后续优化路径（切换组件库或升级 antd 移除 emotion）属独立架构决策，登记于 `OPEN-PERF-01`，不作为预算目标。

**trade-off**：预算显式化让首屏增量可被门禁持续度量；为压缩少量字节而把登录/工作台所需组件拆成更多请求，会增加首次交互延迟与瀑布风险，收益不确定。

### 虚拟滚动

大数据列表的虚拟滚动由 `ResourceListCard` 承载（`web/src/shared/ui/ResourceListCard/ResourceListCard.tsx`，基于 `@tanstack/react-virtual`）：行数超过 `VIRTUAL_ROW_THRESHOLD` 常量时自动切换虚拟模式，否则使用标准 antd Table。阈值以该常量为唯一事实源，本文不复制数值；不存在独立的 `VirtualTable` 组件。

### 代码分割

#### 分层策略

| Tier | 内容 | 分割点 |
|------|------|--------|
| Tier 0 | React + ReactDOM + React Router | 单个 vendor chunk |
| Tier 1 | antd + react-i18next + TanStack Query + Zustand | vendor chunk |
| Tier 2 | recharts + monaco-editor + lodash-es | 路由级 lazy |
| Tier 3 | 各业务域组件 | 按域路由 module lazy |

#### 实现

- 所有页面组件 `lazy(() => import(...))`
- 每个域路由模块独立 chunk
- 第三方重型库（monaco、recharts）在实际使用处 lazy import

### TanStack Query 缓存策略

| 场景 | staleTime | 说明 |
|------|-----------|------|
| 详情页 | 5 分钟 | 创建后短期内不会变 |
| 列表页 | 30 秒 | 允许较快刷新 |
| 权限列表 | 30 分钟 | 很少变 |
| Dropdown/Search 选项 | 10 分钟 | 参考数据 |

#### 缓存失效

- 创建/更新/删除后 `invalidateQueries` 失效相关 key
- 乐观更新：`onMutate` 手动 setQueryData → `onSettled` invalidate
- 跨页面共享数据用 `queryKey` 层级结构（`queryKeys.users.all` → `paginated` → `detail`）

### 避免的性能反模式

| 反模式 | 替代方案 |
|--------|---------|
| 全量加载后前端分页排序 | 后端分页 + 排序，前端只渲染当前页 |
| 在组件 `render` 内做重计算 | `useMemo` 缓存；大数据计算放到 Worker |
| 大组件频繁 re-render | `React.memo` + 合理拆分 + key 稳定化 |
| 图片未压缩 | WebP/AVIF + responsive srcset + CDN |
| 请求 N+1 | 后端批量 API + React Router loader 并行预取 |
