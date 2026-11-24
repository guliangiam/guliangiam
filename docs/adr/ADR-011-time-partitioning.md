# ADR-011: 高增长表时间分区

- 状态：[Current]/[Target]（PartitioningStrategy port + Stub adapter + 真实容量探测 Advisor 已落地——6 张高增长表分区策略模板（records/entries/webhook_events/llm_usage_logs/metering_events/llm_metering_events）+ 容量阈值声明 + pg_total_relation_size/pg_indexes_size 只读探测；分区启用、EXPLAIN 裁剪证据、迁移锁评估、归档演练仍为 [Target]）
- 日期：2026-09-14

## 背景

`records`、`entries`、`webhook_events`、`llm_usage_logs`、`metering_events`、`llm_metering_events` 等高增长表会随运营时间无限膨胀，单表与单索引过大后维护窗口（VACUUM、重建索引、归档）将不可控。但在全新库、尚无真实查询计划与容量曲线时预先分区，会反向约束主键/唯一键设计（分区表的唯一键必须包含分区键），并引入分区裁剪失效与迁移锁风险。因此本决策是一条“顺序决策”：先度量，后分区。

## 决策

高增长表先建立 TTL 策略、归档链路与容量指标；仅当真实数据量与 `EXPLAIN (ANALYZE, BUFFERS)` 证据显示收益时，才启用按时间分区（PostgreSQL 声明式分区）；分区方案必须验证查询裁剪、归档可整块摘除、迁移锁影响与恢复路径。

## 理由与后果

- 正面：避免无证据的预设计复杂性；分区后历史数据可按子表直接 `DETACH/DROP` 归档，查询经分区裁剪只扫近期分区；索引维护粒度变小。
- 负面：分区键一旦选定，所有唯一约束都必须包含它，可能影响外键与 UPSERT 语义；计划器在查询不带时间条件时可能扫全部子表而退化；在线父表改分区属重型变更，迁移锁风险须提前评估；与 ADR-023 多方言存在张力（声明式分区是方言专有构造）。

## 替代方案

- **现在就对所有高增长表按月声明式分区**：放弃理由——决策时为全新库（无生产数据曲线；表数以 `contracts/schema/` 声明与生成 DDL 为准），没有查询计划证据，且分区会立即限制主键/唯一键设计，属于过早优化；《开发规范手册》同样要求”未取得证据前不分区”。
- **只做冷数据归档、在线表永不分区**：放弃理由作为完整方案——归档只控制总量，在线热点与索引仍持续变大，归档窗口本身会越来越长；归档被保留为第 0 阶段措施，但不替代未来的分区决策。
- **按租户哈希分区**：放弃理由——哈希分区不解决时间归档诉求，还会使跨租户报表复杂化；租户隔离的纵深交给 ADR-016，容量问题按时间维度处理。

## 验证证据

**现状事实（[Current] 子事实）**：

- `grep -c 'PARTITION BY' db/postgresql/V1__guliangiam_schema_init.sql` → **0**：DDL 中无任何声明式分区。
- `grep -in 'partition\|TTL' db/postgresql/V1__guliangiam_schema_init.sql` → **0**：无 TTL/归档调度的库内痕迹；高增长表（`records`、`entries`、`webhook_events`、`llm_usage_logs`、`metering_events`、令牌相关表）均已存在但为普通堆表。

**出口条件（[Target] 待取证项）**：

对每张待分区表（`records`、`entries`、`webhook_events`、`llm_usage_logs`、`metering_events`、`llm_metering_events`，时间列分别为 `created_at` / `recorded_at`）逐个满足以下全部条件后转 `[Current]`（本 ADR 按表分批达成）；令牌类表（access_tokens/sessions）经裁决**暂不入列**（低行数、高更新，容量曲线与收益均未证明），若后续指标证明需要再由新 ADR 复审；现行容量探测与裁决实现见 `internal/platform/partitioning/advisor.go` 与 `internal/infra/postgres/partitioning/prober.go`（OPEN-API-40）。

1. **容量阈值触发**（满足其一）：`pg_total_relation_size('{table}') > 50 GB`，或 `pg_indexes_size('{table}') > 20 GB`（`SELECT tablename, pg_size_pretty(pg_total_relation_size(tablename)) FROM pg_tables WHERE schemaname = 'guliangiam' AND tablename = '{table}'` 取证）。
2. **分区裁剪净收益**：对该表的 Top 3 最常执行查询（`SELECT queryid, count(*) FROM pg_stat_statements GROUP BY queryid ORDER BY count(*) DESC LIMIT 3`）做 `EXPLAIN (ANALYZE, BUFFERS)` 对比分区前后：
   - 分区后实际扫描的行数 ≤ 分区前的 30%（分区裁剪生效）；
   - 分区后 BUFFERS read + shared hit 总和 ≤ 分区前的 50%。
3. **迁移锁风险评估完成**：`pg_repack` 或声明式分区 `ALTER TABLE ... PARTITION BY` 操作的锁等待时间 P95 < 30s（在同等规模 staging 表上实测）；存在业务低峰窗口 ≥ 2h 可执行迁移。
4. **归档校验与恢复演练**：`child_table_detach` 操作在 30s 内完成并返回，脱离的子表可独立 `DROP` 或归档到冷存储，`SELECT` 历史归档数据通过 `pg_catalog` 或外部工具可恢复；演练记录归档至《部署与运维指南》。
5. **约束与唯一键调整方案确认**：分区键选定后，该表所有唯一约束（包括 `ON CONFLICT` 仲裁目标）已包含分区键；`git diff` 在 schema 声明文件中可见对应变更。

**待取证项**：Top 3 查询的 `queryid` 与分区键初步方向（通常是 `created_at` 或令牌/审计表的时间列）；分区周期（按月 vs 按周）取决于写入速率与归档频率权衡。

## 复审条件

真实容量曲线触及阈值时按表启动；PostgreSQL 大版本提供更平滑的在线分区演进能力时重评迁移方式；ADR-023 方言目标库（MySQL/国产库）的分区等价构造经能力探测验证时同步更新方案；引入冷热分层存储或独立审计库时复审归档边界。
