# ADR-007: UUIDv7 主键策略（应用层生成，无数据库默认值）

- 状态：[Current]/[Target] — 应用层 UUIDv7 生成器已落地，DDL 库函数默认值已归零（PostgreSQL 与 MySQL 生成基线均无 `uuidv7()` 默认值依赖）；MySQL 第二基线建表记录与 INSERT 显式赋 id 门禁取证为 [Target]
- 日期：2026-09-17（目标态重构；原决策 2026-09-13）

## 背景

GuliangIAM 从第一张表起就要确定主键形态，候选各有硬伤：自增序列（`BIGSERIAL`）暴露业务规模、跨方言不可移植（序列主键曾在 `casbin_rules.id` 踩坑）；UUIDv4 全局随机导致 B-tree 索引随机写与页分裂，且多实例并发生成无序；库内时间有序方案（如专用扩展）绑定数据库大版本与方言。项目同时已确定多方言目标（ADR-023），主键必须在 MySQL/国产库上同构可得，因此候选收敛到“应用层生成的时间有序 UUID”。

## 决策

全部表主键使用 RFC 9562 UUIDv7（时间有序 UUID），由应用层 `internal/platform/uuid`（`uuid.Generator` 端口 + `V7Generator`，基于 `github.com/google/uuid` 的 `NewV7()`）生成并在 INSERT 时显式写入；具体生成函数、方言兼容性与迁移路径必须由 SQL/CI 验证，不以未验证的数据库大版本或扩展为前提。

## 理由与后果

- 正面：时间有序显著降低主键索引随机写；无中心发号器、多实例并发生成不冲突；逻辑类型 `UUID` 在各方言可映射（ADR-023 3.3.2）；16 字节定长，分布式系统中碰撞概率可忽略。
- 负面：比 bigint 多占存储（16 字节）且肉眼不可读；要求全部写路径统一经过生成器端口，绕过端口直接依赖库默认值会形成双轨（见下方目标态小节）；批量导入也须显式赋 id。

## 从零目标态：DDL 无库函数默认值（归零已达成；门禁取证为 [Target]）

本决策的完整形态是「应用层生成 + DDL 无任何主键默认值」双条件；应用层路径已建成，DDL 侧的库函数退场随 [ADR-022](ADR-022-declarative-schema-versioned-migration.md) 声明式迁移一并完成：

1. **应用层路径已建成并被关键写路径使用**：`internal/platform/uuid/generator.go` 实现 V7 生成；组合根 `internal/bootstrap/runtime.go` 注入 `platformuuid.V7Generator{}`；授权码等关键表 INSERT 显式携带 `id` 列。
2. **DDL 侧目标态**：声明式 Schema 生成的 DDL 中主键一律 `id UUID NOT NULL`（无默认值），不包含 `guliangiam.uuidv7()` 等方言专有函数（ADR-022 决策 7）；生成器校验全部 INSERT 语句显式赋 `id`。
3. **现状收敛**：结构链已 squash 为生成式 V1 基线，`grep -c 'DEFAULT guliangiam.uuidv7()' db/postgresql/V1__guliangiam_schema_init.sql` **实测为 0**——方言函数默认值已随声明式 Schema 落地删除，不以"留着备用"为由保留。
4. **出口判据**：DDL 中 `guliangiam.uuidv7()` 零命中、MySQL 第二基线建表无 schema 函数依赖、INSERT 显式赋 id 门禁通过。

## 替代方案

- **以数据库 `uuidv7()` 函数/扩展作为唯一生成源**：放弃理由——正是当前张力的来源：函数依赖 plpgsql/pgcrypto 与具体方言，MySQL/达梦等无等价物（ADR-023）；库函数只适合作为迁移期兜底，不作为目标架构。
- **UUIDv4**：放弃理由——完全随机造成主键索引随机写与页分裂，且无时间序，失去按 id 粗排序与同代记录聚集的能力。
- **BIGSERIAL / 雪花号段**：放弃理由——序列不可移植并暴露业务规模（`casbin_rules.id` 已有前车之鉴）；雪花方案需要中心节点或号段治理与时钟回拨保护，重造发号器基础设施。

## 验证证据

- 应用层生成器：`internal/platform/uuid/generator.go` 核对为 `google/uuid` 的 `NewV7()`；`go test ./internal/platform/uuid/ -count=1` → `ok`；组合根注入点核实；`go build ./...` 退出 0。
- DDL 残留：`grep -c 'DEFAULT guliangiam.uuidv7()' db/postgresql/V1__guliangiam_schema_init.sql` 实测 **0**；`db/mysql/V1__guliangiam_schema_init.sql` 中 UUIDv7 仅出现在列注释（应用层生成说明），无 schema 函数依赖（核验 2026-11-21）。出口判据中的「归零」已满足。
- 待取证项（[Target]）：MySQL 第二基线建表记录、INSERT 显式赋 id 门禁输出（随 ADR-022 声明式迁移取证批次关闭）。

## 复审条件

ADR-022 声明式迁移落地后更新本 ADR 证据并移除"过渡期现状"表述；`github.com/google/uuid` 的 v7 API 或 RFC 9562 发生不兼容变更；ADR-023 第二基线（MySQL 8.4 LTS+）建表验证暴露新的主键兼容性问题时复审。
