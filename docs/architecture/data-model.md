# data-model — 数据模型总纲 + 五条链路闭环

> 目标态规范「data-model — 数据模型总纲 + 五条链路闭环」；写作约束见 [docs/README.md](../README.md)。

## 1. 数据库约定

完整规范见《GuliangIAM 开发规范手册》第 3 章「数据库设计规范」；结构与迁移的事实源模型见 ADR-022（声明式 Schema 唯一源 + 版本化迁移）。

核心约定（逻辑层面）：

| 约定 | 规则 | 事实源 |
|------|------|--------|
| Schema | `guliangiam`（不使用历史 `iam` schema） | ADR-022 |
| 主键 | `id UUID NOT NULL`（UUIDv7，应用层生成，无数据库默认值，无方言函数） | ADR-007 |
| 租户列 | 所有租户表必须 `organization_id UUID NOT NULL` | 《开发规范手册》隔离分级注册表 |
| 软删除 | `deleted_at TIMESTAMPTZ NULL`；活跃唯一性由数据库强制（部分唯一索引） | ADR-016 |
| 乐观锁 | `revision BIGINT NOT NULL DEFAULT 1`；更新时以 `revision` 作为 `WHERE` 条件 | — |
| 状态列 | 统一列名 `status`；`state` 仅作存量例外登记（`contracts/registries/naming.yaml` exceptions），禁止新增；取值全小写 snake_case，DDL 有 `CHECK` | `contracts/registries/naming.yaml` |
| 外键 | 核心强引用必须建 FK 并显式声明 `ON DELETE`（归属 CASCADE / 可选 SET NULL / 核心事实 RESTRICT） | — |
| 金额 | `DECIMAL(19,4)`（never `DOUBLE PRECISION`），币种 `VARCHAR(3)` | — |
| 时间 | 逻辑类型 `TIMESTAMP_TZ`，统一 UTC | — |
| 高增长表 | 预留时间分区设计（分区键进入唯一约束设计） | ADR-011 |
| 宽表拆分 | 身份主数据、凭据、Profile 分离存储；Webhook 投递队列统一走 outbox，`webhook_events` 仅承载投递结果与重试状态跟踪，不作为投递队列 | — |

> 具体表列清单、索引定义、DDL 结构不在本文书写。事实源：`contracts/schema/*.yaml`（复现命令：`ls contracts/schema/*.yaml`）。

## 2. 迁移策略

结构事实源与迁移链遵循 [ADR-022](../adr/ADR-022-declarative-schema-versioned-migration.md)：声明式 Schema（`contracts/schema/*.yaml`）是唯一结构源，DDL、版本化迁移与回滚脚本由生成器产出且禁止手改。

- 迁移链规则、生成器与 CI 结构门禁以 ADR-022 决策 3/4 为准；表族清单以 `contracts/schema/*.yaml` 为准（复现：`ls contracts/schema/*.yaml`）
- 种子数据为人工编写的版本化 SQL（无声明层、无生成器，ADR-022 决策 6）；`AUTO_SEED` 控制是否注入，生产默认关闭。V1 基线覆盖内置组织/应用、管理员与 platform admin、角色/分组/权限、Provider、商业目录（product/plan/pricing/coupon）、AI 网关目录（provider/model/route 不可调用，alias 保留用于「无路由」分支）与组织子表；管理员凭据优先级为环境口令 > 开发哨兵，且强制首登改密（见 `AGENTS.md` §3）。服务启动默认不重放迁移、不导入种子，需要时显式使用 `-migrate`/`-seed`（或 `AUTO_MIGRATE`/`AUTO_SEED=true`）
- MySQL 侧审计哈希链（`records` 的 `LoadLastChainEntry` / `LoadAnchorSeq` / `InsertAnchor` / `ListChainEntries`）不提供（`NewUnimplemented`，ADR-031「MySQL 方言的审计链边界」；开放项 `OPEN-API-35`，状态见 `contracts/registries/open-items.yaml`）：MySQL 定位为事务/业务数据面，链校验与锚定只在 PostgreSQL 部署提供

## 3. 持久化架构

- **驱动与写法**：PostgreSQL 18+ 基线 `pgx/v5`，直接使用参数化 SQL，无 ORM（ADR-006）；每个聚合一个仓储包，SQL 常量集中在各包的 `sql.go`
- **仓储目录**：`internal/infra/postgres/{module}/`（PG 基线实现），Domain 端口接口定义在 `internal/domain/{module}/repository.go`
- **事务**：由 App 层通过 Unit of Work（UOW）显式开启/提交，仓储通过传入的事务参与
- **一次性消费语义**：一次性凭据使用原子 `DELETE ... RETURNING`，保证同一凭据不会被消费两次

### 3.1 租户隔离与跨租户过滤（ADR-016 / ADR-036）

| 请求形态 | 过滤行为 | 适用场景 |
|------|----------|----------|
| `Strict`（默认） | orgId empty → 拒绝（fail-close） | 普通租户请求 |
| `Optional` | orgId empty → 无过滤（仅限显式登记的全局资源） | 跨组织只读查找 |
| 框架旁路 | `system` 内部任务，或平台管理员无显式 target 的全租户总览 | Worker/Cron/迁移、全局 dashboard；必须伴随审计 |

**目标态总则**：受保护请求默认 Strict；显式跨租户必须经 ScopeAuthorizer 批准并落到单一目标租户，按 `TenantFilterCross` 精确过滤，**不以角色名 `super_admin` 放行全库**（C1，`OPEN-API-42`；见 [ADR-036](../adr/ADR-036-scope-aware-rls-hybrid.md) 与 [permissions.md](../design/iam/permissions.md) §1.2）。租户上下文经 `set_config(..., true)` 注入事务级（LOCAL）作用域，事务结束自动失效，并由 RLS 策略兜底；连接池 `AfterConnect` 钩子以 SESSION 级空值兜底（fail-closed，无作用域即空结果；`internal/platform/db/rls.go`）。多租户集合 / scope 数组暂缓，不进入 GUC 契约。

#### 短期状态存储矩阵

所有跨实例的短期状态收敛为「可重建 + 共享 + TTL 上界」三条件；每条声明存储位置、TTL、失败姿态与清理归属：

| 状态 | 存储 | TTL | 失败姿态 | 清理 | 多区域策略 |
|------|------|-----|---------|------|-----------|
| DPoP jti 去重 | PostgreSQL / MySQL `dpop_replay`（跨实例共享，唯一键 jti） | 3min（proof max age + 时钟偏移） | 存储不可用 → DPoP 请求 fail-close | 插入触发器清 3min 前记录 | 每区域独立；跨区域访问按重新认证处理 |
| 账户锁定 | PostgreSQL（唯一事实源） | 按组织级配置的锁定/观察窗 | PG 不可用 → 认证请求 fail-close | 锁定窗口到期由状态机原子复位（`lockout_attempts`） | 每区域独立；跨区域登录触发本地锁定检查 + 控制面注册表查询 |
| 限流计数 | Redis（令牌桶） | 窗口长度 + 缓冲 | 认证面/协议面 fail-close；管理面只读 fail-open | TTL 自动 | 每区域独立；区域间配额按 region_quota 登记表分配 |
| 幂等响应缓存 | PostgreSQL | 24h | PG 不可用 → 拒绝写请求（fail-close） | 定时任务分批清理 | 每区域独立部署（库级隔离） |
| 会话吊销广播 | Redis Pub/Sub + PostgreSQL（事实源） | 广播瞬时；PG 保留至 TTL | Redis 不可用 → PG 周期兜底（上界 30s） | TTL 自动 | 跨区域经控制面 Outbox 扇出 |

#### bridged schema 下的 RLS

`dedicated`（独立库）/ `shared`（共享表 + RLS）/ `bridged`（每租户一 schema）三模式下 RLS 语义不同：bridged 模式在每个租户 schema 内独立创建策略，谓词固定为该 schema 的 `organization_id`，连接层按租户路由 `search_path`；`set_config` 仍设置但不再作为唯一隔离依据。

#### 多方言能力矩阵

声明式 Schema 与生成 DDL 覆盖 PostgreSQL / MySQL / GaussDB / KingbaseES，但**数据结构等价不等于能力等价**。以下能力只有 PostgreSQL 实现，使用其他方言时属受控降级，必须在部署说明与开放项中登记，不得宣称「全方言等价」：

| 能力 | PostgreSQL | MySQL | 说明与事实源 |
|---|---|---|---|
| 行级安全（RLS ENABLE + FORCE + 策略） | 全部租户表 | 无 RLS，靠仓储显式 `organization_id` 过滤 | 策略内容门禁：`scripts/check-rls-policy-content.py`（含运行期 `pg_policies` 校验） |
| 平台管理员跨租户（`platform_admins` / `platform_admin_scopes` / `cross_tenant_operations`） | 由 `is_super_admin()` + SECURITY DEFINER 支撑 | 以 `NewUnimplemented` 返回设计内降级 | `internal/infra/mysql/uow` |
| 审计哈希链（`records` 链校验与锚点） | 链插入 + 锚定 + 校验 | 不提供（`NewUnimplemented`，ADR-031 MySQL 能力边界决策） | `internal/infra/mysql/records`；ADR-031「MySQL 方言的审计链边界」 |
| 显式 `FOR UPDATE SKIP LOCKED` 队列消费 | 支持 | 由方言实现等价语义 | `internal/platform/db` dialect 能力位 |
| Dashboard / LDAP Sync | 直接注入 pgx 池 | 构造期注入「能力不可用」实现：请求返回 503 `unavailable`，LDAP Sync scheduler 不启动（无 nil-pointer panic） | 开放项 `OPEN-API-36`（`contracts/registries/open-items.yaml`）；`internal/bootstrap/unavailable_capabilities.go` |
| SCIM 服务 | `appscim.NewService(unitOfWork)` | 同样走 UoW：`mysqluow` 提供 `Users()` / `Groups()`，可用 | `internal/bootstrap/wire_transport.go` |
| 完整服务运行期 | 受支持（唯一部署目标） | 不支持：identity 仓储（users）与 Casbin 策略存储（`internal/infra/mysql/casbin`）已与规范化 DDL 对齐，并有真实 MySQL 集成用例；AI Gateway 仓储 / payments 对账 / authorization audit / SSF 仍为 PostgreSQL-only，启动受控拒绝（非 panic） | `internal/infra/mysql/users/integration_test.go`；`internal/infra/mysql/casbin/integration_test.go`；`internal/bootstrap/runtime_factory.go`；ADR-023；开放项 `OPEN-API-37` |

嵌套事务：App 层的 `WithinTransaction` 允许嵌套调用。PostgreSQL 用 savepoint 语义；MySQL 侧在事务内再次调用时同样以 `SAVEPOINT` / `ROLLBACK TO SAVEPOINT` / `RELEASE SAVEPOINT` 实现独立回滚边界（`internal/infra/mysql/uow`，回归用例 `TestWithinTransaction_Nested*`）。

### 3.2 Repository 约定

所有 infra repository 模块遵循 4 文件约定：

| File | Responsibility |
|------|---------------|
| `models.go` | DB 行模型文档 |
| `repository.go` | CRUD 方法实现 |
| `sql.go` | SQL 查询常量（按 domain cluster 可拆分为 `sql_core_get.go` / `sql_writes.go` 等） |
| `mapper.go` | Scan/convert 函数 |

**文件拆分要求**：SQL 常量超 500 行时按查询域拆分（`sql_core_get.go` / `sql_writes.go` / `sql_list.go` / `sql_misc.go` / `sql_provider.go`，示例见 `internal/infra/postgres/applications/`）；外部渠道适配器按渠道独立文件（示例见 `internal/infra/messaging/`）；Bootstrap 基础设施 helper 按关注点拆分（`runtime_storage.go` / `runtime_messaging.go` / `runtime_captcha.go` / `runtime_notification.go`）。拆分以零行为变化为前提，由既有回归测试验证。

### 3.3 能力基线：PostgreSQL 18+（RLS 强制）

PostgreSQL 18+ 是唯一受支持的数据库。其 RLS、`SKIP LOCKED`、部分索引、`ON CONFLICT`/`RETURNING`、GIN/JSONB、数组、`DISTINCT ON`、`ILIKE`、:: 转型等语义均直接依赖。声明式 Schema（`contracts/schema/*.yaml`）是唯一结构源，DDL 由生成器产出，禁止手写维护。

### 3.4 引用完整性（外键 vs 逻辑引用）

**原则**：租户内 OLTP 关系一律用**物理外键**表达，完整性由数据库保证；只有「无法用单表单列 FK 表达」或「引用对象在系统边界之外」的引用才允许降级为**逻辑引用**，且必须在 `contracts/registries/referential-integrity.yaml`（渲染层 `docs/generated/referential-integrity-registry.md`）登记分类与理由。门禁 `scripts/check-fk-coverage.py` 强制该裁决。

| 列形态 | 要求 | 理由 |
|--------|------|------|
| UUID 类型 + `_id` 后缀 | 必须被声明式 Schema 的 `fk_constraints` 覆盖，或在裁决表登记为 `logical_references` / `pending_physical_fks` | 数据库级完整性是孤儿行/悬空引用的最后防线；租户列缺失 FK 时 RLS 之外的引用完整性无保证 |
| 非 UUID 的文本/字节类型 + `_id` 后缀 | 视为**外部系统标识**（OAuth `client_id`、第三方流水号、票据号），不建物理 FK，由应用层在写入前校验格式与存在性 | 外部系统不在本库事务边界内，物理 FK 无法表达且会阻断合法外部数据 |
| 非 UUID/非文本类型 + `_id` 后缀 | 命名异常：后缀宣称是标识符但语义不是；登记开放项后按版本化迁移重命名 | 命名规范要求标识符后缀只用于标识符（布尔列用 `is_`/`has_` 前缀） |
| 多态引用（目标表由类型列决定，如 `outbox.aggregate_id`） | 登记为 `logical_references`，写明应用层校验点 | 单表物理 FK 无法表达多态目标；以事件幂等键与写入前校验兜底 |

**外键动作语义（必须显式声明，禁止依赖数据库默认 NO ACTION）**：

- 租户级联删除：租户列 → `ON DELETE CASCADE`（租户注销时数据随主体清理）；
- 可选引用（如站点→证书）：`ON DELETE SET NULL`，避免删除被引用对象时连带删除业务主体；
- 包含关系（如分组树父节点）：`ON DELETE RESTRICT`，删除前必须显式处理子节点，禁止静默孤儿化；
- 令牌族等自引用：`ON DELETE CASCADE`，族根删除即整族失效。

**trade-off（为何不是「所有引用都建 FK」）**：物理 FK 会带来写放大、行锁竞争与跨区域 DDL 协调成本；对高频写入的事件/用量表，过度外键化会放大热点。结论：**租户内业务实体关系坚持物理 FK**（正确性优先），**外部标识与多态引用用登记制逻辑引用 + 应用层校验**（可扩展性优先），分类裁决以 `contracts/registries/referential-integrity.yaml` 与门禁 `scripts/check-fk-coverage.py` 为准。

## 4. 五条链路闭环

### 4.1 写路径（Writes） — [REQ:DATA-WR-001]

```text
请求
→ 认证 → 租户解析 → 授权（fail-close）
→ 幂等检查（Idempotency-Key，ADR-018）
→ 乐观锁检查（ETag/If-Match，ADR-018）
→ BEGIN 事务（事务级 set_config 注入租户上下文）
→ 写业务聚合（领域校验 + 状态机迁移）
→ 写 Outbox 事件（适用域）
→ 写 records 审计记录（正常路径同事务计算链列并写 `chain_status='chained'`；写入器失败/熔断时登记 `pending` 交 Worker 补链，ADR-031）
→ 写幂等响应缓存（PostgreSQL `idempotency_keys`，同事务；TTL 24h，由定时任务分批清理）
→ COMMIT
→ 提交后失效缓存（事件驱动 + TTL 上界兜底）
→ Outbox Worker 异步投递（退避/死信/人工重放，ADR-008）
```

**写路径失败语义表**：

| # | 失败描述 | 影响范围 | AC/ADR 锚点 | 恢复策略 |
|---|----------|----------|-------------|----------|
| 1 | 业务校验失败（状态机迁移不合法 / 字段约束违反 / 领域规则冲突） | 单次请求被拒绝；无业务变更落盘 | ADR-018 §幂等校验 | 事务回滚；幂等记录置 `failed`；返回 422 + 字段级错误详情 + request_id |
| 2 | 审计记录写入失败（records 表 INSERT 触发唯一约束 / 哈希链连续性校验失败 / DB 连接中断） | **事务整体回滚**，业务变更不落盘 | ADR-031 §审计同事务 | 审计与业务同生共死，不产生无审计的变更；返回 503；告警通知审计员 |
| 3 | 乐观锁冲突（ETag/If-Match 校验不通过 / `revision` 落后当前值） | 并发写被拒绝 | ADR-018 §乐观锁 | 返回 412 + 当前 `revision` 值，客户端可合并后重试 |
| 4 | 提交后缓存失效失败（Redis 不可用 / 事件驱动消息丢失） | 读取侧暂存旧值；TTL 上界内自然过期 | ADR-010 §缓存策略 | 事件驱动补偿（重试投递 outbox 事件）+ TTL 上界兜底 |
| 5 | Outbox Worker 投递失败（下游 5xx / 网络中断 / 超时） | 异步事件延迟；不阻塞主链路响应 | ADR-008 §Outbox | 指数退避重试（max_attempts 以 `contracts/registries/outbox.yaml` 为准）→ 超过上限转入死信队列 → 人工修复重放；手动重放幂等键防重复投递（ADR-018） |
| 6 | 事务 COMMIT 失败（DB 主从切换 / 磁盘满 / 网络抖动） | 业务变更落盘状态不确定 | ADR-022 §声明式 Schema | 返回 503 + request_id；幂等客户端可安全重试；服务器端在 30s 内扫描待确认幂等记录并确认最终状态 |

**模板红线**：幂等优先于乐观锁；审计与业务同事务；请求路径内禁止任何同步外呼；高风险写操作响应附 `auditId`。

**审计链写入与异步补链语义**：正常路径在同一业务事务内计算链列并以 `chain_status='chained'` 落库（首条按部署档位取 `sha256`/`sm3`，后续继承链尾 `chain_algo`）；同链写入以 `pg_advisory_xact_lock(hashtext(chain_id))` 持锁串行，事务结束自动释放。写入器失败时审计转本地暂存、业务事务照常提交；熔断时新条目登记 `chain_status='pending'`，由链构建 Worker 在独立事务中补链。锚定由 Worker 按锚定周期把链尾快照写入 `audit_chain_anchors` 并推进 `chained → anchored`；链校验按 `entry_hash = H_chain_algo(canonical)` 重算，发现 `prev_hash` 与上一条 `entry_hash` 不连续时置 `chain_status='broken'` 并触发安全事件。状态机 `pending → chained → anchored`、异常分支 `broken`；链列、约束、`canonical` 构成、锚定/验证周期与断链处置以 `contracts/schema/60_audit.yaml` 与 [审计链设计](../design/infrastructure/audit.md) 为准。

### 4.2 读路径（Reads） — [REQ:DATA-RD-001]

```text
请求
→ 认证 → 租户解析 → 授权（同写路径前三步）
→ 缓存查询（键必须含 organization_id + 主体 + 资源 + 策略版本）
→ 命中 → 返回
→ 未命中 → single-flight 合并并发回源 → PostgreSQL（RLS 兜底）
→ 回填缓存 → 返回
```

**读路径失败语义表**：

| # | 失败描述 | 影响范围 | AC/ADR 锚点 | 恢复策略 |
|---|----------|----------|-------------|----------|
| 1 | Redis 缓存完全不可用（连接池耗尽 / 主从切换 / 网络分区） | 所有读请求回源 PostgreSQL；读延迟上升 | ADR-010 §Fail-Close | **授权/撤销类请求保守拒绝（fail-close）**，不允许本应拒绝的请求因缓存不可用而放行；纯只读列表/详情降级回源；告警通知运维恢复 Redis |
| 2 | RLS 策略错误或缺失（策略注册不完整 / 租户上下文 set_config 失效） | 受保护请求可能越权；或正常请求被误拒绝 | ADR-016 §租户隔离 | 启动自检拒绝不完整策略注册；运行期 RLS 违规抛出并记审计；返回 503 + 告警；人工检查并修复策略后重启 |
| 3 | single-flight 合并失败（底层 group.Do panic / 超时） | 并发请求全部等待超时；大量 goroutine 阻塞 | ADR-010 §single-flight | 单次 panic recover 后降级为每个请求独立回源；全局超时 5s；告警通知；压测验证 group.Do 稳定性 |
| 4 | PostgreSQL 回源失败（主库宕机 / 只读副本延迟过高 / 查询超时） | 读请求无法拿到权威数据 | ADR-006 §pgx | 只读请求自动切只读副本；超过 P99 阈值返回 504；紧急切主或扩容只读副本 |

**读路径红线**：

| 红线 | 说明 | ADR |
|------|------|-----|
| 缓存只缓存可重建数据 | 用户/授权状态的权威存储永远是 PostgreSQL | ADR-010 |
| 缓存不可用时回源 | **授权/撤销类 fail-close**（缓存不可用不得放行本应拒绝的请求） | ADR-010 |
| 吊销必须事件驱动立即失效 | 缓存 TTL 只是上界，不是失效机制 | ADR-010 |
| 一次性消费凭据不进缓存 | `DELETE ... RETURNING` 或条件 `UPDATE ... WHERE consumed` 原子消费 | ADR-006 |
| 令牌轮换按 `family_id` | 重用检测命中即整族撤销 | 见《security-model》 |

### 4.3 异步链路（Async） — [REQ:DATA-AS-001]

所有外部副作用（邮件、短信、Webhook、SCIM 同步、支付回调）统一走 Outbox + Worker 模式：

```text
写事务内 → 写 outbox 事件（同事务，ADR-008）
→ 事务 COMMIT
→ Outbox Worker 领约（SKIP LOCKED）
→ 分发到对应通道
→ 指数退避重试（max_attempts 以 registries/outbox.yaml 为准）
→ 超过最大重试 → 死信 → 人工重放
```

**异步链路失败语义表**：

| # | 失败描述 | 影响范围 | AC/ADR 锚点 | 恢复策略 |
|---|----------|----------|-------------|----------|
| 1 | Worker 消费失败（下游 5xx / 网络超时 / 载荷解析失败） | 该事件投递延迟；同批次后续事件被阻塞（取决于 Worker 并发度） | ADR-008 §Outbox | 指数退避重试（初始 1s，翻倍至上限 60s）；重试次数与间隔由状态机注册表定义；每次失败记 `records` 审计并告警 |
| 2 | 超过最大重试次数仍失败 | 该事件永久丢失风险；业务流程（如注册欢迎邮件、Webhook 通知）中断 | ADR-008 §死信 | 进入死信队列，标记 `dead_letter=true`；死信事件触发 PagerDuty/企业 IM 严重告警；运维通过管理端事件重放接口手动重试（幂等键防止重复投递） |
| 3 | Outbox Worker 自身不可用（进程崩溃 / 资源耗尽 / 调度器挂掉） | 所有待投递事件积压；延迟持续增长 | ADR-008 §Worker | 多实例部署 + 健康检查；Worker 进程崩溃由 supervisor 自动拉起；积压超阈值（> 1000 条 / 10min 未消）自动告警 |
| 4 | Outbox 表被截断或损坏 | 未投递的事件永久丢失 | ADR-022 §声明式 Schema | Outbox 表作为业务事务同事务写入，不单独备份；定期基于 Binlog/事务日志恢复；审计链中记录 outbox 写入时间戳可辅助识别 |
| 5 | 手动重放触发下游幂等键冲突 | 下游已收到同内容事件但返回幂等冲突 | ADR-018 §幂等 | Worker 识别下游幂等冲突（HTTP 409 或 `X-Duplicate: true` header），置 `status=confirmed` 跳过后续重试；避免手动重放导致重复执行 |

> 请求路径内禁止同步外呼。Outbox 是唯一的异步投递入口。

### 4.4 审计链路（Audit） — [REQ:DATA-AU-001]

```text
写事务内 → 写业务事实
→ 写 records 审计记录（同事务）
→ 审计记录含哈希链列（结构以 `contracts/schema/60_audit.yaml` 为准，语义见[审计链设计](../design/infrastructure/audit.md)，ADR-031）
→ 按锚定周期对链尾做 `audit_chain_anchors` 快照锚定（锚定来源与周期见[审计链设计](../design/infrastructure/audit.md)）
→ WORM 归档（不可篡改）
```

**审计链路失败语义表**：

| # | 失败描述 | 影响范围 | AC/ADR 锚点 | 恢复策略 |
|---|----------|----------|-------------|----------|
| 1 | 审计记录写入失败（records 表 INSERT 唯一约束冲突 / 链哈希校验不通过 / DB 连接中断） | **事务整体回滚**，业务变更不落盘 | ADR-031 §审计同事务 | 审计与业务同生共死，不产生无审计的变更；返回 503 + 告警通知安全运营组；审计写入失败计数超阈值自动升级 |
| 2 | 哈希链断裂（链校验工具发现某条记录的 prev_hash 不匹配前条 entry_hash） | 审计完整性被破坏；合规报告不可信 | ADR-031 §哈希链 | 链校验工具按验证周期扫描（区分 `chain_status='pending'` 与真断链）；断裂记录标记 `chain_status='broken'` 并触发安全事件；安全运营组人工介入判定篡改来源 |
| 3 | 链锚定失败（按锚定周期写 `audit_chain_anchors` 失败） | 链尾无锚点；抗事后篡改能力暂时缺失 | ADR-031 §链锚定 | 继续运行（链内哈希仍自校验）；链锚定重试（3 次指数退避）；超阈值告警审计员；锚定记录待重放队列待恢复后重锚 |
| 4 | WORM 归档失败（对象存储不可用 / Write-Once-Read-Many 设备故障） | 热数据保留期过后无法归档；合规审计取证受影响 | ADR-031 §WORM | 归档写入进入重试队列（独立 Worker）；超过归档窗口保留在热表；告警审计员紧急恢复归档设备 |

**审计防篡改**：每条 `records` 记录含前条哈希，链尾按锚定周期快照锚定（`audit_chain_anchors`，外部时间戳回执可注入 `proof`）；链校验工具按 `H_chain_algo(canonical)` 重算并定位首个失配 `chain_seq`。

### 4.5 计量链路（Metering） — [REQ:DATA-MT-001]

```text
写路径旁路 → 写 metering_events Outbox（不阻塞主链路）
→ Metering Worker 聚合
→ metering_aggregates 账期记录（追加型 + 校验和）
→ 喂数 COMM 计费（边界以 SRS METER-005 为准）
```

**计量链路失败语义表**：

| # | 失败描述 | 影响范围 | AC/ADR 锚点 | 恢复策略 |
|---|----------|----------|-------------|----------|
| 1 | 计量事件 Outbox 写入失败（DB 连接中断 / metering_events 表不可用） | 该次操作不记录计量；可能导致计费少计 | METER-001 §旁路写入 | **不阻塞主链路**（fail-open 计量）；记录到应用结构化日志（含 operation_id + actor_id + resource_id + event_type），离线扫描补录；SLO 容忍 1% 以内计量漏计 |
| 2 | 聚合 Worker 处理失败（聚合逻辑 bug / metering_aggregates 写入冲突 / 校验和不匹配） | 账期汇总数值与原始事件不一致；可能导致计费多计或少计 | METER-003 §可重放 | 原始 metering_events 保留不删除；Worker 跳过失败事件继续处理后续；失败事件进入重试队列（独立 Worker）；最多保留 180 天原始事件，支持全量重放聚合 |
| 3 | 账期校验和不通过（聚合记录校验和与原始事件和不一致 / 账期跨日未结） | 该账期无法提交计费；影响客户账单 | METER-005 §争议账期 | 标记该账期为 `disputed`；以原始 metering_events 为准重新聚合；运营介入仲裁；人工修正后标记 `verified`；争议期暂缓扣费不影响客户使用 |
| 4 | 计费系统 COMM 接口失败（HTTP 5xx / 超时 / 载荷解析不通过） | 已聚合账期无法喂数；账单延迟出账 | METER-005 §计费喂数 | 指数退避重试（1min→5min→30min→2h）；超过 24h 告警计费对接组；喂数请求留痕，手动触发重喂；账期状态置 `feed_failed`，不进入客户账单 |

**计量链路红线**：幂等键 `event_uid` 防重；账期记录追加型；写入失败不阻塞主链路。

## 5. 事件和交互设计

- 所有高风险写操作返回资源版本或 `ETag`，使用 `If-Match` 防止并发覆盖
- 列表接口统一分页、排序白名单、过滤和稳定排序；大表优先 keyset pagination
- 外部 IdP、支付、通知和同步操作采用 operation id + 状态查询，禁止无限同步等待
- API 错误只暴露稳定错误码、字段错误和 request id，不暴露 SQL、堆栈或 Secret

### 域汇总

域、表族与接口面的**唯一事实源**分别是：表族 = `contracts/schema/*.yaml` 的文件分组（`ls contracts/schema/*.yaml` 即当前分组清单）；接口面与协议家族 = `contracts/registries/surfaces.yaml`（渲染层 `docs/generated/surfaces-registry.md`）；决策 = `docs/adr/README.md` 索引。**本节不写模块数/接口数/表数/ADR 数**——它们是可生成计数，手写必然漂移（[docs/README.md](../README.md) §1 红线 1）；需要精确数量时按「事实源路径 + 复现命令」现场得出。

| 域 | 职责边界 | 事实源入口 |
|----|----------|-----------|
| 租户与组织 | Organization、组织安全策略、计费/品牌/主题、部署拓扑 | `contracts/schema/10_tenant.yaml` |
| 身份 | 用户、Profile、凭据、MFA、外部身份绑定、Passkey 派生地址映射 | `contracts/schema/20_identity.yaml` |
| 授权 | RBAC（Casbin）、ReBAC 关系元组、ABAC 属性目录、资源与权限 | `contracts/schema/30_authz.yaml` |
| 会话 | Access/Refresh Token、Session、登录挑战、Passkey 凭证与挑战 | `contracts/schema/40_session.yaml` |
| 协议 | 授权码、consent、设备码、QR、CAS/Kerberos 票据 | `contracts/schema/50_protocol.yaml` |
| 审计 | records 哈希链、链锚点、密钥轮换审计 | `contracts/schema/60_audit.yaml` |
| 商业 | 产品、订单、支付、交易、订阅、Plan、优惠券 | `contracts/schema/70_commerce.yaml` |
| AI 网关 | LLM 供应商/模型/路由/API Key/配额/价格/用量 | `contracts/schema/75_ai_gateway.yaml` |
| 治理 | 邀请、证书、锁定与 IP 规则、MCP、RADIUS/LDAP 等接入治理 | `contracts/schema/80_governance.yaml` |
| 计量 | 计量事件与聚合（计费证据链输入） | `contracts/schema/90_metering.yaml` |
| 支撑 | 工单与表单（服务台/审批载体） | `contracts/schema/82_support.yaml` |
| Workload 身份 | 服务账号、身份池、联邦信任（非人主体，ADR-026） | `contracts/schema/85_workload.yaml` |
| 平台内核 | 加密密钥、重加密进度、扩展/函数/角色声明 | `contracts/schema/00_platform.yaml`、`contracts/schema/00_infra.yaml` |

## 6. 目标态数据模型声明

以下是目标态表族的设计意图与承接需求，**不书写具体表名与表列**。表名、列与域归属由声明式 Schema 的文件分组建模（`contracts/schema/*.yaml`，ADR-022）；域 → Schema 文件映射见 §5「域汇总」。

| 设计意图 | 承接需求 |
|---------|---------|
| ReBAC 对象级细粒度授权元组 | REBAC-001~008、REBAC-010 |
| 计量事件与账期聚合 | METER-001~005 |
| 审计哈希链锚定快照 | RISK-013/014 |
| IGA 访问 Campaign | IGA-001/007 |
| 职责分离互斥校验 | IGA-005 |
| Workload 身份主体 | WLI-001/003 |
| OIDC 联邦信任源 | WLI-002 |
| 联邦身份池 | WLI-004 |
| Identity Actions 沙箱 | EXT-001~004 |
| 风险分级决策记录 | RISK-001/005 |
| CAEP/SSF 事件接收方 | RISK-010/012 |
| 多区域部署拓扑 | DR-001、ORG-016 |

> 目标态表族的结构由声明式 Schema 落地，经 `check-db-conventions.sh` 与 `check-schema-parity.py` 门禁；逐表索引/分区/事务一致性设计以 `contracts/schema/*.yaml` 的声明与 ADR-022 为准。

## 7. 核心实体关系图（ER Diagram）

本节不维护手写 ER 图。结构事实（表、列、主键、外键、约束、索引与 RLS 声明）的唯一事实源是声明式 [`contracts/schema/*.yaml`](../../contracts/schema/)（[ADR-022](../adr/ADR-022-declarative-schema-versioned-migration.md)）；DDL、版本化迁移与回滚脚本均由生成器产出，手绘图必然与事实源漂移。

- 枚举表与文件分组（域边界）：`ls contracts/schema/*.yaml`；列、约束、索引从对应 YAML 的 `columns` / `constraints` / `indexes` / `fk_constraints` 段读取。
- 生成物与声明零差异：`python3 scripts/gen_schema.py --check`；声明与产物 DDL 结构等价：`python3 scripts/check-schema-parity.py`。
- 租户隔离与 RLS 语义见 §3.1、§3.3；策略内容门禁 `python3 scripts/check-rls-policy-content.py`。
- 逐类事实源与复现命令的完整口径见本文「数据层设计约束与校验 › 结构事实的查询口径」。

## 数据层设计约束与校验

> 表、列、约束、索引的唯一事实源是声明式 `contracts/schema/*.yaml`（ADR-022）。本文件**不复制表清单、列清单或列计数**；结构事实一律以事实源与生成物为准。

### 结构事实的查询口径

| 口径 | 事实源 / 复现命令 |
| --- | --- |
| 表与域分布 | `ls contracts/schema/*.yaml`（文件名分组即域） |
| 列、约束、索引清单 | `contracts/schema/*.yaml` 的 `columns` / `constraints` / `indexes` 段 |
| 声明式 Schema 与 DDL 一致性 | `python3 scripts/check-schema-parity.py` |
| 方言构造分布 | `bash scripts/check-db-conventions.sh`（PostgreSQL 专有构造在 PG 仓内集中使用是允许的） |
| 回滚脚本独立性 | `bash scripts/dev/check-rollback-independence.sh` |
| 全量重放验证 | `bash scripts/dev/validate_schema_replay.sh` |
| 列注释覆盖率 | `grep -c 'comment:' contracts/schema/*.yaml`（或 `check-schema-defaults.py` 报告） |

### 结构强制约束（提交前逐项自检）

| # | 约束 | 判据 |
| --- | --- | --- |
| D1 | 多租户隔离 | 租户域表声明 `organization_id` 且带租户谓词，PostgreSQL 侧 FORCE RLS 生效 | `bash scripts/check-db-conventions.sh` |
| D2 | 主键 | 逻辑类型统一 `UUID`，UUIDv7 由应用层生成，禁止 `SERIAL` 族与数据库函数默认值 | `check-db-conventions.sh` |
| D3 | 金额与币种 | 金额 `DECIMAL(19,4)`、币种 `VARCHAR(3)`，禁止浮点参与金额运算 | `check-db-conventions.sh` |
| D4 | 时间类型 | 统一 `TIMESTAMPTZ`（PG 基线）对应应用层 RFC3339 契约 | `check-schema-defaults.py` |
| D5 | 命名与软删除 | 约束前缀 `uq_` / `idx_` / `fk_` / `ck_{table}_{column}`；软删除表用部分唯一索引表达「活跃唯一」 | `check-db-conventions.sh` |
| D6 | 外键完整性 | 声明 FK 必须带明确 `ON DELETE` 策略；父表引用不得存在前向引用或未声明的悬挂引用 | `check-schema-parity.py` + 重放验证 |
| D7 | UPSERT 仲裁 | 所有 `ON CONFLICT` 目标必须可解析到唯一约束或部分唯一索引 | `validate_schema_replay.sh` |
| D8 | 跨表引用一致性 | 子表的租户列必须与父表一致；关联绑定表与 JSONB 集合字段不得双写同一事实 | 设计评审 + 重放验证 |
| D9 | 索引经济性 | 唯一约束已隐式建立索引，禁止再建同列显式索引；前缀重叠的冗余索引必须合并或删除 | `check-schema-parity.py` 报告 |
| D10 | 列注释 | 面向业务的高频表列注释覆盖率须达 95% 以上（口径见上表）；新表建表即带注释 | `check-schema-defaults.py` |
| D11 | 生命周期 | 高写入量表按 [registries/lifecycle.yaml](../../contracts/registries/lifecycle.yaml) 声明的保留/归档/清理策略执行（ADR-011） | `contracts/registries/lifecycle.yaml` |
| D12 | PostgreSQL 构造集中 | PostgreSQL 专有构造（`RETURNING`/`SKIP LOCKED`/`DISTINCT ON`/`ILIKE`/`::`）只允许出现在 `internal/infra/postgres/**`；外部域不得出现 | `check-db-conventions.sh` |

### 高频查询与索引要求

本表是**查询场景的设计要求**，不是索引清单：逐表索引/分区是否声明、以何种形态声明，唯一事实源是 `contracts/schema/*.yaml` 的 `indexes` / `constraints` 段（复现：`grep -n 'indexes:' -A 20 contracts/schema/<域>.yaml`）；设计要求与声明不一致时按 ADR-022 Schema 流程补声明或修订本表，不得在本文手写索引定义。

| 查询场景 | 设计要求 |
| --- | --- |
| 用户登录 | 租户内用户名的「活跃唯一」由部分唯一索引强制 |
| 应用列表 | 租户维度列表查询须有 `organization_id` 前缀索引支撑 |
| 审计追溯 | 租户 + 时间倒序追溯须有索引支撑；时间分区按 ADR-011 证据驱动条件启用，不作无条件要求 |
| 角色权限 | 用户→角色绑定查询须有 `user_id` 维度索引支撑 |
| SSO 域名发现 | verified 域名查找须有状态过滤的部分索引支撑 |
| Outbox 轮询 | pending 领取查询须有 `status` + `available_at` 复合索引支撑 |
| Webhook 重试 | 到期重试扫描须有 `status` + `next_retry_at` 复合索引支撑 |

### 读写与一致性要求

- **单写入口**：同一业务事实只允许一个写路径；JSONB 集合字段与关联绑定表并存时必须声明唯一权威方，另一方为投影。
- **软删除一致性**：软删除标记与派生列必须同源（触发器或应用层单点派生），不得双写。
- **冗余快照字段**：为排障保留的反规范化字段（如 `username` / `organization_slug` 快照）必须在 Schema 注释中标明「有意反规范化」，并声明刷新时机。
- **审计列**：写入型表携带 `updated_at` + `revision`（乐观锁）；审计类表本身不参与乐观锁。
