# ADR-033: Face ID Provider SPI 位置与形式

- 状态：[Current]（函数变量 + `SetFaceIdAdapters` 已在 `internal/app/auth/faceid.go`）
- 日期：2026-10-15

## 背景

GuliangIAM 需要对外部 Face ID Provider（阿里云 / 腾讯云 / 百度云 / 自建）做活体检测和人脸比对。SPI 形式涉及：Provider 抽象接口放在哪一层？采用 Go interface 还是函数变量？

同期相关决策：ADR-035（支付域 `ChannelGateway` SPI 放在 Domain 层、用 Go interface）。Face ID Provider SPI 需要裁决是否跟随同一模式。

## 决策

Face ID Provider SPI 暂时采用 **函数变量 + 依赖注入 setter**，留在 **App 层** `internal/app/auth/faceid.go`：

```go
var faceIdVerifyLivenessFunc faceIdVerifyLivenessFuncType
var faceIdCompareFaceFunc    faceIdCompareFaceFuncType

func SetFaceIdAdapters(
    verifyLivenessFn faceIdVerifyLivenessFuncType,
    compareFaceFn    faceIdCompareFaceFuncType,
)
```

两个函数签名（defined type，非别名；以 `internal/app/auth/faceid.go` 为准，未注入时默认实现返回 Unimplemented，fail-closed）：

```go
// 活体检测：返回 (isLive, score 0~1, providerSessionId, error)
type faceIdVerifyLivenessFuncType func(
    ctx context.Context,
    providerType, accessKeyID, accessKeySecret, endpoint string,
    imageData string, deviceInfo string,
) (isLive bool, score float64, sessionID string, err error)

// 人脸比对：返回 (isMatch, confidence 0~1, error)
type faceIdCompareFaceFuncType func(
    ctx context.Context,
    providerType, accessKeyID, accessKeySecret, endpoint string,
    imageData string, storedTemplate string,
) (isMatch bool, confidence float64, err error)
```

## 替代方案

### 替代方案 A：Domain 层 Go interface（ADR-035 模式）

在 `internal/domain/faceidproviders/face_id_provider.go` 定义 `FaceIdGateway` interface，由 Infra 层各 Provider 实现。

否决理由：

1. **使用方唯一** —— 只有 auth App 层调用，没有管理面独立调用（如健康检查、活体测试），也没有跨 App 层编排需求。Domain Interface 的主要收益（可被多个调用方依赖）在当前场景为零。
2. **Provider 接口天然成对** —— 活体检测 + 人脸比对必须在同一 Provider 上下文里执行（endpoint、AccessKeyID、AccessKeySecret、Config 阈值一致），分开两个 interface 反而把一个 Provider 绑定信息打散在两处。Domain Interface 没有 `SetFaceIdAdapters` 的绑定便利（同一个 Provider 同时提供两个函数）。
3. **函数变量足够满足 Clean Architecture** —— App 层依赖的是函数签名而非 infra 实现，依赖方向仍然是 App → Function Signature，没有 App → Infra 的反向依赖。函数变量与 Domain Interface 都满足 ADR-005 "依赖反转"的要求，只是抽象粒度不同。

### 替代方案 B：Platform 层 Go interface

否决理由：Platform 层存放跨域通用能力（`platform/crypto/`、`platform/ssrf/`、`platform/errors/`），Face ID 是 auth 域特有的能力，放 Platform 会污染 Platform 的"跨域通用"定位。

## 理由与后果

**为什么 Face ID 与支付域 SPI 不同构**——两个 SPI 的使用方分布不同：

| 维度 | 支付域 ChannelGateway (ADR-035) | Face ID Provider |
|------|---------------------------------|-----------------|
| 调用方数量 | ≥ 3（订单 / 支付回调 / 对账） | 1（auth App 层） |
| 是否需要跨 App 编排 | 是（订单创建 → 支付 → 回调 → 对账） | 否（Initiate → Verify → Complete 全在 auth 域） |
| 是否有独立管理面操作 | 是（渠道健康检查、手动退款） | 否（Face ID Provider 管理在 CRUD 接口） |
| 是否需要 Domain Event 订阅 | 是（支付成功 → 多域联动） | 否（Face ID 事件只有审计 outbox） |
| 结论 | Domain Interface 合理 | 函数变量足够 |

**后续演进路径**：如果以下任一条件成立，再上移到 Domain Interface：
1. 管理面需要独立调用 Provider 活体测试接口（不经过用户登录链路）
2. Face ID 作为 MFA 因子被其他认证方法编排（如"密码 + Face ID"强制双因子流程，需要 auth App 层 + MFA App 层同时依赖）
3. Provider 健康监控与活体可用性探测成为定期任务，需要独立的 `FaceIdGateway.HealthCheck(ctx)` 方法

## 验证证据

- 依赖方向：`check-layers.sh` 确认 App 层 `faceid.go` 只 import Domain + Platform（另含 `internal/app/uow` 端口），**不 import Infra**，也没有 import 外部 SDK（阿里云/腾讯云 SDK 封装在 setter 注入时实现，App 层不感知；复现：查看 `internal/app/auth/faceid.go` 的 import 块）。
- 函数变量模式先例：`internal/app/auth/faceid.go` 的注入点已存在（`pure_functions_v2_test.go:235` 有 `TestSetFaceIdAdapters_Nil` 覆盖 `SetFaceIdAdapters(nil, nil)` 路径）；支付域 ChannelGateway 当前在 Domain 层（ADR-035）是另一个先例——两者共存不冲突，各自适配自己的使用场景。
- 代码体量：Face ID 完整后端实现（init / verify / complete 三步 + session + provider + adapter）当前约 420 行，上移到 Domain 层拆分接口会增加约 80 行样板代码，收益为零。

## 复审条件

1. 上述"后续演进路径"任一条件成立（调用方增加 / 跨域编排 / 管理面独立操作）
2. 引入新的 Face ID 调用场景（如 WebAuthn 注册时要求活体检测、密码重置时要求 Face ID 确认）
3. 平台层要提供统一的 Provider 健康检查能力，需要抽象出可被 Face ID + Payment + 未来其他 Provider 共用的 `ProviderGateway`
