# ADR-031: 多区域驻留、国密适配与审计证据链（区域桶 / 密钥主权 / SM2-SM3-SM4 能力位 / 哈希锚）

- 状态：[Current]/[Target] — 区域桶表与解析器、`organizations.region_code` 全链路 + 运行期跨区域驻留守卫（`REGION_RESIDENCY_MODE` off/observe/enforce，默认 observe；`OPEN-API-38` 已关闭）、SM4 能力位 + SM3 审计链哈希 + AAD v2 字段信封 + 审计证据链已落地；控制面/数据面拆分、三档 RTO/RPO 演练、SM 独立交叉验证等为 Target
- 日期：2026-09-14

## 背景

商业化合规三件套：①数据驻留与多区域（SRS DR-001~005、PRIV-010）——决策时现状为**单区域单副本部署**（`k8s.yaml` 仅 Service/Deployment，无多区域/多副本编排；`docker-compose.yml` 同；部署形态以部署清单为准）；②国密工程适配（DR-004、PRIV-009）——决策时加密栈为 AES-256-GCM 字段加密（8.7，`internal/platform/crypto/field_encryption.go`）+ Argon2id（`client_secret.go`），无 SM 算法能力位；③审计证据链（RISK-013/014）——`records` 表与链列、链锚表 `audit_chain_anchors` 已随生成式 V1 基线声明（结构事实源 `contracts/schema/60_audit.yaml`）；区域目标表 `deployment_regions` / `tenant_instances` 已随生成式 V1 基线声明（`contracts/schema/10_tenant.yaml`）。

## 决策

1. **区域桶模型**（[multi-region](../architecture/multi-region.md)，DR-001）：`deployment_regions` 区域注册表 + `organizations.region_code` 列（已随生成式 V1 基线声明）声明租户归属唯一区域桶；控制面/数据面拆分由量化阈值触发（12.4：协议面吞吐占比/单租户容量/存储驻留/故障域四条件），不预设时间表；个人数据与审计区域落盘不出区域（PRIV-010）。
2. **复制语义按数据类别分级**（12.2，DR-002）：身份主数据与会话/令牌不跨区域复制（区域自治）；平台配置准实时复制；审计异步复制仅合规汇聚场景（WORM 归档目标可异地只读）；计量事件异步汇聚控制面（METER-005 喂数边界）。
3. **密钥主权**（DR-003）：BYOK（租户供钥、平台 KMS 信封托管）与 HYOK（密钥留租户侧 KMS/HSM、平台仅持引用）与 ADR-009 根密钥外置及 `encryption_keys` 表（表与域模型就绪，接入为 Target（`OPEN-API-39`）；只存引用与指纹）一致；区域级配置随租户 `region_code` 生效；吊销后数据不可读语义明确。
4. **国密落地为 crypto 能力位，不替换国际算法默认值**（DR-004/PRIV-009）：SM2（签名/证书，GB/T 32918）、SM3（摘要，GB/T 32905）、SM4（字段加密，GB/T 32907，与 AES-256-GCM 同级 envelope 口径）作为**可选能力位**，插入点为 `internal/platform/crypto` 现有加密器抽象（`FieldEncryptor`）与 JWT 签名算法白名单面（8.6）；启动探测与披露（与 ADR-023 多方言能力探测同构）；AES-256-GCM / RS256 等国际算法**保持默认**；能力缺失时明确拒绝启用而非静默降级；TLS 国密套件为可选路径。**能力位唯一登记源 = `contracts/registries/crypto.yaml`，档位开关 `CRYPTO_PROFILE`（`default`/`gm`）。SM3 审计链已落地：新链首条按档位选择 `sha256`/`sm3`，后续记录继承链尾 `chain_algo`，同一链内禁止混用；`records.chain_algo` 仍由 `ck_records_chain_algo` 约束，存量链不改写、不回填。SM2 JWT/JWKS 已落地：按应用 `token_signing_method` 选择 `RS256`/`SM2-SM3`，JWKS 按算法族分端点发布（标准端点 `/.well-known/jwks.json` 仅国际算法族，国密 SM2-SM3 由独立端点 `/.well-known/jwks-gm.json` 发布，Discovery 以扩展字段 `gm_jwks_uri`/`gm_alg_values_supported` 宣告），issuer 按 header `alg` 分派并按 kid 前缀隔离。密码派生口径已落地：`organization_security_policies.password_derive_algorithm` 仅允许 `hmac_sha256`/`hmac_sm3`，**不新增 `password_hash_algorithm`**——存储哈希固定 argon2id，SM3 只进入派生口径，避免以国密名义降低密码存储强度；SM4 字段加密 token 头部携带算法标识（`vN-aes`/`vN-sm4`），读路径按 token 选择 cipher，`ENCRYPTION_KEY_COMPAT` 仅用于 `ENCRYPTION_ALGORITHM` 切换重叠窗口的旧算法解密。** 国密实现采用单一 `gmsm` 实现，不引入第二实现；其独立验证以 GM/T 标准向量或第三方实现离线比对为出口证据（Target ③）。

5. **审计证据链**（RISK-013/014）：`records` 目标态新增链列（完整列与约束以 `contracts/schema/60_audit.yaml` 为准，不改既有列语义）；每个组织一条链（`chain_id` 取组织 ID），锚定按当前链尾闭合一个链段、不按自然日滚动；正常路径下同链写入在同一业务事务内以 `pg_advisory_xact_lock(hashtext(chain_id))` 持锁串行，`entry_hash = H_chain_algo(canonical)`；锚点写入 `audit_chain_anchors`（锚定序号与链尾摘要、`proof` 确定性证明、`anchor_source` 锚定来源；外部时间戳回执可注入 `proof`，完整列以 `contracts/schema/60_audit.yaml` 为准）；链校验工具可发现单点篡改并定位断链；WORM 归档与签名导出（平台签名密钥 + 清单校验和），与 IGA-007 审计员证据导出（ADR-025）衔接；链构建不显著拖慢审计写入。`canonical` 构成、锚定/验证周期、失败降级与断链处置以 [审计链设计](../design/infrastructure/audit.md) 为唯一现行口径。

### AAD 绑定信封 v2（GM-G7 前置口径，2026-11-03）

字段加密信封在现有 v1 之上增加 v2 代次，token 头以 `v{KEK}-{alg}-aad` 判别，AAD 固定为 `v2|{table}|{column}|{primary_key_value}`；AES-256-GCM 与 SM4-GCM 共用同一格式与 AAD，`CRYPTO_PROFILE=gm`/`ENCRYPTION_ALGORITHM=sm4-gcm` 不改变 AAD 绑定。格式串、双读窗口、失败姿态、迁移义务与 G7 验收义务以 ADR-015 的「AAD 绑定信封 v2」小节为唯一事实源。收益是跨行仅搬移密文列被阻断；成本是主键变更必须重加密、所有加密列 mapper 必须携带表/列/PK 上下文，v1 迁移窗口内 v1 仍无 AAD 保护。本增补不改 schema、不改 API/DTO、不新增依赖；MySQL 镜像同构义务受 `OPEN-API-37` 约束（`contracts/registries/open-items.yaml`）。G7 已落地：`internal/platform/crypto/envelope.go` 的 v2 判别与 AAD 加解密、`internal/infra/postgres/shared/encrypted_field.go` 的列绑定 mapper、`internal/platform/crypto/reencrypt_job.go` 的 v1→v2 重写；验收锚点见 ADR-015 第 6 节。G8 的 `migrate reencrypt` 与 `CRYPTO_PROFILE=gm` 强制 `sm4-gcm` 已落地；HKDF 字段密钥派生仍为 Target。

## 理由与后果
数据驻留是多个合规市场的准入门槛（硬约束），区域桶 + 分类复制是「最小可承诺」形态：不承诺跨区域多活（RTO/RPO 单独声明且不得优于 L1~L3 口径，12.3）。国密以能力位而非默认切换进入，兼顾国内合规采购与国际生态兼容。哈希链 + 外部时间戳锚定以最低复杂度满足「发现单点篡改」的验收，无需自研可信时间戳基础设施。代价：区域路由/注册表是新的控制面职责；SM 算法独立交叉验证与能力探测是持续维护面；哈希链使 `records` 写入增加一列计算与锚定任务。

### MySQL 方言的审计链边界（2026-10-29 决策）

MySQL 基线定位为**事务/业务数据面**，不承诺合规审计链能力：

- 链校验与锚定（哈希链、断链定位、`audit_chain_anchors`）只在 PostgreSQL 部署提供；MySQL 的 `records` 写入不参与链计算，链相关仓储方法返回「能力不支持」而非伪造结果。
- 理由：链写入器端口以 pgx 类型定义，且 MySQL `records.Repository.Create` 与事务上下文未绑定连接级锁；实现需要 `GET_LOCK`/`RELEASE_LOCK` + canonical hash 复用 + 写路径接入，超出 MySQL 基线范围，且半落地会制造不可验证的链。
- 后果：MySQL 部署必须在使用文档与数据模型能力矩阵中声明「审计链 PostgreSQL-only」；`OPEN-API-35` 按该范围决策关闭（选项 B）。
- 复审条件：若 MySQL 成为需要 RISK-013 合规证据的部署目标，必须重新评审并实现等价链（含插入、锚定、校验三段与真实 MySQL 集成用例）。

## 替代方案

- **单区域硬约束（现状延续）**：丢数据驻留合规市场，ORG-016 dedicated 租户「可指定区域」承诺不可交付；放弃。
- **全量自研审计链产品（自建 TSA/时间戳服务）**：权威时间戳需要资质与外部信任面，RISK-013 验收仅需「校验工具可发现单点篡改 + 快照锚定」，哈希链 + `anchor_type=remote` 外部锚已足；放弃自研 TSA（保留 remote 锚类型对接外部服务）。
- **国密全量替换为默认算法**：存量密文迁移成本高（`encryption_keys` 版本化虽可支持，但双栈并行期无限拉长）且与国际生态/工具链兼容断裂；放弃——能力位可选、默认值不变。

> 历史说明：「全量自研审计链产品」条目保留当时否决自研 TSA 的理由；锚点字段现行口径为 `anchor_source` / `proof`，以[审计链设计](../design/infrastructure/audit.md) 为准。

## 验证证据

- Current：部署形态以 `k8s.yaml` / `docker-compose.yml` 为准（仍为单区域单副本，无多区域编排；可用 `grep -icE 'region|replica' k8s.yaml` 复核）；审计链列与 `audit_chain_anchors` 已随生成式 V1 基线声明（声明源 `contracts/schema/60_audit.yaml`；生成物核对 `grep -n 'audit_chain_anchors' db/postgresql/V1__guliangiam_schema_init.sql`）；`internal/platform/crypto/` 为 `field_encryption.go`（AES-256-GCM）/`client_secret.go`（Argon2id）/`token_hash.go`，SM2/SM3/SM4 能力位由 `suite.go` 探测并登记于 `contracts/registries/crypto.yaml`；`deployment_regions` / `tenant_instances` 已随生成式 V1 基线声明（`contracts/schema/10_tenant.yaml`）。
- 跨区域驻留守卫已落地（`OPEN-API-38` 已关闭，取证见 `contracts/registries/open-items.yaml`）：`organization.region_code` 域/PG/MySQL/应用/transport 全链路读写与回显；`TenantResolutionMiddlewareWithRegionGuard`（`internal/transport/http/middleware/tenant_resolution.go`）在显式跨租户 target 解析后按双方 region 比较，`REGION_RESIDENCY_MODE` off/observe/enforce（默认 observe，env 注册表已登记），enforce 拒绝、无绑定 fail-closed；真实库 `TestCrossRegionGuardPostgres`（`internal/infra/postgres/regions/guard_integration_test.go`）覆盖同区放行、跨区拒绝、无绑定 NotFound，HTTP 用例覆盖 enforce 403/observe 放行/无绑定 403。
- 链校验工具已落地：`cmd/audit-verify -chain <organization-id>` 通过 `audit.VerifyChainRecords` 逐条重算 `entry_hash` 并校验 `prev_hash` 连续性，单行篡改/断链可定位到首个失配 `chain_seq`；单测覆盖篡改与断链（`chain_verify_test.go`）。
- SM3 审计链已落地：`grep -n 'chain_algo IN' contracts/schema/60_audit.yaml` → `expr: chain_algo IN ('sha256', 'sm3')`；`internal/platform/crypto/chain_algo.go` 的 `DefaultChainAlgorithm` 按 `config.CryptoProfile()` 选择链算法，`NormalizeChainAlgorithm` 拒绝未知算法（`chain_algo_test.go` 覆盖 profile 切换，`chain_verify_test.go` 覆盖混链拒绝）。
- Target 出口条件：①三档 RTO/RPO 演练记录归档（DR-005，L1<1min/0、L2<5min/<5min、L3<4h/<15min）；②SM4 信封加密的独立交叉验证（GM/T 标准向量或第三方实现离线比对）通过，且能力缺失时拒绝启用（12.5）；不设第二实现冒烟（本仓库国密实现仅 `gmsm`，见决策 4）；③审计员证据包校验和独立验证（IGA-007 验收 3）。区域路由探针与跨区拒绝负向测试已随 `OPEN-API-38` 闭环（见上）。待取证项：12.6 容量模型 L2 校准（拆分阈值冻结前置）、外部时间戳回执来源（`anchor_source`）选型、国密套件产品选型（不绑定单一实现）。

## 复审条件

出口条件满足后转 `[Current]`；拆分触发阈值经 L2 校准冻结、国密能力位默认值变更（若发生）、或审计链锚定方式变更时，须复审本 ADR。
