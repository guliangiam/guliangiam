# 功能性需求

> 目标态规范「功能性需求」；写作约束见 [docs/README.md](../README.md)。需求编号为唯一事实源，验收条件见 [acceptance-criteria.md](../acceptance-criteria.md)，双向追溯见 [traceability-matrix.md](../traceability-matrix.md)。

---

## 1. 认证与登录 [REQ:MODULE-AUTH]

### 1.1 密码登录 [REQ:AUTH-001 ~ AUTH-009]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-001 | 系统必须支持用户名/密码登录 | P0 |
| AUTH-002 | 系统必须支持邮箱/密码登录 | P0 |
| AUTH-003 | 系统必须支持手机号/密码登录 | P0 |
| AUTH-004 | 系统必须支持组织级密码策略配置 | P0 |
| AUTH-005 | 系统必须支持现代密码哈希算法白名单：argon2id / bcrypt / pbkdf2-salt / sha256-salt / sha512-salt（白名单闭集；plain、md5-user-salt、pbkdf2-django 不在支持范围） | P0 |
| AUTH-006 | 系统必须支持密码盐值（PasswordSalt）机制 | P0 |
| AUTH-007 | 系统必须支持密码过期检查与强制修改流程 | P1 |
| AUTH-008 | 系统必须支持密码历史检查，防止近期密码复用（默认 5 条；历史记录以 Argon2id 存储，`PASSWORD_HISTORY_PEPPER` 参与历史比对，遗留 SHA-256 记录仅兼容校验） | P1 |
| AUTH-009 | 系统必须支持密码泄露检查（Have I Been Pwned k-匿名协议） | P2 |

**认证处理顺序固定**：先判账号锁定状态 → 再校验密码。锁定检查必须先于密码校验，锁定期请求直接返回通用错误，不进入密码哈希比对路径。

### 1.2 验证码登录 [REQ:AUTH-010 ~ AUTH-016]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-010 | 系统必须支持邮箱验证码登录 | P0 |
| AUTH-011 | 系统必须支持短信验证码登录 | P0 |
| AUTH-012 | 系统必须支持图形验证码（内置） | P1 |
| AUTH-013 | 系统必须支持 reCAPTCHA v2/v3 | P2 |
| AUTH-014 | 系统必须支持 hCaptcha | P2 |
| AUTH-015 | 系统必须支持 Cloudflare Turnstile | P2 |
| AUTH-016 | 系统必须支持 GeeTest 极验 | P2 |

验证码安全：有效期 10 分钟、失败 3 次即失效该码、IP 级 10 次/小时、一次性使用（事实源 `internal/app/verifications/send_verify.go`：`verificationCodeTimeout` 与 RateLimiter 口径；限流参数注册表见 `contracts/registries/limits.yaml`）。验证码发送侧频率限制见 SEC-012。

### 1.3 OAuth 2.0 / OIDC [REQ:AUTH-017 ~ AUTH-033]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-017 | 系统必须支持 Authorization Code 授权码模式 | P0 |
| AUTH-018 | 系统必须支持 PKCE (S256 only) | P0 |
| AUTH-019 | 系统必须支持 Client Credentials 客户端凭证模式 | P0 |
| AUTH-020 | 系统必须支持 Refresh Token 刷新令牌模式 | P0 |
| AUTH-021 | 系统必须支持 Device Authorization (RFC 8628) | P1 |
| AUTH-022 | 系统必须支持 Token Exchange (RFC 8693) | P1 |
| AUTH-023 | 系统必须支持 JWT Bearer Grant (RFC 7523) | P1 |
| AUTH-024 | 系统必须支持 DPoP (RFC 9449) 令牌绑定证明 | P1 |
| AUTH-025 | 系统必须支持 Dynamic Client Registration (RFC 7591) | P1 |
| AUTH-026 | 系统必须支持 Token Introspection (RFC 7662) | P0 |
| AUTH-027 | 系统必须支持 Token Revocation (RFC 7009) | P0 |
| AUTH-028 | 系统必须支持 OIDC Discovery | P0 |
| AUTH-029 | 系统必须支持 JWKS 端点 | P0 |
| AUTH-030 | 系统必须支持 Refresh Token 旋转与重用检测 | P1 |
| AUTH-031 | 系统必须支持 Consent 管理（grant/check/revoke） | P0 |
| AUTH-032 | 系统必须支持 Native SSO | P2 |
| AUTH-033 | 系统必须支持 Back-channel Logout | P1 |

### 1.4 SAML 2.0 [REQ:AUTH-034 ~ AUTH-038]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-034 | 系统必须支持 SAML IdP（签发 SAML Response） | P0 |
| AUTH-035 | 系统必须支持 SAML SP-Initiated SSO（完整流程） | P1 |
| AUTH-036 | 系统必须支持 SAML IdP Metadata 端点 | P0 |
| AUTH-037 | 系统必须支持 SAML SP Metadata 生成 | P1 |
| AUTH-038 | 系统必须支持 SAML ACS (Assertion Consumer Service) 回调 | P0 |

### 1.5 CAS 协议 [REQ:AUTH-039 ~ AUTH-040]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-039 | 系统必须支持 CAS 1/2/3 协议 | P1 |
| AUTH-040 | 系统必须支持 /cas/login、/cas/serviceValidate、/cas/p3/serviceValidate | P1 |

### 1.6 WebAuthn / Passkeys [REQ:AUTH-041 ~ AUTH-043]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-041 | 系统必须支持 WebAuthn 注册（Signup Begin/Finish） | P1 |
| AUTH-042 | 系统必须支持 WebAuthn 登录（Signin Begin/Finish） | P1 |
| AUTH-043 | 系统必须存储 WebAuthn 凭证（credential_id, public_key, sign_count, transports, aaguid） | P1 |

### 1.7 多因子认证 (MFA) [REQ:AUTH-044 ~ AUTH-051]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-044 | 系统必须支持 TOTP (Time-based OTP) | P0 |
| AUTH-045 | 系统必须支持 SMS 短信验证码 | P0 |
| AUTH-046 | 系统必须支持 Email 邮件验证码 | P0 |
| AUTH-047 | 系统必须支持 Recovery Code 恢复码 | P0 |
| AUTH-048 | 系统必须支持 WebAuthn 硬件密钥 | P1 |
| AUTH-049 | 系统必须支持 Push MFA 推送认证 | P1 |
| AUTH-050 | 系统必须支持 MFA Challenge 状态机 (pending → push_pending → approved/rejected) | P0 |
| AUTH-051 | 系统必须支持 MFA 记住设备（可配置免验证期） | P2 |

### 1.8 QR Code 扫码登录 [REQ:AUTH-052 ~ AUTH-058]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-052 | 系统必须支持 QR Code 创建 | P1 |
| AUTH-053 | 系统必须支持 QR Code 扫描 | P1 |
| AUTH-054 | 系统必须支持 QR Code 确认 | P1 |
| AUTH-055 | 系统必须支持 QR Code 状态轮询 | P1 |
| AUTH-056 | 系统必须支持 QR Code 令牌交换 | P1 |
| AUTH-057 | QR Code 必须有 5 分钟过期机制 | P1 |
| AUTH-058 | QR Code 状态机严格单向：pending → scanned → confirmed → exchanged | P1 |

### 1.9 Face ID 活体检测 [REQ:AUTH-059 ~ AUTH-062]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-059 | 系统必须支持 Face ID 挑战生成（blink/turn_head/nod/open_mouth） | P1 |
| AUTH-060 | 系统必须支持 Face ID 活体响应验证 | P1 |
| AUTH-061 | 系统必须支持 Face ID 登录完成 | P1 |
| AUTH-062 | Face ID 挑战必须有 2 分钟过期和最大 3 次尝试限制（移动端弱网下 60 秒过短，见 design/auth/faceid.md §2.3） | P1 |

### 1.10 Passkey 无密码登录 [REQ:AUTH-063 ~ AUTH-066]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-063 | 系统必须支持 Passkey 注册 ceremony（WebAuthn create，origin 全等与 challenge 一次性校验） | P1 |
| AUTH-064 | 系统必须支持 Passkey 登录 ceremony（WebAuthn get，assertion 验签与 signCount 单调校验） | P1 |
| AUTH-065 | 注册完成后必须由 Passkey 公钥确定性派生 EVM 地址并写入 `web3_identity_mappings` | P1 |
| AUTH-066 | 注册与认证 challenge 必须一次性消费且 5 分钟过期；不得接受钱包签名登录 | P1 |

### 1.11 注册与密码管理 [REQ:AUTH-067 ~ AUTH-071]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-067 | 系统必须支持用户自注册（含组织级注册项配置） | P0 |
| AUTH-068 | 系统必须支持密码重置流程（验证码 + 新密码） | P0 |
| AUTH-069 | 系统必须支持密码强度检查 | P1 |
| AUTH-070 | 系统必须支持密码过期前 7 天警告 | P1 |
| AUTH-071 | 系统必须支持账号锁定策略（5次失败/15分钟窗口/锁定30分钟）；认证处理顺序固定为先判锁定状态再校验密码 | P0 |

### 1.12 协议能力补录 [REQ:AUTH-072 ~ AUTH-076]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-072 | 系统必须提供 SCIM 2.0 服务端（RFC 7643/7644），全族强制 Bearer 鉴权 | P0 |
| AUTH-073 | 系统必须支持 Kerberos/SPNEGO 协商登录 | P2 |
| AUTH-074 | 系统必须提供 Well-Known 协议发现端点族（逐文件公开，禁止整前缀豁免） | P0 |
| AUTH-075 | 系统必须提供验证码发送/校验/记录服务 | P0 |
| AUTH-076 | 系统必须支持 SAML Single Logout（SLO） | P1 |

### 1.12.1 国家网络身份认证（NCI，规划态）[REQ:AUTH-077 ~ AUTH-080]

> 本节定义 NCI 的目标态需求；运行期启用依赖外部网关准入与同批交付（适配器、声明、provider kind 条目与验收测试）。设计边界见 [network-identity.md](../design/auth/network-identity.md)。

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-077 | NCI 网关不可达、超时、5xx 或时钟偏移超过 3 秒时必须 fail-close：拒绝登录/核验，不建立会话、不回退密码登录，并写入审计 | P0 |
| AUTH-078 | NCI 身份材料必须去标识化：事实源不落明文姓名、证件号、人脸图像或原始报文；审计仅记录 `biz_seq`/`subject_ref`/`verify_result`/`gateway_code` | P0 |
| AUTH-079 | NCI 重复核验必须幂等：同一 `biz_seq` 一次性消费，并发或重放收敛为单条身份记录与单个登录会话，重放返回 `conflict` | P0 |
| AUTH-080 | NCI 登录与实名模式必须隔离：实名不建立会话、不下发 Cookie/令牌；`biz_seq` 绑定组织与应用，跨组织/应用/模式重放返回 `not_found` | P0 |

### 1.13 现代协议族与无密码认证 [REQ:AUTH-081 ~ AUTH-109]

> 本小节为商业化对标目标态需求，覆盖 OAuth 2.1 基线、PAR/JAR/JARM/CIBA/RAR、JWT 客户端认证、mTLS、DPoP nonce、DCR 管理、FAPI 2.0、Device Flow 增强、Token Exchange 委派语义、SAML 企业联邦深化、入站 OIDC 联邦、Passkey-first、Magic Link、HOTP、信任设备、step-up 增强认证、换绑冷静期、注册审批、通知偏好、HIBP 升级、OTP 渠道等。以下为已被 ADR/验收条件引用的逐号条目；未列出的编号为保留号（编号空间 AUTH-081 ~ AUTH-109）。

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| AUTH-093 | 系统必须支持 Token Exchange（RFC 8693）委派语义：subject_token 类型白名单、actor/act 链、换发 scope 只能收敛（ADR-026） | P1 |
| AUTH-094 | 系统必须支持入站 OIDC 联邦登录：外部 Issuer 注册、issuer/JWKS/aud 与 subject 白名单校验（ADR-028） | P1 |
| AUTH-095 | 系统必须支持企业 SAML 联邦深化：多 IdP 绑定、签名算法与元数据轮换（ADR-028） | P1 |
| AUTH-096 | 系统必须支持企业邮箱域归属：按已验证域名将用户自动归属组织（design/iam/organizations.md） | P1 |
| AUTH-097 | 系统必须支持邮箱域 SSO 发现：已验证且配置 SSO 的域自动路由到企业 IdP（ADR-028） | P1 |
| AUTH-098 | 系统必须支持多 IdP 联邦的租户级路由与失败回退，发现响应不泄露组织存在性（ADR-028） | P1 |
| AUTH-099 | 系统必须支持 Passkey-first 注册与登录：可同步 WebAuthn 凭据优先（design/auth/webauthn.md） | P1 |
| AUTH-100 | 系统必须支持 Passkey 跨设备与同步凭据策略配置（design/auth/webauthn.md） | P1 |
| AUTH-101 | 系统必须支持按 AAGUID 实施认证器白/黑名单与同步 passkey 策略（开发规范术语 AAGUID） | P1 |
| AUTH-102 | 系统必须支持无密码优先与受控回退：Passkey 不可用时回退 MFA/密码（design/auth/webauthn.md） | P1 |
| AUTH-104 | 系统必须支持信任设备：免 MFA 期可配置，撤销即时失效，设备指纹变化触发再验证并通知 | P1 |
| AUTH-105 | 系统必须支持 step-up 增强认证：敏感操作或风险升高时要求更强因素，令牌携带 acr/amr/auth_time（ADR-027） | P1 |

### 1.14 端到端流程图

以下以 **OAuth 2.0 Authorization Code + PKCE 登录** 为例描述端到端步骤：

1. **参与者**：终端用户（浏览器）→ **操作**：在业务应用点击「使用 GuliangIAM 登录」，浏览器被重定向到 `/oauth/authorize?client_id=<app_id>&redirect_uri=<回调>&code_challenge=<S256哈希>&code_challenge_method=S256&state=<防CSRF随机值>`
2. **系统**：GuliangIAM 接收请求，依次执行：校验 `client_id` 合法性 → 校验 `redirect_uri` 是否在应用白名单（精确匹配/通配符子域名）→ 校验应用状态（active）→ 生成 PKCE `authorization_code`（绑定 code_challenge、有效期 5 分钟、一次性）
   - **响应**：浏览器被重定向到 `redirect_uri?code=<授权码>&state=<原样回传>`
3. **参与者**：业务应用后端 → **操作**：在服务端用 `code_verifier` 向 `/oauth/token` 发起 POST（携带 `client_id`、`client_secret_basic`、`code`、`code_verifier`、`grant_type=authorization_code`）
4. **系统**：校验授权码有效性（未消费、未过期、code_challenge 匹配）→ 签发 RS256 Access Token（60 分钟）+ Refresh Token（30 天，rotation 首次）
   - **响应**：返回 JSON `{ access_token, refresh_token, token_type: "Bearer", expires_in: 3600 }`；同时创建服务端会话
5. **数据落盘**：`authorization_codes` 表标记已消费（`revoked_at`）；`refresh_tokens` 表写入新族 + 关联 `family_id`；`sessions` 表创建活跃会话（含 user_agent、ip、remember_device 标志）；`access_tokens` 表按需记录 revocation 条目
6. **审计记录**：`records` 表同事务写入事件 `protocol.token.issued`，包含 actor（用户 UUID）、target（client_id）、action、source_ip、user_agent、request_id、签发令牌类型；同时 Outbox 写入 `protocol.token.issued` 域事件供 SIEM 投递（事件名唯一源 `contracts/registries/events.yaml`，下同）

---

## 2. 组织与多租户 [REQ:MODULE-ORG]

核心要点：
- 组织为租户隔离边界，资源隔离分级（直接租户列 / 间接隔离 / 全局）
- 组织级安全策略、主题配置、注册项配置
- 租户分级套餐（shared / bridged / dedicated，ORG-016/019/020）

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| ORG-004 | 组织是租户隔离边界：租户资源一律按 `organization_id` 行过滤，树关系不改变数据面隔离（ADR-028） | P0 |
| ORG-008 | 系统必须支持组织级主题配置，并可声明子组织继承/覆盖/锁定（ADR-028） | P1 |
| ORG-009 | 系统必须支持组织级安全策略（会话 idle/absolute、密码、IP、限流），并可声明继承/覆盖/锁定（ADR-028） | P1 |
| ORG-016 | 系统必须支持 shared / bridged / dedicated 三档租户分级，dedicated 可指定区域（ADR-031） | P1 |
| ORG-019 | 系统必须支持套餐档位与隔离强度、限流配额、备份频率的映射（design/iam/organizations.md） | P1 |
| ORG-020 | 系统必须支持租户配额绑定套餐/计划，超限按显式声明策略处置（AC-METER-002） | P1 |

### 2.1 端到端流程图

以下以 **新租户开通 + 组织安全策略配置** 为例描述端到端步骤：

1. **参与者**：租户管理员（平台运营侧）→ **操作**：在平台控制台提交「开通租户」表单（输入租户名、套餐档位、区域归属、管理员邮箱、计费信息）
2. **系统**：校验租户名唯一性（跨组织全局）→ 依据套餐档位准备数据隔离拓扑（shared：行级过滤；bridged：独立 schema；dedicated：独立实例）→ 生成租户级默认安全策略模板（继承平台基线）
   - **响应**：返回租户 UUID + 管理员邀请链接（有效期 24 小时）+ 初始套餐信息
3. **参与者**：租户管理员 → **操作**：完成首次激活（邮箱验证 + 设置密码 + 配置密码策略最小长度 14 位 + 启用 TOTP）
4. **系统**：写入租户级 `organization_security_policies`（密码/会话/IP 黑白/速率限制参数）→ 热刷新 Casbin 租户 Enforcer（`r = sub, dom, obj, act`）
   - **响应**：返回策略更新成功确认 + 下次登录需 MFA 提示
5. **数据落盘**：`organizations` 表插入新行（含 `region_code`、套餐标识）；`organization_settings`、`organization_security_policies`、`organization_theme_settings` 三个 1:1 表同步插入；`users` 表插入租户管理员账户
6. **审计记录**：`records` 表写入事件 `organization.created`（actor=平台操作人、target=新租户 UUID）、`org.security_policy.updated`（diff JSON）；Outbox 写入 `control.tenant_registered` 供计量/计费链路消费

---

## 3. 用户管理 [REQ:MODULE-USER]

### 3.1 用户 CRUD [REQ:USER-001 ~ USER-005, USER-021]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| USER-001 | 系统必须支持用户 CRUD（含分页、搜索、排序） | P0 |
| USER-002 | 系统必须支持用户按组织过滤 | P0 |
| USER-003 | 系统必须支持用户 Excel 批量导入（excelize） | P1 |
| USER-004 | 系统必须支持用户 Excel 批量导出 | P1 |
| USER-005 | 系统必须支持用户模拟登录（Impersonate） | P2 |
| USER-021 | 系统必须支持邀请码注册（一次性/有效期/发送侧防护） | P0 |

### 3.2 用户身份与凭证 [REQ:USER-006 ~ USER-011]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| USER-006 | 系统必须支持多身份绑定（OAuth/SAML/WebAuthn/Metamask/Kerberos） | P0 |
| USER-007 | 身份必须满足 (provider_id, subject) 唯一约束 | P0 |
| USER-008 | 系统必须支持身份整体替换 | P1 |
| USER-009 | 系统必须支持密码凭证管理（哈希、盐值、算法、更新时间） | P0 |
| USER-010 | 系统必须支持 Access Key / Access Secret 轮换 | P1 |
| USER-011 | Access Secret 创建后仅返回一次，数据库只保存 hash | P0 |

### 3.3 MFA 方法与角色绑定 [REQ:USER-012 ~ USER-020]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| USER-012 | 系统必须支持 MFA 方法列表管理（initiate/verify/enable） | P0 |
| USER-013 | 系统必须支持首选 MFA 设置 | P0 |
| USER-014 | 系统必须支持 MFA 方法整体替换 | P1 |
| USER-015 | 系统必须支持用户角色绑定（多角色，整体替换） | P0 |
| USER-016 | 系统必须支持用户组成员（多组，整体替换和单个移除） | P0 |
| USER-017 | 系统必须支持管理员设置用户密码（独立端点） | P1 |
| USER-018 | 系统必须支持用户 WebAuthn 凭证列表和删除 | P1 |
| USER-019 | 系统必须支持用户交易记录查询 | P1 |
| USER-020 | 系统必须支持用户 OAuth 授权同意记录查询 | P1 |

### 3.4 端到端流程图

以下以 **租户管理员创建用户 + 绑定角色** 为例描述端到端步骤：

1. **参与者**：租户管理员（已认证，admin 角色）→ **操作**：调用 `POST /api/v1/users`（body：email、username、display_name、初始密码（若需要）、归属组织 UUID）
2. **系统**：Casbin 评估（`sub=admin_user, dom=tenant_uuid, obj=users, act=create`）→ 通过 → 校验 email/username 租户内唯一 → 若提供密码则用 Argon2id 生成哈希 + 随机盐
   - **响应**：返回新用户 UUID + 账户初始化状态；若走邀请流程则返回邀请码
3. **参与者**：租户管理员 → **操作**：调用 `PUT /api/v1/users/{uuid}/roles`（整体替换为 `[editor, reviewer]` 两个角色 UUID 列表）
4. **系统**：校验角色归属租户 → 事务内先删后写 `user_role_bindings` → 热刷新该租户 Enforcer
   - **响应**：返回绑定确认 + 最新角色列表
5. **数据落盘**：`users` 表插入新行；`user_profiles` 表同步插入（1:1）；`user_credentials` 表写入密码哈希/盐/算法/更新时间；`user_role_bindings` 表批量 UPSERT
6. **审计记录**：`records` 表写入 `user.created`（actor=admin、target=新用户 UUID、diff=用户字段摘要）和 `user.updated`（diff=旧→新角色列表）；Outbox 写入 `user.created` 供 SCIM 出站 Provisioning 消费

---

## 4. 应用与 OAuth 客户端 [REQ:MODULE-APP]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| APP-001 | 系统必须支持应用 CRUD | P0 |
| APP-002 | 系统必须支持 OAuth Client ID / Client Secret 管理 | P0 |
| APP-003 | 系统必须支持应用 by-client-id 查询 | P0 |
| APP-004 | 系统必须支持应用 org+app slug 解析 | P1 |
| APP-005 | 系统必须支持 Redirect URI 配置与安全校验（精确匹配、通配符子域名、localhost开发） | P0 |
| APP-006 | 系统必须支持 Grant Types 配置（authorization_code, client_credentials, refresh_token, device_code, token-exchange, jwt-bearer） | P0 |
| APP-007 | 系统必须支持 Token Endpoint Auth Method（client_secret_basic, client_secret_post, none） | P0 |
| APP-008 | 系统必须支持自定义 Scope 定义 | P1 |
| APP-009 | 系统必须支持 Provider 绑定（M:N 关系，整体替换） | P0 |
| APP-010 | 系统必须支持应用删除预检（返回关联资源摘要） | P1 |
| APP-011 | 系统必须支持应用自定义 CSS/JavaScript 注入 | P1 |
| APP-012 | 系统必须支持应用自定义登录页 URL | P1 |

### 4.1 端到端流程图

以下以 **注册新应用 + 配置 Redirect URI + 绑定 Provider** 为例描述端到端步骤：

1. **参与者**：应用管理员（app_admin 角色）→ **操作**：调用 `POST /api/v1/applications`（body：name、slug、organization_id、grant_types、redirect_uris=["https://myapp.example.com/callback"]、token_endpoint_auth_method="client_secret_basic"）
2. **系统**：Casbin 评估通过 → 校验 slug 租户内唯一 → 生成 Client ID（UUIDv7）+ Client Secret（32 字节随机，仅存 Argon2id 哈希，永不存可逆密文）→ 对 redirect_uris 做格式校验（精确匹配/通配符子域名/localhost 开发）
   - **响应**：返回应用 UUID + client_id + client_secret（仅本次返回明文，此后数据库只存 Argon2id 哈希，读接口按 `XxxSet` + `XxxMasked` 脱敏）
3. **参与者**：应用管理员 → **操作**：调用 `PUT /api/v1/applications/{uuid}/providers`，整体替换绑定为本地 Provider + Google OAuth Provider
4. **系统**：校验 Provider 归属当前租户 → 事务内更新 `application_provider_bindings` → 热刷新应用级策略
   - **响应**：返回最新绑定列表
5. **数据落盘**：`applications` 表插入应用主行；Client Secret 哈希落 `app_client_secrets.secret_hash`（Argon2id，Schema 事实源 `contracts/schema/20_identity.yaml`；OAuth 配置在 `app_oauth_config.client_secret_hash`）；`application_provider_bindings` 表批量 UPSERT；`keys` 表按需写入新应用级签名密钥引用
6. **审计记录**：`records` 表写入 `application.created`（actor、client_id、grant_types 列表）、`application.updated`（redirect_uris diff）、`application.provider_bindings.replaced`；Outbox 写入 `application.created` 供计量链路消费（事件名唯一源 `contracts/registries/events.yaml`）

---

## 5. Provider 与 IdP 集成 [REQ:MODULE-PROV]

### 5.1 Provider 基础 [REQ:PROV-001 ~ PROV-009]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| PROV-001 | 系统必须支持 Provider CRUD + 配置管理 | P0 |
| PROV-002 | 系统必须支持原生 IdP 集成（具名厂商/平台构造器 + `custom` 通用 OAuth；清单以代码构造器注册为准） | P0 |
| PROV-003 | 系统必须支持 Goth 通用 OAuth2 框架（供应商清单以代码注册为准） | P1 |
| PROV-004 | 系统必须支持验证码 Provider | P1 |
| PROV-005 | 系统必须支持存储 Provider（多后端适配器，清单以代码适配器注册为准） | P1 |
| PROV-006 | 系统必须支持支付 Provider（网关类型清单以代码适配器注册为准） | P1 |
| PROV-007 | 系统必须支持通知 Provider：邮件/短信/IM 推送 | P1 |
| PROV-008 | 系统必须支持 Face ID Provider 接口 | P1 |
| PROV-009 | 系统必须支持 Provider Secret 状态追踪与轮换 | P2 |

### 5.2 SCIM 出站 Provisioning [REQ:PROV-010 ~ PROV-011]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| PROV-010 | 系统必须支持 SCIM 出站连接器框架（连接器配置 + 事件订阅 + Outbox/Worker 统一投递） | P1 |
| PROV-011 | 系统必须支持 SCIM 出站调和（定期 diff + 漂移报告 + 一键修复） | P1 |

### 5.3 端到端流程图

以下以 **Google OAuth Provider 集成 + 测试连接 + 绑定到应用** 为例描述端到端步骤：

1. **参与者**：租户管理员 → **操作**：调用 `POST /api/v1/providers`（type="google_oauth2"、name="Google 登录"、tenant_id），随后调用 `PUT /api/v1/providers/{uuid}/config` 填入 client_id、client_secret（密文存储）
2. **系统**：校验 Provider 类型合法性 → 将 client_secret 用 AES-256-GCM 信封加密（根密钥由 KMS/Vault 管理）→ 写入 `provider_configs`（非敏感）+ `provider_secrets`（密文 + 指纹）
   - **响应**：返回 Provider UUID + 配置保存成功
3. **参与者**：租户管理员 → **操作**：调用 `POST /api/v1/providers/{uuid}/test-connection`
4. **系统**：组装 Google OAuth authorize URL → 生成测试用 state + 临时绑定 → 返回测试链接 → 等待管理员在浏览器完成 OAuth 流程后回跳 → 校验 `(provider_id, subject)` 唯一性
   - **响应**：测试成功（返回 subject、profile 摘要）或失败（含错误代码 + 建议）
5. **数据落盘**：`providers` 表插入；`provider_configs`、`provider_secrets` 表同步写入；测试成功时 `user_identities` 表绑定测试账号（若不存在则自动创建本地用户）
6. **审计记录**：`records` 表写入 `provider.identity.created`、`provider.identity.secret.rotated`（脱敏）；测试连接结果（结果 + 耗时）按《开发规范》§12.5 必填字段留审计痕迹；Outbox 写入 `provider.identity.created`

---

## 6. 授权与权限 [REQ:MODULE-PERM]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| PERM-001 | 系统必须支持 Casbin v3 RBAC/ABAC 权限模型 | P0 |
| PERM-002 | 系统必须支持多 Enforcer 架构（全局/组织/应用级隔离） | P0 |
| PERM-003 | 系统必须支持三层角色体系（admin/editor/viewer） | P0 |
| PERM-004 | 系统必须支持域隔离（每个组织独立策略空间） | P0 |
| PERM-005 | 系统必须支持策略热刷新（无需重启服务） | P0 |
| PERM-006 | 系统必须支持 Permission/Role/Model/Adapter/Enforcer 全链路 CRUD | P0 |
| PERM-007 | 系统必须支持权限执行 API（enforce, batch-enforce） | P0 |
| PERM-008 | 系统必须支持权限查询 API（objects, actions, roles） | P1 |
| PERM-009 | 系统必须支持策略管理 API（policies, update-policy, refresh-engines） | P1 |
| PERM-010 | 系统必须支持 API 级授权中间件（默认拒绝，白名单豁免，Casbin 评估失败时 Fail-Close） | P0 |
| PERM-011 | 策略格式：`r = sub, dom, obj, act`，角色继承 `g = _, _, _` | P0 |
| PERM-012 | 公开端点白名单实行两层规则（精确匹配 + 受控通配例外清单）；Well-Known 逐文件公开；匿名暴露禁令：SCIM 全族必须 Bearer 认证 | P0 |

### 6.1 端到端流程图

以下以 **自定义角色创建 + 权限绑定 + 用户角色分配 + 请求执行** 为例描述端到端步骤：

1. **参与者**：租户管理员 → **操作**：调用 `POST /api/v1/roles`（name="财务编辑"、organization_id），随后调用 `PUT /api/v1/roles/{uuid}/permissions` 整体替换绑定为 `[users:read, orders:read_write, payments:read]`
2. **系统**：校验角色名租户内唯一 → 事务内写入 `roles` + `casbin_rules`（p 行）→ 热刷新该租户 Enforcer（调用 `RefreshPolicy`，无需重启）
   - **响应**：返回角色 UUID + 最新策略摘要
3. **参与者**：租户管理员 → **操作**：调用 `PUT /api/v1/users/{uuid}/roles` 把某用户角色整体替换为 `["财务编辑"]`
4. **系统**：事务内更新 `user_role_bindings` → 写入 Casbin `g` 行（用户 → 角色继承）→ 热刷新
   - **响应**：返回绑定确认
5. **参与者**：该用户 → **操作**：调用 `GET /api/v1/orders/{uuid}`（请求头带 JWT Access Token）
6. **系统**：中间件提取用户 UUID → 解析租户归属 → 从请求上下文提取 Casbin Enforcer → 执行 `Enforce(sub=user_uuid, dom=tenant_uuid, obj=orders, act=read)` → 结果通过 → 执行业务查询
   - **响应**：返回订单 JSON
7. **数据落盘**：`roles`、`permissions`、`casbin_rules`、`user_role_bindings` 表按需写入；请求路径产生的访问日志写入 `entries` 表
8. **审计记录**：`records` 表写入 `role.created`、`role.updated`（权限绑定 diff）、`user.updated`（角色绑定 diff）；请求路径由安全中间件在 Casbin fail-close 时自动审计（`authz.deny`）

---

## 7. 商业与支付 [REQ:MODULE-COMM]

### 7.1 产品与订单 [REQ:COMM-001 ~ COMM-005]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| COMM-001 | 系统必须支持产品 CRUD | P1 |
| COMM-002 | 系统必须支持订单 CRUD + 业务操作 | P1 |
| COMM-003 | 订单状态机唯一口径见 generated/state-machine-registry.md §1.1 | P1 |
| COMM-004 | 系统必须支持优惠券 CRUD + 验证 + 使用计数 | P1 |
| COMM-005 | 系统必须支持用户订单查询 | P1 |

### 7.2 计划与订阅 [REQ:COMM-006 ~ COMM-009]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| COMM-006 | 系统必须支持计划 CRUD（Monthly/Yearly，独占标记） | P1 |
| COMM-007 | 系统必须支持定价 CRUD（关联应用和计划列表） | P1 |
| COMM-008 | 系统必须支持订阅 CRUD；状态机唯一口径见 generated/state-machine-registry.md §1.3 | P1 |
| COMM-009 | 订阅必须包含开始/结束时间 | P1 |

### 7.3 支付与交易 [REQ:COMM-010 ~ COMM-013]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| COMM-010 | 系统必须支持支付 CRUD + 异步通知 | P1 |
| COMM-011 | 系统必须支持交易 CRUD + 退款 | P1 |
| COMM-012 | 交易类型与状态唯一口径见 generated/state-machine-registry.md §1.4 | P1 |
| COMM-013 | 交易状态唯一口径见 generated/state-machine-registry.md §1.4 | P1 |

### 7.4 用量计量与配额 [REQ:METER-001 ~ METER-005]

> 核心指标：MAU、认证次数、签发令牌数；租户配额模型与套餐衔接。

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| METER-001 | 系统必须按租户/时段采集并聚合用量事件（MAU、认证次数、令牌签发数等 ≥3 项） | P1 |
| METER-002 | 计量口径以实际业务事实为准，失败执行不产生计量事件（ADR-029） | P1 |
| METER-003 | 计量事件必须去重，去重口径显式声明，事件不丢不重（Outbox/幂等同栈） | P1 |
| METER-004 | 系统必须按套餐/计划执行配额校验，超限按显式声明策略处置（AC-METER-002） | P1 |
| METER-005 | 计量事件跨区域汇聚必须去标识且不可逆（data.md §5.2） | P1 |

### 7.5 端到端流程图

以下以 **订单创建 → 支付回调 → 订阅激活** 为例描述端到端步骤：

1. **参与者**：租户管理员 → **操作**：在控制台选择套餐档位 + 计划（Monthly）+ 应用，点击「升级」，提交订单请求
2. **系统**：校验当前套餐状态 + 应用数量是否匹配 → 计算应付金额（含配额阶梯）→ 生成订单（状态机 `pending_payment`，状态机口径见 state-machine-registry.md §1.1）→ 创建支付记录（调用 Stripe/支付宝/微信 Pay Provider）
   - **响应**：返回订单 UUID + 支付二维码 / PaymentIntent 链接
3. **参与者**：租户管理员 → **操作**：扫码完成支付
4. **系统**：Payment Provider 向 `/api/v1/payments/callback` 发送异步通知（带 HMAC-SHA256 签名）→ GuliangIAM 验签通过 → 事务内更新订单状态（`paid`）+ 创建交易流水 + 创建/更新订阅（状态机 §1.3，`active`，开始/结束时间写入）
   - **响应**：返回 200 OK（幂等，同笔支付通知多次到达只处理首次）
5. **数据落盘**：`orders` 表状态推进 + `paid_at` 写入；`payments` 表记录支付流水；`transactions` 表追加写入；`subscriptions` 表插入/更新；全部同 Outbox 写入同事务
6. **审计记录**：`records` 表写入 `order.created`、`order.updated`（diff 前后状态）、`subscription.created`/`subscription.updated`；Outbox 写入同名事件供计量/计费/通知链路消费

---

## 8. 基础设施与集成 [REQ:MODULE-INFRA]

### 8.1 Webhook [REQ:INFRA-001 ~ INFRA-006, INFRA-021 ~ INFRA-022]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| INFRA-001 | 系统必须支持 Webhook CRUD（URL、事件订阅、密钥签名） | P0 |
| INFRA-002 | 系统必须支持 Webhook 投递工作器（后台 goroutine） | P1 |
| INFRA-003 | 系统必须支持 HMAC-SHA256 签名验证 | P1 |
| INFRA-004 | 系统必须支持指数退避重试（重试上限与退避参数以 `contracts/registries/outbox.yaml` 为唯一源） | P1 |
| INFRA-005 | 系统必须支持 Webhook Event 生命周期追踪 | P1 |
| INFRA-006 | 系统必须支持 Webhook Event 重放 | P1 |
| INFRA-021 | 系统必须支持 Webhook ContentType 和 CustomBody 自定义请求体 | P2 |
| INFRA-022 | 领域事件扇出（DomainEventOutboxHandler）必须通过创建 WebhookEvent 记录统一由 Outbox Worker 投递（ADR-008 outbox 单轨） | P1 |

### 8.2 Dashboard [REQ:INFRA-007 ~ INFRA-010]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| INFRA-007 | 系统必须支持 Dashboard 统计（专用 SQL COUNT） | P1 |
| INFRA-008 | 系统必须支持 Provider 分布统计（SQL GROUP BY） | P2 |
| INFRA-009 | 系统必须支持 MFA 覆盖率统计（SQL FILTER） | P2 |
| INFRA-010 | 系统必须支持登录热力图统计（按小时聚合） | P2 |

### 8.3 Syncer 与 LDAP [REQ:INFRA-011 ~ INFRA-014]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| INFRA-011 | 系统必须支持数据库同步器 CRUD + 运行 + 测试连接 | P1 |
| INFRA-012 | 系统必须支持 LDAP 配置 CRUD + 内置 LDAP 服务器 | P1 |
| INFRA-013 | 系统必须支持 LDAP 用户预览（含本地存在状态） | P1 |
| INFRA-014 | 系统必须支持 LDAP 同步（UUID-based + 选择性 + 自动调度） | P1 |

### 8.4 网关与其他 [REQ:INFRA-015 ~ INFRA-020]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| INFRA-015 | 系统必须支持 Gateway Server/Site CRUD | P2 |
| INFRA-016 | 系统必须支持 Agent CRUD | P2 |
| INFRA-017 | 系统必须支持 MCP Server CRUD | P2 |
| INFRA-018 | 系统必须支持 Form CRUD | P2 |
| INFRA-019 | 系统必须支持审计记录查询与搜索 | P0 |
| INFRA-020 | 系统必须支持验证记录查询 | P1 |

### 8.5 端到端流程图

以下以 **Webhook 订阅 + 事件投递 + 重试** 为例描述端到端步骤：

1. **参与者**：应用管理员 → **操作**：调用 `POST /api/v1/webhooks`（url="https://myapp.example.com/hook"、events=["protocol.token.issued", "user.created"]、签名密钥）
2. **系统**：Casbin 评估通过 → 校验 URL 白名单（禁止内网 IP/localhost 生产）→ 生成签名密钥（AES-256-GCM 加密存储）→ 创建 WebhookEvent 模板
   - **响应**：返回 Webhook UUID + 测试 Webhook 链接
3. **参与者**：终端用户完成 OAuth 登录 → **系统事件**：领域事件扇出（DomainEventOutboxHandler）消费 `protocol.token.issued` → 事务内写入 Outbox + 写入 `webhook_events`（状态 `pending`，与业务事实同事务）
4. **投递 Worker**：每 500ms 轮询 Outbox → 取出 Webhook 类事件 → 组装 JSON payload → 添加 HMAC-SHA256 签名头 → HTTP POST 到订阅 URL → 收到 2xx 置为 `succeeded`，非 2xx/超时 → 进入重试队列（指数退避；重试上限与退避参数以 `contracts/registries/outbox.yaml` 为唯一源，max_attempts=12），超过置为 `dead`
   - **响应**：更新 `webhook_events` 状态为 `succeeded` / `retrying` / `dead`
5. **数据落盘**：`webhooks` 表插入配置；`webhook_events` 表追加事件记录（状态机：pending → succeeded/retrying → dead）；`outbox` 表写入投递事件
6. **审计记录**：`records` 表写入 `webhook.created`、`webhook_event.failed`（耗时 + 状态 + HTTP 响应码）；失败重试次数、最终 dead 事件触发告警

---

## 9. 安全与防护 [REQ:MODULE-SEC]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| SEC-001 | 系统必须支持 IP 黑白名单（Allow 优先于 Block） | P0 |
| SEC-002 | 系统必须支持 CIDR 匹配、精确匹配、范围匹配 | P0 |
| SEC-003 | 系统必须支持分层速率限制；限流计数必须分布式一致（Redis/Lua，ADR-010）；限流参数唯一口径见 generated/limit-registry.md | P0 |
| SEC-004 | 系统必须支持 CSRF 双提交 Cookie 防护 | P0 |
| SEC-005 | 系统必须支持 WAF/Rule Engine（IP/UA/Rate/WAF 规则类型） | P1 |
| SEC-006 | 系统必须支持 RADIUS 服务器（可选，:1812） | P2 |
| SEC-007 | 系统必须支持 RADIUS 计费配置 | P2 |
| SEC-008 | 系统必须支持 Key 管理（OAuth/API/Encryption/Signing 类型，加密存储，过期时间） | P1 |
| SEC-009 | 系统必须支持 Captcha Provider 配置管理 | P1 |
| SEC-010 | 系统必须支持 Notification Provider 配置管理 | P1 |
| SEC-011 | 系统必须支持 Face ID Provider 配置管理 | P1 |
| SEC-012 | 系统必须对验证码/OTP 发送侧实施频率限制：按手机号/邮箱、来源 IP、租户三维度；计数采用分布式实现（Redis/Lua），与 SEC-003 共用 ADR-010 设施 | P0 |

### 9.1 端到端流程图

以下以 **账号锁定 + IP 黑名单拦截 + 速率限制联动** 为例描述端到端步骤：

1. **参与者**：攻击者（IP 1.2.3.4）→ **操作**：对 `/login/password` 发起撞库攻击（同一账号 15 秒内 6 次密码错误）
2. **系统**：认证顺序固定为先判锁定 → `lockout_attempts` 表单条 `INSERT ... ON CONFLICT` 原子递增失败计数（`internal/infra/postgres/lockoutattempts/sql.go`）→ 达到阈值 5 → 同表写入锁定状态与到期时间 now + 30min
   - **响应**：返回通用错误（"账号或密码错误"，防枚举），P99 < 50ms
3. **参与者**：租户管理员 → **操作**：在控制台把 1.2.3.4 加入 IP Block 列表（CIDR 精确匹配）
4. **系统**：热刷新 IP 规则缓存（Redis）→ 该租户所有请求路径在第一层 middleware 命中 block → 直接返回 403
   - **响应**：拦截确认
5. **数据落盘**：`lockout_attempts` 表写入锁定事件（计数器 + 到期时间）；`ip_rules` 表追加新 rule；Redis 写入 `ip_block:tenant_uuid:cidr` 热缓存（TTL 跟随规则有效期）
6. **审计记录**：`records` 表写入账号锁定审计（actor=用户 UUID、attempts=5、来源 IP）与 IP 黑名单新增审计；Outbox 写入风险联动事件供风险引擎消费（锁定/IP 规则类事件名以 `contracts/registries/events.yaml` 登记为准，新增事件必须先登记）；WAF/SecureOps 仪表盘自动点亮告警

---

## 10. 前端管理控制台 [REQ:MODULE-FE]

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| FE-001 | 系统必须提供管理页面覆盖所有后端域 | P0 |
| FE-002 | 系统必须支持暗色模式（light/dark/system） | P1 |
| FE-003 | 系统必须支持 i18n（14 种语言，含独立 auth 命名空间） | P1 |
| FE-004 | 系统必须支持前端权限控制（usePermissions + PermissionGuard） | P0 |
| FE-005 | 系统必须支持会话空闲（idle）超时提醒（默认 30 分钟，组织可覆盖） | P1 |
| FE-006 | 系统必须支持 401 全局拦截 + 并发去重 | P0 |
| FE-007 | 系统必须支持主题编辑器 | P2 |
| FE-008 | 系统必须支持个人中心页面 | P1 |
| FE-009 | 系统必须支持系统信息页面 | P2 |
| FE-010 | 系统必须支持所有登录方式的前端组件 | P0 |
| FE-011 | 系统必须支持文件上传功能（头像、资源） | P1 |
| FE-012 | 系统必须支持可折叠侧边栏导航 | P1 |
| FE-013 | 系统必须支持统一的全局样式规范 | P2 |
| FE-014 | 系统必须提供 CAS 单点登出控制台入口 | P1 |
| FE-015 | 系统必须提供会话 transcript 查看器（服务端脱敏） | P2 |
| FE-016 | 系统必须提供独立支付二维码页 | P1 |
| FE-017 | 系统必须支持密码重置接受应用上下文（品牌与展示策略） | P1 |

### 10.1 端到端流程图

以下以 **管理员登录控制台 → 权限守卫渲染页面 → 会话过期自动跳转** 为例描述端到端步骤：

1. **参与者**：租户管理员 → **操作**：访问控制台登录页 → 输入邮箱/密码 → 完成 TOTP MFA → 浏览器获得 HttpOnly 会话 Cookie
2. **前端**：加载入口 → `auth` store 恢复会话 → 调用 `GET /api/v1/permissions/me` 拉取当前用户权限集 + 角色列表 → 渲染侧边栏导航（按权限过滤菜单项）
3. **参与者**：管理员 → **操作**：点击「用户管理」侧边栏路由 → 前端 `PermissionGuard` 组件检查 `usePermissions().can("users:read")` → 通过 → 懒加载路由模块
4. **系统**：API 层 Casbin 评估 → 通过 → 返回用户列表 JSON → 前端表格渲染 + 分页 + 搜索（i18n 14 种语言）
5. **参与者**：管理员闲置 31 分钟 → **前端 idle 计时器** 触发 → 弹出 2 分钟倒计时提醒 → 无响应 → 前端发起 `POST /api/v1/sessions/{id}/revoke` 注销 → 401 全局拦截器拦截所有并发请求 → 自动跳回登录页（附带原路由重定向参数）
6. **数据落盘**：`sessions` 表 `revoked_at` 写入；`entries` 表记录 `admin_login` + `admin_logout`
7. **审计记录**：`records` 表写入 `auth.session.revoke`（actor、session_id、reason="idle_timeout"）

---

## 11. 系统接口需求

### 11.1 协议端点

路由声明与 OpenAPI 契约保持一致，门禁由 `scripts/gen_openapi.py --check` 执行。

### 11.2 外部接口

| 外部接口 | 协议 |
|----------|------|
| GitHub OAuth | HTTPS/OAuth 2.0 |
| Google OAuth | HTTPS/OAuth 2.0 |
| Stripe Payment | HTTPS/REST |
| LDAP | LDAP/LDAPS |
| S3 Storage | HTTPS/S3 API |

### 11.3 端到端流程图

以下以 **SCIM 2.0 GET /Users（出站 Provisioning + SCIM 入站）** 为例描述端到端步骤：

1. **参与者**：外部 HR 系统（绑定 SCIM 出站连接器的 GuliangIAM）→ **操作**：调用 `GET /scim/v2/Users?filter=userName eq "jane@example.com"`，携带 `Authorization: Bearer <scim_token>`
2. **系统**：Casbin 评估 `scim_token` 绑定的 Service Account → 解析 filter → 组装 SQL WHERE `LOWER(user_profiles.username) = LOWER($1)` → 执行分页查询
   - **响应**：返回 RFC 7644 合规 SCIM User JSON（含 schemas、id、meta.etag、attributes）
3. **场景二**：某外部 IdP 发起 SCIM POST 创建新用户 → GuliangIAM SCIM 服务端接收 → 校验 Bearer Token → 映射 SCIM attributes → 写入本地 `users` + `user_profiles` + `user_identities`
   - **响应**：201 Created + 生成的 SCIM User 资源
4. **数据落盘**：SCIM 入站：`users`、`user_profiles`、`user_identities` 表写入；出站：Outbox 写入 `user.created` 供 Provisioning 连接器消费（ADR-008）
5. **审计记录**：`records` 表写入 `user.created`（入站创建，actor=Service Account UUID、request_id）；SCIM 读取请求按《开发规范》§12.5 必填字段留审计痕迹（专用 scim.* 事件域未登记，新增须先登记 `contracts/registries/events.yaml`）；Outbox 事件与业务事实同事务写入

---

## 12. 关系与属性访问控制（ReBAC/ABAC）[REQ:MODULE-REBAC]

核心要点：
- 关系元组模型 `(user, relation, object)`，对象带类型与租户归属
- Casbin 负责平台/租户级粗粒度策略执行，ReBAC 负责对象级细粒度共享与层级继承，二者职责不重叠
- Zookie 类一致性快照令牌、对象级共享、组织树继承、组嵌套

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| REBAC-001 | 系统必须支持关系元组 `(user, relation, object)` 的对象级授权（ADR-024） | P1 |
| REBAC-002 | 系统必须提供 OpenFGA 兼容 API：check / write / expand / batch check / list-objects | P1 |
| REBAC-003 | 系统必须支持对象树继承，展开深度受控并进行环检测 | P1 |
| REBAC-004 | 组嵌套复用既有 `groups` 表作为主体来源，元组侧只存 `subject_relation` | P1 |
| REBAC-005 | 系统必须提供 Zookie 一致性令牌，保证写后读快照一致 | P1 |
| REBAC-006 | 系统必须提供 Why 溯源，解释授权路径与命中关系 | P1 |
| REBAC-007 | 系统必须支持 Watch 实时元组变更订阅（进程内事件流，不新增中间件） | P1 |
| REBAC-008 | 决策顺序固定：Casbin 先行 → ReBAC 叠加，ReBAC deny 覆盖 allow，不可用时 fail-close | P1 |
| REBAC-010 | 授权模型（命名空间与关系定义）必须 JSON 化、版本化，变更走迁移门禁 | P1 |

### 12.1 端到端流程图

以下以 **对象级共享（文档 → 用户可读）+ Zookie 快照令牌** 为例描述端到端步骤：

1. **参与者**：文档所有者（object=doc:project_alpha）→ **操作**：调用 `POST /api/v1/rebac/namespaces/{namespaceId}/tuples`（写入 tuple `(user:jane, viewer, doc:project_alpha)`；命名空间先通过 `POST /api/v1/rebac/namespaces` 创建）
2. **系统**：校验对象归属租户 → 校验用户存在 → 写入 `relation_tuples` 表 → 分配 Zookie（单调递增序列号）→ 刷新本地快照缓存
   - **响应**：返回 Zookie token（快照一致性保证）
3. **参与者**：Jane → **操作**：调用 `GET /api/v1/docs/project_alpha`（请求头带 Zookie token）
4. **系统**：ReBAC 引擎以 Zookie 指定的快照视图加载 tuples → 从 `(user:jane, *, doc:project_alpha)` 关系计算可达性 → 找到 `viewer` 关系 → 放行
   - **响应**：返回文档内容
5. **数据落盘**：`relation_tuples` 表写入（`revision_counter` 作为 Zookie 序列）；`relation_namespaces` 计数递增
6. **审计记录**：`records` 表写入 `rebac.tuple.created`（actor=所有者、tuple 三要素、zookie）、`authz.allow`（ReBAC 放行）；请求路径由 ReBAC 引擎在 access denied 时自动审计（`authz.deny`）

---

## 13. 身份治理与管理（IGA/PAM）[REQ:MODULE-IGA]

核心要点：
- 访问认证 Campaign、访问请求工单与多级审批、JIT/JEA 时间窗授权、到期自动回收
- SoD 职责分离互斥校验（申请时 + 角色变更时双时机）
- 特权凭据保险柜、审批式提权、特权会话录制（PAM）
- Impersonate（USER-005）定位为支持/排查用途的受控模拟，不属于 PAM 范畴

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| IGA-001 | 系统必须支持访问评审 Campaign：scope 模板、评审人、due_at、逾期处置与审计联动 | P1 |
| IGA-002 | 系统必须支持访问请求多级审批矩阵，复用平台既有审批/通知设施 | P1 |
| IGA-003 | 系统必须支持 JIT/JEA 时间窗授权，令牌/会话可区分常态与临时权限 | P1 |
| IGA-004 | 系统必须提供到期自动回收 Worker：幂等、分批、失败重试并告警 | P1 |
| IGA-005 | 系统必须支持 SoD 互斥校验：申请时与角色变更时双时机，覆盖 SCIM/同步器写入路径 | P1 |
| IGA-007 | 系统必须提供审计员只读视角与可独立验证的证据包导出 | P1 |
| IGA-008 | 系统必须支持特权凭据保险柜与审批式提权（PAM），与 Impersonate 分域 | P1 |
| IGA-010 | 系统必须支持特权会话隔离与录制，提权限时且到期自动回收 | P1 |

### 13.1 端到端流程图

以下以 **访问请求工单 → 多级审批 → JIT 时间窗授权 → 到期自动回收** 为例描述端到端步骤：

1. **参与者**：普通员工 A（请求者）→ **操作**：在 IGA 工作台提交访问请求（目标角色 `财务编辑`、理由="月度结算临时支援"、期望时间窗 `2026-10-15 09:00 ~ 18:00`）
2. **系统**：SoD 互斥校验（当前角色是否与 `财务编辑` 冲突）→ 通过 → 创建审批工单 `tickets`（状态机以 `contracts/registries/state-machines.yaml` 注册为准：open → pending → resolved/closed）；JIT 授权生命周期由 `access_lifecycle` 承载（`contracts/schema/80_governance.yaml` 已申报，status：active/expired/revoked）
   - **响应**：返回工单 UUID + 审批链路预览
3. **参与者**：部门经理 → **操作**：审批通过 → **SAP 合规专员**：审批通过
4. **系统**：JIT 时间窗授权 → 在 Casbin Enforcer 写入带过期时间的临时 policy（`e = _, _, _, _, expiration=2026-10-15T18:00:00Z`）→ `user_role_bindings` 表记录临时绑定（source="iga_jit"、expires_at）→ `access_lifecycle` 写入授权行（starts_at/expires_at、status=active）
   - **响应**：工单转 `resolved`，通知请求者
5. **参与者**：JIT 到期后 → **系统 Cron**：扫描 `expires_at < now AND source="iga_jit"` 绑定 → 自动删除 Casbin 临时 policy + 清理 `user_role_bindings` → `access_lifecycle` 状态置 `expired`/`revoked` → 触发强制下线（若用户在线）
6. **数据落盘**：`tickets` + `ticket_messages` 表记录审批流；Casbin `casbin_rules` 表写入临时 e 行；`user_role_bindings` 表带 `expires_at`；`access_lifecycle` 记录 JIT 授权生命周期；到期回收由清理器执行
7. **审计记录**：`records` 表写入 `iga.ticket.approved`（actor=两审批人）/`iga.ticket.rejected`、`iga.jit.granted`、`iga.jit.revoked`/`iga.jit.expired`（事件名唯一源 `contracts/registries/events.yaml`；工单提交审计按《开发规范》§12.5 必填字段留痕）；全程留存工单 transcript

---

## 14. 身份风险与零信任 [REQ:MODULE-RISK]

核心要点：
- 自适应风险认证（设备指纹、IP 信誉、地理速度、撞库信号）
- 风险决策与 step-up/MFA 联动（medium → step-up、high → 完整 MFA、critical → 拒绝）
- 会话并发策略、管理员强制下线
- 零信任持续访问评估（CAEP/SSF 事件下推、RISC 事件消费）
- 审计证据哈希链、WORM 存储与签名导出

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| RISK-001 | 系统必须采集设备指纹、IP 信誉、地理速度与撞库信号，并生成可解释的风险决策 | P1 |
| RISK-005 | 系统必须按 low/medium/high/critical 四级映射放行/step-up/MFA 加固/拒绝并通知 | P1 |
| RISK-006 | 风险规则必须版本化、可灰度、可回滚，规则变更全程审计 | P1 |
| RISK-007 | 系统必须支持会话并发策略，并在超限时按声明策略处置 | P1 |
| RISK-008 | 系统必须支持管理员强制下线并即时吊销会话与 refresh family | P1 |
| RISK-009 | 系统必须支持会话内持续访问评估，结果可降级或吊销会话 | P1 |
| RISK-010 | 系统必须支持 CAEP/SSF 事件下推（RFC 8935/8936）至已注册接收方 | P1 |
| RISK-012 | 系统必须提供吊销传播时延指标并纳入 SLO 监控 | P1 |
| RISK-013 | 审计记录必须构建哈希链（prev_hash/entry_hash/chain_seq），可发现单点篡改（ADR-031） | P0 |
| RISK-014 | 系统必须支持链锚定与 WORM 归档，证据包可独立验证（ADR-031） | P0 |

### 14.1 端到端流程图

以下以 **自适应风险认证（新设备 + 异常 IP）→ step-up MFA** 为例描述端到端步骤：

1. **参与者**：终端用户（常用设备在上海，本次从北京新设备发起登录）→ **操作**：输入用户名/密码登录
2. **系统**：密码校验通过 → 进入风险引擎：设备指纹未见过（新设备）、IP 归属地与历史 90 天偏离 > 500km（地理速度异常）、IP 信誉查询 → 综合评分 → 决策 `risk_level=high`
3. **系统**：基于风险决策 → 触发 step-up → 返回 challenge 响应，要求完整 MFA（TOTP + 短信验证二选一，或 WebAuthn）
   - **响应**：返回 `auth_challenge` 状态 + MFA 选项列表 + 挑战 UUID
4. **参与者**：用户完成 TOTP → **操作**：提交 TOTP 码
5. **系统**：校验 TOTP → 通过 → 标记设备为"记住"（可选）→ 完成登录 → 签发令牌
6. **数据落盘**：`risk_signals` 表写入原始信号（设备指纹、IP 信誉、地理速度）；`login_challenges` 表记录 MFA 挑战状态机（pending → approved）；`sessions` 表创建（含 risk_level、remember_device）
7. **审计记录**：`records` 表写入 `risk.evaluation.completed`（actor、信号摘要、决策、step-up 类型）、`auth.step_up.completed`；风险事件同时 Outbox 写入供 SOC 仪表盘消费；整个登录链路纳入审计哈希链（后续 WORM 归档）

---

## 15. B2B 组织与域名治理 [REQ:MODULE-B2B]

核心要点：
- 多级组织树（`parent_id` 自引用，数据面隔离语义不变）
- 策略继承规则（子组织继承/覆盖/锁定三模式）
- 委派管理员、企业域名认领（DNS TXT 状态机）、邮箱域 SSO 发现、Vanity 托管登录域名

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| B2B-001 | 系统必须支持多级组织树：`organizations.parent_id` 自引用、深度受控、环与孤儿节点被拒 | P1 |
| B2B-002 | 系统必须支持策略继承三模式：子组织继承/覆盖/锁定，父级锁定项不可修改 | P1 |
| B2B-003 | 系统必须支持委派管理员：权限按子树作用域过滤，授予/收回审计，跨子树越权被拒 | P1 |
| B2B-004 | 系统必须支持企业域名认领：一次性 TXT 值、DNS 校验状态机、全局唯一归属与续验 | P1 |
| B2B-005 | 系统必须支持邮箱域 SSO 发现：已验证域自动路由企业 IdP，响应不泄露组织存在性并限流 | P1 |
| B2B-006 | 系统必须支持 Vanity 托管登录域名：先认领、ACME 证书自动化、主机头校验防混淆 | P1 |

### 15.1 端到端流程图

以下以 **企业域名认领 + DNS TXT 验证 + 邮箱域 SSO 发现** 为例描述端到端步骤：

1. **参与者**：父组织管理员 → **操作**：调用 `POST /api/v1/organizations/{uuid}/domains`（domain="corp.example.com"），认领企业域名
2. **系统**：生成一次性 DNS TXT 记录值（格式 `GuliangIAM-Verification=v1:<token>`，有效期 72 小时）→ 创建域名验证记录（状态机：pending → verified / failed）
   - **响应**：返回 TXT 值 + 验证状态 `pending`
3. **参与者**：父组织 DNS 管理员 → **操作**：在 DNS 控制台添加 `_guliangiam-verification.corp.example.com` TXT 记录
4. **系统 Cron**：每 2 分钟对 `pending` 状态域名执行 DNS 查询 → 匹配成功 → 状态转 `verified` → 自动启用邮箱域 SSO 发现（任何使用 `@corp.example.com` 邮箱的用户登录时自动路由到本组织 IdP）
   - **响应**：下次用户登录该邮箱时自动触发域发现
5. **参与者**：使用 `@corp.example.com` 邮箱的用户（首次登录）→ **操作**：输入邮箱 → **系统** 执行 domain discovery → 匹配已验证域名 → 显示 SSO 入口
6. **数据落盘**：`organization_domains` 表插入（domain、verification_token、status、verified_at）；DNS 验证记录写入 Outbox 供事件总线
7. **审计记录**：`records` 表写入 `b2b.domain.claimed`、`b2b.domain.verified`（actor、域名、验证耗时）；域名验证结果同步到控制面租户注册表

---

## 16. 工作负载身份（WLI）[REQ:MODULE-WLI]

核心要点：
- Service Account 一等主体、OIDC 联邦信任、短期凭据策略、身份池
- 与 client_credentials / Token Exchange 合流
- SPIFFE 兼容方向

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| WLI-001 | 系统必须将 Service Account 作为非人主体一等公民：独立生命周期、属主、可绑定角色与组、审计可区分 | P1 |
| WLI-002 | 系统必须支持 OIDC 联邦信任：issuer/JWKS/aud 与 subject 白名单校验，配置变更双审 | P1 |
| WLI-003 | 系统必须支持短期凭据：联邦换发令牌默认 ≤ 1 小时，静态 key 仅审批保留并强制轮换 | P1 |
| WLI-004 | 系统必须支持身份池：外部联邦身份映射到平台角色/权限集并作为授权出口 | P1 |
| WLI-005 | 系统必须支持 Token Exchange（RFC 8693）作为联邦换发通道，subject 类型白名单与 actor/act 链 | P1 |
| WLI-006 | 系统必须采用可映射 `spiffe://` 的命名构造并文档化映射规则（SVID 验证为契约预留） | P1 |

### 16.1 端到端流程图

以下以 **Service Account 创建 + OIDC 联邦令牌换取（Token Exchange RFC 8693）** 为例描述端到端步骤：

1. **参与者**：系统运维 / 租户管理员 → **操作**：先在身份池下创建 Service Account：调用 `POST /api/v1/identity-pools/{poolId}/service-accounts`（name="order-service"，绑定角色 `orders:read_write`；身份池通过 `POST /api/v1/identity-pools` 预先创建）
2. **系统**：Casbin 评估通过 → 创建 Service Account（生成 Client ID + 一次性 Client Secret）→ 创建短期凭据策略（access_token_ttl=15min、refresh_token 禁用、scope 限制）
   - **响应**：返回 Workload Identity UUID + Client ID + Client Secret（仅一次返回明文）
3. **参与者**：order-service（K8s Pod）→ **操作**：调用 `/oauth/token`（grant_type=client_credentials、client_id、client_secret、scope="orders:read_write"）
4. **系统**：校验 Client Secret → 签发 RS256 Access Token（15 分钟有效期、包含 SPIFFE 兼容 `sub=spiffe://guliangiam/tenant/{uuid}/workload/order-service`、`aud`、`scope`）
   - **响应**：返回 Access Token + expires_in=900
5. **参与者**：order-service → **操作**：携带 Access Token 调用 `/api/v1/orders/{uuid}`
6. **系统**：API 层 JWT 校验（RS256、aud、iss、exp）→ Casbin Enforce（sub=workload_uuid、dom=tenant_uuid、obj=orders、act=read）→ 通过
7. **数据落盘**：`users` 表插入 Service Account（`type=service_account`）；`user_credentials` 表写入 Client Secret 哈希；`user_role_bindings` 写入角色绑定；令牌签发过程按需写入 `access_tokens` revocation 表
8. **审计记录**：`records` 表写入 `service_account.created`（actor、workload_uuid、scope）、`protocol.token.issued`（workload、令牌类型、scope）；凭据使用审计按《开发规范》§14.8 全链口径落 `records`（who / where / when 可查询）

---

## 17. 扩展与开发者体验（EXT）[REQ:MODULE-EXT]

核心要点：
- Identity Actions/Hooks 事件挂钩（≥5 类触发点）、沙箱化执行、密钥注入与出站白名单
- 邮件/短信模板中心、用户自定义 Schema、SIEM 投递连接器
- Go/Java/Node 三语言服务端 SDK、React/Vue 嵌入式登录组件、Quickstart 与 API 调试台
- 外部 IAM 迁移工具与连接器（Auth0/Keycloak/Casdoor）

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| EXT-001 | 系统必须提供五类同步触发点（pre-login/pre-token/post-registration/consent/MFA），可返回 continue/deny/modify | P1 |
| EXT-002 | 系统必须对 Action 实施资源限额（CPU/内存/执行时长），超限按触发点失败策略处置 | P1 |
| EXT-003 | Action 出站调用必须经代理白名单，密钥运行时注入，越界阻断并审计（ADR-029） | P1 |
| EXT-004 | 安全类触发点默认 fail-close；非安全类可 fail-open，但失败必须留审计与告警 | P1 |

### 17.1 端到端流程图

以下以 **Identity Action 注册（post_registration 触发点，非安全类可 fail-open）+ WASM 沙箱执行 + 出站白名单** 为例描述端到端步骤（触发点集合与失败策略以 ADR-029 与《开发规范》§5.7 为准）：

1. **参与者**：应用开发者 → **操作**：调用 `POST /api/v1/actions`（name="注册后推送 Slack 通知"、trigger="post_registration"、code="<编译为 WASM 的 Slack Webhook 调用逻辑>"；执行沙箱/出站白名单为设计目标，运行期字段随 OPEN-DESIGN-06 收敛）
2. **系统**：Casbin 评估通过 → 注册 Action 与版本（WASM 优先，运行时 wazero；CPU/内存/执行时长限额以《开发规范》§5.7 与 ADR-029 为准）→ 校验出站白名单 → 配置租户级密钥引用（密文存储，运行时注入解密，见 EXT-003）
   - **响应**：返回 Action UUID + 激活状态
3. **参与者**：终端用户完成注册 → **系统事件**：注册事务提交后触发 post_registration Action（非安全类触发点，允许 fail-open）
4. **Action 执行引擎**：启动 WASM 沙箱实例 → 注入 payload（user_uuid、client_id、tenant_id、request_id）→ 执行代码 → 经代理白名单调用 Slack Webhook → 超出执行时长/资源限额即终止 → 捕获输出
   - **响应**：执行结果（成功/失败/超时）；post_registration 失败不阻塞主链路（fail-open），但必须留审计与告警（EXT-004）
5. **数据落盘**：`identity_actions` + `action_versions` 表注册 Action 与版本；执行日志表记录每次执行（耗时、status、stdout、stderr、入站出站 HTTP 元数据；当前登记名 `execution_logs`，ADR-029 目标名 `identity_action_executions`，以 `contracts/schema` 为准）
6. **审计记录**：`records` 表写入 `actions.execution.succeeded`/`actions.execution.failed`（事件名以 `contracts/registries/events.yaml` 登记为准；actor、action_uuid、status、latency_ms、request_id 按《开发规范》§12.5 必填字段留痕）；失败触发告警但不阻塞主注册链路

---

## 角色权限矩阵

> 行：角色；列：主要功能模块；单元格：权限粒度（R=读 / W=写 / A=管理（CRUD + 策略 + 删除） / -=无）。矩阵为权限边界的基线口径，实际策略可按租户套餐/组织层级收紧，但不得突破此处授予的最大权限。

| 角色 \ 功能模块 | 用户管理 | 组织管理 | 应用管理 | 角色权限 | 认证授权 | MFA | 审计 | 商业化 |
|----------------|---------|---------|---------|---------|---------|-----|------|-------|
| **租户管理员** | A | A | A | A | A | A | A | A |
| **应用管理员** | R（本应用关联用户） | - | A（本应用） | R/W（本应用内自定义角色） | A（本应用协议配置） | R/W（本应用相关） | R（本应用审计） | R/W（本应用配额） |
| **普通用户** | R/W（本人） | - | - | R（本人角色） | R（本人会话/令牌） | W（本人 MFA 方法） | - | R（本人订单/订阅） |
| **只读用户** | R（本人） | - | - | - | R（本人会话只读） | - | - | R（本人订阅状态） |
| **系统运维** | A（跨租户审计只读 + 紧急禁用） | R（平台运维视图） | R（跨租户审计只读） | R（策略审计 + 安全违规修复） | A（安全配置/KMS/密钥轮换） | R/W（合规应急） | A（全平台审计 + WORM） | R/W（套餐/定价配置只读 + 故障应急） |

> **单元格说明**：
> - `A`（管理）：全量 CRUD + 策略配置 + 删除/禁用/轮换；通常伴随审计日志强制保留
> - `W`（写）：可创建/修改但不可删除或变更关键策略
> - `R`（读）：仅可查询，包含列表/详情/聚合视图
> - `-`（无）：端点层直接拒绝，Casbin 评估 fail-close
> - 普通用户对「组织管理」「应用管理」「角色权限」列默认 `-`，除非被显式授予（通过 ReBAC 自定义共享或 IGA JIT）
> - 应用管理员对跨租户资源一律 `-`，租户内非本应用资源一律 `-`，由 `dom` 维度 Casbin Enforcer 强制域隔离（PERM-002）
