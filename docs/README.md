# GuliangIAM 文档体系

- **文档定位**：目标态设计规范门户——本目录全部正式文档描述「如果从零开始，这个系统应该怎么设计」的意图、边界、决策与验收
- **基线日期**：2026-10-11
- **维护责任**：架构负责人（架构与设计）、产品负责人（需求）、研发负责人（规范）、运维负责人（部署）
- **下次复审**：2027-01-30

## 1. 文档原则

本目录只保存能够指导设计、评审、测试和运行的正式文档。文档描述稳定约束、设计决策和操作流程，**不复制任何可生成的事实**；实现与文档冲突时，以唯一事实源与生成结果为准，并在同一变更中修正文档。

写作红线：

1. **禁止手写可生成事实**。表数、索引数、路由数、operation 数、环境变量数、错误码数、字段清单、枚举清单一律不得出现在文档正文中；文档引用事实时只写「事实源路径 + 复现命令」。
2. **禁止变更记录入正文**。版本表、修订记录、批次叙事（「第 N 批」）、批次号（`Txx`）、`本轮`、`本批`、收敛叙事、勘误史一律不得出现在本目录；一切历史与版本沿革唯一写入仓库根目录 `CHANGELOG.md`。`scripts/check-doc-facts.sh` 自动拦截（`docs/generated/` 渲染层例外）。
3. **禁止现状偏差登记**。本目录文档为目标态规范，不维护 `[Current]/[Target]/[Open]` 现状三态登记与偏差编号册（`docs/adr/` 按 ADR 自身治理规范保留状态词为例外）。正文不得出现「当前实现 / 现状 / 已实现 / 已落地 / 待接入 / 待落地 / 规划中 / 未实现 / 未落地」等实施进度词；待建设能力一律登记 `contracts/registries/open-items.yaml`。`scripts/check-doc-facts.sh` 自动拦截（`docs/adr/`、`docs/generated/` 例外）。
4. **禁止不可解析的事实源引用**。文档引用「唯一事实源」时必须给出仓库内可解析的路径，并同时给出复现/校验命令；引用生成物时必须写明生成命令。同一文档内章节编号必须唯一，`scripts/check-doc-anchors.py` 会拒绝重复章节号；SLO、留存、RTO/RPO 等注册表数值只允许引用，不得复制到正文。**例外**：显式声明为注册表投影的表格（必须写明事实源路径、冲突以注册表为准、以及双向同步纪律）可以承载投影数值，其行集合与数值必须与注册表一致；未附投影声明的复制一律违规。

数据库结构管理遵循 [ADR-022](adr/ADR-022-declarative-schema-versioned-migration.md)：声明式 Schema 是唯一事实源，DDL、版本化迁移与回滚脚本为生成产物。

## 2. 正式文档地图

`docs/` 根目录只保留四个 Markdown 入口文件：README.md、GuliangIAM-开发规范.md、acceptance-criteria.md、traceability-matrix.md；同级仅保留 `adr/`、`architecture/`、`design/`、`generated/`、`operations/`、`registries/`、`requirements/` 七个目录（文件地图见 §2.1）。

过程报告、审计快照与一次性诊断不得写入本目录根；过程证据写入 `.trae/documents/`（不参与追溯矩阵扫描，门禁 `scripts/check-docs-root.py` 拒绝根目录非入口文件及非 Markdown 垃圾文件）。

### 2.1 四层规范子目录

| 子目录                                     | 核心职责                                        | 读者             | 文件清单                                                                                                   |
| --------------------------------------- | ------------------------------------------- | -------------- | ------------------------------------------------------------------------------------------------------ |
| [architecture/](architecture/README.md) | 目标态蓝图：分层架构、依赖规则、API 三面、数据模型总纲、安全模型、可观测性、多区域、技术选型 | 架构师、技术负责人      | README, system-overview, layers, api-surfaces, data-model, security-model, observability, multi-region, technology-selection |
| [design/](design/README.md)             | 按域拆分的实施细则：认证 / IAM / 协议 / 商业 / AI 网关 / 基础设施 + 前端专项          | 后端/前端开发、架构师    | 各子域文件清单一律以 [design/README](design/README.md) 的文件地图为准（不在此处重复清点）                       |
| [requirements/](requirements/README.md) | 目标态功能/非功能/数据需求（SRS）                         | 产品、QA、架构师      | README, functional, non-functional, data                                                               |
| [operations/](operations/README.md)     | 部署拓扑、运维手册、备份灾备、安全运维                         | DevOps/SRE、架构师 | README, deployment, runbook, backup-dr, security-ops                                                   |

### 2.2 根目录保留文件

| 文件                                               | 职责                                                                                 |
| ------------------------------------------------ | ---------------------------------------------------------------------------------- |
| [GuliangIAM-开发规范.md](GuliangIAM-开发规范.md)         | 工程规范与门禁口径（命名、API、数据库、安全、测试）                                                        |
| [acceptance-criteria.md](acceptance-criteria.md) | Given-When-Then 验收条件（AC 编号唯一事实源）                                                   |
| [traceability-matrix.md](traceability-matrix.md) | 需求 ↔ 设计 ↔ AC ↔ ADR ↔ 开放项双向追溯（生成物，由 gen\_traceability.py 产出）                        |
| [registries/](registries/README.md)              | 注册表治理说明（YAML 唯一事实源位于 `contracts/registries/`，本目录仅保留治理导航）                            |
| [adr/](adr/README.md)                            | 架构决策记录（ADR）目录                                                                        |
| [generated/](generated/README.md)                | 注册表渲染层与生成物（机器生成、禁手改）

### 2.3 依赖关系

```
requirements/ ──┬──> architecture/（目标架构 + 决策建议）
                ├──> design/（实施细则）──> GuliangIAM-开发规范.md
                ├──> acceptance-criteria.md + traceability-matrix.md
                └──> openapi/openapi.yaml
architecture/ ──> adr/（冻结决策）
operations/ ──> 故障演练/恢复证据
```

### 2.4 治理工具

| 工具                                                             | 用途                                                              |
| -------------------------------------------------------------- | --------------------------------------------------------------- |
| [CHANGELOG.md](../CHANGELOG.md)                                | 版本变更历史（唯一变更记录位置）                                                |
| [SECURITY.md](../SECURITY.md)                                  | 漏洞披露和安全支持                                                       |
| [CONTRIBUTING.md](../CONTRIBUTING.md)                          | 贡献、评审和本地验证                                                      |
| [ADR 目录](adr/README.md)                                        | 完整架构决策记录（唯一决策状态源）                                               |
| `scripts/check-doc-anchors.py`                                 | 文档锚点门禁（docs/ 内章节引用可解析）                                          |
| `scripts/check-doc-links.py`                                   | 文档链接门禁（docs/ 与根目录正式文档的相对链接可解析，禁止断链与逃逸仓库根）                            |
| `scripts/check-gate-coverage.py`                               | 门禁接线覆盖（scripts/check-* 必须被调度入口引用；文档中的脚本路径与仓库相对路径必须真实存在）                          |
| `scripts/check-registry-sync.py --strict`                      | 注册表与代码事实源一致性 + 消费覆盖零缺口                                          |
| `scripts/check-generated-zero-diff.sh`                         | 全部生成物零差异门禁（OpenAPI / DDL / TS 类型 / 注册表渲染 / .env.example / 追溯矩阵） |
| `scripts/check-ac-traceability.py`                             | AC ↔ 需求双向追溯完备性                                                  |
| `scripts/check-naming.sh`                                      | 跨层命名唯一裁决扫描                                                      |
| `scripts/check-db-conventions.sh` + `check-sql-portability.sh` | 数据库设计规范 + SQL 可移植性                                              |
| `scripts/check-api-contract.sh` + `check-openapi-contract.sh`  | API 契约门禁                                                        |
| `scripts/check-data-flow.sh`                                   | 数据流闭环门禁                                                         |
| `scripts/check-doc-facts.sh`                                   | 文档事实一致性                                                         |
| `scripts/check-docs-root.py`                                   | docs 根目录布局门禁（仅允许 4 份正式入口；过程证据写入 `.trae/documents/`）              |
| `scripts/check-lifecycle-coverage.py`                          | 数据生命周期覆盖门禁（命中保留/清理类别的表必须有 lifecycle 条目）                              |
| `scripts/check-fk-coverage.py`                                 | 表间引用完整性门禁（UUID `*_id` 列必须物理外键或分类登记）                                    |
| `scripts/check-credential-masking.py`                          | 读接口凭据不回显                                                        |
| `scripts/check-encrypted-columns.py`                           | 加密列注册表 ↔ 仓储 mapper ↔ 声明式 Schema 一致性（含回填列 ⊆ 轮换列）                    |
| `scripts/check-security-readiness.py`                          | 安全就绪度矩阵                                                         |
| `scripts/check-cache-registry.sh`                              | 缓存注册表                                                           |
| `scripts/check-rls.sh` + `check-rls-policy-content.py`         | RLS 强制隔离（租户表声明 / ENABLE+FORCE / 应用角色 / 策略内容非恒真）                          |
| `scripts/check-tenant-isolation.py`                            | 租户隔离分类闭合（direct/indirect/self/platform/global；global 仅 `tenant-isolation.yaml` 登记项） |
| `scripts/check-tenant-provisioning.py`                         | 创建/退役组织的租户接线（同事务写身份与拓扑；退役状态流转 + 软删，禁止回退为只建组织）             |
| `scripts/check-error-response.sh`                              | handler 错误响应必须走统一写入器（禁止裸 WriteHeader+Write）                                    |
| `scripts/check-error-semantics.py`                             | 语义校验失败必须 422（app 层基线，未转换站点见脚本 PENDING）                             |
| `scripts/check-frontend-architecture.py`                       | 前端架构（HTTP 请求必须经 shared/api/http.ts 统一入口）                                  |
| `scripts/check-goroutine-safety.sh`                            | goroutine 安全（长期运行 goroutine 必须 SafeGo 包裹）                                  |
| `scripts/check-target-schema.sh`                               | 目标域表完整性（ADR-024~029 目标表在 Schema 与生成 SQL 中均有定义）                               |
| `scripts/check-ui-wiring.py`                                   | UI 接线门禁                                                         |
| `scripts/check-table-overflow.py`                              | 详情页宽列表格横向滚动门禁（宽列合计 ≥900px 必须声明 `scroll`）                            |
| `scripts/check-write-invalidation.py`                          | 写路径失效门禁（删除/创建 onSuccess 必须失效查询缓存）                                  |
| `scripts/check-content-width.py`                              | 主显示区宽度门禁（控制台页面禁止页面级 max-width，必须用三档语义类）                        |
| `scripts/check-descriptions-responsive.py`                     | Descriptions 响应式门禁（固定多列与固定跨列阻断）                                   |
| `scripts/check-frontend-coverage.py`                           | 前端路由 ↔ 页面 ↔ 菜单闭合                                                |
| `scripts/check-enums.py`                                       | 前端联合类型 ⊆ 状态机注册表                                                 |
| `scripts/check-provider-kinds.py`                              | provider kind 闭集、子系统路由、托管轨道、消费表、运行时 pending 一致（含 messaging 通道与 goth 类型目录） |
| `scripts/check-bundle-budget.py`                               | 首屏字节预算                                                          |
| `scripts/check-token-parity.py`                                | 设计令牌镜像                                                          |
| `scripts/check-i18n-keys.py`                                   | i18n 键完整性                                                       |
| `scripts/check-layers.sh`                                      | 分层依赖方向                                                          |
| `scripts/verify-docs.sh`                                       | 文档门禁总入口（调度全部门禁）                                                 |

> 门禁数量与通过数不在文档中手写；以 `bash scripts/verify-docs.sh` 和 `python3 scripts/check-gate-coverage.py` 的实时输出为准。CI 分层见 `.github/workflows/ci.yml`：backend 负责 Go 构建/测试与 PostgreSQL schema/RLS，frontend 负责前端构建与首屏预算，linter 负责 golangci-lint，gates 负责静态契约、文档门禁与 PostgreSQL sql-runtime/回滚。

## 3. 单一事实源注册表

### 3.1 注册表治理

同一事实只允许一个**可执行事实源**；规范文档定义唯一口径，需求、设计、验收只引用、不复制。

| 事实         | 唯一事实源                                                                                        | 生成物                                                                   | 其他文档的正确做法                 |
| ---------- | -------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- | ------------------------- |
| 数据库结构      | `contracts/schema/*.yaml`（[ADR-022](adr/ADR-022-declarative-schema-versioned-migration.md)）         | 四方言 DDL、版本化迁移、回滚                                                      | 只写表名与逻辑类型，不复制列清单          |
| 运行时路由      | `internal/transport/http/**/routes.go` + `route.Definition`（代码声明）                              | `openapi/openapi.yaml`（生成物；复现：`python3 scripts/gen_openapi.py --check`）、路由契约测试                                                                                 | 路由清单不手写，引用生成结果 + 复现命令            |
| API 契约     | `openapi/openapi.yaml`（生成物；声明源为路由定义，`scripts/gen_openapi.py` 生成，[ADR-012](adr/ADR-012-openapi-source.md)） | 服务端嵌入 `/openapi/*`、契约测试、前端 TS 类型（error-codes.ts / state-machines.ts / api-contracts.ts）       | 只引用生成结果与复现命令；生成物物理位置裁决见 ADR-012 |
| 应用错误码      | `contracts/registries/errors.yaml`                                                                | OpenAPI enum、前端 error-codes.ts                                        | 只引用码名，不得新增未登记码            |
| 协议错误码      | `contracts/registries/errors.yaml` protocol 段（渲染层 `docs/generated/errors-registry.md`）                                                                           | 协议错误写入器、测试向量                                                          | 同上                        |
| 权限点        | `route.Definition.Permission` + OpenAPI 生成器                                                  | Casbin 策略、前端权限                                                        | 权限点清单不手写；域级清单另见 `contracts/registries/permissions.yaml` |
| 状态机        | `contracts/registries/state-machines.yaml`                                                        | DDL CHECK、Go 领域常量、前端 state-machines.ts                                | 状态值不散写，引用注册表              |
| 安全能力就绪度    | `contracts/registries/security-readiness.yaml`                                                    | —                                                                     | 各文档能力声称以其为准               |
| 环境变量       | `contracts/registries/env.yaml`                                                                   | `.env.example`（`scripts/gen_env_vars.py` 生成）                          | 部署指南只引用，不写清单              |
| 加密能力位      | `contracts/registries/crypto.yaml`（能力位 ID、缺失策略、`default`/`gm` 档位）                                  | `docs/generated/crypto-registry.md`（渲染层）；启动裁决在 `internal/platform/crypto/suite.go` | 只引用能力位 ID，不复写策略表 |
| 开放项标识      | `contracts/registries/open-items.yaml`（`OPEN-*` 编号只增不复用）                                          | —                                                                     | 代码注释与文档只写标识               |
| 限流参数       | `contracts/registries/limits.yaml`                                                                | 部署配置                                                                  | 其他章节只引用                   |
| SLO / 错误预算 | `contracts/registries/slo.yaml`                                                                   | 告警规则、看板                                                               | 需求/设计只引用                  |
| Outbox 参数  | `contracts/registries/outbox.yaml`                                                                | Worker 配置                                                             | 只引用                       |
| 缓存策略       | `contracts/registries/cache.yaml`                                                                 | —                                                                     | 只引用                       |
| API 三面分类   | `contracts/registries/surfaces.yaml` + `route.Definition.SurfaceFor`                              | api-contracts.ts（骨架 + 契约组件）                                           | 只引用                       |
| 数据生命周期     | `contracts/registries/lifecycle.yaml`（含 `coverage.required_classes` 与默认策略；覆盖门禁 `scripts/check-lifecycle-coverage.py`） | `docs/generated/lifecycle-registry.md`（渲染层）                                         | 需求/设计只引用                  |
| 短生命周期状态 | `contracts/registries/short-lived-state.yaml`                                                      | 令牌/授权码/会话短周期状态约束                                                   | 只引用                       |
| 领域事件注册表 | `contracts/registries/events.yaml`                                                                | DomainEvent 清单、Outbox Worker 消费表、审计链锚定                                    | 事件名不散写，引用注册表              |
| 表间引用完整性 | `contracts/registries/referential-integrity.yaml`（UUID `*_id` 列的物理 FK / 逻辑引用 / 待补 FK 分类） | `docs/generated/referential-integrity-registry.md`（渲染层）；物理 FK 本体在 `contracts/schema/*.yaml` 的 `fk_constraints` | 只引用分类，不另写第二份外键清单 |
| 问题与风险清单 | `contracts/registries/known-red.yaml`（severity 严重等级 / impact_scope 影响范围 / baseline_failures 量化基线 / exit_condition 出口条件） | `docs/generated/known-red-registry.md`（渲染层）；架构级风险另见 `architecture/README.md`「架构风险登记」 | 只引用规则标识与严重等级，不复写基线数字 |
| 文档版本沿革     | 仓库根 `CHANGELOG.md`                                                                           | 各文档头部版本号                                                              | 文档头部只写「版本 + CHANGELOG 指针」 |
| ADR        | `docs/adr/README.md` + 各 ADR                                                                 | —                                                                     | 其他文档不复写决策状态               |
| 跨层命名       | `contracts/registries/naming.yaml`（渲染层 `docs/generated/naming-registry.md`）                        | 各层拼写裁决表（N1–N15）                                                       | 只引用裁决，不得自造同义词或缩写             |
| 术语         | 本文件 §5                                                                                       | —                                                                     | 不得自造同义词                   |

## 4. 生成链

一切可生成事实由事实源生成产出，CI 以零差异门禁拦截漂移。

```text
事实源（YAML 注册表 / 声明式 Schema / 路由声明）
    │
    ├─ gen_schema.py ──────────> 四方言 DDL + V1 基线 + V{N} 增量迁移 + down.sql ✅
    ├─ gen_domain_constants.py > Go 不变式常量 + 前端 state-machines.ts ✅
    ├─ gen_error_codes.py ────> 前端 error-codes.ts ✅
    ├─ gen_api_contracts.py ──> 前端 api-contracts.ts ✅
    ├─ scripts/gen_openapi.py ──> openapi/openapi.yaml（路由声明驱动） ✅
    ├─ scripts/gen_registry_md.py ──> docs/generated/ 注册表 md 渲染层 ✅
    ├─ scripts/gen_frontend_registry.py ──> design/frontend/registry.md 菜单表 + 页面注册表 ✅
    ├─ scripts/gen_env_vars.py ──> .env.example（env.yaml 驱动） ✅
    └─ scripts/gen_traceability.py ──> traceability-matrix.md（全仓锚点扫描） ✅

下游消费：
    DDL ──────────> PostgreSQL/MySQL/GaussDB/KingBaseES 执行
    OpenAPI ──────> 契约测试 + 前端 SDK
    Go 常量 ──────> domain 层 import
    TS 类型 ──────> 前端 import type from '@/generated/xxx'
    .env.example ──> 部署复制 → .env.local
    注册表渲染 ──> check-registry-sync.py --strict 零漂移
    追溯矩阵 ──> check-ac-traceability.py 完备性
```

**零差异门禁**：`bash scripts/check-generated-zero-diff.sh` 串联以上全部 `--check`，任一失败阻塞合并。

生成物禁止手改；生成器不可表达的例外必须登记 ADR。

## 5. 术语约定

- **组织（Organization）**：租户隔离边界；资源隔离分级在 architecture/data-model.md 声明。
- **应用层（App）**：编排用例和事务，不直接依赖基础设施实现。
- **领域端口**：由 Domain 定义、由 Infra 实现的接口。
- **协议入口**：OAuth/OIDC、SAML、CAS、SCIM、LDAP、RADIUS、Kerberos、WebAuthn 等外部契约。
- **管理面 / 认证面 / 协议面**：API 按契约与消费者的三类划分。
- **`surface`**：API 指标与访问日志的三面标签，取值仅限 `management` / `authentication` / `protocol`；`contracts/registries/limits.yaml` 的 `surface` 字段是「限流挂载面」（`all` / `auth` / `protocol`），两者不是同一枚举，禁止混用。
- **PEP / PDP / PIP**：策略执行点 / 策略决策点 / 策略信息点。
- **Identity Warehouse**：身份仓库（用户、凭证、会话、授权事实的统一视图）。
- **Outbox**：同事务写业务事实与事件、由 Worker 异步投递的收件箱模式（ADR-008/017）。
- **RLS**：PostgreSQL 行级安全（Row Level Security），租户隔离的数据库强制层（ADR-016）。
- **跨层命名**：概念的 JSON/SQL/URL/claim 拼写唯一裁决见 `contracts/registries/naming.yaml`（渲染层 `docs/generated/naming-registry.md`）。

