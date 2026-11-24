# ADR-035: 支付域 ChannelGateway SPI 位置

- 状态：[Current]（`ChannelGateway` 已在 Domain 层定义，Infra 渠道适配器实现，App 编排只依赖 Domain Port）
- 日期：2026-10-14

## 背景

GuliangPay 增强方案定义 `ChannelGateway` SPI 接口（8 方法：Charge / Query / Refund / QueryRefund / Close / ParseCallback / VerifyConfig + Metadata），需裁决该接口定义在哪一层。现有 `PaymentGateway` 接口（4 方法）已在 Domain 层 `internal/domain/payments/gateway.go`，新 SPI 是其完整化。

## 决策

`ChannelGateway` 接口定义在 **Domain 层** `internal/domain/payments/channel.go`。

## 替代方案

- **定义在 Platform 层**（`internal/platform/payment/`）：否决——Platform 层存放跨域通用能力（`platform/ssrf`、`platform/crypto`、`platform/errors`），SPI 是支付域特有的能力抽象，放 Platform 会污染其「跨域通用」定位。注意否决理由是**职责边界**而非依赖方向：App 依赖 Platform 本身是合法的（App 可依赖 Domain + Platform），但支付域 SPI 不属于跨域通用能力。
- **定义在 Infra 层**：否决——App 编排层需要 import SPI 接口，而分层门禁禁止 App → Infra（`check-layers.sh` Rule 1）；Domain 也不依赖 Infra。正确形态是接口在 Domain、Infra 实现接口、组合根注入。

## 理由与后果

SPI 是支付域对外部渠道能力的抽象——它**属于支付域**，不是跨域通用能力。Domain 层定义 SPI 接口，Infra 层实现接口（每个渠道一个适配器），App 层编排只依赖 Domain Port。这完全符合 Clean Architecture 的依赖方向：

```
Domain（定义 SPI 接口）
  ↑
  │ 依赖 Domain Port
  │
App（编排层） → Domain
  ↑           ↑
  │           │ 实现 Domain Port
  │           │
Infra（渠道适配器）
```

## 验证证据

- 依赖方向零违规：`check-layers.sh` 确认 Domain 层只 import Platform（clock/uuid/errors），不 import Infra/Transport。
- 现有 `PaymentGateway`（4 方法）已在 Domain 层 `internal/domain/payments/gateway.go`，扩展为完整 SPI 是自然演化而非架构变更。
- Infra 层渠道适配器已符合"实现 Domain Port"模式（`internal/infra/payment/factory.go` 按渠道类型创建适配器实例；适配器数量以该文件 `case` 分支为准，不在本文手写计数）。

## 复审条件

若出现以下任一情况，复审本 ADR：
1. 非支付场景（如 IAM 的 Webhook 渠道）需要渠道适配器抽象
2. SPI 需要支持与支付域无关的操作（如批量渠道健康检查）
3. 支付域拆分独立进程后 SPI 边界变化
