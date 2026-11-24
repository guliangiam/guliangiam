# ADR-014: 令牌绑定（DPoP 与 mTLS）

- 状态：[Current]/[Target] — DPoP proof/nonce/jti/`ath` 与 mTLS 证书绑定已落地；nonce 协商与跨实例重放拦截故障演练留痕、证书轮换重绑策略为 [Target]（出口条件完成后转 [Current]；索引标注规则见 adr/README.md）
- 日期：2026-09-14

## 背景

纯 Bearer Token 一旦因日志、终端中毒或重定向泄露，即可被任意持有方重放，资源服务器无法区分“合法持有者”与“捡到令牌的攻击者”。OAuth 生态为此提供两类发送方约束（Sender-Constrained Token）：面向浏览器/移动客户端的 DPoP（RFC 9449，持有证明密钥 + proof），与面向机器到机器/机密客户端的 mTLS 客户端证书绑定（RFC 8705，证书指纹 `x5t#S256`）。两者客户端形态不同，本项目决定分别建设、分别验收。

## 决策

签发的 Access Token 支持发送方约束：浏览器与移动端使用 DPoP（RFC 9449），机器到机器使用 mTLS 客户端证书 SAN 指纹绑定；令牌记录绑定类型与绑定值（DPoP 的 `cnf.jkt` 或 mTLS 的 `cnf.x5t#S256`），资源服务器/内省端点校验绑定证明后才接受令牌；刷新令牌不得跨绑定类型复用。

## 验证证据

### DPoP 现状事实 — [Current]（实现已存在，共享存储不完整）

**已建成**：

- `internal/platform/dpop/dpop.go` 完整实现 RFC 9449 proof 校验：`typ=dpop+jwt`、`jwk` 头、`htm/htu/iat/jti` 必填声明校验、时钟偏移窗口、JWK 签名验证（RSA/EC/OKP）、RFC 7638 JWK 缩略图 `ComputeJWKThumbprint`、`BindTokenToDPoP`（写入 `cnf.jkt` 与 `tokenBinding=DPoP` 元数据）。
- `internal/app/auth/dpop.go` 暴露应用服务方法；`internal/app/auth/token_exchange.go` 对 `DpopRequired` 应用在换码路径强制要求 DPoP proof；Transport 层端点声明在 `internal/transport/http/api/v1/auth/routes.go`（`/api/v1/auth/dpop/validate`，另有 `/api/v1/auth/dpop/nonce`；行号随代码演进，以路径模板定位）。
- `go test ./internal/platform/dpop/ -count=1` → `ok ... 2.031s`；`go vet ./internal/platform/dpop/` 无输出。

**缺口闭合情况**：

1. **jti 防重放已跨实例**：`internal/infra/postgres/dpop/` 与 `internal/infra/mysql/dpop/` 提供 DB-backed jti 存储，多实例共享同一去重状态（此前进程内 `ReplayCache` 保留为单进程快速路径）。
2. **DPoP nonce 机制已实现**：`internal/platform/dpop/nonce.go` 提供 `NonceStore` 与服务端 nonce 协商（`grep -rni 'dpop.*nonce\|UseDPoPNonce' internal --include='*.go'` 命中 nonce 存储/校验与 `OAuthErrUseDPoPNonce` 错误码；RFC 9449 §8 的 `use_dpop_nonce` 重新协商路径存在）。

**已建成（对齐项，勿按旧缺口表述）**：算法白名单与 discovery 同源（`internal/platform/dpop.SupportedAlgorithms` 为单一来源，白名单校验下移到包级 `ValidateDPoPProof`，`none`/`HS*` 与未知 `alg` 一律拒绝）；ES256 按 RFC 7518 §3.4 的 raw R‖S 校验；`ProofMaxAge`/`ProofClockSkew` 统一 ±60s；换码 `htu` 使用请求绝对 URL；签发时 DPoP 绑定结果**已持久化**（`cnf.jkt` + `tokenBinding=DPoP`）；RFC 7662 内省返回真实 `cnf.jkt` 且 `token_type=DPoP`；`Service.DPoP` 已装配并有 nil 兜底；proof 非法返回 `invalid_dpop_proof` / 401。

**DPoP 剩余出口条件（待取证项）**：nonce 协商与跨实例重放拦截的故障演练留痕。资源侧每请求 proof（含 `ath`）校验已落地：`dpop.ValidateResourceProof` 先复用 `ValidateDPoPProof`（签名/htm/htu/iat/jti），再以常量时间比较 `ath=base64url(SHA256(access_token))`；`AccessTokenHash`/`ValidateAccessTokenHash` 有已知向量单测。演练通过后可在本 ADR 标注 DPoP 完整 `[Current]`。

### mTLS — [Current]

- 已落地：`internal/platform/mtls` 提供 RFC 8705 `x5t#S256` 计算与 TLS 状态提取；令牌元数据写入 `cnf.x5t#S256`；内省返回该确认值；授权码换码在传输层观测到客户端证书时绑定令牌（V4.71）。
- 现存 `applications.ClientCert` 字段（domain/infra/app 多层出现）是应用联邦 SSL/SAML 的配置属性，不是访问令牌的客户端证书确认（`cnf`）绑定，不得据此声称 mTLS 已支持。
- 证书指纹与确认元数据已落地：`internal/platform/mtls` 提供 RFC 8705 `x5t#S256` 计算（DER SHA-256 → base64url）、`ThumbprintFromRequest`（从 `r.TLS.PeerCertificates` 提取）、`BindTokenToClientCertificate`（写 `cnf.x5t#S256` + `tokenBinding=mTLS`，保留既有 jkt）与 `ClientCertificateThumbprintFromMetadata`；token introspection 在无 DPoP jkt 时返回 `cnf.x5t#S256`（Bearer）。签发调用方已接入：授权码换码命令携带传输层观测到的客户端证书指纹，服务端在无 DPoP 绑定时写 `cnf.x5t#S256`（`BindTokenToClientCertificate`）。可选 TLS 客户端证书透传已落地：`TLS_CERT_FILE`/`TLS_KEY_FILE` 启用进程内 TLS，`TLS_CLIENT_CA_FILE` 加载 CA 并设 `VerifyClientCertIfGiven`（浏览器无证书不受影响，机器客户端可呈现证书）；RS 侧一致性校验已落地：`mtls.ValidateCertificateBinding` 对令牌 `cnf.x5t#S256` 与请求客户端证书指纹做常量时间比较（未绑定令牌无约束、绑定令牌缺证书或指纹不符拒绝）；证书轮换重绑策略与机密客户端集成测试仍为 [Target]。
- 轮换策略：绑定的令牌**不跨证书指纹重绑**；证书轮换后旧令牌因 `cnf.x5t#S256` 不匹配被拒绝，客户端必须重新授权并获取新绑定令牌。
- 集成测试：`internal/platform/mtls/integration_test.go` 用测试 CA 启动 `VerifyClientCertIfGiven` TLS 服务，覆盖客户端证书呈现、无证书请求、令牌绑定匹配与轮换后旧令牌拒绝。

**复审取证项**：nonce 协商与 jti 跨实例重放拦截的故障演练留痕（jti 去重经数据库跨实例共享；nonce 为进程内存储，见「理由与后果」；仅缺演练记录）。

## 理由与后果

- 正面：即使令牌泄露，攻击者不持有证明私钥/客户端证书也无法使用；DPoP 与 mTLS 分别匹配公开客户端与机器客户端的现实能力；绑定信息随内省下发，让资源服务器有统一校验点。
- 负面：内省/资源服务器需要访问客户端证书或每请求 DPoP proof，部署 TLS 与网关配置更复杂；刷新令牌不能跨绑定类型复用，客户端换证明材料需要重新授权；jti 去重经共享存储 `dpop_replay`（PostgreSQL/MySQL）引入对数据库的运行期依赖；nonce 存储为进程内（`internal/platform/dpop/nonce.go`），多副本部署下跨副本 nonce 不互认（部署边界见 [deployment.md §3.6](../operations/deployment.md)）。

## 替代方案

- **仅 Bearer + 极短 TTL（不做发送方约束）**：放弃理由——短 TTL 只缩小窗口不消除重放，且无法满足高敏接口的证明持有要求。
- **只实现 mTLS、公开客户端也走证书**：放弃理由——浏览器/移动端无法现实地分发客户端证书，DPoP 是 RFC 为该形态指定的机制；二者按客户端形态分工，不互相替代。
- **Token Binding（RFC 8471 TLS 通道绑定）**：放弃理由——浏览器与网关生态基本停滞，缺少可互操作实现；保留为复审项而非现期投入。

## 复审条件

引入集中式 PDP/网关统一承担发送方校验时重划边界；TLS Token Binding 生态复活或客户合规明确要求时复审；DPoP 四项缺口关闭时更新本 ADR 状态措辞（DPoP 转为“完整 Current”）。
