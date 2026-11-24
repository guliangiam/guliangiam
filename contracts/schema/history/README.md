# Schema 历史快照

本目录存放声明式 Schema 的版本快照，治理规则由 ADR-022 声明。

## 当前结构

| 路径 | 状态 | 说明 |
|------|------|------|
| `active/V1` | 冻结全量基线 | 迁移折叠后的统一 V1 快照（145 表），禁止随当前声明直接改写 |

2026-11-04 开发期迁移折叠：此前 `active/V2`–`V14` 与 `db/postgresql/V2`–`V14`
全部并入单一 V1 基线（145 表）。折叠前的增量均为纯 DDL、无数据迁移语义，
因此不保留 V2；历史增量只能通过 git 历史追溯，不得在文档或脚本中引用。

## 消费方与事实源

- `contracts/schema/*.yaml` 是当前声明的唯一事实源；本目录不是当前声明的叠加输入。
- `scripts/gen_schema.py --version N` 使用 `active/V{N-1}` 作为 diff 起点，生成 V{N} 结构迁移、回滚及新的活动快照。
- `scripts/check-active-schema-history.py`、`scripts/check-migrations.sh` 和 `scripts/check-generated-zero-diff.sh` 校验快照、迁移及四方言产物的一致性。
- `internal/platform/db` 通过 `internal/bootstrap/migrate.go` 中的显式迁移表执行版本链；项目不使用 Flyway。

## 变更纪律

- 不得手工修改已冻结快照来替代新结构迁移。
- 新增结构变更必须先修改当前声明，再通过生成器产生下一版本及快照；禁止手写编号、跳过版本或修改已应用迁移。
- 删除表/列及列改名必须遵守 ADR-022 的数据保留与 renames 登记流程。
