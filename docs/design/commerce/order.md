# 订单状态机

> 设计锚点：[DESIGN:commerce/order]

> 目标态规范「订单状态机」；写作约束见 [docs/README.md](../../README.md)。

---

## 状态机

订单状态机（状态列 `orders.status`，`CHECK` 约束 `ck_orders_status` 限定规范态；声明事实源 `contracts/schema/70_commerce.yaml`，状态机注册表 `contracts/registries/state-machines.yaml` 的 `orders` 条目；实现于 `app/orders/service.go`）：

```
+---------+
| created |  创建订单默认态（DDL DEFAULT 'created'）
+---------+
  |    |
  |    | 发起支付
  |    v
  | +-----------------+
  | | payment_pending |  合法中间态：进入即创建支付单，超时回 failed
  | +-----------------+
  |    |        |
  |    v        v
  | +------+  +--------+
  | | paid |  | failed |（支付失败，可重新发起 → payment_pending）
  | +------+  +--------+
  |    |
  |    v
  | +-----------+
  | | completed |
  | +-----------+
  |    |       |
  |    v       v
  | +----------+ +--------+
  | | refunded | | closed |
  | +----------+ +--------+
  |
  v（用户/管理员取消，仅未支付；已支付订单取消走 refunded）
+-----------+
| cancelled |
+-----------+
```

## 转换规则

| 转换 | 触发条件 |
|------|---------|
| `created → payment_pending` | 发起支付即进入（创建支付单）；超时回 `failed` |
| `payment_pending → paid` | 支付网关回调确认成功 |
| `payment_pending → failed` | 支付失败（验签失败/金额不符）；`failed → payment_pending` 允许重新发起 |
| `created → cancelled` | 用户/管理员取消（未支付）；拼写为双 l 的 **`cancelled`**（naming-registry N1） |
| `paid → completed` | 支付完成确认 |
| `paid → refunded` | 已支付订单退款（部分/全额） |
| `completed → refunded` | 部分/全额退款（已支付订单取消走 `refunded`） |
| `completed → closed` | 售后终结 |

**终态**：`cancelled`/`refunded`/`closed`，终态不可逆。**不存在 `expired` 出口**；`pending`、`cancel`（含单 l 变体）、`expired` 不是规范状态，禁止使用。

> **状态机唯一口径见 generated/state-machine-registry.md，本图为口径的可视化投影，冲突以注册表为准。**

**非规范值处置**：`CHECK` 约束 `ck_orders_status` 按注册表状态集拦截任意字符串；验收以 `check-db-conventions.sh` + 状态值枚举脚本为准。

## 并发与幂等边界

| 场景 | 规范 |
|---|---|
| 状态迁移 | 条件更新（`WHERE id = ? AND revision = ?`）；0 行受影响即返回 409 `conflict`，禁止读-改-写后无条件写 |
| 优惠券核销 | 以 `used_count < max_uses AND revision = ?` 条件更新；0 行返回 409 `conflict`，不得在应用层用内存计数判定 |
| 支付回调 | 统一幂等键（payment_id + event 标识）；重复回调返回缓存响应，不重复扣减/不重复发货 |
| 订单创建 | 支持 `Idempotency-Key`；同键重放返回同一订单，不产生第二笔订单 |
| 审计 | 状态迁移、核销冲突、幂等重放均写 `records`（事件名以 `contracts/registries/events.yaml` 为准） |
