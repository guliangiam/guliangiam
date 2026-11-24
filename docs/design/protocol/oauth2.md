# OAuth2/OIDC 协议扩展（集中存放）

> 设计锚点：[DESIGN:protocol/oauth2]

> 目标态规范「OAuth2/OIDC 协议扩展（集中存放）」；写作约束见 [docs/README.md](../../README.md)。

---

## 14.1 RFC 7009 Token Revocation

令牌撤销允许客户端主动通知授权服务器废弃已签发令牌。

**端点**：`POST /api/v1/auth/oauth2/revoke`

**关键设计点**：

| 设计项 | 说明 |
|--------|------|
| 客户端认证 | 与 token endpoint 相同：`client_secret_basic` / `client_secret_post` / `private_key_jwt` / `tls_client_auth` / `self_signed_tls_client_auth`；公开客户端仅 `client_id` |
| token_type_hint | 可选，提示令牌类型（`access_token` / `refresh_token`） |
| 撤销策略 | 写入 `revoked_at` 时间戳，不物理删除 |
| 安全原则 | **通过客户端认证后**，无论令牌是否存在/有效/属于当前客户端，均返回 200；**客户端认证本身失败为 `401 invalid_client`**（RFC 7009 §2.1） |
| 归属校验 | 应用层比对 `token.ApplicationID == caller.app.ID`；SQL 谓词 `AND application_id=$caller` |
| 关联撤销 | 撤销 Refresh Token 时联动撤销关联 Access Token |
| DPoP 绑定 | 若令牌有 DPoP 绑定，撤销时同时清理绑定关系 |

> **discovery 宣告对齐**：OIDC discovery 文档宣告的撤销端点必须等于协议面注册路径 `{issuer}/api/v1/auth/oauth2/revoke`（`POST /api/v1/auth/oauth2/revoke`），内省端点同理；不得宣告根级 `/oauth2/*` 别名。

## 14.2 RFC 7662 Token Introspection

令牌内省允许资源服务器查询令牌有效性和元数据。

**端点**：`POST /api/v1/auth/oauth2/introspect`

**关键设计点**：

| 设计项 | 说明 |
|--------|------|
| RS 认证 | 仅允许已注册的客户端；认证方式与 token endpoint 相同（secret / private_key_jwt / mTLS）；**认证失败必须 `401 invalid_client`**（RFC 7662 §2.2） |
| active 字段 | 核心：`true` 有效，`false` 无效（含过期/撤销/不存在/**不属于调用方 client**） |
| 响应字段 | RFC 7662 标准字段 + 扩展字段 `org_id` |
| `client_id` 语义 | 响应里的 `client_id` 必须是**令牌归属应用**的 client identifier，不是调用方传入的 |
| `org_id` 命名空间 | 扩展 claim 仅出现在非 RFC 版响应，租户边界靠调用方 client 归属表达 |
| token_type | 按令牌绑定返回：DPoP 绑定返回 `"DPoP"`，否则 `"Bearer"`，refresh 恒 `"refresh_token"` |
| 缓存策略 | RS 可缓存 Introspection 结果，TTL 不超过令牌剩余有效期 |

**Revoke/Introspect 注册路径对照表**：

| 路径 | 语义定位 |
|------|---------|
| `POST /api/v1/auth/oauth2/revoke` | **协议面 RFC 端点** |
| `POST /api/v1/auth/oauth2/introspect` | **协议面 RFC 端点** |
| `POST /api/v1/auth/tokens/introspect` | 认证面/内部便利形态（响应含 `org_id`） |
| `POST /api/v1/tokens/revoke` | 管理面运维撤销 |
| `POST /api/v1/tokens/introspect` | 管理面运维查询 |

路径以路由声明为准（复现：`grep -n 'introspect\|revoke' internal/transport/http/api/v1/auth/routes.go internal/transport/http/api/v1/tokens/routes.go`）。

## 14.3 RFC 9728 Protected Resource Metadata 发现

资源元数据允许资源服务器发布其元数据和授权服务器关联信息。

**端点**：`GET /.well-known/oauth-protected-resource`

## 14.4 按 Application 的 OIDC Discovery

每个 Application 可配置独立的 OIDC Discovery 端点。

| 配置项 | 数据来源 |
|--------|---------|
| `scopes_supported` | `app_oauth_config.custom_scopes`（JSONB） |
| `grant_types_supported` | `app_oauth_config.grant_types`（TEXT[]） |
| `response_types_supported` | 目标态应用级列 `app_oauth_config.response_types TEXT[]`（**禁从 grant_types 派生**） |
| `token_endpoint_auth_methods_supported` | `app_oauth_config.token_endpoint_auth_method` |
| `code_challenge_methods_supported` | 固定 `["S256"]` |

> 运行期 discovery（根级与按应用）宣告固定能力集，尚未按上述配置列逐应用裁剪（复现：`grep -n 'ResponseTypesSupported\|GrantTypesSupported' internal/app/auth/oidc_discovery.go internal/transport/http/api/v1/wellknown/chi.go`）；按应用读取配置列与 `response_types` 列声明为目标态设计。grant_types 白名单的**运行期判定**（授权与令牌端点）经 application 聚合读取 `app_oauth_config.grant_types`（`internal/app/auth/authorize.go`、`token_exchange.go`）。

Discovery 端点：

| 路径 | 说明 |
|------|------|
| `GET /.well-known/{application}/openid-configuration` | 按应用独立 OIDC Metadata |
| `GET /.well-known/{application}/jwks.json` | 按应用独立密钥集（仅国际算法族） |
| `GET /.well-known/jwks-gm.json` | 国密 SM2-SM3 独立密钥集（扩展通道，供国密 RP 验签） |
| `GET {appSlug}/.well-known/openid-configuration` | issuer 前缀式（RFC 8414 多租户惯例） |

## 14.5 OIDC Back-Channel Logout Token（锚点）

用户登出或会话被撤销时，IdP 向各 RP 注册的 `backchannel_logout_uri` 投递签名 JWT（`typ=logout+jwt`）。

**Claims 名单**：

| Claim | 必填性 | 取值 |
|-------|--------|------|
| `iss` | REQUIRED | OP issuer（精确等于 discovery `issuer`） |
| `aud` | REQUIRED | 必须等于目标 RP 的 `client_id`（单值字符串） |
| `iat` | REQUIRED | 签发时间秒 |
| `jti` | REQUIRED | UUIDv7，RP 去重 |
| `events` | REQUIRED | 唯一成员 `"http://schemas.openid.net/event/backchannel-logout"` 值为空对象 |
| `sub` | 与 `sid` 至少一个 | 用户主体 |
| `sid` | 与 `sub` 至少一个 | 会话标识（宣告 `backchannel_logout_session_supported=true` 时**必带**） |
| `nonce` | OPTIONAL | RP 注册值 |
| `exp` | OPTIONAL（TTL 5min） | |

**投递语义**：走《design/infrastructure/outbox.md》Outbox/Worker，投递重试、死信与 `jti` 去重由 Worker 保证，不得在业务事务内同步发起 HTTP POST。

## 14.6 OAuth 2.1 授权端点强制基线

### 14.6.1 强制基线总表

| # | 控制项 | 规则值 |
|---|--------|--------|
| 1 | PKCE | **全部客户端强制** `code_challenge` + `code_challenge_method=S256`；`plain` 一律拒绝 |
| 2 | state | 客户端 `state` 原样回显（运行期基线）；`__Host-ars` 授权请求 Cookie 绑定为**规划态**增强，见 §14.6.2 |
| 3 | redirect_uri | 精确等值匹配（scheme/host/port/path 逐字节相等）；禁通配/子串/后缀；loopback 例外见 §14.6.3（RFC 8252 §7，`internal/domain/applications/redirect_uri.go` 的 `MatchRedirectURI`） |
| 4 | grant_types 白名单 | 服务端统一以 `app_oauth_config.grant_types` 判定（经 application 聚合读取） |
| 5 | 禁用 response_type=token | OAuth 2.1 移除 Implicit；只接受 `code` |
| 6 | 禁用 password grant | 永不实现 |

### 14.6.2 state 生成、Cookie 绑定与一次性消费（规划态）

> **规划态章节**：运行期 authorize 端点不下发、不消费 `__Host-ars` Cookie（复现：`grep -rn '__Host-ars' internal/` 零命中）；本节定义目标态控制，已登记开放项注册表 OPEN-API-79。

| 设计点 | 规则 |
|--------|------|
| 生成 | `crypto/rand` 32 字节 → base64url（≥128 bit） |
| Cookie 属性 | `__Host-ars`（Secure、Path=/、不设 Domain、HttpOnly、SameSite=Lax） |
| 绑定内容 | 与 `client_id`、规范化 `redirect_uri`、`nonce`、`code_challenge` 摘要绑定 |
| TTL | 与授权码同寿（≤5min） |
| 一次性消费 | 发码或回显错误即删除，重放同一 ars → `invalid_request` |

### 14.6.3 redirect_uri 精确匹配与不可重定向错误决策表

**不可重定向错误就地展示**：当 `client_id` 缺失/无效或 `redirect_uri` 缺失/不匹配时 **MUST NOT** 自动重定向，展示 400 自托管错误页。仅当 client 合法 + redirect_uri 合法但 response_type 不支持/scope 越界/用户拒绝时才 302 回跳。

**localhost 与 `*.localhost` 不放行**：它们是 DNS 名，不构成「留在本机」的保证。开发请用 `http://127.0.0.1:<port>/callback`。

**loopback 例外（RFC 8252 §7）**：仅 IP 字面量 `127.0.0.1` / `::1` 允许明文 http；当请求与注册值同为同一 loopback 字面量、且**注册值未携带端口**时忽略端口差异，path 与 query 仍逐字节精确比对（`internal/domain/applications/redirect_uri.go` 的 `matchesLoopbackException`；复现：`grep -n 'matchesLoopbackException' internal/domain/applications/redirect_uri.go`）。

### 14.6.4 授权码参数

| 设计点 | 规则值 |
|--------|--------|
| 熵 | `crypto/rand`，≥256 bit（32 字节 base64url），前缀 `gl_ac_` |
| TTL | **5 分钟** |
| 存储 | 仅存 SHA-256 哈希 + 前缀 |
| 一次性 | 原子条件更新，重用时撤销该码已签发的全部令牌 |
| 绑定 | application_id、redirect_uri、scopes、nonce、code_challenge、resource |

### 14.6.5 公开客户端分支（token_endpoint_auth_method=none）

- 令牌端点**不接受** secret；强制 PKCE-S256 作为唯一请求认证
- 不签发 refresh token（除非具备发送者约束能力如 DPoP/mTLS）

## 14.7 客户端模式与刷新令牌边界

**Client Credentials（RFC 6749 §4.4）**：

> **运行期边界**：客户端认证通过后，该 grant 一律返回 400 `unsupported_grant_type`（令牌主体仅限用户，application/service account 主体无法落库为令牌 subject；`internal/app/auth/client_credentials.go`，复现：`grep -n 'unsupported_grant_type' internal/app/auth/client_credentials.go`）。discovery 刻意不宣告该 grant（§14.13.2）。下表令牌语义为**目标态**，仅在令牌主体模型扩展后生效；路由 `POST /api/v1/auth/tokens/client-credentials` 存在但不可成功换取令牌。

| 设计点 | 规则 |
|--------|------|
| 客户端资格 | **仅机密客户端**；公开客户端拒绝 |
| 认证 | basic/post，argon2id 校验；失败 `401 invalid_client` |
| 令牌语义（目标态） | **无用户主体**：sub/用户名为 `client:{client_id}`；TTL 1h |
| scope | 请求 scope 必须 ⊆ 应用获准 scope |

**Refresh Token 旋转与 family 重用检测**（RFC 6749 §6）：

- 旧 refresh 一次性失效、新 refresh 同寿命续发
- family 模型：`refresh_tokens.family_id` + 部分索引
- 已撤销 refresh 再次出现 → 整族吊销 + critical 审计
- refresh 密文：`gl_prt_` + base64url（32 字节 `crypto/rand`，无填充；`internal/app/auth/token_exchange.go` 的 `newRefreshTokenSecret`），TTL 30 天

## 14.8 ID Token claims 全表与校验规则

**签发侧 claims**：

| Claim | 规则 |
|-------|------|
| `iss` | **精确等于** discovery `issuer`（单一解析器 `platform/config/issuer.go`） |
| `sub` | 用户主键 `user.ID`；`public` subject |
| `aud` | 数组必含本 client 的 `client_id` |
| `azp` | 多受众时必带，等于本 client_id |
| `exp` | 当前时间 + ID Token 寿命（建议 ≤1h） |
| `iat` | 签发时间（秒） |
| `nonce` | 授权请求带 nonce 时**原样回显** |
| `at_hash` | 隐式/混合流程必带；code 流程 OPTIONAL 签发 |
| `auth_time` | **终端用户实际完成认证的时间**（含 MFA 完成时刻） |
| `acr` | 认证上下文类引用 |
| `amr` | 实际使用的认证方法数组 |
| `sid` | 会话标识（与 Logout Token 中 `sid` 对应） |
| `profile/email/phone` | 按**已授予 scope** 裁剪，未授予的 claim **省略**（不置空） |

**校验侧强制规则**：

1. JOSE 头先验：`alg` 必须在白名单内（`RS256` 起步），**显式拒绝 `alg=none`**；按 `kid` 解析验签密钥
2. `iss` 精确匹配；`aud` 包含自身 client_id；`exp` 晚于当前（±60s leeway）
3. `nonce` 必须逐字符相等；`max_age` 触发 `auth_time` 重认证判定

**唯一签发路径**：授权码换码在 `openid` scope 下签发——`token_exchange.go` 复用 `buildIDToken`，nonce 取授权码行绑定值。

## 14.9 UserInfo 端点

**端点**：`GET {issuer}/oauth2/userinfo`（宣告于 discovery `userinfo_endpoint`）

| 设计点 | 规则 |
|--------|------|
| 认证 | 仅 `Authorization: Bearer <token>`（不接受 URI query）；失效/过期/撤销 → `401` + `WWW-Authenticate: Bearer error="invalid_token"` |
| `sub` 一致性 | 必须与 ID Token 的 `sub` 完全一致 |
| 按 scope 裁剪 | `openid` 恒返回 sub；`profile` 才返回 name/preferred_username 等；未授予 scope 对应 claim 省略 |
| DPoP | 绑定令牌调用同样校验 proof 与 `cnf.jkt` |
| 缓存 | 响应头 `Cache-Control: no-store`、`Pragma: no-cache` |

## 14.10 prompt=none / max_age 语义

| 触发条件 | `prompt=none` | 普通请求 |
|----------|---------------|---------|
| 无有效会话 | `error=login_required` | 转登录交互 |
| 有会话但超 max_age | `error=login_required` | 静默重认证 |
| 有会话、max_age 满足但缺 consent | `error=consent_required` | 展示同意页 |
| 需要 MFA 升级 | `error=interaction_required` | 进入 MFA 页 |
| 全满足 | **直接成功直发码** | 直接成功 |

## 14.11 RP-Initiated Logout

**discovery 宣告**：`end_session_endpoint = {issuer}/api/v1/auth/logout`

| 参数 | 规则 |
|------|------|
| `id_token_hint` | OP 校验签名/iss/aud/exp，据此识别 RP |
| `post_logout_redirect_uri` | 必须与注册的 `post_logout_redirect_uris` **精确等值**（禁通配）；未注册 → 忽略 |
| `state` | 不透明透传，原样回显 |

**处置决策**：已登录 + hint 合法 + 回跳 URI 注册 → 销毁 OP 会话 → 触发 Back-Channel 扇出 → 302 回跳。hint 与当前会话不匹配 → 销毁会话后**不跳转**（防跨用户跳转/钓鱼）。

## 14.12 Back-Channel Logout 完整规格

### 14.12.1 Logout Token claims 全表

| Claim | 必填性 | 规则 |
|-------|--------|------|
| `iss` | REQUIRED | OP issuer（精确等于 discovery `issuer`） |
| `sub` | 与 `sid` 至少一个 | 用户主体 |
| `sid` | 与 `sub` 至少一个 | 会话标识（宣告 `backchannel_logout_session_supported=true` 时**必带**） |
| `aud` | REQUIRED | 必须等于目标 RP 的 `client_id`（单值字符串） |
| `iat` | REQUIRED | 签发时间秒 |
| `jti` | REQUIRED | UUIDv7，RP 与 OP Worker 双侧去重 |
| `events` | REQUIRED | 唯一成员 `"http://schemas.openid.net/event/backchannel-logout"` 值为 `{}` |
| `nonce` | OPTIONAL | RP 注册值 |
| `exp` | OPTIONAL（TTL 5min） | |

JOSE 头：`alg=RS256`（`typ="logout+jwt"`，带 `kid`）。

### 14.12.2 RP 侧校验规则

1. 是签名 JWT；`alg` 必须在白名单内（拒 `none`），按 `kid` 取钥验签
2. `typ` 头为 `logout+jwt`（**该令牌绝不能作为 access token 使用**）
3. `iss` 精确匹配；`aud` 等于自身 client_id（多受众/非自身一律拒绝）
4. `events` 含且仅含 backchannel-logout 成员
5. `sub`、`sid` 至少有一个；带 `sid` 时只销毁该会话，仅有 `sub` 销毁该用户全部会话
6. `jti` 去重：同 jti 重复投递必须幂等返回 200
7. `nonce`（若注册）必须一致；`iat` 新鲜度 ±3min

### 14.12.3 sessions.sid 列与双令牌一致

`sessions` 新增 `sid TEXT NULL`（唯一约束 `uq_sessions_sid`）。授权码同步新增 `sid TEXT NOT NULL DEFAULT ''`——授权码是 sid 的唯一载体，换码端点把 sid 写入 ID Token。

sid 取值为 `gl_sid_` + 32 字节 `crypto/rand`（base64url 无填充），由 `platformcrypto.NewRandomSecret` 统一产出。

### 14.12.4 投递设施

复用《design/infrastructure/outbox.md》统一 Outbox → Worker → 重试 → 死信。投递参数：超时 10s、退避同注册表、顺序保证按 `(aggregate_type, aggregate_id)` 分区内串行。

## 14.13 Discovery 文档宣告字段

### 14.13.1 根级 Discovery

| 字段 | 值 |
|------|-----|
| `issuer` | 外部可达 issuer URL |
| `authorization_endpoint` | `{issuer}/api/v1/auth/authorize` |
| `token_endpoint` | `{issuer}/api/v1/auth/tokens/exchange` |
| `userinfo_endpoint` | `{issuer}/oauth2/userinfo` |
| `jwks_uri` | `{issuer}/.well-known/jwks.json` |
| `revocation_endpoint` | `{issuer}/api/v1/auth/oauth2/revoke` |
| `introspection_endpoint` | `{issuer}/api/v1/auth/oauth2/introspect` |
| `registration_endpoint` | `{issuer}/oauth2/register` |
| `device_authorization_endpoint` | `{issuer}/api/v1/auth/device/authorize` |
| `end_session_endpoint` | `{issuer}/api/v1/auth/logout` |

### 14.13.2 OIDC 能力位宣告

| 能力 | 宣告 |
|------|------|
| `response_types_supported` | `["code"]`（无 Implicit） |
| `grant_types_supported` | `["authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:device_code", "urn:ietf:params:oauth:grant-type:token-exchange", "urn:ietf:params:oauth:grant-type:jwt-bearer"]`（client_credentials 因令牌主体模型暂不宣告） |
| `code_challenge_methods_supported` | `["S256"]` |
| `token_endpoint_auth_methods_supported` | `["client_secret_basic", "client_secret_post"]` |
| `subject_types_supported` | `["public"]` |
| `id_token_signing_alg_values_supported` | `["RS256", "SM2-SM3"]`（单一事实源 `internal/platform/jwks/id_token_algorithms.go`；应用级选择见 `app_oauth_config.token_signing_method`） |
| `gm_jwks_uri` | `{issuer}/.well-known/jwks-gm.json`（GuliangIAM 扩展：国密 SM2-SM3 公钥独立通道；标准客户端只读 `jwks_uri`，国密客户端读此字段） |
| `gm_alg_values_supported` | `["SM2-SM3"]`（GuliangIAM 扩展：`gm_jwks_uri` 通道发布的国密签名算法） |
| `backchannel_logout_supported` | `true` |
| `backchannel_logout_session_supported` | `true` |
| `frontchannel_logout_supported` | `true` |
| `frontchannel_logout_session_supported` | `true` |

## 14.14 密钥版本与轮换

JWKS 按算法族分端点发布：标准端点 `/.well-known/jwks.json` 与按应用端点 `/.well-known/{application}/jwks.json` 只发布国际算法族（RSA/ES）公钥，国密 SM2-SM3 族公钥由独立端点 `/.well-known/jwks-gm.json` 发布。两族各自以 `rsa_`/`gm_` kid 前缀标识，`kid` 前缀必须与算法族一致；验签方按令牌 header `kid` 前缀在对应通道选择公钥（国际 RP 取 `jwks_uri`，国密 RP 取 Discovery 扩展字段 `gm_jwks_uri`），未知或跨族 `kid` 一律拒绝（单一事实源见 [security-model §2.21](../../architecture/security-model.md#221-国密能力位与密码学边界adr-031)）。标准端点恒不含 `crv=SM2` 密钥，避免严格 JOSE 客户端（如 Nimbus）因未知曲线整组解析失败。三个端点在 OpenAPI 中仅声明 200 响应为 JSON Web Key Set，键集与 `kid` 约定按上述执行，不在 OpenAPI 中复写字段清单。

JWKS 多 kid 并列支持滚动窗口。轮换流程：

1. 签发新版本密钥（kid 唯一）
2. 新令牌用新 kid 签名
3. 旧密钥仍在 JWKS 中（过渡期）
4. 过渡期结束后从 JWKS 移除旧密钥（旧令牌过期后不再可验）

**不进行**：旧密钥替换新密钥后旧令牌立即失效——违反令牌生命周期稳定性。

## 14.15 互链

- 服务端会话模型 → [architecture/data-model.md](../../architecture/data-model.md) §3
- DPoP（RFC 9449）→ 本文件 §14.27
- Device Authorization Grant（RFC 8628）→ 本文件 §14.23
- Token Exchange（RFC 8693）→ 本文件 §14.24
- JWT Bearer（RFC 7523）→ 本文件 §14.25
- CAS/SAML → [design/auth/saml.md](../auth/saml.md)
- WebAuthn/Passkey → [design/auth/webauthn.md](../auth/webauthn.md)
- Outbox 投递 → [design/infrastructure/outbox.md](../infrastructure/outbox.md)
- 密钥轮换 → 本文件 §14.14

## 14.16 协议错误体

协议面错误体由 RFC 6749/6750 专用写入器输出（`oauth_error.go`），不复用应用错误体；应用错误码全表见 `docs/generated/errors-registry.md`。

## 14.17 协议覆盖矩阵

协议覆盖以 [traceability-matrix.md](../../traceability-matrix.md) 与路由契约测试为准（互链见 §14.15）。

## 14.18 Pushed Authorization Requests（PAR，RFC 9126）

> **规划态章节**：运行期无 PAR 端点与 `request_uri` 处理（复现：`grep -rn 'request_uri' internal/` 零命中）；本节为目标态规范。

PAR 把授权请求参数前置到 backchannel 端点，前端只携带 `request_uri`，降低授权 URL 长度与参数篡改面。

### 14.18.1 端点与请求/响应

请求参数与授权端点一致（`client_id` / `redirect_uri` / `response_type=code` / `scope` / `state` / `code_challenge`(S256) / `resource`）；响应 `{request_uri, expires_in}`；错误 `invalid_request` / `invalid_client`；授权端点仅接受 `request_uri` + `client_id`。
### 14.18.2 request_uri TTL 建议值表

TTL 默认 90 秒（建议区间 60–600 秒），一次性消费；前缀 `urn:ietf:params:oauth:request_uri:`；存储复用短期状态设施，容量不足按 503 fail-close。
### 14.18.3 客户端级开关、discovery 与降级策略

客户端级开关 `par_required`（默认 false）；discovery 宣告 `pushed_authorization_request_endpoint` 与 `require_pushed_authorization_requests`；端点不可用时拒绝授权请求（fail-close），不回退为明文参数。

## 14.19 JWT Secured Authorization Request（JAR，RFC 9101）

> **规划态章节**：运行期无 request object 提交与校验链；本节为目标态规范。

JAR 以签名 request object 承载授权参数，参数完整性与来源由客户端签名保证。
### 14.19.1 request object 提交与校验链

request 为 JWS（RS256/ES256/PS256），`kid` 必须命中客户端注册 JWKS；校验链：签名 → `iss=client_id` → `aud=issuer` → `exp/iat` → `jti` 重放 → 与 PAR 叠加时的冲突检测；任一失败即 `invalid_request_object`。
### 14.19.2 错误与叠加关系

错误 `invalid_request_uri` / `invalid_request_object`；JAR 与 PAR 可叠加（PAR 承载 request_uri，JAR 承载签名对象）；明文参数与同名 request object 参数不得并存，冲突时拒绝。

## 14.20 JWT Secured Authorization Response Mode（JARM）

> **规划态章节**：运行期 `response_mode` 仅支持 `query`/`fragment`（discovery 宣告同集合），无 `.jwt` 变体；本节为目标态规范。

JARM 以签名 JWT 返回授权响应，响应完整性由 issuer 签名保证。
### 14.20.1 response_modes 与 discovery

`response_mode` 仅允许 `query.jwt` / `fragment.jwt` / `form_post.jwt`；discovery 宣告 `response_modes_supported` 与 `authorization_signing_alg_values_supported`；JWT 携带 `aud=client_id` 与短时 `exp`。
### 14.20.2 与 redirect 模式的兼容矩阵

同一请求只能声明一种 `response_mode`；`query`/`fragment` 与其 `.jwt` 变体遵循同一错误回跳规则；无法签名时返回 self-hosted 错误页（fail-close），不得降级为明文响应。

## 14.21 Client-Initiated Backchannel Authentication（CIBA，OpenID CIBA 1.0）

> **规划态章节**：运行期无 CIBA backchannel 端点与 `auth_req_id` 状态机（复现：`grep -rni 'ciba' internal/` 零命中）；本节为目标态规范。

CIBA 用于无浏览器设备的带外认证：认证设备完成确认，发起端轮询换取令牌。
### 14.21.1 backchannel 端点与请求三件套

请求携带 `login_hint` / `binding_message` / `scope`（可选 `user_code`）；响应 `auth_req_id` / `expires_in` / `interval`；轮询 token 端点使用 `grant_type=urn:openid:params:grant-type:ciba`。
### 14.21.2 认证设备侧与通知

认证设备接受 push 通知并展示 `binding_message`；用户批准/拒绝写审计；通知不可达时回落到端内轮询状态。
### 14.21.3 状态机与 TTL

状态机：pending → approved / denied / expired；`auth_req_id` 一次性；TTL 与轮询间隔以注册表为准；超时返回 `expired_token`。
### 14.21.4 三模式与速率限制

poll / ping / push 三模式按客户端注册选择；速率限制按 `client_id` 与 `auth_req_id` 双维；轮询过快返回 `slow_down`。

## 14.22 Rich Authorization Requests（RAR，RFC 9396）

> **规划态章节**：运行期无 `authorization_details` 解析（复现：`grep -rn 'authorization_details' internal/` 零命中）；本节为目标态规范。

RFC 9396 `authorization_details` 以结构化对象表达细粒度授权；目标态支持 type 白名单（如 payment_initiation），未知 type 返回 `invalid_request`；授权详情随 code 与 token 传递并在令牌中保留声明；与 `scope` 并存时取交集。

## 14.23 Device Authorization Grant 补全（RFC 8628）
> Grant type 已在 §14.13.2 能力宣告中声明支持。

### 14.23.1 实现落点

实现见 `internal/app/auth/device_auth.go` 与认证面 device 端点；能力宣告见 §14.13.2。

**设备端匿名可达**：`/api/v1/auth/device/*` 的 device_code 申请与轮询由**无浏览器会话的设备**发起，不得要求会话；仅用户侧 user_code 确认要求会话。认证面公开白名单必须显式覆盖这两个端点（精确到方法 + 路径模板），不得归入「需会话但跳过 Casbin」名单。

### 14.23.2 轮询状态机与 slow_down 规则

轮询状态机：`authorization_pending` → `slow_down`（间隔递增 5s）→ 终态 approved / denied / `expired_token`；同一 `device_code` 的并发轮询按首次消费串行化。
### 14.23.3 user_code 展示与输入 UX

`user_code` 为 8 字符 `XXXX-XXXX`、无混淆字符；输入页自动大写与连字符；错误输入按 IP 与 device_code 双维限速。
### 14.23.4 verification_uri_complete 与双维限速

`verification_uri_complete` 携带 user_code 一次直达；限速按 IP 维与 device_code 维叠加；QR 展示与短链同源于 issuer。

## 14.24 OAuth 2.0 Token Exchange（RFC 8693）
> Grant type 已在 §14.13.2 能力宣告中声明支持。

### 14.24.1 实现落点

实现见 `internal/app/auth/token_exchange.go`（RFC 8693 委派语义）。

### 14.24.2 全参数与目标态

全参数：`subject_token` / `subject_token_type` / `actor_token` / `requested_token_type` / `audience` / `resource` / `scope`；仅允许服务端客户端发起；拒绝超过 2 跳的链式委托；返回值保留 `act` 声明。

## 14.25 JWT 客户端认证（RFC 7523）

RFC 7523 定义 JWT 客户端认证；本系统仅支持 @@private_key_jwt@@（@@client_secret_jwt@@ 不支持，见 @@OPEN-API-60@@）。

### 14.25.1 断言 claims 与支持范围

| 项 | 规范 |
|---|---|
| 支持方式 | 仅 `private_key_jwt`（`client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer`，签名算法 RS256/ES256/PS256） |
| 不支持 | `client_secret_jwt`（OPEN-API-60 已关闭）：DCR 传该值返回 400 `invalid_client_metadata`，控制台创建返回 422 `unsupported` |
| claims | `iss`=`sub`=client_id；`aud`=issuer 或 token endpoint 绝对 URL；`jti` 唯一；`exp` ≤5min；`iat` 必填 |
| 密钥 | 客户端注册的 JWKS 提供公钥；`kid` 必须命中其 JWKS；验签算法必须命中算法白名单 |
| 时钟 | 允许 ±60s 偏移；`exp`/`iat` 任一越界即拒绝 |

### 14.25.2 jti 重放缓存与语义区分

- `jti` 一次性：重放缓存 TTL = `exp` 窗口，重复出现一律 401 `invalid_client`。
- 与 DPoP `jti` 分命名空间去重，不得共用去重键或共享 TTL 语义。
- 重放存储不可用 → fail-close（拒绝请求），不回退为「不校验」。
- 断言缺失/畸形/算法不在白名单 → 401 `invalid_client`，不泄露内部原因。

## 14.26 mTLS 客户端认证与证书绑定（RFC 8705）

| 项 | 规范 |
|---|---|
| 证书来源 | 默认只认进程内 TLS 的 `r.TLS.PeerCertificates`；仅当 `MTLS_PROXY_CLIENT_CERT_HEADER` 非空且直连 peer 命中 `TRUSTED_PROXY_CIDRS` 时才读取转发证书头 |
| 认证方式 | `tls_client_auth`：以 `TLS_CLIENT_CA_FILE` 根校验客户端证书链；`self_signed_tls_client_auth`：按客户端登记的证书指纹匹配 |
| 证书绑定 | 令牌 `cnf.x5t#S256` = SHA-256(客户端证书 DER)；资源端校验令牌绑定与本次证书一致，不一致即 401 |
| 失败姿态 | 无证书/链校验失败/指纹不匹配/证书过期 → 401 `invalid_client`；fail-close，不回退到 secret 认证 |
| 轮换 | 绑定证书轮换必须有重叠窗口：窗口内新旧证书均可验证，旧证书在窗口结束后失效 |
| 代理边界 | 代理必须自行校验客户端证书并覆盖（不得追加）转发头；应用层在配置 CA 时仍复验链 |

## 14.27 DPoP 服务端 nonce 与共享重放防护（RFC 9449）
> 实现位置 internal/platform/dpop/。

### 14.27.1 实现落点

实现见 `internal/platform/dpop/`（证明校验与共享重放防护）与 `internal/infra/{postgres,mysql}/dpop/`（双方言重放存储）。

### 14.27.2 服务端 nonce 与共享存储

服务端 nonce 由 issuer 签发并缓存（TTL ≤ 5 分钟），经 `DPoP-Nonce` 头返回；`jti` 去重存储不可用时拒绝请求（fail-close）；共享存储按区域独立。

运行期落点：nonce 存储经 `NonceStore` 接口双实现——配置 Redis 会话缓存时走 `RedisNonceStore`（SET NX 签发 / GETDEL 原子一次性消费，跨副本互认，Redis 错误 fail-close）；未配置时回退进程内 `LocalNonceStore`（启动 Warn 宣告单实例假设，多副本边界见 operations/deployment.md §3.6）。装配与 SAML 重放防护同构（复用 session-cache Redis 连接）；短期状态登记见 short-lived-state.yaml `dpop_nonce`（OPEN-API-86 已关闭）。

## 14.28 Dynamic Client Registration 管理（RFC 7591/7592）
> 实现位置 internal/transport/http/api/v1/dcr/。

### 14.28.1 实现落点

实现见 `internal/transport/http/api/v1/dcr/`。

### 14.28.2 目标态

目标态：RFC 7591 注册 + RFC 7592 管理；`registration_access_token` 只存哈希；`client_secret` 一次性返回并存 Argon2id；`redirect_uri` 精确匹配；`software_statement` 验证为预留决策项。

## 14.29 FAPI 2.0 Security Profile 对齐

> **规划态章节**：FAPI profile 依赖 PAR/JAR（§14.18/§14.19，均为规划态）；运行期 discovery 不宣告 `fapi_profile_supported`。本节为目标态规范。

FAPI 2.0 对齐项：强制 PAR + JAR、mTLS 或 DPoP 绑定、code binding、S256 强制、无 implicit/hybrid；与既有 PKCE 基线叠加；discovery 以 `fapi_profile_supported` 区分是否满足 profile。
