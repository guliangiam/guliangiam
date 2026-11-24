# ADR-024: ReBAC 关系元组授权（OpenFGA 兼容 API + PostgreSQL 自研存储适配）

- 状态：[Current]/[Target] — 存储适配器、关系求值器与 OpenFGA 兼容 API 已落地并挂路由；P99 压测与 Zookie 一致性为 Target（出口条件满足后转 [Current]）
- 日期：2026-09-14
- 关系：叠加 ADR-003（Casbin 端点级策略）与 ADR-016（RLS 租户兜底），不替换任一方；决策顺序唯一正文见 `architecture/layers.md` §5

## 背景

SRS REBAC-001~008、REBAC-010 要求对象级细粒度共享能力：关系元组 `(user, relation, object)`（REBAC-001/002）、对象树继承（REBAC-003）、组嵌套（REBAC-004）、Zookie 写后读一致性（REBAC-005）、Why 溯源（REBAC-006）。Casbin 仍承载粗粒度「主体→角色→资源类型→动作」策略（`internal/infra/casbin/manager.go`），ReBAC 已作为对象级授权的叠加层落地：关系元组存储/求值器/OpenFGA 兼容 API 均已实现（见验证证据），不再只是「无承载落点」的规划。目标态数据落点为 `relation_tuples` / `relation_namespaces`（[3.2](../architecture/data-model.md#32-repository-约定)，目标表 `relation_tuples` / `relation_namespaces` 已随生成式 V1 基线声明；运行时求值与粒度展开见本 ADR 验证证据）；决策顺序与三分工唯一正文在 [layers.md §5](../architecture/layers.md#5-授权决策顺序)；[security-model §5](../architecture/security-model.md#5-权限决策基础引用) 只做引用索引。

## 决策

1. **API 形态采用 OpenFGA 兼容语义**：`check` / `write`（元组批量写删）/ `expand`（展开链路）/ `batch check` / `list-objects`（双向枚举，REBAC-002 验收 3）接口与 Zookie 一致性令牌语义对齐 OpenFGA 公开 API；authorization model（命名空间与关系展开规则）以 JSON 定义并版本化于 `relation_namespaces`（`relation_defs` + `version`，REBAC-010）。
2. **不引入独立 OpenFGA 服务进程，存储适配自研并复用 PostgreSQL**：元组与命名空间存 `relation_tuples` / `relation_namespaces`（7.8.2），复用既有连接池、租户列 `organization_id` 行过滤（ADR-016 RLS 落地后同受数据库强制）、备份体系；Watch 实时订阅（REBAC-007）经进程内事件流实现，不新增消息中间件。
3. **与 Casbin 共存且决策顺序固定**（引用 [layers.md §5](../architecture/layers.md#5-授权决策顺序)，不另立顺序）：Casbin RBAC 先行（PEP 装配点不变，6.4.1）→ 路由声明对象级资源类型时叠加 ReBAC `check` → ReBAC deny 覆盖 Casbin allow → ReBAC 引擎不可用按 8.8 fail-close 返回 503，不回退 Casbin allow。
4. **组嵌套复用既有 `groups` 表作为主体来源**（REBAC-004 明示不重建组模型），元组侧仅存 `subject_relation` 引用。
5. **红线沿用 6.4.2**：ReBAC 不接管租户隔离（`TenantFilterMiddleware` + RLS）与字段级权限（8.10）；`object` 命名含租户归属但租户边界强制不依赖 ReBAC 求值正确性。

## 理由与后果

Zanzibar 语义（递归展开、环检测、一致性令牌、命名空间版本化）是本项目自主实现成本最高的部分；OpenFGA API 已是关系授权的事实接口，兼容它可获得成熟语义参考、SDK/测试向量与未来平滑外迁路径。存储复用 PostgreSQL 避免在单体内新增有状态组件与双存储一致性问题。代价：需实现展开求值器（继承深度受控 + 环检测，REBAC-003 验收 3/4）、Zookie 快照语义、模型不兼容变更的迁移门禁（REBAC-010 验收 3）；`relation_tuples` 高写入量须评估时间分区（挂 ADR-011）。

## 替代方案

- **纯自研 DSL + 求值引擎**：语义复杂度等同重新发明 Zanzibar——展开规则、环检测、写后读一致性、Why 溯源全部重造且无可复用的测试向量；放弃。
- **Cedar 策略语言（AWS）**：Cedar 是「策略求值」模型，与「关系元组 + 展开继承」模型错位：无原生元组存储/递归展开/Zookie 语义，引入后仍需自建元组层，等于两套抽象并存；放弃。
- **直接部署独立 OpenFGA 服务进程**：新增有状态组件与跨存储一致性（租户数据出 PG，与 ADR-016/驻留约束交叠），单体第一阶段不引入；保留 API 兼容后路，规模证据充分时再评估（复审条件触发点之一）。

## 验证证据

- Current：`grep -rn 'relation_tuples\|RelationTuple' internal/ db/ --include='*.go' --include='*.sql'` 有命中（行数以命令输出为准）——实现包含 `internal/infra/postgres/rebac/` 与 `internal/infra/mysql/rebac/` 存储适配器、关系求值器（`internal/app/rebac/`、`internal/domain/rebac/`）、OpenFGA 兼容路由（`internal/transport/http/api/v1/rebac/`，组合根 `internal/bootstrap/chi.go` 注册）；`internal/infra/casbin/manager.go` 仍在链上、决策顺序按本 ADR 决策 3。
- Target 出口条件：①关系解析 check P99 < 50ms（L2 压测口径，挂 PERF 基线）；②万级元组批量写入/批量检查压测达标且事务性幂等（REBAC-006 验收 3）；③Zookie 写后读一致性用例通过（REBAC-005）；④与 Casbin 决策顺序（6.4.2 五条）回归测试；⑤命名空间不兼容变更被迁移门禁拦截。待取证项：`relation_tuples` 分区评估结论（ADR-011）、check API 直连/继承/组展开用例集、跨实例一致性演练。

## 复审条件

出口条件全部满足后转 `[Current]`；若引入独立 OpenFGA 服务或更换存储（非 PostgreSQL），须以新 ADR 取代本 ADR 并在索引标注取代关系。
