# ADR-023: 多方言抽象与 MySQL 镜像边界

- 状态：[Current]（PostgreSQL 是唯一产品数据库；MySQL 作为镜像方言保留在 infra 层，运行期已可启动并有真实 MySQL 8.4 集成取证：`OPEN-API-37`/`OPEN-API-73`/`OPEN-API-74` 均已关闭，仓储/schema 一致性启动探针保留；能力边界差异见 `contracts/registries/open-items.yaml` 与 `docs/architecture/data-model.md`）
- 日期：2026-10-29（补齐长期被引用但缺失的 ADR 编号；决策内容与 ADR-002 / ADR-022 / 技术选型文档一致）

## 背景

项目在 `internal/infra/dialect` 与 `internal/infra/mysql/**` 建立了「同一仓储接口、双方言实现」的镜像层，`contracts/schema/*.yaml` 也同时生成 PostgreSQL 与 MySQL DDL（ADR-022）。这套结构带来两个必须写清楚的边界：

1. **产品数据库只有一个**。`docs/architecture/technology-selection.md` 已排除 MySQL 作为主库：它没有 RLS，JSON/分区能力弱，无法承载本项目的租户隔离与审计要求。多方言的价值是「能力探测 + 迁移可行性验证」，不是「功能等价承诺」。
2. **镜像实现如果没有真实运行期验证，就会退化为编译期幻觉**。2026-10-29 的 MySQL 运行期验证发现：`internal/infra/mysql/users/sql.go` 仍按早期「users 扁平列」编写（`last_signin_wrong_time` / `preferred_mfa_type` / `totp_secret` / `balance_currency` 等），而生成器按声明式 Schema 产出的是规范化表（`user_signin_tracking` / `user_mfa_config` / `user_financials` / `user_collections` / `user_custom_attributes`）。真实 MySQL 8.4 上首个身份请求直接返回 `Error 1054 Unknown column 'last_signin_wrong_time'`。既有门禁 `check-schema-parity.py` 只比较声明式与生成物，不检查 Go 仓储 SQL，因此该漂移长期不可见。

## 决策

1. **PostgreSQL 是唯一受支持的运行期数据库**。所有部署、安全（RLS / SECURITY DEFINER / 审计哈希链）、迁移版本链（`db/postgresql/V{N}__*.sql`）以 PostgreSQL 为准。
2. **MySQL 是 infra 层镜像方言**：
   - 仓储接口、UoW 端口、`mysqluow` 子仓储必须与 PostgreSQL 实现同接口同语义（`docs/architecture/layers.md` B8）。
   - 生成 DDL（`db/mysql/V1__guliangiam_schema_init.sql`）是 MySQL 结构唯一事实源；**Go 仓储 SQL 是它的消费者，不是第二事实源**。
   - 不承诺 MySQL 具备 RLS、审计哈希链（ADR-031）、平台管理员跨租户仓储等 PostgreSQL 独占能力；缺失能力必须返回明确的「能力不支持」，不得伪造。
3. **镜像的启用条件是运行期证据，不是编译通过**：
   - 新增或修改 `internal/infra/mysql/**` 的仓储 SQL 时，必须在真实 MySQL 8.4 + 生成 DDL 上跑通至少一条读、一条写与一条拒绝路径。
   - 组合根在 MySQL 启动期执行仓储/schema 一致性探针（当前为 `internal/infra/mysql/users/users.Preflight`）；探针失败时**拒绝启动**并给出开放项编号，而不是带病提供服务。
   - 探针随模块逐步推广；OPEN-API-37 关闭的条件是身份链路与非身份链路（AI Gateway / payments / audit / SSF）全部通过，并移除启动阻断。（现状：该条件已满足，`OPEN-API-37` 及拆分的 `OPEN-API-73`/`OPEN-API-74` 均已关闭；组合根移除了无条件拒绝启动，保留 `Preflight` 一致性探针作为 fail-fast 校验，见 `contracts/registries/open-items.yaml`。）
4. **方言能力探测保持 fail-closed**（ADR-022 第 4 条第 10 项）：缺失 RLS、部分唯一索引、`SKIP LOCKED`、JSON、数组等能力时，必须落地等价方案并登记 ADR，或拒绝启动；禁止静默降级安全能力。

以下两节为决策当日（2026-10-30 / 2026-10-31）的阶段快照，其中「`OPEN-API-37` 保持开放」「拒绝启动完整 MySQL 服务」等状态描述仅反映当时事实；现行状态以本文头部与 `contracts/registries/open-items.yaml` 为准。

### 2026-10-30 第一阶段进展（身份仓储）

- `internal/infra/mysql/users` 已按声明的 5 张 `user_*` 子表重写读/写/分页/锁：聚合 SELECT 使用 LEFT JOIN + COALESCE，写入使用「主表 core INSERT/UPDATE + 5 条子表 `ON DUPLICATE KEY UPDATE`」，乐观锁 `WHERE revision = user.Revision-1` 与 PostgreSQL 语义一致。
- 证据：`internal/infra/mysql/users/integration_test.go` 在真实 MySQL 8.4 + 生成 DDL 上完成 Create → GetByID → List → Update（过期 revision 返回 conflict）→ Delete → `Preflight`/`PreflightWrite`；sqlmock 单测锁定列顺序与参数数量。
- 边界：AI Gateway 仓储、payments 对账、authorization audit 与 SSF 投递仍为 PostgreSQL-only；组合根继续拒绝启动完整 MySQL 服务（受控 `unavailable`），`OPEN-API-37` 保持开放。

### 2026-10-31 第二阶段进展（Casbin 策略存储）

- `internal/infra/casbin.Manager` 不再直接持有 pgx `Adapter`，改为依赖 `PolicyStore` 端口（`internal/infra/casbin/policy_store.go`）：策略读写五个方法 + 角色分组投影 + JIT 时间窗 + enforcer 解析。PostgreSQL 由 `Adapter` 实现，MySQL 由新增的 `internal/infra/mysql/casbin.Store` 实现。
- MySQL 实现的差异仅在方言层：无 `guliangiam` schema 前缀、无 RLS（解析 enforcer 不需要 GUC）、ID 由 Go 生成；JIT 的 `v4`（失效时间）与 `v5`（JSON 元数据）是 VARCHAR，故候选行取出后在 Go 内求值（lifecycle 匹配、`startsAt <= now < expiresAt`），不依赖 `timestamptz`/`jsonb` 运算。
- 证据：`internal/infra/mysql/casbin/integration_test.go` 在真实 MySQL 8.4 + 生成 DDL 上完成 SavePolicy → LoadPolicy → Add/RemovePolicy → ReplaceUserGroupings（两轮收敛）→ JIT 窗口放行/过期/生命周期不匹配 → LoadPolicy 排除 JIT 行 → enforcer 解析，并用真实 `casbin.NewEnforcer` 消费该 store 执行 Enforce；sqlmock 单测覆盖参数数量与通配删除。
- 边界：组合根按 driver 注入存储（`newCasbinPolicyStore`），但完整 MySQL 运行仍因 AI Gateway / payments recon / audit / SSF 被拒绝启动，`OPEN-API-37` 保持开放。

## 后果

- `docs/architecture/data-model.md` 的多方言能力矩阵是 MySQL 边界的唯一事实源；新增差异必须先更新矩阵与开放项，再改代码。
- `DB_DRIVER=mysql` 可正常启动完整服务（`internal/bootstrap/runtime_factory.go` 已移除无条件拒绝）；仓储/schema 一致性探针（`mysqlusers.Preflight`）不匹配时仍受控 fail-fast（`unavailable`，非 panic）——在镜像 SQL 与 DDL 漂移时拒绝带病提供服务是**设计内行为**。
- 已修复的 nil-pool 路径（Outbox ChainWorker、WAF 规则仓储、PoolGuard、`/readyz`）仍然保留：它们保证任意方言下都不会因缺少 pgx 池而崩溃，且 PostgreSQL 行为不变。
- MySQL 不因此获得 RLS、SECURITY DEFINER、分区等 PostgreSQL 独占能力；租户隔离在 MySQL 侧始终依赖仓储显式 `organization_id` 过滤。

## 替代方案

| 方案 | 内容 | 否决理由 |
| --- | --- | --- |
| MySQL 作为主库 | 双主或 MySQL 优先 | 无 RLS、JSON/分区能力弱，无法承载租户隔离与审计哈希链要求（`docs/architecture/technology-selection.md`） |
| 只保编译期镜像 | 镜像仓储通过编译即视为可用 | 2026-10-29 的 `Unknown column` 事故证明编译期幻觉不可接受；启用条件必须是真实运行期证据 |
| 删除 MySQL 镜像 | 只维护 PostgreSQL | 放弃迁移可行性验证与能力探测价值；保留镜像但以运行期证据约束边界 |

## 验证证据

以 `contracts/registries/open-items.yaml` 的 `OPEN-API-37`/`OPEN-API-73`/`OPEN-API-74` 闭环记录为事实源（核验日期 2026-11-21）：

| 检查点 | 事实源/命令 | 结论 |
| --- | --- | --- |
| MySQL 运行期组合根可启动 | `internal/bootstrap/mysql_runtime_test.go: TestMySQLRuntimeStarts`（真实 MySQL 8.4.13 + 生成 DDL） | PASS ✅ |
| 授权审计哈希链 / AI Gateway provider / payments / SSF receiver 仓储 | `TestAppendMySQLHashChain`、`TestProviderRepositoryMySQL`、`TestChannelConfigRepositoryMySQL`、`TestReconBatchRepositoryMySQL`、`TestReceiverRepositoryMySQL` | PASS ✅ |
| AI Gateway completion（成功/失败路径，usage 落 MySQL） | `internal/app/aigateway/gateway_mysql_integration_test.go`（`OPEN-API-73`） | PASS ✅ |
| SSF 投递经 MySQL receiver 注册表 | `internal/outbound/ssf/handler_mysql_integration_test.go`（`OPEN-API-74`） | PASS ✅ |
| 运行期重放脚本 | `bash scripts/dev/verify_mysql_runtime.sh`（隔离 MySQL） | 可重放 ✅ |
| 启动探针 fail-fast | `runtime_factory.go`：`mysqlusers.Preflight` 不匹配 → 受控 `unavailable` | 保留 ✅ |

## 复审条件

1. MySQL 成为合规部署目标（而不仅是镜像验证）→ 必须先修订本 ADR 与 ADR-031，补齐能力等价的正式承诺与证据。
2. 镜像 SQL 与生成 DDL 再次漂移（探针 fail-fast 触发）→ 按第 3 条决策补真实 MySQL 读/写/拒绝路径证据后修复。
3. 新增 PostgreSQL 独占能力的依赖 → 先更新 `docs/architecture/data-model.md` 能力矩阵与开放项，再改代码。
