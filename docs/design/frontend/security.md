# 前端安全

> 设计锚点：[DESIGN:frontend/security]

> 目标态规范「前端安全」；写作约束见 [docs/README.md](../../README.md)。

---

## 会话 Cookie 属性

| Cookie | 属性 | 值 |
|--------|------|-----|
| `session` | HttpOnly | ✓ |
| | Secure | 跟随 `COOKIE_SECURE`；未设置时 production 默认 true、其他环境默认 false |
| | SameSite | 安全模式 **Strict**；HTTP 开发模式 **Lax** |
| | Path | `/` |
| | Max-Age | 30d（与会话 absolute_timeout 是两个独立维度） |
| | 名称 | 安全模式下 `__Host-session`（Path=/、无 Domain、Secure 三条件同时满足） |
| `csrf_token` | HttpOnly=false（JS 可读） | ✓ |
| | Secure | 与会话 Cookie 使用同一 `COOKIE_SECURE` 开关 |
| | SameSite | Lax |
| | 名称 | 安全模式 `__Host-csrf`；HTTP 开发模式 `csrf_token`；256-bit 随机双提交值，不使用会话 HMAC |

## SameSite 矩阵（回调形态 × 属性）

| 场景 | 请求形态 | SameSite=Lax 下会话随行？ | 处置 |
|------|---------|--------------------------|------|
| OIDC 授权码回调 | 跨站顶级 GET redirect | **是**（Lax 允许顶级 GET） | 维持 Lax |
| SAML ACS / CAS serviceValidate | 跨站顶级 POST | **否**（Lax 屏蔽） | 二选一：(a) IdP 发起跳转经本域 302 中转（top-level GET）；(b) 回调端点凭 state/RelayState 绑定的请求上下文重建会话（Cookie 无随行时拒绝静默降级） |
| Front-Channel Logout iframe | 跨站 iframe 内 | 否 | logout 端点凭 URL 携带的 iss+sid 签名参数撤销，不依赖 Cookie 随行 |
| SPA 同站 XHR | 同站 | 是 | 维持 |

**禁止**为协议端点全局改 SameSite=None——CSRF 面扩大，须 Secure+白名单，仅当 (a)(b) 均不可行且经评审。

## CSRF 双提交与豁免边界

### 机制

- **安全方法**（GET/HEAD/OPTIONS）：补发 cookie
- **Mutating 方法**（POST/PUT/PATCH/DELETE）：校验 `X-CSRF-Token` 头 == cookie 值

### 豁免（仅限两类）

1. **已验证的 Bearer/JWT 请求头会话**——`Authorization: Bearer` 或 `X-Access-Token` 中的 JWT 必须先由 `SessionContextMiddleware` 验签；原始请求头或无效令牌不构成豁免，Cookie 会话仍需双提交校验
2. **协议端点显式清单**（无 Cookie 随行语义）：OIDC token/introspection/revocation、SAML ACS、SCIM、CAS、WebAuthn finish 等

清单由 route 层声明而非 method 推断。

## DPoP 客户端

DPoP（RFC 9449）允许公开客户端在 OAuth 流程中以 DPoP 证明替代客户端密钥：

- 客户端生成**非对称密钥对**，公钥（JWK）内嵌在 proof JWT 的 `jwk` 头中
- 每次令牌请求附 DPoP Proof：**非对称签名的 JWT**（`typ: dpop+jwt`，claims 含 `jti` / `htm` / `htu` / `ath`），签名算法限服务端白名单（RS256/ES256/PS256，事实源 `internal/platform/dpop/dpop.go` 的 `SupportedAlgorithms`，Discovery 以 `dpop_signing_alg_values_supported` 宣告）；禁止 `alg=none` 与对称算法（HMAC 无法满足持有证明语义）
- 服务器验证 proof → 把 access token 绑定到该公钥（令牌 `cnf.jkt` 声明）
- 客户端使用 access token 调 API 时同样附 proof；服务器按 `cnf.jkt` 验证

## DPoP 客户端实现要点

- DPoP key 在客户端本地存储（IndexedDB 或 sessionStorage）
- JWT 证明有效期 ≤ 10 秒（防重放）
- 与 WebAuthn Passkey 的关系：DPoP 是密钥绑定，Passkey 是无密码认证；二者可以独立也可以组合


---

### 无障碍（原 a11y.md，全文保留）

> 目标态规范「可访问性规范（a11y）」；写作约束见 [docs/README.md](../../README.md)。

---

### 目标与范围

遵循 **WCAG 2.1 Level AA**（基线版本唯一口径见《[GuliangIAM-开发规范](../../GuliangIAM-开发规范.md)》无障碍章节，其他文档只引用不另定版本）。覆盖键盘导航、屏幕阅读器、颜色对比度、表单可访问性、多媒体替代文本。

### Skip Link（键盘导航起点）

所有页面必须提供 Skip Link——键盘 Tab 的第一个焦点，点击后跳到主内容区：

```tsx
// src/app/layout/AppShell.tsx
<SkipLink href="#main-content">跳转到主内容</SkipLink>
```

Skip Link 默认隐藏，获焦时显示；主内容区必须有 `id="main-content"` + `tabIndex={-1}`。

### ARIA Live Region

Toast 通知、异步加载状态、CRUD 操作结果通知屏幕阅读器：

```tsx
// src/shared/ui/Toast.tsx — useToast hook
export function useToast() {
  const { message, notification } = App.useApp();
  // antd Message/Notification 组件自带 role="status"/role="alert"
  return {
    success: (content: string) => message.success(content),  // role="status"
    error: (content: string) => message.error(content),      // role="alert"
    warning: (content: string) => message.warning(content),
    info: (content: string) => message.info(content),
  };
}
```

### 颜色对比度

| 元素 | 最低对比度 |
|------|-----------|
| 正文（≥14px） | 4.5:1 |
| 大文本（≥18px 粗 / ≥24px） | 3:1 |
| 可交互图标 | 3:1（识别状态需要视觉提示） |

禁用仅靠颜色传达信息（如状态指示必须同时有图标或文字）。

### 表单可访问性

- `<label>` 与 `<input>` 必须通过 `htmlFor`/`id` 关联
- 必填字段有 `aria-required="true"` 或 `required`
- 错误提示有 `aria-invalid="true"` + `aria-describedby` 关联错误元素
- `aria-describedby` 用于描述性文本（如密码策略提示）
- Zod 校验错误同时写视觉（红色文本）和 aria（`aria-invalid`）

```tsx
<FormItem
  label="密码"
  required
  htmlFor="password"
  aria-describedby={field.error ? "password-error" : "password-hint"}
>
  <Input.Password id="password" aria-invalid={!!field.error} />
  {field.error && <span id="password-error" role="alert">{field.error.message}</span>}
  <span id="password-hint" className="text-secondary">至少 14 字符，含大小写、数字与特殊字符</span>
</FormItem>
```

### 对话框与模态框

模态框必须：

- 打开时自动 focus 到对话框内第一个可聚焦元素
- 关闭时 focus 返回触发按钮
- Esc 键关闭
- 背景锁定（焦点不逃逸）
- `role="dialog"` + `aria-modal="true"`
- 有可访问名称（`aria-labelledby` 或 `aria-label`）

```tsx
<Modal
  role="dialog"
  aria-modal="true"
  aria-labelledby="confirm-delete-title"
  title={<span id="confirm-delete-title">确认删除</span>}
  open={open}
  onOk={handleOk}
  onCancel={handleCancel}
  okText="确认"
  cancelText="取消"
  autoFocus
/>
```

### 键盘导航

| 场景 | 要求 |
|------|------|
| 侧边栏菜单 | Tab 可达 + 方向键导航（↑↓ + Enter/Space 激活） |
| 表格 | 方向键导航行 + Enter 打开详情 |
| 标签页 | 方向键切换 Tab（←→ + 数字键 1-9） |
| 日期选择器 | 方向键 + 直接输入日期 |

禁用 `tabIndex` 硬编码 > 0（依赖 DOM 顺序即可）。

### 图像替代文本

| 元素 | alt 要求 |
|------|---------|
| 装饰性图标 | `alt=""`（空字符串，屏幕阅读器跳过） |
| 功能按钮图标 | 按钮文本替代（`<Button icon={<DeleteOutlined />}>删除</Button>`） |
| Logo | 描述性 alt（`alt="GuliangIAM Logo"`） |
| 头像 | 用户名称（`alt={user.displayName}`） |
| 复杂图表 | `aria-label` + 数据表格替代 |

### Reduced Motion

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

### 与设计系统的关系

设计系统 tokens 已内置 reduced-motion 支持（[frontend/README.md §设计系统](README.md#设计系统原-design-systemmd全文保留) 动画体系）。

## 会话超时与重新认证

| 项 | 规范 |
|---|---|
| 策略来源 | 生效策略（idle / absolute）以会话接口返回的服务端策略为准；前端不得硬编码超时值 |
| 展示 | 到期前按剩余时间提示；倒计时以服务端返回的 `expiresAt` 计算，避免本地时钟漂移 |
| 401 处理 | 并发请求同时 401 时只触发一次跳转（single-flight），其余请求排队等待 |
| 续期 | 会话续期由服务端接口完成；前端不得通过重复登录绕过 idle 超时 |
| 重新认证 | step-up / 重新认证弹层成功后重放被拦截的请求一次，失败则跳登录 |
| 跨标签页 | 登出与改密通过 BroadcastChannel 同步，其他标签页立即失效 |
