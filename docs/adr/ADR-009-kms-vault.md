# ADR-009: KMS/Vault 根密钥托管

- 状态：[Current]/[Target] — 根密钥 Provider 端口 + env/Vault（KV v2 HTTP）启动选择已接线；真实 Vault/云 KMS 集成与凭据轮换、故障演练取证为 Target（出口条件见验证证据）
- 日期：2026-09-14

## 背景

字段级加密（见 `internal/platform/crypto/field_encryption.go`）当前以环境变量提供根密钥，属于“双密钥过渡期”方案：能支持“新密钥加密、旧密钥解密”，但密钥生命周期无法被管理——密钥随镜像/ConfigMap/部署清单扩散后无法审计谁见过它，泄露后无法快速撤销，也无法回答“哪条密文由哪个密钥版本加密”。在进入私有化与生产交付前，必须确定根密钥的托管主体与轮换模型（与 ADR-015 的版本状态机配套）。

## 决策

生产根密钥由 KMS / Vault / 等价 Secret Manager 托管，应用启动时经受控身份取钥（或经 KMS 完成信封解包）；密文信封携带 key id/version（ADR-015）；应用不得把根密钥写入镜像、ConfigMap、Git 或日志；取钥与加解密能力定义为端口，允许云 KMS 与私有化 Vault 各自适配。

## 理由与后果

- 正面：密钥可集中轮换、撤销与审计访问；制品与环境分离，泄露面收窄；双人审批与销毁流程有了落点；为 ADR-015 的版本化信封提供运行时支撑。
- 负面：启动路径新增对外部密钥服务的依赖，需要设计不可用时的显式失败（拒绝启动/保守降级，而非静默明文）；云厂商 KMS API 差异需要端口隔离与私有化适配；增加许可、运维与恢复演练成本。

## 替代方案

- **长期保留环境变量密钥（现状方案固化）**：放弃理由——环境变量无法表达版本、退役与销毁（ADR-015），密钥随部署制品与 shell 环境扩散，泄露后无法证明影响范围；只允许作为过渡。
- **应用内保存主密码、对配置中的数据密钥再加密**：放弃理由——主密码仍须存放在某处，只是把同一问题下移一层，且自研加密资产的安全评审成本高于使用成熟 KMS/Vault。
- **客户端端到端字段加密（服务端不持有密钥）**：放弃理由——IAM 服务端需要对大量字段做校验、检索与策略判断，普遍 E2EE 与服务端鉴权威定位冲突；可在个别敏感属性上另行评估，不构成本决策的替代。

## 验证证据

**现状事实（[Current] 子事实）**：

- `internal/platform/crypto/field_encryption.go:35-45` 读取 `ENCRYPTION_KEY_CURRENT`（优先）、`ENCRYPTION_KEY`（legacy）、`ENCRYPTION_KEY_PREVIOUS`（仅解密回退），即环境变量双密钥方案。
- 根密钥 Provider 端口与启动选择已接线：`internal/platform/config/kms.go` 的 `KeyProvider` 端口 + `InitKMSProvider`（`internal/bootstrap/run.go` 在 `ValidateOrPanic` 前调用，`KMS_PROVIDER=env|vault`，默认 env）。`vault` 分支为 Vault KV v2 的 HTTP 读取实现（`VAULT_ADDR`/`VAULT_TOKEN`/`VAULT_KEYS_PATH`，无 Vault SDK 依赖；未配置或读取失败时密钥解析为空、由 `config/validate.go` fail-fast），代码注释自述 stub，**尚无真实 Vault 集成测试与生产演练取证**。
- `internal/platform/crypto/kms/` 另有契约级占位件：`port.go`（`KeyManagementService` 端口）、`local_env.go`（`LocalEnvAdapter`，仅限开发）、`vault_stub.go`（`VaultStubAdapter`，内存明文，**不是安全边界**）；该包未被组合根接线（注释中的 `KMS_BACKEND` 门控无实现，实际门控是上述 `KMS_PROVIDER`）。云 KMS 适配器无实现（`grep -in 'vault\|kms' go.mod` 无外部 KMS/Vault SDK 依赖）；`grep -rn 'key_access' internal --include='*.go'` → 0 命中（KMS 密钥访问审计尚未落地，出口条件 4 的前提）。
- 配套元数据表 `guliangiam.encryption_keys` 已建（见 ADR-015）。

**出口条件（[Target] 待取证项）**：

1. **生产集成取证**：`KeyProvider` 端口与 env/Vault 选择已接线、Vault KV v2 HTTP 读取路径已实现（见现状事实）；剩余：真实 Vault/Secret Manager 集成测试与生产演练取证，云 KMS 适配器实现；应用启动经 `KMS_PROVIDER=vault`（或云 KMS）取钥而非 `ENCRYPTION_KEY_*` 环境变量（`crypto/kms` 占位件不构成生产边界）。
2. **在线轮换与批量重加密**：`go run ./scripts/dev/rotate_key.go --old-version v1 --new-version v2` 执行双版本在线轮换，加密通道自动切换到新密钥，解密通道保留旧密钥版本；批量重加密脚本支持 Ctrl+C 断点续跑（`--resume-from checkpoint.json`），`pg_stat_progress_copy` 可见进度；旧密钥撤销后新密文不再引用该版本。
3. **取钥失败显式拒绝启动**：`KMS_ADDR=invalid go run .` 启动后 **2 秒内 exit code ≠ 0**，stderr 含明确错误 `FATAL: cannot obtain master encryption key from KMS`；**无任何** `plaintext` 或 `fallback` 环境变量触发的明文降级路径（`grep -rn 'fallback\|plaintext' internal/platform/crypto/ --include='*.go'` 在生产代码中为 0）。
4. **密钥访问审计与恢复演练**：所有 KMS/Vault 密钥访问调用生成审计记录（调用方、密钥 id、操作类型、耗时、状态码）存入 `guliangiam.records`（`grep -c 'key_access' records` ≥ 最近轮换周期内的访问次数）；恢复演练：密钥被误撤销后，通过 KMS/Vault 的「软撤销恢复」或紧急密钥版本恢复，密文在 30 分钟内重新可解密，演练记录归档至《部署与运维指南》。

**待取证项**：云与私有化两套交付形态的 KMS 能力差异定型（确定端口抽象的最小公共子集）；双人审批流程的具体接入方式；HSM/FIPS 合规要求的触发条件。

## 复审条件

ADR-015 版本化信封落地时联动评审并共享出口条件；云与私有化两套交付形态的 KMS 能力差异定型时确认端口抽象；出现 HSM/FIPS 等合规要求或监管对密钥托管有明确指定时复审。
