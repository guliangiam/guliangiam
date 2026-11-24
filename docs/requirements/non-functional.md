# 非功能性需求

> 目标态规范「非功能性需求」；写作约束见 [docs/README.md](../README.md)。

---

## 1. 性能需求


### 1.1 量化性能阈值

以下阈值为生产承诺值，所有目标值须经 L2 压测报告校准后作为生产 SLO 对外冻结，冻结后变更需通过变更审批。

| 维度 | 指标 | 阈值 | 测量口径 |
|------|------|------|---------|
| 管理面（Admin API） | P99 响应时间 | **< 500ms** | 端到端 HTTP（网络 + DB + 鉴权），剔除批量导入/导出/报表类 |
| 认证面（Auth API） | P99 响应时间 | **< 100ms** | 端到端 HTTP，含 Argon2id 密码校验、Casbin Enforce、JWT 签发 |
| 认证面（Auth API） | P50 响应时间 | **< 30ms** | 同上 |
| 协议面（OAuth/OIDC/SAML） | P99 响应时间 | **< 100ms** | authorize/introspect/token/revocation 端点 |
| 数据面（DB） | PostgreSQL P99 查询 | **< 30ms** | 单条点查 + 轻量过滤 |
| 安全中间件 | Redis 限流/缓存 P99 | **< 5ms** | SEC-003 分布式 Lua 脚本 |
| **整体吞吐** | 全平台并发请求率 | **≥ 1000 req/s** | 持续稳态；峰值 burst 能力由自动扩容保障 |
| Casbin Enforce | P99 单次评估 | **< 2ms** | 多 Enforcer + 域隔离热路径缓存命中 |
| JWT RS256 签发 | P99 单次签发 | **< 1ms** | ≥2048 位 RSA 密钥（默认 2048） |
| Argon2id 密码哈希 | P99 单次 | **< 100ms** | 用户口令路径 m=64MiB, t=1, p=NumCPU（alexedwards DefaultParams，`internal/infra/cred/argon2id.go`） |

**测量分层**（唯一方法学见《性能测量基线》§1）：

| 层级 | 内容 | 能否作为生产 SLO |
|------|------|-----------------|
| L0 | 哈希/加密/UUID/策略缓存等纯函数 | 否，只用于回归 |
| L1 | Repository/事务/索引/连接池 | 部分 |
| L2 | HTTP API/登录/令牌/分页 | 是 |
| L3 | 多实例/故障/恢复/峰值 | 是 |

---

## 2. 安全性需求


### 2.1 合规基线

| 控制域 | 量化阈值 | 依据 |
|--------|---------|------|
| 等保 | **等保 2.0 三级** | 国家信息安全等级保护三级认证（GB/T 22239-2019） |
| 审计合规 | **SOC 2 Type II** | 美国注册会计师协会服务组织控制报告，连续审计期 ≥ 12 个月 |
| 隐私合规 | **GDPR 全适用** | 欧盟通用数据保护条例，含第 25 条设计默认、第 32 条安全处理、第 30 条处理日志 |

### 2.2 身份与令牌

- 浏览器和移动端使用 Authorization Code + PKCE S256；**严格禁止 implicit grant 和未校验的 redirect URI**
- Access Token 短时有效（默认 60 分钟），Refresh Token 必须旋转、绑定 family、检测重用并支持主动吊销（默认 30 天最大 TTL）
- **JWT 严格校验** `iss`、`aud`、`exp`、`nbf`、`nonce`、`kid` 和算法白名单；**ID Token 签名算法由应用 `token_signing_method` 选择**（封闭枚举 `RS256` / `SM2-SM3`，事实源 `internal/platform/jwks/id_token_algorithms.go`；RSA 密钥不低于 2048 位（默认 2048，常量 `jwtSigningKeySize`），见 `internal/bootstrap/wire_jwks_keys.go` 与 `internal/platform/jwks/manager.go`）。密钥轮换宽限期 ≥ 2 × max token TTL = 60 天（ADR-015）
- MFA 优先 WebAuthn，TOTP 为兼容方式；恢复码只展示一次且只存哈希

### 2.3 传输与密码安全

| 项目 | 量化阈值 |
|------|---------|
| **传输层** | **TLS 1.3 全链路强制**（HTTP → HTTPS 301；HSTS `max-age=63072000; includeSubDomains; preload`）；TLS 1.2 仅允许作为 FAPI 2.0 兼容降级路径且必须 mTLS |
| **密码哈希** | **Argon2id 双路径参数**：用户口令 m=64MiB, t=1, p=NumCPU（alexedwards DefaultParams，`internal/infra/cred/argon2id.go`）；Client Secret m=64MiB, t=3, p=2（`internal/platform/crypto/client_secret.go`）。密码最低长度 ≥ **14 位**，强制大小写 + 数字 + 特殊字符至少 3 类，密码历史回溯默认 5 条（新写入 argon2id + pepper，遗留 SHA-256 历史仅兼容校验），密码过期默认 90 天 |
| **密码泄露检查** | Have I Been Pwned k-匿名协议（P2 优先级），命中即强制重置 |
| **密钥托管** | 生产根密钥由 KMS/Vault 管理（ADR-009/015），禁止写入代码、镜像、ConfigMap、日志或 CI 输出；密钥从 JWKS 移除（retired）到物理删除的宽限期 ≥ 60 天 |

### 2.4 租户与授权

- 所有组织资源默认带 `organization_id`，跨组织查询必须显式授权、显式参数和独立审计
- 授权顺序固定：认证 → 租户解析 → 资源归属 → 业务状态 → 策略评估 → 审计 → 执行
- Casbin 负责策略执行，不替代应用层租户隔离；授权异常必须 fail-close

### 2.5 数据和供应链

- Provider/应用/Webhook/LDAP 等 Secret 使用 **AES-256-GCM envelope encryption**（数据密钥 per-Secret、主密钥由 KMS/Vault 托管）
- 所有 SQL 参数化，动态列名/排序字段/SCIM 属性使用白名单
- 依赖必须锁版本、执行漏洞扫描（CRASH/SCA）和 SBOM 生成（CycloneDX，每 release 产出）
- 依赖扫描阈值：**高危漏洞（CVSS ≥ 7.0）必须在 72 小时内修复或提供受控例外**；严重（CVSS ≥ 9.0）必须 24 小时内响应

### 2.6 安全域总览

| 安全域 | 需求 |
|--------|------|
| 认证安全 | 纵深防御（6 层：网络→传输→认证→授权→令牌→数据） |
| 传输安全 | **TLS 1.3 全链路 + HSTS + CSRF 双提交 Cookie** |
| 密码安全 | **Argon2id（双路径参数见 §2.3）+ ≥14 位 + 密码历史 5 条 + 90 天过期 + HIBP** |
| 令牌安全 | PKCE S256 + DPoP + Token Rotation + 重用检测 + **RS256 ≥2048 位密钥** |
| 授权安全 | Casbin v3 + 多 Enforcer + 域隔离 + 热刷新 |
| 数据安全 | UUIDv7 + 参数化查询 + Schema 隔离 + 敏感字段 AES-256-GCM 字段级加密 |
| 网络安全 | IP 黑白名单 + 分布式速率限制（Redis/Lua）+ CSRF + WAF |
| 审计安全 | 全链路审计 + OTel 追踪 + Prometheus 监控 + WORM 归档 |
| 防暴力破解 | 账号锁定（**5 次失败 / 15 分钟窗口 / 锁定 30 分钟**）+ 通用错误（防枚举） |

---

## 3. 可用性与可靠性


### 3.1 SLA 与 RTO/RPO

服务可用性 SLO 与错误预算以 `contracts/registries/slo.yaml` 为唯一事实源（复现：`python3 scripts/check-registry-sync.py --strict`）。RTO/RPO 三档、演练频率与恢复判据以 [backup-dr.md §7](../operations/backup-dr.md#7-rtorpo-三档与恢复演练) 为唯一数值源；§6.2 只保留需求条目与依赖机制说明。

### 3.2 可用性特性清单

| 需求 | 描述 |
|------|------|
| 优雅关闭 | SIGINT/SIGTERM 信号处理，完成进行中请求（超时上限 30s）后关闭 |
| 存活检查 | `GET /healthz`，用于容器探针；响应体同时披露加密能力位 `status`/`profile`/`missing`/`degraded`；`CRYPTO_PROFILE=gm` 且必需能力缺失时进程在监听前 fail-fast（端点不可达），仅 `gm_tls` 缺失时 200 `status=degraded`；`unavailable`→503 仅覆盖已发布报告的防御/注入用例 |
| 就绪与版本信息 | `GET /readyz` 为 readiness（执行当前驱动 DB Ping，需 `X-Metrics-Token`）；`GET /api/v1/system/version` 返回版本与构建信息（`internal/transport/http/api/v1/system/handler.go`） |
| 故障恢复 | 中间件 Recoverer 捕获 panic，返回 500 + 审计记录 + Prometheus 指标 |
| 数据库连接 | pgx 连接池（min=4, max=20），自动重连，连接检查 P99 < 100ms |
| Schema 迁移 | 声明式 Schema 唯一源，生成器产出版本化迁移与回滚（ADR-022）；生产禁用自动迁移 |
| RADIUS 可选 | 环境变量控制启停，不影响主服务 |
| 外部集成兜底 | 所有外部集成（IdP/支付/短信/Webhook）强制超时（HTTP connect=5s, read=10s）、指数退避（2^n × 100ms，n≤5）、死信队列、人工重放入口 |
| 幂等保证 | 高风险写操作支持幂等键（24 小时 TTL）和 `ETag`/`If-Match`，避免重试造成重复创建或并发覆盖 |

**演练硬性要求**：记录检测时间、恢复时间、实测 RTO/RPO 与目标值差距、数据一致性核验（校验和比对）、告警证据与整改负责人。实测未达标即登记差距项并追踪至关闭。审计事件必须可靠写入；异步投递采用 Outbox（ADR-008），消费端必须幂等。

---

## 4. 可扩展性与可维护性


| 需求 | 描述 |
|------|------|
| 架构可扩展 | Clean Architecture 分层，领域层不依赖基础设施；新增域无侵入（依赖注入 + 接口反转） |
| Provider 可扩展 | Factory 模式，新增 Provider 只需实现 `ProviderFactory` + `StorageFactory` 接口，无需修改核心代码 |
| 权限可扩展 | 多 Enforcer 架构，支持自定义 Casbin 模型（`r = sub, dom, obj, act`），热刷新 < 500ms |
| 数据库可扩展 | PostgreSQL 是唯一运行期数据库（ADR-023）；MySQL 镜像仅用于 DDL 与仓储兼容性验证，运行期缺口由 `OPEN-API-37` 跟踪；大表按时间分区（ADR-011，月分区 + 自动过期） |
| 监控可扩展 | OpenTelemetry + Prometheus，支持自定义指标注册；自定义指标须自带标签集约束（≤ 10 维） |
| 前端可扩展 | API v1 限界上下文模块，懒加载路由（首屏 JS ≤ 200KB gzip） |
| 代码规范 | Go lint + errcheck + gosec 全门禁；函数圈复杂度 ≤ 15；包依赖 ≤ 4 层 |
| 可观测性 | 每请求 trace_id 全链路贯通；业务错误码唯一来源 contracts/registries/errors.yaml，禁止硬编码 |
| DB-REQ-005 | 多数据库支持：主键统一 UUIDv7，由应用层生成，数据库不设库函数默认值；仓储 SQL 按方言隔离 |

---

## 5. 合规与隐私


### 5.1 合规控制矩阵（量化阈值版）

| 控制域 | 需求 | 量化阈值 | 验收证据 |
|--------|------|---------|---------|
| **身份鉴别** | MFA、Argon2id、会话安全属性 | 密码 ≥ 14 位 + Argon2id；MFA 启用率 ≥ 90%（平台目标） | 登录/MFA 测试和配置扫描报告 |
| **访问控制** | RBAC、租户隔离、最小权限、fail-close | 跨租户访问审计覆盖率 100%；Casbin deny 路径 fail-close 测试 | 越权测试和策略审计报告 |
| **安全审计** | 高风险操作可追溯 | 记录 actor/目标/时间/IP/request_id/耗时；审计记录保留 ≥ 1 年（商业合规 ≥ 3 年），WORM 归档（不可篡改） | records 查询和审计演练（每季度） |
| **数据保护** | TLS、字段加密、日志脱敏、Secret 托管 | **TLS 1.3 全链路**；敏感字段 AES-256-GCM 加密；日志中 PII 脱敏（正则覆盖 email/phone/ip）；Secret 不入日志/ConfigMap/CI | 密钥清单和日志扫描（每日自动化） |
| **隐私权利** | 访问/更正/导出/冻结/擦除 | **DSAR SLA 30 天**；同意记录版本化；数据可携导出（JSON Lines + 校验和）；删除请求 30 天内完成匿名化（privacy_erasure） | 数据生命周期测试报告 |
| **可用性** | 备份、RTO/RPO、恢复和故障演练 | **RTO < 4h、RPO < 15min**；演练剧本可复现 | 恢复报告（每季度归档） |
| **供应链** | 锁版本、漏洞扫描、SBOM 和依赖审查 | **高危漏洞 CVSS ≥ 7.0 在 72h 内修复**；严重 CVSS ≥ 9.0 24h 响应；每 release 产出 CycloneDX SBOM | CI/release 证据归档 |
| **合规认证** | 等保 2.0 三级、SOC 2 Type II、GDPR | **等保 2.0 三级**认证；**SOC 2 Type II** 连续 12 个月审计报告；GDPR 第 25/30/32 条合规 | 认证证书 + 年度合规审计报告 |

### 5.2 DSAR 工作台 [PRIV-001 ~ PRIV-010]

> 核心要点：
- 统一的隐私权利请求工作台（访问/更正/冻结/擦除/可携/撤回同意）
- **SLA 时钟 ≤ 30 天**、状态机与全程审计
- 同意记录独立建模并版本化
- 数据可携导出包（JSON Lines/CSV + 清单 + 校验和）

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| PRIV-001 | 系统必须提供统一 DSAR 工作台（访问/更正/冻结/擦除/可携/撤回同意），SLA ≤ 30 天，状态机与全程审计 | P0 |
| PRIV-009 | 加密算法合规：国密 SM2/SM3/SM4 为可选能力位，能力缺失时拒绝启用而非静默降级（ADR-031） | P0 |
| PRIV-010 | 个人数据驻留：存储与处理限声明区域，跨区域不复制个人数据，跨区域访问受控并审计 | P0 |

### 5.3 数据驻留

租户个人数据的存储与处理在其声明区域内完成（`organizations.region_code` 目标列，联动 `deployment_regions`），跨区域访问受控并审计（PRIV-010），区域间**不复制个人数据**（去标识聚合统计除外）。详细区域桶模型与复制语义见 [multi-region.md §1-§2](../architecture/multi-region.md)。

---

## 6. 容量与灾备


### 6.1 支撑容量阈值

| 维度 | 阈值 | 说明 |
|------|------|------|
| **租户总数** | **≤ 100,000（10 万）** | 全平台租户上限，shared 档 + bridged + dedicated 合计 |
| **单租户用户数** | **≤ 1,000,000（100 万）** | dedicated 档独立实例目标；shared 档单租户建议 ≤ 10 万触发 dedicated 迁移评估 |
| **并发认证请求** | **≥ 1,000 并发** | 认证面同时在途请求数（稳态）；峰值 burst 由自动扩容保证 ≥ 3× 稳态 |
| **单租户峰值 QPS** | **≤ α × C_n** | α 的取值由 multi-region.md §4 冻结；C_n = 单实例容量 × n × 超卖比 |
| **审计日增量** | **A_tenant ≤ 分区/归档预算** | records 表按月分区；日增量超限触发 dedicated 迁移评估（multi-region.md §4） |
| **单租户存储预算** | **≤ S_u × U × os_store** | S_u = 每用户存储占用，os_store = 索引+碎片放大（默认 1.3） |

### 6.2 RTO/RPO 三档

| 档 | 场景 | RTO | RPO | 依赖机制 |
|----|------|-----|-----|---------|
| L1 | Pod / 单实例应用故障 | < 1 分钟 | 0 | K8s 自愈（探针） |
| L2 | 数据库主库故障 | < 5 分钟 | < 5 分钟 | Patroni 自动 failover + 同步副本 |
| L3 | 数据库损坏 | < 4 小时 | < 15 分钟（硬闸值） | PITR（物理基备 + WAL ≤ 15min）或逻辑恢复 |

> 注：三档 RTO/RPO 的数值以 [backup-dr.md §7](../operations/backup-dr.md#7-rtorpo-三档与恢复演练) 为唯一数值源，本条只声明需求口径（硬闸值）。跨区域灾备 RTO/RPO 单独声明，且不得优于此处承诺（DR-002 验收 3）。

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| DR-001 | 控制面与数据面分离：控制面仅持租户注册表/套餐/区域路由等 L2 元数据，数据面区域自治（ADR-031） | P0 |
| DR-002 | 跨区域复制按数据分级白名单执行；RTO/RPO 三档（L1 < 1min/0、L2 < 5min/< 5min、L3 < 4h/< 15min）为硬闸 | P0 |
| DR-003 | BYOK/HYOK 密钥主权：密钥材料留在租户 KMS/HSM，不得跨区域传输，加解密路由到密钥所在区域 | P1 |
| DR-004 | 国密能力位（SM2/SM3/SM4）可选，不替换国际算法默认值（ADR-031） | P1 |
| DR-005 | 灾备演练按三档频率执行并归档；实测 RTO/RPO 未达标必须登记差距并追踪至关闭 | P1 |

### 6.3 演练要求

| 档 | 演练频率 | 归档要求 |
|----|---------|---------|
| L1 | 每月单实例恢复演练 | 演练报告归档 ≥ 1 年 |
| L2 | 每季度备份恢复演练 + 密钥版本切换演练 | 演练报告归档 ≥ 1 年 |
| L3 | 每季度 PITR 抽演 + 年度跨场景演练 | 演练报告 WORM 归档 ≥ 3 年 |

演练硬性要求：记录检测时间、恢复时间、实测 RTO/RPO 与目标值差距、数据一致性核验（校验和比对）、告警证据与整改负责人。实测未达标即登记差距项并追踪至关闭。

### 6.4 租户隔离分级套餐容量映射

| 套餐 | 推荐租户规模 | 隔离边界 | 最大并发认证 | 备份频率 |
|------|------------|---------|------------|---------|
| shared | ≤ 10 万用户 | 行级 + RLS | ≤ 200 | 每日增量 + 每周全量 |
| bridged | ≤ 50 万用户 | Schema | ≤ 500 | 每日增量 + 每周全量 |
| dedicated | ≤ 100 万用户 | 独立实例 | ≤ 1000 | 实时 WAL + 每 4 小时全量快照 |

---

## 7. 验收条件引用

§8 附录（需求优先级、编号分配）和 §12 P0/P1 验收条件只做简要引用说明，具体验收条目登记于 `acceptance-criteria.md`。需求编号空间治理规则见 README.md §4。
