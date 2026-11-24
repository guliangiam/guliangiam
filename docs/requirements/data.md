# 数据需求

> 目标态规范「数据需求」；写作约束见 [docs/README.md](../README.md)。

---

## 1. 核心数据模型

所有表位于 `guliangiam` schema；主键统一为 `id UUID`，取值为应用层生成的 UUIDv7（DB-REQ-005），数据库不设库函数默认值；子表 1:1 时以主表主键兼作自身主键。

表、列、约束、索引与域分组不在此复写：唯一事实源是声明式 `contracts/schema/*.yaml`（ADR-022），表族按文件名分组表达。枚举与校验命令：

```bash
ls contracts/schema/*.yaml                  # 文件分组即域
python3 scripts/gen_schema.py --check       # 声明与生成物零差异
python3 scripts/check-schema-parity.py      # 声明与 DDL 结构等价
```

跨表设计约束按主题引用：高增长表时间分区见 ADR-011；事务性发件箱、领域事件与统一幂等见 ADR-008/017 与 ADR-018；密钥版本元数据见 ADR-015；审计记录哈希链见 RISK-013。平台内核设施（事务性发件箱、统一幂等键、密钥版本元数据）不属于任何业务聚合，任何新功能不得绕过它们另建同类表。IGA 工单表由访问请求/审批工单需求域承接，随其落地进入声明式 Schema。

### 1.1 数据链路需求

| 需求编号 | 需求描述 | 优先级 |
|----------|---------|--------|
| DATA-WR-001 | 写路径必须在同一事务内完成业务写入、审计记录与 Outbox 事件（data-model.md §4.1） | P0 |
| DATA-RD-001 | 读路径必须显式租户过滤，分页排序以 `id` 作为 tiebreaker（data-model.md §4.2） | P0 |
| DATA-AS-001 | 异步链路必须经 Outbox 投递，消费端按幂等键去重（data-model.md §4.3） | P0 |
| DATA-AU-001 | 审计链路必须追加写入并维持哈希链，禁止应用层 UPDATE（data-model.md §4.4） | P0 |
| DATA-MT-001 | 计量链路必须以业务事实为源、去标识汇聚，失败执行不产生计量事件（data-model.md §4.5） | P0 |

---

## 2. 数据库规范一致性

| 要求 | 校验方式 |
|------|---------|
| 无破坏性语句 | `scripts/check-db-conventions.sh` |
| 无遗留命名 | `scripts/check-db-conventions.sh` |
| 所有 `CREATE` 幂等 | `scripts/check-db-conventions.sh` |
| 含 `updated_at` 的表必须有 `revision BIGINT NOT NULL DEFAULT 1` | `scripts/check-db-conventions.sh` |
| 唯一约束/索引/检查命名规范 | `scripts/check-db-conventions.sh` |
| 无重复索引名 | `scripts/check-db-conventions.sh` |
| 双副本 SHA-256 一致 | `scripts/check-db-conventions.sh` |
| 无 FK 前向引用 | `scripts/check-db-conventions.sh` |
| 金额 DECIMAL(19,4)、币种 VARCHAR(3) | `scripts/check-db-conventions.sh` |
| 全量重放可收敛且幂等 | `scripts/dev/validate_schema_replay.sh` |
| 所有 `ON CONFLICT` 仲裁目标存在 | `scripts/dev/validate_schema_replay.sh` |

---

## 3. 数据生命周期与归档策略

> 本文与《开发规范手册》19.4 均为 `contracts/registries/lifecycle.yaml` 的投影：手册是工程实现口径，本文是需求口径；两侧行集合与数值必须与该注册表一致，冲突以注册表为准。**数据留存 SLA 的机器可读注册表为 `contracts/registries/lifecycle.yaml`，本文引用其数值，不复制其定义**。

### 3.1 留存 SLA 摘要（唯一事实源：lifecycle.yaml）

| 数据类型 | 唯一口径（引用 lifecycle.yaml） | 归档策略 |
|----------|-------------------------------|---------|
| 审计记录 (records) | **retention_days: 365**；商业合规 ≥ 3 年 | 超期归档至对象存储 + WORM 冷归档链段；严禁应用层 UPDATE；哈希链不断裂（RISK-013） |
| 登录条目 (entries) | **retention_days: 180** | daily 03:00 分批物理删除，保留聚合统计 |
| Webhook Events | **retention_days: 90** | 按 `created_at` 分区/清理 |
| Outbox 已成功事件 | **succeeded_retention_days: 7** | 定时清理 |
| Outbox 死信 | **dead_letter_retention_days: 30**（最短） | 保留至人工处置完成；未处置前不得删除 |
| 幂等键 | **retention_hours: 24**（按 `expires_at`） | 定时清理 |
| 过期 Access Token | **access_token_ttl_minutes: 60** | 按 `expires_at` 定时清理 |
| 过期 Refresh Token | **refresh_token_max_ttl_days: 30**；过期即物理删除 | 保留 `revoked_at` 用于安全分析的时间窗（无硬删 TTL，跟审计记录同归档） |
| 过期 Session | **session_max_ttl_days: 30**；**session_short_ttl_hours: 24** | 按 `expires_at` 定时清理 |
| Remember-me 家族与令牌 (remember_me_families / remember_me_tokens) | **ttl: 30d**（`RememberMeTTL`）；到期即失效 | 到期行由 session-cleanup 每小时分批物理删除（批次 1000、单轮最多 10 批，见 lifecycle.yaml cleanup_cron）；读路径过滤 `expires_at`/`revoked_at`/`consumed_at` 语义不变；随租户/用户/家族级联删除 |
| 信任设备 (trusted_devices) | **ttl: 30d**（`trustedDeviceTrustDuration`）；到期即不再受信 | 到期行由 session-cleanup 每小时分批物理删除（口径同上）；登录侧按 `revoked_at`/`expires_at` 过滤，列表标记 `expired`；随租户/用户级联删除 |
| 过期 Authorization Code | 以 `short-lived-state.yaml` 的 `authorization_code` 为唯一口径 | 定时清理 |
| 密码历史 | 按 `PasswordHistoryCount`（默认 5 条） | 超出数量自动清理 |
| 风险信号原文 | **raw_retention_days: 90** | 到期分批物理删除（清理约束见 lifecycle.yaml cleanup_constraints） |
| JWT 签名密钥 | **deletion_delay_days: 60**（retired → 物理删除宽限期）；**rotation_overlap_days: 60**（轮换重叠窗口）；**max_active_keys: 3** | 状态机：pending → active → retiring → retired；revoked 为强制退役分支（ADR-015） |
| 密钥轮换审计 (secret_rotation_audit) | 以 `lifecycle.yaml` 的 `security_evidence_logs.retention_days` 为唯一口径 | 作为密钥主权合规证据链的一部分保留 |
| 隐私权利请求证据 (dsar_events / consent_records) | 以 `lifecycle.yaml` 的 `dsar_events` / `consent_records` 条目为唯一口径 | 先归档证据包再分批物理删除 |
| 应用日志 | 以 `lifecycle.yaml` 的 `operational_logs`（`execution_logs`）为唯一口径 | stdout / 文件轮转 |
| 访问日志 | 非数据库数据，留存以运维日志策略为准（见 `docs/operations/` 运维文档与 `lifecycle.yaml` operational_logs 口径） | Prometheus + 外部存储 |

**清理任务约束**（与 lifecycle.yaml cleanup_constraints 一致）：
- 必须分批、可断点续跑、幂等，并输出删除行数到指标
- **禁止无 `LIMIT` 的批量 `DELETE`**
- 每表清理任务的 cron 计划在 lifecycle.yaml 注册，调度器读取 YAML 执行

### 3.2 隐私擦除

用户注销或依法行使删除权时，`users`/`user_profiles` 走软删除 + 字段匿名化（邮箱替换为 `<hash>@deleted.invalid`、姓名清空、手机号替换为哈希），审计记录保留但仅存主体标识的哈希（用户 UUID 做 SHA-256 + pepper 后存储），**禁止物理删除导致审计链断裂**。处置时限：**请求受理后 30 天内完成**（completion_days: 30，lifecycle.yaml）。

---

## 4. 数据分类分级表

> 分级口径遵循《网络安全等级保护 2.0 数据分类分级指南》（GB/T 43697-2024）。定义四个敏感级别：
> - **公开（L1）**：对外可公开，泄露无影响
> - **内部（L2）**：仅限内部使用，泄露造成轻微影响
> - **敏感（L3）**：需严格控制访问，泄露造成较大影响；对应 GDPR 个人数据（Art 4.1）
> - **机密（L4）**：最高级保护，泄露造成严重影响；对应 GDPR 特殊个人数据（Art 9）+ 密钥/凭证/审计链

| 分级 \ 表名 | 表清单 | 访问控制 | 存储加密 | 保留约束 |
|------------|--------|---------|---------|---------|
| **公开（L1）** | — | — | — | — |
| **内部（L2）** | `organization_settings`, `organization_theme_settings`, `applications`（不含 client_secret）, `provider_configs`, `products`, `plans`, `pricings`, `roles`, `permissions`, `groups`, `resources`, `webhooks`（不含签名密钥）, `ip_rules`, `models`, `adapters`, `enforcers`, `enforcement_rules`, `keys`（元数据不含材料） | 租户内 RLS + Casbin；跨租户显式授权 | TLS 传输；列级无强制（PKI 元数据除外） | 常规生命周期 + 审计链（records） |
| **敏感（L3）** | `users`（含 email/username/phone）, `user_profiles`（实名/头像）, `user_identities`, `user_role_bindings`, `user_group_memberships`, `sessions`, `access_tokens`, `refresh_tokens`, `authorization_codes`, `consents`, `user_mfa_methods`, `subscriptions`, `orders`（不含金额明细）, `tickets`, `ticket_messages`, `invitations` | 租户内 RLS + Casbin + ReBAC 对象级共享；跨租户访问强制审计 | TLS 1.3 + **AES-256-GCM 字段级加密**（email、phone、real_name）；密钥 KMS 托管 | 留存遵循 lifecycle.yaml；GDPR DSAR SLA 30 天 |
| **机密（L4）** | `user_credentials`（密码哈希）, `user_password_history`, `provider_secrets`（Secret 密文 + 指纹）, `payments`, `transactions`（金额+支付渠道）, `secret_rotation_audit`, `records`（审计记录 + 哈希链）, `lockout_attempts`, `web3_nonces`, `face_id_sessions`, `login_challenges`, `device_auth_codes`, `keys`（含密钥材料引用）, `certs` | **Fail-Close**：默认拒绝，显式授权；访问强制全量审计 + 告警；密钥/凭证/审计链操作额外 requiring 双重审批（IGA 审批式提权） | TLS 1.3 + **字段级 AES-256-GCM 信封加密**；密码 Argon2id（用户口令：alexedwards DefaultParams，m=64MiB/t=1/p=NumCPU，`internal/infra/cred/argon2id.go`；Client Secret：m=64MiB/t=3/p=2，`internal/platform/crypto/client_secret.go`；仅存哈希）；Access Secret 仅哈希不落明文；审计链 WORM 归档 | 审计链 **WORM 归档 ≥ 1 年（商业合规 ≥ 3 年）**；密钥轮换 60 天宽限期；支付/交易流水不可变、追加写入 |

> **单元格说明**：
> - 同一表内不同列可跨分级。`users` 表主体为 L3，但 `password_hash`、`access_secret_hash` 列按 L4 处理（字段级加密 + 独立访问路径）
> - L4 表的任意访问请求必须经过 Casbin + **ReBAC 双重评估**，且审计记录强制实时写入 `records`（不允许异步）
> - L4 表禁止 `DELETE`（物理），仅允许软删除 + 字段匿名化（隐私擦除场景除外，按 lifecycle.yaml privacy_erasure 执行）
> - 新增表/列的分级必须在声明式 Schema 中显式标注，未经分级默认 L3 处理

---

## 5. 数据跨境策略

> **本节为唯一需求口径，工程实现引用架构设计 [multi-region.md](../architecture/multi-region.md)**。租户个人数据的存储与处理在其声明区域内完成，跨区域访问受控并审计。

### 5.1 原则基线

1. **个人数据不出区域**（PRIV-010）：`organizations.region_code` 目标列确定租户个人数据的存储区域；个人数据（users/user_profiles/user_identities/sessions/tokens 等 L3/L4 数据）**不跨区域复制**，跨区域访问触发显式授权 + 独立审计 + 拒绝（默认策略）
2. **控制面与数据面分离**（DR-001）：控制面持有租户注册表、套餐、区域路由等元数据（L2，不含个人数据），准实时复制至各区域；数据面持全部个人数据与认证执行状态（L3/L4），**区域自治、不复制**
3. **聚合统计可跨区域**：去标识（去个人可识别信息）的聚合指标（MAU、认证总量等）可异步汇聚到控制面用于计量/计费/仪表盘；去标识不可逆，不得反推至具体个人

### 5.2 跨区域复制策略（唯一事实源：multi-region.md §2）

| 数据类别 | 分级 | 区域内一致性 | 跨区域复制 | 合规依据 |
|---------|------|------------|-----------|---------|
| 控制面元数据（租户注册表/套餐/策略） | L2 | 强一致（单控制面实例 + 副本） | 准实时复制（发布/订阅，最终收敛） | DR-002 |
| 身份主数据（users/user_profiles/user_credentials） | L3/L4 | 强一致 | **不复制**（区域自治，仅控制面持非敏感注册摘要：user_count_hash） | PRIV-010 |
| 会话/令牌（sessions/access_tokens/refresh_tokens） | L3 | 强一致 | **不复制**（跨区域访问触发重新认证 + 风险评估） | DR-002 |
| 审计记录（records） | L4 | 追加写入 + 哈希链构建 | **异步复制**（合规汇聚场景；WORM 归档目标可异地只读副本；链头锚定见 multi-region.md §7） | ADR-031 + RISK-013 |
| 计量事件/账期 | L2（去标识） | 区域内聚合 | 异步汇聚到控制面（喂数边界 METER-005；去标识不可逆） | — |

### 5.3 跨区域访问控制

| 场景 | 处理方式 | 审计 |
|------|---------|------|
| 跨区域数据面 API 调用（个人数据） | 默认拒绝（Casbin + 区域边界 Enforcer）；仅 dedicated 套餐 + 租户显式授权 + 跨区域访问白名单豁免后可放行 | 每次放行强制 records 写入 `cross_region.access.granted`（actor、source_region、target_region、target_resource） |
| 控制面读取个人数据摘要 | 允许（摘要不含 PII）；强制去标识 + 只读 | records 写入 `control_plane.read.summary` |
| 审计链异地 WORM 归档 | 仅写入，禁止读取/修改；归档目标为独立只读副本 | records 写入 `audit.chain.archived.remote` |
| 跨区域紧急故障恢复 | 仅在 L3 档（数据库损坏场景，RTO/RPO 阈值以 `contracts/registries/dr.yaml` 为唯一源）允许 PITR 跨区域恢复；**恢复后个人数据仍归属原区域**，仅作临时容灾副本 | 全流程审计 + 恢复完成后立即清理临时副本 + 复盘 |

### 5.4 合规特殊处理

- **GDPR Schrems II 应对**：跨欧盟/非欧盟区域访问必须通过欧盟标准合同条款（SCCs）或等效合规机制；无 SCCs 的跨大西洋传输自动拒绝
- **中国境内租户**：个人数据（L3/L4）**禁止出境**；控制面元数据准实时复制仅限中国大陆区域；跨区域灾备副本仅限中国大陆范围
- **BYOK/HYOK 密钥主权**（DR-003）：密钥材料全程留在租户侧 KMS/HSM（HYOK）或租户 KMS（BYOK）；密钥**不得跨区域传输**；加解密调用必须路由到密钥所在区域

### 5.5 租户区域归属变更

租户归属区域一旦设定即**不可随意变更**（multi-region.md §1.1）。必须变更时执行受控流水线：**导出 → 校验（全量校验和）→ 目标区域导入 → 双写比对 → 切换控制面路由 → 清理源区域数据**；全流程审计，完成后输出一致性报告归档 ≥ 1 年。
