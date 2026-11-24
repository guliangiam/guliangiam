# 支付域设计（GuliangPay）

> 设计锚点：[DESIGN:commerce/payment]

> 目标态规范「支付域设计」；写作约束见 [docs/README.md](../../README.md)。

---

## 1. 设计原则

| # | 原则 | 说明 |
|---|------|------|
| 1 | **渠道无关编排** | 编排层不感知任何渠道细节；所有渠道差异收敛在 SPI 适配器内 |
| 2 | **事实源唯一** | `orders` 持有应付金额，`payments` 持有支付执行事实，`pay_refunds` 持有退款事实，渠道机构持有资金事实 |
| 3 | **回调先收件后验签** | 原始报文先落地 `pay_callback_receipts`，验签与状态迁移在后续阶段执行；确保"存储失败不返回成功 ACK" |
| 4 | **并发退款控制** | `refunding_amount` 在支付行上做 SELECT FOR UPDATE 锁内校验 |
| 5 | **UNKNOWN 不推断** | 渠道状态未知时不得推断为成功或失败，必须由查询任务或对账任务收敛 |
| 6 | **配置版本化** | 渠道密钥与配置支持版本化、环境隔离、轮换走状态机 |
| 7 | **默认禁用** | 未完成沙箱验收的渠道保持 disabled，不得保留演示式伪实现 |

---

## 2. 能力矩阵

| # | 能力 | 目标态归属 | 约束 |
|---|------|-----------|------|
| G1 | `ChannelGateway` SPI 完整接口（8 方法 + 核心类型） | Domain `payments/channel.go` | C-10 → ADR-035 |
| G2 | 五阶段回调管线 | App `payments/callback_pipeline.go` | — |
| G3 | `pay_callback_receipts` 表 + RLS | Schema 定义 | 租户隔离强制 |
| G4 | `pay_attempts` 表（渠道调用留痕） | Schema 新增 | P-05（脱敏） |
| G5 | `pay_refunds` 表 + `partially_refunded` 状态 | Schema 新增 + Domain `refund.go` | P-04 |
| G6 | `pay_channel_configs` 表（版本化密钥） | Schema 新增 | P-06 + P-08 |
| G7 | `pay_recon_batches` + `pay_recon_items` 表 | Schema 新增 | — |
| G8 | `payments` 表扩展字段 + CHECK 约束 | Schema 定义 | 金额/状态一致性由 DDL 约束 |
| G9 | 并发退款控制（两字段 + 锁内校验） | App 层 `refund.go` | — |
| G10 | UNKNOWN 收敛（Query Worker） | App 层 `query.go` | — |
| G11 | 对账 Worker + 人工处置 | App 层 `reconcile.go` | P-12（审批流） |
| G12 | 支付页面挂 business 菜单分组 | 前端 `commerce-routes.tsx` | — |

---

## 3. 数据模型（声明式 Schema）

> **Schema 文件**：`payments` 与 pay_* 表声明于 `contracts/schema/70_commerce.yaml`；`contracts/schema/v3/71_payment.yaml` 为同组 pay_* 表的并行声明文件。两处声明一致性由 `python3 scripts/gen_schema.py --check` 与零差异门禁保证，本文不复制结构。

### 3.1 数据模型事实源

表清单（`table_name`）、列、约束、索引、软删除、revision、RLS 与外键声明以 `contracts/schema/70_commerce.yaml`（及 `contracts/schema/v3/71_payment.yaml`）的支付域声明为准（复现：`python3 scripts/gen_schema.py --check`）；正文不复制表结构与 YAML 示例。

**支付域语义约束**：

- **回调收件租户归属**：`pay_callback_receipts` 含 `organization_id` 并按 Schema 声明启用 RLS——回调的支付单属于某租户，收件属于同租户。
- **退款上限**：`payments` 的已退款与处理中金额上限由 Schema 声明的 CHECK 约束与应用层锁内校验双重保险。
- **状态机扩展**：在 `payments` 既有状态基础上新增 `partially_refunded`；状态值清单以 `contracts/registries/state-machines.yaml` 为准。裁决：
  - 全额退款 → `refunded`
  - 部分退款（已退款金额小于应付金额）→ `partially_refunded`
  - UNKNOWN 状态不落状态机——`pay_attempts.state=unknown` 记录瞬时状态，再由 Query Worker 收敛
- **`config_sealed` 读侧规范**：
  - 走 `shared.NewEncryptedFieldMapper()`（与平台信封加密字段同模式）
  - `config_set` 为 JSONB 数组（如 `["app_id", "private_key", "notify_url", "merchant_id", "webhook_secret"]`）
  - `config_fingerprint` 为 SHA-256 前 8 字节十六进制
  - 读侧 API 响应**不含** `config_sealed`，只含 `config_set` + `config_fingerprint`
- **脱敏规范**：`pay_attempts.request_payload` / `response_payload` 可能含 PII（如收货地址），脱敏规则引用 `internal/platform/logging/sanitizer.go`；支付域新增敏感字段须登记于 `contracts/registries/naming.yaml`，并复跑 `python3 scripts/gen_registry_md.py`。

### 3.2 跨方言兼容

| PG 类型 | MySQL 类型 | 涉及字段 | 说明 |
|---------|-----------|---------|------|
| BYTEA | LONGBLOB | `pay_callback_receipts.payload` | PG 二进制列；MySQL 用 LONGBLOB。由 `gen_schema.py` 按 DB_DRIVER 生成 |
| JSONB | JSON | 所有 JSONB 字段 | PG JSONB 支持 GIN 索引 + 键路径查询；MySQL JSON 只支持 JSON 函数。**MySQL 降级**：Headers/RawEvent/ConfigSet 在 MySQL 用 JSON + 业务层反序列化，不依赖 GIN 索引。Repository 接口返回 `map[string]interface{}` 或结构体，PG/MySQL mapper 各自处理 |
| DECIMAL(19,4) | DECIMAL(19,4) | 所有金额 | 无差异 |

---

## 4. ChannelGateway SPI 定义（Domain 层）

> **SPI 位置裁决**：决策依据 ADR-035。`ChannelGateway` 定义在 `internal/domain/payments/channel.go`（Domain 层）。编排层（App）只依赖 Domain Port，不感知渠道细节。ADR-035 记录完整决策。

### 4.1 接口

```go
// internal/domain/payments/channel.go

type ChannelGateway interface {
    Metadata() ChannelMetadata
    Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
    Query(ctx context.Context, req QueryRequest) (*QueryResult, error)
    Refund(ctx context.Context, req RefundRequest) (*RefundResult, error)
    QueryRefund(ctx context.Context, req QueryRefundRequest) (*QueryRefundResult, error)
    Close(ctx context.Context, req CloseRequest) (*CloseResult, error)
    ParseCallback(ctx context.Context, req CallbackRequest) (*CallbackEvent, error)
    VerifyConfig(ctx context.Context, config ChannelConfig) error
}
```

完整类型定义（`ChannelMetadata` / `ChargeRequest` / `ChargeResult` / `AttemptState` / `QueryRequest` / `QueryResult` / `RefundRequest` / `RefundResult` / `QueryRefundRequest` / `QueryRefundResult` / `CloseRequest` / `CloseResult` / `CallbackRequest` / `CallbackEvent` / `ChannelError` / `ErrorCategory`）见完整设计方案（审计 §3.2.1 不影响接口本身，只是位置需要 ADR 记录）。

### 4.2 注册表（`internal/bootstrap/payment_registry.go`）

按配置在 Bootstrap 中初始化，已有 `factory.go` 扩展为显式注册表：

```go
func Init(cfg *config.PaymentChannels) *ChannelRegistry {
    reg := &ChannelRegistry{gateways: make(map[string]payments.ChannelGateway)}
    if cfg.Balance.Enabled   { reg.Register("balance",   balance.NewGateway()) }
    if cfg.Alipay.Enabled    { reg.Register("alipay",    alipay.NewGateway()) }
    // ... 其余渠道
    return reg
}
```

### 4.3 各渠道适配器增强要点

渠道适配器（清单事实源：`internal/infra/payment/` 目录与 `channel_catalog.go`，含 square/checkout）按 SPI 补齐缺失方法：

| 渠道 | 优先级 | 补齐方法 |
|------|--------|---------|
| Balance / Alipay / WeChat Pay | P0 | ParseCallback + QueryRefund + Close + VerifyConfig |
| Stripe | P1 | ParseCallback + QueryRefund + Close + VerifyConfig |
| PayPal | P2 | ParseCallback + QueryRefund + Close + VerifyConfig |
| Adyen / Airwallex / Paddle / FastSpring / LemonSqueezy / Polar / Square / Checkout | P3 | 全部补齐 |
| GC | P3 | Query + Close |

---

## 5. 五阶段回调管线

### 5.1 管线流程

```
阶段 1 · 接收限制（public 端点，无会话认证）
  ├─ 全局限流（按 channel + IP）
  ├─ 请求体上限（按 channel 配置，默认 64KB）
  └─ 返回 HTTP 200 ACK（不代表已验签）
  ▼
阶段 2 · 原始收件（幂等落库）
  ├─ 计算 payload_sha256
  ├─ INSERT pay_callback_receipts(..., process_state='received')
  ├─ 唯一索引冲突 → 重复相同报文 → 直接返回渠道成功 ACK
  └─ DB INSERT 失败 → 返回渠道失败 ACK（存储失败不返回成功）
  ▼
阶段 3 · 验签解密（ChannelGateway.ParseCallback）
  ├─ 按 channel 加载 pay_channel_configs（active + production）
  ├─ 调用 gateway.ParseCallback(ctx, CallbackRequest{...})
  ├─ 验签失败 → UPDATE receipts SET process_state='rejected' + 返回渠道失败 ACK
  └─ 验签成功 → UPDATE receipts SET process_state='verified' + 继续
  ▼
阶段 4 · 事件规范化 + 去重
  ├─ UPSERT pay_callback_receipts(event_key=..., process_state='normalized')
  ├─ ON CONFLICT (channel, event_key) WHERE event_key IS NOT NULL DO NOTHING
  ├─ conflict → process_state='duplicated' → 返回渠道成功 ACK（幂等）
  └─ 唯一 → process_state='normalized' → 继续
  ▼
阶段 5 · 单事务状态迁移（同 UoW 事务）
  ├─ SELECT payments WHERE out_trade_no = ? FOR UPDATE
  ├─ 校验金额、币种、商户号
  ├─ INSERT pay_attempts(attempt_type='callback', ...)
  ├─ UPDATE payments SET status=?, channel_order_no=?, attempt_count=attempt_count+1, ...
  │   （按 TransitionTo 状态机校验，非法迁移拒绝）
  ├─ 若事件类型含 refund.* → INSERT pay_refunds + 更新 refunded_amount/refunding_amount
  ├─ UPDATE pay_callback_receipts SET process_state='accepted'
  ├─ OUTBOX: INSERT outbox(event_type='payment.succeeded', ...)
  ├─ 审计: records 同事务写
  └─ 提交事务 → 返回渠道成功 ACK
```

> **运行期边界**（事实源 `internal/app/payments/callback_pipeline.go`）：现行管线在 `ParseCallback` 内同一步完成验签与归一化，`received→normalized` 直达，**不单独落库 `verified` 中间态**（`verified` 为历史兼容态，闭集与转移表见 `state-machines.yaml` `pay_callback_receipts`）；去重为收件前 `payload_sha256` 预查短路（重复报文返回既有 receipt，不改其状态），`duplicated` 产生于 payment 已在目标状态的幂等分支，而非阶段 4 的 UPSERT 冲突；阶段 5 的 `FOR UPDATE`、`pay_attempts` 落库与 Outbox/审计同事务写入为目标态口径。状态赋值一律经 `CallbackReceipt.TransitionTo`。

### 5.2 幂等键设计

| 阶段 | 幂等键 | 唯一约束 |
|------|--------|---------|
| 原始收件 | `(channel, payload_sha256)` | `uq_receipts_channel_sha256` |
| 事件去重 | `(channel, event_key)` | `uq_receipts_channel_event_key` (partial WHERE event_key IS NOT NULL) |
| 支付幂等 | `Idempotency-Key`（请求级） | `uq_payments_idempotency_key`（payments 表，partial unique）；回调与支付单的关联按 `out_trade_no` 定位，约束/索引以 schema 声明为准 |
| 退款幂等 | `(channel, out_refund_no)` | `uq_pay_refunds_channel_out_no`（partial：`deleted_at IS NULL`） |

---

## 6. 并发退款控制

### 6.1 问题

并发退款请求可能导致超额退款：两个请求同时读到 `refunded_amount = 0`，各自校验通过，然后各自累加，最终超额。

### 6.2 三重保险

**支付行字段**：`refunding_amount`（处理中退款金额，初始 0）

```sql
-- 创建退款时（SELECT FOR UPDATE 锁支付行）
BEGIN;
SELECT id, price, refunded_amount, refunding_amount
FROM guliangiam.payments WHERE id = $1 FOR UPDATE;

-- 🔒 锁内校验（主防线）
-- refunded + refunding + new_refund ≤ price
-- 不满足 → RAISE EXCEPTION → 返回 errRefundExceedsBalance

INSERT INTO guliangiam.pay_refunds (payment_id, out_refund_no, amount, state='requested', ...)
VALUES ($1, $2, $3, ...);

UPDATE guliangiam.payments
SET refunding_amount = refunding_amount + $3,
    attempt_count = attempt_count + 1,
    revision = revision + 1
WHERE id = $1;

COMMIT;
```

**渠道退款成功回调时**：
```sql
UPDATE guliangiam.payments
SET refunded_amount = refunded_amount + $refund_amount,
    refunding_amount = refunding_amount - $refund_amount,
    revision = revision + 1
WHERE id = $1;
UPDATE guliangiam.pay_refunds SET state='succeeded' WHERE id = $refund_id;
```

**Schema CHECK 约束（双重保险）**：
```
ck_payments_refund_limit: refunded_amount + refunding_amount <= price
```

---

## 7. UNKNOWN 状态收敛

### 7.1 触发场景

| 场景 | 处理 |
|------|------|
| Charge 返回 `state=unknown` | `pay_attempts(state=unknown)`；`payments.status` 保持 `processing` |
| 24h 未回调 | Query Worker 主动查询 |
| 回调无法解析 | `process_state=rejected`；对账任务后续收敛 |

### 7.2 Query Worker 流程

```
每 5 分钟定时
  ├─ 查询 payments.status='processing' + 最近 attempt 为 unknown / 无 attempt 超 30min
  ├─ 按 channel 调用 ChannelGateway.Query
  ├─ succeeded → payments.TransitionTo('succeeded') + Outbox
  ├─ failed    → payments.TransitionTo('failed')
  ├─ unknown   → 保持，下次继续查询（最多 5 次，超次进入对账差异）
  └─ 更新 pay_attempts + 审计
```

---

## 8. 对账机制

### 8.1 对账流程

```
每日 02:00 UTC（定时任务）
  ├─ 对每个 enabled 渠道：
  │   ├─ 拉取前一日渠道账单
  │   ├─ 创建 pay_recon_batches(status='pending', batch_date=前一日)
  │   ├─ 逐笔与本地 payments/pay_refunds 对比
  │   ├─ 分类差异（missing_local / missing_channel / amount_mismatch / state_mismatch / refund_mismatch）
  │   ├─ 差异 → 人工处置队列
  │   └─ 批次状态 → completed / failed
  └─ 对账差异超阈值 → 告警通知
```

### 8.2 人工处置（需审批）

对账差异处置走统一审批流：
- 权限点：以路由声明（`internal/transport/http/api/v1/payments/routes.go`）与 `contracts/registries/permissions.yaml` 的 payments 域条目为准（`pay-recon-items` 登记 write/approve 动作；处置与复核分端点），本文不手写权限点变体
- 约束：`operator ≠ approvedBy`（处置人与审批人必须不同）
- 最终操作人和审批人均写入 `records` 审计

---

## 9. 跨域 Outbox 联动


支付域的状态迁移（`payments.succeeded` → `orders.paid`、`payments.refunded` → `orders.refunded`）通过 Outbox 实现最终一致性：

```
payments 域事务
  ├─ SELECT payments WHERE id = ? FOR UPDATE
  ├─ UPDATE payments SET status='succeeded'
  ├─ UPDATE pay_attempts SET state='succeeded' WHERE id = ?
  ├─ INSERT outbox (
  │     event_type='payment.succeeded',
  │     aggregate_type='payment',
  │     aggregate_id=?,
  │     payload={"order_id": ?, "amount": ?, "currency": ?}
  │   )
  └─ COMMIT
       │
       ▼ Outbox Worker（同 GuliangIAM 进程）
       │
       ▼ Consumer: orders 域订阅（幂等）
       ├─ SELECT orders WHERE id = ? FOR UPDATE
       ├─ 校验 order.status = 'payment_pending'（非法迁移拒绝）
       ├─ UPDATE orders SET status='paid'
       └─ COMMIT
```

### Outbox 事件清单（登记到审计事件注册表）

```yaml
payment:
  - event: payment.succeeded
    severity: info
    fields: [organization_id, payment_id, order_id, channel, amount, currency, channel_order_no]
  - event: payment.failed
    severity: info
    fields: [organization_id, payment_id, order_id, channel, error_code]
  - event: payment.refunded
    severity: info
    fields: [organization_id, payment_id, refund_id, amount, channel_refund_no]
  - event: payment.partially_refunded
    severity: info
    fields: [organization_id, payment_id, refund_id, amount, refunded_amount]
```

### metering 对接裁决

>
> **裁决**：支付金额**不写入** metering。metering 是商业计量域（`contracts/schema/90_metering.yaml`，见《[metering.md](metering.md)》），承载可计费业务事件与配额账期，不是支付流水管道；AI 网关用量计量（`ai.usage.*` 事件与 aigateway quotas/prices）归 AI 网关域（见《[design/ai-gateway/README.md](../ai-gateway/README.md)》）。支付事件写 `records`（审计），不写 `metering_events`。

---

## 10. Clean Architecture 四层映射

### 10.1 Domain 层（`internal/domain/payments/`）

在现有 `payment.go` / `gateway.go` / `repository.go` / `state_generated.go` 基础上新增：

```
internal/domain/payments/
├── payment.go              // 已有：Payment 聚合 + 6 状态状态机（扩展 partially_refunded）
├── gateway.go              // 已有：TranslateTradeStatus（扩展新渠道）
├── repository.go           // 已有：扩展 CallbackReceipts / Attempts / Refunds / Recon Port
├── channel.go              // 【新增】ChannelGateway SPI + 核心类型（ChargeRequest / CallbackEvent 等）
├── refund.go               // 【新增】Refund 聚合 + 5 状态状态机
├── callback_receipt.go     // 【新增】CallbackReceipt 实体 + 6 阶段 process_state 状态机
├── attempt.go              // 【新增】PayAttempt 实体
├── channel_config.go       // 【新增】PayChannelConfig 实体 + 版本化 4 状态状态机
├── recon.go                // 【新增】ReconBatch / ReconItem 实体
└── state_generated.go      // 已有（自动生成）
```

### 10.2 App 层（`internal/app/payments/`）

在现有基础上新增：

```
internal/app/payments/
├── service.go              // 已有 + 扩展：编排总入口
├── charge.go               // 【新增】发起支付编排（鉴权→路由渠道→Charge→attempts）
├── refund.go               // 【新增】退款编排（并发控制 + Refund + 状态迁移）
├── query.go                // 【新增】主动查询 + UNKNOWN 收敛（Query Worker）
├── callback_pipeline.go    // 【新增】五阶段回调管线
├── reconcile.go            // 【新增】对账编排 + 差异处置
├── channel_config.go       // 【新增】渠道配置版本化 + 沙箱验收
└── new.go                  // 已有（扩展注入 registry + recon worker 依赖）
```

### 10.3 Infra 层

```
internal/infra/
├── postgres/payments/          // PG Repository 扩展（新增表）
│   ├── sql.go                 // + 新增 CRUD SQL
│   ├── mapper.go
│   └── repository.go
├── mysql/payments/             // MySQL Repository 并行扩展（ADR-023）
│   ├── sql.go                 // BYTEA → LONGBLOB、JSONB → JSON
│   ├── mapper.go
│   └── repository.go
└── payment/                    // 渠道适配器增强（清单以目录为准）
    ├── registry.go             // 【新增】ChannelRegistry 实现
    ├── alipay.go               // 补齐 ParseCallback / QueryRefund / Close / VerifyConfig
    ├── wechat_pay.go           // 补齐（含平台证书轮换）
    ├── stripe.go               // 补齐（含 event.id 去重）
    ├── ...                     // 其余适配器逐步补齐
```

### 10.4 Transport 层（`internal/transport/http/api/v1/payments/`）

**路由声明**：

支付域路由以 `internal/transport/http/api/v1/payments/routes.go` 的 `route.Definition`（方法、路径、权限点、API 面）为准；生成物为 `openapi/openapi.yaml` 的 `management: payments` tag（复现：`python3 scripts/gen_openapi.py --check`）。正文不复制路由清单。
---

## 11. 前端设计（business 菜单分组）

### 11.1 路由分组：`web/src/app/router/commerce-routes.tsx`

支付页面与订单/商品/套餐等商业页面同属 `commerce-routes.tsx` 组装，菜单挂 **business** 分组（`web/src/features/navigation/menu.tsx`）；不存在独立的 `pay-routes.tsx` 或顶级 `/pay` 菜单。路由与菜单的唯一事实源为 `docs/design/frontend/registry.md` 生成块（复现：`python3 scripts/gen_frontend_registry.py --check`），本文不复制路由清单。支付页面族：支付单列表/创建/详情/结果、渠道配置、对账、回单调据（路径以 `commerce-routes.tsx` 为准）。

### 11.2 前端文件契约

| 文件/模块 | 职责 | 门禁 |
|-----------|------|------|
| `web/src/app/router/commerce-routes.tsx` | 商业域（含支付）路由组装 | `check-frontend-coverage.py` |
| `web/src/app/router/router.tsx` | 聚合 `...commerceRoutes` | `check-ui-wiring.py` |
| `web/src/features/navigation/menu.tsx` | business 分组 + 支付子菜单 | `menu.test.ts` |
| `docs/design/frontend/README.md` | 菜单树目标态 | `check-frontend-coverage.py` |

### 11.3 菜单树

支付子菜单位于 business 分组下（支付单 / 渠道配置 / 对账 / 回单调据）；菜单结构与计数以 `docs/design/frontend/registry.md` 生成块为唯一事实源，本文不复制。

---

## 12. 错误码登记

支付域错误码（`payment_*` 前缀）唯一事实源为 `contracts/registries/errors.yaml` 的 application 节（渲染层 `docs/generated/errors-registry.md`；复现：`python3 scripts/gen_registry_md.py && python3 scripts/check-registry-sync.py --strict`），本文不复制码表。

### 12.1 错误码登记与拒绝路径契约

| 错误码 | HTTP | 触发条件 | 重试语义 | 审计 |
|---|---|---|---|---|
| `payment_refund_exceeds_balance` | 422 | 退款金额超出可退余额 | 不重试（客户端修正） | 写 `records` + 拒绝原因 |
| `payment_refund_required` | 422 | 已支付订单直接取消 | 不重试 | 写 `records` |
| `payment_channel_not_activated` | 422 | 渠道配置未激活 | 不重试 | 写 `records` |
| `payment_consistency_mismatch` | 400 | 回调金额/币种/商户不一致 | 不重试，进对账队列 | 写 `records` + 告警 |
| `payment_receipt_storage_failed` | 500 | 回执落库失败 | 回调方按幂等键重试 | 写 `records` |
| `payment_verification_failed` | 400 | 回调验签失败 | 不重试 | 写 `records` + 安全告警 |
| `payment_channel_unavailable` | 503 | 渠道不可用 | 可重试（`Retry-After`） | 写 `records` |
| `payment_channel_config_invalid` | 400 | 渠道配置非法 | 不重试 | 写 `records` |
| `payment_invalid_transition` | 400 | 非法状态迁移 | 不重试 | 写 `records` |

登记纪律：上述错误码必须先登记 `contracts/registries/errors.yaml` 的 application 节、并在 `internal/platform/errors/errors.go` 有同名常量，端点方可返回；登记与实现同批交付，由 `check-registry-sync.py --strict` 双向断言。

实现落点：

| 错误码 | 代码路径 |
|---|---|
| `payment_refund_exceeds_balance` | `internal/app/payments/validation.go` `validateRefundAvailable` |
| `payment_refund_required` | `internal/app/orders/service.go` `CancelOrder`（`PaidAt != nil`） |
| `payment_invalid_transition` | `internal/app/payments/validation.go`（charge/refund/transition 三处守卫） |
| `payment_verification_failed` | `internal/app/payments/callback_pipeline.go`（`ParseCallback` 失败） |
| `payment_receipt_storage_failed` | `internal/app/payments/callback_pipeline.go`（回执落库失败） |
| `payment_consistency_mismatch` | `internal/app/payments/callback_pipeline.go` `advancePaymentFromEvent`（金额一致性） |
| `payment_channel_config_invalid` | `internal/app/payments/channel_config_service.go` `invalidChannelConfig` |
| `payment_channel_unavailable` | `internal/domain/payments/channel.go` `ErrChannelUnavailable` |

`payment_channel_not_activated`：`ChargeOrchestrator.Execute`（`ChannelConfigResolver.ResolveActive` 非 active → 422）；运行期端点 `POST /api/v1/payments/{id}/charge` 由 bootstrap `internal/bootstrap/payment_charge.go` 提供 active 配置解析（默认 production）；真实渠道 E2E 边界见 `OPEN-API-71`。

Charge 审计：`ChargeOrchestrator.persistChargeOutcome` 在事务内重读 payment、提交状态更新并经 `ChargeTx.EmbedAudit` 同事务写 records/outbox（成功 `payment.captured`、渠道失败 `payment.charge.failed`）；bootstrap 适配器见 `internal/bootstrap/payment_charge.go`（`chargeRunner`）。

---

## 13. 权限点登记

权限点唯一声明点为 `route.Definition.Permission`（`internal/transport/http/api/v1/payments/routes.go`）；资源/动作注册表为 `contracts/registries/permissions.yaml` 的 payments 域条目（payments / pay-recon / pay-recon-items / pay-channel-configs）。本文不手写资源名与动作清单（复现：`python3 scripts/gen_registry_md.py && python3 scripts/check-registry-sync.py --strict`）。

---

## 14. 状态机登记

状态机唯一事实源为 `contracts/registries/state-machines.yaml`（渲染层 `docs/generated/state-machine-registry.md`；复现：`python3 scripts/gen_registry_md.py`），本文不复制转移表：

| 状态机 | 注册表条目 / 声明 |
|--------|------------------|
| Payment 支付状态（`payments.status`，含 `partially_refunded`） | `payments` 条目；CHECK `ck_payments_status` |
| Refund 退款状态（`pay_refunds.state`） | `pay_refunds` 条目；CHECK `ck_pay_refunds_state` |
| PayChannelConfig 版本化状态（draft/testing/active/disabled） | `contracts/schema/70_commerce.yaml` 的 `ck_pay_channel_configs_status` |
| CallbackReceipt process_state（received/verified/rejected/duplicated/normalized/accepted） | `contracts/schema/70_commerce.yaml` 的 `ck_receipts_process_state` |

裁决语义：全额退款 → `refunded`；部分退款 → `partially_refunded`（可继续退至 `refunded`）；UNKNOWN 不落支付状态机，由 `pay_attempts.state=unknown` + Query Worker 收敛。

---

## 15. 命名空间登记


渠道闭集唯一事实源：`contracts/schema/20_identity.yaml` 的 `ck_providers_kind` payment 族（含 square/checkout）与运行期渠道目录 `internal/infra/payment/channel_catalog.go`；适配器清单以 `internal/infra/payment/` 目录为准（复现：`python3 scripts/check-provider-kinds.py`）。本文不复制渠道清单。

- **wxpay 是历史回调/数据别名**；API、UI、工厂注册键与新建数据统一用 `wechat_pay`。
- `attempt_state` / `recon_diff_type` 枚举以 `contracts/schema/70_commerce.yaml` 的 CHECK 声明与 domain 常量为准。
- 支付域敏感字段（银行卡号/CVV/有效期/持卡人等）脱敏规则引用 `internal/platform/logging/sanitizer.go`，新增敏感字段登记 `contracts/registries/naming.yaml`。

---

## 16. 渠道错误码映射


| 渠道 | 原始错误码 | 映射结果 | 分类 | 可重试 |
|------|-----------|---------|------|--------|
| Alipay | `ACQ.TRADE_NOT_EXIST` | `payment_channel_order_not_found` | NonRetryable | ❌ |
| Alipay | `ACQ.TRADE_STATUS_ERROR` | `payment_invalid_transition` | NonRetryable | ❌ |
| Alipay | `ACQ.SYSTEM_ERROR` | `payment_channel_unavailable` | Retryable | ✅ |
| Alipay | `ACQ.PRIVILEGE_ERROR` | `payment_balance_insufficient` | NonRetryable | ❌ |
| WeChat Pay | `ORDERNOTEXIST` | `payment_channel_order_not_found` | NonRetryable | ❌ |
| WeChat Pay | `TRADE_STATE_ERROR` | `payment_invalid_transition` | NonRetryable | ❌ |
| WeChat Pay | `SYSTEMERROR` | `payment_channel_unavailable` | Retryable | ✅ |
| Stripe | `resource_missing` (charge) | `payment_channel_order_not_found` | NonRetryable | ❌ |
| Stripe | `card_declined` | `payment_balance_insufficient` | NonRetryable | ❌ |
| Stripe | `rate_limit_exceeded` | `payment_channel_rate_limited` | Retryable | ✅ |
| Stripe | `api_connection_error` | `payment_channel_unavailable` | Retryable | ✅ |

**映射规则**：
- Retryable（网络超时、5xx、限流）→ 可重试 / 可换渠道
- NonRetryable（参数错误、签名失败、余额不足）→ 不可重试，直接返回
- Unknown（状态未知、渠道侧 IN_PROGRESS）→ `ErrorCategoryUnknown`，进入 UNKNOWN 收敛流程

---

## 17. 可观测性与告警

### 17.1 指标

| 指标 | 类型 | 标签 |
|------|------|------|
| `guliangpay_charge_total` | Counter | `channel`, `environment`, `state` |
| `guliangpay_refund_total` | Counter | `channel`, `state` |
| `guliangpay_callback_verified_total` | Counter | `channel`, `result` |

### 17.2 告警

| 告警 | 条件 | 级别 |
|------|------|------|
| 支付成功率下降 | 5min 内 succeeded/(succeeded+failed) < 95% | 高 |
| 回调验签失败率突增 | 5min 内 rejected/(verified+rejected) > 10% | 高 |
| UNKNOWN 堆积 | unknown > 10 且持续 30min | 中 |
| 对账差异超阈值 | 单日差异金额 > 10000 或 diff_items > 100 | 中 |
| Outbox 积压 | pending > 1000 持续 10min | 高 |

---

## 18. 验收条件（AC-PAY-001~010）


| AC 编号 | Given | When | Then | 门禁 |
|--------|-------|------|------|------|
| **AC-PAY-001** | Alipay 渠道配置（active + production） | 发起支付 Charge | 调用 Alipay SDK 返回 charge result（含 channel_order_no） | 集成测试 |
| **AC-PAY-002** | Alipay 回调报文到达 | 回调管线五阶段处理 | `payments.status=succeeded` + `pay_callback_receipts.process_state=accepted` + Outbox 投递 | 集成测试 |
| **AC-PAY-003** | 相同回调报文重复到达 | 第二次 POST 回调端点 | 返回 200 + `pay_callback_receipts.process_state=duplicated` + 不重复处理 | 幂等测试 |
| **AC-PAY-004** | 10 并发退款请求（同一 payment，总额 > price） | 全部同时发起 | 锁内校验拦截超额 → 返回 `payment_refund_exceeds_balance`；零超额通过 | 并发压测 |
| **AC-PAY-005** | 渠道返回 `state=unknown` | Charge 调用 | `payments.status=processing` + `pay_attempts.state=unknown`；Query Worker 15min 内收敛 | UNKNOWN 收敛测试 |
| **AC-PAY-006** | 渠道配置 `draft` 状态 | 尝试 Charge | 返回 `payment_channel_not_activated` | 集成测试 |
| **AC-PAY-007** | 渠道密钥轮换（`encryption_keys` 新增版本） | 用新密钥调用渠道 | 解密成功 + `config_sealed.key_version` 更新 | 密钥轮换测试 |
| **AC-PAY-008** | 对账批次创建 + 差异分类 | 人工处置差异 | 差异标记 resolved + records 审计写入 + operator ≠ approvedBy | 集成测试 |
| **AC-PAY-009** | 回调金额不匹配（渠道报的 amount ≠ payments.price） | 回调管线 | 阶段 5 一致性校验拒绝 → 返回 `payment_consistency_mismatch` | 集成测试 |
| **AC-PAY-010** | 已支付订单需取消 | 尝试 cancel | 返回 `payment_refund_required`（必须先退款） | 集成测试 |

---

## 19. 门禁对接清单


| 门禁 | 触发点 | 对接方式 |
|------|--------|---------|
| `check-schema.sh` | `contracts/schema/70_commerce.yaml`（新增表） | 完整 YAML 声明 + payments 增量字段 ALTER |
| `check-db-conventions.sh` | 新表 DDL + payments 增量字段 | 全部合规（UUIDv7 PK、外键索引、RLS ENABLE+FORCE、CHECK 约束命名 `ck_{table}_{column}`） |
| `check-openapi-contract.sh` | 支付域新增路由 | 路由定义含 operationId + permission |
| `check-api-contract.sh` | API 契约一致性 | 所有管理面路由有 OpenAPI 声明 |
| `check-layers.sh` | 新增代码 | Domain 只 import Platform；App 只 import Domain+Platform；Infra 只 import Domain+Platform |
| `check-i18n-keys.py` | 新增 i18n 键 | 菜单分组 + 状态枚举 + 渠道枚举在全部语言包一致 |
| `check-frontend-coverage.py` | 前端覆盖闭合 | commerce-routes.tsx 路由 ↔ 页面 ↔ menu.tsx 菜单闭合 |
| `check-ui-wiring.py` | 路由注释一致性 | commerce-routes.tsx 文件头 JSDoc 与实际路由一致 |
| `check-ac-traceability.py` | AC 追溯 | 全部 AC-PAY-* 由追溯门禁校验（矩阵见 `docs/traceability-matrix.md`） |
| `check-registry-sync.py` | 注册表同步 | 错误码/权限点/状态机/命名空间/审计事件与代码一致 |
| `check-naming.sh` | 命名规范 | channel 前缀统一；禁用拼写为零 |
| `check-sql-portability.sh` | SQL 可移植性 | Repository 层 PG/MySQL mapper 分离；BYTEA/LONGBLOB + JSONB/JSON 降级方案已覆盖 |

---



## 20. 关键设计裁决摘要

| 议题 | 裁决 | 理由 | ADR |
|------|------|------|-----|
| SPI 接口位置 | Domain 层 `payments/channel.go` | 编排层只依赖 Domain Port | ADR-035 |
| 回调策略 | 先收件后验签，原始报文先落 `pay_callback_receipts` | 防止 DB 慢导致渠道回调超时 | — |
| 并发退款 | `refunding_amount` + SELECT FOR UPDATE + CHECK 三重保险 | 应用层锁内校验主防线，CHECK 双重保险 | — |
| UNKNOWN | 不落状态机，由 Query Worker + 对账收敛 | 不得推断为成功/失败 | — |
| 支付状态机扩展 | 新增 `partially_refunded` | 部分退款（`refunded_amount < price`）进入此态，全额退款进入 `refunded` | — |
| 渠道配置 | 版本化 + 环境隔离 + 4 状态状态机 | 支持密钥轮换、沙箱/生产并行 | — |
| 对账 | 每日 02:00 UTC + 人工处置（需审批） | 自动对账有风险，人工处置确保每笔差异可追溯 | — |
| `config_sealed` 读侧 | EncryptedFieldMapper + ConfigSet + ConfigFingerprint | 与 GuliangIAM 7 组机密列同模式 | — |
| `pay_callback_receipts` RLS | **启用**（原设计矛盾） | 含 `organization_id` 必须启用 RLS | — |
| Schema 文件 | `70_commerce.yaml`（pay_* 表另有 `v3/71_payment.yaml` 并行声明） | 声明式 Schema 唯一事实源，一致性由生成器零差异门禁保证 | — |
| metering 对接 | 不写 metering | metering 是商业计量域（90_metering），不是支付流水管道；AI 用量归 ai-gateway 域 | — |
| Outbox 跨域联动 | payments → orders 通过 Outbox 最终一致 | 事务边界清晰，幂等消费 | — |
| 渠道错误码映射 | 独立于 AI 网关（`payment_` 前缀） | 避免跨域错误码冲突 | — |

## 互链

- 订单状态机 → [order.md](order.md)
- Outbox 投递 → [outbox.md](../infrastructure/outbox.md)
- 审计 → [audit.md](../infrastructure/audit.md)
