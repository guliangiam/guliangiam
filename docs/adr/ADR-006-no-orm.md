# ADR-006: pgx/raw SQL 而非 ORM

- 状态：[Current]
- 日期：2026-09-14

## 背景

GuliangIAM 的持久层重度依赖 PostgreSQL 专有语义：部分唯一索引（软删除活跃唯一性）、`ON CONFLICT` UPSERT、`RETURNING` 原子回填、`FOR UPDATE SKIP LOCKED` 并发取件、JSONB、数组/INET 类型，以及目标态的 RLS（ADR-016）。ORM 要么把这些构造遮挡在不透明的生成 SQL 之后，要么无法表达；而 ADR-023 的多方言路线恰恰要求把方言专有构造显式化、收敛进 Dialect 边界——这要求业务仓储里写的 SQL 是可读、可 grep 的文本。

## 决策

使用 pgx/v5 与参数化 SQL（`$1, $2, ...` 占位符）获取 PostgreSQL 类型系统、事务与查询计划的直接控制；禁止字符串拼接动态 SQL，动态片段必须经白名单构造；Repository 显式编写行扫描与 mapper；复杂查询必须保留 EXPLAIN 证据。

## 理由与后果

- 正面：SQL 与执行计划完全可见，慢查询可直接 `EXPLAIN (ANALYZE, BUFFERS)` 留证；方言专有构造可用静态脚本枚举规模（ADR-023 的 `check-sql-portability.sh` 正是建立在“SQL 是文本”这一前提上）；参数化绑定消除 SQL 注入面；pgx 批量与自定义类型编解码能力可直接使用。
- 负面：每个仓储需要手写 mapper 与查询测试，样板多于 ORM；复杂查询的演进必须伴随迁移与回归；SQL 质量依赖评审、gosec 与规范门禁持续约束开发者纪律。

## 替代方案

- **GORM / ent 等全功能 ORM**：放弃理由——隐式查询不可控、N+1 与黑盒迁移风险高，且无法表达部分唯一索引、RLS、`SKIP LOCKED` 等本项目正确性依赖的语义；与 ADR-023“方言差异收敛进 Dialect”的路线冲突（ORM 本身成为无法穿透的方言层）。
- **sqlx 轻量映射库**：放弃理由——只减少 scan 样板，却引入第二套结构体标签约定；pgx 原生 scan 加显式 mapper 已满足需求，且保留对批量协议与类型编解码的完全控制。
- **在 Domain/App 层直接写裸 SQL（不建 Repository 端口）**：放弃理由——会击穿 ADR-005 的端口边界；raw SQL 只允许出现在 Infra 仓储（以及 ADR-023 落地后的 Dialect 适配器）内。

## 验证证据（Current）

- `grep -E 'gorm|sqlx|entgo' go.mod` → **0 命中**；数据库相关依赖仅 `github.com/jackc/pgx/v5 v5.11.0`。
- 2026-09-14 抽查 `internal/infra/postgres/web3identity/sql.go`：常量 SQL + 编号占位符，原子消费语句为 `UPDATE ... SET consumed = true WHERE nonce = $1 AND consumed = false AND expires_at > NOW() RETURNING ...`；`internal/infra/postgres/authorizationcodes/sql.go:32` 的 INSERT 显式列出 `id` 等 16 列并绑定 `$1::uuid` 等参数，无字符串拼接。
- `go build ./...` → 退出 0；方言专有构造的现存规模已由 `scripts/check-sql-portability.sh` 以白名单方式量化（见 ADR-023 证据），结构规范由 `scripts/check-db-conventions.sh` 9/9 门禁（见 ADR-022 引用）。

## 复审条件

ADR-023 Dialect 层落地后，本 ADR 的适用范围应收窄为“raw SQL 只允许出现在方言适配器内”，届时更新边界表述；pgx 停止维护或出现控制力相当且社区明显更优的驱动时复审；若发现绕过参数化的拼接 SQL（gosec/门禁红灯），作为偏差立即整改而非放宽本决策。
