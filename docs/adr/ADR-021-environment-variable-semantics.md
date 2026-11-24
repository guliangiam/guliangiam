# ADR-021: 环境标识与密钥变量语义统一

- 状态：[Current] — 环境标识/密钥语义收敛与统一校验入口已落地；决策 5 的「生产代码零裸读环境变量」门禁残留已收敛为零（见验证证据）
- 日期：2026-09-13，落地：2026-09-15

## 背景

收敛前代码读取环境信息的方式存在两套并行的语义，导致"是否生产环境"的判定被分散到不同变量：

| 变量 | 读取位置 | 作用 |
|---|---|---|
| `IS_PRODUCTION` | `internal/bootstrap/server.go`、`internal/platform/crypto/field_encryption.go` | 生产标记；决定加密器缺密钥时是否 `panic` |
| `APP_ENV=production` | `internal/platform/config/validate.go` | 环境名；触发配置严格校验 |
| `SESSION_SECRET` | `internal/platform/config/validate.go` | 生产环境必填校验对象 |
| `SESSION_HMAC_KEY` | `internal/transport/http/api/v1/auth/cookie.go` | Session Cookie HMAC 实际密钥；生产环境缺失时 `panic` |
| `GINULIANGIAM_ENV` | `internal/transport/http/api/v1/auth/cookie.go:26` | **拼写错误**的环境变量（应为 `GULIANGIAM_ENV`），任何部署都不会设置它 |

> 上表为收敛前的读取点快照；`SESSION_HMAC_KEY` / `SESSION_SECRET` 其后已退役，退役标记见 `contracts/registries/env.yaml`。

后果：

1. 只设置 `APP_ENV=production` 会让 `NewFieldEncryptor()` 认为处于非生产环境，敏感字段可能以明文写入且不拒绝启动；只设置 `IS_PRODUCTION=true` 又不会触发 `config/validate.go` 的严格校验。
2. 会话密钥同理——校验的是 `SESSION_SECRET`，实际使用的是 `SESSION_HMAC_KEY`，可能出现"校验通过但密钥未配置"或"密钥已配置但校验失败"的错配。
3. **拼写缺陷导致保护永久失效**（收敛前缺陷，已修复，见验证证据）：`auth/cookie.go:26` 当时判断的是 `os.Getenv("GINULIANGIAM_ENV") == "production"`，该变量在任何部署（含本仓库 `.env.example`、`k8s.yaml`、`docker-compose.yml`）中都不存在，条件恒为 `false`，因此「生产环境缺少 `SESSION_HMAC_KEY` 时拒绝启动」的运行期分支**从未生效**，即使同时正确设置了 `APP_ENV=production`。这是本 ADR 中优先级最高的一条，必须在其他收敛动作之前修复。

## 决策

0. **环境名不得存在第二个字面量**：任何模块不得用自造的、部署中不存在的环境变量名作为环境判据（历史上 `auth/cookie.go` 曾读取 `GINULIANGIAM_ENV`，该变量在任何部署中都不存在，使「生产缺密钥即拒绝启动」的分支恒不生效）。判据只能来自 `APP_ENV`。
1. **环境标识唯一为 `APP_ENV`**，取值 `development` / `test` / `staging` / `production`；`IS_PRODUCTION` 作为派生布尔量由 `APP_ENV == "production"` 计算（若保留该变量，只允许作为 `APP_ENV` 的只读同义词并在启动时校验二者一致，不一致则拒绝启动）。`GULIANGIAM_ENV` / `GINULIANGIAM_ENV` 一律删除，不提供别名。
2. **会话签名密钥曾唯一为 `SESSION_HMAC_KEY`**（长度 ≥ 32 字符；`SESSION_SECRET` 为过渡期兼容别名）。该变量已退役：HMAC 签名会话 Cookie 已删除，会话来源改为 `sessions` 仓库行；变量登记与退役标记见 `contracts/registries/env.yaml`。
3. **生产环境的"缺失即拒绝启动"必须由单一入口统一执行**：`platform/config` 校验全部生产必填项（`ENCRYPTION_KEY_CURRENT`、`POSTGRES_DSN`、`ISSUER_URL` 等），任何模块不得各自实现裸 `panic`。
4. **禁止明文降级**：所有环境在密钥缺失且需要写入敏感字段时必须返回错误；非生产环境不得通过"告警 + 明文写入"绕过（见 ADR-015、AC-KEY-003）。
5. **`platform/config` 包为核心业务配置的唯一事实源**（原 `env.go` 已按域拆分为 `env_core.go`/`env_database.go`/`env_infra.go`/`env_security.go`/`env_upload.go` 等，清单以 `ls internal/platform/config/` 为准）：所有"核心业务链路"（DB 连接、Redis/Cache、Session、RateLimit、CORS 预检、密码校验、加密密钥、Pool 大小、Session 清理周期、邮件三大 Provider、Session Cookie）的环境变量读取统一通过 `platform/config` 暴露的 getter，禁止任何生产代码（bootstrap 启动层除外）直接调用 `os.Getenv` / `os.LookupEnv`。验证门禁：`grep -rn 'os\.Getenv\|os\.LookupEnv' internal/ --include='*.go' | grep -v '_test\.go:' | grep -v '/platform/config/' | grep -v '/bootstrap/' | grep -v '/platform/db/migrate.go'` 必须为零（注意排除模式须匹配 `grep -rn` 的 `文件:行号:` 输出形态；残留已收敛为零，见验证证据）。
6. **可选第三方集成统一入口 `envOrEmpty`**：存储（多后端）、SMS（多 Provider）、CAPTCHA（多后端）、Notification（多渠道）、Identity Verification（多后端）等**可选第三方集成**统一通过 bootstrap 层 `envOrEmpty(key)` 读取（调用点数量以 `grep -rn 'envOrEmpty' internal/bootstrap --include='*.go'` 输出为准）。这些变量具有"可选、数量多、生命周期与 Provider 生态同步"的特征，不适合全部收编进 config 包导致其过度膨胀。`envInt` 已废弃（全部 int 型变量已迁移至 config 包的 `parseIntKey` 路径）。

## 理由与后果

生产安全开关分散在多个变量时，任意一处漏配都会静默降低安全等级，且难以通过配置审查发现。收敛为单一环境变量 + 单一密钥变量 + 单一校验入口后，配置正确性可以一次性验证。

代价：需要改动 `server.go`、`field_encryption.go`、`config/validate.go`、`auth/cookie.go` 四处读取点，并同步更新 `contracts/registries/env.yaml`（渲染层 `docs/generated/env-registry.md`）、`docs/operations/deployment.md`、`.env.example` 与 `k8s.yaml`。过渡期内必须同时接受新旧变量名，且对冲突取值拒绝启动而不是择一使用。

## 替代方案

- **保留双变量但交叉校验**：可降低改动量，但仍保留两处真值来源，长期会再次漂移。
- **只用 `IS_PRODUCTION`**：语义上是派生量，作为配置输入会导致 `staging` 无法表达。

## 验证证据（`[Current]`）

- 环境标识唯一为 `APP_ENV`：`internal/platform/config/env_core.go` 的 `IsProduction()`，`IS_PRODUCTION` 仅作只读同义词且与 `APP_ENV` 冲突即拒绝启动（`TestValidateOrPanic_ISProductionContradictsAppEnv`）。
- 会话签名密钥 `SESSION_HMAC_KEY`（含 `SESSION_SECRET` 别名）已退役：唯一消费方 HMAC 签名会话 Cookie 已删除，Go 生产代码无读取点，`contracts/registries/env.yaml` 标记 `retired: true`（OPEN-API-20/22）。
- 生产「缺失即拒绝启动」统一由 `platform/config` 校验，且 `ValidateOrPanic()` 先于一切初始化执行（`TestValidateOrPanic_ProductionValid`）。
- 密钥缺失在**所有环境**拒绝启动，非生产「告警 + 明文写入」分支已删除（`TestFieldEncryptor_MissingKeyRefusesStartup`、`TestValidateOrPanic_MissingEncryptionKey`）。
- 全仓不再存在 `GULIANGIAM_ENV` / `GINULIANGIAM_ENV`（`grep -rn 'GINULIANGIAM_ENV' internal/` → 0）。
- **决策 5/6 落地与裸读收敛**：config 包（`env_*.go`）为核心业务配置唯一事实源入口（读取点数量以 `grep -rn 'os\.Getenv' internal/platform/config/ --include='*.go'` 输出为准）；SMTP/SENDGRID/RESEND 三大邮件 Provider 已提升为 config getter；可选集成保持 `envOrEmpty`；`envInt` 已废弃（代码零引用，保留定义仅为类型安全）。按决策 5 门禁扫描（复核 2026-11-21），生产代码残留裸读点已收敛为零：原四处残留文件（`internal/app/auditdenial/recorder.go`、`internal/platform/crypto/kms/local_env.go`、`internal/platform/webauthn/config.go`、`internal/platform/sqlguard/sqlguard.go`）均改经 config getter 读取（`RateLimitAuditDenialRate` / `EncryptionKeyCurrent|Legacy|Previous` / `IsDevelopment` / `SQLGuardMode`）；唯一豁免读取点为 `platform/db/migrate.go` 的迁移维护作业（决策 5 门禁显式排除）。
- 变量登记唯一源：手册附录 D；`.env.example`、`docker-compose.yml`、`k8s.yaml` 已按新契约提供 `APP_ENV`、`ENCRYPTION_KEY_CURRENT`；`SESSION_HMAC_KEY` 已退役，退役标记见 `contracts/registries/env.yaml`。

## 复审条件

新增任何环境标识或密钥变量、或 `platform/config` 校验入口被拆分时复审。决策 5 门禁的残留裸读点已于 2026-11-21 收敛为零，[Open] 偏差关闭。
