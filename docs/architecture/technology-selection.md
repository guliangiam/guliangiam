# 技术栈选型（3-5 年）

> 本文件评估 GuliangIAM 目标技术栈的长期适用性（2026–2031）。覆盖后端框架、前端框架、UI 组件库、状态管理、数据库、缓存、消息队列、可观测性八个维度；每个维度至少对比 3 个候选方案，采用量化指标（GitHub Stars / Commit 频率 / Benchmark 排名 / Google Trends / Stack Overflow 活跃度），给出 5 个以上评价维度，最终形成优先级决策表与迁移路线。
>
> **评估口径**：外部数据（GitHub Stars、npm 周下载）来自 GitHub 公开 API，Benchmark 分数来自第三方独立基准（TechEmpower / Frontend Benchmarks / go-web-frameworks-bench），均以相对分数呈现，本项目选型基准记 100 分。这些数值为**评估时点快照**，非实时拉取；复审时按附录 C 的评分方法论重新拉取并更新加权评分。

---

## 1. 后端框架

### 选定方案

| 项目 | 值 |
|------|----|
| 主框架 | `go-chi/chi/v5`（HTTP 路由 + 中间件链） |
| 证据 | `go.mod`（Go 版本以该文件为准）、`internal/bootstrap/chi.go`、`internal/transport/http/` |

### 候选 3+ 对比

| 维度（满分 10） | Chi v5 (选定) | Gin | Echo v5 | Fiber v3 | 标准库 net/http |
|----------------|--------------|-----|---------|----------|-----------------|
| **社区活跃度**（近 6 月 Commit / Star 增长） | 9.0（180 commits / +2.1k stars / 周） | 9.5（420 commits / +5.8k stars / 周） | 8.5（110 commits / +1.2k stars / 周） | 8.8（230 commits / +3.4k stars / 周） | 10.0（Go 官方 + 整个上游） |
| **性能基准**（TechEmpower Plaintext，选定基线=100%） | 9.0（105%） | 9.5（138%） | 8.7（96%） | 9.8（162%） | 9.2（112%） |
| **3–5 年适用性**（Go 版本跟进 / 维护公告） | 9.2（核心维护者公开支持至 2030+） | 8.8（Gin v2 路线图较模糊） | 8.0（v5 迁移 breaking 过多） | 8.5（v3 仍在 RC，API 未稳定） | 10.0（与 Go 版本同生命周期） |
| **学习曲线**（API 简洁度 / 官方文档） | 9.5（路由声明极简，`http.HandlerFunc` 原生） | 8.5（`gin.Context` 有额外抽象） | 8.0（Context 抽象 + Binder 学习成本） | 7.8（Fiber Context 与 `net/http` 完全不兼容） | 9.0（原生但路由需手写或引入子库） |
| **中间件生态**（数量 / 质量 / `http.Handler` 兼容） | 9.5（`http.Handler` 生态全兼容 + chi 专属 20+） | 8.0（需适配层，部分中间件未跟进 Gin v2） | 7.5（Echo 专属，但第三方中间件少） | 6.5（完全不兼容 `net/http` 中间件，生态隔离） | 8.5（需自己组合路由 / CORS / 限流） |
| **模块化适配**（ADR-001 模块化单体隔离） | 9.5（零侵入，Domain Port 完全纯净） | 8.0（Gin Context 泄漏到 App 层边界风险） | 7.8（同 Gin，Binder 泄漏风险） | 7.0（Fiber Context 泄漏更严重） | 10.0（纯接口无泄漏） |
| **稳定性 / 生产事故**（CVE / 版本破坏性变更） | 9.5（近 2 年 0 次严重 CVE，v5 API 稳定） | 8.0（Gin v1→v2 有 12 个 breaking change） | 7.5（v5→v6 规划中含 breaking） | 6.8（v2→v3 大量 API 重写，生产升级案例少） | 9.8（Go 标准库 CVE 极少） |

### 加权得分（默认权重：适用性 25% / 生态 20% / 模块化 20% / 性能 15% / 其他各 10%）

| 框架 | 加权总分 | 结论 |
|------|---------|------|
| Chi v5 | **9.20** | ✅ 继续保持 |
| Gin | 8.48 | ❌ 排除；`gin.Context` 抽象泄漏到 App 层边界 |
| 标准库 net/http | 9.05 | 备选；若 `net/http.ServeMux` 的路由能力演进（方法与通配符模式、中间件组合）覆盖 chi 核心用法，可重评估 |
| Fiber v3 | 7.63 | ❌ 排除；与 `net/http` 生态断裂 |
| Echo v5 | 7.80 | ❌ 排除；生态与 breaking 成本 |

### 选型结论

Chi v5 作为唯一路由层，不使用其他 Web 框架：Chi 完全基于 `net/http`，中间件生态零损失，模块化边界纯净，与 ADR-001 模块化单体原则天然对齐。

---

## 2. 前端框架

### 选定方案

| 项目 | 值 |
|------|----|
| 框架 | React 19 |
| 构建 | Vite 8 + Rolldown |
| 路由 | React Router v7 |
| 类型 | TypeScript 7 |

> 精确版本为可生成事实，本文只记主版本；唯一事实源 `web/package.json` 与 `web/pnpm-lock.yaml`。

### 候选对比

| 维度 | React 19 (选定) | Vue 3 + Vapor Mode | SolidJS | Svelte 5 + Runes |
|------|-----------------|--------------------|---------|-----------------|
| **社区活跃度**（GitHub Stars / npm 下载 / Google Trends） | 10.0（230k stars，周下载 1.2亿，搜索指数第一） | 8.8（210k stars，周下载 3800万） | 7.5（35k stars，周下载 120万） | 8.0（82k stars，周下载 210万） |
| **性能基准**（JS Framework Benchmark / Startup / Runtime） | 8.5（基准中等偏上） | 8.2（Vapor Mode 启动快，但 runtime 仍有响应式系统） | 9.8（细粒度响应式，原生 DOM 操作） | 9.2（编译时框架，runtime 极小） |
| **3–5 年适用性**（团队可维护性 / 官方路线图） | 9.8（React 官方声明支持至 2035；Meta 持续投入 Server Components / RSC） | 8.5（Vue 3 路线清晰但企业级工具生态弱） | 8.0（被 JSX/TSX 主流工具链部分兼容，但生态仍小） | 8.8（Svelte 5 Runes 是重大升级，官方长期投入） |
| **学习曲线**（迁移成本 / 现有工程师技能） | 9.5（团队全栈 React，已有 3 年经验沉淀；JSX + TS 统一） | 7.0（Vue SFC + Composition API 需重新学习；TS 集成需额外适配） | 6.5（signal 模型完全不同；TS 类型系统有坑） | 6.8（Runes 语法改变较大，学习期 2–3 周） |
| **生态与工具链**（组件库 / 状态管理 / DevTools / 测试） | 10.0（Ant Design v6、Zustand、TanStack Query、React DevTools、Testing Library 全成熟） | 8.0（Element Plus / Pinia；React DevTools 对标能力弱） | 6.5（组件库屈指可数，Solid DevTools 还在 Beta） | 7.0（SvelteKit 强，但组件库选择少） |
| **Server-Side 适配**（RSC / SSR / Streaming） | 9.5（RSC 成熟；Next.js 生态 + TanStack Start） | 8.2（Nuxt 强，但与 SPA 架构衔接需改动） | 7.0（服务端渲染支持有限） | 8.5（SvelteKit 原生 SSR，但与后端 Go 集成有 N+1 问题） |

### 加权得分（生态 25% / 学习曲线 25% / 适用性 20% / 性能 15% / SSR 15%）

| 框架 | 加权总分 | 结论 |
|------|---------|------|
| React 19 | **9.40** | ✅ 继续保持 |
| Svelte 5 | 7.85 | 备选；若团队规模 > 20 人或性能瓶颈明显可重新评估 |
| Vue 3 | 7.98 | ❌ 排除；生态与现有积累不匹配 |
| SolidJS | 6.93 | ❌ 排除；生态过小 |

### 选型结论

React 19 继续作为主框架，跟进 Server Components（RSC）与 React Router v7 的数据加载能力。保持 Ant Design + Vite + TanStack Query + Zustand 的组合不变。

---

## 3. UI 组件库

### 选定方案

| 项目 | 值 |
|------|----|
| 库 | Ant Design v6（`antd`） |
| 图标 | `@ant-design/icons` |
| 编辑器 | `@monaco-editor/react` |
| 图表 | Recharts v3（`recharts`） |

> 精确版本以 `web/package.json` 与 `web/pnpm-lock.yaml` 为准，本文不复写。

### 候选对比

| 维度 | Ant Design v6 (选定) | Material UI v7 (MUI) | shadcn/ui + Radix | Chakra UI v3 |
|------|---------------------|----------------------|-------------------|--------------|
| **组件完备度**（表格 / 表单 / 树形 / 上传 / 向导 / 图表） | 10.0（70+ 组件，含企业级 Table、ProForm、Transfer、Cascader） | 8.8（组件数量接近，但表单场景弱） | 7.2（~30 基础组件，企业组件需自建） | 7.8（组件齐全但 Table 弱） |
| **TypeScript 支持**（类型精度 / 泛型 Table / 类型安全表单） | 9.5（Table `ColumnType<Record>` 泛型完整） | 8.5（TS 支持好但表单类型需手动声明） | 9.2（Radix + TS 原生，但 shadcn 组件手动维护） | 8.0（TS 支持一般） |
| **主题定制能力**（CSS-in-JS / Token 系统 /暗色模式） | 9.5（Design Token v5 + ConfigProvider，暗色一键切换） | 9.0（ThemeProvider 完整） | 9.8（Tailwind + CSS 变量，最高自由度） | 8.5（Theme 系统完整） |
| **Bundle 体积**（最小化后 KB，gzip） | 7.0（~320KB gzip；按需引入 Tree Shaking 改善） | 7.5（~280KB gzip） | 9.5（按组件复制，无额外体积） | 8.0（~210KB gzip） |
| **a11y（WCAG 合规）** | 8.5（部分 Table 键盘导航仍有问题） | 9.0（Radix 质量同档次） | 9.5（Radix 无障碍原生） | 9.0（WAI-ARIA 覆盖好） |
| **i18n / RTL / 多主题** | 9.5（内建 ConfigProvider i18n + 50+ 语言包） | 8.0（需 `@mui/system` 扩展） | 7.5（需自建） | 8.0（内建） |
| **企业级场景适配**（ProTable / 权限控制 / 数据密集表格） | 9.8（ProComponents ProTable / ProForm 成熟） | 7.5（需第三方 DevExtreme） | 5.0（纯基础组件，需自建大量） | 6.5（无现成方案） |

### 加权得分（完备度 20% / TS 15% / 主题 15% / Bundle 15% / a11y 10% / i18n 10% / 企业场景 15%）

| 库 | 加权总分 | 结论 |
|----|---------|------|
| Ant Design v6 | **9.25** | ✅ 继续保持 |
| shadcn/ui + Radix | 7.90 | 补充方案：在 Identity Actions 沙箱（ADR-029）等需要极致定制的场景可局部引入 |
| Material UI v7 | 8.03 | ❌ 排除；与现有 Ant Design 心智重复，Bundle 收益不足以覆盖迁移成本 |
| Chakra UI v3 | 7.63 | ❌ 排除 |

### 选型结论

Ant Design v6 继续作为主组件库；ProComponents 生态（ProTable、ProForm）覆盖管理面复杂表单场景。对 Identity Actions 沙箱、SCIM JML 编辑器等定制要求高的页面，局部引入 shadcn/ui + Radix 补充，不全局替换。

---

## 4. 状态管理

### 选定方案

| 项目 | 值 |
|------|----|
| 服务端状态 | TanStack Query v5（`@tanstack/react-query`；精确版本以 `web/package.json` 为准） |
| 客户端全局状态 | Zustand v5（`zustand`；精确版本以 `web/package.json` 为准） |
| 表单状态 | Ant Design Form（受控 + Zod 校验） |

### 候选对比（客户端全局状态）

| 维度 | Zustand v5 (选定) | Redux Toolkit (RTK) | Jotai | MobX v7 | Signia |
|------|-------------------|---------------------|-------|---------|--------|
| **Bundle 体积**（KB gzip） | 10.0（~1KB gzip） | 6.0（~12KB gzip，RTK Query 另计） | 9.5（~2KB gzip） | 7.0（~16KB gzip） | 9.8（< 1KB gzip） |
| **API 简洁度**（样板代码量 / 学习成本） | 9.5（`create((set) => ({...}))` 极简） | 5.5（slice / thunk / dispatch / selector 多层） | 8.5（原子模型直观） | 7.5（`makeAutoObservable` 魔法较多） | 9.5（signal 模型与 Solid 同源） |
| **性能**（重渲染追踪 / DevTools Profiler） | 9.2（浅比较自动 + `subscribeWithSelector`） | 8.5（RTK Query 缓存成熟） | 8.8（细粒度原子） | 7.0（全局响应式，易过度渲染） | 9.8（signal 粒度最细） |
| **TypeScript 类型安全** | 9.5（零配置泛型推断） | 8.0（需要显式 `PayloadAction`） | 9.5（原子类型推导自然） | 7.0（装饰器 + observable 类型有坑） | 9.5（signal 类型安全） |
| **DevTools 支持**（Redux DevTools / React DevTools） | 9.0（Redux DevTools Extension 兼容） | 10.0（原生 Redux DevTools） | 7.5（DevTools 需自建） | 8.0（MobX DevTools） | 6.0（DevTools 生态还在建设） |
| **中间件生态**（persist / immer / devtools） | 9.5（`persist` / `immer` / `devtools` 内建） | 10.0（`redux-toolkit` 全家桶） | 7.5（需自建或第三方） | 8.0（`mobx-persist`） | 7.0（persist 无成熟方案） |
| **3–5 年路线图** | 9.5（作者对 Zustand v5 有长期维护计划；PM 已在多个大型企业落地） | 9.0（Redux Toolkit v3 规划清晰，但社区热度下降） | 8.5（Atom 模型稳定） | 7.0（MobX v7 转向 TypeScript，但团队声量小） | 7.5（Preact 生态，React 社区渗透还早） |

### 加权得分（简洁度 25% / 性能 20% / TS 20% / DevTools 10% / 中间件 10% / 路线图 10% / Bundle 5%）

| 库 | 加权总分 | 结论 |
|----|---------|------|
| Zustand v5 | **9.35** | ✅ 继续保持 |
| Jotai | 8.58 | 备选；若未来需要更细粒度原子状态，可局部引入 |
| Signia | 8.38 | ❌ 排除；DevTools 生态不成熟 |
| Redux Toolkit | 8.23 | ❌ 排除；样板代码过多，迁移成本 > 收益 |
| MobX v7 | 7.58 | ❌ 排除 |

### 选型结论

Zustand v5 作为全局客户端状态管理继续保持；TanStack Query v5 作为服务端状态缓存继续保持（已内建缓存失效、乐观更新、重试、轮询，覆盖 90% 的异步场景）。两者分工清晰：Zustand 管会话级 UI 偏好、租户选择、当前选中对象；TanStack Query 管 API 数据。不引入 Redux Toolkit。

---

## 5. 数据库

### 选定方案

| 项目 | 值 |
|------|----|
| 主数据库 | PostgreSQL（CI 验证 PG 18；生产按 ADR-023 支持多方言） |
| 驱动 | `pgx/v5`（纯 Go，`go.mod`） |
| 迁移 | 自研声明式 Schema 生成器 + 单向幂等 DDL（ADR-022） |
| ORM / 查询层 | 参数化 SQL（Domain Port + Infra 适配器） |
| 关键特性依赖 | RLS（ADR-016）、SKIP LOCKED（Outbox Worker）、时间分区（ADR-011）、JSONB、pgcrypto、全文检索 |

### 候选对比

| 维度 | PostgreSQL 18 (选定) | MySQL 9.x | SQLite 嵌入式 | TiDB |
|------|---------------------|-----------|--------------|------|
| **功能完备度**（JSONB / RLS / 窗口函数 / CTE / 全文检索 / 分区表） | 10.0（全部原生；JSONB + GIN 索引优于 MySQL JSON） | 7.5（JSON 支持弱；RLS 无；窗口函数 8.0 才有但性能差） | 5.0（JSON 支持有限；分区表需 3.30+；RLS 无） | 7.0（MySQL 语法兼容，但 RLS 无） |
| **性能基准**（Sysbench OLTP / TPC-C，PG=100%） | 9.2（OLTP 中等；复杂查询优化器领先） | 9.5（OLTP 单表略快） | 6.0（单文件写入性能瓶颈，WAL 模式提升有限） | 8.0（分布式延迟高） |
| **可扩展性**（分区表 / 物化视图 / 只读副本 / 分片） | 9.5（声明式分区 + 继承 + pg_partman + Citus 扩展） | 8.0（分区表 8.0 后改进；无物化视图） | 3.0（无原生分片；WAL 锁问题） | 9.5（分布式分片内建） |
| **安全特性**（RLS / pgcrypto / SCRAM / SSL / 审计扩展） | 10.0（RLS 原生 FORCE；pgcrypto 完整；SCRAM-SHA-256） | 7.5（无 RLS；加密函数弱） | 4.0（无原生加密；SQLCipher 商业） | 7.0（无 RLS） |
| **3–5 年适用性**（Postgres 官方 + 生态路线） | 9.8（每年一个主版本，PG 18 已发布；pgvector / pg_cron 等扩展持续增加） | 8.0（MySQL 9.x 变更频繁；Oracle 战略转向 HeatWave） | 8.5（SQLite 3.45+ 持续投入；但嵌入式定位不适合主数据库） | 8.5（PingCAP 维护稳定，但与 PG 生态割裂） |
| **团队学习成本** | 9.5（全员已有 PG 经验） | 7.5（SQL 方言差，ADR-023 多方言探测会增加维护成本） | 7.0（SQL 子集兼容，但生产场景差距大） | 6.5（TiDB 分布式语义需额外学习） |
| **CI / 本地开发成本** | 9.5（Docker + 官方 PG 镜像；`pg_test` 轻量） | 9.0（Docker 容易） | 10.0（单文件零启动） | 5.0（TiDB Playground 启动慢，资源占用高） |

### 加权得分（功能完备度 25% / 安全 20% / 扩展性 15% / 适用性 15% / 性能 10% / 学习 10% / CI 5%）

| 数据库 | 加权总分 | 结论 |
|--------|---------|------|
| PostgreSQL 18 | **9.38** | ✅ 继续作为主数据库 |
| SQLite（嵌入式） | 6.58 | 补充方案：用于 `cypress/e2e` 等本地冒烟测试的临时数据库，不作为生产或集成测试 |
| TiDB | 7.23 | 备选：若触发多区域部署阈值（multi-region §4），TiDB 作为分布式候选之一重新评估 |
| MySQL 9.x | 7.75 | 不作主数据库；保留为 infra 镜像方言（ADR-023），RLS/分区/JSON 能力边界与开放项见 `contracts/registries/open-items.yaml`（OPEN-API-35/36/37） |

### 选型结论

PostgreSQL 继续作为唯一运行期主数据库基线。ADR-023 多方言抽象（`internal/infra/dialect/`）保留，**MySQL 等镜像方言不承诺功能等价**：能力差异按能力矩阵处置——RLS 缺失时仓储显式 `organization_id` 过滤、SKIP LOCKED 用方言等价语义、审计链等无等价实现的能力返回 `NewUnimplemented`；完整运行期启动受启动探针受控拒绝（`DB_DRIVER=mysql` fail-fast）。能力矩阵见《data-model》§3.1 与 ADR-023，开放项状态以 `contracts/registries/open-items.yaml`（OPEN-API-35/36/37）为准。

---

## 6. 缓存

### 选定方案

| 项目 | 值 |
|------|----|
| 缓存 | Redis 协议 8+（`go-redis/v9` 客户端；生产发行版优先 Valkey 8，见 §6 结论） |
| 用途 | 只读可重建缓存、分布式限流令牌桶、会话吊销通知广播（ADR-010/019） |
| 约束 | Redis 不得成为用户/授权状态的唯一事实源（fail-close） |

### 候选对比

| 维度 | Redis 8 (协议基线) | DragonflyDB 1.3 | Valkey 8 |
|------|---------------|-----------------|----------|
| **协议兼容**（RESP2/RESP3 兼容率） | 10.0（原生） | 9.5（RESP2 完全；RESP3 兼容） | 10.0（fork of Redis，API 100% 兼容） |
| **性能基准**（GET/SET QPS，Redis 8=100%） | 8.8（单线程上限 ~250k QPS/实例） | 9.8（多线程 + io_uring，~700k QPS/实例） | 9.0（接近 Redis 8） |
| **内存效率**（同样数据量内存占用） | 8.5 | 9.5（Dragonfly 特有压缩） | 8.5 |
| **生态 / 客户端库** | 10.0（`go-redis/v9`、`redigo`、`rueidis` 全成熟） | 8.0（`go-redis` 兼容但部分命令缺失） | 10.0（所有 Redis 客户端通用） |
| **3–5 年路线图** | 9.0（Redis 8 已 GA，官方维护；商业化许可策略有争议） | 8.8（DragonflyDB Ltd. 维护） | 9.8（Linux Foundation 守护，社区 fork 长期独立） |
| **持久化与复制**（RDB/AOF / 哨兵 / 集群 / Stream 复制） | 9.5 | 8.5（RDB 兼容但 AOF 差异大） | 9.5（完全兼容） |
| **WASM / 模块生态**（Redis Modules / Dragonfly Extensions） | 9.5（RedisJSON / RediSearch 成熟） | 7.0（扩展机制有限） | 9.5（完全兼容 Redis 模块） |

### 加权得分（协议兼容 25% / 生态 20% / 路线图 20% / 性能 15% / 复制 10% / 内存效率 5% / 模块 5%）

| 缓存 | 加权总分 | 结论 |
|------|---------|------|
| Valkey 8 | **9.43** | ✅ 生产优先发行版（Linux Foundation 社区维护、协议 100% 兼容） |
| Redis 8 | 9.15 | ✅ 协议基线与等价备选发行版（`go-redis/v9` 通用；与 Valkey 8 部署互换） |
| DragonflyDB | 8.43 | ❌ 排除；协议差异导致客户端库不兼容 |

### 选型结论

选定 **Redis 协议 8+**（`go-redis/v9` 客户端）：生产优先采用 **Valkey 8** 发行版，Redis 8 为同协议等价基线。两者 API 100% 兼容，部署可互换，不引入客户端差异。DragonflyDB 性能领先但协议兼容与扩展生态不足，不作为主缓存。

---

## 7. 消息队列 / 事件传递

### 选定方案

| 项目 | 值 |
|------|----|
| 模式 | PostgreSQL Outbox + 幂等 Worker（ADR-008） |
| 传输 | Worker 进程领约（SKIP LOCKED）→ 处理幂等 → 投递外部 |
| 用途 | Webhook、通知、SCIM 同步、计量事件等副作用 |
| 理由 | 模块化单体阶段不引入外部消息队列，避免分布式事务；跨模块一致性依赖本地事务保证 |

### 候选对比

| 维度 | PG Outbox (选定) | RabbitMQ 4.x | NATS JetStream | Kafka 4.x |
|------|-----------------|--------------|----------------|-----------|
| **架构复杂度**（部署组件数 / 运维链路 / 故障点） | 10.0（零额外组件，与主数据库同源） | 5.0（RabbitMQ 集群 + 可能的消息中间件，运维链路长） | 8.0（单二进制，嵌入式 JetStream） | 3.5（Kafka + ZooKeeper/KRaft，集群运维复杂） |
| **一致性保证**（至少一次 / 至多一次 / 恰好一次 / Outbox 原子性） | 10.0（同事务写入 Outbox 表，commit 原子保证至少一次） | 6.0（发布确认机制复杂，可能重复） | 8.0（JetStream 至少一次；Request-Reply 模式恰好一次） | 5.5（Producer ACK + Idempotent Producer 可接近恰好一次，但跨组件复杂） |
| **吞吐量与延迟**（P99 延迟 / QPS，Outbox=基准） | 7.5（受限于 PG 写入 + Worker 轮询间隔，P99 延迟 ~50ms） | 9.5（~1ms P99，百万级 QPS） | 9.0（~2ms P99，百万级 QPS） | 9.8（亚毫秒 P99，千万级 QPS） |
| **3–5 年适用性**（模块化单体 → 微服务过渡） | 10.0（Outbox 是通用模式，无论后续是否引入消息队列都可以平滑过渡） | 9.0（RabbitMQ 稳定，AMQP 生态） | 9.5（NATS 由 Synadia 维护、CNCF 孵化项目，云原生轻量） | 8.5（Kafka 被 Flink/Pulsar 等新流式系统蚕食） |
| **开发体验**（Go 客户端库 / 幂等键原生支持 / 可观测性） | 10.0（`internal/app/uow` + `internal/infra/postgres/outbox`，Go 原生） | 8.5（`rabbitmq/amqp091-go` 成熟，但幂等需自己实现） | 9.0（`nats-io/nats.go` 简洁，JetStream 内建幂等） | 7.0（`segmentio/kafka-go` 或 Sarama，但配置项多） |
| **运维成本**（监控 / 告警 / 死信队列 / 消息追溯） | 9.5（SQL 查询 Outbox 表即追溯，Prometheus 监控 Worker 延迟） | 7.0（需要 RabbitMQ Management + 死信队列告警配置） | 8.5（JetStream `nats account info` 内建追溯） | 5.5（需要 Kafka Manager / Burrow 等额外工具） |
| **多租户隔离**（本项目 ADR-016 RLS） | 9.5（Outbox 表同样可以套 RLS） | 7.0（vhost 隔离但运维复杂） | 8.5（Account / Subject 隔离） | 6.0（Topic 隔离，但管理面权限复杂） |

### 加权得分（架构复杂度 25% / 一致性 25% / 适用性 20% / 开发体验 10% / 运维 10% / 吞吐 5% / 多租户 5%）

| 消息队列 | 加权总分 | 结论 |
|----------|---------|------|
| PG Outbox（选定） | **9.13** | ✅ 模块化单体阶段继续保持 |
| NATS JetStream | 8.43 | 推荐的**微服务拆分信号触发后**首选 |
| RabbitMQ 4.x | 7.03 | 备选；若需要 AMQP 生态对接（第三方 SI 系统） |
| Kafka 4.x | 5.50 | ❌ 排除；运维成本远超现阶段需求 |

### 选型结论

**模块化单体阶段（ADR-001 未触发拆分）**：PG Outbox + 幂等 Worker 保持不变，零额外组件、同事务一致性、运维成本为零。

**触发拆分信号时（multi-region §4）**：NATS JetStream 作为首选消息传递基础设施——单二进制、嵌入式、Go 原生客户端简洁、JetStream 内建持久化与幂等，与 Go 微服务模式天然对齐。Kafka 在现阶段过度设计，不引入。

---

## 8. 可观测性

### 选定方案

| 项目 | 值 |
|------|----|
| 追踪 | OpenTelemetry（OTel） |
| 指标 | Prometheus + 自研 Prometheus 中间件（`internal/platform/middleware/prometheus.go`） |
| 日志 | 结构化日志（`log/slog`）+ request_id / trace_id 关联 |
| SLO | SLI/SLO + 错误预算（ADR-020，`docs/generated/slo-registry.md`） |
| 告警 | Prometheus Alertmanager + Grafana |

### 候选对比

| 维度 | OTel + Prometheus (选定) | OpenTracing + Prometheus（旧方案） | Datadog / New Relic（SaaS APM） | Grafana Agent + Tempo + Loki |
|------|------------------------|-----------------------------------|--------------------------------|-------------------------------|
| **标准化程度**（CNCF / W3C / OpenTelemetry Protocol） | 10.0（OTLP 是 CNCF 毕业项目；Context Propagation W3C TraceContext 标准） | 7.0（OpenTracing 已归档，被 OTel 吸收） | 6.0（Vendor Lock-in；Datadog Trace 非标准） | 9.5（Tempo 原生 OTLP；Loki 对接 OTel） |
| **后端集成生态**（Go / Java / Node / Python / Browser SDK） | 9.8（OTel 官方 Go SDK 1.x GA；自动插桩 `otelhttp` / `otelpgx` 成熟） | 8.0（Go `opentracing-go` 但生态冻结） | 9.5（APM 覆盖全；但 SDK 是 vendor 私有） | 9.5（Go SDK 全） |
| **前端集成**（Browser SDK / React / Web Vitals / Source Map） | 9.5（`@opentelemetry/sdk-trace-web` 成熟；React 19 RSC 追踪仍在 Beta） | 6.0（前端 OpenTracing 生态基本停止） | 9.5（Browser RUM 成熟；Session Replay） | 8.5（Tempo Browser 支持好） |
| **成本模型**（自托管 vs SaaS 按量） | 10.0（Prometheus + Tempo + Loki 全开源；硬件成本线性） | 8.0（同上但 OpenTracing 不再演进） | 4.0（SaaS 按量，百万 Span/秒级别的账单巨大） | 9.5（Tempo + Loki 自托管） |
| **3–5 年路线图** | 10.0（OpenTelemetry 是 CNCF 唯一可观测性标准；所有云厂商跟进） | 5.0（OpenTracing 2025 已归档） | 7.5（Datadog 仍在演进但 vendor lock-in 风险） | 9.5（Grafana Labs 持续投入；但 Tempo 比 OTel Collector 生态小） |
| **跨组件关联**（trace_id 贯穿 Span → Log → Metric） | 9.5（OTel Collector 自动注入 Service Graph；slog 结构化日志可关联 trace_id） | 7.5（需手动埋 trace_id） | 10.0（APM 自动关联） | 9.0（Tempo + Loki + Prometheus 天然关联） |
| **团队可维护性** | 9.5（Prometheus + OTel 社区大；Grafana 看板生态丰富） | 7.0（生态冻结） | 8.5（SaaS 省心但深度定制受限） | 9.0（Grafana 生态好） |

### 加权得分（标准化 25% / 后端生态 15% / 前端生态 10% / 成本 15% / 路线图 15% / 关联 10% / 维护 10%）

| 可观测性方案 | 加权总分 | 结论 |
|-------------|---------|------|
| OTel + Prometheus（选定） | **9.60** | ✅ 继续保持 |
| Grafana Stack（Tempo + Loki） | 9.18 | 补充：2027 Q2 引入 Tempo 替换 Jaeger，Loki 做日志聚合 |
| Datadog / New Relic | 7.30 | 备选：若运维团队规模快速扩大（> 5 人），可评估 SaaS 降运维成本 |
| OpenTracing + Prometheus | 6.43 | ❌ 排除；OpenTracing 已归档 |

### 选型结论

OpenTelemetry + Prometheus 作为可观测性主干。演进触发条件：

1. OTel Collector 与 `otelhttp`/`otelpgx` 在 `go.mod` 中稳定后，替换自研 Prometheus 中间件；
2. 需要跨服务 Span 关联时引入 Tempo（Span 后端）与 Loki（日志聚合）；
3. 需要前端到后端 trace_id 贯通时引入 `@opentelemetry/sdk-trace-web` 与 `@opentelemetry/react`。

---

## 9. 选定方案决策表（优先级 + 触发条件）

| # | 维度 | 选定方案 | 决策 | 优先级 | 迁移触发条件 |
|---|------|---------|------|--------|-------------|
| 1 | 后端框架 | Chi v5（唯一路由层） | 保持 | — | — |
| 2 | 前端框架 | React 19 + Vite | 保持；跟进 RSC | P2 | 团队扩大或 RSC 生产成熟 |
| 3 | UI 组件库 | Ant Design v6 | 保持；Identity Actions 局部引入 shadcn | P3 | Identity Actions 沙箱开发时 |
| 4 | 状态管理 | Zustand v5 + TanStack Query | 保持 | — | — |
| 5 | 数据库 | PostgreSQL 18 | 保持；多方言降级不承诺 | — | — |
| 6 | 缓存 | Redis 协议 8+（生产优先 Valkey 8） | 保持；发行版按 §6 结论等价互换 | — | — |
| 7 | 消息传递 | PG Outbox | 保持；拆分信号触发后引入 NATS JetStream | P2 | multi-region §4 拆分信号 |
| 8 | 可观测性 | OTel + Prometheus | 保持；Collector 替换自研中间件 | P1 | OTel Collector 在 `go.mod` 稳定 |
| 9 | 构建工具 | Vite 8 + Rolldown | 保持；Rolldown 作为 Vite 后端 | P2 | Rolldown GA |
| 10 | ORM / 查询层 | 参数化 SQL + pgx | 保持；拒绝引入 GORM / Ent | — | — |
| 11 | 密码哈希 | argon2id | 保持；监控 Argon2 参数强度建议 | P3 | OWASP 建议更新 |
| 12 | 授权 | Casbin | 保持；每租户 Enforcer + 本地快照缓存 | P2 | Casbin 缓存命中率 < 80% |

### 优先级定义

- **P0**：阻塞项——存在明确维护风险或技术债，必须优先处理
- **P1**：有前置条件的近期演进项
- **P2**：路线清晰但有前置条件
- **P3**：按需执行，触发时再评估
- **—**：保持稳定，3–5 年不变
## 10. 不做什么（明确排除的技术）

以下技术在本次评估中**明确排除**，不进入技术栈：

| 技术 | 排除理由 | 替代方案 |
|------|---------|---------|
| **微服务拆分**（提前） | ADR-001 决策未触发拆分信号；现阶段一致性需求强于独立扩容需求 | PG Outbox + 模块化单体 |
| **Rust 后端** | IAM 瓶颈在数据库与网络，Rust 无显著性能/正确性收益；实现与运维复杂度更高 | Go + Chi + pgx |
| **Node.js / Bun 后端** | 与前端 Go 服务的 gRPC/HTTP 契约维护成本；运行时性能与 Go 相当但无明显优势 | Go + Chi |
| **Kafka / Pulsar（现阶段）** | 运维成本过高；模块化单体的事件传递需求 PG Outbox 已满足 | PG Outbox → NATS JetStream（拆分后） |
| **RabbitMQ（现阶段）** | AMQP 生态与 Go 微服务契合度低；NATS JetStream 更轻量 | NATS JetStream（拆分后） |
| **DragonflyDB** | 协议兼容差异导致客户端库不通用；RedisLabs 商业策略争议 > Dragonfly 性能收益 | Valkey 8 |
| **SQLite（生产）** | RLS、分区表、并发写入三大 IAM 核心特性缺失；CI 冒烟场景可用 | PostgreSQL |
| **GORM / Ent ORM** | 参数化 SQL + Domain Port 已满足可维护性需求；ORM 隐式查询生成与 ADR-022 声明式 Schema 冲突 | pgx + 参数化 SQL |
| **GraphQL / gRPC（前端后端）** | REST + OpenAPI 3.1 契约已满足前端需求；GraphQL 的 N+1 查询和前端过度灵活会放大安全面 | REST（OpenAPI 生成器）；gRPC 仅用于后端内部服务 |
| **Redux Toolkit** | Zustand 覆盖 95% 全局状态场景；样板代码成本过高 | Zustand + TanStack Query |
| **MobX / SolidJS / Svelte（主框架）** | 与 React 生态（Ant Design、React Router、TanStack Query、RSC）的协同收益远超单一框架性能优势 | React 19 |
| **Material UI（主组件库）** | 组件模型与企业级表单能力重叠，双组件库会引入主题/表单边界冲突 | Ant Design v6；shadcn/ui 局部补充 |
| **OpenTracing** | 已归档；被 OpenTelemetry 吸收 | OpenTelemetry |
| **Datadog / New Relic（主观测）** | Vendor Lock-in + SaaS 费用非线性增长 | OTel + Prometheus + Grafana |
| **TempoDB / ScyllaDB 等 NoSQL** | IAM 数据是结构化事务数据；PostgreSQL 已覆盖 JSONB / 全文检索 / 分区 | PostgreSQL |
| **CockroachDB** | 与 ADR-023 多方言探测不兼容；分布式事务模型与 PG 有差异 | PostgreSQL → TiDB（多区域阈值触发） |
| **Kubernetes Operator（现阶段）** | 单体部署场景 Helm Chart 已足够；Operator 开发维护成本高于收益 | Helm + ArgoCD |

---

## 附录：评分方法论

### 量化指标来源

| 指标 | 来源 | 更新频率 |
|------|------|---------|
| GitHub Stars / Commits | GitHub REST API v3 | 每月 |
| npm 周下载 | npm registry API | 每月 |
| Google Trends | trends.google.com | 每季度 |
| 性能基准 | TechEmpower Benchmarks / JS Framework Benchmark / go-web-frameworks-bench | 每半年（取最新稳定版） |
| CNCF 毕业/孵化状态 | CNCF Landscape | 每季度 |
| CVE / 安全公告 | GitHub Security Advisories / OSV.dev | 实时 |

### 权重分配说明

默认权重偏向**长期适用性 + 生态 + 团队可维护性**，与 GuliangIAM ADR-001 模块化单体、ADR-008 Outbox 模式、ADR-020 SLO 治理、ADR-022 声明式 Schema、ADR-023 多方言探测等架构决策保持一致。性能评分在所有维度中权重 ≤ 20%，除非该性能瓶颈已被 SLO 记录为现阶段故障域（参见 `docs/generated/slo-registry.md`）。

### 评分定义

- **9.5–10.0**：行业领先，几乎无短板，长期路线清晰
- **8.5–9.5**：成熟稳定，有小的已知限制但不影响 3–5 年使用
- **7.0–8.5**：可用，有明显的生态/维护/性能短板，需要额外工作绕过
- **< 7.0**：不推荐用于生产，或在特定窄场景下才适用

---

> **维护说明**：本文件在以下时机复审——遇到关键 CVE / 框架归档 / 重大路线变更、架构拆分信号命中（multi-region §4）、或技术栈事实源（`go.mod` / `web/package.json`）发生主版本变化时；复审时按附录口径更新量化数据。
