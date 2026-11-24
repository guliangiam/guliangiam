# 登录认证流程

> 设计锚点：[DESIGN:auth/login]

> 目标态规范「登录认证流程」；写作约束见 [docs/README.md](../../README.md)。

---

## 登录流程调用链

标准用户名密码登录的四层调用链：

```
前端 → Transport → App → Domain
```

1. 前端 `POST /login`
2. Transport 层调用 `Login()`
3. App 层依次调用 `GetOrg()` → `GetUser()` → `VerifyPwd()`
4. Domain 层凭据校验
5. 凭据校验通过但需 MFA 时，返回 `mfa_required`（Challenge 创建逻辑见《design/auth/mfa.md》）
6. MFA Challenge 通过后 → 创建会话（会话模型见《architecture/security-model.md》）

## 涉及的登录路径

| 路径 | 状态机/规格位置 |
|------|----------------|
| 用户名密码 | 本文件 §调用链 |
| QR Code 扫码 | 《design/auth/qrcode.md》 |
| WebAuthn/Passkey | 《design/auth/webauthn.md》 |
| OAuth2 授权码 | 《design/auth/oauth2.md》 + 《design/protocol/oauth2.md》§14.6 |
| SAML | 《design/auth/saml.md》 |
| Face ID | 《design/auth/faceid.md》 |
| 国家网络身份认证 | 《design/auth/network-identity.md》 |

## 国家网络身份认证

登录页的 NCI 方法、提供商配置卡片与实名入口由《[network-identity.md](network-identity.md)》定义；本文件不复制领域端口、表结构与数字信封细节。NCI 方法与密码登录并列展示，网关不可用时停留在本方法并展示 `unavailable`，不自动切换到密码。

## 会话 Cookie 属性

会话 Cookie 为 opaque 会话 ID + 仓库查行模式（非 JWT）。属性矩阵：

| Cookie | 属性 | 值 |
|--------|------|-----|
| `session` | HttpOnly | ✓ |
| | Secure | 跟随 `COOKIE_SECURE` |
| | SameSite | 安全模式 **Strict**；HTTP 开发模式 **Lax** |
| | Path | `/` |
| | Max-Age | 30d（与服务端 idle 超时/`expires_at` 是独立维度，服务端双超时为准，见《design/auth/webauthn.md》§双超时默认值） |
| | 名称 | 安全模式下 `__Host-session`（Path=/、无 Domain、Secure 三条件同时满足）；HTTP 开发模式下 `session_id` |
| `trusted_device` | HttpOnly | ✓ |
| | Secure | 跟随 `COOKIE_SECURE`（与 `session` 同一开关） |
| | SameSite | 继承会话策略：安全模式 **Strict**；HTTP 开发模式 **Lax** |
| | Path | `/api/v1/auth`（仅随认证端点携带） |
| | Max-Age | 30d（与 `trusted_devices.expires_at` 同值；服务端仍以库内到期/吊销为准） |
| | 名称 | 安全模式下 `__Secure-trusted_device`；HTTP 开发模式下 `trusted_device` |

信任设备 Cookie 由注册端点应答下发，浏览器只承担传递职责；登录端点**只从该 Cookie 读取**信任设备凭据（`TrustedDeviceCredentialFromRequest`），请求体不接受该凭据、无回退分支（Cookie-only 口径，OPEN-API-50）。`__Secure-` 前缀只要求 `Secure`，不要求 `Path=/` 或 `SameSite=Strict`；本 Cookie 的 Strict 与会话策略一致，Path 收窄到认证端点。

CSRF Cookie：安全模式下 `__Host-csrf`，HTTP 开发模式下 `csrf_token`；值为服务端生成的 256-bit 随机双提交令牌，不使用会话 HMAC。`COOKIE_SECURE` 未设置时 production 默认启用 Secure，其他环境默认关闭。

## 控制台登录的默认应用解析

控制台登录请求不带 `applicationId` 且无协议 `responseType` 时，按以下顺序解析并绑定会话应用：

1. 组织配置的 `organization_settings.default_application_id`；
2. 缺失时回退 `app-<organizationSlug>`；
3. 解析来源记录在 `default application resolved` 日志（`organization_default_application` / `legacy_app_slug`）。

带 `responseType` 的协议登录必须显式提供 `applicationId`，缺省返回 400 `invalid_argument`。会话表 `sessions.application_id` 为 NOT NULL 外键，解析失败不得创建匿名/空应用会话。

## 控制台实时事件流（SSE）

> **规划态章节**：控制台会话/审计事件流为目标态能力，运行期事件流端点以路由声明为准（复现：`grep -rn 'text/event-stream' internal/transport/`，仅 AI 网关流式响应命中）；本节协议选型决策先行固化。

协议选型：**Server-Sent Events (SSE)**，不采用 WebSocket。IAM 实时需求以单向推送为主（会话事件、审计通知、风险告警），SSE 复用 HTTP 认证与 CORS 且客户端自带断线重连；仅当出现真实双向需求（如交互式权限决策）时再按需引入 WebSocket。

稳定契约规则：

| 维度 | 规则 |
|------|------|
| 传输 | `Content-Type: text/event-stream`，事件类别以 `event:` 字段区分 |
| 认证 | 与同面 API 一致；令牌不得置于 URL 查询串（避免日志与 Referer 泄露） |
| 租户隔离 | 事件流受 RLS 约束，仅推送当前租户可见事件 |
| 续传 | 事件携带 `id:`，客户端以 `Last-Event-ID` 从版本点续传 |
| 心跳 | 空闲期定期发送 `event: ping`，防止中间设备超时断链 |
| 过滤 | 按事件类型的过滤与权限校验都在服务端执行，客户端过滤不可作为授权手段 |

**事实源**：具体路径、方法与事件名一律以 `internal/transport/http/**/routes.go` 的 `route.Definition` 声明为准，并由 `python3 scripts/gen_openapi.py` 生成到 `openapi/openapi.yaml`；本节刻意不列举端点清单，避免与生成物构成第二事实源。

## CSRF 双提交与豁免边界

- 安全方法补发 cookie；mutating 方法校验 `X-CSRF-Token` 头 == cookie 值
- 仅当 `Authorization: Bearer` 或 `X-Access-Token` 中的 JWT 已被 `SessionContextMiddleware` 验签并建立请求头会话时才豁免；原始请求头本身、无效令牌和 Cookie 会话仍必须校验双提交令牌
- 客户端认证协议端点按 `auth_paths.go` 的 `clientAuthenticatedProtocolPaths` **精确 POST 清单**豁免（与 PEP 匿名门共用同一谓词，单一事实源）；SAML SP ACS 走保护链外公开路由、CAS 校验端点为安全方法，均不属于「豁免登记」。完整四分类见《design/auth/webauthn.md》§CSRF 豁免边界

## 强制改密门禁（Password Gate）

- 档位：`AUTH_PASSWORD_CHANGE_ENFORCEMENT=off|warn|enforce`，**未设置或非法值一律回落 `enforce`**（`DefaultPasswordGateMode`）；三处默认值（`contracts/registries/env.yaml`、`.env.example`、`docs/generated/env-registry.md`）必须同进同退。
- 位置：`SessionContext` → `TenantResolution` → 路由级限流 → **PasswordGate** → `APIAuthorization` → `ScopeAuthorizer` → `TenantFilter`。门禁必须早于授权：gated 用户的 landing 接口通常没有权限，若先走 Casbin 只会得到 `insufficient permissions`，`password_change_required` 永远到不了客户端。门禁自身需要的 RLS 租户作用域由 `PasswordGate` 用会话组织钉住（`tenantctx`），不依赖 `TenantFilter` 先执行。
- 两条**相互独立**的自救通道（去掉任一条，另一条仍成立）：
  1. 服务端：任意非豁免请求返回 403 `password_change_required`/`password_expired`（`details.changePasswordUrl=/account`），非浏览器客户端与旧版前端同样可用；
  2. 前端：登录响应/`GET /api/v1/auth/account` 的 `needUpdatePassword` 为真时首屏直达 `/account`，不依赖先拿到 403。403 → event → `navigate` 保留为兜底。
- 豁免路由以 `contracts/registries/password-change-exempt-routes.yaml` 为准（生成 `password_change_exempt_gen.go`）；**产品裁定：不因 `/account` 页签 403 扩豁免表**。

### 预期行为（非缺陷，不接受按缺陷重报）

1. enforce 下 `/account` 仅“改密”页签可用；其余页签的接口会被门禁拦截（403 `password_change_required`），改密成功后其余页签恢复。
2. gated 用户访问不存在的受保护路径得到 **403 而非 404**：路由存在性不对 gated 用户暴露，方向是收紧；仅影响 gated 用户，非 gated 用户的 404 语义不变。
3. gated 用户的授权拒绝审计事件（`EventAuthzDeny`）不再产生——请求在到达授权中间件之前已被门禁拦下。门禁自身以结构化日志记录拦截原因；`password.changed` 等状态变更审计不受影响。**结论：接受该代价，不需要约束方案**；若合规后续要求逐请求归因，新增专用门禁审计事件，而不是把门禁移回授权之后。
4. 带 gated 会话调用客户端认证协议端点（`POST /api/v1/auth/oauth2/revoke`、`POST /api/v1/auth/oauth2/introspect`）会被密码门禁挡成 403 `password_change_required`；无会话的标准 OAuth2 客户端走同一端点不受影响（门禁只对已建立的浏览器会话生效）。
5. `/api/v1/auth/me` 不在改密豁免表（豁免的是 `GET /api/v1/auth/account`），gated 下返回 403 `password_change_required`；前端按错误 `code` 跳转改密即可，不需为它单独加豁免。

### 密码策略与口令历史（PasswordHistoryCount）

- 字段：`organization_security_policies.password_policy` 的 JSONB 键 `passwordHistoryCount`（结构声明见 `contracts/schema/10_tenant.yaml`）；接口字段为 `passwordPolicy.passwordHistoryCount`（`PUT /api/v1/organizations/{id}/security-policy`，`UpdateOrganizationSecurityPolicy`）。
- 默认值：组织策略未配置 `passwordHistoryCount` 时为 `5`，未配置 `passwordMaxAgeDays` 时为 `90`，分别与非功能需求的安全基线一致（`internal/domain/organizations/password_policy.go`）。显式配置 `0` 仍分别表示关闭历史校验或关闭密码过期；`built-in` 种子显式设置历史回溯 `5`、过期 `0`（`db/seeds/V1__guliangiam_seed_init.sql`），该覆盖不改变其他组织的默认值。`DefaultPasswordHistoryCount` 在调用方传入 `<= 0` 时作为历史检查上限兜底。
- 配置入口：组织详情 → 密码策略 → “扩展 JSON”（`passwordPolicyExtraJson`，`web/src/pages/organizations/tabs/PasswordPolicyTab.tsx`），写入 `{"passwordHistoryCount": N}`；平台设置页的安全策略表单未单独暴露该字段。
- 语义：`0` = 不校验历史（“旧口令可复用”属该配置的定义行为，不是校验缺陷）。
- 可用性降级（fail-open）：历史表读取失败（迁移缺失、查询错误、数据库抖动）时**降级为不校验历史、照常放行改密**，同时 `password_history_fail_open_total` 计数器 +1 并输出 `event=password_history_fail_open` 结构化日志。口令历史是策略加分项、不是安全边界：enforce 下“改不了密码”等于把用户彻底锁死，最坏降级只是允许复用一次旧口令。
- 重置流写入口令，不写旧口令：管理员重置与找回口令拿不到旧明文，历史中记录的是**新口令**。因此“管理员重置后用户又把口令改回旧值”无法被历史拦截——这是已知取舍，端点描述不得声称可以拦截。

### 泄露口令检查（AUTH-009）

- 开关：`PASSWORD_BREACH_CHECK_ENABLED`（默认 `false`，进程不出域；拼错的布尔值在启动校验失败）。基地址 `PASSWORD_BREACH_CHECK_BASE_URL`（默认 `https://api.pwnedpasswords.com`，覆盖用于自建镜像/隔离测试）。
- 协议与隐私边界：HIBP k-匿名 range 查询——本地计算 SHA-1（大写 hex），**仅前 5 位前缀出域**，明文与完整哈希不离开进程；请求带 `Add-Padding: true`，填充行（count=0）不判命中；响应体上限 2MiB、超时固定 2s。
- 位置：自助改密、注册、找回/重置三条路径均在**事务之外**执行（出域 HTTP 不得持数据库连接）。改密路径位于策略与历史校验之后；注册与找回路径位于策略校验之后（历史校验在各自写事务内完成，事务内的二次策略校验不重复出域）。找回路径在**用户解析之前**执行：策略/泄露 422 与用户存在性无关，防用户名枚举。命中泄露库返回 422。
- 失败姿态（fail-open，用户裁定）：range 查询或缓存故障时放行并输出结构化 Warn，同时在对应审计事件 payload 标记 `breachCheck=unverified`。泄露检查是策略加分项，不是可用性边界——与口令历史 fail-open 同一取舍方向。
- 缓存：range 响应（公共哈希后缀列表，按前缀键控，不含用户派生秘密）缓存 24h；复用 session-cache Redis 跨副本共享（键前缀 `guliangiam:breach:range:v1:`，登记 `short-lived-state.yaml` `breach_range_cache`），未配置 Redis 回退进程内缓存（容量 4096，启动 Warn 宣告不跨副本共享）。

## 登录页面前端架构（配置驱动 + 统一 Shell）

### 架构概览

登录页面的**方法可用性完全由配置驱动**（`applications.signin_items` JSONB + `providers` 表 + 浏览器能力检测），前端组件树只负责渲染当前启用的方法，不硬编码任何登录路径。

```
┌─────────────────────────────────────────────────────────────────┐
│ LoginShellLayout（统一外壳 + 应用级自定义渲染分支，见下节）       │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ LoginOverlaySection（品牌、语言切换、logo、版权）             │ │
│ └─────────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ LoginPanelSection（方法区；split 布局含 formSideHtml 侧栏）  │ │
│ │ ┌─────────────────────────────────────────────────────────┐ │ │
│ │ │ LoginSigninContent（方法 Tab + 动态方法面板）              │ │ │
│ │ │  ┌───────────────────────────────────────────────────┐ │ │ │
│ │ │  │ MethodTabs（配置驱动 + 浏览器能力检测禁用）           │ │ │ │
│ │ │  │ [密码] [验证码] [WebAuthn] [扫码] [Face ID] [LDAP]  │ │ │ │
│ │ │  └───────────────────────────────────────────────────┘ │ │ │
│ │ │  ┌───────────────────────────────────────────────────┐ │ │ │
│ │ │  │ MethodPanel（方法面板切换；XState 为目标态）           │ │ │ │
│ │ │  │  password          → LoginSigninForm                │ │ │ │
│ │ │  │  verificationCode  → 邮箱/手机验证码表单              │ │ │ │
│ │ │  │  webauthn          → Web3LoginPanel                  │ │ │ │
│ │ │  │  qrcode            → LoginQrcodeCard                 │ │ │ │
│ │ │  │  faceId            → FaceIdCard                      │ │ │ │
│ │ │  │  ldap              → LDAP 表单                        │ │ │ │
│ │ │  └───────────────────────────────────────────────────┘ │ │ │
│ │ └─────────────────────────────────────────────────────────┘ │ │
│ └─────────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ LoginAuxiliarySigninItems（第三方 Provider 区 + 忘记密码 +   │ │
│ │  注册链接 + CaptchaWidget + 协议法律条款模态）                │ │
│ └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 应用级登录页自定义渲染

`LoginShellLayout`（`web/src/pages/auth/LoginShellLayout.tsx`）消费应用级自定义字段（声明层 `contracts/schema/20_identity.yaml` 的 `signin_html` / `custom_css` / `form_css` / `form_css_mobile` / `form_side_html` / `header_html` / `footer_html` / `form_background_url_mobile` 等列），渲染规则：

- **整页覆盖优先级最高**：`customSigninHtml` 非空时经 `sanitizeHtml` 净化后替代整个登录页，其余装饰不再叠加。
- **CSS 叠加顺序**（同特异性后者覆盖前者）：`customCss`（整壳通用）→ `formCss`（桌面表单）→ `formCssMobile`（移动覆盖）；全部经 `sanitizeCss` 净化并加 `.login-content` 作用域，租户 CSS 不波及壳外（portal/body）。
- **净化边界**：HTML 走 `sanitizeHtml`、CSS 走 `sanitizeCss`（`web/src/shared/lib/dom-purify.ts`）；移动端背景图走内联样式（CSSOM 赋值，无 HTML 注入面），不经 `sanitizeCss` 的 url() 过滤。
- **headerHtml / footerHtml**：净化后注入壳顶/壳底。
- **split 布局与预览**：`formSideHtml` 渲染分栏侧；预览面板以同一份输入构造渲染，`previewBreakpoint` 模拟桌面/移动断点，保证「预览」与真实登录页走同一套逻辑。

### 配置驱动数据流

```
后端事实源                               前端消费层
─────────────                           ──────────

applications.signin_items (JSONB)  ─────▶  getSigninItems(application)
  {name, visible, rule, isCustom}                                    ↓
                                                                  getSigninMethodTabs(application, capabilities)
                                                                    → [{ key, label, disabled }]
                                                                         ↓

providers 表                        ─────▶  providers 列表（按 geo 策略排序）
  wechat / github / dingtalk / ...                                   ↓

organization_auth_config             ─────▶  resolveFaceIdProvider → hasFaceIdProvider
  password_obfuscator_type / ldap / ...                              ↓

useBrowserCapabilities()            ─────▶  { supportsWebAuthn,
  navigator.credentials?.create/get          hasMediaDevices,
                                             isSecureContext, ... }
                                                       ↓
                                              getSigninMethodTabs disabled 判定矩阵
```

### 登录方法 Tab 禁用判定矩阵

| 方法 Tab | 禁用条件（任一） | 实现位置 |
|----------|----------------|---------|
| 密码 | `signin_items` 里 "Password" 不可见 | `getSigninMethodTabs` |
| 验证码 | "Verification code" 不可见 | 同上 |
| WebAuthn | "Password" 不可见（共用密码路径的入口能力判定）或 `supportsWebAuthn === false` 或 `isSecureContext === false` | 同上 + 浏览器能力检测 |
| 扫码 | 缺少 `application.id` 或 `organizationId`（`canUseQrLogin`） | 同上 |
| Face ID | `!hasFaceIdProvider`（配置级）或 `hasMediaDevices === false` 或 `isSecureContext === false`（浏览器级；`getUserMedia` 与 WebAuthn 同样要求安全上下文） | `getSigninMethodTabs` + 浏览器 hook |
| LDAP | "Password" 不可见（与密码 Tab 同源，无组织 LDAP 配置检查） | 同上 |

事实源：`web/src/shared/lib/auth-signin-items.ts` 的 `getSigninMethodTabs`（复现：`grep -n 'webAuthnDisabled\|faceIdDisabled' web/src/shared/lib/auth-signin-items.ts`）。Face ID 与 WebAuthn 一致，均在 `isSecureContext === false` 时禁用（`getUserMedia` 仅在安全上下文可用）。

**禁止项**：前端 Tab 渲染层**不得**根据客户端 OS / UA 做额外禁用判定。所有禁用条件必须上收为 `getSigninMethodTabs` 返回的 `disabled: boolean` —— 这与"注册表驱动一切可枚举事实"（系统不变量 §2）一致。

### XState 登录方法切换状态机（目标态）

> **规划态章节**：XState 为目标态技术选型（《frontend/README.md》技术栈表），运行期依赖以 `web/package.json` 为准（复现：`grep -n 'xstate' web/package.json`）；方法切换由 `LoginSigninItems.tsx` 的 `loginMethod` 分支渲染承载。

前端技术栈选择 **XState** 管理登录/MFA/OAuth 授权等异步流程状态机。登录方法切换使用轻量状态机，核心职责是：在不重新装载整个 `LoginShellLayout` 的情况下平滑切换方法面板，保证 Tab 激活状态与表单值独立。

```typescript
// 目标态：登录方法切换状态机（XState）
import { createMachine, assign } from "xstate";

// 与现行 LoginMethodKey 闭集对齐（web/src/shared/lib/auth-signin-items.ts）：
// verificationCode 为邮箱/手机验证码的父键，两个子键独立可切换。
export type LoginMethod =
  | "password"
  | "verificationCode"
  | "verificationCodeEmail"
  | "verificationCodePhone"
  | "webauthn"
  | "qrcode"
  | "faceId"
  | "ldap";

interface LoginMethodContext {
  currentMethod: LoginMethod;
  availableMethods: LoginMethod[];  // 从 getSigninMethodTabs 过滤 disabled
}

const methodMachine = createMachine({
  id: "login-method-switch",
  initial: "idle",
  context: {
    currentMethod: "password",
    availableMethods: ["password", "verificationCode", "webauthn", "qrcode", "faceId", "ldap"],
  } as LoginMethodContext,
  states: {
    idle: {
      on: { INIT: "initialized" },
    },
    initialized: {
      initial: "password",
      states: {
        password: {
          on: { SWITCH: [{ target: "verificationCode", guard: "verificationAvailable" }] },
        },
        verificationCode: {
          on: { SWITCH: [{ target: "password", guard: "passwordAvailable" }] },
        },
        webauthn: {
          on: { SWITCH: [{ target: "password", guard: "passwordAvailable" }] },
        },
        qrcode: { type: "final" },
        faceId: { type: "final" },
        ldap: {
          on: { SWITCH: [{ target: "password" }] },
        },
      },
      on: {
        INIT_MACHINE: {
          target: ".password",
          actions: assign({
            availableMethods: ({ event }) => event.data as LoginMethod[],
          }),
        },
      },
    },
  },
  guards: {
    verificationAvailable: ({ context }) =>
      context.availableMethods.includes("verificationCode"),
    passwordAvailable: ({ context }) => context.availableMethods.includes("password"),
  },
});
```

**为什么轻量状态机**：`LoginSigninItems.tsx` 以 `loginMethod` 字符串分支渲染各方法面板，状态分散为多个 `useState`。XState 切换后：
1. Tab 点击 → `send({ type: "SWITCH" })` 一次 action，Tab 激活与方法面板切换原子化
2. `availableMethods` 初始值由 `useLoginPageController` 从 `getSigninMethodTabs` 计算好传入，Tab disabled 与状态机 guard 语义一致
3. 未来扩展新方法（如 Magic Link / Apple / Google One Tap 独立 Tab）只需加一个 state，不改 switch 分支

**渐进替换策略**：auth 组件清单以 `web/src/pages/auth/` 目录为准（复现：`find web/src/pages/auth -name '*.tsx' ! -name '*.test.tsx'`），XState 引入后先做 **Tab 切换层** 的轻量状态化，再逐步下沉到 MFA / OAuth 授权码流程（这些流程有真正的异步状态需要状态机管理）。

### 浏览器能力检测 Hook

```typescript
// web/src/shared/hooks/useBrowserCapabilities.ts
type BrowserCapabilities = {
  isSecureContext: boolean;      // window.isSecureContext（同步检测）
  supportsWebAuthn: boolean;     // navigator.credentials?.create/get 存在
  supportsPlatformAuth: boolean; // 系统级 passkey（isUserVerifyingPlatformAuthenticatorAvailable，异步回填）
  hasMediaDevices: boolean;      // navigator.mediaDevices?.getUserMedia（Face ID 前置）
  hasFaceDetector: boolean;      // FaceDetector API 原生支持
};
```

| 检测项 | 时机 | 用途 |
|--------|------|------|
| `isSecureContext` | 页面加载立即同步 | WebAuthn Tab 前置守卫 |
| `supportsWebAuthn` | mount 时同步检测 | WebAuthn Tab 禁用判定 |
| `supportsPlatformAuth` | mount 后 Promise 异步回填 | 平台认证器可用性提示 |
| `hasMediaDevices` | mount 时同步检测 | Face ID Tab 禁用判定 |

Conditional UI（`mediation:'conditional'`）检测为目标态增强，接口字段以 Hook 事实源为准（复现：`grep -n 'BrowserCapabilities' web/src/shared/hooks/useBrowserCapabilities.ts`）。

检测失败的处置：**不降级功能**（fail-close，《security-model》§2.8），只禁用对应 Tab 并在 Tab 上悬浮 tooltip 说明原因（i18n 键 `auth:signin.tab.disabledReason.*`）。

### 第三方 Provider 动态渲染

Provider 列表取自 `providers` 表 + `providers.category = "social" | "OAuth"`，前端排序策略：

| 策略 | 实现 | 优先级 |
|------|------|--------|
| geo-based | IP → 国家代码；CN 优先微信/QQ/微博，US 优先 Google/Apple | 1 |
| admin-pinned | 管理员在应用设置里显式置顶的 Provider | 2 |
| frequency | 本地存储 `gl:login:provider-frequency:{app}` | 3 |

渲染尺寸：
- `displayMode="big"` → 大按钮（block large），主登录路径
- `displayMode="small"` → 小按钮网格（`grid auto-fit minmax(160px, 1fr)`），辅助登录

点击处理：`socialAuthorize({ providerId })` → 后端返回安全的 `redirectUrl` → `isSafeRedirectUrl` 校验 → `window.location.assign`。Google One Tap 使用 `GoogleOneTapVirtualButton` 独立嵌入。

### Captcha Widget 降级链

Captcha 是全局组件，嵌入在 password / verificationCode / ldap 三种方法下（qrcode 自动豁免）：

| Captcha 类型 | 前端渲染 | 后端校验 |
|-------------|---------|---------|
| Default（内置图形） | `<Input addonAfter={<img base64 />}>` | captchaToken 比对 |
| reCAPTCHA v2/v3 | Suspense + Google script 懒加载 | token 校验 + score |
| hCaptcha | Suspense + hCaptcha widget | token 校验 |
| Turnstile | Suspense + Cloudflare script | token 校验 |
| GeeTest | Suspense + GeeTest SDK | token 校验 |

Captcha 必需性由 `application.signin_items` 的 "Captcha" 项可见性控制（`getSigninItems` → `isSigninItemShown`）。


## 二次验证（step-up）与信任设备

| 项 | 规范 |
|----|------|
| 触发 | 风险分级 medium 或敏感操作要求 step-up；已完成更高认证等级（mfa/fido2）的会话在有效期内不重复挑战 |
| 权限点 | step-up 与信任设备端点归属 `auth` 资源域，权限点须在 `contracts/registries/permissions.yaml` 登记（资源 `auth`，动作 `step_up` / `trusted_device`），并由默认角色授予 |
| 信任设备 | 信任期由组织策略给出；撤销后该设备下一次请求立即要求重新认证 |
| 凭据存储 | `trusted_devices.device_credential_id` 只存客户端凭据的 **SHA-256 摘要（64 位小写 hex）**，明文不落库；存量明文行由启动期一次性撤销（回退 MFA 并引导重新注册），声明层注释与 DDL 已同步 |
| 客户端生成 | 客户端凭据必须由 ≥128 bit CSPRNG 生成（推荐 32 字节随机）；服务端下限（hex-64 或 base64url ≥32 字符）只做输入校验，不能替代客户端熵要求 |
| 审计 | `auth.step_up.completed` / `auth.trusted_device.registered` / `auth.trusted_device.revoked`，记录认证方式与风险等级 |

端点归属与鉴权矩阵以 `docs/architecture/api-surfaces.md` 与 `internal/transport/http/api/v1/auth/routes.go` 为准；权限注册表登记属注册表变更，须走确认。

## 协议域声明

完整 OAuth2/OIDC 协议扩展见《design/protocol/oauth2.md》。SAML/CAS 规格见《design/auth/saml.md》。
