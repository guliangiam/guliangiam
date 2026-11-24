# ADR-022: 声明式 Schema 唯一源与版本化迁移

- 状态：[Current]/[Target] — 生成式 V1 基线 + 生成式增量链、活动快照与生成物零差异已落地（迁移链现状以 `internal/bootstrap/migrate.go` 的 `migrations` 声明与 `db/postgresql/` 目录为事实源，不在本文手写版本清单）；[Target] 待取证项见本 ADR 验证证据
- 日期：2026-09-17（目标态重构；本 ADR 取代原「单一幂等 DDL 重放（不设迁移账本）」决策）

## 背景

数据库结构管理必须回答四个问题：结构的唯一事实源是什么、变更如何演进、回滚如何执行、多方言如何产出。单一幂等 DDL 全量重放方案（原 ADR-022）在这四个维度均存在结构性缺陷：

1. **移除类变更无法表达**：`CREATE TABLE IF NOT EXISTS` 重放只能"跳过"既有对象，列删除、类型变更、重命名、约束收紧必须走独立人工流程，事实上形成了"新增走 DDL、移除走流程"的双轨。
2. **版本不可回答**：无迁移账本意味着无法回答"该库当前处于哪个结构版本"，灰度期多实例可能同时运行两个结构版本的代码。
3. **回滚缺失**：重放方案天然无回滚脚本，故障恢复只能依赖备份整库还原。
4. **方言绑定**：DDL 直接以 PostgreSQL 语法书写，多方言（ADR-023 第二基线 MySQL）生成无从谈起。
5. **生成链断裂**：结构事实无法驱动 Go 类型、前端类型、文档表格的生成，计数与字段清单靠人工同步，必然漂移。

## 决策

1. **声明式 Schema 是结构唯一事实源**：`contracts/schema/*.yaml` 按表族拆分（清单以 `ls contracts/schema/*.yaml` 为准）描述表、列、索引、约束、CHECK、RLS 策略与隔离分级；使用逻辑类型（`UUID`/`TEXT`/`DECIMAL(p,s)`/`TIMESTAMP_TZ`/`JSON`/`ARRAY<TEXT>`），不绑定方言。声明文件不得与生成产物混放。
2. **全部 DDL 是生成产物**：PostgreSQL（默认基线）与 MySQL/GaussDB/KingBaseES（ADR-023）的全量 V1 DDL、embed 副本、Go/TS 类型全部由生成器从声明式 Schema 产出，**禁止手改**；CI 逐字节校验零差异。V1 不是手写冻结文件。
3. **结构链是 squash 后的生成式 V1 基线 + 生成式增量**：
   - 迁移链由两层产物组成：**当前态全量渲染** `db/generated/<dialect>/current.up.sql`（= 当前声明的完整渲染，供 parity/列检查/文档消费）与**历史迁移链** `db/postgresql/V{N}__*.sql`（V1 = 冻结基线 `history/active/V1` 的渲染；V2+ = 生成式增量）。新安装按版本顺序重放 V1..Vn 得到当前结构终态。
   - 活动冻结快照位于 `contracts/schema/history/active/V{N}/`，最新快照必须与 `contracts/schema/*.yaml` 字节一致（`check-active-schema-history.py`）。
   - 结构变更流程：改声明 → `python3 scripts/gen_schema.py --dialect postgresql --version <N>`。生成器以 `history/active/V{N-1}` 为 diff 起点产出 `V{N}__*.sql` 与配对回滚，成功后冻结 `history/active/V{N}`。
   - **迁移生成必须 fail-closed**：删除表/列、无法表达的类型变更一律拒绝生成；缺少正向/回滚产物时 `--check` 必须失败。删除类变更须先给出经评审的数据保留、备份与回滚方案。
   - **列改名必须显式登记**：结构 diff 无法区分「改名」与「删旧列+加新列」；改名登记在 `contracts/schema/migrations/renames.yaml` 后，生成器把旧快照列名映射为新名再 diff，并产出 `ALTER TABLE ... RENAME COLUMN` 与反向回滚。未登记则按破坏性变更拒绝。
   - **纯组织重构不产生迁移**：声明层文件分组/拆分/改名是「域边界」的表层事实，不改变任何表/列/约束。此类重构用 `gen_schema.py --rebase-active-snapshot`：先证明表级 diff 为空，再把最新活动快照原子重冻结为当前文件布局（V1 等历史快照保持原布局），随后重算生成物；迁移链不新增版本。
   - 历史 `contracts/schema/history/V1..V7/` 是旧设计快照，不属于活动链，不得用于 diff 或升级。
   - 迁移执行器对账本中任何不在当前镜像迁移集内的版本 fail-closed；已应用版本的校验和必须匹配。
4. **CI 结构门禁十一项**（全部阻塞合并）：
   1. 空库全量建表成功；
   2. 生成式增量迁移可前滚（存在增量时按 V{N-1} → V{N} 演练）；
   3. 回滚产物与正向一一配对且可执行；
   4. 所有外键、索引、约束、CHECK 可解析（无悬空引用）；
   5. 所有 `ON CONFLICT` 仲裁目标存在唯一约束/索引；
   6. 所有引用列类型一致；
   7. 所有软删除唯一性由数据库强制（活跃唯一 = 部分唯一索引）；
   8. 所有租户表启用 `ENABLE` + `FORCE ROW LEVEL SECURITY`；
   9. **生成物零差异**：DDL、增量迁移、回滚、OpenAPI、文档表格与工作区文件逐字节比对；
   10. **方言能力探测**：第二基线对方言能力（RLS、部分唯一索引、`SKIP LOCKED`、JSON、数组）探测——缺失必须落地等价方案并登记 ADR，或拒绝启动，禁止静默降级安全能力；
   11. **迁移链完整性与类别纯净**（`scripts/check-migrations.sh`）：版本连续、命名合规、结构与种子脚本语句级别互不混放、种子语句自带 `ON CONFLICT`、种子目录无声明层文件且头部自述链与文件名、账本表仅由 `internal/platform/db` 写入。
5. **RLS 与软删除唯一性由数据库强制**：应用层过滤只是第一道防线；任何 RLS 缺失的租户表不得通过 CI（ADR-016）。
6. **种子数据独立且人工编写**：种子为独立文件 `db/seeds/V{N}__{主题}_{动作}.sql`，与结构声明分目录、分版本链、分开关（`AUTO_SEED`）；数据链不设声明层、不由生成器产出（理由见"两条链的形态不同"）。
7. **主键无库函数默认值**：DDL 中主键一律 `id UUID NOT NULL`，由应用层生成 UUIDv7 显式写入（ADR-007），不包含任何方言专有函数。
8. **版本化迁移链的方言范围**：版本化增量链只对 PostgreSQL 默认基线维护（`db/postgresql/V{N}__*.sql` 是唯一完整版本史）；非 PostgreSQL 方言只维护全量 V1——由同一份声明式 Schema 在对应方言重新生成，产物即当前结构终态，不派生增量 diff。方言产物正确性由"声明 → 方言产物"零差异门禁与 parity check 承担，不由版本号连续性承担。**当前不登记任何方言专有迁移例外**；将来确需引入，必须重新修订本 ADR 并登记例外对象、配对回滚与门禁扩展。

## 理由与后果

声明式 Schema + 生成器产出使"结构事实"获得唯一源：DDL、迁移、回滚、Go 领域类型、前端类型、ER 图、列清单全部由同一源生成，CI 以零差异门禁拦截漂移。squash 为单一生成式 V1 消除了历史手写增量与声明层之间的双源：V1 缺失的表/列、类型漂移、RLS 漂移不再可能通过"手写迁移补丁"长期存活。

代价：生成器基础设施（声明解析、方言后端、diff 引擎）是前置投入；开发者变更入口从改 DDL 变为改声明文件 + 运行生成器，评审多一道生成物 diff 工序；squash 后旧账本数据库的升级兼容不在当前基线承诺范围内（从零建设视角，见 ADR-022 决策 3）。

**两条链的形态不同：结构链声明化 + 生成，数据链人工编写（决策 6 的依据）。**

结构的不变量可由声明静态推导：外键目标表/列是否存在、索引列是否存在、RLS 表达式引用的列是否存在、CHECK 是否可解析、回滚 DROP 顺序的逆拓扑序、多方言产物等价性。声明层因此提供额外蕴含：一台未连库的 CI 能拒绝无法执行的 DDL。

数据链的不变量全部是执行期语义：语句幂等、`ON CONFLICT` 仲裁目标在真实 `pg_index` 中可解析、`(SELECT id FROM …)` 引用闭合、历史取值归一后与运行时注册键一致。这些只能由真实 PostgreSQL 重放证明（`scripts/dev/validate_schema_replay.sh` 连续重放两次 + 逐个解析全部仲裁目标）。再加一层声明只是引入第二份表示，已发生过声明与 SQL 漂移。

## 替代方案

- **单一幂等 DDL 重放（原决策）**：放弃理由——见背景五项结构性缺陷。
- **手写版本化迁移（golang-migrate/goose 模式）**：放弃理由——迁移脚本成为第二事实源，与"结构终态"人工保持一致的成本随表数线性增长，且无法跨方言生成。
- **保留历史手写增量链（V2..V10）并把声明层作为事后校验**：放弃理由——历史增量与声明层已经漂移（缺表/类型/RLS 差异数百项），继续保留等于永久维护两套事实；从零建设视角不存在需要兼容的历史库，squash 是唯一不引入双源的方案。
- **种子也走"声明式 + 生成器"**：放弃理由——种子不变量是执行期语义，声明层只能引入第二份表示（已出现过声明与 SQL 漂移）。
- **种子数据用 Go 代码播种**：放弃理由——数据被编译进二进制，运维无法在不发版的前提下修正引导数据，也无法用 `psql` 复现。
- **纯 ORM 自动迁移**：与 ADR-006（不使用 ORM）冲突。已否决。

## 验证证据

### [Current] 已取证项

1. ✅ **声明式 Schema 全量落地**：`ls contracts/schema/*.yaml`（表族文件清单以命令输出为准）覆盖全部业务表；`gen_schema.py` 输出表/列/语句统计以命令输出为准，不在文档中手写计数。
2. ✅ **当前全量生成器零差异**：`for d in postgresql mysql gaussdb kingbasees; do python3 scripts/gen_schema.py --dialect "$d" --check; done` → 全部 PASS；`scripts/check-schema-parity.py` → 声明式 Schema 与四方言产物结构等价（0 差异）。
3. ✅ **迁移链两层产物分离**：`db/generated/<dialect>/current.up.sql` = 当前声明全量渲染；`db/postgresql/V1__*.sql` = 冻结基线渲染且与 go:embed 副本逐字节一致；历史手写增量已折叠，历史记录见 `CHANGELOG.md` 对应版本。
4. ✅ **活动快照与迁移链一致**：最新活动快照（`contracts/schema/history/active/` 下最大版本目录）与 `contracts/schema/*.yaml` 字节一致；`python3 scripts/check-active-schema-history.py` 与 `bash scripts/check-migrations.sh` 均 PASS。
5. ✅ **增量迁移机制可复现**：V2+ 由 `gen_schema.py --version N` 生成（以 `history/active/V{N-1}` 为 diff 起点）并产出配对回滚；当前结构链与数据链的版本集合、顺序以 `internal/bootstrap/migrate.go` 的 `migrations` 声明与 `db/postgresql/`、`db/seeds/` 目录为事实源（复核：`ls db/postgresql/ db/rollback/`），迁移链折叠历史见 `CHANGELOG.md`。
6. ✅ **CI 门禁阻塞合并**：`verify-docs.sh`（含 `check-migrations` / `check-schema-parity` / `check-schema-defaults` / `check-db-conventions` / `check-sql-columns` / 四方言 `gen_schema --check`）全部真实通过（历史 known-red 基线见 `contracts/registries/known-red.yaml` 的关闭记录）。
7. ✅ **下游生成物**：Go `internal/domain/types/generated/schema_types.go` 与 TS `web/src/shared/types/generated/schema-types.ts` 由生成器产出并纳入零差异门禁。
8. ✅ **非 PostgreSQL 方言产物**：`db/{mysql,gaussdb,kingbasees}/V1__*.sql` 由声明式 Schema 生成，`check-schema-parity.py` 验证结构等价（方言能力差异由 ADR-023 显式豁免）。

### [Target] 待取证项

9. ⏳ **迁移账本行为取证**（需 PostgreSQL 真库验证）：首次启动按 `migrations` 声明逐版本写 `schema_migrations`（`(kind, version)` 落账，版本集合以账本与声明为事实源）；二次启动不重复执行；手动改写已记账版本 → checksum mismatch 拒绝启动；账本含镜像外版本 → 拒绝启动（附录 A.2.2）。已在隔离库确认结构链与数据链落账，checksum 漂移与镜像外版本拒绝仍由 `scripts/dev/validate_schema_replay.sh` 覆盖，待随发布流水线复跑。

## 附录：目录结构与命名规范

### A.1 目录结构（生成器落地的形态）

```text
contracts/schema/                      # 声明层：唯一结构事实源（按表族拆分）
├── 00_infra.yaml                      # 扩展、函数、触发器、RLS 辅助
├── 00_platform.yaml … 90_metering.yaml
└── history/active/V{N}/               # 活动冻结快照（最新 = 当前声明）

contracts/schema/migrations/renames.yaml  # 列改名登记（生成 RENAME COLUMN 的显式输入）

db/
├── seeds/V1__guliangiam_seed_init.sql # 数据链（人工编写，无声明层、无生成器）
├── generated/<dialect>/current.up.sql  # 生成物：当前声明全量渲染（parity/列检查输入）
├── postgresql/V1__guliangiam_schema_init.sql   # 生成物：冻结基线（活动 V1 快照渲染）
├── postgresql/V{N}__*.sql             # 生成物：V2+ 增量迁移
├── <dialect>/V1__guliangiam_schema_init.sql    # 生成物：MySQL/GaussDB/KingBaseES 当前态基线
└── rollback/V{N}__*.down.sql          # 生成物：V2+ 回滚脚本（与正向一一配对）

internal/bootstrap/
├── schema_sql/V1__guliangiam_schema_init.sql  # 生成物：go:embed 副本（与 db/postgresql 逐字节一致）
├── seed_sql/V1__guliangiam_seed_init.sql      # 种子嵌入副本（与 db/seeds 逐字节一致）
└── migrate.go                                 # 嵌入声明与版本顺序（唯一入口）
```

与原始草案的差异及理由：数据链没有声明层（决策 6），`db/seeds/` 只保留人工编写的版本化 `.sql`；结构迁移与种子脚本的嵌入副本分离到 `schema_sql/` 与 `seed_sql/`，使"哪份字节对应哪个声明"不依赖文件名猜测。结构链目录与命名的唯一事实源是 `scripts/gen_schema.py` 的路径常量与 `internal/bootstrap/migrate.go` 的 `migrations` 声明；数据链的唯一事实源是 `db/seeds/V{N}__*.sql` 自身。

### A.2 迁移文件命名规范

```text
# 结构链
db/postgresql/V{N}__{表族或主题}_{动作}.sql          # 正向（生成物）
db/rollback/V{N}__{表族或主题}_{动作}.down.sql       # 回滚（与正向同名 + .down，生成物；V1 由部署流水线生成）
# 数据链（种子）
db/seeds/V{N}__{表族或主题}_{动作}.sql               # 正向，无回滚；人工编写，无生成物
```

规则：① 版本号在各自链内从 1 起单调递增且连续；② 主题段为小写 snake_case（`^V[0-9]+__[a-z0-9_]+(_[a-z0-9_]+)*\.sql$`）；③ 结构链 V2+ 每个版本必须有正向与配对回滚（forward-only 框架下 V1 基线回滚由部署流水线生成，不作为运行时需求）；④ 回滚是正向的精确逆运算（生成器 diff 产出，不手写）；⑤ 数据链每个版本必须在 `migrate.go` 的 `migrations` 声明中存在对应项；⑥ 结构链与数据链版本号互不约束（账本以 `(kind, version)` 为主键区分）；⑦ 数据链目录内只允许版本化 `.sql`，且每个种子脚本头部逐字写出契约行 `-- @seed-chain: authored file={本文件名}`，与实际文件名不一致即失败。

### A.2.1 结构链与数据链的分类约束

结构迁移只允许 DDL（`CREATE`/`ALTER`/`DROP`/`COMMENT`/`GRANT`/`TRUNCATE`），数据迁移只允许 DML（`INSERT`/`UPDATE`/`DELETE`）且每条 `INSERT` 必须自带 `ON CONFLICT` 仲裁（幂等重放）。两条链运行时独立开启：结构链由 `AUTO_MIGRATE` 控制，数据链由 `AUTO_SEED` 控制。`scripts/check-migrations.sh` 按语句级别断言这两条约束（生成物会把一条语句压到极长行上，行级 grep 必然误判）。

### A.2.2 迁移账本

结构链与数据链共用账本 `guliangiam.schema_migrations (kind, version, name, checksum, applied_at, execution_ms)`，主键 `(kind, version)`。语义：① 每个版本在同一事务内完成"执行迁移 + 写账本行"，不存在已执行未记账或已记账未执行的中间态；② 已记账版本的 `checksum` 必须与镜像内该版本 SHA-256 一致，不一致即拒绝启动；③ 账本存在镜像外版本（回滚到旧镜像）时拒绝启动；④ 迁移前向单向，回滚由发布流水线显式执行 `db/rollback/` 产物，应用自身不回滚；⑤ `SCHEMA_CONTENT_HASH` 只对结构链求哈希，编辑种子不触发结构指纹变更。

### A.3 表逻辑与引用逻辑生成规则

表逻辑（所有业务表统一模板）：`id UUID` 主键且无默认值；租户表 `organization_id UUID NOT NULL REFERENCES organizations(id)` 且显式 `ON DELETE`；`status` 全小写 snake_case + CHECK（值集以状态机注册表为准）；`revision BIGINT DEFAULT 1`；`created_at/updated_at TIMESTAMPTZ`；软删除表 `deleted_at TIMESTAMPTZ NULL` + 活跃唯一部分索引。`soft_delete`/`revision` 标志由生成器自动补齐 `deleted_at`/`revision`/`updated_at` 隐式列，所有表自动隐含 `created_at`——声明层只写标志，避免 30+ 张表各写一遍而漏列。

引用逻辑：① 生成器按外键拓扑排序建表，消除前向引用；不可避免时在文件尾 `ALTER TABLE ... ADD CONSTRAINT` 补齐，CI 校验无前向引用；② 每个 `ON CONFLICT` 目标必须有匹配唯一约束/索引，部分索引谓词必须原样重复；③ 外键列与被引用列类型一致；④ 租户表由生成器产出 `ENABLE` + `FORCE ROW LEVEL SECURITY` + 租户策略；⑤ 引用完整性分类（物理 FK / 逻辑引用 / 待补）以 `contracts/registries/referential-integrity.yaml` 为唯一裁决，`check-fk-coverage.py` 阻塞。

## 复审条件

生成器 diff 无法表达的变更类型（跨表数据迁移、分区演进）出现时复审迁移模型；多区域部署（ADR-031）引入区域级结构差异时复审声明式 Schema 的作用域模型；第二基线方言能力探测（ADR-023）暴露声明式构造映射缺口时复审逻辑类型集。
