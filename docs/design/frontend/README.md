# 前端设计总纲

> 设计锚点：[DESIGN:frontend/README]

> 目标态规范「前端设计总纲」；写作约束见 [docs/README.md](../../README.md)。

---

## 技术栈

| 层 | 选型 | 说明 |
|----|------|------|
| 框架 | React 19 + TypeScript | SPA 模式，不使用 SSR |
| 构建 | Vite 8 | 快速 HMR + ESM |
| UI 组件库 | Ant Design 6 | 见下文「设计系统」节主题映射 |
| 路由 | React Router v7（`createBrowserRouter`） | 按域模块化，交互规范见 interaction.md；版本以 `web/package.json` 为准 |
| 服务端状态 | TanStack Query | 与客户端状态（Zustand）分层 |
| 全局客户端状态 | Zustand | session / theme / layout 等 |
| 表单 | antd Form + Zod | 表单状态由 antd Form 承载，契约/schema 校验用 Zod |
| 异步流程编排 | 域 Controller hooks（`useXxxPageController`） | 登录/MFA/OAuth 授权等流程状态 |
| HTTP 客户端 | 自研 `http.ts`（fetch + HttpError 类） | 信封 schema 校验 + 错误分类 |
| i18n | react-i18next | 全部语言包；命名空间由 `ns` 声明枚举 |
| 测试 | Vitest + React Testing Library + Playwright | 详见 testing.md |

## 组件目录结构

目录职责固定；具体文件、子目录与规模以枚举命令为准，本文不手写文件树：

| 目录 | 职责 |
|------|------|
| `web/src/app/` | antd 主题映射、AppShell 布局与路由装配 |
| `web/src/features/` | 跨页面领域逻辑：session/theme/layout store、导航菜单、组织切换、用户与账户条目 hooks |
| `web/src/pages/` | 按域扁平的页面与页面私有组件 |
| `web/src/shared/ui/` | 基础组件 + IAM 业务组件（表单壳、列表壳、状态标签、选择器族） |
| `web/src/shared/hooks/` | 共享 hooks（CRUD 列表、媒体查询、权限） |
| `web/src/shared/api/` | HTTP 客户端、Query Key 构造器、按域 API 模块与路径常量 |
| `web/src/shared/i18n/` | i18n 初始化、命名空间声明与全部语言包 |
| `web/src/shared/{lib,utils,constants,types,components}/` | 认证流程辅助、通用工具、常量、共享类型与跨域组件 |
| `web/src/generated/` | 由契约生成的类型与错误码（禁手改） |

```bash
ls web/src/app web/src/features web/src/pages web/src/shared
ls web/src/shared/i18n/locales                    # 全部语言包
python3 scripts/gen_frontend_registry.py --check  # 路由/页面注册表零差异
```

## 设计子文档索引

> 导航 / 数据层 / 性能 / i18n 等专题在本文件内按章节覆盖，不单列子文档。

| 文件 | 内容 |
|------|------|
| [interaction.md](interaction.md) | 页面模板 + 响应式 + 字段显示规范 + 交互简化 + CrudListPage |
| [security.md](security.md) | Cookie 属性 + CSRF + SameSite 矩阵 + DPoP 客户端 |
| [testing.md](testing.md) | Vitest + RTL 单元/集成 + Playwright E2E |
| [registry.md](registry.md) | 路由 + 菜单唯一事实源（脚本生成物） |
| [casdoor-parity-checklist.md](casdoor-parity-checklist.md) | Casdoor 管理页 → GLIAM 对齐决策台账（KEEP/ADD/ADAPT/REJECT/NO_MATCH） |
| [design-system](#设计系统原-design-systemmd全文保留) | 色彩/字体/间距/圆角/阴影/动画 + antd 主题映射（本文内联） |
| [a11y](#5-a11y-规范) | WCAG 2.1 AA 合规清单 + antd a11y 集成 + 焦点管理（本文内联） |

## 前端结构与改进方向（目标态）

规模数字（页面/组件/工具文件数、首屏字节）为**可生成事实**，以目录枚举与 CI 门禁为准，本文不手写；本节只写设计约束。

**设计约束（目标态）**：

- **数据层单轨**：远程数据一律走 TanStack Query，客户端状态收敛为极少数 Zustand store，禁止双数据源
- **菜单与路由**：由前端注册表生成并与后端 OpenAPI 对齐，禁止在菜单里硬编码路径/顺序
- **API 路径**：统一由 `web/src/shared/api/paths.ts` 提供，业务代码禁止出现 `/api/v1/` 字面量（CI `paths.ts gate`）
- **关联字段**：一律使用标准 Select 组件族（Organization / User / Role / Group / Application / Resource 等），禁止自由输入
- **契约校验**：后端响应经 Zod 解析；错误体三形状统一映射
- **首屏预算**：以 CI 首屏字节预算门禁 `scripts/check-bundle-budget.py` 与 `web/` 构建产物为准

**改进方向（按优先级）**：

| 优先级 | 方向 |
|--------|------|
| P0 | 关联字段选择器组件化（消除各页内联 AsyncSearchSelect 与选项映射）；菜单/路由与后端 OpenAPI 自动对齐；API 路径常量化与门禁常态化 |
| P1 | 首屏字节预算持续收敛；表单校验 schema 集中注册；`shared/components` 与 `shared/ui` 职责边界厘清 |
| P2 | 路由级懒加载全覆盖；`shared/lib` 统一出口；Domain Select 组件族 API 归一；表格 columns schema 集中化；字段命名 lint guard |

### 共享组件消费规则与页面族

页面按「族」复用同一壳组件，禁止逐页重写布局与提交语义：

| 页面族 | 壳组件 | 必须遵守 |
|---|---|---|
| 列表页 | `ResourceListCard` + `CrudListPage` | 筛选/分页/排序走 query 参数；空态用组件内置 empty；主操作固定在页头 |
| 详情/编辑页 | `ResourceFormPage` | `onFinish` 必须返回 Promise；提交锁由壳组件持有，页面不得自行 `loading` 抢跑；`onFinishFailed` 首错定位由壳组件统一处理 |
| 仪表盘/分析页 | 页面级 layout + `gl-panel` 卡片 | 不嵌套 `ResourceFormPage`；图表容器给固定高度，避免加载态跳动 |
| 协议/公开页 | 独立路由（`/login/*`、`/oauth2/*`、`/cas/*`、`/saml/*`） | 不挂管理台侧栏；受限读响应带 `no-store` |

**表单壳契约**（`web/src/shared/ui/ResourceFormPage.tsx`）：

- `onFinish` 返回 Promise 时，提交按钮在 Promise settle 前保持 disabled/loading；返回 `false` 取消关闭。
- 服务端字段错误经 `form.setFields` 回写；页面只负责把错误映射到字段名。
- 草稿与未保存离开保护由壳组件提供，页面不重复实现。
- 390 宽度下栅格退化为单列，`GlFormRow` 负责标签与控件的换行。

**关联选择器清单**：`AsyncSearchSelect`（通用远程搜索）、`GroupTreeSelect`（组织树）、`UserSelect` / `RolePaginateSelect` / `ApplicationSelect` / `OrganizationSelect`、`PlanSelect` / `PricingSelect` / `ProductSelect` / `OrderSelect`、`RegionSelect` / `CountryCodeSelect`。禁止在页面内联 `options` 拉取与 `labelInValue` 映射；新增关联字段必须复用这些组件或按同一契约扩展。

**主显示区宽度**：内容宽度只有三档语义类（`web/src/styles.css`）：`.gl-content__inner`（列表/表格，`max-width: 1600px`）、`.gl-content__inner--form`（表单/详情，`1200px`）、`.gl-content__inner--reading`（协议/长文设置，`960px`）；`<=1024px` 一律退化为 `100%`。页面必须选用语义类，禁止再写页面级横向 `max-width` 或第二套容器。逐页截图与横向溢出断言见 [testing.md](testing.md) 的 L4 覆盖口径。



---

### 设计系统（原 design-system.md，全文保留）

> 目标态规范「设计系统」；写作约束见 [docs/README.md](../../README.md)。

---

### 色彩体系

#### 主色与品牌色

| 色彩 | 值 |
|------|-----|
| `--gl-color-primary` | `#0b8073`（brand teal） |
| `--gl-color-primary-hover` | `#0b7a67` |
| `--gl-color-primary-active` | `#096b5c` |
| `--gl-color-primary-bg` | `color-mix(in srgb, #0b8073 8%, transparent)` |

完整亮/暗调色板以 `web/src/app/theme/brand-palette.ts` 的 `BRAND_PALETTE` 为唯一事实源（设计令牌镜像门禁 `python3 scripts/check-token-parity.py`），本文不复制取值清单。

#### 语义色

| 语义 | CSS 变量 | 亮色 | 用途 |
|------|---------|------|------|
| Success | `--gl-color-success` | #1f9d6c | 操作成功、状态正常 |
| Warning | `--gl-color-warning` | #c67a22 | 警告、即将过期 |
| Error | `--gl-color-error` | #cf2e2e | 操作失败、错误状态 |
| Info | `--gl-color-primary` | #0b8073 | 信息提示 |

#### 中性色阶

| Token | 值 |
|-------|-----|
| `--gl-color-text` | `#102a2c` |
| `--gl-color-text-secondary` | `rgba(16, 42, 44, 0.66)` |
| `--gl-color-text-tertiary` | `#5f7076` |
| `--gl-color-bg` | `#f8fbfb` |
| `--gl-color-bg-elevated` | `#ffffff` |
| `--gl-color-border` | `rgba(16, 42, 44, 0.08)` |
| `--gl-color-border-strong` | `rgba(16, 42, 44, 0.12)` |

### 字体体系

| 维度 | 值 |
|------|-----|
| 字体族 | `"IBM Plex Sans", "Segoe UI", sans-serif` |
| 等宽字体 | `"IBM Plex Mono", "SFMono-Regular", monospace` |

| Token | 大小 | 用途 |
|-------|------|------|
| `--gl-font-size-xs` | 12px | 辅助文本、标签 |
| `--gl-font-size-sm` | 13px | 次要正文 |
| `--gl-font-size-base` | 14px | 正文（默认） |
| `--gl-font-size-lg` | 16px | 小标题 |
| `--gl-font-size-xl` | 20px | 区块标题 |
| `--gl-font-size-2xl` | 22px | 页面标题 |
| `--gl-font-size-3xl` | 26px | 大标题 |
| `--gl-font-size-4xl` | 28px | Hero 标题 |

### 间距体系（8px 基准）

| Token | 值 | 用途 |
|-------|-----|------|
| `--gl-space-1` | 4px | 紧凑间距 |
| `--gl-space-2` | 8px | 基准间距 |
| `--gl-space-3` | 12px | 1.5x |
| `--gl-space-4` | 16px | 常用间距 |
| `--gl-space-5` | 20px | 2.5x |
| `--gl-space-6` | 24px | 区块间距 |
| `--gl-space-8` | 32px | 大区块间距 |
| `--gl-space-12` | 48px | 页面级间距 |

### 圆角体系

| Token | 值 | 用途 |
|-------|-----|------|
| `--gl-radius-sm` | 6px | 标签、小按钮 |
| `--gl-radius-md` | 10px | 下拉、tooltip |
| `--gl-radius-lg` | 14px | 卡片、模态框 |
| `--gl-radius-xl` | 18px | 大卡片 |
| `--gl-radius-full` | 999px | 胶囊、头像 |

### 阴影体系

| Token | 值 |
|-------|-----|
| `--gl-shadow-sm` | `0 1px 3px rgba(12, 48, 50, 0.06)` |
| `--gl-shadow-md` | `0 4px 16px rgba(12, 48, 50, 0.08)` |
| `--gl-shadow-lg` | `0 22px 60px rgba(12, 48, 50, 0.08)` |

### 动画体系

| Token | 值 |
|-------|-----|
| `--gl-duration-fast` | 150ms |
| `--gl-duration-normal` | 250ms |
| `--gl-duration-slow` | 400ms |
| `--gl-easing-standard` | `cubic-bezier(0.4, 0, 0.2, 1)` |
| `--gl-easing-decelerated` | `cubic-bezier(0, 0, 0.2, 1)` |
| `--gl-easing-accelerated` | `cubic-bezier(0.4, 0, 1, 1)` |

**reduced-motion 支持**：`@media (prefers-reduced-motion: reduce)` 强制 0.01ms。

### antd 主题映射

antd 6 ConfigProvider 主题由 `web/src/features/theme/OrganizationThemeProvider.tsx` 构建：以 `BRAND_PALETTE`（`web/src/app/theme/brand-palette.ts`）为基线，按亮/暗模式选择 `theme.defaultAlgorithm` / `theme.darkAlgorithm`，组织主题 token 经白名单净化后覆盖（表面/文字类覆盖仅浅色模式生效，暗黑模式统一回落 `BRAND_PALETTE.dark`）。主题编辑器的预设配色目录为 `web/src/app/theme/theme-presets.ts` 的 `PRESET_THEMES`；间距/字体等设计令牌为 `web/src/app/theme/tokens.ts`（`SPACING` / `TYPOGRAPHY` / `DESIGN_TOKENS`）。token 映射明细以 Provider 源码为事实源，本文不复制。

### 与其他前端子文档的关系

- 组件库实现 → `src/shared/ui/`
- antd Token 完整定义 → `src/app/theme/`

---


### 附录：Ant Design 6 前端设计规范（Design Tokens 与组件规格）

---

### 0. 设计决策概述

#### 0.1 为什么保持纯 Ant Design 6

| 维度 | 决策 | 理由 |
|------|------|------|
| UI 框架 | **纯 antd 6 + @ant-design/icons** | 纯 antd 6 + @ant-design/icons（版本以 `web/package.json` 为准），bundle 稳定 |
| 样式方案 | **CSS 变量（--gl-*）+ ConfigProvider.token 双向映射** | 避免引入 Tailwind/shadcn 导致的样式优先级冲突、主题覆盖层混乱 |
| 组件增强层 | **自建 Gl* 组件**（GlFormRow、GlEditShell、GlPageHeader 等） | 在 antd 之上提供 GuliangIAM 领域语义，不替换底层组件 |

#### 0.2 Gl* 组件增强承诺

| # | 常见封装不足 | GuliangIAM 目标 | 技术要点 |
|---|-------------|----------------|---------|
| 1 | FormRow 只是纯展示组件，与 antd Form.Item 解耦 | **GlFormRow 自动桥接 antd Form.Item validation 状态** | GlFormRow 消费 Form.Item context 中的 meta.errors / meta.touched，自动联动 error 文案、label 红色、行级 shake 动画 |
| 2 | EditPageShell 只做布局，无草稿保存 | **GlEditShell 内置草稿自动保存（debounce 2s → localStorage）** | GlEditShell 监听 Form 的 onValuesChange，debounce 后序列化到 localStorage key `gl:draft:{route}:{entityId}`，页面恢复时自动填充，离开时弹确认 |
| 3 | 表格无虚拟滚动，大数据量直接 DOM 爆炸 | **ResourceListCard 支持虚拟滚动（@tanstack/react-virtual 已在依赖中）** | 行数超过 `VIRTUAL_ROW_THRESHOLD`（事实源 `web/src/shared/ui/ResourceListCard/ResourceListCard.tsx`）时自动切换虚拟模式；ColumnMenu/ColumnSearch/ColumnFilter 三列增强 |
| 4 | 无 Command Palette | **⌘K / Ctrl+K Command Palette 全局搜索** | 搜索资源（用户/角色/应用）、跳转到常用页面、执行全局操作；WAI-ARIA combobox 模式 |
| 5 | 错误提示分散 | **全局 ErrorBoundary + 结构化错误呈现** | AppShell 级 Suspense fallback + 路由级 ErrorBoundary，错误展示遵循 [设计系统](#设计系统原-design-systemmd全文保留) 的 colorError + 统一 i18n key |

---

### 1. Design Tokens

#### 1.1 CSS 变量（--gl-*）

所有 GuliangIAM 设计 Token 以 `--gl-` 前缀定义在 `:root` 和 `.gl-theme-dark` 类下：

```css
/* ============== 亮色主题 ============== */
:root, .gl-theme-light {
  /* 品牌色 */
  --gl-color-primary: #0b8073;
  --gl-color-primary-hover: #0b7a67;
  --gl-color-primary-active: #096b5c;
  --gl-color-primary-bg: color-mix(in srgb, #0b8073 8%, transparent);

  /* 语义色 */
  --gl-color-success: #1f9d6c;
  --gl-color-warning: #c67a22;
  --gl-color-error: #cf2e2e;
  --gl-color-info: #0b8073;

  /* 中性色 */
  --gl-color-text: #102a2c;
  --gl-color-text-secondary: rgba(16, 42, 44, 0.66);
  --gl-color-text-tertiary: #5f7076;
  --gl-color-bg: #f8fbfb;
  --gl-color-bg-elevated: #ffffff;
  --gl-color-bg-tertiary: #f0f5f4;
  --gl-color-border: rgba(16, 42, 44, 0.08);
  --gl-color-border-strong: rgba(16, 42, 44, 0.12);
  --gl-color-divider: rgba(16, 42, 44, 0.06);

  /* 字体 */
  --gl-font-family: "IBM Plex Sans", "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  --gl-font-mono: "IBM Plex Mono", "SFMono-Regular", "JetBrains Mono", monospace;
  --gl-font-size-xs: 12px;
  --gl-font-size-sm: 13px;
  --gl-font-size-base: 14px;
  --gl-font-size-lg: 16px;
  --gl-font-size-xl: 20px;
  --gl-font-size-2xl: 22px;
  --gl-font-size-3xl: 26px;

  /* 间距（8px 基准） */
  --gl-space-1: 4px;
  --gl-space-2: 8px;
  --gl-space-3: 12px;
  --gl-space-4: 16px;
  --gl-space-5: 20px;
  --gl-space-6: 24px;
  --gl-space-8: 32px;
  --gl-space-12: 48px;

  /* 圆角 */
  --gl-radius-sm: 6px;
  --gl-radius-md: 10px;
  --gl-radius-lg: 14px;
  --gl-radius-xl: 18px;
  --gl-radius-full: 999px;

  /* 阴影 */
  --gl-shadow-sm: 0 1px 3px rgba(12, 48, 50, 0.06);
  --gl-shadow-md: 0 4px 16px rgba(12, 48, 50, 0.08);
  --gl-shadow-lg: 0 22px 60px rgba(12, 48, 50, 0.08);

  /* 动画 */
  --gl-duration-fast: 150ms;
  --gl-duration-normal: 250ms;
  --gl-duration-slow: 400ms;
  --gl-easing-standard: cubic-bezier(0.4, 0, 0.2, 1);
  --gl-easing-decelerated: cubic-bezier(0, 0, 0.2, 1);
  --gl-easing-accelerated: cubic-bezier(0.4, 0, 1, 1);
}

/* ============== 暗色主题 ============== */
.gl-theme-dark {
  --gl-color-primary: #0d9b8a;
  --gl-color-primary-hover: #0fae9b;
  --gl-color-primary-active: #12c2ae;
  --gl-color-primary-bg: color-mix(in srgb, #0d9b8a 12%, transparent);

  --gl-color-text: #e8f4f2;
  --gl-color-text-secondary: rgba(232, 244, 242, 0.65);
  --gl-color-text-tertiary: #8ba8a4;
  --gl-color-bg: #0f1f1e;
  --gl-color-bg-elevated: #1a2a28;
  --gl-color-bg-tertiary: #152524;
  --gl-color-border: rgba(232, 244, 242, 0.08);
  --gl-color-border-strong: rgba(232, 244, 242, 0.14);
  --gl-color-divider: rgba(232, 244, 242, 0.06);
}

/* reduced-motion 全局适配 */
@media (prefers-reduced-motion: reduce) {
  :root {
    --gl-duration-fast: 0.01ms;
    --gl-duration-normal: 0.01ms;
    --gl-duration-slow: 0.01ms;
  }
}
```

#### 1.2 antd ConfigProvider.token 映射

`--gl-*` CSS 变量与 antd 6 ConfigProvider 的映射由 `web/src/features/theme/OrganizationThemeProvider.tsx` 承载：

- 基线 palette 来自 `BRAND_PALETTE`（亮/暗两套），算法按模式选择 `defaultAlgorithm` / `darkAlgorithm`；
- 组织主题 token 经 `sanitizeOrganizationTokens` 白名单净化后覆盖 seed/组件 token，避免坏配置破坏全局；表面/文字类覆盖只在浅色模式生效；
- `colorLink` 等 seed token 在 antd 6 会被色板吸附，精确值经组件级 token 在算法之后合并；
- 开启 antd 原生 CSS 变量（`cssVar`）并关闭 hash，与 `--gl-*` 令牌体系共存。

token/组件映射明细以 Provider 源码为唯一事实源（设计令牌镜像门禁 `python3 scripts/check-token-parity.py`），本文不复制取值清单。

#### 1.3 主题切换机制

```
用户偏好（localStorage key: gl-theme）
    │
    ├── 'light'  →  <html class="gl-theme-light">
    ├── 'dark'   →  <html class="gl-theme-dark">
    └── null     →  跟随 prefers-color-scheme 媒体查询

OrganizationThemeProvider（ConfigProvider theme）
    │
    ├── 亮/暗模式由主题偏好或 media query 决定，选择对应算法与 BRAND_PALETTE
    └── 组织主题 token 白名单覆盖 → 更新 antd token 与 CSS 变量
```

---

### 2. 组件设计规范

#### 2.1 GlFormRow — 自动桥接 antd Form.Item validation

**问题**：通用组件库的 FormRow 封装通常只做展示，与 antd Form.Item 的 validation 状态隔离——开发者要手动写 Form.Item + 错误展示 + shake 动画，重复劳动。

**GuliangIAM 方案**：GlFormRow 消费 antd Form.Item 的 Context（`Form.Item.useFormItemStatus()`），自动联动：

```
GlFormRow 自动桥接 antd Form.Item 状态
┌─────────────────────────────────────────────┐
│  Form.Item (name="email")                   │
│  ├── GlFormRow                              │
│  │   ├── 自动读取 Form.Item.context.meta    │
│  │   │   ├── errors[] → 行级 shake + 错误色 │
│  │   │   ├── touched → border-color 变化    │
│  │   │   └── validating → 显示 loading 点   │
│  │   ├── label（支持 i18n key）             │
│  │   ├── extra（帮助文案）                  │
│  │   └── feedback（自动渲染 errors[0]）    │
│  └── Input                                  │
└─────────────────────────────────────────────┘
```

**API 定义**：

```typescript
// web/src/shared/ui/GlForm/GlFormRow.tsx

interface GlFormRowProps {
  /** 表单行标签 */
  label?: React.ReactNode;
  /** 帮助文案（Form.Item.extra 语义） */
  extra?: React.ReactNode;
  /** 是否为必填（自动渲染红色星号） */
  required?: boolean;
  /** 行内表单（inline 模式下标签在左侧） */
  inline?: boolean;
  /** 列宽（GlFormGrid 容器中使用） */
  colSpan?: number;
  /** 子元素：位于 Form.Item 内部，通常是单个 antd 控件 */
  children: React.ReactNode;
}

// 关键 1：Form.Item 通过 cloneElement 注入 value/onChange/id/aria-*，
// GlFormRow 必须把控制属性继续透传给内部控件。
// 关键 2：错误文案由 antd Form.Item 的 explain 单点渲染，GlFormRow 只做
// 视觉桥接（红 label / shake / validating），不重复输出 errors[0]。
export const GlFormRow: React.FC<GlFormRowProps & Record<string, unknown>> = ({
  label, required, inline, colSpan, extra, children, ...controlProps
}) => {
  const itemStatus = Form.Item.useStatus();
  const showError = (itemStatus?.errors?.length ?? 0) > 0;
  const isValidating = itemStatus?.status === "validating";
  const fieldId = typeof controlProps.id === "string" ? controlProps.id : undefined;

  // 透传 id/value/onChange：否则输入不会写回 Form 实例，label 也失去 for 关联。
  const control = React.isValidElement(children)
    ? React.cloneElement(children, controlProps)
    : children;

  return (
    <div className={clsx("gl-form-row", { "gl-form-row--error": showError, "gl-form-row--validating": isValidating, "gl-form-row--inline": inline })}>
      {label && (
        <label className={clsx("gl-form-row__label", required && "gl-form-row__label-required")} htmlFor={fieldId}>
          {label}
        </label>
      )}
      <div className="gl-form-row__control">{control}</div>
      {extra && <div className="gl-form-row__extra" id={fieldId ? `${fieldId}_extra` : undefined}>{extra}</div>}
    </div>
  );
};
```

> 契约（与 `GlFormRow.test.tsx` 同源）：`Form.Item > GlFormRow > 单个 antd 控件` 是唯一受支持写法；行内错误提示只允许一个来源（antd explain），GlFormRow 不再渲染第二份文案。

**样式片段**：

```css
.gl-form-row--error .gl-form-row__label { color: var(--gl-color-error); }
.gl-form-row--error .gl-form-row__control { animation: gl-shake var(--gl-duration-fast); }
.gl-form-row__error { color: var(--gl-color-error); font-size: var(--gl-font-size-xs); margin-top: 2px; }

@keyframes gl-shake {
  0%, 100% { transform: translateX(0); }
  20%, 60% { transform: translateX(-4px); }
  40%, 80% { transform: translateX(4px); }
}
```

#### 2.2 GlFormGrid — 响应式网格容器

**问题**：通用组件库的 FormGrid 通常是简单 CSS Grid 包装，没有断点感知。

**GuliangIAM 方案**：

```typescript
interface GlFormGridProps {
  /** 列数 - 支持响应式：默认 / md / lg */
  columns?: number | { default?: number; md?: number; lg?: number };
  /** 间距 - 支持响应式 */
  gap?: string | { default?: string; md?: string; lg?: string };
  /** 子元素必须是 GlFormRow */
  children: React.ReactNode;
}

// 断点映射
const BREAKPOINTS = {
  md: 768,
  lg: 1024,
};
```

**CSS Grid 实现**：

```css
.gl-form-grid {
  display: grid;
  gap: var(--gl-space-4) var(--gl-space-6);
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}

@media (min-width: 768px) {
  .gl-form-grid[data-columns-md="2"] { grid-template-columns: repeat(2, 1fr); }
}
@media (min-width: 1024px) {
  .gl-form-grid[data-columns-lg="3"] { grid-template-columns: repeat(3, 1fr); }
}
```

#### 2.3 GlEditShell — 草稿自动保存

**问题**：通用 EditPageShell 封装只负责页面头部 + 表单容器 + 底部操作条，没有自动保存，用户误关浏览器会丢失所有输入。

**GuliangIAM 方案**：GlEditShell 内置 `useGlDraft` hook，监听 antd Form 的 `onValuesChange`，debounce 2 秒后持久化到 localStorage。页面再次进入时自动恢复。

```
GlEditShell 内部结构
┌─────────────────────────────────────────────────┐
│  GlPageHeader（继承）                            │
│  ├── back button                                │
│  ├── title (i18n)                               │
│  ├── breadcrumb                                 │
│  └── GlSaveIndicator ●（草稿状态指示）            │
│                                                  │
│  ┌────────────────────────────────────────────┐ │
│  │  <Form> 或 <GlFormGrid>                    │ │
│  │  │  useGlDraft(form, draftKey, opts)      │ │
│  │  │  ├── onValuesChange → debounce 2s      │ │
│  │  │  ├── localStorage.setItem(draftKey)     │ │
│  │  │  ├── mounted 时 → 自动 fill form        │ │
│  │  │  └── unmount 时 → 弹确认（dirty check） │ │
│  │  └─────────────────────────────────────────│ │
│  └────────────────────────────────────────────┘ │
│                                                  │
│  GlEditFooter（底部操作条）                       │
│  ├── 左侧：已保存时间 / 未保存提示                │
│  └── 右侧：取消 / 保存 / 保存并继续               │
└─────────────────────────────────────────────────┘
```

**useGlDraft hook 实现**：

```typescript
// web/src/shared/hooks/useGlDraft.ts

interface DraftOptions {
  /** localStorage key */
  draftKey: string;
  /** debounce 毫秒 */
  debounceMs?: number; // 默认 2000
  /** 是否在 mounted 时自动恢复 */
  autoRestore?: boolean; // 默认 true
  /** 数据版本号（schema 变更时 bump，旧草稿丢弃） */
  version?: number; // 默认 1
}

interface DraftState {
  /** 当前是否有未保存草稿 */
  hasDraft: boolean;
  /** 最后保存时间戳 */
  lastSavedAt: number | null;
  /** 草稿值（用于 compare） */
  draftValues: Record<string, unknown> | null;
  /** 清除草稿 */
  clearDraft: () => void;
}

export function useGlDraft(
  form: FormInstance,
  opts: DraftOptions
): DraftState {
  const { draftKey, debounceMs = 2000, autoRestore = true, version = 1 } = opts;

  // 恢复草稿
  useEffect(() => {
    if (!autoRestore) return;
    const raw = localStorage.getItem(draftKey);
    if (!raw) return;
    try {
      const parsed = JSON.parse(raw);
      if (parsed.version === version && parsed.values) {
        form.setFieldsValue(parsed.values);
        // 通知用户
        message.info(t('form.draft.restored', '已自动恢复上次编辑内容'));
      }
    } catch { /* 损坏 → 忽略 */ }
  }, [draftKey, version]);

  // 自动保存（debounced）
  const debouncedSave = useMemo(
    () => debounce((values) => {
      localStorage.setItem(draftKey, JSON.stringify({
        version,
        values,
        savedAt: Date.now(),
      }));
      setLastSavedAt(Date.now());
    }, debounceMs),
    [draftKey, debounceMs, version]
  );

  // 监听 Form 变更
  useEffect(() => {
    const unsubscribe = form.subscribe(({ values }) => {
      debouncedSave(values);
      setDraftValues(values);
    });
    return unsubscribe;
  }, [form, debouncedSave]);

  // 离开确认（beforeunload）
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (form.isFieldsTouched()) {
        e.preventDefault();
        e.returnValue = ''; // 触发浏览器原生确认
      }
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [form]);

  // 提交后清除草稿
  const clearDraft = useCallback(() => {
    localStorage.removeItem(draftKey);
    setDraftValues(null);
    setLastSavedAt(null);
  }, [draftKey]);

  return {
    hasDraft: !!(draftValues && form.isFieldsTouched()),
    lastSavedAt,
    draftValues,
    clearDraft,
  };
}
```

**GlEditShell API**：

```typescript
interface GlEditShellProps {
  /** 页面标题（i18n key） */
  title: string;
  /** 标题命名空间 */
  titleNs?: string;
  /** 当前实体 ID（用于生成 draftKey） */
  entityId?: string;
  /** 草稿键前缀 */
  draftScope?: string; // 默认 'edit'
  /** 是否显示返回按钮 */
  showBack?: boolean;
  /** 自定义底部 footer */
  footer?: React.ReactNode;
  children: React.ReactNode;
}

// 使用示例
<GlEditShell
  title="users.edit.title"
  entityId={userId}
  draftScope="user-edit"
  showBack
>
  <Form form={form} layout="vertical">
    <GlFormGrid>
      <GlFormRow label="users.fields.name" required><Form.Item name="name"><Input /></Form.Item></GlFormRow>
      <GlFormRow label="users.fields.email" required><Form.Item name="email"><Input /></Form.Item></GlFormRow>
    </GlFormGrid>
  </Form>
</GlEditShell>
```

#### 2.4 GlPageHeader — 继承 antd PageHeader + GuliangIAM 语义

```typescript
interface GlPageHeaderProps {
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  breadcrumbItems?: BreadcrumbItem[];
  /** 返回按钮回调 */
  onBack?: () => void;
  /** 右侧操作区 */
  extra?: React.ReactNode;
  /** 右侧操作区是否有危险操作 → 自动渲染 DangerPopconfirm */
  hasDangerAction?: boolean;
}

// GlPageHeader 与 antd PageHeader 差异：
// 1. 默认带面包屑（从 react-router 自动解析）
// 2. title 自动支持 i18n key
// 3. hasDangerAction=true 时 extra 区自动包一层 Popconfirm + IconWarning
// 4. sticky 定位（滚动时固定在顶部）
```

**CSS**：

```css
.gl-page-header {
  position: sticky;
  top: 0;
  z-index: calc(var(--gl-z-dropdown, 1000) - 100);
  background: var(--gl-color-bg-elevated);
  border-bottom: 1px solid var(--gl-color-border);
  padding: var(--gl-space-3) var(--gl-space-4);
  backdrop-filter: saturate(180%) blur(8px);
}
```

#### 2.5 ResourceListCard — 表格增强（虚拟滚动 + 三列菜单）

**问题**：通用 Table 封装通常只是 antd Table 的直接透传，没有列级菜单、搜索、过滤，大数据量也没有虚拟滚动。

**GuliangIAM 方案**：ResourceListCard 封装 antd Table + @tanstack/react-virtual，提供 ColumnMenu / ColumnSearch / ColumnFilter 三列增强，以及自动虚拟滚动。

```
ResourceListCard 结构
┌──────────────────────────────────────────────────┐
│  Toolbar（与 antd Table.Toolbar 集成）             │
│  ├── GlSearchInput（全局搜索）                     │
│  ├── ColumnSearch（列级搜索：按列 type 自动匹配）    │
│  ├── ColumnFilter（列级过滤：枚举值 / 范围 / 自定义）│
│  ├── ColumnMenu（列级菜单：显隐 / 冻结 / 排序）      │
│  └── DensityToggle（紧凑 / 默认 / 宽松行高）         │
├──────────────────────────────────────────────────┤
│                                                  │
│  antd Table（超过 VIRTUAL_ROW_THRESHOLD 自动虚拟模式）│
│  ├── 虚拟模式：@tanstack/react-virtual            │
│  ├── 非虚拟模式：标准 antd Table                   │
│  └── ColumnRender：自动应用 ColumnMenu 设置         │
│                                                  │
├──────────────────────────────────────────────────┤
│  Pagination（antd Pagination）                     │
│  └── 每页选项：20 / 50 / 100 / 200 / 500          │
└──────────────────────────────────────────────────┘
```

**虚拟滚动判定逻辑**：行数超过 `VIRTUAL_ROW_THRESHOLD` 时启用 `@tanstack/react-virtual` 虚拟模式，否则走标准 antd Table。阈值与行高/overscan 参数以 `web/src/shared/ui/ResourceListCard/ResourceListCard.tsx` 为唯一事实源，本文不复制数值。

**ColumnMenu**：

```typescript
// 列级菜单（从表头下拉触发）
interface ColumnMenuConfig {
  /** 显隐控制 */
  visible: boolean;
  /** 固定位置 */
  fixed: 'left' | 'right' | null;
  /** 排序优先级 */
  order: number;
}

// ColumnMenu 点击后持久化到 localStorage（key: gl:col-pref:{tableId}）
```

**ColumnSearch**（按列 type 自动匹配）：

| 列 type | 搜索控件 | 匹配模式 |
|--------|---------|---------|
| `text` / `string` | Input | 子串包含（case-insensitive） |
| `email` | Input | 精确 + 子串 |
| `uuid` | Input | 精确匹配 |
| `number` / `integer` | InputNumber | 范围（min~max） |
| `boolean` | Select[true/false] | 精确 |
| `enum` | Select 多选 | IN 集合 |
| `date` / `datetime` | DatePicker.RangePicker | between |
| `url` | Input | 子串 |

**ColumnFilter**（列头悬浮图标）：

```
┌──────────────┐
│ Status  ⬇ ⊘ │  ← 列头：降序 + 过滤器图标
└──────────────┘

点击 ⊘ 弹出 Filter 面板：
┌─────────────────────────┐
│ Filter: Status          │
│ ┌─────────────────────┐ │
│ │ 🔘 Active   (123)   │ │
│ │ 🔘 Inactive (45)    │ │
│ │ 🔘 Pending  (12)    │ │
│ └─────────────────────┘ │
│ [Clear]  [Apply]        │
└─────────────────────────┘
```

---

### 3. Layout 规范

#### 3.1 AppShell 结构

```
┌─────────────────────────────────────────────────────────────────┐
│  AppShell (web/src/app/layout/AppShell.tsx)                    │
│                                                                  │
│  ┌──────────┐  ┌─────────────────────────────────────────────┐ │
│  │ Sidebar  │  │  TopBar                                     │ │
│  │          │  │  ├── ⌘K Search / Command Palette 入口      │ │
│  │ rail 模式│  │  ├── 当前组织切换                            │ │
│  │ (窄) ──▶ │  │  ├── 通知 Bell                              │ │
│  │ (宽)展开 │  │  └── UserMenu (Profile / MFA / Logout)     │ │
│  │          │  ├─────────────────────────────────────────────┤ │
│  │ Menu     │  │  MainContent                                │ │
│  │ (antd)   │  │  ├── Suspense fallback（骨架屏）             │ │
│  │          │  │  ├── GlPageHeader（sticky）                  │ │
│  │          │  │  └── <Outlet> / 路由组件                    │ │
│  └──────────┘  └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

#### 3.2 Sidebar Rail + 展开模式

```css
/* rail 模式：48px 宽，仅图标 */
.gl-sidebar[data-mode="rail"] {
  width: 48px;
  transition: width var(--gl-duration-normal) var(--gl-easing-standard);
}

/* 展开模式：240px 宽，图标 + 文字 */
.gl-sidebar[data-mode="expanded"] { width: 240px; }

/* 折叠动画 */
.gl-sidebar-rail-trigger {
  position: absolute;
  top: var(--gl-space-2);
  right: -12px;
  width: 24px;
  height: 24px;
  background: var(--gl-color-bg-elevated);
  border: 1px solid var(--gl-color-border);
  border-radius: 50%;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--gl-shadow-sm);
  z-index: 10;
}
```

#### 3.3 Suspense Fallback

**路由级 Suspense**：每个路由组件用 React.lazy 动态导入，AppShell 用 Suspense 包裹 Outlet：

```typescript
// web/src/app/layout/AppShell.tsx

const UsersPage = lazy(() => import('@/pages/idam/users/UsersPage'));
const RolesPage = lazy(() => import('@/pages/idam/roles/RolesPage'));

// Suspense fallback：与 antd Skeleton 集成
const PageSkeleton = () => (
  <div className="gl-page-skeleton">
    <Skeleton active avatar={{ shape: 'square', size: 40 }} paragraph={{ rows: 2 }} />
    <div className="gl-page-skeleton__body">
      <Skeleton active avatar={false} paragraph={{ rows: 8, width: ['100%', '90%', '95%', '85%', '100%', '80%', '90%', '100%'] }} />
    </div>
  </div>
);
```

#### 3.4 Command Palette（⌘K）

**全局快捷键**：`document.addEventListener('keydown', (e) => (e.metaKey || e.ctrlKey) && e.key === 'k' → openCommandPalette())`

**WAI-ARIA combobox 模式**：

```html
<div role="dialog" aria-modal="true" aria-label="Command Palette">
  <input
    role="combobox"
    aria-expanded="true"
    aria-controls="command-listbox"
    aria-activedescendant="cmd-3"
    placeholder="搜索资源、跳转到页面..."
    autofocus
  />
  <ul role="listbox" id="command-listbox">
    <li role="option" id="cmd-1">🔍 搜索用户: admin@example.com</li>
    <li role="option" id="cmd-2">↗️ 跳转到: 角色管理</li>
    <li role="option" id="cmd-3" aria-selected="true">🛠️ 执行: 创建新应用</li>
  </ul>
</div>
```

**搜索范围**：

| 类型 | 数据源 | 延迟 | 示例 |
|------|--------|------|------|
| 全局导航 | 静态路由配置 | 0ms | "角色管理" → navigate('/iam/roles') |
| 资源搜索 | react-query 缓存 + 按需 API | 200ms debounce | "admin" → 匹配 users |
| 快速操作 | 预定义命令列表 | 0ms | "新建应用" → 打开 Drawer |

**键盘导航**：

| 按键 | 行为 |
|------|------|
| ↑/↓ | 在 listbox 中移动选项 |
| Enter | 选中当前项 |
| Escape | 关闭对话框 |
| / | 直接聚焦搜索框（如果聚焦在别处） |

---

### 4. Micro-interaction 规范

#### 4.1 加载状态层级

| 层级 | 时机 | 组件 | 持续 |
|------|------|------|------|
| Skeleton | 首屏路由加载（React.lazy） | `<Skeleton active />` | 直到组件 ready |
| Spin | 按钮/表单提交中 | `<Button loading />` | 请求完成 |
| Progress | 文件上传 | antd Progress | 上传完成 |
| Global | 长耗时全局操作 | antd message.loading | 30s 超时 |

#### 4.2 Toast / Notification 使用规范

```
antd message vs. notification

message  → 轻量级反馈，自动消失，短消息
  ✓ 保存成功
  ✓ 草稿已自动恢复
  ✓ 操作已排队

notification → 重要事件，可手动关闭，长消息或带 actions
  ✓ 跨租户 Scope 审批通过（含审批人、时间）
  ✓ Passkey 登录成功（含设备信息）
  ✓ 安全告警（SignCount 回退检测）
```

#### 4.3 危险操作反馈

```
危险操作链：
Click → Popconfirm（"确认删除？此操作不可撤销"）
     → 二次确认（"⚠️ 即将删除 5 个用户，输入 DELETE 确认"）
     → 执行 → Spin(按钮loading) → 成功 toast + 可 Undo 3s
     → 失败 → notification.error + 错误详情
```

#### 4.4 空状态

```
三种空状态模板：

1. 首次使用：引导 CTA
   ┌────────────────────────────┐
   │  🎉 欢迎使用 GuliangIAM     │
   │                              │
   │  您还没有任何应用，           │
   │  点击下方按钮创建第一个      │
   │                              │
   │  [创建应用]                  │
   └────────────────────────────┘

2. 无搜索结果：调整搜索
   ┌────────────────────────────┐
   │  🔍 没有找到匹配 "{query}"  │
   │                              │
   │  • 尝试缩短搜索词            │
   │  • 检查拼写                  │
   │  • 使用不同的筛选条件        │
   │                              │
   │  [清除筛选]                  │
   └────────────────────────────┘

3. 权限受限：申请访问
   ┌────────────────────────────┐
   │  🔒 您没有此资源的访问权限   │
   │                              │
   │  需要 roles:read 或更高权限  │
   │  联系管理员申请              │
   │                              │
   │  [申请权限]                  │
   └────────────────────────────┘
```

---

### 5. A11y 规范

#### 5.1 与已有文档的关系

本规范不覆盖独立的无障碍文件，只规定 **antd 集成层的可访问性**；完整的 WCAG 2.1 AA 合规检查清单（基线版本以《GuliangIAM-开发规范》无障碍章节为准）已并入本文件的可访问性章节。

#### 5.2 antd 6 内置 a11y 能力的强制使用

| antd 组件 | 强制要求 | 检查方法 |
|-----------|---------|---------|
| Button | `aria-label` 仅图标按钮必填 | ESLint `jsx-a11y/label-has-associated-control` |
| Modal | `aria-modal="true"`（antd 默认启用） | axe-core |
| Form.Item | `aria-required` 由 `required` prop 自动注入 | antd 内置 |
| Input | `aria-invalid` 当 `status="error"` 时自动设为 true | antd 内置 |
| Table | Row `aria-selected` + `aria-rowindex` | antd Table 虚拟滚动时自动处理 |
| Menu | `role="menubar"` + `role="menuitem"` 层级正确 | antd 内置 |
| Tooltip | `aria-describedby` 关联 | antd 内置 |

#### 5.3 焦点管理

```
焦点陷阱（Modal/Drawer）：
  antd Modal/Drawer 默认启用 focus trap（focus 自动循环）
  确认：Tab / Shift+Tab 不会逃逸到背景页面

首屏焦点：
  路由切换后，焦点移至页面主标题（<h1 id="page-title" tabindex="-1">）
  document.getElementById('page-title')?.focus()

键盘快捷键冲突检测：
  ⌘K / Ctrl+K → Command Palette（全局唯一注册）
  / → 搜索聚焦（当聚焦在可输入元素时不触发）
  Esc → 关闭所有打开层（Modal → Drawer → Tooltip → 全局）
```

#### 5.4 颜色对比度

```
对比度检查（GuliangIAM 亮色主题）：

前景色 --gl-color-text (#102a2c) vs 背景 --gl-color-bg-elevated (#ffffff):
  WCAG AA: 4.5:1 ✅ 实际 16.3:1

前景色 --gl-color-text-secondary (rgba(16,42,44,0.66)) vs 背景 (#ffffff):
  WCAG AA: 4.5:1 ✅ 实际 5.1:1

前景色 --gl-color-error (#cf2e2e) vs 背景 (#ffffff):
  WCAG AA: 4.5:1 ✅ 实际 5.6:1

品牌主色 --gl-color-primary (#0b8073) vs 白色背景:
  WCAG AA: 4.5:1 ✅ 实际 6.2:1

品牌主色（按钮背景）--gl-color-primary (#0b8073) vs 白色文字 (#ffffff):
  WCAG AA: 4.5:1 ✅ 实际 6.2:1
```

**组织主题覆盖的运行时守卫**：组织 token 白名单内的 `colorError` 是危险链接/文字按钮的前景种子，`OrganizationThemeProvider` 注入 antd 前按 WCAG AA 收敛——浅色向黑压暗（落底取容器底与斑马纹底中更暗者）、暗黑向白提亮（基准最浅 elevated `#2a2a2a`，与暗黑链接色同口径）；解析失败的颜色（命名色/hsl）回落品牌错误色。

**验证工具**：`pnpm e2e:a11y`（Playwright + @axe-core/playwright）运行 `web/e2e/a11y.spec.ts`

#### 5.5 Screen Reader 测试

| 屏幕阅读器 | 浏览器 | 测试内容 |
|-----------|--------|---------|
| VoiceOver | Safari (macOS/iOS) | 主流程导航、表单填写、Modal 操作 |
| NVDA | Firefox (Windows) | 表格虚拟滚动、复杂 Form 提交 |
| TalkBack | Chrome (Android) | 移动端响应式 + Touch 手势 |

---

### 6. Icon 规范

#### 6.1 依赖确认

**当前 package.json**（web/package.json）：

```json
"@ant-design/icons": "^6.3.4"
```

✅ 确认所有 @ant-design/icons 导入均存在于 v6.x API。

#### 6.2 使用规则

| 规则 | 说明 |
|------|------|
| **始终使用 `@ant-design/icons`** | 不引入 Lucide/Feather 等额外图标库；GuliangIAM 品牌一致性优先于个人偏好 |
| **不内联 SVG** | 除非是复杂的自定义品牌图形（此时走 SVG 文件 + 组件化封装） |
| **尺寸 Token** | 使用 `fontSize: var(--gl-font-size-base)` (14px) 作为默认，不硬编码数字 |
| **颜色 Token** | 使用 `color: var(--gl-color-*)` CSS 变量，不使用硬编码 HEX |
| **语义优先** | 图标必须有 accompanying label 或 `aria-label`；纯装饰图标加 `aria-hidden="true"` |

#### 6.3 常用图标映射表

| 语义 | 图标组件 | 场景 |
|------|---------|------|
| 返回 | `<ArrowLeftOutlined />` | GlPageHeader 返回按钮 |
| 保存 | `<SaveOutlined />` | GlEditShell 底部操作 |
| 搜索 | `<SearchOutlined />` | ColumnSearch / GlSearchInput |
| 过滤器 | `<FilterOutlined />` | ColumnFilter 列头 |
| 更多 | `<DownOutlined />` | ColumnMenu 触发 |
| 危险 | `<WarningOutlined />` | DangerPopconfirm / error 状态 |
| 权限锁定 | `<SafetyOutlined />` | ScopeAuthorizer / MFA |
| 跨租户 | `<TeamOutlined />` | 平台管理员跨租户操作 |
| 草稿 | `<FileTextOutlined />` | GlSaveIndicator |
| Command Palette | `<CommandOutlined />` | ⌘K 入口 |
| 组织 | `<BankOutlined />` | Organization 切换 |
| 用户 | `<UserOutlined />` | UserMenu / users 列表 |
| 角色 | `<IdcardOutlined />` | Roles 页面 |
| 应用 | `<AppstoreOutlined />` | Applications 页面 |
| 审计 | `<FileSearchOutlined />` | 审计日志入口 |
| MFA | `<LockOutlined />` | Passkey / MFA 强制 |

#### 6.4 自定义图标（必要时）

当 @ant-design/icons 无法覆盖时，使用 SVG + React 组件封装，保持与 antd 图标 API 一致：

```typescript
// 组件放在 web/src/shared/ui/ 下，与现有共享组件同目录
import React from 'react';

export const GlCrossTenantIcon: React.FC<React.SVGProps<SVGSVGElement>> = (props) => (
  <svg viewBox="0 0 24 24" width="1em" height="1em" fill="currentColor" {...props}>
    <path d="..." />
  </svg>
);
```

---
