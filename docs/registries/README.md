# 注册表目录（Registries）——数据唯一口径

- **定位**：接口/数据型唯一口径的**唯一事实源为 YAML**，落点 `contracts/registries/*.yaml`。本目录（`docs/registries/`）现仅保留本 README 作为导航与治理说明，不再承载文件本体。规范文档只引用注册表，不得复写其中的数值、清单与枚举。
- **渲染层**：YAML 的机器生成渲染 md 位于 `docs/generated/`（由 `scripts/gen_registry_md.py` 产出，禁止手改）。
- **与文档体系的关系**（三层架构）：
  1. **注册表层**（`contracts/registries/*.yaml`）——数据唯一口径，变更即登记 `CHANGELOG.md`；
  2. **规范层**（docs/ 正式文档）——意图、边界、决策、验收；引用注册表，不复写；
  3. **生成物层**（`docs/generated/*.md`、`openapi/openapi.yaml`、DDL/迁移/前端类型/端点清单）——禁止手改，CI 零差异门禁。
- **门禁**：本目录文件由 `scripts/verify-docs.sh` 一并校验；注册表内容与代码事实源冲突时，以代码为准并在同一变更中修正注册表。

## 文件清单

**YAML 唯一事实源**（`contracts/registries/`，清单以目录为准，复现：`ls contracts/registries/*.yaml`）：

| YAML | 内容 | 代码事实源 |
|---|---|---|
| [naming.yaml](../../contracts/registries/naming.yaml) | 术语与跨层命名映射（概念 → 中文/英文/JSON/SQL/URL/协议 claim） | —（命名裁决表） |
| [errors.yaml](../../contracts/registries/errors.yaml) | 应用错误码与协议错误码全量清单 | `internal/platform/errors/errors.go` |
| [env.yaml](../../contracts/registries/env.yaml) | 环境变量全量清单 | 代码读取点（`os.Getenv` 等口径） |
| [state-machines.yaml](../../contracts/registries/state-machines.yaml) | 全量状态机：状态值、合法迁移边、终态、DDL CHECK 映射 | 领域常量（`validXxxStates`/`CanTransitionTo`） |
| [limits.yaml](../../contracts/registries/limits.yaml) | 限流器参数唯一权威表 | `internal/platform/middleware/ratelimit.go` |
| [slo.yaml](../../contracts/registries/slo.yaml) | 三面 SLO / 错误预算唯一权威表 | 告警规则与看板配置 |
| [security-readiness.yaml](../../contracts/registries/security-readiness.yaml) | 安全能力就绪度矩阵：设计状态 / 实现状态 / 验收证据 / 出口条件 | 各设计章节与 ADR |
| [outbox.yaml](../../contracts/registries/outbox.yaml) | Outbox Worker 参数、Webhook 投递参数、去重层次（**max_attempts=12 唯一源**） | Outbox Worker 实现 |
| [cache.yaml](../../contracts/registries/cache.yaml) | Redis 缓存分层、TTL/抖动/失效策略、键命名规则 | `internal/infra/cache/` + Redis Client 初始化 |
| [surfaces.yaml](../../contracts/registries/surfaces.yaml) | API 三面分类（显式端点面归属）、协议面家族清单、端点契约要素、公开白名单 | `route.Definition` 声明 + `internal/platform/registry/surface.go`（唯一分类器，`transport/http/route` 类型别名桥接） |
| [open-items.yaml](../../contracts/registries/open-items.yaml) | 开放项稳定标识（`OPEN-*`）：能力缺口、当前处置、关闭条件、代码引用点 | 引用点自身（编号只增不复用） |
| [lifecycle.yaml](../../contracts/registries/lifecycle.yaml) | 数据生命周期（保留/归档/清理/隐私擦除策略 + `coverage.required_classes` 覆盖口径） | 结构门禁 `scripts/check-registry-sync.py` + 覆盖门禁 `scripts/check-lifecycle-coverage.py` + 渲染层；保留/清理任务以代码实现为准 |
| [permissions.yaml](../../contracts/registries/permissions.yaml) | 域级权限点清单 | 结构门禁 `scripts/check-registry-sync.py` + 渲染层；逐端点权限点唯一源为 `route.Definition.Permission` |
| [events.yaml](../../contracts/registries/events.yaml) | 领域/审计事件名按域登记（事件名 + 域） | 结构门禁 `scripts/check-registry-sync.py` + 渲染层；事件常量生成见 `contracts/registries/events.yaml` 与 `scripts/check-registry-sync.py` |
| [short-lived-state.yaml](../../contracts/registries/short-lived-state.yaml) | 短期状态矩阵：存储位置 / TTL / 多区域 / 失败姿态 / 清理 | 专用门禁 `scripts/check-short-lived-state.py`（四要素闭集校验）；Redis/PG 实现逐项对齐 |
| [referential-integrity.yaml](../../contracts/registries/referential-integrity.yaml) | 表间引用完整性裁决：UUID `*_id` 列的物理 FK / 逻辑引用 / 待补 FK 分类与理由 | 门禁 `scripts/check-fk-coverage.py` + 渲染层 |
| [known-red.yaml](../../contracts/registries/known-red.yaml) | **问题与风险清单** / 门禁已知红项基线：severity 严重等级、impact_scope 影响范围、baseline_failures 量化基线、exit_condition 出口条件 | `scripts/verify-docs.sh` 的 `run_gate` 基线机制 + `scripts/check-registry-sync.py` 结构校验 + 渲染层 |
| [crypto.yaml](../../contracts/registries/crypto.yaml) | 加密能力位（`sm2_sign`/`sm2_encrypt`/`sm3`/`sm4`/`gm_tls`）、缺失策略与 `default`/`gm` 档位 | `internal/platform/crypto/suite.go` 启动裁决 + `scripts/check-registry-sync.py` 结构校验 + 渲染层 |
| [dr.yaml](../../contracts/registries/dr.yaml) | RTO/RPO 三档与恢复演练机器可读口径（L1/L2/L3 场景、机制、演练频率） | `scripts/check-registry-sync.py` 三档覆盖断言；人类可读说明 `docs/operations/backup-dr.md` §7（source_ref） |
| [grant-types.yaml](../../contracts/registries/grant-types.yaml) | OAuth/OIDC grant type 闭集（可保存 = 可换发，OPEN-API-68） | `scripts/gen_grant_types.py` 生成链 + `scripts/check-registry-sync.py` 结构校验 |
| [password-change-exempt-routes.yaml](../../contracts/registries/password-change-exempt-routes.yaml) | 强制改密门禁豁免路由登记（中间件只查生成物，不写路径字符串） | `scripts/gen_password_change_exempt.py` → `password_change_exempt_gen.go`（禁止手改生成物） |
| [provider-kinds.yaml](../../contracts/registries/provider-kinds.yaml) | provider kind 闭集 → 消费子系统映射（routable/托管轨道/运行时 pending） | `scripts/check-provider-kinds.py` 专用门禁 |
| [route-resource-exemptions.yaml](../../contracts/registries/route-resource-exemptions.yaml) | 路由 Permission 声明与 `resourceFromPath` 派生不一致的过渡豁免（必须带 expires_on，过期即红） | `scripts/check-route-resource-derivation.py` 专用门禁 |
| [tenant-isolation.yaml](../../contracts/registries/tenant-isolation.yaml) | 租户身份与隔离拓扑分类（direct/indirect/self/platform/global；global 仅登记项） | `scripts/check-tenant-isolation.py` + `scripts/check-tenant-provisioning.py` 专用门禁 |

**渲染层 md**（`docs/generated/`，由生成器产出，禁止手改）：`naming-registry.md`、`errors-registry.md`、`env-registry.md`、`state-machine-registry.md`、`limit-registry.md`、`slo-registry.md`、`security-readiness.md`、`outbox-registry.md`、`cache-registry.md`、`surfaces-registry.md`、`open-item-registry.md`、`lifecycle-registry.md`、`event-registry.md`、`permissions-registry.md`、`known-red-registry.md`、`referential-integrity-registry.md`、`crypto-registry.md`、`provider-kinds-registry.md`。渲染层是否存在的判据是「是否有唯一事实源 YAML」，清单由 `python3 scripts/gen_registry_md.py --check` 与 `scripts/check-generated-zero-diff.sh` 强制。

## 不入本目录的注册表（唯一事实源声明）

| 注册表 | 唯一事实源 | 理由 |
|---|---|---|
| API 端点清单 | `openapi/openapi.yaml`（生成物） | 由 `scripts/gen_openapi.py` 生成 |
| 数据库表/列/索引 | 声明式 Schema（[ADR-022](../adr/ADR-022-declarative-schema-versioned-migration.md)） | 结构事实源，ER 图与列清单为生成物 |
| 术语定义 | 《开发规范手册》术语表 + `docs/generated/naming-registry.md` 的跨层命名映射 | 语义定义在手册，命名映射在注册表 |
| 权限点核准清单 | `route.Definition.Permission` 字段（`internal/transport/http/api/v1/**/routes.go`）+ `scripts/gen_openapi.py` 生成物 | 逐端点权限点由生成器产出；域级清单见 `contracts/registries/permissions.yaml` |
| 事件 payload schema | 待建（事件名已入 `contracts/registries/events.yaml`） | 事件名与 payload 是两层事实：payload schema 引入写路径校验器后建注册表并接入生成链，在此之前不建无消费方的 YAML（docs/README.md §3 单一事实源纪律） |

## 机器可读化出口条件

YAML 文件已全部落地为唯一事实源。剩余纪律：

1. **当前形态**：每张注册表对应一个 YAML 文件，由生成链消费产出 DDL CHECK、Go 常量、前端枚举、限流配置、SLO 看板配置、渲染 md。
2. **渲染层迁移纪律（防双源）**：YAML 为唯一源；渲染 md 一律为「渲染层」（自动生成、禁止手改），落点 `docs/generated/`。CI 校验 YAML → 渲染 md 的零差异。
3. **消费方登记**：每个 YAML 的消费方（生成器目标或门禁脚本）登记于本 README 文件清单。`contracts/registries/` 下全部 YAML 均已有门禁消费方（`python3 scripts/check-registry-sync.py --strict` 零缺口）；无渲染层的注册表由专用门禁消费（如 `known-red.yaml` → `verify-docs.sh::run_gate`、`short-lived-state.yaml` → `check-short-lived-state.py`）。
4. **不入册项不变**：API 端点清单的唯一事实源仍是生成器与代码（见上文「不入本目录的注册表」）。

## 引用规则

1. 规范文档引用注册表一律使用「`contracts/registries/` 文件名 + 表/小节锚点」或渲染层锚点（如「见 `docs/generated/errors-registry.md` 应用错误码表」）。
2. 注册表内部条目变更（新增错误码、调整 SLO、新增状态值）必须在同一变更中：更新代码事实源 → 更新注册表 → `CHANGELOG.md` 登记。
3. 规范文档中发现与注册表不一致的数值/清单，**一律以 `contracts/registries/*.yaml` 为准**（文档 ↔ 注册表方向），并在同一变更中修正复写处；注册表与代码事实源冲突时，以代码为准并在同一变更中修正注册表（注册表 ↔ 代码方向，见本文头部门禁说明）。两条规则适用方向不同，不得混用。
