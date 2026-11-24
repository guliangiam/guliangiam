# 审计哈希证据链

> 设计锚点：[DESIGN:infrastructure/audit]

> 目标态规范「审计哈希证据链」；写作约束见 [docs/README.md](../../README.md)。

---

## 定位

审计哈希证据链为 `records` 审计表构建不可篡改的链式哈希结构，支持离线验证与合规证据导出。

## 数据模型

链列与约束的完整声明以 `contracts/schema/60_audit.yaml` 为准（校验：`python3 scripts/gen_schema.py --check`）；下表只列链语义关键列。

| 落点 | 关键列 | 说明 |
|------|--------|------|
| `records` | `chain_id`、`chain_seq`、`prev_hash`、`entry_hash`、`chain_algo`、`chain_status` | 首条记录按部署档位选择 `sha256`/`sm3`，后续记录无条件继承链尾 `chain_algo`；`prev_hash` 为前一条 `entry_hash`（首条为 NULL）；同链内 `chain_seq` 从 0 起严格递增；`chain_status` 随锚定从 `pending`/`chained` 推进到 `anchored`，校验失败标记 `broken` |
| `audit_chain_anchors` | `chain_id`、`anchor_seq`、`entry_hash`、`proof`、`anchor_source`、`anchored_at` | 链段快照锚定：`anchor_seq` 为被锚定的链尾 `chain_seq`，`entry_hash` 为该链尾摘要，`proof` 为 `algo:hex` 确定性证明（可注入外部回执），`anchor_source` 标识锚定来源 |

约束：`uq_records_chain_seq`（organization_id+chain_id+chain_seq）保证同链序号唯一；`uq_audit_chain_anchors_org_chain_seq`（organization_id+chain_id+anchor_seq）防重锚；校验按 `chain_id` + `chain_seq` 升序扫描。

**链段划分**：每个组织一条链，`chain_id` 取组织 ID；锚点按当前链尾闭合一个链段，不按自然日滚动。

## 链构建模型（异步回填，不拖慢写入路径）

### 写入链路与并发串行化

- **入口**：全部审计写入经单一审计写入器组件收口（业务事务内调用），禁止旁路直插 `records`
- **同事务链计算**：写入器在业务事务内落记录体并同事务计算链列
- **串行化**：同链写入以 PostgreSQL 事务级 advisory lock（`pg_advisory_xact_lock(hashtext(chain_id))`）持锁串行，事务结束自动释放；跨组织分链天然并行
- **失败路径**：写入器异常不得阻塞业务主事务——审计转本地暂存，业务事务照常提交；暂存条目由链构建 Worker 重放补链
- **熔断**：连续失败超阈值后写入器熔断，新条目直接登记 `chain_status='pending'` 交 Worker 补链；恢复探测通过后回切同事务路径

### 锚定 Worker

按 `anchor_interval` / `verify_interval`（缺省值唯一事实源：`internal/platform/outbox/providers/audit/chain_worker.go` 的 NewChainWorker 默认参数，组合根可显式覆盖）对每链产出 `audit_chain_anchors` 快照：`anchor_seq` 取当前链尾 `chain_seq`，`entry_hash` 取链尾摘要；`proof` 为空时由写入器生成 `algo:hex` 确定性证明，外部时间戳回执可显式注入 `proof`。WORM 归档按链段导出（签名 + 清单校验和）。

## 查询与证据导出 API

| 接口 | 语义 |
|------|------|
| `GET /api/v1/records` | 分页查询；支持 `actorId` / `action` / `resourceType` / `requestId` / `keyword` / 排序 |
| `POST /api/v1/records/search` | 带时间窗的检索；支持 `requestId` 精确匹配 |
| `GET /api/v1/records/export` | 按相同过滤条件导出证据包：`manifest{count, sha256, generatedAt, filters}` + `records`；上限 10000 条，超限 422 |

导出包 `sha256` 为 `records` 数组规范 JSON 的 SHA-256；导出动作本身写入 `records.exported` 审计事件，包含条数与校验和。

## 验证算法

1. **段内校验**：按 `chain_seq` 升序校验同链 `chain_algo` 一致与 `prev_hash` 连续性（首条 `prev_hash` 必须为 NULL），再重算 `entry_hash = H_chain_algo(canonical)` 与存储值比对——任何单点篡改即哈希失配并**定位到首个失配 `chain_seq`**。`canonical` 为 `key=value` 行的固定顺序拼接（行间 `\n`、末尾无换行）：`organization_id`、`actor_id`、`action`、`resource_type`、`resource_id`、`ip`、`user_agent`、`surface`、`request_id`、`trace_id`、`metadata`（键排序）、`created_at`（UTC RFC3339Nano）、`prev_hash`（小写 hex）；`chain_seq` 不进入摘要。实现：`cmd/audit-verify -chain <organization-id>`（`platform/outbox/providers/audit.VerifyChainRecords`，DB 查询在 cmd 内），输出已验证条目数或首个失配 `chain_seq`。
2. **锚点比对**：按 `anchor_seq` 取对应 `records` 行，比对其 `entry_hash` 与 `audit_chain_anchors.entry_hash`；锚点闭环后不可变
3. **证据包导出**：链段记录 + 锚点 + 校验和清单打包，审计员侧独立重算校验

## 断链处置

校验工具命中断链：

1. 定位失配 seq 与受影响区间
2. 告警 + 冻结该链段锚定（防污染锚）
3. 处置留 `records`（安全事件工单流转）
4. 按受影响区间从锚点回放重建或整段作废重开新链（`chain_id` 轮转，旧段保留供取证）

## 拒绝与越权事件

| 类别 | 事件名 | 必填字段 |
|---|---|---|
| 权限拒绝 | `authz.deny` | actor、org、resource、action、reason、ip、request_id |
| 跨租户拒绝 | `authz.cross_tenant_denied` | actor、requested_org、target_org、reason、ip、request_id |
| 白名单外匿名 | `security.whitelist_miss` | path 模板、method、ip、request_id |
| 依赖不可用 | `unavailable` 决策 | 依赖名、fail_mode、ip、request_id |

拒绝事件与成功事件使用同一 `records` 写入器（同事务 + 哈希链）；拒绝事件不得携带被拒请求的敏感字段（密码、token、验证码）。

## 失败场景

| 场景 | 处置 |
|------|------|
| 链构建 Worker 积压 | 验证工具区分 `chain_status='pending'` 与真断链；积压指标告警 |
| 锚定服务不可用 | 锚定重试退避（参数以 `contracts/registries/outbox.yaml` 的 `backoff_strategy` 为唯一口径），段闭环延迟但段内校验不受影响 |
| 并发锚定/链轮转竞态 | `uq_audit_chain_anchors_org_chain_seq` 唯一仲裁 + advisory lock 串行化（`internal/infra/postgres/records/chain_sql.go`）；链段封存状态机（open/sealing/sealed）为目标态设计，运行期以唯一约束与 advisory lock 仲裁 |
| 物理删除与链完整矛盾 | **保留策略边界声明**：链定记录在对应链段锚定并完成合规汇聚前禁止物理删除；过期删除按链段整段处理（段级 WORM 归档替代行级删除） |

## 边界与复用

- 复用 `records` 既有表加列（[architecture/data-model.md §3.2](../../architecture/data-model.md#32-repository-约定)）；归档/保留策略以 `contracts/registries/lifecycle.yaml` 为唯一口径（段级 WORM 合并执行）；锚定重试退避以 `contracts/registries/outbox.yaml` 为唯一口径
- **不自研 TSA**（ADR-031 替代方案否决自建可信时间戳）
- 分期出口 = 链校验工具单点篡改命中并定位 + 审计员证据包校验和独立验证

## 互链

- Outbox 投递 → 《design/infrastructure/outbox.md》（审计写入事件可经 Outbox 扇出到 SIEM）
