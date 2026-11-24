# 外部系统接口适配

> 设计锚点：[DESIGN:infrastructure/integration]

> 目标态规范「外部系统接口适配」；写作约束见 [docs/README.md](../../README.md)。

---

## 防腐层架构

所有外部系统集成通过 `internal/infra/` 下的适配器实现，Domain 层不直接依赖外部协议。适配器作为防腐层（Anti-Corruption Layer）隔离外部 API 变更。

## 外部系统适配器矩阵

| 外部系统 | 适配器位置 | 接口模式 | 安全要点 |
|----------|-----------|---------|---------|
| GitHub/Google 等 IdP | `infra/idp/` | OAuth 2.0 标准流程 | state 绑定 + PKCE S256 |
| Goth 通用 Provider | `infra/idp/goth.go` | Goth 框架适配 | 同 OAuth 2.0 |
| SAML IdP | `infra/saml/` | etree + dsig 签名 | RSA-SHA256 验签 + RelayState 绑定 |
| LDAP | `infra/ldap/` | go-ldap/v3 | 凭据信封加密存储 |
| RADIUS | `infra/radius/` | layeh.com/radius | PAP/CHAP |
| Stripe/PayPal | `infra/payment/` | 各 Provider SDK | Webhook 验签（HMAC-SHA256 / 证书链） |
| S3/OSS | `infra/storage/` | aws-sdk-go-v2 S3 兼容 | 预签名 URL + server-side encryption |
| 邮件/短信 | `infra/messaging/` | Factory 模式 | Outbox 投递（不阻塞主链路） |

## SSRF 出口管控（覆盖所有出站 HTTP）

1. 禁内网 IP 段（RFC 1918 全部 + `127.0.0.0/8` + `::1` + `169.254.0.0/16`）
2. 禁止跟随重定向；确需支持时**限 1 跳**且对目标重新执行全部校验
3. DNS rebinding 防护：解析后**固定 IP 连接**
4. URL scheme 白名单 `https`（开发环境允许 `http` 并在启动日志标注）

**覆盖范围**：Webhook、pgtUrl 回调、LDAP/Syncer 出站连接、SCIM 出站 Client、支付网关回调确认、邮件/短信/通知投递。

## 凭据安全存储

所有外部系统凭据（bind_password、webhook secret、支付网关密钥、SMTP 密码等）加密存储走 AES-256-GCM 字段加密（[security-model.md](../../architecture/security-model.md) §2.7；密钥治理见 `contracts/registries/crypto.yaml` 与 [security-ops.md](../../operations/security-ops.md)），不落明文。

## 通知集成

邮件/短信/WebPush 通知：

- **不阻塞主事务**：通知写入 Outbox 后由 Worker 异步投递
- **重试与死信**：复用《design/infrastructure/outbox.md》Outbox/Worker 参数
- **Provider 抽象**：`infra/messaging/` Factory 模式，支持 SMTP/SendGrid/阿里云短信等 Provider 可插拔

## 控制台实时事件流（SSE）

**协议选型**：Server-Sent Events (SSE)，不采用 WebSocket——IAM 的实时需求以单向推送为主（会话事件、审计通知、风险告警），SSE 复用 HTTP 认证与 CORS 且客户端自带断线重连。

**稳定契约规则**：

| 维度 | 规则 |
|------|------|
| 传输 | `Content-Type: text/event-stream`，事件类别以 `event:` 字段区分 |
| 认证 | 与同面 API 一致；令牌不得置于 URL 查询串（避免日志与 Referer 泄露） |
| 租户隔离 | 事件流受 RLS 约束，仅推送当前租户可见事件 |
| 续传 | 事件携带 `id:`，客户端以 `Last-Event-ID` 从版本点续传，不丢事件 |
| 心跳 | 空闲期定期发送 `event: ping`，防止中间设备超时断链 |
| 过滤 | 按事件类型的过滤与权限校验都在服务端执行，客户端过滤不可作为授权手段 |

**事实源**：具体路径、方法与事件名一律以 `internal/transport/http/**/routes.go` 的 `route.Definition` 声明为准，并由 `python3 scripts/gen_openapi.py` 生成到 `openapi/openapi.yaml`。

## 协议发现端点

| 方法 | 端点 | 用途 |
|------|------|------|
| GET | `/.well-known/openid-configuration` | OIDC Discovery（必读） |
| GET | `/.well-known/oauth-authorization-server` | OAuth AS Metadata |
| GET | `/.well-known/oauth-protected-resource` | OAuth Resource Metadata |
| GET | `/.well-known/jwks.json` | 全局密钥集（仅国际算法族 RSA/ES） |
| GET | `/.well-known/jwks-gm.json` | 国密 SM2-SM3 独立密钥集（扩展通道，Discovery `gm_jwks_uri` 宣告） |
| GET | `/.well-known/scim-configuration` | SCIM Discovery |
| GET | `/.well-known/saml-metadata` | SAML IdP Metadata |

**最佳实践**：客户端永远从 Discovery 动态发现所有端点，不要硬编码。

## 互链

- Outbox 投递 → 《design/infrastructure/outbox.md》
- LDAP 详细规格 → 《design/protocol/ldap.md》
- SAML/CAS 规格 → 《design/auth/saml.md》
- 支付集成 → 《design/commerce/payment.md》
- SCIM → 《design/protocol/scim.md》
