# security-model — 安全红线 + 安全架构 + 风险引擎 + Workload 身份

> 目标态规范「security-model — 安全红线 + 安全架构 + 风险引擎 + Workload 身份」；写作约束见 [docs/README.md](../README.md)。

## 1. 安全红线（§6.6）

- 浏览器端不得持有长期 Refresh Token；优先 HttpOnly、Secure、SameSite Cookie
- OAuth/OIDC 浏览器和移动端**只使用 Authorization Code + PKCE S256**；**禁止 implicit grant**（`response_type=token`/`id_token` 一律返回 `unsupported_response_type`，discovery 仅宣告 `["code"]`）。PKCE 对**全部客户端强制**（含机密客户端），授权码取 256 bit `crypto/rand`，一次性消费 TTL 5 分钟，重复兑换即吊销该码签发的全部令牌
- 新密码只使用 Argon2id；遗留算法仅用于验证并在成功登录后升级
- Token、Cookie、密码、MFA Secret、私钥和个人证件信息不得进入日志
- 上传、Webhook、外部 URL 和同步器必须限制大小、目标地址、协议、超时和重试次数，防止 SSRF 与资源耗尽
- **BFF（Backend for Frontend）边界**：浏览器端只与同源后端交互，令牌**不得**下发到 `localStorage`/`sessionStorage`。HttpOnly Cookie 会话承载浏览器路径；长期刷新令牌**仅存服务端**，前端不持有任何可长期使用的凭据

### 1.1 安全不变量清单（S-01…S-21）

> 本节是安全不变量的**唯一可枚举清单**：每条给出声明锚点、AC 覆盖与实现落点。§7 的加固项（S1-S14）与 §8 的不得回退基线（B1-B10）是执行与验收附件，不重复计入本条数。

| ID | 安全不变量 | 声明锚点 | AC 覆盖 | 落点 |
|---|---|---|---|---|
| S-01 | 三类密码学资产（随机串 / 密钥对 / 证书）各有独立存储形态与读面，不得混用 | 本节；§2.7 | （待补 AC） | `contracts/schema/20_identity.yaml`（`client_secret_hash`）、`contracts/schema/40_session.yaml`（`token_hash`）、`contracts/schema/80_governance.yaml`（`keys`/`certs`） |
| S-02 | 私钥只以密文形态落库或仅在部署期注入，读接口与导出不得回传明文 | §2.7；AC-JWKS-001 | AC-JWKS-001 | `internal/app/keys/dto.go`、`internal/app/certs/mapper.go` |
| S-03 | 缺失密钥 fail-close：所有环境拒绝启动，无明文降级；签发路径不得改用占位密钥 | §2.7 红线；§2.21 | AC-KEY-003、AC-CRYPTO-001 | `internal/platform/crypto/field_encryption.go`、`internal/bootstrap/wire_jwks_keys.go` |
| S-04 | 所有 bearer 凭据只存哈希（可带前缀），明文仅在创建/轮换响应出现一次 | §2.17 | AC-TOKEN-013、AC-SESSION-002 | `internal/platform/crypto/token_hash.go`、`contracts/schema/40_session.yaml` |
| S-05 | 审计与业务同事务写入，`records` 以哈希链串联并锚定 | [ADR-031](../adr/ADR-031-region-residency-sm-crypto-audit-chain.md)；[audit.md](../design/infrastructure/audit.md) | AC-AUDITCHAIN-001、AC-AUDITCHAIN-002 | `contracts/schema/60_audit.yaml`、`internal/platform/outbox/providers/audit/chain.go` |
| S-06 | 租户隔离双防线：RLS FORCE 强制 + TenantFilter 作用域注入（StrictMode=false，未认证请求显式回落 built-in 供公开 discovery），无作用域即空结果 | §2.9.1；ADR-016、ADR-036 | AC-TENANT-003、AC-RLS-001、AC-RLS-002 | `contracts/registries/tenant-isolation.yaml`、`internal/platform/db/rls.go`、`internal/transport/http/middleware/tenant_filter.go` |
| S-07 | 授权 fail-close：Casbin/ReBAC 不可用或评估出错一律拒绝（503） | §2.8 | AC-PERM-001、AC-PERM-003、AC-PERM-004 | `internal/transport/http/api/v1/auth/auth_middleware.go`、`route.Definition.Permission` |
| S-08 | 跨租户访问统一返回 404，与「资源不存在」不可区分 | §2.9.1 | AC-TENANT-001、AC-TENANT-002 | `internal/transport/http/middleware/tenant_resolution.go`、`internal/transport/http/route` |
| S-09 | 公开白名单精确到「方法 + 路径模板」，禁止整前缀豁免 | §2.8 | AC-PERM-004 | `internal/transport/http/api/v1/auth/auth_paths.go`、`contracts/registries/surfaces.yaml` |
| S-10 | 拒绝与越权同样写审计，不存在「只拒绝不记录」路径 | §2.9 | AC-TENANT-001/002 的审计子句 | `internal/app/*/audit_outbox.go`、`records` |
| S-11 | 缓存只承载可重建数据，缺失回源，吊销事件驱动失效 | §2.8；ADR-010 | AC-TOKEN-004、AC-PERM-002 | `contracts/registries/cache.yaml` |
| S-12 | 一切外部副作用经 Outbox 异步投递，请求路径禁同步外呼 | ADR-008；[outbox.md](../design/infrastructure/outbox.md) | AC-OUTBOX-001/002/003、AC-EVENT-001/002 | `internal/platform/outbox`、`contracts/registries/outbox.yaml` |
| S-13 | 令牌轮换绑定 family，重用即整族撤销 | §2.17；ADR-013 | AC-TOKEN-001、AC-TOKEN-005 | `contracts/schema/40_session.yaml`、`internal/infra/postgres/tokens/sql.go` |
| S-14 | DPoP jti 去重 fail-close：去重存储不可用即拒绝 DPoP 请求 | §2.5 | AC-AUTH-005、AC-TOKEN-015 | `internal/platform/dpop`、`contracts/registries/short-lived-state.yaml` |
| S-15 | JWT 签名算法白名单（禁 none/HS*），ID Token 算法由应用封闭枚举选择 | §2.6；§8 B4 | AC-TOKEN-007、AC-TOKEN-012 | `internal/platform/jwks/allowed_algorithms.go`、`internal/platform/jwks/id_token_algorithms.go` |
| S-16 | 密钥轮换重叠窗口 ≥ 2 × max token TTL；retired 后宽限期结束才物理删除 | ADR-015；§2.17 | AC-JWKS-001、AC-JWKS-002、AC-TOKEN-007 | `contracts/registries/lifecycle.yaml`（`rotation_overlap_days`） |
| S-17 | 审计链锚定 + WORM 归档，链段闭环前禁止物理删除 | ADR-031；[audit.md](../design/infrastructure/audit.md) | AC-AUDITCHAIN-002 | `contracts/schema/60_audit.yaml`（`audit_chain_anchors`） |
| S-18 | 个人数据与审计数据按区域落盘，不跨区域复制 | ADR-031；[multi-region.md](multi-region.md) | AC-DR-002 | `contracts/schema/10_tenant.yaml`（`organizations.region_code`） |
| S-19 | 平台管理员跨租户 scope 有效性 = 未过期 && (is_bootstrap ‖ approved_by 非空) | [permissions.md §7.4](../design/iam/permissions.md#74-作用域有效性判定口径)；ADR-036 | （待补 AC） | `contracts/registries/lifecycle.yaml`（`platform_admin_scopes`）、`internal/infra/postgres/platform_admins/sql.go` |
| S-20 | 密码存储用 Argon2id；平台基线 ≥14 位 + 密码历史 + 过期 + 泄露检查 | §2.11、§2.12 | AC-AUTH-003（复杂度）；历史/过期/泄露检查（待补 AC） | `internal/app/users/password_validation.go`（复杂度与历史）、`internal/domain/organizations/password_policy.go`（`DefaultPasswordMinLength`）、`internal/app/auth/password_state.go`（过期） |
| S-21 | 账号锁定：15 分钟窗口 5 次失败 → 锁定 30 分钟，锁定判定先于密码校验 | §2.18 | AC-AUTH-002、AC-AUTH-006 | `internal/app/auth/account_lockout.go`、`contracts/schema/80_governance.yaml` |

## 2. 安全架构（§8）

### 2.1 URL 安全校验

前端 `shared/lib/url.ts` 提供三层 URL 安全验证：

| 函数 | 用途 | 安全保障 |
|------|------|---------|
| `isSafeRedirectUrl(url)` | 内部重定向校验 | 拒绝 `//evil.com`、`javascript:`、`data:`、跨域 URL |
| `safeRedirectUrl(url, fallback)` | 安全重定向（带降级） | 不安全时回退至 `/` |
| `isSafeExternalUrl(url)` | 外部链接校验 | 仅允许 `http:` / `https:` 协议 |

### 2.2 SAML RelayState 签名验证（后端）

RelayState 的签名与校验**只发生在后端**，密钥不进入浏览器产物：

- 签发：`internal/infra/saml.RelayStateSigner` 在 SP 发起路径（`BuildSPAuthnRequest`）输出 `v1.<issuedAtUnix>.<hexHMAC>.<original>` 信封。
- 校验：SP 消费路径（`HandleSPInitiatedACS`）校验签名、10 分钟 TTL 与 `organizationSlug`/`applicationSlug` 绑定，常量时间比较。
- 密钥：由 `ENCRYPTION_KEY_CURRENT` 以标签 `guliangiam:saml-relay-state:v1` 做域分离派生，不新增部署变量。
- 前端：只透传 RelayState；`stripRelayStateEnvelope` 仅用于本地解析，不作为安全校验。
- IdP 角色（`HandleSAMLRedirect`、`handleLoggedInSAML`）：外部 SP 传入的 RelayState 必须原样回传（SAML 2.0 §3.4），不重签、不改写。

### 2.3 TLS 1.3 强制

- 生产环境 `MinVersion: tls.VersionTLS13`
- 密码套件：`TLS_AES_128_GCM_SHA256`, `TLS_AES_256_GCM_SHA384`, `TLS_CHACHA20_POLY1305_SHA256`
- 曲线偏好：`X25519`, `CurveP256`
- HTTP 请求 → 301 重定向至 HTTPS

### 2.4 HSTS Preload

`Strict-Transport-Security: max-age=31536000; includeSubDomains; preload`

### 2.5 DPoP 验证加固

| Enhancement | Detail |
|-------------|--------|
| Replay Cache | jti 去重存 PostgreSQL/MySQL `dpop_replay`（跨实例共享，唯一键 jti；实现 `internal/infra/postgres/dpop/replay.go`、`internal/infra/mysql/dpop/`），每区域独立；跨区域访问按重新认证处理；存储介质、TTL 与失败姿态的唯一事实源为 `contracts/registries/short-lived-state.yaml`（`dpop_jti`），矩阵见《data-model》§3.1「短期状态存储矩阵」 |
| Algorithm Whitelist | RS256, ES256, PS256 only |
| Max Proof Age | 60 seconds |
| Enforcement | DPoP-enabled app without proof → 401 `invalid_dpop_proof` |
| Replay Store 不可用 | fail-close：拒绝所有 DPoP 请求（不回退为「不校验」） |

### 2.6 JWT 签名算法白名单

允许与禁用的 JWT 算法由 `internal/platform/jwks/allowed_algorithms.go` 单一给出（复现：`grep -n 'AllowedSigningAlgorithms\|ForbiddenSigningAlgorithms' internal/platform/jwks/allowed_algorithms.go`）；本文不复写枚举清单。ID Token 的 `id_token_signing_alg_values_supported` 由 `internal/platform/jwks/id_token_algorithms.go` 单一给出；应用级选择由 `app_oauth_config.token_signing_method` 的封闭枚举（`RS256` / `SM2-SM3`）决定，默认 `RS256`。同一应用不得混签。JWKS 按算法族**分端点发布**：标准端点 `/.well-known/jwks.json`（及按应用变体）只发布国际算法族（RSA/ES），国密 SM2-SM3 族由独立端点 `/.well-known/jwks-gm.json` 发布，两族各自以 `rsa_`/`gm_` kid 前缀标识；OIDC Discovery 以扩展字段 `gm_jwks_uri` / `gm_alg_values_supported` 宣告国密通道。验签方按 header `kid` 前缀选择公钥：国际 RP 取 `jwks_uri`，国密 RP 取 `gm_jwks_uri`，未知或跨族 `kid` 一律拒绝。国密能力位与跨算法边界见 §2.21。

### 2.7 AES-256-GCM 字段加密

敏感字段使用 `internal/platform/crypto/field_encryption.go` 透明加密。**适用边界**——全仓库经 `NewEncryptedFieldMapper()` 接入字段加密的机密列；接入族与列清单为可生成事实，本文不复写，按事实源查询（复现：`grep -rln 'NewEncryptedFieldMapper' internal/infra/ | grep -v _test.go`）；轮换覆盖列全集以 `internal/platform/crypto/reencrypt_job.go` 的 `DefaultEncryptedColumns` 为唯一事实源。

provider 凭据由仓储 mapper 在写入前加密、读取时只接受 envelope 令牌；历史明文行读取 fail-closed，必须由 `go run ./cmd/migrate encrypt-provider-secrets` 回填（`-dry-run` 只统计、`-verify` 存在明文即非零退出）。密钥轮换覆盖的列以 `internal/platform/crypto/reencrypt_job.go` 的 `DefaultEncryptedColumns` 为唯一事实源，一致性由 `scripts/check-encrypted-columns.py` 校验；轮换必须发生在回填之后。

> **红线**：`ENCRYPTION_KEY_CURRENT` 缺失时**所有环境**拒绝启动（无明文降级路径，ADR-021）。`applications.client_secret` 使用 Argon2id 单向哈希存储，不属于 AES-256-GCM 加密适用场景。哈希 vs 字段加密的判定边界见《开发规范手册》。

### 2.8 Fail-Close 机制

授权系统处于不确定状态时，任何请求都不会被放行。逐项处置：

| Condition | Response | HTTP Status |
|-----------|----------|-------------|
| 授权引擎不可用 | 拒绝并告警 | 503 |
| 受保护路由无权限声明 | 启动自检失败；运行期 503 | 启动失败 / 503 |
| 无身份访问受保护资源 | 拒绝 | 401/403 |
| 管理面请求命中公开白名单 | 不允许前缀豁免，精确到「方法 + 路径模板」 | 403/404 |
| Casbin 评估错误 / 适配器超时 | 503 + 告警 | 503 |
| Casbin 评估成功 → 拒绝 | 403 | 403 |

**启动自检与公开白名单**：进程启动时遍历全部 `route.Definition`，对每条非公开路由断言 `Permission` 非空、且其资源/动作已在 `contracts/registries/permissions.yaml` 登记，任一违例即拒绝启动；公开路由必须命中 `contracts/registries/surfaces.yaml` 的 `public_whitelist`，匹配粒度为「方法 + 路径模板」，禁止整前缀豁免；管理面路径不得命中任何豁免项。

**Fail-Close 总则**：

1. 授权系统处于不确定状态时任何请求都不会被放行
2. 进程启动时对受保护组执行权限自检（任违例即拒绝启动）
3. 「无身份」「引擎不可用」「无权限声明」「命中白名单」四类放行分支全部关闭，以防回归测试钉死
4. 管理面路径不得命中任何豁免前缀
5. PEP 实现唯一，防止双实现行为漂移
6. 缓存类依赖不可用时高风险操作保守拒绝


### 2.8.1 拒绝审计的降级例外

写路径审计要求与业务同事务 fail-close（S-05）；**拒绝路径审计是唯一例外**：

- L1 显式打点：租户裁决/认证上下文（transport 中间件与 authtransport 助手，如 `TrustedOrganizationID` 观察到伪造/异租户 target）判定跨租户时经 `platform/auditdenial.Record` → `app/auditdenial.Recorder` 打点（调用点：`internal/transport/http/middleware/tenant_resolution.go`、`internal/transport/http/api/v1/auth/context.go`），正常 404 不冒充跨租户。
- L2 中间件兜底：只对 403 权限拒绝补记，404 不兜底（无法区分“不存在”与“跨租户”）。
- L3 去重：同一 `(actor, target, resourceType, reason)` 60s 内只落一条，重复次数计入内存/Redis 计数；Redis 不可用回退进程内。
- L4 独立限流：`RATE_LIMIT_AUDIT_DENIAL_RATE` 独立令牌桶；超限明细丢弃并告警，响应不受影响。
- L5 尽力而为：审计写入失败进入有界异步补偿队列重试；队列满/重试耗尽只递增 `guliangiam_audit_denial_dropped_total{reason}` 并告警，不把拒绝响应变成 503；与 AC-AUTH-002 的写操作审计不冲突。

对应验收：AC-AUDIT-DENIAL-001。

### 2.9 日志脱敏

`internal/platform/logging/sanitizer.go` — slog custom Handler，在日志输出前将以下类别替换为 `***`：

- Password、Access Secret、Tokens（accessToken/refreshToken）
- Client Secret、LDAP bindPassword、Agent token
- Crypto（privateKey/totpSecret）、Master 口令类
- PII（idCard）

日志字段口径：

- 访问日志（`internal/platform/middleware/access_log.go`）只记 `path` 的路由模板（UUID 与 query 不入日志）、`status`、`durationMs`、`bytes`、`surface`、`requestId`、`traceId`；探针端点不写访问日志。
- 业务日志记录主体用用户/租户标识（如 `username`、`organizationId`），不得把凭据、Secret、令牌、密码哈希写入字段值；`scripts/check-credential-masking.py` 与 `scripts/check-audit-actor.py` 是门禁。
- 写路径的审计主体必须是 UUID 形状的 actor（或显式匿名的系统主体），由 `scripts/check-audit-actor.py` 校验；调用点计数以脚本输出为准，本文不复写。

### 2.9.1 租户隔离的双路径（RLS 与显式过滤）

隔离强度取决于方言，目标态要求两条路径都成立：

| 路径 | 实现 | 适用 | 失败姿态 |
|---|---|---|---|
| RLS（PostgreSQL） | 租户表 `ENABLE` + `FORCE ROW LEVEL SECURITY`，策略谓词由 `contracts/schema` 声明；请求经 `set_config(..., true)` 注入事务级（LOCAL）作用域，事务结束自动失效；连接池 `AfterConnect` 钩子以 SESSION 级空值兜底（fail-closed，`internal/platform/db/rls.go`） | 全部带 `organization_id` 的表 | 无作用域 → 空结果（fail-close） |
| 显式过滤（MySQL 等无 RLS 方言） | 仓储语句显式 `WHERE organization_id = ?`；租户值来自 handler 的可信上下文，不由客户端决定 | 同上 | 缺失租户 → 拒绝执行 |

**handler 侧强制约束**：请求体/query/header 里的 `organizationId` 不是可信输入，必须经 `auth.TrustedOrganizationID` 解析；只有平台管理员（显式跨租户经 ScopeAuthorizer 批准后落到单一目标租户）才能指定目标租户。该规则把「MySQL 无 RLS」从依赖数据库兜底转成依赖代码契约。显式跨租户的 target 精确过滤决策见 [ADR-036](../adr/ADR-036-scope-aware-rls-hybrid.md)，实现跟踪 `OPEN-API-42`；`system` 与平台管理员无显式 target 的旁路必须写审计。

**中间件层兜底**：`TenantResolutionMiddleware` 把 query `organizationId` 与请求体/header 的
目标租户一并解析；非平台管理员使用他租户 slug（无法在中间件层解析为 UUID、ScopeAuthorizer
也无从授权）直接返回 403 `forbidden`，不进入 handler。本租户 UUID/slug 不标记为目标，避免
普通用户携带自身租户参数时被 ScopeAuthorizer 的「非 platform_admin 不得跨租户」规则误拒。

**E2E 限流边界**：浏览器五档扫描由同一 IP 高并发请求，测试后端必须显式覆盖
`RATE_LIMIT_PER_IP_REQUESTS` / `RATE_LIMIT_GLOBAL_RATE` / `RATE_LIMIT_AUTH_RATE`；
生产默认值不变，测试覆盖不得进入部署模板。会话读被 429 后前端跳登录属于**测试环境失真**，
E2E 必须判失败而不是 SKIP（`web/e2e/route-sweep.spec.ts`）。

### 2.10 API 响应过滤

增强版 `secret_masker.go` — 在日志脱敏类别之外新增：

- Password Hash、Password Salt、Password Algorithm（non-admin only）
- Access Secret Hash、Client Secret Hash、TOTP Secret

### 2.11 密码策略加固

- 组织级 `PasswordPolicy` 强制：MinLength/MaxLength/RequireUppercase/RequireLowercase/RequireDigit/RequireSpecial
- 密码历史检查：新密码与 `user_password_history` 表中最近 N 个哈希对比
- 密码过期：登录检查 `passwordExpireDays`，过期时设置 `NeedUpdatePassword=true`

### 2.12 密码哈希

| Algorithm | Usage | Notes |
|-----------|-------|-------|
| **argon2id** | **默认** | memory-hard，抗 GPU 攻击；自动升级遗留算法 |
| bcrypt | Legacy support | 兼容迁移账户 |
| PBKDF2-SHA256 | Legacy support | NIST 推荐回退 |
| PBKDF2-SHA512 | Legacy support | 更高安全变体 |
| SHA-256 | Legacy support | 仅兼容 |
| MD5 | Legacy migration | 仅用于导入遗留系统 |
| Plain | Testing only | 生产环境绝不使用 |

**算法自动升级**：登录时若检测到遗留算法，自动以 argon2id 重新哈希并持久化升级。

**国密边界**：国密档位不改变存储哈希，密码与客户端密钥固定使用 argon2id；组织安全策略只允许 `password_derive_algorithm` 在 `hmac_sha256` / `hmac_sm3` 间切换，SM3 只进入一次性验证码摘要的派生口径，不替代存储哈希。详见 §2.21。

### 2.13 Secret 轮换审计

`secret_rotation_audit` 表记录每次 Provider/Application/Agent 密钥轮换：actor_id、resource_type、resource_id、secret_type、rotated_at、ip、user_agent。

密钥版本与轮换机制见 ADR-015；KMS/Vault 根密钥外置见 ADR-009。

### 2.14 CSRF 保护

Double Submit Cookie 模式通过 `CSRFTokenMiddleware` 实现：

1. 服务端在 cookie 中签发 CSRF token（non-HttpOnly，JS 可读）
2. 客户端读取 cookie 并在自定义 header（`X-CSRF-Token`）中包含 token
3. 中间件比较 cookie 值与 header 值
4. 不匹配返回 `403 Forbidden`

### 2.15 WAF（Web 应用防火墙）

Coraza 引擎通过 `WAFMiddleware` 集成：

- 基于 OWASP Core Rule Set (CRS) 评估请求
- 阻断恶意载荷（SQL 注入、XSS、路径遍历等）
- 可配置规则集和严重性阈值
- **在认证中间件之前运行**（认证不应暴露安全载荷）

### 2.16 DPoP（Demonstration of Proof-of-Possession）

Token 绑定 + JWK 指纹验证：

1. 客户端生成临时密钥对，在 DPoP header 中包含公钥
2. 服务端验证 DPoP proof JWT 签名
3. Access token 绑定至 JWK 指纹（`cnf` claim）
4. 资源服务器验证 token 持有者拥有对应私钥
5. 防止 token 盗窃和重放攻击

### 2.17 Token 轮换与重用检测

Refresh token 轮换 + 族撤销：

1. 每次签发新 refresh token 时使旧 token 失效
2. Token 按族组织（`family_id` 关联）
3. 检测到重用（已失效 token）时撤销整个 token 族
4. 检测并缓解 token 盗窃：攻击者使用窃取的 token 后，合法客户端的下一次轮换将触发族撤销
5. Access token 和 refresh token 以 SHA-256 哈希存储；原始值创建后不再返回
6. Refresh token 密文以固定可识别前缀标记；落库只存 SHA-256 哈希，原始密文仅存在于换码端点的内存中，不落盘、不入日志

### 2.18 账户锁定

| Parameter | Value | Notes |
|-----------|-------|-------|
| Max failed attempts | 5 | Within the observation window |
| Observation window | 15 minutes | Rolling window from first failure |
| Lockout duration | 30 minutes | Account remains locked |
| Persistence | PostgreSQL（唯一事实源）；**每区域独立** | `lockout_attempts` 表单条 `INSERT ... ON CONFLICT` 原子状态机维护观察窗、计数与退避级别（`window_started_at`、`lockout_level`；`internal/infra/postgres/lockoutattempts/sql.go`、`internal/app/auth/account_lockout.go`）；不存在跨实例 Redis 计数层 |
| Backoff | 30 min → 6 h → 24 h | 反复触发时退避，成功登录后清零；首次锁定时长和阈值可由组织策略覆盖 |
| Reset on success | Yes | Successful login clears attempt counter |

### 2.19 速率限制（引用）

限流器参数唯一权威表在 `generated/limit-registry.md`；本节仅保留指向。各限流器（代码变量、维度、默认速率与 Burst、挂载面/挂载点、环境变量覆盖）与口径纪律均以该注册表为准；代码事实源为 `internal/platform/middleware/ratelimit.go`。分布式令牌桶目标见 ADR-019。

### 2.20 会话安全

| Property | Value | Notes |
|----------|-------|-------|
| Cookie HttpOnly | Yes | Prevents XSS-based token theft |
| Cookie Secure | Yes | HTTPS only |
| Cookie SameSite | Lax | Prevents CSRF while allowing top-level navigations |
| Session storage | Server-side (PostgreSQL) | Cookie contains only session ID |
| Session expiry | Configurable per application | Idle + absolute timeout |
| Session state machine | 见 ADR-013 | 固定/劫持防护 |

### 2.21 国密能力位与密码学边界（ADR-031）

国密是可选档位，不替换国际算法默认值。能力位 ID、缺失策略、档位取值与探测口径的唯一事实源是 `contracts/registries/crypto.yaml`（渲染层 `docs/generated/crypto-registry.md`，生成命令 `python3 scripts/gen_registry_md.py`；启动裁决 `internal/platform/crypto/suite.go`）；部署开关与材料变量登记于 `contracts/registries/env.yaml`。本节只定义不变量与边界，不复写能力位或变量清单。

| 边界 | 不变量 |
|------|--------|
| 档位裁决 | `CRYPTO_PROFILE=gm` 时 `failure_mode=refuse` 的能力位任一缺失即拒绝启动，禁止静默降级为国际算法；`gm_tls` 为 `failure_mode=degrade`，缺失只把 `/healthz` 标记为 `degraded` 并给出原因，不阻断启动。能力位一致性由 `python3 scripts/check-registry-sync.py --strict` 校验。 |
| 传输层 | `gm_tls` 是部署层能力位：只做双证书、私钥与受信 CA 材料自检，不实现 TLCP 握手。启用开关为真不等于可用：签名证书、加密证书、对应私钥、SM2 公钥与 SM2-SM3 签名、私钥匹配、有效期与 CA 链任一不通过即不可用。`gm_tls` 只影响传输层，不改变业务密码学；NCI 通道见 [design/auth/network-identity.md](../design/auth/network-identity.md)。 |
| 密码存储 | 密码与客户端密钥的存储哈希固定 Argon2id，不提供国密存储哈希，也不新增 `password_hash_algorithm`。组织安全策略只允许 `password_derive_algorithm` 在 `hmac_sha256` / `hmac_sm3` 间切换；其唯一消费点是一次性验证码摘要的 HMAC-SM3 派生，不是密码存储哈希选择器。事实源 `contracts/schema/10_tenant.yaml`。 |
| 令牌签名 | 应用 OAuth 配置 `app_oauth_config.token_signing_method` 是封闭枚举 `RS256` / `SM2-SM3`，默认 `RS256`；同一应用内不得混签，且该选择只作用于 ID Token。SSF SET 与 MCP access token 等系统级 JWT 载体固定 `RS256` / `rsa_`，不参与 `token_signing_method`。JWKS 按算法族分端点发布：标准端点 `/.well-known/jwks.json`（及按应用变体）只发布国际算法族（RSA/ES）公钥，国密 SM2-SM3 族公钥由独立端点 `/.well-known/jwks-gm.json` 发布（进程本地 legacy 模式两族均生成，但分别经各自端点发布；key-ring 模式按实际配置族发布，缺失族对应端点为空集）；`kid` 前缀区分算法族，验签方按令牌 header 的 `kid` 前缀在对应通道选择公钥（国际 RP 取 `jwks_uri`，国密 RP 取 Discovery 扩展字段 `gm_jwks_uri`），未知 `kid`、跨族 `kid`、算法与密钥族不匹配一律拒绝。标准端点恒不含 `crv=SM2` 密钥，以保证严格 JOSE 客户端（如 Nimbus）可解析。事实源 `contracts/schema/20_identity.yaml`；实现 `internal/platform/jwks`。 |
| 审计链 | `records.chain_algo` 封闭为 `sha256` / `sm3`；新链首条按档位选择，后续记录继承链尾算法，同一链禁止混用；校验器对未知算法报错而非回退默认值。事实源 `contracts/schema/60_audit.yaml`；实现 `internal/platform/crypto/chain_algo.go`。 |
| 字段加密 | 默认 AES-256-GCM；SM4-GCM 是显式可选能力位。信封 token 头部携带算法标识，读路径按 token 选择 cipher；切换窗口内旧算法仅用于解密。缺密钥或算法不可用时拒绝启动，不存在明文写入路径。事实源 `contracts/registries/env.yaml` 与 `internal/platform/crypto/field_encryption.go`。 |
| NCI 通道 | 国家网络身份认证适配器使用 `gm_tls` 双证书与 SM2+SM4 数字信封；该通道为规划态，端口、失败姿态与审计口径见 [design/auth/network-identity.md](../design/auth/network-identity.md)。NCI 的国密要求不改变上述业务密码学边界。 |

### 2.21.1 实现边界与命名

- **签名配置字段**：`token_signing_method` 位于 `app_oauth_config`（`contracts/schema/20_identity.yaml`），封闭枚举 `RS256` / `SM2-SM3`，ID Token 算法选择全链路以该字段为准；不存在 `applications.token_signing_algorithm` 列，应用级配置经 `application_id` 关联读取。
- **密码存储算法**：`password_hash_algorithm` 无表、无 DDL，且不新增；密码与客户端密钥固定 Argon2id。`organization_security_policies.password_derive_algorithm`（`hmac_sha256` / `hmac_sm3`）的唯一消费点是一次性验证码摘要的 HMAC-SM3 派生，不是密码存储算法选择器。
- **SM2-SM3 签名实现**：JWS 签名由 `internal/platform/crypto` 与 `internal/platform/jwks` 手写实现，不实现 `jwt.SigningMethod` 适配；issuer 按令牌 header `alg` 分派，并按 `kid` 前缀隔离算法族。
- **数字信封边界**：`sm2_encrypt` 是 `CRYPTO_PROFILE=gm` 的启动能力位（`contracts/registries/crypto.yaml`），不是独立可交付项；数字信封业务落地仅随国家网络身份认证（NCI）批次，规划态边界见 [design/auth/network-identity.md](../design/auth/network-identity.md)。

验证锚点见 [acceptance-criteria.md](../acceptance-criteria.md) 的 CRYPTO 小节；`gm_tls` 材料自检的降级语义由 `internal/platform/crypto/gm_tls_test.go` 覆盖。

## 3. 风险引擎与持续访问评估（ADR-027）

风险管线覆盖八个环节：

| 环节 | 内容 | 关键设计 |
|------|------|---------|
| 信号采集 | 设备指纹（哈希存储）、IP 信誉与 ASN、地理速度（不可能旅行）、撞库/密码喷洒 | 信号端口可插拔（domain 端口 + infra 适配器）；与既有 `lockout_attempts` 叠加不替换 |
| 规则评分 | 规则引擎先行（确定性规则+阈值，版本化、可灰度、可回滚）；ML 评分为可插拔接口 | 规则命中/决策分布可观测；规则变更有审计 |
| 分级决策 | low/medium/high/critical 四级 → 放行 / step-up / MFA 加固 / 拒绝+通知 | 阈值租户可配；信号输入缺失按保守默认决策（fail-close） |
| 会话治理 | 租户可配会话并发策略；管理员强制下线 API（高危 step-up + 审计） | 被踢会话与 refresh family 即时吊销 |
| 持续访问评估 | 会话生命周期内周期/事件驱动重估 | 评估框架与策略输入复用 ReBAC ABAC 属性目录 |
| 事件下推 | CAEP/SSF：RFC 8935/8936 向已注册接收方下发会话/令牌失效类事件 | 签名、重试与死信走 Outbox 语义 |
| 审计保全 | `records` 构建哈希链 + 链头/链尾定期快照锚定 + WORM 归档 | 链校验工具可发现单点篡改 |
| 安全运营 | 异常登录通知、登录历史自助导出、PSIRT 流程 | 通知风暴被发送侧限流抑制 |

**决策点接入**：风险分级决策作为授权决策顺序的**前置输入**之一（环境属性注入），并作为认证流程的 step-up 分支。

**信号缺失默认决策**（保守策略，以下表为唯一事实源）：

| 缺失信号 | 默认决策 | 理由 |
|---------|---------|------|
| 设备指纹 | 强制 MFA | 无指纹无法判断是否为可信设备，升级 step-up |
| IP 信誉 | 强制 MFA | 无信誉评级时保守拒绝低置信度认证 |
| 地理速度 | 放行 + 审计标签 | 不可能旅行检测信号不可用时不阻断，但标记审计待运营复核 |
| ASN | 放行 + 审计标签 | ASN 仅作辅助属性 |
| User-Agent | 放行 + 审计标签 | UA 可伪造，不影响核心决策 |

> 缺失信号决策阈值与覆盖范围**以本节下表为唯一事实源**；运行时日志需结构化记录命中默认决策的信号名与最终决策。

## 4. Workload 身份平面（ADR-026）

### 4.1 主体模型

Service Account（SA）作为**非人主体一等公民**，与人类用户并列参与认证与授权全链路：

- 独立生命周期（创建/轮换/禁用/删除，删除前预检关联凭据与授权）
- 属主（用户或组织）
- 可绑定角色与组
- 独立审计身份（审计中与人类用户可区分，且不可登录交互式 UI）

### 4.2 联邦信任与身份池

外部 OIDC Issuer（云厂商/CI）注册为信任源——校验 issuer、JWKS、aud 白名单与 **subject 白名单**（精确或受控通配，如 `repo:org/app:ref:main`）；信任仅授予声明的 subject，未命中拒绝并审计；信任配置变更双审。

身份池把一组外部联邦身份映射到平台内角色/权限集：池定义信任源、subject 映射规则、角色映射与作用域上限；池是联邦信任的授权出口。

### 4.3 与令牌通道的合流

- SA 走 client_credentials（协议面令牌端点承载）为「直接凭证」路径
- 联邦信任经 Token Exchange（协议面 `POST /api/v1/auth/tokens:exchange`）为「外部身份换发」路径
- 两条路径产出同一令牌语义（相同 claim 框架、可区分 actor/subject）

### 4.4 短期凭据与凭据主权

- 联邦换发令牌短时效（默认 ≤ 1 小时）
- 静态 Access Key/Secret 仅在不可联邦场景经审批保留并强制轮换周期
- **禁止新增长效 secret 用例**（门禁拦截、豁免留痕）
- 凭据使用记录（who/where/when）可查询

### 4.5 SPIFFE 兼容方向

SA/身份池身份命名采用可映射到 SPIFFE ID（`spiffe://trust-domain/path`）的构造，映射规则文档化；SPIFFE SVID 验证为后续演进接口预留。

## 5. 权限决策基础引用

- Casbin 多租户边界：ADR-003
- RLS 租户隔离：ADR-016；显式跨租户的 target 精确过滤：ADR-036
- PKCE S256 强制：§1 安全红线
- PDP/PEP/PIP 映射与 fail-close：《layers》§5.1
- RBAC/ABAC/ReBAC 三引擎分工：《layers》§5.2

## 6. 等保 2.0 三级合规口径

本节只登记等保条款到控制类的承接关系，不登记实现状态、计数或覆盖率：

- **能力就绪**：唯一事实源 `contracts/registries/security-readiness.yaml`；人类可读渲染层 [security-readiness](../generated/security-readiness.md)，生成命令 `python3 scripts/gen_registry_md.py`。
- **能力缺口与出口条件**：唯一事实源 `contracts/registries/open-items.yaml`；渲染层 [open-item-registry](../generated/open-item-registry.md)。
- **测评证据**：逐控制点的代码位置与验证标准取 `security-readiness` 的 `design_anchor` / `ac`（对应验收条件），本文不复写第二份清单。

| 等保控制类 | 承接章节与能力域 |
|-----------|------------------|
| 身份鉴别 | §2.2 SAML 校验、§2.3 TLS、§2.5/§2.16 DPoP、§2.11~§2.12 密码策略与哈希、§2.17~§2.18 令牌与锁定、§2.20 会话；[webauthn.md](../design/auth/webauthn.md) |
| 访问控制 | §2.8 fail-close、§5 权限决策基础；[ADR-003](../adr/ADR-003-casbin-tenant-boundary.md)、[ADR-016](../adr/ADR-016-rls-tenant-isolation.md)、[ADR-024](../adr/ADR-024-rebac-relation-tuple-authz.md) |
| 安全审计 | §3 审计保全与事件下推；[ADR-031](../adr/ADR-031-region-residency-sm-crypto-audit-chain.md)、[audit.md](../design/infrastructure/audit.md) |
| 入侵防范 | §2.14 CSRF、§2.15 WAF、§2.19 速率限制、§3 风险引擎 |
| 安全计算环境 | §2.7 字段加密、§2.9 日志脱敏、§2.12 密码哈希；[ADR-022](../adr/ADR-022-declarative-schema-versioned-migration.md) |
| 安全区域边界 | §2.1 URL 安全校验、§2.15 WAF；[ADR-012](../adr/ADR-012-openapi-source.md) |
| 安全通信网络 | §2.3 TLS 1.3、§2.4 HSTS、§2.16~§2.17 DPoP 与令牌轮换 |
| 数据完整性 | §2.16 DPoP、§3 审计哈希链；[ADR-015](../adr/ADR-015-key-version-rotation.md)、[ADR-018](../adr/ADR-018-unified-idempotency.md) |
| 数据保密性 | §2.7 AES-256-GCM、§2.12 密码哈希；[ADR-009](../adr/ADR-009-kms-vault.md)、[ADR-021](../adr/ADR-021-environment-variable-semantics.md) |
| 剩余信息保护 | §2.12 凭据生命周期、§2.20 会话安全 |
| 通信完整性 | §2.3 TLS 1.3、§2.16 DPoP |
| 通信保密性 | §2.3 TLS 1.3、§2.4 HSTS |
| 软件容错 | §2.8 fail-close、§3 风险引擎 |
| 资源控制 | §2.18 账户锁定、§2.19 速率限制、§2.20 会话并发；`contracts/registries/limits.yaml` |

> 等保测评如需逐控制点证据，从 `security-readiness.yaml` 各条目的 `design_anchor` 与 `ac` 字段提取；能力缺口的验收由 `open-items.yaml` 的 `close_condition` 裁定，本表不承载状态、计数或百分比。

## 7. 安全加固项（目标态要求）

以下条目是安全审计归纳的目标态设计约束；实现状态以 `contracts/registries/security-readiness.yaml` 与对应 ADR 为唯一口径。

| # | 加固要求 | 设计约束 | 落点 |
|---|---------|---------|------|
| S1 | SAML SSO 身份来源 | 身份必须由签名会话/JWT 派生，禁止采信 `X-Organization-Slug` / `X-Username` / `X-Access-Token` 等原始请求头；无有效会话一律判未认证（fail-close） | `internal/transport/http/api/v1/saml/chi.go` |
| S2 | SAML ACS 地址 | 断言消费者地址必须命中应用已注册回调白名单（scheme+host+path 完全相等）；`AuthnRequest` 中的 ACS 仅作候选，不匹配即拒绝并审计 | `internal/app/auth/saml.go` |
| S3 | 资源上传 | 落盘前按「扩展名白名单 + 服务端内容嗅探」双重校验，拒绝 HTML/SVG/JS 等主动内容；目标路径与归属由服务端按会话派生，不得读 query；`/uploads/*` 输出强制 `X-Content-Type-Options: nosniff` | `internal/app/resources/upload_resource.go`、`internal/bootstrap/chi.go` |
| S4 | 读面凭据不回显 | 组织/Provider/Adapter 等详情接口对管理凭据一律采用 `XxxSet` + `XxxMasked` 遮蔽范式，禁止明文回显 `masterPassword` / `masterVerificationCode` / `defaultPassword` / `kerberosKeytab` | `internal/app/organizations/dto.go` 等 |
| S5 | CSRF cookie 名 | 前端读取的 CSRF cookie 名必须同时覆盖 `__Host-csrf`（生产）与 `csrf_token`（开发），与后端 `csrfCookieName()` 保持一致 | `web/src/shared/api/http.ts`、`internal/platform/middleware/csrf.go` |
| S6 | CSP 强制 | 为 SAML POST binding 页面生成一次性 nonce（或脚本哈希）后恢复 CSP 强制；不得长期停留在 Report-Only | `internal/transport/http/api/v1/saml/chi.go`、`internal/transport/http/middleware/security_headers.go` |
| S7 | 出站请求 | 所有出站 HTTP（JWKS/SCIM/actions 代理等）统一使用 SSRF 安全 Transport + 发送前 URL 校验；host 白名单作为额外一层而非替代。SCIM 连接器默认走 SSRF 安全 Transport，仅当必须访问内网目标时显式 `SCIM_ALLOW_PRIVATE_TARGETS=true`（trade-off：放宽内网可达性以支持本地部署） | `internal/platform/ssrf/ssrf.go`、`internal/outbound/scim/client.go` |
| S8 | CSRF 豁免 | 豁免必须基于「已被鉴权中间件验证有效」的 Bearer/JWT，不得仅凭 `X-Access-Token` 非空 | `internal/platform/middleware/csrf.go` |
| S9 | 不可信 CSS | 渲染不可信 CSS 采用「结构性逃逸阻断 + 正则黑名单」：按顺序移除注释、`<`/`>` 与反斜杠，再剔除 `@import`、`url()`、`expression()`、`javascript:`、`vbscript:`、`behavior:`、`-moz-binding`；任何 `<`/`>` 都不得进入 `<style>` 文本。实现必须满足上述阻断规则；解析式白名单重建（CSSOM）为强制目标形态 | `web/src/shared/lib/dom-purify.ts` |
| S10 | postMessage | 通配 targetOrigin `"*"` 收敛为已知父页来源白名单；消费方校验 `event.origin` / `event.source` | `web/src/shared/hooks/useAutoSigninFlow.ts` |
| S11 | 外链安全 | `target="_blank"` 一律补 `rel="noopener noreferrer"` | 前端外链按钮 |
| S12 | 同步器数据库 TLS | 同步源数据库 DSN 的 `sslmode` 提为可配置项（默认 `require`/`verify-full`），仅在显式配置且网络隔离时允许 `disable` | `internal/infra/syncer/database_syncer.go` |
| S13 | 客户端 IP 信任边界 | 只有来自 `TRUSTED_PROXY_CIDRS` 的反向代理才采信 `X-Forwarded-For`/`X-Real-IP`；默认空=不信任任何代理，防止伪造 IP 绕过限流/IP 白名单或污染会话绑定；配置非法 CIDR 时 fail-close | `internal/platform/http/clientip.go`、`internal/platform/config/env_security.go` |
| S14 | mTLS 证书来源信任边界 | 默认只认进程内 TLS 的 `r.TLS.PeerCertificates`；仅当 `MTLS_PROXY_CLIENT_CERT_HEADER` 非空且直连 peer 命中 `TRUSTED_PROXY_CIDRS` 时才读取该头（显式代理模式）；XFCC 只取最后一个 `Cert` 元素，畸形/超限/解析失败一律 fail-close，不回退到代理自身的 TLS 证书；代理必须自行校验客户端证书并覆盖该头；设置 `TLS_CLIENT_CA_FILE` 时应用层仍复验转发链 | `internal/platform/mtls/forwarded.go`、`internal/platform/config/trusted_proxy.go`、`internal/bootstrap/server.go` |

## 8. 安全基线（不得回退的控制）

以下为审计确认已成立的安全控制，作为**目标态基线**登记：任何改动不得使其弱化或回退。

| # | 基线控制 | 约束要点 | 落点 |
|---|---------|---------|------|
| B1 | CORS 精确白名单 | 只回显精确匹配的单个来源并带 `Vary: Origin`；禁止裸 `*`、禁止 http 通配、点边界匹配防 `evilexample.com`；`Allow-Headers` 取服务端固定清单、不反射请求头 | `internal/transport/http/middleware/cors.go` |
| B2 | 服务器超时防 Slowloris | `ReadHeaderTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout` 全部显式设置，不得留零值 | `internal/bootstrap/server.go` |
| B3 | HSTS 仅在 TLS 下输出 | 以 `r.TLS != nil` 门控，避免明文/本地开发环境被锁死；**不得**改为无条件输出 | `internal/transport/http/middleware/security_headers.go` |
| B4 | JWT 算法策略 fail-close | 只接受 `RS256` / `SM2-SM3` 签名（issuer 能力）；平台白名单与禁用表先行裁决，缺 `alg`、非白名单算法（含显式覆盖 `none`/`HS*`）、未知 `crit` 一律拒绝 | `internal/infra/jwt/issuer.go`、`internal/platform/jwks/allowed_algorithms.go` |
| B5 | 出站 SSRF 安全 Transport | scheme 白名单 + 拒绝回环/私有/链路本地/云 metadata/CGNAT；所有出站流量复用同一套逐跳校验 | `internal/platform/ssrf/ssrf.go` |
| B6 | 统一错误写出器 | 非 `*apperrors.Error` 一律输出通用文案，不回显内部细节；协议面走 RFC 写入器 | `internal/transport/http/route/http.go`、`presenter/error.go` |
| B7 | 本地存储路径穿越防护 | 拒绝绝对路径；`Clean` 后拒绝 `..` 前缀；`Abs` 前缀包含校验 | `internal/infra/storage/local_storage.go` |
| B8 | SQL 标识符白名单 | 拼接表名/列名前校验标识符合法性；取值一律参数化 | `internal/infra/syncer/database_syncer.go` |
| B9 | 敏感字段单一来源 + 请求体上限 | 敏感字段集由统一 redact 定义（供日志脱敏与读面遮蔽共用）；请求体统一施加 `MaxBytesReader`，仅白名单路径放宽 | `internal/platform/redact/redact.go`、`internal/bootstrap/chi.go` |
| B10 | 响应安全头基线 | 全部 HTML/JSON 响应输出 `X-Content-Type-Options: nosniff`、`Referrer-Policy`、`Permissions-Policy`、COOP；敏感响应追加 `Cache-Control: no-store, no-transform`；CSP 的强制模式是目标态（过渡期见 §7 S6，不得长期停留在 Report-Only） | `internal/transport/http/middleware/security_headers.go`；口径见《规范手册》§12 安全响应头表与 `contracts/registries/security-readiness.yaml`（S36） |

> 上表为「不得回退」的验收基线，不构成能力完成度声明；能力就绪度以 `contracts/registries/security-readiness.yaml` 为唯一口径。

---


## 附录 A：合规验证方法

等保条款到控制类的承接关系见 §6；本附录只保留验证方法，不登记实现状态。

### A.1 验证方法论

#### A.1.1 自动化测试覆盖

| 控制点类别 | 测试类型 | 脚本 |
|-----------|---------|------|
| RLS 隔离 | 集成测试 | `scripts/check-rls.sh` |
| 审计完整性 | 单元测试 + 哈希链验证 | `internal/platform/scopeauthorizer/audit_chain_test.go` |
| 密码安全 | 单元测试 | `internal/app/users/password_validation_test.go`（`TestValidatePasswordComplexity_*` 系列） |
| Passkey 安全 | 单元测试 | `internal/platform/webauthn/verify_test.go` |
| ScopeAuthorizer | 集成测试 | `internal/bootstrap/api_test.go` |
| gosec 静态扫描 | CI 门禁 | `tests/security/gosec-scan.sh` |
| 等保条款映射 | 注册表渲染 | [security-readiness](../generated/security-readiness.md)；本文 §6 |

#### A.1.2 手动验证清单（上线前）

| # | 验证项 | 方法 | 预期结果 |
|---|--------|------|---------|
| 1 | 钓鱼防护 | 用 evil-guliangiam.com 尝试登录 | Origin mismatch → 拒绝 + 审计 |
| 2 | RLS 绕过 | 用无 scope 连接直接查 DB | 返回空结果 |
| 3 | 审计篡改 | 手动修改 authorization_audit_records 某条记录 | `GET /api/v1/risk/audit-integrity` 检测到 |
| 4 | 中间件故障 | 模拟 ScopeAuthorizer panic | 所有请求 500 + 不返回数据 |
| 5 | 跨租户拒绝 | platform_admin 尝试操作不在 scope 的租户 | ScopeAuthorizer → 403 + 审计 |
| 6 | Scope 过期 | 设置 scope.expires_at = now() - 1h | 跨租户请求 → deny |
| 7 | 密码复杂度 | 输入不足平台基线长度（≥14 位）的弱密码 | 被 validatePassword 拒绝 |
| 8 | Session 闲置超时 | 闲置 31 分钟后操作 | 自动登出 |
| 9 | Passkey 吊销 | 吊销后用原凭证尝试登录 | credential revoked → 拒绝 |
| 10 | OperationTicket 过期 | 2 小时后使用 ticket | ticket_expired → 需要重新审批 |
| 11 | platform_admin 无 MFA | 尝试创建 platform_admin 但无 Passkey/TOTP | 被拒绝 |
| 12 | 剩余信息 | 吊销用户后检查所有关联表 | 密码哈希重置 + Passkey 吊销 + Session 撤销 |

#### A.1.3 等保测评对接

等保测评的技术依据为本文 §6 的条款承接关系与 `security-readiness.yaml` 条目的 `design_anchor` / `ac` 验收锚点；测评现场按 A.1.2 清单逐项验证。
