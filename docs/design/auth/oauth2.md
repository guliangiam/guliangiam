# OAuth2 授权码流程

> 设计锚点：[DESIGN:auth/oauth2]

> 目标态规范「OAuth2 授权码流程」；写作约束见 [docs/README.md](../../README.md)。

---

## 授权码流程时序

```
用户       应用(SP)      GuliangIAM(IdP)
 │           │               │
 │──访问──>  │               │
 │           │──重定向──────>│ GET /authorize?client_id=xxx&redirect_uri=xxx&code_challenge=xxx
 │           │               │ 展示登录页
 │<──────────────────────登录│
 │           │               │ 展示 Consent 页
 │<──────────────────────授权│
 │           │<──重定向+code─│ 302 redirect_uri?code=xxx
 │           │               │
 │           │──POST token──>│ POST /api/v1/auth/tokens/exchange {code, code_verifier}
 │           │<──access_token│ {access_token, refresh_token, id_token}
```

路由声明见 `internal/transport/http/api/v1/auth/routes.go` 与 `chi_protocol.go`（复现：`grep -n 'tokens/exchange' internal/transport/http/api/v1/auth/routes.go`）；discovery 宣告同路径（`internal/app/auth/oidc_discovery.go` 的 `TokenEndpointPath`）。

## 强制基线控制项

授权端点强制基线（PKCE、state、redirect_uri 精确匹配与 loopback 例外、grant_types 白名单、禁用 Implicit/password grant）与授权码参数（熵/TTL/哈希存储/一次性消费/绑定项）的**单点定义**见《design/protocol/oauth2.md》§14.6.1–§14.6.4，本文不重复定义。其中 `__Host-ars` 授权请求 Cookie 绑定为规划态控制（该文件 §14.6.2），不属于运行期基线。

## 协议域声明

完整 OAuth2/OIDC 协议扩展见《design/protocol/oauth2.md》（条目以该文件章节为准，本文不计数）。
