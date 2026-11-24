# SAML/CAS 协议规格

> 设计锚点：[DESIGN:auth/saml]

> 目标态规范「SAML/CAS 协议规格」；写作约束见 [docs/README.md](../../README.md)。

---

## 双角色端点总览

系统同时承担 **IdP 角色**（对企业 SP 签发断言）与 **SP 角色**（联邦外部 IdP 登录）。

| 角色 | 端点 |
|------|------|
| IdP | `GET /api/v1/saml/sso/{applicationId}`（HTTP-Redirect）、`POST`（HTTP-POST） |
| IdP | `GET /api/v1/saml/metadata/{applicationId}` |
| IdP | `GET\|POST /api/v1/saml/slo/{applicationId}`、`GET\|POST /api/v1/saml/slo` |
| IdP | `GET /api/v1/auth/saml/redirect/{organizationSlug}/{applicationSlug}`（接收外部 SP 的 AuthnRequest） |
| IdP | `POST /api/v1/auth/saml/acs`（已登录后生成 SAML Response，供前端回投外部 SP） |
| SP | `POST /api/v1/auth/saml/acs/{organizationSlug}/{providerId}`（外部 IdP 浏览器表单回调） |

## CAS 票据类型

| 票据类型 | 前缀 | TTL | 备注 |
|----------|------|-----|------|
| Ticket Granting Ticket | `TGT-` | = 用户 SSO 会话 TTL | CAS 服务端登录态票据 |
| Service Ticket | `ST-` | **5 分钟** | 单次使用，访问特定服务 |
| Proxy-Granting Ticket | `PGT-` | **30 分钟** | 授权代理访问的凭证 |
| Proxy-Granting Ticket IOU | `PGTIOU-` | 一次性，回调完成即失效 | serviceValidate 返回 PGTIOU，IdP 经 pgtUrl 回调投递 PGT |
| Proxy Ticket | `PT-` | **5 分钟** | 单次使用，代理访问目标服务 |

## CAS Proxy Ticket 流程（PGT → PT）

用户 → SP1（首次 ST 验证，serviceValidate 返回 PGTIOU）→ IdP 回调 pgtUrl 投递 PGT → SP1 用 PGT 兑换 PT → 代理访问 SP2（proxyValidate 回显代理链）→ SP2 代理访问 SP3。代理链深度限制（最多 5 层）为目标态约束；运行期票据校验逻辑以 `internal/app/auth/cas.go` 为准（复现：`grep -n -i 'depth' internal/app/auth/cas.go`），该约束的落地已登记开放项注册表 OPEN-API-82。

## SAML SLO 流程

### SP-Initiated SLO

SP1 发起注销 → 用户浏览器跳转 IdP SLO 端点（携带 SAMLRequest）→ IdP Back-Channel 异步通知其他所有已登录 SP（SOAP POST LogoutRequest）→ IdP 清理会话与令牌 → 302 回跳发起 SP 的 SLO 回调

### IdP-Initiated SLO

IdP 侧触发 → Back-Channel 通知所有已登录 SP → 清理 IdP 侧会话 → 返回注销完成

**SLO 安全约束**：

| 设计项 | 说明 |
|--------|------|
| 签名要求 | 所有 LogoutRequest/LogoutResponse 必须签名（RSA-SHA256 或 ECDSA-SHA256，禁用 SHA1） |
| 重放防护 | LogoutRequest ID 缓存 5 分钟 |
| 超时处理 | Back-Channel 请求超时 10s，超时后继续通知其他 SP |
| 部分失败 | 不阻塞全局注销流程，记录审计日志 |
| Session 索引 | LogoutRequest 中携带 SessionIndex，精确匹配会话 |
| Front-Channel 回退 | SP 不支持 Back-Channel 时通过 iframe/重定向执行 |

## AuthnRequest 校验清单（IdP 角色）

| # | 校验 | 规则 |
|---|------|------|
| 1 | Issuer ↔ 应用 entityID | 精确等于该 application 的注册 entityID |
| 2 | ACS 精确匹配 | AssertionConsumerServiceURL 必须命中该应用注册 ACS 白名单 |
| 3 | Destination | 等于本 IdP 实际 SSO 端点绝对 URL |
| 4 | IssueInstant 窗口 | `|now - IssueInstant| ≤ 5min` |
| 5 | ID 重放 | AuthnRequest ID 入共享重放缓存 |
| 6 | NameIDPolicy | Format 必须在宣告集合内 |
| 7 | 签名（可选强制） | 应用级开关升级为 true 时校验 |

## Response/Assertion 生成规格

| # | 项 | 目标态 |
|---|---|--------|
| 1 | 统一签名 | Response + Assertion 双重签名（RSA-SHA256 起步、ECDSA-SHA256 可选，禁 SHA1） |
| 2 | EncryptedAssertion | 应用级开关：SP 注册证书存在时以 AES-128-CBC + RSA-OAEP 封装 |
| 3 | Audience/entityID | 收敛为持久 URI（`{issuer-base}/saml/{applicationId}`），弃用 slug |
| 4 | AuthnContextClassRef | 按 amr 映射：密码 → PasswordProtectedTransport；MFA → MobileTwoFactor；passkey → FIDO |
| 5 | SessionIndex | 绑定 IdP 会话标识（与 WebAuthn `sid` 同源），SLO 才能精确匹配会话 |
| 6 | NameID persistent | 按 (user, sp) 对生成不可关联稳定 ID 并持久化映射表 |

## SP 侧断言校验清单

1. 验签（Response 与嵌套 Assertion 双层，至少覆盖 Assertion）
2. **Reference URI 指向校验**：签名 Reference URI 必须解析到 Response ID 或 Assertion ID，且使用验签上下文返回的被签名元素
3. InResponseTo = 本端 AuthnRequest ID
4. Audience 含本端 entityID；Recipient = 本端 ACS 绝对 URL
5. NotBefore/NotOnOrAfter 窗口判定
6. **Assertion ID 重放缓存**：`(issuer, assertionID)` 入共享存储（Redis，TTL = NotOnOrAfter + 5min），重复即拒绝。**运行期回退边界**：会话缓存 Redis 连接存在时使用 Redis `SET NX` 存储（`internal/bootstrap/wire_saml_replay.go`），否则回退进程内 `saml.MemoryReplayStore`（`internal/infra/saml/replay.go`）——进程内存储仅对单副本部署正确，多副本部署必须配置 Redis 会话缓存，否则跨副本重放不可拦截；`ReplayStore` 为 nil 时校验 fail-closed（`internal/infra/saml/response.go`）。
7. SubjectConfirmationData NotOnOrAfter/InResponseTo 同步校验

## 时钟偏移容忍（全协议统一表）

| 字段 | 容忍值 |
|------|--------|
| SAML IssueInstant / NotBefore | ±60s（SP 与 IdP 双侧一致） |
| SAML NotOnOrAfter 有效窗 | +5min（可配 60–300s） |
| SLO LogoutRequest | ±60s |
| JWT/OIDC nbf/exp | ±30s |

## RelayState 规范

- **服务端信封上限 4096 字节**（`infra/saml.RelayStateSigner` 超长拒绝）；IdP 兼容建议控制在 **80 字节**以内，Okta/ADFS 等外部 IdP 对 RelayState 长度的实际限制未验证。
- SP 发起的外部 IdP 登录使用服务端生成的签名 RelayState：payload 携带 `organizationSlug` / `providerId` / 一次性 nonce，签名绑定二者，TTL 10 分钟；ACS 消费时校验签名、TTL、org/provider 绑定，并把 nonce 放入共享 ReplayStore 一次性消费。
- 禁止把业务回跳 URL 直接放 RelayState（走服务端映射）
- **签名只在后端**：SP 发起时由 `infra/saml.RelayStateSigner` 签发
  `v1.<issuedAtUnix>.<hexHMAC>.<originalRelayState>`，签名绑定
  `organizationSlug` / `applicationSlug`，TTL 10 分钟（时钟回拨容忍 60 秒）；
  SP 消费时（ACS）校验签名、TTL 与绑定关系。
- **前端不持有密钥**：浏览器只透传 RelayState，不得注入任何 `VITE_` 签名密钥；
  本地剥离信封仅用于展示与请求构造，不构成安全校验。
- **IdP 角色原样回传**：`GET /api/v1/auth/saml/redirect/{org}/{app}` 与登录签发路径
  必须把外部 SP 的 RelayState 原字节返回（SAML 2.0 §3.4），不得重签或改写。

## 元数据发布

- **单一权威元数据端点**：`/api/v1/saml/metadata/{applicationId}`
- Location 全部绝对 URL（issuer 基址与 discovery 同源解析）
- entityID/Audience 收敛为持久 URI
- `KeyDescriptor use=signing`（+加密开关开启时 `use=encryption`）挂 X509 证书
- 补 `SingleLogoutService`（Redirect+POST）指向实际 SLO 端点

## SLO 投递与统一后通道设施

Back-Channel LogoutRequest 投递复用 **《design/infrastructure/outbox.md》** 统一 Outbox → Worker → 重试 → 死信设施（与 OIDC Back-Channel Logout、SCIM 出站、Webhook 同栈），SOAP 载荷为投递编码的一种。


## 失败与拒绝审计

| 场景 | 事件 | 处置 |
|------|------|------|
| 断言签名/摘要校验失败 | `protocol.saml.assertion_rejected` | 拒绝并写 `records`；不向 RP 泄露具体校验项 |
| AuthnRequest/Assertion ID 重放 | `protocol.saml.assertion_rejected` | 拒绝并审计；共享重放缓存命中同样留痕 |
| 断言被正常消费 | `protocol.saml.assertion_consumed` | 记录 IdP、应用、NameID 类型与结果 |
| IdP 签发拒绝（ACS 不在应用注册白名单） | `protocol.saml.response_rejected` | 拒绝签发并写 `records`（reason=acs_url_not_registered；best-effort 写入，失败仅告警不阻断拒绝） |

失败路径与成功路径使用同一 `records` 写入点（同事务 + 哈希链），不得只写结构化日志。

## 协议域声明

完整 OAuth2/OIDC 协议扩展见《design/protocol/oauth2.md》。
