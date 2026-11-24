# architecture/ — 架构总纲与阅读路径

> 目标态规范「architecture/ — 架构总纲与阅读路径」；写作约束见 [docs/README.md](../README.md)。

## 目录定位

GuliangIAM 的架构层总纲——描述系统目标态的分层、数据、安全、API、可观测性与多区域形态。本目录是技术决策的**唯一架构级出口**；实现与本目录冲突时，以代码事实源 + ADR 为准，并同步修正文档。

## 文件地图

| 文件 | 核心职责 | 读者 |
|------|---------|------|
| 《system-overview》 | 产品定位、能力域对齐、总原则 | 架构师、产品负责人 |
| 《layers》 | Clean Architecture 分层、依赖规则、技术栈、领域模块、模块解耦 | 后端工程师、架构师 |
| 《api-surfaces》 | API 三面契约、SurfaceFor 分类器、OpenAPI 路由覆盖 | 后端工程师、前端工程师 |
| 《data-model》 | 数据模型设计原则、统一数据流模板、五条链路闭环 | 后端工程师、架构师 |
| 《security-model》 | 安全红线与不变量清单、安全架构、风险引擎、Workload 身份 | 安全工程师、后端工程师 |
| 《observability》 | 可观测性四支柱、性能基线、SLO 引用 | SRE、后端工程师 |
| 《multi-region》 | 区域桶模型、复制语义、灾备分级、密钥主权、容量模型推导 | SRE、架构师 |
| 《technology-selection》 | 技术选型的加权对比与决策边界 | 架构师、技术负责人 |

## 阅读路径

### 新成员三天路径

| 天数 | 阅读 | 目标 |
|------|------|------|
| Day 1 | 本 README → 《system-overview》 → 《layers》 | 理解产品边界、分层约束 |
| Day 2 | 《api-surfaces》 → 《data-model》 → 《security-model》 | 掌握接口、数据流、安全基线 |
| Day 3 | 《observability》 → 《multi-region》 → docs/adr/README.md | 理解运行时与决策索引 |

### 按角色路径

| 角色 | 必读 | 选读 |
|------|------|------|
| 架构师 | 全目录 + docs/adr/README.md | contracts/registries/ |
| 后端工程师 | 《layers》→《api-surfaces》→《data-model》→《security-model》 | 《observability》、docs/adr/ |
| 前端工程师 | 《api-surfaces》 | 《security-model》中 CSRF/CSP 部分 |
| SRE | 《observability》→《multi-region》 | 《security-model》中密钥轮换与审计链 |
| 安全工程师 | 《security-model》 | 《data-model》中审计链章节 |

## 架构风险登记（设计权衡）

> 作用：把「从零设计的架构为什么可接受」显性化——每项风险给出影响范围、严重等级、缓释机制与残余风险（接受理由）。严重等级定义与执行层可量化红项共用 `contracts/registries/known-red.yaml` 的口径（P0 阻断 / P1 高 / P2 中）；后者的机器可读清单与渲染层见 `docs/generated/known-red-registry.md`。本表只登记**架构级**风险（设计选择带来的系统性风险），不复写门禁基线数字。

| 编号 | 风险 | 影响范围 | 等级 | 缓释机制与证据锚点 | 残余风险（接受理由） |
|------|------|----------|------|--------------------|----------------------|
| AR1 | 租户数据穿透（RLS 或授权判定被绕过） | 全部租户数据面（管理面/协议面读路径） | P0 | RLS fail-closed + 连接级 GUC（[ADR-016](../adr/ADR-016-rls-tenant-isolation.md)）；`check-rls.sh` / `check-rls-policy-content.py`；租户只经 PrincipalResolver 取得 | 新增表忘启用 RLS 由门禁拦截；策略**语义**正确性仍需评审（门禁只能验证恒真等形态） |
| AR2 | 认证凭据泄露与重放（session / refresh / DPoP / WebAuthn / mTLS） | 认证面与协议面全部登录与会话链路 | P0 | PKCE S256、DPoP token binding、refresh 密文哈希、会话状态机、客户端 IP/UA 绑定与 Front/Back-Channel Logout（ADR-004/013）；mTLS 证书指纹/`cnf.x5t#S256`/内省/可选 TLS 客户端证书透传/RS 侧一致性校验以 [ADR-014](../adr/ADR-014-token-binding.md) 验证证据为准；密钥版本化（[ADR-015](../adr/ADR-015-key-version-rotation.md)）；`check-security-readiness.py` | 终端侧恶意软件与用户误操作不可控；以短 TTL + 即时吊销 + 风险引擎补偿 |
| AR3 | 审计证据链断裂或不可验证 | 合规证据、审计导出、事故追溯 | P1 | 审计与业务同事务 + 链哈希（data-model §4.1、[audit.md](../design/infrastructure/audit.md)）；单写者/分链；锚定任务；AC-AUDITCHAIN-001 | 外部锚定存储的可用性与时钟信任；断链处置需人工介入（runbook） |
| AR4 | 单一事实源漂移（文档/契约/DDL/前端类型/领域常量之间） | 全系统设计与交付链 | P1 | docs/README §3 注册表 + 生成链；`check-generated-zero-diff.sh`、`check-registry-sync.py --strict`、`check-doc-facts.sh`、`check-doc-links.py`、`check-doc-anchors.py` | 生成器的语义缺陷超出零差异门禁可见范围，由生成器测试与评审兜底 |
| AR5 | 版本化迁移链风险（升级/回滚） | 数据库结构、发布与回滚路径 | P1 | 声明式 Schema + 版本化迁移（[ADR-022](../adr/ADR-022-declarative-schema-versioned-migration.md)）；PostgreSQL 唯一方言（[ADR-002](../adr/ADR-002-postgresql-source-of-truth.md)）；`check-migrations.sh`、schema parity | 生成式增量升级/回滚的演练与取证要求见 ADR-022 出口条件；接受理由是迁移与回滚脚本同源生成且由零差异门禁强制 |
| AR6 | 最终一致性窗口（Outbox 投递延迟/重复） | 通知、计量、Webhook、审计锚定、SCIM 出站等异步链路 | P1 | 同事务 outbox + 幂等消费 + 指数退避/死信（ADR-008/017/018）；`contracts/registries/outbox.yaml`；AC-OUTBOX/AC-IDEM；SCIM 调和（diff + 分页拉取，ADR-030）；报告落库、审计与修复的出口条件见 ADR-030 | 下游外部系统非幂等时仍需对账补偿（支付域对账 Worker、SCIM 漂移） |
| AR7 | 缓存与吊销竞态（已吊销权限/令牌短时可用） | 授权决策、会话、密钥元数据缓存 | P1 | 缓存只可重建、吊销事件驱动失效、一次性凭据不入缓存、键含租户（ADR-010、`cache.yaml`）；授权判定 fail-close | Redis 整体不可用时降级直读数据库（延迟上升），以容量冗余吸收 |
| AR8 | 密钥轮换与 KMS 依赖 | 签名、字段加密、JWKS 全局 | P1 | 信封加密 + 版本化 + 多版本并存窗口 + 轮换状态机（ADR-009/015）；security-ops 轮换演练 | KMS 区域故障；以本地密钥包与定期演练作为补偿控制 |
| AR9 | 多区域数据驻留违规 | 合规（数据主权）、跨区访问、灾备 | P1 | 区域桶模型 + `region_code` 断言 + 控制面/数据面拆分（[ADR-031](../adr/ADR-031-region-residency-sm-crypto-audit-chain.md)、《multi-region》）；区域路由探针 | 运维误配置由启动断言与探针拦截；新表需评审区域列并接入门禁 |
| AR10 | 三面边界混用（错误语义/限流/鉴权） | 外部协议客户端集成与契约一致性 | P1 | `SurfaceFor` 启动自检 + `surfaces.yaml` 家族清单 + 三面错误写入器 + OpenAPI 契约门禁（[ADR-012](../adr/ADR-012-openapi-source.md)） | 新协议族必须同步注册表与写入器，由门禁强制，但协议语义取舍仍需评审 |
| AR11 | 安全能力声称与实现偏差 | 合规审计、对外安全承诺 | P1 | `contracts/registries/security-readiness.yaml`（设计/实现/验收证据）+ `check-security-readiness.py`；对外声称以注册表为准 | 部分实现项以基线记账，出口条件逐项登记 |
| AR12 | 单库/单体容量阈值 | 管理面/认证面延迟与吞吐、单租户容量 | P2 | SLO / 错误预算（[ADR-020](../adr/ADR-020-slo-error-budget.md)、`slo.yaml`）、多窗口燃尽率告警与发布门禁、时间分区（[ADR-011](../adr/ADR-011-time-partitioning.md)）、读缓存、控制面/数据面拆分量化阈值（《multi-region》） | 阈值临近需提前扩容或拆分；容量模型按季度复核；分面看板与 L1/L2 连续测量的出口条件以 [ADR-020](../adr/ADR-020-slo-error-budget.md) 与 `contracts/registries/known-red.yaml` 为准 |
| AR13 | 前端供应链与首屏预算 | 控制台可用性、XSS/CSRF、首屏性能 | P2 | CSP/CSRF 矩阵、`check-bundle-budget.py`（ratchet 阈值与实测以门禁输出为准）、`check-app-barrel-imports.py`、`check-token-parity.py`、i18n/a11y 门禁；依赖锁定与 SCA 扫描 | 第三方依赖 0-day 不可控；300 KB 目标已由 `OPEN-PERF-01` 评审取消，预算以 ratchet 门禁与框架地板约束为准 |
| AR14 | 客户端来源 IP 不可信（X-Forwarded-For/X-Real-IP 伪造） | 认证限流、IP 白名单/黑名单、审计来源、会话 IP 绑定 | P1 | `TRUSTED_PROXY_CIDRS` 可信代理白名单：仅当 RemoteAddr 命中可信 CIDR 才采信转发头，未配置/畸形配置 fail-close；安全模型 S13 | 代理 CIDR 配错会让客户端 IP 退化为代理 IP，需部署审查与真实网络基线验证 |
| AR15 | 出站集成 SSRF（SCIM/Webhook/身份源等） | 内网元数据、管理面与数据库可达性 | P1 | 默认 SSRF 安全 Transport + 发送前 URL 校验；SCIM 默认拒绝私有/回环，内网连接器需显式 `SCIM_ALLOW_PRIVATE_TARGETS=true`（S7 SSRF 控制） | 内网连接器 opt-in 后失去默认 SSRF 保护，依赖网络策略与连接器配置审计 |
| AR16 | God File / 超 600 行文件债务（AI 网关及其他域） | 可维护性、单点改动风险 | P2 | 《layers》§8.3 定义 >900 行阻塞合并、>600 行拆分的评审口径；超限清单以《layers》§8.3 复现命令输出为准（无独立 `check-file-size` 脚本，判定依赖评审执行）；可量化红项基线以 `contracts/registries/known-red.yaml` 为准 | 评审口径缺少自动门禁承接时债务可能继续增长；拆分按域推进（agent/session/prompt/tool/model 五维等），不设永久豁免 |

## 外部引用

- 决策索引：《docs/adr/README.md》——全部架构决策的唯一状态源；本目录每个决策引用其对应 ADR
- 数据注册表：《contracts/registries/》——命名、错误码、环境变量、状态机、限流、SLO 的唯一口径；本目录只引用不复写
- 数据库结构：声明式 Schema（`contracts/schema/*.yaml`，ADR-022）——本目录只写表名与逻辑类型，不复制列清单
- OpenAPI：`openapi/openapi.yaml`（ADR-012）——本目录只引用路由契约门禁，不复写端点清单
- 问题与风险清单：`contracts/registries/known-red.yaml`（渲染层 `docs/generated/known-red-registry.md`）——执行层可量化红项；架构级风险见本节「架构风险登记」

## 红线重申（docs/README.md §1）

> 三条写作红线（禁止手写可生成事实 / 禁止变更记录入正文 / 禁止现状偏差登记）以 `docs/README.md §1` 为唯一口径，本目录不复写。

## 跨文档引用约定

本目录内部引用使用「《文档名》§N.M」格式，例如《layers》§3.1 指本目录 layers.md 第 3.1 节。外部引用写明路径，例如《docs/adr/ADR-008-outbox.md》。
