# ADR-012: OpenAPI 作为 API 契约源（生成链产出字段级 Schema）

- 状态：[Current]/[Target]（路由→路径骨架生成已建成；字段级 Schema 契约源与类型生成链为 [Target]）
- 日期：2026-09-17（目标态重构；原决策 2026-09-13）

## 背景

前端、协议面集成方（RP/SP/SCIM 客户端）与外部开发者都需要稳定的 API 契约；在 500+ 操作规模下手工维护端点清单必然与实际路由漂移。契约必须从代码中的路由定义单向生成，并纳入变更纪律。

同时必须承认生成链的现状边界：路由→路径骨架生成已建成且与运行时注册双向一致；但请求/响应 Schema 仍是"按 operationId 前缀套模板"的自由对象（`List*`→`PaginatedResponse{items[自由对象]}`、`Get*`→`ResourceDetail`），字段级契约缺失，前端类型只能手写维护——这违背"唯一事实源、数据不漂移"的总原则。

## 决策

1. **路由声明是端点唯一事实源**：`internal/transport/http/api/v1/**/routes.go` 的 `route.Definition`（Method/Pattern/OperationID/Summary/Permission/Surface）声明集必须与运行时注册双向差集为空；`openapi/openapi.yaml` 由生成器产出，端点数量与 operationId 不得手工维护；生成器发现非白名单重复路由时必须失败。
2. **请求/响应 Schema 从类型化契约源生成字段级定义**（[Target]）：以 Go DTO 类型为契约源（结构体字段标注 binding/openapi tag）或 TypeSpec/IDL 描述，生成器产出字段级 Schema 并替换自由对象模板；同一契约源同步产出前端 TypeScript 类型与 zod schema，删除手工维护的 `web/src/shared/api/**` 类型与 `search→keyword`、`sortBy→sortField` 式人工映射。
3. **契约参数全量入 OpenAPI**：分页（`page`/`pageSize`/`cursor`）、排序（`sortField`/`sortOrder`）、搜索（`keyword`）、过滤（`filters`）与幂等并发头（`Idempotency-Key`/`ETag`/`If-Match`）必须作为契约组件声明，禁止"后端读取但契约未声明"的暗参数。
4. **契约测试闭环**：生成物包含契约测试骨架；前端请求参数与 handler 读取参数的一致性由门禁脚本校验（`scripts/check-api-params.py`）。
5. **同一变更纪律**：路由、DTO 契约源、错误码与 `openapi/openapi.yaml` 必须在同一变更中更新；生成 diff 与 `scripts/check-openapi-contract.sh` 为 CI 阻塞步骤。

## 理由与后果

- 正面：契约与注册路由同源，operationId 可与权限点、追踪和文档交叉追溯；字段级 Schema 使前端类型、SDK、契约测试全部进入生成链，消除手工同步漂移；路由重复/缺失在生成期即暴露。
- 负面：DTO 类型标注要求业务代码维护契约注解（或引入 TypeSpec 的双源同步成本）；PR 作者必须运行生成器并提交产物 diff，评审多一道工序；协议面特化行为（错误格式、限流组）仍需代码侧表达，生成器不能替代。

## 替代方案

- **先手写 OpenAPI，再由 codegen 生成服务端路由**：放弃理由——chi 路由承载协议面特化的中间件链、限流组与标准错误写入器，通用 codegen 无法表达这些约束，会把协议面错误格式等关键行为挤出代码。
- **纯手工维护 openapi.yaml**：放弃理由——500+ 操作手写必然漂移。
- **维持自由对象 Schema 模板**：放弃理由——字段级契约缺失使前端类型手写、契约测试无法断言字段，违背唯一事实源原则。
- **不要机器契约，靠文档描述**：放弃理由——无法被前端 SDK、测试与合规评审消费，评审无法对 diff。

## 验证证据

**[Current]**：`python3 scripts/gen_openapi.py` 从路由定义生成 `openapi/openapi.yaml`；生成器无重复声明白名单，任一路径+方法重复声明硬失败；路由契约测试（`internal/bootstrap/route_contract_test.go`）验证声明集与运行时注册双向差集为空；OpenAPI 零差异由 `scripts/check-generated-zero-diff.sh` 与 `scripts/check-api-contract.sh` 双门禁校验，二者在生成器或产物缺失时**硬失败**（禁止 `if [ -f ]` 静默跳过），并接入 CI `gates` 作业。

**[Target] 出口条件**：字段级 Schema 契约源落地并替换自由对象模板；前端 TS 类型与 zod schema 由契约源生成（`web/src/shared/api/**` 手写类型删除）；`Idempotency-Key`/`ETag`/`If-Match`/`keyword` 等参数与头入契约；契约测试骨架接入 CI。

## 生成链目录规划

生成链的物理布局分两层，约束「声明唯一源 → 生成物」的单向依赖：

- **声明层**：`contracts/`（DTO/IDL 契约源）与 `routes/`（`route.Definition` 或等价声明）。现状为 `internal/transport/http/api/v1/**/routes.go` 代码内声明——是否迁移到独立声明文件由出口条件裁决，不在本 ADR 强制。
- **生成物层**：生成物集合包含 `openapi/openapi.yaml`、`db/`（ADR-022 的 migrations/dialects）、`web/src/generated/`（前端 TS 类型 + zod）、`docs/generated/`（端点清单/ER 图渲染）。**生成物保持各自既有物理位置，不强制合并为单一 `generated/` 目录**（避免无收益的大挪移）；「generated/」是概念集合，由 CI 零差异门禁统一约束。
- **CI 零差异**：声明层变更 → 运行生成器 → 与工作区比对，不一致即失败（与 ADR-022 门禁第 9 项「生成物零差异」同一机制，共用门禁基础设施）。

### 生成物物理位置裁决：OpenAPI 留在 `openapi/`

`contracts/` 只承载**声明层**（`schema/` 结构声明、`registries/` 运行时事实），不承载由声明生成的产物。OpenAPI 生成物与其运行时消费包同址，即 `openapi/openapi.yaml`：

- **决策**：`scripts/gen_openapi.py` 的唯一输出路径固定为 `openapi/openapi.yaml`；`contracts/` 下**不设** API 契约子目录。
- **理由**：生成物由 `openapi/embed.go` 以 `go:embed` 编译进二进制并对外提供 `/openapi/openapi.yaml` 端点。Go 的 `go:embed` 只能嵌入**同包目录**内的文件；若把生成物移入 `contracts/`，只能二选一——把 Go 包迁入 `contracts/`（声明目录混入代码，声明/生成物分层被破坏），或构建期复制一份副本（同一事实两个物理副本，正是零差异门禁要消灭的形态）。
- **trade-off**：契约消费方（前端类型生成、契约测试）需要多记一个路径，不如「所有契约类资产都集中在 `contracts/`」直观；换来的是声明/生成物分层完整、无构建期复制步骤、embed 依赖保持最小。权衡结论：**分层纯度与单一物理副本优先于路径整齐**。
- **防漂移**：`openapi/openapi.yaml` 是生成物，禁止手改；一致性由 `python3 scripts/gen_openapi.py --check` 判定，`scripts/check-generated-zero-diff.sh` 与 `scripts/check-api-contract.sh` 强制执行，生成器缺失即 FAIL。

## 复审条件

字段级契约源落地后更新本 ADR 证据；TypeSpec 与 Go DTO 标注两条路线选型复核；协议面家族路由分组（`contracts/registries/surfaces.yaml`）收敛后复核契约 tag 结构。
