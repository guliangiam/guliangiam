# QR Code 登录时序

> 设计锚点：[DESIGN:auth/qrcode]

> 目标态规范「QR Code 登录时序」；写作约束见 [docs/README.md](../../README.md)。

---

## 四步时序

PC 端创建二维码并轮询，扫码端（移动设备）扫码确认，PC 端以确认结果换取令牌与会话。路由声明见 `internal/transport/http/api/v1/auth/routes.go`。

| 步骤 | 端点 | 请求体 | 响应体 | 匿名可达 |
|------|------|--------|--------|---------|
| 1 创建 | `POST /api/v1/auth/qrcode` | `{organizationId, applicationId}` | `{codeId, code, expiresIn, interval}`（201） | 是 |
| 2 扫码 | `POST /api/v1/auth/qrcode/scan` | `{code, username?, displayName?, ipAddress?, userAgent?}` | `{status:"ok", isNewUser}` | 是 |
| 3 确认 | `POST /api/v1/auth/qrcode/confirm` | `{code, approved}` | `{status:"ok"}` | 否（确认者须有会话） |
| 轮询 | `GET /api/v1/auth/qrcode/{codeId}/status` | — | `{status, codeId, expiresIn?}` | 是 |
| 4 换取 | `POST /api/v1/auth/qrcode/exchange` | `{codeId, responseType?, redirectUri?, state?, nonce?, scope?}` | 登录结果 + **会话 cookie** | 是 |

## 句柄分工

- `code` 是二维码载荷（随机串，扫码端唯一持有的句柄），`scan`/`confirm` 按它寻址
- `codeId` 是发起端的交换句柄，只出现在 `create` 响应与 `poll`/`exchange` 中
- 二维码里不放 `codeId`（否则任何看到二维码的人都能抢先 `exchange`）
- 服务端**不返回 `qrUrl`**：`code` 必须由客户端编码成二维码图形

## 扫码者身份

- 调用方已有会话 → 身份只取自会话，请求体身份字段一概不读；与二维码会话所属组织不一致的账号被拒绝
- 调用方匿名 → 只允许**自动注册一个尚不存在的账号**（`username` 必填），按组织 `DisableSignup`/`DisableSignin` 开关门控；命中既有账号一律 403
- 匿名注册成功时同时为扫码端签发会话（响应 `Set-Cookie`）
- `confirm` 只接受扫码绑定的那个人

## 访客账号与设备记忆

- 匿名扫码注册出来的是**访客账号**（`user_type = guest`，无密码凭据，`register_type = QR Code Scan`）
- 同一设备下次扫码时 `scan` 走已认证路径、身份取自会话，直接复用同一访客账号
- 访客账号可通过 `PATCH /api/v1/auth/account` 自助编辑资料并绑定联系方式升级为可找回的正常账号

## 契约约束

| 约束 | 值 |
|------|-----|
| 轮询节奏 | 由服务端给定（`interval` + `expiresIn`），客户端不得自造更密节奏 |
| 状态闭集 | `pending → scanned → confirmed`；过期或被拒绝为 `expired`；`exchanged` 不是轮询可见状态——一次性凭据 |
| 拒绝即销毁 | 第 3 步 `approved=false` 删除会话，PC 侧轮询随即得到 `expired` |
| 令牌形态 | **会话 Cookie + 登录结果（LoginResponse）**：exchange 在事务内为发起端创建会话并经 `handleLoggedIn` 收口，按应用 responseType/grant 配置可携带 `code`/`redirectTo`/`idToken`/`refreshToken`/`accessToken` 等协议字段；不自行签发一次性 access token（OPEN-API-16/17，`internal/app/auth/qrcode.go` 的 `ExchangeQRCode`） |
| 限流 | 扫码端点在认证面限流组内（`RATE_LIMIT_AUTH_RATE`/`BURST`） |

## 完整时序图

```
PC 端（匿名）                 GuliangIAM                     扫码端（可匿名）
 │──1 POST /qrcode─────────>│                                   │
 │<─{codeId, code,          │                                   │
 │   expiresIn, interval}───│                                   │
 │──把 code 渲染为二维码      │                                   │
 │──轮询 GET …/status───────>│                                   │
 │<─pending─────────────────│<──2 POST /qrcode/scan {code, …}───│
 │                          │───（匿名则自动注册 + 签发会话）────>│
 │<─scanned─────────────────│<──3 POST /qrcode/confirm {code}───│
 │<─confirmed───────────────│                                   │
 │──4 POST /qrcode/exchange─>│                                   │
 │<─Set-Cookie + 登录结果────│                                   │
 │  （协议上下文下可携带      │                                   │
 │   code/redirectTo/idToken/│                                   │
 │   refreshToken 等字段）    │                                   │
```

## 协议域声明

完整 OAuth2/OIDC 协议扩展见《design/protocol/oauth2.md》。
