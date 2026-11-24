# ADR-026: Workload Identity 联邦（OIDC 联邦信任 + Token Exchange，SPIFFE 兼容方向）

- 状态：[Current]/[Target] — Service Account/身份池/联邦信任存储与 Token Exchange 端点已落地；JWKS 轮换演练与凭据门禁证据为 Target（出口条件：轮换演练 + 门禁证据）
- 日期：2026-09-14

## 背景

非人主体已有一等公民：Service Account/身份池/联邦信任的 domain/app/infra 存储与服务已落地（`internal/domain/workload/`、`internal/app/workload/`、`internal/infra/{postgres,mysql}/workload/`），HTTP 路由在 `internal/transport/http/api/v1/identitypools/` 注册；Token Exchange 端点 `tokens:exchange` 已挂载（`internal/transport/http/api/v1/auth/chi_protocol.go` 与 `handler.go` 的 `GetTokenExchangeToken`）。工作负载凭据不再只有 `applications` client_credentials 与 `user_credentials.access_key` 两种形态。SRS WLI-001~006 要求 SA 一等主体、OIDC 联邦信任（云厂商/CI 短时身份换发）、短期凭据策略、身份池与 SPIFFE 兼容方向；目标表 `service_accounts` / `service_account_credentials` / `federation_trusts` / `identity_pools`（[3.2](../architecture/data-model.md#32-repository-约定)；`service_accounts`、`federation_trusts`、`identity_pools` 已随生成式 V1 基线声明，`service_account_credentials` 未单独建表——凭据内嵌于 `service_accounts` 列）。协议面 `POST /api/v1/auth/tokens:exchange` 端点已存在（`internal/transport/http/api/v1/auth/routes.go:30` 注册、`chi.go:314` 挂 handler，[Current]；联邦交换语义为 [Target]，SRS AUTH-093 / 《架构设计》6.15）。

## 决策

1. **SA 作为非人主体一等公民**（WLI-001）：独立生命周期（创建/轮换/禁用/删除，删除前预检关联凭据与授权）、属主（用户或组织）、可绑定角色与组、审计身份与人类可区分、不可登录交互式 UI；落 `service_accounts` / `service_account_credentials`（`credential_kind`=federated/static_key）。
2. **OIDC 联邦信任**（WLI-002）：外部 OIDC Issuer 注册为信任源，校验 issuer、JWKS、aud 白名单与 **subject 白名单**（精确或受控通配，如 `repo:org/app:ref:main`）；白名单外断言拒绝并审计；信任配置变更双审；落 `federation_trusts`。
3. **Token Exchange（RFC 8693）为联邦换发通道**（WLI-005）：在既有 `tokens:exchange` 端点扩展 subject_token 类型白名单（access_token/id_token JWT）与 actor/act 链语义（AUTH-093 目标态）；换发令牌 scope 只能收敛；身份池 `identitypools` 把外部联邦身份映射到平台角色/权限集并作为授权出口（WLI-004）。SA 直接凭证（client_credentials）与联邦换发两条路径产出同一 claim 框架、可区分 actor/subject。
4. **短期凭据与凭据主权**（WLI-003）：联邦换发令牌默认 ≤ 1 小时；静态 Access Key/Secret 仅在不可联邦场景经审批保留并强制轮换（`last_rotated_at`）；**禁止新增长效 secret 用例**（门禁拦截、豁免留痕，与 8.13 `secret_rotation_audit` 衔接）。
5. **SPIFFE 兼容方向**（WLI-006）：SA/身份池命名采用可映射 `spiffe://trust-domain/path` 的构造并文档化映射规则；SPIFFE SVID（X.509/JWT）验证仅为契约预留（`[Target]`），不承诺本期交付。

## 理由与后果

联邦信任把「密钥分发」问题转化为「信任源注册」问题：云厂商/CI 签发的 OIDC 断言天然短时效，平台不落任何共享 secret，泄露面从「长期凭据存量」收敛为「白名单配置」。复用既有 Token Exchange 端点避免新增协议面路由族（与 CAS 双前缀收敛教训一致）。代价：subject 白名单治理（双审）引入流程成本；JWKS 缓存须版本化失效以保轮换平滑；静态 key 审批通道需要保留期与豁免审计支撑。

## 替代方案

- **静态 service account key 为主（现状延续）**：长期有效、轮换靠人、泄露即长期可用，WLI-003 明确禁止新增长效 secret 用例；放弃作为目标态，仅保留审批后例外通道。
- **mTLS mesh 全量（直接落地 SPIFFE SVID X.509）**：要求存量调用方全部纳入服务网格，非 mesh 环境不可达，改造面与商业交付节奏不匹配；放弃本期落地，仅保留命名兼容与契约预留。
- **按云厂商专用集成（AWS IRSA / GCP Workload Identity 各写一套）**：N 个云 N 套代码且每套自带信任校验；OIDC 联邦一层抽象后各云只是不同 issuer，专用集成无增量价值；放弃。

## 验证证据

- Current：`grep -rniE 'service.?account|workload|identity.pool|federat|spiffe' internal/ --include='*.go'` 命中数百行（含 IdP ADFS 等真阳性外溢），核心实现见 `internal/app/workload/`（SA/池/信任服务）、`internal/infra/{postgres,mysql}/workload/`（存储）、`internal/transport/http/api/v1/identitypools/`（API）；Token Exchange 已实现（`internal/transport/http/api/v1/auth/chi_protocol.go` + `handler.go`）。`service_accounts` / `federation_trusts` / `identity_pools` 随生成式 V1 基线声明，`service_account_credentials` 未单独建表（凭据内嵌 `service_accounts` 列）。
- Target 出口条件：①两路径（client_credentials / 联邦交换）令牌 claim 框架一致性测试通过（6.15 合流要求）；②白名单外断言被拒且审计；③JWKS 轮换平滑（缓存版本化失效，WLI-002 验收 3）；④长效 secret 创建被门禁拦截且豁免留痕。待取证项：subject 白名单治理与双审流程定稿、`spiffe://` 映射规则文档。

## 复审条件

出口条件满足后转 `[Current]`；SPIFFE SVID 验证启动实现、或联邦信任扩展至非 OIDC 断言源（如 SAML 断言换发）时，须复审并新增/取代本 ADR。
