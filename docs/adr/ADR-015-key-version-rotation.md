# ADR-015: 密钥版本与轮换状态机

- 状态：[Current]/[Target] — 状态机 KeyStatus / IsTerminalKeyStatus、`encryption_keys` 表与只读查询仓储、AAD 绑定信封 v2 已落地；轮换调度与 `encryption_keys` 写入方为 Target（出口条件：调度器、写入方与轮换演练取证）
- 日期：2026-09-14

## 背景

当前字段加密只有“当前密钥加密、上个密钥解密”的环境变量双密钥过渡（ADR-009）：无法表达版本、退役与销毁，无法审计“哪条密文由哪个密钥版本加密”，批量换钥只能一次性全量完成、不可中断续跑，旧密钥在“确认零密文引用”之前无法安全销毁。进入生产与私有化交付前，密钥必须成为有状态、有版本的受管实体。

## 决策

密钥从“环境变量 + 双密钥过渡”升级为显式版本模型：每个逻辑密钥（`key_id`）拥有单调递增的 `key_version`，生命周期状态机为 `pending -> active -> retiring -> retired`，其中 `revoked` 是任一非终态（除 `retired` 外）的强制下线分支；状态取值以 `ck_encryption_keys_status` 检查约束为准（`pending/active/retiring/retired/revoked`）。密文信封携带算法、`key_id`、`key_version`、nonce、ciphertext 和 tag，解密时依据信封版本选择密钥，禁止“逐个尝试所有密钥”。

### AAD 绑定信封 v2（GM-G7 前置口径，2026-11-03）

本节定义字段加密信封的代次、AAD 口径与迁移义务。实现已落地：`internal/platform/crypto/envelope.go` 的 `-aad` 判别与 v2 加解密、`internal/platform/crypto/field_encryption.go` 的 `EncryptColumn`/`DecryptColumn`/`RotateColumn`/`DecryptEnvelopeFieldWithAAD`、`internal/platform/crypto/reencrypt_job.go` 的 v1→v2 重写，以及 `internal/infra/postgres/shared/encrypted_field.go` 的列绑定 mapper；验收锚点见第 6 节。

#### 1. 代次与格式

- **代次命名**：`v1`/`v2` 指信封格式代次，不是 token 头部的 `v{KEK}`；后者是 KEK 版本。不得用 `v2:` 作为代次前缀——`v{N}` 已被 KEK 版本占用，会让存量 v1 解析与 KEK 版本 2 混淆。
- **v0（历史直连 KEK，不属于 v1/v2）**：`base64(nonce||ciphertext)`，无版本头；仅 `FieldEncryptor.DecryptField` 兼容，`FieldEncryptor.DecryptEnvelopeField` 拒绝。
- **v1（现有信封，无 AAD）**：`v{KEK}-{alg}:base64(encrypted_dek):base64(nonce):base64(ciphertext)`；`{alg}` 取 `aes`/`sm4`，`base64` 为标准编码（含填充）。历史无算法后缀的 `v{KEK}:...` 仍在 v1 读取分支。现有 `EnvelopeEncryptor.Encrypt` 对 KEK 封装与明文加密两层 GCM 均传 `nil` AAD（`internal/platform/crypto/envelope.go`）。
- **v2（AAD 绑定信封）**：`v{KEK}-{alg}-aad:base64(encrypted_dek):base64(nonce):base64(ciphertext)`；`-aad` 是唯一判别后缀，其余三段与 v1 相同。AAD 不写入 token。
- **判别顺序**：`v{KEK}-{alg}-aad:` → v2；`v{KEK}[-{alg}]:` → v1；其余才允许 v0/legacy。`envelopeTokenPattern`、`parseEnvelopeHeader`、`EnvelopeEncryptor.Decrypt` 的 fallback 必须按此顺序收紧（G7）。

#### 2. AAD 口径

- AAD 固定为 `v2|{table}|{column}|{primary_key_value}`。
- `{table}` 取 `EncryptedColumn.Table`（全限定名，如 `guliangiam.providers`）；`{column}` 取 `EncryptedColumn.Column`；`{primary_key_value}` 取该行 `EncryptedColumn.PKColumn` 的规范化文本（当前加密列均为 UUID，使用小写、带连字符的文本）。
- `table`/`column` 必须通过 `EncryptedColumn.Validate` 的安全标识符校验；`primary_key_value` 不得为空、不得含 `|`、换行或 `NUL`；含分隔符直接拒绝，不做转义。
- AAD 同时用于 KEK 封装层与 DEK→明文层；任一层 AAD 不匹配即失败。
- **不含租户**：租户是授权属性，不是信封解密上下文；解密路径不保证持有租户 ID，合法租户迁移不应使密文失效。跨租户整行搬移由 RLS/授权层阻断，不由 AAD 承担。
- **不含 `updated_at` 等可变字段**：任何更新都会改变 AAD，使旧密文不可解；AAD 只含行身份（表/列/PK）。
- **不含行外字段**：AAD 必须在一次读写内可构造；引入额外查询/Join 会把解密路径变成跨表耦合。
- **不含算法**：算法与 KEK 版本由 token header 选择；AES↔SM4 切换只改变 header 与 KEK 集，不改变 AAD，因此 v2 可在算法切换窗口内重封而保持行绑定。

#### 3. 双读窗口与迁移

- 新写入一律 v2；`FieldEncryptor.EncryptField` 的调用方必须提供 AAD 上下文（`table`/`column`/`primary_key_value`）。
- 例外（非列绑定封装）：JSON 配置封存、登出令牌缓存等没有行身份的调用点不进入 v2 强制范围，继续使用无 AAD 的 `EncryptField`/`DecryptField`（v1/v0）；有 `table`/`column`/`primary_key_value` 的加密列必须走列绑定 API（`EncryptColumn`/`DecryptColumn`/`RotateColumn`），不得用可选参数静默跳过 AAD。
- v1 读取分支永久保留；存量 v1 不因 v2 上线失效（可解性仍取决于对应 KEK 版本可用，密钥退役按零引用规则）。v1 兼容不等于 v1 获得 AAD 保护。
- `ReencryptJob.RunFor(ctx, columns)` 按 `EncryptedColumn` 逐列重写 v1→v2：`processColumn` 已按 `PKColumn` 取 `pk`，用 `col.Table`/`col.Column`/`row.pk` 构造 AAD；同一行重复执行幂等（已是 v2 且 AAD/KEK/算法一致则跳过）。
- `EnvelopeEncryptor.isCurrentEnvelope` 必须把 v1 视为非当前代次，否则 `RunFor` 会跳过 v1；v2 同 KEK、同算法、同 AAD 才可跳过。
- 密钥轮换对 v2 必须用同一 AAD 重封；`EnvelopeEncryptor.Reencrypt`/`FieldEncryptor.RotateEncryptedData` 增加 AAD 参数或上下文，禁止从 token 反推 AAD。
- `RunFor` 以 `PKColumn` 为游标；若 `updateRow` 返回 0 行（PK 在扫描后变化），该行计为错误并由下一轮重试，不得静默跳过。
- 列集合以 `DefaultEncryptedColumns` / `ProviderCredentialColumns` 为准；本 ADR 不复制列清单或计数。
- v1 迁移窗口内，尚未重写的 v1 仍无 AAD 保护；跨行搬移检测只对 v2 生效。

#### 4. 失败姿态

- AAD 不匹配、密文跨行搬移、v2 token 缺 AAD 上下文：解密失败并返回结构化数据完整性错误（crypto 包哨兵，G7 实现；HTTP 映射沿用现有 `internal` 码，**不新增错误码**）。
- v2 认证失败不得回退 legacy/base64 分支；只有 token 不匹配任何 envelope 形状时才允许 v0 兼容。
- `EncryptedFieldMapper.Decrypt`/`DecryptEnvelope` 必须原样返回错误；禁止转换为空串、零值、明文或“跳过该字段”。
- 日志/审计只记录 `table`/`column` 与 PK 指纹，不记录密文、明文、AAD 原文或 PK 明文。

#### 5. 代价与边界

| 维度 | 结论 |
|---|---|
| 收益 | 跨行仅搬移密文列被 AAD 阻断；密文与行身份绑定。 |
| 成本 | 主键变更必须重加密；所有加密列 mapper 必须携带 `table`/`column`/PK 上下文；v1 迁移窗口内 v1 仍无 AAD 保护。 |
| 查询 | 列表/详情现有查询已返回 PK，不引入额外查询；只有密文、没有 PK 的路径必须改为携带 PK 或拒绝 v2。 |
| 方言 | MySQL 镜像复用同一 mapper/AAD 口径；SQL/DDL 同构义务受 `OPEN-API-37` 约束（`contracts/registries/open-items.yaml`），本文不宣称 MySQL 已验证。 |
| 变更面 | 无 schema 变更、无新依赖、无 API/DTO 变更；AAD 使用现有 GCM `additionalData` 参数。 |

#### 6. G7 验收锚点（已落地）

1. v2 加解密往返（AES-256-GCM 与 SM4-GCM）：`TestAADEnvelopeV2_RoundTrip`。
2. 跨行/跨列搬移密文必失败（AAD 不匹配）：`TestAADEnvelopeV2_CrossRowAndCrossColumnFail`；整行搬移由授权/RLS 用例覆盖。
3. v1 兼容解码；v0 legacy 仅经 `DecryptField`，`DecryptEnvelopeField` 拒绝 v0/明文：`TestAADEnvelopeV2_V1AndV0Compatibility`。
4. `RunFor` v1→v2 转换幂等；密钥轮换后同一 AAD 仍可解：`TestRotateColumn_V1ToV2Idempotent`、`TestReencryptJob_RunFor_V2TokenIsCurrent`。
5. 列表/详情路径携带 PK；缺 PK 路径拒绝 v2：`TestAADEnvelopeV2_RequiresContext`。
6. `Decrypt` 对 v2 认证失败不回退 legacy；mapper 不吞错：`TestAADEnvelopeV2_AuthFailureNoFallback`，`EncryptedFieldMapper.DecryptColumn`/`DecryptEnvelopeColumn` 原样返回错误（`internal/infra/postgres/shared/encrypted_field.go`）。

G8（HKDF + `migrate reencrypt` + `CRYPTO_PROFILE=gm` 强制 `sm4-gcm`）依赖 G7 的 v2 信封口径与 AAD API，必须与 G7 串行。

## 理由与后果

- 正面：可精确审计密文与密钥版本的对应关系；批量重加密可断点续跑（按版本扫描、切走流量、再退役）；旧版本仅临时保留解密能力；退役/销毁需双人审批，销毁前有“零引用”证据；与 KMS/Vault（ADR-009）互补——KMS 托管根密钥，本表管版本元数据与信封。
- 负面：新增密钥元数据表、信封编解码路径与版本健康指标（超出当前 `HasPreviousKey()` 的二元判断）；每次加解密多一次版本解析；重加密作业需要限流，避免与在线流量争用；KMS 解包路径与本地信封路径要长期保持语义一致。

## 替代方案

- **维持双环境变量手工轮换（现状固化）**：放弃理由——无版本元数据、无法证明旧密钥零引用、无法断点续跑，密钥泄露后无法界定影响范围与完成销毁。
- **停机全量重加密**：放弃理由——密文量增长后维护窗口不可接受，失败后没有天然回滚点；版本化信封支持滚动轮换与灰度，停机方案不具备。
- **无版本信封、轮换即全员重新加密再删旧密钥**：放弃理由——正是停机方案的在线版，任何中途失败都会留下“不知用哪把钥匙”的密文；版本信封用密文自描述消除该歧义。

## 验证证据

**现状事实（`[Current]` 子事实）**：

- `db/postgresql/V1__guliangiam_schema_init.sql` 已建 `guliangiam.encryption_keys`：`key_id/key_version/algorithm/purpose/status/key_material_ref/key_fingerprint/rotate_after` 列齐备；`ck_encryption_keys_status` 校验五态（`pending/active/retiring/retired/revoked`）、`uq_encryption_keys_purpose_key_id_version` 唯一约束（`(purpose, key_id, key_version)` 三元组，不同 purpose 下允许相同 key_id/key_version 共存）、`rotate_after` 上 `WHERE status = 'active'` 部分索引——与本决策完全一致。
- 已落地：AAD v2 信封与 `ReencryptJob`（见第 6 节验收锚点）；`encryption_keys` 只读查询仓储已接入（`internal/infra/postgres/crypto/repository.go` 的 `ListActiveForOrg`/`ListForDecrypt`/`GetByVersion`，MySQL 镜像在 `internal/infra/mysql/crypto/repository.go`），领域接口无写入方法。
- 目标缺口：应用侧加密仍走 `field_encryption.go` 的环境变量路径（ADR-009 证据）；`encryption_keys` 无写入方，轮换调度未接入生产调用方。

**出口条件（[Target] 待取证项）**：信封 `{alg,key_id,key_version,nonce,ciphertext,tag}` 落地且按版本选择密钥；密钥状态机迁移（含 `revoked` 强制下线）有单测；重加密作业可续跑并有版本健康指标；旧版本零引用 + 双人审批销毁流程演练；与 ADR-009 KMS 适配器联调通过后转 [Current]。

## 复审条件

KMS/Vault 已提供等价的服务端自动版本化信封与轮换（ADR-009 落地后评估），应用无需自持密钥版本时复审本表与信封路径的存废；信封算法集需要扩充（如国密算法）时复审 `ck_encryption_keys_algorithm` 与编解码兼容策略；AAD 绑定信封 v2 的代次或 AAD 口径变更（租户/可变字段进入 AAD、改变 v1 永久可读承诺）时复审「AAD 绑定信封 v2」小节。
