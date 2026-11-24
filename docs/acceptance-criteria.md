# GuliangIAM 验收条件清单

**基线日期**：2026-09-17
**定位**：本文档描述目标系统的验收条件（Given-When-Then 行为规范），端点以 `openapi/openapi.yaml` 与设计文档为准；变更历史唯一写入 `CHANGELOG.md`。

每项 P0/P1 需求必须覆盖成功、拒绝、租户越权、重复请求、并发冲突、依赖不可用和审计结果。

## 标注约定

本文档为纯目标态规范：所有 AC 描述目标系统必须满足的可验收行为，不以「已实现/未实现」区分实施进度；实施证据由代码、CI 与运行记录另行证明。

AC↔需求的双向全量索引在 `traceability-matrix.md`（以 SRS 原编号为准，由 `scripts/gen_traceability.py` 合并 AC 块作用域解析与附录 A 的显式映射生成）；需求定义见 [requirements/functional.md](requirements/functional.md)；本文件只维护验收条件，不重复需求索引。

## 七子句覆盖规则（P0 判定基线）

每条 P0/P1 需求必须覆盖七子句：**成功 / 拒绝 / 越权 / 重复 / 并发 / 依赖不可用 / 审计**。子句定义与典型验证方式：

| 子句 | 定义 | 典型验证方式 |
|---|---|---|
| 成功 | 合法输入得到预期结果，含结果可核验（状态、响应体、副作用） | 正向用例 + 响应/状态断言 |
| 拒绝 | 非法输入被拒，返回**确定错误码**（错误码注册表 §1 业务码或 C.2 协议码）与量化阈值 | 负向用例 + 错误码断言 |
| 越权 | 跨租户/跨权限点访问被拒，且响应与「资源不存在」不可区分（404 `not_found` 防枚举） | 换租户/降权重放 + 响应形态与时延对比 |
| 重复 | 同一操作重放：有幂等键语义的重放返回缓存响应（`Idempotency-Replayed: true`）；无幂等键语义的重复产生 `conflict`/`already_exists`，不产生重复副作用 | 同键重放 + 副作用计数 |
| 并发 | 并发提交相同/冲突操作：恰好一个成功或全部原子生效，计数类不超发不少发 | ≥2 并发压测 + 终态断言 |
| 依赖不可用 | 下游（DB/Redis/Casbin/KMS/限流存储）故障时 fail-close：认证/授权/写路径拒绝（503 `unavailable` 或保守限流），只读可按 [observability](architecture/observability.md) 降级并告警 | 故障注入 + 拒绝断言 + 告警检查 |
| 审计 | 写操作与安全事件落 `records`：actor/目标/时间/IP/原因可检索，拒绝与越权同样留痕 | 审计查询 + 字段断言 |

**适用与判定规则**：

1. 每条 P0 AC 至少显式覆盖「成功、拒绝、审计」三基线子句；其余子句按场景判定，标 N/A 必须写明理由（如纯只读查询无重复副作用语义）；基线子句确无语义时同样允许 N/A，但必须写明理由（如 fail-close 类 AC 无成功路径、防枚举拒绝类 AC 无成功响应形态）。
2. 子句由本条 Given/When/Then 直接承载（✅），或由显式引用的同域补充 AC 承载（➕ AC-xxx-yyy）；承载子句的 AC 同样必须含量化阈值或确定错误码。
3. 覆盖矩阵中存在「未覆盖且未标 N/A」子句的 P0 需求不得判通过；P1 参照执行，允许一条 AC 合并覆盖相邻两子句。
4. **403/503 判定规则（全局统一，消除二义）**：配额/策略/权限类拒绝返回 **403 `forbidden`**（Casbin 拒绝、CSRF 失败、策略 deny、租户配额超限）；容量/依赖类返回 **503 `unavailable`**（DB/Redis/Casbin/KMS 不可用、平台级容量不足）。二者不得混用，错误码以 错误码注册表 §1/C.2 为唯一事实源。

### P0 覆盖矩阵（认证/令牌/权限/租户四域）

| AC 编号 | 成功 | 拒绝 | 越权 | 重复 | 并发 | 依赖不可用 | 审计 | 缺口补齐说明 |
|---|---|---|---|---|---|---|---|---|
| AC-AUTH-001 | ✅ | ✅ | ✅ | N/A（登录非幂等写：重复登录新建会话属预期，无脏副作用） | ➕ AC-AUTH-006 | ➕ AC-PERM-001 / AC-RATE-002 | ✅ | 并发子句由 AC-AUTH-006（锁定计数原子性）补齐 |
| AC-AUTH-002 | ✅（锁定事件可查） | ✅（阈值 5 次/锁定 30min） | ✅（不泄露存在性） | ✅（锁定期重复提交不递增计数） | ➕ AC-AUTH-006 | ✅（先判锁定，不进入哈希路径） | ✅ | 15min 滑窗子句由 AC-AUTH-002 正文承载 |
| AC-AUTH-004 | ✅ | ✅（±90s 漂移外拒绝） | ✅（他人 challenge 拒绝） | ✅（challenge 一次性） | ✅（同一 challenge 并发提交恰好一次成功） | ➕ AC-RATE-002 | ✅ | — |
| AC-OAUTH-001 | ✅ | ✅（400 `invalid_grant`） | ✅（redirect_uri 精确匹配） | ✅（code 一次性） | ➕ AC-TOKEN-005 | ➕ AC-PERM-001 | ✅ | 令牌侧并发由 AC-TOKEN-005（refresh 并发轮换）补齐 |
| AC-TOKEN-001 | ✅ | ✅（重用撤销 family） | ✅ | ✅ | ➕ AC-TOKEN-005 | ➕ AC-TOKEN-004（缓存回退 DB） | ✅ | — |
| AC-TOKEN-002 | ✅ | ✅（`active=false`） | ✅ | ✅（只读幂等） | N/A（内省无写副作用） | ✅（AC-TOKEN-004 回退路径） | ✅ | — |
| AC-PERM-001 | N/A（fail-close 无成功路径） | ✅（403/503 判定规则） | ✅ | N/A | N/A | ✅（本条即依赖不可用子句） | ✅（告警+审计） | — |
| AC-PERM-002 | ✅ | N/A（无拒绝语义） | ➕ AC-TENANT-002 | ✅（重复变更幂等收敛） | ✅（30s TTL 上界内并发读写收敛） | ➕ AC-PERM-001 | ✅ | — |
| AC-TENANT-001 | N/A（跨租户访问防枚举 AC：一律 404 拒绝，无成功路径） | ✅（404 防枚举） | ✅（本条即越权子句） | N/A | ➕ AC-TENANT-004 | ➕ AC-TENANT-003 | ✅ | 并发子句由 AC-TENANT-004（跨租户写并发）补齐 |
| AC-TENANT-002 | N/A（越权拒绝 AC：无成功路径） | ✅ | ✅（本条即越权子句） | N/A | ➕ AC-TENANT-004 | ➕ AC-TENANT-003 | ✅ | — |
| AC-TENANT-003 | ✅（空结果而非全量） | ✅ | ✅ | N/A | ✅（上下文事务内局部生效） | ✅（Strict 模式即 fail-close） | ✅ | — |

## 1. 认证与令牌

### AC-AUTH-001 密码登录

**Given** 组织允许密码登录，用户存在且未禁用
**When** 提交正确凭证
**Then** 创建安全会话，Cookie 具备 HttpOnly、Secure、SameSite；响应不返回密码、Secret 或堆栈。

**验证锚点**：`internal/transport/http/api/v1/auth/cookie_test.go:67::TestDefaultCookieAttrs_SecureFollowsCookieSecure`（HttpOnly 恒开、Secure 跟随开关、SameSite/Path/Domain 断言）；`internal/transport/http/api/v1/auth/cookie_test.go:89::TestSetOpaqueSessionCookie_AttributesPerMode`（写入会话 Cookie 的属性按安全模式断言）；`internal/app/auth/login_test.go:792::TestPasswordLogin_Success`（认证成功创建 active 会话并写入 session 仓储）。

### AC-AUTH-002 账号锁定

**Given** 组织策略启用账号锁定（SRS AUTH-071：同一用户 **5 次失败 / 15 分钟窗口**，触发后锁定 **30 分钟**）
**When** 窗口内第 5 次认证失败
**Then** 锁定状态持久化并写入审计（actor/原因/IP）；锁定期内再次提交：响应与「凭证错误」形态一致（防枚举），且**先判锁定→不进入密码哈希比对路径**，失败计数在锁定后不再递增；锁定事件与管理/状态面可使用 C.1 `account_locked`（423），登录路径不得以任何错误码/文案/时延差异泄露锁定态与用户存在性；锁定状态以 PostgreSQL `lockout_attempts` 为唯一事实源（跨实例经数据库共享，无 Redis 计数层）；锁定窗口与时长按组织策略可配，反复触发按 30min → 6h → 24h 指数退避。

**验证锚点**：`internal/app/auth/login_failure_hardening_test.go:143::TestPasswordLogin_LockedAndWrongPasswordResponsesMatch`（锁定与凭证错误的 Code/Message/MessageKey/Meta/HTTPStatus 全等，锁定后失败计数不递增，审计 `reason=account_locked`）；`internal/app/auth/login_failure_hardening_test.go:73::TestPasswordLogin_CredentialFailureWritesAudit`（失败审计含 actor/原因/IP/UA）；`internal/app/auth/account_lockout_test.go:255::TestLockout_ConsecutiveFailuresReachThreshold`（失败达到阈值写入 `LockedUntil`）；`internal/domain/lockoutattempts/attempt_test.go::TestAttemptRecordFailureWindowAndBackoff`（观察窗与三级退避、成功后重置）；真实 PostgreSQL 验收锚点 `internal/infra/postgres/lockoutattempts/concurrency_pg_test.go::TestLockoutAttemptsPostgres_ObservationWindowAndBackoff`、`::TestLockoutAttemptsPostgres_ObservationWindowResetsAttempts`（需 `GIAM_TEST_PG_DSN`）；`internal/app/auth/account_lockout_test.go:426::TestLockout_RecordFailureDoesNotIncrementWhenLocked`（锁定后失败计数不递增）。

### AC-OAUTH-001 PKCE 授权码

**Given** 客户端 redirect URI 精确匹配（禁通配/正则/子串，[14.6.3](design/protocol/oauth2.md#1463-redirect_uri-精确匹配与不可重定向错误决策表)）
**When** 使用 Authorization Code + PKCE S256 完成授权
**Then** code 一次性消费且 TTL 为 **5 分钟**（`authorizationCodeTTL = 5 * time.Minute`，[14.6.4](design/protocol/oauth2.md#1464-授权码参数)），过期判定无宽限窗口；错误 verifier、redirect URI、过期 code 或重复兑换均返回 400 `invalid_grant`（STD C.2 协议码）；**PKCE 对全部客户端强制**（含机密客户端，`applications.pkce_required` 不再是门控）：授权端点缺 `code_challenge` 或方法非 `S256` 即拒绝；**授权码随机熵 ≥128 bit**（实现取 256 bit，`crypto/rand` + base64url，禁止 UUIDv7 形态）；**重复兑换必须撤销该码此前签发的全部访问/刷新令牌**（OAuth 2.1 I-D §4.1.3，[14.6.4](design/protocol/oauth2.md#1464-授权码参数)）；`openid` scope 下令牌响应签发 ID Token。

**验证锚点**：`internal/app/auth/protocol_conformance_test.go:24::TestNewAuthorizationCodeSecret_Has256BitEntropy`（≥128 bit 熵）、`internal/app/auth/protocol_conformance_test.go:112::TestNewAuthorizationCode_RequiresPKCE`、`internal/app/auth/protocol_conformance_test.go:171::TestValidateAuthorizationCodeExchange_EnforcesPKCE`（PKCE 强制）；`internal/app/auth/token_test.go:29::TestAuthorizationCodeTTL`（`authorizationCodeTTL = 5m`）；`internal/domain/authorizationcodes/code_validate_test.go:64::TestAuthorizationCode_IsUsed` 与 `internal/domain/authorizationcodes/code_test.go:39::TestAuthorizationCode_Expiration`（一次性与过期判定）。

### AC-TOKEN-001 令牌轮换

**Given** refresh token 属于有效 family
**When** 客户端刷新令牌
**Then** 旧 token 失效并签发新 token；重用旧 token 时撤销 family 并产生安全审计。

**验证锚点**：`internal/app/tokens/create_token_test.go:368::TestService_RefreshAccessToken_Success`（刷新签发新 access/refresh token）、`internal/app/tokens/create_token_test.go:491::TestService_RefreshAccessToken_RevokedRefreshToken_ReuseDetection`（复用已撤销 refresh token 被拒，进入 family 重用检测路径）。

### AC-AUTH-003 密码策略

**Given** 组织密码策略要求长度、大小写、数字和特殊字符
**When** 提交不符合策略的密码
**Then** 返回字段级校验错误，不保存凭证，不泄露密码内容；策略 `PasswordHistoryCount > 0` 时新密码不得与最近 N 条历史密码任一匹配（改密/重置均写入 peppered 历史并按上限裁剪；历史读取失败为显式 fail-open——跳过复用检查、计入 `password_history_fail_open_total` 并告警，不阻断改密）；策略 `PasswordMaxAgeDays > 0` 时达到年龄上限的密码判定过期（`UserCredentials.IsPasswordExpired` 为唯一阈值实现；`PasswordUpdatedAt` 缺失按 fail-closed 判过期），登录响应携带 `passwordExpired` 通知（附 `passwordExpiresAt`），请求期门禁返回 403 `password_expired`（reason `max_age_exceeded`）；强制改密标记优先于 max-age（reason `need_update_password`，403 `password_change_required`）；无本地密码用户（SSO/LDAP）永久豁免年龄门禁，`PasswordMaxAgeDays=0` 关闭 max-age 分支。

**验证锚点**：`internal/app/auth/password_policy_test.go:96::TestValidatePasswordAgainstPolicy_TooShort`、`internal/app/auth/password_policy_test.go:113::TestValidatePasswordAgainstPolicy_MissingUppercase`、`internal/app/auth/password_policy_test.go:145::TestValidatePasswordAgainstPolicy_ContainsUsername`（分别断言长度不足、缺少大写、包含用户名均被策略拒绝）；`internal/app/auth/password_change_history_test.go:230::TestChangePasswordRejectsPasswordFromHistory`（历史重用拒绝）、`:283::TestChangePasswordAllowsReuseWhenHistoryDisabled`（Count=0 关闭检查）、`:290::TestPasswordHistoryLookupFailureFailsOpenAndCounts`（读取失败 fail-open 并计数）、`:333::TestChangePasswordHistoryWriteFailureRollsBack`（历史写入与改密同事务回滚）、`:387::TestResetPasswordWritesPepperedHistory`（重置路径写入 peppered 历史）；`internal/app/auth/password_state_test.go:11::TestEvaluatePasswordState`（强制标记优先、max-age fail-closed、SSO/LDAP 豁免与 max-age=0 关闭）。

### AC-AUTH-004 MFA

**Given** 用户已绑定 TOTP 或 WebAuthn（挑战状态机 pending → push_pending → approved/rejected，AUTH-050）
**When** 提交有效二次认证
**Then** 创建会话；错误、过期、重复使用的挑战均被拒绝（401 `unauthorized`，C.1）并计入审计；TOTP 时钟漂移容忍 **±90s**（±1 个 30s 步长，[observability](architecture/observability.md)，`totp.ValidateCustom Skew:1` 口径）；同一 challenge 并发提交**恰好一次**成功，其余拒绝。

**验证锚点**：`internal/domain/mfachallenges/challenge_test.go:88::TestChallenge_Expiration`、`internal/domain/mfachallenges/challenge_test.go:75::TestChallenge_FailedAttemptsIncrement`（挑战过期与失败计数）；`internal/app/auth/webauthn_test.go:120::TestWebAuthnAuthenticationFinish_IssuesSessionAndConsumesChallenge`（成功建立会话并一次性消费挑战）、`internal/app/auth/webauthn_test.go:269::TestWebAuthnAuthenticationFinish_RejectsMismatchedChallenge`、`internal/app/auth/webauthn_test.go:402::TestWebAuthnAuthenticationFinish_RejectsChallengeOfAnotherCeremony`（拒绝不匹配与跨 ceremony 挑战）。

### AC-AUTH-005 DPoP 重放（ADR-014）

**Given** DPoP 令牌绑定生效，且已使用过的 DPoP proof `jti` 已入库去重
**When** 再次提交相同 proof
**Then** 返回 401 `invalid_dpop_proof`，不创建或刷新令牌；jti 去重状态跨实例共享（存储介质与 TTL 唯一事实源为 `contracts/registries/short-lived-state.yaml` `dpop_jti`：PostgreSQL/MySQL `dpop_replay` 表、TTL 3m、fail_close；渲染矩阵见《data-model》§3.1「短期状态存储矩阵」），**多区域部署时 jti 去重每区域独立**（不跨区域广播，按区域数据面路由天然隔离）；去重存储（`dpop_replay`，PostgreSQL/MySQL）不可用时 DPoP 请求 fail-close（不降级为不校验）。

**验证锚点**：`internal/app/auth/dpop_conformance_test.go:219::TestDPoP_ReplayedJTIIsRejected`（同一 `jti` 第二次出现被拒）、`internal/app/auth/dpop_conformance_test.go:176::TestDPoP_UnsupportedAlgorithmIsRejectedOnTheProductionPath`（生产路径算法白名单与 `invalid_dpop_proof`）、`internal/app/auth/dpop_conformance_test.go:135::TestDPoP_ProofHTUMustMatchTheRequestURL`、`internal/app/auth/dpop_conformance_test.go:192::TestDPoP_ClockSkewIsSixtySeconds`、`internal/app/auth/dpop_conformance_test.go:236::TestDPoP_NilValidatorFallsBackToTheDefault`。

### AC-TOKEN-012 ID Token claims 与校验基线（[14.9](design/protocol/oauth2.md#149-userinfo-端点)）

**Given** 应用以 `openid` scope 通过 Authorization Code + PKCE 换取令牌，`ISSUER_URL` 已配置为部署的外部可达地址
**When** 服务端签发 ID Token 并在后续请求中以平台公钥校验 JWT
**Then** 同时成立以下五条：

1. **issuer 一致**：`id_token.iss` ≡ `logout_token.iss` ≡ discovery 的 `issuer`，三者逐字符相等；任何部署域名下均不得出现常量 `"guliangiam"`。
2. **auth_time 语义**：`auth_time` 是终端用户实际完成认证（含 MFA）的时刻，**不得**等于发码/签发时刻；会话复用场景（会话建立早于授权请求）必须反映更早的会话时刻。
3. **scope 裁剪**：`openid` 仅保证 `sub`；`profile`/`email`/`phone` 各自的 claim 仅在对应 scope 授予时出现，且**未授予时省略而非置空**；UserInfo 端点对不含 `openid` 的 access token 返回 403。
4. **at_hash**：code 流程随行签发 `at_hash = BASE64URL(LEFTMOST_HALF(SHA256(access_token)))`；无随行 access token 时省略。
5. **验签**：`alg` 显式白名单（`none` 与对称算法在平台禁用表内即拒）；`crit` 非空即拒；`kid` 必填且必须已知，未知 `kid` 一律拒绝而**不回落当前密钥**。

**验证锚点**：`internal/app/auth/oidc_claims_conformance_test.go:60::TestBuildIDToken_UsesTheDeploymentIssuer`（iss 逐字符一致、无常量 `"guliangiam"`）、`internal/app/auth/oidc_claims_conformance_test.go:112::TestBuildIDToken_AuthTimeIsTheAuthenticationMoment`、`internal/app/auth/oidc_claims_conformance_test.go:140::TestBuildIDToken_TrimsClaimsByScope`、`internal/app/auth/oidc_claims_conformance_test.go:226::TestBuildIDToken_SetsAtHash`、`internal/app/auth/oidc_claims_conformance_test.go:334::TestGetUserInfo_RequiresOpenIDScope`（无 `openid` 的 access token 返回 403）。

### AC-TOKEN-002 Introspection

**Given** access token 已签发或已撤销
**When** 资源服务器调用 introspection
**Then** 有效令牌返回 `active=true`，撤销/过期令牌返回 `active=false`，不泄露哈希或内部 ID。

**验证锚点**：`internal/app/tokens/service_test.go:794::TestService_IntrospectToken_ActiveToken`、`internal/app/tokens/service_test.go:834::TestService_IntrospectToken_ExpiredToken`、`internal/app/tokens/service_test.go:861::TestService_IntrospectToken_RevokedToken`（active、过期、撤销三态与响应字段）。

### AC-AUTH-006 锁定计数并发原子性（并发子句补齐）

**Given** 同一账号在 15 分钟窗口内并发提交 N≥5 个失败认证请求
**When** 全部并发请求完成
**Then** 失败计数恰好为 N（原子递增，不超计/不少计）；达到 5 次阈值后**恰好触发一次**锁定事件（审计中锁定事件唯一）；第 6 次起直接命中锁定短路路径（不进入密码哈希比对，AC-AUTH-002 顺序约束）。

**验证锚点**：`internal/infra/postgres/lockoutattempts/concurrency_pg_test.go:113::TestLockoutAttemptsPostgres_ConcurrentRecordFailureCountsEveryAttempt`（16 并发、阈值高于 N 时 `attempts == N`，无丢失/重复递增；`GIAM_TEST_PG_DSN` 缺失时 SKIP）、`internal/infra/postgres/lockoutattempts/concurrency_pg_test.go:145::TestLockoutAttemptsPostgres_ConcurrentThresholdLocksOnce`（阈值 5 时 16 并发只有一次锁定窗口写入、`attempts` 饱和为 5 且锁定期不递增）、`internal/app/auth/lockout_short_circuit_test.go:21::TestPasswordLogin_LockedAccountSkipsPasswordHashVerification`（锁定后登录在密码哈希解析前短路，审计 `reason=account_locked`）；`internal/app/auth/login_failure_hardening_test.go:143::TestPasswordLogin_LockedAndWrongPasswordResponsesMatch`（锁定/凭证错误响应全等，锁定期失败计数不递增）。

### AC-AUTH-007 WebAuthn 注册 ceremony（14 项校验，AUTH-041/043，[webauthn](design/auth/webauthn.md)）

**Given** 已认证会话发起 `registration:begin/finish`（finish 校验会话用户 == user handle，[webauthn](design/auth/webauthn.md)）
**When** 提交 attestation
**Then** [webauthn](design/auth/webauthn.md) 的 **14 项注册校验**全部通过方可落库：challenge 一次性（TTL 5min，见 webauthn.md「Challenge 生命周期」）、origin/RP ID 精确匹配、attestation 算法白名单、`credentialId` 全局唯一（跨租户，冲突 409 `credentialExcluded`，见 webauthn.md「Passkey 语义与 Conditional UI」residentKey/discoverable 口径）、signCount 初始值校验、BE/BS 备份标志、AAGUID 登记等；任一校验失败拒绝注册并审计；凭据私钥与 Secret 不落库不落日志（《开发规范》§9.5 日志脱敏清单为唯一事实源：MFA Secret 与恢复码不得写入日志）。

**验证锚点**：`internal/app/users/webauthn_admin_test.go:395::TestBeginAdminWebAuthnRegistration_PersistsOnlyHash`、`:431::TestBeginAdminWebAuthnRegistration_ExcludesUsersActiveCredentials`、`:578::TestFinishAdminWebAuthnRegistration_DuplicateCredentialIsAlreadyExists`；注册 ceremony 的 challenge/origin 校验见 `internal/app/auth/webauthn_registration_hardening_test.go:196::TestWebAuthnRegistrationBeginIncludesOnlyUsersActiveCredentials`；COSE/attestation 解析见 `internal/platform/webauthn/attestation_test.go`。

### AC-AUTH-008 WebAuthn 认证闭环与克隆检测（AUTH-042，[webauthn](design/auth/webauthn.md) / [webauthn](design/auth/webauthn.md)）

**Given** 已注册凭据发起 `authentication:begin/finish`
**When** 提交 assertion
**Then** challenge 一次性 + origin/RP ID 精确匹配 + signCount 单调递增校验全部通过方可创建会话；signCount 回退/持平（非「双零豁免」与同步式 passkey 例外，见 webauthn.md「凭证状态闭环」与「Passkey 语义与 Conditional UI」）判为克隆风险——**撤销该凭据**并触发 `critical` 安全事件与用户通知；匿名可达场景受同 IP 匿名 challenge 配额 ≤10/h 约束（见 webauthn.md「路由与鉴权矩阵」）。

**验证锚点**：`internal/platform/webauthn/verify.go:140`（`ErrSignCountRegression`：signCount 回退/持平返回哨兵错误）、`internal/app/auth/webauthn_authenticate.go:251`（克隆风险判定）、`internal/app/auth/webauthn_reason_test.go:21::TestWebAuthnFailureReason`（`sign_count_regression` 原因映射）；challenge 一次性/状态区分见 `internal/app/auth/webauthn_finish_status_test.go:18::TestWebAuthnRegistrationFinish_ChallengeGoneIsIndistinguishable410`。

### AC-AUTH-009 WebAuthn 凭证删除联动强制下线（[webauthn](design/auth/webauthn.md)）

**Given** 管理员执行 `DELETE /api/v1/users/{id}/webauthn-credentials/{credentialId}`（`users:write` + Admin）或用户完成密码修改
**When** 删除/变更成功
**Then** 撤销该用户**全部**活跃会话与派生 refresh token family，内省缓存即时失效（Cache-Aside，[permissions](design/iam/permissions.md)），审计记录撤销者/原因/来源 IP；强制下线三端点（`GET /users/{id}/sessions`、`DELETE /users/{id}/sessions/{sessionId}`、`DELETE /users/{id}/sessions`）权限点独立，越权返回 403 `forbidden`。

**验证锚点**：`internal/app/auth/webauthn_credentials_test.go:69::TestRevokeWebAuthnCredential_RevokesOwnCredential`、`:83::TestRevokeWebAuthnCredential_UnknownCredentialIsNotFound`；会话撤销能力见 `internal/app/sessions/service_test.go:435::TestService_RevokeSession`、`internal/app/sessions/service_cleanup_test.go:382::TestService_DeleteSession_Success`；`DELETE /api/v1/users/{id}/webauthn-credentials/{credentialId}` 的 `users:write` 权限见 `internal/transport/http/api/v1/users/routes.go:277`。

### AC-AUTH-010 会话恢复令牌（remember-me，[webauthn](design/auth/webauthn.md)）

**Given** 用户登录时显式勾选「记住设备」并取得恢复令牌（有效期 30d，仅哈希入库）
**When** 会话过期后携恢复令牌请求恢复
**Then** 令牌一次性使用并轮换；重用旧令牌撤销该 family 全部令牌并告警（对齐 webauthn.md「记住设备与恢复令牌（remember-me）」轮换语义）；过期/已撤销/跨租户令牌一律按无效拒绝（401）；交换端点本身不建会话，返回轮换结果供后续会话签发。

**验证锚点**：`internal/app/auth/remember_me_pure_test.go:139::TestExchangeRememberMeRotatesToken`、`:170::TestExchangeRememberMeReuseRevokesFamily`、`:191::TestExchangeRememberMeRejectsExpiredAndRevoked`、`:224::TestExchangeRememberMeRejectsCrossTenantAndUnknown`、`:114::TestRememberMeTTLIsThirtyDays`；仓储清理见 `internal/infra/postgres/rememberme/repository_test.go:39::TestDeleteExpiredDeletesTokensThenFamilies`。

### AC-AUTH-011 TOTP 失步与重同步

**Given** 用户 TOTP 时钟漂移超出校验窗口（前后各 1 个时间步，mfa.md §TOTP 失步与重同步）导致连续校验失败
**When** challenge 内累计失败达到重同步阈值（3 次，`totpResyncAfterFailures`；challenge 内独立计数，**非** AC-AUTH-002 组织登录锁定的同源计数）
**Then** 进入受控重同步门控：只接受晚于最后一次失败时间步的码（进入门控写 `mfa.totp.resync_required` 审计），不因连续失败放宽窗口；管理员重绑 TOTP 须重新走注册 ceremony 并写 `mfa.credential.rebound` 审计；该门控与 AC-AUTH-002 锁定策略相互独立、叠加不冲突。

**验证锚点**：`internal/app/auth/login_totp_resync_test.go:37::TestVerifyLoginMfaTOTPAcceptsValidCode`、`:52::TestVerifyLoginMfaTOTPRejectsReplayedCode`、`:67::TestVerifyLoginMfaTOTPControlledResyncRequiresLaterWindow`、`:106::TestVerifyLoginMfaTOTPLocksOutAfterMaxAttemptsAcrossReloads`。

### AC-AUTH-012 OTP 通道安全（SMS/Email，SEC-012，[functional §9](requirements/functional.md)）

**Given** 用户请求发送短信/邮箱验证码
**When** 发送与校验
**Then** 验证码**有效期 10 分钟、失败 3 次即失效该码、一次性使用**（事实源 `internal/app/verifications/send_verify.go`：`verificationCodeTimeout = 10 * time.Minute` 与失败计数口径；需求口径见 [functional §1.2](requirements/functional.md)）；发送侧频率限制（SEC-012，IP 级 **10 次/小时**，同事实源）生效，超限返回 429 `too_many_requests` + `Retry-After`（C.1）；校验成功后旧码立即失效，未校验旧码被新码顶替；错误响应不泄露「号码/邮箱是否存在」。

**验证锚点**：`internal/app/verifications/send_verify_test.go:274::TestVerifyCode_DelegatesToProviderVerifier`、`:299::TestVerifyCode_ProviderRejectsCode`、`:322::TestSendVerification_DeliveryFailureStoresNothing`、`:339::TestSendAndVerifyCode_DigestAlgorithms`；IP 配额见 `internal/app/verifications/send_verify_ip_test.go:10::TestConsumeVerificationCodeWithClientIP_RejectsExhaustedIP`。

### AC-AUTH-013 Recovery Code（AUTH-047）

**Given** 用户持有恢复码集合（仅哈希存储、明文仅展示一次）
**When** 使用恢复码完成 MFA 旁路登录
**Then** 单码一次性消费，重放已用码被拒并触发安全通知；全部恢复码耗尽后强制重新生成流程并审计；恢复码不写入日志（《开发规范》§9.5 日志脱敏清单含 `recoveryCodes`）。

**验证锚点**：`internal/app/auth/login_pure_test.go:365::TestVerifyRecoveryCode`（单码一次性消费、已用码重放被拒）；`internal/app/auth/login_validation_test.go:144::TestValidateLoginCommand_PasscodeAndRecoveryCodeConflict`。

### AC-AUTH-014 MFA 记住设备与撤销（AUTH-051/104）

**Given** 用户勾选记住设备（免验证期租户可配）
**When** 免验证期内同设备（指纹一致）再次登录
**Then** 跳过 MFA 且设备信任记录可审计；用户删除或管理员撤销信任设备后**立即**失效（下次登录要求完整 MFA）；设备指纹变化触发重新验证 + 通知（AUTH-104）；信任设备列表在个人中心与管理侧均可查并审计。

**验证锚点**：`internal/app/auth/trusted_devices_test.go:29::TestTrustedDeviceStatus`、`:72::TestVerifyTrustedDeviceSignatureECDSA`、`:145::TestTrustedDeviceViewNeverExposesKeyMaterial`、`:242::TestTrustedDeviceCredentialDigestAndMask`；域规则见 `internal/domain/trusteddevices/device_test.go:11::TestValidateCredentialInput`。

### AC-AUTH-015 step-up 二次验证（AUTH-105）

**Given** 用户已建立低保证级会话，请求敏感操作（改密/换绑/管理员强制下线等）
**When** 触发 step-up
**Then** 要求重新认证并升级 `authentication_level` 后方可执行目标操作；step-up 挑战一次性、过期返回 401 `unauthorized`；未完成 step-up 时敏感操作不产生任何副作用，step-up 成功/失败与目标操作均写审计。

**验证锚点**：`internal/app/auth/step_up_test.go:104::TestBeginStepUpRequiresBoundIdentifiers`、`:133::TestBeginStepUpCreatesBoundChallenge`、`:184::TestFinishStepUpRejectsInvalidChallenges`、`:291::TestFinishStepUpElevatesOnlyCurrentSession`；passkey 通道见 `internal/app/auth/step_up_passkey_test.go:99::TestFinishStepUpWithPasskeyElevatesOnlyCurrentSession`。

### AC-AUTH-016 用户自注册与密码重置（AUTH-067/068）

**Given** 组织允许自注册或密码重置，且请求携带有效验证码
**When** 提交注册命令或密码重置命令
**Then** 注册：校验组织注册开关、用户名/邮箱/手机号唯一性与密码策略，成功后创建待验证账号并写审计；密码重置：验证码一次性消费（TTL 10 分钟，事实源 `internal/app/verifications/send_verify.go` 的 `verificationCodeTimeout`，与 AC-AUTH-012 同源）、新密码通过策略校验、重置成功后吊销该用户全部既有会话与 refresh token family；重复用户名返回 409 `conflict`，验证码失效/重放返回 400 `invalid_argument`，跨租户重置尝试返回 404 `not_found`（防枚举）；两条路径均不泄露账号是否存在。

**验证锚点**：`internal/app/auth/password_reset_signup.go`；`internal/app/auth/password_reset_signup_test.go`（`TestValidateSignupCommand` / `TestValidateResetPasswordCommand` / `TestResolveVerificationType`）；设计见 [design/auth/login.md](design/auth/login.md)。

### AC-TOKEN-005 Refresh Token 并发轮换（并发子句补齐）

**Given** 同一 refresh token 的 K≥2 个并发刷新请求
**When** 全部请求完成
**Then** 恰好一个成功签发新 token，且不产生分支 family；其余请求由管理面 `POST /api/v1/tokens/refresh` 按实际契约拒绝——无效/过期/跨租户返回 400 `invalid_argument`，已撤销/重用与并发串行化败者返回 401 `unauthorized`，**本端点不产生 OAuth `invalid_grant`**；若拒绝方在成功方提交前已消费旧 token（竞态窗口），按 family 重用检测撤销整个 family 并审计——以服务端串行化仲裁结果为准，任何时刻至多一个活跃 family。并发败者的整族撤销会连带撤销 winner 新签发 refresh token 的残余风险登记为 `OPEN-API-47`。

**验证锚点**：`internal/infra/postgres/tokens/concurrency_pg_test.go:130::TestTokenRepositoryPostgres_RevokeRefreshIsSingleWinner`（同一 refresh 行 8 并发条件 UPDATE 恰好 1 个成功、其余 `ErrRefreshAlreadyRevoked`，revision 只 +1）、`internal/app/tokens/refresh_concurrency_pg_test.go:202::TestServiceRefreshAccessTokenPostgres_ConcurrentRotationHasSingleWinner`（8 并发刷新恰好 1 次签发、无分支 family、至多一个活跃 family；串行化败者命中 family 重用检测时整族撤销提交并写 `token.reuse_detected` 审计）、`internal/app/tokens/create_token_test.go:491::TestService_RefreshAccessToken_RevokedRefreshToken_ReuseDetection`（已撤销 refresh 走复用检测拒绝，mock 层）。

### AC-TOKEN-006 Token Revocation（RFC 7009，AUTH-027，[14.1](design/protocol/oauth2.md#141-rfc-7009-token-revocation)）

**Given** 已签发的 access/refresh token
**When** 客户端调用撤销端点（discovery 宣告 `{issuer}/oauth2/revoke`）
**Then** 撤销成功返回 200（RFC 7009：对无效 token 同样返回 200，不泄露有效性）；撤销后 introspection 在缓存失效上界 **30s**（[observability](architecture/observability.md)）内返回 `active=false`，refresh family 联动失效；审计记录撤销者与 client_id；未认证客户端返回 401 `invalid_client`（C.2）。

**验证锚点**：`internal/app/auth/token_revocation_auth_test.go:16::TestRevokeToken_AcceptsPrivateKeyJWTAssertion`、`:38::TestRevokeToken_RejectsInvalidPrivateKeyJWTAssertion`、`:51::TestRevokeToken_AcceptsMTLSClientCertificate`、`:75::TestRevokeToken_RejectsMTLSWithoutCertificate`（客户端认证失败→401 invalid_client）；撤销端点 `/api/v1/auth/oauth2/revoke`。

### AC-TOKEN-007 Discovery 与 JWKS（OIDC Discovery 1.0 / RFC 8414，[14.14](design/protocol/oauth2.md#1414-密钥版本与轮换) / [observability](architecture/observability.md)）

**Given** RP 请求 `/.well-known/openid-configuration` 与 `jwks_uri`
**When** 解析文档并拉取 JWKS
**Then** 14.13 必备字段与实际路由一致；JWKS 多 kid 重叠窗口：旧 key 保留 ≥ **2× max token TTL**（26.5）；`kid` 未知回源一次（14.8 规则），回源失败拒绝验签且不得使用未知 kid。

**验证锚点**：`internal/app/auth/oidc_discovery_test.go:16::TestGetOIDCDiscovery`（issuer、authorize/token/jwks 端点与 scopes/grants/claims 列表）；`internal/platform/jwks/remote_unknown_kid_test.go:142::TestUnknownKidTriggersSingleSourceRefreshAndResolvesNewKey`、`internal/platform/jwks/remote_unknown_kid_test.go:172::TestUnknownKidAfterRefreshRejects`、`internal/platform/jwks/remote_unknown_kid_test.go:379::TestRetiredKeyRemovedFromJWKSStillVerifiesFromCache`（未知 `kid` 单次回源、回源失败拒绝、退役 key 仍可验证）；`internal/platform/jwks/keyring_test.go:63::TestParseKeyRingAcceptsMultipleKeys`（多 `kid` 重叠窗口）。

### AC-TOKEN-008 Consent 管理（AUTH-031）

**Given** RP 请求超出已授权 scope
**When** 引导用户进入 Consent 决策
**Then** 未决策前授权流程终止（OIDC `consent_required` 语义）；grant/deny 均写 `consents` 表与审计（含 client/scope/时间）；revoke 后下次同 scope 须重新 Consent，**且撤销必须连带吊销该用户在该应用的全部 access/refresh 令牌与活跃会话**（否则旧令牌在 TTL 内仍可访问）；已授权 scope 静默通过不重复弹窗。

**验证锚点**：`internal/app/auth/consent.go:119::RevokeConsent`（级联吊销令牌 + 会话，`:205` 日志）、`:235`（会话撤销）；撤销语义测试 `internal/app/auth/service_coverage2_test.go:336::TestRevokeConsent_NoConsent`、`:362::TestRevokeConsent_AlreadyRevoked`；同意生命周期 `internal/app/auth/consent_coverage_test.go:161::TestToConsentDetail_WithRevokedAt`。

### AC-TOKEN-009 Client Credentials（[14.7](design/protocol/oauth2.md#147-客户端模式与刷新令牌边界)）

> **行为边界**（principals 模型不在宣告范围）：`client_credentials` 对已认证的机密/服务账号客户端返回 **400 `unsupported_grant_type`**，不签发 token，也不写审计/Outbox（见 [oauth2.md:287](design/protocol/oauth2.md#14132-oidc-能力位宣告) `grant_types_supported` 列表不含 client_credentials，备注"因令牌主体模型暂不宣告"）。以下 Given/When/Then 描述 `client_credentials` 能力宣告纳入范围后的验收标准；既有行为以验证锚点测试清单为准。

**Given** 机密客户端以 `client_credentials` 请求令牌（grant_types 服务端白名单已启用）
**When** 完成客户端认证（client_secret_basic/post；JWT/mTLS 目标态见 14.25/14.26）
**Then** 签发**不含 refresh_token 与用户上下文**的 access token；公开客户端或未启用 grant 返回 400 `unsupported_grant_type`、认证失败返回 401 `invalid_client`（C.2）；令牌签发写审计（client 维度，无用户 actor）。

**验证锚点（既有行为，principals 模型不在宣告范围时全部命中）**：`internal/app/auth/wli_auth_test.go:147::TestExchangeClientCredentials_ServiceAccountIsUnsupportedUntilPrincipalsLand`（ServiceAccount 因 principals 模型不在宣告范围返回 `unsupported_grant_type`、无 token/审计/outbox 副作用）、`internal/app/auth/wli_auth_test.go:175::TestExchangeClientCredentials_ServiceAccountAuthenticationFailuresNeverFallBack`（认证失败永不回退到 Application，返回 `invalid_client`）、`internal/app/auth/wli_auth_test.go:258::TestExchangeClientCredentials_ApplicationFallbackAndProtocolErrors`（Application client_credentials 同样返回 `unsupported_grant_type`：已认证、公开客户端、grant 禁用、认证失败四条子路径）。

### AC-TOKEN-010 Device Flow（RFC 8628，[14.23](design/protocol/oauth2.md#1423-device-authorization-grant-补全rfc-8628)）

**Given** 客户端发起 device authorization 请求
**When** 用户在第二端输入 user_code 并授权
**Then** `device_code` TTL **15 分钟**；user_code 保持 8 字符 `XXXX-XXXX` 无混淆字符集，熵由 TTL + 一次性验证 + 输入限速补偿（14.23）；轮询遵守 `interval`，过快返回 400 `slow_down`、等待中返回 400 `authorization_pending`、过期返回 400 `expired_token`（轮询终止）、用户拒绝返回 400 `access_denied`；全部轮询错误码与 14.23 错误表一致；`verification_uri` 为**绝对 URL**（`{issuer}/device`）并可选下发 `verification_uri_complete`；同一 `device_code` 连续 5 次过快轮询即终止授权。

**验证锚点**：`internal/app/auth/device_flow_conformance_test.go:147::TestDeviceToken_SlowDownWhenPollingTooFast`、`internal/app/auth/device_flow_conformance_test.go:156::TestDeviceToken_TerminatesAfterRepeatedViolations`、`internal/app/auth/device_flow_conformance_test.go:200::TestDeviceToken_ExpiredCodeIsExpiredTokenAndIsRemoved`、`internal/app/auth/device_flow_conformance_test.go:213::TestDeviceToken_RejectsWrongClient`、`internal/app/auth/device_flow_conformance_test.go:225::TestVerificationURI_IsAbsolute`、`internal/app/auth/device_flow_conformance_test.go:254::TestDeviceErrorCodes_MatchRFC8628`。

### AC-TOKEN-011 Token Exchange（RFC 8693，[14.24](design/protocol/oauth2.md#1424-oauth-20-token-exchangerfc-8693)）

**Given** 已认证主体持有效 subject_token 请求换发
**When** 调用令牌交换端点
**Then** 换发 token 的 aud/act/scope 不超出 subject_token 授权边界；**subject_token 归属必须等于调用方应用**（否则 403，禁止横向越权）；`actor_token` 提供时必须先行验真且 `actor_token_type` 必填，actor 身份以嵌套 `act` 结构入令牌（深度上限 3）；`issued_token_type` 返回**实际**签发类型而非回显请求值；`resource`/`audience` 越界或两者同时给出返回 400 `invalid_target`；subject_token 重放、过期或签名无效返回 400 `invalid_grant`；跨租户 subject_token 按 404 防枚举口径拒绝（AC-TENANT-002）；换发链路审计记录 subject/actor/目标 client。

**验证锚点**：`internal/app/auth/token_exchange_conformance_test.go:94::TestTokenExchange_RejectsForeignSubjectToken`、`:107::TestTokenExchange_RequiresClientAuthentication`；短期/审计 `internal/app/auth/rfc8693_shortlived_test.go:52::TestTokenExchange_FederatedTrustMustMatchCallerOrganization`；主体解析 `internal/app/auth/rfc8693_federation_test.go:89::TestResolveJWTSubject_FederationFound`。

### AC-TOKEN-013 DCR 管理凭据与防枚举（RFC 7591 §3.2.1 / RFC 7592，[14.28](design/protocol/oauth2.md#1428-dynamic-client-registration-管理rfc-75917592)）

**Given** 客户端通过 `POST /oauth2/register` 完成动态注册
**When** 之后携带 `registration_access_token` 调用 `GET/PUT/DELETE /oauth2/register/{clientID}`
**Then** 同时成立以下四条：

1. **一次性下发**：注册响应体含 `registration_access_token`（不透明，`gl_rat_` 前缀）与 `registration_client_uri`，与该 client 绑定；
2. **只存哈希**：库中保存的是令牌的 SHA-256 哈希（`applications.registration_access_token_hash`），明文不落库、不写日志；
3. **强制校验**：三个管理操作均校验凭据，凭据缺失/错误返回 401；凭据可从 `Authorization: Bearer` 或请求体 `registration_access_token` 提取；
4. **防枚举**：凭据缺失/错误/**客户端不存在**/**非 dcr 类别**四种情形的响应体（Code 与 Message）**逐字相同**——不得借状态码或文案区分客户端是否存在；比较使用常量时间算法。

**验证锚点**：`internal/app/applications/dcr_auth_test.go:180::TestDCR_RegistrationIssuesRegistrationAccessToken`（一次性下发 `gl_rat_` 与 `registration_client_uri`）、`internal/app/applications/dcr_auth_test.go:61::TestDCR_ManagementRequiresRegistrationAccessToken`（三个管理操作强制校验凭据）、`internal/app/applications/dcr_auth_test.go:135::TestDCR_UnknownClientDoesNotLeakExistence`（凭据错误与客户端不存在响应不可区分）、`internal/app/applications/dcr_auth_test.go:156::TestDCR_ManagementSucceedsWithTheIssuedToken`。

### AC-TOKEN-014 redirect_uri 精确匹配与 loopback 例外（OAuth 2.1 I-D §1.4 / RFC 8252 §7，[14.6.3](design/protocol/oauth2.md#1463-redirect_uri-精确匹配与不可重定向错误决策表)）

**Given** 应用注册了 `redirect_uris` 集合（管理面或 DCR 写入）
**When** 客户端在授权端点或登录后发码端点提交 `redirectUri`
**Then** 同时成立以下五条：

1. **精确等值**：只有与某个注册值**完整字符串相等**的请求被接受；`/cb` ≠ `/cb/`；注册值不含 query 时请求不得附带额外 query；注册值含 query 时必须全等（含键序）。
2. **无放宽**：不存在通配（`*.example.com` 的注册值**不放行任何**子域，也不放行裸域）、正则、后缀或子串匹配；注册集合为空或只含空白时**拒绝一切**（此前的 fail-open 是开放重定向）。
3. **形状校验**：请求必须是绝对 URI、无 fragment、无 userinfo；scheme 必须为 `https`，唯一例外是 loopback（见第 4 条）。
4. **loopback 例外**：`http` + host ∈ {`127.0.0.1`, `[::1]`}；当注册值**不含端口**时端口可变（RFC 8252 §7 的原生应用场景），`path` 与 `query` 仍须精确相等；两个字面量**不互相认**；`localhost` 与 `*.localhost` 一律拒绝（它们是 DNS 名，不保证留在本机）。
5. **注册期同龄约束**：写入 `redirect_uris` 的值不得含 `*`、fragment、userinfo，必须 https（loopback 例外同上）；管理面与 DCR 走同一校验函数。

**验证锚点**：`internal/domain/applications/redirect_uri_conformance_test.go:31::TestMatchRedirectURI`（精确等值、无通配/正则/子串、query 全等、loopback 端口可变、`localhost` 拒绝）、`internal/domain/applications/redirect_uri_conformance_test.go:111::TestValidateRedirectURI`（注册期形状校验与 422）、`internal/domain/applications/redirect_uri_conformance_test.go:155::TestValidateRedirectURI_AgreesWithTheMatcher`。

### AC-TOKEN-015 DPoP 证明绑定、算法白名单与重放防护（RFC 9449，[14.27](design/protocol/oauth2.md#1427-dpop-服务端-nonce-与共享重放防护rfc-9449)）

**Given** 应用配置 `dpop_required = true`，客户端以带 DPoP 证明的请求换取令牌
**When** 服务端校验证明并签发令牌
**Then** 同时成立以下六条：

1. **`htu` 匹配**：证明的 `htu` 必须等于请求的绝对 URL；服务端用于比较的上界由请求派生（或回落为 issuer + `/api/v1/auth/tokens:exchange`，与 discovery `token_endpoint` 同字面量），**不得为空**——空值会让比较恒失败，使 `dpop_required` 应用无法换码。
2. **算法白名单**：证明的 `alg` 必须属于 `RS256`/`ES256`/`PS256`，且与 JWK 类型一致（RSA 仅 RS256/PS256；EC 仅 ES256 且曲线为 P-256，签名为 64 字节 raw R‖S）；白名单校验在**生产路径**上生效（即包级 `ValidateDPoPProof` 内部）。
3. **时间窗**：`iat` 落在 ±60s 内（RFC 9449 §11.1）。
4. **重放防护**：同一 `jti` 第二次出现即拒绝（缓存有容量上限，不得无界增长）。
5. **绑定落地**：令牌 metadata 写入 `cnf.jkt` = 证明的 JWK 指纹与 `tokenBinding=DPoP`；内省响应对已绑定令牌报 `cnf.jkt` 且 `token_type=DPoP`，未绑定令牌**省略** `cnf`（不得回传 device secret hash 之类无关值）。
6. **错误码**：证明失败返回 `401 invalid_dpop_proof`（不得压成通用 `unauthorized`）。

**验证锚点**：`internal/platform/dpop/dpop_test.go:16::TestValidateDPoPProof_EmptyProof`、`:106::TestValidateDPoPProof_MissingRequiredClaims`、`:133::TestValidateDPoPProof_MethodMismatch`、`:148::TestValidateDPoPProof_UrlMismatch`、`:163::TestValidateDPoPProof_TokenTooOld`、`:178::TestValidateDPoPProof_TokenInFuture`（六条校验）；绑定与错误码 `internal/app/auth/dpop_conformance_test.go`。

### AC-TOKEN-016 会话 sid 与双令牌一致（OIDC Back-Channel Logout 1.0 §2.4，[14.12.3](design/protocol/oauth2.md#14123-sessionssid-列与双令牌一致)）

**Given** 一次成功登录建立了会话
**When** 该会话产生 ID Token（授权码流程），之后该会话被登出并触发后通道投递
**Then** 同时成立以下五条：

1. **同值**：`id_token.sid` 与 `logout_token.sid` **逐字符相等**。
2. **独立标识**：`sid` ≠ `sessions.id`。它是 `gl_sid_` + `crypto/rand` 32 字节（≥128 bit，base64url 无填充）；内部主键是会话 Cookie 的值与 CAS 票据的组成部分，不得作为 `sid` 下发给依附方。
3. **会话为界**：带 `sid` 的登出事件只清理该会话；扇出时为**每个应用**取该用户在该应用下的会话 sid，不得用一个共享值通知所有 RP。
4. **不泄露**：`sid` 不出现在重定向 URL、日志或 API 响应中（仅入令牌）。
5. **可缺省**：会话 sid 缺失（列为 NULL 的存量行或解析不到）时**省略** `sid` 声明，不得下发内部 id 或占位值；`backchannel_logout_session_supported` 为 `true`。

**验证锚点**：`internal/app/auth/session_sid_conformance_test.go:27::TestSessionSid_IsNotTheInternalSessionID`、`internal/app/auth/session_sid_conformance_test.go:94::TestSessionSid_IsTheSameValueInBothTokens`、`internal/app/auth/session_sid_conformance_test.go:147::TestSessionSidFor_ResolvesAndOmits`、`internal/app/auth/session_sid_conformance_test.go:203::TestAuthorizationRedirect_DoesNotLeakTheSid`。

### AC-PROTO-007 协议面错误的状态分类与挑战头（RFC 6749 §5.2 / RFC 6750 §3 / RFC 9110 §11.6.1，[14.16](design/protocol/oauth2.md#1416-协议错误体)）

**Given** 协议面端点（`/oauth2/*`、`/.well-known/*`、`/api/v1/saml/*`、协议面 `/api/v1/auth/*`）收到非法请求或令牌
**When** 服务端返回错误
**Then** 同时成立以下五条：

1. **状态正确**：客户端的输入错误返回 **4xx**，不得因错误对象未携带状态码而被兜底成 500。任何协议面 handler 包**不得**声明「有 `Error() string` 但既无 `HTTPStatus() int` 也无 `ProtocolCode() string`」的类型。
2. **形态正确**：错误体为 `{"error","error_description"}`（受保护资源与令牌端点一致），`error` 取自错误码注册表 §2 的协议码集合。
3. **挑战头**：401 附 `WWW-Authenticate`，值按 RFC 9110 §5.6.4 作 quoted-string 转义，空参数**省略**；受保护资源固定 RFC 6750 §3.1 的 `invalid_token`（不因服务层抛 forbidden 而变成 `insufficient_scope`）；挑战头必须在写体**之前**设置。
4. **不泄露内部消息**：5xx 的 `error_description` 为通用句；4xx 可保留应用消息以便调试，但不得回显堆栈/SQL/内部路径。
5. **429 附 `Retry-After`**。

**验证锚点**：`internal/transport/http/route/oauth_error_test.go:23::TestWriteOAuthError_Envelope`、`:54::TestWriteProtocolError_MapsApplicationCodes`、`:95::TestWriteProtocolError_DoesNotLeakInternalMessage`、`:110::TestWriteProtocolError_TooManyRequestsSetsRetryAfter`、`:120::TestWriteError_DispatchesBySurface`。

### AC-JWKS-001 签名密钥持久化与多 kid 重叠窗口（ADR-015，[observability](architecture/observability.md)）

**Given** 签名密钥由部署期 env/文件密钥库承载（`internal/platform/jwks/manager.go`），`encryption_keys` 为密钥主权目标态（BYOK/HYOK 的缺口与出口条件见 `OPEN-API-39`），系统持有多个 `kid`
**When** 轮换签名密钥
**Then** 新旧密钥重叠窗口 ≥ **2× max token TTL**；重叠期内旧 `kid` 签发的令牌仍可验证，新签发令牌一律使用新 `kid`；私钥材料不落库不落日志。

**验证锚点**：`internal/platform/jwks/keyring.go::LoadKeyRing`、`ParseKeyRing`、`NewManagerWithKeyRing`（`DefaultRetirementOverlap=60d`，`lifecycle.yaml` `rotation_overlap_days: 60`、kid 唯一/前缀/算法闭集/时间序/PEM 解析 fail-close）；`internal/platform/jwks/manager.go::GetKeyPair`、`GetGMKeyPair`、`GetKeyPairByID`、`GetGMKeyPairByID`、`GetJWKS`、`validWithinRetirementWindow`；`internal/platform/crypto/sm2_pem.go::ParseSM2PrivateKeyPEM`；`internal/bootstrap/wire_jwks_keys.go::newJWKSManager`（`JWT_SIGNING_KEYS_FILE` 接线与 ephemeral 告警）；测试 `internal/platform/jwks/keyring_test.go::TestParseKeyRingAcceptsMultipleKeys`、`TestParseKeyRingSelectsNewestActiveKey`、`TestParseKeyRingRejectsInvalidDocuments`、`internal/platform/jwks/manager_keyring_test.go::TestManagerKeyRingRetiredKeyVerifiesUntilOverlapCloses`、`TestManagerKeyRingJWKSPublishesAllWindowKidsWithoutPrivateMaterial`、`TestManagerKeyRingNeverGeneratesMissingFamily`、`TestManagerKeyRingConcurrentReads`、`internal/bootstrap/wire_jwks_keys_test.go::TestNewJWKSManagerWithoutKeyRingWarnsEphemeral`、`TestNewJWKSManagerLoadsKeyRingFile`。

### AC-JWKS-002 轮换期间 JWKS 发布与令牌验证

**Given** 签名密钥轮换处于重叠窗口
**When** RP 拉取 JWKS 并验证令牌
**Then** JWKS 同时发布重叠期内全部 `kid`；已退役（`retired`/`revoked`）密钥从 JWKS 移除后，其在重叠窗口内签发且未过期的令牌仍可经本地缓存或回源验证；未知 `kid` 回源一次，回源失败拒绝验签（AC-TOKEN-007 同口径）。

**验证锚点**：`internal/platform/jwks/remote.go::GetPublicKey`（未知 `kid` 单次回源、冷却防抖、按 issuer 限流与 single-flight）；`internal/platform/jwks/remote_unknown_kid_test.go::TestUnknownKidTriggersSingleSourceRefreshAndResolvesNewKey`、`TestUnknownKidAfterRefreshRejects`、`TestUnknownKidCooldownSuppressesRepeatedRefresh`、`TestUnknownKidFloodIsThrottledPerIssuer`、`TestConcurrentUnknownKidLookupsShareOneRefresh`、`TestUnknownKidRefreshFailureRejectsAndDoesNotCacheSuccess`、`TestRetiredKeyRemovedFromJWKSStillVerifiesFromCache`；指标 `guliangiam_jwks_refresh_total`、`guliangiam_jwks_unknown_kid_total`（`internal/platform/jwks/metrics.go`）。

### AC-NCIDAS-001 网关不可达 fail-close

**需求锚点**：`AUTH-077`

**Given** NCI 网关不可达、连接超时或返回 5xx
**When** 发起登录或实名核验
**Then** 返回 503 `unavailable`（结构化错误体），不建立会话、不签发令牌、不写实名标记；不自动回退密码登录、不复用上一次成功结果；审计记录 `gateway_code`。

**验证锚点**：NCI 规划态；设计条款 `docs/design/auth/network-identity.md` §3/§5.3；错误码 `unavailable`（`contracts/registries/errors.yaml`）；同批交付测试（当前不存在）：`internal/infra/identity/ncidas/*_test.go`（失败注入）。

### AC-NCIDAS-002 无明文个人数据

**需求锚点**：`AUTH-078`

**Given** 认证或实名核验成功
**When** 写入 `records` 与 `network_identity_sessions`
**Then** 两张事实源中不存在明文姓名、证件号、人脸图像或原始报文；身份材料仅为去标识化 `subject_ref`；列清单与审计字段断言覆盖该约束。

**验证锚点**：NCI 规划态；`contracts/schema/20_identity.yaml`、`contracts/schema/60_audit.yaml`（现有声明不含 `network_identity_sessions`，目标表随适配器同批引入）；同批交付测试（当前不存在）：`internal/infra/identity/ncidas/*_test.go`（载荷断言）。

### AC-NCIDAS-003 时钟偏移自检拒绝启用

**需求锚点**：`AUTH-077`

**Given** 适配器启用前执行网关与本地时钟自检，并在每次发起认证前复核
**When** 时钟偏移绝对值 > **3 秒**
**Then** 适配器拒绝启用或返回 `unavailable`，不发起任何认证请求；偏移 ≤3 秒时方可启用；拒绝启用写入审计。

**验证锚点**：NCI 规划态；`contracts/registries/crypto.yaml` 的 `gm_tls`（通用国密 TLS 材料自检见 `internal/platform/crypto/gm_tls_test.go`，仅证明通用能力，不能证明 NCI 已实现）；≤/>3 秒时钟自检随适配器同批交付（当前不存在），落地测试路径 `internal/infra/identity/ncidas`（时钟自检）。

### AC-NCIDAS-004 重复核验幂等收敛

**需求锚点**：`AUTH-079`

**Given** 同一网络身份应用标识（`subject_ref`）的核验请求重复或并发到达（≥2 个请求）
**When** 登录模式或实名模式处理回调
**Then** 幂等收敛为一条身份记录，不创建重复身份记录；登录模式恰好建立一个会话，实名模式不重复写实名标记；已消费 `biz_seq` 的重放返回 409 `conflict`。

**验证锚点**：NCI 规划态；`uq_identity_provider_subject`（`contracts/schema/20_identity.yaml`）仅覆盖 `user_identities` 身份绑定唯一性，不证明 `biz_seq` 重放/并发幂等；错误码 `conflict`（`contracts/registries/errors.yaml`）；同批交付测试（当前不存在）：`internal/app/networkidentity/*_test.go`（并发重放）。

### AC-NCIDAS-005 实名不创建登录会话且状态隔离

**需求锚点**：`AUTH-080`

**Given** `mode=real_name` 或 `mode=login` 的核验请求
**When** 网关返回 `Verified=true`
**Then** 实名模式只写实名标记，**不建立会话、不下发 Cookie 或令牌**；登录模式建立会话；登录与实名共用同一网关适配器，但会话状态、`biz_seq` 绑定与写入目标互不污染。

**验证锚点**：NCI 规划态；`docs/design/auth/network-identity.md` §5 流程隔离表；同批交付测试（当前不存在）：`internal/app/networkidentity/*_test.go`（登录/实名分离）；失败路径复用 AC-NCIDAS-001。

### AC-NCIDAS-006 登录成功与会话建立

**需求锚点**：`AUTH-080`

**Given** 组织与应用已启用 `network_identity`，服务端生成的 `biz_seq`（32 位十六进制）未过期且绑定 `org_id`/`app_id`/`mode=login`
**When** 网关回调核验返回 `Verified=true`
**Then** 消费 `biz_seq` 并建立安全会话，会话 `acr=ncidas`、`amr=network_identity`；身份绑定仅写去标识化 `subject_ref`，不写姓名、证件号或人脸；同一 `biz_seq` 不产生第二个会话。

**验证锚点**：NCI 规划态；设计条款 `docs/design/auth/network-identity.md` §5.1（登录流程第 4–6 步）与 §10.1 的 AC-NCIDAS-006~008 映射；目标表 `network_identity_sessions` 当前未在声明层定义，随适配器同批引入 `contracts/schema/20_identity.yaml`（列清单以声明为准）；断言：`Verified=true` 后 `biz_seq` 一次性消费、会话 `acr=ncidas`/`amr=network_identity`、同一 `biz_seq` 并发或重放只产生 1 个会话；同批交付测试（当前不存在）：`internal/app/networkidentity/*_test.go`（会话建立与单会话计数）。

### AC-NCIDAS-007 租户与应用绑定及越权拒绝

**需求锚点**：`AUTH-080`

**Given** `biz_seq` 属于组织 A、应用 A、`mode=login`
**When** 在组织 B、应用 B 或不同 `mode` 下重放回调
**Then** 返回 404 `not_found`，与资源不存在的响应状态、错误体形态与时延不可区分；响应不泄露组织 A/应用 A 的信息；拒绝写入审计。

**验证锚点**：NCI 规划态；设计条款 `docs/design/auth/network-identity.md` §5.3（`biz_seq` 不存在/跨组织/跨应用 → 404 `not_found`）与 §10.1 的 AC-NCIDAS-006~008 映射；错误码 `not_found`（`contracts/registries/errors.yaml`）；断言：组织 B、应用 B 或不同 `mode` 重放与「资源不存在」在状态码、错误体形态与响应时延上不可区分，响应不回显组织 A/应用 A 信息且不建立会话；同批交付测试（当前不存在）：`internal/app/networkidentity/*_test.go`（跨租户重放与响应形态对比，审计留痕见 AC-NCIDAS-008）。

### AC-NCIDAS-008 审计字段最小化

**需求锚点**：`AUTH-078`

**Given** 登录、实名、拒绝或重放请求
**When** 写入 `records`
**Then** 仅记录 `biz_seq`、`subject_ref`、`verify_result`、`gateway_code`；不记录姓名、证件号、人脸、证书、会话密钥、密文或原始报文；成功、拒绝、越权与重放均留痕。

**验证锚点**：NCI 规划态；设计条款 `docs/design/auth/network-identity.md` §6（审计口径）与 §10.1 的 AC-NCIDAS-006~008 映射；`contracts/schema/60_audit.yaml`（`records` 行结构）；断言：审计载荷仅 `biz_seq`、`subject_ref`、`verify_result`、`gateway_code` 四项，成功、拒绝、越权与重放四类事件均留痕，且不含姓名、证件号、人脸、证书、会话密钥、密文或原始报文；同批交付测试（当前不存在）：`internal/app/networkidentity/*_test.go`（审计字段最小化断言）。

> **规划态备注**：`AC-NCIDAS-001` ~ `AC-NCIDAS-008` 的「NCI 规划态」表示运行期启用依赖外部 NCI 网关准入与同批交付（适配器、`network_identity_sessions` 声明、provider kind 条目与验收测试）；指向 `contracts/schema/**`、`contracts/registries/**`、`internal/platform/crypto/gm_tls_test.go` 与设计条款的锚点只能证明通用能力（结构声明、错误码登记、国密 TLS 材料自检），不能证明 NCI 已实现；同批交付测试路径当前不存在。

### NCIDAS 七子句覆盖矩阵

| AC 编号 | 成功 | 拒绝 | 越权 | 重复 | 并发 | 依赖不可用 | 审计 |
|---|---|---|---|---|---|---|---|
| AC-NCIDAS-001 | N/A（fail-close 无成功路径） | ✅（503 `unavailable`） | ➕ AC-NCIDAS-007 | N/A（失败路径无幂等副作用） | N/A（失败路径不写状态） | ✅（本条即依赖不可用子句） | ✅ |
| AC-NCIDAS-002 | ✅（成功数据无明文个人数据） | ➕ AC-NCIDAS-001 | ➕ AC-NCIDAS-007 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-001 | ✅ |
| AC-NCIDAS-003 | ✅（偏移 ≤3 秒可启用） | ✅（>3 秒拒绝启用） | N/A（启动自检无租户上下文） | N/A（自检无重复副作用） | N/A（启动期单次判定） | ✅（时钟源不可用即拒绝启用） | ✅ |
| AC-NCIDAS-004 | ✅（幂等收敛） | ➕ AC-NCIDAS-001 | ➕ AC-NCIDAS-007 | ✅（本条即重复子句） | ✅（本条即并发子句） | ➕ AC-NCIDAS-001 | ✅ |
| AC-NCIDAS-005 | ✅（实名标记不建会话） | ➕ AC-NCIDAS-001 | ➕ AC-NCIDAS-007 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-001 | ✅ |
| AC-NCIDAS-006 | ✅ | ➕ AC-NCIDAS-001 | ➕ AC-NCIDAS-007 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-001 | ✅ |
| AC-NCIDAS-007 | N/A（绑定校验无成功路径） | ✅（404 `not_found` 防枚举） | ✅（本条即越权子句） | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-001 | ✅ |
| AC-NCIDAS-008 | ✅（字段齐备） | ➕ AC-NCIDAS-001 | ➕ AC-NCIDAS-007 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-004 | ➕ AC-NCIDAS-001 | ✅ |

## 2. 租户、授权与安全

### AC-TENANT-001 租户隔离

**Given** 用户只属于组织 A
**When** 请求组织 B 的资源或伪造组织参数
**Then** 返回 404 `not_found`（C.1），与「资源不存在」的响应状态码、错误体形态与时延不可区分（**404 防枚举**）；拒绝原因写入审计，响应体不回显组织 B 侧任何信息。

**验证锚点**：`internal/transport/http/api/v1/users/tenant_byid_test.go:266::TestUserByID_ForeignTenantRejected`（伪造 query `organizationId` 被忽略，跨租户 by-ID 全为 404 且不回显、不写；该用例同时断言拒绝路径当前不写审计，与本条「拒绝原因写入审计」不一致，审计子句未达标）。

### AC-TENANT-002 跨租户资源

**Given** 用户属于组织 A，资源属于组织 B
**When** 用户请求资源详情或修改资源
**Then** 跨租户请求一律 **404 `not_found`**（防枚举——与同租户内不存在的资源不可区分，消除「统一 403/404」二义）；仅当同租户内身份已认证且策略明确判定「资源存在但无权」时才返回 403 `forbidden`（403/503 判定规则见七子句节）；拒绝原因（不含敏感目标信息）写入审计。

**验证锚点**：`internal/transport/http/api/v1/users/tenant_byid_test.go:266::TestUserByID_ForeignTenantRejected`、`internal/transport/http/api/v1/users/tenant_delete_get_test.go:248::TestUserReadByID_ForeignTenantRejected`（跨租户详情/子资源 404、无子查询、无写；同一用例同时断言拒绝路径当前不写审计，与本条「拒绝原因写入审计」不一致，审计子句未达标）。

### AC-AUDIT-DENIAL-001 拒绝审计的降级与去重

**Given** 跨租户/权限拒绝事件产生
**When** 审计写入受限流或存储故障
**Then** 响应状态不受影响；同一 `(actor, target, resourceType, reason)` 60s 内只落一条明细，重复次数计入计数；超限明细丢弃并告警，损失计入 `guliangiam_audit_denial_dropped_total`；L1 业务层显式判定跨租户，L2 中间件仅兜底 403，404 不由中间件冒充。

**验证锚点**：`internal/platform/auditdenial/middleware.go`（`AuthorizationDenialMiddleware` 仅对 403 兜底）；`internal/platform/auditdenial/middleware_test.go:16::TestAuthorizationDenialMiddlewareRecordsOnly403`；`internal/app/auditdenial/recorder.go`（60s 去重 + 独立令牌桶 + 丢弃计数 `guliangiam_audit_denial_dropped_total`），`internal/app/auditdenial/recorder_test.go:20::TestRecorderDeduplicatesWithinWindow`、`:47::TestRecorderIgnoresMissingTenant`。

### AC-TENANT-003 RLS 隔离（ADR-016）

**Given** 租户表启用 Row-Level Security，且请求已建立租户上下文
**When** 未设置租户上下文就查询租户表
**Then** 返回空结果而非全量数据；管理员放行走独立策略，且上下文在事务内局部生效。

**验证锚点**：`internal/platform/db/rls.go`（`SetRLSContext` 以 `set_config(..., true)` 在事务内注入 `app.current_role` / `app.current_organization_id`）；`internal/infra/postgres/uow/rls_integration_test.go:17::TestSetTenantContextFailsClosedAndResetsPooledConnection`；应用层严格模式谓词 `internal/infra/postgres/tenant_filter_test.go:81::TestTenantWhereClause_StrictModeEmptyOrg_Panics`；结构门禁 `scripts/check-rls.sh`。

### AC-RLS-001 租户表 RLS 全量强制（ADR-016）

**Given** 所有租户域数据表
**When** 检查 DDL 与数据库策略
**Then** 每张租户表均 `ENABLE ROW LEVEL SECURITY` 且 `FORCE ROW LEVEL SECURITY`（表 owner 同样受策略约束）；策略谓词强制 `organization_id` 与会话租户上下文匹配；管理员放行走独立受控策略；RLS 隔离回归测试纳入 CI。

**验证锚点**：结构门禁 `scripts/check-rls.sh`（生成 SQL 必须 `ENABLE`+`FORCE ROW LEVEL SECURITY`，应用角色不得是 owner）、`scripts/check-rls-policy-content.py`（策略谓词内容）；运行期回归 `internal/infra/postgres/uow/rls_integration_test.go:17::TestSetTenantContextFailsClosedAndResetsPooledConnection`、`internal/infra/postgres/uow/rls_cross_integration_test.go:34::TestTenantFilterCrossScopesToSingleTenant`。

### AC-RLS-002 连接租户上下文生命周期（ADR-016）

**Given** 连接池中的连接归还
**When** 执行 `RESET ALL` 清理会话状态
**Then** 租户上下文不跨请求残留；未设置租户上下文的查询命中 RLS 策略被**拒绝**（空结果/报错），绝不返回跨租户数据（与应用层谓词构成双防线，AC-TENANT-003 同源）。

**验证锚点**：`internal/infra/postgres/uow/rls_integration_test.go:17::TestSetTenantContextFailsClosedAndResetsPooledConnection`（`RESET ALL` 后上下文不残留、无租户上下文的查询被 RLS 拒绝）；`internal/platform/db/rls.go`（连接池生命周期与 GUC 陷阱注释）；`internal/infra/postgres/tenant_filter_test.go:10::TestWithTenantScope`。

### AC-PERM-001 fail-close

**Given** 授权引擎不可用或策略不确定
**When** 请求受保护资源
**Then** 不得放行；按 **403/503 判定规则**二选一：策略明确 deny → 403 `forbidden`；授权引擎/依赖不可用或策略不确定 → **503 `unavailable`**（C.1，消除「503/拒绝错误」二义）；同时触发告警并写审计，禁止 fail-open 或以 200 空数据替代。

**验证锚点**：`internal/transport/http/api/v1/auth/authorization_test.go:369::TestAPIAuthorizationMiddleware_PrivilegedSessionFailsClosedWithoutCapability`（授权能力缺失返回 503 且不进入 handler）、`internal/transport/http/api/v1/auth/authorization_test.go:326::TestAPIAuthorizationMiddleware_OrdinarySessionCannotUseJIT`（策略拒绝返回 403 且不进入 handler）。

### AC-PERM-002 权限变更

**Given** 角色权限被修改
**When** 用户立即访问受保护资源
**Then** 策略缓存由 `user.updated`/`org.updated` 类领域事件驱动主动失效，且 TTL 上界 **30s** 内自然过期兜底（[observability](architecture/observability.md) 失效联动：事件丢失/延迟时以 30s 上界为准），旧权限生效时间不得超过 30s + 单次请求处理时长；并发读写观察到的权限要么为旧值要么为新值，不出现混合态。

**验证锚点**：`internal/infra/casbin/cache.go:41`（enforce 决策缓存 TTL = 5s，≤30s 上界）、`internal/infra/casbin/cache_test.go:54::TestEnforceCache_ExpiredEntriesEvicted`、`:107::TestEnforceCache_Invalidate`；策略变更后的主动失效见 `internal/infra/casbin/manager.go:428`（写策略后 `Invalidate`/`InvalidateOrg` + 决策缓存清空）。注：`internal/app/auth/permissions_query.go:202` 的 5 分钟缓存只服务于控制台菜单可见性（非安全判定），不改变后端 enforce 结果。

### AC-PERM-003 受保护路由必须声明权限（缺失即拒绝启动）

**Given** 受保护路由集合
**When** 进程启动并完成路由注册
**Then** 遍历全部受保护 `route.Definition`，逐条断言 `Permission` 非空；任一为空则**记录违例并拒绝启动**（fail-close），不得以「无权限声明则放行」处理。

**验证锚点**：`internal/bootstrap/route_contract.go:26`（非公开路由 Permission 为空即拒启动）、`internal/bootstrap/route_contract_test.go:199::TestRouteContractMatchesRuntimeRouter`（声明集合 ⇔ 运行期路由集合）。

### AC-PERM-004 公开白名单精确到「方法 + 路径模板」

**Given** 默认拒绝策略与 PERM-012 的精确白名单（方法 + 路径模板）
**When** 对全量路由执行豁免判定
**Then** 全部断言成立方可判通过：

| # | 断言 |
|---|------|
| 1 | 白名单为**单一声明**（同一常量集合），不存在并行的前缀式白名单实现 |
| 2 | 白名单条目全部为「方法 + 路径模板」，不存在整前缀豁免 |
| 3 | 管理面条目（含 `GET /api/v1/auth/account`、`GET /api/v1/auth/permissions`、`POST /api/v1/auth/exit-impersonate`、`/api/v1/auth/login-challenges/*`、`/api/v1/auth/consents:*`、`/api/v1/auth/client:authenticate`、`/api/v1/auth/dpop:validate`）**不命中**任何豁免项 |
| 4 | 豁免判定路径唯一，不存在未被统一判定的放行分支 |
| 5 | 启动自检断言「管理面路径不得命中任何豁免项」，违例时**拒绝启动**（与 AC-PERM-003 同一机制） |

**验证锚点**：`internal/transport/http/api/v1/auth/auth_paths.go`（单一声明的方法+路径精确白名单，`IsPublicRequest` 为唯一判定入口）、`internal/bootstrap/route_contract.go:26`（启动期逐条校验豁免判定）；精确到段的回归：`internal/transport/http/api/v1/auth/login_challenge_paths_test.go:16::TestIsTrulyPublicRequest_LoginChallengeReadMatchesRealSegment`、`internal/transport/http/api/v1/auth/mfa_push_paths_test.go:19::TestPushMfaPathsAreSessionRequired`。

### AC-SEC-001 速率限制

**Given** 登录端速率限制为 10 req/min
**When** 同一来源在窗口内超过限制
**Then** 返回 429、`Retry-After` 和限额头；高风险接口不得因 Redis/缓存故障而无限放行；协议面限流按 `oauth2/token`、`oauth2/introspect`、`oauth2/revoke` 三个端点族独立配置（唯一口径见 `contracts/registries/limits.yaml`）。

**验证锚点**：`internal/platform/middleware/ratelimit_test.go:113::TestRateLimit_DeniedWritesUnifiedErrorBody`、`:138::TestRateLimit_EmitsIETFHeaders`（IETF 限额头）；Redis 耗尽/故障路径 `internal/platform/middleware/ratelimit_redis_test.go:25::TestRateLimit_Redis_ExhaustBurst`、`:59::TestRateLimit_Redis_DeniedWritesUnifiedErrorBody`；限流口径 `contracts/registries/limits.yaml`。

### AC-SEC-002 CSRF

**Given** 用户拥有会话 Cookie
**When** 修改请求缺少或伪造 CSRF Token
**Then** 返回 403 `forbidden`（策略/伪造类拒绝，按七子句节 403/503 判定规则），不执行写入，并计入审计。

**验证锚点**：`internal/platform/middleware/csrf_test.go:102::TestCSRFTokenMiddleware_MutatingMethod_InvalidToken`、`internal/platform/middleware/csrf_test.go:124::TestCSRFTokenMiddleware_InvalidTokenCanonicalEnvelope`（403 `forbidden` + 统一错误体）、`internal/platform/middleware/csrf_test.go:139::TestCSRFTokenMiddleware_MutatingMethod_MissingHeader`、`internal/platform/middleware/csrf_test.go:160::TestCSRFTokenMiddleware_MutatingMethod_MissingCookie`（缺 token/cookie 均拒绝且不进入 handler）。

### AC-SEC-003 SSRF 与上传

**Given** Webhook 指向云元数据地址或上传超过配置大小
**When** 触发投递或上传
**Then** 请求被拒绝（400 `invalid_argument`）并记录安全事件；出站 SSRF 按 [outbox](design/infrastructure/outbox.md) 四规则管控：禁 RFC 1918 全段/`127.0.0.0/8`/`::1`/链路本地 `169.254.0.0/16`；默认禁重定向（豁免限 1 跳且对目标重新全量校验）；DNS 解析结果**固定 IP 连接**（防 rebinding）；URL scheme 白名单 `https`（开发环境放宽 `http` 并启动日志标注）。不写入文件系统边界外路径。

**验证锚点**：`internal/infra/webhook/http_sender_test.go:92::TestHTTPDeliverySender_Send_InvalidURL`、`internal/infra/webhook/http_sender_test.go:99::TestHTTPDeliverySender_Send_LoopbackBlocked`、`internal/infra/webhook/http_sender_test.go:107::TestHTTPDeliverySender_Send_MetadataBlocked`（非法 URL、loopback、云元数据地址均被 SSRF 拦截）。

### AC-TENANT-004 跨租户写并发隔离（并发/重复子句补齐）

**Given** 两个组织并发创建各自组织内相同的业务唯一键（如同名 username/slug）
**When** 并发提交
**Then** 各组织内各自创建成功（部分唯一索引按组织隔离，AC-DATA-005 同源），互不冲突；全局唯一键（如 WebAuthn `credentialId`）并发插入恰好一个成功、其余 409 `conflict`；任何一方在各自事务边界内读取不到对方数据（应用层谓词 + RLS 双防线，AC-TENANT-003）。

**验证锚点**：`internal/app/users/crud_test.go:923::TestCreateUser_DuplicateUsername`（同组织重复唯一冲突）；全局唯一键 `internal/app/users/webauthn_admin_test.go:578::TestFinishAdminWebAuthnRegistration_DuplicateCredentialIsAlreadyExists`；部分唯一索引声明 `contracts/schema/20_identity.yaml`（`uq_users_org_username_active`）与 `credential_id` 唯一约束；`[证据不足：当前 L4，两组织并发创建与跨租户读写并发需 L6 夹具]`。

### AC-SEC-004 IP 黑白名单（SEC-001/002）

**Given** 组织配置 IP 名单（Allow 与 Block 并存）
**When** 请求命中名单
**Then** **Allow 优先于 Block**（SEC-001）；匹配支持 CIDR/精确/范围三种方式（SEC-002）；名单外默认放行或默认拒绝的取向必须在配置中**显式声明**，不允许隐式行为；命中拒绝返回 403 `forbidden` 并审计命中规则 ID；名单变更经缓存失效后 ≤30s 生效（[observability](architecture/observability.md)）。

**验证锚点**：`internal/transport/http/middleware/ip_rules_pure_test.go:11::TestMatchRuleConditions_IP_EQ`、`internal/transport/http/middleware/ip_rules_pure_test.go:27::TestMatchRuleConditions_IP_CIDR`、`internal/transport/http/middleware/ip_rules_pure_test.go:51::TestMatchRuleConditions_IP_Range`（精确/CIDR/范围三种匹配）、`internal/transport/http/middleware/ip_rules_pure_test.go:165::TestIPRulesMiddleware_isIPBlocked_WhitelistPrecedence`（Allow 优先于 Block）。

### AC-SEC-005 审计查询与证据导出

**Given** 管理员或审计员（IGA-007 只读视角）查询审计
**When** 按 actor/目标/时间段/组织过滤并导出
**Then** 查询为只读，越权访问返回 403 `forbidden`；分页遵循统一列表封装（AC-API-002）；按 `request_id` 检索走 `records.request_id` 列直查（`records.request_id` 列直查，配部分索引）；导出含清单与校验和，导出操作本身写审计。

**验证锚点**：`internal/transport/http/api/v1/records/handler.go:112`（`ExportManifest` 含清单/计数/SHA-256）、`:133`（导出落 `records.exported` 审计）；`internal/infra/postgres/records/chain_sql.go:61`（含 `request_id` 的批查列）；租户审计回归 `internal/transport/http/api/v1/records/tenant_audit_regression_test.go:133::TestCreateRecord_ActorIPUserAgentDerivedFromRequest`。

### AC-AUDITCHAIN-001 审计哈希链防篡改（RISK-013，ADR-031）

**Given** `records` 审计记录按链式哈希写入（每条记录含前条哈希）
**When** 单点篡改任一记录后运行链完整性校验工具
**Then** 校验工具发现篡改并**定位断链位置**（断点及其后续记录全部报验失败）；链构建不阻塞审计写入；链构建状态机为 `pending → chained → anchored`（异常分支 `broken`），链构建/锚定失败只重试并告警、不阻塞业务；校验工具只读、越权调用 403 `forbidden`；初始化/轮转流程可重放。

**验证锚点**：`records.chain_status` 写入即 `chained`（`internal/infra/postgres/records/sql.go::queryCreateRecord`）；锚点写入后按锚点覆盖范围推进为 `anchored`（`internal/infra/postgres/records/chain_sql.go` + `internal/platform/outbox/providers/audit/chain.go::Anchor`，锚点与状态推进同一语句）；`ChainWorker` 经 `recover.SafeGo` 启动（`internal/platform/outbox/providers/audit/chain_worker.go`）；只读校验按 `chain_seq` 定位首个断点（`internal/app/risk/service.go::VerifyAuditIntegrity`）：`internal/platform/outbox/providers/audit/chain_verify_test.go:64::TestVerifyChainRecords_DetectsTamperedEntry`、`internal/platform/outbox/providers/audit/chain_verify_test.go:75::TestVerifyChainRecords_SM3TamperLocatesSeq`、`internal/platform/outbox/providers/audit/chain_verify_test.go:86::TestVerifyChainRecords_DetectsBrokenPrevHash`、`internal/platform/outbox/providers/audit/chain_verify_test.go:96::TestVerifyChainRecords_RejectsMixedAlgorithms`。

### AC-AUDITCHAIN-002 链锚定与 WORM 归档独立验证（RISK-013/014，ADR-031）

**Given** 链头/链尾定期快照锚定于 `audit_chain_anchors`（含外部时间戳或异地存储）
**When** 执行 WORM 归档导出
**Then** 导出包含清单与校验和，可由**独立第三方工具**基于锚点快照离线验证（不依赖平台在线服务）；锚点与归档操作全部审计；保留期内 WORM 目标不可改写/删除。

**验证锚点**：`internal/platform/outbox/providers/audit/chain_anchor_test.go:33::TestComputeAnchorProof_DeterministicRecipe`、`:164::TestAnchor_NewTailInsertsAnchorAndAdvancesStatus`、`:181::TestAnchor_TailAlreadyAnchoredBackfillsWithoutInsert`、`:196::TestAnchor_ExternalProofIsStoredVerbatim`（锚点快照/外部证明/状态推进）；导出清单与校验和 `internal/transport/http/api/v1/records/handler.go:112`（`ExportManifest` + SHA-256）；`[证据不足：当前 L4，独立第三方离线验证工具与 WORM 保留期不可改写未发现，需确认]` → DEF-29。

### AC-SEC-006 SBOM 与供应链门禁（STD 9.9）

**Given** release 流水线
**When** 构建制品
**Then** 生成 SPDX JSON 格式 SBOM（Syft 固定版本，STD 9.9）并随 release 归档，依赖锁版本；SBOM 产物缺失或损坏时 release job 断言失败（扫描非阻断，但**产物完整性必须断言**）；CI 作业断言失败即阻塞合并。

**验证锚点**：`scripts/release/generate_sbom.sh`（`SYFT_VERSION` 固定、`spdx-json` 输出、`check_sbom.py` 校验、`sha256` 归档）、`scripts/release/check_sbom.py`（缺失/损坏即非零退出）；流水线接线 `.github/workflows/release.yml`。

## 3. 协议与集成

### AC-API-006 分页边界与 422（4.7.1）

**Given** 客户端请求任一列表端点
**When** 携带 `page` 与/或 `pageSize`
**Then** 同时成立以下五条：

1. **缺省放行**：未携带这两个参数不是错误，服务端按默认值处理（`page=1`、`pageSize=20`）。
2. **存在即必须可用**：`page`/`pageSize` 非整数、`page < 1`、`pageSize < 1`、`pageSize > 100` 一律返回 **422**，`details` 含 `{field, value, max}`——不得静默回退默认值（调用方会一直在第 1 页而不知为何）。
3. **上界统一为 100**：OpenAPI 的 `PageSize.maximum`、规范 4.7.1 与服务端强制值三者一致，且**不得静默截断**。
4. **LIMIT 与 OFFSET 同源**：同一页的 LIMIT 与 OFFSET 由**同一个**钳制后的页大小导出，`pageSize ∈ [1,19]` 时相邻页不得重叠。
5. **小页如实返回**：请求 5 条不得返回 20 条（`max(pageSize, N)` 是地板不是天花板）。

**验证锚点**：`internal/transport/http/middleware/pagination_test.go`（页大小上限/回落/非法 422）；门禁 `python3 scripts/check-api-params.py`。

### AC-API-007 排序白名单、tiebreaker 与 422（4.8.1）

**Given** 列表端点接收 `sortField` + `sortOrder`
**When** 客户端请求排序
**Then** 同时成立以下五条：

1. **camelCase 白名单**：白名单键是 API 字段名（如 `createdAt`），映射到物理列（`created_at`）；数据库列名不得出现在 API 参数中。
2. **非法即 422**：白名单外字段（含 snake_case 写法、敏感列、注入尝试）与 `sortOrder ∉ {asc, desc}` 一律返回 422，`details` 列出允许集合——不得回退为默认排序。
3. **稳定排序**：任何 `ORDER BY` 末尾强制追加唯一键 tiebreaker（`ORDER BY <col> <dir>, id ASC`）。
4. **参数真正生效**：所有**声明**了排序参数的端点其值必须进入 SQL；不得出现「handler 读取、仓储丢弃、语句硬编码」的断链。
5. **注入免疫**：进入语句文本的只有白名单列名与 `ASC`/`DESC` 两个字面量，请求值从不进入；`sortField=username; DROP TABLE …` 之类一律被拒。

**验证锚点**：`internal/platform/sorting/sorting_test.go:63::TestSpec_RejectsFieldsOutsideTheWhitelist`、`:42::TestSpec_AlwaysAppendsTheTiebreaker`、`:186::TestSpec_CustomTiebreaker`；SQL 列名门禁 `python3 scripts/check-sql-columns.py`。

### AC-FEGOV-007 控制台发出的排序与分页取值必须在服务端值域内

**Given** 控制台任一列表页向管理面发出 `sortField` / `pageSize`
**When** 该列表页加载或用户点击表头排序、切换每页条数
**Then** 同时成立以下四条：

1. **排序值域**：发出的 `sortField` 值必须是**某个后端排序白名单**的键（后端全部排序白名单键的并集）。发 `created_at` 这类 snake_case 不是「被忽略」而是 **422**，会让**整个列表请求失败**。
2. **可排序列受白名单约束**：列上启用 `sorter` 即意味着该列会被发往服务端；因此**只有白名单内的列**允许有 `sorter`。跨表列（如 `users` 页面的 `displayName`/`email`，实际位于 `user_profiles`）不得有 `sorter`。
3. **分页上限**：发出的 `pageSize` 必须 ≤ `MaxPageSize`（100）。超限返回 422，不静默截断。
4. **交互不得静默消失**：列表组件在任意行数下都必须保留其配置声明过的分页、勾选与排序能力；不得因跨越某个阈值而改渲染无法承载这些交互的替代实现且无任何提示。

**验证锚点**：`internal/platform/sorting/sorting_test.go`（白名单/tiebreaker）、`internal/transport/http/middleware/pagination_test.go`（分页边界）；门禁 `scripts/check-api-params.py`。

### AC-FEGOV-008 控制台调用的端点必须真实存在，422 必须可操作

**Given** 控制台任一页面/页签发起请求
**When** 请求到达管理面
**Then** 同时成立以下三条：

1. **无幻影端点**：控制台每个 `fetch` 路径都对应一个已注册路由；不得存在「前端调用、后端全仓无此路由」的死调用（此类缺陷表现为页面永久为空或操作恒 404，且不报错到任何门禁）。
2. **端点语义一致**：即使路径存在，也必须与后端**语义**一致——不得把「替换全部」的语义打到「重命名单条」的端点上（这类错配返回 200 却什么也没做，比 404 更难发现）。
3. **422 可定位**：管理面的 422 带有 `error.details`（`{field, value, max|allowed}`）时，控制台必须把它呈现为「字段 + 非法值 + 期望」的可操作提示，而不是丢弃 `details` 只显示一句散文。兼容窗口内 deprecated `error.meta` 与 `details` 同值，控制台统一经 `getErrorDetails` 读取。

**验证锚点**：统一错误体 `internal/transport/http/route/http_test.go`；页面请求↔路由↔字段绑定门禁 `scripts/check-ui-wiring.py`。

### AC-API-008 枚举入参闭集与跨端取值一致（4.14.3）

**Given** 任一枚举字段作为入参被提交，或作为出参被消费
**When** 写入方向提交了一个取值，或读取方向收到一个未知取值
**Then** 同时成立以下五条：

1. **入参闭集**：枚举字段只接受服务端登记值集内的取值；未知值返回 **422** 且 `details` 含 `{field, value, allowed, set}`。不得静默落库、不得悄悄归一化为默认值。
2. **空值与未知值可区分**：缺字段是 400 `invalid_argument`（调用方漏了），未知取值是 422 `unprocessable_entity`（调用方说了个不存在的东西）。两者合并会让客户端重试一个不改就不可能成功的请求。
3. **出参开放**：服务端新增枚举值属向后兼容变更；客户端必须对未知值容错（走原始值兜底展示，不得白屏/崩溃/空单元格）。
4. **取值集事实源是运行时注册表**：闭集的成员是**运行时可解析**的类型。静态可支持但因为密钥未配置而暂未注册的类型应当接受（「先建记录、后配密钥」是合法工作流）；**任何代码路径都不注册**的类型必须拒绝——纳入等于承诺服务端做不到的能力。
5. **跨语言逐值一致**：控制台清单与服务端集合**双向相等**，且由阻塞门禁校验。前端多值 → 表单提交必被 422；服务端多值 → 有后端能力却没有 UI。

**范围边界**：`users.status`/`userType`、`applications.status`、`organizations.status` 等业务枚举同样纳入闭集校验；通用 `providers` 表的 `(category, providerType)` 为**复合**值域（providerType 按 category 取值），按 `map[category]Set` 组织；端点/凭据来自 provider 记录的连接器类型，由工厂支持按记录构造后纳入闭集。

**验证锚点**：门禁 `python3 scripts/check-enums.py`（枚举闭集/跨端一致）；枚举命名口径 `contracts/registries/naming.yaml`（`aigateway_enums` 等）。

### AC-API-009 控制台请求的方法与路径必须命中一条已声明路由

**Given** 控制台任一页面发起请求
**When** 请求到达服务端
**Then** 同时成立以下三条：

1. **动词必须匹配**：请求使用的 HTTP 方法必须是该路径**已声明**的方法。方法不匹配时路由没有对应处理器，请求得不到响应——这是**硬失败**而非软失败，页面会永久空白。
2. **路径形态一致**：`({id}/:action)` 形式的端点，id 在**路径**还是在**请求体**以服务端声明为准。例如 `orders:place` 接收的是 `CreateOrderRequest` 请求体（新建订单），不存在 `orders/{id}:place`。
3. **响应字段真实存在**：控制台声明的字段必须来自服务端 DTO。绑定服务端从不发送的字段，会渲染为空白或**回退成看似真实的假值**。

**验证锚点**：门禁 `scripts/check-ui-wiring.py`（页面请求↔路由声明↔字段绑定）、`scripts/check-http-methods.py`（方法与路径族）；前端路由审计 `web/src/app/router/router.tsx`。

### AC-FEGOV-010 列表页交互控件必须可达服务端

**Given** 列表页渲染筛选下拉或列排序
**When** 用户选择筛选值或点击表头
**Then** 同时成立以下四条：

1. **筛选值必须进入请求**：持有 `filterValues` 的页面必须把它交给请求（约定 `filters: filterValues`）。仅把值传给列表组件的 prop 是**不完整**的——组件只负责渲染与回传，不代为请求。
2. **排序回调必须有可排序列**：传 `onSortChange` 的页面必须存在 `sorter:` 列。该回调只在 antd `Table.onChange` 中触发，仅对可排序列触发；无 `sorter` 时回调永不运行。
3. **筛选键必须是后端读取的参数名**：`key` 取值须为 handler 实际读取的名称，且后端存在对应列/参数。无支撑的筛选项应移除，不得保留为「看着能用、实际无效」的控件。
4. **不得补一个无效的控件**：后端尚未登记 `sorting.Spec` 白名单时，**不得**加 `sorter: true`——服务端不解析 `sortField` 时点击表头只会造出新的静默无效。
5. **服务端排序**：列表排序一律由服务端完成（模块登记 `sorting.Spec` 白名单 + SQL `{ORDER_BY}` 槽位）；仅对当前页生效的客户端排序不满足本条。

**验证锚点**：门禁 `scripts/check-ui-wiring.py`（客户端过滤白名单 `CLIENT_SIDE_FILTER`）与 `scripts/check-http-methods.py`（方法/路径族）。

### AC-API-010 控制台请求的助手族必须与路径族匹配

**Given** 控制台任一层（`shared/api/iam/*` 或 `shared/lib/*`）发起请求
**When** 请求的响应被解析
**Then** 同时成立以下三条：

1. **响应形态必须与 helper 匹配**：`request`/`requestEnvelope`/`requestJson` 只解析 casdoor 的 `{status, msg, data, data2}` 信封，**不得**指向 `v1` 文档面——v1 返回裸文档，`responseEnvelopeSchema` 要求 `status: z.string()`，解析在拿到路由之前就必然抛错。反向地，`requestDocumentJson` 只解析裸文档，不得指向遗留 `/api/*` 信封面。
2. **句柄不得绕过门禁**：新增请求所使用的 helper 必须已被 `check-http-methods.py` 的助手族覆盖（名字含动词或选项含动词），否则该请求不受任何方法与路径检查。
3. **不得指向未注册路由**：路径必须命中 `routes.go` 已声明的 `(Method, Pattern)`。

**验证锚点**：门禁 `python3 scripts/check-http-methods.py`（`PATH_WRAPPERS` 含 `withOrganization`/`withQuery`/`withOrgQuery`）；前端请求封装 `web/src/shared/api/http.ts`。

### AC-API-011 数据访问调用的参数个数必须等于语句绑定

**Given** 仓储执行一条 SQL
**When** 语句被绑定参数
**Then** 同时成立以下三条：

1. **个数必须相等**：调用点传的参数个数必须等于语句绑定的最大 `$N`。不足时 pgx 在运行期拒绝（`wrong number of parameters`），**并且 `PREPARE` 不会失败**——`PREPARE` 看不到调用点，故这一缺陷在语句级门禁下完全隐形。
2. **无绑定即不传参**：语句不含 `$N` 时必须不传任何参数（pgx 同样拒绝多余参数）。
3. **增删谓词必须同步调用点**：给语句增加 `$N` 时，list 与 COUNT **两条**语句及**两个**调用点必须同批更新——COUNT 在行扫描后无条件执行，漏改即整个列表接口失败。
4. **分页边界只能有一个入口**：page size 的上限/回落属于 `shared.ClampPageSize`，偏移属于 `shared.PageOffset`；**包内不得再定义接收 page size 的函数**。

**验证锚点**：门禁 `python3 scripts/check-sql-args.py`（SQL 占位符↔参数个数）、`python3 scripts/check-sql-runtime.py`（真实库 PREPARE 全量回放）；分页入口 `internal/transport/http/middleware/pagination.go`。

### AC-FEGOV-011 筛选下拉的键必须是后端读取的参数

**Given** 列表页渲染筛选下拉
**When** 用户选择筛选值并发出请求
**Then** 同时成立以下三条：

1. **键名必须是后端参数名**：`filters` 条目的 `key` 是**查询参数名**，必须被某个 handler 读取。键名错（如五张表都无 `status` 列却发 `status=enabled`）时下拉渲染、转发、无效。
2. **值必须符合参数类型**：布尔参数的取值是 `"true"`/`"false"`，不是 `"enabled"`/`"disabled"`。
3. **控件必须有后端支撑**：键对应的列不存在且后端无该参数时**移除控件**，不得保留为「看着能用、实际无效」。

**验证锚点**：门禁 `scripts/check-ui-wiring.py`（允许的客户端过滤白名单之外的键必须由服务端读取）与 `scripts/check-api-params.py`。

### AC-PROTO-001 SAML

**Given** SAML 请求签名、Issuer、Audience 和时间窗口有效
**When** 处理 ACS/SLO
**Then** 完成会话联动和审计；签名错误、重放和过期请求被拒绝：AuthnRequest ID 与 Assertion ID 入**共享重放缓存**（Redis，TTL = NotOnOrAfter 窗口 + 5min，[saml](design/auth/saml.md)），时间窗口容忍 ±60s（15.3.6），重复 ID 拒绝并审计。

**验证锚点**：`internal/infra/idp/saml_sp_security_test.go:204::TestSAMLProvider_AcceptsValidResponse`、`internal/infra/idp/saml_sp_security_test.go:283::TestSAMLProvider_RejectsReplayedResponse`（重放拒绝）、`internal/infra/idp/saml_sp_security_test.go:261::TestSAMLProvider_RejectsExpiredAssertion`、`internal/infra/idp/saml_sp_security_test.go:234::TestSAMLProvider_RejectsWrongAudience`、`internal/infra/idp/saml_sp_security_test.go:225::TestSAMLProvider_RejectsUnsignedResponse`。

### AC-PROTO-002 SCIM

**Given** SCIM 客户端已认证且字段通过白名单
**When** 创建或更新 User
**Then** 返回 SCIM 标准资源；重复请求幂等或返回 409 + `scimType=uniqueness`（RFC 7644 §3.7.3，[scim](design/protocol/scim.md)），不产生重复用户。

**验证锚点**：`internal/app/scim/service_uow_extra_test.go:18::TestService_CreateUser_Success`、`:41::TestService_CreateUser_Duplicate`；SCIM 状态码映射 `internal/transport/http/api/v1/scim/chi_test.go:74::TestScimStatusCodeFromError`；`[证据不足：当前 L4，响应体 `scimType` 仅在结构体声明未见赋值，重复请求的 `scimType=uniqueness` 需补实现 → DEF-30]`。

### AC-PROTO-003 LDAP/Kerberos

**Given** 外部目录不可达、超时或字段映射异常
**When** 执行认证/同步
**Then** 认证失败返回 401 `unauthorized`、目录不可达/超时返回 503 `unavailable`（C.1，依赖类），附同步报告；不绕过本地权限，不写入未验证字段。

**验证锚点**：LDAP 登录/会话撤销 `internal/app/auth/session_rotation_revoke_test.go:199::TestLoginWithLDAP_RevokePreviousSessionFails_RollsBackRotation`；LDAP 客户端 `internal/infra/ldap/client_test.go:7::TestLDAPURL`；Kerberos `internal/infra/kerberos/authenticator_test.go:7::TestParseAuthorizationHeader`、`:44::TestNewAuthenticator`；`[证据不足：目录不可达→503 与同步报告需 L6 夹具]`。

### AC-PROTO-004 CAS 代理票据（[saml](design/auth/saml.md)）

**Given** SP 携 `pgtUrl` 调用 serviceValidate
**When** ST 验证通过
**Then** 响应 `<cas:proxyGrantingTicket>` 元素返回 **PGTIOU**（非 PGT 本体），随后 IdP 向 `pgtUrl` 回调投递 `pgtId=PGT-xxx&pgtIou=PGTIOU-xxx`（HTTPS，SSRF 出口管控适用 AC-SEC-003），SP 以 PGTIOU 建立 PGT 映射；回调失败返回 `INVALID_PROXY_CALLBACK`（15.4.5）；票据 TTL 与一次性：ST/PT **5 分钟**、PGT 30 分钟、全部一次性消费；TGT/TGC/PGTIOU 落库（`cas_tickets`）。

**验证锚点**：`internal/app/auth/cas_test.go:386::TestCASServiceValidate_MissingTicket`、`:426::TestCASServiceValidate_InvalidTicket`、`:456::TestCAS_GenerateTicketID`；pgtUrl 出站 SSRF 管控沿用 AC-SEC-003 锚点（webhook sender loopback/metadata 拦截）；`[证据不足：PGTIOU 回调/PT/PGT TTL/落库需 L6]`。

### AC-PROTO-005 SAML SP 侧断言校验细化（[saml](design/auth/saml.md)）

**Given** SP 收到 SAML Response 并处理 ACS
**When** 执行断言校验
**Then** 断言签名（证书链/算法白名单）验证通过；`(issuer, assertionID)` 入共享重放缓存（Redis，TTL = NotOnOrAfter 窗口 + 5min），重复断言拒绝并审计；NotBefore/NotOnOrAfter 容忍 ±60s（15.3.6）；XML Signature Wrapping（XSW）典型攻击用例全部被拒（15.3.5）。

**验证锚点**：`internal/infra/saml/response_test.go:207::TestValidateSAMLResponse_RejectsTamperedAssertion`、`:227::TestValidateSAMLResponse_RejectsReplay`、`:284::TestValidateSAMLResponse_RejectsExpiredAssertion`、`:313::TestMemoryReplayStore_Reserve`；ACS 侧 `internal/app/auth/saml_sp_acs_test.go:191::TestCompleteSAMLSocialLogin_RejectsReplayedState`、`:239::TestCompleteSAMLSocialLogin_RejectsInvalidSignature`。

### AC-PROTO-006 SCIM 入站同步与 scimType 错误体（[scim](design/protocol/scim.md)）

**Given** SCIM 客户端执行入站同步（30.4 作业）
**When** 创建/更新/删除资源或执行 filter 查询
**Then** filter 语法错误返回 400 `scimType=invalidFilter`（非法字符不得静默跳过，30.3.1）；count 超上限返回 400 `scimType=tooMany`（不得静默 clamp）；唯一冲突返回 409 `scimType=uniqueness`；错误体 `schemas=["urn:ietf:params:scim:api:messages:2.0:Error"]`/`detail`/`status` 与 `scimType` 七码符合 30.3.7；入站作业幂等（重复同步不产生重复用户），下游 409 记死信并映射冲突原因、不自动改写（30.5 冲突解决）。

**验证锚点**：`internal/app/scim/service_uow_extra_test.go:41::TestService_CreateUser_Duplicate`（重复创建冲突）、`:18::TestService_CreateUser_Success`；错误体 `internal/transport/http/route/oauth_error_test.go:157::TestWriteSCIMError_EnvelopeAndMediaType`；`[证据不足：当前 L4，invalidFilter/tooMany/七码 scimType 未见赋值 → DEF-30]`。

## 4. 数据、异步和运维

### AC-DATA-001 乐观锁

**Given** 同一资源 revision 为 1
**When** 两个并发更新同时提交
**Then** 一个更新成功，另一个返回 409，不得静默覆盖。

**验证锚点**：`internal/app/subscriptions/service_test.go:294::TestPatchSubscription_RevisionMismatch`、`internal/app/coupons/service_test.go:328::TestService_PatchCoupon_RevisionMismatch`（revision 冲突→409）；错误码契约 `internal/platform/errors/errors.go`。

### AC-DATA-002 迁移

**Given** 生产 `AUTO_MIGRATE=false`
**When** 发布包含数据库变更的新版本
**Then** 迁移独立执行、先备份、可观测、可前滚，失败时不破坏旧版本读取。

**验证锚点**：`docs/operations/deployment.md`（迁移 Job/备份/观察窗口/回滚）；门禁 `bash scripts/check-migrations.sh`（迁移链完整性/幂等）。

### AC-DATA-003 数据库设计规范门禁

**Given** 以声明式 Schema 为唯一结构源（ADR-022，收敛条件见其出口条件），且 `internal/bootstrap/schema_sql/` 下为 DDL 的嵌入副本
**When** 提交包含 DDL 变更的改动并执行 `bash scripts/check-db-conventions.sh`
**Then** 全部 9 项检查通过：(1) 无 `DROP SCHEMA/TABLE/INDEX` 与 `TRUNCATE`；(2) 无 `uk_`/`gateway_*`/`iam` 等遗留命名；(3) 所有 `CREATE` 带 `IF NOT EXISTS`；(4) 所有含 `updated_at` 的表定义 `revision BIGINT NOT NULL DEFAULT 1`；(5) 唯一约束/索引使用 `uq_`、普通索引用 `idx_`、检查约束用 `ck_`，表级 `UNIQUE` 已显式命名；(6) 无重复索引名；(7) `db/postgresql/` 与 `internal/bootstrap/schema_sql/` SHA-256 逐字节一致；(8) 无 FK 前向引用（内联 `REFERENCES` 的被引用表不得晚于引用点创建）；(9) 定点数列均为 `DECIMAL(19,4)`、币种列均为 `VARCHAR(3)`；任一失败则门禁返回非零。

**验证锚点**：`bash scripts/check-db-conventions.sh`（9 项规范化检查，含命名/IF NOT EXISTS/revision/定点数/币种）；生成物与镜像一致性由 `${(8)}` 项内建 SHA-256 比对覆盖。

### AC-DATA-004 幂等重放

**Given** 一个空的 `guliangiam` schema（通过 `scripts/dev/reset_schema.sql` 创建）
**When** 执行 `ALLOW_SCHEMA_RESET=1 DATABASE_URL=... bash scripts/dev/validate_schema_replay.sh`（squash 后内部按序执行 `reset_schema`、`V1-pass1`、`V1-pass2`、`Seed-V1-pass1`、`Seed-V1-pass2`，均带 `-v ON_ERROR_STOP=1`）
**Then** 五次执行均退出 0，且脚本内置的 8 项断言全部通过：7 项结构断言（无非 `DECIMAL(19,4)` 定点数列、无非 `VARCHAR(3)` 币种列、无非 `TIMESTAMPTZ` 时间列、无缺主键表、主键无数据库函数默认值（UUIDv7 由应用层生成）、三个软删除部分唯一索引齐备、两个前向引用 FK 已补全）+ 1 项 UPSERT 仲裁目标可解析性断言（扫描全部 Go 仓储与 V2 种子中的 `ON CONFLICT`，逐个校验库内存在列集合一致的唯一约束/索引，且谓词有无一致）。

**验证锚点**：`bash scripts/dev/validate_schema_replay.sh`（reset + 两次 V1 + 种子重放 + 8 项断言）；`scripts/dev/reset_schema.sql`。

### AC-DATA-005 软删除下的业务唯一性

**Given** 组织 `A` 下已存在用户 `alice` 并被软删除（`deleted_at IS NOT NULL`）
**When** 在同一组织再次创建用户名 `alice`
**Then** 创建成功；而组织 `A` 下存在未删除的 `alice` 时重复创建返回唯一冲突（由 `uq_users_org_username_active` 部分唯一索引保证）。`organizations.slug` 与 `sites.host` 同规则。

**验证锚点**：声明源 `contracts/schema/20_identity.yaml`（`uq_users_org_username_active` 等部分唯一索引）；`bash scripts/check-db-conventions.sh` 对部分唯一索引齐备性的断言。

### AC-WEBHOOK-001 投递

**Given** Webhook 目标不可达
**When** 触发事件
**Then** 使用幂等键和指数退避重试：退避 `2^n 秒、上限 300s、±20% 抖动`（[outbox](design/infrastructure/outbox.md)）；最大尝试次数 **见 `generated/outbox-registry.md`，值为 12**（Outbox Worker 唯一权威口径，DDL 默认、Worker 配置、AC 均以此为准）；超过上限进入死信（`dead` 行数 >0 即告警）；4xx（除 408/429）不重试、410 直接 `dead`（25.6 状态码分类）；人工重放按 `event_id` 去重（作用域 `(aggregate_type, aggregate_id, event_type)`，去重记录 TTL 7d），不会重复产生不可逆副作用。

**验证锚点**：退避与上限 `internal/platform/outbox/outbox_test.go:118::TestBackoffDuration`；状态码分类（DEF-28）`internal/app/webhooks/delivery_outbox_test.go:168::TestDeliveryOutbox_NonRetryableStatus_FinalizesFailed`、`:184::TestDeliveryOutbox_RetryableStatus_RecordsAttempt`；重放幂等 `internal/app/webhooks/domain_outbox_test.go:105::TestDomainEventOutboxHandler_ReplayIsIdempotent`、`internal/app/webhooks/service_test.go:541::TestService_ReplayWebhookEvent`、`internal/app/webhooks/delivery_outbox_test.go:85::TestDeliveryOutbox_ReplayHeadersEventID_MapsToLedger`。

### AC-WEBHOOK-002 签名头与防重放（[outbox](design/infrastructure/outbox.md)）

**Given** 订阅方收到 Webhook 投递
**When** 校验签名头
**Then** 签名头为 `GuliangIAM-Signature: t=<unix_sec>,v1=<hex_hmac_sha256(t + '.' + body)>`（密钥为解密后的 `webhooks.secret`，INFRA-003）；接收侧校验 `|now - t| ≤ 300s`（**±5min 防重放窗**）且 `(t, v1)` 首次出现，窗外或重复 `(t,v1)` 一律拒绝。

**验证锚点**：签名头契约 `internal/infra/webhook/http_sender_test.go:118::TestHTTPDeliverySender_SignatureContract`；头构造 `internal/app/webhooks/delivery_outbox_test.go:66::TestDeliveryOutbox_Success_MarksLedgerDelivered`、`:269::TestDeliveryOutbox_CustomBodyAndHeaders`；`[证据不足：接收侧 ±300s 窗口与 (t,v1) 首次出现校验在订阅方，平台侧仅生成签名，需对端夹具确认]`。

### AC-OUTBOX-001 事务性 Outbox 与 Worker 可靠投递（ADR-008，[outbox](design/infrastructure/outbox.md)）

**Given** 任一需要异步外发的业务写操作（Webhook/邮件/短信/SCIM 出站/计量等）
**When** 业务事务提交
**Then** 事件与业务事实在**同一事务**写入 `outbox`；Worker 以至少一次语义投递，失败按指数退避重试（`2^n 秒、上限 300s、±20% 抖动`），超过 `max_attempts`（默认 12）进入死信并告警；状态迁移与租约回收语义符合 AC-EVENT-001 状态表。

**验证锚点**：`internal/platform/outbox/outbox_test.go:46::TestTryTransition_Legal`、`:81::TestTryTransition_Illegal`（状态机合法/非法迁移）、`:118::TestBackoffDuration`（指数退避上限）；`internal/platform/outbox/worker_test.go:140::TestWorker_Success`、`:212::TestWorker_MaxRetries_GoDead`（超过 max_attempts 死信）；配置口径 `contracts/registries/outbox.yaml`。

### AC-OUTBOX-002 请求路径无同步外部调用（ADR-008）

**Given** 任一 API 请求处理路径（认证/管理/协议面）
**When** 请求触发邮件、短信、Webhook、SCIM 出站等外部投递
**Then** 投递一律经 Outbox/Worker 异步执行，请求路径内**无同步外部调用**；外部依赖故障不阻塞请求响应（计量类旁路降级同口径，METER-001）。

**验证锚点**：设计 `docs/design/infrastructure/outbox.md`（请求路径仅写 outbox，投递异步）；`internal/platform/outbox/worker_test.go:140::TestWorker_Success`（投递由 Worker 执行、失败不回灌请求）；`[证据不足：当前 L4，请求路径无同步外部调用需静态扫描/L6 证据]`。

### AC-OUTBOX-003 消费方幂等（ADR-008）

**Given** 同一事件被重复投递（Worker 重试/人工重放/崩溃回收）
**When** 消费方处理
**Then** 以 `event_id` 与 `(aggregate_type, aggregate_id, event_type)` 去重作用域保证重复投递**不产生重复副作用**（不重复发送、不重复入账）；去重记录 TTL 7d（AC-EVENT-002 同源）。

**验证锚点**：`internal/platform/outbox/worker_test.go:177::TestWorker_NoHandler_MarkDead`、`:250::TestWorker_PermanentFailure_MarkDeadImmediately`、`:288::TestWorker_MultipleWorkers_NoDuplicatePickup`（多 Worker 不重复领取=消费幂等前提）；去重/TTL 口径 `contracts/registries/outbox.yaml` + `contracts/schema/00_platform.yaml`。

### AC-DATA-006 平台内核表齐备

**Given** 全新建设的数据库（不依赖任何历史数据）
**When** 在空库上重放 `db/postgresql/V1__guliangiam_schema_init.sql`
**Then** `guliangiam` schema 中存在三张平台内核表且约束齐备：`outbox`（含 `ck_outbox_status`、`ck_outbox_attempts` 与 `idx_outbox_pending_available_at` 部分索引）、`idempotency_keys`（含 `uq_idempotency_keys_scope_key`、`ck_idempotency_keys_status`）、`encryption_keys`（含 `uq_encryption_keys_key_id_version`、`ck_encryption_keys_algorithm`、`ck_encryption_keys_status`、`ck_encryption_keys_key_version`）；且 `encryption_keys` 中不存在任何密钥材料列。

**验证锚点**：DDL 生成物 `db/postgresql/V1__guliangiam_schema_init.sql`（三张内核表约束齐备）；`bash scripts/check-target-schema.sh`。

### AC-DATA-007 版本化迁移通道（ADR-022）

**Given** 声明式 Schema 为唯一结构源（ADR-022）
**When** 数据库结构变更提交并由生成器处理
**Then** 生成器从声明式 Schema 产出版本化迁移与对应回滚脚本；迁移按版本号顺序执行且幂等可重放；回滚脚本可将库回退至上一版本；生产环境禁用自动迁移（`AUTO_MIGRATE=false`），迁移独立执行、先备份、可观测；任何代码路径不创建/读取手写迁移账本表；文档中不出现「双写」「回填」「expand → migrate → contract」等迁移中间态表述。

**验证锚点**：`scripts/gen_schema.py --check`（声明→生成零差异）、`scripts/check-migrations.sh`、`scripts/check-schema-evolution-audit.py`。

### AC-DATA-008 变更可收敛性

**Given** 需要修改已存在的表结构
**When** 变更类型为「新增列 / 新增表 / 新增索引 / 放宽约束」
**Then** 改写声明式 Schema 并由生成器产出前向迁移，重放两次即可收敛（第二次退出 0）；
**When** 变更类型为「删除列 / 改类型 / 重命名 / 收紧约束」
**Then** 必须先重放两次 `V1` 的**旧版本**并备份，再按开发规范手册第 19.4 节的显式上线流程执行，禁止直接改写后原地重放。

**验证锚点**：`python3 scripts/check-schema-parity.py`（声明/迁移/DDL 一致）、`python3 scripts/gen_schema.py --check`；流程见 `docs/GuliangIAM-开发规范.md` 第 19.4 节。

### AC-MIGRATE-001 迁移全链路 CI 验证（ADR-022）

**Given** CI 流水线与声明式 Schema 唯一源
**When** 提交包含 Schema 变更的改动
**Then** CI 自动验证「空库全量建表 → 按版本顺序升级 → 回滚至上版本 → 再升级」全通过，且终态与声明式 Schema 一致；生成器产出的 DDL、版本化迁移、回滚、OpenAPI 与文档表格同工作区文件零差异；PostgreSQL 18 能力探测通过（RLS、部分唯一索引、`SKIP LOCKED`、JSON、数组能力缺失时拒绝启动，禁止静默降级安全能力）；任一步失败则门禁返回非零。

**验证锚点**：`scripts/check-migrations.sh`（空库建表/升级/回滚/再升级 + 生成物零差异）、`scripts/dev/validate_schema_replay.sh`（五次重放 + 8 项断言）、`scripts/check-schema-parity.py`、`scripts/gen_schema.py --check`；PostgreSQL 版本前置 `scripts/prod-init-db.sh:113`（`server_version_num`）；`[证据不足：当前 L4，「RLS/部分唯一索引/SKIP LOCKED/JSON/数组能力缺失时拒绝启动」未发现 Go 侧启动探测，需确认]` → LEFT-R24-01。

### AC-MIGRATE-002 迁移后约束强制（ADR-022）

**Given** 版本化迁移在目标库执行完成
**When** 校验库内约束
**Then** 所有 `ON CONFLICT` UPSERT 的仲裁目标在库内存在列集合一致的唯一约束/索引；软删除下的业务唯一性由**数据库**强制（部分唯一索引），不依赖应用层检查；非法状态值写入被 CHECK 约束拒绝。

**验证锚点**：`scripts/dev/validate_schema_replay.sh`（UPSERT 仲裁目标可解析断言 + 部分唯一索引断言）、`scripts/check-db-conventions.sh`（`ck_`/部分唯一索引/命名规范）、`scripts/check-enums.py`（状态闭集与 CHECK 一致）。

### AC-RELEASE-001 回滚

**Given** 发布后出现 P0 错误率或登录故障
**When** 执行预案回滚
**Then** 应用恢复健康，错误率回落，迁移兼容性和审计记录可验证；回滚属应用层故障，RTO/RPO 按三档口径的 **L1 档：RTO < 1 分钟、RPO = 0**（数值唯一源 `contracts/registries/dr.yaml`，冲突以注册表为准；三档全口径见 AC-DR-001 与 [non-functional §6.2](requirements/non-functional.md)）。

**验证锚点**：`docs/operations/runbook.md`（回滚预案与操作步骤）、`scripts/release/check_release_readiness.py`（发布前检查）、`scripts/dev/check-rollback-independence.sh`（回滚独立性）；`[证据不足：当前 L4，RTO/RPO 实测需 L6 演练]`。

### AC-RELEASE-002 优雅停机（[runbook.md §1](operations/runbook.md#1-三探针分离与优雅停机)）

**Given** 实例收到 SIGTERM（滚动更新/缩容）
**When** 执行停机流程
**Then** 先摘流（readiness 探针检查依赖健康，失败即摘流不重启）后排空在途请求，超时上限后强制退出；停机窗口内新请求不被接受为 5xx；Outbox/Webhook Worker 将未完成批次释放回 `pending`（重置 `locked_by`/`locked_at`，[outbox](design/infrastructure/outbox.md) Outbox Worker 条款），租约回收 ≤60s 兜底，不丢事件。

**验证锚点**：`internal/bootstrap/run.go:312`（SIGINT/SIGTERM 信号处理与有序停机）、`internal/platform/workpool/pool_test.go`（在途任务排空）；Worker 释放批次/租约兜底见 outbox 设计 `docs/design/infrastructure/outbox.md`；`[证据不足：当前 L4，摘流/排空超时/Worker 释放需 L6 SIGTERM 夹具]`。

### AC-DATA-015 仓储 SQL 与 DDL 一致

**Given** 以声明式 Schema 为唯一结构源（ADR-022），列解析以当前 DDL 产物 `db/postgresql/V1__guliangiam_schema_init.sql` 为输入，且仓储把 SQL 写在 Go 字符串字面量里
**When** 执行 `python3 scripts/check-sql-columns.py` 与 `SQL_CHECK_DSN=… REQUIRE_SQL_RUNTIME_CHECK=1 python3 scripts/check-sql-runtime.py`
**Then** 下列四条全部成立：

| # | 断言 | 验证方式 |
|---|---|---|
| 1 | 每条 SQL 字面量引用的表与列都存在于 DDL（含 INSERT 列、UPDATE SET 目标、`alias.column`、占位符 cast 与目标列类型） | `check-sql-columns.py` 解析 DDL 后逐条解析字面量 |
| 2 | 每条完整语句都能在**按 DDL 重建**的 schema 上解析（能发现类型不匹配、参数缺口、多表同名列歧义、指向不存在的表） | `check-sql-runtime.py`：`DROP SCHEMA` → 重放 V1 → `PREPARE` 全部语句 |
| 3 | 列表端点的「取哪些行」与「共多少行」过滤器一致，`page.total` 与页内容不矛盾 | `check-sql-columns.py` 的 list/COUNT 一致性检查 |
| 4 | 破坏性重置不会误伤非 scratch 数据库 | DSN 取自 `SQL_CHECK_DSN` 时自动重置；取自 `POSTGRES_DSN`/`DATABASE_URL` 时必须 `ALLOW_SCHEMA_RESET=1`，否则拒绝执行 |
| 5 | 声明式 Schema 强制数据库约束完整性：租户表 `organization_id NOT NULL` + FK + RLS FORCE；`status` 列有 CHECK 且取值 ⊆ `contracts/registries/state-machines.yaml`；所有 FK 显式声明 `ON DELETE` 策略；无被唯一约束前缀覆盖的冗余索引 | `bash scripts/check-db-conventions.sh` |

**判定规则**：四条同时成立方可通过。第 2 条的必要性在于它是**唯一**能发现上述四类缺陷的手段——静态列检查看不到类型与参数序号问题，而集成测试带 `//go:build integration` 标签、CI 默认不执行。

> **重要**：本项必须对**按 DDL 重建**的 schema 校验，不得对既有库直接校验——DDL 使用 `CREATE TABLE IF NOT EXISTS`，在既有库上重放不会补列也不会删列，会产生假通过（DDL 删了列而库仍有）与假失败（DDL 加了列而库没有）。这与 ADR-022「整套 DDL 重放」的语义一致。

## 5. 会话、密钥、事件与限流

> 本节条件对应 [design/README.md](design/README.md) 索引中的会话、密钥、事件与限流设计，以及 ADR-013 至 ADR-020。**验证锚点**：`python3 scripts/check-sql-columns.py`（DDL 列解析↔SQL 字面量）、`SQL_CHECK_DSN=… REQUIRE_SQL_RUNTIME_CHECK=1 python3 scripts/check-sql-runtime.py`（真实库 PREPARE 全量回放）。

### AC-SESSION-001 会话状态机

**Given** 会话定义了 idle 与 absolute 双超时
**When** 会话超过 idle 超时或 absolute 超时
**Then** 状态分别进入 `idle`/`expired` 并拒绝使用；状态转换可审计，不允许非法跃迁。

**验证锚点**：`internal/domain/sessions/state_machine_test.go:54::TestTryTransition_Legal`、`internal/domain/sessions/state_machine_test.go:88::TestTryTransition_Illegal`（active/idle/expired/revoked 合法与非法迁移）；`internal/app/sessionpolicy/policy_test.go:11::TestIsIdleExpired`（idle 超时边界）。

### AC-SESSION-002 会话固定防护

**Given** 用户完成认证
**When** 登录成功
**Then** 会话标识重新生成，旧会话被撤销；会话 Token 仅以哈希入库，明文只存在于 HttpOnly Cookie。

**验证锚点**：`internal/app/sessions/rotate_test.go:14::TestService_RotateCurrentSession`（标识轮换/旧会话撤销）；会话实体 `internal/domain/sessions/session_test.go:12::TestSession_Creation`；登录后会话策略 `internal/app/auth/post_login_test.go:55::TestApplyLoginSessionPolicies_ExclusiveSigninRevokesSiblings`。

### AC-SESSION-003 会话劫持检测

**Given** 会话绑定了设备指纹或风险 IP
**When** 指纹/IP 在会话有效期内发生高风险变化
**Then** 撤销会话并记录 `critical` 安全事件，返回 401 `unauthorized`（C.1），不泄露预期指纹。

**验证锚点**：证据边界（DEF-25）：未发现「指纹/IP 高风险变化 → 撤销会话 + critical 事件」的实现与测试；`sessions.device_info`/`ip` 仅为登录快照（`contracts/schema/40_session.yaml`）；风险信号 `internal/app/risk/service.go:739`（device_fingerprint_new）未接线会话撤销。

### AC-SESSION-004 注销传播

**Given** 应用注册了 back-channel logout URI
**When** 用户登出或会话被撤销
**Then** 向各 RP 投递含 `sid` 的 Logout Token，并由 Outbox 保证不丢失；投递失败按 `contracts/registries/outbox.yaml` 的 `backoff_strategy` 退避重试，超过最大尝试次数进死信并告警，不静默丢弃。

**验证锚点**：`internal/app/auth/backchannel_logout_test.go:46::TestSignLogoutTokenWithJWKS_Success`、`:104::TestLogoutTokenSigningAlgorithm_RejectsOutOfEnum`、`:169::TestSendBackchannelLogout_ClientIDRequired`；`sid` 一致性 `internal/app/auth/session_sid_conformance_test.go`；退避/死信策略 `contracts/registries/outbox.yaml`（`backoff_strategy`）。

### AC-TOKEN-003 令牌绑定

**Given** 令牌绑定了 DPoP 密钥或 mTLS 证书
**When** 资源请求或内省携带不匹配的绑定凭证
**Then** 视为无效令牌并拒绝，不因绑定缺失而降级为 Bearer 通过。

**验证锚点**：`internal/app/auth/dpop_test.go`、`internal/app/auth/dpop_conformance_test.go`（DPoP 绑定/缺失证明拒绝且不降级）；mTLS 绑定 `internal/platform/mtls/fingerprint_test.go:59::TestBindTokenToClientCertificate`、`:70::TestClientCertificateThumbprintFromMetadata_Unbound`。

### AC-TOKEN-004 内省缓存一致性

**Given** 令牌已被撤销
**When** 资源服务器调用内省且缓存中仍有旧结果
**Then** 返回 `active=false`；缓存故障时回退数据库查询，禁止因缓存缺失放行已撤销令牌。

**验证锚点**：`internal/app/auth/token_introspection.go`（内省直读令牌状态、无缓存层）、`internal/app/auth/token_boundary_test.go:16::TestTokenIntrospection_Boundary`；观察：当前无内省缓存，撤销后 `active=false` 由直读满足（需 L6 撤销→内省夹具）。

### AC-KEY-001 密钥版本与信封

**Given** 密文信封携带 `key_id` 与 `key_version`
**When** 解密
**Then** 依信封版本选择密钥；版本不在允许集合时返回结构化错误并告警，不以逐版本试错作为识别方式。

**验证锚点**：`internal/platform/crypto/field_encryption_test.go:114::TestFieldEncryptor_KeyRotation_DecryptWithPreviousKey`（旧 key 密文在新进程经 previous key 解密、新写入用 current key）、`internal/platform/crypto/field_encryption_test.go:167::TestFieldEncryptor_KeyRotation_CurrentKeyTakesPriority`（current key 优先，legacy key 单独无法解密）。

### AC-KEY-002 密钥轮换

**Given** 执行密钥轮换
**When** 新版本激活并批量重加密
**Then** 重加密幂等且可断点续跑；回退窗口内新旧版本均可解密，旧版本密文清零后方可 `retired`。

**验证锚点**：`internal/platform/crypto/key_rotation_test.go:50::TestTryKeyTransition_Legal`、`:90::TestTryKeyTransition_Illegal`、`:123::TestAllKeyStates_CoversTransitions`（状态机含 `retired` 前置门禁）；重加密幂等 `internal/platform/crypto/aad_test.go:173::TestRotateColumn_V1ToV2Idempotent`；回退窗口新旧可解 `internal/platform/crypto/envelope_switch_test.go:12::TestEnvelopeEncryptor_AlgorithmSwitchOverlap`、`:65::TestFieldEncryptor_AlgorithmSwitchViaEnv`。

### AC-KEY-003 明文降级拒绝

**Given** 敏感字段启用加密（`syncers`/`adapters`/`agents`/`organizations` 等敏感字段启用 AES-256-GCM 信封加密）
**When** 未配置有效密钥（**所有环境**，含非生产）
**Then** **拒绝启动**（`internal/platform/crypto/field_encryption.go` 在 `ENCRYPTION_KEY_CURRENT`/`ENCRYPTION_KEY` 均缺失时 panic；`platform/config/validate.go` 在单一入口给出聚合错误），不存在「禁用加密后明文写入」的路径：`EncryptedFieldMapper.IsEnabled()` 恒为 `true`。验收测试：`TestFieldEncryptor_MissingKeyRefusesStartup`、`TestValidateOrPanic_MissingEncryptionKey`。测试基线自身使用专用固定密钥（`internal/platform/crypto/cryptotest`），不关闭加密路径。

**验证锚点**：`internal/platform/crypto/field_encryption_test.go:99::TestFieldEncryptor_MissingKeyRefusesStartup`、`internal/platform/config/validate_test.go:37::TestValidateOrPanic_MissingEncryptionKey`（缺钥拒绝启动）；无明文降级 `internal/platform/crypto/envelope_strict_test.go:89::TestDecryptEnvelopeField_RejectsPlaintext`、`:121::TestDecryptEnvelopeField_RejectsLegacyToken`、`:133::TestDecryptEnvelopeField_WrongKeyFailsClosed`。

### AC-CRYPTO-001 国密能力缺失即拒绝启动（PRIV-009/DR-004，ADR-031）

**Given** 部署配置 `CRYPTO_PROFILE=gm`
**When** `contracts/registries/crypto.yaml` 中 `failure_mode=refuse` 的能力位任一缺失
**Then** `sm2_sign`/`sm2_encrypt`/`sm3`/`sm4` 四个 `failure_mode=refuse` 能力位任一缺失时进程拒绝启动（fail-close），不得静默降级为国际算法；`gm_tls` 缺失不阻断启动，`/healthz` 返回 200 且 `status=degraded`，以 `degraded` 能力位与 `degraded_reasons` 披露原因，不静默降级。能力矩阵与降级边界以 ADR-031 与 [security-model](architecture/security-model.md) 为准；`/healthz` 的 `status=unavailable` → 503 分支是注入缺失能力的单测语义（仅在进程已发布报告后出现），正常启动路径在必需能力缺失时已于监听前终止，不产生运行期 503。

**验证锚点**：`internal/platform/crypto/suite_test.go::TestDetectCapabilities`、`TestAssessProfile`、`TestValidateRuntimeProfileHonorsEnv`；`internal/platform/crypto/gm_tls_test.go::TestValidateRuntimeProfileGMTLSFailureOnlyDegrades`；能力位 ID 与 `failure_mode` 以 `contracts/registries/crypto.yaml` 为准，一致性门禁 `python3 scripts/check-registry-sync.py --strict`。

- 能力裁决与启动拒绝：`internal/platform/crypto/suite.go::ValidateRuntimeProfile`（探测器经 `detectCapabilities` 注入）、`AssessProfile`、`SetRuntimeReport`、`HealthReport`；`internal/bootstrap/run.go::applyCryptoProfile`（启动裁决失败即返回错误，先于 `NewHTTPServer`/监听；仅成功才发布 `/healthz` 状态）。用例：`internal/platform/crypto/profile_failclose_test.go::TestAssessProfileGMRefusesEachMissingRequiredCapability`（逐位覆盖 `sm2_sign`、`sm2_encrypt`、`sm3`、`sm4`）、`TestAssessProfileGMAllRequiredCapabilitiesAvailable`、`TestAssessProfileGMGMTLSMissingOnlyDegrades`、`TestAssessProfileDefaultNeverRefuses`、`TestAssessProfileRejectsUnsupportedProfileWithoutFallback`、`TestValidateRuntimeProfileRefusesStartupOnMissingGMPCapability`、`TestValidateRuntimeProfileDefaultToleratesMissingCapabilities`。
- `/healthz` 不可用标记：`internal/bootstrap/chi.go::healthzHandler`（`status=unavailable` 时返回 503）。用例：`internal/bootstrap/healthz_test.go::TestHealthzUnavailableWhenGMPRequiredCapabilityMissing`、`TestHealthzOKForHealthyGMProfile`、`TestHealthzDegradedGMTLSStaysAvailable`、`TestHealthzOKForDefaultProfile`、`TestApplyCryptoProfileSkipsPublishOnFailure`、`TestApplyCryptoProfilePublishesReportOnSuccess`。
- 复现：`go test ./internal/platform/crypto -run 'TestAssessProfile|TestValidateRuntimeProfile' -count=1`、`go test ./internal/bootstrap -run 'TestHealthz|TestApplyCryptoProfile' -count=1`。

### AC-CRYPTO-002 ID Token 的 SM2-SM3 签名与 JWKS 多算法发布（ADR-031）

**Given** 应用 OAuth 配置 `app_oauth_config.token_signing_method=SM2-SM3`
**When** 签发 ID Token 并发布 JWKS
**Then** ID Token 以 SM2-SM3 签名；JWKS 按算法族分端点发布——标准端点 `/.well-known/jwks.json`（及按应用变体）只发布国际算法族（RSA/ES）公钥，国密 SM2-SM3 公钥由独立端点 `/.well-known/jwks-gm.json` 发布，Discovery 以扩展字段 `gm_jwks_uri`/`gm_alg_values_supported` 宣告国密通道；`kid` 前缀区分算法族，下游按前缀在对应通道选择验签密钥（国际 RP 取 `jwks_uri`，国密 RP 取 `gm_jwks_uri`）；标准端点恒不含 `crv=SM2` 密钥；同一应用不得混用两种算法，跨算法 `kid` 必须拒绝。

**IN（本条覆盖）**：

- ID Token 签名：`internal/app/auth/handle_logged_in.go::buildIDToken` → `internal/infra/jwt/issuer.go::IssueIDToken`，算法取自 `app_oauth_config.token_signing_method` 的封闭枚举 `RS256` / `SM2-SM3`。
- JWKS 多算法分端点发布：标准端点 `/.well-known/jwks.json` 与按应用端点只发布国际算法族（RSA/ES）公钥，国密 SM2-SM3 公钥由独立端点 `/.well-known/jwks-gm.json` 发布；下游按令牌 header 的 `kid` 前缀在对应通道选择公钥，未知或跨族 `kid` 一律拒绝；标准端点不得出现 `crv=SM2` 密钥。

**OUT（不属本条）**：

- Back-Channel Logout `logout+jwt` 复用同一算法（`internal/app/auth/backchannel_logout.go`），但不纳入本条覆盖断言。
- SSF SET（`internal/platform/ssf/set.go`）与 MCP access token（`internal/app/mcpservers/access_token.go`）是系统级 JWT 载体，固定 `alg=RS256`、`kid=rsa_`，不参与 `token_signing_method`；将来支持 SM2 需独立决策。
- access_token 不覆盖：生产签发路径（授权码、client_credentials、device_code、token-exchange、jwt-bearer）均经 `internal/app/auth/token_exchange.go::newAccessTokenSecret` 生成 `gl_pat_` 开头的 opaque token，数据库仅存 `access_tokens.token_hash`；`internal/infra/jwt/issuer.go::IssueAccessToken` 无生产调用点，不得据此宣称 access_token 已覆盖 SM2-SM3。
- 字段边界：`token_signing_method` 是 `app_oauth_config` 的字段（`contracts/schema/20_identity.yaml`），不是 `applications` 新增列；声明层不存在 `applications.token_signing_algorithm`。

**验证锚点**：`internal/app/auth/handle_logged_in_test.go::TestBuildIDToken_SigningAlgorithmFollowsApplication`、`TestBuildIDToken_RejectsUnknownSigningAlgorithm`；`internal/platform/jwks/manager_test.go::TestManager_GetJWKS`、`internal/platform/jwks/gm_test.go::TestManager_CrossAlgorithmKidLookupFails`；`internal/infra/jwt/issuer_sm2_test.go::TestIssuer_IssueIDToken_SM2SM3`、`TestIssuer_CrossAlgorithmKidRejected`；`internal/transport/http/api/v1/wellknown/jwks_rp_simulation_test.go::TestPublishedJWKSContainsRSAAndSM2Keys`、`TestRelyingPartyRejectsCrossAlgorithmKey`、`TestRelyingPartyRejectsUnknownKidWithoutFallback`。

### AC-CRYPTO-003 审计链 SM3 篡改定位与算法一致性（RISK-013，ADR-031）

**Given** 审计链 `chain_algo=sm3`
**When** 单点篡改任一链段记录后运行 `cmd/audit-verify` 或 `VerifyChainRecords`
**Then** 摘要按链段 `chain_algo` 选择（`sha256` / `sm3`）；校验定位到具体 `chain_seq`（断点及其后续记录报验失败）；同一条链混用 `sha256`/`sm3` 必须被拒绝，不得静默切换算法。

**验证锚点**：`internal/platform/outbox/providers/audit/chain_verify_test.go::TestVerifyChainRecords_SM3TamperLocatesSeq`、`TestVerifyChainRecords_RejectsMixedAlgorithms`；`internal/platform/crypto/chain_algo_test.go::TestHashByChainAlgorithm`；`cmd/audit-verify/main.go::verifyChain`、`loadChainRecords`（查询+校验可注入，CLI 参数/输出/退出码不变）；`cmd/audit-verify/integration_test.go::TestVerifyChainIntegration`（真实 PG：3 条 SM3 链、`chain_seq=2` 篡改定位、`chain_seq=3` 计数、SHA-256 控制组、自清理）；真实 PG 复现：`AUDIT_VERIFY_TEST_DSN=postgres://.../guliangiam_audit_crypto003_20261103 go test ./cmd/audit-verify/... -run TestVerifyChainIntegration -count=1`、`go run ./cmd/audit-verify -chain <organization-id> -dsn "$DSN"`。

**真实 PG 用例锚点**：`internal/infra/postgres/records/chain_sm3_pg_test.go::TestAuditChainSM3Postgres`（`GIAM_TEST_PG_DSN` 门控；DSN 须指向已迁移到当前基线的可丢弃库，且角色可绕过 FORCE RLS——迁移角色、表所有者或 Superuser；测试自建组织与链，只清理自身行）：单链 4 条 SM3 记录全链通过；payload 单点篡改与 `entry_hash` 单点篡改均按 `chain_seq` 与记录 id 定位到断点；同链混用 `sha256` 在切换记录处拒绝；`chain_algo='gost'` 同时被 `ck_records_chain_algo` CHECK（SQLSTATE `23514`）与平台算法分派拒绝，且无 `sha256` 回落。复现：`GIAM_TEST_PG_DSN='postgres://.../guliangiam_sm3_verify?sslmode=disable' GOCACHE=/tmp/giam-r2-cache go test ./internal/infra/postgres/records -run TestAuditChainSM3Postgres -count=1 -v`。

### AC-EVENT-001 领域事件

**Given** 关键写操作产生领域事件
**When** 写入业务事实
**Then** 事件与事实在同一事务写入 outbox，具备事件 ID、类型、聚合 ID、组织 ID、发生时间和 schema 版本。

**状态迁移与租约（ADR-017/ADR-008）**：判定需额外满足以下四条，供应商（Worker）与投递解耦：

| 阶段 | 允许的 `status` 迁移 | 关键不变量 |
|---|---|---|
| 取件 | `pending` → `processing` | 同一事务内 `UPDATE ... SET status='processing', locked_by=$worker, locked_at=now(), attempts=attempts+1 WHERE status='pending' AND available_at <= now()` |
| 成功 | `processing` → `succeeded` | 同步写入 `processed_at`；同 `event_type`+`aggregate_id` 的消费者完成记录幂等可判 |
| 可重试失败 | `processing` → `pending` | 必须同时写入 `available_at = now() + 退避`，清空 `locked_by`/`locked_at` |
| 超限失败 | `processing` → `failed`/`dead` | `attempts >= max_attempts` 时不得再回 `pending`，进死信待人工 |

**崩溃恢复语义**：Worker 在 `processing` 状态下崩溃时，事件由**租约超时**回收到 `pending`（回收条件：`status='processing' AND locked_at < now() - lease_timeout`），不得因进程崩溃导致事件永久卡死；回收必须交出 `locked_by` 以避免双投递。`Status` 与 `locked_by`/`locked_at` 的组合由 `ck_outbox_status` 与部分索引 `idx_outbox_pending_available_at` 兼顾。

**验证锚点**：`internal/platform/outbox/outbox_test.go:10::TestIsValidStatus`、`:23::TestIsTerminalStatus`、`:46::TestTryTransition_Legal`、`:93::TestAllStatuses_CoversTransitions`（状态闭集与迁移全覆盖）；事件字段/约束声明 `contracts/schema/00_platform.yaml`（outbox 表）。

### AC-IDEM-001 统一幂等

**Given** 请求携带 `Idempotency-Key`
**When** 相同键相同请求体重复提交
**Then** 返回首次响应并标记 `Idempotency-Replayed: true`；相同键不同请求体返回冲突，不重复产生副作用。

**验证锚点**：`internal/platform/middleware/idempotency.go`（重放缓存与 `Idempotency-Replayed` 头实现）、`internal/platform/middleware/idempotency_test.go:69::TestIdempotencyMiddleware_KeyReuseConflictCanonicalEnvelope`；`[证据不足：当前 L4，成功重放路径缺专属测试]`。

### AC-IDEM-002 幂等键同键异体冲突（ADR-018）

**Given** 某写请求已以 `Idempotency-Key: K` 成功处理
**When** 携同一 `K` 但不同请求体（`request_hash` 不一致）的请求到达
**Then** 返回 `409 conflict`（同键异体），不进入业务处理，不覆盖首次响应缓存。

**验证锚点**：`internal/platform/middleware/idempotency_test.go:69::TestIdempotencyMiddleware_KeyReuseConflictCanonicalEnvelope`（同键异体 409 + 不覆盖缓存）、`:101::TestIdempotencyMiddleware_InProgressUnavailableCanonicalEnvelope`。

### AC-IDEM-003 幂等键并发请求恰好一次副作用（ADR-018）

**Given** K≥2 个携带同一 `Idempotency-Key` 的并发请求（请求体一致）
**When** 全部请求完成
**Then** 恰好一个请求执行业务写入，其余返回首次响应（`Idempotency-Replayed: true`）或等待后重放；副作用计数恰好为 1（`uq_idempotency_keys_scope_key` 唯一约束仲裁；与 `If-Match` 组合场景判定见 AC-API-003 (d)）。

**验证锚点**：唯一约束声明 `contracts/schema/00_platform.yaml`（`uq_idempotency_keys_scope_key`）；中间件仲裁 `internal/platform/middleware/idempotency_test.go:69`、`:101`；`[证据不足：当前 L4，K≥2 并发恰好一次副作用需 L6 并发夹具]` → LEFT-R22-01。

### AC-RATE-001 分布式限流（ADR-019）

**Given** 多实例共享限流计数（Redis/Lua，ADR-019）
**When** 跨实例超过配额
**Then** 计数一致并返回 429；共享存储不可用时退化为保守本地限流，不得无限放行。

**验证锚点**：`internal/platform/middleware/ratelimit_redis_test.go:25::TestRateLimit_Redis_ExhaustBurst`、`:78::TestRateLimit_Redis_MultiInstanceConsistency`、`:132::TestRateLimit_Redis_FailClose`（共享存储故障保守拒绝）；参数口径 `contracts/registries/limits.yaml`。

### AC-SLO-001 SLO 与错误预算

**Given** 定义了可用性、延迟、登录成功率和令牌签发成功率 SLI
**When** 查询监控
**Then** SLI 与错误预算燃尽率可计算、可告警——可用性按 generated/slo-registry.md（STD 11.1 展示）**分面**口径：**管理面 99.5%、认证面 99.9%、协议面 99.9%**（「月度」为展示层；判定与错误预算按 **28 天滚动窗口**，换算口径见该注册表），健康检查/SLI 按面分别计算（禁止笼统单一 99.9%）；延迟/登录成功率/令牌签发成功率/会话恢复成功率 SLI 引 [observability](architecture/observability.md) 表（API P99 < 500ms、登录成功率 99.9%（`slo.yaml authentication_business`）、令牌签发 99.9%）；不得用 SLO 数据替代安全事故响应。

**验证锚点**：SLO 注册表 `contracts/registries/slo.yaml`（分面可用性/28 天滚动窗口/登录与令牌成功率）；渲染层 `docs/generated/slo-registry.md`；注册表一致性 `scripts/check-registry-sync.py --strict`。

### AC-RATE-002 限流四维取最严与降级（[observability](architecture/observability.md)）

**Given** 多实例部署与共享限流存储（Redis）
**When** IP/用户/client/端点族四维中任一维度超限
**Then** 取最严维度返回 429 + `Retry-After` + IETF `RateLimit-Limit`/`RateLimit-Remaining`/`RateLimit-Reset` + `RateLimit-Policy`；Redis 不可用时认证面 **fail-close**（本地桶容量减半）、管理面只读 fail-open 并告警（26.4 降级表），均不得无限放行；多实例计数一致性按 AC-RATE-001 验收；限流参数只允许使用 STD 附录 D 已登记变量。

**验证锚点**：`internal/platform/middleware/ratelimit_redis_test.go:191::TestRateLimit_Redis_IETFHeaders`（IETF 头）、`:231::TestRateLimit_Redis_GlobalInjection`；四维取最严 `internal/platform/middleware/ratelimit_test.go:99::TestRLDimension_SubSecondRatesKeepAPositiveQuota`、`:138::TestRateLimit_EmitsIETFHeaders`；降级表 `docs/architecture/observability.md`。

### AC-RATELIMIT-001 多实例分布式限流一致（ADR-019，SEC-003）

**Given** ≥2 个服务实例共享限流计数（Redis/Lua）
**When** 同一来源跨实例合计超过认证面单组限额
**Then** 合计被拒（429），任一单实例视角均不超发——分布式计数下集群总通过量不超过单组限额；限流参数只允许使用 STD 附录 D 已登记变量。

**验证锚点**：`internal/platform/middleware/ratelimit_redis_test.go:78::TestRateLimit_Redis_MultiInstanceConsistency`（多实例合计不超发）、`:164::TestRateLimit_Redis_TTL`（窗口过期一致性）。

### AC-RATELIMIT-002 共享存储故障保守拒绝（ADR-019，[observability](architecture/observability.md)）

**Given** 共享限流存储（Redis）不可用
**When** 高风险操作（认证/令牌签发/密码重置等）到达
**Then** 按 [observability](architecture/observability.md) 降级表执行：认证面 fail-close（本地桶容量减半后仍拒绝超限请求），高风险操作在无法确认配额时**保守拒绝**，不得无限放行；管理面只读可 fail-open 并告警。

**验证锚点**：`internal/platform/middleware/ratelimit_redis_test.go:132::TestRateLimit_Redis_FailClose`（Redis 不可用 fail-close）、`:59::TestRateLimit_Redis_DeniedWritesUnifiedErrorBody`；降级矩阵 `docs/architecture/observability.md`。

### AC-EVENT-002 Outbox 事件幂等与崩溃恢复（[outbox](design/infrastructure/outbox.md)）

> **投递设施唯一性**：Webhook 投递统一由 `outbox` 承载（`aggregate_type='webhook'`），本条的幂等与租约回收验收同时覆盖 Webhook 投递；`outbox.max_attempts` 默认值以 `contracts/registries/outbox.yaml` 为唯一口径。

**Given** 同一 `(aggregate_type, aggregate_id, event_type)` 事件
**When** 人工重放或 Worker 重复领取
**Then** 按 `event_id` 去重作用域保证同聚合同类型事件仅一个在途实例，去重记录 TTL 7d；Worker 崩溃后事件按租约回收（≤60s）回到 `pending` 并交出 `locked_by`，不产生双投递副作用；与 AC-EVENT-001 状态迁移表联合验收。

**验证锚点**：`internal/platform/outbox/worker_test.go:288::TestWorker_MultipleWorkers_NoDuplicatePickup`（重复领取/双投递防护）；`[证据不足：当前 L4，租约超时回收（≤60s）需 L6 崩溃夹具]`。

### AC-SLO-002 API 延迟与压测刻度（[observability](architecture/observability.md) / PERF）

**Given** L2 压测环境（[observability.md §2](architecture/observability.md#2-性能基线l0-l3-分层) L2 定义）
**When** 稳态采样 ≥10 分钟
**Then** API P99 延迟 < **500ms**（[observability](architecture/observability.md)，排除长连接探针）；L1 数据库单表目标刻度：点查 <1ms、范围查 <10ms、写 <50ms（PERF L1 目标刻度，非 SLO，与 L2 分层不可互比）；压测数据集的分档 QPS 刻度（100/1,000/10,000）仅为压测口径，不得引用为生产 SLO；生产容量口径见 [non-functional §6.1](requirements/non-functional.md)。

**验证锚点**：分层口径 `docs/architecture/observability.md`（L0–L3、API P99<500ms、L1 单表刻度）；SLO 注册表 `contracts/registries/slo.yaml`；`[证据不足：当前 L4，L2 压测稳态 P99 需 L6 压测报告]`。

## 6. 前端功能、设计系统与导航

> 本节条件对应 [design/frontend/README.md](design/frontend/README.md) 与 [开发规范 §2.2](GuliangIAM-开发规范.md#22-样式管理) / [§2.11](GuliangIAM-开发规范.md#211-导航与侧边栏规范)。
> **编号同源**：`AC-FE-0nn` 与 [requirements/functional.md §10](requirements/functional.md#10-前端管理控制台-reqmodule-fe) 的 `FE-0nn` **一一对应、同号同义**；前端治理登记项（SRS 无数值编号的承载项，见 §8.3）使用独立号段 `AC-FEGOV-`，不得占用 `AC-FE-` 号段。

### AC-FE-001 管理页面覆盖所有后端域

**Given** 后端按模块声明的路由（各 `internal/transport/http/api/v1/*/routes.go`）与前端路由表 `web/src/app/router/router.tsx`
**When** 逐模块比对可管理资源
**Then** 每个具备管理语义的后端域在控制台至少有一个可达的管理页面，且该页面经菜单可达或在 `NON_NAV_ROUTES`/`NON_NAV_PREFIXES` 中登记理由；`scripts/check-frontend-coverage.py` 的全部规则通过；判定为「无归属」的资源必须登记为需求编号（如 FE-014~FE-017），不得静默缺页。

**验证锚点**：门禁 `scripts/check-frontend-coverage.py`（路由↔页面↔菜单↔《控制台设计》菜单表四者闭合）；路由事实源 `web/src/app/router/router.tsx`；菜单事实源 `web/src/features/navigation/menu.tsx`。

### AC-FE-002 暗色模式

**Given** 用户在个人设置中选择 `light` / `dark` / `system`
**When** 切换选择或操作系统配色偏好变化
**Then** 全部视觉常量经 `--gl-*` 令牌切换，页面内不出现随主题漏改的硬编码色值；`system` 档必须跟随 `prefers-color-scheme` 实时生效；偏好持久化并在首帧前应用（不出现白屏闪烁）；首帧前应用不得阻塞交互。

**验证锚点**：`web/src/features/theme/theme-store.test.ts`（light/dark/system 持久化与首帧应用）；设计令牌镜像 `web/src/app/theme/brand-palette.ts` + 门禁 `scripts/check-token-parity.py`。

### AC-FE-003 国际化

**Given** 语言包位于 `web/src/shared/i18n/locales/`，14 种语言（`de`/`en`/`es`/`fr`/`ja`/`ko`/`pl`/`pt`/`ru`/`tr`/`uk`/`vi`/`zh-CN`/`zh-TW`），并含独立 `auth` 命名空间
**When** 切换语言或新增用户可见文案
**Then** 全部语言包存在同名键，`auth` 命名空间独立不与其他命名空间串键；语言选择持久化并可由 URL/浏览器偏好推断；日期、数字、货币经 `Intl` 按当前语言格式化，不得使用硬编码格式；缺失键在开发模式显式暴露，不得静默显示空白。

**验证锚点**：门禁 `scripts/check-i18n-keys.py`（14 语言键全量一致 + 命名空间解析）；语言包 `web/src/shared/i18n/locales/en/common.json`；初始化 `web/src/shared/i18n/index.ts`。

### AC-FE-004 前端权限控制非安全边界

**Given** 用户无某权限点
**When** 前端渲染对应菜单/按钮与直连后端端点
**Then** `usePermissions`/`PermissionGuard` 对不可用入口**不渲染**（非禁用，与 AC-FEGOV-002 一致）；对应后端端点独立执行权限校验，越权返回 403 `forbidden`——前端隐藏不构成授权依据，任何仅依赖前端隐藏的端点均按 AC-PERM-003 违例处理。

**验证锚点**：`web/src/shared/ui/PermissionGuard.test.tsx`（无权限不渲染）；权限 key 登记门禁 `scripts/check-permission-keys.py`。

### AC-FE-005 会话空闲超时提醒

**Given** 生效会话策略（ORG-009：组织可在受控范围内覆盖 idle/absolute 超时，系统默认 idle 提醒为 30 分钟）
**When** 前端读取生效策略并跟踪用户活动
**Then** 前端**读取**生效策略而非固化默认值；临近 idle 超时前给出可取消的提醒倒计时；用户活动按策略重置 idle 计时；absolute 超时**不因活动而延长**；策略在会话中被修改时前端以服务端返回的生效值为准；超时后跳转登录并保留可安全恢复的返回目标。

**验证锚点**：`web/src/features/session/session-store.test.ts`（会话策略/活动重置）；生效策略读取 `web/src/features/session/useSessionBootstrap.ts`。

### AC-FE-006 401 全局拦截与并发去重

**Given** SPA 任一请求收到 401，且存在多个并发请求同时失效
**When** 全局拦截器处理
**Then** 并发去重后仅触发**一次**会话失效处理（登出或刷新），不重复弹窗/重复跳转、不产生多个并发刷新竞态；处理行为在全部 14 语言包均有用户可见文案（AC-FEGOV-004 联动）。

**验证锚点**：`web/src/shared/api/http.test.ts`（401 全局拦截与并发去重 single-flight）。

### AC-FE-007 主题编辑器

**Given** 主题编辑器页面与令牌白名单
**When** 用户修改令牌并保存或导入外部主题
**Then** 仅允许修改白名单内的 `--gl-*` 令牌，白名单外字段被拒绝；保存前提供实时预览与对比；非法值（越界数值、非法颜色、含 `url(`/`expression(` 等可注入构造）被校验拒绝，不得注入任意 CSS；导出的主题可被再次导入并复现同一结果；主题作用域限定于当前组织/用户，不污染系统默认值。

**验证锚点**：`web/src/pages/theme-editor/ThemeEditorPage.tsx`（白名单令牌编辑/预览/导入导出）；令牌镜像与门禁见 `web/src/app/theme/brand-palette.ts`、`scripts/check-token-parity.py`。

### AC-FE-008 个人中心

**Given** 已登录用户进入个人中心
**When** 执行资料修改、密码修改、MFA 管理、会话语查看与撤销
**Then** 用户仅能读取与修改自己的资源，越权访问他人资源返回 404；密码修改与 MFA 解绑属高风险操作，须重新认证后方可提交；会话/设备列表展示当前会话并支持撤销其他会话，撤销后对应会话立即失效；本人安全事件可查看，但不泄露他人信息。

**验证锚点**：`web/src/pages/account/AccountPage.test.tsx`、`web/src/pages/account/AccountPasswordSection.test.tsx`、`web/src/pages/account/AccountLoginsSection.test.tsx`（本人资源、改密与登录设备列表）。

### AC-FE-009 系统信息页面

**Given** 系统信息页面
**When** 具有相应权限的用户访问，无权用户直接请求同一数据面
**Then** 页面展示版本、构建标识/提交号、启用的能力开关、数据库与迁移版本等运行期事实，取值来自服务端而非前端硬编码；响应中不得包含密钥、令牌、连接串口令或其他敏感配置；无权用户被拒绝（403）且不返回部分数据。

**验证锚点**：`web/src/pages/system-info/SystemInfoPage.tsx`（取值来自服务端运行期事实；无前端硬编码）。注：该页面当前无专属组件测试，属证据边界的测试缺口。

### AC-FE-010 全部登录方式的前端组件

**Given** 系统支持的登录方式（密码、短信/邮箱 OTP、TOTP、WebAuthn 硬件密钥、Passkey、扫码（微信/钉钉等）、Face ID 活体、设备码、CAS/SAML/OIDC 联邦）
**When** 用户走完每条登录流程的异常分支
**Then** 每种方式均有对应前端组件且可从登录入口到达；每个组件处理过期、重复提交、限流与取消分支并给出可操作提示；一次性凭据与验证码在界面上自动遮蔽、不写入日志与前端持久化存储；联邦登录的回跳目标经白名单校验。

**验证锚点**：`web/src/pages/auth/LoginSigninForm.test.tsx`、`LoginQrcodeCard.test.tsx`、`Web3LoginPanel.test.tsx`、`SamlCallbackPage.test.tsx`（各登录方式异常分支与回跳处理）。

### AC-FE-011 文件上传

**Given** 头像与资源等上传入口
**When** 上传合法文件、超限文件与可执行/带脚本内容
**Then** 类型与大小在**前后端双侧**校验且服务端为权威判定（绕过前端不得成功）；拒绝带脚本的内容类型与可内联执行的 SVG 等载荷；上传过程提供进度与可取消，取消后不留残留对象；返回的对象引用为稳定标识，不返回本地路径或凭据。

**验证锚点**：`web/src/shared/ui/ImageCropUpload.tsx`（前端类型/大小校验与裁剪上传）；服务端权威校验见上传端点实现（`internal/transport/http/api/v1/users/` 上传用例）与 `[证据不足：当前 L4，需 L6 上传边界夹具]`。

### AC-FE-012 可折叠侧边栏导航

**Given** 控制台主框架的侧边栏
**When** 折叠/展开、刷新页面、键盘操作
**Then** 折叠状态持久化并在刷新后保留；折叠态下图标可通过键盘聚焦、悬停/聚焦显示完整标签；层级展开状态在导航后保持合理还原；折叠动画遵循 `prefers-reduced-motion`；折叠不导致菜单项不可达（不得依赖仅鼠标可用的交互）。

**验证锚点**：`web/src/app/layout/AppShell.tsx`（侧边栏折叠/键盘可达/持久化）；折叠持久化状态由 layout 组件与浏览器存储承载；`prefers-reduced-motion` 处理见全局样式。

### AC-FE-013 统一全局样式规范

**Given** 表单分区、空状态、页面过渡三类通用呈现
**When** 逐个页面核对
**Then** 表单按分区组织且分区标题/说明/错误位一致；列表与详情空状态使用统一空状态组件并给出下一步动作，不得出现空白页；页面过渡与加载使用统一组件，全部动画在 `prefers-reduced-motion: reduce` 下退化为无动画但不丢失信息。

**验证锚点**：`web/src/shared/ui/FormSection.test.tsx`（统一表单分区标题/说明/错误位）；设计约束见 `docs/design/frontend/interaction.md`。

### AC-FE-014 CAS 单点登出控制台入口

**Given** 控制台 CAS 单点登出入口与后端 `POST /api/v1/auth/cas/logout`
**When** 携带 `service` 参数发起登出并重复触发
**Then** 回跳目标仅在白名单内方可使用，白名单外的 `service` 被拒绝（不得发生开放重定向）；登出为幂等操作，重复进入不产生错误态或重复会话终止；无活动会话时仍返回成功语义；登出完成后的落地页为白名单内目标或 CAS 登录页。

**验证锚点**：`web/src/pages/auth/CasLogoutPage.tsx`、`CasLogoutPageCard.tsx`（CAS 登出入口与白名单回跳）；后端 `POST /api/v1/auth/cas/logout` 路由声明见 `internal/transport/http/api/v1/auth/routes.go`。

### AC-FE-015 会话 transcript 查看器

**Given** 会话 transcript 读取端点与查看器页面
**When** 有权用户打开一次会话的 transcript，无权用户或越权租户请求同一会话
**Then** 有权用户可加载消息序列并支持分页或虚拟滚动；服务端完成脱敏，响应中不含凭据、令牌明文或未脱敏的个人信息；越权请求返回 404（而非 403），不得通过状态码或响应体差异泄露会话是否存在。

**验证锚点**：`web/src/pages/entries/EntryTranscriptPageCard.test.tsx`、`useEntryTranscriptPageController.test.tsx`（transcript 加载/分页/越权 404 处理）。

### AC-FE-016 独立支付二维码页

**Given** 独立二维码页与非宿主浏览器场景
**When** 页面展示二维码并推进支付结果
**Then** 二维码与轮询均只读订单资源，订单状态完全由服务端状态机裁决，前端不据本地推断改变状态；轮询在页面不可见（`visibilitychange` 隐藏）时暂停，在终态（成功/失败/取消/过期）停止；页面刷新或重新进入后可由服务端状态恢复显示，不产生重复支付副作用。

**验证锚点**：`web/src/pages/orders/OrderPayPage.tsx`（独立支付页：二维码/轮询/终态停止由服务端状态机驱动）。

### AC-FE-017 密码重置的应用上下文

**Given** 带应用上下文的密码重置入口（路径段或查询参数）
**When** 分别提交存在的账户名与不存在的账户名
**Then** 应用上下文只影响品牌与展示策略，不参与账户定位；两类提交的用户可见响应一致（不区分账户是否存在），响应时序与状态码不构成枚举通道；应用上下文无效时回落到系统默认品牌而非报错泄露。

**验证锚点**：`web/src/pages/auth/ForgotPasswordPage.tsx`（应用上下文仅影响品牌/展示，账户定位与响应不区分存在性）；设计约束见 `docs/design/frontend/interaction.md`。

### AC-FEGOV-001 设计令牌单一来源

**Given** 前端视觉常量集中在 `web/src/styles.css` 的 `--gl-*` 变量
**When** 检索 `web/src/**/*.tsx` 与 `web/src/**/*.css`
**Then** 不存在行内颜色（`style={{ color: ... }}` 中的字面色值）与未登记的十六进制色值；新增的视觉常量在 `styles.css` 中已声明对应 `--gl-*` 令牌；`@media (prefers-reduced-motion: reduce)` 覆盖所有过渡与动画。

**验证锚点**：门禁 `scripts/check-token-parity.py`（令牌单一来源/硬编码色值扫描）；令牌镜像 `web/src/app/theme/brand-palette.ts`。

### AC-FEGOV-002 导航单一事实源

**Given** 菜单集中在 `web/src/features/navigation/menu.tsx` 的 `menuGroups`
**When** 新增一个页面
**Then** 该页面已在 `menuGroups` 注册（含 `key`/`label`/`icon`，分组 `routeKeys` 覆盖其路径），面包屑由 `getBreadcrumbs()` 派生而非页面自建；无权限时该菜单项**不渲染**（而非禁用）；组内全部子项不可见时分组自动隐藏。

**验证锚点**：菜单单一事实源 `web/src/features/navigation/menu.tsx`；渲染层生成 `scripts/gen_frontend_registry.py`（`docs/design/frontend/registry.md`）。

### AC-FEGOV-003 菜单一致性门禁

**Given** `menu.test.ts` 与 CI
**When** 提交包含菜单或路由变更
**Then** 三项校验全部通过：(a) 每个菜单 `key` 在路由表中存在；(b) 分组 `routeKeys` 覆盖该组全部子项路径；(c) 所有 `nav.*` i18n 键在 `web/src/shared/i18n/locales/` 的全部 14 个语言包中存在；任一失败即门禁失败。

**扩展范围**：门禁的键范围应从 `nav.*` 扩到**全部用户可见键**——遍历 `web/src/**/*.tsx` 中的 `t('...')` 字面量键，要求 14 个语言包均存在同名键；缺失键清单必须在失败输出中逐条列出（不得只报总数）。动态拼接键（模板字符串）必须短路为「不校验但计数」，并在输出中给出计数，避免因动态键导致门禁长期被跳过。

**验证锚点**：门禁 `scripts/check-registry-sync.py --strict`、`scripts/gen_frontend_registry.py`、`scripts/check-i18n-keys.py`（全量键遍历与动态键计数）。

### AC-FEGOV-004 国际化覆盖

**Given** 语言包位于 `web/src/shared/i18n/locales/`，14 种语言（`de`/`en`/`es`/`fr`/`ja`/`ko`/`pl`/`pt`/`ru`/`tr`/`uk`/`vi`/`zh-CN`/`zh-TW`）
**When** 新增用户可见文案
**Then** 全部语言包均存在同名键；缺失键在开发模式下必须显式暴露（回落到键名或默认语言），不得静默显示空白。

## 7. API 契约

> 本节条件对应 [openapi/openapi.yaml](../openapi/openapi.yaml) 的错误契约与《开发规范手册》附录 C。应用错误码定义源**唯一**为附录 C。

**验证锚点**：门禁 `scripts/check-i18n-keys.py`（14 语言键一致）；语言包 `web/src/shared/i18n/locales/en/common.json`；错误码契约 `docs/GuliangIAM-开发规范.md` 附录 C。

### AC-API-001 错误码命名空间分离

**Given** 应用错误码定义于 `internal/platform/errors/errors.go`，协议错误码属 RFC 6749/6750/9449
**When** 遍历 `internal/**/*.go`（排除 `_test.go`）
**Then** 业务端点只使用 `errors.Code` 常量，不出现手写的 `"code": "<字面量>"` 错误体；协议错误码（`invalid_client`/`invalid_grant`/`invalid_scope`/`invalid_redirect_uri`/`unsupported_grant_type`/`invalid_token`）只出现在协议端点；`bad_request`、`internal_error`、`method_not_allowed`、`service_unavailable` 等未登记名称不存在于源码。 <!-- naming-allow：本行仅声明该字面量已废弃，不得在代码或契约中使用 -->

**验证锚点**：门禁 `scripts/check-errors.py`（错误码命名空间与字面量扫描）、`scripts/check-error-response.sh`；错误码常量契约 `internal/platform/errors/errors_test.go`。

### AC-API-002 统一响应封装

**Given** 所有错误经统一错误中间件序列化
**When** 触发任一类错误
**Then** 响应体为 `{"error":{"code","message","details"},"requestId","traceId"}`（唯一目标形态），`requestId` 与 `X-Request-Id` 一致且可在日志/审计/Sentry 中检索；响应不含堆栈、SQL、内部路径或密钥值；列表响应固定为 `{"items":[...],"page":{"page","pageSize","total"}}`。

**验证锚点**：门禁 `scripts/check-error-response.sh`（统一错误/列表封装体）；`internal/transport/http/route/http_test.go`（`route.WriteError` 唯一出口）。

### AC-API-003 幂等与乐观锁

**Given** 写接口声明需要幂等，且可编辑资源返回 `ETag`
**When** (a) 以相同 `Idempotency-Key` 与相同请求体重放；(b) 以相同 key 但不同请求体重放；(c) 使用过期 `If-Match` 提交更新
**Then** (a) 返回首次响应并附 `Idempotency-Replayed: true`，且**不**重复写入 `records`；(b) 返回 `409 conflict`；(c) 返回 `409` 且错误码属于冲突语义并附当前 `revision`。三种情况下 `idempotency_keys` 与 `revision` 的状态均可核验。

**(d) 幂等键与 `If-Match` 同时出现（组合场景，必须明确定义）**

**Given** 同一请求同时携带 `Idempotency-Key` 与 `If-Match`
**Then** 判定顺序为**先幂等、后乐观锁**：

| 场景 | 判定结果 | 说明 |
|---|---|---|
| 键已存在且 `request_hash` 一致 | 直接重放缓存响应，**不**校验 `If-Match` | 重放不得因当前 `revision` 已变化而失败；否则客户端永远无法安全重试 |
| 键已存在、`request_hash` 不一致 | `409`（同键异体） | 不进入乐观锁判定 |
| 键不存在（首次请求）、`If-Match` 不匹配 | `409` 乐观锁冲突 | 键记录以 `failed` 写入，**不**缓存响应体，允许客户端以新 `If-Match` 重试同一键 |
| 键不存在、`If-Match` 匹配 | 成功；写业务事实 + 递增 `revision` + 回填 `idempotency_keys.response_*` | 幂等重放本身**不**递增 `revision`，也**不**新增 `records` 行 |
| `If-Match: *` | 视为无条件更新 | 仅用于明确声明“不在乎并发”的接口，默认禁止 |

**验证锚点**：`internal/platform/middleware/idempotency_test.go`（幂等先于乐观锁）；乐观锁冲突用例 `internal/app/subscriptions/service_test.go:294::TestPatchSubscription_RevisionMismatch`（`409 conflict`）。

### AC-API-004 契约与路由同源

**Given** `openapi/openapi.yaml` 由 `scripts/gen_openapi.py` 生成，`x-permission` 声明 Casbin 策略对象
**When** 运行 `RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh`
**Then** 生成结果与已提交的 `openapi/openapi.yaml` 零差异；每条受保护路由均声明 `x-permission`；高风控写接口（转让所有权、删除组织、批量删除、导出全量数据、退出模拟登录）声明返回 `audit_id`。

**验证锚点**：`python3 scripts/gen_openapi.py --check`（生成零差异）、`bash scripts/check-openapi-contract.sh`（路由↔OpenAPI 同源）、`python3 scripts/check-route-resource-derivation.py`（资源派生）。

### AC-API-005 API 三面可分离

**Given** 系统对外 HTTP 接口分为管理面 `/api/v1/*`（除认证/协议端点）、认证面 `/api/v1/auth/*` 页面与会话类端点、协议面（多个前缀家族：根级 `/oauth2/*`、`/.well-known/*`、`/cas/{organizationSlug}/{applicationSlug}/*`、`/scim/v2/*`、`/api/v1/saml/*`、`/api/v1/ai/faces/*`，以及 `/api/v1/auth/*` 内的协议端点；家族清单唯一事实源为 `contracts/registries/surfaces.yaml`），三面定义见 [开发规范 §4.1.3](GuliangIAM-开发规范.md#413-api-面划分管理面--认证面--协议面)、[architecture/api-surfaces.md](architecture/api-surfaces.md) 与 [system-overview.md §3](architecture/system-overview.md#3-物理部署视图)
**When** 对任一面分别执行路由枚举、限流验证、错误体验证与指标/审计采样
**Then** 下列四条断言全部成立：

| # | 断言 | 验证方式 |
|---|---|---|
| 1 | 路由定义带 `Surface` 标记，可由程序枚举出三面的完整端点集合，无需人工维护名单 | `route.Definition.Surface` + `route.SurfaceFor(pattern)`；门禁 `internal/bootstrap/surface_contract_test.go::TestSurfaceContract` 断言每条声明的 `Surface` 非空、合法且等于 `SurfaceFor(pattern)`，并断言三面计数均非零 |
| 2 | 三面各自绑定独立的限流组，且互相叠加而非替代：管理面 `RATE_LIMIT_GLOBAL_*`、认证面 `RATE_LIMIT_AUTH_*` + 防枚举、协议面 `RATE_LIMIT_TOKEN_*` | 分别以超过限额的请求量触发，断言返回 429 且触发的是该面的限流组；认证面额外断言对不存在账号与存在账号的响应状态码与响应体形态一致；防枚举由认证面中间件强制 |
| 3 | 协议面错误体符合 RFC：OAuth2 返回 `error`/`error_description`/`error_uri`；SCIM 返回 `schemas`/`detail`/`status`；不输出 `{error:{code,...}}` 信封 | 分别对 `POST /api/v1/auth/tokens:exchange` 与 `/scim/v2/Users` 构造错误请求，断言响应体字段；SCIM 保留 `application/scim+json`；5xx 不回显内部消息、401 带 `WWW-Authenticate`、429 带 `Retry-After` |
| 4 | 指标与审计可按面切片：指标带 `surface="management\|authentication\|protocol"` 标签；`records` 表可按 `surface` 分面聚合审计日志 | 查询 Prometheus 指标标签；`SELECT surface, count(*) FROM records ... GROUP BY surface`；`records.surface` 列（含 CHECK 约束，取值闭集 `management`/`authentication`/`protocol`）+ `(surface, created_at DESC)` 索引，写入链路带 `surface`/`request_id`/`trace_id` 且不接受客户端传参 |

**判定规则**：四条断言同时成立方可通过。协议面端点路径以《需求分析报告》§5.11.1 登记的标准家族为唯一权威。

**验证锚点**：`internal/bootstrap/surface_contract_test.go::TestSurfaceContract`（Surface 非空/合法/等于 `SurfaceFor`、三面计数非零）；`python3 scripts/check-surface-registry.py`；家族清单 `contracts/registries/surfaces.yaml`。

## 8. 商业与支付

> 本节对应 SRS COMM-001~013 与 [outbox](design/infrastructure/outbox.md)；金额精度与幂等口径与 AC-DATA-003/AC-API-003 同源。订单/订阅/交易状态集以 `contracts/registries/state-machines.yaml`（渲染层 [state-machine-registry](generated/state-machine-registry.md)）与声明式 Schema CHECK 约束为准（全小写 snake_case）。

### AC-COMM-001 订单状态机（COMM-002/003）

**Given** 用户对已上架产品执行下单/支付/取消（COMM-002 业务操作）
**When** 状态迁移发生
**Then** 状态仅沿业务声明的合法边迁移（状态集与迁移表以 COMM-003 与 DDL 权威值为准）；非法迁移（如取消后再支付）返回 409 `conflict` 并审计；取消已支付订单走退款路径（AC-COMM-006）而非直接改状态；下单幂等：同 `Idempotency-Key` 重放返回缓存响应不重复建单（25.7）。

**验证锚点**：`internal/app/orders/service_test.go:299::TestService_CreateOrder`、`:362::TestService_CancelOrder`、`:478::TestService_PayOrder_IdempotentOnSettledOrder`；状态机定义见 `internal/domain/orders/order.go` 与 `contracts/registries/state-machines.yaml`（由 `scripts/check-registry-sync.py` 校验）。

### AC-COMM-002 订阅状态族（COMM-008/009）

**Given** 订阅进入生命周期变更（开通/续费/逾期/取消/到期）
**When** 状态变更
**Then** 状态取值限定 `active`/`pending`/`trialing`/`past_due`/`expired`/`cancelled`（全小写 snake_case，统一双 l 拼写，废止单 l 变体，到期终态统一 `expired`，COMM-008；数据库 CHECK 约束与枚举注册表强制）；开始/结束时间必填且结束晚于开始（COMM-009）。

**验证锚点**：`internal/app/subscriptions/service_test.go:212::TestCreateSubscription_Validation`、`:233::TestCreateSubscription_Success`；枚举闭集门禁 `scripts/check-enums.py`；状态注册表 `contracts/registries/state-machines.yaml`。

### AC-COMM-003 支付回调签名验证（INFRA-003/[outbox](design/infrastructure/outbox.md)）

**Given** 支付渠道异步回调到达
**When** 处理回调
**Then** 验签失败（HMAC-SHA256 与渠道/平台密钥不符）返回 400 且不入账、不触发任何状态迁移，并记录安全事件；验签通过但重复投递（同渠道事件 ID 幂等键重复）返回缓存响应不重复入账（25.7）；回调处理幂等，乱序回调按事件时间与状态机仲裁，**不回退终态**。

**验证锚点**：`internal/app/payments/callback_pipeline_test.go:145::TestCallbackPipeline_NormalPath`、`:192::TestCallbackPipeline_DuplicatePayload`、`:439::TestCallbackPipeline_AmountMismatch`；渠道签名校验与用例见 `internal/infra/payment/payment_enhanced_test.go`。

### AC-COMM-004 金额精度（DECIMAL(19,4)）

**Given** 订单/支付/退款/价格相关金额字段
**When** 创建与结算
**Then** 全部定点金额列为 `DECIMAL(19,4)`、币种列为 `VARCHAR(3)`（与 AC-DATA-003 第 9 项门禁同源）；金额运算在定点域完成，禁止浮点类型参与金额计算；存储与展示精度一致，舍入规则显式声明，不隐式截断。

**验证锚点**：`scripts/check-db-conventions.sh`（金额 DECIMAL(19,4)/币种 VARCHAR(3) 全量扫描）；声明源 `contracts/schema/70_commerce.yaml`。

### AC-COMM-005 支付幂等与对账（[outbox](design/infrastructure/outbox.md) 双轨收敛）

**Given** 同一支付请求重复提交或回调重复投递
**When** 处理
**Then** 统一以 `idempotency_keys` 表仲裁（scope/`request_hash` 按 25.7）；收敛前业务幂等列（`orders`/`payments`/`webhook_events`）与统一表**双写且值相等**，收敛后业务列退化为查询冗余；对账任务按日核验「渠道侧流水 vs 本地交易记录」金额与状态，差异生成对账单并告警；对账**不改写**终态交易，差异处置全程留痕。

**验证锚点**：`internal/app/payments/callback_pipeline_test.go:192::TestCallbackPipeline_DuplicatePayload`、`:318::TestCallbackPipeline_AlreadyAtTargetIdempotent`；幂等 scope 见 `internal/app/idempotency/service.go`；对账差异双人复核见 AC-PAY-008 及其 `pay-recon-items:approve` 权限与 `internal/app/payments` 对账用例。

### AC-COMM-006 交易与退款状态（COMM-011/012/013）

**Given** 交易记录创建（类型 `payment`/`refund`/`cancellation`，COMM-012）
**When** 状态推进
**Then** 状态限定 `pending`/`completed`/`failed`/`cancelled`（COMM-013；退款语义经 `transaction_type=refund` 表达；充值类型如需启用须先扩展枚举注册表与 CHECK 约束）；退款金额合计不超过原支付金额，并发部分退款超额时返回 409 `conflict`；退款成功后原订单状态联动（AC-COMM-001）。

**验证锚点**：`internal/app/payments/refund_exec_test.go:333::TestRefundOrchestrator_Execute_ExceedsAvailable`、`:320::TestRefundOrchestrator_Execute_StateGateRejected`、`:365::TestRefundOrchestrator_Execute_PartiallyRefundedSourceState`；交易写入 `internal/app/transactions/business_test.go:136::TestRefundTransaction_Success`。

### AC-COMM-007 优惠券使用计数并发（COMM-004）

**Given** 优惠券剩余可用次数 N=1
**When** K≥2 个订单并发核销
**Then** 恰好一个核销成功、其余返回 409 `conflict`（原子扣减不超发）；核销计数与实际核销记录数对账一致；订单取消联动回补计数且幂等（重复回补不超发）。

**验证锚点**：`internal/infra/postgres/coupons/sql.go`（`QueryUpdate` 带 revision 谓词，`RowsAffected=0` 映射冲突）；`internal/app/coupons/service_test.go` 与 `internal/infra/postgres/coupons/repository_exec_test.go`；`[证据不足：当前 L4，需 L6 并发核销夹具]`。

### AC-PAY-001 渠道配置与发起支付（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** Alipay 渠道配置为 `active` + `production`
**When** 发起支付 Charge
**Then** 调用 Alipay SDK 返回 charge result（含 `channel_order_no`）。

**验证锚点**：`internal/app/payments/refund_exec_test.go:66::TestChargeOrchestrator_Execute_HappyPath`；渠道状态映射 `internal/infra/payment/alipay_stripe_test.go:11::TestMapAlipayTradeStatus`；渠道配置激活前置状态见 `internal/app/payments/channel_config_service_test.go:173::TestChannelConfigService_Transition_Valid`。

### AC-PAY-002 回调五阶段管线（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** Alipay 回调报文到达
**When** 回调管线按五阶段处理
**Then** `payments.state=succeeded` + `pay_callback_receipts.process_state=accepted` + Outbox 投递事件。

**验证锚点**：`internal/app/payments/callback_pipeline_test.go:145::TestCallbackPipeline_NormalPath`；回执落库失败路径 `:420::TestCallbackPipeline_ReceiptStorageFailed`。

### AC-PAY-003 回调幂等（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** 相同回调报文重复到达
**When** 第二次 POST 回调端点
**Then** 返回 200 + `pay_callback_receipts.process_state=duplicated`，不重复处理业务。

**验证锚点**：`internal/app/payments/callback_pipeline_test.go:192::TestCallbackPipeline_DuplicatePayload`、`:318::TestCallbackPipeline_AlreadyAtTargetIdempotent`。

### AC-PAY-004 并发退款超额拦截（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** 10 个并发退款请求（同一 payment，退款总额 > 订单金额）
**When** 全部同时发起
**Then** 锁内校验拦截超额并返回 `payment_refund_exceeds_balance`；未超额请求通过。

**验证锚点**：`internal/app/payments/refund_exec_test.go:333::TestRefundOrchestrator_Execute_ExceedsAvailable`、`:347::TestRefundOrchestrator_Execute_ExactlyAtCeiling_IsOk`、`:320::TestRefundOrchestrator_Execute_StateGateRejected`；`[证据不足：当前 L4，需 L6 —— K=10 并发退款超额拦截]`。

### AC-PAY-005 UNKNOWN 状态收敛（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** 渠道返回 `state=unknown`
**When** Charge 调用返回
**Then** `payments.state=processing` + `pay_attempts.state=unknown`，Query Worker 在 15min 内收敛为终态。

**验证锚点**：`internal/app/payments/query_worker_test.go:245::TestQueryWorker_Tick_Unknown_Passthrough`、`:188::TestQueryWorker_Tick_ConvergesSucceeded`、`:220::TestQueryWorker_Tick_ConvergesFailed`；Worker 周期与批次上限见 `internal/app/payments/query_worker.go`。

### AC-PAY-006 未激活渠道拒绝（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** 渠道配置为 `draft`
**When** 尝试 Charge
**Then** 返回 `payment_channel_not_activated`。

**验证锚点**：`internal/transport/http/api/v1/payments/payment_charge_test.go:27::TestChargePayment_DraftConfigRejected`；渠道状态迁移激活语义 `internal/app/payments/channel_config_service_test.go:194::TestChannelConfigService_Transition_ActiveSetsActivatedAt`。

### AC-PAY-007 渠道密钥轮换（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** `encryption_keys` 新增版本
**When** 用新密钥调用渠道
**Then** 解密成功且 `config_sealed.key_version` 随之更新。

**验证锚点**：`internal/platform/crypto/envelope_switch_test.go:12::TestEnvelopeEncryptor_AlgorithmSwitchOverlap`、`:65::TestFieldEncryptor_AlgorithmSwitchViaEnv`；渠道配置密文落库与指纹 `internal/app/payments/channel_config_service_test.go:253::TestChannelConfigService_Create_FingerprintUsesCanonicalPlaintext`、`:313::TestChannelConfigService_Create_FailsClosedWithoutCipher`。

### AC-PAY-008 对账差异双人复核（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** 对账批次已创建并完成差异分类
**When** 人工处置差异
**Then** 差异标记 `resolved` + 写 records 审计，且处置人 ≠ 复核人（双人复核）。

**验证锚点**：`internal/app/payments/recon_approval_pg_test.go:20::TestReconciliationApprovalPostgres`（真实 PG：审批通过、audit/outbox 各 1 条、self-approval 拒绝）；处置编排见 `internal/app/payments/recon_service.go`。

### AC-PAY-009 回调金额一致性（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** 回调金额与 `payments.price` 不一致
**When** 回调管线执行阶段 5 一致性校验
**Then** 拒绝并返回 `payment_consistency_mismatch`。

**验证锚点**：`internal/app/payments/callback_pipeline_test.go:439::TestCallbackPipeline_AmountMismatch`；阶段顺序见 `internal/app/payments/callback_pipeline.go`。

### AC-PAY-010 已支付订单取消前置退款（[design/commerce/payment.md](design/commerce/payment.md)）
**Given** 已支付订单需要取消
**When** 尝试 cancel
**Then** 返回 `payment_refund_required`（必须先退款）。

**验证锚点**：`internal/app/orders/service_test.go:362::TestService_CancelOrder`（:382 断言 `CodePaymentRefundRequired`）；错误码登记 `internal/platform/errors/errors.go:60`；取消语义 `internal/app/orders/service.go:472`。

## 9. 新能力域（V5 商业对标需求对应）

> 编号前缀：AC-REBAC-/AC-IGA-/AC-WLI-/AC-RISK-/AC-B2B-/AC-EXT-/AC-METER-/AC-DR-/AC-PRIV-，每条与 SRS 需求编号一一对应。

### AC-REBAC-001 ReBAC check API 与延迟出口（REBAC-001，ADR-024）

**Given** 关系元组与命名空间已写入（OpenFGA 兼容 API：check/write/expand/batch check/list-objects）
**When** 调用 `check(user, relation, object)`
**Then** 直接关系/继承关系/组展开均正确判定；未注册命名空间/关系/主体类型拒绝（400 `invalid_argument`）并审计；关系解析 check **P99 < 50ms**（L2 压测口径，ADR-024 出口条件①，挂 PERF 基线）。

**验证锚点**：`internal/app/rebac/evaluator.go`（直接/继承/组展开、循环检测与深度上限）；`internal/app/rebac/evaluator_test.go:43::TestCheck_DirectHit`、`:61::TestCheck_OneHopInheritance`、`:96::TestCheck_CycleDetection`、`:146::TestCheck_DepthLimitTermination`；未注册命名空间/关系/主体类型的 400 拒绝见 `internal/app/rebac/service.go`。P99 为 L2 压测口径：`[证据不足：当前 L4，需 L2 压测报告]`。

### AC-REBAC-002 Zookie 一致性与对象共享撤销（REBAC-002/005）

**Given** 关系变更后取得一致性令牌（Zookie 语义）
**When** 携令牌调用 check
**Then** 必见该写入（写后读一致）；无令牌 check 返回最新可用快照并附当前令牌，令牌防伪造；对象级共享撤销后**立即**失效（下一次 check 拒绝）；双向枚举（谁共享给我/我共享给谁）分页可用；跨组织共享按租户策略门控并审计。

**验证锚点**：`internal/app/rebac/rebac_runtime_pg_test.go:27::TestRebacCheckZookieRevocationPostgres`（真实 PG：写后读一致 + 撤销后 check 拒绝）；zookie 读取与校验见 `internal/infra/postgres/rebac/repository.go`；关系变更审计见 `internal/app/rebac/service.go:188`（`rebac.tuple.created/deleted`）。

### AC-IGA-001 JIT 时间窗授权与到期回收（IGA-003/004）

**Given** 经审批的 JIT/JEA 授权（开始时间 + 时长 + 声明范围）
**When** 窗口内/外访问
**Then** 窗口内按声明范围放行且临时权限在令牌/审计中可区分；超范围申请被拒（403 `forbidden`）；到期回收器在承诺时延内自动回收（角色移除/凭据吊销/账号禁用），回收幂等（重复扫描不重复副作用）、失败重试并告警；提前回收即时生效。

**验证锚点**：`internal/app/iga/worker.go`（`ExpireWorker` 扫描/回收/失败记录）、`internal/app/iga/jit.go`；`internal/app/iga/worker_test.go:152::TestExpireWorker_ReclaimsResidualWithoutBinding`、`:174::TestExpireWorker_RepeatedScanHasNoDuplicateSideEffects`、`:193::TestExpireWorker_FailureIsRecordedAndRetried`、`:216::TestExpireWorker_RereadsAndSkipsFutureGrant`；真实 PG 回归 `internal/app/iga/worker_runtime_pg_test.go:24::TestExpireWorkerRuntimePostgres`；回收事件 `iga.jit.expired` 见 `contracts/registries/events.yaml`。

### AC-IGA-002 审批式提权与特权会话隔离（IGA-010）

**Given** 特权操作提权请求走「申请→审批（IGA-002 矩阵）→时限授予→自动回收（IGA-004）」
**When** 未审批或审批被拒
**Then** 不产生特权会话（403 `forbidden`）；批准后提权会话带独立标记（独立令牌/会话标记）可被下游识别，特权期间操作全审计并支持实时告警；到期自动回收；回收动作写事件与审计。

**验证锚点**：`internal/app/iga/pam.go`（特权会话提升与到期夹紧）、`internal/app/iga/access.go`（多级审批与 JIT 授予）；`internal/app/iga/pam_test.go:33::TestStartPrivilegedSession_ElevatesAndClampsExpiry`、`:52::TestStartPrivilegedSession_RejectsExpiredAndFutureLifecycle`、`:80::TestAuditPrivilegedOperation_RechecksLifecycleWindow`；`internal/app/iga/access_test.go:273::TestApproveAccessRequest_TwoLevelsProvisionJITWithSlug`。

### AC-WLI-001 Service Account 一等主体（WLI-001）

**Given** 组织创建 service account 并绑定角色/组
**When** SA 参与认证与授权
**Then** SA 可完成非交互认证与 API 调用；生命周期操作（创建/轮换/禁用/删除）带预检与审计，删除前预检关联凭据/授权；SA 与人类用户在审计身份中可区分；SA 调用交互式登录端点被拒（403 `forbidden`，SA 不可登录 UI）。

**验证锚点**：`internal/app/auth/wli_auth_test.go:214::TestExchangeClientCredentials_ServiceAccountScopeAndTTLBoundaries`、`:309::TestPasswordLogin_RejectsServiceAccountIdentifiersWithoutCreatingLoginState`；生命周期 `internal/app/workload/service_test.go:405::TestCreateAccountDefaultsSecretAndAudit`、`:459::TestRotateAccountSecretKeepsClientIDAndRevokesTokens`、`:484::TestSetAccountStatusRevisionAndDeletedGuard`。

### AC-WLI-002 OIDC 联邦信任与短期凭据（WLI-002/003）

**Given** 外部 OIDC Issuer 注册为信任源（issuer/JWKS/aud 白名单 + subject 白名单，受控通配）
**When** 工作负载以联邦断言换取本系统令牌
**Then** 白名单内 subject 换发成功、白名单外拒绝并审计；aud 不匹配被拒；换发令牌时效 **≤ 1 小时**（默认短期，WLI-003）；JWKS 轮换平滑（AC-TOKEN-007 同口径）；静态长效凭据创建被门禁拦截（豁免留痕），凭据使用记录可查询。

**验证锚点**：`internal/app/auth/rfc8693_federation_test.go:89::TestResolveJWTSubject_FederationFound`、`:139::TestResolveJWTSubject_FederationDisabled_Skips`；短期令牌 `internal/app/auth/rfc8693_shortlived_test.go:17::TestTokenExchange_FederatedSubjectGetsNoRefreshToken`、`:52::TestTokenExchange_FederatedTrustMustMatchCallerOrganization`；白名单与拒绝审计 `internal/app/auth/wli_auth_test.go:375::TestResolveJWTSubject_FederationCandidatesRequireCompleteMatch`、`:408::TestResolveJWTSubject_FederationRejectionAuditAndOutbox`；信任/TTL 校验 `internal/app/workload/service_test.go:319::TestCreateTrustValidationMatrix`、`:392::TestAccessTokenTTLValidation`。

### AC-RISK-001 风险分级决策与 step-up 联动（RISK-005）

**Given** 租户启用自适应风险认证（规则引擎先行，ML 可插拔接口预留）
**When** 登录请求被分级为 medium/high/critical
**Then** medium 触发 step-up（AUTH-105）、high 触发完整 MFA、critical 拒绝并**同时**通知用户与管理员；决策对全部登录通道（QR/Device Flow/Passkey 等）一致生效；决策不降低既有静态策略强度；误报申诉可发起且处理留痕、全程审计。

**验证锚点**：`internal/app/risk/service.go`（决策复核/申诉/治理迁移 + 审计 Outbox）；`internal/app/risk/service_test.go:250::TestReviewDecision_PersistsWorkflowAuditAndOutbox`、`:285::TestReviewDecision_RejectsWeakeningAndStaleRevision`、`:313::TestAppealWorkflow_SubmitResolveAndTerminalState`；真实 PG `internal/app/risk/risk_runtime_pg_test.go:23::TestRiskDecisionAppealAuditPostgres`。

### AC-RISK-002 会话并发策略与强制下线（RISK-007/008）

**Given** 租户配置会话并发策略（不限/按应用上限/全局上限/单会话互踢）
**When** 超限登录或管理员执行强制下线 API（AC-AUTH-009 三端点，含组织级批量紧急吊销）
**Then** 超限处置按配置执行（拒新/踢旧）并通知用户；被踢会话与服务端令牌（refresh、可撤销 access）**立即**失效（内省在 30s 上界内 `active=false`）；强制下线返回吊销清单且准确（组织级批量吊销按 `users.organization_id` 解析目标会话）；批量操作限速、独立权限点、step-up + 审计。

**验证锚点**：`internal/app/risk/service.go:613::TransitionGovernance`（会话治理状态机 + 审计 outbox）；`internal/app/risk/service_test.go:347::TestTransitionGovernance_AppendsLegalTransitionAndEnforcesTerminalState`、`:380::TestWriteFailure_DoesNotCommitOrEnqueueOutbox`；会话立即失效能力见 `internal/app/sessions/service_test.go:435::TestService_RevokeSession`、`internal/app/sessions/service_cleanup_test.go:382::TestService_DeleteSession_Success`；路由权限点见 `internal/transport/http/api/v1/risk/routes.go`（iga/risk 独立权限 + step-up 中间件）。

### AC-RISK-003 审计哈希链与完整性校验（RISK-013）

**Given** `records` 审计记录按链式哈希写入（每条含前条哈希）
**When** 运行链完整性校验工具
**Then** 单点篡改可被发现并定位断链位置；链头/链尾定期快照锚定（外部时间戳或异地存储）；校验工具只读、越权调用 403 `forbidden`；链构建不阻塞审计写入（延迟语义见 [audit.md「链构建模型」](design/infrastructure/audit.md)）；初始化/轮转流程可重放。

**验证锚点**：`internal/app/risk/service.go:978::VerifyAuditIntegrity`（只读校验）、`internal/app/risk/service_test.go:395::TestVerifyAuditIntegrity_ReadOnlyResult`、`:492::TestVerifyAuditIntegrity_RejectsCrossTenantChain`；链写入与锚定 `internal/platform/outbox/providers/audit/chain.go`、`internal/platform/outbox/providers/audit/chain_anchor_test.go:33::TestComputeAnchorProof_DeterministicRecipe`、`:164::TestAnchor_NewTailInsertsAnchorAndAdvancesStatus`；真实 PG 链 `internal/infra/postgres/records/chain_sm3_pg_test.go:52::TestAuditChainSM3Postgres`。

### AC-B2B-001 组织树策略继承（B2B-001/002）

**Given** 多级组织树（parent_id）与策略继承声明（安全策略/主题/注册项可声明继承/覆盖/锁定）
**When** 子组织访问受继承策略约束的资源
**Then** 继承/覆盖/锁定三模式行为正确；父级锁定项子级修改被拒（403 `forbidden`）；继承解析为确定性求值且可预览生效链（预览与实际一致）；解析不引入跨租户数据泄露（AC-TENANT-002 同口径）。

**验证锚点**：`internal/app/organizations/policy_inheritance_test.go:19::TestPreviewPolicyInheritance_RootToChild`、`:61::TestUpdateSecurityPolicy_AncestorLockForbidden`、`:100::TestUpdateTheme_AncestorLockForbidden`；真实 PG 继承解析 `internal/app/organizations/policy_inheritance_pg_test.go:20::TestPreviewPolicyInheritancePostgres`；预览路由 `/api/v1/organizations/{id}/policy-inheritance/preview`（`internal/transport/http/api/v1/organizations/routes.go`）。

### AC-B2B-002 企业域名认领与 SSO 发现（B2B-004/005）

**Given** 企业提交域名认领（DNS TXT 验证）
**When** 用户以该域邮箱登录
**Then** **未验证域不得触发 SSO 发现**（防域名抢注劫持，拒绝并审计）；验证通过后邮箱域 SSO 发现路由到正确 IdP；同域多组织认领冲突有显式仲裁规则且审计；TXT 记录移除后的失效策略显式声明并执行。

**验证锚点**：`internal/app/b2b/service_test.go:293::TestClaimDomain_Success`、`:373::TestVerifySingle_MatchesToken`、`:497::TestFindForSSO_VerifiedDomain`；租户隔离 `internal/transport/http/api/v1/b2b/tenant_by_id_test.go:208::TestB2BListDomains_UsesTrustedOrganization`、`:323::TestB2BVerifyDomain_ForeignTenantRejected`。

### AC-EXT-001 Actions 沙箱资源限额（EXT-002）

**Given** Action 挂载于登录管线触发点（触发点目录 ≥5，EXT-001）
**When** 执行超出资源限额
**Then** CPU/内存/时长限额生效并可观测；超时/超内存按失败策略处理（EXT-004）；沙箱逃逸与资源耗尽防护测试用例全部被阻断；沙箱版本可升级且升级有回归测试。

**验证锚点**：`internal/app/actions/sandbox_test.go:11::TestNormalizeExecOptions_Defaults`、`:27::TestNormalizeExecOptions_ClampTimeout`、`:49::TestNormalizeExecOptions_PreserveExplicit`；WASM 内存上限 `internal/app/actions/wasm_executor_test.go:149::TestMemoryLimitPages`；隔离与迭代用例 `internal/app/actions/wasm_iteration3_test.go`。

### AC-EXT-002 Actions 密钥注入与出站白名单（EXT-003）

**Given** Action 声明所需密钥与出站域名
**When** 运行时执行
**Then** 密钥仅注入态可见、不落配置明文不进日志；出站网络调用强制白名单（域名/网段），越界请求被阻断并审计（403 `forbidden` 语义 + 安全事件）；白名单变更审计；密钥轮换后新密钥即时生效。

**验证锚点**：`internal/app/actions/proxy_test.go:25::TestWhitelistProxy_Allowed`、`:91::TestWhitelistProxy_GetDenied`、`:119::TestWhitelistProxy_EmptyWhitelistDeniesEverything`（出站白名单强制）；拒绝短路 `internal/app/actions/dispatcher_test.go:234::TestDispatch_DenyShortCircuit`；`[证据不足：当前 L4，密钥注入态/轮换即时生效需 L6 夹具]`。

### AC-METER-001 用量计量事件管道（METER-001/002）

**Given** 计量指标 ≥3 项（MAU/认证次数/令牌数等）
**When** 计量事件产生
**Then** 事件经管道聚合去重（去重口径显式声明，METER-003），按租户/时段可查询；事件不丢不重（Outbox/幂等同栈，AC-EVENT-002）；指标定义变更版本化并保持历史口径可解释。

**验证锚点**：`internal/app/metering/service.go:136::Ingest`（`inserted`/`duplicates` 去重口径）；`internal/app/metering/aggregate_worker_test.go:65::TestAggregateWorker_WindowsInRange`、`:102::TestAggregateWorker_IdempotentReRun`；审计/Outbox 事件 `internal/app/metering/service.go:184`（`metering.events.ingested`，**该事件未登记 events.yaml —— 见 DEF-14**）。

### AC-METER-002 租户配额与超限处置（METER-004）

**Given** 租户配额绑定套餐/计划（ORG-020/COMM-008 衔接）
**When** 用量达到/超过配额
**Then** 软阈值告警、硬阈值按显式声明策略处置（拒新会话/降级/只读）；配额超限拒绝为 **403 `forbidden`**（策略/配额类，七子句节判定规则），503 保留给平台级容量故障；配额变更即时生效并审计；用量对账与 METER-001 管道一致。

**验证锚点**：`internal/domain/metering/quota.go`（`QuotaThresholdOK/Soft/Hard` 决策）；`internal/domain/metering/quota_test.go:11::TestTenantQuota_ConsumeBoundaries`、`:53::TestApplySoftThreshold`、`:70::TestPlanQuotaValidate`；`[证据不足：当前 L4，配额决策尚未接线到运行时拒绝路径，需代码修复 + L6]`。

### AC-DR-001 RTO/RPO 三档演练（DR-002，[non-functional §6.2](requirements/non-functional.md)）

**Given** 三档故障场景（L1 应用层/L2 数据库主库/L3 数据损坏）
**When** 执行恢复演练
**Then** 实测 RTO/RPO 达标：**L1 RTO<1min/RPO=0；L2 RTO<5min/RPO<5min；L3 RTO<4h/RPO<15min**（本行为 `contracts/registries/dr.yaml` tiers 的验收投影，冲突以注册表为准）；跨区域场景单独声明且不得优于 L1~L3 承诺；复制模式（同步/异步）按数据类别可查；演练记录（剧本/实测值/差距项与整改）归档且可在审计员视角复核。

**验证锚点**：`docs/operations/backup-dr.md`（三档场景/恢复流程/演练记录模板）；恢复演练脚本 `scripts/release/rehearse_v1_restore.py`。

### AC-DR-002 区域桶与数据面驻留（DR-001）

**Given** 多区域部署（控制面/数据面分离，区域桶承载租户数据）
**When** 区域内租户读写数据
**Then** 数据面读写**不跨区域落盘**；租户-区域归属在控制面注册表可查；区域内故障不跨区域扩散；控制面故障时数据面按既定策略降级运行并告警；区域路由拓扑有探针。

**口径注记**：区域路由/探测接线的缺口与出口条件见 `OPEN-API-38`。

**验证锚点**：`internal/platform/regions/stub_test.go`（区域解析桩/驻留校验）；传输层区域守卫 `internal/transport/http/middleware/tenant_resolution_region_test.go`；缺口与出口条件以 `OPEN-API-38` 为准。

### AC-PRIV-001 DSAR 处置时限与擦除边界（PRIV 域，[data.md §3.2](requirements/data.md) / [non-functional §5.2](requirements/non-functional.md)）

**Given** 数据主体提交访问/更正/冻结/擦除请求（DSAR 工作台）
**When** 请求受理
**Then** 受理后 **30 天内**完成处置（权威口径 `contracts/registries/lifecycle.yaml` completion_days: 30，需求侧见 [data.md §3.2](requirements/data.md)）；擦除执行软删除 + 字段匿名化（用户名/邮箱/手机号替换为不可逆标识），会话/令牌/凭证全部物理删除；审计骨架保留且主体标识仅存哈希（审计链不断裂）；请求状态流转全程可查询并审计。

**验证锚点**：`internal/app/privacy/dsar_service_test.go:315::TestService_Export_FulfilledAccessRequest`、`:338::TestService_Export_RequiresFulfilledState`、`:355::TestService_Export_RejectsEraseRequest`、`:372::TestService_ReviewErasure_AllowedScope`、`:394::TestService_ReviewErasure_LegalRetentionBlocks`；路由/权限 `internal/transport/http/api/v1/privacy/chi_test.go:172::TestPrivacyRoutes_ReviewErasureScope`；`[证据不足：当前 L4，30 天时限/匿名化物理删除需 L6]`。

### AC-PRIV-002 同意版本与冷静期（PRIV 域，[non-functional §5.2](requirements/non-functional.md)）

**Given** 用户同意记录版本化与擦除冷静期机制
**When** 同意版本更新或冷静期内撤回擦除请求
**Then** 同意记录含版本号/时间/范围且历史版本可追溯；冷静期内撤回可恢复且全程审计；冷静期外处置不可逆（二次确认）；强制类同意（安全通知）不可关闭。

**验证锚点**：`internal/domain/consents/consent_test.go:10::TestConsent_Fields`、`:31::TestConsent_Validate`、`:86::TestConsent_OAuthConsentLifecycle`（版本/范围/撤销生命周期）；`[证据不足：当前 L4，擦除冷静期撤回未发现实现/测试，需代码补全 + L6]` → DEF-23。

## 10. AI 网关

### AC-AIGW-001 OpenAI 协议面兼容（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** OpenAI SDK 客户端
**When** 调用 `POST /api/v1/ai/faces/openai/chat/completions`（alias=`gpt-4o`）
**Then** 返回 200 + 响应体符合 OpenAI 格式 + Token 用量正确。

**验证锚点**：`internal/app/aigateway/converter_test.go:15::TestConvertChatRequestForFace_OpenAI`（OpenAI 协议请求转换）；OpenAI 兼容协议端点态用见 `internal/transport/http/api/v1/aigateway/` 的 chat completions handler 测试。

### AC-AIGW-002 Anthropic 协议面兼容（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** Anthropic SDK 客户端
**When** 调用 `POST /api/v1/ai/faces/anthropic/chat/completions` 或原生 `POST /api/v1/ai/faces/anthropic/messages`（alias=`claude-sonnet`；Anthropic Messages 请求体经适配器映射）
**Then** 返回 200 + 响应体符合 Anthropic Messages 格式。

**验证锚点**：`internal/app/aigateway/converter_test.go:38::TestConvertChatRequestForFace_AnthropicNative`、`:184::TestConvertChatResponseAnthropic`、`:357::TestAnthropicResponse_JSONShape`、`:266::TestDomainEventToAnthropicSSE_ContentDelta`。

### AC-AIGW-003 请求幂等重放（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** 同一请求带相同 `Idempotency-Key`
**When** 重复调用 Chat Completions
**Then** 第二次返回缓存响应 + 响应头 `Idempotency-Replayed: true`。

**验证锚点**：全局幂等中间件挂载 `internal/bootstrap/chi.go:244`（`IdempotencyMiddleware`）；`internal/platform/middleware/idempotency_test.go`（重放/缓存响应）；`[证据不足：当前 L4，协议面是否豁免幂等需 L6]`。

### AC-AIGW-004 配额限流（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** 已配置 RPM 限制为 60 的 API Key
**When** 1 秒内发送 61 次请求
**Then** 第 61 次返回 429 + `Retry-After`。

**验证锚点**：`internal/app/aigateway/gateway_service_outbox_test.go:144::TestGatewayService_Chat_RPMExceeded_EmitsQuotaExceeded`、`:188::TestGatewayService_Chat_TPMExceeded_EmitsQuotaExceeded`；配额判定 `internal/app/aigateway/gateway_quota.go`。

### AC-AIGW-005 路由故障转移（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** 路由配置 provider A（权重 100）+ provider B（权重 100），且 A 熔断 open
**When** 发起 Chat Completions
**Then** 自动 failover 到 B，并将 `failover_from=provider_a` 写入 `llm_usage_logs`。

**验证锚点**：`internal/domain/aigateway/router.go`（priority/weight/failover 语义）与 `internal/domain/aigateway/topology.go`；健康熔断/半开 `internal/domain/aigateway/provider_health_test.go:36::TestProviderHealth_CircuitOpensOnThreshold`、`:57::TestProviderHealth_HalfOpenSuccessClosesBreaker`；执行链路 `internal/app/aigateway/gateway_exec.go`。

### AC-AIGW-006 跨协议面适配无语义损失（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** 三协议面同时注册，且某 alias 使用 Anthropic 供应商
**When** 从 OpenAI 协议面调用该 alias
**Then** Canonical → Anthropic 适配器 → Anthropic Upstream 正确转换，无语义损失。

**验证锚点**：`internal/app/aigateway/converter_test.go`（OpenAI/Anthropic/Gemini 请求、响应与 SSE 全量转换用例，`:109` 起含 Gemini native、`:244`/`:266`/`:308` 三种协议 SSE）。

### AC-AIGW-007 已撤销 API Key 拒绝（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** API Key 被撤销
**When** 用该 Key 调用数据面
**Then** 返回 401 + 错误码 `ai_api_key_revoked`。

**验证锚点**：`internal/app/aigateway/gateway_core.go:106`（`APIKeyStatusRevoked` → unauthorized）；API Key 状态闭集 `internal/domain/aigateway/api_key_test.go`。

### AC-AIGW-008 跨租户隔离（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** 组织 A 的 API Key
**When** 尝试访问组织 B 的 alias
**Then** 路由解析失败（RLS 拦截）并返回 404。

**验证锚点**：`internal/app/aigateway/admin_model_tenant_test.go:117::TestAdminService_ManagementByIDFailsClosedWithoutScopedRepos`；`internal/app/aigateway/admin_usage_tenant_test.go:159::TestGetHealthForOrganization_ForeignProviderUsesOnlyScopedReads`、`:216::...ForeignHealthRecordIDIsNotFound`；传输层 `internal/transport/http/api/v1/aigateway/admin_handler_test.go:731::TestAdminHandler_ProviderByIDForeignTenantIsNotFoundAndUnwritten`。

### AC-AIGW-009 tool/MCP 调用闭环（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** 启用 tool 的 alias 且 MCP Server 已注册工具
**When** 请求含 `tools[]` + `tool_choice=auto`
**Then** 模型返回 tool_use → 网关调用 MCP Server → 工具结果回传模型继续推理。

**验证锚点**：`internal/app/aigateway/stream_tool_test.go:63::TestChatStream_ToolUse_AutoExecute`、`:158::TestChatStream_ToolUse_NoMCPBinding`、`:195::TestChatStream_ToolUse_MultipleDeltas_AggregatesInput`；结果脱敏 `internal/app/aigateway/tool_service_sanitize_test.go:32::TestExecuteTools_SanitizesToolResultAndLog`。

### AC-AIGW-010 流式透传（[design/ai-gateway/README.md](design/ai-gateway/README.md)）
**Given** OpenAI SDK 调用方请求流式
**When** 发起 Chat Completions + `stream: true`
**Then** SSE 透传 + 首 Token 延迟可观测 + 心跳正常。

**验证锚点**：`internal/app/aigateway/gateway_service_test.go:418::TestGatewayService_ChatStream_Success`、`:439::TestGatewayService_ChatStream_UpstreamFails`；协议 SSE 编码 `internal/app/aigateway/converter_test.go:244`/`:266`/`:308`。

## 11. 架构与分层合规

### AC-ARCH-001 分层依赖零违规（layers §8.2.2，9 规则）

**Given** 全仓 Go 生产代码（`internal/**/*.go`）
**When** 执行 `bash scripts/check-layers.sh`
**Then** Rule 1–9 全部执行：阻塞规则（Rule 1–5、Rule 9）的违规数必须为 **0**；Rule 6（`TODO(phase3)` 标记）只报告计数，Rule 7（platform → app/infra/transport）与 Rule 8（infra → app，排除 `*/uow/uow.go`）为 **informational**，均不累加退出码；脚本不包含 Bootstrap 包反向依赖规则。

**验证锚点**：`scripts/check-layers.sh`（Rule 1–9；阻塞规则违规数必须为 0，Rule 6/7/8 informational 不累加退出码）。

### AC-ARCH-002 Platform 边界纯净（layers §7.1 B2）

**Given** `internal/platform/` 目录下的生产代码
**When** 扫描 import 路径
**Then** 不存在对 `internal/app/`、`internal/infra/`、`internal/domain/`、`internal/transport/` 具体包的 import（共享接口例外，由 `check-layers.sh` 第 7 条验证）。

**验证锚点**：`scripts/check-layers.sh`（Rule 7：platform 不 import app/infra/transport/domain 具体包）；前端/应用边界辅助门禁 `scripts/check-app-barrel-imports.py`。

### AC-ARCH-003 状态列闭集（开发规范 §3.2，命名裁决 N15）

**Given** `contracts/schema/*.yaml` 与 `internal/platform/enumset`
**When** 扫描所有生产表与枚举注册
**Then** 状态列一律命名为 `status`（`state` 仅存量例外）；每个状态机的合法值集在 `contracts/registries/state-machines.yaml` 中唯一登记，`enumset.NewSet` 同步生成；`scripts/check-enums.py` 阻塞保证前后端闭集一致。

**验证锚点**：`scripts/check-enums.py`（前后端闭集一致 + 状态列命名）；唯一登记源 `contracts/registries/state-machines.yaml`；enumset 行为 `internal/platform/enumset/enumset_test.go:12::TestNewSet_KeepsDeclarationOrderAndMembership`、`:40::TestNewSet_PanicsOnMalformedDeclaration`、`:59::TestSet_Validate`。

## 签署与证据规则

- AC 的通过证据一律以**运行证据**承载：可复现的脚本/命令输出，并链接到 `traceability-matrix.md` 对应需求行与 CI/发布记录；不维护人工签字行。
- AC 覆盖统计（数量、域分布）以脚本输出为准，不在文档中手写计数。

## 补充验收条件（身份、应用、Provider、前端契约）

> 本节承载 USER / APP / PROV 三域与前端契约的补充验收条件（R0 完备性审查发现其原无专属 AC），需求编号锚点见各条标题。

### AC-USER-001 用户 CRUD 与状态机（USER-001/002/021）

**Given** 已认证管理员且具备 `users:write` 权限
**When** 创建、更新、删除用户或查询用户详情
**Then** 成功路径返回脱敏用户视图；重复用户名返回 409 `conflict`；非法字段返回 422 `unprocessable_entity`；跨租户 by-ID 一律 404 `not_found`；删除内置管理员或操作者自身返回 403 `forbidden`（删除保护）；写操作同事务写 `records`（actor/目标/结果）。

**验证锚点**：`internal/app/users/crud_test.go`（`TestCreateUser_Success` / `TestCreateUser_DuplicateUsername` / `TestCreateUser_InvalidFields` / `TestDeleteUser_Success` / `TestGetUser_NotFound`）；路由 `internal/transport/http/api/v1/users/routes.go`；设计见 [design/iam/users.md](design/iam/users.md)。

### AC-USER-002 身份绑定与凭证子资源（USER-006/007/009/011）

**Given** 用户存在且调用者具备 `users:write`
**When** 绑定/解绑身份、更新凭证、增删角色绑定或组成员关系
**Then** 子资源写操作成功并写审计；重复绑定返回 409 `conflict`；跨租户子资源访问返回 404 `not_found`；凭证读接口只返回 `XxxSet` + `XxxMasked`（不回显明文）；解绑最后一种登录身份被拒绝并给出原因。

**验证锚点**：`internal/app/users/identity_subresource_test.go`；`internal/app/users/delete_identity.go`、`delete_group_membership.go`、`delete_role_binding.go`；路由 `/api/v1/users/{id}/identities`、`/role-bindings`、`/group-memberships`、`/credentials`（`internal/transport/http/api/v1/users/routes.go`）。

### AC-USER-003 MFA 方法与偏好（USER-012/013/015/016）

**Given** 用户存在且调用者具备管理权限
**When** 管理员重绑 TOTP、设置首选 MFA 方法、查询或删除 MFA 方法
**Then** 重绑受冷却期约束（冷却期内拒绝并给出可重试时间）；MFA 方法删除联动撤销该用户相关会话；首选方法变更即时生效并写审计；读接口不返回 TOTP secret 或恢复码明文。

**验证锚点**：`internal/app/users/admin_mfa_test.go`（`TestAdminRebindCoolingDown` / `TestService_AdminRebindTOTP_CoolingDown`）；路由 `/api/v1/users/{id}/mfa-methods`、`/preferred-mfa`；设计见 [design/auth/mfa.md](design/auth/mfa.md)。

### AC-APP-001 应用 CRUD 与凭据（APP-001/002/003/005）

**Given** 已认证管理员且具备 `applications:write`
**When** 创建/更新/删除 OAuth 客户端应用，或轮换 client secret
**Then** 创建成功返回一次性 secret（此后只存 Argon2id 哈希、读接口掩码）；重复 `client_id` 返回 409 `conflict`；非法 `redirect_uri` 返回 422 `unprocessable_entity`；删除前预检关联绑定，存在依赖时拒绝并给出原因；全部写操作写 `records`。

**验证锚点**：`internal/app/applications/create_application.go`、`client_secret.go`；`internal/app/applications/client_secret_redline_test.go`、`client_secret_fake_test.go`；路由 `internal/transport/http/api/v1/applications/routes.go`；设计见 [design/iam/applications.md](design/iam/applications.md)。

### AC-APP-002 DCR 与导入导出（APP-006/007/009）

**Given** 调用者具备应用管理权限
**When** 通过 RFC 7591/7592 动态注册或管理客户端，或导入/导出应用配置
**Then** DCR 校验 `redirect_uri` 长度与白名单、`private_key_jwt` 必须携带 JWKS（缺失 400 `invalid_client_metadata`）；注册响应一次性返回 secret，管理凭据只存哈希；导入不得接收 secret 字段；导出不含任何 secret/私钥；以上均写审计。

**验证锚点**：`internal/app/applications/dcr.go`；`internal/app/applications/dcr_test.go`（`TestDCRRegistration_RejectsPrivateKeyJWTWithoutJWKS` / `TestDCRRegistration_AcceptsPrivateKeyJWTWithJWKS` / `TestDCRRegistration_DefaultsTokenSigningMethodToRS256`）；设计见 [design/protocol/oauth2.md §14.28](design/protocol/oauth2.md)。

### AC-PROV-001 Provider 注册与健康（PROV-001/002）

**Given** 调用者具备 `providers:write`
**When** 创建/更新/删除统一 Provider、测试连接或查看健康状态
**Then** 托管 kind（LDAP/验证码/Face ID 族）在统一入口创建返回 422 并引导专用控制台；一致性 Worker 产出校验证据、异常时回滚证据写入；健康 degraded 不得改变授权/认证判定；全部写操作写 `records`。

**验证锚点**：`internal/app/providers/create_provider.go`、`delegated.go`；`internal/app/providers/consistency_worker_test.go`（`TestConsistencyWorkerCleanRunWritesCompletedEvidence` / `TestConsistencyWorkerRollsBackEvidenceWhenEnqueueFails`）；`internal/app/providers/delegated_test.go`（`TestUnifiedCreateRejectsDelegatedIdentityKinds`）；设计见 [design/infrastructure/providers.md §10](design/infrastructure/providers.md)。

### AC-FE-018 页面↔API 契约（FE-001/004/006/010）

**Given** 控制台任一页面
**When** 发起列表、筛选、排序、详情或表单提交请求
**Then** 请求的「方法 + 路径」必须命中已声明路由（未声明端点一律不得调用）；筛选下拉的键必须是后端读取参数；排序字段必须命中服务端白名单；校验失败必须映射到字段级错误并可聚焦；页面入口与行操作受权限点保护，无权时不渲染。

**验证锚点**：设计 [design/frontend/interaction.md](design/frontend/interaction.md)「页面与 API 契约硬约束」；门禁 `scripts/check-ui-wiring.py`、`scripts/check-frontend-coverage.py`、`scripts/check-http-methods.py`。

## 附录 A：需求覆盖索引（显式映射）

> 用途：登记「需求编号 → 覆盖该需求的 AC」的显式映射，供 `scripts/gen_traceability.py` 合并块作用域解析结果，生成 `traceability-matrix.md` 的交叉索引。本表只登记映射，不复制 AC 正文；与 AC 正文引用冲突时以 AC 正文为准并修正本表。

| 需求编号 | 覆盖 AC |
|---|---|
| AUTH-001 | AC-AUTH-001 |
| AUTH-002 | AC-AUTH-001 |
| AUTH-003 | AC-AUTH-001 |
| AUTH-004 | AC-AUTH-003 |
| AUTH-005 | AC-AUTH-003 |
| AUTH-006 | AC-AUTH-003 |
| AUTH-010 | AC-AUTH-012 |
| AUTH-011 | AC-AUTH-012 |
| AUTH-017 | AC-OAUTH-001 |
| AUTH-018 | AC-OAUTH-001 |
| AUTH-019 | AC-TOKEN-009 |
| AUTH-020 | AC-TOKEN-001、AC-TOKEN-005 |
| AUTH-026 | AC-TOKEN-002 |
| AUTH-028 | AC-TOKEN-007 |
| AUTH-029 | AC-TOKEN-007 |
| AUTH-034 | AC-PROTO-001 |
| AUTH-036 | AC-PROTO-001 |
| AUTH-038 | AC-PROTO-001 |
| AUTH-044 | AC-AUTH-004 |
| AUTH-045 | AC-AUTH-012 |
| AUTH-046 | AC-AUTH-012 |
| AUTH-050 | AC-AUTH-004 |
| AUTH-072 | AC-PROTO-002 |
| AUTH-074 | AC-TOKEN-007、AC-PERM-004 |
| AUTH-075 | AC-AUTH-012 |
| AUTH-077 | AC-NCIDAS-001 |
| AUTH-078 | AC-NCIDAS-002 |
| AUTH-079 | AC-NCIDAS-004 |
| AUTH-080 | AC-NCIDAS-005 |
| ORG-004 | AC-TENANT-001、AC-TENANT-002、AC-TENANT-003 |
| PERM-001 | AC-PERM-001 |
| PERM-002 | AC-PERM-002 |
| PERM-003 | AC-PERM-001 |
| PERM-004 | AC-PERM-002 |
| PERM-005 | AC-PERM-002 |
| PERM-006 | AC-PERM-002 |
| PERM-007 | AC-PERM-001 |
| PERM-010 | AC-PERM-001、AC-PERM-003 |
| PERM-011 | AC-PERM-001 |
| PERM-012 | AC-PERM-004 |
| INFRA-001 | AC-WEBHOOK-001、AC-WEBHOOK-002 |
| INFRA-019 | AC-SEC-005 |
| SEC-001 | AC-SEC-004 |
| SEC-002 | AC-SEC-004 |
| SEC-003 | AC-SEC-001、AC-RATELIMIT-001 |
| SEC-004 | AC-SEC-002 |
| SEC-012 | AC-AUTH-012、AC-RATELIMIT-001 |
| PRIV-001 | AC-PRIV-001 |
| PRIV-010 | AC-DR-002 |
| DATA-WR-001 | AC-OUTBOX-001、AC-AUDITCHAIN-001 |
| DATA-RD-001 | AC-TENANT-002、AC-API-006 |
| DATA-AS-001 | AC-OUTBOX-001、AC-IDEM-001 |
| DATA-AU-001 | AC-AUDITCHAIN-001、AC-AUDITCHAIN-002 |
| DATA-MT-001 | AC-METER-001 |
| AUTH-067 | AC-AUTH-016 |
| AUTH-068 | AC-AUTH-016 |
| USER-001 | AC-USER-001 |
| USER-002 | AC-USER-001 |
| USER-021 | AC-USER-001 |
| USER-006 | AC-USER-002 |
| USER-007 | AC-USER-002 |
| USER-009 | AC-USER-002 |
| USER-011 | AC-USER-002 |
| USER-012 | AC-USER-003 |
| USER-013 | AC-USER-003 |
| USER-015 | AC-USER-003 |
| USER-016 | AC-USER-003 |
| APP-001 | AC-APP-001 |
| APP-002 | AC-APP-001 |
| APP-003 | AC-APP-001 |
| APP-005 | AC-APP-001 |
| APP-006 | AC-APP-002 |
| APP-007 | AC-APP-002 |
| APP-009 | AC-APP-002 |
| PROV-001 | AC-PROV-001 |
| PROV-002 | AC-PROV-001 |
| FE-001 | AC-FE-018 |
| FE-004 | AC-FE-018 |
| FE-006 | AC-FE-018 |
| FE-010 | AC-FE-018 |
